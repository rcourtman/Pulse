package api

import (
	"context"
	"errors"
	"testing"
)

// branchcov0723pmBoolPtr boxes a bool so NodeConfigRequest.VerifySSL can be
// driven through its *bool pointer in both the nil and the explicitly-set
// arms of testProxmoxPlatformConnection.
func branchcov0723pmBoolPtr(v bool) *bool {
	return &v
}

// branchcov0723pmSpyConnect returns an injectable connect func matching the
// signature testProxmoxPlatformConnection expects. It records the verifySSL
// value it was actually called with (so a test can assert the req.VerifySSL
// dereference happened) and then returns either connectErr (exercising the
// create_client error arm) or a probe whose own result is probeErr
// (exercising the connection error arm, or the success path when both nil).
func branchcov0723pmSpyConnect(captured *bool, probeErr, connectErr error) func(verifySSL bool) (func(context.Context) error, error) {
	return func(verifySSL bool) (func(context.Context) error, error) {
		*captured = verifySSL
		if connectErr != nil {
			return nil, connectErr
		}
		return func(context.Context) error {
			return probeErr
		}, nil
	}
}

// TestBranchcov0723pmPlatformConnection_VerifySSL covers both arms of the
// req.VerifySSL dereference: a nil pointer must default to false before it
// reaches connect, while an explicit true / false pointer must be passed
// through verbatim. The spy asserts the exact bool handed to connect.
func TestBranchcov0723pmPlatformConnection_VerifySSL(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		verifySSL *bool
		want      bool
	}{
		{"nil_defaults_to_false", nil, false},
		{"explicit_true_passed_through", branchcov0723pmBoolPtr(true), true},
		{"explicit_false_passed_through", branchcov0723pmBoolPtr(false), false},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			req := NodeConfigRequest{VerifySSL: tc.verifySSL}
			var captured bool
			got := testProxmoxPlatformConnection(req, "ok", branchcov0723pmSpyConnect(&captured, nil, nil))

			if captured != tc.want {
				t.Errorf("verifySSL handed to connect = %v, want %v", captured, tc.want)
			}
			if status, _ := got["status"].(string); status != "success" {
				t.Fatalf("status = %q, want success (probe should succeed); message=%v", status, got["message"])
			}
		})
	}
}

// TestBranchcov0723pmPlatformConnection_ConnectError covers the arm where
// connect itself fails: the result status must be "error" and the message
// must be the sanitized create_client string (not the raw error), proving
// the error flowed through sanitizeErrorMessage with that context.
func TestBranchcov0723pmPlatformConnection_ConnectError(t *testing.T) {
	t.Parallel()

	req := NodeConfigRequest{}
	var captured bool
	got := testProxmoxPlatformConnection(req, "ok", branchcov0723pmSpyConnect(&captured, nil, errors.New("boom: dial tcp 10.0.0.1:443")))

	if status, _ := got["status"].(string); status != "error" {
		t.Fatalf("status = %q, want error", status)
	}
	const wantMsg = "Failed to initialize connection"
	if msg, _ := got["message"].(string); msg != wantMsg {
		t.Errorf("message = %q, want sanitized %q (create_client context)", msg, wantMsg)
	}
	// connect was still invoked with the defaulted verifySSL (false) before failing.
	if captured != false {
		t.Errorf("verifySSL handed to connect = %v, want false (default)", captured)
	}
	// The error arm must not surface a latency reading.
	if _, ok := got["latency"]; ok {
		t.Errorf("latency key present on connect-error arm, want absent")
	}
}

// TestBranchcov0723pmPlatformConnection_ProbeError covers the arm where
// connect succeeds but the probe call itself returns an error: the result
// status must be "error" and the message must be the sanitized connection
// string, proving the error flowed through sanitizeErrorMessage with the
// "connection" context (distinct from the create_client context above).
func TestBranchcov0723pmPlatformConnection_ProbeError(t *testing.T) {
	t.Parallel()

	req := NodeConfigRequest{}
	var seenVerifySSL bool
	got := testProxmoxPlatformConnection(req, "ok", func(verifySSL bool) (func(context.Context) error, error) {
		seenVerifySSL = verifySSL
		return func(context.Context) error {
			return errors.New("boom: GetVersion rpc failed: 401 Unauthorized")
		}, nil
	})

	if seenVerifySSL != false {
		t.Errorf("verifySSL handed to connect = %v, want false (default)", seenVerifySSL)
	}
	if status, _ := got["status"].(string); status != "error" {
		t.Fatalf("status = %q, want error", status)
	}
	const wantMsg = "Connection failed. Please check your credentials and network settings"
	if msg, _ := got["message"].(string); msg != wantMsg {
		t.Errorf("message = %q, want sanitized %q (connection context)", msg, wantMsg)
	}
	if _, ok := got["latency"]; ok {
		t.Errorf("latency key present on probe-error arm, want absent")
	}
}

// TestBranchcov0723pmPlatformConnection_Success covers the success path:
// status "success", the caller-supplied successMsg passed through verbatim,
// a latency key present and non-negative, and confirmation that the probe
// was actually invoked with a context carrying the 10s deadline the
// function sets up.
func TestBranchcov0723pmPlatformConnection_Success(t *testing.T) {
	t.Parallel()

	const successMsg = "Connected to PBS instance"
	req := NodeConfigRequest{}

	var probeCalled bool
	got := testProxmoxPlatformConnection(req, successMsg, func(verifySSL bool) (func(context.Context) error, error) {
		if verifySSL != false {
			t.Errorf("verifySSL = %v, want false (default)", verifySSL)
		}
		return func(ctx context.Context) error {
			probeCalled = true
			if _, ok := ctx.Deadline(); !ok {
				t.Errorf("probe ctx has no deadline, want the 10s timeout set by the function")
			}
			return nil
		}, nil
	})

	if !probeCalled {
		t.Fatal("probe was not invoked on the success path")
	}
	if status, _ := got["status"].(string); status != "success" {
		t.Fatalf("status = %q, want success", status)
	}
	if msg, _ := got["message"].(string); msg != successMsg {
		t.Errorf("message = %q, want the caller-supplied successMsg %q", msg, successMsg)
	}
	latency, ok := got["latency"]
	if !ok {
		t.Fatal("latency key missing on success path")
	}
	ms, ok := latency.(int64)
	if !ok {
		t.Fatalf("latency value type = %T, want int64", latency)
	}
	if ms < 0 {
		t.Errorf("latency = %d, want >= 0", ms)
	}
}
