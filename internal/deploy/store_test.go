package deploy

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestOpenRejectsSymlink(t *testing.T) {
	dir := t.TempDir()
	real := filepath.Join(dir, "real.db")
	link := filepath.Join(dir, "link.db")

	// Create real db first so the symlink target exists.
	s, err := Open(real)
	if err != nil {
		t.Fatalf("Open real: %v", err)
	}
	_ = s.Close()

	if err := os.Symlink(real, link); err != nil {
		t.Fatalf("Symlink: %v", err)
	}

	_, err = Open(link)
	if err == nil {
		t.Fatal("expected error opening symlink db path")
	}
	if !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("expected symlink error, got: %v", err)
	}
}

func testStore(t *testing.T) *Store {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "deploy_test.db")
	s, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func TestJobRoundTrip(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)

	job := &Job{
		ID:            "job-1",
		ClusterID:     "cluster-abc",
		ClusterName:   "prod-cluster",
		SourceAgentID: "agent-1",
		SourceNodeID:  "node-1",
		OrgID:         "org-1",
		Status:        JobQueued,
		MaxParallel:   2,
		RetryMax:      3,
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	if err := s.CreateJob(ctx, job); err != nil {
		t.Fatalf("CreateJob: %v", err)
	}

	got, err := s.GetJob(ctx, "job-1")
	if err != nil {
		t.Fatalf("GetJob: %v", err)
	}
	if got == nil {
		t.Fatal("GetJob returned nil")
	}
	if got.ClusterName != "prod-cluster" {
		t.Errorf("expected clusterName prod-cluster, got %s", got.ClusterName)
	}
	if got.Status != JobQueued {
		t.Errorf("expected status queued, got %s", got.Status)
	}
	if got.MaxParallel != 2 {
		t.Errorf("expected maxParallel 2, got %d", got.MaxParallel)
	}
	if got.CompletedAt != nil {
		t.Errorf("expected nil completedAt")
	}
}

func TestJobNotFound(t *testing.T) {
	s := testStore(t)
	got, err := s.GetJob(context.Background(), "nonexistent")
	if err != nil {
		t.Fatalf("GetJob: %v", err)
	}
	if got != nil {
		t.Fatal("expected nil for nonexistent job")
	}
}

func TestUpdateJobStatus(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)

	job := &Job{
		ID: "job-2", ClusterID: "c", ClusterName: "c", SourceAgentID: "a",
		SourceNodeID: "n", OrgID: "org-1", Status: JobQueued,
		MaxParallel: 2, RetryMax: 3, CreatedAt: now, UpdatedAt: now,
	}
	if err := s.CreateJob(ctx, job); err != nil {
		t.Fatalf("CreateJob: %v", err)
	}

	if err := s.UpdateJobStatus(ctx, "job-2", JobRunning); err != nil {
		t.Fatalf("UpdateJobStatus to running: %v", err)
	}
	got, _ := s.GetJob(ctx, "job-2")
	if got.Status != JobRunning {
		t.Errorf("expected running, got %s", got.Status)
	}
	if got.CompletedAt != nil {
		t.Error("expected nil completedAt for non-terminal status")
	}

	// Terminal status should set completedAt.
	if err := s.UpdateJobStatus(ctx, "job-2", JobSucceeded); err != nil {
		t.Fatalf("UpdateJobStatus to succeeded: %v", err)
	}
	got, _ = s.GetJob(ctx, "job-2")
	if got.Status != JobSucceeded {
		t.Errorf("expected succeeded, got %s", got.Status)
	}
	if got.CompletedAt == nil {
		t.Error("expected completedAt to be set for terminal status")
	}
}

func TestUpdateJobStatus_NotFound(t *testing.T) {
	s := testStore(t)
	err := s.UpdateJobStatus(context.Background(), "nonexistent", JobRunning)
	if err == nil {
		t.Fatal("expected error for nonexistent job")
	}
}

