package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/notifications"
	"github.com/rcourtman/pulse-go-rewrite/internal/securityutil"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rcourtman/pulse-go-rewrite/pkg/reporting"
	"github.com/rs/zerolog/log"
)

const (
	reportScheduleAttachmentLimitBytes = 15 * 1024 * 1024
	reportScheduleTickInterval         = time.Minute
	reportScheduleMissedRunGrace       = 24 * time.Hour
)

type reportScheduleListResponse struct {
	Schedules []config.ReportSchedule `json:"schedules"`
}

type reportScheduleRunResponse struct {
	Schedule config.ReportSchedule `json:"schedule"`
	Status   string                `json:"status"`
	Path     string                `json:"path,omitempty"`
	Email    string                `json:"email,omitempty"`
}

func (h *ReportingHandlers) HandleListReportSchedules(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	store, err := h.loadReportScheduleStore(r.Context())
	if err != nil {
		writeErrorResponse(w, http.StatusInternalServerError, "schedule_store_unavailable", "Report schedules are unavailable", nil)
		return
	}
	writeReportScheduleJSON(w, http.StatusOK, reportScheduleListResponse{Schedules: store.Schedules})
}

func (h *ReportingHandlers) HandleCreateReportSchedule(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var incoming config.ReportSchedule
	if err := decodeReportScheduleBody(w, r, &incoming); err != nil {
		return
	}

	persistence, store, err := h.reportSchedulePersistenceAndStore(r.Context())
	if err != nil {
		writeErrorResponse(w, http.StatusInternalServerError, "schedule_store_unavailable", "Report schedules are unavailable", nil)
		return
	}
	now := time.Now().UTC()
	incoming.ID = uuid.NewString()
	incoming.CreatedAt = now
	incoming.UpdatedAt = now
	schedule, err := h.prepareReportSchedule(r.Context(), incoming, now, false)
	if err != nil {
		writeReportScheduleValidationError(w, err)
		return
	}
	store.Schedules = append(store.Schedules, schedule)
	if err := persistence.SaveReportScheduleStore(*store); err != nil {
		writeErrorResponse(w, http.StatusInternalServerError, "schedule_save_failed", "Failed to save report schedule", nil)
		return
	}
	writeReportScheduleJSON(w, http.StatusCreated, schedule)
}

func (h *ReportingHandlers) HandleUpdateReportSchedule(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	scheduleID := strings.TrimSpace(r.PathValue("id"))
	if scheduleID == "" {
		writeErrorResponse(w, http.StatusBadRequest, "missing_schedule_id", "Schedule ID is required", nil)
		return
	}
	var incoming config.ReportSchedule
	if err := decodeReportScheduleBody(w, r, &incoming); err != nil {
		return
	}

	persistence, store, err := h.reportSchedulePersistenceAndStore(r.Context())
	if err != nil {
		writeErrorResponse(w, http.StatusInternalServerError, "schedule_store_unavailable", "Report schedules are unavailable", nil)
		return
	}
	index := findReportScheduleIndex(store.Schedules, scheduleID)
	if index < 0 {
		writeErrorResponse(w, http.StatusNotFound, "schedule_not_found", "Report schedule not found", nil)
		return
	}

	now := time.Now().UTC()
	existing := store.Schedules[index]
	incoming.ID = existing.ID
	incoming.CreatedAt = existing.CreatedAt
	incoming.UpdatedAt = now
	incoming.LastRunAt = existing.LastRunAt
	incoming.LastRunStatus = existing.LastRunStatus
	incoming.LastError = existing.LastError
	incoming.LastOccurrenceKey = existing.LastOccurrenceKey
	schedule, err := h.prepareReportSchedule(r.Context(), incoming, now, false)
	if err != nil {
		writeReportScheduleValidationError(w, err)
		return
	}
	store.Schedules[index] = schedule
	if err := persistence.SaveReportScheduleStore(*store); err != nil {
		writeErrorResponse(w, http.StatusInternalServerError, "schedule_save_failed", "Failed to save report schedule", nil)
		return
	}
	writeReportScheduleJSON(w, http.StatusOK, schedule)
}

