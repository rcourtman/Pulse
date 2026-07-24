package truenas

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const defaultHTTPTimeout = 30 * time.Second

const maxResponseBodyBytes int64 = 4 * 1024 * 1024

const defaultRealtimeIntervalSeconds = 2

const defaultAppStatsIntervalSeconds = defaultRealtimeIntervalSeconds

const defaultDiskTemperatureAggregateWindowDays = 7

const defaultAppLogInitialWait = 2 * time.Second

const defaultAppLogIdleWait = 250 * time.Millisecond

const maxAppLogTailLines = 500

// ClientConfig configures the TrueNAS API client.
type ClientConfig struct {
	Host               string
	Port               int
	APIKey             string
	Username           string
	Password           string
	UseHTTPS           bool
	InsecureSkipVerify bool
	Fingerprint        string
	Timeout            time.Duration
}

// Client owns one connection-local TrueNAS transport decision. Supported
// current SCALE releases use JSON-RPC 2.0 over WebSocket; REST is retained
// only as a version-gated boundary for legacy SCALE and CORE appliances.
type Client struct {
	config     ClientConfig
	httpClient *http.Client
	baseURL    string
	rpcURL     string

	rpcMu     sync.Mutex
	rpc       *trueNASRPCClient
	mode      TransportMode
	closed    bool
	reconnect int

	statusMu sync.RWMutex
	status   TransportStatus

	// Tests exercise protocol behavior with httptest's plaintext websocket.
	// Production clients never set this escape hatch.
	allowInsecureRPC bool
}

// APIError represents an HTTP-level error from the legacy TrueNAS REST API.
type APIError struct {
	StatusCode int
	Method     string
	Path       string
	Body       string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("truenas request %s %s failed: status=%d body=%q", e.Method, e.Path, e.StatusCode, e.Body)
}

// NewClient creates a new TrueNAS API client.
func NewClient(config ClientConfig) (*Client, error) {
	host := strings.TrimSpace(config.Host)
	if host == "" {
		return nil, fmt.Errorf("truenas host is required")
	}

	useHTTPS, hostPort, err := resolveEndpoint(host, config.UseHTTPS, config.Port)
	if err != nil {
		return nil, err
	}

	timeout := config.Timeout
	if timeout <= 0 {
		timeout = defaultHTTPTimeout
	}

	transport := http.DefaultTransport.(*http.Transport).Clone()
	if useHTTPS {
		tlsConfig, err := buildTLSConfig(config.InsecureSkipVerify, config.Fingerprint)
		if err != nil {
			return nil, err
		}
		transport.TLSClientConfig = tlsConfig
	}

	config.Host = host
	config.UseHTTPS = useHTTPS
	config.Timeout = timeout
	if config.Port <= 0 {
		_, portText, splitErr := net.SplitHostPort(hostPort)
		if splitErr == nil {
			if parsedPort, parseErr := strconv.Atoi(portText); parseErr == nil {
				config.Port = parsedPort
			}
		}
	}

	scheme := "http"
	if useHTTPS {
		scheme = "https"
	}
	wsScheme := "ws"
	if useHTTPS {
		wsScheme = "wss"
	}

	client := &Client{
		config: config,
		httpClient: &http.Client{
			Timeout:   timeout,
			Transport: transport,
		},
		baseURL: fmt.Sprintf("%s://%s/api/v2.0", scheme, hostPort),
		rpcURL:  fmt.Sprintf("%s://%s/api/current", wsScheme, hostPort),
		mode:    TransportUnknown,
	}
	client.status = TransportStatus{
		Mode:     TransportUnknown,
		Endpoint: client.rpcURL,
		TLS:      useHTTPS,
	}
	return client, nil
}

// TestConnection validates that the endpoint is reachable and authenticated.
func (c *Client) TestConnection(ctx context.Context) error {
	if _, err := c.GetSystemInfo(ctx); err != nil {
		return fmt.Errorf("truenas connection test failed: %w", err)
	}
	return nil
}

// Close releases the persistent websocket session and idle HTTP transport
// connections held by the client.
func (c *Client) Close() {
	if c == nil {
		return
	}
	c.rpcMu.Lock()
	c.closed = true
	c.closeRPCLocked()
	c.rpcMu.Unlock()
	c.updateTransportStatus(func(status *TransportStatus) {
		status.Connected = false
	})
	if c.httpClient != nil && c.httpClient.Transport != nil {
		if transport, ok := c.httpClient.Transport.(interface{ CloseIdleConnections() }); ok {
			transport.CloseIdleConnections()
		}
	}
}

// GetSystemInfo returns high-level system metadata.
func (c *Client) GetSystemInfo(ctx context.Context) (*SystemInfo, error) {
	var response systemInfoResponse
	mode, err := c.ensureTransport(ctx)
	if err != nil {
		return nil, err
	}
	if mode == TransportLegacyREST {
		err = c.getJSON(ctx, http.MethodGet, "/system/info", &response)
	} else {
		err = c.callRPC(ctx, "system.info", []any{}, &response)
	}
	if err != nil {
		return nil, err
	}
	c.updateTransportStatus(func(status *TransportStatus) {
		status.ApplianceVersion = strings.TrimSpace(response.Version)
	})
	return systemInfoFromResponse(response), nil
}

func systemInfoFromResponse(response systemInfoResponse) *SystemInfo {
	// MachineID is the raw DMI serial only. It must never fall back to the
	// reported hostname: hostname is not a machine identity, and the old
	// fallback gave two serial-less systems that report the same hostname
	// identical machine keys, fully merging them in the resource registry
	// (#1573, #1575).
	machineID := strings.TrimSpace(response.SystemSerial)

	build := strings.TrimSpace(response.BuildTime.String())
	if build == "" {
		build = strings.TrimSpace(response.Version)
	}
	cpuCount := response.PhysicalCores
	if cpuCount <= 0 {
		cpuCount = response.Cores
	}

	return &SystemInfo{
		Hostname:         strings.TrimSpace(response.Hostname),
		Version:          strings.TrimSpace(response.Version),
		Build:            build,
		UptimeSeconds:    response.UptimeSeconds.Int64(),
		Healthy:          true,
		MachineID:        machineID,
		CPUCount:         cpuCount,
		MemoryTotalBytes: response.Physmem,
	}
}

// GetSystemTelemetry retrieves live system telemetry from the modern TrueNAS
// JSON-RPC websocket API. This path is best-effort; callers should treat any
// transport or endpoint failure as "telemetry unavailable" rather than a
// system identity failure.
func (c *Client) GetSystemTelemetry(ctx context.Context) (*SystemInfo, error) {
	var telemetry *SystemInfo
	var temperatures map[string]float64
	err := c.withRPC(ctx, func(rpc *trueNASRPCClient) error {
		temperatures, _ = rpc.getSystemTemperatures(ctx)
		subscriptionName := fmt.Sprintf("reporting.realtime:{\"interval\":%d}", defaultRealtimeIntervalSeconds)
		subscriptionID, err := rpc.subscribe(ctx, subscriptionName)
		if err != nil {
			return err
		}
		telemetry, err = rpc.readSystemTelemetryEvent(ctx, defaultRealtimeIntervalSeconds)
		if err != nil {
			return discardRPCSessionForStreamError(err)
		}
		return rpc.unsubscribe(ctx, subscriptionID)
	})
	if err != nil {
		return nil, err
	}
	if len(temperatures) > 0 {
		telemetry.TemperatureCelsius = cloneTemperatureMap(temperatures)
	}
	return telemetry, nil
}

// GetSystemMetricHistory retrieves historical system metrics from the native
// TrueNAS reporting API for the canonical host-chart fallback path.
func (c *Client) GetSystemMetricHistory(ctx context.Context, duration time.Duration) (*SystemMetricHistory, error) {
	var history *SystemMetricHistory
	err := c.withRPC(ctx, func(rpc *trueNASRPCClient) error {
		var err error
		history, err = rpc.getSystemMetricHistory(ctx, duration)
		return err
	})
	return history, err
}

// GetPools returns storage pools.
func (c *Client) GetPools(ctx context.Context) ([]Pool, error) {
	legacy, err := c.useLegacyREST(ctx)
	if err != nil {
		return nil, err
	}
	if legacy {
		return c.getPoolsREST(ctx)
	}
	return c.getPoolsRPC(ctx)
}

func (c *Client) getPoolsRPC(ctx context.Context) ([]Pool, error) {
	var response []map[string]any
	if err := c.queryRPC(ctx, "pool.query", &response); err != nil {
		return nil, err
	}

	pools := make([]Pool, 0, len(response))
	for _, item := range response {
		pool, ok := parsePoolState(item, false)
		if ok {
			pools = append(pools, pool)
		}
	}
	return pools, nil
}

// GetBootPool returns the boot pool from boot.get_state. The endpoint is
// separate from pool.query on supported CORE and SCALE releases.
func (c *Client) GetBootPool(ctx context.Context) (*Pool, error) {
	var response map[string]any
	legacy, err := c.useLegacyREST(ctx)
	if err != nil {
		return nil, err
	}
	if !legacy {
		if err := c.callRPC(ctx, "boot.get_state", []any{}, &response); err != nil {
			return nil, err
		}
		if pool, ok := parseBootPoolState(response); ok {
			return &pool, nil
		}
		return nil, fmt.Errorf("boot.get_state returned no boot pool identity")
	}

	response = nil
	if err := c.getJSON(ctx, http.MethodGet, "/boot/get_state", &response); err != nil {
		return nil, err
	}
	if pool, ok := parseBootPoolState(response); ok {
		return &pool, nil
	}
	return nil, fmt.Errorf("boot/get_state returned no boot pool identity")
}

func parseBootPoolState(item map[string]any) (Pool, bool) {
	return parsePoolState(item, true)
}

func parsePoolState(item map[string]any, isBoot bool) (Pool, bool) {
	if len(item) == 0 {
		return Pool{}, false
	}
	name := strings.TrimSpace(readStringAny(item, "name", "id"))
	id := strings.TrimSpace(readStringAny(item, "id", "name"))
	if name == "" {
		name = id
	}
	if id == "" || id == "0" {
		id = name
	}
	if name == "" {
		return Pool{}, false
	}

	status := strings.TrimSpace(readStringAny(item, "status", "state"))
	if status == "" && readBoolAny(item, "healthy") {
		status = "ONLINE"
	}
	properties := readMapAny(item, "properties")
	topology := item["topology"]
	if topology == nil {
		topology = item["groups"]
	}
	vdevs, diskMembers := poolTopologyFromTopology(topology)
	readErrors, writeErrors, checksumErrors := poolErrorTotals(topology)

	pool := Pool{
		ID:             id,
		GUID:           strings.TrimSpace(readStringAny(item, "guid")),
		Name:           name,
		Status:         status,
		StatusCode:     strings.TrimSpace(readStringAny(item, "status_code", "statusCode")),
		StatusDetail:   strings.TrimSpace(readStringAny(item, "status_detail", "statusDetail")),
		TotalBytes:     readInt64Any(item, "size", "total", "total_bytes", "totalBytes"),
		UsedBytes:      readInt64Any(item, "allocated", "used", "used_bytes", "usedBytes"),
		FreeBytes:      readInt64Any(item, "free", "free_bytes", "freeBytes", "available"),
		ReadErrors:     readErrors,
		WriteErrors:    writeErrors,
		ChecksumErrors: checksumErrors,
		Scan:           poolScanFromAny(item["scan"]),
		VDevs:          vdevs,
		IsBoot:         isBoot,
		DiskMembers:    diskMembers,
	}
	if pool.TotalBytes == 0 {
		pool.TotalBytes = readInt64Any(properties, "size", "total", "total_bytes", "totalBytes")
	}
	if pool.UsedBytes == 0 {
		pool.UsedBytes = readInt64Any(properties, "allocated", "used", "used_bytes", "usedBytes")
	}
	if pool.FreeBytes == 0 {
		pool.FreeBytes = readInt64Any(properties, "free", "free_bytes", "freeBytes", "available")
	}
	return pool, true
}

func mergeBootPool(pools []Pool, boot Pool) []Pool {
	bootID := strings.TrimSpace(boot.ID)
	bootName := strings.TrimSpace(boot.Name)
	for i := range pools {
		poolID := strings.TrimSpace(pools[i].ID)
		poolName := strings.TrimSpace(pools[i].Name)
		if (bootID == "" || poolID == "" || bootID != poolID) &&
			(bootName == "" || poolName == "" || bootName != poolName) {
			continue
		}
		pools[i].IsBoot = true
		if boot.Status != "" {
			pools[i].Status = boot.Status
		}
		if boot.TotalBytes > 0 {
			pools[i].TotalBytes = boot.TotalBytes
			pools[i].UsedBytes = boot.UsedBytes
			pools[i].FreeBytes = boot.FreeBytes
		} else {
			if boot.UsedBytes > 0 {
				pools[i].UsedBytes = boot.UsedBytes
			}
			if boot.FreeBytes > 0 {
				pools[i].FreeBytes = boot.FreeBytes
			}
		}
		if len(boot.DiskMembers) > 0 {
			pools[i].DiskMembers = append([]PoolDiskMember(nil), boot.DiskMembers...)
		}
		if len(boot.VDevs) > 0 {
			pools[i].VDevs = append([]PoolVDev(nil), boot.VDevs...)
		}
		if boot.GUID != "" {
			pools[i].GUID = boot.GUID
		}
		if boot.StatusCode != "" {
			pools[i].StatusCode = boot.StatusCode
		}
		if boot.StatusDetail != "" {
			pools[i].StatusDetail = boot.StatusDetail
		}
		if boot.Scan != nil {
			scan := *boot.Scan
			pools[i].Scan = &scan
		}
		pools[i].ReadErrors = boot.ReadErrors
		pools[i].WriteErrors = boot.WriteErrors
		pools[i].ChecksumErrors = boot.ChecksumErrors
		return pools
	}
	return append(pools, boot)
}

// poolDiskMembersFromTopology collects the leaf disks of a pool.query
// topology object across every vdev group, including detached/unavailable
// members that only appear through their unavail_disk datastore row.
func poolDiskMembersFromTopology(topology any) []PoolDiskMember {
	_, members := poolTopologyFromTopology(topology)
	return members
}

// poolTopologyFromTopology flattens native vdev topology without discarding
// group roles or parentage. It separately returns leaf membership for the
// existing disk-enrichment path.
func poolTopologyFromTopology(topology any) ([]PoolVDev, []PoolDiskMember) {
	groups, ok := topology.(map[string]any)
	if !ok {
		return nil, nil
	}

	groupNames := make([]string, 0, len(groups))
	for groupName := range groups {
		groupNames = append(groupNames, groupName)
	}
	sort.Strings(groupNames)

	var vdevs []PoolVDev
	var members []PoolDiskMember
	var walk func(node any, role, parentID, position string)
	walk = func(node any, role, parentID, position string) {
		vdev, ok := node.(map[string]any)
		if !ok {
			return
		}

		stats := readMapAny(vdev, "stats")
		device := strings.TrimSpace(readStringAny(vdev, "device"))
		path := strings.TrimSpace(readStringAny(vdev, "path"))
		if device == "" {
			device = strings.TrimPrefix(path, "/dev/")
		}
		disk := strings.TrimSpace(readStringAny(vdev, "disk"))
		unavailableDisk, hasUnavailableDisk := vdev["unavail_disk"].(map[string]any)
		if disk == "" && hasUnavailableDisk {
			disk = strings.TrimSpace(readStringAny(unavailableDisk, "devname", "name"))
		}
		if disk == "" {
			disk = wholeDiskFromDevice(device)
		}
		vdevType := strings.TrimSpace(readStringAny(vdev, "type"))
		state := strings.TrimSpace(readStringAny(vdev, "status", "state"))
		guid := strings.TrimSpace(readStringAny(vdev, "guid"))
		name := strings.TrimSpace(readStringAny(vdev, "name"))
		if name == "" {
			name = firstNonEmptyString(disk, device, path, vdevType, role)
		}
		id := guid
		if id == "" {
			id = role + ":" + position + ":" + name
		}
		readErrors := readInt64Any(vdev, "read_errors", "readErrors")
		writeErrors := readInt64Any(vdev, "write_errors", "writeErrors")
		checksumErrors := readInt64Any(vdev, "checksum_errors", "checksumErrors")
		if readErrors == 0 {
			readErrors = readInt64Any(stats, "read_errors", "readErrors")
		}
		if writeErrors == 0 {
			writeErrors = readInt64Any(stats, "write_errors", "writeErrors")
		}
		if checksumErrors == 0 {
			checksumErrors = readInt64Any(stats, "checksum_errors", "checksumErrors")
		}
		message := strings.TrimSpace(readStringAny(vdev, "status_detail", "statusDetail", "message"))
		missing := hasUnavailableDisk || strings.EqualFold(vdevType, "unavail_disk")

		vdevs = append(vdevs, PoolVDev{
			ID:             id,
			ParentID:       parentID,
			GUID:           guid,
			Name:           name,
			Type:           vdevType,
			Role:           role,
			Disk:           disk,
			Device:         device,
			Path:           path,
			Status:         state,
			ReadErrors:     readErrors,
			WriteErrors:    writeErrors,
			ChecksumErrors: checksumErrors,
			Missing:        missing,
			Message:        message,
		})

		children, hasChildren := vdev["children"].([]any)
		if hasChildren && len(children) > 0 {
			for index, child := range children {
				walk(child, role, id, position+"."+strconv.Itoa(index))
			}
			return
		}

		member := PoolDiskMember{
			Disk:           disk,
			Device:         device,
			Path:           path,
			GUID:           guid,
			Type:           vdevType,
			Role:           role,
			Status:         state,
			Missing:        missing,
			ReadErrors:     readErrors,
			WriteErrors:    writeErrors,
			ChecksumErrors: checksumErrors,
			Message:        message,
		}
		if member.Disk == "" && member.Device == "" {
			return
		}
		members = append(members, member)
	}

	for _, groupName := range groupNames {
		group := groups[groupName]
		nodes, ok := group.([]any)
		if !ok {
			continue
		}
		for index, node := range nodes {
			walk(node, groupName, "", strconv.Itoa(index))
		}
	}
	return vdevs, members
}

func poolErrorTotals(topology any) (int64, int64, int64) {
	groups, ok := topology.(map[string]any)
	if !ok {
		return 0, 0, 0
	}
	var readErrors, writeErrors, checksumErrors int64
	for _, group := range groups {
		nodes, ok := group.([]any)
		if !ok {
			continue
		}
		for _, node := range nodes {
			vdev, ok := node.(map[string]any)
			if !ok {
				continue
			}
			stats := readMapAny(vdev, "stats")
			readValue := readInt64Any(vdev, "read_errors", "readErrors")
			writeValue := readInt64Any(vdev, "write_errors", "writeErrors")
			checksumValue := readInt64Any(vdev, "checksum_errors", "checksumErrors")
			if readValue == 0 {
				readValue = readInt64Any(stats, "read_errors", "readErrors")
			}
			if writeValue == 0 {
				writeValue = readInt64Any(stats, "write_errors", "writeErrors")
			}
			if checksumValue == 0 {
				checksumValue = readInt64Any(stats, "checksum_errors", "checksumErrors")
			}
			readErrors += readValue
			writeErrors += writeValue
			checksumErrors += checksumValue
		}
	}
	return readErrors, writeErrors, checksumErrors
}

