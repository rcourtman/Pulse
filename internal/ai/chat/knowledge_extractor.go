package chat

import (
	"encoding/json"
	"fmt"
	"strings"
)

// FactEntry is the output of ExtractFacts — ready to feed into KnowledgeAccumulator.AddFact.
type FactEntry struct {
	Category FactCategory
	Key      string
	Value    string
}

// ExtractFacts deterministically extracts knowledge facts from a tool result.
// No LLM calls. Parses the JSON text from FormatToolResult() output.
// Returns empty slice on parse errors or unrecognized tools — never panics.
//
// Tool results use NewJSONResult (direct struct marshaling), NOT ToolResponse wrapper.
// So the resultText is the JSON of the response struct directly (e.g. ResourceResponse).
func ExtractFacts(toolName string, toolInput map[string]interface{}, resultText string) []FactEntry {
	switch toolName {
	case "pulse_query":
		return extractQueryFacts(toolInput, resultText)
	case "pulse_storage":
		return extractStorageFacts(toolInput, resultText)
	case "pulse_discovery":
		return extractDiscoveryFacts(toolInput, resultText)
	case "pulse_read", "pulse_run_command":
		return extractExecFacts(toolInput, resultText)
	case "pulse_metrics":
		return extractMetricsFacts(toolInput, resultText)
	case "patrol_report_finding":
		return extractFindingFacts(toolInput, resultText)
	default:
		return nil
	}
}

// --- pulse_query ---

func extractQueryFacts(input map[string]interface{}, resultText string) []FactEntry {
	action := strFromMap(input, "action")
	if action == "" {
		action = strFromMap(input, "type")
	}

	switch action {
	case "get":
		return extractQueryGetFacts(input, resultText)
	case "search":
		return extractQuerySearchFacts(input, resultText)
	default:
		return nil
	}
}

func extractQueryGetFacts(input map[string]interface{}, resultText string) []FactEntry {
	// Tool results are direct JSON (NewJSONResult), no ToolResponse wrapper.
	// ResourceResponse has nested CPU/Memory structs.
	var resource struct {
		Type   string `json:"type"`
		Name   string `json:"name"`
		Status string `json:"status"`
		Node   string `json:"node"`
		ID     string `json:"id"`
		VMID   int    `json:"vmid"`
		Host   string `json:"host"`
		CPU    struct {
			Percent float64 `json:"percent"`
		} `json:"cpu"`
		Memory struct {
			Percent float64 `json:"percent"`
		} `json:"memory"`
		// Error field for not-found responses
		Error string `json:"error"`
	}
	if err := json.Unmarshal([]byte(resultText), &resource); err != nil {
		return nil
	}

	// Skip error/not-found responses
	if resource.Error != "" {
		return nil
	}
	if resource.Name == "" && resource.ID == "" {
		return nil
	}

	resType := resource.Type
	if resType == "" {
		resType = strFromMap(input, "resource_type")
	}
	node := resource.Node
	if node == "" {
		node = resource.Host
	}
	id := resource.ID
	if id == "" && resource.VMID > 0 {
		id = fmt.Sprintf("%d", resource.VMID)
	}
	if id == "" {
		id = resource.Name
	}

	key := fmt.Sprintf("%s:%s:%s:status", resType, node, id)

	var parts []string
	if resource.Status != "" {
		parts = append(parts, resource.Status)
	}
	if resource.Name != "" {
		parts = append(parts, resource.Name)
	}
	if resource.CPU.Percent > 0 {
		parts = append(parts, fmt.Sprintf("CPU=%.1f%%", resource.CPU.Percent))
	}
	if resource.Memory.Percent > 0 {
		parts = append(parts, fmt.Sprintf("Mem=%.1f%%", resource.Memory.Percent))
	}

	if len(parts) == 0 {
		return nil
	}

	return []FactEntry{{
		Category: FactCategoryResource,
		Key:      key,
		Value:    strings.Join(parts, ", "),
	}}
}

