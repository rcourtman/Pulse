package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
	"github.com/rcourtman/pulse-go-rewrite/internal/recovery"
	recoverymanager "github.com/rcourtman/pulse-go-rewrite/internal/recovery/manager"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rcourtman/pulse-go-rewrite/pkg/reporting"
)

// validResourceID matches safe resource identifiers (includes colon for guest IDs like "instance:node:vmid")
var validResourceID = regexp.MustCompile(`^[a-zA-Z0-9._:-]+$`)

func normalizeReportResourceType(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", fmt.Errorf("resourceType is required")
	}
	normalized := strings.ToLower(trimmed)
	canonical := reporting.CanonicalResourceType(normalized)
	if canonical == "" {
		return "", fmt.Errorf("unsupported resourceType %q", trimmed)
	}
	if normalized != canonical {
		return "", fmt.Errorf("unsupported resourceType %q", trimmed)
	}
	return canonical, nil
}

// ReportingHandlers handles reporting-related requests
type ReportingHandlers struct {
	mtMonitor       *monitoring.MultiTenantMonitor
	recoveryManager *recoverymanager.Manager
}

type reportingEnrichmentSnapshot struct {
	Nodes            []models.Node
	VMs              []models.VM
	Containers       []models.Container
	ActiveAlerts     []models.Alert
	RecentlyResolved []models.ResolvedAlert
	LegacyBackups    models.PVEBackups
	Resources        []unifiedresources.Resource
}

// NewReportingHandlers creates a new ReportingHandlers
func NewReportingHandlers(mtMonitor *monitoring.MultiTenantMonitor, recoveryManager *recoverymanager.Manager) *ReportingHandlers {
	return &ReportingHandlers{
		mtMonitor:       mtMonitor,
		recoveryManager: recoveryManager,
	}
}

func (h *ReportingHandlers) getReportingEnrichmentSnapshot(ctx context.Context, orgID string) (reportingEnrichmentSnapshot, bool) {
	_ = ctx
	if h == nil || h.mtMonitor == nil {
		return reportingEnrichmentSnapshot{}, false
	}

	monitor, err := h.mtMonitor.GetMonitor(orgID)
	if err != nil || monitor == nil {
		return reportingEnrichmentSnapshot{}, false
	}

	return reportingEnrichmentSnapshot{
		Nodes:            monitor.NodesSnapshot(),
		VMs:              monitor.VMsSnapshot(),
		Containers:       monitor.ContainersSnapshot(),
		ActiveAlerts:     monitor.ActiveAlertsSnapshot(),
		RecentlyResolved: monitor.RecentlyResolvedSnapshot(),
		LegacyBackups:    monitor.PVEBackupsSnapshot(),
		Resources:        monitor.GetUnifiedResources(),
	}, true
}

