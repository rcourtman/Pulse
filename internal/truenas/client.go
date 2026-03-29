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
	"strconv"
	"strings"
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

// ClientConfig configures the TrueNAS REST API client.
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

// Client is a thin HTTP wrapper around the TrueNAS REST API v2.0.
type Client struct {
	config     ClientConfig
	httpClient *http.Client
	baseURL    string
	rpcURL     string
}

// APIError represents an HTTP-level error from the TrueNAS REST API.
type APIError struct {
	StatusCode int
	Method     string
	Path       string
	Body       string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("truenas request %s %s failed: status=%d body=%q", e.Method, e.Path, e.StatusCode, e.Body)
}

// NewClient creates a new TrueNAS REST API client.
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

	return &Client{
		config: config,
		httpClient: &http.Client{
			Timeout:   timeout,
			Transport: transport,
		},
		baseURL: fmt.Sprintf("%s://%s/api/v2.0", scheme, hostPort),
		rpcURL:  fmt.Sprintf("%s://%s/api/current", wsScheme, hostPort),
	}, nil
}

// TestConnection validates that the endpoint is reachable and authenticated.
func (c *Client) TestConnection(ctx context.Context) error {
	if _, err := c.GetSystemInfo(ctx); err != nil {
		return fmt.Errorf("truenas connection test failed: %w", err)
	}
	return nil
}

// Close releases idle HTTP transport connections held by the client.
func (c *Client) Close() {
	if c == nil || c.httpClient == nil || c.httpClient.Transport == nil {
		return
	}
	if transport, ok := c.httpClient.Transport.(interface{ CloseIdleConnections() }); ok {
		transport.CloseIdleConnections()
	}
}

// GetSystemInfo returns high-level system metadata.
func (c *Client) GetSystemInfo(ctx context.Context) (*SystemInfo, error) {
	var response systemInfoResponse
	if err := c.getJSON(ctx, http.MethodGet, "/system/info", &response); err != nil {
		return nil, err
	}

	machineID := strings.TrimSpace(response.SystemSerial)
	if machineID == "" {
		machineID = strings.TrimSpace(response.Hostname)
	}

	build := strings.TrimSpace(response.BuildTime)
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
		UptimeSeconds:    response.UptimeSeconds,
		Healthy:          true,
		MachineID:        machineID,
		CPUCount:         cpuCount,
		MemoryTotalBytes: response.Physmem,
	}, nil
}

// GetSystemTelemetry retrieves live system telemetry from the modern TrueNAS
// JSON-RPC websocket API. This path is best-effort; callers should treat any
// transport or endpoint failure as "telemetry unavailable" rather than a
// system identity failure.
func (c *Client) GetSystemTelemetry(ctx context.Context) (*SystemInfo, error) {
	conn, err := c.dialRPC(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = conn.Close() }()

	rpc := trueNASRPCClient{
		conn:   conn,
		nextID: 1,
	}
	if err := rpc.authenticate(ctx, c.config); err != nil {
		return nil, err
	}

	temperatures, err := rpc.getSystemTemperatures(ctx)
	if err != nil {
		temperatures = nil
	}

	subscriptionName := fmt.Sprintf("reporting.realtime:{\"interval\":%d}", defaultRealtimeIntervalSeconds)
	if err := rpc.call(ctx, "core.subscribe", []any{subscriptionName}, nil); err != nil {
		return nil, err
	}

	telemetry, err := rpc.readSystemTelemetryEvent(ctx, defaultRealtimeIntervalSeconds)
	if err != nil {
		return nil, err
	}
	if len(temperatures) > 0 {
		telemetry.TemperatureCelsius = cloneTemperatureMap(temperatures)
	}
	return telemetry, nil
}

// GetPools returns storage pools.
func (c *Client) GetPools(ctx context.Context) ([]Pool, error) {
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
		pools = append(pools, Pool{
			ID:         id,
			Name:       strings.TrimSpace(item.Name),
			Status:     strings.TrimSpace(item.Status),
			TotalBytes: item.Size,
			UsedBytes:  item.Allocated,
			FreeBytes:  item.Free,
		})
	}

	return pools, nil
}

