package config

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// This file adds branch-coverage tests for the TrueNAS/VMware credential
// masking helpers and the VMware vCenter instance lifecycle methods. It
// targets branches that the existing *_test.go files in this package do not
// exercise: nil receivers, whitespace-tolerant mask matching, both sides of
// every conditional in PreserveMaskedSecrets/Validate/ApplyDefaults/Redacted,
// and the <= 0 / empty / whitespace-only boundary inputs.

func TestBranchCovIsTrueNASSensitiveMask(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want bool
	}{
		{"exact mask", trueNASSensitiveMask, true},
		{"empty string", "", false},
		{"whitespace padded mask still matches", "  " + trueNASSensitiveMask + "\t", true},
		{"newline wrapped mask still matches", "\n" + trueNASSensitiveMask + "\n", true},
		{"whitespace only input", "   \t\n", false},
		{"unrelated secret value", "api-token-12345", false},
		{"shorter star run not equal", "****", false},
		{"longer star run not equal", "**********", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, IsTrueNASSensitiveMask(tt.in))
		})
	}
}

func TestBranchCovIsVMwareSensitiveMask(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want bool
	}{
		{"exact mask", vmwareSensitiveMask, true},
		{"empty string", "", false},
		{"whitespace padded mask still matches", " " + vmwareSensitiveMask + " ", true},
		{"whitespace only input", "\t\t", false},
		{"unrelated secret value", "vmware-password", false},
		{"substring of mask not equal", "****", false},
		{"longer star run not equal", "**********", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, IsVMwareSensitiveMask(tt.in))
		})
	}
}

func TestBranchCovTrueNASPreserveMaskedSecrets(t *testing.T) {
	t.Run("nil receiver is a no-op and does not panic", func(t *testing.T) {
		var inst *TrueNASInstance
		require.NotPanics(t, func() {
			inst.PreserveMaskedSecrets(TrueNASInstance{APIKey: "stored-key", Password: "stored-pass"})
		})
	})

	t.Run("both fields masked restores both from existing", func(t *testing.T) {
		inst := TrueNASInstance{APIKey: trueNASSensitiveMask, Password: trueNASSensitiveMask}
		inst.PreserveMaskedSecrets(TrueNASInstance{APIKey: "stored-key", Password: "stored-pass"})
		require.Equal(t, "stored-key", inst.APIKey)
		require.Equal(t, "stored-pass", inst.Password)
	})

	t.Run("only api key masked leaves password from payload intact", func(t *testing.T) {
		inst := TrueNASInstance{APIKey: trueNASSensitiveMask, Password: "new-pass"}
		inst.PreserveMaskedSecrets(TrueNASInstance{APIKey: "stored-key", Password: "stored-pass"})
		require.Equal(t, "stored-key", inst.APIKey)
		require.Equal(t, "new-pass", inst.Password)
	})

	t.Run("only password masked leaves api key from payload intact", func(t *testing.T) {
		inst := TrueNASInstance{APIKey: "new-key", Password: trueNASSensitiveMask}
		inst.PreserveMaskedSecrets(TrueNASInstance{APIKey: "stored-key", Password: "stored-pass"})
		require.Equal(t, "new-key", inst.APIKey)
		require.Equal(t, "stored-pass", inst.Password)
	})

	t.Run("neither field masked keeps payload values unchanged", func(t *testing.T) {
		inst := TrueNASInstance{APIKey: "fresh-key", Password: "fresh-pass"}
		inst.PreserveMaskedSecrets(TrueNASInstance{APIKey: "stored-key", Password: "stored-pass"})
		require.Equal(t, "fresh-key", inst.APIKey)
		require.Equal(t, "fresh-pass", inst.Password)
	})

	t.Run("masked fields with empty existing restore to empty", func(t *testing.T) {
		inst := TrueNASInstance{APIKey: trueNASSensitiveMask, Password: trueNASSensitiveMask}
		inst.PreserveMaskedSecrets(TrueNASInstance{})
		require.Equal(t, "", inst.APIKey)
		require.Equal(t, "", inst.Password)
	})

	t.Run("whitespace padded mask is treated as the placeholder", func(t *testing.T) {
		inst := TrueNASInstance{
			APIKey:   " " + trueNASSensitiveMask + " ",
			Password: "\t" + trueNASSensitiveMask + "\n",
		}
		inst.PreserveMaskedSecrets(TrueNASInstance{APIKey: "stored-key", Password: "stored-pass"})
		require.Equal(t, "stored-key", inst.APIKey)
		require.Equal(t, "stored-pass", inst.Password)
	})
}

func TestBranchCovVMwarePreserveMaskedSecrets(t *testing.T) {
	t.Run("nil receiver is a no-op and does not panic", func(t *testing.T) {
		var v *VMwareVCenterInstance
		require.NotPanics(t, func() {
			v.PreserveMaskedSecrets(VMwareVCenterInstance{Password: "stored-secret"})
		})
	})

	t.Run("masked password restored from existing", func(t *testing.T) {
		v := VMwareVCenterInstance{Password: vmwareSensitiveMask}
		v.PreserveMaskedSecrets(VMwareVCenterInstance{Password: "stored-secret"})
		require.Equal(t, "stored-secret", v.Password)
	})

	t.Run("unmasked password kept from payload", func(t *testing.T) {
		v := VMwareVCenterInstance{Password: "new-secret"}
		v.PreserveMaskedSecrets(VMwareVCenterInstance{Password: "stored-secret"})
		require.Equal(t, "new-secret", v.Password)
	})

	t.Run("masked password with empty existing restores to empty", func(t *testing.T) {
		v := VMwareVCenterInstance{Password: vmwareSensitiveMask}
		v.PreserveMaskedSecrets(VMwareVCenterInstance{})
		require.Equal(t, "", v.Password)
	})

	t.Run("whitespace padded mask is treated as the placeholder", func(t *testing.T) {
		v := VMwareVCenterInstance{Password: "  " + vmwareSensitiveMask + "  "}
		v.PreserveMaskedSecrets(VMwareVCenterInstance{Password: "stored-secret"})
		require.Equal(t, "stored-secret", v.Password)
	})
}