func poolScanFromAny(raw any) *PoolScan {
	scan, ok := raw.(map[string]any)
	if !ok || len(scan) == 0 {
		return nil
	}
	function := strings.TrimSpace(readStringAny(scan, "function"))
	state := strings.TrimSpace(readStringAny(scan, "state"))
	if function == "" && state == "" {
		return nil
	}
	result := &PoolScan{
		Function:              function,
		State:                 state,
		Percentage:            readFloatAny(scan, "percentage", "percent"),
		Errors:                readInt64Any(scan, "errors"),
		BytesExamined:         readInt64Any(scan, "bytes_examined", "bytesExamined", "processed_bytes", "processedBytes"),
		BytesToProcess:        readInt64Any(scan, "bytes_to_process", "bytesToProcess", "total_bytes", "totalBytes"),
		TotalSecondsRemaining: readInt64Any(scan, "total_secs_left", "totalSecondsRemaining"),
		StartedAt:             readTimeAny(scan, "start_time", "startTime", "started_at", "startedAt"),
		EndedAt:               readTimeAny(scan, "end_time", "endTime", "ended_at", "endedAt"),
	}
	return result
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func wholeDiskFromDevice(device string) string {
	device = strings.TrimPrefix(strings.TrimSpace(device), "/dev/")
	if device == "" || strings.Contains(device, "/") {
		return ""
	}

	if partition := strings.LastIndex(device, "p"); partition > 0 && partition < len(device)-1 {
		suffix := device[partition+1:]
		if allASCIIBytes(suffix, func(value byte) bool { return value >= '0' && value <= '9' }) {
			return device[:partition]
		}
	}

	// Linux sd/vd/hd/xvd partitions conventionally append digits without a
	// separator. Do not apply this to CORE names such as ada4/da9 or whole
	// NVMe names such as nvme0n1, where the trailing digit is the disk.
	end := len(device)
	for end > 0 && device[end-1] >= '0' && device[end-1] <= '9' {
		end--
	}
	prefix := device[:end]
	if end > 0 && end < len(device) &&
		(strings.HasPrefix(prefix, "sd") ||
			strings.HasPrefix(prefix, "vd") ||
			strings.HasPrefix(prefix, "hd") ||
			strings.HasPrefix(prefix, "xvd")) {
		return device[:end]
	}
	return device
}

func allASCIIBytes(value string, accept func(byte) bool) bool {
	if value == "" {
		return false
	}
	for i := 0; i < len(value); i++ {
		if !accept(value[i]) {
			return false
		}
	}
	return true
}

func (c *Client) getPoolsREST(ctx context.Context) ([]Pool, error) {
	var response []poolResponse
	if err := c.getJSON(ctx, http.MethodGet, "/pool", &response); err != nil {
		return nil, err
	}

	pools := make([]Pool, 0, len(response))
	for _, item := range response {
		id := strconv.FormatInt(item.ID, 10)
		if id == "0" && strings.TrimSpace(item.Name) != "" {
			id = strings.TrimSpace(item.Name)
		}
		var topology any
		if len(item.Topology) > 0 {
			if err := json.Unmarshal(item.Topology, &topology); err != nil {
				topology = nil
			}
		}
		var scan any
		if len(item.Scan) > 0 {
			if err := json.Unmarshal(item.Scan, &scan); err != nil {
				scan = nil
			}
		}
		vdevs, diskMembers := poolTopologyFromTopology(topology)
		readErrors, writeErrors, checksumErrors := poolErrorTotals(topology)

		pools = append(pools, Pool{
			ID:             id,
			GUID:           strings.TrimSpace(item.GUID),
			Name:           strings.TrimSpace(item.Name),
			Status:         strings.TrimSpace(item.Status),
			StatusCode:     strings.TrimSpace(item.StatusCode),
			StatusDetail:   strings.TrimSpace(item.StatusDetail),
			TotalBytes:     item.Size,
			UsedBytes:      item.Allocated,
			FreeBytes:      item.Free,
			ReadErrors:     readErrors,
			WriteErrors:    writeErrors,
			ChecksumErrors: checksumErrors,
			Scan:           poolScanFromAny(scan),
			VDevs:          vdevs,
			DiskMembers:    diskMembers,
		})
	}

	return pools, nil
}

// GetDatasets returns datasets and normalized capacity/read-only fields.
func (c *Client) GetDatasets(ctx context.Context) ([]Dataset, error) {
	legacy, err := c.useLegacyREST(ctx)
	if err != nil {
		return nil, err
	}
	if legacy {
		return c.getDatasetsREST(ctx)
	}
	return c.getDatasetsRPC(ctx)
}

func (c *Client) getDatasetsRPC(ctx context.Context) ([]Dataset, error) {
	var response []map[string]any
	if err := c.queryRPC(ctx, "pool.dataset.query", &response); err != nil {
		return nil, err
	}

	datasets := make([]Dataset, 0, len(response))
	for _, item := range response {
		name := strings.TrimSpace(readStringAny(item, "name", "id"))
		id := strings.TrimSpace(readStringAny(item, "id"))
		if id == "" {
			id = name
		}
		poolName := strings.TrimSpace(readStringAny(item, "pool"))
		if poolName == "" {
			poolName = parentPoolFromDataset(name)
		}
		used := readInt64Any(item, "used", "used_bytes", "usedBytes")
		available := readInt64Any(item, "available", "avail", "avail_bytes", "available_bytes", "availableBytes")

		locked := readBoolAny(item, "locked")
		datasets = append(datasets, Dataset{
			ID:         id,
			Name:       name,
			Pool:       poolName,
			UsedBytes:  used,
			AvailBytes: available,
			// pool.dataset.query has never returned a "mounted" field on any
			// TrueNAS version (CORE 13 and SCALE both strip it from the
			// property allowlist), so absence must not read as unmounted —
			// that marked every dataset Offline (#1573). A dataset the API
			// lists is treated as mounted unless it is locked (encrypted
			// with the key unloaded), the one unmounted state the API does
			// report. An explicit "mounted" value still wins when present.
			Mounted:  readBoolAnyDefault(item, true, "mounted") && !locked,
			Locked:   locked,
			ReadOnly: readBoolAny(item, "readonly", "read_only", "readOnly"),
		})
	}
	return datasets, nil
}

func (c *Client) getDatasetsREST(ctx context.Context) ([]Dataset, error) {
	var response []datasetResponse
	if err := c.getJSON(ctx, http.MethodGet, "/pool/dataset", &response); err != nil {
		return nil, err
	}

	datasets := make([]Dataset, 0, len(response))
	for _, item := range response {
		used, err := item.Used.int64Value()
		if err != nil {
			return nil, fmt.Errorf("parse dataset %q used bytes: %w", strings.TrimSpace(item.ID), err)
		}
		available, err := item.Available.int64Value()
		if err != nil {
			return nil, fmt.Errorf("parse dataset %q available bytes: %w", strings.TrimSpace(item.ID), err)
		}
		readOnly, err := item.ReadOnly.boolValue()
		if err != nil {
			return nil, fmt.Errorf("parse dataset %q readonly flag: %w", strings.TrimSpace(item.ID), err)
		}

		name := strings.TrimSpace(item.Name)
		id := strings.TrimSpace(item.ID)
		if name == "" {
			name = id
		}

		poolName := strings.TrimSpace(item.Pool)
		if poolName == "" {
			poolName = parentPoolFromDataset(name)
		}

		// See getDatasetsRPC: "mounted" does not exist in the API response,
		// so a listed dataset counts as mounted unless locked or explicitly
		// reported otherwise.
		mounted := !item.Locked
		if item.Mounted != nil {
			mounted = *item.Mounted && !item.Locked
		}

		datasets = append(datasets, Dataset{
			ID:         id,
			Name:       name,
			Pool:       poolName,
			UsedBytes:  used,
			AvailBytes: available,
			Mounted:    mounted,
			Locked:     item.Locked,
			ReadOnly:   readOnly,
		})
	}

	return datasets, nil
}

// GetDisks returns the system disk inventory.
func (c *Client) GetDisks(ctx context.Context) ([]Disk, error) {
	legacy, err := c.useLegacyREST(ctx)
	if err != nil {
		return nil, err
	}
	if legacy {
		return c.getDisksREST(ctx)
	}
	return c.getDisksRPC(ctx)
}

func (c *Client) getDisksRPC(ctx context.Context) ([]Disk, error) {
	var response []map[string]any
	if err := c.callRPC(ctx, "disk.query", []any{[]any{}, map[string]any{
		"extra": map[string]any{"pools": true},
	}}, &response); err != nil {
		return nil, err
	}
	return c.disksFromMaps(ctx, response)
}

func (c *Client) getDisksREST(ctx context.Context) ([]Disk, error) {
	var response []diskResponse
	if err := c.getJSON(ctx, http.MethodGet, "/disk", &response); err != nil {
		return nil, err
	}
	if len(response) == 0 {
		return nil, nil
	}
	identifiers := diskReportingIdentifiers(response)
	temperatures, err := c.getDiskTemperaturesWithFallback(ctx, identifiers)
	if err != nil {
		temperatures = nil
	}
	aggregates, err := c.getDiskTemperatureAggregates(ctx, identifiers, defaultDiskTemperatureAggregateWindowDays)
	if err != nil {
		aggregates = nil
	}

	disks := make([]Disk, 0, len(response))
	for _, item := range response {
		rotationRate := item.RotationRate
		rotational := rotationRate > 0
		if rotationRate == 0 {
			switch strings.ToLower(strings.TrimSpace(item.Type)) {
			case "hdd":
				rotational = true
			case "ssd", "nvme":
				rotational = false
			}
		}

		diskID := strings.TrimSpace(item.Identifier)
		if diskID == "" {
			diskID = strings.TrimSpace(item.Name)
		}
		health, healthPresent := diskSMARTHealthFromRaw(item.SmartStatus)

		disks = append(disks, Disk{
			ID:                   diskID,
			Name:                 strings.TrimSpace(item.Name),
			Pool:                 strings.TrimSpace(item.Pool),
			Status:               strings.TrimSpace(item.Status),
			Health:               health,
			HealthStatusPresent:  healthPresent,
			Model:                strings.TrimSpace(item.Model),
			Serial:               strings.TrimSpace(item.Serial),
			SizeBytes:            item.Size,
			Temperature:          temperatureForTrueNASDisk(temperatures, item),
			TemperatureAggregate: temperatureAggregateForTrueNASDisk(aggregates, item),
			Transport:            strings.ToLower(strings.TrimSpace(item.Bus)),
			Rotational:           rotational,
		})
	}

	return disks, nil
}

func (c *Client) disksFromMaps(ctx context.Context, response []map[string]any) ([]Disk, error) {
	if len(response) == 0 {
		return nil, nil
	}
	identifiers := diskReportingIdentifiersFromMaps(response)
	temperatures, err := c.getDiskTemperaturesWithFallback(ctx, identifiers)
	if err != nil {
		temperatures = nil
	}
	aggregates, err := c.getDiskTemperatureAggregates(ctx, identifiers, defaultDiskTemperatureAggregateWindowDays)
	if err != nil {
		aggregates = nil
	}

	disks := make([]Disk, 0, len(response))
	for _, item := range response {
		rotationRate := readIntAny(item, "rotationrate", "rotation_rate", "rotationRate")
		rotational := rotationRate > 0
		diskType := strings.ToLower(strings.TrimSpace(readStringAny(item, "type")))
		if rotationRate == 0 {
			switch diskType {
			case "hdd":
				rotational = true
			case "ssd", "nvme":
				rotational = false
			}
		}

		diskID := strings.TrimSpace(readStringAny(item, "identifier", "id"))
		if diskID == "" {
			diskID = strings.TrimSpace(readStringAny(item, "name", "devname"))
		}
		name := strings.TrimSpace(readStringAny(item, "name", "devname"))
		health, healthPresent := diskSMARTHealthFromMap(item)
		disk := Disk{
			ID:                   diskID,
			Name:                 name,
			Pool:                 strings.TrimSpace(readStringAny(item, "pool", "pool_name", "poolName")),
			Status:               strings.TrimSpace(readStringAny(item, "status")),
			Health:               health,
			HealthStatusPresent:  healthPresent,
			Model:                strings.TrimSpace(readStringAny(item, "model")),
			Serial:               strings.TrimSpace(readStringAny(item, "serial")),
			SizeBytes:            readInt64Any(item, "size", "size_bytes", "sizeBytes"),
			Temperature:          temperatureForTrueNASDiskMap(temperatures, item),
			TemperatureAggregate: temperatureAggregateForTrueNASDiskMap(aggregates, item),
			Transport:            strings.ToLower(strings.TrimSpace(readStringAny(item, "bus", "transport"))),
			Rotational:           rotational,
		}
		if disk.Transport == "" {
			disk.Transport = diskType
		}
		disks = append(disks, disk)
	}

	return disks, nil
}

// enrichDisksFromPoolTopology fills each disk's pool membership and ZFS
// member state from the pools' vdev topology. disk.query itself reports no
// health/status field on any TrueNAS version and only names the pool behind
// an extra option the REST bridge cannot pass, so without this every disk
// rendered as an unparented "Unknown" (#1573).
func enrichDisksFromPoolTopology(pools []Pool, disks []Disk) []Disk {
	type membership struct {
		pool   string
		status string
	}
	byDevice := make(map[string]membership)
	knownDevices := make(map[string]struct{}, len(disks)*2)
	for _, disk := range disks {
		for _, identity := range []string{disk.Name, disk.ID} {
			if identity = strings.TrimSpace(identity); identity != "" {
				knownDevices[identity] = struct{}{}
			}
		}
	}
	for _, pool := range pools {
		for _, member := range pool.DiskMembers {
			for _, device := range []string{member.Disk, member.Device} {
				device = strings.TrimSpace(device)
				if device == "" {
					continue
				}
				if _, exists := byDevice[device]; !exists {
					byDevice[device] = membership{pool: pool.Name, status: member.Status}
				}
			}
			if !member.Missing {
				continue
			}
			name := firstNonEmptyString(member.Disk, wholeDiskFromDevice(member.Device), member.Device)
			if name == "" {
				continue
			}
			if _, exists := knownDevices[name]; exists {
				continue
			}
			// A synthetic disk resource is emitted only from explicit
			// unavail_disk topology evidence. Missing disk.query rows alone are
			// not enough to assert that a disk is absent.
			disks = append(disks, Disk{
				ID:     firstNonEmptyString(member.GUID, name),
				Name:   name,
				Pool:   pool.Name,
				Status: member.Status,
				Health: "UNKNOWN",
			})
			knownDevices[name] = struct{}{}
		}
	}
	if len(byDevice) == 0 {
		return disks
	}

	for i := range disks {
		disk := &disks[i]
		member, ok := byDevice[strings.TrimSpace(disk.Name)]
		if !ok {
			member, ok = byDevice[strings.TrimSpace(disk.ID)]
		}
		if !ok {
			continue
		}
		if strings.TrimSpace(disk.Pool) == "" {
			disk.Pool = member.pool
		}
		if strings.TrimSpace(disk.Status) == "" {
			disk.Status = member.status
		}
	}
	return disks
}

// GetDiskTemperatures returns the current temperature by TrueNAS disk name.
func (c *Client) GetDiskTemperatures(ctx context.Context) (map[string]int, error) {
	return c.getDiskTemperaturesWithFallback(ctx, nil)
}

// GetDiskTemperatureHistory returns recent disk temperature series by TrueNAS
// disk identifier using the native reporting API.
func (c *Client) GetDiskTemperatureHistory(ctx context.Context, identifiers []string, duration time.Duration) (map[string][]TimeSeriesPoint, error) {
	identifiers = dedupeStrings(identifiers)
	if len(identifiers) == 0 {
		return nil, fmt.Errorf("truenas disk temperature history requires disk identifiers")
	}

	var history map[string][]TimeSeriesPoint
	err := c.withRPC(ctx, func(rpc *trueNASRPCClient) error {
		var err error
		history, err = rpc.getDiskTemperatureHistory(ctx, identifiers, duration)
		return err
	})
	return history, err
}

func (c *Client) getDiskTemperaturesWithFallback(ctx context.Context, identifiers []string) (map[string]int, error) {
	legacy, err := c.useLegacyREST(ctx)
	if err != nil {
		return nil, err
	}
	if len(identifiers) == 0 {
		reportingIdentifiers, err := c.listDiskReportingIdentifiers(ctx)
		if err == nil {
			identifiers = reportingIdentifiers
		}
	}

	if !legacy {
		// Native JSON-RPC reporting is the only supported path on SCALE
		// 25.04+. An unavailable reporting method is best-effort telemetry;
		// it must never wake the deprecated REST bridge.
		return c.getDiskTemperaturesFromReporting(ctx, identifiers)
	}

	// disk.temperatures takes parameters, so REST v2.0 serves it as POST
	// with a body keyed by parameter name — on CORE 13.x (which has no
	// JSON-RPC endpoint at all) this is the only path that can answer.
	// An empty names list means "all disks eligible for temp monitoring";
	// the remaining parameter (powermode on CORE, options on SCALE) is
	// omitted so each version applies its own default.
	var response any
	restErr := c.postJSON(ctx, "/disk/temperatures", map[string]any{"names": []string{}}, &response)
	if restErr == nil {
		if temperatures := parseDiskTemperatures(response); len(temperatures) > 0 {
			return temperatures, nil
		}
	}

	if restErr != nil {
		return nil, restErr
	}

	return parseDiskTemperatures(response), nil
}

func (c *Client) listDiskReportingIdentifiers(ctx context.Context) ([]string, error) {
	legacy, err := c.useLegacyREST(ctx)
	if err != nil {
		return nil, err
	}
	if !legacy {
		var response []map[string]any
		if err := c.callRPC(ctx, "disk.query", []any{[]any{}, map[string]any{}}, &response); err != nil {
			return nil, err
		}
		return diskReportingIdentifiersFromMaps(response), nil
	}
	var response []diskResponse
	if err := c.getJSON(ctx, http.MethodGet, "/disk", &response); err != nil {
		return nil, err
	}
	return diskReportingIdentifiers(response), nil
}

func diskReportingIdentifiers(disks []diskResponse) []string {
	identifiers := make([]string, 0, len(disks))
	for _, disk := range disks {
		if name := strings.TrimSpace(disk.Name); name != "" {
			identifiers = append(identifiers, name)
			continue
		}
		if identifier := strings.TrimSpace(disk.Identifier); identifier != "" {
			identifiers = append(identifiers, identifier)
		}
	}
	return dedupeStrings(identifiers)
}

func diskReportingIdentifiersFromMaps(disks []map[string]any) []string {
	identifiers := make([]string, 0, len(disks))
	for _, disk := range disks {
		if name := strings.TrimSpace(readStringAny(disk, "name", "devname")); name != "" {
			identifiers = append(identifiers, name)
			continue
		}
		if identifier := strings.TrimSpace(readStringAny(disk, "identifier", "id")); identifier != "" {
			identifiers = append(identifiers, identifier)
		}
	}
	return dedupeStrings(identifiers)
}

func (c *Client) getDiskTemperaturesFromReporting(ctx context.Context, identifiers []string) (map[string]int, error) {
	identifiers = dedupeStrings(identifiers)
	if len(identifiers) == 0 {
		return nil, fmt.Errorf("truenas reporting disk temperature fallback requires disk identifiers")
	}

	var temperatures map[string]int
	err := c.withRPC(ctx, func(rpc *trueNASRPCClient) error {
		var err error
		temperatures, err = rpc.getDiskTemperatures(ctx, identifiers)
		return err
	})
	return temperatures, err
}

func (c *Client) getDiskTemperatureAggregates(ctx context.Context, identifiers []string, windowDays int) (map[string]DiskTemperatureAggregate, error) {
	identifiers = dedupeStrings(identifiers)
	if len(identifiers) == 0 {
		return nil, fmt.Errorf("truenas disk temperature aggregates require disk identifiers")
	}

	var aggregates map[string]DiskTemperatureAggregate
	err := c.withRPC(ctx, func(rpc *trueNASRPCClient) error {
		var err error
		aggregates, err = rpc.getDiskTemperatureAggregates(ctx, identifiers, windowDays)
		return err
	})
	return aggregates, err
}

// GetAlerts returns active and dismissed TrueNAS alerts.
func (c *Client) GetAlerts(ctx context.Context) ([]Alert, error) {
	legacy, err := c.useLegacyREST(ctx)
	if err != nil {
		return nil, err
	}
	if legacy {
		return c.getAlertsREST(ctx)
	}
	return c.getAlertsRPC(ctx)
}

func (c *Client) getAlertsRPC(ctx context.Context) ([]Alert, error) {
	var response []map[string]any
	if err := c.callRPC(ctx, "alert.list", []any{}, &response); err != nil {
		return nil, err
	}

	alerts := make([]Alert, 0, len(response))
	for _, item := range response {
		id := strings.TrimSpace(readStringAny(item, "uuid", "id", "key"))
		if id == "" {
			id = strings.TrimSpace(readStringAny(item, "klass"))
		}
		var datetime time.Time
		if t := readTimeAny(item, "datetime", "last_occurrence", "lastOccurrence"); t != nil {
			datetime = *t
		}
		alerts = append(alerts, Alert{
			ID:        id,
			Level:     strings.TrimSpace(readStringAny(item, "level")),
			Message:   strings.TrimSpace(readStringAny(item, "formatted", "text", "message", "klass")),
			Source:    strings.TrimSpace(readStringAny(item, "source", "node")),
			Dismissed: readBoolAny(item, "dismissed"),
			Datetime:  datetime,
		})
	}
	return alerts, nil
}

func (c *Client) getAlertsREST(ctx context.Context) ([]Alert, error) {
	var response []alertResponse
	if err := c.getJSON(ctx, http.MethodGet, "/alert/list", &response); err != nil {
		return nil, err
	}

	alerts := make([]Alert, 0, len(response))
	for _, item := range response {
		id, err := rawIDToString(item.ID)
		if err != nil {
			return nil, fmt.Errorf("parse alert id: %w", err)
		}

		ms, err := parseInt64FromAny(item.Datetime.Date)
		if err != nil {
			return nil, fmt.Errorf("parse alert %q datetime: %w", id, err)
		}

		alerts = append(alerts, Alert{
			ID:        id,
			Level:     strings.TrimSpace(item.Level),
			Message:   strings.TrimSpace(item.Formatted),
			Source:    strings.TrimSpace(item.Source),
			Dismissed: item.Dismissed,
			Datetime:  time.UnixMilli(ms).UTC(),
		})
	}

	return alerts, nil
}

// GetServices returns the native TrueNAS system service inventory.
func (c *Client) GetServices(ctx context.Context) ([]Service, error) {
	legacy, err := c.useLegacyREST(ctx)
	if err != nil {
		return nil, err
	}
	if legacy {
		return c.getServicesREST(ctx)
	}
	return c.getServicesRPC(ctx)
}

func (c *Client) getServicesRPC(ctx context.Context) ([]Service, error) {
	var response []map[string]any
	if err := c.callRPC(ctx, "service.query", []any{[]any{}, map[string]any{
		"extra": map[string]any{"include_state": true},
	}}, &response); err != nil {
		return nil, err
	}
	return parseServices(response), nil
}

func (c *Client) getServicesREST(ctx context.Context) ([]Service, error) {
	var response []map[string]any
	if err := c.getJSON(ctx, http.MethodGet, "/service", &response); err != nil {
		return nil, err
	}
	return parseServices(response), nil
}

func parseServices(response []map[string]any) []Service {
	services := make([]Service, 0, len(response))
	for _, item := range response {
		name := strings.TrimSpace(readStringAny(item, "service", "name"))
		id := strings.TrimSpace(readStringAny(item, "id"))
		if id == "" {
			id = name
		}
		services = append(services, Service{
			ID:      id,
			Service: name,
			Enabled: readBoolAny(item, "enable", "enabled"),
			State:   strings.TrimSpace(readStringAny(item, "state", "status")),
			PIDs:    readIntSliceAny(item, "pids", "pid"),
		})
	}
	return services
}

// GetVMs returns the best-effort native TrueNAS VM inventory. TrueNAS 25.04+
// uses vm.query on JSON-RPC; recognized pre-25.04 SCALE and CORE releases use
// the connection's negotiated legacy REST transport.
func (c *Client) GetVMs(ctx context.Context) ([]VirtualMachine, error) {
	legacy, err := c.useLegacyREST(ctx)
	if err != nil {
		return nil, err
	}
	if legacy {
		return c.getVMsREST(ctx)
	}
	return c.getVMsRPC(ctx)
}

func (c *Client) getVMsRPC(ctx context.Context) ([]VirtualMachine, error) {
	var response []map[string]any
	if err := c.queryRPC(ctx, "vm.query", &response); err != nil {
		return nil, err
	}
	return parseVirtualMachines(response), nil
}

func (c *Client) getVMsREST(ctx context.Context) ([]VirtualMachine, error) {
	var response []map[string]any
	if err := c.getJSON(ctx, http.MethodGet, "/vm", &response); err != nil {
		return nil, err
	}
	return parseVirtualMachines(response), nil
}

// GetNetworkShares returns the best-effort native TrueNAS SMB/NFS sharing
// inventory. Modern connections use JSON-RPC query methods; recognized legacy
// connections remain on REST for the lifetime of that client.
func (c *Client) GetNetworkShares(ctx context.Context) ([]NetworkShare, error) {
	legacy, err := c.useLegacyREST(ctx)
	if err != nil {
		return nil, err
	}
	var shares []NetworkShare
	var errors []error

	var smb []NetworkShare
	if legacy {
		smb, err = c.getNetworkSharesREST(ctx, "/sharing/smb", "SMB")
	} else {
		smb, err = c.getNetworkSharesRPC(ctx, "sharing.smb.query", "SMB")
	}
	if err != nil {
		errors = append(errors, fmt.Errorf("smb: %w", err))
	} else {
		shares = append(shares, smb...)
	}

	var nfs []NetworkShare
	if legacy {
		nfs, err = c.getNetworkSharesREST(ctx, "/sharing/nfs", "NFS")
	} else {
		nfs, err = c.getNetworkSharesRPC(ctx, "sharing.nfs.query", "NFS")
	}
	if err != nil {
		errors = append(errors, fmt.Errorf("nfs: %w", err))
	} else {
		shares = append(shares, nfs...)
	}

	if len(shares) == 0 && len(errors) > 0 {
		return nil, fmt.Errorf("fetch truenas network shares: %v", errors)
	}
	return shares, nil
}

func (c *Client) getNetworkSharesRPC(ctx context.Context, method, protocol string) ([]NetworkShare, error) {
	var response []map[string]any
	if err := c.queryRPC(ctx, method, &response); err != nil {
		return nil, err
	}
	return parseNetworkShares(response, protocol), nil
}

func (c *Client) getNetworkSharesREST(ctx context.Context, path, protocol string) ([]NetworkShare, error) {
	var response []map[string]any
	if err := c.getJSON(ctx, http.MethodGet, path, &response); err != nil {
		return nil, err
	}
	return parseNetworkShares(response, protocol), nil
}

func (c *Client) queryRPC(ctx context.Context, method string, result any) error {
	return c.callRPC(ctx, method, []any{[]any{}, map[string]any{}}, result)
}

// GetApps returns the best-effort TrueNAS app inventory as canonical workload
// candidates. TrueNAS 25.04+ uses app.query on JSON-RPC; recognized legacy
// connections remain on their negotiated REST transport.
func (c *Client) GetApps(ctx context.Context) ([]App, error) {
	legacy, err := c.useLegacyREST(ctx)
	if err != nil {
		return nil, err
	}
	if legacy {
		return c.getAppsREST(ctx)
	}
	return c.getAppsRPC(ctx)
}

func (c *Client) getAppsRPC(ctx context.Context) ([]App, error) {
	var response []map[string]any
	if err := c.queryRPC(ctx, "app.query", &response); err != nil {
		return nil, err
	}
	return c.parseAppsWithStats(ctx, response), nil
}

func (c *Client) getAppsREST(ctx context.Context) ([]App, error) {
	var response []map[string]any
	if err := c.getJSON(ctx, http.MethodGet, "/app", &response); err != nil {
		return nil, err
	}
	return c.parseAppsWithStats(ctx, response), nil
}

func (c *Client) parseAppsWithStats(ctx context.Context, response []map[string]any) []App {
	statsByApp, err := c.GetAppStats(ctx)
	if err != nil {
		statsByApp = nil
	}

	apps := make([]App, 0, len(response))
	for _, item := range response {
		activeWorkloads := readMapAny(item, "active_workloads", "activeWorkloads")

		app := App{
			ID:                    strings.TrimSpace(readStringAny(item, "id")),
			Name:                  strings.TrimSpace(readStringAny(item, "name")),
			State:                 strings.TrimSpace(readStringAny(item, "state")),
			Version:               strings.TrimSpace(readStringAny(item, "version")),
			HumanVersion:          strings.TrimSpace(readStringAny(item, "human_version", "humanVersion")),
			CustomApp:             readBoolAny(item, "custom_app", "customApp"),
			UpgradeAvailable:      readBoolAny(item, "upgrade_available", "upgradeAvailable"),
			ImageUpdatesAvailable: readBoolAny(item, "image_updates_available", "imageUpdatesAvailable"),
			Notes:                 strings.TrimSpace(readStringAny(item, "notes")),
			ContainerCount:        readIntAny(activeWorkloads, "containers"),
			UsedHostIPs:           dedupeStrings(readStringSliceAny(activeWorkloads, "used_host_ips", "usedHostIps")),
			UsedPorts:             parseAppPorts(readSliceAny(activeWorkloads, "used_ports", "usedPorts")),
			Containers:            parseAppContainers(readSliceAny(activeWorkloads, "container_details", "containerDetails")),
			Volumes:               parseAppVolumes(readSliceAny(activeWorkloads, "volumes")),
			Images:                dedupeStrings(readStringSliceAny(activeWorkloads, "images")),
			Networks:              parseAppNetworks(readSliceAny(activeWorkloads, "networks")),
		}

		if app.Name == "" {
			app.Name = app.ID
		}
		if app.ContainerCount <= 0 {
			app.ContainerCount = len(app.Containers)
		}
		if len(app.Images) == 0 {
			app.Images = dedupeStrings(appImagesFromContainers(app.Containers))
		}
		if len(app.Volumes) == 0 {
			app.Volumes = dedupeAppVolumes(appVolumesFromContainers(app.Containers))
		}
		if len(statsByApp) > 0 {
			if stats, ok := statsByApp[normalizeAppStatsKey(app.ID)]; ok {
				statsCopy := stats
				app.Stats = &statsCopy
			} else if stats, ok := statsByApp[normalizeAppStatsKey(app.Name)]; ok {
				statsCopy := stats
				app.Stats = &statsCopy
			}
		}

		apps = append(apps, app)
	}

	return apps
}

// GetAppStats retrieves live TrueNAS app workload telemetry from the modern
// JSON-RPC websocket API. This path is best-effort; callers should treat any
// transport or endpoint failure as "stats unavailable" rather than inventory
// failure.
func (c *Client) GetAppStats(ctx context.Context) (map[string]AppStats, error) {
	var stats map[string]AppStats
	err := c.withRPC(ctx, func(rpc *trueNASRPCClient) error {
		subscriptionName := fmt.Sprintf("app.stats:{\"interval\":%d}", defaultAppStatsIntervalSeconds)
		subscriptionID, err := rpc.subscribe(ctx, subscriptionName)
		if err != nil {
			return err
		}
		stats, err = rpc.readAppStatsEvent(ctx, defaultAppStatsIntervalSeconds)
		if err != nil {
			return discardRPCSessionForStreamError(err)
		}
		return rpc.unsubscribe(ctx, subscriptionID)
	})
	return stats, err
}

// StartApp requests that TrueNAS start the named app through the canonical
// JSON-RPC control path.
func (c *Client) StartApp(ctx context.Context, appID string) error {
	return c.executeAppAction(ctx, "app.start", appID)
}

// StopApp requests that TrueNAS stop the named app through the canonical
// JSON-RPC control path.
func (c *Client) StopApp(ctx context.Context, appID string) error {
	return c.executeAppAction(ctx, "app.stop", appID)
}

// GetAppLogs retrieves a bounded tail of one TrueNAS app container log stream
// through the canonical JSON-RPC event path.
func (c *Client) GetAppLogs(ctx context.Context, appName, containerID string, tailLines int) ([]AppLogLine, error) {
	appName = strings.TrimSpace(appName)
	containerID = strings.TrimSpace(containerID)
	if appName == "" {
		return nil, fmt.Errorf("truenas app name is required")
	}
	if containerID == "" {
		return nil, fmt.Errorf("truenas container id is required")
	}
	if tailLines <= 0 {
		tailLines = 100
	}
	if tailLines > maxAppLogTailLines {
		tailLines = maxAppLogTailLines
	}

	subscriptionArgs := map[string]any{
		"app_name":     appName,
		"container_id": containerID,
		"tail_lines":   tailLines,
	}
	subscriptionJSON, err := json.Marshal(subscriptionArgs)
	if err != nil {
		return nil, fmt.Errorf("marshal truenas app log subscription: %w", err)
	}
	subscriptionName := fmt.Sprintf("app.container_log_follow:%s", string(subscriptionJSON))
	var lines []AppLogLine
	err = c.withRPC(ctx, func(rpc *trueNASRPCClient) error {
		subscriptionID, err := rpc.subscribe(ctx, subscriptionName)
		if err != nil {
			return err
		}
		var reusable bool
		lines, reusable, err = rpc.readAppLogEvents(ctx, tailLines)
		if err != nil {
			return discardRPCSessionForStreamError(err)
		}
		if !reusable {
			return errRPCStreamSessionConsumed
		}
		return rpc.unsubscribe(ctx, subscriptionID)
	})
	return lines, err
}

func (c *Client) executeAppAction(ctx context.Context, method, appID string) error {
	appID = strings.TrimSpace(appID)
	if appID == "" {
		return fmt.Errorf("truenas app id is required")
	}

	if err := c.callRPCAction(ctx, method, []any{appID}, nil); err != nil {
		return err
	}
	return nil
}

// GetZFSSnapshots returns a best-effort list of ZFS snapshots. Modern TrueNAS
// uses JSON-RPC; recognized legacy connections remain on REST.
func (c *Client) GetZFSSnapshots(ctx context.Context) ([]ZFSSnapshot, error) {
	legacy, err := c.useLegacyREST(ctx)
	if err != nil {
		return nil, err
	}
	if legacy {
		return c.getZFSSnapshotsREST(ctx)
	}
	return c.getZFSSnapshotsRPC(ctx)
}

func (c *Client) getZFSSnapshotsRPC(ctx context.Context) ([]ZFSSnapshot, error) {
	var response []map[string]any
	err := c.callRPC(ctx, "zfs.resource.snapshot.query", []any{map[string]any{
		"paths":      []string{},
		"recursive":  true,
		"properties": []string{"creation", "used", "referenced"},
	}}, &response)
	if err == nil {
		return parseZFSSnapshots(response), nil
	}
	if !isMethodUnavailable(err) {
		return nil, err
	}

	if err := c.queryRPC(ctx, "pool.snapshot.query", &response); err != nil {
		return nil, err
	}
	return parseZFSSnapshots(response), nil
}

func (c *Client) getZFSSnapshotsREST(ctx context.Context) ([]ZFSSnapshot, error) {
	var response []map[string]any
	if err := c.getJSON(ctx, http.MethodGet, "/zfs/snapshot", &response); err != nil {
		return nil, err
	}
	return parseZFSSnapshots(response), nil
}

func parseZFSSnapshots(response []map[string]any) []ZFSSnapshot {
	snapshots := make([]ZFSSnapshot, 0, len(response))
	for _, item := range response {
		full := readStringAny(item, "id", "name", "snapshot", "snapshot_name", "snapshotName")
		dataset := readStringAny(item, "dataset", "dataset_name", "datasetName")
		snapName := readStringAny(item, "snapshot_name", "snapshotName", "name")

		if dataset == "" || snapName == "" {
			if parsedDataset, parsedSnap := splitSnapshotName(full); dataset == "" || snapName == "" {
				if dataset == "" {
					dataset = parsedDataset
				}
				if snapName == "" {
					snapName = parsedSnap
				}
			}
		}

		var createdAt *time.Time
		properties, _ := item["properties"].(map[string]any)
		if t := readTimeAny(item,
			"created_at",
			"createdAt",
			"creation_time",
			"creationTime",
			"created",
			"creation",
			"datetime",
		); t != nil {
			createdAt = t
		} else if properties != nil {
			createdAt = readTimeAny(properties, "creation", "created", "datetime")
		}

		usedBytes := readInt64PtrAny(item, "used", "used_bytes", "usedBytes")
		referenced := readInt64PtrAny(item, "referenced", "referenced_bytes", "referencedBytes")
		if properties != nil {
			if usedBytes == nil {
				usedBytes = readInt64PtrAny(properties, "used", "used_bytes", "usedBytes")
			}
			if referenced == nil {
				referenced = readInt64PtrAny(properties, "referenced", "referenced_bytes", "referencedBytes")
			}
		}

		if full == "" && dataset != "" && snapName != "" {
			full = dataset + "@" + snapName
		}

		snapshots = append(snapshots, ZFSSnapshot{
			ID:         full,
			Dataset:    dataset,
			Name:       snapName,
			FullName:   full,
			CreatedAt:  createdAt,
			UsedBytes:  usedBytes,
			Referenced: referenced,
		})
	}

	return snapshots
}

// GetReplicationTasks returns a best-effort list of replication tasks including last-run state.
func (c *Client) GetReplicationTasks(ctx context.Context) ([]ReplicationTask, error) {
	legacy, err := c.useLegacyREST(ctx)
	if err != nil {
		return nil, err
	}
	if legacy {
		return c.getReplicationTasksREST(ctx)
	}
	return c.getReplicationTasksRPC(ctx)
}

func (c *Client) getReplicationTasksRPC(ctx context.Context) ([]ReplicationTask, error) {
	var response []map[string]any
	if err := c.queryRPC(ctx, "replication.query", &response); err != nil {
		return nil, err
	}
	return parseReplicationTasks(response), nil
}

func (c *Client) getReplicationTasksREST(ctx context.Context) ([]ReplicationTask, error) {
	var response []map[string]any
	if err := c.getJSON(ctx, http.MethodGet, "/replication", &response); err != nil {
		return nil, err
	}
	return parseReplicationTasks(response), nil
}

func parseReplicationTasks(response []map[string]any) []ReplicationTask {
	tasks := make([]ReplicationTask, 0, len(response))
	for _, item := range response {
		id := readStringAny(item, "id")
		name := readStringAny(item, "name")
		direction := readStringAny(item, "direction")
		targetDataset := readStringAny(item, "target_dataset", "targetDataset", "target")
		transport := readStringAny(item, "transport")
		readOnlyMode := readStringAny(item, "readonly", "read_only", "readOnly")
		targetHost := ""
		if credentials := readMapAny(item, "ssh_credentials", "sshCredentials"); credentials != nil {
			if attributes := readMapAny(credentials, "attributes"); attributes != nil {
				targetHost = readStringAny(attributes, "host", "hostname")
			}
		}

		sourceDatasets := readStringSliceAny(item, "source_datasets", "sourceDatasets", "sources", "source")

		var lastRun *time.Time
		if t := readTimeAny(item, "last_run", "lastRun", "last_run_time", "lastRunTime"); t != nil {
			lastRun = t
		}
		lastState := readStringAny(item, "state", "last_state", "lastState", "last_result", "lastResult")
		lastError := readStringAny(item, "error", "last_error", "lastError", "last_failure", "lastFailure")
		lastSnapshot := readStringAny(item, "last_snapshot", "lastSnapshot")

		// Some TrueNAS versions nest state under "state".
		if stateObj, ok := item["state"].(map[string]any); ok {
			if lastRun == nil {
				lastRun = readTimeAny(stateObj, "datetime", "time", "last_run", "lastRun")
			}
			if strings.TrimSpace(lastState) == "" {
				lastState = readStringAny(stateObj, "state", "status", "result")
			}
			if strings.TrimSpace(lastError) == "" {
				lastError = readStringAny(stateObj, "error", "message", "stderr")
			}
			if strings.TrimSpace(lastSnapshot) == "" {
				lastSnapshot = readStringAny(stateObj, "last_snapshot", "lastSnapshot", "snapshot")
			}
		}

		tasks = append(tasks, ReplicationTask{
			ID:             strings.TrimSpace(id),
			Name:           strings.TrimSpace(name),
			SourceDatasets: dedupeStrings(sourceDatasets),
			TargetDataset:  strings.TrimSpace(targetDataset),
			Direction:      strings.TrimSpace(direction),
			Transport:      strings.TrimSpace(transport),
			ReadOnlyMode:   strings.TrimSpace(readOnlyMode),
			TargetHost:     strings.TrimSpace(targetHost),
			LastRun:        lastRun,
			LastState:      strings.TrimSpace(lastState),
			LastError:      strings.TrimSpace(lastError),
			LastSnapshot:   strings.TrimSpace(lastSnapshot),
		})
	}

	return tasks
}

// FetchSnapshot collects a complete fixture-compatible snapshot.
func (c *Client) FetchSnapshot(ctx context.Context) (*FixtureSnapshot, error) {
	system, err := c.GetSystemInfo(ctx)
	if err != nil {
		return nil, fmt.Errorf("fetch truenas system info: %w", err)
	}
	if telemetry, err := c.GetSystemTelemetry(ctx); err == nil && telemetry != nil {
		mergeSystemTelemetry(system, telemetry)
	}

	pools, err := c.GetPools(ctx)
	if err != nil {
		return nil, fmt.Errorf("fetch truenas pools: %w", err)
	}
	// boot.get_state is best-effort because older/minimally privileged API
	// keys may not expose it. When available it supplies the only boot-pool
	// topology and per-member ZFS state.
	if bootPool, bootErr := c.GetBootPool(ctx); bootErr == nil && bootPool != nil {
		pools = mergeBootPool(pools, *bootPool)
	}

	datasets, err := c.GetDatasets(ctx)
	if err != nil {
		return nil, fmt.Errorf("fetch truenas datasets: %w", err)
	}

	disks, err := c.GetDisks(ctx)
	if err != nil {
		return nil, fmt.Errorf("fetch truenas disks: %w", err)
	}
	disks = enrichDisksFromPoolTopology(pools, disks)

	alerts, err := c.GetAlerts(ctx)
	if err != nil {
		return nil, fmt.Errorf("fetch truenas alerts: %w", err)
	}

	// Recovery artifacts are best-effort: do not fail monitoring if additional endpoints are unavailable.
	services, _ := c.GetServices(ctx)
	apps, _ := c.GetApps(ctx)
	vms, _ := c.GetVMs(ctx)
	shares, _ := c.GetNetworkShares(ctx)
	zfsSnapshots, _ := c.GetZFSSnapshots(ctx)
	replicationTasks, _ := c.GetReplicationTasks(ctx)

	return &FixtureSnapshot{
		CollectedAt:      time.Now().UTC(),
		System:           *system,
		Pools:            pools,
		Datasets:         datasets,
		Disks:            disks,
		Alerts:           alerts,
		Services:         services,
		Apps:             apps,
		VMs:              vms,
		Shares:           shares,
		ZFSSnapshots:     zfsSnapshots,
		ReplicationTasks: replicationTasks,
	}, nil
}

func mergeSystemTelemetry(system *SystemInfo, telemetry *SystemInfo) {
	if system == nil || telemetry == nil {
		return
	}
	if telemetry.CPUCount > 0 {
		system.CPUCount = telemetry.CPUCount
	}
	if telemetry.MemoryTotalBytes > 0 {
		system.MemoryTotalBytes = telemetry.MemoryTotalBytes
	}
	if telemetry.MemoryAvailableBytes > 0 {
		system.MemoryAvailableBytes = telemetry.MemoryAvailableBytes
	}
	system.CPUPercent = telemetry.CPUPercent
	system.NetInRate = telemetry.NetInRate
	system.NetOutRate = telemetry.NetOutRate
	system.DiskReadRate = telemetry.DiskReadRate
	system.DiskWriteRate = telemetry.DiskWriteRate
	if len(telemetry.TemperatureCelsius) > 0 {
		system.TemperatureCelsius = cloneTemperatureMap(telemetry.TemperatureCelsius)
	}
	if telemetry.IntervalSeconds > 0 {
		system.IntervalSeconds = telemetry.IntervalSeconds
	}
	if !telemetry.CollectedAt.IsZero() {
		system.CollectedAt = telemetry.CollectedAt
	}
}

func temperatureForTrueNASDisk(temperatures map[string]int, item diskResponse) int {
	if len(temperatures) == 0 {
		return 0
	}
	keys := []string{
		strings.TrimSpace(item.Name),
		strings.TrimSpace(item.Identifier),
		strings.TrimSpace(item.Serial),
	}
	for _, key := range keys {
		if key == "" {
			continue
		}
		if temperature, ok := temperatures[key]; ok {
			return temperature
		}
	}
	return 0
}

func temperatureForTrueNASDiskMap(temperatures map[string]int, item map[string]any) int {
	if len(temperatures) == 0 {
		return 0
	}
	keys := []string{
		strings.TrimSpace(readStringAny(item, "name", "devname")),
		strings.TrimSpace(readStringAny(item, "identifier", "id")),
		strings.TrimSpace(readStringAny(item, "serial")),
	}
	for _, key := range keys {
		if key == "" {
			continue
		}
		if temperature, ok := temperatures[key]; ok {
			return temperature
		}
	}
	return 0
}

func temperatureAggregateForTrueNASDisk(aggregates map[string]DiskTemperatureAggregate, item diskResponse) DiskTemperatureAggregate {
	if len(aggregates) == 0 {
		return DiskTemperatureAggregate{}
	}
	keys := []string{
		strings.TrimSpace(item.Name),
		strings.TrimSpace(item.Identifier),
		strings.TrimSpace(item.Serial),
	}
	for _, key := range keys {
		if key == "" {
			continue
		}
		if aggregate, ok := aggregates[key]; ok {
			return aggregate
		}
	}
	return DiskTemperatureAggregate{}
}

func temperatureAggregateForTrueNASDiskMap(aggregates map[string]DiskTemperatureAggregate, item map[string]any) DiskTemperatureAggregate {
	if len(aggregates) == 0 {
		return DiskTemperatureAggregate{}
	}
	keys := []string{
		strings.TrimSpace(readStringAny(item, "name", "devname")),
		strings.TrimSpace(readStringAny(item, "identifier", "id")),
		strings.TrimSpace(readStringAny(item, "serial")),
	}
	for _, key := range keys {
		if key == "" {
			continue
		}
		if aggregate, ok := aggregates[key]; ok {
			return aggregate
		}
	}
	return DiskTemperatureAggregate{}
}

func parseDiskTemperatures(raw any) map[string]int {
	switch typed := raw.(type) {
	case nil:
		return nil
	case map[string]any:
		temperatures := make(map[string]int, len(typed))
		for diskName, value := range typed {
			appendDiskTemperature(temperatures, diskName, value)
		}
		if len(temperatures) == 0 {
			return nil
		}
		return temperatures
	case []any:
		temperatures := make(map[string]int, len(typed))
		for _, entry := range typed {
			record, ok := entry.(map[string]any)
			if !ok {
				continue
			}
			diskName := readStringAny(record, "disk", "name", "devname", "identifier", "serial")
			if diskName == "" {
				continue
			}
			value, found := firstAny(
				record,
				"temperature",
				"temp",
				"temperature_celsius",
				"temperatureCelsius",
				"value",
				"parsed",
			)
			if !found {
				continue
			}
			appendDiskTemperature(temperatures, diskName, value)
		}
		if len(temperatures) == 0 {
			return nil
		}
		return temperatures
	default:
		return nil
	}
}

func appendDiskTemperature(out map[string]int, diskName string, value any) {
	if out == nil {
		return
	}
	diskName = strings.TrimSpace(diskName)
	if diskName == "" || value == nil {
		return
	}
	if nested, ok := value.(map[string]any); ok {
		if parsed, ok := firstAny(nested, "parsed", "rawvalue", "value"); ok {
			value = parsed
		}
	}
	temperature, ok := parseInt64Any(value)
	if !ok || temperature <= 0 || temperature >= 150 {
		return
	}
	out[diskName] = int(temperature)
}

type trueNASRPCClient struct {
	conn   *websocket.Conn
	nextID int64
}

func (c *trueNASRPCClient) subscribe(ctx context.Context, event string) (string, error) {
	var subscriptionID string
	if err := c.call(ctx, "core.subscribe", []any{event}, &subscriptionID); err != nil {
		return "", err
	}
	subscriptionID = strings.TrimSpace(subscriptionID)
	if subscriptionID == "" {
		return "", &discardRPCSessionError{err: fmt.Errorf("truenas rpc core.subscribe returned an empty subscription id")}
	}
	return subscriptionID, nil
}

func (c *trueNASRPCClient) unsubscribe(ctx context.Context, subscriptionID string) error {
	subscriptionID = strings.TrimSpace(subscriptionID)
	if subscriptionID == "" {
		return fmt.Errorf("truenas rpc subscription id is required")
	}
	if err := c.call(ctx, "core.unsubscribe", []any{subscriptionID}, nil); err != nil {
		return &discardRPCSessionError{err: fmt.Errorf("unsubscribe %q: %w", subscriptionID, err)}
	}
	return nil
}

type trueNASRPCRequest struct {
	JSONRPC string `json:"jsonrpc"`
	ID      int64  `json:"id"`
	Method  string `json:"method"`
	Params  any    `json:"params,omitempty"`
}

type trueNASRPCResponse struct {
	JSONRPC string           `json:"jsonrpc"`
	ID      int64            `json:"id,omitempty"`
	Method  string           `json:"method,omitempty"`
	Result  json.RawMessage  `json:"result,omitempty"`
	Params  json.RawMessage  `json:"params,omitempty"`
	Error   *trueNASRPCError `json:"error,omitempty"`
}

type trueNASRPCError struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}