func (h *ReportingHandlers) HandleDeleteReportSchedule(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	scheduleID := strings.TrimSpace(r.PathValue("id"))
	if scheduleID == "" {
		writeErrorResponse(w, http.StatusBadRequest, "missing_schedule_id", "Schedule ID is required", nil)
		return
	}
	persistence, store, err := h.reportSchedulePersistenceAndStore(r.Context())
	if err != nil {
		writeErrorResponse(w, http.StatusInternalServerError, "schedule_store_unavailable", "Report schedules are unavailable", nil)
		return
	}
	index := findReportScheduleIndex(store.Schedules, scheduleID)
	if index < 0 {
		writeErrorResponse(w, http.StatusNotFound, "schedule_not_found", "Report schedule not found", nil)
		return
	}
	store.Schedules = append(store.Schedules[:index], store.Schedules[index+1:]...)
	if err := persistence.SaveReportScheduleStore(*store); err != nil {
		writeErrorResponse(w, http.StatusInternalServerError, "schedule_save_failed", "Failed to save report schedule", nil)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *ReportingHandlers) HandleRunReportSchedule(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	scheduleID := strings.TrimSpace(r.PathValue("id"))
	if scheduleID == "" {
		writeErrorResponse(w, http.StatusBadRequest, "missing_schedule_id", "Schedule ID is required", nil)
		return
	}
	persistence, store, err := h.reportSchedulePersistenceAndStore(r.Context())
	if err != nil {
		writeErrorResponse(w, http.StatusInternalServerError, "schedule_store_unavailable", "Report schedules are unavailable", nil)
		return
	}
	index := findReportScheduleIndex(store.Schedules, scheduleID)
	if index < 0 {
		writeErrorResponse(w, http.StatusNotFound, "schedule_not_found", "Report schedule not found", nil)
		return
	}

	result, updated := h.runReportSchedule(r.Context(), persistence, store.Schedules[index], time.Now().UTC(), true, "")
	store.Schedules[index] = updated
	if err := persistence.SaveReportScheduleStore(*store); err != nil {
		writeErrorResponse(w, http.StatusInternalServerError, "schedule_save_failed", "Failed to save report schedule status", nil)
		return
	}
	status := http.StatusOK
	if updated.LastRunStatus == config.ReportScheduleLastRunFailed {
		status = http.StatusInternalServerError
	}
	writeReportScheduleJSON(w, status, reportScheduleRunResponse{
		Schedule: updated,
		Status:   updated.LastRunStatus,
		Path:     result.path,
		Email:    result.email,
	})
}

func decodeReportScheduleBody(w http.ResponseWriter, r *http.Request, target *config.ReportSchedule) error {
	r.Body = http.MaxBytesReader(w, r.Body, reportingMultiReportBodyMax)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "invalid_body", "Invalid report schedule body", nil)
		return err
	}
	if err := decoder.Decode(&struct{}{}); err != nil && !errors.Is(err, io.EOF) {
		writeErrorResponse(w, http.StatusBadRequest, "invalid_body", "Invalid report schedule body", nil)
		return err
	}
	return nil
}

func writeReportScheduleJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

type reportScheduleValidationError struct {
	code    string
	message string
}

func (e reportScheduleValidationError) Error() string {
	return e.message
}

func writeReportScheduleValidationError(w http.ResponseWriter, err error) {
	var validationErr reportScheduleValidationError
	if errors.As(err, &validationErr) {
		writeErrorResponse(w, http.StatusBadRequest, validationErr.code, validationErr.message, nil)
		return
	}
	writeErrorResponse(w, http.StatusBadRequest, "invalid_schedule", err.Error(), nil)
}

func (h *ReportingHandlers) reportSchedulePersistenceAndStore(ctx context.Context) (*config.ConfigPersistence, *config.ReportScheduleStore, error) {
	persistence, err := h.reportSchedulePersistence(ctx)
	if err != nil {
		return nil, nil, err
	}
	store, err := persistence.LoadReportScheduleStore()
	if err != nil {
		return nil, nil, err
	}
	if store == nil {
		empty := config.EmptyReportScheduleStore()
		store = &empty
	}
	return persistence, store, nil
}

