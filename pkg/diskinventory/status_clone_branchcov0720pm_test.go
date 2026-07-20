package diskinventory

import (
	"reflect"
	"testing"
)

// TestCloneStatusBranchcov0720pm exercises CloneStatus for nil-safety,
// deep value equality on fully-populated and zero-value inputs, and
// bidirectional deep-copy independence (mutating the clone must not affect
// the original and vice versa).
func TestCloneStatusBranchcov0720pm(t *testing.T) {
	t.Run("nil input returns nil", func(t *testing.T) {
		got := CloneStatus(nil)
		if got != nil {
			t.Fatalf("CloneStatus(nil) = %v, want nil", got)
		}
	})

	t.Run("zero value clones equal", func(t *testing.T) {
		orig := &CollectionStatus{}
		clone := CloneStatus(orig)
		if clone == nil {
			t.Fatalf("CloneStatus returned nil for non-nil zero-value input")
		}
		if clone == orig {
			t.Fatalf("CloneStatus returned identical pointer; want distinct value-equal instance")
		}
		if !reflect.DeepEqual(*clone, *orig) {
			t.Fatalf("zero-value clone not equal to original: clone=%+v orig=%+v", *clone, *orig)
		}
	})

	t.Run("fully populated clones deeply equal", func(t *testing.T) {
		orig := fullyPopulatedStatus()
		clone := CloneStatus(orig)
		if clone == orig {
			t.Fatalf("CloneStatus returned identical pointer; want distinct value-equal instance")
		}
		if !reflect.DeepEqual(*clone, *orig) {
			t.Fatalf("clone not deeply equal to original: clone=%+v orig=%+v", *clone, *orig)
		}
	})

	t.Run("clone mutations leave original unchanged", func(t *testing.T) {
		orig := fullyPopulatedStatus()
		snapshot := *orig
		clone := CloneStatus(orig)

		// Mutate every field on the clone. CollectionStatus contains only
		// value-typed FieldStatus fields (no slices/maps/pointers), so this
		// exercises each value field's independence directly.
		clone.Serial = Unavailable("clone-mut", "serial")
		clone.Temperature = Available("clone-mut")
		clone.IO = Unsupported("clone-mut", "io")
		clone.Controller = Missing("clone-mut", "controller")
		clone.Pool = Unavailable("clone-mut", "pool")

		if !reflect.DeepEqual(*orig, snapshot) {
			t.Fatalf("original changed after mutating clone: orig=%+v snapshot=%+v", *orig, snapshot)
		}
	})

	t.Run("original mutations leave clone unchanged", func(t *testing.T) {
		orig := fullyPopulatedStatus()
		clone := CloneStatus(orig)
		cloneSnapshot := *clone

		orig.Serial = Unavailable("orig-mut", "serial")
		orig.Temperature = Available("orig-mut")
		orig.IO = Unsupported("orig-mut", "io")
		orig.Controller = Missing("orig-mut", "controller")
		orig.Pool = Unavailable("orig-mut", "pool")

		if !reflect.DeepEqual(*clone, cloneSnapshot) {
			t.Fatalf("clone changed after mutating original: clone=%+v snapshot=%+v", *clone, cloneSnapshot)
		}
	})
}

// fullyPopulatedStatus returns a CollectionStatus with every field set to a
// distinct, non-zero FieldStatus so equality and independence checks are
// sensitive to every field on the struct.
func fullyPopulatedStatus() *CollectionStatus {
	return &CollectionStatus{
		Serial:      Available("smartctl"),
		Temperature: Unavailable("smartctl", "deadline exceeded"),
		IO:          Missing("kernel", "counter absent"),
		Controller:  Unsupported("provider", "not exposed"),
		Pool:        Available("provider"),
	}
}
