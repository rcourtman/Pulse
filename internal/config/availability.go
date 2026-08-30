package config

import (
	"fmt"
	"net"
	"net/url"
	"reflect"
	"sort"
	"strconv"
	"strings"

	"github.com/google/uuid"
)

const (
	DefaultAvailabilityPollIntervalSecs        = 60
	DefaultAvailabilityTimeoutMillis           = 2000
	DefaultAvailabilityFailureThreshold        = 2
	DefaultCertificateExpiryWarningDays        = 30
	MaxAvailabilityHTTPBodyBytes               = 8192
	MaxAvailabilityHTTPResponseBytes           = 65536
	AvailabilityObservationLocationLocal       = "pulse:local"
	AvailabilityObservationLocationAgentPrefix = "agent:"
)

type AvailabilityHTTPMethod string

const (
	AvailabilityHTTPMethodHEAD AvailabilityHTTPMethod = "HEAD"
	AvailabilityHTTPMethodGET  AvailabilityHTTPMethod = "GET"
	AvailabilityHTTPMethodPOST AvailabilityHTTPMethod = "POST"
)

type AvailabilityHTTPAuthType string

const (
	AvailabilityHTTPAuthNone   AvailabilityHTTPAuthType = "none"
	AvailabilityHTTPAuthBasic  AvailabilityHTTPAuthType = "basic"
	AvailabilityHTTPAuthBearer AvailabilityHTTPAuthType = "bearer"
)

// AvailabilityHTTPHeader uses a stable ID so API clients can edit a redacted
// header without receiving its stored value. A nil Value means "leave the
// stored value unchanged" on update; an explicit empty string clears it.
type AvailabilityHTTPHeader struct {
	ID    string  `json:"id"`
	Name  string  `json:"name"`
	Value *string `json:"value,omitempty"`
}

// AvailabilityHTTPAuthentication contains execution credentials. Secret
// values are pointers so update requests can distinguish omitted from empty.
// The API must redact them before returning a target to a client.
type AvailabilityHTTPAuthentication struct {
	Type        AvailabilityHTTPAuthType `json:"type"`
	Username    string                   `json:"username,omitempty"`
	Password    *string                  `json:"password,omitempty"`
	BearerToken *string                  `json:"bearerToken,omitempty"`
}

// AvailabilityHTTPConfig is an explicit application response contract. A nil
// config preserves the legacy HEAD-with-GET-fallback reachability semantics.
type AvailabilityHTTPConfig struct {
	Method            AvailabilityHTTPMethod         `json:"method"`
	Headers           []AvailabilityHTTPHeader       `json:"headers,omitempty"`
	Authentication    AvailabilityHTTPAuthentication `json:"authentication"`
	Body              *string                        `json:"body,omitempty"`
	ExpectedStatusMin int                            `json:"expectedStatusMin"`
	ExpectedStatusMax int                            `json:"expectedStatusMax"`
	TextContains      string                         `json:"textContains,omitempty"`
	JSONPath          string                         `json:"jsonPath,omitempty"`
	JSONEquals        string                         `json:"jsonEquals,omitempty"`
}

type AvailabilityProbeProtocol string

const (
	AvailabilityProbeICMP  AvailabilityProbeProtocol = "icmp"
	AvailabilityProbeTCP   AvailabilityProbeProtocol = "tcp"
	AvailabilityProbeHTTP  AvailabilityProbeProtocol = "http"
	AvailabilityProbeHTTPS AvailabilityProbeProtocol = "https"
	AvailabilityProbeUDP   AvailabilityProbeProtocol = "udp"

	availabilityProbePingAlias AvailabilityProbeProtocol = "ping"
)

type AvailabilityUDPMode string

