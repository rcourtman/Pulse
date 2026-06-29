package alerts

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// AlertNoiseFindingKind identifies the class of alert noise detected by the auditor.
type AlertNoiseFindingKind string

const (
	AlertNoiseFindingIdentityChurn   AlertNoiseFindingKind = "identity_churn"
	AlertNoiseFindingDuplicateActive AlertNoiseFindingKind = "duplicate_active"
)

// AlertNoiseIdentity is the stable grouping key the auditor uses when comparing
// alert records across active state and history.
type AlertNoiseIdentity struct {
	Key          string `json:"key"`
	Source       string `json:"source"`
	ResourceType string `json:"resourceType,omitempty"`
}

// AlertNoiseFinding describes one identity/noise problem found in alert records.
type AlertNoiseFinding struct {
	Kind          AlertNoiseFindingKind `json:"kind"`
	Identity      AlertNoiseIdentity    `json:"identity"`
	AlertIDs      []string              `json:"alertIds"`
	ResourceIDs   []string              `json:"resourceIds"`
	ResourceNames []string              `json:"resourceNames,omitempty"`
	Count         int                   `json:"count"`
	FirstSeen     time.Time             `json:"firstSeen,omitempty"`
	LastSeen      time.Time             `json:"lastSeen,omitempty"`
	Message       string                `json:"message"`
}

// AlertNoiseReport is a point-in-time read-only summary of identity churn and
// duplicate active alerts. It is intentionally separate from notification flow.
type AlertNoiseReport struct {
	GeneratedAt  time.Time           `json:"generatedAt"`
	WindowStart  time.Time           `json:"windowStart,omitempty"`
	ActiveCount  int                 `json:"activeCount"`
	HistoryCount int                 `json:"historyCount"`
	Findings     []AlertNoiseFinding `json:"findings"`
}

type alertNoiseBucket struct {
	identity            AlertNoiseIdentity
	alertIDs            map[string]struct{}
	resourceIDs         map[string]struct{}
	resourceNames       map[string]struct{}
	activeAlertIDs      map[string]struct{}
	activeResourceIDs   map[string]struct{}
	activeResourceNames map[string]struct{}
	count               int
	firstSeen           time.Time
	lastSeen            time.Time
}

