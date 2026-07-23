package models

import (
	"encoding/json"
	"math"
	"testing"
)

func TestUnavailableMemorySurvivesJSONReload(t *testing.T) {
	before := UnavailableMemory(8 << 30)

	payload, err := json.Marshal(before)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	var after Memory
	if err := json.Unmarshal(payload, &after); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if !after.UsageUnavailable {
		t.Fatalf("UsageUnavailable = false after reload; payload=%s", payload)
	}
	if after.Total != before.Total {
		t.Fatalf("Total = %d, want %d", after.Total, before.Total)
	}
	if after.HasKnownUsage() {
		t.Fatalf("HasKnownUsage() = true after unavailable reload: %+v", after)
	}
}

func TestMemoryHasKnownUsageAcceptsTrustedZeroUsage(t *testing.T) {
	memory := Memory{Total: 8 << 30, Free: 8 << 30}
	if !memory.HasKnownUsage() {
		t.Fatalf("HasKnownUsage() = false for a valid zero-usage sample: %+v", memory)
	}
}

func TestMemoryHasKnownUsageRejectsNonFiniteAndContradictoryValues(t *testing.T) {
	for name, memory := range map[string]Memory{
		"nan":          {Total: 8 << 30, Used: 4 << 30, Free: 4 << 30, Usage: math.NaN()},
		"infinity":     {Total: 8 << 30, Used: 4 << 30, Free: 4 << 30, Usage: math.Inf(1)},
		"free overflow": {Total: 8 << 30, Free: 9 << 30},
	} {
		t.Run(name, func(t *testing.T) {
			if memory.HasKnownUsage() {
				t.Fatalf("HasKnownUsage() = true for invalid memory: %+v", memory)
			}
		})
	}
}
