package unifiedresources

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
)

// ResourceCriticality is an operator-set hint about how important a resource
// is to the operator. Patrol uses it as a sort-order tiebreaker on findings
// of equal severity — high-criticality resources surface first. Empty string
// is the default ("no operator-set criticality"), distinct from "medium".
type ResourceCriticality string

// ResourceMonitoringMode is the canonical per-resource alerting posture. It
// separates expected availability from an explicit all-signal mute so callers
// do not have to overload one boolean with incompatible meanings.
type ResourceMonitoringMode string

const (
	MonitoringModeNormal          ResourceMonitoringMode = "normal"
	MonitoringModeExpectedOffline ResourceMonitoringMode = "expected_offline"
	MonitoringModeMuted           ResourceMonitoringMode = "muted"
)

// ResourceLifecycleState records whether a provider-owned inventory item is
// still operationally active in Pulse. Retired resources remain discoverable
// because their owning provider may continue to report them, but Pulse removes
// them from alert attention and refuses automated remediation.
type ResourceLifecycleState string

const (
	LifecycleStateActive  ResourceLifecycleState = "active"
	LifecycleStateRetired ResourceLifecycleState = "retired"
)

const (
	CriticalityHigh   ResourceCriticality = "high"
	CriticalityMedium ResourceCriticality = "medium"
	CriticalityLow    ResourceCriticality = "low"
)

// AutoRemediationWindow is an optional recurring daily window in an IANA
// timezone. StartMinute is inclusive and EndMinute is exclusive. Windows may
// cross midnight; a nil window means any time. The explicit timezone keeps
// policy evaluation stable across server moves and daylight-saving changes.
type AutoRemediationWindow struct {
	Timezone    string `json:"timezone"`
	StartMinute int    `json:"startMinute"`
	EndMinute   int    `json:"endMinute"`
}

// AutoRemediationPolicy is an optional per-resource narrowing policy. Tenant
// Patrol mode and capability eligibility are the default authority; an empty
// policy inherits those global bounds. When enabled, this policy narrows
// automatic execution to the named capabilities and optional daily window.
// NeverAutoRemediate remains the explicit resource-wide opt-out.
type AutoRemediationPolicy struct {
	Enabled         bool                   `json:"enabled"`
	CapabilityNames []string               `json:"capabilityNames"`
	Window          *AutoRemediationWindow `json:"window,omitempty"`
}

// MaintenanceScope controls whether a window applies only to the resource it
// is stored on or also to resources below it in the canonical inventory tree.
// Descendant propagation is deliberately opt-in: a host maintenance window
// should not silence guests unless the operator chose that scope.
type MaintenanceScope string

const (
	MaintenanceScopeResource               MaintenanceScope = "resource"
	MaintenanceScopeResourceAndDescendants MaintenanceScope = "resource_and_descendants"
)

// RecurringMaintenanceWindow defines a weekly maintenance schedule in an IANA
// timezone. Weekdays name the local day on which an occurrence starts.
// StartMinute is inclusive and EndMinute is exclusive; end may be earlier than
// start to represent an overnight window.
type RecurringMaintenanceWindow struct {
	Timezone    string   `json:"timezone"`
	Weekdays    []string `json:"weekdays"`
	StartMinute int      `json:"startMinute"`
	EndMinute   int      `json:"endMinute"`
}

// MaintenanceWindowOccurrence is one concrete evaluated occurrence. Keeping
// exact boundaries in the canonical model lets Alerts, Patrol, timelines, and
// post-maintenance verification agree on when suppression ends.
type MaintenanceWindowOccurrence struct {
	StartAt time.Time `json:"startAt"`
	EndAt   time.Time `json:"endAt"`
}

// IsValidCriticality reports whether the value is empty or one of the three
// canonical levels. Empty is valid (operator has not set a hint). Anything
// else is rejected at the API boundary so freeform strings cannot accumulate
// into per-deployment dialects.
func IsValidCriticality(value string) bool {
	switch ResourceCriticality(value) {
	case "", CriticalityHigh, CriticalityMedium, CriticalityLow:
		return true
	}
	return false
}

func IsValidMonitoringMode(value string) bool {
	switch ResourceMonitoringMode(value) {
	case MonitoringModeNormal, MonitoringModeExpectedOffline, MonitoringModeMuted:
		return true
	}
	return false
}

func IsValidLifecycleState(value string) bool {
	switch ResourceLifecycleState(value) {
	case LifecycleStateActive, LifecycleStateRetired:
		return true
	}
	return false
}