type trueNASCollectionUpdate struct {
	Collection string                 `json:"collection"`
	Fields     []trueNASAppStatsEvent `json:"fields"`
}

type trueNASAppStatsEvent struct {
	AppName  string                   `json:"app_name"`
	CPUUsage int64                    `json:"cpu_usage"`
	Memory   int64                    `json:"memory"`
	Networks []trueNASAppNetworkStats `json:"networks"`
	BlkIO    trueNASAppBlkIOStats     `json:"blkio"`
}

type trueNASAppNetworkStats struct {
	InterfaceName string `json:"interface_name"`
	RXBytes       int64  `json:"rx_bytes"`
	TXBytes       int64  `json:"tx_bytes"`
}

type trueNASAppBlkIOStats struct {
	Read  int64 `json:"read"`
	Write int64 `json:"write"`
}

type trueNASAppLogNotification struct {
	Collection string `json:"collection"`
	Fields     any    `json:"fields"`
}

type trueNASRealtimeNotification struct {
	Collection string         `json:"collection"`
	Fields     map[string]any `json:"fields"`
}

type trueNASReportingGetDataResponse struct {
	Name         string                       `json:"name"`
	Identifier   any                          `json:"identifier"`
	Data         []any                        `json:"data"`
	Aggregations trueNASReportingAggregations `json:"aggregations"`
	Start        int64                        `json:"start"`
	End          int64                        `json:"end"`
	Legend       []string                     `json:"legend"`
}

