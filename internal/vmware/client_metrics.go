package vmware

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"strings"
)

type perfCounterCatalog struct {
	idsByKey map[string]int
}

type perfCounterDefinition struct {
	logical string
	group   string
	name    string
	rollup  string
}

type viJSONPerfCounterInfo struct {
	Key        int                      `json:"key"`
	GroupInfo  viJSONElementDescription `json:"groupInfo"`
	NameInfo   viJSONElementDescription `json:"nameInfo"`
	RollupType string                   `json:"rollupType"`
}

type viJSONElementDescription struct {
	Key string `json:"key"`
}

type viJSONPerfProviderSummary struct {
	CurrentSupported bool `json:"currentSupported"`
	RefreshRate      int  `json:"refreshRate"`
}

type viJSONPerfMetricID struct {
	CounterID int    `json:"counterId"`
	Instance  string `json:"instance"`
}

type viJSONPerfEntityMetric struct {
	Entity     viJSONReference          `json:"entity"`
	SampleInfo []viJSONPerfSampleInfo   `json:"sampleInfo"`
	Value      []viJSONPerfMetricSeries `json:"value"`
}

type viJSONPerfSampleInfo struct {
	Interval int `json:"interval"`
}

type viJSONPerfMetricSeries struct {
	ID    viJSONPerfMetricID `json:"id"`
	Value []int64            `json:"value"`
}

type perfMetricAccumulator struct {
	sum   float64
	count int
}

const (
	vmwarePerfLogicalCPUUsage        = "cpu_usage"
	vmwarePerfLogicalMemoryUsage     = "memory_usage"
	vmwarePerfLogicalHostMemoryTotal = "host_memory_total"
	vmwarePerfLogicalNetIn           = "net_in"
	vmwarePerfLogicalNetOut          = "net_out"
	vmwarePerfLogicalDiskRead        = "disk_read"
	vmwarePerfLogicalDiskWrite       = "disk_write"
)

var vmwareHostPerfCounters = []perfCounterDefinition{
	{logical: vmwarePerfLogicalCPUUsage, group: "cpu", name: "usage", rollup: "average"},
	{logical: vmwarePerfLogicalMemoryUsage, group: "mem", name: "usage", rollup: "average"},
	{logical: vmwarePerfLogicalHostMemoryTotal, group: "mem", name: "totalCapacity", rollup: "average"},
	{logical: vmwarePerfLogicalNetIn, group: "net", name: "bytesRx", rollup: "average"},
	{logical: vmwarePerfLogicalNetOut, group: "net", name: "bytesTx", rollup: "average"},
	{logical: vmwarePerfLogicalDiskRead, group: "disk", name: "read", rollup: "average"},
	{logical: vmwarePerfLogicalDiskWrite, group: "disk", name: "write", rollup: "average"},
}

var vmwareVMPerfCounters = []perfCounterDefinition{
	{logical: vmwarePerfLogicalCPUUsage, group: "cpu", name: "usage", rollup: "average"},
	{logical: vmwarePerfLogicalMemoryUsage, group: "mem", name: "usage", rollup: "average"},
	{logical: vmwarePerfLogicalNetIn, group: "net", name: "bytesRx", rollup: "average"},
	{logical: vmwarePerfLogicalNetOut, group: "net", name: "bytesTx", rollup: "average"},
	{logical: vmwarePerfLogicalDiskRead, group: "disk", name: "read", rollup: "average"},
	{logical: vmwarePerfLogicalDiskWrite, group: "disk", name: "write", rollup: "average"},
}

func (c *Client) loadPerfCounterCatalog(ctx context.Context, release, sessionID, perfManagerMoID string) (perfCounterCatalog, error) {
	perfManagerMoID = strings.TrimSpace(perfManagerMoID)
	if perfManagerMoID == "" {
		return perfCounterCatalog{}, &ConnectionError{Category: "endpoint", Message: "VMware VI JSON API service-instance response did not include a performance manager reference"}
	}

	var counters []viJSONPerfCounterInfo
	path := fmt.Sprintf("/sdk/vim25/%s/PerformanceManager/%s/perfCounter", release, perfManagerMoID)
	if err := c.getVIJSONJSON(ctx, sessionID, path, "vmware performance counter catalog", &counters); err != nil {
		return perfCounterCatalog{}, err
	}

	catalog := perfCounterCatalog{idsByKey: make(map[string]int, len(counters))}
	for _, counter := range counters {
		if counter.Key <= 0 {
			continue
		}
		key := perfCounterCatalogKey(counter.GroupInfo.Key, counter.NameInfo.Key, counter.RollupType)
		if key == "" {
			continue
		}
		catalog.idsByKey[key] = counter.Key
	}
	return catalog, nil
}

