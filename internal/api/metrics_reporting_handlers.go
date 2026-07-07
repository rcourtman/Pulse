package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
	"github.com/rcourtman/pulse-go-rewrite/internal/recovery"
	recoverymanager "github.com/rcourtman/pulse-go-rewrite/internal/recovery/manager"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rcourtman/pulse-go-rewrite/pkg/reporting"
	"github.com/rs/zerolog/log"
)

// rangeLabel maps a [start, end) window to one of the operator-facing
// catalog ranges (24h / 7d / 30d) when the duration matches within an
// hour, falling back to a freeform "<hours>h" string otherwise. The
// label is logged on every report generation so usage telemetry can be
// grouped without parsing the timestamps.
func rangeLabel(start, end time.Time) string {
	if start.IsZero() || end.IsZero() || !end.After(start) {
		return ""
	}
	duration := end.Sub(start)
	switch {
	case absDuration(duration-24*time.Hour) <= time.Hour:
		return "24h"
	case absDuration(duration-7*24*time.Hour) <= time.Hour:
		return "7d"
	case absDuration(duration-30*24*time.Hour) <= time.Hour:
		return "30d"
	default:
		return fmt.Sprintf("%dh", int(duration.Hours()))
	}
}

func absDuration(d time.Duration) time.Duration {
	if d < 0 {
		return -d
	}
	return d
}

// validResourceID matches safe resource identifiers (includes colon for guest IDs like "instance:node:vmid")
var validResourceID = regexp.MustCompile(`^[a-zA-Z0-9._:-]+$`)
var validReportingMetricType = regexp.MustCompile(`^[a-zA-Z0-9._:-]+$`)

const (
	reportingMaxMetricTypeLength = 64
	reportingMaxTitleLength      = 256
	reportingMaxRangeDuration    = 366 * 24 * time.Hour
	reportingMultiReportBodyMax  = 1 << 20
)

type reportingValidationError struct {
	code    string
	message string
}

func (e *reportingValidationError) Error() string {
	if e == nil {
		return ""
	}
	return e.message
}

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
	mtMonitor        *monitoring.MultiTenantMonitor
	recoveryManager  *recoverymanager.Manager
	narratorResolver func(ctx context.Context) (reporting.Narrator, reporting.FleetNarrator, reporting.FindingsProvider)
	settingsStore    reportingSystemSettingsStore
	scheduleRunMu    sync.Mutex
}

type reportingSystemSettingsStore interface {
	LoadSystemSettings() (*config.SystemSettings, error)
}

// SetNarratorResolver wires an optional resolver that returns the
// per-tenant AI narrator, fleet narrator, and Patrol findings provider
// for a request. When unset, or when the resolver returns nil, reports
// use the deterministic heuristic narrators and skip findings
// enrichment.
func (h *ReportingHandlers) SetNarratorResolver(resolver func(ctx context.Context) (reporting.Narrator, reporting.FleetNarrator, reporting.FindingsProvider)) {
	if h == nil {
		return
	}
	h.narratorResolver = resolver
}

func (h *ReportingHandlers) SetSystemSettingsStore(store reportingSystemSettingsStore) {
	if h == nil {
		return
	}
	h.settingsStore = store
}

func (h *ReportingHandlers) resolveNarrator(ctx context.Context) (reporting.Narrator, reporting.FleetNarrator, reporting.FindingsProvider) {
	if h == nil || h.narratorResolver == nil {
		return nil, nil, nil
	}
	return h.narratorResolver(ctx)
}

func (h *ReportingHandlers) resolveReportBranding(ctx context.Context) reporting.ReportBranding {
	service := getLicenseServiceForContext(ctx)
	entitled := service != nil && service.HasFeature(featureWhiteLabelValue)

	branding := reporting.ReportBranding{
		Entitled:        entitled,
		ProviderDefault: reportBrandFromEnv(),
	}
	if h == nil || h.settingsStore == nil {
		return branding
	}
	settings, err := h.settingsStore.LoadSystemSettings()
	if err != nil || settings == nil {
		return branding
	}
	if settings.ReportBranding != nil {
		branding.WorkspaceOverride = reportBrandFromWorkspaceSettings(*settings.ReportBranding)
	}
	return branding
}

