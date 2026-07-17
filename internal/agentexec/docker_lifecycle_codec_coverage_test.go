package agentexec

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// This file is a table-driven coverage test for the docker container
// lifecycle codec. It exercises every public codec function in
// docker_lifecycle_codec.go plus the unexported digest helper, with emphasis
// on the validation guards, the strict-JSON rejection shape (unknown fields /
// trailing data / malformed JSON / empty body), and the request/result
// cross-validation mismatch branches. It is the strict-codec sibling of
// docker_update_codec_coverage_test.go and asserts the same closed-contract
// rejection behaviour. (docker_lifecycle_contract_test.go covers the wire
// shape but does not exercise these codec functions at runtime.)

var (
	testValidLifecycleContainerID  = "abcdef123456"
	testValidLifecycleContainerIDB = "0123456789ab"
	testValidLifecycleDigest       = "sha256:" + strings.Repeat("a", 64)
	testValidLifecycleDigestAlt    = "sha256:" + strings.Repeat("b", 64)
)

// newBoundLifecyclePayload returns a fully bound, valid
// DockerContainerLifecyclePayload suitable for both validation and decode
// round-trips. Note that BindDockerContainerLifecyclePayload (unlike the
// update variant) does NOT stamp Operation, so it is set explicitly here.
func newBoundLifecyclePayload(t *testing.T) DockerContainerLifecyclePayload {
	t.Helper()
	payload := DockerContainerLifecyclePayload{
		RequestID:     "lc-request-1",
		ActionID:      "lc-action-1",
		Operation:     DockerContainerOperationStart,
		Runtime:       "docker",
		ContainerID:   testValidLifecycleContainerID,
		ExpectedState: "running",
		Timeout:       60,
	}
	if err := BindDockerContainerLifecyclePayload(&payload); err != nil {
		t.Fatalf("bind baseline lifecycle payload: %v", err)
	}
	return payload
}

// newValidLifecycleResult returns a minimal result that passes
// ValidateDockerContainerLifecycleResultPayload on its own (preflight phase,
// no completion requirements).
func newValidLifecycleResult() DockerContainerLifecycleResultPayload {
	return DockerContainerLifecycleResultPayload{
		RequestID:        "lc-request-1",
		ActionID:         "lc-action-1",
		Operation:        DockerContainerOperationStart,
		OperationVersion: DockerContainerLifecycleOperationVersion,
		RequestDigest:    testValidLifecycleDigest,
		ContainerID:      testValidLifecycleContainerID,
		ExecutionPhase:   DockerContainerPhasePreflight,
	}
}

// newValidCompleteLifecycleResult returns a result in the terminal phase that
// matches the bound request on every identity field and satisfies every
// completion guard (mutation done, readback observation present).
func newValidCompleteLifecycleResult(req DockerContainerLifecyclePayload) DockerContainerLifecycleResultPayload {
	observed := time.Date(2026, 7, 17, 12, 0, 0, 0, time.UTC)
	return DockerContainerLifecycleResultPayload{
		RequestID:         req.RequestID,
		ActionID:          req.ActionID,
		Operation:         req.Operation,
		OperationVersion:  req.OperationVersion,
		RequestDigest:     req.RequestDigest,
		ContainerID:       req.ContainerID,
		ExecutionPhase:    DockerContainerPhaseComplete,
		MutationStarted:   true,
		MutationCompleted: true,
		ReadbackRan:       true,
		After: DockerContainerLifecycleSnapshot{
			ContainerID: req.ContainerID,
			State:       "running",
			Running:     true,
			ObservedAt:  observed,
		},
	}
}

// mutateLifecyclePayload returns a copy of p with the mutator applied.
func mutateLifecyclePayload(p DockerContainerLifecyclePayload, fn func(*DockerContainerLifecyclePayload)) DockerContainerLifecyclePayload {
	cp := p
	fn(&cp)
	return cp
}

// mutateLifecycleResult returns a copy of r with the mutator applied.
func mutateLifecycleResult(r DockerContainerLifecycleResultPayload, fn func(*DockerContainerLifecycleResultPayload)) DockerContainerLifecycleResultPayload {
	cp := r
	fn(&cp)
	return cp
}

// --- DecodeDockerContainerLifecyclePayload: strict acceptance & rejection ---

