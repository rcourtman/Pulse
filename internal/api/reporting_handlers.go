package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rcourtman/pulse-go-rewrite/pkg/reporting"
)

// validResourceID matches safe resource identifiers (includes colon for guest IDs like "instance:node:vmid")
var validResourceID = regexp.MustCompile(`^[a-zA-Z0-9._:-]+$`)

// ReportingHandlers handles reporting-related requests
type ReportingHandlers struct {
	mtMonitor        *monitoring.MultiTenantMonitor
	resourceRegistry *unifiedresources.ResourceRegistry
}

// NewReportingHandlers creates a new ReportingHandlers
func NewReportingHandlers(mtMonitor *monitoring.MultiTenantMonitor, registry *unifiedresources.ResourceRegistry) *ReportingHandlers {
	return &ReportingHandlers{
		mtMonitor:        mtMonitor,
		resourceRegistry: registry,
	}
}

// HandleGenerateReport generates a report
func (h *ReportingHandlers) HandleGenerateReport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	engine := reporting.GetEngine()
	if engine == nil {
		writeErrorResponse(w, http.StatusInternalServerError, "engine_unavailable", "Reporting engine not initialized", nil)
		return
	}

	q := r.URL.Query()
	format := reporting.ReportFormat(q.Get("format"))
	if format == "" {
		format = reporting.FormatPDF
	}

	// Validate format is one of known values
	if format != reporting.FormatPDF && format != reporting.FormatCSV {
		writeErrorResponse(w, http.StatusBadRequest, "invalid_format", "Format must be 'pdf' or 'csv'", nil)
		return
	}

	resourceType := q.Get("resourceType")
	resourceID := q.Get("resourceId")
	if resourceType == "" || resourceID == "" {
		writeErrorResponse(w, http.StatusBadRequest, "missing_params", "resourceType and resourceId are required", nil)
		return
	}

	// Validate resourceType and resourceID format to prevent injection in filename
	if !validResourceID.MatchString(resourceType) || len(resourceType) > 64 {
		writeErrorResponse(w, http.StatusBadRequest, "invalid_resource_type", "Invalid resourceType format", nil)
		return
	}
	if !validResourceID.MatchString(resourceID) || len(resourceID) > 128 {
		writeErrorResponse(w, http.StatusBadRequest, "invalid_resource_id", "Invalid resourceId format", nil)
		return
	}

	metricType := q.Get("metricType")

	// Parse range
	end := time.Now()
	if q.Get("end") != "" {
		if t, err := time.Parse(time.RFC3339, q.Get("end")); err == nil {
			end = t
		}
	}

	start := end.Add(-24 * time.Hour)
	if q.Get("start") != "" {
		if t, err := time.Parse(time.RFC3339, q.Get("start")); err == nil {
			start = t
		}
	}

	req := reporting.MetricReportRequest{
		ResourceType: resourceType,
		ResourceID:   resourceID,
		MetricType:   metricType,
		Start:        start,
		End:          end,
		Format:       format,
		Title:        q.Get("title"),
	}

	// Enrich with resource data if monitor is available
	if h.mtMonitor != nil {
		orgID := GetOrgID(r.Context())
		if monitor, err := h.mtMonitor.GetMonitor(orgID); err == nil && monitor != nil {
			h.enrichReportRequest(&req, monitor.GetState(), start, end)
		}
	}

	data, contentType, err := engine.Generate(req)
	if err != nil {
		writeErrorResponse(w, http.StatusInternalServerError, "generation_failed", "Failed to generate report", nil)
		return
	}

	w.Header().Set("Content-Type", contentType)
	// Build safe filename - sanitize resourceID to prevent header injection
	safeResourceID := sanitizeFilename(resourceID)
	filename := fmt.Sprintf("report-%s-%s.%s", safeResourceID, time.Now().Format("20060102"), format)
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))
	w.Write(data)
}

// sanitizeFilename removes or replaces characters that could cause issues in filenames or headers
func sanitizeFilename(s string) string {
	// Remove any characters that could break Content-Disposition header
	s = strings.ReplaceAll(s, "\"", "")
	s = strings.ReplaceAll(s, "\\", "")
	s = strings.ReplaceAll(s, "/", "-")
	s = strings.ReplaceAll(s, ":", "-")
	s = strings.ReplaceAll(s, "\r", "")
	s = strings.ReplaceAll(s, "\n", "")

	// Limit length
	if len(s) > 64 {
		s = s[:64]
	}
	return s
}