const (
	// AvailabilityUDPResponseRequired requires a datagram response and treats
	// silence as failure. This is the safe default for alerting.
	AvailabilityUDPResponseRequired AvailabilityUDPMode = "response_required"
	// AvailabilityUDPOpenOrFiltered treats silence as an indeterminate,
	// non-failing result; an ICMP port-unreachable response still fails.
	AvailabilityUDPOpenOrFiltered AvailabilityUDPMode = "open_or_filtered"
)

type AvailabilityTargetKind string

const (
	AvailabilityTargetMachine AvailabilityTargetKind = "machine"
	AvailabilityTargetService AvailabilityTargetKind = "service"
	AvailabilityTargetDevice  AvailabilityTargetKind = "device"
)

// AvailabilityTarget represents an agentless endpoint monitored through a
// lightweight availability probe.
type AvailabilityTarget struct {
	ID               string                    `json:"id"`
	ConfigRevision   int64                     `json:"configRevision"`
	Name             string                    `json:"name"`
	TargetKind       AvailabilityTargetKind    `json:"targetKind,omitempty"`
	Address          string                    `json:"address"`
	Protocol         AvailabilityProbeProtocol `json:"protocol"`
	Port             int                       `json:"port,omitempty"`
	Path             string                    `json:"path,omitempty"`
	UDPMode          AvailabilityUDPMode       `json:"udpMode,omitempty"`
	UDPRequest       string                    `json:"udpRequest,omitempty"`
	UDPExpected      string                    `json:"udpExpectedResponse,omitempty"`
	Enabled          bool                      `json:"enabled"`
	PollIntervalSecs int                       `json:"pollIntervalSeconds,omitempty"`
	TimeoutMillis    int                       `json:"timeoutMillis,omitempty"`
	FailureThreshold int                       `json:"failureThreshold,omitempty"`
	LinkedResourceID string                    `json:"linkedResourceId,omitempty"`
	// CertificateMonitoringDisabled is an explicit opt-out. HTTPS targets
	// monitor certificate validity by default so existing saved targets gain
	// the 30-day warning without a migration rewrite.
	CertificateMonitoringDisabled bool `json:"certificateMonitoringDisabled,omitempty"`
	CertificateExpiryWarningDays  int  `json:"certificateExpiryWarningDays,omitempty"`
	// ProbeAgentID assigns execution to a remote host agent. Empty means the
	// check runs from the local Pulse instance. It remains a compatibility field
	// for pre-location clients; ObservationLocationIDs is canonical.
	ProbeAgentID string `json:"probeAgentId,omitempty"`
	// ObservationLocationIDs selects every source-owned path that observes this
	// one logical verification. pulse:local is the Pulse server and agent:<id>
	// names an eligible connected host agent. Names and network details stay out
	// of persisted target configuration and are resolved at presentation time.
	ObservationLocationIDs []string `json:"observationLocationIds,omitempty"`
	// HTTP is absent for legacy targets. Its request body, credentials, and
	// header values are encrypted with the rest of availability target storage
	// and are never returned by the API.
	HTTP *AvailabilityHTTPConfig `json:"http,omitempty"`
}

// NewAvailabilityTarget returns a new target with generated ID and defaults.
func NewAvailabilityTarget() AvailabilityTarget {
	return AvailabilityTarget{
		ID:               uuid.NewString(),
		ConfigRevision:   1,
		TargetKind:       AvailabilityTargetService,
		Protocol:         AvailabilityProbeICMP,
		Enabled:          true,
		PollIntervalSecs: DefaultAvailabilityPollIntervalSecs,
		TimeoutMillis:    DefaultAvailabilityTimeoutMillis,
		FailureThreshold: DefaultAvailabilityFailureThreshold,
	}
}