func TestDecodeDockerContainerLifecyclePayloadStrictAcceptance(t *testing.T) {
	valid := newBoundLifecyclePayload(t)
	validJSON, err := json.Marshal(valid)
	if err != nil {
		t.Fatal(err)
	}
	base := string(validJSON)

	for _, tc := range []struct {
		name      string
		body      string
		wantError string
	}{
		{
			name: "valid happy path decodes",
			body: base,
		},
		{
			name:      "empty body rejected",
			body:      "",
			wantError: "empty",
		},
		{
			name:      "whitespace-only body rejected",
			body:      "   \n\t ",
			wantError: "empty",
		},
		{
			name:      "malformed json rejected",
			body:      `{"request_id":"r1","action_id":`,
			wantError: "EOF",
		},
		{
			name:      "unknown field rejected",
			body:      strings.TrimSuffix(base, "}") + `,"command":"docker ps"}`,
			wantError: "unknown field",
		},
		{
			name:      "trailing json rejected",
			body:      base + `{}`,
			wantError: "trailing",
		},
		{
			name:      "trailing malformed data rejected",
			body:      base + "garbage",
			wantError: "trailing",
		},
		{
			name:      "missing required request id rejected at validate step",
			body:      rejson(t, valid, "request_id", ""),
			wantError: "request or action id",
		},
		{
			name:      "unsupported operation rejected at validate step",
			body:      rejson(t, mutateLifecyclePayload(valid, func(p *DockerContainerLifecyclePayload) { p.Operation = "bogus_op" }), "operation", "bogus_op"),
			wantError: "unsupported docker container lifecycle operation",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			decoded, err := DecodeDockerContainerLifecyclePayload([]byte(tc.body))
			if tc.wantError == "" {
				if err != nil {
					t.Fatalf("expected decode success, got error: %v", err)
				}
				if decoded.RequestID != valid.RequestID || decoded.ActionID != valid.ActionID {
					t.Fatalf("decoded identity drift: %+v", decoded)
				}
				return
			}
			if err == nil {
				t.Fatalf("expected error containing %q, got success: %+v", tc.wantError, decoded)
			}
			if !strings.Contains(err.Error(), tc.wantError) {
				t.Fatalf("error %q does not contain %q", err.Error(), tc.wantError)
			}
		})
	}
}

// --- DecodeDockerContainerLifecycleResultPayload: strict acceptance & rejection ---

func TestDecodeDockerContainerLifecycleResultPayloadStrictAcceptance(t *testing.T) {
	validResult := newValidCompleteLifecycleResult(newBoundLifecyclePayload(t))
	validJSON, err := json.Marshal(validResult)
	if err != nil {
		t.Fatal(err)
	}
	base := string(validJSON)

	for _, tc := range []struct {
		name      string
		body      string
		wantError string
	}{
		{
			name: "valid complete result decodes",
			body: base,
		},
		{
			name:      "empty body rejected",
			body:      "",
			wantError: "empty",
		},
		{
			name:      "malformed json rejected",
			body:      `{"request_id":"r1", oops}`,
			wantError: "invalid character",
		},
		{
			name:      "unknown field rejected",
			body:      strings.TrimSuffix(base, "}") + `,"verification":"verified"}`,
			wantError: "unknown field",
		},
		{
			name:      "trailing json rejected",
			body:      base + `  {}`,
			wantError: "trailing",
		},
		{
			name:      "missing request id rejected at validate step",
			body:      rejson(t, validResult, "request_id", ""),
			wantError: "result identity",
		},
		{
			name:      "unsupported execution phase rejected at validate step",
			body:      rejson(t, mutateLifecycleResult(validResult, func(r *DockerContainerLifecycleResultPayload) { r.ExecutionPhase = "frobnicate" }), "execution_phase", "frobnicate"),
			wantError: "execution phase",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			decoded, err := DecodeDockerContainerLifecycleResultPayload([]byte(tc.body))
			if tc.wantError == "" {
				if err != nil {
					t.Fatalf("expected decode success, got error: %v", err)
				}
				if decoded.RequestID != validResult.RequestID {
					t.Fatalf("decoded identity drift: %+v", decoded)
				}
				return
			}
			if err == nil {
				t.Fatalf("expected error containing %q, got success: %+v", tc.wantError, decoded)
			}
			if !strings.Contains(err.Error(), tc.wantError) {
				t.Fatalf("error %q does not contain %q", err.Error(), tc.wantError)
			}
		})
	}
}