func (h *ReportingHandlers) loadReportScheduleStore(ctx context.Context) (*config.ReportScheduleStore, error) {
	_, store, err := h.reportSchedulePersistenceAndStore(ctx)
	return store, err
}

func (h *ReportingHandlers) reportSchedulePersistence(ctx context.Context) (*config.ConfigPersistence, error) {
	if h == nil || h.mtMonitor == nil {
		return nil, fmt.Errorf("multi-tenant monitor is not configured")
	}
	orgID := GetOrgID(ctx)
	if strings.TrimSpace(orgID) == "" {
		orgID = "default"
	}
	monitor, err := h.mtMonitor.GetMonitor(orgID)
	if err != nil {
		return nil, err
	}
	persistence := monitor.GetConfigPersistence()
	if persistence == nil {
		return nil, fmt.Errorf("config persistence is not configured")
	}
	return persistence, nil
}

func findReportScheduleIndex(schedules []config.ReportSchedule, id string) int {
	for i := range schedules {
		if schedules[i].ID == id {
			return i
		}
	}
	return -1
}

func (h *ReportingHandlers) prepareReportSchedule(ctx context.Context, schedule config.ReportSchedule, now time.Time, statusOnly bool) (config.ReportSchedule, error) {
	schedule = config.NormalizeReportSchedule(schedule)
	if schedule.ID == "" {
		return schedule, reportScheduleValidationError{code: "invalid_schedule_id", message: "Schedule ID is required"}
	}
	if statusOnly {
		return schedule, nil
	}
	if schedule.Name == "" || len(schedule.Name) > 80 {
		return schedule, reportScheduleValidationError{code: "invalid_name", message: "Schedule name is required and must be 80 characters or fewer"}
	}
	if schedule.Format == "" {
		schedule.Format = config.ReportScheduleFormatPDF
	}
	if schedule.Format != config.ReportScheduleFormatPDF && schedule.Format != config.ReportScheduleFormatCSV {
		return schedule, reportScheduleValidationError{code: "invalid_format", message: "Schedule format must be pdf or csv"}
	}
	if schedule.Delivery.Method == "" {
		schedule.Delivery.Method = config.ReportScheduleDeliveryEmail
	}
	if schedule.Delivery.Method != config.ReportScheduleDeliveryEmail && schedule.Delivery.Method != config.ReportScheduleDeliveryDisk {
		return schedule, reportScheduleValidationError{code: "invalid_delivery", message: "Delivery method must be email or disk"}
	}
	if !schedule.Delivery.Attach && !schedule.Delivery.SaveToDisk {
		schedule.Delivery.Attach = true
		schedule.Delivery.SaveToDisk = true
	}
	if err := validateReportScheduleCadence(schedule.Cadence); err != nil {
		return schedule, err
	}
	if err := h.validateReportScheduleScope(ctx, schedule.Scope); err != nil {
		return schedule, err
	}
	next, err := nextReportScheduleRunAt(schedule, now)
	if err != nil {
		return schedule, err
	}
	schedule.NextRunAt = &next
	return schedule, nil
}

func validateReportScheduleCadence(cadence config.ReportScheduleCadence) error {
	if cadence.Type != config.ReportScheduleCadenceMonthly && cadence.Type != config.ReportScheduleCadenceWeekly {
		return reportScheduleValidationError{code: "invalid_cadence", message: "Cadence must be monthly or weekly"}
	}
	if cadence.Type == config.ReportScheduleCadenceMonthly && (cadence.DayOfMonth < 1 || cadence.DayOfMonth > 28) {
		return reportScheduleValidationError{code: "invalid_day_of_month", message: "Monthly schedules must use a day from 1 to 28"}
	}
	if cadence.Type == config.ReportScheduleCadenceWeekly {
		if _, ok := parseReportScheduleWeekday(cadence.Weekday); !ok {
			return reportScheduleValidationError{code: "invalid_weekday", message: "Weekly schedules must include a weekday"}
		}
	}
	if _, err := time.Parse("15:04", cadence.Time); err != nil {
		return reportScheduleValidationError{code: "invalid_time", message: "Schedule time must use HH:MM"}
	}
	if _, err := reportScheduleLocation(cadence.Timezone); err != nil {
		return reportScheduleValidationError{code: "invalid_timezone", message: "Schedule timezone must be a valid IANA timezone"}
	}
	return nil
}

