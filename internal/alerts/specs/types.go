package specs

import (
	"fmt"
	"math"
	"slices"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

type AlertSpecKind string

const (
	AlertSpecKindMetricThreshold        AlertSpecKind = "metric-threshold"
	AlertSpecKindConnectivity           AlertSpecKind = "connectivity"
	AlertSpecKindPoweredState           AlertSpecKind = "powered-state"
	AlertSpecKindProviderIncident       AlertSpecKind = "provider-incident"
	AlertSpecKindResourceIncidentRollup AlertSpecKind = "resource-incident-rollup"
	AlertSpecKindServiceGap             AlertSpecKind = "service-gap"
	AlertSpecKindDiscreteState          AlertSpecKind = "discrete-state"
)

func (k AlertSpecKind) valid() bool {
	switch k {
	case AlertSpecKindMetricThreshold,
		AlertSpecKindConnectivity,
		AlertSpecKindPoweredState,
		AlertSpecKindProviderIncident,
		AlertSpecKindResourceIncidentRollup,
		AlertSpecKindServiceGap,
		AlertSpecKindDiscreteState:
		return true
	default:
		return false
	}
}

type AlertSeverity string

const (
	AlertSeverityInfo     AlertSeverity = "info"
	AlertSeverityWarning  AlertSeverity = "warning"
	AlertSeverityCritical AlertSeverity = "critical"
)

func (s AlertSeverity) valid() bool {
	switch s {
	case AlertSeverityInfo, AlertSeverityWarning, AlertSeverityCritical:
		return true
	default:
		return false
	}
}

type ThresholdDirection string

const (
	ThresholdDirectionAbove ThresholdDirection = "above"
	ThresholdDirectionBelow ThresholdDirection = "below"
)

func (d ThresholdDirection) valid() bool {
	switch d {
	case ThresholdDirectionAbove, ThresholdDirectionBelow:
		return true
	default:
		return false
	}
}

type PowerState string

const (
	PowerStateOn        PowerState = "on"
	PowerStateOff       PowerState = "off"
	PowerStateSuspended PowerState = "suspended"
	PowerStateUnknown   PowerState = "unknown"
)

func (s PowerState) valid() bool {
	switch s {
	case PowerStateOn, PowerStateOff, PowerStateSuspended, PowerStateUnknown:
		return true
	default:
		return false
	}
}

type AlertState string

const (
	AlertStateClear      AlertState = "clear"
	AlertStatePending    AlertState = "pending"
	AlertStateFiring     AlertState = "firing"
	AlertStateSuppressed AlertState = "suppressed"
)

func (s AlertState) valid() bool {
	switch s {
	case AlertStateClear, AlertStatePending, AlertStateFiring, AlertStateSuppressed:
		return true
	default:
		return false
	}
}

// ResourceAlertSpec is the canonical contract for alerting a unified resource.
type ResourceAlertSpec struct {
	ID                         string                        `json:"id"`
	ResourceID                 string                        `json:"resourceId"`
	ResourceType               unifiedresources.ResourceType `json:"resourceType"`
	ParentResourceID           string                        `json:"parentResourceId,omitempty"`
	ChildResourceIDs           []string                      `json:"childResourceIds,omitempty"`
	SuppressionKeys            []string                      `json:"suppressionKeys,omitempty"`
	Kind                       AlertSpecKind                 `json:"kind"`
	Severity                   AlertSeverity                 `json:"severity"`
	Title                      string                        `json:"title,omitempty"`
	Disabled                   bool                          `json:"disabled,omitempty"`
	ConfirmationsRequired      int                           `json:"confirmationsRequired,omitempty"`
	SuppressOnConnectivityLoss bool                          `json:"suppressOnConnectivityLoss,omitempty"`

	MetricThreshold        *MetricThresholdSpec        `json:"metricThreshold,omitempty"`
	Connectivity           *ConnectivitySpec           `json:"connectivity,omitempty"`
	PoweredState           *PoweredStateSpec           `json:"poweredState,omitempty"`
	ProviderIncident       *ProviderIncidentSpec       `json:"providerIncident,omitempty"`
	ResourceIncidentRollup *ResourceIncidentRollupSpec `json:"resourceIncidentRollup,omitempty"`
	ServiceGap             *ServiceGapSpec             `json:"serviceGap,omitempty"`
	DiscreteState          *DiscreteStateSpec          `json:"discreteState,omitempty"`
}

func (s ResourceAlertSpec) Validate() error {
	if strings.TrimSpace(s.ID) == "" {
		return fmt.Errorf("spec id is required")
	}
	if strings.TrimSpace(s.ResourceID) == "" {
		return fmt.Errorf("resource id is required")
	}
	if !isKnownResourceType(s.ResourceType) {
		return fmt.Errorf("resource type %q is not a canonical unified resource type", s.ResourceType)
	}
	if !s.Kind.valid() {
		return fmt.Errorf("spec kind %q is invalid", s.Kind)
	}
	if !s.Severity.valid() {
		return fmt.Errorf("severity %q is invalid", s.Severity)
	}
	if s.ConfirmationsRequired < 0 {
		return fmt.Errorf("confirmations required must not be negative")
	}

	payloads := 0
	if s.MetricThreshold != nil {
		payloads++
	}
	if s.Connectivity != nil {
		payloads++
	}
	if s.PoweredState != nil {
		payloads++
	}
	if s.ProviderIncident != nil {
		payloads++
	}
	if s.ResourceIncidentRollup != nil {
		payloads++
	}
	if s.ServiceGap != nil {
		payloads++
	}
	if s.DiscreteState != nil {
		payloads++
	}
	if payloads != 1 {
		return fmt.Errorf("exactly one kind-specific payload is required")
	}

	switch s.Kind {
	case AlertSpecKindMetricThreshold:
		if s.MetricThreshold == nil {
			return fmt.Errorf("metric threshold payload is required")
		}
		return s.MetricThreshold.Validate()
	case AlertSpecKindConnectivity:
		if s.Connectivity == nil {
			return fmt.Errorf("connectivity payload is required")
		}
		return s.Connectivity.Validate()
	case AlertSpecKindPoweredState:
		if s.PoweredState == nil {
			return fmt.Errorf("powered state payload is required")
		}
		return s.PoweredState.Validate()
	case AlertSpecKindProviderIncident:
		if s.ProviderIncident == nil {
			return fmt.Errorf("provider incident payload is required")
		}
		return s.ProviderIncident.Validate()
	case AlertSpecKindResourceIncidentRollup:
		if s.ResourceIncidentRollup == nil {
			return fmt.Errorf("resource incident rollup payload is required")
		}
		return s.ResourceIncidentRollup.Validate()
	case AlertSpecKindServiceGap:
		if s.ServiceGap == nil {
			return fmt.Errorf("service gap payload is required")
		}
		return s.ServiceGap.Validate()
	case AlertSpecKindDiscreteState:
		if s.DiscreteState == nil {
			return fmt.Errorf("discrete state payload is required")
		}
		return s.DiscreteState.Validate()
	default:
		return fmt.Errorf("spec kind %q is invalid", s.Kind)
	}
}

type MetricThresholdSpec struct {
	Metric    string             `json:"metric"`
	Direction ThresholdDirection `json:"direction"`
	Trigger   float64            `json:"trigger"`
	Recovery  *float64           `json:"recovery,omitempty"`
	Window    time.Duration      `json:"window,omitempty"`
}

func (s MetricThresholdSpec) Validate() error {
	if strings.TrimSpace(s.Metric) == "" {
		return fmt.Errorf("metric is required")
	}
	if !s.Direction.valid() {
		return fmt.Errorf("threshold direction %q is invalid", s.Direction)
	}
	if !isFinite(s.Trigger) {
		return fmt.Errorf("trigger must be finite")
	}
	if s.Window < 0 {
		return fmt.Errorf("window must be zero or positive")
	}
	if s.Recovery == nil {
		return nil
	}
	if !isFinite(*s.Recovery) {
		return fmt.Errorf("recovery must be finite")
	}
	switch s.Direction {
	case ThresholdDirectionAbove:
		if *s.Recovery >= s.Trigger {
			return fmt.Errorf("recovery must be below trigger when direction is above")
		}
	case ThresholdDirectionBelow:
		if *s.Recovery <= s.Trigger {
			return fmt.Errorf("recovery must be above trigger when direction is below")
		}
	}
	return nil
}

type ConnectivitySpec struct {
	Signal     string        `json:"signal"`
	LostAfter  time.Duration `json:"lostAfter"`
	GraceAfter time.Duration `json:"graceAfter,omitempty"`
}

func (s ConnectivitySpec) Validate() error {
	if strings.TrimSpace(s.Signal) == "" {
		return fmt.Errorf("signal is required")
	}
	if s.LostAfter <= 0 {
		return fmt.Errorf("lost after must be positive")
	}
	if s.GraceAfter < 0 {
		return fmt.Errorf("grace after must be zero or positive")
	}
	return nil
}

type PoweredStateSpec struct {
	Expected    PowerState    `json:"expected"`
	MismatchFor time.Duration `json:"mismatchFor,omitempty"`
}

func (s PoweredStateSpec) Validate() error {
	if !s.Expected.valid() {
		return fmt.Errorf("expected power state %q is invalid", s.Expected)
	}
	if s.MismatchFor < 0 {
		return fmt.Errorf("mismatch duration must be zero or positive")
	}
	return nil
}

type ProviderIncidentSpec struct {
	Provider  string   `json:"provider"`
	Codes     []string `json:"codes,omitempty"`
	NativeIDs []string `json:"nativeIds,omitempty"`
}

func (s ProviderIncidentSpec) Validate() error {
	if strings.TrimSpace(s.Provider) == "" {
		return fmt.Errorf("provider is required")
	}
	s.Codes = canonicalStringSet(s.Codes)
	s.NativeIDs = canonicalStringSet(s.NativeIDs)
	if len(s.Codes) == 0 && len(s.NativeIDs) == 0 {
		return fmt.Errorf("at least one code or native id is required")
	}
	return nil
}

type ResourceIncidentRollupSpec struct {
	Code          string    `json:"code"`
	IncidentCount int       `json:"incidentCount"`
	StartedAt     time.Time `json:"startedAt,omitempty"`
}

func (s ResourceIncidentRollupSpec) Validate() error {
	if strings.TrimSpace(s.Code) == "" {
		return fmt.Errorf("code is required")
	}
	if s.IncidentCount <= 0 {
		return fmt.Errorf("incident count must be positive")
	}
	return nil
}

type ServiceGapSpec struct {
	Service         string        `json:"service"`
	GapAfter        time.Duration `json:"gapAfter"`
	WarningPercent  float64       `json:"warningPercent,omitempty"`
	CriticalPercent float64       `json:"criticalPercent,omitempty"`
}

func (s ServiceGapSpec) Validate() error {
	if strings.TrimSpace(s.Service) == "" {
		return fmt.Errorf("service is required")
	}
	if s.GapAfter < 0 {
		return fmt.Errorf("gap after must be zero or positive")
	}
	if s.WarningPercent < 0 || s.WarningPercent > 100 {
		return fmt.Errorf("warning percent must be between 0 and 100")
	}
	if s.CriticalPercent < 0 || s.CriticalPercent > 100 {
		return fmt.Errorf("critical percent must be between 0 and 100")
	}
	if s.WarningPercent == 0 && s.CriticalPercent == 0 && s.GapAfter <= 0 {
		return fmt.Errorf("gap after or percentage thresholds are required")
	}
	if s.CriticalPercent > 0 && s.WarningPercent > 0 && s.CriticalPercent < s.WarningPercent {
		return fmt.Errorf("critical percent must be greater than or equal to warning percent")
	}
	return nil
}

type DiscreteStateSpec struct {
	StateKey      string   `json:"stateKey"`
	TriggerStates []string `json:"triggerStates"`
}

func (s DiscreteStateSpec) Validate() error {
	if strings.TrimSpace(s.StateKey) == "" {
		return fmt.Errorf("state key is required")
	}
	if len(canonicalStringSet(s.TriggerStates)) == 0 {
		return fmt.Errorf("at least one trigger state is required")
	}
	return nil
}

type AlertEvidence struct {
	ObservedAt      time.Time         `json:"observedAt"`
	Summary         string            `json:"summary,omitempty"`
	Attributes      map[string]string `json:"attributes,omitempty"`
	ParentConnected *bool             `json:"parentConnected,omitempty"`

	MetricThreshold        *MetricThresholdEvidence        `json:"metricThreshold,omitempty"`
	Connectivity           *ConnectivityEvidence           `json:"connectivity,omitempty"`
	PoweredState           *PoweredStateEvidence           `json:"poweredState,omitempty"`
	ProviderIncident       *ProviderIncidentEvidence       `json:"providerIncident,omitempty"`
	ResourceIncidentRollup *ResourceIncidentRollupEvidence `json:"resourceIncidentRollup,omitempty"`
	ServiceGap             *ServiceGapEvidence             `json:"serviceGap,omitempty"`
	DiscreteState          *DiscreteStateEvidence          `json:"discreteState,omitempty"`
}

func (e AlertEvidence) validateForKind(kind AlertSpecKind) error {
	if e.ObservedAt.IsZero() {
		return fmt.Errorf("observed at is required")
	}

	payloads := 0
	if e.MetricThreshold != nil {
		payloads++
	}
	if e.Connectivity != nil {
		payloads++
	}
	if e.PoweredState != nil {
		payloads++
	}
	if e.ProviderIncident != nil {
		payloads++
	}
	if e.ResourceIncidentRollup != nil {
		payloads++
	}
	if e.ServiceGap != nil {
		payloads++
	}
	if e.DiscreteState != nil {
		payloads++
	}
	if payloads != 1 {
		return fmt.Errorf("exactly one evidence payload is required")
	}

	switch kind {
	case AlertSpecKindMetricThreshold:
		if e.MetricThreshold == nil {
			return fmt.Errorf("metric threshold evidence is required")
		}
		return e.MetricThreshold.Validate()
	case AlertSpecKindConnectivity:
		if e.Connectivity == nil {
			return fmt.Errorf("connectivity evidence is required")
		}
		return e.Connectivity.Validate()
	case AlertSpecKindPoweredState:
		if e.PoweredState == nil {
			return fmt.Errorf("powered state evidence is required")
		}
		return e.PoweredState.Validate()
	case AlertSpecKindProviderIncident:
		if e.ProviderIncident == nil {
			return fmt.Errorf("provider incident evidence is required")
		}
		return e.ProviderIncident.Validate()
	case AlertSpecKindResourceIncidentRollup:
		if e.ResourceIncidentRollup == nil {
			return fmt.Errorf("resource incident rollup evidence is required")
		}
		return e.ResourceIncidentRollup.Validate()
	case AlertSpecKindServiceGap:
		if e.ServiceGap == nil {
			return fmt.Errorf("service gap evidence is required")
		}
		return e.ServiceGap.Validate()
	case AlertSpecKindDiscreteState:
		if e.DiscreteState == nil {
			return fmt.Errorf("discrete state evidence is required")
		}
		return e.DiscreteState.Validate()
	default:
		return fmt.Errorf("spec kind %q is invalid", kind)
	}
}

type MetricThresholdEvidence struct {
	Metric    string             `json:"metric"`
	Direction ThresholdDirection `json:"direction"`
	Observed  float64            `json:"observed"`
	Trigger   float64            `json:"trigger"`
	Recovery  *float64           `json:"recovery,omitempty"`
}

func (e MetricThresholdEvidence) Validate() error {
	return MetricThresholdSpec{
		Metric:    e.Metric,
		Direction: e.Direction,
		Trigger:   e.Trigger,
		Recovery:  e.Recovery,
	}.Validate()
}

type ConnectivityEvidence struct {
	Signal     string        `json:"signal"`
	Connected  bool          `json:"connected"`
	MissingFor time.Duration `json:"missingFor,omitempty"`
}

func (e ConnectivityEvidence) Validate() error {
	if strings.TrimSpace(e.Signal) == "" {
		return fmt.Errorf("signal is required")
	}
	if e.MissingFor < 0 {
		return fmt.Errorf("missing duration must be zero or positive")
	}
	return nil
}

type PoweredStateEvidence struct {
	Expected PowerState `json:"expected"`
	Observed PowerState `json:"observed"`
}

func (e PoweredStateEvidence) Validate() error {
	if !e.Expected.valid() {
		return fmt.Errorf("expected power state %q is invalid", e.Expected)
	}
	if !e.Observed.valid() {
		return fmt.Errorf("observed power state %q is invalid", e.Observed)
	}
	return nil
}

type ProviderIncidentEvidence struct {
	Provider string `json:"provider"`
	NativeID string `json:"nativeId,omitempty"`
	Code     string `json:"code,omitempty"`
	Source   string `json:"source,omitempty"`
}

func (e ProviderIncidentEvidence) Validate() error {
	if strings.TrimSpace(e.Provider) == "" {
		return fmt.Errorf("provider is required")
	}
	if strings.TrimSpace(e.NativeID) == "" && strings.TrimSpace(e.Code) == "" {
		return fmt.Errorf("code or native id is required")
	}
	return nil
}

type ResourceIncidentRollupEvidence struct {
	Code          string `json:"code"`
	IncidentCount int    `json:"incidentCount"`
}

func (e ResourceIncidentRollupEvidence) Validate() error {
	if strings.TrimSpace(e.Code) == "" {
		return fmt.Errorf("code is required")
	}
	if e.IncidentCount <= 0 {
		return fmt.Errorf("incident count must be positive")
	}
	return nil
}

type ServiceGapEvidence struct {
	Service    string        `json:"service"`
	MissingFor time.Duration `json:"missingFor"`
	Desired    int           `json:"desired,omitempty"`
	Running    int           `json:"running,omitempty"`
}

func (e ServiceGapEvidence) Validate() error {
	if strings.TrimSpace(e.Service) == "" {
		return fmt.Errorf("service is required")
	}
	if e.MissingFor < 0 {
		return fmt.Errorf("missing duration must be zero or positive")
	}
	if e.Desired < 0 {
		return fmt.Errorf("desired must not be negative")
	}
	if e.Running < 0 {
		return fmt.Errorf("running must not be negative")
	}
	if e.Desired == 0 && e.MissingFor == 0 {
		return fmt.Errorf("desired/running or missing duration is required")
	}
	return nil
}

type DiscreteStateEvidence struct {
	StateKey string `json:"stateKey"`
	Observed string `json:"observed"`
}

func (e DiscreteStateEvidence) Validate() error {
	if strings.TrimSpace(e.StateKey) == "" {
		return fmt.Errorf("state key is required")
	}
	if strings.TrimSpace(e.Observed) == "" {
		return fmt.Errorf("observed state is required")
	}
	return nil
}

// AlertTransition captures a lifecycle state change for one canonical spec.
type AlertTransition struct {
	SpecID     string        `json:"specId"`
	ResourceID string        `json:"resourceId"`
	Kind       AlertSpecKind `json:"kind"`
	From       AlertState    `json:"from"`
	To         AlertState    `json:"to"`
	At         time.Time     `json:"at"`
	Evidence   AlertEvidence `json:"evidence"`
	Reason     string        `json:"reason,omitempty"`
	Note       string        `json:"note,omitempty"`
}

func (t AlertTransition) Validate() error {
	if strings.TrimSpace(t.SpecID) == "" {
		return fmt.Errorf("spec id is required")
	}
	if strings.TrimSpace(t.ResourceID) == "" {
		return fmt.Errorf("resource id is required")
	}
	if !t.Kind.valid() {
		return fmt.Errorf("spec kind %q is invalid", t.Kind)
	}
	if !t.From.valid() {
		return fmt.Errorf("from state %q is invalid", t.From)
	}
	if !t.To.valid() {
		return fmt.Errorf("to state %q is invalid", t.To)
	}
	if t.From == t.To {
		return fmt.Errorf("transition must change state")
	}
	if t.At.IsZero() {
		return fmt.Errorf("transition time is required")
	}
	return t.Evidence.validateForKind(t.Kind)
}

// OverrideTarget scopes policy to an explicit canonical resource ID, with optional narrowing.
type OverrideTarget struct {
	ResourceID string        `json:"resourceId"`
	SpecID     string        `json:"specId,omitempty"`
	Kind       AlertSpecKind `json:"kind,omitempty"`
}

func (t OverrideTarget) Validate() error {
	if strings.TrimSpace(t.ResourceID) == "" {
		return fmt.Errorf("resource id is required")
	}
	if t.Kind != "" && !t.Kind.valid() {
		return fmt.Errorf("spec kind %q is invalid", t.Kind)
	}
	return nil
}

func (t OverrideTarget) Matches(spec ResourceAlertSpec) bool {
	if strings.TrimSpace(t.ResourceID) == "" {
		return false
	}
	if strings.TrimSpace(t.ResourceID) != strings.TrimSpace(spec.ResourceID) {
		return false
	}
	if strings.TrimSpace(t.SpecID) != "" && strings.TrimSpace(t.SpecID) != strings.TrimSpace(spec.ID) {
		return false
	}
	if t.Kind != "" && t.Kind != spec.Kind {
		return false
	}
	return true
}

type OverridePolicy struct {
	Target    OverrideTarget `json:"target"`
	Disabled  bool           `json:"disabled,omitempty"`
	Reason    string         `json:"reason,omitempty"`
	ExpiresAt *time.Time     `json:"expiresAt,omitempty"`
}

func (p OverridePolicy) Validate() error {
	if err := p.Target.Validate(); err != nil {
		return err
	}
	if p.ExpiresAt != nil && p.ExpiresAt.IsZero() {
		return fmt.Errorf("expires at must be set when provided")
	}
	return nil
}

func isKnownResourceType(rt unifiedresources.ResourceType) bool {
	if rt == "" {
		return false
	}
	// "node", "agent-disk", and "docker-host" remain migration bridges while live alerts are
	// still keyed separately from the canonical unified resource graph.
	if rt != unifiedresources.ResourceType("node") &&
		rt != unifiedresources.ResourceType("agent-disk") &&
		rt != unifiedresources.ResourceType("docker-host") &&
		unifiedresources.CanonicalResourceType(rt) != rt {
		return false
	}
	switch rt {
	case unifiedresources.ResourceTypeAgent,
		unifiedresources.ResourceType("node"),
		unifiedresources.ResourceType("agent-disk"),
		unifiedresources.ResourceType("docker-host"),
		unifiedresources.ResourceTypeVM,
		unifiedresources.ResourceTypeSystemContainer,
		unifiedresources.ResourceTypeAppContainer,
		unifiedresources.ResourceTypeDockerService,
		unifiedresources.ResourceTypeK8sCluster,
		unifiedresources.ResourceTypeK8sNode,
		unifiedresources.ResourceTypePod,
		unifiedresources.ResourceTypeK8sDeployment,
		unifiedresources.ResourceTypeStorage,
		unifiedresources.ResourceTypePBS,
		unifiedresources.ResourceTypePMG,
		unifiedresources.ResourceTypeCeph,
		unifiedresources.ResourceTypePhysicalDisk:
		return true
	default:
		return false
	}
}

func canonicalStringSet(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		out = append(out, trimmed)
	}
	slices.Sort(out)
	if len(out) == 0 {
		return nil
	}
	return out
}

func isFinite(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0)
}
