package chat

import (
	"errors"
	"fmt"
	"testing"
)

// chatinvesterrorCustom is a concrete error type used to exercise
// errors.As across both failure channels of InvestigationRunError.
type chatinvesterrorCustom struct{ msg string }

func (e *chatinvesterrorCustom) Error() string { return e.msg }

// chatinvesterrorWrap is a second distinct concrete type so that an
// errors.As match on one channel does not accidentally match the other.
type chatinvesterrorWrap struct{ inner error }

func (e *chatinvesterrorWrap) Error() string { return "wrap:" + e.inner.Error() }

func (e *chatinvesterrorWrap) Unwrap() error { return e.inner }

func TestNewInvestigationRunError(t *testing.T) {
	runErr := errors.New("runtime exploded")
	proposalErr := errors.New("proposal invalid")

	cases := []struct {
		name           string
		runErr         error
		proposalErr    error
		wantNil        bool
		wantRunFailure error
		wantPropFail   error
	}{
		{
			name:           "both nil returns nil",
			runErr:         nil,
			proposalErr:    nil,
			wantNil:        true,
			wantRunFailure: nil,
			wantPropFail:   nil,
		},
		{
			name:           "run only populates run channel",
			runErr:         runErr,
			proposalErr:    nil,
			wantNil:        false,
			wantRunFailure: runErr,
			wantPropFail:   nil,
		},
		{
			name:           "proposal only populates proposal channel",
			runErr:         nil,
			proposalErr:    proposalErr,
			wantNil:        false,
			wantRunFailure: nil,
			wantPropFail:   proposalErr,
		},
		{
			name:           "both populated preserves both channels",
			runErr:         runErr,
			proposalErr:    proposalErr,
			wantNil:        false,
			wantRunFailure: runErr,
			wantPropFail:   proposalErr,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := NewInvestigationRunError(tc.runErr, tc.proposalErr)
			if tc.wantNil {
				if got != nil {
					t.Fatalf("expected nil error, got %+v", got)
				}
				return
			}
			if got == nil {
				t.Fatalf("expected non-nil error, got nil")
			}
			if run := got.RunFailure(); run != tc.wantRunFailure {
				t.Errorf("RunFailure() = %v, want %v", run, tc.wantRunFailure)
			}
			if prop := got.ProposalFailure(); prop != tc.wantPropFail {
				t.Errorf("ProposalFailure() = %v, want %v", prop, tc.wantPropFail)
			}
		})
	}
}

func TestInvestigationRunError_Error(t *testing.T) {
	runErr := errors.New("run failed")
	proposalErr := errors.New("proposal failed")

	cases := []struct {
		name    string
		err     *InvestigationRunError
		wantStr string
	}{
		{
			name:    "nil receiver returns empty string",
			err:     nil,
			wantStr: "",
		},
		{
			name:    "run channel only renders run message",
			err:     NewInvestigationRunError(runErr, nil),
			wantStr: "run failed",
		},
		{
			name:    "proposal channel only renders proposal message",
			err:     NewInvestigationRunError(nil, proposalErr),
			wantStr: "proposal failed",
		},
		{
			name:    "both channels join with newline",
			err:     NewInvestigationRunError(runErr, proposalErr),
			wantStr: "run failed\nproposal failed",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.err.Error()
			if got != tc.wantStr {
				t.Errorf("Error() = %q, want %q", got, tc.wantStr)
			}
		})
	}
}