// enrichReportRequest populates enrichment data from the monitor state
func (h *ReportingHandlers) enrichReportRequest(req *reporting.MetricReportRequest, state models.StateSnapshot, start, end time.Time) {
	switch req.ResourceType {
	case "node":
		h.enrichNodeReport(req, state, start, end)
	case "vm":
		h.enrichVMReport(req, state, start, end)
	case "container":
		h.enrichContainerReport(req, state, start, end)
	}
}

// enrichNodeReport adds node-specific data to the report request
func (h *ReportingHandlers) enrichNodeReport(req *reporting.MetricReportRequest, state models.StateSnapshot, start, end time.Time) {
	// Find the node
	var node *models.Node
	for i := range state.Nodes {
		if state.Nodes[i].ID == req.ResourceID {
			node = &state.Nodes[i]
			break
		}
	}
	if node == nil {
		return
	}

	// Build resource info
	req.Resource = &reporting.ResourceInfo{
		Name:          node.Name,
		DisplayName:   node.DisplayName,
		Status:        node.Status,
		Host:          node.Host,
		Instance:      node.Instance,
		Uptime:        node.Uptime,
		KernelVersion: node.KernelVersion,
		PVEVersion:    node.PVEVersion,
		CPUModel:      node.CPUInfo.Model,
		CPUCores:      node.CPUInfo.Cores,
		CPUSockets:    node.CPUInfo.Sockets,
		MemoryTotal:   node.Memory.Total,
		DiskTotal:     node.Disk.Total,
		LoadAverage:   node.LoadAverage,
		ClusterName:   node.ClusterName,
		IsCluster:     node.IsClusterMember,
	}
	if node.Temperature != nil && node.Temperature.CPUPackage > 0 {
		temp := node.Temperature.CPUPackage
		req.Resource.Temperature = &temp
	}

	// Find alerts for this node
	for _, alert := range state.ActiveAlerts {
		if alert.ResourceID == req.ResourceID || alert.Node == node.Name {
			req.Alerts = append(req.Alerts, reporting.AlertInfo{
				Type:      alert.Type,
				Level:     alert.Level,
				Message:   alert.Message,
				Value:     alert.Value,
				Threshold: alert.Threshold,
				StartTime: alert.StartTime,
			})
		}
	}
	for _, resolved := range state.RecentlyResolved {
		if (resolved.ResourceID == req.ResourceID || resolved.Node == node.Name) &&
			resolved.ResolvedTime.After(start) && resolved.ResolvedTime.Before(end) {
			resolvedTime := resolved.ResolvedTime
			req.Alerts = append(req.Alerts, reporting.AlertInfo{
				Type:         resolved.Type,
				Level:        resolved.Level,
				Message:      resolved.Message,
				Value:        resolved.Value,
				Threshold:    resolved.Threshold,
				StartTime:    resolved.StartTime,
				ResolvedTime: &resolvedTime,
			})
		}
	}

	// Find storage pools for this node via unified resources
	// No fallback: if the registry is nil, skip this section.
	if h.resourceRegistry != nil {
		for _, r := range h.resourceRegistry.List() {
			if r.Type != unifiedresources.ResourceTypeStorage {
				continue
			}
			storageNode := r.ParentName
			if storageNode == "" && len(r.Identity.Hostnames) > 0 {
				storageNode = r.Identity.Hostnames[0]
			}
			if storageNode != node.Name {
				continue
			}

			var total, used, available int64
			var usagePerc float64
			if r.Metrics != nil && r.Metrics.Disk != nil {
				if r.Metrics.Disk.Total != nil {
					total = *r.Metrics.Disk.Total
				}
				if r.Metrics.Disk.Used != nil {
					used = *r.Metrics.Disk.Used
				}
				if total > 0 {
					available = total - used
				}
				usagePerc = r.Metrics.Disk.Percent
				if usagePerc == 0 && total > 0 {
					usagePerc = (float64(used) / float64(total)) * 100
				}
			}

			var storageType, content string
			if r.Storage != nil {
				storageType = r.Storage.Type
				content = r.Storage.Content
			}

			req.Storage = append(req.Storage, reporting.StorageInfo{
				Name:      r.Name,
				Type:      storageType,
				Status:    string(r.Status),
				Total:     total,
				Used:      used,
				Available: available,
				UsagePerc: usagePerc,
				Content:   content,
			})
		}
	}

	// Find physical disks for this node via unified resources
	if h.resourceRegistry != nil {
		for _, r := range h.resourceRegistry.List() {
			if r.Type != unifiedresources.ResourceTypePhysicalDisk || r.PhysicalDisk == nil {
				continue
			}
			diskNode := r.ParentName
			if diskNode == "" && len(r.Identity.Hostnames) > 0 {
				diskNode = r.Identity.Hostnames[0]
			}
			if diskNode != node.Name {
				continue
			}
			pd := r.PhysicalDisk
			req.Disks = append(req.Disks, reporting.DiskInfo{
				Device:      pd.DevPath,
				Model:       pd.Model,
				Serial:      pd.Serial,
				Type:        pd.DiskType,
				Size:        pd.SizeBytes,
				Health:      pd.Health,
				Temperature: pd.Temperature,
				WearLevel:   pd.Wearout,
			})
		}
	}
}

