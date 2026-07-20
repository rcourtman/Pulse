package config

import (
	"fmt"
	"net"
	"net/url"
	"strings"

	"github.com/google/uuid"
)

const (
	DefaultAvailabilityPollIntervalSecs = 60
	DefaultAvailabilityTimeoutMillis    = 2000
	DefaultAvailabilityFailureThreshold = 2
)

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
}

// NewAvailabilityTarget returns a new target with generated ID and defaults.
func NewAvailabilityTarget() AvailabilityTarget {
	return AvailabilityTarget{
		ID:               uuid.NewString(),
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
	if t.PollIntervalSecs > 0 && t.PollIntervalSecs < 10 {
		return fmt.Errorf("availability poll interval must be at least 10 seconds")
	}
	if t.TimeoutMillis > 0 && t.TimeoutMillis < 250 {
		return fmt.Errorf("availability timeout must be at least 250 milliseconds")
	}
	if t.FailureThreshold > 0 && t.FailureThreshold > 10 {
		return fmt.Errorf("availability failure threshold must be 10 or less")
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
		return nil, fmt.Errorf("invalid http availability address: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, fmt.Errorf("http availability targets require http or https scheme")
	}
	if strings.TrimSpace(u.Hostname()) == "" {
		return nil, fmt.Errorf("http availability target host is required")
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
	target.ApplyDefaults()
	return target
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