func (h *ReportingHandlers) validateReportScheduleScope(ctx context.Context, scope config.ReportScheduleScope) error {
	if len(scope.Resources) == 0 && len(scope.Tags) == 0 {
		return reportScheduleValidationError{code: "invalid_scope", message: "Select at least one resource or tag"}
	}
	definition := performanceReportDefinition()
	if len(scope.Resources) > definition.MultiResourceMax {
		return reportScheduleValidationError{code: "too_many_resources", message: fmt.Sprintf("Maximum %d resources allowed", definition.MultiResourceMax)}
	}
	for _, res := range scope.Resources {
		if !validResourceID.MatchString(res.ResourceID) || len(res.ResourceID) > 128 {
			return reportScheduleValidationError{code: "invalid_resource_id", message: "Resource IDs must match [a-zA-Z0-9._:-]+ and be 128 characters or fewer"}
		}
		if _, err := normalizeReportResourceType(res.ResourceType); err != nil {
			return reportScheduleValidationError{code: "invalid_resource_type", message: err.Error()}
		}
	}
	for _, tag := range scope.Tags {
		if len(tag) > 64 || strings.ContainsAny(tag, "\r\n\t") {
			return reportScheduleValidationError{code: "invalid_tag", message: "Tags must be 64 characters or fewer and cannot contain control characters"}
		}
	}
	if len(scope.Tags) > 0 {
		if _, err := h.resolveReportScheduleResources(ctx, GetOrgID(ctx), scope); err != nil {
			return err
		}
	}
	return nil
}

func (h *ReportingHandlers) resolveReportScheduleResources(ctx context.Context, orgID string, scope config.ReportScheduleScope) ([]multiReportResourceEntry, error) {
	definition := performanceReportDefinition()
	entries := make([]multiReportResourceEntry, 0, len(scope.Resources))
	seen := map[string]struct{}{}
	appendEntry := func(resourceType, resourceID string) error {
		resourceType, err := normalizeReportResourceType(resourceType)
		if err != nil {
			return err
		}
		key := resourceType + "\x00" + resourceID
		if _, exists := seen[key]; exists {
			return nil
		}
		seen[key] = struct{}{}
		entries = append(entries, multiReportResourceEntry{ResourceType: resourceType, ResourceID: resourceID})
		return nil
	}
	for _, res := range scope.Resources {
		if err := appendEntry(res.ResourceType, res.ResourceID); err != nil {
			return nil, reportScheduleValidationError{code: "invalid_resource_type", message: err.Error()}
		}
	}

	if len(scope.Tags) > 0 {
		snapshot, ok := h.getReportingEnrichmentSnapshot(ctx, orgID)
		if !ok {
			return nil, reportScheduleValidationError{code: "scope_unavailable", message: "Tagged schedule scope cannot be resolved until resource inventory is available"}
		}
		tags := map[string]struct{}{}
		for _, tag := range scope.Tags {
			tags[strings.ToLower(strings.TrimSpace(tag))] = struct{}{}
		}
		for _, resource := range snapshot.Resources {
			if !resourceHasAnyReportScheduleTag(resource, tags) {
				continue
			}
			resourceType := reporting.CanonicalResourceType(string(resource.Type))
			if resourceType == "" || strings.TrimSpace(resource.ID) == "" {
				continue
			}
			if err := appendEntry(resourceType, resource.ID); err != nil {
				continue
			}
		}
	}
	if len(entries) == 0 {
		return nil, reportScheduleValidationError{code: "empty_scope", message: "Schedule scope did not match any reportable resources"}
	}
	if len(entries) > definition.MultiResourceMax {
		return nil, reportScheduleValidationError{code: "too_many_resources", message: fmt.Sprintf("Maximum %d resources allowed", definition.MultiResourceMax)}
	}
	return entries, nil
}