// ResourceOperatorState captures operator-set per-resource intent that
// modulates Patrol's behavior on findings against this resource. The shape
// is intentionally narrow: every field encodes a specific operator intent
// (intentionally offline, never auto-remediate, maintenance window,
// criticality hint) rather than a freeform metadata bag, so the
// finding-suppression and severity-weighting logic that consumes this state
// has a fixed contract to honor.
//
// All fields are optional; an empty record is equivalent to the default
// "no operator-set state" posture. Persistence stores must treat the
// canonical_id as the primary key and overwrite the entire record on
// SetResourceOperatorState — there is no per-field merge, so callers who
// want to flip a single flag must read-modify-write.
type ResourceOperatorState struct {
	// CanonicalID is the resource identity this state attaches to. Must
	// match the canonical resource ID format used elsewhere in the
	// unified-resources store; the API boundary trims and rejects empty.
	CanonicalID string `json:"canonicalId"`

	// MonitoringMode is the canonical alert and Patrol posture. Normal keeps
	// all monitoring active, expected_offline suppresses availability noise,
	// and muted suppresses all alert and finding attention while preserving
	// inventory visibility.
	MonitoringMode ResourceMonitoringMode `json:"monitoringMode"`

	// LifecycleState is active by default. Retired is an operator-owned
	// lifecycle decision for resources that remain in provider inventory but
	// should no longer participate in alerting or automated remediation.
	LifecycleState ResourceLifecycleState `json:"lifecycleState"`

	// IntentionallyOffline is the compatibility projection for clients that
	// predate MonitoringMode. NormalizeResourceOperatorState derives it from
	// MonitoringMode, and maps legacy true writes to expected_offline when the
	// new field is absent. Runtime policy must consume MonitoringMode instead.
	IntentionallyOffline bool `json:"intentionallyOffline"`

	// NeverAutoRemediate forbids Patrol from dispatching automated fixes
	// against this resource even under approval policy. The action broker
	// must refuse a dispatch targeting this resource with a stable error
	// ("resource_remediation_locked") rather than silently degrading. The
	// operator must clear the flag to allow remediation.
	NeverAutoRemediate bool `json:"neverAutoRemediate"`

	// AutoRemediationPolicy optionally narrows automatic execution for this
	// resource. An empty policy follows the tenant Patrol mode. It never
	// overrides NeverAutoRemediate, capability eligibility, tenant mode, MFA,
	// dry-run, or any execution-time safety gate.
	AutoRemediationPolicy AutoRemediationPolicy `json:"autoRemediationPolicy"`

	// MaintenanceStartAt and MaintenanceEndAt define a time-bounded
	// suppression window. When now is within [start, end), all findings
	// raised against this resource get auto-acknowledged with
	// reason=maintenance. Both values must be set together; either alone
	// is treated as no window. End must be strictly after Start.
	MaintenanceStartAt *time.Time `json:"maintenanceStartAt,omitempty"`
	MaintenanceEndAt   *time.Time `json:"maintenanceEndAt,omitempty"`

	// MaintenanceRecurrence is the recurring alternative to the one-shot
	// start/end pair. The two forms are mutually exclusive so there is only one
	// schedule to explain and audit for a resource.
	MaintenanceRecurrence *RecurringMaintenanceWindow `json:"maintenanceRecurrence,omitempty"`

	// MaintenanceScope defaults to resource. resource_and_descendants is
	// resolved against the canonical unified-resource parent chain.
	MaintenanceScope MaintenanceScope `json:"maintenanceScope"`

	// MaintenanceReason is freeform operator note attached to the window
	// for audit / Assistant context. Surfaced verbatim in the
	// auto-acknowledge note so the operator can see WHY future findings
	// were quiet during the window.
	MaintenanceReason string `json:"maintenanceReason,omitempty"`

	// Criticality is an operator hint that affects finding sort order on
	// the Patrol surface. Empty = default; CriticalityHigh promotes
	// findings on this resource above same-severity peers; CriticalityLow
	// demotes them. Severity itself is not modified — escalation paths
	// stay deterministic.
	Criticality ResourceCriticality `json:"criticality,omitempty"`

	// Note is a freeform operator explanation surfaced alongside the
	// state on the resource detail surface. Distinct from
	// MaintenanceReason which is window-scoped.
	Note string `json:"note,omitempty"`

	// SetAt and SetBy track who last touched the state for audit. SetAt
	// must be populated on every Set; SetBy may be empty when the state
	// was set by a system path (e.g. a maintenance window completing
	// itself).
	SetAt time.Time `json:"setAt"`
	SetBy string    `json:"setBy,omitempty"`
}

