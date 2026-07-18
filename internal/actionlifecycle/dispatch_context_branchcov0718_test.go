package actionlifecycle

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/actionplanner"
	unified "github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

// TestDispatchContext_RoundTrip covers ContextWithCommittedDispatchAttempt and
// DispatchAttemptFromContext: the populated arm (stored value is returned with
// ok=true) and the empty-context arm (zero value, ok=false).
func TestDispatchContext_RoundTrip(t *testing.T) {
	attempt, err := unified.NewActionDispatchAttempt("act-branchcov-0718", time.Date(2026, 7, 18, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("NewActionDispatchAttempt: %v", err)
	}

	t.Run("populated_context_round_trips", func(t *testing.T) {
		ctx := ContextWithCommittedDispatchAttempt(context.Background(), attempt)
		got, ok := DispatchAttemptFromContext(ctx)
		if !ok {
			t.Fatalf("DispatchAttemptFromContext ok=false; want true")
		}
		if got.ID != attempt.ID || got.ActionID != attempt.ActionID || got.State != attempt.State || got.CreatedAt != attempt.CreatedAt {
			t.Fatalf("round-trip mismatch:\n got =%#v\n want=%#v", got, attempt)
		}
	})

	// Empty (but non-nil) context: no key set, so the type assertion fails and
	// the function reports ok=false with the zero value. (A nil context.Context
	// panics inside ctx.Value — that violates Go's context contract, so it is
	// deliberately not exercised here.)
	t.Run("empty_context_returns_zero_value_and_false", func(t *testing.T) {
		got, ok := DispatchAttemptFromContext(context.Background())
		if ok {
			t.Fatalf("ok=true on empty context; want false")
		}
		if (got != unified.ActionDispatchAttempt{}) {
			t.Fatalf("zero-value mismatch: got=%#v", got)
		}
	})

	// A context that carries an unrelated key type must also report ok=false.
	t.Run("unrelated_value_returns_zero_value_and_false", func(t *testing.T) {
		ctx := context.WithValue(context.Background(), unrelatedContextKey{}, "not-an-attempt")
		got, ok := DispatchAttemptFromContext(ctx)
		if ok {
			t.Fatalf("ok=true on unrelated value; want false")
		}
		if (got != unified.ActionDispatchAttempt{}) {
			t.Fatalf("zero-value mismatch: got=%#v", got)
		}
	})
}

type unrelatedContextKey struct{}

// TestTypedError_Error_FormatsFields exercises every typed-error Error() in
// service.go, asserting the identifying fields appear in the formatted message
// and exercising both arms of AvailabilityRefusedError.Error's reason branch.
func TestTypedError_Error_FormatsFields(t *testing.T) {
	wrapped := errors.New("disk unavailable")

	cases := []struct {
		name   string
		err    error
		needle string
	}{
		{"ResourceNotFoundError_id", &ResourceNotFoundError{ResourceID: "vm:42"}, `"vm:42"`},
		{"ResourceNotFoundError_prefix", &ResourceNotFoundError{ResourceID: "vm:42"}, `resource`},

		{"ActionNotFoundError_id", &ActionNotFoundError{ActionID: "act-9"}, `"act-9"`},
		{"ActionNotFoundError_prefix", &ActionNotFoundError{ActionID: "act-9"}, `action`},

		{"CapabilityNotFoundError_capability", &CapabilityNotFoundError{ResourceID: "vm:42", CapabilityName: "restart"}, `restart`},
		{"CapabilityNotFoundError_resource", &CapabilityNotFoundError{ResourceID: "vm:42", CapabilityName: "restart"}, `vm:42`},

		{"AvailabilityRefusedError_explicit_reason", &AvailabilityRefusedError{ResourceID: "vm:42", CapabilityName: "restart", Readiness: unified.ResourceActionReadiness{Reason: "agent disconnected"}}, `agent disconnected`},
		{"AvailabilityRefusedError_blank_reason_falls_back", &AvailabilityRefusedError{ResourceID: "vm:42", CapabilityName: "restart", Readiness: unified.ResourceActionReadiness{Reason: "   "}}, `action execution is unavailable`},
		{"AvailabilityRefusedError_default_reason_omits_raw", &AvailabilityRefusedError{ResourceID: "vm:42", CapabilityName: "restart", Readiness: unified.ResourceActionReadiness{}}, `unavailable`},

		{"PersistError_op", &PersistError{Op: "create", Err: wrapped}, `persist create`},
		{"PersistError_wrapped", &PersistError{Op: "create", Err: wrapped}, `disk unavailable`},

		{"QueryError_op", &QueryError{Op: "list_pending", Err: wrapped}, `query list_pending`},
		{"QueryError_wrapped", &QueryError{Op: "list_pending", Err: wrapped}, `disk unavailable`},

		{"FreshnessCheckError_prefix", &FreshnessCheckError{Err: wrapped}, `plan freshness check`},
		{"FreshnessCheckError_wrapped", &FreshnessCheckError{Err: wrapped}, `disk unavailable`},

		{"PolicyCheckError_prefix", &PolicyCheckError{Err: wrapped}, `execution policy check`},
		{"PolicyCheckError_wrapped", &PolicyCheckError{Err: wrapped}, `disk unavailable`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			msg := tc.err.Error()
			if !strings.Contains(msg, tc.needle) {
				t.Fatalf("Error()=%q; want substring %q", msg, tc.needle)
			}
		})
	}
}