func extractQuerySearchFacts(input map[string]interface{}, resultText string) []FactEntry {
	query := strFromMap(input, "query")
	if query == "" {
		query = strFromMap(input, "search")
	}

	// ResourceSearchResponse — direct JSON, no wrapper
	var resp struct {
		Matches []struct {
			Name   string `json:"name"`
			Status string `json:"status"`
			Type   string `json:"type"`
		} `json:"matches"`
		Total int `json:"total"`
	}
	if err := json.Unmarshal([]byte(resultText), &resp); err != nil {
		return nil
	}

	total := resp.Total
	if total == 0 && len(resp.Matches) == 0 {
		return nil
	}

	// Summarize first 5 matches
	var summaryParts []string
	limit := 5
	if len(resp.Matches) < limit {
		limit = len(resp.Matches)
	}
	for _, m := range resp.Matches[:limit] {
		entry := m.Name
		if m.Status != "" {
			entry += " (" + m.Status + ")"
		}
		summaryParts = append(summaryParts, entry)
	}

	value := fmt.Sprintf("%d results: %s", total, strings.Join(summaryParts, ", "))

	return []FactEntry{{
		Category: FactCategoryResource,
		Key:      fmt.Sprintf("search:%s:summary", query),
		Value:    truncateValue(value),
	}}
}

// --- pulse_storage ---

func extractStorageFacts(input map[string]interface{}, resultText string) []FactEntry {
	action := strFromMap(input, "action")
	if action == "" {
		action = strFromMap(input, "type")
	}

	switch action {
	case "pools":
		return extractStoragePoolFacts(resultText)
	case "backup_tasks":
		return extractBackupTaskFacts(resultText)
	default:
		return nil
	}
}

func extractStoragePoolFacts(resultText string) []FactEntry {
	// StorageResponse — direct JSON, no wrapper
	var resp struct {
		Pools []struct {
			Name         string   `json:"name"`
			Node         string   `json:"node"`
			Nodes        []string `json:"nodes"`
			Type         string   `json:"type"`
			Status       string   `json:"status"`
			Active       bool     `json:"active"`
			UsagePercent float64  `json:"usage_percent"`
			TotalGB      float64  `json:"total_gb"`
			UsedGB       float64  `json:"used_gb"`
		} `json:"pools"`
	}
	if err := json.Unmarshal([]byte(resultText), &resp); err != nil {
		return nil
	}

	var facts []FactEntry
	for _, pool := range resp.Pools {
		node := pool.Node
		if node == "" && len(pool.Nodes) > 0 {
			node = strings.Join(pool.Nodes, "+")
		}

		freeGB := pool.TotalGB - pool.UsedGB
		var parts []string
		if pool.Type != "" {
			parts = append(parts, pool.Type)
		}
		if pool.Status != "" {
			parts = append(parts, pool.Status)
		}
		if pool.Active {
			parts = append(parts, fmt.Sprintf("active on %s", node))
		}
		parts = append(parts, fmt.Sprintf("%.1f%% used", pool.UsagePercent))
		if freeGB > 0 {
			parts = append(parts, fmt.Sprintf("%.0fGB free", freeGB))
		}

		facts = append(facts, FactEntry{
			Category: FactCategoryStorage,
			Key:      fmt.Sprintf("storage:%s:%s", node, pool.Name),
			Value:    truncateValue(strings.Join(parts, ", ")),
		})
	}
	return facts
}

func extractBackupTaskFacts(resultText string) []FactEntry {
	// BackupTasksListResponse — direct JSON, no wrapper
	var resp struct {
		Tasks []struct {
			VMID      string `json:"vmid"`
			Node      string `json:"node"`
			Status    string `json:"status"`
			StartTime string `json:"start_time"`
			Error     string `json:"error"`
		} `json:"tasks"`
	}
	if err := json.Unmarshal([]byte(resultText), &resp); err != nil {
		return nil
	}

	var facts []FactEntry
	for _, task := range resp.Tasks {
		// Only record failures
		if task.Status == "ok" || task.Status == "success" || task.Status == "" {
			continue
		}
		var parts []string
		parts = append(parts, task.Status)
		if task.StartTime != "" {
			parts = append(parts, "at "+task.StartTime)
		}
		if task.Error != "" {
			parts = append(parts, "error="+task.Error)
		}
		facts = append(facts, FactEntry{
			Category: FactCategoryStorage,
			Key:      fmt.Sprintf("backup:%s:%s", task.VMID, task.Node),
			Value:    truncateValue(strings.Join(parts, ", ")),
		})
	}
	return facts
}

// --- pulse_discovery ---

