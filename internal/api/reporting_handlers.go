package api

import (
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
	"github.com/rcourtman/pulse-go-rewrite/pkg/reporting"
)

// validResourceID matches safe resource identifiers for filenames
var validResourceID = regexp.MustCompile(`^[a-zA-Z0-9._-]+$`)

// ReportingHandlers handles reporting-related requests
type ReportingHandlers struct {
	mtMonitor *monitoring.MultiTenantMonitor
}

// NewReportingHandlers creates a new ReportingHandlers
func NewReportingHandlers(mtMonitor *monitoring.MultiTenantMonitor) *ReportingHandlers {
	return &ReportingHandlers{
		mtMonitor: mtMonitor,
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

	// Find storage pools for this node
	for _, storage := range state.Storage {
		if storage.Node == node.Name {
			req.Storage = append(req.Storage, reporting.StorageInfo{
				Name:      storage.Name,
				Type:      storage.Type,
				Status:    storage.Status,
				Total:     storage.Total,
				Used:      storage.Used,
				Available: storage.Free,
				UsagePerc: storage.Usage,
				Content:   storage.Content,
			})
		}
	}

	// Find physical disks for this node
	for _, disk := range state.PhysicalDisks {
		if disk.Node == node.Name {
			req.Disks = append(req.Disks, reporting.DiskInfo{
				Device:      disk.DevPath,
				Model:       disk.Model,
				Serial:      disk.Serial,
				Type:        disk.Type,
				Size:        disk.Size,
				Health:      disk.Health,
				Temperature: disk.Temperature,
				WearLevel:   disk.Wearout,
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