// AuditAlertNoise groups alert records by canonical identity and reports
// duplicate active alerts or records whose alert/resource IDs churned for the
// same logical alert. The inputs are value slices so callers can pass active
// alerts, history, or test fixtures without exposing manager internals.
func AuditAlertNoise(activeAlerts, historyAlerts []Alert, windowStart time.Time) AlertNoiseReport {
	report := AlertNoiseReport{
		GeneratedAt:  time.Now(),
		WindowStart:  windowStart,
		ActiveCount:  len(activeAlerts),
		HistoryCount: len(historyAlerts),
	}

	buckets := make(map[string]*alertNoiseBucket)
	for _, alert := range historyAlerts {
		if !alertInNoiseWindow(alert, windowStart) {
			continue
		}
		addAlertNoiseRecord(buckets, alert, false)
	}
	for _, alert := range activeAlerts {
		addAlertNoiseRecord(buckets, alert, true)
	}

	keys := make([]string, 0, len(buckets))
	for key := range buckets {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	for _, key := range keys {
		bucket := buckets[key]
		alertIDs := sortedStringSet(bucket.alertIDs)
		resourceIDs := sortedStringSet(bucket.resourceIDs)
		resourceNames := sortedStringSet(bucket.resourceNames)

		if len(bucket.activeAlertIDs) > 1 || len(bucket.activeResourceIDs) > 1 {
			report.Findings = append(report.Findings, AlertNoiseFinding{
				Kind:          AlertNoiseFindingDuplicateActive,
				Identity:      bucket.identity,
				AlertIDs:      sortedStringSet(bucket.activeAlertIDs),
				ResourceIDs:   sortedStringSet(bucket.activeResourceIDs),
				ResourceNames: sortedStringSet(bucket.activeResourceNames),
				Count:         len(bucket.activeAlertIDs),
				FirstSeen:     bucket.firstSeen,
				LastSeen:      bucket.lastSeen,
				Message:       "Multiple active alerts share one canonical identity.",
			})
		}

		if len(alertIDs) > 1 || len(resourceIDs) > 1 {
			report.Findings = append(report.Findings, AlertNoiseFinding{
				Kind:          AlertNoiseFindingIdentityChurn,
				Identity:      bucket.identity,
				AlertIDs:      alertIDs,
				ResourceIDs:   resourceIDs,
				ResourceNames: resourceNames,
				Count:         bucket.count,
				FirstSeen:     bucket.firstSeen,
				LastSeen:      bucket.lastSeen,
				Message:       "One canonical identity appears under multiple alert or resource IDs.",
			})
		}
	}

	sort.SliceStable(report.Findings, func(i, j int) bool {
		if report.Findings[i].Kind != report.Findings[j].Kind {
			return report.Findings[i].Kind < report.Findings[j].Kind
		}
		return report.Findings[i].Identity.Key < report.Findings[j].Identity.Key
	})

	return report
}

// AuditAlertNoise reads the manager's active alerts and history and builds an
// AlertNoiseReport without mutating alert state, cooldowns, resolved records, or
// notification tracking.
func (m *Manager) AuditAlertNoise(since time.Time, historyLimit int) AlertNoiseReport {
	return AuditAlertNoise(
		m.GetActiveAlerts(),
		m.GetAlertHistorySince(since, historyLimit),
		since,
	)
}

// AlertNoiseIdentityFor returns the canonical identity used by the auditor.
// Guest metric alerts are grouped by instance and VMID so node moves do not look
// like a different logical alert. Per-disk guest alerts add the stable disk key.
func AlertNoiseIdentityFor(alert Alert) AlertNoiseIdentity {
	metricType := strings.TrimSpace(alert.Type)
	if metricType == "" {
		metricType = "unknown"
	}
	resourceType := metadataStringValue(alert.Metadata, "resourceType")

	if key, source, ok := guestAlertNoiseIdentity(alert, metricType); ok {
		return AlertNoiseIdentity{
			Key:          key,
			Source:       source,
			ResourceType: resourceType,
		}
	}

	if identityKey := metadataStringValue(alert.Metadata, "identityKey"); identityKey != "" {
		return AlertNoiseIdentity{
			Key:          fmt.Sprintf("%s::%s", identityKey, metricType),
			Source:       "metadata.identityKey",
			ResourceType: resourceType,
		}
	}

	resourceID := strings.TrimSpace(alert.ResourceID)
	source := "resourceId"
	if resourceID == "" {
		resourceID = strings.TrimSpace(alert.ID)
		source = "alertId"
	}

	return AlertNoiseIdentity{
		Key:          fmt.Sprintf("%s::%s", resourceID, metricType),
		Source:       source,
		ResourceType: resourceType,
	}
}

func guestAlertNoiseIdentity(alert Alert, metricType string) (string, string, bool) {
	resourceID := strings.TrimSpace(alert.ResourceID)
	if resourceID == "" {
		return "", "", false
	}

	parsed, ok := parseGuestMetricResourceIdentity(resourceID)
	if !ok {
		return "", "", false
	}

	if base, _, ok := splitGuestDiskResourceID(resourceID); ok {
		baseIdent, ok := parseCanonicalGuestKey(base)
		if !ok {
			return "", "", false
		}
		diskKey, hasDiskKey := guestDiskKeyForAlert(&alert, parsed)
		if !hasDiskKey || diskKey == "" {
			return "", "", false
		}
		return fmt.Sprintf("guest:%s:%d/disk:%s::%s", baseIdent.instance, baseIdent.vmid, diskKey, metricType), "guest.disk", true
	}

	baseIdent, ok := parseCanonicalGuestKey(resourceID)
	if !ok {
		return "", "", false
	}
	return fmt.Sprintf("guest:%s:%d::%s", baseIdent.instance, baseIdent.vmid, metricType), "guest.metric", true
}

func addAlertNoiseRecord(buckets map[string]*alertNoiseBucket, alert Alert, active bool) {
	identity := AlertNoiseIdentityFor(alert)
	bucket := buckets[identity.Key]
	if bucket == nil {
		bucket = &alertNoiseBucket{
			identity:            identity,
			alertIDs:            make(map[string]struct{}),
			resourceIDs:         make(map[string]struct{}),
			resourceNames:       make(map[string]struct{}),
			activeAlertIDs:      make(map[string]struct{}),
			activeResourceIDs:   make(map[string]struct{}),
			activeResourceNames: make(map[string]struct{}),
		}
		buckets[identity.Key] = bucket
	}

	bucket.count++
	addNonEmptyString(bucket.alertIDs, alert.ID)
	addNonEmptyString(bucket.resourceIDs, alert.ResourceID)
	addNonEmptyString(bucket.resourceNames, alert.ResourceName)
	if active {
		addNonEmptyString(bucket.activeAlertIDs, alert.ID)
		addNonEmptyString(bucket.activeResourceIDs, alert.ResourceID)
		addNonEmptyString(bucket.activeResourceNames, alert.ResourceName)
	}

	firstSeen := alertNoiseFirstSeen(alert)
	if !firstSeen.IsZero() && (bucket.firstSeen.IsZero() || firstSeen.Before(bucket.firstSeen)) {
		bucket.firstSeen = firstSeen
	}
	lastSeen := alertNoiseLastSeen(alert)
	if !lastSeen.IsZero() && (bucket.lastSeen.IsZero() || lastSeen.After(bucket.lastSeen)) {
		bucket.lastSeen = lastSeen
	}
}

func alertInNoiseWindow(alert Alert, windowStart time.Time) bool {
	if windowStart.IsZero() {
		return true
	}
	lastSeen := alertNoiseLastSeen(alert)
	if lastSeen.IsZero() {
		return true
	}
	return !lastSeen.Before(windowStart)
}

func alertNoiseFirstSeen(alert Alert) time.Time {
	if !alert.StartTime.IsZero() {
		return alert.StartTime
	}
	return alert.LastSeen
}

func alertNoiseLastSeen(alert Alert) time.Time {
	if !alert.LastSeen.IsZero() {
		return alert.LastSeen
	}
	return alert.StartTime
}

func addNonEmptyString(values map[string]struct{}, value string) {
	value = strings.TrimSpace(value)
	if value == "" {
		return
	}
	values[value] = struct{}{}
}

func sortedStringSet(values map[string]struct{}) []string {
	if len(values) == 0 {
		return nil
	}
	sorted := make([]string, 0, len(values))
	for value := range values {
		sorted = append(sorted, value)
	}
	sort.Strings(sorted)
	return sorted
}
