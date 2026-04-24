package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/approval"
)

func TestRunCreateAndGetPersistsProductShapedApproval(t *testing.T) {
	dataDir := t.TempDir()

	createArgs := cliArgs{
		action:     "create",
		approvalID: "approval-1",
		command:    "systemctl restart pulse-relay",
		context:    "Approve the seeded relay restart fix.",
		dataDir:    dataDir,
		orgID:      "org-1",
		risk:       "high",
		targetID:   "relay_123",
		targetName: "Pulse Relay",
		targetType: defaultTargetType,
		timeout:    15 * time.Minute,
		toolID:     "pulse_control",
	}
	if err := run(createArgs); err != nil {
		t.Fatalf("run create: %v", err)
	}

	approvalsPath := filepath.Join(dataDir, "ai_approvals.json")
	if _, err := os.Stat(approvalsPath); err != nil {
		t.Fatalf("approval file was not persisted: %v", err)
	}

	store, err := approval.NewStore(approval.StoreConfig{DataDir: dataDir})
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	got, ok := store.GetApproval("approval-1")
	if !ok {
		t.Fatal("approval was not loaded")
	}
	if got.OrgID != "org-1" {
		t.Fatalf("OrgID = %q, want org-1", got.OrgID)
	}
	if got.TargetType != defaultTargetType {
		t.Fatalf("TargetType = %q, want %q", got.TargetType, defaultTargetType)
	}
	if got.CommandHash == "" {
		t.Fatal("CommandHash was not populated")
	}
	if got.Plan == nil || !got.Plan.RequiresApproval || got.Plan.PlanHash == "" {
		t.Fatalf("approval plan was not populated: %+v", got.Plan)
	}
	if got.ContextConfidence == nil || got.ContextConfidence.Level != approval.ContextConfidenceVerified {
		t.Fatalf("context confidence = %+v, want verified", got.ContextConfidence)
	}
	if got.Preflight == nil || got.Preflight.Target != "agent:relay_123" {
		t.Fatalf("preflight = %+v, want agent:relay_123 target", got.Preflight)
	}

	getArgs := createArgs
	getArgs.action = "get"
	getArgs.command = ""
	if err := run(getArgs); err != nil {
		t.Fatalf("run get: %v", err)
	}
}

func TestParseArgsRequiresCreateFields(t *testing.T) {
	defer func() {
		if recovered := recover(); recovered == nil {
			t.Fatal("expected usage to exit")
		}
	}()
	oldExit := osExit
	osExit = func(code int) {
		panic(code)
	}
	defer func() { osExit = oldExit }()

	parseArgs([]string{"create", "--data-dir", t.TempDir(), "--approval-id", "approval-1"})
}
