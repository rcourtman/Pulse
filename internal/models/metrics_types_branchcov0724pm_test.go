package models

import (
	"reflect"
	"testing"
)

// This file is a purpose-built branch-coverage test set (selected via
// `-run "^TestBranchcov0724pm"`) for one pure helper in package models that
// previously had 0.0% coverage:
//
//   - (p IOCounterPresence) Effective() IOCounterPresence — metrics_types.go:51
//
// Every arm of the function is exercised directly here. Conventions match
// sibling in-package tests in this directory (see metrics_types_test.go):
// stdlib `testing` only, table-driven subtests, reflect.DeepEqual /
// t.Fatalf assertions, no testify.

func TestBranchcov0724pmIOCounterPresenceEffective(t *testing.T) {
	// Arm: zero value (Explicit=false) takes the non-explicit path and returns
	// the canonical all-true presence contract regardless of any stray
	// sub-field values.
	t.Run("zero value returns all-true presence", func(t *testing.T) {
		var p IOCounterPresence
		got := p.Effective()
		want := IOCounterPresence{
			Explicit:   true,
			DiskRead:   true,
			DiskWrite:  true,
			DiskBusy:   true,
			NetworkIn:  true,
			NetworkOut: true,
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("zero-value Effective() = %+v, want %+v", got, want)
		}
	})

	// Arm: Explicit=false with stray sub-fields set — the non-explicit path
	// ignores sub-field values entirely and always returns all-true.
	t.Run("non-explicit with stray sub-fields still returns all-true", func(t *testing.T) {
		p := IOCounterPresence{DiskRead: true}
		got := p.Effective()
		if !got.DiskBusy || !got.NetworkOut || !got.Explicit || !got.DiskWrite {
			t.Fatalf("non-explicit Effective() = %+v, want every field true", got)
		}
	})

	// Arm: Explicit=true returns the struct verbatim, preserving all-false.
	t.Run("explicit with all counters false is preserved", func(t *testing.T) {
		p := IOCounterPresence{Explicit: true}
		got := p.Effective()
		if !reflect.DeepEqual(got, p) {
			t.Fatalf("explicit all-false Effective() = %+v, want %+v", got, p)
		}
		if got.DiskRead || got.DiskWrite || got.DiskBusy || got.NetworkIn || got.NetworkOut {
			t.Fatalf("explicit all-false should keep counters false: %+v", got)
		}
	})

	// Arm: Explicit=true with a selective mix is preserved exactly (not
	// overwritten with all-true).
	t.Run("explicit selective presence is preserved exactly", func(t *testing.T) {
		p := IOCounterPresence{
			Explicit:   true,
			DiskRead:   true,
			DiskWrite:  false,
			DiskBusy:   true,
			NetworkIn:  false,
			NetworkOut: true,
		}
		got := p.Effective()
		if !reflect.DeepEqual(got, p) {
			t.Fatalf("selective Effective() = %+v, want %+v", got, p)
		}
	})

	// Arm: Explicit=true with all sub-fields true — same shape as the
	// non-explicit result but reached via the explicit branch.
	t.Run("explicit all-true is preserved", func(t *testing.T) {
		explicit := IOCounterPresence{
			Explicit:   true,
			DiskRead:   true,
			DiskWrite:  true,
			DiskBusy:   true,
			NetworkIn:  true,
			NetworkOut: true,
		}
		if got := explicit.Effective(); !reflect.DeepEqual(got, explicit) {
			t.Fatalf("explicit all-true Effective() = %+v, want %+v", got, explicit)
		}
	})
}