func TestBranchCovVMwareValidate(t *testing.T) {
	tests := []struct {
		name      string
		instance  *VMwareVCenterInstance
		wantError string
	}{
		{
			name:      "nil instance returns required error",
			instance:  nil,
			wantError: "vmware vcenter instance is required",
		},
		{
			name:      "missing host returns host error",
			instance:  &VMwareVCenterInstance{Username: "admin", Password: "pass"},
			wantError: "vmware vcenter host is required",
		},
		{
			name:      "whitespace only host returns host error",
			instance:  &VMwareVCenterInstance{Host: "   ", Username: "admin", Password: "pass"},
			wantError: "vmware vcenter host is required",
		},
		{
			name:      "missing username returns credentials error",
			instance:  &VMwareVCenterInstance{Host: "vc.local", Password: "pass"},
			wantError: "vmware credentials are required",
		},
		{
			name:      "missing password returns credentials error",
			instance:  &VMwareVCenterInstance{Host: "vc.local", Username: "admin"},
			wantError: "vmware credentials are required",
		},
		{
			name:      "whitespace only username returns credentials error",
			instance:  &VMwareVCenterInstance{Host: "vc.local", Username: "  ", Password: "pass"},
			wantError: "vmware credentials are required",
		},
		{
			name:      "whitespace only password returns credentials error",
			instance:  &VMwareVCenterInstance{Host: "vc.local", Username: "admin", Password: "\t"},
			wantError: "vmware credentials are required",
		},
		{
			name:      "valid host and credentials pass",
			instance:  &VMwareVCenterInstance{Host: "vc.local", Username: "admin", Password: "pass"},
			wantError: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.instance.Validate()
			if tt.wantError == "" {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			require.Contains(t, err.Error(), tt.wantError)
		})
	}
}

func TestBranchCovVMwareApplyDefaults(t *testing.T) {
	t.Run("nil receiver is a no-op and does not panic", func(t *testing.T) {
		var v *VMwareVCenterInstance
		require.NotPanics(t, func() { v.ApplyDefaults() })
	})

	t.Run("positive port preserved and explicit single surface kept", func(t *testing.T) {
		v := VMwareVCenterInstance{Port: 8443, MonitorHosts: true}
		v.ApplyDefaults()
		require.Equal(t, 8443, v.Port)
		require.False(t, v.MonitorVMs)
		require.True(t, v.MonitorHosts)
		require.False(t, v.MonitorDatastores)
	})

	t.Run("negative port reset to default and legacy scope migrated", func(t *testing.T) {
		v := VMwareVCenterInstance{Port: -1}
		v.ApplyDefaults()
		require.Equal(t, defaultVMwarePort, v.Port)
		require.True(t, v.MonitorVMs)
		require.True(t, v.MonitorHosts)
		require.True(t, v.MonitorDatastores)
	})

	t.Run("all surfaces explicitly enabled are left enabled", func(t *testing.T) {
		v := VMwareVCenterInstance{
			Port:              defaultVMwarePort,
			MonitorVMs:        true,
			MonitorHosts:      true,
			MonitorDatastores: true,
		}
		v.ApplyDefaults()
		require.True(t, v.MonitorVMs)
		require.True(t, v.MonitorHosts)
		require.True(t, v.MonitorDatastores)
		require.Equal(t, defaultVMwarePort, v.Port)
	})
}

func TestBranchCovVMwareRedacted(t *testing.T) {
	t.Run("nil receiver returns zero value and does not panic", func(t *testing.T) {
		var v *VMwareVCenterInstance
		require.NotPanics(t, func() {
			require.Equal(t, VMwareVCenterInstance{}, v.Redacted())
		})
	})

	t.Run("empty password is not masked", func(t *testing.T) {
		v := VMwareVCenterInstance{Host: "vc.local", Password: ""}
		got := v.Redacted()
		require.Equal(t, "", got.Password)
	})

	t.Run("whitespace only password is not masked", func(t *testing.T) {
		v := VMwareVCenterInstance{Host: "vc.local", Password: "   "}
		got := v.Redacted()
		require.Equal(t, "   ", got.Password)
	})

	t.Run("whitespace padded real password is masked", func(t *testing.T) {
		v := VMwareVCenterInstance{Host: "vc.local", Password: "  real-secret  "}
		got := v.Redacted()
		require.Equal(t, vmwareSensitiveMask, got.Password)
	})

	t.Run("receiver is not mutated by redaction", func(t *testing.T) {
		v := VMwareVCenterInstance{Host: "vc.local", Username: "admin", Password: "super-secret"}
		got := v.Redacted()
		require.Equal(t, "super-secret", v.Password)
		require.Equal(t, vmwareSensitiveMask, got.Password)
		require.Equal(t, "admin", got.Username)
		require.Equal(t, "vc.local", got.Host)
	})
}
