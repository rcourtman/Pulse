package truenas

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net/http"
	"sort"
	"strings"
	"time"
)

// CORE's RRD graphs are device-scoped. Ask the reporting catalogue for its
// identifiers, not disk serials or configured (possibly renamed) interfaces.
// Older REST bridges without the catalogue retain the unscoped compatibility
// request. Authentication, transport and malformed replies are never retried.
func (c *Client) legacyReportingGraphs(ctx context.Context) ([]map[string]any, error) {
	var catalogue []struct {
		Name        string   `json:"name"`
		Identifiers []string `json:"identifiers"`
	}
	if err := c.getJSON(ctx, http.MethodGet, "/reporting/graphs", &catalogue); err != nil {
		if isAPIStatus(err, http.StatusNotFound) || isAPIStatus(err, http.StatusMethodNotAllowed) {
			return legacyRESTReportingGraphs(), nil
		}
		return nil, err
	}
	graphs := []map[string]any{
		reportingGraph("cpu", ""), reportingGraph("memory", ""),
		reportingGraph("arcsize", ""), reportingGraph("cputemp", ""),
	}
	seen := make(map[string]bool)
	for _, graph := range catalogue {
		name := strings.ToLower(strings.TrimSpace(graph.Name))
		if name != "disk" && name != "interface" {
			continue
		}
		for _, identifier := range dedupeStrings(graph.Identifiers) {
			identifier = strings.TrimSpace(identifier)
			key := name + "\x00" + identifier
			if identifier == "" || seen[key] {
				continue
			}
			seen[key] = true
			// Refuse an oversized catalogue, rather than silently publishing a
			// truncated host total or allowing an unbounded split retry loop.
			if len(graphs) >= 256 {
				return nil, fmt.Errorf("truenas reporting catalogue exceeds 256 graphs")
			}
			graphs = append(graphs, reportingGraph(name, identifier))
		}
	}
	return graphs, nil
}

func isAPIStatus(err error, status int) bool {
	var apiErr *APIError
	return errors.As(err, &apiErr) && apiErr.StatusCode == status
}

func finiteReportingValues(values map[string]float64) map[string]float64 {
	for key, value := range values {
		if math.IsNaN(value) || math.IsInf(value, 0) {
			delete(values, key)
		}
	}
	return values
}

// A native row has exactly one value per legend; its timestamp is external.
// An explicit-timestamp row has one extra element. Never guess an epoch from a
// value-only row with missing/invalid timing or accept a misaligned short row.
func reportingRow(response trueNASReportingGetDataResponse, index int) (time.Time, map[string]float64, bool) {
	raw := response.Data[index]
	if row, ok := raw.([]any); ok {
		if len(response.Legend) == 0 {
			return time.Time{}, nil, false
		}
		if len(row) == len(response.Legend) {
			if response.Start <= 0 || response.End < response.Start || response.Step <= 0 ||
				int64(index) > (response.End-response.Start)/response.Step {
				return time.Time{}, nil, false
			}
			timestamp := time.Unix(response.Start+int64(index)*response.Step, 0).UTC()
			values := finiteReportingValues(extractReportingLegendFloatValues(row, response.Legend))
			return timestamp, values, len(values) > 0
		}
		if len(row) != len(response.Legend)+1 {
			return time.Time{}, nil, false
		}
	}
	timestamp, values, ok := parseReportingSeriesValues(raw, response.Legend)
	values = finiteReportingValues(values)
	return timestamp, values, ok && timestamp.Unix() > 0 && len(values) > 0
}

// Keep native History intact, but do not label an old RRD point as a current
// reading. Two returned steps allow the usual unfinished/null RRD bucket;
// older points stay available only through History. Preserve row indices.
func liveReportingResponses(responses []trueNASReportingGetDataResponse, end int64) []trueNASReportingGetDataResponse {
	live := append([]trueNASReportingGetDataResponse(nil), responses...)
	for i, response := range live {
		if response.Step <= 0 {
			continue // timestamped legacy/modern compatibility payload
		}
		live[i].Data = append([]any(nil), response.Data...)
		for index := range response.Data {
			timestamp, _, ok := reportingRow(response, index)
			gap := end - timestamp.Unix()
			if !ok || timestamp.Unix() > end || (gap > response.Step && gap-response.Step > response.Step) {
				live[i].Data[index] = nil
			}
		}
	}
	return live
}

func latestReportingValues(response trueNASReportingGetDataResponse) map[string]float64 {
	var latest time.Time
	var values map[string]float64
	for index := range response.Data {
		timestamp, row, ok := reportingRow(response, index)
		if ok && (values == nil || timestamp.After(latest)) {
			latest, values = timestamp, row
		}
	}
	return values
}

var coreCPUStates = []string{"interrupt", "system", "user", "nice", "idle"}
var coreMemoryClasses = []string{"memory-active_value", "memory-inactive_value", "memory-wired_value", "memory-laundry_value", "memory-free_value"}

