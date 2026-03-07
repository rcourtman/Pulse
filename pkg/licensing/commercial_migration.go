package licensing

import (
	"errors"
	"strings"
)

type CommercialMigrationSource string
type CommercialMigrationState string
type CommercialMigrationReason string
type CommercialMigrationAction string

const (
	CommercialMigrationSourceV5License CommercialMigrationSource = "v5_license"

	CommercialMigrationStatePending CommercialMigrationState = "pending"
	CommercialMigrationStateFailed  CommercialMigrationState = "failed"

	CommercialMigrationReasonExchangeUnavailable    CommercialMigrationReason = "exchange_unavailable"
	CommercialMigrationReasonExchangeRateLimited    CommercialMigrationReason = "exchange_rate_limited"
	CommercialMigrationReasonExchangeConflict       CommercialMigrationReason = "exchange_conflict"
	CommercialMigrationReasonExchangeInvalid        CommercialMigrationReason = "exchange_invalid"
	CommercialMigrationReasonExchangeMalformed      CommercialMigrationReason = "exchange_malformed"
	CommercialMigrationReasonExchangeRevoked        CommercialMigrationReason = "exchange_revoked"
	CommercialMigrationReasonExchangeNonMigratable  CommercialMigrationReason = "exchange_non_migratable"
	CommercialMigrationReasonExchangeUnsupportedKey CommercialMigrationReason = "exchange_unsupported"

	CommercialMigrationActionRetryActivation  CommercialMigrationAction = "retry_activation"
	CommercialMigrationActionUseV6Activation  CommercialMigrationAction = "use_v6_activation_key"
	CommercialMigrationActionEnterSupportedV5 CommercialMigrationAction = "enter_supported_v5_key"
)

// CommercialMigrationStatus is the explicit v6-owned contract for unresolved
// paid-license migrations entering from pre-v6 commercial state.
type CommercialMigrationStatus struct {
	Source            CommercialMigrationSource `json:"source,omitempty"`
	State             CommercialMigrationState  `json:"state,omitempty"`
	Reason            CommercialMigrationReason `json:"reason,omitempty"`
	RecommendedAction CommercialMigrationAction `json:"recommended_action,omitempty"`
}

func (s *CommercialMigrationStatus) Active() bool {
	return s != nil && strings.TrimSpace(string(s.State)) != ""
}

func NormalizeCommercialMigrationStatus(status *CommercialMigrationStatus) *CommercialMigrationStatus {
	if status == nil {
		return nil
	}

	normalized := *status
	normalized.Source = CommercialMigrationSource(strings.TrimSpace(string(normalized.Source)))
	normalized.State = CommercialMigrationState(strings.TrimSpace(string(normalized.State)))
	normalized.Reason = CommercialMigrationReason(strings.TrimSpace(string(normalized.Reason)))
	normalized.RecommendedAction = CommercialMigrationAction(strings.TrimSpace(string(normalized.RecommendedAction)))

	switch normalized.State {
	case CommercialMigrationStatePending, CommercialMigrationStateFailed:
	default:
		return nil
	}

	if normalized.Source == "" {
		normalized.Source = CommercialMigrationSourceV5License
	}
	if normalized.RecommendedAction == "" {
		switch normalized.State {
		case CommercialMigrationStatePending:
			normalized.RecommendedAction = CommercialMigrationActionRetryActivation
		case CommercialMigrationStateFailed:
			normalized.RecommendedAction = CommercialMigrationActionUseV6Activation
		}
	}

	return &normalized
}

func CloneCommercialMigrationStatus(status *CommercialMigrationStatus) *CommercialMigrationStatus {
	if status == nil {
		return nil
	}
	cloned := *status
	return &cloned
}

// ClassifyLegacyExchangeError converts startup/manual exchange errors into a
// retryable or terminal v6 migration contract.
func ClassifyLegacyExchangeError(err error) *CommercialMigrationStatus {
	if err == nil {
		return nil
	}

	status := &CommercialMigrationStatus{
		Source:            CommercialMigrationSourceV5License,
		State:             CommercialMigrationStatePending,
		Reason:            CommercialMigrationReasonExchangeUnavailable,
		RecommendedAction: CommercialMigrationActionRetryActivation,
	}

	var serverErr *LicenseServerError
	if errors.As(err, &serverErr) {
		switch serverErr.StatusCode {
		case 400:
			status.State = CommercialMigrationStateFailed
			status.Reason = CommercialMigrationReasonExchangeMalformed
			status.RecommendedAction = CommercialMigrationActionEnterSupportedV5
		case 401:
			status.State = CommercialMigrationStateFailed
			status.Reason = CommercialMigrationReasonExchangeInvalid
			status.RecommendedAction = CommercialMigrationActionEnterSupportedV5
		case 403:
			status.State = CommercialMigrationStateFailed
			status.Reason = CommercialMigrationReasonExchangeRevoked
			status.RecommendedAction = CommercialMigrationActionUseV6Activation
		case 409:
			status.State = CommercialMigrationStatePending
			status.Reason = CommercialMigrationReasonExchangeConflict
		case 410:
			status.State = CommercialMigrationStateFailed
			status.Reason = CommercialMigrationReasonExchangeNonMigratable
			status.RecommendedAction = CommercialMigrationActionUseV6Activation
		case 429:
			status.State = CommercialMigrationStatePending
			status.Reason = CommercialMigrationReasonExchangeRateLimited
		default:
			if serverErr.Retryable || serverErr.StatusCode >= 500 {
				status.State = CommercialMigrationStatePending
				status.Reason = CommercialMigrationReasonExchangeUnavailable
			}
		}
		return status
	}

	msg := strings.ToLower(strings.TrimSpace(err.Error()))
	switch {
	case strings.Contains(msg, "not a supported v6 activation key or migratable v5 license"):
		status.State = CommercialMigrationStateFailed
		status.Reason = CommercialMigrationReasonExchangeUnsupportedKey
		status.RecommendedAction = CommercialMigrationActionEnterSupportedV5
	case strings.Contains(msg, "license server client not configured"):
		status.State = CommercialMigrationStatePending
		status.Reason = CommercialMigrationReasonExchangeUnavailable
	}

	return status
}
