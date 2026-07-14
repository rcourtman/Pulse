package agentexec

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// This file is a table-driven coverage test for the docker container update
// codec. It exercises every public codec function in docker_update_codec.go
// plus the unexported digest helper, with emphasis on the validation guards,
// the strict-JSON rejection shape (unknown fields / trailing data / malformed
// JSON / empty body), and the request/result cross-validation mismatch
// branches. It is the strict-codec sibling of apt_codec_test.go and asserts
// the same closed-contract rejection behaviour.

var (
	testValidUpdateContainerID    = "abcdef123456"
	testValidUpdateContainerIDB   = "0123456789ab"
	testValidUpdateImageDigest    = "sha256:" + strings.Repeat("a", 64)
	testValidUpdateImageDigestAlt = "sha256:" + strings.Repeat("b", 64)
	testValidUpdateRequestDigest  = "sha256:" + strings.Repeat("c", 64)
)

// newBoundUpdatePayload returns a fully bound, valid DockerContainerUpdatePayload
// suitable for both validation and decode round-trips.
func newBoundUpdatePayload(t *testing.T) DockerContainerUpdatePayload {
	t.Helper()
	payload := DockerContainerUpdatePayload{
		RequestID:           "update-request-1",
		ActionID:            "update-action-1",
		Runtime:             "docker",
		ContainerID:         testValidUpdateContainerID,
		ExpectedImageDigest: testValidUpdateImageDigest,
		Timeout:             600,
	}
	if err := BindDockerContainerUpdatePayload(&payload); err != nil {
		t.Fatalf("bind baseline update payload: %v", err)
	}
	return payload
}

// newValidUpdateResult returns a result that passes ValidateDockerContainerUpdateResultPayload
// on its own (preflight phase, no completion requirements).
func newValidUpdateResult() DockerContainerUpdateResultPayload {
	return DockerContainerUpdateResultPayload{
		RequestID:        "update-request-1",
		ActionID:         "update-action-1",
		Operation:        DockerContainerOperationUpdate,
		OperationVersion: DockerContainerUpdateOperationVersion,
		RequestDigest:    testValidUpdateRequestDigest,
		ContainerID:      testValidUpdateContainerID,
		ExecutionPhase:   DockerContainerPhasePreflight,
	}
}

// newValidCompleteUpdateResult returns a result in the terminal phase that
// satisfies every completion guard (replacement container, mutation done, no
// error, readback observation).
func newValidCompleteUpdateResult(req DockerContainerUpdatePayload) DockerContainerUpdateResultPayload {
	observed := time.Date(2026, 7, 14, 12, 0, 0, 0, time.UTC)
	return DockerContainerUpdateResultPayload{
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
		NewContainerID:    testValidUpdateContainerIDB,
		After: DockerContainerLifecycleSnapshot{
			ContainerID: testValidUpdateContainerIDB,
			State:       "running",
			Running:     true,
			ObservedAt:  observed,
		},
	}
}

// --- DecodeDockerContainerUpdatePayload: strict acceptance & rejection ---

