package config

import (
	"fmt"
	"strings"

	"github.com/google/uuid"
)

const trueNASSensitiveMask = "********"
const defaultTrueNASPollIntervalSecs = 60

// IsTrueNASSensitiveMask reports whether the value is the redacted placeholder
// used by the TrueNAS settings API.
func IsTrueNASSensitiveMask(value string) bool {
	return strings.TrimSpace(value) == trueNASSensitiveMask
}

// TrueNASInstance represents a configured TrueNAS endpoint.
type TrueNASInstance struct {
	ID                 string `json:"id"`
	Name               string `json:"name"`
	Host               string `json:"host"`
	Port               int    `json:"port,omitempty"`
	APIKey             string `json:"apiKey,omitempty"`
	Username           string `json:"username,omitempty"`
	Password           string `json:"password,omitempty"`
	UseHTTPS           bool   `json:"useHttps"`
	InsecureSkipVerify bool   `json:"insecureSkipVerify,omitempty"`
	Fingerprint        string `json:"fingerprint,omitempty"`
	Enabled            bool   `json:"enabled"`
	PollIntervalSecs   int    `json:"pollIntervalSeconds,omitempty"`

	// Per-surface collection scope. Positive booleans. Existing records that
	// predate these fields have all-zero Monitor* values — ApplyDefaults
	// treats that as "legacy" and enables every surface so behavior doesn't
	// silently change on upgrade.
	MonitorDatasets    bool `json:"monitorDatasets"`
	MonitorPools       bool `json:"monitorPools"`
	MonitorReplication bool `json:"monitorReplication"`
}

// NewTrueNASInstance returns a new instance with generated ID and sane defaults.
func NewTrueNASInstance() TrueNASInstance {
	return TrueNASInstance{
		ID:                 uuid.NewString(),
		UseHTTPS:           true,
		Enabled:            true,
		PollIntervalSecs:   defaultTrueNASPollIntervalSecs,
		MonitorDatasets:    true,
		MonitorPools:       true,
		MonitorReplication: true,
	}
}

// EffectivePollIntervalSecs returns the configured poll interval or the
// canonical default when the stored config still uses the zero-value legacy
// form.
func (t TrueNASInstance) EffectivePollIntervalSecs() int {
	if t.PollIntervalSecs > 0 {
		return t.PollIntervalSecs
	}
	return defaultTrueNASPollIntervalSecs
}

// ApplyDefaults normalizes legacy zero-value config onto the canonical stored
// defaults without changing explicitly configured values.
func (t *TrueNASInstance) ApplyDefaults() {
	if t == nil {
		return
	}
	if t.PollIntervalSecs <= 0 {
		t.PollIntervalSecs = defaultTrueNASPollIntervalSecs
	}
	// Legacy records predate per-surface scope booleans. Zero-value false
	// across all three means "never configured" — enable everything so
	// existing users keep full visibility after upgrade. Users who
	// deliberately disable a surface keep at least one other enabled, so
	// the all-false state will not recur.
	if !t.MonitorDatasets && !t.MonitorPools && !t.MonitorReplication {
		t.MonitorDatasets = true
		t.MonitorPools = true
		t.MonitorReplication = true
	}
}

// Validate performs required TrueNAS configuration checks.
func (t *TrueNASInstance) Validate() error {
	if t == nil {
		return fmt.Errorf("truenas instance is required")
	}

	if strings.TrimSpace(t.Host) == "" {
		return fmt.Errorf("truenas host is required")
	}

	if strings.TrimSpace(t.APIKey) != "" {
		return nil
	}

	if strings.TrimSpace(t.Username) == "" || strings.TrimSpace(t.Password) == "" {
		return fmt.Errorf("truenas credentials are required: provide api key or username/password")
	}

	return nil
}

// Redacted returns a copy with sensitive credentials masked.
func (t *TrueNASInstance) Redacted() TrueNASInstance {
	if t == nil {
		return TrueNASInstance{}
	}

	redacted := *t
	if strings.TrimSpace(redacted.APIKey) != "" {
		redacted.APIKey = trueNASSensitiveMask
	}
	if strings.TrimSpace(redacted.Password) != "" {
		redacted.Password = trueNASSensitiveMask
	}
	return redacted
}

// PreserveMaskedSecrets restores stored credentials when an update payload uses
// the API redaction placeholder for unchanged secret fields.
func (t *TrueNASInstance) PreserveMaskedSecrets(existing TrueNASInstance) {
	if t == nil {
		return
	}
	if IsTrueNASSensitiveMask(t.APIKey) {
		t.APIKey = existing.APIKey
	}
	if IsTrueNASSensitiveMask(t.Password) {
		t.Password = existing.Password
	}
}
