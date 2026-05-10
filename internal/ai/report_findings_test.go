package ai

import (
	"testing"
	"time"
)

func TestFindingOverlapsWindow_DetectedInsideWindow(t *testing.T) {
	start := time.Now().Add(-1 * time.Hour)
	end := time.Now()
	f := &Finding{DetectedAt: start.Add(15 * time.Minute)}
	if !findingOverlapsWindow(f, start, end) {
		t.Fatal("finding detected inside window should overlap")
	}
}

func TestFindingOverlapsWindow_DetectedAfterWindow(t *testing.T) {
	start := time.Now().Add(-2 * time.Hour)
	end := time.Now().Add(-1 * time.Hour)
	f := &Finding{DetectedAt: time.Now()}
	if findingOverlapsWindow(f, start, end) {
		t.Fatal("finding detected after window should not overlap")
	}
}

func TestFindingOverlapsWindow_DetectedBeforeButResolvedInside(t *testing.T) {
	start := time.Now().Add(-1 * time.Hour)
	end := time.Now()
	resolved := start.Add(10 * time.Minute)
	f := &Finding{
		DetectedAt: start.Add(-2 * time.Hour),
		ResolvedAt: &resolved,
	}
	if !findingOverlapsWindow(f, start, end) {
		t.Fatal("finding resolved inside window should overlap")
	}
}

func TestFindingOverlapsWindow_DetectedAndResolvedBefore(t *testing.T) {
	start := time.Now().Add(-1 * time.Hour)
	end := time.Now()
	resolved := start.Add(-30 * time.Minute)
	f := &Finding{
		DetectedAt: start.Add(-2 * time.Hour),
		ResolvedAt: &resolved,
	}
	if findingOverlapsWindow(f, start, end) {
		t.Fatal("finding fully before window should not overlap")
	}
}

func TestFindingOverlapsWindow_NilFinding(t *testing.T) {
	if findingOverlapsWindow(nil, time.Now(), time.Now().Add(time.Hour)) {
		t.Fatal("nil finding should not overlap")
	}
}

func TestFindingOverlapsWindow_StillActive(t *testing.T) {
	start := time.Now().Add(-1 * time.Hour)
	end := time.Now()
	// Detected before window, never resolved -> still active during window
	f := &Finding{DetectedAt: start.Add(-2 * time.Hour)}
	if !findingOverlapsWindow(f, start, end) {
		t.Fatal("active finding should overlap any later window")
	}
}