func TestInvestigationRunError_Unwrap(t *testing.T) {
	runSentinel := errors.New("sentinel-run")
	proposalSentinel := errors.New("sentinel-proposal")
	runTyped := &chatinvesterrorCustom{msg: "typed-run"}
	proposalTyped := &chatinvesterrorWrap{inner: errors.New("inner-proposal")}

	t.Run("nil receiver returns nil slice", func(t *testing.T) {
		var err *InvestigationRunError
		if got := err.Unwrap(); got != nil {
			t.Fatalf("nil receiver Unwrap() = %v, want nil", got)
		}
	})

	t.Run("both channels surface in slice preserving order and nil entries", func(t *testing.T) {
		err := NewInvestigationRunError(runSentinel, proposalSentinel)
		got := err.Unwrap()
		if len(got) != 2 {
			t.Fatalf("Unwrap() length = %d, want 2", len(got))
		}
		if got[0] != runSentinel {
			t.Errorf("Unwrap()[0] = %v, want %v", got[0], runSentinel)
		}
		if got[1] != proposalSentinel {
			t.Errorf("Unwrap()[1] = %v, want %v", got[1], proposalSentinel)
		}
	})

	t.Run("run-only slice keeps proposal slot nil", func(t *testing.T) {
		err := NewInvestigationRunError(runSentinel, nil)
		got := err.Unwrap()
		if len(got) != 2 {
			t.Fatalf("Unwrap() length = %d, want 2", len(got))
		}
		if got[0] != runSentinel {
			t.Errorf("Unwrap()[0] = %v, want %v", got[0], runSentinel)
		}
		if got[1] != nil {
			t.Errorf("Unwrap()[1] = %v, want nil", got[1])
		}
	})

	t.Run("proposal-only slice keeps run slot nil", func(t *testing.T) {
		err := NewInvestigationRunError(nil, proposalSentinel)
		got := err.Unwrap()
		if len(got) != 2 {
			t.Fatalf("Unwrap() length = %d, want 2", len(got))
		}
		if got[0] != nil {
			t.Errorf("Unwrap()[0] = %v, want nil", got[0])
		}
		if got[1] != proposalSentinel {
			t.Errorf("Unwrap()[1] = %v, want %v", got[1], proposalSentinel)
		}
	})

	t.Run("errors.Is matches run sentinel", func(t *testing.T) {
		err := NewInvestigationRunError(runSentinel, proposalSentinel)
		if !errors.Is(err, runSentinel) {
			t.Errorf("errors.Is(err, runSentinel) = false, want true")
		}
	})

	t.Run("errors.Is matches proposal sentinel", func(t *testing.T) {
		err := NewInvestigationRunError(runSentinel, proposalSentinel)
		if !errors.Is(err, proposalSentinel) {
			t.Errorf("errors.Is(err, proposalSentinel) = false, want true")
		}
	})

	t.Run("errors.Is false for unrelated error", func(t *testing.T) {
		err := NewInvestigationRunError(runSentinel, proposalSentinel)
		other := errors.New("unrelated")
		if errors.Is(err, other) {
			t.Errorf("errors.Is(err, unrelated) = true, want false")
		}
	})

	t.Run("errors.Is walks wrapped inner chain via proposal channel", func(t *testing.T) {
		inner := errors.New("inner-proposal")
		err := NewInvestigationRunError(nil, &chatinvesterrorWrap{inner: inner})
		if !errors.Is(err, inner) {
			t.Errorf("errors.Is(err, inner) = false, want true")
		}
	})

	t.Run("errors.As extracts run channel typed error", func(t *testing.T) {
		err := NewInvestigationRunError(runTyped, nil)
		var target *chatinvesterrorCustom
		if !errors.As(err, &target) {
			t.Fatalf("errors.As for run typed error = false, want true")
		}
		if target != runTyped {
			t.Errorf("errors.As target = %p, want %p", target, runTyped)
		}
	})

	t.Run("errors.As extracts proposal channel typed error", func(t *testing.T) {
		err := NewInvestigationRunError(nil, proposalTyped)
		var target *chatinvesterrorWrap
		if !errors.As(err, &target) {
			t.Fatalf("errors.As for proposal typed error = false, want true")
		}
		if target != proposalTyped {
			t.Errorf("errors.As target = %p, want %p", target, proposalTyped)
		}
	})

	t.Run("errors.As does not match unrelated type", func(t *testing.T) {
		err := NewInvestigationRunError(runTyped, proposalTyped)
		var target *chatinvesterrorCustom
		_ = target
		var wrap *chatinvesterrorWrap
		if !errors.As(err, &wrap) {
			t.Errorf("errors.As for wrap type = false, want true")
		}
	})
}