// GetDatasets returns datasets and normalized capacity/read-only fields.
func (c *Client) GetDatasets(ctx context.Context) ([]Dataset, error) {
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

		datasets = append(datasets, Dataset{
			ID:         id,
			Name:       name,
			Pool:       poolName,
			UsedBytes:  used,
			AvailBytes: available,
			Mounted:    item.Mounted,
			ReadOnly:   readOnly,
		})
	}

	return datasets, nil
}

// GetDisks returns the system disk inventory.
func (c *Client) GetDisks(ctx context.Context) ([]Disk, error) {
	var response []diskResponse
	if err := c.getJSON(ctx, http.MethodGet, "/disk", &response); err != nil {
		return nil, err
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

		disks = append(disks, Disk{
			ID:                   diskID,
			Name:                 strings.TrimSpace(item.Name),
			Pool:                 strings.TrimSpace(item.Pool),
			Status:               strings.TrimSpace(item.Status),
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

	conn, err := c.dialRPC(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = conn.Close() }()

	rpc := &trueNASRPCClient{conn: conn, nextID: 1}
	if err := rpc.authenticate(ctx, c.config); err != nil {
		return nil, err
	}
	return rpc.getDiskTemperatureHistory(ctx, identifiers, duration)
}

func (c *Client) getDiskTemperaturesWithFallback(ctx context.Context, identifiers []string) (map[string]int, error) {
	var response any
	restErr := c.getJSON(ctx, http.MethodGet, "/disk/temperatures", &response)
	if restErr == nil {
		temperatures := parseDiskTemperatures(response)
		if len(temperatures) > 0 {
			return temperatures, nil
		}
	}

	if len(identifiers) == 0 {
		reportingIdentifiers, err := c.listDiskReportingIdentifiers(ctx)
		if err == nil {
			identifiers = reportingIdentifiers
		}
	}

	reportingTemperatures, reportingErr := c.getDiskTemperaturesFromReporting(ctx, identifiers)
	if reportingErr == nil && len(reportingTemperatures) > 0 {
		return reportingTemperatures, nil
	}

	if restErr != nil {
		if reportingErr != nil {
			return nil, fmt.Errorf("fetch truenas disk temperatures via rest and reporting: rest=%w reporting=%v", restErr, reportingErr)
		}
		return nil, restErr
	}

	return parseDiskTemperatures(response), nil
}

func (c *Client) listDiskReportingIdentifiers(ctx context.Context) ([]string, error) {
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

func (c *Client) getDiskTemperaturesFromReporting(ctx context.Context, identifiers []string) (map[string]int, error) {
	identifiers = dedupeStrings(identifiers)
	if len(identifiers) == 0 {
		return nil, fmt.Errorf("truenas reporting disk temperature fallback requires disk identifiers")
	}

	conn, err := c.dialRPC(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = conn.Close() }()

	rpc := &trueNASRPCClient{conn: conn, nextID: 1}
	if err := rpc.authenticate(ctx, c.config); err != nil {
		return nil, err
	}
	return rpc.getDiskTemperatures(ctx, identifiers)
}

func (c *Client) getDiskTemperatureAggregates(ctx context.Context, identifiers []string, windowDays int) (map[string]DiskTemperatureAggregate, error) {
	identifiers = dedupeStrings(identifiers)
	if len(identifiers) == 0 {
		return nil, fmt.Errorf("truenas disk temperature aggregates require disk identifiers")
	}

	conn, err := c.dialRPC(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = conn.Close() }()

	rpc := &trueNASRPCClient{conn: conn, nextID: 1}
	if err := rpc.authenticate(ctx, c.config); err != nil {
		return nil, err
	}
	return rpc.getDiskTemperatureAggregates(ctx, identifiers, windowDays)
}

// GetAlerts returns active and dismissed TrueNAS alerts.
func (c *Client) GetAlerts(ctx context.Context) ([]Alert, error) {
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

// GetApps returns the best-effort TrueNAS app inventory as canonical workload
// candidates. The API surface varies across TrueNAS releases, so we parse the
// documented app.query response shape loosely.
func (c *Client) GetApps(ctx context.Context) ([]App, error) {
	var response []map[string]any
	if err := c.getJSON(ctx, http.MethodGet, "/app", &response); err != nil {
		return nil, err
	}
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

	return apps, nil
}

// GetAppStats retrieves live TrueNAS app workload telemetry from the modern
// JSON-RPC websocket API. This path is best-effort; callers should treat any
// transport or endpoint failure as "stats unavailable" rather than inventory
// failure.
func (c *Client) GetAppStats(ctx context.Context) (map[string]AppStats, error) {
	conn, err := c.dialRPC(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = conn.Close() }()

	rpc := trueNASRPCClient{
		conn:   conn,
		nextID: 1,
	}
	if err := rpc.authenticate(ctx, c.config); err != nil {
		return nil, err
	}

	subscriptionName := fmt.Sprintf("app.stats:{\"interval\":%d}", defaultAppStatsIntervalSeconds)
	if err := rpc.call(ctx, "core.subscribe", []any{subscriptionName}, nil); err != nil {
		return nil, err
	}

	return rpc.readAppStatsEvent(ctx, defaultAppStatsIntervalSeconds)
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

	conn, err := c.dialRPC(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = conn.Close() }()

	rpc := trueNASRPCClient{
		conn:   conn,
		nextID: 1,
	}
	if err := rpc.authenticate(ctx, c.config); err != nil {
		return nil, err
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
	if err := rpc.call(ctx, "core.subscribe", []any{subscriptionName}, nil); err != nil {
		return nil, err
	}

	return rpc.readAppLogEvents(ctx, tailLines)
}

func (c *Client) executeAppAction(ctx context.Context, method, appID string) error {
	appID = strings.TrimSpace(appID)
	if appID == "" {
		return fmt.Errorf("truenas app id is required")
	}

	conn, err := c.dialRPC(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close() }()

	rpc := trueNASRPCClient{
		conn:   conn,
		nextID: 1,
	}
	if err := rpc.authenticate(ctx, c.config); err != nil {
		return err
	}
	if err := rpc.call(ctx, method, []any{appID}, nil); err != nil {
		return err
	}
	return nil
}

// GetZFSSnapshots returns a best-effort list of ZFS snapshots.
//
// NOTE: TrueNAS exposes multiple snapshot APIs across versions. We intentionally parse the
// response loosely and fall back to parsing dataset/snapshot names from the snapshot ID.
func (c *Client) GetZFSSnapshots(ctx context.Context) ([]ZFSSnapshot, error) {
	var response []map[string]any
	if err := c.getJSON(ctx, http.MethodGet, "/zfs/snapshot", &response); err != nil {
		return nil, err
	}

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
		} else if props, ok := item["properties"].(map[string]any); ok {
			createdAt = readTimeAny(props, "creation", "created", "datetime")
		}

		usedBytes := readInt64PtrAny(item, "used", "used_bytes", "usedBytes")
		referenced := readInt64PtrAny(item, "referenced", "referenced_bytes", "referencedBytes")

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

	return snapshots, nil
}

// GetReplicationTasks returns a best-effort list of replication tasks including last-run state.
func (c *Client) GetReplicationTasks(ctx context.Context) ([]ReplicationTask, error) {
	var response []map[string]any
	if err := c.getJSON(ctx, http.MethodGet, "/replication", &response); err != nil {
		return nil, err
	}

	tasks := make([]ReplicationTask, 0, len(response))
	for _, item := range response {
		id := readStringAny(item, "id")
		name := readStringAny(item, "name")
		direction := readStringAny(item, "direction")
		targetDataset := readStringAny(item, "target_dataset", "targetDataset", "target")

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
			LastRun:        lastRun,
			LastState:      strings.TrimSpace(lastState),
			LastError:      strings.TrimSpace(lastError),
			LastSnapshot:   strings.TrimSpace(lastSnapshot),
		})
	}

	return tasks, nil
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

	datasets, err := c.GetDatasets(ctx)
	if err != nil {
		return nil, fmt.Errorf("fetch truenas datasets: %w", err)
	}

	disks, err := c.GetDisks(ctx)
	if err != nil {
		return nil, fmt.Errorf("fetch truenas disks: %w", err)
	}

	alerts, err := c.GetAlerts(ctx)
	if err != nil {
		return nil, fmt.Errorf("fetch truenas alerts: %w", err)
	}

	// Recovery artifacts are best-effort: do not fail monitoring if additional endpoints are unavailable.
	apps, _ := c.GetApps(ctx)
	zfsSnapshots, _ := c.GetZFSSnapshots(ctx)
	replicationTasks, _ := c.GetReplicationTasks(ctx)

	return &FixtureSnapshot{
		CollectedAt:      time.Now().UTC(),
		System:           *system,
		Pools:            pools,
		Datasets:         datasets,
		Disks:            disks,
		Alerts:           alerts,
		Apps:             apps,
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
	if !ok || temperature <= 0 {
		return
	}
	out[diskName] = int(temperature)
}

type trueNASRPCClient struct {
	conn   *websocket.Conn
	nextID int64
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
	Code    int    `json:"code"`
	Message string `json:"message"`
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
		if response != nil {
			return nil, fmt.Errorf("dial truenas rpc websocket: status=%d: %w", response.StatusCode, err)
		}
		return nil, fmt.Errorf("dial truenas rpc websocket: %w", err)
	}
	return conn, nil
}

func (c *trueNASRPCClient) authenticate(ctx context.Context, config ClientConfig) error {
	if apiKey := strings.TrimSpace(config.APIKey); apiKey != "" {
		return c.call(ctx, "auth.login_with_api_key", []any{apiKey}, nil)
	}
	if config.Username != "" || config.Password != "" {
		return c.call(ctx, "auth.login", []any{config.Username, config.Password}, nil)
	}
	return fmt.Errorf("truenas rpc authentication requires api key or username/password")
}

func (c *trueNASRPCClient) call(ctx context.Context, method string, params any, result any) error {
	if c == nil || c.conn == nil {
		return fmt.Errorf("truenas rpc connection is nil")
	}

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
		return fmt.Errorf("write truenas rpc %s request: %w", method, err)
	}

	for {
		if deadline, ok := ctx.Deadline(); ok {
			_ = c.conn.SetReadDeadline(deadline)
		}

		var message trueNASRPCResponse
		if err := c.conn.ReadJSON(&message); err != nil {
			return fmt.Errorf("read truenas rpc %s response: %w", method, err)
		}
		if message.Method != "" {
			continue
		}
		if message.ID != request.ID {
			continue
		}
		if message.Error != nil {
			return fmt.Errorf("truenas rpc %s failed: code=%d message=%q", method, message.Error.Code, strings.TrimSpace(message.Error.Message))
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

	for {
		if deadline, ok := ctx.Deadline(); ok {
			_ = c.conn.SetReadDeadline(deadline)
		}

		var message trueNASRPCResponse
		if err := c.conn.ReadJSON(&message); err != nil {
			return nil, fmt.Errorf("read truenas rpc app.stats notification: %w", err)
		}
		if message.Method == "" {
			if message.Error != nil {
				return nil, fmt.Errorf("truenas rpc app.stats failed: code=%d message=%q", message.Error.Code, strings.TrimSpace(message.Error.Message))
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

func (c *trueNASRPCClient) readAppLogEvents(ctx context.Context, tailLines int) ([]AppLogLine, error) {
	if c == nil || c.conn == nil {
		return nil, fmt.Errorf("truenas rpc connection is nil")
	}
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
				return trimAppLogLines(lines, tailLines), nil
			}
			return nil, fmt.Errorf("read truenas rpc app.container_log_follow notification: %w", err)
		}
		if message.Method == "" {
			if message.Error != nil {
				return nil, fmt.Errorf("truenas rpc app.container_log_follow failed: code=%d message=%q", message.Error.Code, strings.TrimSpace(message.Error.Message))
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
					return trimAppLogLines(lines, tailLines), nil
				}
				idleDeadline = time.Now().Add(defaultAppLogIdleWait)
			}
			continue
		}

		var raw any
		if err := json.Unmarshal(message.Params, &raw); err != nil {
			return nil, fmt.Errorf("decode truenas app.container_log_follow notification: %w", err)
		}
		appended := appendAppLogLines(lines, raw)
		if len(appended) > len(lines) {
			lines = appended
			if len(lines) >= tailLines {
				return trimAppLogLines(lines, tailLines), nil
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

	for {
		if deadline, ok := ctx.Deadline(); ok {
			_ = c.conn.SetReadDeadline(deadline)
		}

		var message trueNASRPCResponse
		if err := c.conn.ReadJSON(&message); err != nil {
			return nil, fmt.Errorf("read truenas rpc reporting.realtime notification: %w", err)
		}
		if message.Method == "" {
			if message.Error != nil {
				return nil, fmt.Errorf("truenas rpc reporting.realtime failed: code=%d message=%q", message.Error.Code, strings.TrimSpace(message.Error.Message))
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
	switch typed := raw.(type) {
	case []any:
		if len(typed) < 2 {
			return time.Time{}, 0, false
		}
		timestamp, ok := parseReportingTimestampAny(typed[0])
		if !ok {
			return time.Time{}, 0, false
		}
		if parsed, ok := parseFloat64Any(typed[1]); ok {
			return timestamp, parsed, true
		}
		values := extractReportingLegendFloatValues(typed[1:], legends)
		for _, legend := range legends {
			if value, ok := values[legend]; ok {
				return timestamp, value, true
			}
		}
	case map[string]any:
		timestamp, ok := parseReportingTimestampAny(
			firstNonNilMapValue(typed, "timestamp", "time", "ts", "x"),
		)
		if !ok {
			return time.Time{}, 0, false
		}
		values := extractReportingLegendFloatValues(typed, legends)
		for _, legend := range legends {
			if value, ok := values[legend]; ok {
				return timestamp, value, true
			}
		}
		if value, ok := readFloatValueAny(typed, "value", "y", "temperature"); ok {
			return timestamp, value, true
		}
	}
	return time.Time{}, 0, false
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
			if parsed, ok := firstAny(typed, "parsed", "rawvalue", "value"); ok {
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
		if parsed, ok := parseInt64Any(value); ok {
			return int(parsed)
		}
	}
	return 0
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
	request, err := c.newRequest(ctx, method, path)
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
	url := c.endpoint(path)
	request, err := http.NewRequestWithContext(ctx, method, url, nil)
	if err != nil {
		return nil, fmt.Errorf("build truenas request %s %s: %w", method, path, err)
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
	Hostname      string `json:"hostname"`
	Version       string `json:"version"`
	BuildTime     string `json:"buildtime"`
	UptimeSeconds int64  `json:"uptime_seconds"`
	SystemSerial  string `json:"system_serial"`
	SystemVendor  string `json:"system_manufacturer"`
	Cores         int    `json:"cores"`
	PhysicalCores int    `json:"physical_cores"`
	Physmem       int64  `json:"physmem"`
}

type poolResponse struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	Status    string `json:"status"`
	Size      int64  `json:"size"`
	Allocated int64  `json:"allocated"`
	Free      int64  `json:"free"`
}

type datasetResponse struct {
	ID        string      `json:"id"`
	Name      string      `json:"name"`
	Pool      string      `json:"pool"`
	Used      nestedValue `json:"used"`
	Available nestedValue `json:"available"`
	ReadOnly  nestedValue `json:"readonly"`
	Mounted   bool        `json:"mounted"`
	MountPath string      `json:"mountpoint"`
}

type diskResponse struct {
	Identifier   string `json:"identifier"`
	Name         string `json:"name"`
	Serial       string `json:"serial"`
	Size         int64  `json:"size"`
	Model        string `json:"model"`
	Type         string `json:"type"`
	Pool         string `json:"pool"`
	Bus          string `json:"bus"`
	Status       string `json:"status"`
	RotationRate int    `json:"rotationrate"`
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
			if s := readStringAny(typed, "rawvalue", "value", "parsed"); s != "" {
				return s
			}
		}
	}
	return ""
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
		if parsed, ok := firstAny(typed, "parsed", "value", "rawvalue"); ok {
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
