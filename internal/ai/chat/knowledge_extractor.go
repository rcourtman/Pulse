package chat

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/rs/zerolog/log"
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
	case "pulse_alerts":
		return extractAlertsFacts(toolInput, resultText)
	case "pulse_docker":
		return extractDockerFacts(toolInput, resultText)
	case "pulse_kubernetes":
		return extractKubernetesFacts(toolInput, resultText)
	case "pulse_pmg":
		return extractPMGFacts(toolInput, resultText)
	case "patrol_report_finding":
		return extractFindingFacts(toolInput, resultText)
	case "patrol_get_findings":
		return extractPatrolGetFindingsFacts(resultText)
	default:
		log.Debug().Str("tool", toolName).Msg("[KnowledgeExtractor] No extractor for tool")
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
	case "topology":
		return extractQueryTopologyFacts(resultText)
	case "health":
		return extractQueryHealthFacts(resultText)
	case "list":
		return extractQueryListFacts(resultText)
	case "config":
		return extractQueryConfigFacts(input, resultText)
	default:
		log.Debug().Str("tool", "pulse_query").Str("action", action).
			Msg("[KnowledgeExtractor] No extractor for action")
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

	// Return negative fact for error/not-found responses
	if resource.Error != "" {
		resourceID := strFromMap(input, "resource_id")
		if resourceID == "" {
			return nil
		}
		return []FactEntry{{
			Category: FactCategoryResource,
			Key:      fmt.Sprintf("query:get:%s:error", resourceID),
			Value:    fmt.Sprintf("not found: %s", resource.Error),
		}}
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
	// Prefer VMID (numeric) over composite ID (which may contain node prefix like "delly:minipc:100")
	id := ""
	if resource.VMID > 0 {
		id = fmt.Sprintf("%d", resource.VMID)
	}
	if id == "" {
		// Fall back to resource_id from tool input (user-provided, e.g. "100")
		id = strFromMap(input, "resource_id")
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

	value := strings.Join(parts, ", ")

	facts := []FactEntry{{
		Category: FactCategoryResource,
		Key:      key,
		Value:    value,
	}}

	// Secondary fact with predictable key for gate matching
	resourceID := strFromMap(input, "resource_id")
	if resourceID != "" {
		facts = append(facts, FactEntry{
			Category: FactCategoryResource,
			Key:      fmt.Sprintf("query:get:%s:cached", resourceID),
			Value:    truncateValue(value),
		})
	}

	return facts
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

func extractQueryTopologyFacts(resultText string) []FactEntry {
	// TopologyResponse — direct JSON. Has summary + proxmox.nodes array.
	// Real format: nodes are under "proxmox.nodes", LXC count is "total_lxc_containers".
	var resp struct {
		Summary struct {
			TotalNodes         int `json:"total_nodes"`
			TotalVMs           int `json:"total_vms"`
			RunningVMs         int `json:"running_vms"`
			TotalLXCContainers int `json:"total_lxc_containers"`
			RunningLXC         int `json:"running_lxc"`
			TotalDockerHost    int `json:"total_docker_hosts"`
		} `json:"summary"`
		Proxmox struct {
			Nodes []struct {
				Name   string `json:"name"`
				Status string `json:"status"`
			} `json:"nodes"`
		} `json:"proxmox"`
	}
	if err := json.Unmarshal([]byte(resultText), &resp); err != nil {
		return nil
	}

	s := resp.Summary

	// Build node list
	var nodeDescs []string
	for _, n := range resp.Proxmox.Nodes {
		status := n.Status
		if status == "" {
			status = "unknown"
		}
		nodeDescs = append(nodeDescs, fmt.Sprintf("%s=%s", n.Name, status))
	}

	var parts []string
	if s.TotalNodes > 0 || len(resp.Proxmox.Nodes) > 0 {
		nodeCount := s.TotalNodes
		if nodeCount == 0 {
			nodeCount = len(resp.Proxmox.Nodes)
		}
		nodeStr := fmt.Sprintf("%d nodes", nodeCount)
		if len(nodeDescs) > 0 {
			nodeStr += " (" + strings.Join(nodeDescs, ", ") + ")"
		}
		parts = append(parts, nodeStr)
	}
	if s.TotalVMs > 0 {
		parts = append(parts, fmt.Sprintf("%d VMs (%d running)", s.TotalVMs, s.RunningVMs))
	}
	if s.TotalLXCContainers > 0 {
		parts = append(parts, fmt.Sprintf("%d LXC (%d running)", s.TotalLXCContainers, s.RunningLXC))
	}
	if s.TotalDockerHost > 0 {
		parts = append(parts, fmt.Sprintf("%d docker host", s.TotalDockerHost))
	}

	if len(parts) == 0 {
		return nil
	}

	return []FactEntry{{
		Category: FactCategoryResource,
		Key:      "topology:summary",
		Value:    truncateValue(strings.Join(parts, ", ")),
	}}
}

func extractQueryHealthFacts(resultText string) []FactEntry {
	// ConnectionHealthResponse — direct JSON.
	// Real format uses "instance_id" as the identifier field.
	var resp struct {
		Connections []struct {
			InstanceID string `json:"instance_id"`
			Name       string `json:"name"`
			Instance   string `json:"instance"`
			Connected  bool   `json:"connected"`
			Status     string `json:"status"`
		} `json:"connections"`
	}
	if err := json.Unmarshal([]byte(resultText), &resp); err != nil {
		return nil
	}
	if len(resp.Connections) == 0 {
		return nil
	}

	total := len(resp.Connections)
	connected := 0
	var disconnected []string
	for _, c := range resp.Connections {
		if c.Connected {
			connected++
		} else {
			name := c.InstanceID
			if name == "" {
				name = c.Name
			}
			if name == "" {
				name = c.Instance
			}
			if name != "" {
				disconnected = append(disconnected, name)
			}
		}
	}

	value := fmt.Sprintf("%d/%d connected", connected, total)
	if len(disconnected) > 0 {
		value += ", disconnected: " + strings.Join(disconnected, ", ")
	}

	return []FactEntry{{
		Category: FactCategoryResource,
		Key:      "health:connections",
		Value:    truncateValue(value),
	}}
}

func extractQueryListFacts(resultText string) []FactEntry {
	var resp struct {
		Nodes []struct {
			Name   string `json:"name"`
			Status string `json:"status"`
		} `json:"nodes"`
		VMs []struct {
			Name   string `json:"name"`
			Status string `json:"status"`
		} `json:"vms"`
		Containers []struct {
			Name   string `json:"name"`
			Status string `json:"status"`
		} `json:"containers"`
		DockerHosts []struct {
			Hostname       string `json:"hostname"`
			DisplayName    string `json:"display_name"`
			ContainerCount int    `json:"container_count"`
		} `json:"docker_hosts"`
		Total struct {
			Nodes       int `json:"nodes"`
			VMs         int `json:"vms"`
			Containers  int `json:"containers"`
			DockerHosts int `json:"docker_hosts"`
		} `json:"total"`
	}
	if err := json.Unmarshal([]byte(resultText), &resp); err != nil {
		return nil
	}

	// Count running resources
	runningVMs := 0
	for _, vm := range resp.VMs {
		if vm.Status == "running" {
			runningVMs++
		}
	}
	runningLXC := 0
	for _, ct := range resp.Containers {
		if ct.Status == "running" {
			runningLXC++
		}
	}

	var parts []string
	nodeCount := resp.Total.Nodes
	if nodeCount == 0 {
		nodeCount = len(resp.Nodes)
	}
	vmCount := resp.Total.VMs
	if vmCount == 0 {
		vmCount = len(resp.VMs)
	}
	ctCount := resp.Total.Containers
	if ctCount == 0 {
		ctCount = len(resp.Containers)
	}

	if nodeCount > 0 {
		parts = append(parts, fmt.Sprintf("%d nodes", nodeCount))
	}
	if vmCount > 0 {
		parts = append(parts, fmt.Sprintf("%d VMs (%d running)", vmCount, runningVMs))
	}
	if ctCount > 0 {
		parts = append(parts, fmt.Sprintf("%d LXC (%d running)", ctCount, runningLXC))
	}
	dockerCount := resp.Total.DockerHosts
	if dockerCount == 0 {
		dockerCount = len(resp.DockerHosts)
	}
	if dockerCount > 0 {
		totalContainers := 0
		for _, dh := range resp.DockerHosts {
			totalContainers += dh.ContainerCount
		}
		parts = append(parts, fmt.Sprintf("%d docker hosts (%d containers)", dockerCount, totalContainers))
	}

	if len(parts) == 0 {
		return nil
	}

	return []FactEntry{{
		Category: FactCategoryResource,
		Key:      "inventory:summary",
		Value:    truncateValue(strings.Join(parts, ", ")),
	}}
}

func extractQueryConfigFacts(input map[string]interface{}, resultText string) []FactEntry {
	var resp struct {
		GuestType string `json:"guest_type"`
		VMID      int    `json:"vmid"`
		Name      string `json:"name"`
		Node      string `json:"node"`
		Hostname  string `json:"hostname"`
		OSType    string `json:"os_type"`
		Onboot    *bool  `json:"onboot"`
		Mounts    []struct {
			Key        string `json:"key"`
			Mountpoint string `json:"mountpoint"`
		} `json:"mounts"`
		Disks []struct {
			Key string `json:"key"`
		} `json:"disks"`
	}
	if err := json.Unmarshal([]byte(resultText), &resp); err != nil {
		return nil
	}

	if resp.VMID == 0 && resp.Name == "" {
		return nil
	}

	id := fmt.Sprintf("%d", resp.VMID)
	if id == "0" {
		id = resp.Name
	}

	var parts []string
	if resp.GuestType != "" {
		parts = append(parts, resp.GuestType)
	}
	if resp.Hostname != "" {
		parts = append(parts, "hostname="+resp.Hostname)
	}
	if resp.OSType != "" {
		parts = append(parts, "os="+resp.OSType)
	}
	if resp.Onboot != nil {
		if *resp.Onboot {
			parts = append(parts, "onboot=yes")
		} else {
			parts = append(parts, "onboot=no")
		}
	}
	if len(resp.Mounts) > 0 {
		parts = append(parts, fmt.Sprintf("%d mounts", len(resp.Mounts)))
	}
	if len(resp.Disks) > 0 {
		parts = append(parts, fmt.Sprintf("%d disks", len(resp.Disks)))
	}

	if len(parts) == 0 {
		return nil
	}

	facts := []FactEntry{{
		Category: FactCategoryResource,
		Key:      fmt.Sprintf("config:%s:%s", resp.Node, id),
		Value:    truncateValue(strings.Join(parts, ", ")),
	}}

	// Secondary fact with predictable key for gate matching.
	// The model often omits "node" from the tool input, so config:{node}:{id}
	// can't be predicted. This key uses just the resource_id.
	resourceID := strFromMap(input, "resource_id")
	if resourceID != "" {
		facts = append(facts, FactEntry{
			Category: FactCategoryResource,
			Key:      fmt.Sprintf("config:%s:cached", resourceID),
			Value:    truncateValue(strings.Join(parts, ", ")),
		})
	}

	return facts
}

// categoryForPredictedKey infers the FactCategory from a predicted key prefix.
// Used when storing negative markers for text/error responses.
func categoryForPredictedKey(key string) FactCategory {
	switch {
	case strings.HasPrefix(key, "storage:") || strings.HasPrefix(key, "disk_health:") ||
		strings.HasPrefix(key, "raid:") || strings.HasPrefix(key, "backups:") ||
		strings.HasPrefix(key, "physical_disks:") || strings.HasPrefix(key, "ceph:") ||
		strings.HasPrefix(key, "ceph_details:") || strings.HasPrefix(key, "snapshots:") ||
		strings.HasPrefix(key, "replication:") || strings.HasPrefix(key, "pbs_jobs:") ||
		strings.HasPrefix(key, "resource_disks:") || strings.HasPrefix(key, "backup_tasks:") ||
		strings.HasPrefix(key, "backup:"):
		return FactCategoryStorage
	case strings.HasPrefix(key, "metrics:") || strings.HasPrefix(key, "baseline:") ||
		strings.HasPrefix(key, "baselines:") || strings.HasPrefix(key, "temperatures:"):
		return FactCategoryMetrics
	case strings.HasPrefix(key, "exec:"):
		return FactCategoryExec
	case strings.HasPrefix(key, "discovery:"):
		return FactCategoryDiscovery
	case strings.HasPrefix(key, "topology:") || strings.HasPrefix(key, "health:") ||
		strings.HasPrefix(key, "search:") || strings.HasPrefix(key, "query:") ||
		strings.HasPrefix(key, "inventory:") || strings.HasPrefix(key, "config:") ||
		strings.HasPrefix(key, "docker_") || strings.HasPrefix(key, "k8s_") ||
		strings.HasPrefix(key, "pmg:") || strings.HasPrefix(key, "pmg_"):
		return FactCategoryResource
	case strings.HasPrefix(key, "finding:") || strings.HasPrefix(key, "findings:") ||
		strings.HasPrefix(key, "patrol_findings:"):
		return FactCategoryFinding
	case strings.HasPrefix(key, "alert:") || strings.HasPrefix(key, "alerts:"):
		return FactCategoryAlert
	default:
		return FactCategoryResource
	}
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
	case "disk_health":
		return extractDiskHealthFacts(resultText)
	case "raid":
		return extractStorageRAIDFacts(resultText)
	case "backups":
		return extractStorageBackupsFacts(resultText)
	case "ceph":
		return extractCephFacts(resultText)
	case "ceph_details":
		return extractCephDetailsFacts(resultText)
	case "snapshots":
		return extractSnapshotsFacts(resultText)
	case "replication":
		return extractReplicationFacts(resultText)
	case "pbs_jobs":
		return extractPBSJobsFacts(resultText)
	case "resource_disks":
		return extractResourceDisksFacts(resultText)
	default:
		log.Debug().Str("tool", "pulse_storage").Str("action", action).
			Msg("[KnowledgeExtractor] No extractor for action")
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

	// Emit marker fact so PredictFactKeys can gate repeat calls
	var facts []FactEntry
	facts = append(facts, FactEntry{
		Category: FactCategoryStorage,
		Key:      "storage:pools:queried",
		Value:    fmt.Sprintf("%d pools extracted", len(resp.Pools)),
	})

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
	// Note: vmid is an int in the API response (can be 0 for cluster-level tasks)
	var resp struct {
		Tasks []struct {
			VMID      int    `json:"vmid"`
			Node      string `json:"node"`
			Status    string `json:"status"`
			StartTime string `json:"start_time"`
			Error     string `json:"error"`
		} `json:"tasks"`
		Total int `json:"total"`
	}
	if err := json.Unmarshal([]byte(resultText), &resp); err != nil {
		return nil
	}

	total := resp.Total
	if total == 0 {
		total = len(resp.Tasks)
	}

	// Count failures — status is case-insensitive
	failed := 0
	for _, task := range resp.Tasks {
		s := strings.ToUpper(task.Status)
		if s != "OK" && s != "SUCCESS" && s != "" {
			failed++
		}
	}

	markerFact := FactEntry{
		Category: FactCategoryStorage,
		Key:      "backup_tasks:queried",
		Value:    fmt.Sprintf("%d tasks, %d failed", total, failed),
	}

	facts := []FactEntry{markerFact}
	for _, task := range resp.Tasks {
		// Only record failures as individual facts
		s := strings.ToUpper(task.Status)
		if s == "OK" || s == "SUCCESS" || s == "" {
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
		vmidStr := fmt.Sprintf("%d", task.VMID)
		facts = append(facts, FactEntry{
			Category: FactCategoryStorage,
			Key:      fmt.Sprintf("backup:%s:%s", vmidStr, task.Node),
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

// buildExecKeyCmd derives the command portion of the KA fact key from tool input.
// Shared by extractExecFacts and PredictFactKeys to ensure consistent key generation.
// Returns empty string if no distinguishing command/action can be determined.
func buildExecKeyCmd(input map[string]interface{}) string {
	cmd := strFromMap(input, "command")
	if cmd == "" {
		action := strFromMap(input, "action")
		path := strFromMap(input, "path")
		if action != "" && path != "" {
			cmd = action + ":" + path
		} else if action == "logs" {
			// For log queries, include distinguishing params to avoid key collisions.
			// Different since/grep/source/unit combos must produce different keys.
			var parts []string
			parts = append(parts, "logs")
			for _, param := range []string{"since", "grep", "source", "unit"} {
				if v := strFromMap(input, param); v != "" {
					parts = append(parts, param+"="+v)
				}
			}
			cmd = strings.Join(parts, ":")
		} else if action != "" {
			cmd = action
		}
	}
	if cmd == "" {
		return ""
	}
	if len(cmd) > 60 {
		cmd = cmd[:60]
	}
	return cmd
}

func extractExecFacts(input map[string]interface{}, resultText string) []FactEntry {
	host := strFromMap(input, "target_host")
	if host == "" {
		host = strFromMap(input, "host")
	}
	cmdPrefix := buildExecKeyCmd(input)
	if cmdPrefix == "" {
		return nil
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

	switch action {
	case "performance":
		return extractMetricsPerformanceFacts(input, resultText)
	case "baselines":
		return extractMetricsBaselinesFacts(resultText)
	case "disks":
		return extractMetricsDisksFacts(resultText)
	case "temperatures":
		return extractMetricsTemperaturesFacts(resultText)
	default:
		log.Debug().Str("tool", "pulse_metrics").Str("action", action).
			Msg("[KnowledgeExtractor] No extractor for action")
		return nil
	}
}

func extractMetricsPerformanceFacts(input map[string]interface{}, resultText string) []FactEntry {
	resourceID := strFromMap(input, "resource_id")
	if resourceID == "" {
		return nil
	}

	var resp struct {
		ResourceID string `json:"resource_id"`
		Period     string `json:"period"`
		Points     []struct {
			CPU    float64 `json:"cpu"`
			Memory float64 `json:"memory"`
			Disk   float64 `json:"disk"`
		} `json:"points"`
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

	var avgCPU, maxCPU, avgMem, maxMem float64
	var trend string

	// Single-resource queries return Points array; compute summary from points
	if len(resp.Points) > 0 {
		var sumCPU, sumMem float64
		for _, p := range resp.Points {
			sumCPU += p.CPU
			sumMem += p.Memory
			if p.CPU > maxCPU {
				maxCPU = p.CPU
			}
			if p.Memory > maxMem {
				maxMem = p.Memory
			}
		}
		n := float64(len(resp.Points))
		avgCPU = sumCPU / n
		avgMem = sumMem / n
	} else if len(resp.Summary) > 0 {
		// Multi-resource queries return Summary map
		if s, ok := resp.Summary[resourceID]; ok {
			avgCPU = s.AvgCPU
			maxCPU = s.MaxCPU
			avgMem = s.AvgMemory
			maxMem = s.MaxMemory
			trend = s.Trend
		} else {
			for _, s := range resp.Summary {
				avgCPU = s.AvgCPU
				maxCPU = s.MaxCPU
				avgMem = s.AvgMemory
				maxMem = s.MaxMemory
				trend = s.Trend
				break
			}
		}
	}

	var parts []string
	if avgCPU > 0 {
		parts = append(parts, fmt.Sprintf("avg_cpu=%.1f%%", avgCPU))
	}
	if maxCPU > 0 {
		parts = append(parts, fmt.Sprintf("max_cpu=%.1f%%", maxCPU))
	}
	if avgMem > 0 {
		parts = append(parts, fmt.Sprintf("avg_mem=%.1f%%", avgMem))
	}
	if maxMem > 0 {
		parts = append(parts, fmt.Sprintf("max_mem=%.1f%%", maxMem))
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

func extractMetricsBaselinesFacts(resultText string) []FactEntry {
	// BaselinesResponse — real format is nested: baselines.{nodeName}.{resourceKey:metricType}
	// where each metric entry has mean/std_dev/min/max.
	// Example: baselines.delly."delly:101:cpu" = {mean: 0.9, std_dev: 0.5, min: -0.2, max: 2.1}
	// Node-level metrics use just "cpu"/"memory" as keys.
	var resp struct {
		Baselines map[string]map[string]struct {
			Mean   float64 `json:"mean"`
			StdDev float64 `json:"std_dev"`
			Min    float64 `json:"min"`
			Max    float64 `json:"max"`
		} `json:"baselines"`
	}
	if err := json.Unmarshal([]byte(resultText), &resp); err != nil {
		return nil
	}

	// Emit marker fact so PredictFactKeys can gate repeat calls (even for empty results)
	markerFact := FactEntry{
		Category: FactCategoryMetrics,
		Key:      "baselines:queried",
		Value:    fmt.Sprintf("%d nodes extracted", len(resp.Baselines)),
	}

	if len(resp.Baselines) == 0 {
		return []FactEntry{markerFact}
	}

	// Aggregate per-node: collect cpu and memory stats for each node
	facts := []FactEntry{markerFact}
	count := 0
	for nodeName, metrics := range resp.Baselines {
		if count >= 10 {
			break
		}

		// Separate node-level metrics from resource-level metrics
		var cpuMeans, memMeans []float64
		var cpuMax, memMax float64
		for metricKey, stat := range metrics {
			// Keys are like "delly:101:cpu", "delly:101:memory", or bare "cpu", "memory"
			if strings.HasSuffix(metricKey, ":cpu") || metricKey == "cpu" {
				cpuMeans = append(cpuMeans, stat.Mean)
				if stat.Max > cpuMax {
					cpuMax = stat.Max
				}
			}
			if strings.HasSuffix(metricKey, ":memory") || metricKey == "memory" {
				memMeans = append(memMeans, stat.Mean)
				if stat.Max > memMax {
					memMax = stat.Max
				}
			}
		}

		var parts []string
		if len(cpuMeans) > 0 {
			avgCPU := average(cpuMeans)
			parts = append(parts, fmt.Sprintf("cpu: avg=%.1f%% max=%.1f%%", avgCPU, cpuMax))
		}
		if len(memMeans) > 0 {
			avgMem := average(memMeans)
			parts = append(parts, fmt.Sprintf("memory: avg=%.1f%% max=%.1f%%", avgMem, memMax))
		}
		if len(parts) == 0 {
			parts = append(parts, fmt.Sprintf("%d metrics tracked", len(metrics)))
		}

		facts = append(facts, FactEntry{
			Category: FactCategoryMetrics,
			Key:      fmt.Sprintf("baseline:%s", nodeName),
			Value:    truncateValue(strings.Join(parts, ", ")),
		})
		count++
	}
	return facts
}

func average(vals []float64) float64 {
	if len(vals) == 0 {
		return 0
	}
	sum := 0.0
	for _, v := range vals {
		sum += v
	}
	return sum / float64(len(vals))
}

func extractMetricsDisksFacts(resultText string) []FactEntry {
	// PhysicalDisksResponse
	var resp struct {
		Disks []struct {
			Host   string `json:"host"`
			Device string `json:"device"`
			Model  string `json:"model"`
			Health string `json:"health"`
			Status string `json:"status"`
		} `json:"disks"`
	}
	if err := json.Unmarshal([]byte(resultText), &resp); err != nil {
		return nil
	}

	// Emit marker fact so PredictFactKeys can gate repeat calls (even for empty results)
	markerFact := FactEntry{
		Category: FactCategoryStorage,
		Key:      "physical_disks:queried",
		Value:    "summary extracted",
	}

	if len(resp.Disks) == 0 {
		return []FactEntry{markerFact}
	}

	total := len(resp.Disks)
	failed := 0
	var failedDescs []string
	for _, d := range resp.Disks {
		health := strings.ToUpper(d.Health)
		if health == "" {
			health = strings.ToUpper(d.Status)
		}
		if health != "PASSED" && health != "OK" && health != "" {
			failed++
			desc := d.Host + " " + d.Device
			if d.Model != "" {
				desc += " " + d.Model
			}
			failedDescs = append(failedDescs, desc)
		}
	}

	var value string
	if failed == 0 {
		value = fmt.Sprintf("%d disks total, all PASSED", total)
	} else {
		value = fmt.Sprintf("%d disks, %d FAILED: %s", total, failed, strings.Join(failedDescs, "; "))
	}

	return []FactEntry{markerFact, {
		Category: FactCategoryStorage,
		Key:      "physical_disks:summary",
		Value:    truncateValue(value),
	}}
}

// --- pulse_storage: disk_health ---

func extractDiskHealthFacts(resultText string) []FactEntry {
	// DiskHealthResponse — per-host SMART data.
	// Real format uses "smart" array, not "disks". Try both for robustness.
	var resp struct {
		Hosts []struct {
			Hostname string `json:"hostname"`
			Host     string `json:"host"`
			Smart    []struct {
				Device string `json:"device"`
				Model  string `json:"model"`
				Health string `json:"health"`
				Status string `json:"status"`
			} `json:"smart"`
			Disks []struct {
				Device string `json:"device"`
				Model  string `json:"model"`
				Health string `json:"health"`
				Status string `json:"status"`
			} `json:"disks"`
		} `json:"hosts"`
	}
	if err := json.Unmarshal([]byte(resultText), &resp); err != nil {
		return nil
	}

	// Emit marker fact so PredictFactKeys can gate repeat calls (even for empty results)
	markerFact := FactEntry{
		Category: FactCategoryStorage,
		Key:      "disk_health:queried",
		Value:    fmt.Sprintf("%d hosts extracted", len(resp.Hosts)),
	}

	if len(resp.Hosts) == 0 {
		return []FactEntry{markerFact}
	}

	facts := []FactEntry{markerFact}

	for _, host := range resp.Hosts {
		hostname := host.Hostname
		if hostname == "" {
			hostname = host.Host
		}
		if hostname == "" {
			continue
		}

		// Prefer "smart" field (real format), fall back to "disks" (test compat)
		disks := host.Smart
		if len(disks) == 0 {
			disks = host.Disks
		}
		total := len(disks)
		passed := 0
		failed := 0
		var failedDescs []string
		for _, d := range disks {
			health := strings.ToUpper(d.Health)
			if health == "" {
				health = strings.ToUpper(d.Status)
			}
			if health == "PASSED" || health == "OK" {
				passed++
			} else if health != "" {
				failed++
				desc := d.Device
				if d.Model != "" {
					desc += " " + d.Model
				}
				failedDescs = append(failedDescs, desc)
			} else {
				passed++ // Unknown treated as passed
			}
		}

		var value string
		if failed == 0 {
			value = fmt.Sprintf("%d disks all PASSED", total)
		} else {
			value = fmt.Sprintf("%d SMART disks: %d PASSED, %d FAILED (%s)", total, passed, failed, strings.Join(failedDescs, ", "))
		}

		facts = append(facts, FactEntry{
			Category: FactCategoryStorage,
			Key:      fmt.Sprintf("disk_health:%s", hostname),
			Value:    truncateValue(value),
		})
	}
	return facts
}

// --- pulse_alerts ---

func extractAlertsFacts(input map[string]interface{}, resultText string) []FactEntry {
	action := strFromMap(input, "action")
	if action == "" {
		action = strFromMap(input, "type")
	}

	switch action {
	case "findings":
		return extractAlertsFindingsFacts(resultText)
	case "list":
		return extractAlertsListFacts(resultText)
	default:
		log.Debug().Str("tool", "pulse_alerts").Str("action", action).
			Msg("[KnowledgeExtractor] No extractor for action")
		return nil
	}
}

func extractAlertsFindingsFacts(resultText string) []FactEntry {
	// Response format: {"active": [...], "counts": {"active": N, "dismissed": N}}
	var resp struct {
		Active []struct {
			Key        string `json:"key"`
			Severity   string `json:"severity"`
			Title      string `json:"title"`
			ResourceID string `json:"resource_id"`
		} `json:"active"`
		Counts struct {
			Active    int `json:"active"`
			Dismissed int `json:"dismissed"`
		} `json:"counts"`
	}
	if err := json.Unmarshal([]byte(resultText), &resp); err != nil {
		return nil
	}

	active := resp.Counts.Active
	dismissed := resp.Counts.Dismissed
	if active == 0 && dismissed == 0 && len(resp.Active) == 0 {
		return nil
	}

	var facts []FactEntry
	facts = append(facts, FactEntry{
		Category: FactCategoryFinding,
		Key:      "findings:overview",
		Value:    fmt.Sprintf("%d active, %d dismissed", active, dismissed),
	})

	// Per-finding facts (cap at 5)
	limit := 5
	if len(resp.Active) < limit {
		limit = len(resp.Active)
	}
	for _, f := range resp.Active[:limit] {
		if f.Key == "" {
			continue
		}
		var parts []string
		if f.Severity != "" {
			parts = append(parts, f.Severity)
		}
		if f.Title != "" {
			parts = append(parts, f.Title)
		}
		if f.ResourceID != "" {
			parts = append(parts, "(resource="+f.ResourceID+")")
		}
		if len(parts) > 0 {
			// Include resource_id in key to avoid upsert collisions when multiple
			// findings share the same key (e.g. two "backup-failed" for different resources)
			factKey := fmt.Sprintf("finding:%s", f.Key)
			if f.ResourceID != "" {
				factKey = fmt.Sprintf("finding:%s:%s", f.Key, f.ResourceID)
			}
			facts = append(facts, FactEntry{
				Category: FactCategoryFinding,
				Key:      factKey,
				Value:    truncateValue(strings.Join(parts, ": ")),
			})
		}
	}

	return facts
}

func extractAlertsListFacts(resultText string) []FactEntry {
	var resp struct {
		Alerts []struct {
			ResourceName string  `json:"resource_name"`
			Type         string  `json:"type"`
			Severity     string  `json:"severity"`
			Value        float64 `json:"value"`
			Threshold    float64 `json:"threshold"`
			Status       string  `json:"status"`
		} `json:"alerts"`
	}
	if err := json.Unmarshal([]byte(resultText), &resp); err != nil {
		return nil
	}

	if len(resp.Alerts) == 0 {
		return nil
	}

	// Count active
	active := 0
	for _, a := range resp.Alerts {
		if strings.ToLower(a.Status) != "resolved" {
			active++
		}
	}

	var facts []FactEntry
	facts = append(facts, FactEntry{
		Category: FactCategoryAlert,
		Key:      "alerts:overview",
		Value:    fmt.Sprintf("%d active alerts", active),
	})

	// Per-alert facts (cap at 5)
	limit := 5
	if len(resp.Alerts) < limit {
		limit = len(resp.Alerts)
	}
	for _, a := range resp.Alerts[:limit] {
		if a.ResourceName == "" && a.Type == "" {
			continue
		}
		var parts []string
		if a.Severity != "" {
			parts = append(parts, a.Severity)
		}
		if a.Value > 0 && a.Threshold > 0 {
			parts = append(parts, fmt.Sprintf("%.1f%% (threshold %.0f%%)", a.Value, a.Threshold))
		}
		key := fmt.Sprintf("alert:%s:%s", a.ResourceName, a.Type)
		if len(parts) > 0 {
			facts = append(facts, FactEntry{
				Category: FactCategoryAlert,
				Key:      key,
				Value:    truncateValue(strings.Join(parts, ": ")),
			})
		}
	}

	return facts
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

// --- pulse_docker ---

func extractDockerFacts(input map[string]interface{}, resultText string) []FactEntry {
	action := strFromMap(input, "action")
	if action == "" {
		action = strFromMap(input, "type")
	}

	switch action {
	case "services":
		return extractDockerServicesFacts(resultText)
	case "updates":
		return extractDockerUpdatesFacts(resultText)
	case "swarm":
		return extractDockerSwarmFacts(resultText)
	case "tasks":
		return extractDockerTasksFacts(resultText)
	default:
		log.Debug().Str("tool", "pulse_docker").Str("action", action).
			Msg("[KnowledgeExtractor] No extractor for action")
		return nil
	}
}

func extractDockerServicesFacts(resultText string) []FactEntry {
	var resp struct {
		Host     string `json:"host"`
		Services []struct {
			Name         string `json:"name"`
			Mode         string `json:"mode"`
			DesiredTasks int    `json:"desired_tasks"`
			RunningTasks int    `json:"running_tasks"`
			UpdateStatus string `json:"update_status"`
		} `json:"services"`
		Total int `json:"total"`
	}
	if err := json.Unmarshal([]byte(resultText), &resp); err != nil {
		return nil
	}

	total := resp.Total
	if total == 0 {
		total = len(resp.Services)
	}

	markerFact := FactEntry{
		Category: FactCategoryResource,
		Key:      "docker_services:queried",
		Value:    fmt.Sprintf("%d services", total),
	}

	if total == 0 {
		return []FactEntry{markerFact}
	}

	// Count healthy vs unhealthy
	healthy := 0
	for _, s := range resp.Services {
		if s.RunningTasks >= s.DesiredTasks {
			healthy++
		}
	}

	summaryValue := fmt.Sprintf("%d services, %d healthy, %d degraded", total, healthy, total-healthy)

	return []FactEntry{markerFact, {
		Category: FactCategoryResource,
		Key:      "docker_services:summary",
		Value:    truncateValue(summaryValue),
	}}
}

func extractDockerUpdatesFacts(resultText string) []FactEntry {
	var resp struct {
		Updates []struct {
			ContainerName   string `json:"container_name"`
			UpdateAvailable bool   `json:"update_available"`
			Error           string `json:"error"`
		} `json:"updates"`
		Total int `json:"total"`
	}
	if err := json.Unmarshal([]byte(resultText), &resp); err != nil {
		return nil
	}

	total := resp.Total
	if total == 0 {
		total = len(resp.Updates)
	}

	available := 0
	for _, u := range resp.Updates {
		if u.UpdateAvailable {
			available++
		}
	}

	markerFact := FactEntry{
		Category: FactCategoryResource,
		Key:      "docker_updates:queried",
		Value:    fmt.Sprintf("%d containers, %d updates available", total, available),
	}

	return []FactEntry{markerFact}
}

func extractDockerSwarmFacts(resultText string) []FactEntry {
	var resp struct {
		Host   string `json:"host"`
		Status struct {
			NodeRole         string `json:"node_role"`
			LocalState       string `json:"local_state"`
			ControlAvailable bool   `json:"control_available"`
			ClusterName      string `json:"cluster_name"`
			Error            string `json:"error"`
		} `json:"status"`
	}
	if err := json.Unmarshal([]byte(resultText), &resp); err != nil {
		return nil
	}

	host := resp.Host
	s := resp.Status

	var parts []string
	if s.NodeRole != "" {
		parts = append(parts, "role="+s.NodeRole)
	}
	if s.LocalState != "" {
		parts = append(parts, "state="+s.LocalState)
	}
	if s.ControlAvailable {
		parts = append(parts, "manager")
	}
	if s.Error != "" {
		parts = append(parts, "error="+s.Error)
	}

	if len(parts) == 0 {
		return nil
	}

	if host != "" {
		parts = append(parts, "host="+host)
	}

	return []FactEntry{{
		Category: FactCategoryResource,
		Key:      "docker_swarm:status",
		Value:    truncateValue(strings.Join(parts, ", ")),
	}}
}

func extractDockerTasksFacts(resultText string) []FactEntry {
	var resp struct {
		Host    string `json:"host"`
		Service string `json:"service"`
		Tasks   []struct {
			CurrentState string `json:"current_state"`
			Error        string `json:"error"`
		} `json:"tasks"`
		Total int `json:"total"`
	}
	if err := json.Unmarshal([]byte(resultText), &resp); err != nil {
		return nil
	}

	total := resp.Total
	if total == 0 {
		total = len(resp.Tasks)
	}

	running := 0
	failed := 0
	for _, t := range resp.Tasks {
		switch strings.ToLower(t.CurrentState) {
		case "running":
			running++
		case "failed", "rejected":
			failed++
		}
	}

	value := fmt.Sprintf("%d tasks, %d running, %d failed", total, running, failed)
	if resp.Service != "" {
		value = fmt.Sprintf("service=%s, %s", resp.Service, value)
	}

	return []FactEntry{{
		Category: FactCategoryResource,
		Key:      "docker_tasks:queried",
		Value:    truncateValue(value),
	}}
}

// --- pulse_kubernetes ---

func extractKubernetesFacts(input map[string]interface{}, resultText string) []FactEntry {
	action := strFromMap(input, "action")
	if action == "" {
		action = strFromMap(input, "type")
	}

	switch action {
	case "clusters":
		return extractK8sClustersFacts(resultText)
	case "nodes":
		return extractK8sNodesFacts(resultText)
	case "pods":
		return extractK8sPodsFacts(resultText)
	case "deployments":
		return extractK8sDeploymentsFacts(resultText)
	default:
		log.Debug().Str("tool", "pulse_kubernetes").Str("action", action).
			Msg("[KnowledgeExtractor] No extractor for action")
		return nil
	}
}

func extractK8sClustersFacts(resultText string) []FactEntry {
	var resp struct {
		Clusters []struct {
			Name        string `json:"name"`
			DisplayName string `json:"display_name"`
			Status      string `json:"status"`
			NodeCount   int    `json:"node_count"`
			PodCount    int    `json:"pod_count"`
			ReadyNodes  int    `json:"ready_nodes"`
		} `json:"clusters"`
		Total int `json:"total"`
	}
	if err := json.Unmarshal([]byte(resultText), &resp); err != nil {
		return nil
	}

	total := resp.Total
	if total == 0 {
		total = len(resp.Clusters)
	}

	markerFact := FactEntry{
		Category: FactCategoryResource,
		Key:      "k8s_clusters:queried",
		Value:    fmt.Sprintf("%d clusters", total),
	}

	if total == 0 {
		return []FactEntry{markerFact}
	}

	facts := []FactEntry{markerFact}
	limit := 5
	if len(resp.Clusters) < limit {
		limit = len(resp.Clusters)
	}
	for _, c := range resp.Clusters[:limit] {
		name := c.DisplayName
		if name == "" {
			name = c.Name
		}
		if name == "" {
			continue
		}
		value := fmt.Sprintf("%s, %d nodes (%d ready), %d pods",
			c.Status, c.NodeCount, c.ReadyNodes, c.PodCount)
		facts = append(facts, FactEntry{
			Category: FactCategoryResource,
			Key:      fmt.Sprintf("k8s_cluster:%s", name),
			Value:    truncateValue(value),
		})
	}
	return facts
}

func extractK8sNodesFacts(resultText string) []FactEntry {
	var resp struct {
		Cluster string `json:"cluster"`
		Nodes   []struct {
			Name  string   `json:"name"`
			Ready bool     `json:"ready"`
			Roles []string `json:"roles"`
		} `json:"nodes"`
		Total int `json:"total"`
	}
	if err := json.Unmarshal([]byte(resultText), &resp); err != nil {
		return nil
	}

	total := resp.Total
	if total == 0 {
		total = len(resp.Nodes)
	}

	ready := 0
	for _, n := range resp.Nodes {
		if n.Ready {
			ready++
		}
	}

	value := fmt.Sprintf("%d nodes, %d ready, %d not ready", total, ready, total-ready)
	if resp.Cluster != "" {
		value = fmt.Sprintf("cluster=%s, %s", resp.Cluster, value)
	}

	return []FactEntry{{
		Category: FactCategoryResource,
		Key:      "k8s_nodes:queried",
		Value:    truncateValue(value),
	}}
}

func extractK8sPodsFacts(resultText string) []FactEntry {
	var resp struct {
		Cluster string `json:"cluster"`
		Pods    []struct {
			Name      string `json:"name"`
			Namespace string `json:"namespace"`
			Phase     string `json:"phase"`
			Restarts  int    `json:"restarts"`
		} `json:"pods"`
		Total int `json:"total"`
	}
	if err := json.Unmarshal([]byte(resultText), &resp); err != nil {
		return nil
	}

	total := resp.Total
	if total == 0 {
		total = len(resp.Pods)
	}

	// Count by phase
	phaseCounts := make(map[string]int)
	crashLoop := 0
	for _, p := range resp.Pods {
		phase := p.Phase
		if phase == "" {
			phase = "Unknown"
		}
		phaseCounts[phase]++
		if p.Restarts > 5 {
			crashLoop++
		}
	}

	var parts []string
	if resp.Cluster != "" {
		parts = append(parts, "cluster="+resp.Cluster)
	}
	parts = append(parts, fmt.Sprintf("%d pods", total))
	for phase, count := range phaseCounts {
		parts = append(parts, fmt.Sprintf("%d %s", count, phase))
	}
	if crashLoop > 0 {
		parts = append(parts, fmt.Sprintf("%d high-restart", crashLoop))
	}

	return []FactEntry{{
		Category: FactCategoryResource,
		Key:      "k8s_pods:queried",
		Value:    truncateValue(strings.Join(parts, ", ")),
	}}
}

func extractK8sDeploymentsFacts(resultText string) []FactEntry {
	var resp struct {
		Cluster     string `json:"cluster"`
		Deployments []struct {
			Name              string `json:"name"`
			Namespace         string `json:"namespace"`
			DesiredReplicas   int32  `json:"desired_replicas"`
			ReadyReplicas     int32  `json:"ready_replicas"`
			AvailableReplicas int32  `json:"available_replicas"`
		} `json:"deployments"`
		Total int `json:"total"`
	}
	if err := json.Unmarshal([]byte(resultText), &resp); err != nil {
		return nil
	}

	total := resp.Total
	if total == 0 {
		total = len(resp.Deployments)
	}

	healthy := 0
	degraded := 0
	for _, d := range resp.Deployments {
		if d.ReadyReplicas >= d.DesiredReplicas {
			healthy++
		} else {
			degraded++
		}
	}

	value := fmt.Sprintf("%d deployments, %d healthy, %d degraded", total, healthy, degraded)
	if resp.Cluster != "" {
		value = fmt.Sprintf("cluster=%s, %s", resp.Cluster, value)
	}

	return []FactEntry{{
		Category: FactCategoryResource,
		Key:      "k8s_deployments:queried",
		Value:    truncateValue(value),
	}}
}

// --- pulse_pmg ---

func extractPMGFacts(input map[string]interface{}, resultText string) []FactEntry {
	action := strFromMap(input, "action")
	if action == "" {
		action = strFromMap(input, "type")
	}

	switch action {
	case "status":
		return extractPMGStatusFacts(resultText)
	case "mail_stats":
		return extractPMGMailStatsFacts(resultText)
	case "queues":
		return extractPMGQueuesFacts(resultText)
	case "spam":
		return extractPMGSpamFacts(resultText)
	default:
		log.Debug().Str("tool", "pulse_pmg").Str("action", action).
			Msg("[KnowledgeExtractor] No extractor for action")
		return nil
	}
}

func extractPMGStatusFacts(resultText string) []FactEntry {
	var resp struct {
		Instances []struct {
			Name    string `json:"name"`
			Host    string `json:"host"`
			Status  string `json:"status"`
			Version string `json:"version"`
		} `json:"instances"`
		Total int `json:"total"`
	}
	if err := json.Unmarshal([]byte(resultText), &resp); err != nil {
		return nil
	}

	total := resp.Total
	if total == 0 {
		total = len(resp.Instances)
	}

	markerFact := FactEntry{
		Category: FactCategoryResource,
		Key:      "pmg:queried",
		Value:    fmt.Sprintf("%d instances", total),
	}

	if total == 0 {
		return []FactEntry{markerFact}
	}

	facts := []FactEntry{markerFact}
	for _, inst := range resp.Instances {
		name := inst.Name
		if name == "" {
			name = inst.Host
		}
		if name == "" {
			continue
		}
		var parts []string
		if inst.Status != "" {
			parts = append(parts, inst.Status)
		}
		if inst.Version != "" {
			parts = append(parts, "v"+inst.Version)
		}
		if len(parts) == 0 {
			continue
		}
		facts = append(facts, FactEntry{
			Category: FactCategoryResource,
			Key:      fmt.Sprintf("pmg:%s", name),
			Value:    truncateValue(strings.Join(parts, ", ")),
		})
	}
	return facts
}

func extractPMGMailStatsFacts(resultText string) []FactEntry {
	var resp struct {
		Instance string `json:"instance"`
		Stats    struct {
			TotalIn  float64 `json:"total_in"`
			TotalOut float64 `json:"total_out"`
			SpamIn   float64 `json:"spam_in"`
			VirusIn  float64 `json:"virus_in"`
		} `json:"stats"`
	}
	if err := json.Unmarshal([]byte(resultText), &resp); err != nil {
		return nil
	}

	s := resp.Stats
	value := fmt.Sprintf("in=%.0f out=%.0f spam=%.0f virus=%.0f",
		s.TotalIn, s.TotalOut, s.SpamIn, s.VirusIn)
	if resp.Instance != "" {
		value = resp.Instance + ": " + value
	}

	return []FactEntry{{
		Category: FactCategoryResource,
		Key:      "pmg_mail_stats:queried",
		Value:    truncateValue(value),
	}}
}

func extractPMGQueuesFacts(resultText string) []FactEntry {
	var resp struct {
		Instance string `json:"instance"`
		Queues   []struct {
			Node     string `json:"node"`
			Active   int    `json:"active"`
			Deferred int    `json:"deferred"`
			Total    int    `json:"total"`
		} `json:"queues"`
	}
	if err := json.Unmarshal([]byte(resultText), &resp); err != nil {
		return nil
	}

	totalQueued := 0
	totalDeferred := 0
	for _, q := range resp.Queues {
		totalQueued += q.Total
		totalDeferred += q.Deferred
	}

	value := fmt.Sprintf("%d nodes, %d queued, %d deferred",
		len(resp.Queues), totalQueued, totalDeferred)
	if resp.Instance != "" {
		value = resp.Instance + ": " + value
	}

	return []FactEntry{{
		Category: FactCategoryResource,
		Key:      "pmg_queues:queried",
		Value:    truncateValue(value),
	}}
}

func extractPMGSpamFacts(resultText string) []FactEntry {
	var resp struct {
		Instance   string `json:"instance"`
		Quarantine struct {
			Spam  int `json:"spam"`
			Virus int `json:"virus"`
			Total int `json:"total"`
		} `json:"quarantine"`
	}
	if err := json.Unmarshal([]byte(resultText), &resp); err != nil {
		return nil
	}

	q := resp.Quarantine
	value := fmt.Sprintf("quarantine: %d total (%d spam, %d virus)", q.Total, q.Spam, q.Virus)
	if resp.Instance != "" {
		value = resp.Instance + ": " + value
	}

	return []FactEntry{{
		Category: FactCategoryResource,
		Key:      "pmg_spam:queried",
		Value:    truncateValue(value),
	}}
}

// --- patrol_get_findings ---

func extractPatrolGetFindingsFacts(resultText string) []FactEntry {
	var resp struct {
		OK       bool `json:"ok"`
		Count    int  `json:"count"`
		Findings []struct {
			Key        string `json:"key"`
			Severity   string `json:"severity"`
			Title      string `json:"title"`
			ResourceID string `json:"resource_id"`
		} `json:"findings"`
	}
	if err := json.Unmarshal([]byte(resultText), &resp); err != nil {
		return nil
	}

	total := resp.Count
	if total == 0 {
		total = len(resp.Findings)
	}

	markerFact := FactEntry{
		Category: FactCategoryFinding,
		Key:      "patrol_findings:queried",
		Value:    fmt.Sprintf("%d findings", total),
	}

	if total == 0 {
		return []FactEntry{markerFact}
	}

	// Count by severity
	severityCounts := make(map[string]int)
	for _, f := range resp.Findings {
		sev := f.Severity
		if sev == "" {
			sev = "unknown"
		}
		severityCounts[sev]++
	}

	var parts []string
	parts = append(parts, fmt.Sprintf("%d total", total))
	for sev, count := range severityCounts {
		parts = append(parts, fmt.Sprintf("%d %s", count, sev))
	}

	facts := []FactEntry{markerFact, {
		Category: FactCategoryFinding,
		Key:      "patrol_findings:summary",
		Value:    truncateValue(strings.Join(parts, ", ")),
	}}

	return facts
}

// PredictFactKeys returns the KA fact keys that this tool call would produce,
// based solely on the tool input (without needing the result).
// Used by the gate to check if we already have facts for this call.
// Returns nil if the key can't be predicted from input alone.
func PredictFactKeys(toolName string, toolInput map[string]interface{}) []string {
	switch toolName {
	case "pulse_query":
		action := strFromMap(toolInput, "action")
		if action == "" {
			action = strFromMap(toolInput, "type")
		}
		switch action {
		case "topology":
			return []string{"topology:summary"}
		case "health":
			return []string{"health:connections"}
		case "get":
			resourceID := strFromMap(toolInput, "resource_id")
			if resourceID != "" {
				return []string{
					fmt.Sprintf("query:get:%s:cached", resourceID),
					fmt.Sprintf("query:get:%s:error", resourceID),
				}
			}
			return nil
		case "search":
			query := strFromMap(toolInput, "query")
			if query == "" {
				query = strFromMap(toolInput, "search")
			}
			if query != "" {
				return []string{fmt.Sprintf("search:%s:summary", query)}
			}
		case "list":
			return []string{"inventory:summary"}
		case "config":
			resourceID := strFromMap(toolInput, "resource_id")
			if resourceID != "" {
				// Predict the cached key (always available) + node-specific key if node is provided
				keys := []string{fmt.Sprintf("config:%s:cached", resourceID)}
				node := strFromMap(toolInput, "node")
				if node != "" {
					keys = append(keys, fmt.Sprintf("config:%s:%s", node, resourceID))
				}
				return keys
			}
		}
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
		cmdPrefix := buildExecKeyCmd(toolInput)
		if host != "" && cmdPrefix != "" {
			return []string{fmt.Sprintf("exec:%s:%s", host, cmdPrefix)}
		}
	case "pulse_storage":
		action := strFromMap(toolInput, "action")
		if action == "" {
			action = strFromMap(toolInput, "type")
		}
		switch action {
		case "pools":
			return []string{"storage:pools:queried"}
		case "disk_health":
			return []string{"disk_health:queried"}
		case "raid":
			return []string{"raid:queried"}
		case "backups":
			return []string{"backups:queried"}
		case "ceph":
			return []string{"ceph:queried"}
		case "ceph_details":
			return []string{"ceph_details:queried"}
		case "snapshots":
			return []string{"snapshots:queried"}
		case "replication":
			return []string{"replication:queried"}
		case "pbs_jobs":
			return []string{"pbs_jobs:queried"}
		case "resource_disks":
			return []string{"resource_disks:queried"}
		case "backup_tasks":
			return []string{"backup_tasks:queried"}
		}
	case "pulse_metrics":
		action := strFromMap(toolInput, "action")
		if action == "" {
			action = strFromMap(toolInput, "type")
		}
		switch action {
		case "performance":
			resourceID := strFromMap(toolInput, "resource_id")
			if resourceID != "" {
				return []string{fmt.Sprintf("metrics:%s", resourceID)}
			}
		case "baselines":
			// Always predict the marker key — the extractor emits baselines:queried
			// regardless of whether resource_id is provided.
			return []string{"baselines:queried"}
		case "disks":
			return []string{"physical_disks:queried"}
		case "temperatures":
			return []string{"temperatures:queried"}
		}
	case "pulse_alerts":
		action := strFromMap(toolInput, "action")
		if action == "" {
			action = strFromMap(toolInput, "type")
		}
		switch action {
		case "findings":
			return []string{"findings:overview"}
		case "list":
			return []string{"alerts:overview"}
		}
	case "pulse_docker":
		action := strFromMap(toolInput, "action")
		if action == "" {
			action = strFromMap(toolInput, "type")
		}
		switch action {
		case "services":
			return []string{"docker_services:queried", "docker_services:summary"}
		case "updates":
			return []string{"docker_updates:queried"}
		case "swarm":
			return []string{"docker_swarm:status"}
		case "tasks":
			return []string{"docker_tasks:queried"}
		}
	case "pulse_kubernetes":
		action := strFromMap(toolInput, "action")
		if action == "" {
			action = strFromMap(toolInput, "type")
		}
		switch action {
		case "clusters":
			return []string{"k8s_clusters:queried"}
		case "nodes":
			return []string{"k8s_nodes:queried"}
		case "pods":
			return []string{"k8s_pods:queried"}
		case "deployments":
			return []string{"k8s_deployments:queried"}
		}
	case "pulse_pmg":
		action := strFromMap(toolInput, "action")
		if action == "" {
			action = strFromMap(toolInput, "type")
		}
		switch action {
		case "status":
			return []string{"pmg:queried"}
		case "mail_stats":
			return []string{"pmg_mail_stats:queried"}
		case "queues":
			return []string{"pmg_queues:queried"}
		case "spam":
			return []string{"pmg_spam:queried"}
		}
	case "patrol_get_findings":
		return []string{"patrol_findings:queried"}
	}
	return nil
}

// --- pulse_metrics: temperatures ---

func extractMetricsTemperaturesFacts(resultText string) []FactEntry {
	var hosts []struct {
		Hostname  string             `json:"hostname"`
		CPUTemps  map[string]float64 `json:"cpu_temps"`
		DiskTemps map[string]float64 `json:"disk_temps"`
	}
	if err := json.Unmarshal([]byte(resultText), &hosts); err != nil {
		return nil
	}

	markerFact := FactEntry{
		Category: FactCategoryMetrics,
		Key:      "temperatures:queried",
		Value:    fmt.Sprintf("%d hosts", len(hosts)),
	}
	if len(hosts) == 0 {
		return []FactEntry{markerFact}
	}

	facts := []FactEntry{markerFact}
	for _, h := range hosts {
		if h.Hostname == "" {
			continue
		}
		var maxCPU float64
		for _, t := range h.CPUTemps {
			if t > maxCPU {
				maxCPU = t
			}
		}
		var maxDisk float64
		for _, t := range h.DiskTemps {
			if t > maxDisk {
				maxDisk = t
			}
		}
		var parts []string
		if maxCPU > 0 {
			parts = append(parts, fmt.Sprintf("cpu_max=%.0f°C", maxCPU))
		}
		if maxDisk > 0 {
			parts = append(parts, fmt.Sprintf("disk_max=%.0f°C", maxDisk))
		}
		if len(parts) == 0 {
			continue
		}
		facts = append(facts, FactEntry{
			Category: FactCategoryMetrics,
			Key:      fmt.Sprintf("temperatures:%s", h.Hostname),
			Value:    truncateValue(strings.Join(parts, ", ")),
		})
	}
	return facts
}

// --- pulse_storage: raid ---

func extractStorageRAIDFacts(resultText string) []FactEntry {
	var resp struct {
		Hosts []struct {
			Hostname string `json:"hostname"`
			Arrays   []struct {
				Device        string `json:"device"`
				Level         string `json:"level"`
				State         string `json:"state"`
				FailedDevices int    `json:"failed_devices"`
				TotalDevices  int    `json:"total_devices"`
			} `json:"arrays"`
		} `json:"hosts"`
	}
	if err := json.Unmarshal([]byte(resultText), &resp); err != nil {
		return nil
	}

	markerFact := FactEntry{
		Category: FactCategoryStorage,
		Key:      "raid:queried",
		Value:    fmt.Sprintf("%d hosts", len(resp.Hosts)),
	}
	if len(resp.Hosts) == 0 {
		return []FactEntry{markerFact}
	}

	facts := []FactEntry{markerFact}
	for _, h := range resp.Hosts {
		if h.Hostname == "" {
			continue
		}
		totalArrays := len(h.Arrays)
		degraded := 0
		for _, a := range h.Arrays {
			if (a.State != "clean" && a.State != "active") || a.FailedDevices > 0 {
				degraded++
			}
		}
		var value string
		if degraded == 0 {
			value = fmt.Sprintf("%d arrays, all clean", totalArrays)
		} else {
			value = fmt.Sprintf("%d arrays, %d degraded/failed", totalArrays, degraded)
		}
		facts = append(facts, FactEntry{
			Category: FactCategoryStorage,
			Key:      fmt.Sprintf("raid:%s", h.Hostname),
			Value:    truncateValue(value),
		})
	}
	return facts
}

// --- pulse_storage: backups ---

func extractStorageBackupsFacts(resultText string) []FactEntry {
	var resp struct {
		PBS        []json.RawMessage `json:"pbs"`
		PVE        []json.RawMessage `json:"pve"`
		PBSServers []struct {
			Name       string `json:"name"`
			Status     string `json:"status"`
			Datastores []struct {
				Name         string  `json:"name"`
				UsagePercent float64 `json:"usage_percent"`
			} `json:"datastores"`
		} `json:"pbs_servers"`
	}
	if err := json.Unmarshal([]byte(resultText), &resp); err != nil {
		return nil
	}

	totalBackups := len(resp.PBS) + len(resp.PVE)
	markerFact := FactEntry{
		Category: FactCategoryStorage,
		Key:      "backups:queried",
		Value:    fmt.Sprintf("%d PBS + %d PVE backups, %d PBS servers", len(resp.PBS), len(resp.PVE), len(resp.PBSServers)),
	}

	facts := []FactEntry{markerFact}

	for _, srv := range resp.PBSServers {
		if srv.Name == "" {
			continue
		}
		var parts []string
		parts = append(parts, srv.Status)
		for _, ds := range srv.Datastores {
			parts = append(parts, fmt.Sprintf("%s: %.1f%% used", ds.Name, ds.UsagePercent))
		}
		facts = append(facts, FactEntry{
			Category: FactCategoryStorage,
			Key:      fmt.Sprintf("backups:server:%s", srv.Name),
			Value:    truncateValue(strings.Join(parts, ", ")),
		})
	}

	if totalBackups > 0 {
		facts = append(facts, FactEntry{
			Category: FactCategoryStorage,
			Key:      "backups:summary",
			Value:    fmt.Sprintf("%d PBS backups, %d PVE backups", len(resp.PBS), len(resp.PVE)),
		})
	}

	return facts
}

// --- pulse_storage: ceph ---

func extractCephFacts(resultText string) []FactEntry {
	var clusters []struct {
		Name    string `json:"name"`
		Health  string `json:"health"`
		Details struct {
			OSDCount     int     `json:"osd_count"`
			OSDsUp       int     `json:"osds_up"`
			OSDsDown     int     `json:"osds_down"`
			Monitors     int     `json:"monitors"`
			UsagePercent float64 `json:"usage_percent"`
		} `json:"details"`
	}
	if err := json.Unmarshal([]byte(resultText), &clusters); err != nil {
		return nil
	}

	markerFact := FactEntry{
		Category: FactCategoryStorage,
		Key:      "ceph:queried",
		Value:    fmt.Sprintf("%d clusters", len(clusters)),
	}

	if len(clusters) == 0 {
		return []FactEntry{markerFact}
	}

	facts := []FactEntry{markerFact}
	limit := 5
	if len(clusters) < limit {
		limit = len(clusters)
	}
	for _, c := range clusters[:limit] {
		if c.Name == "" {
			continue
		}
		d := c.Details
		value := fmt.Sprintf("%s, %d OSDs (%d up, %d down), %d monitors, %.0f%% used",
			c.Health, d.OSDCount, d.OSDsUp, d.OSDsDown, d.Monitors, d.UsagePercent)
		facts = append(facts, FactEntry{
			Category: FactCategoryStorage,
			Key:      fmt.Sprintf("ceph:%s", c.Name),
			Value:    truncateValue(value),
		})
	}
	return facts
}

// --- pulse_storage: ceph_details ---

func extractCephDetailsFacts(resultText string) []FactEntry {
	var resp struct {
		Hosts []struct {
			Hostname string `json:"hostname"`
			Health   struct {
				Status string `json:"status"`
			} `json:"health"`
			OSDMap struct {
				NumOSDs int `json:"num_osds"`
				NumUp   int `json:"num_up"`
				NumDown int `json:"num_down"`
			} `json:"osd_map"`
			PGMap struct {
				UsagePercent float64 `json:"usage_percent"`
			} `json:"pg_map"`
			Pools []json.RawMessage `json:"pools"`
		} `json:"hosts"`
		Total int `json:"total"`
	}
	if err := json.Unmarshal([]byte(resultText), &resp); err != nil {
		return nil
	}

	markerFact := FactEntry{
		Category: FactCategoryStorage,
		Key:      "ceph_details:queried",
		Value:    fmt.Sprintf("%d hosts", len(resp.Hosts)),
	}

	if len(resp.Hosts) == 0 {
		return []FactEntry{markerFact}
	}

	facts := []FactEntry{markerFact}
	limit := 5
	if len(resp.Hosts) < limit {
		limit = len(resp.Hosts)
	}
	for _, h := range resp.Hosts[:limit] {
		if h.Hostname == "" {
			continue
		}
		value := fmt.Sprintf("%s, %d OSDs (%d up), %.0f%% used, %d pools",
			h.Health.Status, h.OSDMap.NumOSDs, h.OSDMap.NumUp, h.PGMap.UsagePercent, len(h.Pools))
		facts = append(facts, FactEntry{
			Category: FactCategoryStorage,
			Key:      fmt.Sprintf("ceph_details:%s", h.Hostname),
			Value:    truncateValue(value),
		})
	}
	return facts
}

// --- pulse_storage: snapshots ---

func extractSnapshotsFacts(resultText string) []FactEntry {
	var resp struct {
		Snapshots []struct {
			VMID         int    `json:"vmid"`
			VMName       string `json:"vm_name"`
			Type         string `json:"type"`
			Node         string `json:"node"`
			SnapshotName string `json:"snapshot_name"`
		} `json:"snapshots"`
		Total int `json:"total"`
	}
	if err := json.Unmarshal([]byte(resultText), &resp); err != nil {
		return nil
	}

	total := resp.Total
	if total == 0 {
		total = len(resp.Snapshots)
	}

	markerFact := FactEntry{
		Category: FactCategoryStorage,
		Key:      "snapshots:queried",
		Value:    fmt.Sprintf("%d snapshots", total),
	}

	if total == 0 {
		return []FactEntry{markerFact}
	}

	// Count by guest
	guestCounts := make(map[string]int)
	for _, s := range resp.Snapshots {
		name := s.VMName
		if name == "" {
			name = fmt.Sprintf("%d", s.VMID)
		}
		guestCounts[name]++
	}

	var summaryParts []string
	count := 0
	for guest, cnt := range guestCounts {
		if count >= 5 {
			break
		}
		summaryParts = append(summaryParts, fmt.Sprintf("%s: %d", guest, cnt))
		count++
	}

	summaryValue := fmt.Sprintf("%d total; %s", total, strings.Join(summaryParts, ", "))

	return []FactEntry{markerFact, {
		Category: FactCategoryStorage,
		Key:      "snapshots:summary",
		Value:    truncateValue(summaryValue),
	}}
}

// --- pulse_storage: replication ---

func extractReplicationFacts(resultText string) []FactEntry {
	// Response is TEXT-wrapped JSON array
	var jobs []struct {
		ID         string `json:"id"`
		GuestID    int    `json:"guest_id"`
		GuestName  string `json:"guest_name"`
		SourceNode string `json:"source_node"`
		TargetNode string `json:"target_node"`
		Status     string `json:"status"`
		Error      string `json:"error"`
	}
	if err := json.Unmarshal([]byte(resultText), &jobs); err != nil {
		return nil
	}

	markerFact := FactEntry{
		Category: FactCategoryStorage,
		Key:      "replication:queried",
		Value:    fmt.Sprintf("%d jobs", len(jobs)),
	}

	if len(jobs) == 0 {
		return []FactEntry{markerFact}
	}

	errorCount := 0
	var errorNames []string
	for _, j := range jobs {
		if j.Error != "" {
			errorCount++
			name := j.GuestName
			if name == "" {
				name = fmt.Sprintf("%d", j.GuestID)
			}
			errorNames = append(errorNames, name)
		}
	}

	var summaryValue string
	if errorCount == 0 {
		summaryValue = fmt.Sprintf("%d jobs, all ok", len(jobs))
	} else {
		summaryValue = fmt.Sprintf("%d jobs, %d with errors: %s", len(jobs), errorCount, strings.Join(errorNames, ", "))
	}

	return []FactEntry{markerFact, {
		Category: FactCategoryStorage,
		Key:      "replication:summary",
		Value:    truncateValue(summaryValue),
	}}
}

// --- pulse_storage: pbs_jobs ---

func extractPBSJobsFacts(resultText string) []FactEntry {
	var resp struct {
		Instance string `json:"instance"`
		Jobs     []struct {
			ID     string `json:"id"`
			Type   string `json:"type"`
			Store  string `json:"store"`
			Status string `json:"status"`
			Error  string `json:"error"`
		} `json:"jobs"`
		Total int `json:"total"`
	}
	if err := json.Unmarshal([]byte(resultText), &resp); err != nil {
		return nil
	}

	total := resp.Total
	if total == 0 {
		total = len(resp.Jobs)
	}

	markerFact := FactEntry{
		Category: FactCategoryStorage,
		Key:      "pbs_jobs:queried",
		Value:    fmt.Sprintf("%d jobs", total),
	}

	if total == 0 {
		return []FactEntry{markerFact}
	}

	// Count by type and errors
	typeCounts := make(map[string]int)
	errorCount := 0
	for _, j := range resp.Jobs {
		jType := j.Type
		if jType == "" {
			jType = "unknown"
		}
		typeCounts[jType]++
		if j.Error != "" {
			errorCount++
		}
	}

	var typeParts []string
	for jType, cnt := range typeCounts {
		typeParts = append(typeParts, fmt.Sprintf("%d %s", cnt, jType))
	}

	summaryValue := strings.Join(typeParts, ", ")
	if errorCount > 0 {
		summaryValue += fmt.Sprintf("; %d with errors", errorCount)
	}

	return []FactEntry{markerFact, {
		Category: FactCategoryStorage,
		Key:      "pbs_jobs:summary",
		Value:    truncateValue(summaryValue),
	}}
}

// --- pulse_storage: resource_disks ---

func extractResourceDisksFacts(resultText string) []FactEntry {
	var resp struct {
		Resources []struct {
			VMID  int    `json:"vmid"`
			Name  string `json:"name"`
			Type  string `json:"type"`
			Node  string `json:"node"`
			Disks []struct {
				Mountpoint string  `json:"mountpoint"`
				Usage      float64 `json:"usage_percent"`
			} `json:"disks"`
		} `json:"resources"`
		Total int `json:"total"`
	}
	if err := json.Unmarshal([]byte(resultText), &resp); err != nil {
		return nil
	}

	total := resp.Total
	if total == 0 {
		total = len(resp.Resources)
	}

	markerFact := FactEntry{
		Category: FactCategoryStorage,
		Key:      "resource_disks:queried",
		Value:    fmt.Sprintf("%d resources", total),
	}

	if total == 0 {
		return []FactEntry{markerFact}
	}

	totalDisks := 0
	highUsage := 0
	for _, r := range resp.Resources {
		totalDisks += len(r.Disks)
		for _, d := range r.Disks {
			if d.Usage > 80 {
				highUsage++
			}
		}
	}

	summaryValue := fmt.Sprintf("%d resources, %d disks total, %d disks over 80%% usage", total, totalDisks, highUsage)

	return []FactEntry{markerFact, {
		Category: FactCategoryStorage,
		Key:      "resource_disks:summary",
		Value:    truncateValue(summaryValue),
	}}
}

// MarkerExpansions maps marker fact keys to the prefix used to find related per-resource facts.
// Used by the gate to enrich marker-based cache hits with actual data.
var MarkerExpansions = map[string]string{
	// Storage markers
	"storage:pools:queried":  "storage:",
	"disk_health:queried":    "disk_health:",
	"raid:queried":           "raid:",
	"backups:queried":        "backups:",
	"ceph:queried":           "ceph:",
	"ceph_details:queried":   "ceph_details:",
	"snapshots:queried":      "snapshots:",
	"replication:queried":    "replication:",
	"pbs_jobs:queried":       "pbs_jobs:",
	"resource_disks:queried": "resource_disks:",
	"backup_tasks:queried":   "backup:",
	// Metrics markers
	"baselines:queried":      "baseline:",
	"physical_disks:queried": "physical_disks:",
	"temperatures:queried":   "temperatures:",
	// Docker markers
	"docker_services:queried": "docker_services:",
	// Kubernetes markers
	"k8s_clusters:queried": "k8s_cluster:",
	// PMG markers
	"pmg:queried": "pmg:",
	// Alert & finding markers → expand to per-item facts
	"alerts:overview":         "alert:",
	"findings:overview":       "finding:",
	"patrol_findings:queried": "patrol_findings:",
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
