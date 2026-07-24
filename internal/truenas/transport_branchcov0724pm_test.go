package truenas

import (
	"errors"
	"fmt"
	"testing"
)

// sentinelHandshakeInner is a distinct base error used to verify
// errors.Is/errors.As walk the chain established by RPCHandshakeError.Unwrap.
var sentinelHandshakeInner = errors.New("dial tcp: connection refused")

// TestBranchcov0724pmRPCHandshakeErrorUnwrap covers the wrapped-inner and
// nil-inner arms of (*RPCHandshakeError).Unwrap and the resulting
// errors.Is / errors.As behaviour.
func TestBranchcov0724pmRPCHandshakeErrorUnwrap(t *testing.T) {
	t.Run("wrapped inner error is exposed by Unwrap", func(t *testing.T) {
		handshake := &RPCHandshakeError{StatusCode: 404, Err: sentinelHandshakeInner}

		if unwrapped := handshake.Unwrap(); unwrapped != sentinelHandshakeInner {
			t.Fatalf("Unwrap() = %v, want %v", unwrapped, sentinelHandshakeInner)
		}
		if !errors.Is(handshake, sentinelHandshakeInner) {
			t.Fatalf("errors.Is(handshake, sentinel) = false, want true (Unwrap must expose the inner error)")
		}

		var target *RPCHandshakeError
		if !errors.As(handshake, &target) {
			t.Fatalf("errors.As(handshake, *RPCHandshakeError) = false, want true")
		}
		if target == nil || target.StatusCode != 404 {
			t.Fatalf("errors.As target = %+v, want StatusCode 404", target)
		}
	})

	t.Run("Unwrap threads through a deeper error chain", func(t *testing.T) {
		inner := fmt.Errorf("tls handshake: %w", sentinelHandshakeInner)
		handshake := &RPCHandshakeError{StatusCode: 0, Err: inner}

		if !errors.Is(handshake, sentinelHandshakeInner) {
			t.Fatalf("errors.Is through nested wrap = false, want true (Unwrap must be recursive)")
		}
	})

	t.Run("nil inner error makes Unwrap return nil", func(t *testing.T) {
		handshake := &RPCHandshakeError{Err: nil}

		if unwrapped := handshake.Unwrap(); unwrapped != nil {
			t.Fatalf("Unwrap() = %v, want nil for nil inner error", unwrapped)
		}
		// With no wrapping, errors.Is must not match an unrelated sentinel.
		if errors.Is(handshake, sentinelHandshakeInner) {
			t.Fatalf("errors.Is(handshake, sentinel) = true, want false when inner error is nil")
		}
	})
}

// TestBranchcov0724pmRPCAuthErrorError covers every branch of
// (*RPCAuthError).Error: the nil-receiver guard, the empty/whitespace
// ResponseType arm, and the populated-ResponseType arm.
func TestBranchcov0724pmRPCAuthErrorError(t *testing.T) {
	t.Run("nil receiver returns generic message", func(t *testing.T) {
		var e *RPCAuthError
		got := e.Error()
		const want = "truenas rpc authentication failed"
		if got != want {
			t.Fatalf("nil (*RPCAuthError).Error() = %q, want %q", got, want)
		}
	})

	tests := []struct {
		name string
		err  *RPCAuthError
		want string
	}{
		{
			name: "empty response type omits response_type",
			err:  &RPCAuthError{Mechanism: "api-key-plain"},
			want: "truenas rpc api-key-plain authentication failed",
		},
		{
			name: "whitespace only response type treated as empty",
			err:  &RPCAuthError{Mechanism: "api-key-plain", ResponseType: "   "},
			want: "truenas rpc api-key-plain authentication failed",
		},
		{
			name: "populated response type included with %q quoting",
			err:  &RPCAuthError{Mechanism: "api-key-plain", ResponseType: "FAILURE"},
			want: `truenas rpc api-key-plain authentication failed: response_type="FAILURE"`,
		},
		{
			// A different mechanism proves the mechanism field is interpolated.
			name: "password mechanism is interpolated",
			err:  &RPCAuthError{Mechanism: "password", ResponseType: "OTP_REQUIRED"},
			want: `truenas rpc password authentication failed: response_type="OTP_REQUIRED"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.err.Error(); got != tt.want {
				t.Fatalf("(*RPCAuthError).Error() = %q, want %q", got, tt.want)
			}
		})
	}

	// The typed error must also participate in errors.As chains so callers can
	// distinguish an authoritative auth refusal from a retryable transport error.
	t.Run("participates in errors.As chains", func(t *testing.T) {
		wrapped := fmt.Errorf("call failed: %w", &RPCAuthError{Mechanism: "api-key-plain"})
		var target *RPCAuthError
		if !errors.As(wrapped, &target) {
			t.Fatal("errors.As(*RPCAuthError) = false, want true")
		}
		if target.Mechanism != "api-key-plain" {
			t.Fatalf("errors.As target Mechanism = %q, want api-key-plain", target.Mechanism)
		}
	})
}

// Compile-time guard: both target error types satisfy the error contract.
var (
	_ error = (*RPCHandshakeError)(nil)
	_ error = (*RPCAuthError)(nil)
)