func reportBrandFromEnv() reporting.ReportBrand {
	return reportBrandFromSettings(config.ReportBrandSettings{
		DisplayName: os.Getenv("PULSE_REPORT_PROVIDER_BRAND_DISPLAY_NAME"),
		LogoPath:    os.Getenv("PULSE_REPORT_PROVIDER_BRAND_LOGO_PATH"),
		LogoBase64:  os.Getenv("PULSE_REPORT_PROVIDER_BRAND_LOGO_BASE64"),
		LogoFormat:  os.Getenv("PULSE_REPORT_PROVIDER_BRAND_LOGO_FORMAT"),
	})
}

func reportBrandFromWorkspaceSettings(settings config.ReportBrandSettings) reporting.ReportBrand {
	brand := reportBrandFromSettings(settings)
	brand.LogoPath = ""
	return brand
}

func reportBrandFromSettings(settings config.ReportBrandSettings) reporting.ReportBrand {
	logoData, err := config.DecodeReportBrandLogoBase64(settings.LogoBase64)
	if err != nil {
		log.Warn().Err(err).Msg("Ignoring invalid report brand logoBase64")
	}
	format, ok := config.CanonicalReportBrandLogoFormat(settings.LogoFormat)
	if !ok {
		format = ""
	}
	return reporting.ReportBrand{
		DisplayName: strings.TrimSpace(settings.DisplayName),
		LogoPath:    strings.TrimSpace(settings.LogoPath),
		LogoData:    logoData,
		LogoFormat:  format,
	}
}

func performanceReportDefinition() reporting.PerformanceReportDefinition {
	return reporting.DescribePerformanceReport()
}

func normalizePerformanceReportFormat(raw string, definition reporting.PerformanceReportDefinition) (reporting.ReportFormat, error) {
	format := reporting.ReportFormat(strings.TrimSpace(raw))
	if format == "" {
		format = definition.DefaultFormat
	}
	if !definition.SupportsFormat(format) {
		return "", errors.New(definition.InvalidFormatError())
	}
	return format, nil
}

func normalizePerformanceReportOptionalFields(
	definition reporting.PerformanceReportDefinition,
	metricType string,
	title string,
) (string, string, error) {
	normalizedMetricType := strings.TrimSpace(metricType)
	normalizedTitle := strings.TrimSpace(title)

	if !definition.SupportsMetricFilter {
		normalizedMetricType = ""
	} else if normalizedMetricType != "" {
		if len(normalizedMetricType) > reportingMaxMetricTypeLength || !validReportingMetricType.MatchString(normalizedMetricType) {
			return "", "", &reportingValidationError{
				code:    "invalid_metric_type",
				message: fmt.Sprintf("metricType must match [a-zA-Z0-9._:-]+ and be <= %d chars", reportingMaxMetricTypeLength),
			}
		}
	}
	if !definition.SupportsCustomTitle {
		normalizedTitle = ""
	} else if len(normalizedTitle) > reportingMaxTitleLength {
		return "", "", &reportingValidationError{
			code:    "invalid_title",
			message: fmt.Sprintf("title must be <= %d chars", reportingMaxTitleLength),
		}
	}

	return normalizedMetricType, normalizedTitle, nil
}

func normalizePerformanceReportTimeRange(
	definition reporting.PerformanceReportDefinition,
	startRaw string,
	endRaw string,
	now time.Time,
) (time.Time, time.Time, error) {
	end := now
	if strings.TrimSpace(endRaw) != "" {
		parsed, err := time.Parse(time.RFC3339, endRaw)
		if err != nil {
			return time.Time{}, time.Time{}, fmt.Errorf("end must be RFC3339")
		}
		end = parsed
	}

	start := end.Add(-definition.DefaultRangeDuration())
	if strings.TrimSpace(startRaw) != "" {
		parsed, err := time.Parse(time.RFC3339, startRaw)
		if err != nil {
			return time.Time{}, time.Time{}, fmt.Errorf("start must be RFC3339")
		}
		start = parsed
	}

	if !end.After(start) {
		return time.Time{}, time.Time{}, fmt.Errorf("end must be after start")
	}
	if end.Sub(start) > reportingMaxRangeDuration {
		return time.Time{}, time.Time{}, fmt.Errorf("report window must be %d days or less", int(reportingMaxRangeDuration/(24*time.Hour)))
	}

	return start, end, nil
}

