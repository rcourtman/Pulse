package api

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCanonicalAgentDownloadExecutables(t *testing.T) {
	root := os.Getenv("PULSE_AGENT_RELEASE_TEST_DIR")
	if root == "" {
		t.Skip("requires canonical cross-build worker packet")
	}
	for _, target := range []string{"linux-amd64", "windows-amd64", "darwin-arm64"} {
		path := filepath.Join(root, target)
		if err := validateUnifiedAgentBinary(path, "v6.2.1", target); err != nil {
			t.Fatalf("canonical %s: %v", target, err)
		}
		if err := validateUnifiedAgentBinary(path, "v1.35.1", target); err == nil {
			t.Fatalf("dependency literal accepted for %s", target)
		}
	}
}
