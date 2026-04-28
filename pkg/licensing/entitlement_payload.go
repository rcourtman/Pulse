package licensing

import (
	"math"
	"time"
)

// EntitlementPayload is the normalized entitlement response for frontend consumption.
// Frontend should use this instead of inferring capabilities from tier names.
type EntitlementPayload struct {
	// Capabilities lists all granted capability keys.
	Capabilities []string `json:"capabilities"`

	// Limits lists quantitative limits with current usage.
	Limits []LimitStatus `json:"limits"`

	// SubscriptionState is the current subscription lifecycle state.
	SubscriptionState string `json:"subscription_state"`

	// UpgradeReasons provides user-actionable upgrade prompts.
	UpgradeReasons []UpgradeReason `json:"upgrade_reasons"`

	// PlanVersion preserves grandfathered terms.
	PlanVersion string `json:"plan_version,omitempty"`

	// Tier is the marketing tier name (for display only, never gate on this).
	Tier string `json:"tier"`

	// TrialExpiresAt is the trial expiration Unix timestamp when in trial state.
	TrialExpiresAt *int64 `json:"trial_expires_at,omitempty"`

	// TrialDaysRemaining is the number of whole or partial days remaining in trial.
	TrialDaysRemaining *int `json:"trial_days_remaining,omitempty"`

	// HostedMode indicates that this server is running in Pulse hosted mode.
	// It is used by the frontend to gate hosted-control-plane-only UI.
	HostedMode bool `json:"hosted_mode"`

	// Valid mirrors the effective license validity for display surfaces.
	Valid bool `json:"valid"`

	// LicensedEmail is the activated license email when available.
	LicensedEmail string `json:"licensed_email,omitempty"`

	// ExpiresAt is the RFC3339 expiration timestamp when available.
	ExpiresAt *string `json:"expires_at,omitempty"`

	// IsLifetime indicates a lifetime entitlement with no expiration.
	IsLifetime bool `json:"is_lifetime"`

	// DaysRemaining is the number of days left until expiration.
	DaysRemaining int `json:"days_remaining"`

	// InGracePeriod indicates whether the entitlement is currently in grace.
	InGracePeriod bool `json:"in_grace_period,omitempty"`

	// GracePeriodEnd is the RFC3339 grace period end timestamp when available.
	GracePeriodEnd *string `json:"grace_period_end,omitempty"`

	// TrialEligible is retained for compatibility with retired self-hosted trial clients.
	// New self-hosted payloads leave it false; active trial state is exposed through
	// SubscriptionState, TrialExpiresAt, and TrialDaysRemaining.
	TrialEligible bool `json:"trial_eligible"`

	// TrialEligibilityReason is retained for compatibility with retired self-hosted trial clients.
	TrialEligibilityReason string `json:"trial_eligibility_reason,omitempty"`

	// MaxHistoryDays is the maximum metrics history retention in days for the current tier.
	MaxHistoryDays int `json:"max_history_days"`

	// OverflowDaysRemaining is set when the onboarding overflow (+1 host) is active.
	// Indicates the number of days remaining in the 14-day overflow window.
	OverflowDaysRemaining *int `json:"overflow_days_remaining,omitempty"`

	// LegacyConnections is retained for response compatibility. Monitored-system
	// enforcement now counts API-backed and agent-backed top-level systems
	// together, so this field is informational only.
	LegacyConnections LegacyConnectionCounts `json:"legacy_connections"`

	// HasMigrationGap is retained for response compatibility. API-backed systems
	// now count toward the same monitored-system cap as agent-backed systems.
	HasMigrationGap bool `json:"has_migration_gap"`

	// CommercialMigration reports unresolved paid-license migration work entering
	// from v5-era commercial state.
	CommercialMigration *CommercialMigrationStatus `json:"commercial_migration,omitempty"`

	// MonitoredSystemContinuity exposes migrated monitored-system continuity
	// state for billing and support-grade plan-limit presentation.
	MonitoredSystemContinuity *MonitoredSystemContinuityStatus `json:"monitored_system_continuity,omitempty"`

	// MonitoredSystemCapacity exposes the canonical monitored-system
	// admission posture so the frontend can distinguish between a hard cap,
	// an admission freeze, and uncapped continuity without inferring that
	// behavior from raw current/limit math.
	MonitoredSystemCapacity *MonitoredSystemCapacityStatus `json:"monitored_system_capacity,omitempty"`
}