func decodeMultiReportRequestBody(w http.ResponseWriter, r *http.Request) (multiReportRequestBody, bool) {
	r.Body = http.MaxBytesReader(w, r.Body, reportingMultiReportBodyMax)

	var body multiReportRequestBody
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&body); err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			writeErrorResponse(w, http.StatusBadRequest, "body_too_large", "Request body must be 1MB or less", nil)
			return multiReportRequestBody{}, false
		}
		writeErrorResponse(w, http.StatusBadRequest, "invalid_body", "Invalid request body", nil)
		return multiReportRequestBody{}, false
	}
	if decoder.More() {
		writeErrorResponse(w, http.StatusBadRequest, "invalid_body", "Invalid request body", nil)
		return multiReportRequestBody{}, false
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		writeErrorResponse(w, http.StatusBadRequest, "invalid_body", "Invalid request body", nil)
		return multiReportRequestBody{}, false
	}

	return body, true
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

func emptyReportingEnrichmentSnapshot() reportingEnrichmentSnapshot {
	snapshot := reportingEnrichmentSnapshot{}
	snapshot.normalizeCollections()
	return snapshot
}

func (s *reportingEnrichmentSnapshot) normalizeCollections() {
	if s.Nodes == nil {
		s.Nodes = []models.Node{}
	}
	if s.VMs == nil {
		s.VMs = []models.VM{}
	}
	if s.Containers == nil {
		s.Containers = []models.Container{}
	}
	if s.ActiveAlerts == nil {
		s.ActiveAlerts = []models.Alert{}
	}
	if s.RecentlyResolved == nil {
		s.RecentlyResolved = []models.ResolvedAlert{}
	}
	if s.LegacyBackups.BackupTasks == nil {
		s.LegacyBackups.BackupTasks = []models.BackupTask{}
	}
	if s.LegacyBackups.StorageBackups == nil {
		s.LegacyBackups.StorageBackups = []models.StorageBackup{}
	}
	if s.LegacyBackups.GuestSnapshots == nil {
		s.LegacyBackups.GuestSnapshots = []models.GuestSnapshot{}
	}
	if s.Resources == nil {
		s.Resources = []unifiedresources.Resource{}
	}
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
		return emptyReportingEnrichmentSnapshot(), false
	}

	monitor, err := h.mtMonitor.GetMonitor(orgID)
	if err != nil || monitor == nil {
		return emptyReportingEnrichmentSnapshot(), false
	}

	// Resources must come from the canonical unified view (the same
	// re-ingested registry the UI and /api/state read), NOT the raw
	// resource store: the raw store's canonical IDs depend on per-boot
	// ingest order for merged-source hosts, so a UI-supplied resource ID
	// can be missing from it entirely.
	unifiedResources, _ := monitor.UnifiedResourceSnapshot()
	snapshot := reportingEnrichmentSnapshot{
		Nodes:            monitor.NodesSnapshot(),
		VMs:              monitor.VMsSnapshot(),
		Containers:       monitor.ContainersSnapshot(),
		ActiveAlerts:     monitor.ActiveAlertsSnapshot(),
		RecentlyResolved: monitor.RecentlyResolvedSnapshot(),
		LegacyBackups:    monitor.PVEBackupsSnapshot(),
		Resources:        unifiedResources,
	}
	snapshot.normalizeCollections()
	return snapshot, true
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

	definition := performanceReportDefinition()
	q := r.URL.Query()
	format, err := normalizePerformanceReportFormat(q.Get("format"), definition)
	if err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "invalid_format", err.Error(), nil)
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

	metricType, title, err := normalizePerformanceReportOptionalFields(
		definition,
		q.Get("metricType"),
		q.Get("title"),
	)
	if err != nil {
		code := "invalid_metric_type"
		var validationErr *reportingValidationError
		if errors.As(err, &validationErr) && validationErr.code != "" {
			code = validationErr.code
		}
		writeErrorResponse(w, http.StatusBadRequest, code, err.Error(), nil)
		return
	}

	start, end, err := normalizePerformanceReportTimeRange(
		definition,
		q.Get("start"),
		q.Get("end"),
		time.Now(),
	)
	if err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "invalid_time_range", err.Error(), nil)
		return
	}

	req := reporting.MetricReportRequest{
		ResourceType: resourceType,
		ResourceID:   resourceID,
		MetricType:   metricType,
		Start:        start,
		End:          end,
		Format:       format,
		Title:        title,
		Branding:     h.resolveReportBranding(r.Context()),
	}

	// Enrich with resource data if monitor is available
	orgID := GetOrgID(r.Context())
	if snapshot, ok := h.getReportingEnrichmentSnapshot(r.Context(), orgID); ok {
		h.enrichReportRequest(r.Context(), orgID, &req, snapshot, start, end)
	}

	// Wire the per-tenant AI narrator and Patrol findings provider when
	// configured. Both are nil-safe at the engine layer; absence falls
	// back to the heuristic narrator with no findings section. Single-
	// resource reports do not use the FleetNarrator.
	narrator, _, findings := h.resolveNarrator(r.Context())
	req.Narrator = narrator
	req.FindingsProvider = findings

	data, contentType, err := engine.Generate(req)
	if err != nil {
		writeErrorResponse(w, http.StatusInternalServerError, "generation_failed", "Failed to generate report", nil)
		return
	}

	// Telemetry: structured event line per generation so usage can be
	// audited without parsing handler timing or response bodies. Logged
	// at info so it surfaces in default-level operator logs and so an
	// agent grepping transcripts can group by event/org/format/range
	// without needing a separate metrics pipeline.
	log.Info().
		Str("event", "reporting.single.generated").
		Str("org_id", orgID).
		Str("resource_type", resourceType).
		Str("metric_type", metricType).
		Str("format", string(format)).
		Str("range", rangeLabel(start, end)).
		Bool("ai_configured", narrator != nil).
		Bool("findings_configured", findings != nil).
		Int("bytes", len(data)).
		Time("window_start", start).
		Time("window_end", end).
		Msg("Reporting: single-resource report generated")

	w.Header().Set("Content-Type", contentType)
	// Build safe filename - sanitize resourceID to prevent header injection
	safeResourceID := sanitizeFilename(resourceID)
	filename := definition.SingleAttachmentFilename(safeResourceID, time.Now().UTC(), format)
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
	h.resolveReportSubject(orgID, req, snapshot)
	h.resolveReportAvailability(orgID, req, snapshot, start, end)
	switch req.ResourceType {
	case "node":
		h.enrichNodeReport(req, snapshot, start, end)
	case "vm":
		h.enrichVMReport(ctx, orgID, req, snapshot, start, end)
	case "system-container", "oci-container":
		h.enrichContainerReport(ctx, orgID, req, snapshot, start, end)
	}
}