func resourceHasAnyReportScheduleTag(resource unifiedresources.Resource, tags map[string]struct{}) bool {
	for _, tag := range resource.Tags {
		if _, ok := tags[strings.ToLower(strings.TrimSpace(tag))]; ok {
			return true
		}
	}
	return false
}

type reportScheduleRunResult struct {
	path  string
	email string
}

func (h *ReportingHandlers) runReportSchedule(ctx context.Context, persistence *config.ConfigPersistence, schedule config.ReportSchedule, now time.Time, manual bool, occurrenceKey string) (reportScheduleRunResult, config.ReportSchedule) {
	h.scheduleRunMu.Lock()
	defer h.scheduleRunMu.Unlock()

	schedule = config.NormalizeReportSchedule(schedule)
	result := reportScheduleRunResult{}
	schedule.LastRunAt = &now
	schedule.UpdatedAt = now
	if occurrenceKey != "" {
		schedule.LastOccurrenceKey = occurrenceKey
	}
	if next, err := nextReportScheduleRunAt(schedule, now); err == nil {
		schedule.NextRunAt = &next
	}

	orgID := GetOrgID(ctx)
	resources, err := h.resolveReportScheduleResources(ctx, orgID, schedule.Scope)
	if err != nil {
		return result, markReportScheduleFailed(schedule, now, err)
	}
	start, end, err := reportScheduleWindow(schedule, now, manual)
	if err != nil {
		return result, markReportScheduleFailed(schedule, now, err)
	}

	report, err := h.generateMultiReportFromBody(ctx, multiReportRequestBody{
		Resources: resources,
		Format:    schedule.Format,
		Start:     start.UTC().Format(time.RFC3339),
		End:       end.UTC().Format(time.RFC3339),
		Title:     schedule.Name,
	}, now)
	if err != nil {
		return result, markReportScheduleFailed(schedule, now, err)
	}

	path, err := saveGeneratedReport(persistence, schedule, report)
	if err != nil {
		return result, markReportScheduleFailed(schedule, now, err)
	}
	result.path = path

	if schedule.Delivery.Method == config.ReportScheduleDeliveryEmail {
		emailStatus, err := sendScheduledReportEmail(persistence, schedule, report, path)
		if err != nil {
			return result, markReportScheduleFailed(schedule, now, err)
		}
		result.email = emailStatus
	}

	schedule.LastRunStatus = config.ReportScheduleLastRunOK
	schedule.LastError = ""
	return result, schedule
}

func markReportScheduleFailed(schedule config.ReportSchedule, now time.Time, err error) config.ReportSchedule {
	schedule.LastRunStatus = config.ReportScheduleLastRunFailed
	schedule.LastError = strings.TrimSpace(err.Error())
	schedule.LastRunAt = &now
	schedule.UpdatedAt = now
	return schedule
}

func reportScheduleWindow(schedule config.ReportSchedule, now time.Time, manual bool) (time.Time, time.Time, error) {
	location, err := reportScheduleLocation(schedule.Cadence.Timezone)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	end := now.In(location)
	if !manual {
		occurrence, _, err := lastReportScheduleOccurrenceAt(schedule, now)
		if err == nil {
			end = occurrence.In(location)
		}
	}
	switch schedule.Cadence.Type {
	case config.ReportScheduleCadenceMonthly:
		firstThisMonth := time.Date(end.Year(), end.Month(), 1, 0, 0, 0, 0, location)
		firstPreviousMonth := firstThisMonth.AddDate(0, -1, 0)
		return firstPreviousMonth.UTC(), firstThisMonth.UTC(), nil
	case config.ReportScheduleCadenceWeekly:
		return end.AddDate(0, 0, -7).UTC(), end.UTC(), nil
	default:
		return time.Time{}, time.Time{}, fmt.Errorf("unsupported cadence %q", schedule.Cadence.Type)
	}
}