// --- BindDockerContainerLifecyclePayload ---

func TestBindDockerContainerLifecyclePayload(t *testing.T) {
	t.Run("nil payload rejected", func(t *testing.T) {
		if err := BindDockerContainerLifecyclePayload(nil); err == nil ||
			!strings.Contains(err.Error(), "docker container lifecycle payload is required") {
			t.Fatalf("nil payload was accepted by bind: %v", err)
		}
	})

	t.Run("bind stamps operation version and recomputes digest", func(t *testing.T) {
		payload := DockerContainerLifecyclePayload{
			RequestID:     "r1",
			ActionID:      "a1",
			Operation:     DockerContainerOperationRestart,
			Runtime:       "  Docker  ",
			ContainerID:   "  ABCDEF123456  ",
			ExpectedState: "  RUNNING  ",
		}
		if err := BindDockerContainerLifecyclePayload(&payload); err != nil {
			t.Fatalf("bind failed: %v", err)
		}
		// The lifecycle bind intentionally does not overwrite Operation; the
		// caller-selectable operation (start/stop/restart) must survive.
		if payload.Operation != DockerContainerOperationRestart {
			t.Fatalf("operation drifted during bind: %q", payload.Operation)
		}
		if payload.OperationVersion != DockerContainerLifecycleOperationVersion {
			t.Fatalf("operation version not stamped: %d", payload.OperationVersion)
		}
		if payload.RequestDigest == "" || !strings.HasPrefix(payload.RequestDigest, "sha256:") {
			t.Fatalf("request digest not stamped as sha256: %q", payload.RequestDigest)
		}
		// The bound payload must pass validation: Validate normalises the same
		// fields the digest helper does, so the stamped digest matches.
		if err := ValidateDockerContainerLifecyclePayload(&payload); err != nil {
			t.Fatalf("bound payload did not validate: %v", err)
		}
	})
}

// --- dockerContainerLifecycleRequestDigest (white-box) ---

func TestDockerContainerLifecycleRequestDigest(t *testing.T) {
	base := DockerContainerLifecyclePayload{
		ActionID:         "action-1",
		Operation:        DockerContainerOperationStart,
		OperationVersion: DockerContainerLifecycleOperationVersion,
		Runtime:          "docker",
		ContainerID:      testValidLifecycleContainerID,
		ExpectedState:    "running",
	}
	digestA, err := dockerContainerLifecycleRequestDigest(base)
	if err != nil {
		t.Fatalf("digest failed: %v", err)
	}
	if !strings.HasPrefix(digestA, "sha256:") {
		t.Fatalf("digest not sha256-prefixed: %q", digestA)
	}

	t.Run("deterministic for identical input", func(t *testing.T) {
		digestB, err := dockerContainerLifecycleRequestDigest(base)
		if err != nil {
			t.Fatal(err)
		}
		if digestA != digestB {
			t.Fatalf("digest not deterministic: %q vs %q", digestA, digestB)
		}
	})

	t.Run("case and whitespace normalised", func(t *testing.T) {
		upper := base
		upper.Runtime = "  DOCKER  "
		upper.ContainerID = "  ABCDEF123456  "
		upper.ExpectedState = "  RUNNING  "
		upper.ActionID = "  action-1  "
		digestUpper, err := dockerContainerLifecycleRequestDigest(upper)
		if err != nil {
			t.Fatal(err)
		}
		if digestUpper != digestA {
			t.Fatalf("digest not normalised for case/whitespace: %q vs %q", digestUpper, digestA)
		}
	})

	t.Run("expected state change alters digest", func(t *testing.T) {
		changed := base
		changed.ExpectedState = "exited"
		digestChanged, err := dockerContainerLifecycleRequestDigest(changed)
		if err != nil {
			t.Fatal(err)
		}
		if digestChanged == digestA {
			t.Fatal("expected state change did not alter digest")
		}
	})
}

// --- ValidateDockerContainerLifecyclePayload: every branch ---

