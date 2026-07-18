package alerts

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

// GetActiveAlerts returns all active alerts
func (m *Manager) GetActiveAlerts() []Alert {
	if m == nil {
		return nil
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	alerts := make([]Alert, 0, len(m.activeAlerts))
	for _, alert := range m.activeAlerts {
		a := *cloneAlertForOutput(alert)
		// Ensure display name is current (handles upgrades, renames, and
		// alerts created before the cache was populated).
		if dn := m.resolveNodeDisplayName(a.Instance, a.Node); dn != "" {
			a.NodeDisplayName = dn
		}
		alerts = append(alerts, a)
	}

	sort.Slice(alerts, func(i, j int) bool {
		if left, right := alertSeveritySortRank(alerts[i]), alertSeveritySortRank(alerts[j]); left != right {
			return left > right
		}
		if left, right := alertProtectionSortRank(alerts[i]), alertProtectionSortRank(alerts[j]); left != right {
			return left > right
		}
		if left, right := alertRecoverabilitySortRank(alerts[i]), alertRecoverabilitySortRank(alerts[j]); left != right {
			return left > right
		}
		if left, right := alertImpactSortRank(alerts[i]), alertImpactSortRank(alerts[j]); left != right {
			return left > right
		}
		if left, right := alertTypeSortRank(alerts[i]), alertTypeSortRank(alerts[j]); left != right {
			return left > right
		}
		if !alerts[i].StartTime.Equal(alerts[j].StartTime) {
			return alerts[i].StartTime.Before(alerts[j].StartTime)
		}
		if alerts[i].Node != alerts[j].Node {
			return alerts[i].Node < alerts[j].Node
		}
		return alerts[i].ID < alerts[j].ID
	})

	return alerts
}

func alertProtectionSortRank(alert Alert) int {
	switch {
	case metadataBoolValue(alert.Metadata, "protectionReduced"):
		return 2
	case metadataBoolValue(alert.Metadata, "rebuildInProgress"):
		return 1
	default:
		return 0
	}
}

func alertSeveritySortRank(alert Alert) int {
	switch alert.Level {
	case AlertLevelCritical:
		return 2
	case AlertLevelWarning:
		return 1
	default:
		return 0
	}
}

func alertImpactSortRank(alert Alert) int {
	if alert.Metadata == nil {
		return 0
	}
	return metadataIntValue(alert.Metadata["consumerCount"])
}

func alertRecoverabilitySortRank(alert Alert) int {
	switch {
	case metadataBoolValue(alert.Metadata, "backupTarget") && metadataIntValue(alert.Metadata["protectedWorkloadCount"]) > 0:
		return 2
	case metadataBoolValue(alert.Metadata, "backupServer") && metadataIntValue(alert.Metadata["protectedWorkloadCount"]) > 0:
		return 2
	case metadataBoolValue(alert.Metadata, "backupTarget"):
		return 1
	case metadataBoolValue(alert.Metadata, "backupServer") && metadataIntValue(alert.Metadata["affectedDatastoreCount"]) > 0:
		return 1
	case metadataBoolValue(alert.Metadata, "backupServer"):
		return 1
	default:
		return 0
	}
}

func alertTypeSortRank(alert Alert) int {
	switch alert.Type {
	case "backup-posture-incident":
		return 6
	case "backup-storage-incident":
		return 5
	case "storage-incident", "zfs-pool-state", "zfs-pool-errors":
		return 4
	case "resource-incident":
		return 4
	case "disk-health", "disk-wearout", "zfs-device":
		return 3
	case "offline", "connectivity", "powered-off", "docker-host-offline":
		return 2
	default:
		return 1
	}
}

func metadataIntValue(value interface{}) int {
	switch v := value.(type) {
	case int:
		return v
	case int8:
		return int(v)
	case int16:
		return int(v)
	case int32:
		return int(v)
	case int64:
		return int(v)
	case uint:
		return int(v)
	case uint8:
		return int(v)
	case uint16:
		return int(v)
	case uint32:
		return int(v)
	case uint64:
		return int(v)
	case float32:
		return int(v)
	case float64:
		return int(v)
	case json.Number:
		if parsed, err := v.Int64(); err == nil {
			return int(parsed)
		}
	case string:
		if parsed, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
			return parsed
		}
	}
	return 0
}

func metadataFloatValue(metadata map[string]interface{}, key string) (float64, bool) {
	if metadata == nil {
		return 0, false
	}
	value, ok := metadata[key]
	if !ok {
		return 0, false
	}
	return numericConditionValue(value)
}