func (t *AvailabilityTarget) ApplyDefaults() {
	if t == nil {
		return
	}
	if strings.TrimSpace(t.ID) == "" {
		t.ID = uuid.NewString()
	}
	if t.ConfigRevision <= 0 {
		t.ConfigRevision = 1
	}
	if strings.TrimSpace(string(t.Protocol)) == "" {
		t.Protocol = AvailabilityProbeICMP
	} else {
		t.Protocol = normalizeAvailabilityProbeProtocol(t.Protocol)
	}
	if strings.TrimSpace(string(t.TargetKind)) == "" {
		t.TargetKind = AvailabilityTargetService
	}
	if t.PollIntervalSecs <= 0 {
		t.PollIntervalSecs = DefaultAvailabilityPollIntervalSecs
	}
	if t.TimeoutMillis <= 0 {
		t.TimeoutMillis = DefaultAvailabilityTimeoutMillis
	}
	if t.FailureThreshold <= 0 {
		t.FailureThreshold = DefaultAvailabilityFailureThreshold
	}
	if t.Protocol == AvailabilityProbeUDP {
		t.UDPMode = normalizeAvailabilityUDPMode(t.UDPMode)
		if t.UDPMode == "" {
			t.UDPMode = AvailabilityUDPResponseRequired
		}
	}
	if t.HTTP != nil {
		if t.HTTP.Method == "" {
			t.HTTP.Method = AvailabilityHTTPMethodGET
		}
		if t.HTTP.Authentication.Type == "" {
			t.HTTP.Authentication.Type = AvailabilityHTTPAuthNone
		}
		if t.HTTP.ExpectedStatusMin == 0 {
			t.HTTP.ExpectedStatusMin = 200
		}
		if t.HTTP.ExpectedStatusMax == 0 {
			t.HTTP.ExpectedStatusMax = 399
		}
		for i := range t.HTTP.Headers {
			if strings.TrimSpace(t.HTTP.Headers[i].ID) == "" {
				t.HTTP.Headers[i].ID = uuid.NewString()
			}
		}
	}
}

func (t AvailabilityTarget) EffectivePollIntervalSecs() int {
	if t.PollIntervalSecs > 0 {
		return t.PollIntervalSecs
	}
	return DefaultAvailabilityPollIntervalSecs
}

func (t AvailabilityTarget) EffectiveTimeoutMillis() int {
	if t.TimeoutMillis > 0 {
		return t.TimeoutMillis
	}
	return DefaultAvailabilityTimeoutMillis
}

func (t AvailabilityTarget) EffectiveFailureThreshold() int {
	if t.FailureThreshold > 0 {
		return t.FailureThreshold
	}
	return DefaultAvailabilityFailureThreshold
}

// AvailabilityExecutionConfigChanged reports whether an edit changes what is
// executed or where it executes. Display, correlation, alert-threshold, and
// certificate-presentation edits intentionally stay within the same revision.
func AvailabilityExecutionConfigChanged(previous, next AvailabilityTarget) bool {
	legacyProbeAssignmentChanged := strings.TrimSpace(previous.ProbeAgentID) != strings.TrimSpace(next.ProbeAgentID)
	previous = NormalizeAvailabilityTarget(previous)
	next = NormalizeAvailabilityTarget(next)
	return previous.Address != next.Address ||
		previous.Protocol != next.Protocol ||
		previous.Port != next.Port ||
		previous.Path != next.Path ||
		previous.UDPMode != next.UDPMode ||
		previous.UDPRequest != next.UDPRequest ||
		previous.UDPExpected != next.UDPExpected ||
		!reflect.DeepEqual(previous.HTTP, next.HTTP) ||
		previous.EffectiveTimeoutMillis() != next.EffectiveTimeoutMillis() ||
		previous.EffectivePollIntervalSecs() != next.EffectivePollIntervalSecs() ||
		legacyProbeAssignmentChanged ||
		!reflect.DeepEqual(previous.EffectiveObservationLocationIDs(), next.EffectiveObservationLocationIDs())
}

func AvailabilityAgentObservationLocationID(agentID string) string {
	agentID = strings.TrimSpace(agentID)
	if agentID == "" {
		return ""
	}
	return AvailabilityObservationLocationAgentPrefix + agentID
}