// CommercialPosturePayload is the canonical non-billing commercial contract
// for upgrade messaging and monitored-system migration copy.
// It intentionally excludes billing identity, grandfathered plan terms, and
// other full-entitlement details that belong only to billing surfaces.
type CommercialPosturePayload struct {
	// SubscriptionState is the current subscription lifecycle state.
	SubscriptionState string `json:"subscription_state"`

	// UpgradeReasons provides user-actionable upgrade prompts.
	UpgradeReasons []UpgradeReason `json:"upgrade_reasons"`

	// Tier is the marketing tier name (for display only, never gate on this).
	Tier string `json:"tier"`

	// TrialExpiresAt is the trial expiration Unix timestamp when in trial state.
	TrialExpiresAt *int64 `json:"trial_expires_at,omitempty"`

	// TrialDaysRemaining is the number of whole or partial days remaining in trial.
	TrialDaysRemaining *int `json:"trial_days_remaining,omitempty"`

	// TrialEligible is retained for compatibility with retired self-hosted trial clients.
	TrialEligible bool `json:"trial_eligible"`

	// TrialEligibilityReason is retained for compatibility with retired self-hosted trial clients.
	TrialEligibilityReason string `json:"trial_eligibility_reason,omitempty"`

	// OverflowDaysRemaining is set when the onboarding overflow (+1 host) is active.
	OverflowDaysRemaining *int `json:"overflow_days_remaining,omitempty"`

	// LegacyConnections is retained for response compatibility. Monitored-system
	// enforcement now counts API-backed and agent-backed top-level systems
	// together, so this field is informational only.
	LegacyConnections LegacyConnectionCounts `json:"legacy_connections"`

	// HasMigrationGap is retained for response compatibility. API-backed systems
	// now count toward the same monitored-system cap as agent-backed systems.
	HasMigrationGap bool `json:"has_migration_gap"`

	// CommercialMigration reports unresolved paid-license migration work entering
	// from v5-era commercial state.
	CommercialMigration *CommercialMigrationStatus `json:"commercial_migration,omitempty"`

	// MonitoredSystemCapacity exposes the canonical monitored-system admission
	// posture without exposing billing identity or plan-term internals.
	MonitoredSystemCapacity *MonitoredSystemCapacityStatus `json:"monitored_system_capacity,omitempty"`
}

// RuntimeCapabilitiesPayload is the canonical non-commercial license contract
// for feature gating and runtime retention/limit decisions.
type RuntimeCapabilitiesPayload struct {
	// Capabilities lists all granted capability keys.
	Capabilities []string `json:"capabilities"`

	// Limits lists quantitative limits with current usage.
	Limits []LimitStatus `json:"limits"`

	// HostedMode indicates that this server is running in Pulse hosted mode.
	HostedMode bool `json:"hosted_mode"`

	// MaxHistoryDays is the maximum metrics history retention in days for the current tier.
	MaxHistoryDays int `json:"max_history_days"`

	// MonitoredSystemCapacity exposes the canonical monitored-system runtime
	// posture for warning banners and admission-freeze UX.
	MonitoredSystemCapacity *MonitoredSystemCapacityStatus `json:"monitored_system_capacity,omitempty"`
}

// LimitStatus represents a quantitative limit with current usage state.
type LimitStatus struct {
	// Key is the limit identifier (e.g., "max_monitored_systems").
	Key string `json:"key"`

	// Limit is the maximum allowed value (0 = unlimited).
	Limit int64 `json:"limit"`

	// Current is the observed current usage.
	Current int64 `json:"current"`

	// CurrentAvailable reports whether Current reflects a resolved runtime
	// usage value rather than an unavailable best-effort fallback.
	CurrentAvailable *bool `json:"current_available,omitempty"`

	// CurrentUnavailableReason explains why Current is unavailable when
	// CurrentAvailable is false.
	CurrentUnavailableReason string `json:"current_unavailable_reason,omitempty"`

	// State describes the over-limit UX state.
	// Values: "ok", "warning", "enforced"
	State string `json:"state"`
}