type trueNASReportingAggregations struct {
	Min  any `json:"min"`
	Mean any `json:"mean"`
	Max  any `json:"max"`
}

func (c *Client) dialRPC(ctx context.Context) (*websocket.Conn, error) {
	if c == nil {
		return nil, fmt.Errorf("truenas client is nil")
	}
	dialer := websocket.Dialer{
		Proxy: http.ProxyFromEnvironment,
	}
	if c.config.UseHTTPS {
		tlsConfig, err := buildTLSConfig(c.config.InsecureSkipVerify, c.config.Fingerprint)
		if err != nil {
			return nil, err
		}
		dialer.TLSClientConfig = tlsConfig
	}
	if deadline, ok := ctx.Deadline(); ok {
		timeout := time.Until(deadline)
		if timeout > 0 {
			dialer.HandshakeTimeout = timeout
		}
	}

	conn, response, err := dialer.DialContext(ctx, c.rpcURL, nil)
	if err != nil {
		statusCode := 0
		if response != nil {
			statusCode = response.StatusCode
		}
		return nil, &RPCHandshakeError{StatusCode: statusCode, Err: err}
	}
	return conn, nil
}

func (c *trueNASRPCClient) authenticate(ctx context.Context, config ClientConfig) (string, error) {
	if apiKey := strings.TrimSpace(config.APIKey); apiKey != "" {
		if username := strings.TrimSpace(config.Username); username != "" {
			var response struct {
				ResponseType string `json:"response_type"`
			}
			err := c.call(ctx, "auth.login_ex", []any{map[string]any{
				"mechanism": "API_KEY_PLAIN",
				"username":  username,
				"api_key":   apiKey,
				"login_options": map[string]any{
					"user_info": false,
				},
			}}, &response)
			if err != nil {
				return "", err
			}
			if !strings.EqualFold(strings.TrimSpace(response.ResponseType), "SUCCESS") {
				return "", &RPCAuthError{
					Mechanism:    "api-key-plain",
					ResponseType: strings.TrimSpace(response.ResponseType),
				}
			}
			return "api-key-plain", nil
		}

		var authenticated bool
		if err := c.call(ctx, "auth.login_with_api_key", []any{apiKey}, &authenticated); err != nil {
			if isMethodUnavailable(err) {
				return "", fmt.Errorf("truenas rpc API-key authentication on this release requires the API key owner username; edit this connection and add it: %w", err)
			}
			return "", err
		}
		if !authenticated {
			return "", &RPCAuthError{Mechanism: "legacy-api-key"}
		}
		return "legacy-api-key", nil
	}
	if config.Username != "" || config.Password != "" {
		var response struct {
			ResponseType string `json:"response_type"`
		}
		err := c.call(ctx, "auth.login_ex", []any{map[string]any{
			"mechanism": "PASSWORD_PLAIN",
			"username":  config.Username,
			"password":  config.Password,
			"login_options": map[string]any{
				"user_info": false,
			},
		}}, &response)
		if err != nil {
			return "", err
		}
		if !strings.EqualFold(strings.TrimSpace(response.ResponseType), "SUCCESS") {
			return "", &RPCAuthError{
				Mechanism:    "password-plain",
				ResponseType: strings.TrimSpace(response.ResponseType),
			}
		}
		return "password-plain", nil
	}
	return "", &RPCAuthError{
		Mechanism:    "configuration",
		ResponseType: "credentials required",
	}
}

func (c *trueNASRPCClient) call(ctx context.Context, method string, params any, result any) error {
	if c == nil || c.conn == nil {
		return fmt.Errorf("truenas rpc connection is nil")
	}
	stopContext := c.armContext(ctx)
	defer stopContext()

	request := trueNASRPCRequest{
		JSONRPC: "2.0",
		ID:      c.nextID,
		Method:  method,
		Params:  params,
	}
	c.nextID++

	if deadline, ok := ctx.Deadline(); ok {
		_ = c.conn.SetWriteDeadline(deadline)
	}
	if err := c.conn.WriteJSON(request); err != nil {
		return &RPCTransportError{Method: method, Phase: "write", Err: err}
	}

	for {
		if deadline, ok := ctx.Deadline(); ok {
			_ = c.conn.SetReadDeadline(deadline)
		}

		var message trueNASRPCResponse
		if err := c.conn.ReadJSON(&message); err != nil {
			return &RPCTransportError{Method: method, Phase: "read", Err: err}
		}
		if message.Method != "" {
			continue
		}
		if message.ID != request.ID {
			continue
		}
		if message.Error != nil {
			return rpcErrorFromWire(method, message.Error)
		}
		if result == nil || len(message.Result) == 0 || string(message.Result) == "null" {
			return nil
		}
		if err := json.Unmarshal(message.Result, result); err != nil {
			return fmt.Errorf("decode truenas rpc %s response: %w", method, err)
		}
		return nil
	}
}

