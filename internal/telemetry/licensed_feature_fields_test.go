package telemetry

import (
	"reflect"
	"strings"
	"testing"
)

// Every registered name must be a real Ping field, so a rename or removal
// cannot leave the registry pointing at nothing while still looking enforced.
func TestLicensedFeatureAdoptionFieldsExistOnPing(t *testing.T) {
	pingType := reflect.TypeOf(Ping{})
	present := make(map[string]struct{}, pingType.NumField())
	for i := 0; i < pingType.NumField(); i++ {
		jsonName := strings.Split(pingType.Field(i).Tag.Get("json"), ",")[0]
		if jsonName == "" || jsonName == "-" {
			continue
		}
		present[jsonName] = struct{}{}
	}

	if len(LicensedFeatureAdoptionFields) == 0 {
		t.Fatal("licensed-feature adoption registry must not be empty")
	}
	for _, name := range LicensedFeatureAdoptionFields {
		if _, ok := present[name]; !ok {
			t.Errorf("registered licensed-feature field %q is not a Ping field", name)
		}
	}
}

func TestLicensedFeatureAdoptionFieldsHaveNoDuplicates(t *testing.T) {
	seen := make(map[string]struct{}, len(LicensedFeatureAdoptionFields))
	for _, name := range LicensedFeatureAdoptionFields {
		if _, ok := seen[name]; ok {
			t.Errorf("duplicate licensed-feature field %q", name)
		}
		seen[name] = struct{}{}
	}
}

// The removed v6 fields must not come back under their old names. Both were
// sourced from an unconditionally-installed audit store, so both read
// identically on every install regardless of entitlement.
func TestRetiredNonDiscriminatingFieldsStayRemoved(t *testing.T) {
	pingType := reflect.TypeOf(Ping{})
	retired := map[string]string{
		"audit_logging_persistent":                "the SQLite audit logger is installed on every install, so this was always true",
		"audit_events_30d":                        "counted defense-in-depth write volume, not audit-log use, and saturated the receiver clamp",
		"pulse_intelligence_patrol_autofixes_30d": "AutoFixCount was hardcoded to zero with no increment site",
	}
	for i := 0; i < pingType.NumField(); i++ {
		jsonName := strings.Split(pingType.Field(i).Tag.Get("json"), ",")[0]
		if reason, ok := retired[jsonName]; ok {
			t.Errorf("retired telemetry field %q reintroduced: %s", jsonName, reason)
		}
	}
}
