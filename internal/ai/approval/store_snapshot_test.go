package approval

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

func approvalSnapshotFixture() *ApprovalRequest {
	preflight := &ActionPreflight{
		Target: "agent:fixture-host", SafetyChecks: []string{"fixture check"},
		VerificationSteps: []string{"fixture verification"},
	}
	return &ApprovalRequest{
		ID: "fixture-approval", OrgID: "fixture-org", ExecutionID: "fixture-execution",
		Command: "printf fixture", TargetType: "agent", TargetID: "fixture-host", TargetName: "Fixture host",
		ContextConfidence: &ContextConfidence{Level: ContextConfidenceVerified, Evidence: []string{"fixture evidence"}},
		Preflight:         preflight,
		Plan: &unifiedresources.ActionPlan{
			ActionID: "fixture-action", RequiresApproval: true, Preflight: preflight,
			PredictedBlastRadius: []string{"fixture-host"},
			PolicyDecision: unifiedresources.ActionPolicyDecisionProvenance{
				Authorities: []unifiedresources.ActionPolicyAuthorityFactor{{
					SourceID: "fixture-policy", ReasonCodes: []unifiedresources.ActionPolicyReasonCode{"fixture-reason"},
				}},
			},
		},
	}
}

func newApprovalSnapshotStore(t *testing.T) *Store {
	t.Helper()
	store, err := NewStore(StoreConfig{DataDir: t.TempDir(), DisablePersistence: true})
	if err != nil {
		t.Fatal(err)
	}
	return store
}

func approvalSnapshotJSON(t *testing.T, req *ApprovalRequest) string {
	t.Helper()
	body, err := json.Marshal(req)
	if err != nil {
		t.Fatal(err)
	}
	return string(body)
}

func mutateApprovalSnapshot(req *ApprovalRequest) {
	req.OrgID = "changed-fixture-org"
	req.Command = "changed fixture command"
	req.Status = StatusDenied
	req.Consumed = !req.Consumed
	if req.DecidedAt != nil {
		*req.DecidedAt = req.DecidedAt.Add(-time.Hour)
	}
	req.ContextConfidence.Evidence[0] = "changed evidence"
	req.Preflight.SafetyChecks[0] = "changed check"
	req.Preflight.VerificationSteps[0] = "changed verification"
	req.Plan.PredictedBlastRadius[0] = "changed target"
	req.Plan.PolicyDecision.Authorities[0].SourceID = "changed policy"
	req.Plan.PolicyDecision.Authorities[0].ReasonCodes[0] = "changed reason"
	req.Plan.Preflight.Target = "changed preflight target"
}

func TestApprovalSnapshotsCreationOwnsRequest(t *testing.T) {
	store := newApprovalSnapshotStore(t)
	req := approvalSnapshotFixture()
	if err := store.CreateApproval(req); err != nil {
		t.Fatal(err)
	}
	want := approvalSnapshotJSON(t, req)
	mutateApprovalSnapshot(req)
	got, ok := store.GetApproval("fixture-approval")
	if !ok || approvalSnapshotJSON(t, got) != want {
		t.Fatal("editing the creation input changed the store-owned approval")
	}
	if len(store.GetPendingApprovalsForOrg("fixture-org")) != 1 {
		t.Fatal("editing the creation input changed organization membership")
	}
}

func TestApprovalSnapshotsReadOwnership(t *testing.T) {
	readers := map[string]func(*Store) *ApprovalRequest{
		"detail":       func(s *Store) *ApprovalRequest { req, _ := s.GetApproval("fixture-approval"); return req },
		"pending":      func(s *Store) *ApprovalRequest { return s.GetPendingApprovals()[0] },
		"organization": func(s *Store) *ApprovalRequest { return s.GetPendingApprovalsForOrg("fixture-org")[0] },
		"execution":    func(s *Store) *ApprovalRequest { return s.GetApprovalsByExecution("fixture-execution")[0] },
		"expired_detail": func(s *Store) *ApprovalRequest {
			s.mu.Lock()
			s.approvals["fixture-approval"].ExpiresAt = time.Unix(1, 0)
			s.mu.Unlock()
			req, _ := s.GetApproval("fixture-approval")
			return req
		},
	}
	for name, read := range readers {
		t.Run(name, func(t *testing.T) {
			store := newApprovalSnapshotStore(t)
			if err := store.CreateApproval(approvalSnapshotFixture()); err != nil {
				t.Fatal(err)
			}
			got := read(store)
			want := approvalSnapshotJSON(t, got)
			mutateApprovalSnapshot(got)
			stored, found := store.GetApproval("fixture-approval")
			if !found || approvalSnapshotJSON(t, stored) != want {
				t.Fatal("editing a read snapshot changed the store-owned approval")
			}
		})
	}
}