// resolveReportSubject maps the request's canonical unified-resource ID onto
// the metrics-store ID space and seeds a display name. The v6 UI and API
// callers address resources by unified ID (`vm-<hash>`), while the metrics
// store is keyed by each platform's native source ID (the resource's
// metricsTarget) — without this translation the store query silently
// returns zero points. Recovery points and Patrol findings stay keyed by
// the unified ID, so the translation rides MetricsResourceID instead of
// rewriting ResourceID.
func (h *ReportingHandlers) resolveReportSubject(orgID string, req *reporting.MetricReportRequest, snapshot reportingEnrichmentSnapshot) {
	if req == nil {
		return
	}
	for i := range snapshot.Resources {
		resource := &snapshot.Resources[i]
		if resource.ID != req.ResourceID {
			continue
		}
		// The registry computes metrics targets on demand; the Resource
		// structs in the snapshot only carry one when a fixture populated
		// it explicitly, so ask the tenant monitor first and fall back to
		// the struct field.
		var target *unifiedresources.MetricsTarget
		if h != nil && h.mtMonitor != nil {
			if monitor, err := h.mtMonitor.GetMonitor(orgID); err == nil && monitor != nil {
				target = monitor.MetricsTargetForResource(req.ResourceID)
			}
		}
		if target == nil {
			target = resource.MetricsTarget
		}
		if target != nil {
			if targetID := strings.TrimSpace(target.ResourceID); targetID != "" {
				req.MetricsResourceID = targetID
			}
		}
		if req.Resource == nil && strings.TrimSpace(resource.Name) != "" {
			req.Resource = &reporting.ResourceInfo{
				Name:   resource.Name,
				Status: string(resource.Status),
			}
		}
		return
	}
}