// IsEmpty reports whether the state carries no operator intent — every
// field is at its zero value. Stores may treat an IsEmpty record as
// equivalent to "no entry" for the purpose of the GET API surface, though
// the persistence layer MAY keep an audit row to track that the operator
// explicitly cleared the state.
func (s ResourceOperatorState) IsEmpty() bool {
	normalized := NormalizeResourceOperatorState(s)
	return normalized.MonitoringMode == MonitoringModeNormal &&
		normalized.LifecycleState == LifecycleStateActive &&
		!s.NeverAutoRemediate &&
		!s.AutoRemediationPolicy.Enabled &&
		len(s.AutoRemediationPolicy.CapabilityNames) == 0 &&
		s.AutoRemediationPolicy.Window == nil &&
		s.MaintenanceStartAt == nil &&
		s.MaintenanceEndAt == nil &&
		s.MaintenanceRecurrence == nil &&
		strings.TrimSpace(s.MaintenanceReason) == "" &&
		s.Criticality == "" &&
		strings.TrimSpace(s.Note) == ""
}

// SuppressesAllAttention reports whether every Alerts and Patrol signal is
// outside the resource's current operator-owned monitoring posture.
func (s ResourceOperatorState) SuppressesAllAttention() bool {
	s = NormalizeResourceOperatorState(s)
	return s.MonitoringMode == MonitoringModeMuted || s.LifecycleState == LifecycleStateRetired
}

// ExpectsOffline reports whether availability loss is expected while other
// monitoring signals remain eligible.
func (s ResourceOperatorState) ExpectsOffline() bool {
	s = NormalizeResourceOperatorState(s)
	return s.MonitoringMode == MonitoringModeExpectedOffline || s.SuppressesAllAttention()
}

// BlocksRemediation reports whether automated action is incompatible with the
// operator's explicit state. Retirement is a lifecycle lock even when the
// legacy NeverAutoRemediate flag was not separately set.
func (s ResourceOperatorState) BlocksRemediation() bool {
	s = NormalizeResourceOperatorState(s)
	return s.NeverAutoRemediate || s.LifecycleState == LifecycleStateRetired
}

// IsInMaintenanceAt reports whether `now` falls within the configured
// maintenance window. Returns false when no window is configured, when only
// one of start/end is set (treated as no window), or when end <= start.
func (s ResourceOperatorState) IsInMaintenanceAt(now time.Time) bool {
	_, ok := s.ActiveMaintenanceOccurrenceAt(now)
	return ok
}

// ActiveMaintenanceOccurrenceAt returns the exact one-shot or recurring
// occurrence containing now. It fails closed for malformed schedules; writes
// are still rejected by ValidateResourceOperatorState.
func (s ResourceOperatorState) ActiveMaintenanceOccurrenceAt(now time.Time) (MaintenanceWindowOccurrence, bool) {
	if s.MaintenanceStartAt != nil && s.MaintenanceEndAt != nil &&
		s.MaintenanceEndAt.After(*s.MaintenanceStartAt) &&
		!now.Before(*s.MaintenanceStartAt) && now.Before(*s.MaintenanceEndAt) {
		return MaintenanceWindowOccurrence{StartAt: s.MaintenanceStartAt.UTC(), EndAt: s.MaintenanceEndAt.UTC()}, true
	}
	recurrence := NormalizeRecurringMaintenanceWindow(s.MaintenanceRecurrence)
	if recurrence == nil {
		return MaintenanceWindowOccurrence{}, false
	}
	location, err := time.LoadLocation(recurrence.Timezone)
	if err != nil {
		return MaintenanceWindowOccurrence{}, false
	}
	localNow := now.In(location)
	for _, dayOffset := range []int{0, -1} {
		day := localNow.AddDate(0, 0, dayOffset)
		if !recurringMaintenanceIncludesWeekday(recurrence, day.Weekday()) {
			continue
		}
		start := time.Date(day.Year(), day.Month(), day.Day(), recurrence.StartMinute/60, recurrence.StartMinute%60, 0, 0, location)
		endDay := day
		if recurrence.EndMinute <= recurrence.StartMinute {
			endDay = day.AddDate(0, 0, 1)
		}
		end := time.Date(endDay.Year(), endDay.Month(), endDay.Day(), recurrence.EndMinute/60, recurrence.EndMinute%60, 0, 0, location)
		if !localNow.Before(start) && localNow.Before(end) {
			return MaintenanceWindowOccurrence{StartAt: start.UTC(), EndAt: end.UTC()}, true
		}
	}
	return MaintenanceWindowOccurrence{}, false
}

