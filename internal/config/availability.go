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
	AvailabilityProbeICMP AvailabilityProbeProtocol = "icmp"
	AvailabilityProbeTCP  AvailabilityProbeProtocol = "tcp"
	AvailabilityProbeHTTP AvailabilityProbeProtocol = "http"
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
	switch t.Protocol {
	case AvailabilityProbeICMP:
		if t.Port != 0 {
			return fmt.Errorf("icmp availability targets must not set a port")
		}
	case AvailabilityProbeTCP:
		if t.Port <= 0 || t.Port > 65535 {
			return fmt.Errorf("tcp availability targets require a valid port")
		}
	case AvailabilityProbeHTTP:
		if t.Port < 0 || t.Port > 65535 {
			return fmt.Errorf("http availability target port must be valid")
		}
	default:
		return fmt.Errorf("unsupported availability protocol %q", t.Protocol)
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
	if t.Protocol == AvailabilityProbeHTTP {
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
		raw = "http://" + raw
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
	target.Protocol = AvailabilityProbeProtocol(strings.ToLower(strings.TrimSpace(string(target.Protocol))))
	if target.Protocol == AvailabilityProbeHTTP {
		target.Address = strings.TrimSpace(target.Address)
	} else {
		target.Address = normalizeAvailabilityAddress(target.Address)
	}
	target.Path = strings.TrimSpace(target.Path)
	target.LinkedResourceID = strings.TrimSpace(target.LinkedResourceID)
	target.ApplyDefaults()
	return target
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