// enrichVMReport adds VM-specific data to the report request
func (h *ReportingHandlers) enrichVMReport(req *reporting.MetricReportRequest, state models.StateSnapshot, start, end time.Time) {
	// Find the VM
	var vm *models.VM
	for i := range state.VMs {
		if state.VMs[i].ID == req.ResourceID {
			vm = &state.VMs[i]
			break
		}
	}
	if vm == nil {
		return
	}

	// Build resource info
	req.Resource = &reporting.ResourceInfo{
		Name:        vm.Name,
		Status:      vm.Status,
		Node:        vm.Node,
		Instance:    vm.Instance,
		Uptime:      vm.Uptime,
		OSName:      vm.OSName,
		OSVersion:   vm.OSVersion,
		IPAddresses: vm.IPAddresses,
		CPUCores:    vm.CPUs,
		MemoryTotal: vm.Memory.Total,
		DiskTotal:   vm.Disk.Total,
		Tags:        vm.Tags,
	}

	// Find alerts for this VM
	for _, alert := range state.ActiveAlerts {
		if alert.ResourceID == req.ResourceID {
			req.Alerts = append(req.Alerts, reporting.AlertInfo{
				Type:      alert.Type,
				Level:     alert.Level,
				Message:   alert.Message,
				Value:     alert.Value,
				Threshold: alert.Threshold,
				StartTime: alert.StartTime,
			})
		}
	}
	for _, resolved := range state.RecentlyResolved {
		if resolved.ResourceID == req.ResourceID &&
			resolved.ResolvedTime.After(start) && resolved.ResolvedTime.Before(end) {
			resolvedTime := resolved.ResolvedTime
			req.Alerts = append(req.Alerts, reporting.AlertInfo{
				Type:         resolved.Type,
				Level:        resolved.Level,
				Message:      resolved.Message,
				Value:        resolved.Value,
				Threshold:    resolved.Threshold,
				StartTime:    resolved.StartTime,
				ResolvedTime: &resolvedTime,
			})
		}
	}

	// Find backups for this VM
	for _, backup := range state.PVEBackups.StorageBackups {
		if backup.VMID == vm.VMID && backup.Node == vm.Node {
			req.Backups = append(req.Backups, reporting.BackupInfo{
				Type:      "vzdump",
				Storage:   backup.Storage,
				Timestamp: backup.Time,
				Size:      backup.Size,
				Protected: backup.Protected,
				VolID:     backup.Volid,
			})
		}
	}
}

// multiReportResourceEntry represents a single resource in a multi-report request body.
type multiReportResourceEntry struct {
	ResourceType string `json:"resourceType"`
	ResourceID   string `json:"resourceId"`
}

// multiReportRequestBody is the JSON body for multi-resource report generation.
type multiReportRequestBody struct {
	Resources  []multiReportResourceEntry `json:"resources"`
	Format     string                     `json:"format"`
	Start      string                     `json:"start"`
	End        string                     `json:"end"`
	Title      string                     `json:"title"`
	MetricType string                     `json:"metricType"`
}

