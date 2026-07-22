package hostagent

import (
	"errors"
	"fmt"
	"testing"

	agentshost "github.com/rcourtman/pulse-go-rewrite/pkg/agents/host"
)

// These tests target four previously-uncovered functions in the hostagent
// package, asserting each function's real branch behaviour (every matched
// substring with case/whitespace variations, nil/empty inputs, the mutation
// contract for the SMART annotator, and errors.Is/errors.As traversal through
// the error wrappers) without modifying any source file.
//
// NOTE: the task brief lists "(*clientError).Unwrap()" at proxmox_setup.go:1208,
// but the type at that line is permanentError, not clientError. clientError
// (proxmox_setup.go:1192) only defines Error(); it has no Unwrap() method, so
// it cannot be unwrapped. The Unwrap() arms are therefore exercised against
// permanentError, which is the type that actually owns them. See the report for
// details.

// branchcovTestErr is a concrete error type used to verify errors.As traversal
// through a *permanentError wrapper.
type branchcovTestErr struct{ msg string }

func (e *branchcovTestErr) Error() string { return e.msg }

// TestBranchcov0722R2IsAlreadyExistsOutput covers isAlreadyExistsOutput
// (proxmox_setup.go:1487). The function lowercases + trims its input and then
// tests for the single substring "already exists", so every arm below drives
// that one match against the normalisation rules plus the no-match path.
func TestBranchcov0722R2IsAlreadyExistsOutput(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want bool
	}{
		{"exact match", "already exists", true},
		{"uppercase normalised by ToLower", "ALREADY EXISTS", true},
		{"mixed case normalised by ToLower", "Already Exists", true},
		{"leading and trailing whitespace trimmed", "   already exists   ", true},
		{"tabs and newlines trimmed", "\talready exists\n", true},
		{"substring embedded in longer output", "create VM failed: vm 100 already exists on node pve1", true},
		{"only the substring", "already exists", true},
		{"empty string is no match", "", false},
		{"whitespace only trims to empty", "    ", false},
		{"unrelated output", "created successfully", false},
		{"near match missing trailing s", "already exist", false},
		{"near match misspelled", "alredy exists", false},
		{"contains words but not the phrase", "exists already without the phrase", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isAlreadyExistsOutput(tt.in); got != tt.want {
				t.Errorf("isAlreadyExistsOutput(%q) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

// TestBranchcov0722R2ClientErrorError covers clientError.Error()
// (proxmox_setup.go:1198), asserting the exact formatted message for a
// representative status code + body and for an empty body. It also verifies
// that *clientError is reachable via errors.As when wrapped, which is how the
// caller at proxmox_setup.go:1473 decides to skip retries.
func TestBranchcov0722R2ClientErrorError(t *testing.T) {
	t.Run("FormatsStatusCodeAndBody", func(t *testing.T) {
		ce := &clientError{statusCode: 404, body: `{"errors":["no such object"]}`}
		want := `auto-register returned HTTP 404: {"errors":["no such object"]}`
		if got := ce.Error(); got != want {
			t.Errorf("Error() = %q, want %q", got, want)
		}
	})

	t.Run("EmptyBody", func(t *testing.T) {
		ce := &clientError{statusCode: 400, body: ""}
		want := "auto-register returned HTTP 400: "
		if got := ce.Error(); got != want {
			t.Errorf("Error() = %q, want %q", got, want)
		}
	})

	t.Run("ErrorsAsExtractsWrappedClientError", func(t *testing.T) {
		ce := &clientError{statusCode: 401, body: "unauthorized"}
		wrapped := fmt.Errorf("registration failed: %w", ce)
		var got *clientError
		if !errors.As(wrapped, &got) {
			t.Fatal("errors.As failed to extract *clientError from wrapped error")
		}
		if got.statusCode != 401 || got.body != "unauthorized" {
			t.Errorf("extracted *clientError = {statusCode:%d, body:%q}, want {401, unauthorized}", got.statusCode, got.body)
		}
	})

	t.Run("ClientErrorHasNoUnwrapSoChainTerminates", func(t *testing.T) {
		// clientError defines no Unwrap() method; its error chain must end at
		// itself, so errors.Is against an unrelated sentinel is false.
		ce := &clientError{statusCode: 403}
		if errors.Is(ce, errors.New("unrelated")) {
			t.Errorf("errors.Is(ce, unrelated) = true; want false (no Unwrap, no match)")
		}
	})
}

// TestBranchcov0722R2PermanentErrorUnwrap covers permanentError.Error()
// (proxmox_setup.go:1207) and permanentError.Unwrap() (proxmox_setup.go:1208),
// which is the only Unwrap() defined in this file. Both arms are exercised:
// a present wrapped error and a nil wrapped error. errors.Is / errors.As are
// asserted through the wrapper rather than by pointer comparison alone.
func TestBranchcov0722R2PermanentErrorUnwrap(t *testing.T) {
	t.Run("WrappedErrorPresent", func(t *testing.T) {
		inner := &branchcovTestErr{msg: "malformed registration URL"}
		pe := &permanentError{err: inner}

		if got := pe.Unwrap(); got != inner {
			t.Errorf("Unwrap() = %v, want %v", got, inner)
		}
		if got := pe.Error(); got != inner.msg {
			t.Errorf("Error() = %q, want %q (delegates to wrapped error)", got, inner.msg)
		}

		// errors.Is walks Unwrap to locate the sentinel.
		if !errors.Is(pe, inner) {
			t.Errorf("errors.Is(pe, inner) = false; want true")
		}

		// errors.As extracts the concrete wrapped type through the chain.
		var target *branchcovTestErr
		if !errors.As(pe, &target) {
			t.Fatal("errors.As failed to extract wrapped *branchcovTestErr")
		}
		if target != inner {
			t.Errorf("errors.As extracted %p, want %p", target, inner)
		}
	})

	t.Run("NilWrappedError", func(t *testing.T) {
		pe := &permanentError{err: nil}

		if got := pe.Unwrap(); got != nil {
			t.Errorf("Unwrap() = %v, want nil", got)
		}

		// With a nil wrapped error the chain terminates, so errors.Is against
		// any non-nil target is false. (Error() is deliberately not called:
		// permanentError.Error delegates to err.Error(), which would panic on
		// a nil err.)
		if errors.Is(pe, errors.New("anything")) {
			t.Errorf("errors.Is(pe, anything) = true; want false (chain is nil)")
		}
	})
}

// TestBranchcov0722R2AnnotateSMARTWithZFSPools covers annotateSMARTWithZFSPools
// (zfs.go:145). Because the function mutates the caller's backing array in
// place, every assertion is made on the original slice. The early-return arms
// (nil/empty pools, nil/empty smartData), the pre-populated guard, the matching
// arm and the non-matching arm are all driven, and the device-name normalisation
// the source performs (lowercasing + /dev/ prefix stripping) is asserted
// explicitly.
func TestBranchcov0722R2AnnotateSMARTWithZFSPools(t *testing.T) {
	t.Run("NilPoolsLeavesSliceUntouched", func(t *testing.T) {
		data := []agentshost.DiskSMART{{Device: "sda"}, {Device: "sdb"}}
		annotateSMARTWithZFSPools(data, nil)
		if data[0].Pool != "" || data[1].Pool != "" {
			t.Fatalf("nil pools mutated slice: %+v", data)
		}
	})

	t.Run("EmptyPoolsLeavesSliceUntouched", func(t *testing.T) {
		data := []agentshost.DiskSMART{{Device: "sda", Pool: ""}}
		annotateSMARTWithZFSPools(data, map[string]string{})
		if data[0].Pool != "" {
			t.Fatalf("empty pools mutated slice: %+v", data)
		}
	})

	t.Run("NilSmartDataIsNoOp", func(t *testing.T) {
		annotateSMARTWithZFSPools(nil, map[string]string{"sda": "tank"})
		// Reaching here without panicking is the assertion.
	})

	t.Run("EmptySmartDataIsNoOp", func(t *testing.T) {
		annotateSMARTWithZFSPools([]agentshost.DiskSMART{}, map[string]string{"sda": "tank"})
		// Reaching here without panicking is the assertion.
	})

	t.Run("PrePopulatedPoolIsLeftUntouched", func(t *testing.T) {
		// The function skips entries whose Pool is already non-empty, so a
		// caller that pre-populated Pool from another source is not clobbered
		// even when the device matches a pool.
		data := []agentshost.DiskSMART{{Device: "sda", Pool: "unraid"}}
		annotateSMARTWithZFSPools(data, map[string]string{"sda": "tank"})
		if data[0].Pool != "unraid" {
			t.Errorf("pre-populated Pool was overwritten: got %q, want unraid", data[0].Pool)
		}
	})

	t.Run("MatchingDeviceStampsPoolField", func(t *testing.T) {
		data := []agentshost.DiskSMART{{Device: "sda"}}
		annotateSMARTWithZFSPools(data, map[string]string{"sda": "tank"})
		if data[0].Pool != "tank" {
			t.Errorf("matching device not annotated: Pool = %q, want tank", data[0].Pool)
		}
	})

	t.Run("DeviceNormalisationStripsDevPrefix", func(t *testing.T) {
		// normalizeZFSMemberKeys strips a leading "/dev/" (and other by-path
		// prefixes) before lookup, so /dev/sda must match a bare "sda" key.
		data := []agentshost.DiskSMART{{Device: "/dev/sda"}}
		annotateSMARTWithZFSPools(data, map[string]string{"sda": "tank"})
		if data[0].Pool != "tank" {
			t.Errorf("normalised /dev/sda did not match key %q: Pool = %q, want tank", "sda", data[0].Pool)
		}
	})

	t.Run("DeviceNormalisationIsCaseInsensitive", func(t *testing.T) {
		// The lookup lowercases the derived device name, so an upper-case
		// device matches a lower-case pool key. (Note: the pool map's own keys
		// are NOT normalised, so the key itself must already be lower case.)
		data := []agentshost.DiskSMART{{Device: "/dev/SDA"}}
		annotateSMARTWithZFSPools(data, map[string]string{"sda": "tank"})
		if data[0].Pool != "tank" {
			t.Errorf("upper-case device was not lower-cased for lookup: Pool = %q, want tank", data[0].Pool)
		}
	})

	t.Run("NonMatchingDeviceStaysEmpty", func(t *testing.T) {
		data := []agentshost.DiskSMART{{Device: "sdb"}}
		annotateSMARTWithZFSPools(data, map[string]string{"sda": "tank"})
		if data[0].Pool != "" {
			t.Errorf("non-matching device was annotated: Pool = %q, want empty", data[0].Pool)
		}
	})

	t.Run("OnlyMatchingEntriesAreMutatedMixedSlice", func(t *testing.T) {
		data := []agentshost.DiskSMART{
			{Device: "sda"},
			{Device: "sdb"},
			{Device: "sda", Pool: "preset"},
		}
		annotateSMARTWithZFSPools(data, map[string]string{"sda": "tank"})
		if data[0].Pool != "tank" {
			t.Errorf("data[0] (sda) not annotated: Pool = %q, want tank", data[0].Pool)
		}
		if data[1].Pool != "" {
			t.Errorf("data[1] (sdb) unexpectedly annotated: Pool = %q, want empty", data[1].Pool)
		}
		if data[2].Pool != "preset" {
			t.Errorf("data[2] (pre-populated) clobbered: Pool = %q, want preset", data[2].Pool)
		}
	})
}