// MaintenanceOccurrencesEndingBetween returns concrete occurrences whose end
// is in (since, until]. It gives the maintenance sentinel a bounded,
// restart-safe way to verify every recurring window without inventing a
// second scheduler or persisting mutable "last run" state.
func (s ResourceOperatorState) MaintenanceOccurrencesEndingBetween(since, until time.Time) []MaintenanceWindowOccurrence {
	if until.Before(since) {
		return nil
	}
	occurrences := make([]MaintenanceWindowOccurrence, 0)
	if s.MaintenanceStartAt != nil && s.MaintenanceEndAt != nil &&
		s.MaintenanceEndAt.After(*s.MaintenanceStartAt) &&
		s.MaintenanceEndAt.After(since) && !s.MaintenanceEndAt.After(until) {
		occurrences = append(occurrences, MaintenanceWindowOccurrence{StartAt: s.MaintenanceStartAt.UTC(), EndAt: s.MaintenanceEndAt.UTC()})
	}
	recurrence := NormalizeRecurringMaintenanceWindow(s.MaintenanceRecurrence)
	if recurrence == nil {
		return occurrences
	}
	location, err := time.LoadLocation(recurrence.Timezone)
	if err != nil {
		return occurrences
	}
	firstDay := since.In(location).AddDate(0, 0, -1)
	lastDay := until.In(location)
	for day := time.Date(firstDay.Year(), firstDay.Month(), firstDay.Day(), 0, 0, 0, 0, location); !day.After(lastDay); day = day.AddDate(0, 0, 1) {
		if !recurringMaintenanceIncludesWeekday(recurrence, day.Weekday()) {
			continue
		}
		start := time.Date(day.Year(), day.Month(), day.Day(), recurrence.StartMinute/60, recurrence.StartMinute%60, 0, 0, location)
		endDay := day
		if recurrence.EndMinute <= recurrence.StartMinute {
			endDay = day.AddDate(0, 0, 1)
		}
		end := time.Date(endDay.Year(), endDay.Month(), endDay.Day(), recurrence.EndMinute/60, recurrence.EndMinute%60, 0, 0, location)
		if end.After(since) && !end.After(until) {
			occurrences = append(occurrences, MaintenanceWindowOccurrence{StartAt: start.UTC(), EndAt: end.UTC()})
		}
	}
	sort.Slice(occurrences, func(i, j int) bool { return occurrences[i].EndAt.Before(occurrences[j].EndAt) })
	return occurrences
}

// ErrResourceOperatorStateInvalid is returned by stores when the supplied
// state fails validation (empty canonical ID, malformed maintenance window,
// unknown criticality value). The action broker translates it into a 400
// at the API boundary; the audit path treats it as a refused write.
var ErrResourceOperatorStateInvalid = errors.New("resource_operator_state_invalid")

// ValidateResourceOperatorState applies the contract checks the Set path
// must enforce before persisting. Returns nil on a valid record, an
// ErrResourceOperatorStateInvalid-wrapped error on a violation. Validation
// is structural only — operator-set meaning (was the maintenance window
// actually intended? is the note correct?) is the operator's call.
func ValidateResourceOperatorState(state ResourceOperatorState) error {
	state = NormalizeResourceOperatorState(state)
	if strings.TrimSpace(state.CanonicalID) == "" {
		return fmt.Errorf("%w: canonical_id is required", ErrResourceOperatorStateInvalid)
	}
	if !IsValidCriticality(string(state.Criticality)) {
		return fmt.Errorf("%w: criticality %q is not one of (high, medium, low, empty)", ErrResourceOperatorStateInvalid, state.Criticality)
	}
	if !IsValidMonitoringMode(string(state.MonitoringMode)) {
		return fmt.Errorf("%w: monitoring_mode %q is not one of (normal, expected_offline, muted)", ErrResourceOperatorStateInvalid, state.MonitoringMode)
	}
	if !IsValidLifecycleState(string(state.LifecycleState)) {
		return fmt.Errorf("%w: lifecycle_state %q is not one of (active, retired)", ErrResourceOperatorStateInvalid, state.LifecycleState)
	}
	if err := ValidateAutoRemediationPolicy(state.AutoRemediationPolicy); err != nil {
		return fmt.Errorf("%w: %v", ErrResourceOperatorStateInvalid, err)
	}
	startSet := state.MaintenanceStartAt != nil
	endSet := state.MaintenanceEndAt != nil
	if startSet != endSet {
		return fmt.Errorf("%w: maintenance window requires both start_at and end_at", ErrResourceOperatorStateInvalid)
	}
	if startSet && endSet {
		if !state.MaintenanceEndAt.After(*state.MaintenanceStartAt) {
			return fmt.Errorf("%w: maintenance end_at must be strictly after start_at", ErrResourceOperatorStateInvalid)
		}
	}
	if startSet && state.MaintenanceRecurrence != nil {
		return fmt.Errorf("%w: one-shot and recurring maintenance windows are mutually exclusive", ErrResourceOperatorStateInvalid)
	}
	if err := ValidateRecurringMaintenanceWindow(state.MaintenanceRecurrence); err != nil {
		return fmt.Errorf("%w: %v", ErrResourceOperatorStateInvalid, err)
	}
	if !IsValidMaintenanceScope(string(state.MaintenanceScope)) {
		return fmt.Errorf("%w: maintenance_scope %q is not one of (resource, resource_and_descendants)", ErrResourceOperatorStateInvalid, state.MaintenanceScope)
	}
	return nil
}