func AvailabilityObservationLocationAgentID(locationID string) string {
	locationID = strings.TrimSpace(locationID)
	if !strings.HasPrefix(locationID, AvailabilityObservationLocationAgentPrefix) {
		return ""
	}
	return strings.TrimSpace(strings.TrimPrefix(locationID, AvailabilityObservationLocationAgentPrefix))
}

func (t AvailabilityTarget) EffectiveObservationLocationIDs() []string {
	locations := normalizeAvailabilityObservationLocationIDs(t.ObservationLocationIDs)
	if len(locations) > 0 {
		return locations
	}
	if agentID := strings.TrimSpace(t.ProbeAgentID); agentID != "" {
		return []string{AvailabilityAgentObservationLocationID(agentID)}
	}
	return []string{AvailabilityObservationLocationLocal}
}

func (t AvailabilityTarget) IncludesLocalObservation() bool {
	for _, locationID := range t.EffectiveObservationLocationIDs() {
		if locationID == AvailabilityObservationLocationLocal {
			return true
		}
	}
	return false
}

func (t AvailabilityTarget) AssignedProbeAgentIDs() []string {
	agents := make([]string, 0, len(t.ObservationLocationIDs))
	for _, locationID := range t.EffectiveObservationLocationIDs() {
		if agentID := AvailabilityObservationLocationAgentID(locationID); agentID != "" {
			agents = append(agents, agentID)
		}
	}
	return agents
}

func (t AvailabilityTarget) IsAssignedToProbeAgent(agentID string) bool {
	agentID = strings.TrimSpace(agentID)
	if agentID == "" {
		return false
	}
	for _, assignedID := range t.AssignedProbeAgentIDs() {
		if assignedID == agentID {
			return true
		}
	}
	return false
}

func (t AvailabilityTarget) CertificateMonitoringEnabled() bool {
	return normalizeAvailabilityProbeProtocol(t.Protocol) == AvailabilityProbeHTTPS && !t.CertificateMonitoringDisabled
}

func (t AvailabilityTarget) EffectiveCertificateExpiryWarningDays() int {
	if t.CertificateExpiryWarningDays > 0 {
		return t.CertificateExpiryWarningDays
	}
	return DefaultCertificateExpiryWarningDays
}

func (t AvailabilityTarget) DisplayName() string {
	if name := strings.TrimSpace(t.Name); name != "" {
		return name
	}
	return strings.TrimSpace(t.Address)
}

func (t AvailabilityTarget) ProbeAddress() string {
	return normalizeAvailabilityAddress(t.Address)
}