func saveGeneratedReport(persistence *config.ConfigPersistence, schedule config.ReportSchedule, report generatedMultiReport) (string, error) {
	baseDir, err := securityutil.JoinStorageLeaf(persistence.DataDir(), "reports")
	if err != nil {
		return "", err
	}
	generatedDir, err := securityutil.JoinStorageLeaf(baseDir, "generated")
	if err != nil {
		return "", err
	}
	scheduleDir, err := securityutil.JoinStorageLeaf(generatedDir, securityutil.HashedStorageName(schedule.ID))
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(scheduleDir, 0o700); err != nil {
		return "", fmt.Errorf("create report output directory: %w", err)
	}
	filename := sanitizeFilename(report.Filename)
	if filename == "" {
		filename = "report." + string(report.Format)
	}
	path, err := securityutil.JoinStorageLeaf(scheduleDir, filename)
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(path, report.Data, 0o600); err != nil {
		return "", fmt.Errorf("write generated report: %w", err)
	}
	pruneGeneratedReports(scheduleDir, schedule.RetentionCount)
	return path, nil
}

func pruneGeneratedReports(dir string, retentionCount int) {
	if retentionCount <= 0 {
		retentionCount = config.DefaultReportScheduleRetentionCount
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	type generatedFile struct {
		path    string
		modTime time.Time
	}
	files := make([]generatedFile, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		path, err := securityutil.JoinStorageLeaf(dir, entry.Name())
		if err != nil {
			continue
		}
		files = append(files, generatedFile{path: path, modTime: info.ModTime()})
	}
	sort.Slice(files, func(i, j int) bool {
		return files[i].modTime.After(files[j].modTime)
	})
	for i := retentionCount; i < len(files); i++ {
		_ = os.Remove(files[i].path)
	}
}

func sendScheduledReportEmail(persistence *config.ConfigPersistence, schedule config.ReportSchedule, report generatedMultiReport, path string) (string, error) {
	emailCfg, err := persistence.LoadEmailConfig()
	if err != nil {
		return "", fmt.Errorf("load email config: %w", err)
	}
	if emailCfg == nil || !emailCfg.Enabled {
		return "saved_to_disk_email_unconfigured", nil
	}
	recipients := schedule.Delivery.To
	if len(recipients) == 0 {
		recipients = emailCfg.To
	}
	if len(recipients) == 0 && strings.TrimSpace(emailCfg.From) != "" {
		recipients = []string{emailCfg.From}
	}
	if len(recipients) == 0 {
		return "saved_to_disk_email_unconfigured", nil
	}

	providerConfig := notifications.EmailProviderConfig{
		EmailConfig: notifications.EmailConfig{
			Provider:  emailCfg.Provider,
			From:      emailCfg.From,
			To:        recipients,
			SMTPHost:  emailCfg.SMTPHost,
			SMTPPort:  emailCfg.SMTPPort,
			Username:  emailCfg.Username,
			Password:  emailCfg.Password,
			TLS:       emailCfg.TLS,
			StartTLS:  emailCfg.StartTLS,
			RateLimit: emailCfg.RateLimit,
		},
		Provider:     emailCfg.Provider,
		MaxRetries:   2,
		RetryDelay:   3,
		RateLimit:    emailCfg.RateLimit,
		StartTLS:     emailCfg.StartTLS,
		AuthRequired: emailCfg.Username != "" && emailCfg.Password != "",
	}
	manager := notifications.NewEnhancedEmailManager(providerConfig)
	subject := "Pulse report: " + schedule.Name
	htmlBody := "<p>Your scheduled Pulse report is ready.</p>"
	textBody := "Your scheduled Pulse report is ready."
	attachments := []notifications.EmailAttachment{}
	if schedule.Delivery.Attach && len(report.Data) <= reportScheduleAttachmentLimitBytes {
		attachments = append(attachments, notifications.EmailAttachment{
			Filename:    report.Filename,
			ContentType: report.ContentType,
			Data:        report.Data,
		})
	} else if schedule.Delivery.Attach && len(report.Data) > reportScheduleAttachmentLimitBytes {
		htmlBody = "<p>Your scheduled Pulse report was generated but is too large to attach.</p><p>Saved path: " + htmlEscape(path) + "</p>"
		textBody = "Your scheduled Pulse report was generated but is too large to attach.\nSaved path: " + path
	} else {
		htmlBody = "<p>Your scheduled Pulse report was generated and saved to disk.</p><p>Saved path: " + htmlEscape(path) + "</p>"
		textBody = "Your scheduled Pulse report was generated and saved to disk.\nSaved path: " + path
	}
	if err := manager.SendEmailWithAttachments(subject, htmlBody, textBody, attachments); err != nil {
		return "", err
	}
	if len(attachments) > 0 {
		return "sent_with_attachment", nil
	}
	return "sent_saved_path", nil
}

func htmlEscape(value string) string {
	replacer := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", "\"", "&quot;", "'", "&#39;")
	return replacer.Replace(value)
}