func metadataStringValue(metadata map[string]interface{}, key string) string {
	if metadata == nil {
		return ""
	}
	value, ok := metadata[key]
	if !ok {
		return ""
	}
	switch v := value.(type) {
	case string:
		return strings.TrimSpace(v)
	case []byte:
		return strings.TrimSpace(string(v))
	case json.Number:
		return strings.TrimSpace(v.String())
	default:
		return strings.TrimSpace(fmt.Sprint(v))
	}
}

func metadataBoolValue(metadata map[string]interface{}, key string) bool {
	if metadata == nil {
		return false
	}
	value, ok := metadata[key]
	if !ok {
		return false
	}
	switch v := value.(type) {
	case bool:
		return v
	case string:
		switch strings.ToLower(strings.TrimSpace(v)) {
		case "1", "true", "yes", "on":
			return true
		}
	case int:
		return v != 0
	case int8:
		return v != 0
	case int16:
		return v != 0
	case int32:
		return v != 0
	case int64:
		return v != 0
	case uint:
		return v != 0
	case uint8:
		return v != 0
	case uint16:
		return v != 0
	case uint32:
		return v != 0
	case uint64:
		return v != 0
	case float32:
		return v != 0
	case float64:
		return v != 0
	case json.Number:
		if parsed, err := v.Int64(); err == nil {
			return parsed != 0
		}
	}
	return false
}

// NotifyExistingAlert re-dispatches a notification for an existing active alert
// Used when activation state changes from pending to active
func (m *Manager) NotifyExistingAlert(alertID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	alert, exists := m.getActiveAlertNoLock(alertID)
	if !exists {
		return
	}

	// Dispatch notification for existing alert while holding lock
	// dispatchAlert expects caller to hold m.mu for checkFlapping safety
	m.dispatchAlert(alert, true)
}

// GetRecentlyResolved returns recently resolved alerts
func (m *Manager) GetRecentlyResolved() []models.ResolvedAlert {
	if m == nil {
		return nil
	}

	m.resolvedMutex.RLock()
	defer m.resolvedMutex.RUnlock()

	resolved := make([]models.ResolvedAlert, 0, len(m.recentlyResolved))
	for _, alert := range m.recentlyResolved {
		exported := cloneAlertForOutput(alert.Alert)
		resolved = append(resolved, models.ResolvedAlert{
			Alert: models.Alert{
				ID:           exported.ID,
				Type:         exported.Type,
				Level:        string(exported.Level),
				ResourceID:   exported.ResourceID,
				ResourceName: exported.ResourceName,
				Node:         exported.Node,
				Instance:     exported.Instance,
				Message:      exported.Message,
				Value:        exported.Value,
				Threshold:    exported.Threshold,
				StartTime:    exported.StartTime,
				Acknowledged: exported.Acknowledged,
				// exported is a deep clone, so the map is already private.
				Metadata: exported.Metadata,
			},
			ResolvedTime: alert.ResolvedTime,
		})
	}
	return resolved
}

// GetResolvedAlert returns a copy of a recently resolved alert by ID.
func (m *Manager) GetResolvedAlert(alertID string) *ResolvedAlert {
	// Write lock: getResolvedAlertNoLock backfills the resolvedAlias map on a
	// canonical-identity miss, so concurrent read-locked callers would race
	// each other on that write.
	m.resolvedMutex.Lock()
	defer m.resolvedMutex.Unlock()

	resolved, ok := m.getResolvedAlertNoLock(alertID)
	if !ok || resolved == nil || resolved.Alert == nil {
		return nil
	}

	return &ResolvedAlert{
		Alert:        cloneAlertForOutput(resolved.Alert),
		ResolvedTime: resolved.ResolvedTime,
	}
}

// GetAlertHistory returns alert history
func (m *Manager) GetAlertHistory(limit int) []Alert {
	return canonicalizeAlertHistoryForOutput(m.historyManager.GetAllHistory(limit))
}

// GetAlertHistorySince returns alert history entries created after the provided time.
func (m *Manager) GetAlertHistorySince(since time.Time, limit int) []Alert {
	if since.IsZero() {
		return m.GetAlertHistory(limit)
	}

	return canonicalizeAlertHistoryForOutput(m.historyManager.GetHistory(since, limit))
}

// ClearAlertHistory clears all alert history
func (m *Manager) ClearAlertHistory() error {
	return m.historyManager.ClearAllHistory()
}

// OnAlertHistory registers a callback to be called when alerts are added to history.
// This enables external systems like pattern detection to track alerts.
func (m *Manager) OnAlertHistory(cb AlertCallback) {
	if m.historyManager != nil {
		m.historyManager.OnAlert(cb)
	}
}