func (c *trueNASRPCClient) readAppStatsEvent(ctx context.Context, intervalSeconds int) (map[string]AppStats, error) {
	if c == nil || c.conn == nil {
		return nil, fmt.Errorf("truenas rpc connection is nil")
	}
	stopContext := c.armContext(ctx)
	defer stopContext()

	for {
		if deadline, ok := ctx.Deadline(); ok {
			_ = c.conn.SetReadDeadline(deadline)
		}

		var message trueNASRPCResponse
		if err := c.conn.ReadJSON(&message); err != nil {
			return nil, &RPCTransportError{Method: "app.stats", Phase: "read", Err: err}
		}
		if message.Method == "" {
			if message.Error != nil {
				return nil, rpcErrorFromWire("app.stats", message.Error)
			}
			continue
		}
		if message.Method != "collection_update" || len(message.Params) == 0 {
			continue
		}

		var update trueNASCollectionUpdate
		if err := json.Unmarshal(message.Params, &update); err != nil {
			return nil, fmt.Errorf("decode truenas app.stats notification: %w", err)
		}
		if !strings.HasPrefix(strings.TrimSpace(update.Collection), "app.stats") {
			continue
		}

		collectedAt := time.Now().UTC()
		stats := make(map[string]AppStats, len(update.Fields))
		for _, field := range update.Fields {
			key := normalizeAppStatsKey(field.AppName)
			if key == "" {
				continue
			}

			appStats := AppStats{
				CPUPercent:      float64(field.CPUUsage),
				MemoryBytes:     field.Memory,
				BlockReadBytes:  field.BlkIO.Read,
				BlockWriteBytes: field.BlkIO.Write,
				IntervalSeconds: intervalSeconds,
				CollectedAt:     collectedAt,
			}
			if len(field.Networks) > 0 {
				appStats.Interfaces = make([]AppInterfaceStats, 0, len(field.Networks))
			}
			for _, network := range field.Networks {
				appStats.NetInRate += float64(network.RXBytes)
				appStats.NetOutRate += float64(network.TXBytes)
				appStats.Interfaces = append(appStats.Interfaces, AppInterfaceStats{
					Name:      strings.TrimSpace(network.InterfaceName),
					RxBytesPS: float64(network.RXBytes),
					TxBytesPS: float64(network.TXBytes),
				})
			}
			stats[key] = appStats
		}
		return stats, nil
	}
}

func (c *trueNASRPCClient) readAppLogEvents(ctx context.Context, tailLines int) ([]AppLogLine, bool, error) {
	if c == nil || c.conn == nil {
		return nil, false, fmt.Errorf("truenas rpc connection is nil")
	}
	stopContext := c.armContext(ctx)
	defer stopContext()
	if tailLines <= 0 {
		tailLines = 100
	}
	if tailLines > maxAppLogTailLines {
		tailLines = maxAppLogTailLines
	}

	initialDeadline := time.Now().Add(defaultAppLogInitialWait)
	idleDeadline := time.Time{}
	lines := make([]AppLogLine, 0, tailLines)

	for {
		deadline := initialDeadline
		if !idleDeadline.IsZero() {
			deadline = idleDeadline
		}
		if ctxDeadline, ok := ctx.Deadline(); ok && ctxDeadline.Before(deadline) {
			deadline = ctxDeadline
		}
		_ = c.conn.SetReadDeadline(deadline)

		var message trueNASRPCResponse
		if err := c.conn.ReadJSON(&message); err != nil {
			if isTimeoutError(err) {
				// Gorilla WebSocket documents a timed-out read as terminal for
				// the connection. Preserve the collected log data, then make
				// the caller discard this stream session instead of reusing it.
				return trimAppLogLines(lines, tailLines), false, nil
			}
			return nil, false, &RPCTransportError{Method: "app.container_log_follow", Phase: "read", Err: err}
		}
		if message.Method == "" {
			if message.Error != nil {
				return nil, false, rpcErrorFromWire("app.container_log_follow", message.Error)
			}
			continue
		}
		if len(message.Params) == 0 {
			continue
		}

		var notification trueNASAppLogNotification
		if err := json.Unmarshal(message.Params, &notification); err == nil && (notification.Collection != "" || notification.Fields != nil) {
			if notification.Collection != "" && !strings.HasPrefix(strings.TrimSpace(notification.Collection), "app.container_log_follow") {
				continue
			}
			appended := appendAppLogLines(lines, notification.Fields)
			if len(appended) > len(lines) {
				lines = appended
				if len(lines) >= tailLines {
					return trimAppLogLines(lines, tailLines), true, nil
				}
				idleDeadline = time.Now().Add(defaultAppLogIdleWait)
			}
			continue
		}

		var raw any
		if err := json.Unmarshal(message.Params, &raw); err != nil {
			return nil, false, fmt.Errorf("decode truenas app.container_log_follow notification: %w", err)
		}
		appended := appendAppLogLines(lines, raw)
		if len(appended) > len(lines) {
			lines = appended
			if len(lines) >= tailLines {
				return trimAppLogLines(lines, tailLines), true, nil
			}
			idleDeadline = time.Now().Add(defaultAppLogIdleWait)
		}
	}
}

func appendAppLogLines(lines []AppLogLine, raw any) []AppLogLine {
	for _, line := range extractAppLogLines(raw) {
		if strings.TrimSpace(line.Data) == "" {
			continue
		}
		lines = append(lines, line)
	}
	return lines
}

func extractAppLogLines(raw any) []AppLogLine {
	switch typed := raw.(type) {
	case nil:
		return nil
	case []any:
		var lines []AppLogLine
		for _, entry := range typed {
			lines = append(lines, extractAppLogLines(entry)...)
		}
		return lines
	case map[string]any:
		if fields, ok := typed["fields"]; ok {
			return extractAppLogLines(fields)
		}
		if data, ok := typed["data"]; ok {
			text := strings.TrimSpace(fmt.Sprint(data))
			if text == "" || text == "<nil>" {
				return nil
			}
			line := AppLogLine{Data: text}
			if timestamp, ok := typed["timestamp"]; ok && timestamp != nil {
				ts := strings.TrimSpace(fmt.Sprint(timestamp))
				if ts != "" && ts != "<nil>" {
					line.Timestamp = ts
				}
			}
			return []AppLogLine{line}
		}
		return nil
	default:
		return nil
	}
}

func trimAppLogLines(lines []AppLogLine, tailLines int) []AppLogLine {
	if len(lines) == 0 {
		return nil
	}
	if tailLines > 0 && len(lines) > tailLines {
		lines = lines[len(lines)-tailLines:]
	}
	out := make([]AppLogLine, len(lines))
	copy(out, lines)
	return out
}

func isTimeoutError(err error) bool {
	var netErr net.Error
	return errors.As(err, &netErr) && netErr.Timeout()
}