// NormalizeResourceOperatorState applies the canonical trim / default
// behavior expected before persisting. Returns a copy with whitespace
// trimmed on string fields and Criticality coerced to lower-case. Does
// NOT validate — call ValidateResourceOperatorState afterward.
func NormalizeResourceOperatorState(state ResourceOperatorState) ResourceOperatorState {
	state.CanonicalID = strings.TrimSpace(state.CanonicalID)
	state.MonitoringMode = ResourceMonitoringMode(strings.ToLower(strings.TrimSpace(string(state.MonitoringMode))))
	if state.MonitoringMode == "" {
		if state.IntentionallyOffline {
			state.MonitoringMode = MonitoringModeExpectedOffline
		} else {
			state.MonitoringMode = MonitoringModeNormal
		}
	}
	state.IntentionallyOffline = state.MonitoringMode == MonitoringModeExpectedOffline
	state.LifecycleState = ResourceLifecycleState(strings.ToLower(strings.TrimSpace(string(state.LifecycleState))))
	if state.LifecycleState == "" {
		state.LifecycleState = LifecycleStateActive
	}
	state.MaintenanceReason = strings.TrimSpace(state.MaintenanceReason)
	state.MaintenanceScope = MaintenanceScope(strings.ToLower(strings.TrimSpace(string(state.MaintenanceScope))))
	if state.MaintenanceScope == "" {
		state.MaintenanceScope = MaintenanceScopeResource
	}
	state.MaintenanceRecurrence = NormalizeRecurringMaintenanceWindow(state.MaintenanceRecurrence)
	state.Note = strings.TrimSpace(state.Note)
	state.SetBy = strings.TrimSpace(state.SetBy)
	state.Criticality = ResourceCriticality(strings.ToLower(strings.TrimSpace(string(state.Criticality))))
	state.AutoRemediationPolicy = NormalizeAutoRemediationPolicy(state.AutoRemediationPolicy)
	return state
}

func IsValidMaintenanceScope(value string) bool {
	switch MaintenanceScope(value) {
	case MaintenanceScopeResource, MaintenanceScopeResourceAndDescendants:
		return true
	}
	return false
}

var maintenanceWeekdayOrder = map[string]int{
	"monday": 0, "tuesday": 1, "wednesday": 2, "thursday": 3,
	"friday": 4, "saturday": 5, "sunday": 6,
}

// NormalizeRecurringMaintenanceWindow canonicalizes timezone and weekday
// spelling/order without mutating the caller's schedule.
func NormalizeRecurringMaintenanceWindow(window *RecurringMaintenanceWindow) *RecurringMaintenanceWindow {
	if window == nil {
		return nil
	}
	normalized := *window
	normalized.Timezone = strings.TrimSpace(normalized.Timezone)
	seen := make(map[string]struct{}, len(normalized.Weekdays))
	weekdays := make([]string, 0, len(normalized.Weekdays))
	for _, weekday := range normalized.Weekdays {
		weekday = strings.ToLower(strings.TrimSpace(weekday))
		if weekday == "" {
			continue
		}
		if _, exists := seen[weekday]; exists {
			continue
		}
		seen[weekday] = struct{}{}
		weekdays = append(weekdays, weekday)
	}
	sort.Slice(weekdays, func(i, j int) bool {
		left, leftOK := maintenanceWeekdayOrder[weekdays[i]]
		right, rightOK := maintenanceWeekdayOrder[weekdays[j]]
		if leftOK && rightOK {
			return left < right
		}
		if leftOK != rightOK {
			return leftOK
		}
		return weekdays[i] < weekdays[j]
	})
	normalized.Weekdays = weekdays
	return &normalized
}

func ValidateRecurringMaintenanceWindow(window *RecurringMaintenanceWindow) error {
	window = NormalizeRecurringMaintenanceWindow(window)
	if window == nil {
		return nil
	}
	if window.Timezone == "" {
		return errors.New("recurring maintenance requires an IANA timezone")
	}
	if _, err := time.LoadLocation(window.Timezone); err != nil {
		return fmt.Errorf("recurring maintenance timezone %q is invalid", window.Timezone)
	}
	if len(window.Weekdays) == 0 {
		return errors.New("recurring maintenance requires at least one weekday")
	}
	for _, weekday := range window.Weekdays {
		if _, ok := maintenanceWeekdayOrder[weekday]; !ok {
			return fmt.Errorf("recurring maintenance weekday %q is invalid", weekday)
		}
	}
	if window.StartMinute < 0 || window.StartMinute > 1439 || window.EndMinute < 0 || window.EndMinute > 1439 {
		return errors.New("recurring maintenance minutes must be between 0 and 1439")
	}
	if window.StartMinute == window.EndMinute {
		return errors.New("recurring maintenance start and end must differ")
	}
	return nil
}

