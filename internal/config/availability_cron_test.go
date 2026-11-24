package config

import (
	"testing"
	"time"
)

func TestParseAvailabilityCron_ValidAndMatching(t *testing.T) {
	sched, err := ParseAvailabilityCron("0 3 * * *")
	if err != nil {
		t.Fatalf("ParseAvailabilityCron() unexpected error: %v", err)
	}
	if !sched.Matches(time.Date(2026, 11, 24, 3, 0, 0, 0, time.UTC)) {
		t.Fatalf("expected 03:00 UTC to match")
	}
	if sched.Matches(time.Date(2026, 11, 24, 3, 1, 0, 0, time.UTC)) {
		t.Fatalf("expected 03:01 UTC not to match")
	}

	prev, ok := sched.Prev(time.Date(2026, 11, 24, 3, 5, 30, 0, time.UTC))
	if !ok {
		t.Fatalf("expected Prev() to succeed")
	}
	wantPrev := time.Date(2026, 11, 24, 3, 0, 0, 0, time.UTC)
	if !prev.Equal(wantPrev) {
		t.Fatalf("Prev() = %s, want %s", prev, wantPrev)
	}

	next, ok := sched.Next(time.Date(2026, 11, 24, 3, 0, 0, 0, time.UTC))
	if !ok {
		t.Fatalf("expected Next() to succeed")
	}
	wantNext := time.Date(2026, 11, 25, 3, 0, 0, 0, time.UTC)
	if !next.Equal(wantNext) {
		t.Fatalf("Next() = %s, want %s", next, wantNext)
	}
}

func TestParseAvailabilityCron_StepsListsAndSundaySeven(t *testing.T) {
	sched, err := ParseAvailabilityCron("*/15 9-17 * * 7")
	if err != nil {
		t.Fatalf("ParseAvailabilityCron() unexpected error: %v", err)
	}
	// 2026-11-29 is a Sunday.
	sun := time.Date(2026, 11, 29, 9, 30, 0, 0, time.UTC)
	if !sched.Matches(sun) {
		t.Fatalf("expected Sunday 09:30 UTC to match dow=7")
	}
	mon := time.Date(2026, 11, 30, 9, 30, 0, 0, time.UTC)
	if sched.Matches(mon) {
		t.Fatalf("expected Monday not to match dow=7")
	}

	prev, ok := sched.Prev(time.Date(2026, 11, 30, 12, 0, 0, 0, time.UTC))
	if !ok {
		t.Fatalf("expected Prev() to find prior Sunday")
	}
	want := time.Date(2026, 11, 29, 17, 45, 0, 0, time.UTC)
	if !prev.Equal(want) {
		t.Fatalf("Prev() = %s, want %s", prev, want)
	}
}

func TestParseAvailabilityCron_RejectsInvalidExpressions(t *testing.T) {
	invalid := []string{
		"",
		"* * * *",
		"* * * * * *",
		"60 * * * *",
		"* 24 * * *",
		"* * 0 * *",
		"* * * 13 *",
		"* * * * 8",
		"*/0 * * * *",
		"10-5 * * * *",
		"0 0 31 2 *",
		"abc * * * *",
	}
	for _, expr := range invalid {
		if _, err := ParseAvailabilityCron(expr); err == nil {
			t.Errorf("ParseAvailabilityCron(%q) unexpectedly succeeded", expr)
		}
	}
}
