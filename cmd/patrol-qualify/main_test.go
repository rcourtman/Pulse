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