func recurringMaintenanceIncludesWeekday(window *RecurringMaintenanceWindow, weekday time.Weekday) bool {
	name := strings.ToLower(weekday.String())
	for _, candidate := range window.Weekdays {
		if candidate == name {
			return true
		}
	}
	return false
}

// NormalizeAutoRemediationPolicy returns a deterministic copy for storage,
// hashing, and exact capability matching.
func NormalizeAutoRemediationPolicy(policy AutoRemediationPolicy) AutoRemediationPolicy {
	seen := map[string]struct{}{}
	names := make([]string, 0, len(policy.CapabilityNames))
	for _, name := range policy.CapabilityNames {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		names = append(names, name)
	}
	sort.Strings(names)
	policy.CapabilityNames = names
	if policy.Window != nil {
		window := *policy.Window
		window.Timezone = strings.TrimSpace(window.Timezone)
		policy.Window = &window
	}
	return policy
}

func ValidateAutoRemediationPolicy(policy AutoRemediationPolicy) error {
	policy = NormalizeAutoRemediationPolicy(policy)
	if policy.Enabled && len(policy.CapabilityNames) == 0 {
		return errors.New("enabled auto remediation requires at least one capability")
	}
	if policy.Window == nil {
		return nil
	}
	if policy.Window.Timezone == "" {
		return errors.New("auto remediation window requires an IANA timezone")
	}
	if _, err := time.LoadLocation(policy.Window.Timezone); err != nil {
		return fmt.Errorf("auto remediation timezone %q is invalid", policy.Window.Timezone)
	}
	if policy.Window.StartMinute < 0 || policy.Window.StartMinute > 1439 || policy.Window.EndMinute < 0 || policy.Window.EndMinute > 1439 {
		return errors.New("auto remediation window minutes must be between 0 and 1439")
	}
	if policy.Window.StartMinute == policy.Window.EndMinute {
		return errors.New("auto remediation window start and end must differ")
	}
	return nil
}

// AllowsAutoRemediationAt evaluates the optional per-resource narrowing scope.
// An empty policy inherits the tenant mode and capability class. An explicit
// policy must allow the capability and current time. Resource-wide locks and
// retirement always win.
func (s ResourceOperatorState) AllowsAutoRemediationAt(capabilityName string, now time.Time) bool {
	s = NormalizeResourceOperatorState(s)
	if s.BlocksRemediation() {
		return false
	}
	policy := NormalizeAutoRemediationPolicy(s.AutoRemediationPolicy)
	if !policy.Enabled && len(policy.CapabilityNames) == 0 && policy.Window == nil {
		return true
	}
	if !policy.Enabled {
		return false
	}
	capabilityName = strings.TrimSpace(capabilityName)
	allowed := false
	for _, name := range policy.CapabilityNames {
		if name == capabilityName {
			allowed = true
			break
		}
	}
	if !allowed {
		return false
	}
	if policy.Window == nil {
		return true
	}
	location, err := time.LoadLocation(policy.Window.Timezone)
	if err != nil {
		return false
	}
	local := now.In(location)
	minute := local.Hour()*60 + local.Minute()
	if policy.Window.StartMinute < policy.Window.EndMinute {
		return minute >= policy.Window.StartMinute && minute < policy.Window.EndMinute
	}
	return minute >= policy.Window.StartMinute || minute < policy.Window.EndMinute
}

type resourceOperatorStateLifecycleStore interface {
	SetResourceOperatorStateWithMaintenanceLifecycle(state ResourceOperatorState) (ResourceOperatorState, error)
	ClearResourceOperatorStateWithMaintenanceLifecycle(canonicalID string, observedAt time.Time, actor string) error
}

// SetResourceOperatorStateWithMaintenanceLifecycle persists operator state and
// any derived maintenance-window timeline change through one store-owned write.
func SetResourceOperatorStateWithMaintenanceLifecycle(store ResourceStore, state ResourceOperatorState) (ResourceOperatorState, error) {
	lifecycleStore, ok := store.(resourceOperatorStateLifecycleStore)
	if !ok {
		return ResourceOperatorState{}, errors.New("resource operator state maintenance lifecycle projection requires atomic store support")
	}
	return lifecycleStore.SetResourceOperatorStateWithMaintenanceLifecycle(state)
}