func (c *Client) collectHostPerformanceMetrics(
	ctx context.Context,
	release string,
	sessionID string,
	perfManagerMoID string,
	perfCounters perfCounterCatalog,
	host InventoryHost,
) (*InventoryMetrics, error) {
	return c.collectEntityPerformanceMetrics(
		ctx,
		release,
		sessionID,
		perfManagerMoID,
		perfCounters,
		"HostSystem",
		host.Host,
		vmwareHostPerfCounters,
		0,
	)
}

func (c *Client) collectVMPerformanceMetrics(
	ctx context.Context,
	release string,
	sessionID string,
	perfManagerMoID string,
	perfCounters perfCounterCatalog,
	vm InventoryVM,
) (*InventoryMetrics, error) {
	var configuredMemoryBytes int64
	if vm.MemorySizeMiB > 0 {
		configuredMemoryBytes = vm.MemorySizeMiB * 1024 * 1024
	}
	return c.collectEntityPerformanceMetrics(
		ctx,
		release,
		sessionID,
		perfManagerMoID,
		perfCounters,
		"VirtualMachine",
		vm.VM,
		vmwareVMPerfCounters,
		configuredMemoryBytes,
	)
}

func (c *Client) collectEntityPerformanceMetrics(
	ctx context.Context,
	release string,
	sessionID string,
	perfManagerMoID string,
	perfCounters perfCounterCatalog,
	entityType string,
	entityMoID string,
	definitions []perfCounterDefinition,
	fallbackMemoryTotalBytes int64,
) (*InventoryMetrics, error) {
	entityMoID = strings.TrimSpace(entityMoID)
	if entityMoID == "" {
		return nil, nil
	}

	summary, err := c.queryPerfProviderSummary(ctx, release, sessionID, perfManagerMoID, entityType, entityMoID)
	if err != nil {
		return nil, err
	}
	if !summary.CurrentSupported || summary.RefreshRate <= 0 {
		return nil, nil
	}

	available, err := c.queryAvailablePerfMetrics(ctx, release, sessionID, perfManagerMoID, entityType, entityMoID, summary.RefreshRate)
	if err != nil {
		return nil, err
	}

	selectedMetricIDs, defsByCounterID := perfMetricIDsForEntity(perfCounters, available, definitions)
	if len(selectedMetricIDs) == 0 {
		return nil, nil
	}

	result, err := c.queryPerfMetrics(ctx, release, sessionID, perfManagerMoID, entityType, entityMoID, summary.RefreshRate, selectedMetricIDs)
	if err != nil {
		return nil, err
	}

	metrics := inventoryMetricsFromPerf(result, defsByCounterID, fallbackMemoryTotalBytes)
	if !hasInventoryMetrics(metrics) {
		return nil, nil
	}
	return metrics, nil
}

func (c *Client) queryPerfProviderSummary(
	ctx context.Context,
	release string,
	sessionID string,
	perfManagerMoID string,
	entityType string,
	entityMoID string,
) (viJSONPerfProviderSummary, error) {
	var summary viJSONPerfProviderSummary
	path := fmt.Sprintf("/sdk/vim25/%s/PerformanceManager/%s/QueryPerfProviderSummary", release, perfManagerMoID)
	body := map[string]any{
		"entity": map[string]string{
			"type":  strings.TrimSpace(entityType),
			"value": strings.TrimSpace(entityMoID),
		},
	}
	if err := c.postVIJSONJSON(ctx, sessionID, path, "vmware performance provider summary", body, &summary); err != nil {
		return viJSONPerfProviderSummary{}, err
	}
	return summary, nil
}

func (c *Client) queryAvailablePerfMetrics(
	ctx context.Context,
	release string,
	sessionID string,
	perfManagerMoID string,
	entityType string,
	entityMoID string,
	intervalID int,
) ([]viJSONPerfMetricID, error) {
	path := fmt.Sprintf("/sdk/vim25/%s/PerformanceManager/%s/QueryAvailablePerfMetric", release, perfManagerMoID)
	body := map[string]any{
		"entity": map[string]string{
			"type":  strings.TrimSpace(entityType),
			"value": strings.TrimSpace(entityMoID),
		},
		"intervalId": intervalID,
	}
	var available []viJSONPerfMetricID
	if err := c.postVIJSONJSON(ctx, sessionID, path, "vmware available performance metrics", body, &available); err != nil {
		return nil, err
	}
	return available, nil
}