func extractDiscoveryFacts(input map[string]interface{}, resultText string) []FactEntry {
	// ResourceDiscoveryInfo — direct JSON, no wrapper
	var disc struct {
		ServiceType string `json:"service_type"`
		Hostname    string `json:"hostname"`
		HostID      string `json:"host_id"`
		ResourceID  string `json:"resource_id"`
		Ports       []struct {
			Port int `json:"port"`
		} `json:"ports"`
	}
	if err := json.Unmarshal([]byte(resultText), &disc); err != nil {
		return nil
	}

	host := disc.HostID
	if host == "" {
		host = strFromMap(input, "host")
	}
	id := disc.ResourceID
	if id == "" {
		id = strFromMap(input, "resource_id")
	}

	var parts []string
	if disc.ServiceType != "" {
		parts = append(parts, "service="+disc.ServiceType)
	}
	if disc.Hostname != "" {
		parts = append(parts, "hostname="+disc.Hostname)
	}
	if len(disc.Ports) > 0 {
		var portStrs []string
		for _, p := range disc.Ports {
			portStrs = append(portStrs, fmt.Sprintf("%d", p.Port))
		}
		parts = append(parts, "ports=["+strings.Join(portStrs, ",")+"]")
	}

	if len(parts) == 0 {
		return nil
	}

	return []FactEntry{{
		Category: FactCategoryDiscovery,
		Key:      fmt.Sprintf("discovery:%s:%s", host, id),
		Value:    truncateValue(strings.Join(parts, ", ")),
	}}
}

// --- pulse_read / pulse_run_command ---

func extractExecFacts(input map[string]interface{}, resultText string) []FactEntry {
	host := strFromMap(input, "target_host")
	if host == "" {
		host = strFromMap(input, "host")
	}
	cmd := strFromMap(input, "command")
	if cmd == "" {
		// For pulse_read file/tail/find actions, use action+path to distinguish
		// different file reads on the same host.
		action := strFromMap(input, "action")
		path := strFromMap(input, "path")
		if action != "" && path != "" {
			cmd = action + ":" + path
		} else if action != "" {
			cmd = action
		}
	}
	if cmd == "" {
		return nil
	}

	// Use first 60 chars of command as key prefix (longer to accommodate path)
	cmdPrefix := cmd
	if len(cmdPrefix) > 60 {
		cmdPrefix = cmdPrefix[:60]
	}

	// Try to parse as CommandResponse (direct JSON, no wrapper)
	var cmdResp struct {
		Success  bool   `json:"success"`
		ExitCode int    `json:"exit_code"`
		Output   string `json:"output"`
		Stdout   string `json:"stdout"`
		Error    string `json:"error"`
	}

	var value string
	if err := json.Unmarshal([]byte(resultText), &cmdResp); err == nil && (cmdResp.Output != "" || cmdResp.Stdout != "" || cmdResp.Error != "") {
		output := cmdResp.Output
		if output == "" {
			output = cmdResp.Stdout
		}
		if output == "" {
			output = cmdResp.Error
		}
		// Take first 2 lines
		lines := strings.SplitN(output, "\n", 3)
		summary := strings.Join(lines[:min(2, len(lines))], "; ")
		value = fmt.Sprintf("exit=%d, %s", cmdResp.ExitCode, summary)
	} else {
		// Fallback: use first 2 lines of raw result text
		lines := strings.SplitN(resultText, "\n", 3)
		summary := strings.Join(lines[:min(2, len(lines))], "; ")
		value = summary
	}

	return []FactEntry{{
		Category: FactCategoryExec,
		Key:      fmt.Sprintf("exec:%s:%s", host, cmdPrefix),
		Value:    truncateValue(value),
	}}
}

// --- pulse_metrics ---