func (h *ReportingHandlers) listBackupsForReport(ctx context.Context, orgID string, subjectResourceID string, start, end time.Time) []reporting.BackupInfo {
	if h == nil || h.recoveryManager == nil {
		return nil
	}
	if strings.TrimSpace(orgID) == "" {
		orgID = "default"
	}
	subjectResourceID = strings.TrimSpace(subjectResourceID)
	if subjectResourceID == "" {
		return nil
	}

	store, err := h.recoveryManager.StoreForOrg(orgID)
	if err != nil {
		return nil
	}

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	from := start.UTC()
	to := end.UTC()

	// Page through results to avoid silently truncating backups in reports.
	const limit = 500
	opts := recovery.ListPointsOptions{
		SubjectResourceID: subjectResourceID,
		From:              &from,
		To:                &to,
		Page:              1,
		Limit:             limit,
	}

	points, total, err := store.ListPoints(ctx, opts)
	if err != nil {
		return nil
	}

	totalPages := 1
	if limit > 0 {
		totalPages = (total + limit - 1) / limit
		if totalPages < 1 {
			totalPages = 1
		}
	}

	all := make([]recovery.RecoveryPoint, 0, min(total, limit*totalPages))
	all = append(all, points...)

	for page := 2; page <= totalPages; page++ {
		opts.Page = page
		more, _, err := store.ListPoints(ctx, opts)
		if err != nil {
			break
		}
		if len(more) == 0 {
			break
		}
		all = append(all, more...)
	}

	out := make([]reporting.BackupInfo, 0, len(all))
	for _, p := range all {
		if p.Kind != recovery.KindBackup {
			continue
		}

		var ts time.Time
		if p.CompletedAt != nil && !p.CompletedAt.IsZero() {
			ts = p.CompletedAt.UTC()
		} else if p.StartedAt != nil && !p.StartedAt.IsZero() {
			ts = p.StartedAt.UTC()
		} else {
			continue
		}
		if ts.Before(from) || ts.After(to) {
			continue
		}

		typ := "backup"
		if p.Provider == recovery.ProviderProxmoxPBS || p.Mode == recovery.ModeRemote {
			typ = "pbs"
		} else if p.Provider == recovery.ProviderProxmoxPVE {
			typ = "vzdump"
		}

		getStringDetail := func(key string) string {
			if p.Details == nil {
				return ""
			}
			if v, ok := p.Details[key]; ok {
				if s, ok := v.(string); ok {
					return strings.TrimSpace(s)
				}
			}
			return ""
		}

		storage := getStringDetail("storage")
		if storage == "" && p.Provider == recovery.ProviderProxmoxPBS {
			storage = getStringDetail("datastore")
		}

		var size int64
		if p.SizeBytes != nil && *p.SizeBytes > 0 {
			size = *p.SizeBytes
		}
		verified := p.Verified != nil && *p.Verified
		protected := p.Immutable != nil && *p.Immutable

		out = append(out, reporting.BackupInfo{
			Type:      typ,
			Storage:   storage,
			Timestamp: ts,
			Size:      size,
			Verified:  verified,
			Protected: protected,
			VolID:     getStringDetail("volid"),
		})
	}

	return out
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

	resourceTypeRaw := q.Get("resourceType")
	resourceID := q.Get("resourceId")
	if resourceTypeRaw == "" || resourceID == "" {
		writeErrorResponse(w, http.StatusBadRequest, "missing_params", "resourceType and resourceId are required", nil)
		return
	}

	// Validate resourceType and resourceID format to prevent injection in filename
	if !validResourceID.MatchString(resourceTypeRaw) || len(resourceTypeRaw) > 64 {
		writeErrorResponse(w, http.StatusBadRequest, "invalid_resource_type", "Invalid resourceType format", nil)
		return
	}
	if !validResourceID.MatchString(resourceID) || len(resourceID) > 128 {
		writeErrorResponse(w, http.StatusBadRequest, "invalid_resource_id", "Invalid resourceId format", nil)
		return
	}
	resourceType, err := normalizeReportResourceType(resourceTypeRaw)
	if err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "invalid_resource_type", err.Error(), nil)
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
	orgID := GetOrgID(r.Context())
	if snapshot, ok := h.getReportingEnrichmentSnapshot(r.Context(), orgID); ok {
		h.enrichReportRequest(r.Context(), orgID, &req, snapshot, start, end)
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
func (h *ReportingHandlers) enrichReportRequest(ctx context.Context, orgID string, req *reporting.MetricReportRequest, snapshot reportingEnrichmentSnapshot, start, end time.Time) {
	switch req.ResourceType {
	case "node":
		h.enrichNodeReport(req, snapshot, start, end)
	case "vm":
		h.enrichVMReport(ctx, orgID, req, snapshot, start, end)
	case "system-container", "oci-container":
		h.enrichContainerReport(ctx, orgID, req, snapshot, start, end)
	}
}

// enrichNodeReport adds node-specific data to the report request
func (h *ReportingHandlers) enrichNodeReport(req *reporting.MetricReportRequest, snapshot reportingEnrichmentSnapshot, start, end time.Time) {
	// Find the node
	var node *models.Node
	for i := range snapshot.Nodes {
		if snapshot.Nodes[i].ID == req.ResourceID {
			node = &snapshot.Nodes[i]
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
	for _, alert := range snapshot.ActiveAlerts {
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
	for _, resolved := range snapshot.RecentlyResolved {
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

	// Find storage pools and physical disks for this node via unified resources.
	for _, r := range snapshot.Resources {
		switch r.Type {
		case unifiedresources.ResourceTypeStorage:
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
		case unifiedresources.ResourceTypePhysicalDisk:
			if r.PhysicalDisk == nil {
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
func (h *ReportingHandlers) enrichVMReport(ctx context.Context, orgID string, req *reporting.MetricReportRequest, snapshot reportingEnrichmentSnapshot, start, end time.Time) {
	// Find the VM
	var vm *models.VM
	for i := range snapshot.VMs {
		if snapshot.VMs[i].ID == req.ResourceID {
			vm = &snapshot.VMs[i]
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
	for _, alert := range snapshot.ActiveAlerts {
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
	for _, resolved := range snapshot.RecentlyResolved {
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

	// Backups are sourced from the recovery store so the report stays platform-agnostic.
	// Fall back to legacy state backups only when the recovery manager isn't configured.
	if backups := h.listBackupsForReport(ctx, orgID, req.ResourceID, start, end); len(backups) > 0 {
		req.Backups = append(req.Backups, backups...)
	} else {
		for _, backup := range snapshot.LegacyBackups.StorageBackups {
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
	orgID := GetOrgID(r.Context())
	snapshot, hasSnapshot := h.getReportingEnrichmentSnapshot(r.Context(), orgID)

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
		resourceType, err := normalizeReportResourceType(res.ResourceType)
		if err != nil {
			writeErrorResponse(w, http.StatusBadRequest, "invalid_resource_type", err.Error(), nil)
			return
		}

		req := reporting.MetricReportRequest{
			ResourceType: resourceType,
			ResourceID:   res.ResourceID,
			MetricType:   body.MetricType,
			Start:        start,
			End:          end,
			Format:       format,
			Title:        body.Title,
		}

		// Enrich with resource data
		if hasSnapshot {
			h.enrichReportRequest(r.Context(), orgID, &req, snapshot, start, end)
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
func (h *ReportingHandlers) enrichContainerReport(ctx context.Context, orgID string, req *reporting.MetricReportRequest, snapshot reportingEnrichmentSnapshot, start, end time.Time) {
	// Find the container
	var ct *models.Container
	for i := range snapshot.Containers {
		if snapshot.Containers[i].ID == req.ResourceID {
			ct = &snapshot.Containers[i]
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
	for _, alert := range snapshot.ActiveAlerts {
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
	for _, resolved := range snapshot.RecentlyResolved {
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

	// Backups are sourced from the recovery store so the report stays platform-agnostic.
	// Fall back to legacy state backups only when the recovery manager isn't configured.
	if backups := h.listBackupsForReport(ctx, orgID, req.ResourceID, start, end); len(backups) > 0 {
		req.Backups = append(req.Backups, backups...)
	} else {
		for _, backup := range snapshot.LegacyBackups.StorageBackups {
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
}
