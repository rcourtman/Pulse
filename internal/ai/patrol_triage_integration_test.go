package ai

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

func TestRunAIAnalysis_TriageQuietSkipsLLM(t *testing.T) {
	ps := NewPatrolService(&Service{}, nil)
	state := models.StateSnapshot{}

	triage := ps.RunDeterministicTriage(context.Background(), state, nil, nil)
	if triage == nil {
		t.Fatal("expected triage result")
	}
	if !triage.IsQuiet {
		t.Fatalf("expected quiet triage state, got IsQuiet=%v", triage.IsQuiet)
	}

	res, err := ps.runAIAnalysis(context.Background(), state, nil)
	if err != nil {
		t.Fatalf("runAIAnalysis() unexpected error for quiet triage: %v", err)
	}
	if res == nil {
		t.Fatal("expected analysis result")
	}
	if !strings.Contains(res.Response, "Infrastructure healthy") {
		t.Fatalf("expected quiet short-circuit response, got: %q", res.Response)
	}
}

func TestBuildTriageSeedContext_FlaggedOnly(t *testing.T) {
	ps := NewPatrolService(nil, nil)
	state := triageIntegrationState(10)
	flaggedIDs := map[string]bool{
		"qemu/102": true,
		"qemu/107": true,
	}
	triage := triageIntegrationResult(state, flaggedIDs)

	seed, _ := ps.buildTriageSeedContext(triage, state, nil, nil)
	if !strings.Contains(seed, "# Deterministic Triage Results") {
		t.Fatalf("expected triage briefing header, got:\n%s", seed)
	}

	for _, expected := range []string{"vm-02", "vm-07"} {
		if !strings.Contains(seed, expected) {
			t.Fatalf("expected flagged resource %q in triage seed, got:\n%s", expected, seed)
		}
	}
	for _, unexpected := range []string{"vm-00", "vm-03", "vm-09"} {
		if strings.Contains(seed, unexpected) {
			t.Fatalf("did not expect unflagged resource %q in triage seed, got:\n%s", unexpected, seed)
		}
	}
}

func TestBuildTriageSeedContext_SmallOutput(t *testing.T) {
	ps := NewPatrolService(nil, nil)
	state := triageIntegrationState(40)
	flaggedIDs := map[string]bool{
		"qemu/102": true,
		"qemu/107": true,
	}
	triage := triageIntegrationResult(state, flaggedIDs)

	triageSeed, _ := ps.buildTriageSeedContext(triage, state, nil, nil)
	fullSeed, _ := ps.buildSeedContext(state, nil, nil)

	if len(fullSeed) == 0 {
		t.Fatal("expected non-empty full seed context")
	}
	if len(triageSeed) == 0 {
		t.Fatal("expected non-empty triage seed context")
	}
	if len(triageSeed) >= len(fullSeed) {
		t.Fatalf("expected triage seed smaller than full seed, got triage=%d full=%d", len(triageSeed), len(fullSeed))
	}
	if len(triageSeed)*2 > len(fullSeed) {
		t.Fatalf("expected triage seed to be significantly smaller, got triage=%d full=%d", len(triageSeed), len(fullSeed))
	}
}

func TestComputeTriageMaxTurns(t *testing.T) {
	if got := computeTriageMaxTurns(0, nil); got != 8 {
		t.Fatalf("0 flags: expected 8 turns, got %d", got)
	}
	if got := computeTriageMaxTurns(1, nil); got != 8 {
		t.Fatalf("1 flag: expected 8 turns, got %d", got)
	}
	if got := computeTriageMaxTurns(3, nil); got != 14 {
		t.Fatalf("3 flags: expected 14 turns, got %d", got)
	}
	if got := computeTriageMaxTurns(10, nil); got != 35 {
		t.Fatalf("10 flags: expected 35 turns, got %d", got)
	}
	if got := computeTriageMaxTurns(15, nil); got != 40 {
		t.Fatalf("15 flags: expected 40 turns (cap), got %d", got)
	}

	quickScope := &PatrolScope{Depth: PatrolDepthQuick}
	if got := computeTriageMaxTurns(15, quickScope); got != 20 {
		t.Fatalf("quick scope expected 20-turn cap, got %d", got)
	}
}

func TestGetPatrolSystemPromptForTriage(t *testing.T) {
	ps := NewPatrolService(&Service{
		cfg: &config.AIConfig{PatrolAutoFix: false},
	}, nil)

	prompt := ps.getPatrolSystemPromptForTriage()
	if !strings.Contains(prompt, "Deterministic triage has already scanned all resources") {
		t.Fatalf("expected triage preamble in prompt, got:\n%s", prompt)
	}
	if !strings.Contains(prompt, "## Investigation Tools") || !strings.Contains(prompt, "pulse_query") {
		t.Fatalf("expected tool descriptions from base prompt, got:\n%s", prompt)
	}
	if strings.Contains(prompt, "Your job is to find issues that simple threshold-based alerts CANNOT catch") {
		t.Fatalf("expected standard opening to be replaced in triage prompt, got:\n%s", prompt)
	}
}

func triageIntegrationState(vmCount int) models.StateSnapshot {
	state := models.StateSnapshot{
		Nodes: []models.Node{
			{
				ID:     "node/pve1",
				Name:   "pve1",
				Status: "online",
				CPU:    0.15,
				Memory: models.Memory{Usage: 20.0},
				Disk:   models.Disk{Usage: 30.0},
			},
		},
		VMs: make([]models.VM, 0, vmCount),
	}

	for i := 0; i < vmCount; i++ {
		state.VMs = append(state.VMs, models.VM{
			ID:       fmt.Sprintf("qemu/%d", 100+i),
			Name:     fmt.Sprintf("vm-%02d", i),
			Node:     "pve1",
			Status:   "running",
			Template: false,
			CPU:      0.10,
			Memory:   models.Memory{Usage: 42.0},
			Disk:     models.Disk{Usage: 36.0},
		})
	}

	return state
}

func triageIntegrationResult(state models.StateSnapshot, flaggedIDs map[string]bool) *TriageResult {
	flags := make([]TriageFlag, 0, len(flaggedIDs))
	for _, vm := range state.VMs {
		if !flaggedIDs[vm.ID] {
			continue
		}
		flags = append(flags, TriageFlag{
			ResourceID:   vm.ID,
			ResourceName: vm.Name,
			ResourceType: "vm",
			Category:     "performance",
			Severity:     "warning",
			Reason:       "Memory at 92% (threshold: 88%)",
			Metric:       "memory",
			Value:        92,
			Threshold:    88,
		})
	}

	return &TriageResult{
		Flags: flags,
		Summary: TriageSummary{
			TotalNodes:    len(state.Nodes),
			TotalGuests:   len(state.VMs),
			RunningGuests: len(state.VMs),
			FlaggedCount:  len(flags),
		},
		FlaggedIDs: flaggedIDs,
	}
}