func (t AvailabilityTarget) Validate() error {
	if strings.TrimSpace(t.Address) == "" {
		return fmt.Errorf("availability target address is required")
	}
	switch t.TargetKind {
	case AvailabilityTargetMachine, AvailabilityTargetService, AvailabilityTargetDevice:
	default:
		return fmt.Errorf("unsupported availability target kind %q", t.TargetKind)
	}
	protocol := normalizeAvailabilityProbeProtocol(t.Protocol)
	switch protocol {
	case AvailabilityProbeICMP:
		if t.Port != 0 {
			return fmt.Errorf("icmp availability targets must not set a port")
		}
	case AvailabilityProbeTCP, AvailabilityProbeUDP:
		if t.Port <= 0 || t.Port > 65535 {
			return fmt.Errorf("%s availability targets require a valid port", protocol)
		}
	case AvailabilityProbeHTTP, AvailabilityProbeHTTPS:
		if t.Port < 0 || t.Port > 65535 {
			return fmt.Errorf("http availability target port must be valid")
		}
	default:
		return fmt.Errorf("unsupported availability protocol %q", t.Protocol)
	}
	if protocol == AvailabilityProbeUDP {
		mode := normalizeAvailabilityUDPMode(t.UDPMode)
		switch mode {
		case AvailabilityUDPResponseRequired, AvailabilityUDPOpenOrFiltered:
		default:
			return fmt.Errorf("unsupported UDP availability mode %q", t.UDPMode)
		}
		if mode == AvailabilityUDPResponseRequired && len(t.UDPRequest) == 0 {
			return fmt.Errorf("UDP response-required checks require a request payload")
		}
		if len(t.UDPRequest) > 512 {
			return fmt.Errorf("UDP request payload must be 512 bytes or less")
		}
		if len(t.UDPExpected) > 4096 {
			return fmt.Errorf("UDP expected response must be 4096 bytes or less")
		}
	} else if t.UDPMode != "" || t.UDPRequest != "" || t.UDPExpected != "" {
		return fmt.Errorf("UDP settings may only be used with UDP availability targets")
	}
	if protocol == AvailabilityProbeHTTP || protocol == AvailabilityProbeHTTPS {
		if err := validateAvailabilityHTTPConfig(t.HTTP); err != nil {
			return err
		}
	} else if t.HTTP != nil {
		return fmt.Errorf("HTTP response contracts may only be used with HTTP or HTTPS availability targets")
	}
	if protocol != AvailabilityProbeHTTPS && (t.CertificateMonitoringDisabled || t.CertificateExpiryWarningDays != 0) {
		return fmt.Errorf("certificate monitoring settings may only be used with HTTPS availability targets")
	}
	if t.CertificateExpiryWarningDays < 0 || t.CertificateExpiryWarningDays > 3650 {
		return fmt.Errorf("certificate expiry warning must be between 0 and 3650 days")
	}
	if t.PollIntervalSecs > 0 && t.PollIntervalSecs < 10 {
		return fmt.Errorf("availability poll interval must be at least 10 seconds")
	}
	if t.TimeoutMillis > 0 && t.TimeoutMillis < 250 {
		return fmt.Errorf("availability timeout must be at least 250 milliseconds")
	}
	if t.FailureThreshold > 0 && t.FailureThreshold > 10 {
		return fmt.Errorf("availability failure threshold must be 10 or less")
	}
	locations := t.EffectiveObservationLocationIDs()
	if len(locations) == 0 {
		return fmt.Errorf("availability target requires at least one observation location")
	}
	if len(locations) > 16 {
		return fmt.Errorf("availability target supports at most 16 observation locations")
	}
	for _, locationID := range locations {
		if locationID == AvailabilityObservationLocationLocal {
			continue
		}
		if AvailabilityObservationLocationAgentID(locationID) == "" {
			return fmt.Errorf("unsupported availability observation location %q", locationID)
		}
	}
	if protocol == AvailabilityProbeHTTP || protocol == AvailabilityProbeHTTPS {
		if _, err := t.HTTPURL(); err != nil {
			return err
		}
		return nil
	}
	host := t.ProbeAddress()
	if host == "" {
		return fmt.Errorf("availability target address is required")
	}
	if strings.ContainsAny(host, " \t\r\n") {
		return fmt.Errorf("availability target address must not contain whitespace")
	}
	return nil
}

func (t AvailabilityTarget) HTTPURL() (*url.URL, error) {
	raw := strings.TrimSpace(t.Address)
	if raw == "" {
		return nil, fmt.Errorf("availability target address is required")
	}
	if !strings.Contains(raw, "://") {
		if t.Protocol == AvailabilityProbeHTTPS {
			raw = "https://" + raw
		} else {
			raw = "http://" + raw
		}
	}
	u, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("invalid http availability address")
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, fmt.Errorf("http availability targets require http or https scheme")
	}
	if strings.TrimSpace(u.Hostname()) == "" {
		return nil, fmt.Errorf("http availability target host is required")
	}
	if u.User != nil {
		return nil, fmt.Errorf("http availability target credentials must use the authentication fields")
	}
	if t.Port > 0 {
		u.Host = net.JoinHostPort(u.Hostname(), fmt.Sprintf("%d", t.Port))
	}
	if path := strings.TrimSpace(t.Path); path != "" {
		if !strings.HasPrefix(path, "/") {
			path = "/" + path
		}
		u.Path = path
	}
	return u, nil
}