func TestApprovalSnapshotsDecisionOwnership(t *testing.T) {
	for _, decision := range []string{"approve", "approve_again", "deny", "consume"} {
		t.Run(decision, func(t *testing.T) {
			store := newApprovalSnapshotStore(t)
			if err := store.CreateApproval(approvalSnapshotFixture()); err != nil {
				t.Fatal(err)
			}
			var got *ApprovalRequest
			var err error
			switch decision {
			case "approve", "approve_again", "consume":
				got, err = store.Approve("fixture-approval", "fixture-reviewer")
				if err == nil && decision == "approve_again" {
					got, err = store.Approve("fixture-approval", "other-fixture-reviewer")
				}
				if err == nil && decision == "consume" {
					got, err = store.ConsumeApproval("fixture-approval", "printf fixture", "agent", "fixture-host")
				}
			case "deny":
				got, err = store.Deny("fixture-approval", "fixture-reviewer", "fixture reason")
			}
			if err != nil {
				t.Fatal(err)
			}
			want := approvalSnapshotJSON(t, got)
			mutateApprovalSnapshot(got)
			stored, _ := store.GetApproval("fixture-approval")
			if approvalSnapshotJSON(t, stored) != want {
				t.Fatal("editing a decision result changed the store-owned approval")
			}
		})
	}
}

func TestApprovalSnapshotsRetainedAcrossTransitions(t *testing.T) {
	store := newApprovalSnapshotStore(t)
	input := approvalSnapshotFixture()
	if err := store.CreateApproval(input); err != nil {
		t.Fatal(err)
	}
	pending, _ := store.GetApproval(input.ID)
	approved, err := store.Approve(input.ID, "fixture-reviewer")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = store.ConsumeApproval(input.ID, input.Command, input.TargetType, input.TargetID); err != nil {
		t.Fatal(err)
	}
	if input.Status != StatusPending || pending.Status != StatusPending || pending.DecidedAt != nil {
		t.Fatal("a later decision rewrote an earlier pending snapshot")
	}
	if approved.Status != StatusApproved || approved.Consumed {
		t.Fatal("consumption rewrote the returned approval-decision snapshot")
	}
}

func TestApprovalSnapshotsCallbackOwnership(t *testing.T) {
	store := newApprovalSnapshotStore(t)
	result := make(chan *ApprovalRequest, 1)
	store.SetOnApprovalCreated(func(req *ApprovalRequest) { result <- req })
	input := approvalSnapshotFixture()
	if err := store.CreateApproval(input); err != nil {
		t.Fatal(err)
	}
	want := approvalSnapshotJSON(t, input)
	select {
	case callback := <-result:
		mutateApprovalSnapshot(callback)
		got, _ := store.GetApproval(input.ID)
		if approvalSnapshotJSON(t, got) != want || approvalSnapshotJSON(t, input) != want {
			t.Fatal("editing callback context changed the store or creation input")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("creation callback did not finish")
	}
}

func TestApprovalSnapshotsConcurrentDecisionJSON(t *testing.T) {
	store := newApprovalSnapshotStore(t)
	var retained []*ApprovalRequest
	for i := 0; i < 32; i++ {
		req := approvalSnapshotFixture()
		req.ID = fmt.Sprintf("fixture-%d", i)
		if err := store.CreateApproval(req); err != nil {
			t.Fatal(err)
		}
		got, _ := store.GetApproval(req.ID)
		retained = append(retained, got)
	}
	start := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		close(start)
		for i := 0; i < 100; i++ {
			if _, err := json.Marshal(retained); err != nil {
				t.Error(err)
			}
		}
	}()
	go func() {
		defer wg.Done()
		<-start
		for i := 0; i < 32; i++ {
			id := fmt.Sprintf("fixture-%d", i)
			if _, err := store.Approve(id, "fixture-reviewer"); err != nil {
				t.Error(err)
			}
			if _, err := store.ConsumeApproval(id, "printf fixture", "agent", "fixture-host"); err != nil {
				t.Error(err)
			}
		}
	}()
	wg.Wait()
	for _, req := range retained {
		if req.Status != StatusPending || req.Consumed {
			t.Fatal("a concurrent decision changed a retained serialization snapshot")
		}
	}
}

func TestApprovalSnapshotsPersistenceOwnsRequest(t *testing.T) {
	for _, scheduled := range []bool{false, true} {
		t.Run(fmt.Sprintf("scheduled_%t", scheduled), func(t *testing.T) {
			store, err := NewStore(StoreConfig{DataDir: t.TempDir()})
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(store.Flush)
			input := approvalSnapshotFixture()
			if err := store.CreateApproval(input); err != nil {
				t.Fatal(err)
			}
			want := approvalSnapshotJSON(t, input)
			mutateApprovalSnapshot(input)
			if scheduled {
				// Exercise the real callback, without waiting its production debounce.
				store.mu.Lock()
				store.saveTimer.Reset(0)
				store.mu.Unlock()
			} else {
				store.Flush()
			}
			deadline := time.Now().Add(2 * time.Second)
			for {
				data, readErr := os.ReadFile(store.approvalsFile())
				var saved []*ApprovalRequest
				if readErr == nil && json.Unmarshal(data, &saved) == nil && len(saved) == 1 {
					if approvalSnapshotJSON(t, saved[0]) != want {
						t.Fatal("persistence saved edits to a detached creation input")
					}
					break
				}
				if time.Now().After(deadline) {
					t.Fatal("approval snapshot was not persisted")
				}
				time.Sleep(time.Millisecond)
			}
		})
	}
}