// TestTypedError_Unwrap exercises the Unwrap() methods: the wrapped sentinel
// or inner error must be reachable through both errors.Is and errors.Unwrap.
func TestTypedError_Unwrap(t *testing.T) {
	t.Run("CapabilityNotFoundError_unwraps_to_ErrCapabilityNotFound", func(t *testing.T) {
		err := &CapabilityNotFoundError{ResourceID: "vm:42", CapabilityName: "restart"}
		if !errors.Is(err, actionplanner.ErrCapabilityNotFound) {
			t.Fatalf("errors.Is(err, ErrCapabilityNotFound)=false; want true (err=%v)", err)
		}
		if got := errors.Unwrap(err); got != actionplanner.ErrCapabilityNotFound {
			t.Fatalf("errors.Unwrap=%v; want ErrCapabilityNotFound", got)
		}
	})

	t.Run("PersistError_unwraps_to_inner", func(t *testing.T) {
		inner := errors.New("persist io error")
		err := &PersistError{Op: "create", Err: inner}
		if !errors.Is(err, inner) {
			t.Fatalf("errors.Is(err, inner)=false; want true")
		}
		if got := errors.Unwrap(err); got != inner {
			t.Fatalf("errors.Unwrap=%v; want %v", got, inner)
		}
	})

	t.Run("QueryError_unwraps_to_inner", func(t *testing.T) {
		inner := errors.New("scan error")
		err := &QueryError{Op: "list", Err: inner}
		if !errors.Is(err, inner) {
			t.Fatalf("errors.Is(err, inner)=false; want true")
		}
		if got := errors.Unwrap(err); got != inner {
			t.Fatalf("errors.Unwrap=%v; want %v", got, inner)
		}
	})

	t.Run("FreshnessCheckError_unwraps_to_inner", func(t *testing.T) {
		inner := errors.New("registry offline")
		err := &FreshnessCheckError{Err: inner}
		if !errors.Is(err, inner) {
			t.Fatalf("errors.Is(err, inner)=false; want true")
		}
		if got := errors.Unwrap(err); got != inner {
			t.Fatalf("errors.Unwrap=%v; want %v", got, inner)
		}
	})

	t.Run("PolicyCheckError_unwraps_to_inner", func(t *testing.T) {
		inner := errors.New("policy store unreachable")
		err := &PolicyCheckError{Err: inner}
		if !errors.Is(err, inner) {
			t.Fatalf("errors.Is(err, inner)=false; want true")
		}
		if got := errors.Unwrap(err); got != inner {
			t.Fatalf("errors.Unwrap=%v; want %v", got, inner)
		}
	})
}
