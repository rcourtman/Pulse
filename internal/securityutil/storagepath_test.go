package securityutil

import "testing"

func TestJoinStorageLeaf(t *testing.T) {
	path, err := JoinStorageLeaf("/tmp/pulse", "session.json")
	if err != nil {
		t.Fatalf("JoinStorageLeaf() error = %v", err)
	}
	if path != "/tmp/pulse/session.json" {
		t.Fatalf("JoinStorageLeaf() = %q", path)
	}
}

func TestJoinStorageLeafRejectsPathSeparators(t *testing.T) {
	tests := []string{
		"../session.json",
		"nested/session.json",
		`nested\session.json`,
		".",
		"..",
		"",
	}

	for _, leaf := range tests {
		if _, err := JoinStorageLeaf("/tmp/pulse", leaf); err == nil {
			t.Fatalf("expected %q to be rejected", leaf)
		}
	}
}