// MonitoredSystemCapacityStatus describes the canonical monitored-system
// admission posture. It makes explicit that Pulse blocks net-new monitored
// systems at or above the plan limit while keeping already-counted systems
// visible and reporting.
type MonitoredSystemCapacityStatus struct {
	// Mode is the canonical monitored-system capacity posture.
	// Values: "usage_unavailable", "unlimited", "within_limit",
	// "at_limit_blocking_new", "over_limit_frozen"
	Mode string `json:"mode"`

	// Urgency mirrors the user-facing severity of the current posture.
	// Values: "ok", "warning", "enforced"
	Urgency string `json:"urgency"`

	// Current is the observed current monitored-system usage.
	Current int64 `json:"current"`

	// Limit is the plan limit for monitored systems (0 = unlimited).
	Limit int64 `json:"limit"`

	// CurrentAvailable reports whether Current reflects a resolved runtime
	// usage value rather than an unavailable best-effort fallback.
	CurrentAvailable bool `json:"current_available"`

	// CurrentUnavailableReason explains why Current is unavailable when
	// CurrentAvailable is false.
	CurrentUnavailableReason string `json:"current_unavailable_reason,omitempty"`

	// AvailableSlots reports how many net-new monitored systems can be added
	// before the plan blocks additional admissions.
	AvailableSlots int64 `json:"available_slots"`

	// Overage reports how far above the current plan limit this installation
	// is while existing monitoring continues.
	Overage int64 `json:"overage"`

	// Reason explains why the current monitored-system posture is legitimate.
	// Values: "limit_reached", "preexisting_usage",
	// "legacy_migration_capture_pending"
	Reason string `json:"reason,omitempty"`

	// BlocksNewSystems indicates that Pulse will reject net-new monitored
	// systems until capacity is freed or the plan changes.
	BlocksNewSystems bool `json:"blocks_new_systems"`

	// ExistingMonitoringContinues indicates that already-counted monitored
	// systems remain visible and reporting under the current posture.
	ExistingMonitoringContinues bool `json:"existing_monitoring_continues"`
}

// UpgradeReason provides context for why a user should upgrade.
type UpgradeReason struct {
	// Key is the capability or limit this reason relates to.
	Key string `json:"key"`

	// Reason is a user-facing description of why upgrading helps.
	Reason string `json:"reason"`

	// ActionURL is where the user can go to upgrade.
	ActionURL string `json:"action_url,omitempty"`
}

type LegacyConnectionCounts struct {
	ProxmoxNodes       int64 `json:"proxmox_nodes"`
	DockerHosts        int64 `json:"docker_hosts"`
	KubernetesClusters int64 `json:"kubernetes_clusters"`
}

func cloneCapabilityKeys(values []string) []string {
	if len(values) == 0 {
		return []string{}
	}
	return append([]string(nil), values...)
}

func cloneLimitStatuses(values []LimitStatus) []LimitStatus {
	if len(values) == 0 {
		return []LimitStatus{}
	}
	return append([]LimitStatus(nil), values...)
}

func (c LegacyConnectionCounts) Total() int64 {
	return c.ProxmoxNodes + c.DockerHosts + c.KubernetesClusters
}

type EntitlementUsageSnapshot struct {
	MonitoredSystems                  int64
	MonitoredSystemsAvailable         bool
	MonitoredSystemsUnavailableReason string
	// Nodes is retained only as a deprecated compatibility field. V6 monitored-
	// system billing must be backed by an explicit canonical availability signal.
	Nodes             int64
	Guests            int64
	LegacyConnections LegacyConnectionCounts
}

func (s EntitlementUsageSnapshot) monitoredSystemCount() int64 {
	if !s.MonitoredSystemsAvailable || s.MonitoredSystems < 0 {
		return 0
	}
	return s.MonitoredSystems
}

func (s EntitlementUsageSnapshot) monitoredSystemCountAvailable() bool {
	return s.MonitoredSystemsAvailable
}

func (s EntitlementUsageSnapshot) monitoredSystemCountUnavailableReason() string {
	return s.MonitoredSystemsUnavailableReason
}

