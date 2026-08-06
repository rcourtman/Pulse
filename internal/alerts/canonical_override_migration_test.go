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

func TestMigrateDockerContainerOverrideKeys(t *testing.T) {
	fullID := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	appContainer := func(id, hostID, containerID, name string) unifiedresources.Resource {
		return unifiedresources.Resource{
			ID:   id,
			Type: unifiedresources.ResourceTypeAppContainer,
			Name: name,
			Docker: &unifiedresources.DockerData{
				HostSourceID: hostID,
				ContainerID:  containerID,
			},
		}
	}

	config := AlertConfig{Overrides: map[string]ThresholdConfig{
		// Live legacy key: re-homed onto the name key.
		"docker:host-1/aaaaaaaaaaaa": {Disabled: true},
		// Live legacy key whose name key already exists: dropped, name key wins.
		"docker:host-1/" + fullID: {Disabled: false},
		"docker:host-1/db":        {Disabled: true},
		// Orphaned container-ID keys: pruned (12 and 64 char hex).
		"docker:host-1/dddddddddddd": {Disabled: true},
		// Unified hash key the v6 UI wrote: re-homed via the resource snapshot.
		"docker:host-1/app-container-0011223344556677": {Disabled: true},
		// Orphaned unified hash key: pruned.
		"docker:host-1/app-container-8899aabbccddeeff": {Disabled: true},
		// Name-keyed override for an absent container: kept.
		"docker:host-1/retired-cron": {Disabled: true},
		// Swarm service key: untouched.
		"docker:host-1/service/web": {Disabled: true},
		// A host not in the snapshot: untouched.
		"docker:host-9/eeeeeeeeeeee": {Disabled: true},
		// Docker host override (no slash): untouched.
		"host-1": {DisableConnectivity: true},
	}}

	resources := []unifiedresources.Resource{
		appContainer("app-container-1111111111111111", "host-1", "aaaaaaaaaaaa", "media-server"),
		appContainer("app-container-2222222222222222", "host-1", fullID, "db"),
		appContainer("app-container-0011223344556677", "host-1", "ffffffffffff", "proxy"),
	}

	original := config.Overrides
	if !MigrateDockerContainerOverrideKeys(&config, resources) {
		t.Fatalf("expected migration to report changes")
	}

	if _, exists := original["docker:host-1/aaaaaaaaaaaa"]; !exists {
		t.Fatalf("migration must be copy-on-write; the input map was mutated in place")
	}

	got := config.Overrides
	if override, exists := got["docker:host-1/media-server"]; !exists || !override.Disabled {
		t.Fatalf("expected live legacy key to move onto the name key, got %#v", got)
	}
	if override, exists := got["docker:host-1/proxy"]; !exists || !override.Disabled {
		t.Fatalf("expected live unified hash key to move onto the name key, got %#v", got)
	}
	for _, gone := range []string{
		"docker:host-1/aaaaaaaaaaaa",
		"docker:host-1/" + fullID,
		"docker:host-1/dddddddddddd",
		"docker:host-1/app-container-0011223344556677",
		"docker:host-1/app-container-8899aabbccddeeff",
	} {
		if _, exists := got[gone]; exists {
			t.Fatalf("expected %s to be removed", gone)
		}
	}
	if override := got["docker:host-1/db"]; !override.Disabled {
		t.Fatalf("expected existing name key to win over the legacy key")
	}
	for _, kept := range []string{
		"docker:host-1/retired-cron",
		"docker:host-1/service/web",
		"docker:host-9/eeeeeeeeeeee",
		"host-1",
	} {
		if _, exists := got[kept]; !exists {
			t.Fatalf("expected %s to be kept", kept)
		}
	}
}

func TestMigrateDockerContainerOverrideKeysNoChanges(t *testing.T) {
	resources := []unifiedresources.Resource{{
		ID:   "app-container-1111111111111111",
		Type: unifiedresources.ResourceTypeAppContainer,
		Name: "media-server",
		Docker: &unifiedresources.DockerData{
			HostSourceID: "host-1",
			ContainerID:  "aaaaaaaaaaaa",
		},
	}}

	config := AlertConfig{Overrides: map[string]ThresholdConfig{
		"docker:host-1/media-server": {Disabled: true},
	}}
	if MigrateDockerContainerOverrideKeys(&config, resources) {
		t.Fatalf("expected no changes when overrides already use stable keys")
	}

	// A host with no docker-sourced app-containers in the snapshot may be a
	// transient collection failure; nothing under it may be pruned.
	config = AlertConfig{Overrides: map[string]ThresholdConfig{
		"docker:host-9/aaaaaaaaaaaa": {Disabled: true},
	}}
	if MigrateDockerContainerOverrideKeys(&config, resources) {
		t.Fatalf("expected hosts absent from the snapshot to be untouched")
	}
}

func TestLooksLikeDockerContainerID(t *testing.T) {
	cases := map[string]bool{
		"aaaaaaaaaaaa": true,
		"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef": true,
		"ABCDEF123456": true,
		"media-server": false,
		"aaaaaaaaaaa":  false, // 11 chars
		"gggggggggggg": false, // non-hex
		"":             false,
	}
	for value, want := range cases {
		if got := looksLikeDockerContainerID(value); got != want {
			t.Errorf("looksLikeDockerContainerID(%q) = %v, want %v", value, got, want)
		}
	}
}
