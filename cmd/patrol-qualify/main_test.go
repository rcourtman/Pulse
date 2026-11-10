package main

import (
	"strings"
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

func TestSelectFaultSetOrdersByLabelAndNarrowsToNamedFaults(t *testing.T) {
	catalog, err := qualification.LoadCatalog("../../tests/qualification/patrol/scenarios")
	if err != nil {
		t.Fatal(err)
	}
	labels := func(manifests []qualification.Manifest) string {
		var out []string
		for _, manifest := range manifests {
			out = append(out, manifest.Metadata["fault_label"])
		}
		return strings.Join(out, " ")
	}
	all, err := selectFaultSet(catalog, "v1", "")
	if err != nil || labels(all) != "D1 D2 D3 D4 D5 D6" {
		t.Fatalf("fault set v1 = %q, %v", labels(all), err)
	}
	some, err := selectFaultSet(catalog, "v1", " d4, D1 ")
	if err != nil || labels(some) != "D4 D1" {
		t.Fatalf("named faults = %q, %v", labels(some), err)
	}
	if _, err := selectFaultSet(catalog, "v1", "D9"); err == nil {
		t.Fatal("an unknown fault label must fail")
	}
	if _, err := selectFaultSet(catalog, "v2-proxmox", ""); err == nil {
		t.Fatal("a fault set with no scenarios must fail")
	}
	// The host disk fault is the only one that needs a mount handed over,
	// so the rest of the set runs without one.
	if missing := qualification.MissingHostFilesystems(all, nil); len(missing) != 1 || missing[0] != "/mnt/nvr" {
		t.Fatalf("missing host filesystems = %v", missing)
	}
	if missing := qualification.MissingHostFilesystems(some, nil); len(missing) != 0 {
		t.Fatalf("D1 and D4 need no host filesystem, got %v", missing)
	}
	if missing := qualification.MissingHostFilesystems(all, []string{"/mnt/nvr"}); len(missing) != 0 {
		t.Fatalf("a handed-over mount is still reported missing: %v", missing)
	}
}

// A response scenario is graded, never gated, so it must stay out of the
// launch suites and their statistics.
func TestLiveSuiteDoesNotSelectResponseScenarios(t *testing.T) {
	catalog, err := qualification.LoadCatalog("../../tests/qualification/patrol/scenarios")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := selectLiveManifests(catalog, "live-suite", "", qualification.TrackResponse); err == nil {
		t.Fatal("live-suite accepted the response track")
	}
	for _, track := range []qualification.Track{qualification.TrackWatch, qualification.TrackInvestigation, qualification.TrackRemediation} {
		selected, err := selectLiveManifests(catalog, "live-suite", "", track)
		if err != nil {
			t.Fatal(err)
		}
		for _, manifest := range selected {
			if strings.HasPrefix(manifest.ID, "response.") {
				t.Fatalf("%s suite selected %s", track, manifest.ID)
			}
		}
	}
}