// armContext makes websocket I/O observe cancellation even when a caller
// supplies a cancellable context without a deadline. The cleanup waits for the
// cancellation watcher before clearing deadlines, preventing a completed call
// from poisoning the next serialized exchange on this connection.
func (c *trueNASRPCClient) armContext(ctx context.Context) func() {
	if c == nil || c.conn == nil {
		return func() {}
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if deadline, ok := ctx.Deadline(); ok {
		_ = c.conn.SetReadDeadline(deadline)
		_ = c.conn.SetWriteDeadline(deadline)
	} else {
		_ = c.conn.SetReadDeadline(time.Time{})
		_ = c.conn.SetWriteDeadline(time.Time{})
	}

	cancelled := ctx.Done()
	if cancelled == nil {
		return func() {
			_ = c.conn.SetReadDeadline(time.Time{})
			_ = c.conn.SetWriteDeadline(time.Time{})
		}
	}

	stop := make(chan struct{})
	stopped := make(chan struct{})
	go func() {
		defer close(stopped)
		select {
		case <-cancelled:
			now := time.Now()
			_ = c.conn.SetReadDeadline(now)
			_ = c.conn.SetWriteDeadline(now)
		case <-stop:
		}
	}()
	return func() {
		close(stop)
		<-stopped
		_ = c.conn.SetReadDeadline(time.Time{})
		_ = c.conn.SetWriteDeadline(time.Time{})
	}
}

func (c *trueNASRPCClient) getSystemMetricHistory(ctx context.Context, duration time.Duration) (*SystemMetricHistory, error) {
	if c == nil || c.conn == nil {
		return nil, fmt.Errorf("truenas rpc connection is nil")
	}
	if duration <= 0 {
		duration = 24 * time.Hour
	}

	end := time.Now().Unix()
	start := end - int64(duration.Seconds())
	if start <= 0 {
		start = end
	}

	response, err := c.getReportingDataWithQuery(ctx, []map[string]any{
		{"name": "cpu", "identifier": nil},
		{"name": "memory", "identifier": nil},
		{"name": "interface", "identifier": nil},
		{"name": "disk", "identifier": nil},
	}, map[string]any{
		"aggregate": false,
		"start":     start,
		"end":       end,
	})
	if err != nil {
		return nil, err
	}
	return parseSystemMetricHistory(response), nil
}

func (c *trueNASRPCClient) getSystemTemperatures(ctx context.Context) (map[string]float64, error) {
	if c == nil || c.conn == nil {
		return nil, fmt.Errorf("truenas rpc connection is nil")
	}

	response, err := c.getReportingData(ctx, []map[string]any{{
		"name":       "cputemp",
		"identifier": nil,
	}})
	if err != nil {
		return nil, err
	}
	return parseSystemTemperatures(response), nil
}

func (c *trueNASRPCClient) getDiskTemperatures(ctx context.Context, identifiers []string) (map[string]int, error) {
	if c == nil || c.conn == nil {
		return nil, fmt.Errorf("truenas rpc connection is nil")
	}

	graphs := make([]map[string]any, 0, len(identifiers))
	for _, identifier := range dedupeStrings(identifiers) {
		if identifier == "" {
			continue
		}
		graphs = append(graphs, map[string]any{
			"name":       "disktemp",
			"identifier": identifier,
		})
	}
	if len(graphs) == 0 {
		return nil, fmt.Errorf("truenas rpc disk temperature query requires at least one identifier")
	}

	response, err := c.getReportingData(ctx, graphs)
	if err != nil {
		return nil, err
	}
	return parseReportingDiskTemperatures(response), nil
}

func (c *trueNASRPCClient) getDiskTemperatureAggregates(ctx context.Context, identifiers []string, windowDays int) (map[string]DiskTemperatureAggregate, error) {
	if c == nil || c.conn == nil {
		return nil, fmt.Errorf("truenas rpc connection is nil")
	}
	identifiers = dedupeStrings(identifiers)
	if len(identifiers) == 0 {
		return nil, fmt.Errorf("truenas rpc disk temperature aggregate query requires at least one identifier")
	}
	if windowDays <= 0 {
		windowDays = defaultDiskTemperatureAggregateWindowDays
	}

	var response any
	if err := c.call(ctx, "disk.temperature_agg", []any{identifiers, windowDays}, &response); err != nil {
		return nil, err
	}
	return parseDiskTemperatureAggregates(response, windowDays), nil
}

func (c *trueNASRPCClient) getDiskTemperatureHistory(ctx context.Context, identifiers []string, duration time.Duration) (map[string][]TimeSeriesPoint, error) {
	if c == nil || c.conn == nil {
		return nil, fmt.Errorf("truenas rpc connection is nil")
	}
	identifiers = dedupeStrings(identifiers)
	if len(identifiers) == 0 {
		return nil, fmt.Errorf("truenas rpc disk temperature history query requires at least one identifier")
	}
	if duration <= 0 {
		duration = 24 * time.Hour
	}

	end := time.Now().Unix()
	start := end - int64(duration.Seconds())
	if start <= 0 {
		start = end
	}

	graphs := make([]map[string]any, 0, len(identifiers))
	for _, identifier := range identifiers {
		graphs = append(graphs, map[string]any{
			"name":       "disktemp",
			"identifier": identifier,
		})
	}

	response, err := c.getReportingDataWithQuery(ctx, graphs, map[string]any{
		"aggregate": false,
		"start":     start,
		"end":       end,
	})
	if err != nil {
		return nil, err
	}
	return parseReportingDiskTemperatureHistory(response), nil
}

func (c *trueNASRPCClient) getReportingData(ctx context.Context, graphs []map[string]any) ([]trueNASReportingGetDataResponse, error) {
	if c == nil || c.conn == nil {
		return nil, fmt.Errorf("truenas rpc connection is nil")
	}
	if len(graphs) == 0 {
		return nil, fmt.Errorf("truenas reporting query requires at least one graph")
	}

	end := time.Now().Unix()
	start := end - 300
	if start <= 0 {
		start = end
	}

	return c.getReportingDataWithQuery(ctx, graphs, map[string]any{
		"aggregate": true,
		"start":     start,
		"end":       end,
	})
}

func (c *trueNASRPCClient) getReportingDataWithQuery(ctx context.Context, graphs []map[string]any, query map[string]any) ([]trueNASReportingGetDataResponse, error) {
	if c == nil || c.conn == nil {
		return nil, fmt.Errorf("truenas rpc connection is nil")
	}
	if len(graphs) == 0 {
		return nil, fmt.Errorf("truenas reporting query requires at least one graph")
	}
	if len(query) == 0 {
		return nil, fmt.Errorf("truenas reporting query requires options")
	}

	params := []any{
		graphs,
		query,
	}
	var response []trueNASReportingGetDataResponse
	if err := c.call(ctx, "reporting.get_data", params, &response); err != nil {
		return nil, err
	}
	return response, nil
}

func (c *trueNASRPCClient) readSystemTelemetryEvent(ctx context.Context, intervalSeconds int) (*SystemInfo, error) {
	if c == nil || c.conn == nil {
		return nil, fmt.Errorf("truenas rpc connection is nil")
	}
	stopContext := c.armContext(ctx)
	defer stopContext()

	for {
		if deadline, ok := ctx.Deadline(); ok {
			_ = c.conn.SetReadDeadline(deadline)
		}

		var message trueNASRPCResponse
		if err := c.conn.ReadJSON(&message); err != nil {
			return nil, &RPCTransportError{Method: "reporting.realtime", Phase: "read", Err: err}
		}
		if message.Method == "" {
			if message.Error != nil {
				return nil, rpcErrorFromWire("reporting.realtime", message.Error)
			}
			continue
		}

		fields, ok, err := parseRealtimeFields(message, "reporting.realtime")
		if err != nil {
			return nil, err
		}
		if !ok {
			continue
		}

		return parseSystemTelemetry(fields, intervalSeconds, time.Now().UTC()), nil
	}
}

func normalizeAppStatsKey(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func parseRealtimeFields(message trueNASRPCResponse, collectionPrefix string) (map[string]any, bool, error) {
	method := strings.TrimSpace(message.Method)
	switch method {
	case "collection_update":
		var notification trueNASRealtimeNotification
		if err := json.Unmarshal(message.Params, &notification); err != nil {
			return nil, false, fmt.Errorf("decode truenas %s notification: %w", collectionPrefix, err)
		}
		if !strings.HasPrefix(strings.TrimSpace(notification.Collection), collectionPrefix) {
			return nil, false, nil
		}
		if len(notification.Fields) == 0 {
			return nil, false, nil
		}
		return notification.Fields, true, nil
	default:
		if !strings.HasPrefix(method, collectionPrefix) {
			return nil, false, nil
		}
		var payload map[string]any
		if len(message.Params) == 0 {
			return nil, false, nil
		}
		if err := json.Unmarshal(message.Params, &payload); err != nil {
			return nil, false, fmt.Errorf("decode truenas %s notification: %w", collectionPrefix, err)
		}
		if fields := readMapAny(payload, "fields"); len(fields) > 0 {
			return fields, true, nil
		}
		return payload, true, nil
	}
}

func parseSystemTelemetry(fields map[string]any, intervalSeconds int, collectedAt time.Time) *SystemInfo {
	if len(fields) == 0 {
		return &SystemInfo{
			IntervalSeconds: intervalSeconds,
			CollectedAt:     collectedAt,
		}
	}

	telemetry := &SystemInfo{
		IntervalSeconds: intervalSeconds,
		CollectedAt:     collectedAt,
	}

	cpu := readMapAny(fields, "cpu")
	cpuPercent := readFloatAny(cpu,
		"usage",
		"percent",
		"usage_percent",
		"cpu_usage",
		"cpu_percent",
		"total",
		"overall",
	)
	if cpuPercent == 0 {
		if usage := readMapAny(cpu, "usage", "total"); len(usage) > 0 {
			cpuPercent = readFloatAny(usage, "percent", "value", "usage")
		}
	}
	telemetry.CPUPercent = cpuPercent

	memory := readMapAny(fields, "memory")
	total := readInt64Any(memory, "physical_memory_total", "total", "memory_total", "total_bytes")
	available := readInt64Any(memory, "physical_memory_available", "available", "free", "available_bytes", "free_bytes")
	telemetry.MemoryTotalBytes = total
	telemetry.MemoryAvailableBytes = available

	interfaces := readMapAny(fields, "interfaces")
	for _, raw := range interfaces {
		record, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		telemetry.NetInRate += readFloatAny(record,
			"rx_bytes",
			"received_bytes",
			"received_bytes_rate",
			"rx_bytes_rate",
			"bytes_recv",
			"bytes_received",
		)
		telemetry.NetOutRate += readFloatAny(record,
			"tx_bytes",
			"sent_bytes",
			"sent_bytes_rate",
			"tx_bytes_rate",
			"bytes_sent",
			"bytes_transmitted",
		)
	}

	disks := readMapAny(fields, "disks", "disls")
	for _, raw := range disks {
		record, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		telemetry.DiskReadRate += readFloatAny(record, "read_bytes", "read_bytes_rate", "bytes_read")
		telemetry.DiskWriteRate += readFloatAny(record, "write_bytes", "write_bytes_rate", "bytes_written")
	}

	return telemetry
}

func parseSystemTemperatures(responses []trueNASReportingGetDataResponse) map[string]float64 {
	if len(responses) == 0 {
		return nil
	}

	temperatures := make(map[string]float64)
	for _, response := range responses {
		if strings.TrimSpace(strings.ToLower(response.Name)) != "cputemp" || len(response.Legend) == 0 {
			continue
		}

		values := extractReportingLegendFloatValues(response.Aggregations.Mean, response.Legend)
		if len(values) == 0 && len(response.Data) > 0 {
			values = extractReportingLegendFloatValues(response.Data[len(response.Data)-1], response.Legend)
		}
		if len(values) == 0 {
			continue
		}

		for index, legend := range response.Legend {
			value, ok := values[legend]
			if !ok || value <= 0 {
				continue
			}
			key := canonicalSystemTemperatureKey(legend, index, len(response.Legend))
			if key == "" {
				continue
			}
			temperatures[key] = value
		}
	}
	if len(temperatures) == 0 {
		return nil
	}
	return temperatures
}

func parseSystemMetricHistory(responses []trueNASReportingGetDataResponse) *SystemMetricHistory {
	if len(responses) == 0 {
		return nil
	}

	history := &SystemMetricHistory{}
	for _, response := range responses {
		name := strings.TrimSpace(strings.ToLower(response.Name))
		switch name {
		case "cpu", "memory", "interface", "disk":
		default:
			continue
		}

		for _, raw := range response.Data {
			timestamp, values, ok := parseReportingSeriesValues(raw, response.Legend)
			if !ok || timestamp.IsZero() || len(values) == 0 {
				continue
			}

			switch name {
			case "cpu":
				if value, ok := parseSystemCPUPercent(values); ok {
					history.CPUPercent = appendTimeSeriesPoint(history.CPUPercent, timestamp, value)
				}
			case "memory":
				if value, ok := parseSystemMemoryPercent(values); ok {
					history.MemoryPercent = appendTimeSeriesPoint(history.MemoryPercent, timestamp, value)
				}
				if value, ok := pickReportingValue(values, "used", "memory_used", "used_bytes", "active", "memory"); ok {
					history.MemoryUsedBytes = appendTimeSeriesPoint(history.MemoryUsedBytes, timestamp, value)
				}
				if value, ok := pickReportingValue(values, "available", "free", "available_bytes", "free_bytes"); ok {
					history.MemoryAvailableBytes = appendTimeSeriesPoint(history.MemoryAvailableBytes, timestamp, value)
				}
				if value, ok := pickReportingValue(values, "total", "memory_total", "total_bytes", "physical_memory_total"); ok {
					history.MemoryTotalBytes = appendTimeSeriesPoint(history.MemoryTotalBytes, timestamp, value)
				}
			case "interface":
				if value, ok := pickReportingValue(values, "received", "received_bytes", "rx", "rx_bytes", "netin", "in"); ok {
					history.NetInRate = appendTimeSeriesPoint(history.NetInRate, timestamp, value)
				}
				if value, ok := pickReportingValue(values, "sent", "sent_bytes", "tx", "tx_bytes", "netout", "out"); ok {
					history.NetOutRate = appendTimeSeriesPoint(history.NetOutRate, timestamp, value)
				}
			case "disk":
				if value, ok := pickReportingValue(values, "read", "read_bytes", "diskread", "bytes_read"); ok {
					history.DiskReadRate = appendTimeSeriesPoint(history.DiskReadRate, timestamp, value)
				}
				if value, ok := pickReportingValue(values, "write", "write_bytes", "diskwrite", "bytes_written"); ok {
					history.DiskWriteRate = appendTimeSeriesPoint(history.DiskWriteRate, timestamp, value)
				}
			}
		}
	}

	if len(history.CPUPercent) == 0 &&
		len(history.MemoryPercent) == 0 &&
		len(history.MemoryUsedBytes) == 0 &&
		len(history.MemoryAvailableBytes) == 0 &&
		len(history.MemoryTotalBytes) == 0 &&
		len(history.NetInRate) == 0 &&
		len(history.NetOutRate) == 0 &&
		len(history.DiskReadRate) == 0 &&
		len(history.DiskWriteRate) == 0 {
		return nil
	}
	return history
}

func parseReportingDiskTemperatures(responses []trueNASReportingGetDataResponse) map[string]int {
	if len(responses) == 0 {
		return nil
	}

	temperatures := make(map[string]int)
	for _, response := range responses {
		if strings.TrimSpace(strings.ToLower(response.Name)) != "disktemp" {
			continue
		}

		identifier := readStringAny(map[string]any{"identifier": response.Identifier}, "identifier")
		if identifier == "" {
			continue
		}

		value, ok := extractSingleReportingFloatValue(response)
		if !ok || value <= 0 {
			continue
		}
		temperatures[identifier] = int(math.Round(value))
	}
	if len(temperatures) == 0 {
		return nil
	}
	return temperatures
}

func parseReportingDiskTemperatureHistory(responses []trueNASReportingGetDataResponse) map[string][]TimeSeriesPoint {
	if len(responses) == 0 {
		return nil
	}

	history := make(map[string][]TimeSeriesPoint)
	for _, response := range responses {
		if strings.TrimSpace(strings.ToLower(response.Name)) != "disktemp" {
			continue
		}

		identifier := readStringAny(map[string]any{"identifier": response.Identifier}, "identifier")
		if identifier == "" {
			continue
		}

		points := make([]TimeSeriesPoint, 0, len(response.Data))
		for _, raw := range response.Data {
			timestamp, value, ok := parseReportingSeriesPoint(raw, response.Legend)
			if !ok || timestamp.IsZero() {
				continue
			}
			points = append(points, TimeSeriesPoint{Timestamp: timestamp, Value: value})
		}
		if len(points) == 0 {
			continue
		}
		history[identifier] = points
	}
	if len(history) == 0 {
		return nil
	}
	return history
}

func parseDiskTemperatureAggregates(raw any, defaultWindowDays int) map[string]DiskTemperatureAggregate {
	if defaultWindowDays <= 0 {
		defaultWindowDays = defaultDiskTemperatureAggregateWindowDays
	}

	aggregates := make(map[string]DiskTemperatureAggregate)
	switch typed := raw.(type) {
	case map[string]any:
		for identifier, entry := range typed {
			aggregate, ok := parseDiskTemperatureAggregateEntry(entry, defaultWindowDays)
			if !ok {
				continue
			}
			aggregates[strings.TrimSpace(identifier)] = aggregate
		}
	case []any:
		for _, entry := range typed {
			record, ok := entry.(map[string]any)
			if !ok {
				continue
			}
			identifier := readStringAny(record, "identifier", "name", "disk", "disk_name", "diskName")
			if identifier == "" {
				continue
			}
			aggregate, ok := parseDiskTemperatureAggregateEntry(record, defaultWindowDays)
			if !ok {
				continue
			}
			aggregates[identifier] = aggregate
		}
	}
	if len(aggregates) == 0 {
		return nil
	}
	return aggregates
}

func parseDiskTemperatureAggregateEntry(raw any, defaultWindowDays int) (DiskTemperatureAggregate, bool) {
	record, ok := raw.(map[string]any)
	if !ok {
		return DiskTemperatureAggregate{}, false
	}

	aggRecord := record
	for _, key := range []string{"aggregations", "aggregation", "stats", "temperature_agg", "temperatureAgg"} {
		if nested := readMapAny(record, key); len(nested) > 0 {
			aggRecord = nested
			break
		}
	}

	minimum, okMinimum := readFloatValueAny(aggRecord, "min", "minimum", "low")
	average, okAverage := readFloatValueAny(aggRecord, "avg", "average", "mean")
	maximum, okMaximum := readFloatValueAny(aggRecord, "max", "maximum", "high")
	if !okMinimum && !okAverage && !okMaximum {
		return DiskTemperatureAggregate{}, false
	}

	windowDays := readIntValueAny(record, "days", "window_days", "windowDays")
	if windowDays <= 0 {
		windowDays = readIntValueAny(aggRecord, "days", "window_days", "windowDays")
	}
	if windowDays <= 0 {
		windowDays = defaultWindowDays
	}

	return DiskTemperatureAggregate{
		WindowDays: windowDays,
		MinCelsius: minimum,
		AvgCelsius: average,
		MaxCelsius: maximum,
	}, true
}

func readFloatValueAny(record map[string]any, keys ...string) (float64, bool) {
	for _, key := range keys {
		value, ok := record[key]
		if !ok {
			continue
		}
		if parsed, ok := parseFloat64Any(value); ok {
			return parsed, true
		}
	}
	return 0, false
}

func parseReportingSeriesPoint(raw any, legends []string) (time.Time, float64, bool) {
	timestamp, values, ok := parseReportingSeriesValues(raw, legends)
	if !ok || timestamp.IsZero() {
		return time.Time{}, 0, false
	}
	for _, legend := range legends {
		if value, ok := values[legend]; ok {
			return timestamp, value, true
		}
	}
	return time.Time{}, 0, false
}

func parseReportingSeriesValues(raw any, legends []string) (time.Time, map[string]float64, bool) {
	switch typed := raw.(type) {
	case []any:
		if len(typed) < 2 {
			return time.Time{}, nil, false
		}
		timestamp, ok := parseReportingTimestampAny(typed[0])
		if !ok {
			return time.Time{}, nil, false
		}
		if parsed, ok := parseFloat64Any(typed[1]); ok && len(legends) == 1 {
			return timestamp, map[string]float64{legends[0]: parsed}, true
		}
		values := extractReportingLegendFloatValues(typed[1:], legends)
		if len(values) == 0 {
			return time.Time{}, nil, false
		}
		return timestamp, values, true
	case map[string]any:
		timestamp, ok := parseReportingTimestampAny(
			firstNonNilMapValue(typed, "timestamp", "time", "ts", "x"),
		)
		if !ok {
			return time.Time{}, nil, false
		}
		values := extractReportingLegendFloatValues(typed, legends)
		if len(values) == 0 && len(legends) == 1 {
			if value, ok := readFloatValueAny(typed, "value", "y", "temperature"); ok {
				values = map[string]float64{legends[0]: value}
			}
		}
		if len(values) == 0 {
			return time.Time{}, nil, false
		}
		return timestamp, values, true
	default:
		return time.Time{}, nil, false
	}
}

func appendTimeSeriesPoint(series []TimeSeriesPoint, timestamp time.Time, value float64) []TimeSeriesPoint {
	return append(series, TimeSeriesPoint{Timestamp: timestamp, Value: value})
}

func parseSystemCPUPercent(values map[string]float64) (float64, bool) {
	if value, ok := pickReportingValue(values, "usage", "percent", "cpu", "total", "system_cpu"); ok {
		return value, true
	}
	if idle, ok := pickReportingValue(values, "idle"); ok {
		return 100 - idle, true
	}
	return 0, false
}

func parseSystemMemoryPercent(values map[string]float64) (float64, bool) {
	if value, ok := pickReportingValue(values, "usage", "percent", "used_percent", "memory_percent"); ok {
		return value, true
	}
	used, hasUsed := pickReportingValue(values, "used", "memory_used", "used_bytes", "active", "memory")
	total, hasTotal := pickReportingValue(values, "total", "memory_total", "total_bytes", "physical_memory_total")
	if hasUsed && hasTotal && total > 0 {
		return (used / total) * 100, true
	}
	available, hasAvailable := pickReportingValue(values, "available", "free", "available_bytes", "free_bytes")
	if hasAvailable && hasTotal && total > 0 {
		return ((total - available) / total) * 100, true
	}
	return 0, false
}

func pickReportingValue(values map[string]float64, candidates ...string) (float64, bool) {
	if len(values) == 0 {
		return 0, false
	}

	normalized := make(map[string]float64, len(values))
	for key, value := range values {
		normalized[normalizeTemperatureLegendLabel(key)] = value
	}

	for _, candidate := range candidates {
		if value, ok := normalized[normalizeTemperatureLegendLabel(candidate)]; ok {
			return value, true
		}
	}
	return 0, false
}

func firstNonNilMapValue(record map[string]any, keys ...string) any {
	for _, key := range keys {
		if value, ok := record[key]; ok && value != nil {
			return value
		}
	}
	return nil
}

func parseReportingTimestampAny(raw any) (time.Time, bool) {
	switch typed := raw.(type) {
	case time.Time:
		if typed.IsZero() {
			return time.Time{}, false
		}
		return typed.UTC(), true
	case string:
		trimmed := strings.TrimSpace(typed)
		if trimmed == "" {
			return time.Time{}, false
		}
		if parsed, err := time.Parse(time.RFC3339, trimmed); err == nil {
			return parsed.UTC(), true
		}
		if integer, err := strconv.ParseInt(trimmed, 10, 64); err == nil {
			return unixReportingTimestamp(integer), true
		}
	case json.Number:
		if integer, err := typed.Int64(); err == nil {
			return unixReportingTimestamp(integer), true
		}
		if value, err := typed.Float64(); err == nil {
			return unixReportingTimestamp(int64(math.Round(value))), true
		}
	default:
		if value, ok := parseFloat64Any(raw); ok {
			return unixReportingTimestamp(int64(math.Round(value))), true
		}
	}
	return time.Time{}, false
}

func unixReportingTimestamp(value int64) time.Time {
	switch {
	case value >= 1_000_000_000_000:
		return time.UnixMilli(value).UTC()
	default:
		return time.Unix(value, 0).UTC()
	}
}

func readIntValueAny(record map[string]any, keys ...string) int {
	for _, key := range keys {
		value, ok := record[key]
		if !ok {
			continue
		}
		switch typed := value.(type) {
		case int:
			return typed
		case int64:
			return int(typed)
		case float64:
			return int(math.Round(typed))
		case json.Number:
			if integer, err := typed.Int64(); err == nil {
				return int(integer)
			}
			if floatValue, err := typed.Float64(); err == nil {
				return int(math.Round(floatValue))
			}
		case string:
			if parsed, err := strconv.Atoi(strings.TrimSpace(typed)); err == nil {
				return parsed
			}
		}
	}
	return 0
}

func extractReportingLegendFloatValues(raw any, legends []string) map[string]float64 {
	if raw == nil || len(legends) == 0 {
		return nil
	}

	values := make(map[string]float64)
	switch typed := raw.(type) {
	case []any:
		for index, value := range typed {
			if index >= len(legends) {
				break
			}
			if parsed, ok := parseFloat64Any(value); ok {
				values[legends[index]] = parsed
			}
		}
	case map[string]any:
		for index, legend := range legends {
			for _, key := range reportingLegendLookupKeys(legend, index) {
				value, ok := typed[key]
				if !ok {
					continue
				}
				if parsed, ok := parseFloat64Any(value); ok {
					values[legend] = parsed
					break
				}
			}
		}
		if len(values) == 0 && len(legends) == 1 {
			for _, value := range typed {
				if parsed, ok := parseFloat64Any(value); ok {
					values[legends[0]] = parsed
					break
				}
			}
		}
	}
	if len(values) == 0 {
		return nil
	}
	return values
}

func extractSingleReportingFloatValue(response trueNASReportingGetDataResponse) (float64, bool) {
	values := extractReportingLegendFloatValues(response.Aggregations.Mean, response.Legend)
	if len(values) == 0 && len(response.Data) > 0 {
		values = extractReportingLegendFloatValues(response.Data[len(response.Data)-1], response.Legend)
	}

	for _, legend := range response.Legend {
		value, ok := values[legend]
		if ok && value > 0 {
			return value, true
		}
	}
	for _, value := range values {
		if value > 0 {
			return value, true
		}
	}

	for _, raw := range []any{response.Aggregations.Mean, response.Data} {
		if value, ok := extractFirstReportingFloatValue(raw); ok && value > 0 {
			return value, true
		}
	}

	return 0, false
}

func extractFirstReportingFloatValue(raw any) (float64, bool) {
	switch typed := raw.(type) {
	case nil:
		return 0, false
	case []any:
		for _, item := range typed {
			if value, ok := extractFirstReportingFloatValue(item); ok {
				return value, true
			}
		}
	case map[string]any:
		for _, value := range typed {
			if parsed, ok := parseFloat64Any(value); ok {
				return parsed, true
			}
			if parsed, ok := extractFirstReportingFloatValue(value); ok {
				return parsed, true
			}
		}
	default:
		if parsed, ok := parseFloat64Any(raw); ok {
			return parsed, true
		}
	}
	return 0, false
}

func reportingLegendLookupKeys(legend string, index int) []string {
	trimmed := strings.TrimSpace(legend)
	keys := []string{trimmed}
	normalized := normalizeTemperatureLegendLabel(trimmed)
	if normalized != "" && normalized != trimmed {
		keys = append(keys, normalized)
	}
	keys = append(keys, strconv.Itoa(index))
	return keys
}

func canonicalSystemTemperatureKey(legend string, index int, total int) string {
	normalized := normalizeTemperatureLegendLabel(legend)
	switch {
	case normalized == "":
		if total == 1 {
			return "cpu_package"
		}
		return fmt.Sprintf("cpu_temp_%d", index)
	case normalized == "cpu" || normalized == "temp" || normalized == "temperature":
		return "cpu_package"
	case strings.Contains(normalized, "package"):
		return "cpu_package"
	case strings.HasPrefix(normalized, "cpu"):
		return normalized
	case strings.HasPrefix(normalized, "core"):
		return "cpu_" + normalized
	default:
		if total == 1 {
			return "cpu_package"
		}
		return normalized
	}
}

func normalizeTemperatureLegendLabel(value string) string {
	trimmed := strings.TrimSpace(strings.ToLower(value))
	if trimmed == "" {
		return ""
	}

	var builder strings.Builder
	builder.Grow(len(trimmed))
	lastUnderscore := false
	for _, r := range trimmed {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			builder.WriteRune(r)
			lastUnderscore = false
			continue
		}
		if !lastUnderscore {
			builder.WriteByte('_')
			lastUnderscore = true
		}
	}
	return strings.Trim(builder.String(), "_")
}

func cloneTemperatureMap(src map[string]float64) map[string]float64 {
	if len(src) == 0 {
		return nil
	}
	out := make(map[string]float64, len(src))
	for key, value := range src {
		out[key] = value
	}
	return out
}

func firstAny(record map[string]any, keys ...string) (any, bool) {
	if record == nil {
		return nil, false
	}
	for _, key := range keys {
		value, ok := record[key]
		if ok && value != nil {
			return value, true
		}
	}
	return nil, false
}

func splitSnapshotName(full string) (dataset string, snapshot string) {
	full = strings.TrimSpace(full)
	if full == "" {
		return "", ""
	}
	parts := strings.SplitN(full, "@", 2)
	if len(parts) != 2 {
		return "", ""
	}
	return strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
}

func readMapAny(record map[string]any, keys ...string) map[string]any {
	if record == nil {
		return nil
	}
	for _, key := range keys {
		value, ok := record[key]
		if !ok || value == nil {
			continue
		}
		if typed, ok := value.(map[string]any); ok {
			return typed
		}
	}
	return nil
}

func readSliceAny(record map[string]any, keys ...string) []any {
	if record == nil {
		return nil
	}
	for _, key := range keys {
		value, ok := record[key]
		if !ok || value == nil {
			continue
		}
		if typed, ok := value.([]any); ok {
			return typed
		}
	}
	return nil
}

func readBoolAny(record map[string]any, keys ...string) bool {
	if record == nil {
		return false
	}
	for _, key := range keys {
		value, ok := record[key]
		if !ok || value == nil {
			continue
		}
		switch typed := value.(type) {
		case bool:
			return typed
		case string:
			switch strings.ToLower(strings.TrimSpace(typed)) {
			case "1", "true", "yes", "on":
				return true
			}
		case map[string]any:
			if parsed, ok := firstAny(typed, "parsed", "rawvalue", "value", "raw"); ok {
				if readBoolAny(map[string]any{"value": parsed}, "value") {
					return true
				}
			}
		}
	}
	return false
}

func readIntAny(record map[string]any, keys ...string) int {
	if record == nil {
		return 0
	}
	for _, key := range keys {
		value, ok := record[key]
		if !ok || value == nil {
			continue
		}
		if parsed, ok := parseInt64Any(value); ok && parsed >= math.MinInt32 && parsed <= math.MaxInt32 {
			return int(parsed)
		}
	}
	return 0
}

func readIntSliceAny(record map[string]any, keys ...string) []int {
	if record == nil {
		return nil
	}
	for _, key := range keys {
		value, ok := record[key]
		if !ok || value == nil {
			continue
		}
		switch typed := value.(type) {
		case []any:
			out := make([]int, 0, len(typed))
			for _, item := range typed {
				if parsed, ok := parseInt64Any(item); ok && parsed >= math.MinInt32 && parsed <= math.MaxInt32 {
					out = append(out, int(parsed))
				}
			}
			return out
		default:
			if parsed, ok := parseInt64Any(value); ok && parsed >= math.MinInt32 && parsed <= math.MaxInt32 {
				return []int{int(parsed)}
			}
		}
	}
	return nil
}

func readInt64Any(record map[string]any, keys ...string) int64 {
	if record == nil {
		return 0
	}
	for _, key := range keys {
		value, ok := record[key]
		if !ok || value == nil {
			continue
		}
		if parsed, ok := parseInt64Any(value); ok {
			return parsed
		}
	}
	return 0
}

func readFloatAny(record map[string]any, keys ...string) float64 {
	if record == nil {
		return 0
	}
	for _, key := range keys {
		value, ok := record[key]
		if !ok || value == nil {
			continue
		}
		if parsed, ok := parseFloat64Any(value); ok {
			return parsed
		}
	}
	return 0
}

func parseVirtualMachines(entries []map[string]any) []VirtualMachine {
	if len(entries) == 0 {
		return nil
	}
	vms := make([]VirtualMachine, 0, len(entries))
	for _, item := range entries {
		if item == nil {
			continue
		}

		status := readMapAny(item, "status")
		deviceCounts := parseVMDeviceCounts(readSliceAny(item, "devices"))
		memory := trueNASVMMemoryBytes(readInt64Any(item, "memory"))
		minMemory := trueNASVMMemoryBytes(readInt64Any(item, "min_memory", "minMemory"))
		id := strings.TrimSpace(readStringAny(item, "id"))
		name := strings.TrimSpace(readStringAny(item, "name"))
		if id == "" {
			id = name
		}
		if name == "" {
			name = id
		}

		vm := VirtualMachine{
			ID:                    id,
			Name:                  name,
			Description:           strings.TrimSpace(readStringAny(item, "description")),
			State:                 strings.TrimSpace(readStringAny(status, "state")),
			DomainState:           strings.TrimSpace(readStringAny(status, "domain_state", "domainState")),
			PID:                   readIntAny(status, "pid"),
			VCPUs:                 readIntAny(item, "vcpus"),
			Cores:                 readIntAny(item, "cores"),
			Threads:               readIntAny(item, "threads"),
			MemoryBytes:           memory,
			MinMemoryBytes:        minMemory,
			CPUMode:               strings.TrimSpace(readStringAny(item, "cpu_mode", "cpuMode")),
			CPUModel:              strings.TrimSpace(readStringAny(item, "cpu_model", "cpuModel")),
			Bootloader:            strings.TrimSpace(readStringAny(item, "bootloader")),
			Autostart:             readBoolAny(item, "autostart"),
			SuspendOnSnapshot:     readBoolAny(item, "suspend_on_snapshot", "suspendOnSnapshot"),
			TrustedPlatformModule: readBoolAny(item, "trusted_platform_module", "trustedPlatformModule"),
			SecureBoot:            readBoolAny(item, "enable_secure_boot", "enableSecureBoot"),
			Time:                  strings.TrimSpace(readStringAny(item, "time")),
			ArchType:              strings.TrimSpace(readStringAny(item, "arch_type", "archType")),
			MachineType:           strings.TrimSpace(readStringAny(item, "machine_type", "machineType")),
			UUID:                  strings.TrimSpace(readStringAny(item, "uuid")),
			DisplayAvailable:      readBoolAny(item, "display_available", "displayAvailable"),
			DeviceCount:           deviceCounts.total,
			DiskCount:             deviceCounts.disks,
			NICCount:              deviceCounts.nics,
			DisplayCount:          deviceCounts.displays,
			CDROMCount:            deviceCounts.cdroms,
			USBCount:              deviceCounts.usbs,
			PCICount:              deviceCounts.pcis,
		}
		if vm.State == "" {
			vm.State = strings.TrimSpace(readStringAny(item, "state"))
		}
		if vm.DomainState == "" {
			vm.DomainState = strings.TrimSpace(readStringAny(item, "domain_state", "domainState"))
		}
		if vm.DeviceCount == 0 {
			vm.DeviceCount = readIntAny(item, "device_count", "deviceCount")
		}

		if vm.ID == "" && vm.Name == "" {
			continue
		}
		vms = append(vms, vm)
	}
	if len(vms) == 0 {
		return nil
	}
	return vms
}

func parseNetworkShares(entries []map[string]any, protocol string) []NetworkShare {
	if len(entries) == 0 {
		return nil
	}
	protocol = strings.ToUpper(strings.TrimSpace(protocol))
	shares := make([]NetworkShare, 0, len(entries))
	for _, item := range entries {
		if item == nil {
			continue
		}

		share := NetworkShare{
			ID:                     strings.TrimSpace(readStringAny(item, "id")),
			Name:                   strings.TrimSpace(readStringAny(item, "name")),
			Protocol:               protocol,
			Path:                   strings.TrimSpace(readStringAny(item, "path")),
			Dataset:                strings.TrimSpace(readStringAny(item, "dataset")),
			RelativePath:           strings.TrimSpace(readStringAny(item, "relative_path", "relativePath")),
			Comment:                strings.TrimSpace(readStringAny(item, "comment")),
			Enabled:                readBoolAnyDefault(item, true, "enabled"),
			Locked:                 readBoolAny(item, "locked"),
			Aliases:                dedupeStrings(readStringSliceAny(item, "aliases")),
			Hosts:                  dedupeStrings(readStringSliceAny(item, "hosts")),
			Networks:               dedupeStrings(readStringSliceAny(item, "networks")),
			Security:               dedupeStrings(readStringSliceAny(item, "security")),
			MapRootUser:            strings.TrimSpace(readStringAny(item, "maproot_user", "maprootUser")),
			MapRootGroup:           strings.TrimSpace(readStringAny(item, "maproot_group", "maprootGroup")),
			MapAllUser:             strings.TrimSpace(readStringAny(item, "mapall_user", "mapallUser")),
			MapAllGroup:            strings.TrimSpace(readStringAny(item, "mapall_group", "mapallGroup")),
			ExposeSnapshots:        readBoolAny(item, "expose_snapshots", "exposeSnapshots"),
			AccessBasedEnumeration: readBoolAny(item, "access_based_share_enumeration", "accessBasedShareEnumeration"),
		}

		switch protocol {
		case "SMB":
			share.ReadOnly = readBoolAny(item, "readonly", "read_only", "readOnly")
			share.Browsable = readBoolAnyDefault(item, true, "browsable")
			share.AuditEnabled = readBoolAny(readMapAny(item, "audit"), "enable")
		case "NFS":
			share.ReadOnly = readBoolAny(item, "ro", "readonly", "read_only", "readOnly")
		}

		if share.Dataset == "" {
			share.Dataset = datasetFromSharePath(share.Path)
		}
		if share.Name == "" {
			share.Name = networkShareDisplayName(share)
		}
		if share.ID == "" {
			share.ID = share.Name
		}
		if share.ID == "" && share.Path == "" {
			continue
		}
		shares = append(shares, share)
	}
	if len(shares) == 0 {
		return nil
	}
	return shares
}

func readBoolAnyDefault(record map[string]any, defaultValue bool, keys ...string) bool {
	if record == nil {
		return defaultValue
	}
	for _, key := range keys {
		if value, ok := record[key]; ok && value != nil {
			return readBoolAny(map[string]any{"value": value}, "value")
		}
	}
	return defaultValue
}

func datasetFromSharePath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" || strings.EqualFold(path, "EXTERNAL") {
		return ""
	}
	path = strings.TrimPrefix(path, "/mnt/")
	path = strings.Trim(path, "/")
	if path == "" {
		return ""
	}
	parts := strings.Split(path, "/")
	if len(parts) == 1 {
		return strings.TrimSpace(parts[0])
	}
	return strings.TrimSpace(parts[0] + "/" + parts[1])
}

