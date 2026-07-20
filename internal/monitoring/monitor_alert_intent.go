package monitoring

import (
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rs/zerolog/log"
)

const backupIntentEvidenceMaxAge = 5 * time.Minute

type resourceOperatorIntentReader interface {
	GetResourceOperatorState(canonicalID string) (unifiedresources.ResourceOperatorState, bool, error)
}

type resourceIntentIdentityReader interface {
	ResolveCanonicalResourceID(ref string) (string, bool)
}

func (m *Monitor) installOperatorIntentResolver(store ResourceStoreInterface) {
	if m == nil || m.alertManager == nil {
		return
	}
	identityReader, hasIdentityReader := store.(resourceIntentIdentityReader)
	if hasIdentityReader && identityReader != nil {
		m.alertManager.SetResourceIntentIdentityResolver(identityReader.ResolveCanonicalResourceID)
	} else {
		m.alertManager.SetResourceIntentIdentityResolver(nil)
	}
	reader, ok := store.(resourceOperatorIntentReader)
	if !ok || reader == nil {
		m.alertManager.SetOperatorIntentContextResolver(nil)
		return
	}
	m.alertManager.SetOperatorIntentContextResolver(func(resourceID string, _ time.Time) (alerts.OperatorIntentContext, bool) {
		if hasIdentityReader {
			if canonicalID, found := identityReader.ResolveCanonicalResourceID(resourceID); found {
				resourceID = canonicalID
			}
		}
		state, found, err := reader.GetResourceOperatorState(resourceID)
		if err != nil {
			log.Warn().Err(err).Str("resourceID", resourceID).Msg("Failed to read operator state for alert intent")
			return alerts.OperatorIntentContext{}, false
		}
		if !found {
			return alerts.OperatorIntentContext{}, false
		}
		return alerts.OperatorIntentContext{
			IntentionallyOffline: state.IntentionallyOffline,
			MaintenanceStartAt:   state.MaintenanceStartAt,
			MaintenanceEndAt:     state.MaintenanceEndAt,
			MaintenanceReason:    state.MaintenanceReason,
		}, true
	})
}

func (m *Monitor) resolveBackupIntentContext(_ string, instance, node string, vmid int, now time.Time) (alerts.BackupIntentContext, bool) {
	if m == nil || m.state == nil || vmid <= 0 {
		return alerts.BackupIntentContext{}, false
	}
	instance = strings.TrimSpace(instance)
	node = strings.TrimSpace(node)
	if now.IsZero() {
		now = time.Now().UTC()
	}

	for _, task := range m.state.GetSnapshot().PVEBackups.BackupTasks {
		if task.VMID != vmid || (instance != "" && task.Instance != instance) || (node != "" && task.Node != "" && task.Node != node) {
			continue
		}
		if task.ObservedAt.IsZero() || now.Sub(task.ObservedAt) > backupIntentEvidenceMaxAge || task.ObservedAt.After(now.Add(time.Minute)) {
			continue
		}
		status := strings.ToLower(strings.TrimSpace(task.Status))
		if !task.EndTime.IsZero() || status == "ok" || status == "stopped" || status == "error" || status == "warning" {
			continue
		}
		return alerts.BackupIntentContext{
			Active:     true,
			ObservedAt: task.ObservedAt,
			Evidence:   "pve_vzdump_task:" + task.ID,
		}, true
	}
	return alerts.BackupIntentContext{}, false
}