// BuildEntitlementPayload constructs the normalized payload from LicenseStatus.
func BuildEntitlementPayload(status *LicenseStatus, subscriptionState string) EntitlementPayload {
	return BuildEntitlementPayloadWithUsage(status, subscriptionState, EntitlementUsageSnapshot{}, nil)
}

// BuildCommercialPosturePayload constructs the canonical non-billing
// commercial posture payload from LicenseStatus.
func BuildCommercialPosturePayload(
	status *LicenseStatus,
	subscriptionState string,
) CommercialPosturePayload {
	return BuildCommercialPosturePayloadWithUsage(
		status,
		subscriptionState,
		EntitlementUsageSnapshot{},
		nil,
	)
}

// BuildRuntimeCapabilitiesPayload constructs the canonical non-commercial
// runtime capability payload from LicenseStatus.
func BuildRuntimeCapabilitiesPayload(
	status *LicenseStatus,
	subscriptionState string,
) RuntimeCapabilitiesPayload {
	return BuildRuntimeCapabilitiesPayloadWithUsage(
		status,
		subscriptionState,
		EntitlementUsageSnapshot{},
	)
}

// BuildRuntimeCapabilitiesPayloadWithUsage constructs the canonical
// non-commercial runtime capability payload from LicenseStatus and observed usage.
func BuildRuntimeCapabilitiesPayloadWithUsage(
	status *LicenseStatus,
	subscriptionState string,
	usage EntitlementUsageSnapshot,
) RuntimeCapabilitiesPayload {
	entitlementPayload := BuildEntitlementPayloadWithUsage(status, subscriptionState, usage, nil)
	return RuntimeCapabilitiesPayload{
		Capabilities:   cloneCapabilityKeys(entitlementPayload.Capabilities),
		Limits:         cloneLimitStatuses(entitlementPayload.Limits),
		HostedMode:     entitlementPayload.HostedMode,
		MaxHistoryDays: entitlementPayload.MaxHistoryDays,
		MonitoredSystemCapacity: cloneMonitoredSystemCapacityStatus(
			entitlementPayload.MonitoredSystemCapacity,
		),
	}
}

// BuildCommercialPosturePayloadWithUsage constructs the canonical non-billing
// commercial posture payload from LicenseStatus and observed usage.
func BuildCommercialPosturePayloadWithUsage(
	status *LicenseStatus,
	subscriptionState string,
	usage EntitlementUsageSnapshot,
	trialEndsAtUnix *int64,
) CommercialPosturePayload {
	return CommercialPosturePayloadFromEntitlementPayload(
		BuildEntitlementPayloadWithUsage(status, subscriptionState, usage, trialEndsAtUnix),
	)
}

// CommercialPosturePayloadFromEntitlementPayload projects the non-billing
// commercial posture fields out of the full entitlement payload.
func CommercialPosturePayloadFromEntitlementPayload(
	payload EntitlementPayload,
) CommercialPosturePayload {
	sanitized := CommercialPosturePayload{
		SubscriptionState:      payload.SubscriptionState,
		UpgradeReasons:         append([]UpgradeReason(nil), payload.UpgradeReasons...),
		Tier:                   payload.Tier,
		TrialExpiresAt:         payload.TrialExpiresAt,
		TrialDaysRemaining:     payload.TrialDaysRemaining,
		TrialEligible:          payload.TrialEligible,
		TrialEligibilityReason: payload.TrialEligibilityReason,
		OverflowDaysRemaining:  payload.OverflowDaysRemaining,
		LegacyConnections:      payload.LegacyConnections,
		HasMigrationGap:        payload.HasMigrationGap,
	}
	if payload.CommercialMigration != nil {
		sanitized.CommercialMigration = CloneCommercialMigrationStatus(payload.CommercialMigration)
	}
	if payload.MonitoredSystemCapacity != nil {
		sanitized.MonitoredSystemCapacity = cloneMonitoredSystemCapacityStatus(
			payload.MonitoredSystemCapacity,
		)
	}
	if sanitized.UpgradeReasons == nil {
		sanitized.UpgradeReasons = []UpgradeReason{}
	}
	return sanitized
}