// ClearResourceOperatorStateWithMaintenanceLifecycle clears operator state and
// any derived maintenance-window timeline change through one store-owned write.
func ClearResourceOperatorStateWithMaintenanceLifecycle(store ResourceStore, canonicalID string, observedAt time.Time, actor string) error {
	lifecycleStore, ok := store.(resourceOperatorStateLifecycleStore)
	if !ok {
		return errors.New("resource operator state maintenance lifecycle projection requires atomic store support")
	}
	return lifecycleStore.ClearResourceOperatorStateWithMaintenanceLifecycle(canonicalID, observedAt, actor)
}

const (
	MaintenanceWindowLifecycleEventScheduled = "maintenance_window_scheduled"
	MaintenanceWindowLifecycleEventUpdated   = "maintenance_window_updated"
	MaintenanceWindowLifecycleEventCleared   = "maintenance_window_cleared"

	resourceOperatorStateSourceAdapter ChangeSourceAdapter = "operator_state"
)

type maintenanceWindowLifecycleSnapshot struct {
	start      *time.Time
	end        *time.Time
	recurrence *RecurringMaintenanceWindow
	scope      MaintenanceScope
	reason     string
}

// BuildMaintenanceWindowLifecycleChange returns the canonical resource
// timeline record for a maintenance-window lifecycle transition. It is
// intentionally scoped to the maintenance window fields; other
// operator-state flags have their own product meaning and must not be
// folded into this lifecycle evidence.
func BuildMaintenanceWindowLifecycleChange(previous ResourceOperatorState, previousFound bool, current ResourceOperatorState, currentFound bool, observedAt time.Time, actor string) (ResourceChange, bool) {
	if !previousFound {
		previous = ResourceOperatorState{}
	}
	if !currentFound {
		current = ResourceOperatorState{CanonicalID: previous.CanonicalID}
	}
	canonicalID := CanonicalResourceID(current.CanonicalID)
	if canonicalID == "" {
		canonicalID = CanonicalResourceID(previous.CanonicalID)
	}
	if canonicalID == "" {
		return ResourceChange{}, false
	}

	before, beforeOK := maintenanceWindowSnapshot(previous)
	after, afterOK := maintenanceWindowSnapshot(current)

	event := ""
	switch {
	case !beforeOK && afterOK:
		event = MaintenanceWindowLifecycleEventScheduled
	case beforeOK && afterOK && !before.equal(after):
		event = MaintenanceWindowLifecycleEventUpdated
	case beforeOK && !afterOK:
		event = MaintenanceWindowLifecycleEventCleared
	default:
		return ResourceChange{}, false
	}

	if observedAt.IsZero() {
		observedAt = maintenanceWindowObservedAt(previous, previousFound, current, currentFound)
	} else {
		observedAt = observedAt.UTC()
	}
	actor = strings.TrimSpace(actor)
	if actor == "" && currentFound {
		actor = strings.TrimSpace(current.SetBy)
	}
	if actor == "" && previousFound {
		actor = strings.TrimSpace(previous.SetBy)
	}

	metadata := map[string]any{
		"activityType":        event,
		"operatorStateChange": "maintenance_window_lifecycle",
	}
	if beforeOK {
		if before.start != nil && before.end != nil {
			metadata["previousMaintenanceStartAt"] = before.start.UTC().Format(time.RFC3339)
			metadata["previousMaintenanceEndAt"] = before.end.UTC().Format(time.RFC3339)
		}
		if before.recurrence != nil {
			metadata["previousMaintenanceRecurrence"] = before.recurrence
		}
		metadata["previousMaintenanceScope"] = before.scope
		if before.reason != "" {
			metadata["previousMaintenanceReason"] = before.reason
		}
	}
	if afterOK {
		if after.start != nil && after.end != nil {
			metadata["maintenanceStartAt"] = after.start.UTC().Format(time.RFC3339)
			metadata["maintenanceEndAt"] = after.end.UTC().Format(time.RFC3339)
		}
		if after.recurrence != nil {
			metadata["maintenanceRecurrence"] = after.recurrence
		}
		metadata["maintenanceScope"] = after.scope
		if after.reason != "" {
			metadata["maintenanceReason"] = after.reason
		}
	}

	return ResourceChange{
		ID:            resourceChangeID("resource-operator-state", canonicalID, event, observedAt),
		ObservedAt:    observedAt,
		ResourceID:    canonicalID,
		Kind:          ChangeActivity,
		From:          maintenanceWindowSummary(before, beforeOK),
		To:            maintenanceWindowSummary(after, afterOK),
		SourceType:    SourceUserAction,
		SourceAdapter: resourceOperatorStateSourceAdapter,
		Confidence:    ConfidenceHigh,
		Actor:         actor,
		Reason:        maintenanceWindowLifecycleReason(event),
		Metadata:      metadata,
	}, true
}

