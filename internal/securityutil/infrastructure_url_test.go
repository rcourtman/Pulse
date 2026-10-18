package securityutil

import (
	"reflect"
	"strings"
	"testing"
)

func TestInfrastructureURLDiagnosticDoesNotBorrowInput(t *testing.T) {
	for _, raw := range []string{
		"https://fixture:synthetic-secret@pve.invalid",
		"https://pve.invalid?token=synthetic-secret",
		"https://pve.invalid#synthetic-secret",
		"https://fixture:synthetic-secret@pve.invalid:bad-port",
		"https://pve.invalid/%invalid?token=synthetic-secret",
		"https://[synthetic-secret",
		"https://pve.invalid:\nsynthetic-secret",
	} {
		u, err := NormalizeInfrastructureBaseURL(raw, "https")
		if err == nil || u != nil {
			t.Fatal("invalid infrastructure URL admitted")
		}
		if strings.Contains(err.Error(), "synthetic-secret") {
			t.Error("syntax diagnostic copied rejected input")
		}
		if got := InfrastructureEndpointDiagnostic(raw); got != "[invalid endpoint]" {
			t.Error("invalid endpoint acquired a diagnostic label")
		}
	}

	raw := []string{"pve.invalid/tenant", "https://fixture:synthetic-secret@pve.invalid", "pve.invalid/tenant"}
	before := append([]string(nil), raw...)
	labels := InfrastructureEndpointsDiagnostic(raw)
	if !reflect.DeepEqual(labels, []string{"https://pve.invalid/tenant", "[invalid endpoint]", "https://pve.invalid/tenant"}) || !reflect.DeepEqual(raw, before) {
		t.Error("label generation changed membership, order or duplicates")
	}
	labels[0] = "changed-label"
	if !reflect.DeepEqual(raw, before) {
		t.Error("display labels retained mutable routing storage")
	}
}

func TestInfrastructureURLValidationPolicyUnchanged(t *testing.T) {
	for _, raw := range []string{
		"pve.invalid/tenant/../proxy/",
		"http://pve.invalid:8006/proxy",
		"https://[::1]:8006/proxy",
		"https://[fe80::1%25ZoneA]:8006/proxy",
		"https://pve.invalid//proxy",
		"ftp://pve.invalid",
		"",
	} {
		ordinary, ordinaryErr := NormalizeHTTPBaseURL(raw, "https")
		private, privateErr := NormalizeInfrastructureBaseURL(raw, "https")
		if (ordinaryErr == nil) != (privateErr == nil) || !reflect.DeepEqual(ordinary, private) {
			t.Errorf("diagnostic wrapper changed validation/routing for non-secret fixture: %q", raw)
		}
	}
}