func TestInvestigationRunError_RunFailure(t *testing.T) {
	runErr := errors.New("runtime failure")

	cases := []struct {
		name string
		err  *InvestigationRunError
		want error
	}{
		{
			name: "nil receiver returns nil",
			err:  nil,
			want: nil,
		},
		{
			name: "run set returns run error",
			err:  NewInvestigationRunError(runErr, nil),
			want: runErr,
		},
		{
			name: "only proposal set returns nil for run channel",
			err:  NewInvestigationRunError(nil, errors.New("proposal")),
			want: nil,
		},
		{
			name: "both set returns run error only",
			err:  NewInvestigationRunError(runErr, errors.New("proposal")),
			want: runErr,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.err.RunFailure()
			if got != tc.want {
				t.Errorf("RunFailure() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestInvestigationRunError_ProposalFailure(t *testing.T) {
	proposalErr := errors.New("proposal failure")

	cases := []struct {
		name string
		err  *InvestigationRunError
		want error
	}{
		{
			name: "nil receiver returns nil",
			err:  nil,
			want: nil,
		},
		{
			name: "proposal set returns proposal error",
			err:  NewInvestigationRunError(nil, proposalErr),
			want: proposalErr,
		},
		{
			name: "only run set returns nil for proposal channel",
			err:  NewInvestigationRunError(errors.New("run"), nil),
			want: nil,
		},
		{
			name: "both set returns proposal error only",
			err:  NewInvestigationRunError(errors.New("run"), proposalErr),
			want: proposalErr,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.err.ProposalFailure()
			if got != tc.want {
				t.Errorf("ProposalFailure() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestInvestigationRunError_NilReceiverChainedMethodSafety(t *testing.T) {
	// A typed-nil *InvestigationRunError must be safe to call every
	// method on without panicking; this guards the nil-receiver arms of
	// Error, Unwrap, RunFailure and ProposalFailure.
	var err *InvestigationRunError
	ensure := func(name string, fn func()) {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("%s panicked on nil receiver: %v", name, r)
			}
		}()
		fn()
	}
	ensure("Error", func() {
		if got := err.Error(); got != "" {
			t.Errorf("nil Error() = %q, want %q", got, "")
		}
	})
	ensure("Unwrap", func() {
		if got := err.Unwrap(); got != nil {
			t.Errorf("nil Unwrap() = %v, want nil", got)
		}
	})
	ensure("RunFailure", func() {
		if got := err.RunFailure(); got != nil {
			t.Errorf("nil RunFailure() = %v, want nil", got)
		}
	})
	ensure("ProposalFailure", func() {
		if got := err.ProposalFailure(); got != nil {
			t.Errorf("nil ProposalFailure() = %v, want nil", got)
		}
	})
}

func TestInvestigationRunError_FmtFormatting(t *testing.T) {
	// Guards the implicit fmt-handling: InvestigationRunError has an
	// Error() method, so %s/%v render its joined message and %v on the
	// value (not pointer) still resolves through Error().
	runErr := errors.New("fmt-run")
	proposalErr := fmt.Errorf("fmt-proposal-%d", 7)
	err := NewInvestigationRunError(runErr, proposalErr)
	wantStr := "fmt-run\nfmt-proposal-7"
	if got := fmt.Sprintf("%v", err); got != wantStr {
		t.Errorf("fmt %%v = %q, want %q", got, wantStr)
	}
	if got := fmt.Sprintf("%s", err); got != wantStr {
		t.Errorf("fmt %%s = %q, want %q", got, wantStr)
	}
}