func TestListJobs(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)

	for i, id := range []string{"j1", "j2", "j3"} {
		j := &Job{
			ID: id, ClusterID: "c", ClusterName: "c", SourceAgentID: "a",
			SourceNodeID: "n", OrgID: "org-1", Status: JobQueued,
			MaxParallel: 2, RetryMax: 3,
			CreatedAt: now.Add(time.Duration(i) * time.Second),
			UpdatedAt: now.Add(time.Duration(i) * time.Second),
		}
		if err := s.CreateJob(ctx, j); err != nil {
			t.Fatalf("CreateJob %s: %v", id, err)
		}
	}
	// Different org.
	other := &Job{
		ID: "j4", ClusterID: "c", ClusterName: "c", SourceAgentID: "a",
		SourceNodeID: "n", OrgID: "org-2", Status: JobQueued,
		MaxParallel: 2, RetryMax: 3, CreatedAt: now, UpdatedAt: now,
	}
	if err := s.CreateJob(ctx, other); err != nil {
		t.Fatalf("CreateJob: %v", err)
	}

	jobs, err := s.ListJobs(ctx, "org-1", 10)
	if err != nil {
		t.Fatalf("ListJobs: %v", err)
	}
	if len(jobs) != 3 {
		t.Fatalf("expected 3 jobs for org-1, got %d", len(jobs))
	}
	// Should be ordered by created_at DESC.
	if jobs[0].ID != "j3" {
		t.Errorf("expected first job j3, got %s", jobs[0].ID)
	}

	// Limit.
	jobs, err = s.ListJobs(ctx, "org-1", 1)
	if err != nil {
		t.Fatalf("ListJobs limit=1: %v", err)
	}
	if len(jobs) != 1 {
		t.Fatalf("expected 1 job, got %d", len(jobs))
	}
}

func TestTargetRoundTrip(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)

	job := &Job{
		ID: "j1", ClusterID: "c", ClusterName: "c", SourceAgentID: "a",
		SourceNodeID: "n", OrgID: "default", Status: JobQueued,
		MaxParallel: 2, RetryMax: 3, CreatedAt: now, UpdatedAt: now,
	}
	if err := s.CreateJob(ctx, job); err != nil {
		t.Fatalf("CreateJob: %v", err)
	}

	target := &Target{
		ID: "t1", JobID: "j1", NodeID: "pve-node-2", NodeName: "pve2",
		NodeIP: "10.0.0.2", Arch: "amd64", Status: TargetPending,
		CreatedAt: now, UpdatedAt: now,
	}
	if err := s.CreateTarget(ctx, target); err != nil {
		t.Fatalf("CreateTarget: %v", err)
	}

	targets, err := s.GetTargetsForJob(ctx, "j1")
	if err != nil {
		t.Fatalf("GetTargetsForJob: %v", err)
	}
	if len(targets) != 1 {
		t.Fatalf("expected 1 target, got %d", len(targets))
	}
	if targets[0].NodeIP != "10.0.0.2" {
		t.Errorf("expected nodeIP 10.0.0.2, got %s", targets[0].NodeIP)
	}
	if targets[0].Arch != "amd64" {
		t.Errorf("expected arch amd64, got %s", targets[0].Arch)
	}
}

func TestUpdateTargetStatus(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)

	job := &Job{
		ID: "j1", ClusterID: "c", ClusterName: "c", SourceAgentID: "a",
		SourceNodeID: "n", OrgID: "default", Status: JobQueued,
		MaxParallel: 2, RetryMax: 3, CreatedAt: now, UpdatedAt: now,
	}
	_ = s.CreateJob(ctx, job)

	target := &Target{
		ID: "t1", JobID: "j1", NodeID: "n2", NodeName: "pve2",
		NodeIP: "10.0.0.2", Status: TargetPending,
		CreatedAt: now, UpdatedAt: now,
	}
	_ = s.CreateTarget(ctx, target)

	if err := s.UpdateTargetStatus(ctx, "t1", TargetFailedRetryable, "SSH timeout"); err != nil {
		t.Fatalf("UpdateTargetStatus: %v", err)
	}

	targets, _ := s.GetTargetsForJob(ctx, "j1")
	if targets[0].Status != TargetFailedRetryable {
		t.Errorf("expected failed_retryable, got %s", targets[0].Status)
	}
	if targets[0].ErrorMessage != "SSH timeout" {
		t.Errorf("expected error msg 'SSH timeout', got %q", targets[0].ErrorMessage)
	}
}