// BuildEntitlementPayloadWithUsage constructs the normalized payload from LicenseStatus and observed usage.
func BuildEntitlementPayloadWithUsage(
	status *LicenseStatus,
	subscriptionState string,
	usage EntitlementUsageSnapshot,
	trialEndsAtUnix *int64,
) EntitlementPayload {
	if status == nil {
		return EntitlementPayload{
			Capabilities:      []string{},
			Limits:            []LimitStatus{},
			SubscriptionState: string(SubStateExpired),
			UpgradeReasons:    []UpgradeReason{},
			Tier:              string(TierFree),
			MaxHistoryDays:    TierHistoryDays[TierFree],
		}
	}

	maxHistDays := TierHistoryDays[status.Tier]
	if maxHistDays == 0 {
		maxHistDays = TierHistoryDays[TierFree]
	}

	payload := EntitlementPayload{
		Capabilities:      FilterPublicCapabilities(status.Features),
		Limits:            []LimitStatus{},
		PlanVersion:       status.PlanVersion,
		Tier:              string(status.Tier),
		UpgradeReasons:    []UpgradeReason{},
		Valid:             status.Valid,
		LicensedEmail:     status.Email,
		ExpiresAt:         status.ExpiresAt,
		IsLifetime:        status.IsLifetime,
		DaysRemaining:     status.DaysRemaining,
		InGracePeriod:     status.InGracePeriod,
		GracePeriodEnd:    status.GracePeriodEnd,
		MaxHistoryDays:    maxHistDays,
		LegacyConnections: usage.LegacyConnections,
		HasMigrationGap:   false,
	}
	if status.MonitoredSystemContinuity != nil {
		continuity := *status.MonitoredSystemContinuity
		payload.MonitoredSystemContinuity = &continuity
	}
	payload.MonitoredSystemCapacity = buildMonitoredSystemCapacityStatus(
		int64(status.MaxMonitoredSystems),
		usage,
		status.MonitoredSystemContinuity,
	)

	if payload.Capabilities == nil {
		payload.Capabilities = []string{}
	}

	// Use provided subscription state when present; otherwise derive from status.
	if subscriptionState == "" {
		subState := SubStateActive
		if !status.Valid {
			subState = SubStateExpired
		} else if status.InGracePeriod {
			subState = SubStateGrace
		}
		subscriptionState = string(subState)
	}
	payload.SubscriptionState = string(GetBehavior(SubscriptionState(subscriptionState)).State)

	if payload.SubscriptionState == string(SubStateTrial) {
		applyTrialWindow(&payload, status, trialEndsAtUnix, time.Now().Unix())
	}

	// When subscription state doesn't grant paid features, cap history to free tier.
	if !subscriptionStateHasPaidFeatures(SubscriptionState(payload.SubscriptionState)) {
		payload.MaxHistoryDays = TierHistoryDays[TierFree]
	}

	// Build limits.
	if status.MaxMonitoredSystems > 0 {
		currentSystems := usage.monitoredSystemCount()
		limit := LimitStatus{
			Key:              MaxMonitoredSystemsLicenseGateKey,
			Limit:            int64(status.MaxMonitoredSystems),
			Current:          currentSystems,
			CurrentAvailable: boolPointer(usage.monitoredSystemCountAvailable()),
			State:            LimitState(currentSystems, int64(status.MaxMonitoredSystems)),
		}
		if !usage.monitoredSystemCountAvailable() {
			limit.CurrentUnavailableReason = usage.monitoredSystemCountUnavailableReason()
		}
		payload.Limits = append(payload.Limits, limit)
	}
	if status.MaxGuests > 0 {
		payload.Limits = append(payload.Limits, LimitStatus{
			Key:     "max_guests",
			Limit:   int64(status.MaxGuests),
			Current: usage.Guests,
			State:   LimitState(usage.Guests, int64(status.MaxGuests)),
		})
	}

	reasons := GenerateUpgradeReasons(payload.Capabilities)
	payload.UpgradeReasons = make([]UpgradeReason, 0, len(reasons))
	for _, reason := range reasons {
		payload.UpgradeReasons = append(payload.UpgradeReasons, UpgradeReason{
			Key:       reason.Feature,
			Reason:    reason.Reason,
			ActionURL: reason.ActionURL,
		})
	}

	return payload
}