func TestValidateDockerContainerLifecyclePayloadBranches(t *testing.T) {
	base := newBoundLifecyclePayload(t)

	for _, tc := range []struct {
		name      string
		mutate    func(*DockerContainerLifecyclePayload)
		wantError string
	}{
		{
			name:   "valid happy path",
			mutate: func(*DockerContainerLifecyclePayload) {},
		},
		{
			name:      "missing request id",
			mutate:    func(p *DockerContainerLifecyclePayload) { p.RequestID = "" },
			wantError: "request or action id",
		},
		{
			name:      "missing action id",
			mutate:    func(p *DockerContainerLifecyclePayload) { p.ActionID = "" },
			wantError: "request or action id",
		},
		{
			name:      "oversized request id",
			mutate:    func(p *DockerContainerLifecyclePayload) { p.RequestID = strings.Repeat("x", maxRequestIDLength+1) },
			wantError: "request or action id",
		},
		{
			name:      "oversized action id",
			mutate:    func(p *DockerContainerLifecyclePayload) { p.ActionID = strings.Repeat("y", maxRequestIDLength+1) },
			wantError: "request or action id",
		},
		{
			name:      "unsupported operation",
			mutate:    func(p *DockerContainerLifecyclePayload) { p.Operation = DockerContainerOperationUpdate },
			wantError: "unsupported docker container lifecycle operation",
		},
		{
			name:      "unsupported operation version",
			mutate:    func(p *DockerContainerLifecyclePayload) { p.OperationVersion = 99 },
			wantError: "unsupported docker container lifecycle operation version",
		},
		{
			name:      "unsupported runtime",
			mutate:    func(p *DockerContainerLifecyclePayload) { p.Runtime = "containerd" },
			wantError: "unsupported container runtime",
		},
		{
			name:      "podman runtime accepted",
			mutate:    func(p *DockerContainerLifecyclePayload) { p.Runtime = "podman" },
			wantError: "",
		},
		{
			name:      "container id wrong format",
			mutate:    func(p *DockerContainerLifecyclePayload) { p.ContainerID = "xyz123" },
			wantError: "immutable hexadecimal id",
		},
		{
			name:      "container id too short",
			mutate:    func(p *DockerContainerLifecyclePayload) { p.ContainerID = "abc" },
			wantError: "immutable hexadecimal id",
		},
		{
			name:      "expected state empty",
			mutate:    func(p *DockerContainerLifecyclePayload) { p.ExpectedState = "" },
			wantError: "expected container state is required",
		},
		{
			name:      "expected state too long",
			mutate:    func(p *DockerContainerLifecyclePayload) { p.ExpectedState = strings.Repeat("s", 33) },
			wantError: "expected container state is required",
		},
		{
			name:      "expected state change triggers digest mismatch",
			mutate:    func(p *DockerContainerLifecyclePayload) { p.ExpectedState = "stopped" },
			wantError: "request digest mismatch",
		},
		{
			name:      "request digest tampered directly",
			mutate:    func(p *DockerContainerLifecyclePayload) { p.RequestDigest = testValidLifecycleDigestAlt },
			wantError: "request digest mismatch",
		},
		{
			name:      "negative timeout rejected",
			mutate:    func(p *DockerContainerLifecyclePayload) { p.Timeout = -1 },
			wantError: "timeout must be between 0 and",
		},
		{
			name:      "timeout exceeding maximum rejected",
			mutate:    func(p *DockerContainerLifecyclePayload) { p.Timeout = 301 },
			wantError: "timeout must be between 0 and",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := base
			tc.mutate(&p)
			// podman runtime is accepted; its digest must be rebound so the
			// digest-mismatch check stays quiet and we observe the runtime
			// branch's accept path rather than a digest drift.
			if p.Runtime == "podman" {
				if err := BindDockerContainerLifecyclePayload(&p); err != nil {
					t.Fatalf("rebind for podman failed: %v", err)
				}
			}
			err := ValidateDockerContainerLifecyclePayload(&p)
			if tc.wantError == "" {
				if err != nil {
					t.Fatalf("expected validation success, got: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("expected error containing %q, got success", tc.wantError)
			}
			if !strings.Contains(err.Error(), tc.wantError) {
				t.Fatalf("error %q does not contain %q", err.Error(), tc.wantError)
			}
		})
	}

	t.Run("nil payload rejected", func(t *testing.T) {
		if err := ValidateDockerContainerLifecyclePayload(nil); err == nil ||
			!strings.Contains(err.Error(), "docker container lifecycle payload is required") {
			t.Fatalf("nil payload accepted: %v", err)
		}
	})

	t.Run("zero timeout is defaulted", func(t *testing.T) {
		p := base
		p.Timeout = 0
		if err := ValidateDockerContainerLifecyclePayload(&p); err != nil {
			t.Fatalf("zero timeout should be defaulted, got: %v", err)
		}
		if p.Timeout != 120 {
			t.Fatalf("timeout not defaulted: got %d want 120", p.Timeout)
		}
	})

	t.Run("exact maximum timeout accepted", func(t *testing.T) {
		p := base
		p.Timeout = 300
		if err := ValidateDockerContainerLifecyclePayload(&p); err != nil {
			t.Fatalf("maximum timeout should be accepted, got: %v", err)
		}
		if p.Timeout != 300 {
			t.Fatalf("timeout mutated unexpectedly: got %d want 300", p.Timeout)
		}
	})
}

// --- ValidateDockerContainerLifecycleResultPayload: every branch ---

func TestValidateDockerContainerLifecycleResultPayloadBranches(t *testing.T) {
	base := newValidCompleteLifecycleResult(newBoundLifecyclePayload(t))

	for _, tc := range []struct {
		name      string
		mutate    func(*DockerContainerLifecycleResultPayload)
		wantError string
	}{
		{
			name:      "valid complete happy path",
			mutate:    func(*DockerContainerLifecycleResultPayload) {},
			wantError: "",
		},
		{
			name:      "missing request id",
			mutate:    func(r *DockerContainerLifecycleResultPayload) { r.RequestID = "" },
			wantError: "result identity",
		},
		{
			name:      "missing action id",
			mutate:    func(r *DockerContainerLifecycleResultPayload) { r.ActionID = "" },
			wantError: "result identity",
		},
		{
			name: "oversized request id",
			mutate: func(r *DockerContainerLifecycleResultPayload) {
				r.RequestID = strings.Repeat("x", maxRequestIDLength+1)
			},
			wantError: "result identity",
		},
		{
			name:      "unsupported operation",
			mutate:    func(r *DockerContainerLifecycleResultPayload) { r.Operation = DockerContainerOperationUpdate },
			wantError: "unsupported docker lifecycle result operation",
		},
		{
			name:      "wrong operation version in binding",
			mutate:    func(r *DockerContainerLifecycleResultPayload) { r.OperationVersion = 99 },
			wantError: "invalid docker lifecycle result binding",
		},
		{
			name:      "bad container id in binding",
			mutate:    func(r *DockerContainerLifecycleResultPayload) { r.ContainerID = "xyz123" },
			wantError: "invalid docker lifecycle result binding",
		},
		{
			name:      "bad request digest format in binding",
			mutate:    func(r *DockerContainerLifecycleResultPayload) { r.RequestDigest = "not-a-digest" },
			wantError: "invalid docker lifecycle result binding",
		},
		{
			name:      "unsupported execution phase",
			mutate:    func(r *DockerContainerLifecycleResultPayload) { r.ExecutionPhase = "frobnicate" },
			wantError: "execution phase",
		},
		{
			name: "error string exceeds bound",
			mutate: func(r *DockerContainerLifecycleResultPayload) {
				r.Error = strings.Repeat("e", 1025)
			},
			wantError: "exceeds bounded contract",
		},
		{
			name: "before restart count negative",
			mutate: func(r *DockerContainerLifecycleResultPayload) {
				r.Before = DockerContainerLifecycleSnapshot{RestartCount: -1}
			},
			wantError: "exceeds bounded contract",
		},
		{
			name: "after restart count negative",
			mutate: func(r *DockerContainerLifecycleResultPayload) {
				r.After.RestartCount = -1
			},
			wantError: "exceeds bounded contract",
		},
		{
			name: "snapshot invalid container id",
			mutate: func(r *DockerContainerLifecycleResultPayload) {
				r.After = DockerContainerLifecycleSnapshot{
					ContainerID: "not-a-real-id",
					ObservedAt:  time.Date(2026, 7, 17, 12, 0, 0, 0, time.UTC),
				}
			},
			wantError: "invalid container id",
		},
		{
			name: "snapshot non-utc observation timestamp",
			mutate: func(r *DockerContainerLifecycleResultPayload) {
				r.After = DockerContainerLifecycleSnapshot{
					ContainerID: testValidLifecycleContainerID,
					ObservedAt:  time.Date(2026, 1, 1, 0, 0, 0, 0, time.FixedZone("PST", -28800)),
				}
			},
			wantError: "must be UTC",
		},
		{
			name: "mutation completed without mutation started",
			mutate: func(r *DockerContainerLifecycleResultPayload) {
				r.MutationStarted = false
				r.MutationCompleted = true
			},
			wantError: "requires mutation start",
		},
		{
			name: "readback ran without after observation",
			mutate: func(r *DockerContainerLifecycleResultPayload) {
				r.ReadbackRan = true
				r.After = DockerContainerLifecycleSnapshot{
					ContainerID: testValidLifecycleContainerID,
					ObservedAt:  time.Time{},
				}
			},
			wantError: "readback requires an observation",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := base
			tc.mutate(&r)
			err := ValidateDockerContainerLifecycleResultPayload(&r)
			if tc.wantError == "" {
				if err != nil {
					t.Fatalf("expected validation success, got: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("expected error containing %q, got success", tc.wantError)
			}
			if !strings.Contains(err.Error(), tc.wantError) {
				t.Fatalf("error %q does not contain %q", err.Error(), tc.wantError)
			}
		})
	}

	t.Run("nil result rejected", func(t *testing.T) {
		if err := ValidateDockerContainerLifecycleResultPayload(nil); err == nil ||
			!strings.Contains(err.Error(), "docker container lifecycle result is required") {
			t.Fatalf("nil result accepted: %v", err)
		}
	})

	t.Run("preflight result without completion requirements accepted", func(t *testing.T) {
		r := newValidLifecycleResult()
		if err := ValidateDockerContainerLifecycleResultPayload(&r); err != nil {
			t.Fatalf("preflight result should be accepted, got: %v", err)
		}
	})

	t.Run("verify phase accepted", func(t *testing.T) {
		r := newValidCompleteLifecycleResult(newBoundLifecyclePayload(t))
		r.ExecutionPhase = DockerContainerPhaseVerify
		if err := ValidateDockerContainerLifecycleResultPayload(&r); err != nil {
			t.Fatalf("verify phase should be accepted, got: %v", err)
		}
	})
}

// --- DockerContainerLifecycleOperationIdentity ---

func TestDockerContainerLifecycleOperationIdentity(t *testing.T) {
	req := newBoundLifecyclePayload(t)
	identity := DockerContainerLifecycleOperationIdentity("  agent-42  ", req)
	if identity.AttemptID != req.RequestID {
		t.Fatalf("attempt id mismatch: %q vs %q", identity.AttemptID, req.RequestID)
	}
	if identity.ActionID != req.ActionID {
		t.Fatalf("action id mismatch: %q vs %q", identity.ActionID, req.ActionID)
	}
	if identity.OperationKind != req.Operation {
		t.Fatalf("operation kind mismatch: %q vs %q", identity.OperationKind, req.Operation)
	}
	if identity.OperationVersion != req.OperationVersion {
		t.Fatalf("operation version mismatch: %d vs %d", identity.OperationVersion, req.OperationVersion)
	}
	if identity.RequestDigest != req.RequestDigest {
		t.Fatalf("request digest mismatch: %q vs %q", identity.RequestDigest, req.RequestDigest)
	}
	if identity.AgentID != "agent-42" {
		t.Fatalf("agent id not trimmed: %q", identity.AgentID)
	}

	t.Run("empty agent id passes through trimmed", func(t *testing.T) {
		empty := DockerContainerLifecycleOperationIdentity("   ", req)
		if empty.AgentID != "" {
			t.Fatalf("empty agent id not trimmed to empty: %q", empty.AgentID)
		}
	})
}

// --- ValidateDockerContainerLifecycleResultForRequest: every cross-validation branch ---

func TestValidateDockerContainerLifecycleResultForRequestBranches(t *testing.T) {
	req := newBoundLifecyclePayload(t)

	t.Run("happy path", func(t *testing.T) {
		result := newValidCompleteLifecycleResult(req)
		if err := ValidateDockerContainerLifecycleResultForRequest(req, result); err != nil {
			t.Fatalf("expected success, got: %v", err)
		}
	})

	t.Run("invalid request rejected before result checked", func(t *testing.T) {
		badReq := req
		badReq.RequestID = ""
		result := newValidCompleteLifecycleResult(req)
		err := ValidateDockerContainerLifecycleResultForRequest(badReq, result)
		if err == nil || !strings.Contains(err.Error(), "request or action id") {
			t.Fatalf("expected request-id error, got: %v", err)
		}
	})

	t.Run("invalid result rejected", func(t *testing.T) {
		result := newValidCompleteLifecycleResult(req)
		result.ExecutionPhase = "frobnicate"
		err := ValidateDockerContainerLifecycleResultForRequest(req, result)
		if err == nil || !strings.Contains(err.Error(), "execution phase") {
			t.Fatalf("expected execution-phase error, got: %v", err)
		}
	})

	// NOTE: the result.OperationVersion != req.OperationVersion operand in the
	// identity-mismatch guard (docker_lifecycle_codec.go:191) is effectively
	// dead: both operands are forced to DockerContainerLifecycleOperationVersion
	// (== 1) by their respective validators, so the version can never differ on
	// a pair that reaches that comparison. Operation-kind mismatch, by
	// contrast, IS reachable (start vs stop are both individually valid), and
	// is covered below. The dead version branch is noted in GLM_REPORT.md.
	for _, tc := range []struct {
		name      string
		mutate    func(*DockerContainerLifecycleResultPayload)
		wantError string
	}{
		{
			name:      "request id mismatch",
			mutate:    func(r *DockerContainerLifecycleResultPayload) { r.RequestID = "other-request" },
			wantError: "identity mismatch",
		},
		{
			name:      "action id mismatch",
			mutate:    func(r *DockerContainerLifecycleResultPayload) { r.ActionID = "other-action" },
			wantError: "identity mismatch",
		},
		{
			name:      "operation kind mismatch",
			mutate:    func(r *DockerContainerLifecycleResultPayload) { r.Operation = DockerContainerOperationStop },
			wantError: "identity mismatch",
		},
		{
			name:      "request digest mismatch on result",
			mutate:    func(r *DockerContainerLifecycleResultPayload) { r.RequestDigest = testValidLifecycleDigestAlt },
			wantError: "identity mismatch",
		},
		{
			name:      "container mismatch on result",
			mutate:    func(r *DockerContainerLifecycleResultPayload) { r.ContainerID = testValidLifecycleContainerIDB },
			wantError: "identity mismatch",
		},
		{
			name: "before-state container mismatch",
			mutate: func(r *DockerContainerLifecycleResultPayload) {
				r.Before = DockerContainerLifecycleSnapshot{
					ContainerID: testValidLifecycleContainerIDB,
				}
			},
			wantError: "before-state container mismatch",
		},
		{
			name: "after-state container mismatch",
			mutate: func(r *DockerContainerLifecycleResultPayload) {
				r.After = DockerContainerLifecycleSnapshot{
					ContainerID: testValidLifecycleContainerIDB,
					ObservedAt:  time.Date(2026, 7, 17, 12, 0, 0, 0, time.UTC),
				}
			},
			wantError: "after-state container mismatch",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := newValidCompleteLifecycleResult(req)
			tc.mutate(&result)
			err := ValidateDockerContainerLifecycleResultForRequest(req, result)
			if err == nil {
				t.Fatalf("expected error containing %q, got success", tc.wantError)
			}
			if !strings.Contains(err.Error(), tc.wantError) {
				t.Fatalf("error %q does not contain %q", err.Error(), tc.wantError)
			}
		})
	}

	t.Run("after container equal to request by case is accepted", func(t *testing.T) {
		result := newValidCompleteLifecycleResult(req)
		// The request container id is lowercase hex; force the after-state to
		// the same id but uppercase to confirm EqualFold accepts it.
		result.After = DockerContainerLifecycleSnapshot{
			ContainerID: strings.ToUpper(req.ContainerID),
			State:       "running",
			Running:     true,
			ObservedAt:  time.Date(2026, 7, 17, 12, 0, 0, 0, time.UTC),
		}
		if err := ValidateDockerContainerLifecycleResultForRequest(req, result); err != nil {
			t.Fatalf("expected case-insensitive after-state match to pass, got: %v", err)
		}
	})
}
