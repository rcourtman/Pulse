package manager

import (
	"testing"
)

func TestNew(t *testing.T) {
	// Test that New returns a non-nil Manager
	m := New(nil)
	if m == nil {
		t.Error("New(nil) returned nil")
	}

	// Verify the manager is properly initialized
	if m.stores == nil {
		t.Error("Manager.stores is nil")
	}
}

func TestNew_NilPersistence(t *testing.T) {
	// Test that New handles nil persistence gracefully
	// The Manager itself should be created, even if persistence is nil
	m := New(nil)
	if m == nil {
		t.Fatal("New(nil) should return non-nil Manager")
	}

	// The Manager struct should be valid but operations requiring
	// persistence should fail appropriately
	if m.stores == nil {
		t.Error("Manager.stores should be initialized even with nil persistence")
	}
}
