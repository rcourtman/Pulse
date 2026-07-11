package unifiedresources

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"sort"
	"strings"
	"time"
)

const (
	PatrolAutopilotAcknowledgementVersionV1      = 1
	PatrolAutopilotAcknowledgementVersion        = PatrolAutopilotAcknowledgementVersionV1
	PatrolAutopilotAcknowledgementVersionV2      = 2
	PatrolAutopilotCurrentAcknowledgementVersion = PatrolAutopilotAcknowledgementVersionV1
	PatrolAutopilotActivationVersion             = 1
	PatrolAutopilotRevocationVersion             = 1

	PatrolAutopilotStatusActive                   = "active"
	PatrolAutopilotStatusNotRequested             = "not_requested"
	PatrolAutopilotStatusAcknowledgementRequired  = "acknowledgement_required"
	PatrolAutopilotStatusStaleVersion             = "acknowledgement_stale_version"
	PatrolAutopilotStatusWrongOrg                 = "acknowledgement_wrong_org"
	PatrolAutopilotStatusWrongActor               = "acknowledgement_wrong_actor"
	PatrolAutopilotStatusUserRequired             = "acknowledgement_user_required"
	PatrolAutopilotStatusDigestInvalid            = "acknowledgement_digest_invalid"
	PatrolAutopilotStatusExpired                  = "acknowledgement_expired"
	PatrolAutopilotStatusRevoked                  = "acknowledgement_revoked"
	PatrolAutopilotStatusConflict                 = "acknowledgement_conflict"
	PatrolAutopilotStatusActivationDigestInvalid  = "activation_digest_invalid"
	PatrolAutopilotStatusLegacyBooleanIgnored     = "legacy_unlock_ignored"
	PatrolAutopilotStatusStoreUnavailable         = "acknowledgement_store_unavailable"
	PatrolAutopilotStatusActivationRace           = "acknowledgement_activation_raced"
	PatrolAutopilotStatusLicenseRequired          = "license_required"
	PatrolAutopilotScopePolicyAuthorizedActions   = "policy_authorized_actions"
	PatrolAutopilotScopeCapabilityAllowlistedOnly = "capability_allowlisted_only"
	PatrolAutopilotScopeOutcomeTruthNotInferred   = "outcome_truth_not_inferred"
	PatrolAutopilotScopeRevocationVersionBound    = "revocation_and_version_rotation_bound"
)

var patrolAutopilotAcknowledgementIDPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]{7,127}$`)

// PatrolAutopilotAcceptedLimits is the server-owned risk envelope accepted by
// one acknowledgement version. Clients may display it, but cannot author it.
type PatrolAutopilotAcceptedLimits struct {
	PolicyAllowlistRequired             bool `json:"policyAllowlistRequired"`
	EmergencyStopHonored                bool `json:"emergencyStopHonored"`
	ApprovalFloorsHonored               bool `json:"approvalFloorsHonored"`
	VerificationReconciledWhenSupported bool `json:"verificationReconciledWhenSupported"`
	EvidenceClassDisclosed              bool `json:"evidenceClassDisclosed"`
	InconclusiveOutcomeAllowed          bool `json:"inconclusiveOutcomeAllowed"`
	ExecutionSuccessIsNotOutcomeTruth   bool `json:"executionSuccessIsNotOutcomeTruth"`
	ActivationRevocationBound           bool `json:"activationRevocationBound,omitempty"`
}

// PatrolAutopilotAcknowledgementContract is the immutable server-owned risk
// contract for one supported acknowledgement version. Historical records are
// always validated against their own version's entry.
type PatrolAutopilotAcknowledgementContract struct {
	Version        int
	AcceptedScope  []string
	AcceptedLimits PatrolAutopilotAcceptedLimits
	Lifetime       time.Duration
}

func PatrolAutopilotContractForVersion(version int) (PatrolAutopilotAcknowledgementContract, bool) {
	baseScope := []string{
		PatrolAutopilotScopePolicyAuthorizedActions,
		PatrolAutopilotScopeCapabilityAllowlistedOnly,
		PatrolAutopilotScopeOutcomeTruthNotInferred,
	}
	baseLimits := PatrolAutopilotAcceptedLimits{
		PolicyAllowlistRequired:             true,
		EmergencyStopHonored:                true,
		ApprovalFloorsHonored:               true,
		VerificationReconciledWhenSupported: true,
		EvidenceClassDisclosed:              true,
		InconclusiveOutcomeAllowed:          true,
		ExecutionSuccessIsNotOutcomeTruth:   true,
	}
	switch version {
	case PatrolAutopilotAcknowledgementVersionV1:
		return PatrolAutopilotAcknowledgementContract{Version: version, AcceptedScope: baseScope, AcceptedLimits: baseLimits}, true
	case PatrolAutopilotAcknowledgementVersionV2:
		baseScope = append(baseScope, PatrolAutopilotScopeRevocationVersionBound)
		baseLimits.ActivationRevocationBound = true
		return PatrolAutopilotAcknowledgementContract{Version: version, AcceptedScope: baseScope, AcceptedLimits: baseLimits}, true
	default:
		return PatrolAutopilotAcknowledgementContract{}, false
	}
}

func CurrentPatrolAutopilotAcceptedScope() []string {
	contract, _ := PatrolAutopilotContractForVersion(PatrolAutopilotCurrentAcknowledgementVersion)
	return contract.AcceptedScope
}

func CurrentPatrolAutopilotAcceptedLimits() PatrolAutopilotAcceptedLimits {
	contract, _ := PatrolAutopilotContractForVersion(PatrolAutopilotCurrentAcknowledgementVersion)
	return contract.AcceptedLimits
}

// PatrolAutopilotServerPolicy is injected by the server boundary. It is never
// decoded from a public request, which keeps version and lifetime rotation
// deterministic and server-owned.
type PatrolAutopilotServerPolicy struct {
	CurrentVersion int
	Now            time.Time
	Lifetime       time.Duration
}

func CurrentPatrolAutopilotServerPolicy(now time.Time) PatrolAutopilotServerPolicy {
	return PatrolAutopilotServerPolicy{CurrentVersion: PatrolAutopilotCurrentAcknowledgementVersion, Now: now.UTC()}
}

func normalizePatrolAutopilotServerPolicy(policy PatrolAutopilotServerPolicy) (PatrolAutopilotServerPolicy, error) {
	if policy.CurrentVersion <= 0 || policy.Now.IsZero() || policy.Lifetime < 0 {
		return PatrolAutopilotServerPolicy{}, fmt.Errorf("invalid server acknowledgement policy")
	}
	if _, supported := PatrolAutopilotContractForVersion(policy.CurrentVersion); !supported {
		return PatrolAutopilotServerPolicy{}, fmt.Errorf("unsupported server acknowledgement version %d", policy.CurrentVersion)
	}
	policy.Now = policy.Now.UTC()
	return policy, nil
}

// PatrolAutopilotAcknowledgement is immutable evidence that a human accepted
// the current server-owned Autopilot scope and limits.
type PatrolAutopilotAcknowledgement struct {
	Version        int                           `json:"version"`
	ID             string                        `json:"id"`
	OrgID          string                        `json:"orgId"`
	Actor          ActionActor                   `json:"actor"`
	AcceptedScope  []string                      `json:"acceptedScope"`
	AcceptedLimits PatrolAutopilotAcceptedLimits `json:"acceptedLimits"`
	AcceptedAt     time.Time                     `json:"acceptedAt"`
	ExpiresAt      time.Time                     `json:"expiresAt,omitempty"`
	Digest         string                        `json:"digest"`
}

// PatrolAutopilotRevocation is an append-only fact. Revoking activation never
// mutates or deletes the acknowledgement that was originally accepted.
type PatrolAutopilotRevocation struct {
	Version           int         `json:"version"`
	AcknowledgementID string      `json:"acknowledgementId"`
	OrgID             string      `json:"orgId"`
	Actor             ActionActor `json:"actor"`
	Reason            string      `json:"reason,omitempty"`
	RevokedAt         time.Time   `json:"revokedAt"`
	Digest            string      `json:"digest"`
}

// PatrolAutopilotActivation binds a requested full-mode setting to one
// persisted acknowledgement. The binding is replaced only by a new explicit
// activation and is cleared when a lower mode is selected.
type PatrolAutopilotActivation struct {
	Version               int         `json:"version"`
	AcknowledgementID     string      `json:"acknowledgementId"`
	AcknowledgementDigest string      `json:"acknowledgementDigest"`
	OrgID                 string      `json:"orgId"`
	Actor                 ActionActor `json:"actor"`
	ActivatedAt           time.Time   `json:"activatedAt"`
	Digest                string      `json:"digest"`
}

type PatrolAutopilotStatus struct {
	Code                   string                        `json:"code"`
	Active                 bool                          `json:"active"`
	CurrentVersion         int                           `json:"currentVersion"`
	AcknowledgementVersion int                           `json:"acknowledgementVersion,omitempty"`
	AcknowledgementID      string                        `json:"acknowledgementId,omitempty"`
	AcknowledgementDigest  string                        `json:"acknowledgementDigest,omitempty"`
	AcknowledgedBy         string                        `json:"acknowledgedBy,omitempty"`
	AcceptedAt             time.Time                     `json:"acceptedAt,omitempty"`
	ExpiresAt              time.Time                     `json:"expiresAt,omitempty"`
	AcceptedScope          []string                      `json:"acceptedScope"`
	AcceptedLimits         PatrolAutopilotAcceptedLimits `json:"acceptedLimits"`
}

type PatrolAutopilotContractError struct {
	Code string
	Err  error
}

func (e *PatrolAutopilotContractError) Error() string {
	if e == nil {
		return ""
	}
	if e.Err == nil {
		return e.Code
	}
	return e.Code + ": " + e.Err.Error()
}

func (e *PatrolAutopilotContractError) Unwrap() error { return e.Err }

func PatrolAutopilotErrorCode(err error) string {
	var contractErr *PatrolAutopilotContractError
	if errors.As(err, &contractErr) {
		return contractErr.Code
	}
	return ""
}

func patrolAutopilotError(code string, err error) error {
	return &PatrolAutopilotContractError{Code: code, Err: err}
}

func canonicalPatrolAutopilotDigest(value any) string {
	data, err := json.Marshal(value)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func PatrolAutopilotAcknowledgementDigest(record PatrolAutopilotAcknowledgement) string {
	record.Digest = ""
	record.Actor = NormalizeActionActor(record.Actor)
	record.ID = strings.TrimSpace(record.ID)
	record.OrgID = strings.TrimSpace(record.OrgID)
	record.AcceptedAt = record.AcceptedAt.UTC()
	if !record.ExpiresAt.IsZero() {
		record.ExpiresAt = record.ExpiresAt.UTC()
	}
	record.AcceptedScope = append([]string(nil), record.AcceptedScope...)
	sort.Strings(record.AcceptedScope)
	return canonicalPatrolAutopilotDigest(record)
}

func PatrolAutopilotRevocationDigest(record PatrolAutopilotRevocation) string {
	record.Digest = ""
	record.AcknowledgementID = strings.TrimSpace(record.AcknowledgementID)
	record.OrgID = strings.TrimSpace(record.OrgID)
	record.Actor = NormalizeActionActor(record.Actor)
	record.Reason = strings.TrimSpace(record.Reason)
	record.RevokedAt = record.RevokedAt.UTC()
	return canonicalPatrolAutopilotDigest(record)
}

func PatrolAutopilotActivationDigest(record PatrolAutopilotActivation) string {
	record.Digest = ""
	record.AcknowledgementID = strings.TrimSpace(record.AcknowledgementID)
	record.AcknowledgementDigest = strings.TrimSpace(record.AcknowledgementDigest)
	record.OrgID = strings.TrimSpace(record.OrgID)
	record.Actor = NormalizeActionActor(record.Actor)
	record.ActivatedAt = record.ActivatedAt.UTC()
	return canonicalPatrolAutopilotDigest(record)
}

// ValidatePatrolAutopilotStoredEvidence is the single captured-evidence
// validator used before durable config mutation. It validates immutable fact
// shape and binding only; current authorization and version/lifetime policy are
// enforced by the API/runtime boundary.
func ValidatePatrolAutopilotStoredEvidence(acknowledgements []PatrolAutopilotAcknowledgement, revocations []PatrolAutopilotRevocation, activation *PatrolAutopilotActivation) error {
	acknowledgementByID := make(map[string]PatrolAutopilotAcknowledgement, len(acknowledgements))
	for _, record := range acknowledgements {
		normalizedActor := NormalizeActionActor(record.Actor)
		if record.ID != strings.TrimSpace(record.ID) || record.OrgID != strings.TrimSpace(record.OrgID) || record.Actor != normalizedActor || !patrolAutopilotAcknowledgementIDPattern.MatchString(record.ID) {
			return fmt.Errorf("patrol autopilot acknowledgement id is invalid")
		}
		if _, duplicate := acknowledgementByID[record.ID]; duplicate {
			return fmt.Errorf("patrol autopilot acknowledgement id is duplicated")
		}
		contract, supported := PatrolAutopilotContractForVersion(record.Version)
		if !supported {
			return fmt.Errorf("patrol autopilot acknowledgement version is unsupported")
		}
		if err := ValidateActionActor(record.Actor); err != nil || record.Actor.Kind != ActionActorUser || record.Actor.OrgID != record.OrgID {
			return fmt.Errorf("patrol autopilot acknowledgement actor binding is invalid")
		}
		if !slices.Equal(record.AcceptedScope, contract.AcceptedScope) || record.AcceptedLimits != contract.AcceptedLimits {
			return fmt.Errorf("patrol autopilot acknowledgement scope or limits are invalid")
		}
		if record.AcceptedAt.IsZero() || (!record.ExpiresAt.IsZero() && !record.ExpiresAt.After(record.AcceptedAt)) {
			return fmt.Errorf("patrol autopilot acknowledgement validity interval is invalid")
		}
		if record.Digest == "" || record.Digest != PatrolAutopilotAcknowledgementDigest(record) {
			return fmt.Errorf("patrol autopilot acknowledgement digest is invalid")
		}
		acknowledgementByID[record.ID] = record
	}

	revokedIDs := make(map[string]struct{}, len(revocations))
	for _, record := range revocations {
		acknowledgement, found := acknowledgementByID[record.AcknowledgementID]
		if !found {
			return fmt.Errorf("patrol autopilot revocation has no acknowledgement")
		}
		if _, duplicate := revokedIDs[record.AcknowledgementID]; duplicate {
			return fmt.Errorf("patrol autopilot acknowledgement is revoked more than once")
		}
		if err := validatePatrolAutopilotRevocationForAcknowledgement(record, acknowledgement); err != nil {
			return err
		}
		revokedIDs[record.AcknowledgementID] = struct{}{}
	}

	if activation != nil {
		record := *activation
		acknowledgement, found := acknowledgementByID[record.AcknowledgementID]
		if !found {
			return fmt.Errorf("patrol autopilot activation has no acknowledgement")
		}
		normalizedActor := NormalizeActionActor(record.Actor)
		if record.Version != PatrolAutopilotActivationVersion || record.ActivatedAt.IsZero() || record.ActivatedAt.Before(acknowledgement.AcceptedAt) ||
			(!acknowledgement.ExpiresAt.IsZero() && !record.ActivatedAt.Before(acknowledgement.ExpiresAt)) ||
			record.AcknowledgementID != strings.TrimSpace(record.AcknowledgementID) || record.AcknowledgementDigest != strings.TrimSpace(record.AcknowledgementDigest) || record.OrgID != strings.TrimSpace(record.OrgID) || record.Actor != normalizedActor ||
			ValidateActionActor(record.Actor) != nil || record.Actor.Kind != ActionActorUser || record.OrgID != acknowledgement.OrgID || record.Actor.OrgID != acknowledgement.OrgID ||
			!ActionActorsEqual(record.Actor, acknowledgement.Actor) || record.AcknowledgementDigest != acknowledgement.Digest || record.Digest == "" || record.Digest != PatrolAutopilotActivationDigest(record) {
			return fmt.Errorf("patrol autopilot activation binding is invalid")
		}
	}
	return nil
}

func validatePatrolAutopilotRevocationForAcknowledgement(record PatrolAutopilotRevocation, acknowledgement PatrolAutopilotAcknowledgement) error {
	normalizedActor := NormalizeActionActor(record.Actor)
	if record.Version != PatrolAutopilotRevocationVersion || record.RevokedAt.IsZero() || record.RevokedAt.Before(acknowledgement.AcceptedAt) ||
		record.AcknowledgementID != strings.TrimSpace(record.AcknowledgementID) || record.OrgID != strings.TrimSpace(record.OrgID) || record.Reason != strings.TrimSpace(record.Reason) || record.Actor != normalizedActor ||
		ValidateActionActor(record.Actor) != nil || record.Actor.Kind != ActionActorUser || record.OrgID != acknowledgement.OrgID || record.Actor.OrgID != acknowledgement.OrgID ||
		record.Digest == "" || record.Digest != PatrolAutopilotRevocationDigest(record) {
		return fmt.Errorf("patrol autopilot revocation binding is invalid")
	}
	return nil
}

func IssuePatrolAutopilotAcknowledgement(existing []PatrolAutopilotAcknowledgement, acknowledgementID string, actor ActionActor, policy PatrolAutopilotServerPolicy) (PatrolAutopilotAcknowledgement, bool, error) {
	acknowledgementID = strings.TrimSpace(acknowledgementID)
	actor = NormalizeActionActor(actor)
	policy, policyErr := normalizePatrolAutopilotServerPolicy(policy)
	if policyErr != nil {
		return PatrolAutopilotAcknowledgement{}, false, patrolAutopilotError(PatrolAutopilotStatusConflict, policyErr)
	}
	if !patrolAutopilotAcknowledgementIDPattern.MatchString(acknowledgementID) {
		return PatrolAutopilotAcknowledgement{}, false, patrolAutopilotError(PatrolAutopilotStatusConflict, fmt.Errorf("invalid acknowledgement id"))
	}
	if err := ValidateActionActor(actor); err != nil || actor.Kind != ActionActorUser {
		return PatrolAutopilotAcknowledgement{}, false, patrolAutopilotError(PatrolAutopilotStatusUserRequired, err)
	}
	contract, _ := PatrolAutopilotContractForVersion(policy.CurrentVersion)
	expiresAt := time.Time{}
	lifetime := contract.Lifetime
	if policy.Lifetime > 0 {
		lifetime = policy.Lifetime
	}
	if lifetime > 0 {
		expiresAt = policy.Now.Add(lifetime)
	}
	for _, record := range existing {
		if strings.TrimSpace(record.ID) != acknowledgementID {
			continue
		}
		if record.Version == policy.CurrentVersion && record.OrgID == actor.OrgID && ActionActorsEqual(record.Actor, actor) && slices.Equal(record.AcceptedScope, contract.AcceptedScope) && record.AcceptedLimits == contract.AcceptedLimits && record.Digest == PatrolAutopilotAcknowledgementDigest(record) {
			return record, false, nil
		}
		return PatrolAutopilotAcknowledgement{}, false, patrolAutopilotError(PatrolAutopilotStatusConflict, fmt.Errorf("acknowledgement id already bound"))
	}
	record := PatrolAutopilotAcknowledgement{
		Version:        policy.CurrentVersion,
		ID:             acknowledgementID,
		OrgID:          actor.OrgID,
		Actor:          actor,
		AcceptedScope:  append([]string(nil), contract.AcceptedScope...),
		AcceptedLimits: contract.AcceptedLimits,
		AcceptedAt:     policy.Now,
		ExpiresAt:      expiresAt,
	}
	record.Digest = PatrolAutopilotAcknowledgementDigest(record)
	return record, true, nil
}

func RevokePatrolAutopilotAcknowledgement(acknowledgements []PatrolAutopilotAcknowledgement, revocations []PatrolAutopilotRevocation, acknowledgementID string, actor ActionActor, reason string, policy PatrolAutopilotServerPolicy) (PatrolAutopilotRevocation, bool, error) {
	acknowledgementID = strings.TrimSpace(acknowledgementID)
	actor = NormalizeActionActor(actor)
	reason = strings.TrimSpace(reason)
	policy, policyErr := normalizePatrolAutopilotServerPolicy(policy)
	if policyErr != nil {
		return PatrolAutopilotRevocation{}, false, patrolAutopilotError(PatrolAutopilotStatusConflict, policyErr)
	}
	if err := ValidateActionActor(actor); err != nil || actor.Kind != ActionActorUser {
		return PatrolAutopilotRevocation{}, false, patrolAutopilotError(PatrolAutopilotStatusUserRequired, err)
	}
	var acknowledgement *PatrolAutopilotAcknowledgement
	for index := range acknowledgements {
		if acknowledgements[index].ID == acknowledgementID {
			acknowledgement = &acknowledgements[index]
			break
		}
	}
	if acknowledgement == nil || acknowledgement.OrgID != actor.OrgID {
		return PatrolAutopilotRevocation{}, false, patrolAutopilotError(PatrolAutopilotStatusWrongOrg, fmt.Errorf("acknowledgement not found for organization"))
	}
	for _, revocation := range revocations {
		if revocation.AcknowledgementID == acknowledgementID {
			if revocation.Version == PatrolAutopilotRevocationVersion && revocation.OrgID == actor.OrgID && ActionActorsEqual(revocation.Actor, actor) && revocation.Reason == reason && revocation.Digest == PatrolAutopilotRevocationDigest(revocation) {
				return revocation, false, nil
			}
			return PatrolAutopilotRevocation{}, false, patrolAutopilotError(PatrolAutopilotStatusConflict, fmt.Errorf("revocation already bound to different evidence"))
		}
	}
	record := PatrolAutopilotRevocation{
		Version:           PatrolAutopilotRevocationVersion,
		AcknowledgementID: acknowledgementID,
		OrgID:             actor.OrgID,
		Actor:             actor,
		Reason:            reason,
		RevokedAt:         policy.Now,
	}
	record.Digest = PatrolAutopilotRevocationDigest(record)
	return record, true, nil
}

func BindPatrolAutopilotActivation(acknowledgements []PatrolAutopilotAcknowledgement, revocations []PatrolAutopilotRevocation, current *PatrolAutopilotActivation, acknowledgementID string, actor ActionActor, policy PatrolAutopilotServerPolicy) (PatrolAutopilotActivation, bool, error) {
	actor = NormalizeActionActor(actor)
	policy, policyErr := normalizePatrolAutopilotServerPolicy(policy)
	if policyErr != nil {
		return PatrolAutopilotActivation{}, false, patrolAutopilotError(PatrolAutopilotStatusConflict, policyErr)
	}
	if err := ValidateActionActor(actor); err != nil || actor.Kind != ActionActorUser {
		return PatrolAutopilotActivation{}, false, patrolAutopilotError(PatrolAutopilotStatusUserRequired, err)
	}
	acknowledgement, status := ValidatePatrolAutopilotAcknowledgement(acknowledgements, revocations, acknowledgementID, actor.OrgID, &actor, policy)
	if !status.Active {
		return PatrolAutopilotActivation{}, false, patrolAutopilotError(status.Code, fmt.Errorf("acknowledgement is not eligible for activation"))
	}
	if current != nil && current.AcknowledgementID == acknowledgement.ID && current.AcknowledgementDigest == acknowledgement.Digest && current.OrgID == actor.OrgID && ActionActorsEqual(current.Actor, actor) && current.Digest == PatrolAutopilotActivationDigest(*current) {
		return *current, false, nil
	}
	binding := PatrolAutopilotActivation{
		Version:               PatrolAutopilotActivationVersion,
		AcknowledgementID:     acknowledgement.ID,
		AcknowledgementDigest: acknowledgement.Digest,
		OrgID:                 actor.OrgID,
		Actor:                 actor,
		ActivatedAt:           policy.Now,
	}
	binding.Digest = PatrolAutopilotActivationDigest(binding)
	return binding, true, nil
}

func ValidatePatrolAutopilotAcknowledgement(acknowledgements []PatrolAutopilotAcknowledgement, revocations []PatrolAutopilotRevocation, acknowledgementID, orgID string, actor *ActionActor, policy PatrolAutopilotServerPolicy) (PatrolAutopilotAcknowledgement, PatrolAutopilotStatus) {
	policy, policyErr := normalizePatrolAutopilotServerPolicy(policy)
	if policyErr != nil {
		return PatrolAutopilotAcknowledgement{}, newPatrolAutopilotStatus(PatrolAutopilotStatusConflict, PatrolAutopilotCurrentAcknowledgementVersion)
	}
	contract, _ := PatrolAutopilotContractForVersion(policy.CurrentVersion)
	status := newPatrolAutopilotStatus(PatrolAutopilotStatusAcknowledgementRequired, policy.CurrentVersion)
	acknowledgementID = strings.TrimSpace(acknowledgementID)
	orgID = strings.TrimSpace(orgID)
	var record *PatrolAutopilotAcknowledgement
	for index := range acknowledgements {
		if strings.TrimSpace(acknowledgements[index].ID) == acknowledgementID {
			record = &acknowledgements[index]
			break
		}
	}
	if record == nil {
		return PatrolAutopilotAcknowledgement{}, status
	}
	status.AcknowledgementID = record.ID
	status.AcknowledgementVersion = record.Version
	status.AcknowledgementDigest = record.Digest
	status.AcknowledgedBy = record.Actor.SubjectID
	status.AcceptedAt = record.AcceptedAt
	status.ExpiresAt = record.ExpiresAt
	if record.Version != policy.CurrentVersion || !slices.Equal(record.AcceptedScope, contract.AcceptedScope) || record.AcceptedLimits != contract.AcceptedLimits {
		status.Code = PatrolAutopilotStatusStaleVersion
		return *record, status
	}
	if record.OrgID != orgID || record.Actor.OrgID != orgID {
		status.Code = PatrolAutopilotStatusWrongOrg
		return *record, status
	}
	if err := ValidateActionActor(record.Actor); err != nil || record.Actor.Kind != ActionActorUser {
		status.Code = PatrolAutopilotStatusUserRequired
		return *record, status
	}
	if actor != nil && !ActionActorsEqual(record.Actor, NormalizeActionActor(*actor)) {
		status.Code = PatrolAutopilotStatusWrongActor
		return *record, status
	}
	if record.Digest == "" || record.Digest != PatrolAutopilotAcknowledgementDigest(*record) {
		status.Code = PatrolAutopilotStatusDigestInvalid
		return *record, status
	}
	if record.AcceptedAt.IsZero() || record.AcceptedAt.After(policy.Now) || (!record.ExpiresAt.IsZero() && (!record.ExpiresAt.After(record.AcceptedAt) || !policy.Now.Before(record.ExpiresAt))) {
		status.Code = PatrolAutopilotStatusExpired
		return *record, status
	}
	for _, revocation := range revocations {
		if revocation.AcknowledgementID != record.ID {
			continue
		}
		if revocation.OrgID != record.OrgID || NormalizeActionActor(revocation.Actor).OrgID != record.OrgID {
			status.Code = PatrolAutopilotStatusWrongOrg
			return *record, status
		}
		if err := validatePatrolAutopilotRevocationForAcknowledgement(revocation, *record); err != nil {
			status.Code = PatrolAutopilotStatusDigestInvalid
			return *record, status
		}
		status.Code = PatrolAutopilotStatusRevoked
		return *record, status
	}
	status.Code = PatrolAutopilotStatusActive
	status.Active = true
	return *record, status
}

func EvaluatePatrolAutopilot(requestedMode, fallbackMode, orgID string, legacyUnlocked bool, acknowledgements []PatrolAutopilotAcknowledgement, revocations []PatrolAutopilotRevocation, activation *PatrolAutopilotActivation, policy PatrolAutopilotServerPolicy) (string, PatrolAutopilotStatus) {
	policy, policyErr := normalizePatrolAutopilotServerPolicy(policy)
	if policyErr != nil {
		return fallbackMode, newPatrolAutopilotStatus(PatrolAutopilotStatusConflict, PatrolAutopilotCurrentAcknowledgementVersion)
	}
	if requestedMode != "full" {
		return requestedMode, newPatrolAutopilotStatus(PatrolAutopilotStatusNotRequested, policy.CurrentVersion)
	}
	if activation == nil {
		code := PatrolAutopilotStatusAcknowledgementRequired
		if legacyUnlocked {
			code = PatrolAutopilotStatusLegacyBooleanIgnored
		}
		return fallbackMode, newPatrolAutopilotStatus(code, policy.CurrentVersion)
	}
	if activation.Version != PatrolAutopilotActivationVersion || activation.Digest == "" || activation.Digest != PatrolAutopilotActivationDigest(*activation) {
		return fallbackMode, newPatrolAutopilotStatus(PatrolAutopilotStatusActivationDigestInvalid, policy.CurrentVersion)
	}
	if activation.OrgID != strings.TrimSpace(orgID) || activation.Actor.OrgID != strings.TrimSpace(orgID) {
		return fallbackMode, newPatrolAutopilotStatus(PatrolAutopilotStatusWrongOrg, policy.CurrentVersion)
	}
	if err := ValidateActionActor(activation.Actor); err != nil || activation.Actor.Kind != ActionActorUser {
		return fallbackMode, newPatrolAutopilotStatus(PatrolAutopilotStatusUserRequired, policy.CurrentVersion)
	}
	acknowledgement, status := ValidatePatrolAutopilotAcknowledgement(acknowledgements, revocations, activation.AcknowledgementID, orgID, &activation.Actor, policy)
	if !status.Active {
		return fallbackMode, status
	}
	if activation.AcknowledgementDigest != acknowledgement.Digest {
		status.Active = false
		status.Code = PatrolAutopilotStatusDigestInvalid
		return fallbackMode, status
	}
	status.Active = true
	status.Code = PatrolAutopilotStatusActive
	return requestedMode, status
}

func newPatrolAutopilotStatus(code string, currentVersion int) PatrolAutopilotStatus {
	status := PatrolAutopilotStatus{
		Code:           code,
		CurrentVersion: currentVersion,
	}
	if contract, supported := PatrolAutopilotContractForVersion(currentVersion); supported {
		status.AcceptedScope = contract.AcceptedScope
		status.AcceptedLimits = contract.AcceptedLimits
	}
	return status
}
