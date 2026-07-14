package main

import (
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/qualification"
)

func TestLiveRepeatCountUsesManifestProfile(t *testing.T) {
	manifest := qualification.Manifest{Repeat: qualification.RepeatSpec{Development: 3, Nightly: 5, Qualification: 30}}
	for profile, want := range map[string]int{"": 2, "development": 3, "nightly": 5, "qualification": 30} {
		got, err := liveRepeatCount(manifest, 2, profile)
		if err != nil {
			t.Fatalf("profile %q: %v", profile, err)
		}
		if got != want {
			t.Fatalf("profile %q count = %d, want %d", profile, got, want)
		}
	}
	if _, err := liveRepeatCount(manifest, 1, "best-of-three"); err == nil {
		t.Fatal("unknown repeat profile must fail")
	}
}

func TestSelectLiveManifestsSupportsOneScenarioOrWholeTrack(t *testing.T) {
	watchA := qualification.Manifest{ID: "watch.a", Track: qualification.TrackWatch}
	watchB := qualification.Manifest{ID: "watch.b", Track: qualification.TrackWatch}
	investigation := qualification.Manifest{ID: "investigation.a", Track: qualification.TrackInvestigation}
	catalog := qualification.Catalog{
		Manifests: []qualification.Manifest{watchA, watchB, investigation},
		ByID: map[string]qualification.Manifest{
			watchA.ID: watchA, watchB.ID: watchB, investigation.ID: investigation,
		},
	}

	selected, err := selectLiveManifests(catalog, "live", "watch.b", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(selected) != 1 || selected[0].ID != "watch.b" {
		t.Fatalf("single selection = %+v", selected)
	}

	selected, err = selectLiveManifests(catalog, "live-suite", "", qualification.TrackWatch)
	if err != nil {
		t.Fatal(err)
	}
	if len(selected) != 2 || selected[0].ID != "watch.a" || selected[1].ID != "watch.b" {
		t.Fatalf("suite selection = %+v", selected)
	}
	if _, err := selectLiveManifests(catalog, "live-suite", "", ""); err == nil {
		t.Fatal("suite without a track must fail")
	}
	if _, err := selectLiveManifests(catalog, "live", "missing", ""); err == nil {
		t.Fatal("unknown single scenario must fail")
	}
}
