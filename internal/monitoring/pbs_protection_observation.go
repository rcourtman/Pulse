package monitoring

import (
	"fmt"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/operationaltrust"
	"github.com/rcourtman/pulse-go-rewrite/internal/recovery"
)

func buildPBSProtectionProviderObservation(
	instanceName string,
	datastoreCount int,
	datastoreFetches int,
	datastoreErrors int,
	datastoreTerminalFailures int,
	observedAt time.Time,
) (recovery.ProtectionProviderObservation, error) {
	instanceName = strings.TrimSpace(instanceName)
	if instanceName == "" {
		return recovery.ProtectionProviderObservation{}, fmt.Errorf(
			"PBS protection observation requires an instance",
		)
	}
	if observedAt.IsZero() {
		return recovery.ProtectionProviderObservation{}, fmt.Errorf(
			"PBS protection observation requires an observation time",
		)
	}

	jobState := recovery.OutcomeSuccess
	historyCompleteness := recovery.ProtectionHistoryComplete
	permissions := operationaltrust.EvidencePermissionsSufficient
	var reason *operationaltrust.EvidenceReason

	switch {
	case datastoreErrors == 0:
		// A successful empty enumeration is complete evidence that the
		// connection currently exposes no backup history.
	case datastoreFetches > 0:
		jobState = recovery.OutcomeWarning
		historyCompleteness = recovery.ProtectionHistoryPartial
		permissions = operationaltrust.EvidencePermissionsUnknown
		reason = &operationaltrust.EvidenceReason{
			Code:    "pbs_partial_enumeration",
			Message: "Some PBS datastore or namespace history could not be enumerated.",
		}
		if datastoreTerminalFailures > 0 {
			permissions = operationaltrust.EvidencePermissionsPartial
			reason = &operationaltrust.EvidenceReason{
				Code:    "pbs_partial_provider_access",
				Message: "PBS authorized only part of the configured backup-history scope.",
			}
		}
	default:
		jobState = recovery.OutcomeFailed
		historyCompleteness = recovery.ProtectionHistoryUnavailable
		permissions = operationaltrust.EvidencePermissionsUnknown
		reason = &operationaltrust.EvidenceReason{
			Code:    "pbs_collection_unavailable",
			Message: "PBS backup history could not be collected; retained points may be stale.",
		}
		if datastoreCount > 0 &&
			datastoreTerminalFailures >= datastoreCount {
			permissions = operationaltrust.EvidencePermissionsDenied
			reason = &operationaltrust.EvidenceReason{
				Code:    "pbs_provider_access_denied",
				Message: "PBS rejected every configured datastore history request.",
			}
		}
	}

	return recovery.NewProtectionProviderObservation(
		recovery.ProviderProxmoxPBS,
		"pbs-backup-enumeration",
		instanceName,
		jobState,
		historyCompleteness,
		permissions,
		true,
		observedAt,
		observedAt,
		reason,
	)
}
