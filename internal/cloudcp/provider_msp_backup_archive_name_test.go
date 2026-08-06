package cloudcp

import "testing"

// cleanProviderMSPArchiveName is the first gate on attacker-influenced tar
// entry names during a provider MSP backup restore. pathIsInside catches an
// escape downstream, but this gate must reject traversal on its own so a
// future caller that skips the join check does not inherit a hole.
func TestCleanProviderMSPArchiveNameRejectsTraversal(t *testing.T) {
	unsafe := []string{
		"..",
		"../",
		"../etc/passwd",
		"./..",
		"a/../..",
		"control-plane/../../..",
		`..\..\windows`,
		"/etc/passwd",
		"/",
		"",
		"   ",
	}

	for _, raw := range unsafe {
		t.Run(raw, func(t *testing.T) {
			cleaned, err := cleanProviderMSPArchiveName(raw)
			if err == nil {
				t.Fatalf("cleanProviderMSPArchiveName(%q) = %q, want an error", raw, cleaned)
			}
		})
	}
}

func TestCleanProviderMSPArchiveNameAcceptsLegitimateEntries(t *testing.T) {
	cases := map[string]string{
		"control-plane/state.json":        "control-plane/state.json",
		"./control-plane/state.json":      "control-plane/state.json",
		"tenants/acme/pulse.db":           "tenants/acme/pulse.db",
		`tenants\acme\pulse.db`:           "tenants/acme/pulse.db",
		"tenants/acme/./pulse.db":         "tenants/acme/pulse.db",
		"tenants/acme/nested/../pulse.db": "tenants/acme/pulse.db",
		"manifest.json":                   "manifest.json",
	}

	for raw, want := range cases {
		t.Run(raw, func(t *testing.T) {
			got, err := cleanProviderMSPArchiveName(raw)
			if err != nil {
				t.Fatalf("cleanProviderMSPArchiveName(%q) error = %v", raw, err)
			}
			if got != want {
				t.Fatalf("cleanProviderMSPArchiveName(%q) = %q, want %q", raw, got, want)
			}
		})
	}
}