func TestUpdateTargetStatus_NotFound(t *testing.T) {
	s := testStore(t)
	err := s.UpdateTargetStatus(context.Background(), "nonexistent", TargetSucceeded, "")
	if err == nil {
		t.Fatal("expected error for nonexistent target")
	}
}

func TestIncrementTargetAttempts(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)

	job := &Job{
		ID: "j1", ClusterID: "c", ClusterName: "c", SourceAgentID: "a",
		SourceNodeID: "n", OrgID: "default", Status: JobQueued,
		MaxParallel: 2, RetryMax: 3, CreatedAt: now, UpdatedAt: now,
	}
	_ = s.CreateJob(ctx, job)

	target := &Target{
		ID: "t1", JobID: "j1", NodeID: "n2", NodeName: "pve2",
		NodeIP: "10.0.0.2", Status: TargetPending,
		CreatedAt: now, UpdatedAt: now,
	}
	_ = s.CreateTarget(ctx, target)

	for i := 0; i < 3; i++ {
		if err := s.IncrementTargetAttempts(ctx, "t1"); err != nil {
			t.Fatalf("IncrementTargetAttempts: %v", err)
		}
	}
	targets, _ := s.GetTargetsForJob(ctx, "j1")
	if targets[0].Attempts != 3 {
		t.Errorf("expected 3 attempts, got %d", targets[0].Attempts)
	}
}

func TestIncrementTargetAttempts_NotFound(t *testing.T) {
	s := testStore(t)
	err := s.IncrementTargetAttempts(context.Background(), "nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent target")
	}
}

func TestEventRoundTrip(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)

	job := &Job{
		ID: "j1", ClusterID: "c", ClusterName: "c", SourceAgentID: "a",
		SourceNodeID: "n", OrgID: "default", Status: JobQueued,
		MaxParallel: 2, RetryMax: 3, CreatedAt: now, UpdatedAt: now,
	}
	_ = s.CreateJob(ctx, job)

	events := []Event{
		{ID: "e1", JobID: "j1", Type: EventJobCreated, Message: "Job created", CreatedAt: now},
		{ID: "e2", JobID: "j1", TargetID: "t1", Type: EventTargetStatusChanged, Message: "pending → preflighting", CreatedAt: now.Add(time.Second)},
		{ID: "e3", JobID: "j1", TargetID: "t1", Type: EventPreflightResult, Message: "Preflight passed", Data: `{"arch":"amd64"}`, CreatedAt: now.Add(2 * time.Second)},
	}
	for _, e := range events {
		if err := s.AppendEvent(ctx, &e); err != nil {
			t.Fatalf("AppendEvent %s: %v", e.ID, err)
		}
	}

	// By job.
	jobEvents, err := s.GetEventsForJob(ctx, "j1")
	if err != nil {
		t.Fatalf("GetEventsForJob: %v", err)
	}
	if len(jobEvents) != 3 {
		t.Fatalf("expected 3 events, got %d", len(jobEvents))
	}
	if jobEvents[2].Data != `{"arch":"amd64"}` {
		t.Errorf("expected data json, got %q", jobEvents[2].Data)
	}

	// By target.
	targetEvents, err := s.GetEventsForTarget(ctx, "t1")
	if err != nil {
		t.Fatalf("GetEventsForTarget: %v", err)
	}
	if len(targetEvents) != 2 {
		t.Fatalf("expected 2 target events, got %d", len(targetEvents))
	}
}