func poolFromSharePath(path string) string {
	dataset := datasetFromSharePath(path)
	if dataset == "" {
		return ""
	}
	if idx := strings.Index(dataset, "/"); idx > 0 {
		return strings.TrimSpace(dataset[:idx])
	}
	return strings.TrimSpace(dataset)
}

type vmDeviceCounts struct {
	total    int
	disks    int
	nics     int
	displays int
	cdroms   int
	usbs     int
	pcis     int
}

func parseVMDeviceCounts(entries []any) vmDeviceCounts {
	counts := vmDeviceCounts{total: len(entries)}
	for _, entry := range entries {
		record, ok := entry.(map[string]any)
		if !ok {
			continue
		}
		attributes := readMapAny(record, "attributes")
		dtype := strings.ToUpper(strings.TrimSpace(readStringAny(attributes, "dtype")))
		if dtype == "" {
			dtype = strings.ToUpper(strings.TrimSpace(readStringAny(record, "dtype", "type")))
		}
		switch dtype {
		case "DISK", "RAW":
			counts.disks++
		case "NIC":
			counts.nics++
		case "DISPLAY":
			counts.displays++
		case "CDROM":
			counts.cdroms++
		case "USB":
			counts.usbs++
		case "PCI":
			counts.pcis++
		}
	}
	return counts
}

func trueNASVMMemoryBytes(memory int64) int64 {
	if memory <= 0 {
		return 0
	}
	if memory >= 1<<30 {
		return memory
	}
	return memory * 1024 * 1024
}

func parseAppPorts(entries []any) []AppPort {
	if len(entries) == 0 {
		return nil
	}
	ports := make([]AppPort, 0, len(entries))
	for _, entry := range entries {
		record, ok := entry.(map[string]any)
		if !ok {
			continue
		}

		port := AppPort{
			ContainerPort: readIntAny(record, "container_port", "containerPort"),
			Protocol:      strings.ToLower(strings.TrimSpace(readStringAny(record, "protocol"))),
			HostPorts:     parseAppHostPorts(readSliceAny(record, "host_ports", "hostPorts")),
		}
		if port.ContainerPort == 0 && len(port.HostPorts) == 0 {
			continue
		}
		ports = append(ports, port)
	}
	if len(ports) == 0 {
		return nil
	}
	return ports
}

func parseAppHostPorts(entries []any) []AppHostPort {
	if len(entries) == 0 {
		return nil
	}
	ports := make([]AppHostPort, 0, len(entries))
	for _, entry := range entries {
		record, ok := entry.(map[string]any)
		if !ok {
			continue
		}
		hostPort := AppHostPort{
			HostPort: readIntAny(record, "host_port", "hostPort"),
			HostIP:   strings.TrimSpace(readStringAny(record, "host_ip", "hostIp")),
		}
		if hostPort.HostPort == 0 && hostPort.HostIP == "" {
			continue
		}
		ports = append(ports, hostPort)
	}
	if len(ports) == 0 {
		return nil
	}
	return ports
}

func parseAppContainers(entries []any) []AppContainer {
	if len(entries) == 0 {
		return nil
	}
	containers := make([]AppContainer, 0, len(entries))
	for _, entry := range entries {
		record, ok := entry.(map[string]any)
		if !ok {
			continue
		}
		container := AppContainer{
			ID:           strings.TrimSpace(readStringAny(record, "id")),
			ServiceName:  strings.TrimSpace(readStringAny(record, "service_name", "serviceName")),
			Image:        strings.TrimSpace(readStringAny(record, "image")),
			State:        strings.TrimSpace(readStringAny(record, "state")),
			PortConfig:   parseAppPorts(readSliceAny(record, "port_config", "portConfig")),
			VolumeMounts: parseAppVolumes(readSliceAny(record, "volume_mounts", "volumeMounts")),
		}
		if container.ID == "" && container.ServiceName == "" && container.Image == "" {
			continue
		}
		containers = append(containers, container)
	}
	if len(containers) == 0 {
		return nil
	}
	return containers
}

func parseAppVolumes(entries []any) []AppVolume {
	if len(entries) == 0 {
		return nil
	}
	volumes := make([]AppVolume, 0, len(entries))
	for _, entry := range entries {
		record, ok := entry.(map[string]any)
		if !ok {
			continue
		}
		volume := AppVolume{
			Source:      strings.TrimSpace(readStringAny(record, "source")),
			Destination: strings.TrimSpace(readStringAny(record, "destination")),
			Mode:        strings.TrimSpace(readStringAny(record, "mode")),
			Type:        strings.TrimSpace(readStringAny(record, "type")),
		}
		if volume.Source == "" && volume.Destination == "" {
			continue
		}
		volumes = append(volumes, volume)
	}
	return dedupeAppVolumes(volumes)
}

func parseAppNetworks(entries []any) []AppNetwork {
	if len(entries) == 0 {
		return nil
	}
	networks := make([]AppNetwork, 0, len(entries))
	for _, entry := range entries {
		record, ok := entry.(map[string]any)
		if !ok {
			continue
		}
		network := AppNetwork{
			ID:     strings.TrimSpace(readStringAny(record, "id")),
			Name:   strings.TrimSpace(readStringAny(record, "name", "Name")),
			Labels: readStringMapAny(record, "labels", "Labels"),
		}
		if network.ID == "" && network.Name == "" {
			continue
		}
		networks = append(networks, network)
	}
	if len(networks) == 0 {
		return nil
	}
	return networks
}

func readStringMapAny(record map[string]any, keys ...string) map[string]string {
	if record == nil {
		return nil
	}
	for _, key := range keys {
		value, ok := record[key]
		if !ok || value == nil {
			continue
		}
		typed, ok := value.(map[string]any)
		if !ok {
			continue
		}
		out := make(map[string]string, len(typed))
		for labelKey, labelValue := range typed {
			out[strings.TrimSpace(labelKey)] = strings.TrimSpace(fmt.Sprintf("%v", labelValue))
		}
		if len(out) == 0 {
			continue
		}
		return out
	}
	return nil
}

func appImagesFromContainers(containers []AppContainer) []string {
	if len(containers) == 0 {
		return nil
	}
	images := make([]string, 0, len(containers))
	for _, container := range containers {
		image := strings.TrimSpace(container.Image)
		if image == "" {
			continue
		}
		images = append(images, image)
	}
	return images
}

func appVolumesFromContainers(containers []AppContainer) []AppVolume {
	if len(containers) == 0 {
		return nil
	}
	volumes := make([]AppVolume, 0, len(containers))
	for _, container := range containers {
		volumes = append(volumes, container.VolumeMounts...)
	}
	return volumes
}

func dedupeAppVolumes(volumes []AppVolume) []AppVolume {
	if len(volumes) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(volumes))
	out := make([]AppVolume, 0, len(volumes))
	for _, volume := range volumes {
		key := strings.Join([]string{
			strings.TrimSpace(volume.Source),
			strings.TrimSpace(volume.Destination),
			strings.TrimSpace(volume.Mode),
			strings.TrimSpace(volume.Type),
		}, "|")
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, volume)
	}
	return out
}

func dedupeStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, v := range values {
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	return out
}

func (c *Client) getJSON(ctx context.Context, method string, path string, destination any) (err error) {
	return c.requestJSON(ctx, method, path, nil, destination)
}

// postJSON issues a POST with a JSON body. REST v2.0 exposes middleware
// methods that take parameters (disk.temperatures, reporting.get_data) as
// POST endpoints whose body is a dict keyed by parameter name; a GET on
// those paths fails on every TrueNAS version.
func (c *Client) postJSON(ctx context.Context, path string, payload any, destination any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("encode truenas request body for POST %s: %w", path, err)
	}
	return c.requestJSON(ctx, http.MethodPost, path, bytes.NewReader(body), destination)
}

func (c *Client) requestJSON(ctx context.Context, method string, path string, body io.Reader, destination any) (err error) {
	request, err := c.newRequestWithBody(ctx, method, path, body)
	if err != nil {
		return fmt.Errorf("build truenas request %s %s: %w", method, path, err)
	}

	response, err := c.httpClient.Do(request)
	if err != nil {
		return fmt.Errorf("truenas request %s %s failed: %w", method, path, err)
	}
	defer func() {
		if closeErr := response.Body.Close(); closeErr != nil {
			wrappedCloseErr := fmt.Errorf("close truenas response body for %s %s: %w", method, path, closeErr)
			if err != nil {
				err = errors.Join(err, wrappedCloseErr)
				return
			}
			err = wrappedCloseErr
		}
	}()

	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		body, readErr := io.ReadAll(io.LimitReader(response.Body, 4096))
		if readErr != nil {
			return fmt.Errorf("read truenas error response body for %s %s: %w", method, path, readErr)
		}
		message := strings.TrimSpace(string(body))
		if message == "" {
			message = http.StatusText(response.StatusCode)
		}
		return &APIError{
			StatusCode: response.StatusCode,
			Method:     method,
			Path:       path,
			Body:       message,
		}
	}

	return decodeJSONResponseWithLimit(response.Body, method, path, destination)
}

func (c *Client) newRequest(ctx context.Context, method string, path string) (*http.Request, error) {
	return c.newRequestWithBody(ctx, method, path, nil)
}

func (c *Client) newRequestWithBody(ctx context.Context, method string, path string, body io.Reader) (*http.Request, error) {
	url := c.endpoint(path)
	request, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, fmt.Errorf("build truenas request %s %s: %w", method, path, err)
	}
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}

	request.Header.Set("Accept", "application/json")
	if apiKey := strings.TrimSpace(c.config.APIKey); apiKey != "" {
		request.Header.Set("Authorization", "Bearer "+apiKey)
	} else if c.config.Username != "" || c.config.Password != "" {
		request.SetBasicAuth(c.config.Username, c.config.Password)
	}

	return request, nil
}

func (c *Client) endpoint(path string) string {
	if strings.HasPrefix(path, "/") {
		return c.baseURL + path
	}
	return c.baseURL + "/" + path
}

