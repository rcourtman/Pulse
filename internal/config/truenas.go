package config

import (
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/rcourtman/pulse-go-rewrite/internal/securityutil"
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
	// OutboundPolicy is server-owned save provenance, not an editable API
	// field. Empty preserves legacy/instance-managed connections. Tenant
	// handlers overwrite caller input and Redacted omits it from responses.
	OutboundPolicy securityutil.ConnectionOutboundPolicy `json:"outboundPolicy,omitempty"`

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

	// Per-surface collection scope. All combinations, including all off, are
	// explicit choices. Storage records whether the choice has been saved so
	// historical zero values retain their legacy all-on behavior on upgrade.
	MonitorDatasets    bool `json:"monitorDatasets"`
	MonitorPools       bool `json:"monitorPools"`
	MonitorReplication bool `json:"monitorReplication"`
}

// storedTrueNASInstance keeps migration provenance out of API payloads and the
// exported embedded response type. Historical saves can contain all three false
// fields even when no scope was chosen, so presence alone is not provenance.
// Every new SaveTrueNASConfig records the choice explicitly, including all off.
type storedTrueNASInstance struct {
	TrueNASInstance
	ScopeConfigured bool `json:"collectionScopeConfigured,omitempty"`
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

// ApplyDefaults normalizes the poll interval without changing collection scope.
// Legacy scope defaults are applied only by LoadTrueNASConfig, using save provenance.
func (t *TrueNASInstance) ApplyDefaults() {
	if t == nil {
		return
	}
	if t.PollIntervalSecs <= 0 {
		t.PollIntervalSecs = defaultTrueNASPollIntervalSecs
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
		if strings.TrimSpace(t.Username) == "" {
			return fmt.Errorf("truenas API key owner username is required for supported authentication")
		}
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
	redacted.OutboundPolicy = ""
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
