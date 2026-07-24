package alerts

import (
	"testing"
	"time"
)

// This file is a purpose-built branch-coverage test set (selected via
// `-run "^TestBranchcov0724pm"`) for one pure helper in package alerts that
// previously had 0.0% coverage:
//
//   - intentTimePointer(value time.Time) *time.Time — intent_policy.go:298
//
// The function takes time.Time by value and returns a pointer to a copy.
// Every observable behaviour is exercised: the zero-time case, a real
// timestamp carried through exactly, and two-direction independence between
// the caller's variable and the returned pointer (the value parameter is a
// copy, so neither side can affect the other after the call returns).
//
// Conventions match sibling in-package tests in this directory (see
// intent_policy_test.go): stdlib `testing` only, t.Fatalf assertions,
// no testify.

func TestBranchcov0724pmIntentTimePointer(t *testing.T) {
	t.Run("zero time yields non-nil pointer to zero time", func(t *testing.T) {
		ptr := intentTimePointer(time.Time{})
		if ptr == nil {
			t.Fatal("intentTimePointer(time.Time{}) returned nil pointer")
		}
		if !ptr.IsZero() {
			t.Fatalf("dereferenced pointer = %v, want zero time", *ptr)
		}
	})

	t.Run("real time is carried through exactly", func(t *testing.T) {
		ts := time.Date(2026, 7, 24, 13, 45, 0, 0, time.UTC)
		ptr := intentTimePointer(ts)
		if ptr == nil {
			t.Fatal("intentTimePointer returned nil pointer")
		}
		if !ptr.Equal(ts) {
			t.Fatalf("dereferenced pointer = %v, want %v", *ptr, ts)
		}
	})

	t.Run("returned pointer is independent of caller variable both directions", func(t *testing.T) {
		// intentTimePointer receives value by copy (&value points at the
		// local copy), so mutating the returned pointer must not affect the
		// caller's original variable...
		original := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
		ptr := intentTimePointer(original)

		*ptr = original.Add(48 * time.Hour)
		if !original.Equal(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)) {
			t.Fatalf("mutating returned pointer changed caller variable: original = %v", original)
		}

		// ...and reassigning the caller variable after the call must not
		// retroactively change the already-returned pointer's value.
		original = original.Add(30 * 24 * time.Hour)
		if !ptr.Equal(time.Date(2026, 1, 3, 0, 0, 0, 0, time.UTC)) {
			t.Fatalf("reassigning caller variable changed returned pointer: ptr = %v, want 2026-01-03", *ptr)
		}
	})
}
