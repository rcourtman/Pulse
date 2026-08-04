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

	return recovery.NewProtectionProviderObservation(
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
