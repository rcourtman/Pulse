package monitoring

import (
	"fmt"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/operationaltrust"
	"github.com/rcourtman/pulse-go-rewrite/internal/recovery"
)

type pveProtectionCollectionStats struct {
	NodeCount              int
	SuccessfulNodeQueries  int
	BackupStorageCount     int
	ContentSuccesses       int
	ContentFailures        int
	PermissionFailureCount int
	UnreadScopes           []recovery.ProtectionUnreadScope
}

// pveUnreadReads is the unread part of one PVE backup enumeration.
type pveUnreadReads struct {
	nodes       map[string]struct{}                // nodes whose storage list was not read
	local       map[pveLocalBackupStorage]struct{} // node-local storages not read
	shared      map[string]struct{}                // shared storages no node read
	sharedReads map[pveLocalBackupStorage]struct{} // nodes a shared read failed on
	denied      map[pveLocalBackupStorage]struct{} // reads rejected by authorization
}

// pveUnreadProtectionScopes names what a PVE enumeration could not read. A
// shared storage names its node only when exactly one node attempted it.
func pveUnreadProtectionScopes(unread pveUnreadReads) []recovery.ProtectionUnreadScope {
	var scopes []recovery.ProtectionUnreadScope
	for node := range unread.nodes {
		_, denied := unread.denied[pveLocalBackupStorage{node: node}]
		scopes = append(scopes, recovery.ProtectionUnreadScope{
			Kind:         recovery.ProtectionUnreadScopeNode,
			Node:         node,
			AccessDenied: denied,
		})
	}
	for local := range unread.local {
		_, denied := unread.denied[local]
		scopes = append(scopes, recovery.ProtectionUnreadScope{
			Kind:         recovery.ProtectionUnreadScopeStorage,
			Node:         local.node,
			Storage:      local.storage,
			AccessDenied: denied,
		})
	}
	for storage := range unread.shared {
		scope := recovery.ProtectionUnreadScope{
			Kind:    recovery.ProtectionUnreadScopeStorage,
			Storage: storage,
		}
		attempts := 0
		for read := range unread.sharedReads {
			if read.storage != storage {
				continue
			}
			attempts++
			scope.Node = read.node
			if _, denied := unread.denied[read]; denied {
				scope.AccessDenied = true
			}
		}
		if attempts != 1 {
			scope.Node = ""
		}
		scopes = append(scopes, scope)
	}
	return recovery.NormalizeProtectionUnreadScopes(scopes)
}

func buildPVEProtectionProviderObservation(
	instanceName string,
	stats pveProtectionCollectionStats,
	observedAt time.Time,
) (recovery.ProtectionProviderObservation, error) {
	instanceName = strings.TrimSpace(instanceName)
	if instanceName == "" {
		return recovery.ProtectionProviderObservation{}, fmt.Errorf(
			"PVE protection observation requires an instance",
		)
	}
	if observedAt.IsZero() {
		return recovery.ProtectionProviderObservation{}, fmt.Errorf(
			"PVE protection observation requires an observation time",
		)
	}

	jobState := recovery.OutcomeSuccess
	historyCompleteness := recovery.ProtectionHistoryComplete
	permissions := operationaltrust.EvidencePermissionsSufficient
	var reason *operationaltrust.EvidenceReason

	complete := stats.NodeCount > 0 &&
		stats.SuccessfulNodeQueries == stats.NodeCount &&
		stats.ContentFailures == 0
	allBackupContentUnavailable := stats.BackupStorageCount > 0 &&
		stats.ContentSuccesses == 0
	hasUsableHistory := stats.SuccessfulNodeQueries > 0 && !allBackupContentUnavailable

	switch {
	case complete:
		// A successful empty enumeration is complete evidence that this PVE
		// instance currently exposes no backup files.
	case hasUsableHistory:
		jobState = recovery.OutcomeWarning
		historyCompleteness = recovery.ProtectionHistoryPartial
		permissions = operationaltrust.EvidencePermissionsUnknown
		reason = &operationaltrust.EvidenceReason{
			Code:    "pve_partial_enumeration",
			Message: "Some PVE node or storage backup history could not be enumerated.",
		}
		if stats.PermissionFailureCount > 0 {
			permissions = operationaltrust.EvidencePermissionsPartial
			reason = &operationaltrust.EvidenceReason{
				Code:    "pve_partial_provider_access",
				Message: "PVE authorized only part of the configured backup-history scope.",
			}
		}
	default:
		jobState = recovery.OutcomeFailed
		historyCompleteness = recovery.ProtectionHistoryUnavailable
		permissions = operationaltrust.EvidencePermissionsUnknown
		reason = &operationaltrust.EvidenceReason{
			Code:    "pve_collection_unavailable",
			Message: "PVE backup history could not be collected; retained points may be stale.",
		}
		allNodeQueriesDenied := stats.NodeCount > 0 &&
			stats.SuccessfulNodeQueries == 0 &&
			stats.PermissionFailureCount >= stats.NodeCount
		allBackupContentDenied := stats.BackupStorageCount > 0 &&
			stats.ContentSuccesses == 0 &&
			stats.PermissionFailureCount >= stats.BackupStorageCount
		if allNodeQueriesDenied || allBackupContentDenied {
			permissions = operationaltrust.EvidencePermissionsDenied
			reason = &operationaltrust.EvidenceReason{
				Code:    "pve_provider_access_denied",
				Message: "PVE rejected every node or storage backup-history request.",
			}
		}
	}

	observation, err := recovery.NewProtectionProviderObservation(
		recovery.ProviderProxmoxPVE,
		"pve-backup-enumeration",
		instanceName,
		jobState,
		historyCompleteness,
		permissions,
		false,
		observedAt,
		observedAt,
		reason,
	)
	if err != nil {
		return recovery.ProtectionProviderObservation{}, err
	}
	return observation.WithUnreadScopes(stats.UnreadScopes), nil
}

func isPVEBackupPermissionError(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "403") ||
		strings.Contains(message, "401") ||
		strings.Contains(message, "permission") ||
		strings.Contains(message, "forbidden")
}
