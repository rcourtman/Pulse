package config

import (
	"fmt"
	"strings"

	"github.com/google/uuid"
)

const vmwareSensitiveMask = "********"
const defaultVMwarePort = 443

// IsVMwareSensitiveMask reports whether the value is the redacted placeholder
// used by the VMware settings API.
func IsVMwareSensitiveMask(value string) bool {
	return strings.TrimSpace(value) == vmwareSensitiveMask
}

// VMwareVCenterInstance represents a configured VMware vCenter endpoint.
type VMwareVCenterInstance struct {
	ID                 string `json:"id"`
	Name               string `json:"name"`
	Host               string `json:"host"`
	Port               int    `json:"port,omitempty"`
	Username           string `json:"username,omitempty"`
	Password           string `json:"password,omitempty"`
	InsecureSkipVerify bool   `json:"insecureSkipVerify,omitempty"`
	Enabled            bool   `json:"enabled"`
}

// NewVMwareVCenterInstance returns a new instance with generated ID and sane
// defaults.
func NewVMwareVCenterInstance() VMwareVCenterInstance {
	return VMwareVCenterInstance{
		ID:      uuid.NewString(),
		Port:    defaultVMwarePort,
		Enabled: true,
	}
}

// ApplyDefaults normalizes legacy zero-value config onto the canonical stored
// defaults without changing explicitly configured values.
func (v *VMwareVCenterInstance) ApplyDefaults() {
	if v == nil {
		return
	}
	if v.Port <= 0 {
		v.Port = defaultVMwarePort
	}
}

// Validate performs required VMware configuration checks.
func (v *VMwareVCenterInstance) Validate() error {
	if v == nil {
		return fmt.Errorf("vmware vcenter instance is required")
	}
	if strings.TrimSpace(v.Host) == "" {
		return fmt.Errorf("vmware vcenter host is required")
	}
	if strings.TrimSpace(v.Username) == "" || strings.TrimSpace(v.Password) == "" {
		return fmt.Errorf("vmware credentials are required: provide username and password")
	}
	return nil
}

// Redacted returns a copy with sensitive credentials masked.
func (v *VMwareVCenterInstance) Redacted() VMwareVCenterInstance {
	if v == nil {
		return VMwareVCenterInstance{}
	}
	redacted := *v
	if strings.TrimSpace(redacted.Password) != "" {
		redacted.Password = vmwareSensitiveMask
	}
	return redacted
}

// PreserveMaskedSecrets restores stored credentials when an update payload uses
// the API redaction placeholder for unchanged secret fields.
func (v *VMwareVCenterInstance) PreserveMaskedSecrets(existing VMwareVCenterInstance) {
	if v == nil {
		return
	}
	if IsVMwareSensitiveMask(v.Password) {
		v.Password = existing.Password
	}
}
