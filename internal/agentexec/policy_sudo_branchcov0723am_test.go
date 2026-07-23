package agentexec

import (
	"errors"
	"fmt"
	"testing"
)

// branchcovSentinelErr is a concrete sentinel error type used to verify that
// errors.Is/errors.As traverse the ApprovalGrantVerificationError wrapper.
type branchcovSentinelErr struct{ msg string }

func (e *branchcovSentinelErr) Error() string { return e.msg }

// TestBranchcov0723AmSudoLongOptionNeedsValue exhaustively covers every long
// option that sudoLongOptionNeedsValue identifies as value-requiring, plus the
// negative arms: value-less long option, inline =value form (which the source
// treats as an exact-match miss), unknown long option, short option, empty
// string and whitespace-only string.
func TestBranchcov0723AmSudoLongOptionNeedsValue(t *testing.T) {
	cases := []struct {
		name  string
		token string
		want  bool
	}{
		// Every option the source lists as value-requiring must return true.
		{"user needs value", "--user", true},
		{"group needs value", "--group", true},
		{"host needs value", "--host", true},
		{"prompt needs value", "--prompt", true},
		{"chdir needs value", "--chdir", true},
		{"close-from needs value", "--close-from", true},
		{"command-timeout needs value", "--command-timeout", true},
		{"role needs value", "--role", true},
		{"type needs value", "--type", true},

		// Long option that does NOT need a value: source falls into default arm.
		{"help does not need value", "--help", false},

		// Inline =value form: the source performs an exact string match against
		// the bare option name, so "--user=root" is not equal to "--user" and
		// the function returns false. (extractSudoCommand pre-filters the
		// "=value" form before calling this, but this function in isolation
		// does not special-case it.)
		{"inline equals form does not match", "--user=root", false},

		// Unknown long option: default arm.
		{"unknown long option", "--not-a-real-sudo-flag", false},

		// Short option (not a long option): default arm.
		{"short option", "-u", false},

		// Empty string: default arm.
		{"empty string", "", false},

		// Whitespace-only string: default arm.
		{"whitespace only", "   ", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := sudoLongOptionNeedsValue(tc.token)
			if got != tc.want {
				t.Fatalf("sudoLongOptionNeedsValue(%q) = %v, want %v", tc.token, got, tc.want)
			}
		})
	}
}

// TestBranchcov0723AmApprovalGrantVerificationErrorUnwrap covers both arms of
// Unwrap(): nil receiver and a receiver wrapping a cause. It asserts that the
// standard errors.Is/errors.As machinery reaches the wrapped cause THROUGH the
// wrapper rather than merely checking that Unwrap returns non-nil.
func TestBranchcov0723AmApprovalGrantVerificationErrorUnwrap(t *testing.T) {
	t.Run("wrapped cause is reachable via errors.Is and errors.As", func(t *testing.T) {
		cause := &branchcovSentinelErr{msg: "boom"}
		// Wrap once more with %w so we exercise two Unwrap hops before reaching
		// the sentinel: ApprovalGrantVerificationError -> fmt wrap -> sentinel.
		nested := fmt.Errorf("outer wrap: %w", cause)
		verr := &ApprovalGrantVerificationError{Reason: "test", Err: nested}

		if !errors.Is(verr, cause) {
			t.Fatalf("errors.Is must find sentinel cause through ApprovalGrantVerificationError wrapper")
		}

		var found *branchcovSentinelErr
		if !errors.As(verr, &found) {
			t.Fatalf("errors.As must extract sentinel cause through ApprovalGrantVerificationError wrapper")
		}
		if found != cause {
			t.Fatalf("errors.As resolved wrong instance: got %p, want %p", found, cause)
		}
	})

	t.Run("nil cause yields nil unwrap and no false matches", func(t *testing.T) {
		verr := &ApprovalGrantVerificationError{Reason: "test", Err: nil}

		if got := verr.Unwrap(); got != nil {
			t.Fatalf("Unwrap() = %v, want nil when cause is nil", got)
		}

		sentinel := errors.New("unrelated sentinel")
		if errors.Is(verr, sentinel) {
			t.Fatalf("errors.Is must be false when cause is nil")
		}
	})

	t.Run("nil receiver yields nil unwrap", func(t *testing.T) {
		var nilVerr *ApprovalGrantVerificationError
		if got := nilVerr.Unwrap(); got != nil {
			t.Fatalf("Unwrap() on nil receiver = %v, want nil", got)
		}
	})
}

// TestBranchcov0723AmApprovalGrantVerificationErrorMessage covers Error() for
// all three branches: wrapped cause (returns e.Err.Error()), nil cause, and
// nil receiver (both fall back to the default message).
func TestBranchcov0723AmApprovalGrantVerificationErrorMessage(t *testing.T) {
	t.Run("wrapped cause surfaces underlying message", func(t *testing.T) {
		cause := errors.New("the underlying message")
		verr := &ApprovalGrantVerificationError{Reason: "test", Err: cause}

		const want = "the underlying message"
		if got := verr.Error(); got != want {
			t.Fatalf("Error() = %q, want %q", got, want)
		}
	})

	t.Run("nil cause returns default message", func(t *testing.T) {
		verr := &ApprovalGrantVerificationError{Reason: "test", Err: nil}

		const want = "approval grant verification failed"
		if got := verr.Error(); got != want {
			t.Fatalf("Error() = %q, want %q", got, want)
		}
	})

	t.Run("nil receiver returns default message", func(t *testing.T) {
		var nilVerr *ApprovalGrantVerificationError

		const want = "approval grant verification failed"
		if got := nilVerr.Error(); got != want {
			t.Fatalf("Error() on nil receiver = %q, want %q", got, want)
		}
	})
}