func applyTrialWindow(payload *EntitlementPayload, status *LicenseStatus, trialEndsAtUnix *int64, nowUnix int64) {
	if payload == nil || status == nil {
		return
	}
	// Prefer billing-state trial timestamps (hosted/self-hosted trial) over license ExpiresAt.
	if trialEndsAtUnix != nil {
		expiresAtUnix := *trialEndsAtUnix
		payload.TrialExpiresAt = &expiresAtUnix
		daysRemaining := remainingTrialDays(expiresAtUnix, nowUnix)
		payload.TrialDaysRemaining = &daysRemaining
		return
	}
	if status.ExpiresAt == nil {
		return
	}
	expiresAt, err := time.Parse(time.RFC3339, *status.ExpiresAt)
	if err != nil {
		return
	}
	expiresAtUnix := expiresAt.Unix()
	payload.TrialExpiresAt = &expiresAtUnix
	daysRemaining := remainingTrialDays(expiresAtUnix, nowUnix)
	payload.TrialDaysRemaining = &daysRemaining
}

func remainingTrialDays(expiresAtUnix, nowUnix int64) int {
	daysRemaining := int(math.Ceil(float64(expiresAtUnix-nowUnix) / 86400.0))
	if daysRemaining < 0 {
		daysRemaining = 0
	}
	return daysRemaining
}

func boolPointer(value bool) *bool {
	v := value
	return &v
}

func buildMonitoredSystemCapacityStatus(
	limit int64,
	usage EntitlementUsageSnapshot,
	continuity *MonitoredSystemContinuityStatus,
) *MonitoredSystemCapacityStatus {
	currentAvailable := usage.monitoredSystemCountAvailable()
	if !currentAvailable {
		return &MonitoredSystemCapacityStatus{
			Mode:                        "usage_unavailable",
			Urgency:                     "ok",
			Current:                     0,
			Limit:                       limit,
			CurrentAvailable:            false,
			CurrentUnavailableReason:    usage.monitoredSystemCountUnavailableReason(),
			AvailableSlots:              0,
			Overage:                     0,
			BlocksNewSystems:            false,
			ExistingMonitoringContinues: false,
		}
	}

	current := usage.monitoredSystemCount()
	if limit <= 0 {
		return &MonitoredSystemCapacityStatus{
			Mode:                        "unlimited",
			Urgency:                     "ok",
			Current:                     current,
			Limit:                       0,
			CurrentAvailable:            true,
			AvailableSlots:              0,
			Overage:                     0,
			BlocksNewSystems:            false,
			ExistingMonitoringContinues: true,
		}
	}

	status := &MonitoredSystemCapacityStatus{
		Current:                     current,
		Limit:                       limit,
		CurrentAvailable:            true,
		AvailableSlots:              0,
		Overage:                     0,
		BlocksNewSystems:            false,
		ExistingMonitoringContinues: true,
		Urgency:                     LimitState(current, limit),
	}

	switch {
	case current < limit:
		status.Mode = "within_limit"
		status.AvailableSlots = limit - current
	case current == limit:
		status.Mode = "at_limit_blocking_new"
		status.Reason = "limit_reached"
		status.BlocksNewSystems = true
	case current > limit:
		status.Mode = "over_limit_frozen"
		status.BlocksNewSystems = true
		status.Overage = current - limit
		if continuity != nil && continuity.CapturePending {
			status.Reason = "legacy_migration_capture_pending"
		} else {
			status.Reason = "preexisting_usage"
		}
	default:
		status.Mode = "within_limit"
	}

	return status
}

func cloneMonitoredSystemCapacityStatus(
	status *MonitoredSystemCapacityStatus,
) *MonitoredSystemCapacityStatus {
	if status == nil {
		return nil
	}
	cloned := *status
	return &cloned
}

// LimitState returns the over-limit UX state string.
func LimitState(current, limit int64) string {
	if limit <= 0 {
		return "ok" // unlimited
	}
	if current >= limit {
		return "enforced"
	}
	// For small limits (≤10, but >1), warn at N-1 so users get notice before hitting the wall.
	// For larger limits, use 90% threshold.
	if limit > 1 && limit <= 10 {
		if current >= limit-1 {
			return "warning"
		}
	} else if current*10 >= limit*9 {
		return "warning"
	}
	return "ok"
}