func maintenanceWindowSnapshot(state ResourceOperatorState) (maintenanceWindowLifecycleSnapshot, bool) {
	state = NormalizeResourceOperatorState(state)
	if (state.MaintenanceStartAt == nil || state.MaintenanceEndAt == nil) && state.MaintenanceRecurrence == nil {
		return maintenanceWindowLifecycleSnapshot{}, false
	}
	var start, end *time.Time
	if state.MaintenanceStartAt != nil && state.MaintenanceEndAt != nil {
		startUTC := state.MaintenanceStartAt.UTC()
		endUTC := state.MaintenanceEndAt.UTC()
		start = &startUTC
		end = &endUTC
	}
	return maintenanceWindowLifecycleSnapshot{
		start:      start,
		end:        end,
		recurrence: NormalizeRecurringMaintenanceWindow(state.MaintenanceRecurrence),
		scope:      state.MaintenanceScope,
		reason:     strings.TrimSpace(state.MaintenanceReason),
	}, true
}

func (s maintenanceWindowLifecycleSnapshot) equal(other maintenanceWindowLifecycleSnapshot) bool {
	if (s.start == nil) != (other.start == nil) || (s.end == nil) != (other.end == nil) ||
		(s.recurrence == nil) != (other.recurrence == nil) {
		return false
	}
	if s.start != nil && !s.start.Equal(*other.start) {
		return false
	}
	if s.end != nil && !s.end.Equal(*other.end) {
		return false
	}
	if s.recurrence != nil {
		if s.recurrence.Timezone != other.recurrence.Timezone ||
			s.recurrence.StartMinute != other.recurrence.StartMinute ||
			s.recurrence.EndMinute != other.recurrence.EndMinute ||
			strings.Join(s.recurrence.Weekdays, ",") != strings.Join(other.recurrence.Weekdays, ",") {
			return false
		}
	}
	return s.scope == other.scope && s.reason == other.reason
}

func maintenanceWindowObservedAt(previous ResourceOperatorState, previousFound bool, current ResourceOperatorState, currentFound bool) time.Time {
	if currentFound && !current.SetAt.IsZero() {
		return current.SetAt.UTC()
	}
	if previousFound && !previous.SetAt.IsZero() {
		return previous.SetAt.UTC()
	}
	return time.Now().UTC()
}

func maintenanceWindowSummary(window maintenanceWindowLifecycleSnapshot, ok bool) string {
	if !ok {
		return "no maintenance window"
	}
	summary := ""
	if window.start != nil && window.end != nil {
		summary = window.start.UTC().Format(time.RFC3339) + " to " + window.end.UTC().Format(time.RFC3339)
	} else if window.recurrence != nil {
		summary = fmt.Sprintf("%s %s-%s (%s)", strings.Join(window.recurrence.Weekdays, ", "), maintenanceMinuteSummary(window.recurrence.StartMinute), maintenanceMinuteSummary(window.recurrence.EndMinute), window.recurrence.Timezone)
	}
	if window.scope == MaintenanceScopeResourceAndDescendants {
		summary += ", including descendants"
	}
	if window.reason != "" {
		summary += " (" + window.reason + ")"
	}
	return summary
}

func maintenanceMinuteSummary(minute int) string {
	return fmt.Sprintf("%02d:%02d", minute/60, minute%60)
}

func maintenanceWindowLifecycleReason(event string) string {
	switch event {
	case MaintenanceWindowLifecycleEventScheduled:
		return "Maintenance window scheduled"
	case MaintenanceWindowLifecycleEventUpdated:
		return "Maintenance window updated"
	case MaintenanceWindowLifecycleEventCleared:
		return "Maintenance window cleared"
	default:
		return "Maintenance window lifecycle changed"
	}
}

func resourceChangeID(prefix, canonicalID, event string, observedAt time.Time) string {
	if observedAt.IsZero() {
		observedAt = time.Now().UTC()
	}
	return fmt.Sprintf("%s:%s:%s:%d", prefix, sanitizeResourceChangeIDComponent(canonicalID), sanitizeResourceChangeIDComponent(event), observedAt.UTC().UnixNano())
}

func sanitizeResourceChangeIDComponent(value string) string {
	var b strings.Builder
	b.Grow(len(value))
	for _, r := range value {
		switch {
		case (r >= 'a' && r <= 'z'), (r >= 'A' && r <= 'Z'), (r >= '0' && r <= '9'):
			b.WriteRune(r)
		case r == '-' || r == '_' || r == '.':
			b.WriteRune(r)
		default:
			b.WriteRune('_')
		}
	}
	return b.String()
}