// reportSubjectIDMatches reports whether a legacy state-model ID refers to
// the report subject. Legacy snapshots (and the alerts raised on them) are
// keyed by the metrics-target ID, while the request carries the canonical
// unified ID, so both are accepted.
func reportSubjectIDMatches(req *reporting.MetricReportRequest, candidateID string) bool {
	if req == nil || candidateID == "" {
		return false
	}
	if candidateID == req.ResourceID {
		return true
	}
	return req.MetricsResourceID != "" && candidateID == req.MetricsResourceID
}

// enrichNodeReport adds node-specific data to the report request
func (h *ReportingHandlers) enrichNodeReport(req *reporting.MetricReportRequest, snapshot reportingEnrichmentSnapshot, start, end time.Time) {
	// Find the node
	var node *models.Node
	for i := range snapshot.Nodes {
		if reportSubjectIDMatches(req, snapshot.Nodes[i].ID) {
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
		if reportSubjectIDMatches(req, alert.ResourceID) || alert.Node == node.Name {
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
		if (reportSubjectIDMatches(req, resolved.ResourceID) || resolved.Node == node.Name) &&
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
		if reportSubjectIDMatches(req, snapshot.VMs[i].ID) {
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
		if reportSubjectIDMatches(req, alert.ResourceID) {
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
		if reportSubjectIDMatches(req, resolved.ResourceID) &&
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

type generatedMultiReport struct {
	Data          []byte
	ContentType   string
	Filename      string
	Format        reporting.ReportFormat
	Start         time.Time
	End           time.Time
	ResourceCount int
}

type reportingRequestError struct {
	status  int
	code    string
	message string
}

func (e *reportingRequestError) Error() string {
	if e == nil {
		return ""
	}
	return e.message
}

func writeReportingRequestError(w http.ResponseWriter, err error) bool {
	var reqErr *reportingRequestError
	if !errors.As(err, &reqErr) {
		return false
	}
	writeErrorResponse(w, reqErr.status, reqErr.code, reqErr.message, nil)
	return true
}

func (h *ReportingHandlers) generateMultiReportFromBody(ctx context.Context, body multiReportRequestBody, now time.Time) (generatedMultiReport, error) {
	engine := reporting.GetEngine()
	if engine == nil {
		return generatedMultiReport{}, &reportingRequestError{
			status:  http.StatusInternalServerError,
			code:    "engine_unavailable",
			message: "Reporting engine not initialized",
		}
	}

	definition := performanceReportDefinition()
	if len(body.Resources) == 0 {
		return generatedMultiReport{}, &reportingRequestError{
			status:  http.StatusBadRequest,
			code:    "no_resources",
			message: "At least one resource is required",
		}
	}
	if len(body.Resources) > definition.MultiResourceMax {
		return generatedMultiReport{}, &reportingRequestError{
			status:  http.StatusBadRequest,
			code:    "too_many_resources",
			message: fmt.Sprintf("Maximum %d resources allowed", definition.MultiResourceMax),
		}
	}

	format, err := normalizePerformanceReportFormat(body.Format, definition)
	if err != nil {
		return generatedMultiReport{}, &reportingRequestError{status: http.StatusBadRequest, code: "invalid_format", message: err.Error()}
	}
	metricType, title, err := normalizePerformanceReportOptionalFields(definition, body.MetricType, body.Title)
	if err != nil {
		code := "invalid_metric_type"
		var validationErr *reportingValidationError
		if errors.As(err, &validationErr) && validationErr.code != "" {
			code = validationErr.code
		}
		return generatedMultiReport{}, &reportingRequestError{status: http.StatusBadRequest, code: code, message: err.Error()}
	}

	start, end, err := normalizePerformanceReportTimeRange(definition, body.Start, body.End, now)
	if err != nil {
		return generatedMultiReport{}, &reportingRequestError{status: http.StatusBadRequest, code: "invalid_time_range", message: err.Error()}
	}

	multiReq := reporting.MultiReportRequest{
		Format:     format,
		Start:      start,
		End:        end,
		Title:      title,
		MetricType: metricType,
		Branding:   h.resolveReportBranding(ctx),
	}

	orgID := GetOrgID(ctx)
	snapshot, hasSnapshot := h.getReportingEnrichmentSnapshot(ctx, orgID)
	for _, res := range body.Resources {
		if !validResourceID.MatchString(res.ResourceType) || len(res.ResourceType) > 64 {
			return generatedMultiReport{}, &reportingRequestError{
				status:  http.StatusBadRequest,
				code:    "invalid_resource_type",
				message: fmt.Sprintf("Invalid resourceType: %s", res.ResourceType),
			}
		}
		if !validResourceID.MatchString(res.ResourceID) || len(res.ResourceID) > 128 {
			return generatedMultiReport{}, &reportingRequestError{
				status:  http.StatusBadRequest,
				code:    "invalid_resource_id",
				message: fmt.Sprintf("Invalid resourceId: %s", res.ResourceID),
			}
		}
		resourceType, err := normalizeReportResourceType(res.ResourceType)
		if err != nil {
			return generatedMultiReport{}, &reportingRequestError{status: http.StatusBadRequest, code: "invalid_resource_type", message: err.Error()}
		}

		req := reporting.MetricReportRequest{
			ResourceType: resourceType,
			ResourceID:   res.ResourceID,
			MetricType:   metricType,
			Start:        start,
			End:          end,
			Format:       format,
			Title:        title,
			Branding:     multiReq.Branding,
		}
		if hasSnapshot {
			h.enrichReportRequest(ctx, orgID, &req, snapshot, start, end)
		}
		multiReq.Resources = append(multiReq.Resources, req)
	}

	_, fleetNarrator, findings := h.resolveNarrator(ctx)
	multiReq.FleetNarrator = fleetNarrator
	multiReq.FindingsProvider = findings

	data, contentType, err := engine.GenerateMulti(multiReq)
	if err != nil {
		return generatedMultiReport{}, &reportingRequestError{
			status:  http.StatusInternalServerError,
			code:    "generation_failed",
			message: "Failed to generate multi-resource report",
		}
	}

	log.Info().
		Str("event", "reporting.fleet.generated").
		Str("org_id", orgID).
		Str("format", string(format)).
		Str("range", rangeLabel(start, end)).
		Int("resource_count", len(multiReq.Resources)).
		Bool("ai_configured", fleetNarrator != nil).
		Bool("findings_configured", findings != nil).
		Int("bytes", len(data)).
		Time("window_start", start).
		Time("window_end", end).
		Msg("Reporting: fleet report generated")

	return generatedMultiReport{
		Data:          data,
		ContentType:   contentType,
		Filename:      definition.MultiAttachmentFilename(now.UTC(), format),
		Format:        format,
		Start:         start,
		End:           end,
		ResourceCount: len(multiReq.Resources),
	}, nil
}

// HandleGenerateMultiReport generates a multi-resource report.
func (h *ReportingHandlers) HandleGenerateMultiReport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	body, ok := decodeMultiReportRequestBody(w, r)
	if !ok {
		return
	}

	report, err := h.generateMultiReportFromBody(r.Context(), body, time.Now())
	if err != nil {
		if !writeReportingRequestError(w, err) {
			writeErrorResponse(w, http.StatusInternalServerError, "generation_failed", "Failed to generate multi-resource report", nil)
		}
		return
	}

	w.Header().Set("Content-Type", report.ContentType)
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", report.Filename))
	w.Write(report.Data)
}

// enrichContainerReport adds container-specific data to the report request
func (h *ReportingHandlers) enrichContainerReport(ctx context.Context, orgID string, req *reporting.MetricReportRequest, snapshot reportingEnrichmentSnapshot, start, end time.Time) {
	// Find the container
	var ct *models.Container
	for i := range snapshot.Containers {
		if reportSubjectIDMatches(req, snapshot.Containers[i].ID) {
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
		if reportSubjectIDMatches(req, alert.ResourceID) {
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
		if reportSubjectIDMatches(req, resolved.ResourceID) &&
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
