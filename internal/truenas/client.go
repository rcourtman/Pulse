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
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const defaultHTTPTimeout = 30 * time.Second

const maxResponseBodyBytes int64 = 4 * 1024 * 1024

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

	return &Client{
		config: config,
		httpClient: &http.Client{
			Timeout:   timeout,
			Transport: transport,
		},
		baseURL: fmt.Sprintf("%s://%s/api/v2.0", scheme, hostPort),
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

	return &SystemInfo{
		Hostname:      strings.TrimSpace(response.Hostname),
		Version:       strings.TrimSpace(response.Version),
		Build:         build,
		UptimeSeconds: response.UptimeSeconds,
		Healthy:       true,
		MachineID:     machineID,
	}, nil
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
			ID:         diskID,
			Name:       strings.TrimSpace(item.Name),
			Pool:       strings.TrimSpace(item.Pool),
			Status:     strings.TrimSpace(item.Status),
			Model:      strings.TrimSpace(item.Model),
			Serial:     strings.TrimSpace(item.Serial),
			SizeBytes:  item.Size,
			Transport:  strings.ToLower(strings.TrimSpace(item.Bus)),
			Rotational: rotational,
		})
	}

	return disks, nil
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
	zfsSnapshots, _ := c.GetZFSSnapshots(ctx)
	replicationTasks, _ := c.GetReplicationTasks(ctx)

	return &FixtureSnapshot{
		CollectedAt:      time.Now().UTC(),
		System:           *system,
		Pools:            pools,
		Datasets:         datasets,
		Disks:            disks,
		Alerts:           alerts,
		ZFSSnapshots:     zfsSnapshots,
		ReplicationTasks: replicationTasks,
	}, nil
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