func TestCascadeDelete(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)

	job := &Job{
		ID: "j1", ClusterID: "c", ClusterName: "c", SourceAgentID: "a",
		SourceNodeID: "n", OrgID: "default", Status: JobQueued,
		MaxParallel: 2, RetryMax: 3, CreatedAt: now, UpdatedAt: now,
	}
	_ = s.CreateJob(ctx, job)
	_ = s.CreateTarget(ctx, &Target{
		ID: "t1", JobID: "j1", NodeID: "n2", NodeName: "pve2",
		NodeIP: "10.0.0.2", Status: TargetPending, CreatedAt: now, UpdatedAt: now,
	})
	_ = s.AppendEvent(ctx, &Event{
		ID: "e1", JobID: "j1", TargetID: "t1", Type: EventJobCreated,
		Message: "created", CreatedAt: now,
	})

	// Delete the job directly — cascade should remove targets and events.
	_, err := s.db.ExecContext(ctx, "DELETE FROM deploy_jobs WHERE id = ?", "j1")
	if err != nil {
		t.Fatalf("DELETE: %v", err)
	}

	targets, _ := s.GetTargetsForJob(ctx, "j1")
	if len(targets) != 0 {
		t.Errorf("expected 0 targets after cascade, got %d", len(targets))
	}
	events, _ := s.GetEventsForJob(ctx, "j1")
	if len(events) != 0 {
		t.Errorf("expected 0 events after cascade, got %d", len(events))
	}
}

func TestPruneOldJobs(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)
	old := now.Add(-48 * time.Hour)

	// Old completed job.
	oldJob := &Job{
		ID: "old", ClusterID: "c", ClusterName: "c", SourceAgentID: "a",
		SourceNodeID: "n", OrgID: "default", Status: JobSucceeded,
		MaxParallel: 2, RetryMax: 3, CreatedAt: old, UpdatedAt: old,
		CompletedAt: &old,
	}
	_ = s.CreateJob(ctx, oldJob)

	// Recent completed job.
	recentTime := now.Add(-1 * time.Hour)
	recentJob := &Job{
		ID: "recent", ClusterID: "c", ClusterName: "c", SourceAgentID: "a",
		SourceNodeID: "n", OrgID: "default", Status: JobSucceeded,
		MaxParallel: 2, RetryMax: 3, CreatedAt: recentTime, UpdatedAt: recentTime,
		CompletedAt: &recentTime,
	}
	_ = s.CreateJob(ctx, recentJob)

	// Still running job (no completedAt).
	runningJob := &Job{
		ID: "running", ClusterID: "c", ClusterName: "c", SourceAgentID: "a",
		SourceNodeID: "n", OrgID: "default", Status: JobRunning,
		MaxParallel: 2, RetryMax: 3, CreatedAt: old, UpdatedAt: old,
	}
	_ = s.CreateJob(ctx, runningJob)

	pruned, err := s.PruneOldJobs(ctx, 24*time.Hour)
	if err != nil {
		t.Fatalf("PruneOldJobs: %v", err)
	}
	if pruned != 1 {
		t.Errorf("expected 1 pruned, got %d", pruned)
	}

	jobs, _ := s.ListJobs(ctx, "default", 10)
	if len(jobs) != 2 {
		t.Fatalf("expected 2 remaining jobs, got %d", len(jobs))
	}
}

func TestUpdateTargetArch(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)

	_ = s.CreateJob(ctx, &Job{
		ID: "j1", ClusterID: "c", ClusterName: "c", SourceAgentID: "a",
		SourceNodeID: "n", OrgID: "default", Status: JobQueued,
		MaxParallel: 2, RetryMax: 3, CreatedAt: now, UpdatedAt: now,
	})
	_ = s.CreateTarget(ctx, &Target{
		ID: "t1", JobID: "j1", NodeID: "n2", NodeName: "pve2",
		NodeIP: "10.0.0.2", Status: TargetPending,
		CreatedAt: now, UpdatedAt: now,
	})

	// Initially no arch.
	target, _ := s.GetTarget(ctx, "t1")
	if target.Arch != "" {
		t.Fatalf("expected empty arch initially, got %q", target.Arch)
	}

	// Set arch.
	if err := s.UpdateTargetArch(ctx, "t1", "arm64"); err != nil {
		t.Fatalf("UpdateTargetArch: %v", err)
	}

	target, _ = s.GetTarget(ctx, "t1")
	if target.Arch != "arm64" {
		t.Fatalf("expected arch arm64, got %q", target.Arch)
	}
}