// HandleGenerateMultiReport generates a multi-resource report.
func (h *ReportingHandlers) HandleGenerateMultiReport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	engine := reporting.GetEngine()
	if engine == nil {
		writeErrorResponse(w, http.StatusInternalServerError, "engine_unavailable", "Reporting engine not initialized", nil)
		return
	}

	var body multiReportRequestBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "invalid_body", "Invalid request body", nil)
		return
	}

	// Validate resource count
	if len(body.Resources) == 0 {
		writeErrorResponse(w, http.StatusBadRequest, "no_resources", "At least one resource is required", nil)
		return
	}
	if len(body.Resources) > 50 {
		writeErrorResponse(w, http.StatusBadRequest, "too_many_resources", "Maximum 50 resources allowed", nil)
		return
	}

	// Validate format
	format := reporting.ReportFormat(body.Format)
	if format == "" {
		format = reporting.FormatPDF
	}
	if format != reporting.FormatPDF && format != reporting.FormatCSV {
		writeErrorResponse(w, http.StatusBadRequest, "invalid_format", "Format must be 'pdf' or 'csv'", nil)
		return
	}

	// Parse time range
	end := time.Now()
	if body.End != "" {
		if t, err := time.Parse(time.RFC3339, body.End); err == nil {
			end = t
		}
	}
	start := end.Add(-24 * time.Hour)
	if body.Start != "" {
		if t, err := time.Parse(time.RFC3339, body.Start); err == nil {
			start = t
		}
	}

	// Build multi-report request
	multiReq := reporting.MultiReportRequest{
		Format:     format,
		Start:      start,
		End:        end,
		Title:      body.Title,
		MetricType: body.MetricType,
	}

	// Get monitor state for enrichment
	var state models.StateSnapshot
	var hasState bool
	if h.mtMonitor != nil {
		orgID := GetOrgID(r.Context())
		if monitor, err := h.mtMonitor.GetMonitor(orgID); err == nil && monitor != nil {
			state = monitor.GetState()
			hasState = true
		}
	}

	// Validate and build each resource request
	for _, res := range body.Resources {
		if !validResourceID.MatchString(res.ResourceType) || len(res.ResourceType) > 64 {
			writeErrorResponse(w, http.StatusBadRequest, "invalid_resource_type", fmt.Sprintf("Invalid resourceType: %s", res.ResourceType), nil)
			return
		}
		if !validResourceID.MatchString(res.ResourceID) || len(res.ResourceID) > 128 {
			writeErrorResponse(w, http.StatusBadRequest, "invalid_resource_id", fmt.Sprintf("Invalid resourceId: %s", res.ResourceID), nil)
			return
		}

		req := reporting.MetricReportRequest{
			ResourceType: res.ResourceType,
			ResourceID:   res.ResourceID,
			MetricType:   body.MetricType,
			Start:        start,
			End:          end,
			Format:       format,
			Title:        body.Title,
		}

		// Enrich with resource data
		if hasState {
			h.enrichReportRequest(&req, state, start, end)
		}

		multiReq.Resources = append(multiReq.Resources, req)
	}

	data, contentType, err := engine.GenerateMulti(multiReq)
	if err != nil {
		writeErrorResponse(w, http.StatusInternalServerError, "generation_failed", "Failed to generate multi-resource report", nil)
		return
	}

	w.Header().Set("Content-Type", contentType)
	filename := fmt.Sprintf("fleet-report-%s.%s", time.Now().Format("20060102"), format)
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))
	w.Write(data)
}

// enrichContainerReport adds container-specific data to the report request
func (h *ReportingHandlers) enrichContainerReport(req *reporting.MetricReportRequest, state models.StateSnapshot, start, end time.Time) {
	// Find the container
	var ct *models.Container
	for i := range state.Containers {
		if state.Containers[i].ID == req.ResourceID {
			ct = &state.Containers[i]
			break
		}
	}
	if ct == nil {
		return
	}

	// Build resource info
	req.Resource = &reporting.ResourceInfo{
		Name:        ct.Name,
		Status:      ct.Status,
		Node:        ct.Node,
		Instance:    ct.Instance,
		Uptime:      ct.Uptime,
		OSName:      ct.OSName,
		IPAddresses: ct.IPAddresses,
		CPUCores:    ct.CPUs,
		MemoryTotal: ct.Memory.Total,
		DiskTotal:   ct.Disk.Total,
		Tags:        ct.Tags,
	}

	// Find alerts for this container
	for _, alert := range state.ActiveAlerts {
		if alert.ResourceID == req.ResourceID {
			req.Alerts = append(req.Alerts, reporting.AlertInfo{
				Type:      alert.Type,
				Level:     alert.Level,
				Message:   alert.Message,
				Value:     alert.Value,
				Threshold: alert.Threshold,
				StartTime: alert.StartTime,
			})
		}
	}
	for _, resolved := range state.RecentlyResolved {
		if resolved.ResourceID == req.ResourceID &&
			resolved.ResolvedTime.After(start) && resolved.ResolvedTime.Before(end) {
			resolvedTime := resolved.ResolvedTime
			req.Alerts = append(req.Alerts, reporting.AlertInfo{
				Type:         resolved.Type,
				Level:        resolved.Level,
				Message:      resolved.Message,
				Value:        resolved.Value,
				Threshold:    resolved.Threshold,
				StartTime:    resolved.StartTime,
				ResolvedTime: &resolvedTime,
			})
		}
	}

	// Find backups for this container
	for _, backup := range state.PVEBackups.StorageBackups {
		if backup.VMID == ct.VMID && backup.Node == ct.Node {
			req.Backups = append(req.Backups, reporting.BackupInfo{
				Type:      "vzdump",
				Storage:   backup.Storage,
				Timestamp: backup.Time,
				Size:      backup.Size,
				Protected: backup.Protected,
				VolID:     backup.Volid,
			})
		}
	}
}
