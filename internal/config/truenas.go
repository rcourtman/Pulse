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
}

// NewTrueNASInstance returns a new instance with generated ID and sane defaults.
func NewTrueNASInstance() TrueNASInstance {
	return TrueNASInstance{
		ID:               uuid.NewString(),
		UseHTTPS:         true,
		Enabled:          true,
		PollIntervalSecs: defaultTrueNASPollIntervalSecs,
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