func TestApprovalSnapshotsPreservePolicyJSONAndPreflightIdentity(t *testing.T) {
	for _, empty := range []bool{false, true} {
		for _, shared := range []bool{false, true} {
			t.Run(fmt.Sprintf("empty_%t/shared_%t", empty, shared), func(t *testing.T) {
				req := approvalSnapshotFixture()
				if empty {
					req.Plan.PolicyDecision.Authorities[0].ReasonCodes = []unifiedresources.ActionPolicyReasonCode{}
				} else {
					req.Plan.PolicyDecision.Authorities[0].ReasonCodes = nil
				}
				if !shared {
					preflight := *req.Preflight
					req.Plan.Preflight = &preflight
				}
				copy := snapshotApproval(req)
				if approvalSnapshotJSON(t, copy) != approvalSnapshotJSON(t, req) {
					t.Fatal("copy changed policy JSON or empty-slice shape")
				}
				if (copy.Plan.Preflight == copy.Preflight) != shared {
					t.Fatal("copy changed internal preflight identity")
				}
				copy.Plan.Preflight.Target = "changed fixture target"
				if req.Plan.Preflight.Target == copy.Plan.Preflight.Target {
					t.Fatal("copy retained the source preflight pointer")
				}
				if empty {
					req.Plan.PolicyDecision.Authorities = []unifiedresources.ActionPolicyAuthorityFactor{}
				} else {
					req.Plan.PolicyDecision.Authorities = nil
				}
				if approvalSnapshotJSON(t, snapshotApproval(req)) != approvalSnapshotJSON(t, req) {
					t.Fatal("copy changed nil/empty authorities")
				}
			})
		}
	}
	if snapshotApproval(nil) != nil {
		t.Fatal("nil request did not remain nil")
	}
	plain := &ApprovalRequest{ID: "plain-fixture"}
	if approvalSnapshotJSON(t, snapshotApproval(plain)) != approvalSnapshotJSON(t, plain) {
		t.Fatal("copy populated absent optional fields")
	}
}

func TestApprovalSnapshotsConcurrentSavesKeepConsumedDecision(t *testing.T) {
	store, err := NewStore(StoreConfig{DataDir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(store.Flush)
	input := approvalSnapshotFixture()
	if err := store.CreateApproval(input); err != nil {
		t.Fatal(err)
	}
	store.Flush()
	start, done := make(chan struct{}), make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		<-start
		for i := 0; i < 40; i++ {
			store.Flush()
		}
	}()
	go func() {
		defer wg.Done()
		<-start
		for i := 0; i < 40; i++ {
			store.save()
		}
	}()
	go func() { wg.Wait(); close(done) }()
	close(start)
	if _, err := store.Approve(input.ID, "fixture-reviewer"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.ConsumeApproval(input.ID, input.Command, input.TargetType, input.TargetID); err != nil {
		t.Fatal(err)
	}
	// The consumption's timer may race Flush; the final publication must not
	// regress it, and readers must never observe a truncated in-place write.
	store.Flush()
	for {
		data, err := os.ReadFile(store.approvalsFile())
		if err != nil || !json.Valid(data) {
			t.Fatalf("concurrent save exposed an incomplete approval file: %v", err)
		}
		select {
		case <-done:
			reloaded, err := NewStore(StoreConfig{DataDir: store.dataDir})
			if err != nil {
				t.Fatal(err)
			}
			got, found := reloaded.GetApproval(input.ID)
			if !found || !got.Consumed || got.Status != StatusApproved {
				t.Fatal("concurrent saves lost the consumed approval decision")
			}
			if _, err := reloaded.ConsumeApproval(input.ID, input.Command, input.TargetType, input.TargetID); err == nil {
				t.Fatal("reload allowed a consumed approval to be reused")
			}
			return
		default:
			time.Sleep(time.Millisecond)
		}
	}
}

func TestApprovalStoreFilePublicationIsPrivateAndAtomic(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/approvals.json"
	if err := writeStoreFile(path, []byte("fixture-original")); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatalf("store file must be owner-only: %v", err)
	}
	if err := writeStoreFile(path, []byte("fixture-replacement")); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != "fixture-replacement" {
		t.Fatalf("complete replacement missing: %v", err)
	}
	// A failed rename must not damage the destination or leave a temp file.
	blocked := dir + "/blocked"
	if err := os.Mkdir(blocked, 0700); err != nil {
		t.Fatal(err)
	}
	if err := writeStoreFile(blocked, []byte("fixture-not-published")); err == nil {
		t.Fatal("publishing over a directory unexpectedly succeeded")
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 2 {
		t.Fatalf("publication left temporary files: %v", err)
	}
	if _, err := os.Stat(blocked); err != nil {
		t.Fatal("failed publication damaged its destination")
	}
}