func (c *Client) queryPerfMetrics(
	ctx context.Context,
	release string,
	sessionID string,
	perfManagerMoID string,
	entityType string,
	entityMoID string,
	intervalID int,
	metricIDs []viJSONPerfMetricID,
) ([]viJSONPerfEntityMetric, error) {
	path := fmt.Sprintf("/sdk/vim25/%s/PerformanceManager/%s/QueryPerf", release, perfManagerMoID)
	body := map[string]any{
		"querySpec": []map[string]any{{
			"entity": map[string]string{
				"type":  strings.TrimSpace(entityType),
				"value": strings.TrimSpace(entityMoID),
			},
			"intervalId": intervalID,
			"maxSample":  1,
			"metricId":   metricIDs,
		}},
	}
	var result []viJSONPerfEntityMetric
	if err := c.postVIJSONJSON(ctx, sessionID, path, "vmware performance metrics", body, &result); err != nil {
		return nil, err
	}
	return result, nil
}

func (c *Client) postVIJSONJSON(ctx context.Context, sessionID, path, label string, body any, target any) error {
	payload, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshal %s request: %w", label, err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL.String()+path, strings.NewReader(string(payload)))
	if err != nil {
		return fmt.Errorf("build %s request: %w", label, err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("vmware-api-session-id", sessionID)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return classifyTransportError(label, err)
	}
	defer resp.Body.Close()
	responseBody, readErr := io.ReadAll(io.LimitReader(resp.Body, inventoryResponseLimitByte))
	if readErr != nil {
		return fmt.Errorf("read %s response: %w", label, readErr)
	}
	switch resp.StatusCode {
	case http.StatusOK:
	case http.StatusUnauthorized:
		return &ConnectionError{Category: "auth", Message: fmt.Sprintf("VMware authentication failed while reading %s", label)}
	case http.StatusForbidden:
		return &ConnectionError{Category: "permission", Message: fmt.Sprintf("VMware permissions are insufficient for %s", label)}
	case http.StatusNotFound:
		return &ConnectionError{Category: "not_found", Message: fmt.Sprintf("VMware %s endpoint is unavailable", label)}
	default:
		return &ConnectionError{Category: "endpoint", Message: fmt.Sprintf("VMware %s request failed with HTTP %d", label, resp.StatusCode)}
	}
	if err := json.Unmarshal(responseBody, target); err != nil {
		return &ConnectionError{Category: "endpoint", Message: fmt.Sprintf("VMware %s response was not valid JSON", label)}
	}
	return nil
}

func perfMetricIDsForEntity(
	catalog perfCounterCatalog,
	available []viJSONPerfMetricID,
	definitions []perfCounterDefinition,
) ([]viJSONPerfMetricID, map[int]string) {
	if len(definitions) == 0 || len(available) == 0 {
		return nil, nil
	}

	selected := make([]viJSONPerfMetricID, 0, len(definitions))
	defsByCounterID := make(map[int]string, len(definitions))
	for _, definition := range definitions {
		counterID, ok := catalog.idsByKey[perfCounterCatalogKey(definition.group, definition.name, definition.rollup)]
		if !ok {
			continue
		}
		candidates := filterAvailablePerfMetrics(available, counterID)
		if len(candidates) == 0 {
			continue
		}
		defsByCounterID[counterID] = definition.logical
		selected = append(selected, preferredPerfMetricInstances(candidates)...)
	}
	return selected, defsByCounterID
}

func filterAvailablePerfMetrics(available []viJSONPerfMetricID, counterID int) []viJSONPerfMetricID {
	filtered := make([]viJSONPerfMetricID, 0, len(available))
	for _, metric := range available {
		if metric.CounterID == counterID {
			filtered = append(filtered, metric)
		}
	}
	return filtered
}

func preferredPerfMetricInstances(candidates []viJSONPerfMetricID) []viJSONPerfMetricID {
	if len(candidates) == 0 {
		return nil
	}
	for _, candidate := range candidates {
		if strings.TrimSpace(candidate.Instance) == "" {
			return []viJSONPerfMetricID{candidate}
		}
	}
	out := make([]viJSONPerfMetricID, len(candidates))
	copy(out, candidates)
	return out
}

func inventoryMetricsFromPerf(
	result []viJSONPerfEntityMetric,
	defsByCounterID map[int]string,
	fallbackMemoryTotalBytes int64,
) *InventoryMetrics {
	if len(result) == 0 || len(defsByCounterID) == 0 {
		return nil
	}

	accumulators := make(map[string]perfMetricAccumulator)
	for _, entityMetric := range result {
		for _, series := range entityMetric.Value {
			logical, ok := defsByCounterID[series.ID.CounterID]
			if !ok {
				continue
			}
			value, ok := latestPerfMetricValue(series.Value)
			if !ok {
				continue
			}
			accumulator := accumulators[logical]
			accumulator.sum += float64(value)
			accumulator.count++
			accumulators[logical] = accumulator
		}
	}

	metrics := &InventoryMetrics{}
	if accumulator, ok := accumulators[vmwarePerfLogicalCPUUsage]; ok && accumulator.count > 0 {
		cpuPercent := clampVMwarePercent(accumulator.sum / float64(accumulator.count) / 100.0)
		metrics.CPUPercent = float64Ptr(cpuPercent)
	}
	if accumulator, ok := accumulators[vmwarePerfLogicalMemoryUsage]; ok && accumulator.count > 0 {
		memoryPercent := clampVMwarePercent(accumulator.sum / float64(accumulator.count) / 100.0)
		metrics.MemoryPercent = float64Ptr(memoryPercent)
	}

	memoryTotalBytes := fallbackMemoryTotalBytes
	if accumulator, ok := accumulators[vmwarePerfLogicalHostMemoryTotal]; ok && accumulator.count > 0 {
		memoryTotalBytes = int64(math.Round(accumulator.sum)) * 1024 * 1024
	}
	if memoryTotalBytes > 0 && metrics.MemoryPercent != nil {
		metrics.MemoryTotalBytes = int64Ptr(memoryTotalBytes)
		memoryUsedBytes := int64(math.Round((float64(memoryTotalBytes) * *metrics.MemoryPercent) / 100.0))
		metrics.MemoryUsedBytes = int64Ptr(memoryUsedBytes)
	}

	if accumulator, ok := accumulators[vmwarePerfLogicalNetIn]; ok && accumulator.count > 0 {
		metrics.NetInBytesPerSecond = float64Ptr(accumulator.sum * 1024.0)
	}
	if accumulator, ok := accumulators[vmwarePerfLogicalNetOut]; ok && accumulator.count > 0 {
		metrics.NetOutBytesPerSecond = float64Ptr(accumulator.sum * 1024.0)
	}
	if accumulator, ok := accumulators[vmwarePerfLogicalDiskRead]; ok && accumulator.count > 0 {
		metrics.DiskReadBytesPerSecond = float64Ptr(accumulator.sum * 1024.0)
	}
	if accumulator, ok := accumulators[vmwarePerfLogicalDiskWrite]; ok && accumulator.count > 0 {
		metrics.DiskWriteBytesPerSecond = float64Ptr(accumulator.sum * 1024.0)
	}

	if !hasInventoryMetrics(metrics) {
		return nil
	}
	return metrics
}

func latestPerfMetricValue(values []int64) (int64, bool) {
	if len(values) == 0 {
		return 0, false
	}
	value := values[len(values)-1]
	if value < 0 {
		return 0, false
	}
	return value, true
}

func perfCounterCatalogKey(group, name, rollup string) string {
	group = strings.TrimSpace(strings.ToLower(group))
	name = strings.TrimSpace(strings.ToLower(name))
	rollup = strings.TrimSpace(strings.ToLower(rollup))
	if group == "" || name == "" || rollup == "" {
		return ""
	}
	return group + "." + name + "." + rollup
}

func clampVMwarePercent(value float64) float64 {
	if value < 0 {
		return 0
	}
	if value > 100 {
		return 100
	}
	return value
}

func hasInventoryMetrics(metrics *InventoryMetrics) bool {
	if metrics == nil {
		return false
	}
	return metrics.CPUPercent != nil ||
		metrics.MemoryPercent != nil ||
		metrics.MemoryUsedBytes != nil ||
		metrics.MemoryTotalBytes != nil ||
		metrics.NetInBytesPerSecond != nil ||
		metrics.NetOutBytesPerSecond != nil ||
		metrics.DiskReadBytesPerSecond != nil ||
		metrics.DiskWriteBytesPerSecond != nil
}

func float64Ptr(value float64) *float64 {
	return &value
}

func int64Ptr(value int64) *int64 {
	return &value
}