func extractMetricsFacts(input map[string]interface{}, resultText string) []FactEntry {
	action := strFromMap(input, "action")
	if action == "" {
		action = strFromMap(input, "type")
	}

	if action != "performance" {
		return nil
	}

	resourceID := strFromMap(input, "resource_id")
	if resourceID == "" {
		return nil
	}

	// MetricsResponse — direct JSON, no wrapper.
	// Summary is map[string]ResourceMetricsSummary keyed by resource ID.
	var resp struct {
		Summary map[string]struct {
			AvgCPU    float64 `json:"avg_cpu"`
			MaxCPU    float64 `json:"max_cpu"`
			AvgMemory float64 `json:"avg_memory"`
			MaxMemory float64 `json:"max_memory"`
			Trend     string  `json:"trend"`
		} `json:"summary"`
	}
	if err := json.Unmarshal([]byte(resultText), &resp); err != nil {
		return nil
	}

	// Look up the summary for this resource (or take the first entry)
	var avgCPU, maxCPU float64
	var trend string
	if s, ok := resp.Summary[resourceID]; ok {
		avgCPU = s.AvgCPU
		maxCPU = s.MaxCPU
		trend = s.Trend
	} else {
		// Take first entry if resource ID doesn't match exactly
		for _, s := range resp.Summary {
			avgCPU = s.AvgCPU
			maxCPU = s.MaxCPU
			trend = s.Trend
			break
		}
	}

	var parts []string
	if avgCPU > 0 {
		parts = append(parts, fmt.Sprintf("avg_cpu=%.1f%%", avgCPU))
	}
	if maxCPU > 0 {
		parts = append(parts, fmt.Sprintf("max=%.1f%%", maxCPU))
	}
	if trend != "" {
		parts = append(parts, "trend="+trend)
	}

	if len(parts) == 0 {
		return nil
	}

	return []FactEntry{{
		Category: FactCategoryMetrics,
		Key:      fmt.Sprintf("metrics:%s", resourceID),
		Value:    truncateValue(strings.Join(parts, ", ")),
	}}
}

// --- patrol_report_finding ---

func extractFindingFacts(input map[string]interface{}, resultText string) []FactEntry {
	key := strFromMap(input, "key")
	if key == "" {
		key = strFromMap(input, "finding_key")
	}
	severity := strFromMap(input, "severity")
	title := strFromMap(input, "title")
	resourceID := strFromMap(input, "resource_id")

	if key == "" || title == "" {
		return nil
	}

	var parts []string
	if severity != "" {
		parts = append(parts, severity)
	}
	parts = append(parts, title)
	if resourceID != "" {
		parts = append(parts, "on "+resourceID)
	}

	return []FactEntry{{
		Category: FactCategoryFinding,
		Key:      fmt.Sprintf("finding:%s", key),
		Value:    truncateValue(strings.Join(parts, ": ")),
	}}
}

// PredictFactKeys returns the KA fact keys that this tool call would produce,
// based solely on the tool input (without needing the result).
// Used by the gate to check if we already have facts for this call.
// Returns nil if the key can't be predicted from input alone.
func PredictFactKeys(toolName string, toolInput map[string]interface{}) []string {
	switch toolName {
	case "pulse_discovery":
		host := strFromMap(toolInput, "host_id")
		if host == "" {
			host = strFromMap(toolInput, "host")
		}
		id := strFromMap(toolInput, "resource_id")
		if host != "" && id != "" {
			return []string{fmt.Sprintf("discovery:%s:%s", host, id)}
		}
	case "pulse_read", "pulse_run_command":
		host := strFromMap(toolInput, "target_host")
		if host == "" {
			host = strFromMap(toolInput, "host")
		}
		cmd := strFromMap(toolInput, "command")
		if cmd == "" {
			// For file/tail/find actions, include path to distinguish different file reads
			action := strFromMap(toolInput, "action")
			path := strFromMap(toolInput, "path")
			if action != "" && path != "" {
				cmd = action + ":" + path
			} else if action != "" {
				cmd = action
			}
		}
		if host != "" && cmd != "" {
			cmdPrefix := cmd
			if len(cmdPrefix) > 60 {
				cmdPrefix = cmdPrefix[:60]
			}
			return []string{fmt.Sprintf("exec:%s:%s", host, cmdPrefix)}
		}
	case "pulse_metrics":
		action := strFromMap(toolInput, "action")
		if action == "" {
			action = strFromMap(toolInput, "type")
		}
		resourceID := strFromMap(toolInput, "resource_id")
		if action == "performance" && resourceID != "" {
			return []string{fmt.Sprintf("metrics:%s", resourceID)}
		}
	}
	// pulse_query get/search and pulse_storage: keys depend on result data,
	// can't predict from input alone. Return nil — these calls won't be gated.
	return nil
}

// --- helpers ---

func strFromMap(m map[string]interface{}, key string) string {
	if m == nil {
		return ""
	}
	v, ok := m[key]
	if !ok {
		return ""
	}
	s, ok := v.(string)
	if !ok {
		return ""
	}
	return s
}

func truncateValue(s string) string {
	if len(s) > maxValueLen {
		return s[:maxValueLen]
	}
	return s
}
