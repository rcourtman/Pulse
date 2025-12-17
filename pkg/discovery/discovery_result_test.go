package discovery

import (
	"testing"
)

func TestDiscoveryResultAddError_PopulatesStructuredAndLegacy(t *testing.T) {
	t.Parallel()

	var r DiscoveryResult
	r.AddError("docker_bridge_network", "timeout", "request timed out", "192.168.1.10", 8006)

	if len(r.StructuredErrors) != 1 {
		t.Fatalf("StructuredErrors len = %d, want %d", len(r.StructuredErrors), 1)
	}
	if len(r.Errors) != 1 {
		t.Fatalf("Errors len = %d, want %d", len(r.Errors), 1)
	}

	se := r.StructuredErrors[0]
	if se.Phase != "docker_bridge_network" {
		t.Fatalf("StructuredErrors[0].Phase = %q, want %q", se.Phase, "docker_bridge_network")
	}
	if se.ErrorType != "timeout" {
		t.Fatalf("StructuredErrors[0].ErrorType = %q, want %q", se.ErrorType, "timeout")
	}
	if se.Message != "request timed out" {
		t.Fatalf("StructuredErrors[0].Message = %q, want %q", se.Message, "request timed out")
	}
	if se.IP != "192.168.1.10" || se.Port != 8006 {
		t.Fatalf("StructuredErrors[0] address = %s:%d, want %s:%d", se.IP, se.Port, "192.168.1.10", 8006)
	}
	if se.Timestamp.IsZero() {
		t.Fatalf("StructuredErrors[0].Timestamp should be set")
	}

	if r.Errors[0] != "Docker bridge network [192.168.1.10:8006]: request timed out" {
		t.Fatalf("Errors[0] = %q", r.Errors[0])
	}
}

func TestDiscoveryResultAddError_LegacyFormattingVariants(t *testing.T) {
	t.Parallel()

	t.Run("ip-only", func(t *testing.T) {
		var r DiscoveryResult
		r.AddError("extra_targets", "phase_error", "failed", "10.0.0.1", 0)
		if len(r.Errors) != 1 {
			t.Fatalf("Errors len = %d, want %d", len(r.Errors), 1)
		}
		if r.Errors[0] != "Additional targets [10.0.0.1]: failed" {
			t.Fatalf("Errors[0] = %q", r.Errors[0])
		}
	})

	t.Run("no-address", func(t *testing.T) {
		var r DiscoveryResult
		r.AddError("unknown_phase", "phase_error", "failed", "", 0)
		if len(r.Errors) != 1 {
			t.Fatalf("Errors len = %d, want %d", len(r.Errors), 1)
		}
		if r.Errors[0] != "unknown_phase: failed" {
			t.Fatalf("Errors[0] = %q", r.Errors[0])
		}
	})
}