func reportScheduleLocation(name string) (*time.Location, error) {
	name = strings.TrimSpace(name)
	if name == "" || name == "Local" {
		name = "UTC"
	}
	return time.LoadLocation(name)
}

func parseReportScheduleClock(clock string) (hour, minute int, err error) {
	parsed, err := time.Parse("15:04", strings.TrimSpace(clock))
	if err != nil {
		return 0, 0, err
	}
	return parsed.Hour(), parsed.Minute(), nil
}

func parseReportScheduleWeekday(value string) (time.Weekday, bool) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "sunday", "sun":
		return time.Sunday, true
	case "monday", "mon":
		return time.Monday, true
	case "tuesday", "tue":
		return time.Tuesday, true
	case "wednesday", "wed":
		return time.Wednesday, true
	case "thursday", "thu":
		return time.Thursday, true
	case "friday", "fri":
		return time.Friday, true
	case "saturday", "sat":
		return time.Saturday, true
	default:
		return time.Sunday, false
	}
}

func nextReportScheduleRunAt(schedule config.ReportSchedule, after time.Time) (time.Time, error) {
	location, err := reportScheduleLocation(schedule.Cadence.Timezone)
	if err != nil {
		return time.Time{}, err
	}
	hour, minute, err := parseReportScheduleClock(schedule.Cadence.Time)
	if err != nil {
		return time.Time{}, err
	}
	localAfter := after.In(location)
	switch schedule.Cadence.Type {
	case config.ReportScheduleCadenceMonthly:
		candidate := time.Date(localAfter.Year(), localAfter.Month(), schedule.Cadence.DayOfMonth, hour, minute, 0, 0, location)
		if !candidate.After(localAfter) {
			candidate = candidate.AddDate(0, 1, 0)
		}
		return candidate.UTC(), nil
	case config.ReportScheduleCadenceWeekly:
		weekday, ok := parseReportScheduleWeekday(schedule.Cadence.Weekday)
		if !ok {
			return time.Time{}, fmt.Errorf("invalid weekday")
		}
		dayDelta := (int(weekday) - int(localAfter.Weekday()) + 7) % 7
		candidateDate := localAfter.AddDate(0, 0, dayDelta)
		candidate := time.Date(candidateDate.Year(), candidateDate.Month(), candidateDate.Day(), hour, minute, 0, 0, location)
		if !candidate.After(localAfter) {
			candidate = candidate.AddDate(0, 0, 7)
		}
		return candidate.UTC(), nil
	default:
		return time.Time{}, fmt.Errorf("unsupported cadence %q", schedule.Cadence.Type)
	}
}