func hasReportingLegends(legends, required []string) bool {
	present := make(map[string]bool, len(legends))
	for _, legend := range legends {
		present[normalizeTemperatureLegendLabel(legend)] = true
	}
	for _, legend := range required {
		if !present[normalizeTemperatureLegendLabel(legend)] {
			return false
		}
	}
	return true
}

// RRD CPU state rates are not necessarily percentages (the supplied states sum
// above 100). Normalize the complete state vector; an all-zero bucket is not
// 100% busy. Explicit usage=0 and a measured all-idle vector remain real zeros.
func reportingCPUPercent(values map[string]float64, legends []string) (float64, bool) {
	if !hasReportingLegends(legends, coreCPUStates) {
		return parseSystemCPUPercent(values)
	}
	var total, idle float64
	for _, state := range coreCPUStates {
		value, ok := pickReportingValue(values, state)
		if !ok || value < 0 {
			return 0, false
		}
		total += value
		if state == "idle" {
			idle = value
		}
	}
	if total <= 0 || math.IsInf(total, 0) {
		return 0, false
	}
	return (total - idle) / total * 100, true
}

// Five zero memory classes cannot describe physical RAM. Keep this native RRD
// empty-bucket sentinel distinct from a real zero-free reading with used pages.
func reportingMemoryRowValid(values map[string]float64, legends []string) bool {
	if !hasReportingLegends(legends, coreMemoryClasses) {
		return true
	}
	var total float64
	for _, class := range coreMemoryClasses {
		value, ok := pickReportingValue(values, class)
		if !ok || value < 0 {
			return false
		}
		total += value
	}
	return total > 0 && !math.IsInf(total, 0)
}

// Sum only simultaneous observations of every returned device. Null/missing
// members are not zero, duplicate device replies are not additional traffic,
// and values are already rates: step is for time, not another division.
func sumReportingDevices(devices map[string][]TimeSeriesPoint) []TimeSeriesPoint {
	if len(devices) == 0 {
		return nil
	}
	var totals map[int64]float64
	for _, series := range devices {
		points := make(map[int64]float64, len(series))
		for _, point := range series {
			points[point.Timestamp.Unix()] = point.Value
		}
		if totals == nil {
			totals = points
			continue
		}
		for timestamp, total := range totals {
			value, ok := points[timestamp]
			if !ok {
				delete(totals, timestamp)
			} else {
				totals[timestamp] = total + value
			}
		}
	}
	timestamps := make([]int64, 0, len(totals))
	for timestamp := range totals {
		timestamps = append(timestamps, timestamp)
	}
	sort.Slice(timestamps, func(i, j int) bool { return timestamps[i] < timestamps[j] })
	points := make([]TimeSeriesPoint, 0, len(timestamps))
	for _, timestamp := range timestamps {
		if !math.IsInf(totals[timestamp], 0) {
			points = appendTimeSeriesPoint(points, time.Unix(timestamp, 0).UTC(), totals[timestamp])
		}
	}
	return points
}

// Bind device responses to the requested catalogue. A failed/omitted device
// graph remains an absent member, not a smaller apparently complete host sum.
func bindLegacyReportingResponses(graphs []map[string]any, responses []trueNASReportingGetDataResponse) []trueNASReportingGetDataResponse {
	byKey := make(map[string]trueNASReportingGetDataResponse)
	for _, response := range responses {
		name := strings.ToLower(strings.TrimSpace(response.Name))
		identifier := readStringAny(map[string]any{"identifier": response.Identifier}, "identifier")
		key := name + "\x00" + identifier
		if _, exists := byKey[key]; !exists {
			byKey[key] = response
		}
	}
	bound := make([]trueNASReportingGetDataResponse, 0, len(graphs))
	for _, graph := range graphs {
		name := readStringAny(graph, "name")
		identifier := readStringAny(graph, "identifier")
		response, ok := byKey[name+"\x00"+identifier]
		if !ok {
			response = trueNASReportingGetDataResponse{Name: name, Identifier: identifier}
		}
		bound = append(bound, response)
	}
	return bound
}

func systemTemperatureHistory(sensors map[string][]TimeSeriesPoint) []TimeSeriesPoint {
	at := make(map[int64]map[string]float64)
	for key, series := range sensors {
		for _, point := range series {
			timestamp := point.Timestamp.Unix()
			if at[timestamp] == nil {
				at[timestamp] = make(map[string]float64)
			}
			at[timestamp][key] = point.Value
		}
	}
	timestamps := make([]int64, 0, len(at))
	for timestamp := range at {
		timestamps = append(timestamps, timestamp)
	}
	sort.Slice(timestamps, func(i, j int) bool { return timestamps[i] < timestamps[j] })
	var points []TimeSeriesPoint
	for _, timestamp := range timestamps {
		if value := maxTrueNASSystemTemperature(SystemInfo{TemperatureCelsius: at[timestamp]}); value != nil {
			points = appendTimeSeriesPoint(points, time.Unix(timestamp, 0).UTC(), *value)
		}
	}
	return points
}