func NormalizeAvailabilityTarget(target AvailabilityTarget) AvailabilityTarget {
	target.ID = strings.TrimSpace(target.ID)
	target.Name = strings.TrimSpace(target.Name)
	target.TargetKind = AvailabilityTargetKind(strings.ToLower(strings.TrimSpace(string(target.TargetKind))))
	target.Protocol = normalizeAvailabilityProbeProtocol(target.Protocol)
	if target.Protocol == AvailabilityProbeHTTP || target.Protocol == AvailabilityProbeHTTPS {
		target.Address = strings.TrimSpace(target.Address)
	} else {
		target.Address = normalizeAvailabilityAddress(target.Address)
	}
	target.Path = strings.TrimSpace(target.Path)
	target.UDPMode = normalizeAvailabilityUDPMode(target.UDPMode)
	if target.Protocol != AvailabilityProbeUDP {
		target.UDPMode = ""
		target.UDPRequest = ""
		target.UDPExpected = ""
	}
	target.LinkedResourceID = strings.TrimSpace(target.LinkedResourceID)
	target.ProbeAgentID = strings.TrimSpace(target.ProbeAgentID)
	target.ObservationLocationIDs = normalizeAvailabilityObservationLocationIDs(target.ObservationLocationIDs)
	if target.ProbeAgentID != "" && len(target.ObservationLocationIDs) == 1 && target.ObservationLocationIDs[0] == AvailabilityObservationLocationLocal {
		// A pre-location client edits ProbeAgentID on a normalized local target.
		target.ObservationLocationIDs = []string{AvailabilityAgentObservationLocationID(target.ProbeAgentID)}
	}
	if len(target.ObservationLocationIDs) == 0 {
		if target.ProbeAgentID != "" {
			target.ObservationLocationIDs = []string{AvailabilityAgentObservationLocationID(target.ProbeAgentID)}
		} else {
			target.ObservationLocationIDs = []string{AvailabilityObservationLocationLocal}
		}
	}
	// Keep the legacy field truthful only for the shape it can represent.
	if len(target.ObservationLocationIDs) == 1 {
		target.ProbeAgentID = AvailabilityObservationLocationAgentID(target.ObservationLocationIDs[0])
	} else {
		target.ProbeAgentID = ""
	}
	if target.HTTP != nil {
		target.HTTP.Method = AvailabilityHTTPMethod(strings.ToUpper(strings.TrimSpace(string(target.HTTP.Method))))
		target.HTTP.Authentication.Type = AvailabilityHTTPAuthType(strings.ToLower(strings.TrimSpace(string(target.HTTP.Authentication.Type))))
		target.HTTP.Authentication.Username = strings.TrimSpace(target.HTTP.Authentication.Username)
		target.HTTP.TextContains = strings.TrimSpace(target.HTTP.TextContains)
		target.HTTP.JSONPath = strings.TrimSpace(target.HTTP.JSONPath)
		target.HTTP.JSONEquals = strings.TrimSpace(target.HTTP.JSONEquals)
		for i := range target.HTTP.Headers {
			target.HTTP.Headers[i].ID = strings.TrimSpace(target.HTTP.Headers[i].ID)
			target.HTTP.Headers[i].Name = strings.TrimSpace(target.HTTP.Headers[i].Name)
		}
		if target.HTTP.Authentication.Type != AvailabilityHTTPAuthBasic {
			target.HTTP.Authentication.Username = ""
			target.HTTP.Authentication.Password = nil
		}
		if target.HTTP.Authentication.Type != AvailabilityHTTPAuthBearer {
			target.HTTP.Authentication.BearerToken = nil
		}
		if target.HTTP.Method != AvailabilityHTTPMethodPOST {
			target.HTTP.Body = nil
		}
	}
	if target.Protocol != AvailabilityProbeHTTP && target.Protocol != AvailabilityProbeHTTPS {
		target.HTTP = nil
	}
	target.ApplyDefaults()
	return target
}