func TestDecodeDockerContainerUpdatePayloadStrictAcceptance(t *testing.T) {
	valid := newBoundUpdatePayload(t)
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
			body:      strings.TrimSuffix(base, "}") + `,"command":"docker pull"}`,
			wantError: "unknown field",
		},
		{
			name:      "trailing json rejected",
			body:      base + `{}`,
			wantError: "trailing",
		},
		{
			name:      "missing required request id rejected at validate step",
			body:      rejson(t, valid, "request_id", ""),
			wantError: "request or action id",
		},
		{
			name:      "non-digest expected image rejected at validate step",
			body:      rejson(t, mutatePayload(valid, func(p *DockerContainerUpdatePayload) { p.ExpectedImageDigest = "busybox:latest" }), "", ""),
			wantError: "expected image digest",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			decoded, err := DecodeDockerContainerUpdatePayload([]byte(tc.body))
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

// --- DecodeDockerContainerUpdateResultPayload: strict acceptance & rejection ---

func TestDecodeDockerContainerUpdateResultPayloadStrictAcceptance(t *testing.T) {
	validResult := newValidCompleteUpdateResult(newBoundUpdatePayload(t))
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
			body:      rejson(t, mutateResult(validResult, func(r *DockerContainerUpdateResultPayload) { r.ExecutionPhase = "frobnicate" }), "", ""),
			wantError: "execution phase",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			decoded, err := DecodeDockerContainerUpdateResultPayload([]byte(tc.body))
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

// --- BindDockerContainerUpdatePayload ---

func TestBindDockerContainerUpdatePayload(t *testing.T) {
	t.Run("nil payload rejected", func(t *testing.T) {
		if err := BindDockerContainerUpdatePayload(nil); err == nil {
			t.Fatal("nil payload was accepted by bind")
		}
	})

	t.Run("bind stamps operation version and recomputes digest", func(t *testing.T) {
		payload := DockerContainerUpdatePayload{
			RequestID:           "r1",
			ActionID:            "a1",
			Runtime:             "  Docker  ",
			ContainerID:         "  ABCDEF123456  ",
			ExpectedImageDigest: "  SHA256:" + strings.Repeat("A", 64) + "  ",
		}
		if err := BindDockerContainerUpdatePayload(&payload); err != nil {
			t.Fatalf("bind failed: %v", err)
		}
		if payload.Operation != DockerContainerOperationUpdate {
			t.Fatalf("operation not stamped: %q", payload.Operation)
		}
		if payload.OperationVersion != DockerContainerUpdateOperationVersion {
			t.Fatalf("operation version not stamped: %d", payload.OperationVersion)
		}
		if payload.RequestDigest == "" {
			t.Fatal("request digest not stamped")
		}
		// The bound payload must pass validation once its raw fields are
		// trimmed/lowercased (validation normalises the same way the digest
		// does, so the stamped digest matches).
		if err := ValidateDockerContainerUpdatePayload(&payload); err != nil {
			t.Fatalf("bound payload did not validate: %v", err)
		}
	})
}

// --- dockerContainerUpdateRequestDigest (white-box) ---

func TestDockerContainerUpdateRequestDigest(t *testing.T) {
	base := DockerContainerUpdatePayload{
		ActionID:            "action-1",
		Operation:           DockerContainerOperationUpdate,
		OperationVersion:    DockerContainerUpdateOperationVersion,
		Runtime:             "docker",
		ContainerID:         testValidUpdateContainerID,
		ExpectedImageDigest: testValidUpdateImageDigest,
	}
	digestA, err := dockerContainerUpdateRequestDigest(base)
	if err != nil {
		t.Fatalf("digest failed: %v", err)
	}
	if !strings.HasPrefix(digestA, "sha256:") {
		t.Fatalf("digest not sha256-prefixed: %q", digestA)
	}

	t.Run("deterministic for identical input", func(t *testing.T) {
		digestB, err := dockerContainerUpdateRequestDigest(base)
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
		upper.ExpectedImageDigest = "  SHA256:" + strings.Repeat("A", 64) + "  "
		upper.ActionID = "  action-1  "
		digestUpper, err := dockerContainerUpdateRequestDigest(upper)
		if err != nil {
			t.Fatal(err)
		}
		if digestUpper != digestA {
			t.Fatalf("digest not normalised for case/whitespace: %q vs %q", digestUpper, digestA)
		}
	})

	t.Run("action id change alters digest", func(t *testing.T) {
		changed := base
		changed.ActionID = "action-2"
		digestChanged, err := dockerContainerUpdateRequestDigest(changed)
		if err != nil {
			t.Fatal(err)
		}
		if digestChanged == digestA {
			t.Fatal("action id change did not alter digest")
		}
	})
}

// --- ValidateDockerContainerUpdatePayload: every branch ---

func TestValidateDockerContainerUpdatePayloadBranches(t *testing.T) {
	base := newBoundUpdatePayload(t)

	for _, tc := range []struct {
		name      string
		mutate    func(*DockerContainerUpdatePayload)
		wantError string
	}{
		{
			name:   "valid happy path",
			mutate: func(*DockerContainerUpdatePayload) {},
		},
		{
			name:      "missing request id",
			mutate:    func(p *DockerContainerUpdatePayload) { p.RequestID = "" },
			wantError: "request or action id",
		},
		{
			name:      "missing action id",
			mutate:    func(p *DockerContainerUpdatePayload) { p.ActionID = "" },
			wantError: "request or action id",
		},
		{
			name:      "oversized request id",
			mutate:    func(p *DockerContainerUpdatePayload) { p.RequestID = strings.Repeat("x", maxRequestIDLength+1) },
			wantError: "request or action id",
		},
		{
			name:      "oversized action id",
			mutate:    func(p *DockerContainerUpdatePayload) { p.ActionID = strings.Repeat("y", maxRequestIDLength+1) },
			wantError: "request or action id",
		},
		{
			name:      "unsupported operation",
			mutate:    func(p *DockerContainerUpdatePayload) { p.Operation = DockerContainerOperationRestart },
			wantError: "unsupported docker container update operation",
		},
		{
			name:      "unsupported operation version",
			mutate:    func(p *DockerContainerUpdatePayload) { p.OperationVersion = 99 },
			wantError: "unsupported docker container update operation version",
		},
		{
			name:      "unsupported runtime",
			mutate:    func(p *DockerContainerUpdatePayload) { p.Runtime = "containerd" },
			wantError: "unsupported container runtime",
		},
		{
			name:      "podman runtime accepted",
			mutate:    func(p *DockerContainerUpdatePayload) { p.Runtime = "podman" },
			wantError: "", // podman is accepted; bound digest already covers it though, so this needs rebinding
		},
		{
			name:      "container id wrong format",
			mutate:    func(p *DockerContainerUpdatePayload) { p.ContainerID = "xyz123" },
			wantError: "immutable hexadecimal id",
		},
		{
			name:      "container id too short",
			mutate:    func(p *DockerContainerUpdatePayload) { p.ContainerID = "abc" },
			wantError: "immutable hexadecimal id",
		},
		{
			name:      "expected image digest not a digest",
			mutate:    func(p *DockerContainerUpdatePayload) { p.ExpectedImageDigest = "busybox:latest" },
			wantError: "invalid docker update expected image digest",
		},
		{
			name:      "expected image digest tampered but still pattern-valid triggers digest mismatch",
			mutate:    func(p *DockerContainerUpdatePayload) { p.ExpectedImageDigest = testValidUpdateImageDigestAlt },
			wantError: "request digest mismatch",
		},
		{
			name:      "request digest tampered directly",
			mutate:    func(p *DockerContainerUpdatePayload) { p.RequestDigest = testValidUpdateImageDigestAlt },
			wantError: "request digest mismatch",
		},
		{
			name:      "negative timeout rejected",
			mutate:    func(p *DockerContainerUpdatePayload) { p.Timeout = -1 },
			wantError: "timeout must be between 0 and",
		},
		{
			name:      "timeout exceeding maximum rejected",
			mutate:    func(p *DockerContainerUpdatePayload) { p.Timeout = maxDockerContainerUpdateTimeoutSeconds + 1 },
			wantError: "timeout must be between 0 and",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := base
			tc.mutate(&p)
			// podman runtime is accepted; its digest must be rebound for the
			// digest-mismatch check to stay quiet so we observe the runtime
			// branch's accept path rather than a digest drift.
			if p.Runtime == "podman" {
				if err := BindDockerContainerUpdatePayload(&p); err != nil {
					t.Fatalf("rebind for podman failed: %v", err)
				}
			}
			err := ValidateDockerContainerUpdatePayload(&p)
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
		if err := ValidateDockerContainerUpdatePayload(nil); err == nil {
			t.Fatal("nil payload accepted")
		}
	})

	t.Run("zero timeout is defaulted", func(t *testing.T) {
		p := base
		p.Timeout = 0
		if err := ValidateDockerContainerUpdatePayload(&p); err != nil {
			t.Fatalf("zero timeout should be defaulted, got: %v", err)
		}
		if p.Timeout != defaultDockerContainerUpdateTimeoutSeconds {
			t.Fatalf("timeout not defaulted: got %d want %d", p.Timeout, defaultDockerContainerUpdateTimeoutSeconds)
		}
	})

	t.Run("exact maximum timeout accepted", func(t *testing.T) {
		p := base
		p.Timeout = maxDockerContainerUpdateTimeoutSeconds
		if err := ValidateDockerContainerUpdatePayload(&p); err != nil {
			t.Fatalf("maximum timeout should be accepted, got: %v", err)
		}
	})
}

// --- ValidateDockerContainerUpdateResultPayload: every branch ---

func TestValidateDockerContainerUpdateResultPayloadBranches(t *testing.T) {
	base := newValidCompleteUpdateResult(newBoundUpdatePayload(t))

	for _, tc := range []struct {
		name      string
		mutate    func(*DockerContainerUpdateResultPayload)
		wantError string
	}{
		{
			name:      "valid complete happy path",
			mutate:    func(*DockerContainerUpdateResultPayload) {},
			wantError: "",
		},
		{
			name:      "missing request id",
			mutate:    func(r *DockerContainerUpdateResultPayload) { r.RequestID = "" },
			wantError: "result identity",
		},
		{
			name:      "missing action id",
			mutate:    func(r *DockerContainerUpdateResultPayload) { r.ActionID = "" },
			wantError: "result identity",
		},
		{
			name:      "oversized request id",
			mutate:    func(r *DockerContainerUpdateResultPayload) { r.RequestID = strings.Repeat("x", maxRequestIDLength+1) },
			wantError: "result identity",
		},
		{
			name:      "unsupported operation",
			mutate:    func(r *DockerContainerUpdateResultPayload) { r.Operation = DockerContainerOperationRestart },
			wantError: "unsupported docker update result operation",
		},
		{
			name:      "wrong operation version in binding",
			mutate:    func(r *DockerContainerUpdateResultPayload) { r.OperationVersion = 99 },
			wantError: "invalid docker update result binding",
		},
		{
			name:      "bad container id in binding",
			mutate:    func(r *DockerContainerUpdateResultPayload) { r.ContainerID = "xyz123" },
			wantError: "invalid docker update result binding",
		},
		{
			name:      "bad request digest format in binding",
			mutate:    func(r *DockerContainerUpdateResultPayload) { r.RequestDigest = "not-a-digest" },
			wantError: "invalid docker update result binding",
		},
		{
			name:      "unsupported execution phase",
			mutate:    func(r *DockerContainerUpdateResultPayload) { r.ExecutionPhase = "frobnicate" },
			wantError: "execution phase",
		},
		{
			name:      "invalid replacement container id",
			mutate:    func(r *DockerContainerUpdateResultPayload) { r.NewContainerID = "xyz123" },
			wantError: "invalid replacement container id",
		},
		{
			name: "error string exceeds bound",
			mutate: func(r *DockerContainerUpdateResultPayload) {
				r.Error = strings.Repeat("e", 1025)
				r.ExecutionPhase = DockerContainerPhaseMutate
				r.MutationCompleted = false
			},
			wantError: "exceeds bounded contract",
		},
		{
			name: "container name exceeds bound",
			mutate: func(r *DockerContainerUpdateResultPayload) {
				r.ContainerName = strings.Repeat("n", maxDockerContainerNameLength+1)
			},
			wantError: "exceeds bounded contract",
		},
		{
			name: "backup container name exceeds bound",
			mutate: func(r *DockerContainerUpdateResultPayload) {
				r.BackupContainer = strings.Repeat("b", maxDockerContainerNameLength+1)
			},
			wantError: "exceeds bounded contract",
		},
		{
			name: "old image digest exceeds length bound",
			mutate: func(r *DockerContainerUpdateResultPayload) {
				r.OldImageDigest = strings.Repeat("o", maxDockerImageDigestLength+1)
			},
			wantError: "digest exceeds bounded contract",
		},
		{
			name: "new image digest exceeds length bound",
			mutate: func(r *DockerContainerUpdateResultPayload) {
				r.NewImageDigest = strings.Repeat("m", maxDockerImageDigestLength+1)
			},
			wantError: "digest exceeds bounded contract",
		},
		{
			name: "mutation completed without mutation started",
			mutate: func(r *DockerContainerUpdateResultPayload) {
				r.ExecutionPhase = DockerContainerPhaseMutate
				r.MutationStarted = false
				r.MutationCompleted = true
				r.NewContainerID = ""
			},
			wantError: "completed docker update mutation requires mutation start",
		},
		{
			name: "rolled back without rollback attempted",
			mutate: func(r *DockerContainerUpdateResultPayload) {
				r.ExecutionPhase = DockerContainerPhaseMutate
				r.MutationStarted = true
				r.MutationCompleted = false
				r.RolledBack = true
				r.RollbackAttempted = false
				r.NewContainerID = ""
				r.Error = "create failed"
			},
			wantError: "rollback success requires a rollback attempt",
		},
		{
			name: "rollback attempted without mutation started",
			mutate: func(r *DockerContainerUpdateResultPayload) {
				r.ExecutionPhase = DockerContainerPhaseMutate
				r.MutationStarted = false
				r.MutationCompleted = false
				r.RollbackAttempted = true
				r.RolledBack = false
				r.NewContainerID = ""
			},
			wantError: "rollback requires mutation start",
		},
		{
			name: "readback ran without after observation",
			mutate: func(r *DockerContainerUpdateResultPayload) {
				r.ExecutionPhase = DockerContainerPhaseMutate
				r.MutationStarted = true
				r.MutationCompleted = false
				r.ReadbackRan = true
				r.After = DockerContainerLifecycleSnapshot{}
				r.NewContainerID = ""
			},
			wantError: "readback requires an observation",
		},
		{
			name:      "complete phase with error set",
			mutate:    func(r *DockerContainerUpdateResultPayload) { r.Error = "something went wrong" },
			wantError: "complete docker update requires a replacement container and no error",
		},
		{
			name:      "complete phase without mutation completed",
			mutate:    func(r *DockerContainerUpdateResultPayload) { r.MutationCompleted = false },
			wantError: "complete docker update requires a replacement container and no error",
		},
		{
			name:      "complete phase without replacement container",
			mutate:    func(r *DockerContainerUpdateResultPayload) { r.NewContainerID = "" },
			wantError: "complete docker update requires a replacement container and no error",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := base
			tc.mutate(&r)
			err := ValidateDockerContainerUpdateResultPayload(&r)
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
		if err := ValidateDockerContainerUpdateResultPayload(nil); err == nil {
			t.Fatal("nil result accepted")
		}
	})

	t.Run("preflight result without completion requirements accepted", func(t *testing.T) {
		r := newValidUpdateResult()
		if err := ValidateDockerContainerUpdateResultPayload(&r); err != nil {
			t.Fatalf("preflight result should be accepted, got: %v", err)
		}
	})

	t.Run("verify phase accepted", func(t *testing.T) {
		r := newValidCompleteUpdateResult(newBoundUpdatePayload(t))
		r.ExecutionPhase = DockerContainerPhaseVerify
		if err := ValidateDockerContainerUpdateResultPayload(&r); err != nil {
			t.Fatalf("verify phase should be accepted, got: %v", err)
		}
	})
}

// --- DockerContainerUpdateOperationIdentity ---

func TestDockerContainerUpdateOperationIdentity(t *testing.T) {
	req := newBoundUpdatePayload(t)
	identity := DockerContainerUpdateOperationIdentity("  agent-42  ", req)
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
	if identity.AgentID == "" {
		t.Fatal("empty agent id should still be passed through (trimmed)")
	}
}

// --- ValidateDockerContainerUpdateResultForRequest: every cross-validation branch ---

func TestValidateDockerContainerUpdateResultForRequestBranches(t *testing.T) {
	req := newBoundUpdatePayload(t)

	t.Run("happy path", func(t *testing.T) {
		result := newValidCompleteUpdateResult(req)
		if err := ValidateDockerContainerUpdateResultForRequest(req, result); err != nil {
			t.Fatalf("expected success, got: %v", err)
		}
	})

	t.Run("invalid request rejected before result checked", func(t *testing.T) {
		badReq := req
		badReq.RequestID = ""
		result := newValidCompleteUpdateResult(req)
		err := ValidateDockerContainerUpdateResultForRequest(badReq, result)
		if err == nil || !strings.Contains(err.Error(), "request or action id") {
			t.Fatalf("expected request-id error, got: %v", err)
		}
	})

	t.Run("invalid result rejected", func(t *testing.T) {
		result := newValidCompleteUpdateResult(req)
		result.ExecutionPhase = "frobnicate"
		err := ValidateDockerContainerUpdateResultForRequest(req, result)
		if err == nil || !strings.Contains(err.Error(), "execution phase") {
			t.Fatalf("expected execution-phase error, got: %v", err)
		}
	})

	// NOTE: result.Operation and result.OperationVersion mismatch branches
	// (line 186 of docker_update_codec.go) cannot be exercised here, because
	// ValidateDockerContainerUpdateResultPayload requires Operation ==
	// DockerContainerOperationUpdate and OperationVersion ==
	// DockerContainerUpdateOperationVersion to even pass; the request's own
	// validator enforces the same constants. Those two comparison operands
	// are therefore effectively dead in the cross-validator and are noted as
	// suspected dead branches in GLM_REPORT.md (not fixed).
	for _, tc := range []struct {
		name      string
		mutate    func(*DockerContainerUpdateResultPayload)
		wantError string
	}{
		{
			name:      "request id mismatch",
			mutate:    func(r *DockerContainerUpdateResultPayload) { r.RequestID = "other-request" },
			wantError: "identity mismatch",
		},
		{
			name:      "action id mismatch",
			mutate:    func(r *DockerContainerUpdateResultPayload) { r.ActionID = "other-action" },
			wantError: "identity mismatch",
		},
		{
			name:      "container mismatch on result",
			mutate:    func(r *DockerContainerUpdateResultPayload) { r.ContainerID = testValidUpdateContainerIDB },
			wantError: "container mismatch",
		},
		{
			name: "after-state container mismatch",
			mutate: func(r *DockerContainerUpdateResultPayload) {
				r.After = DockerContainerLifecycleSnapshot{
					ContainerID: "1234567890ab",
					ObservedAt:  time.Now().UTC(),
				}
			},
			wantError: "after-state container mismatch",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := newValidCompleteUpdateResult(req)
			tc.mutate(&result)
			err := ValidateDockerContainerUpdateResultForRequest(req, result)
			if err == nil {
				t.Fatalf("expected error containing %q, got success", tc.wantError)
			}
			if !strings.Contains(err.Error(), tc.wantError) {
				t.Fatalf("error %q does not contain %q", err.Error(), tc.wantError)
			}
		})
	}

	t.Run("after container equal to new container by case is accepted", func(t *testing.T) {
		result := newValidCompleteUpdateResult(req)
		// NewContainerID is lowercase hex; force the after-state to use the
		// same id but uppercase to confirm EqualFold passes.
		result.NewContainerID = testValidUpdateContainerID
		result.After.ContainerID = strings.ToUpper(testValidUpdateContainerID)
		if err := ValidateDockerContainerUpdateResultForRequest(req, result); err != nil {
			t.Fatalf("expected case-insensitive after-state match to pass, got: %v", err)
		}
	})
}

// --- helpers ---

// mutatePayload returns a copy of p with the mutator applied. Useful for
// building a base JSON whose fields have already passed Bind, then perturbing
// them for a decode test.
func mutatePayload(p DockerContainerUpdatePayload, mutator func(*DockerContainerUpdatePayload)) DockerContainerUpdatePayload {
	cp := p
	mutator(&cp)
	return cp
}

// mutateResult returns a copy of r with the mutator applied.
func mutateResult(r DockerContainerUpdateResultPayload, mutator func(*DockerContainerUpdateResultPayload)) DockerContainerUpdateResultPayload {
	cp := r
	mutator(&cp)
	return cp
}

// rejson marshals the given value to canonical JSON. If key/value are both
// non-empty, the resulting JSON object has its key overwritten with value
// (used to delete or rewrite a single field while staying strict-decodable).
// When key/value are empty, the value is marshalled unchanged.
func rejson(t *testing.T, v any, key, value string) string {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	if key == "" && value == "" {
		return string(raw)
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	if value == "" {
		delete(m, key)
	} else {
		m[key] = value
	}
	out, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	return string(out)
}
