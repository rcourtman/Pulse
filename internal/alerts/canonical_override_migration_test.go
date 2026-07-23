package alerts

import (
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

func TestMigrateCanonicalOverrideKeysRehomesUnambiguousSupersededIdentity(t *testing.T) {
	const (
		oldID = "agent-535886018cb53055"
		newID = "agent-b9ed6d0e20e94eaf"
	)
	config := AlertConfig{
		Overrides: map[string]ThresholdConfig{
			oldID: {
				Memory: &HysteresisThreshold{Trigger: 95, Clear: 90},
			},
		},
	}
	resources := []unifiedresources.Resource{{
		ID:                     newID,
		Type:                   unifiedresources.ResourceTypeAgent,
		SupersededCanonicalIDs: []string{oldID},
	}}

	if !MigrateCanonicalOverrideKeys(&config, resources) {
		t.Fatal("expected superseded TrueNAS override identity to migrate")
	}
	if _, exists := config.Overrides[oldID]; exists {
		t.Fatalf("override remained under superseded identity %s", oldID)
	}
	override, exists := config.Overrides[newID]
	if !exists || override.Memory == nil {
		t.Fatalf("override missing under current canonical identity %s: %+v", newID, config.Overrides)
	}
	if override.Memory.Trigger != 95 || override.Memory.Clear != 90 {
		t.Fatalf("migrated override changed threshold values: %+v", override.Memory)
	}
}

func TestMigrateCanonicalOverrideKeysRefusesAmbiguousOrLiveIdentity(t *testing.T) {
	const oldID = "agent-shared"
	for name, resources := range map[string][]unifiedresources.Resource{
		"ambiguous successor": {
			{ID: "agent-a", SupersededCanonicalIDs: []string{oldID}},
			{ID: "agent-b", SupersededCanonicalIDs: []string{oldID}},
		},
		"still live": {
			{ID: oldID},
			{ID: "agent-a", SupersededCanonicalIDs: []string{oldID}},
		},
	} {
		t.Run(name, func(t *testing.T) {
			config := AlertConfig{
				Overrides: map[string]ThresholdConfig{
					oldID: {Memory: &HysteresisThreshold{Trigger: 95, Clear: 90}},
				},
			}
			if MigrateCanonicalOverrideKeys(&config, resources) {
				t.Fatal("unsafe succession unexpectedly migrated")
			}
			if _, exists := config.Overrides[oldID]; !exists {
				t.Fatal("unsafe succession removed the original override")
			}
		})
	}
}

func TestMigrateCanonicalOverrideKeysCurrentIdentityWinsAndCleansRetiredOrphan(t *testing.T) {
	const (
		oldID = "agent-535886018cb53055"
		newID = "agent-b9ed6d0e20e94eaf"
	)
	config := AlertConfig{
		Overrides: map[string]ThresholdConfig{
			oldID: {
				Memory: &HysteresisThreshold{Trigger: 90, Clear: 85},
			},
			newID: {
				Memory: &HysteresisThreshold{Trigger: 95, Clear: 90},
			},
		},
	}
	resources := []unifiedresources.Resource{{
		ID:                     newID,
		SupersededCanonicalIDs: []string{oldID},
	}}

	if !MigrateCanonicalOverrideKeys(&config, resources) {
		t.Fatal("expected the retired duplicate override to be cleaned")
	}
	if _, exists := config.Overrides[oldID]; exists {
		t.Fatalf("retired duplicate override %s was not removed", oldID)
	}
	override := config.Overrides[newID]
	if override.Memory == nil || override.Memory.Trigger != 95 || override.Memory.Clear != 90 {
		t.Fatalf("current canonical override did not win: %+v", override.Memory)
	}
	if MigrateCanonicalOverrideKeys(&config, resources) {
		t.Fatal("idempotent migration reported a second change")
	}
}

func TestMigrateCanonicalOverrideKeysRetainsUnknownOrphanUntilSuccessionIsProven(t *testing.T) {
	const orphanID = "agent-not-currently-polled"
	config := AlertConfig{
		Overrides: map[string]ThresholdConfig{
			orphanID: {
				Memory: &HysteresisThreshold{Trigger: 95, Clear: 90},
			},
		},
	}

	if MigrateCanonicalOverrideKeys(&config, []unifiedresources.Resource{{
		ID: "agent-other-system",
	}}) {
		t.Fatal("unknown override was deleted without a provider-declared succession")
	}
	if _, exists := config.Overrides[orphanID]; !exists {
		t.Fatal("transiently absent TrueNAS override was not retained for a later repoll")
	}
}