func normalizeAvailabilityObservationLocationIDs(locationIDs []string) []string {
	if len(locationIDs) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(locationIDs))
	locations := make([]string, 0, len(locationIDs))
	for _, raw := range locationIDs {
		locationID := strings.TrimSpace(raw)
		if locationID == "local" {
			locationID = AvailabilityObservationLocationLocal
		}
		if agentID := AvailabilityObservationLocationAgentID(locationID); agentID != "" {
			locationID = AvailabilityAgentObservationLocationID(agentID)
		}
		if locationID == "" {
			continue
		}
		if _, ok := seen[locationID]; ok {
			continue
		}
		seen[locationID] = struct{}{}
		locations = append(locations, locationID)
	}
	sort.SliceStable(locations, func(i, j int) bool {
		if locations[i] == AvailabilityObservationLocationLocal {
			return true
		}
		if locations[j] == AvailabilityObservationLocationLocal {
			return false
		}
		return locations[i] < locations[j]
	})
	return locations
}

func validateAvailabilityHTTPConfig(contract *AvailabilityHTTPConfig) error {
	if contract == nil {
		return nil
	}
	switch contract.Method {
	case AvailabilityHTTPMethodHEAD, AvailabilityHTTPMethodGET, AvailabilityHTTPMethodPOST:
	default:
		return fmt.Errorf("HTTP response contract method must be HEAD, GET, or POST")
	}
	if contract.ExpectedStatusMin < 100 || contract.ExpectedStatusMin > 599 ||
		contract.ExpectedStatusMax < 100 || contract.ExpectedStatusMax > 599 ||
		contract.ExpectedStatusMin > contract.ExpectedStatusMax {
		return fmt.Errorf("HTTP expected status range must be between 100 and 599")
	}
	if contract.Body != nil {
		if contract.Method != AvailabilityHTTPMethodPOST {
			return fmt.Errorf("HTTP request bodies may only be used with POST")
		}
		if len(*contract.Body) > MaxAvailabilityHTTPBodyBytes {
			return fmt.Errorf("HTTP request body must be %d bytes or less", MaxAvailabilityHTTPBodyBytes)
		}
	}
	if len(contract.TextContains) > 256 {
		return fmt.Errorf("HTTP text assertion must be 256 bytes or less")
	}
	if len(contract.JSONPath) > 256 || len(contract.JSONEquals) > 512 {
		return fmt.Errorf("HTTP JSON assertion is too long")
	}
	if contract.JSONPath != "" && !validAvailabilityJSONPath(contract.JSONPath) {
		return fmt.Errorf("HTTP JSON path must use dot fields and numeric array indexes")
	}
	if contract.Method == AvailabilityHTTPMethodHEAD && (contract.TextContains != "" || contract.JSONPath != "") {
		return fmt.Errorf("HTTP HEAD contracts cannot assert a response body")
	}
	if contract.JSONEquals != "" && contract.JSONPath == "" {
		return fmt.Errorf("HTTP JSON expected value requires a JSON path")
	}
	if len(contract.Headers) > 16 {
		return fmt.Errorf("HTTP response contracts support at most 16 request headers")
	}
	seenIDs := make(map[string]struct{}, len(contract.Headers))
	seenNames := make(map[string]struct{}, len(contract.Headers))
	totalHeaderValueBytes := 0
	for _, header := range contract.Headers {
		if header.ID == "" || header.Name == "" || !validAvailabilityHTTPHeaderName(header.Name) {
			return fmt.Errorf("HTTP request headers require a valid name and stable id")
		}
		name := strings.ToLower(header.Name)
		switch name {
		case "authorization", "host", "content-length", "connection", "transfer-encoding", "proxy-authorization":
			return fmt.Errorf("HTTP request header %q is reserved", header.Name)
		}
		if _, ok := seenIDs[header.ID]; ok {
			return fmt.Errorf("HTTP request header ids must be unique")
		}
		if _, ok := seenNames[name]; ok {
			return fmt.Errorf("HTTP request header names must be unique")
		}
		seenIDs[header.ID] = struct{}{}
		seenNames[name] = struct{}{}
		if header.Value != nil {
			if strings.ContainsAny(*header.Value, "\r\n") {
				return fmt.Errorf("HTTP request header values must not contain newlines")
			}
			totalHeaderValueBytes += len(*header.Value)
		}
	}
	if totalHeaderValueBytes > 8192 {
		return fmt.Errorf("HTTP request header values must total 8192 bytes or less")
	}
	switch contract.Authentication.Type {
	case AvailabilityHTTPAuthNone:
	case AvailabilityHTTPAuthBasic:
		if contract.Authentication.Username == "" || contract.Authentication.Password == nil || *contract.Authentication.Password == "" {
			return fmt.Errorf("HTTP basic authentication requires a username and password")
		}
	case AvailabilityHTTPAuthBearer:
		if contract.Authentication.BearerToken == nil || *contract.Authentication.BearerToken == "" {
			return fmt.Errorf("HTTP bearer authentication requires a token")
		}
	default:
		return fmt.Errorf("HTTP authentication type must be none, basic, or bearer")
	}
	return nil
}