func lastReportScheduleOccurrenceAt(schedule config.ReportSchedule, now time.Time) (time.Time, string, error) {
	location, err := reportScheduleLocation(schedule.Cadence.Timezone)
	if err != nil {
		return time.Time{}, "", err
	}
	hour, minute, err := parseReportScheduleClock(schedule.Cadence.Time)
	if err != nil {
		return time.Time{}, "", err
	}
	localNow := now.In(location)
	var occurrence time.Time
	switch schedule.Cadence.Type {
	case config.ReportScheduleCadenceMonthly:
		occurrence = time.Date(localNow.Year(), localNow.Month(), schedule.Cadence.DayOfMonth, hour, minute, 0, 0, location)
		if occurrence.After(localNow) {
			occurrence = occurrence.AddDate(0, -1, 0)
		}
	case config.ReportScheduleCadenceWeekly:
		weekday, ok := parseReportScheduleWeekday(schedule.Cadence.Weekday)
		if !ok {
			return time.Time{}, "", fmt.Errorf("invalid weekday")
		}
		dayDelta := (int(localNow.Weekday()) - int(weekday) + 7) % 7
		occurrenceDate := localNow.AddDate(0, 0, -dayDelta)
		occurrence = time.Date(occurrenceDate.Year(), occurrenceDate.Month(), occurrenceDate.Day(), hour, minute, 0, 0, location)
		if occurrence.After(localNow) {
			occurrence = occurrence.AddDate(0, 0, -7)
		}
	default:
		return time.Time{}, "", fmt.Errorf("unsupported cadence %q", schedule.Cadence.Type)
	}
	return occurrence.UTC(), occurrenceKey(schedule, occurrence), nil
}

func occurrenceKey(schedule config.ReportSchedule, occurrence time.Time) string {
	timezone := strings.TrimSpace(schedule.Cadence.Timezone)
	if timezone == "" {
		timezone = "UTC"
	}
	return schedule.Cadence.Type + ":" + occurrence.UTC().Format("2006-01-02T15:04:05Z07:00") + ":" + timezone
}

func (h *ReportingHandlers) RunReportScheduleScheduler(ctx context.Context) {
	if h == nil {
		return
	}
	h.runDueReportSchedules(ctx, time.Now().UTC())
	ticker := time.NewTicker(reportScheduleTickInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			h.runDueReportSchedules(ctx, now.UTC())
		}
	}
}

func (h *ReportingHandlers) runDueReportSchedules(ctx context.Context, now time.Time) {
	if h == nil || h.mtMonitor == nil {
		return
	}
	orgIDs, err := h.mtMonitor.ListOrganizationIDs()
	if err != nil {
		log.Warn().Err(err).Msg("report schedules: list organizations")
		return
	}
	for _, orgID := range orgIDs {
		orgCtx := context.WithValue(ctx, OrgIDContextKey, orgID)
		h.runDueReportSchedulesForOrg(orgCtx, orgID, now)
	}
}

func (h *ReportingHandlers) runDueReportSchedulesForOrg(ctx context.Context, orgID string, now time.Time) {
	persistence, store, err := h.reportSchedulePersistenceAndStore(ctx)
	if err != nil {
		log.Warn().Err(err).Str("org_id", orgID).Msg("report schedules: load store")
		return
	}
	changed := false
	for i := range store.Schedules {
		schedule := config.NormalizeReportSchedule(store.Schedules[i])
		if !schedule.Enabled {
			continue
		}
		occurrence, key, err := lastReportScheduleOccurrenceAt(schedule, now)
		if err != nil {
			store.Schedules[i] = markReportScheduleFailed(schedule, now, err)
			changed = true
			continue
		}
		if key == "" || key == schedule.LastOccurrenceKey || occurrence.After(now) {
			continue
		}
		if now.Sub(occurrence) > reportScheduleMissedRunGrace {
			schedule.LastOccurrenceKey = key
			if next, err := nextReportScheduleRunAt(schedule, now); err == nil {
				schedule.NextRunAt = &next
			}
			schedule.UpdatedAt = now
			store.Schedules[i] = schedule
			changed = true
			continue
		}
		_, updated := h.runReportSchedule(ctx, persistence, schedule, now, false, key)
		store.Schedules[i] = updated
		changed = true
	}
	if changed {
		if err := persistence.SaveReportScheduleStore(*store); err != nil {
			log.Warn().Err(err).Str("org_id", orgID).Msg("report schedules: save statuses")
		}
	}
}