func TestUpdateTargetArch_NotFound(t *testing.T) {
	s := testStore(t)
	err := s.UpdateTargetArch(context.Background(), "nonexistent", "amd64")
	if err == nil {
		t.Fatal("expected error for nonexistent target")
	}
}

func TestResetTargetsForRetry(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)

	_ = s.CreateJob(ctx, &Job{
		ID: "j1", ClusterID: "c", ClusterName: "c", SourceAgentID: "a",
		SourceNodeID: "n", OrgID: "default", Status: JobFailed,
		MaxParallel: 2, RetryMax: 3, CreatedAt: now, UpdatedAt: now,
	})

	// Create targets in various states.
	_ = s.CreateTarget(ctx, &Target{
		ID: "t1", JobID: "j1", NodeID: "n1", NodeName: "pve1",
		NodeIP: "10.0.0.1", Status: TargetFailedRetryable,
		ErrorMessage: "SSH failed", Attempts: 1,
		CreatedAt: now, UpdatedAt: now,
	})
	_ = s.CreateTarget(ctx, &Target{
		ID: "t2", JobID: "j1", NodeID: "n2", NodeName: "pve2",
		NodeIP: "10.0.0.2", Status: TargetFailedPermanent,
		ErrorMessage: "bad arch", Attempts: 2,
		CreatedAt: now, UpdatedAt: now,
	})
	_ = s.CreateTarget(ctx, &Target{
		ID: "t3", JobID: "j1", NodeID: "n3", NodeName: "pve3",
		NodeIP: "10.0.0.3", Status: TargetSucceeded,
		CreatedAt: now, UpdatedAt: now,
	})

	// Reset t1 and t2 (failed states). t3 (succeeded) should not be affected.
	count, err := s.ResetTargetsForRetry(ctx, []string{"t1", "t2", "t3"})
	if err != nil {
		t.Fatalf("ResetTargetsForRetry: %v", err)
	}
	if count != 2 {
		t.Fatalf("expected 2 reset, got %d", count)
	}

	// Verify t1 — attempts NOT incremented (that happens on next failure).
	target, _ := s.GetTarget(ctx, "t1")
	if target.Status != TargetPending {
		t.Fatalf("t1: expected pending, got %q", target.Status)
	}
	if target.ErrorMessage != "" {
		t.Fatalf("t1: expected empty error, got %q", target.ErrorMessage)
	}
	if target.Attempts != 1 { // was 1, NOT incremented by reset
		t.Fatalf("t1: expected 1 attempt (unchanged), got %d", target.Attempts)
	}

	// Verify t2 — attempts NOT incremented.
	target, _ = s.GetTarget(ctx, "t2")
	if target.Status != TargetPending {
		t.Fatalf("t2: expected pending, got %q", target.Status)
	}
	if target.Attempts != 2 { // was 2, NOT incremented by reset
		t.Fatalf("t2: expected 2 attempts (unchanged), got %d", target.Attempts)
	}

	// Verify t3 was NOT affected.
	target, _ = s.GetTarget(ctx, "t3")
	if target.Status != TargetSucceeded {
		t.Fatalf("t3: expected succeeded (unchanged), got %q", target.Status)
	}
}

func TestResetTargetsForRetry_EmptyList(t *testing.T) {
	s := testStore(t)
	count, err := s.ResetTargetsForRetry(context.Background(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected 0, got %d", count)
	}
}