func validAvailabilityHTTPHeaderName(name string) bool {
	if name == "" {
		return false
	}
	for _, char := range name {
		if !((char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') ||
			(char >= '0' && char <= '9') || strings.ContainsRune("!#$%&'*+-.^_`|~", char)) {
			return false
		}
	}
	return true
}

func validAvailabilityJSONPath(rawPath string) bool {
	path := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(rawPath), "$"))
	path = strings.TrimPrefix(path, ".")
	if path == "" {
		return true
	}
	for _, segment := range strings.Split(path, ".") {
		if segment == "" {
			return false
		}
		name := segment
		indexes := ""
		if bracket := strings.IndexByte(segment, '['); bracket >= 0 {
			name = segment[:bracket]
			indexes = segment[bracket:]
		}
		if strings.ContainsAny(name, "[]") || (name == "" && indexes == "") {
			return false
		}
		for indexes != "" {
			if !strings.HasPrefix(indexes, "[") {
				return false
			}
			end := strings.IndexByte(indexes, ']')
			if end <= 1 {
				return false
			}
			index, err := strconv.Atoi(indexes[1:end])
			if err != nil || index < 0 {
				return false
			}
			indexes = indexes[end+1:]
		}
	}
	return true
}

func normalizeAvailabilityUDPMode(mode AvailabilityUDPMode) AvailabilityUDPMode {
	return AvailabilityUDPMode(strings.ToLower(strings.TrimSpace(string(mode))))
}

func normalizeAvailabilityProbeProtocol(protocol AvailabilityProbeProtocol) AvailabilityProbeProtocol {
	normalized := AvailabilityProbeProtocol(strings.ToLower(strings.TrimSpace(string(protocol))))
	if normalized == availabilityProbePingAlias {
		return AvailabilityProbeICMP
	}
	return normalized
}

func normalizeAvailabilityAddress(raw string) string {
	value := strings.TrimSpace(raw)
	if value == "" {
		return ""
	}
	if strings.Contains(value, "://") {
		if u, err := url.Parse(value); err == nil && strings.TrimSpace(u.Hostname()) != "" {
			return strings.TrimSpace(u.Hostname())
		}
	}
	if host, _, err := net.SplitHostPort(value); err == nil && strings.TrimSpace(host) != "" {
		return strings.Trim(strings.TrimSpace(host), "[]")
	}
	return strings.Trim(value, "[]")
}