func resolveEndpoint(host string, useHTTPS bool, port int) (bool, string, error) {
	rawHost := strings.TrimSpace(host)
	if rawHost == "" {
		return false, "", fmt.Errorf("truenas host is required")
	}

	if strings.Contains(rawHost, "://") {
		parsed, err := url.Parse(rawHost)
		if err != nil {
			return false, "", fmt.Errorf("parse truenas host %q: %w", host, err)
		}
		if parsed.Host == "" {
			return false, "", fmt.Errorf("parse truenas host %q: missing host", host)
		}
		if parsed.User != nil {
			return false, "", fmt.Errorf("parse truenas host %q: credentials are not supported", host)
		}
		if parsed.Path != "" && parsed.Path != "/" {
			return false, "", fmt.Errorf("parse truenas host %q: path is not supported", host)
		}
		if parsed.RawQuery != "" || parsed.ForceQuery {
			return false, "", fmt.Errorf("parse truenas host %q: query is not supported", host)
		}
		if parsed.Fragment != "" {
			return false, "", fmt.Errorf("parse truenas host %q: fragment is not supported", host)
		}
		switch strings.ToLower(parsed.Scheme) {
		case "https":
			useHTTPS = true
		case "http":
			useHTTPS = false
		default:
			return false, "", fmt.Errorf("unsupported truenas scheme %q", parsed.Scheme)
		}
		rawHost = parsed.Host
	}

	if !useHTTPS && port == 0 {
		// Config defaults to HTTPS when no explicit scheme/port hints are present.
		useHTTPS = true
	}

	hostName, hostPort, err := splitHostPort(rawHost)
	if err != nil {
		return false, "", fmt.Errorf("split truenas host %q: %w", rawHost, err)
	}
	if hostName == "" {
		return false, "", fmt.Errorf("invalid truenas host %q", host)
	}

	resolvedPort := port
	if resolvedPort == 0 && hostPort != "" {
		parsedPort, parseErr := strconv.Atoi(hostPort)
		if parseErr != nil {
			return false, "", fmt.Errorf("invalid truenas port %q: %w", hostPort, parseErr)
		}
		resolvedPort = parsedPort
	}
	if resolvedPort == 0 {
		if useHTTPS {
			resolvedPort = 443
		} else {
			resolvedPort = 80
		}
	}
	if resolvedPort < 1 || resolvedPort > 65535 {
		return false, "", fmt.Errorf("invalid truenas port %d", resolvedPort)
	}

	return useHTTPS, net.JoinHostPort(hostName, strconv.Itoa(resolvedPort)), nil
}

func splitHostPort(rawHost string) (string, string, error) {
	rawHost = strings.TrimSpace(rawHost)
	if rawHost == "" {
		return "", "", nil
	}

	host, port, err := net.SplitHostPort(rawHost)
	if err == nil {
		return strings.TrimSpace(host), strings.TrimSpace(port), nil
	}

	if strings.Contains(err.Error(), "missing port in address") {
		if strings.HasPrefix(rawHost, "[") && strings.HasSuffix(rawHost, "]") {
			return strings.Trim(rawHost, "[]"), "", nil
		}
		if strings.Count(rawHost, ":") > 1 {
			return rawHost, "", nil
		}
		return rawHost, "", nil
	}

	return "", "", fmt.Errorf("invalid truenas host %q: %w", rawHost, err)
}

func decodeJSONResponseWithLimit(body io.Reader, method string, path string, destination any) error {
	responseBody, err := io.ReadAll(io.LimitReader(body, maxResponseBodyBytes+1))
	if err != nil {
		return fmt.Errorf("read truenas response for %s %s: %w", method, path, err)
	}
	if int64(len(responseBody)) > maxResponseBodyBytes {
		return fmt.Errorf("decode truenas response for %s %s: response body exceeds %d bytes", method, path, maxResponseBodyBytes)
	}

	decoder := json.NewDecoder(bytes.NewReader(responseBody))
	decoder.UseNumber()
	if err := decoder.Decode(destination); err != nil {
		return fmt.Errorf("decode truenas response for %s %s: %w", method, path, err)
	}

	var trailing json.RawMessage
	if err := decoder.Decode(&trailing); err != io.EOF {
		return fmt.Errorf("decode truenas response for %s %s: unexpected trailing data", method, path)
	}

	return nil
}

func buildTLSConfig(insecureSkipVerify bool, fingerprint string) (*tls.Config, error) {
	normalizedFingerprint, err := normalizeFingerprint(fingerprint)
	if err != nil {
		return nil, err
	}

	tlsConfig := &tls.Config{
		InsecureSkipVerify: insecureSkipVerify,
		MinVersion:         tls.VersionTLS12,
	}

	if normalizedFingerprint != "" {
		tlsConfig.VerifyConnection = func(state tls.ConnectionState) error {
			if len(state.PeerCertificates) == 0 {
				return fmt.Errorf("truenas tls pinning failed: missing peer certificate")
			}

			sum := sha256.Sum256(state.PeerCertificates[0].Raw)
			actual := hex.EncodeToString(sum[:])
			if actual != normalizedFingerprint {
				return fmt.Errorf("truenas tls pinning failed: fingerprint mismatch")
			}
			return nil
		}
	}

	return tlsConfig, nil
}

func normalizeFingerprint(fingerprint string) (string, error) {
	normalized := strings.ToLower(strings.TrimSpace(fingerprint))
	normalized = strings.TrimPrefix(normalized, "sha256:")
	normalized = strings.ReplaceAll(normalized, ":", "")
	normalized = strings.ReplaceAll(normalized, " ", "")

	if normalized == "" {
		return "", nil
	}
	if len(normalized) != 64 {
		return "", fmt.Errorf("invalid truenas fingerprint %q: expected 64 hex characters", fingerprint)
	}
	if _, err := hex.DecodeString(normalized); err != nil {
		return "", fmt.Errorf("invalid truenas fingerprint %q: %w", fingerprint, err)
	}

	return normalized, nil
}

type systemInfoResponse struct {
	Hostname      string             `json:"hostname"`
	Version       string             `json:"version"`
	BuildTime     textResponseField  `json:"buildtime"`
	UptimeSeconds int64ResponseField `json:"uptime_seconds"`
	SystemSerial  string             `json:"system_serial"`
	SystemVendor  string             `json:"system_manufacturer"`
	Cores         int                `json:"cores"`
	PhysicalCores int                `json:"physical_cores"`
	Physmem       int64              `json:"physmem"`
}

type textResponseField string

func (f textResponseField) String() string {
	return string(f)
}

func (f *textResponseField) UnmarshalJSON(data []byte) error {
	value, err := parseTextResponseField(data)
	if err != nil {
		return err
	}
	*f = textResponseField(value)
	return nil
}

type int64ResponseField int64

func (f int64ResponseField) Int64() int64 {
	return int64(f)
}

func (f *int64ResponseField) UnmarshalJSON(data []byte) error {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		*f = 0
		return nil
	}
	value, err := parseInt64FromAny(trimmed)
	if err != nil {
		return fmt.Errorf("parse int64 response field: %w", err)
	}
	*f = int64ResponseField(value)
	return nil
}

type poolResponse struct {
	ID           int64           `json:"id"`
	GUID         string          `json:"guid"`
	Name         string          `json:"name"`
	Status       string          `json:"status"`
	StatusCode   string          `json:"status_code"`
	StatusDetail string          `json:"status_detail"`
	Size         int64           `json:"size"`
	Allocated    int64           `json:"allocated"`
	Free         int64           `json:"free"`
	Topology     json.RawMessage `json:"topology"`
	Scan         json.RawMessage `json:"scan"`
}

type datasetResponse struct {
	ID        string      `json:"id"`
	Name      string      `json:"name"`
	Pool      string      `json:"pool"`
	Used      nestedValue `json:"used"`
	Available nestedValue `json:"available"`
	ReadOnly  nestedValue `json:"readonly"`
	// Mounted is a pointer because the real API never sends the field
	// (see getDatasetsRPC); only fixtures and hypothetical future versions
	// populate it, and absence must stay distinguishable from false.
	Mounted   *bool  `json:"mounted"`
	Locked    bool   `json:"locked"`
	MountPath string `json:"mountpoint"`
}

type diskResponse struct {
	Identifier   string          `json:"identifier"`
	Name         string          `json:"name"`
	Serial       string          `json:"serial"`
	Size         int64           `json:"size"`
	Model        string          `json:"model"`
	Type         string          `json:"type"`
	Pool         string          `json:"pool"`
	Bus          string          `json:"bus"`
	Status       string          `json:"status"`
	SmartStatus  json.RawMessage `json:"smart_status"`
	RotationRate int             `json:"rotationrate"`
}

type alertResponse struct {
	ID        json.RawMessage `json:"id"`
	Level     string          `json:"level"`
	Formatted string          `json:"formatted"`
	Source    string          `json:"source"`
	Dismissed bool            `json:"dismissed"`
	Datetime  struct {
		Date json.RawMessage `json:"$date"`
	} `json:"datetime"`
}

type nestedValue struct {
	RawValue string          `json:"rawvalue"`
	Parsed   json.RawMessage `json:"parsed"`
}

func (n nestedValue) int64Value() (int64, error) {
	if value, err := parseInt64FromAny(n.Parsed); err == nil {
		return value, nil
	}
	if value := strings.TrimSpace(n.RawValue); value != "" {
		parsed, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return 0, fmt.Errorf("parse int64 from rawvalue %q: %w", value, err)
		}
		return parsed, nil
	}
	return 0, fmt.Errorf("missing numeric field")
}

func (n nestedValue) boolValue() (bool, error) {
	if value, err := parseBoolFromAny(n.Parsed); err == nil {
		return value, nil
	}

	raw := strings.ToLower(strings.TrimSpace(n.RawValue))
	switch raw {
	case "on", "true", "1", "yes":
		return true, nil
	case "off", "false", "0", "no":
		return false, nil
	case "":
		return false, nil
	default:
		return false, fmt.Errorf("parse bool from rawvalue %q", raw)
	}
}

func parseInt64FromAny(raw json.RawMessage) (int64, error) {
	var decoded any
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err := decoder.Decode(&decoded); err != nil {
		return 0, err
	}

	switch value := decoded.(type) {
	case json.Number:
		if integer, err := value.Int64(); err == nil {
			return integer, nil
		}
		floatValue, err := value.Float64()
		if err != nil {
			return 0, fmt.Errorf("parse json number %q: %w", value.String(), err)
		}
		return int64(floatValue), nil
	case string:
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			return 0, fmt.Errorf("empty numeric string")
		}
		integer, err := strconv.ParseInt(trimmed, 10, 64)
		if err != nil {
			return 0, fmt.Errorf("parse numeric string %q: %w", value, err)
		}
		return integer, nil
	case float64:
		return int64(value), nil
	case int64:
		return value, nil
	case nil:
		return 0, fmt.Errorf("numeric value is null")
	default:
		return 0, fmt.Errorf("unsupported numeric type %T", decoded)
	}
}

func parseBoolFromAny(raw json.RawMessage) (bool, error) {
	var decoded any
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err := decoder.Decode(&decoded); err != nil {
		return false, err
	}

	switch value := decoded.(type) {
	case bool:
		return value, nil
	case json.Number:
		num, err := value.Int64()
		if err != nil {
			return false, fmt.Errorf("parse json number %q as bool: %w", value.String(), err)
		}
		return num != 0, nil
	case string:
		switch strings.ToLower(strings.TrimSpace(value)) {
		case "on", "true", "1", "yes":
			return true, nil
		case "off", "false", "0", "no", "":
			return false, nil
		default:
			return false, fmt.Errorf("parse bool from string %q", value)
		}
	case nil:
		return false, nil
	default:
		return false, fmt.Errorf("unsupported bool type %T", decoded)
	}
}

func readStringAny(record map[string]any, keys ...string) string {
	if record == nil {
		return ""
	}
	for _, key := range keys {
		value, ok := record[key]
		if !ok || value == nil {
			continue
		}
		switch typed := value.(type) {
		case string:
			if s := strings.TrimSpace(typed); s != "" {
				return s
			}
		case json.Number:
			if s := strings.TrimSpace(typed.String()); s != "" {
				return s
			}
		case float64:
			// JSON decoder returns float64 for numbers when not using json.Number.
			if typed == float64(int64(typed)) {
				return strconv.FormatInt(int64(typed), 10)
			}
			return strconv.FormatFloat(typed, 'f', -1, 64)
		case int64:
			return strconv.FormatInt(typed, 10)
		case map[string]any:
			// Try common wrapper shapes: { "rawvalue": "...", "parsed": ... }
			if s := readStringAny(typed, "rawvalue", "value", "parsed", "raw"); s != "" {
				return s
			}
		}
	}
	return ""
}

func parseTextResponseField(raw json.RawMessage) (string, error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return "", nil
	}

	var decoded any
	decoder := json.NewDecoder(bytes.NewReader(trimmed))
	decoder.UseNumber()
	if err := decoder.Decode(&decoded); err != nil {
		return "", err
	}
	return textFromDecodedAny(decoded), nil
}

func textFromDecodedAny(value any) string {
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	case json.Number:
		return strings.TrimSpace(typed.String())
	case map[string]any:
		return readStringAny(typed, "rawvalue", "value", "parsed", "$date", "date", "datetime", "text", "string")
	default:
		return ""
	}
}

func readStringSliceAny(record map[string]any, keys ...string) []string {
	if record == nil {
		return nil
	}
	for _, key := range keys {
		value, ok := record[key]
		if !ok || value == nil {
			continue
		}
		switch typed := value.(type) {
		case []any:
			out := make([]string, 0, len(typed))
			for _, item := range typed {
				switch v := item.(type) {
				case string:
					if s := strings.TrimSpace(v); s != "" {
						out = append(out, s)
					}
				case json.Number:
					if s := strings.TrimSpace(v.String()); s != "" {
						out = append(out, s)
					}
				}
			}
			if len(out) > 0 {
				return out
			}
		case []string:
			if len(typed) > 0 {
				return append([]string(nil), typed...)
			}
		case string:
			// Comma-separated fallback.
			parts := strings.Split(typed, ",")
			out := make([]string, 0, len(parts))
			for _, part := range parts {
				if s := strings.TrimSpace(part); s != "" {
					out = append(out, s)
				}
			}
			if len(out) > 0 {
				return out
			}
		}
	}
	return nil
}

func readInt64PtrAny(record map[string]any, keys ...string) *int64 {
	if record == nil {
		return nil
	}
	for _, key := range keys {
		value, ok := record[key]
		if !ok || value == nil {
			continue
		}
		if v, ok := parseInt64Any(value); ok {
			return &v
		}
		// Common wrapper: { rawvalue, parsed }
		if nested, ok := value.(map[string]any); ok {
			if v, ok := parseInt64Any(nested["parsed"]); ok {
				return &v
			}
			if v, ok := parseInt64Any(nested["rawvalue"]); ok {
				return &v
			}
			if v, ok := parseInt64Any(nested["raw"]); ok {
				return &v
			}
		}
	}
	return nil
}

func parseInt64Any(value any) (int64, bool) {
	switch typed := value.(type) {
	case int64:
		return typed, true
	case int:
		return int64(typed), true
	case float64:
		return int64(typed), true
	case json.Number:
		if v, err := typed.Int64(); err == nil {
			return v, true
		}
		if f, err := typed.Float64(); err == nil {
			return int64(f), true
		}
	case string:
		s := strings.TrimSpace(typed)
		if s == "" {
			return 0, false
		}
		if v, err := strconv.ParseInt(s, 10, 64); err == nil {
			return v, true
		}
	case map[string]any:
		if parsed, ok := firstAny(typed, "parsed", "value", "rawvalue", "raw"); ok {
			return parseInt64Any(parsed)
		}
	}
	return 0, false
}

func parseFloat64Any(value any) (float64, bool) {
	switch typed := value.(type) {
	case float64:
		return typed, true
	case float32:
		return float64(typed), true
	case int64:
		return float64(typed), true
	case int:
		return float64(typed), true
	case json.Number:
		if v, err := typed.Float64(); err == nil {
			return v, true
		}
	case string:
		s := strings.TrimSpace(typed)
		if s == "" {
			return 0, false
		}
		if v, err := strconv.ParseFloat(s, 64); err == nil {
			return v, true
		}
	case map[string]any:
		if parsed, ok := firstAny(typed, "parsed", "value", "rawvalue", "raw"); ok {
			return parseFloat64Any(parsed)
		}
	}
	return 0, false
}

func readTimeAny(record map[string]any, keys ...string) *time.Time {
	if record == nil {
		return nil
	}
	for _, key := range keys {
		value, ok := record[key]
		if !ok || value == nil {
			continue
		}
		if t := parseTimeAny(value); t != nil {
			return t
		}
		// Wrapper: { "$date": ... }
		if nested, ok := value.(map[string]any); ok {
			if dateValue, ok := nested["$date"]; ok {
				if t := parseTimeAny(dateValue); t != nil {
					return t
				}
			}
			if parsedValue, ok := nested["parsed"]; ok {
				if t := parseTimeAny(parsedValue); t != nil {
					return t
				}
			}
			if rawValue, ok := nested["rawvalue"]; ok {
				if t := parseTimeAny(rawValue); t != nil {
					return t
				}
			}
			if rawValue, ok := nested["raw"]; ok {
				if t := parseTimeAny(rawValue); t != nil {
					return t
				}
			}
		}
	}
	return nil
}

func parseTimeAny(value any) *time.Time {
	switch typed := value.(type) {
	case time.Time:
		if typed.IsZero() {
			return nil
		}
		t := typed.UTC()
		return &t
	case string:
		s := strings.TrimSpace(typed)
		if s == "" {
			return nil
		}
		// Prefer RFC3339, but accept epoch millis/seconds as strings too.
		if parsed, err := time.Parse(time.RFC3339, s); err == nil {
			t := parsed.UTC()
			return &t
		}
		if parsed, err := time.Parse(time.RFC3339Nano, s); err == nil {
			t := parsed.UTC()
			return &t
		}
		if n, ok := parseInt64Any(s); ok {
			return epochToTimePtr(n)
		}
	case json.Number:
		if n, err := typed.Int64(); err == nil {
			return epochToTimePtr(n)
		}
		if f, err := typed.Float64(); err == nil {
			return epochToTimePtr(int64(f))
		}
	case float64:
		return epochToTimePtr(int64(typed))
	case int64:
		return epochToTimePtr(typed)
	case int:
		return epochToTimePtr(int64(typed))
	}
	return nil
}

func epochToTimePtr(value int64) *time.Time {
	if value <= 0 {
		return nil
	}
	// Heuristic: treat >= 1e12 as millis, otherwise seconds.
	var t time.Time
	if value >= 1_000_000_000_000 {
		t = time.UnixMilli(value).UTC()
	} else {
		t = time.Unix(value, 0).UTC()
	}
	return &t
}

func rawIDToString(raw json.RawMessage) (string, error) {
	var asString string
	if err := json.Unmarshal(raw, &asString); err == nil {
		return strings.TrimSpace(asString), nil
	}

	if asInt, err := parseInt64FromAny(raw); err == nil {
		return strconv.FormatInt(asInt, 10), nil
	}

	return "", fmt.Errorf("unsupported alert id: %s", string(raw))
}
