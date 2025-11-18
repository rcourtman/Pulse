package main

import (
	"strings"
	"testing"
)

func TestSanitizeDuplicateAllowedNodesBlocks_RemovesExtraBlocks(t *testing.T) {
	raw := `
allowed_nodes:
  - delly
  - minipc

# Cluster nodes (auto-discovered during installation)
# These nodes are allowed to request temperature data when cluster IPC validation is unavailable
allowed_nodes:
  - delly
  - minipc
  - extra
`

	sanitized, out := sanitizeDuplicateAllowedNodesBlocks("", []byte(raw))
	if !sanitized {
		t.Fatalf("expected sanitization to occur")
	}

	result := string(out)
	if strings.Count(result, "allowed_nodes:") != 1 {
		t.Fatalf("expected only one allowed_nodes block, got %q", result)
	}
	if strings.Contains(result, "extra") {
		t.Fatalf("duplicate entries should be removed, got %q", result)
	}
	if strings.Contains(result, "Cluster nodes (auto-discovered during installation)") {
		t.Fatalf("duplicate comment block should be removed")
	}
}

func TestSanitizeDuplicateAllowedNodesBlocks_NoChangeWhenUnique(t *testing.T) {
	raw := `
metrics_address: 127.0.0.1:9127
allowed_nodes:
  - delly
`
	sanitized, out := sanitizeDuplicateAllowedNodesBlocks("", []byte(raw))
	if sanitized {
		t.Fatalf("unexpected sanitization for unique config")
	}
	if string(out) != raw {
		t.Fatalf("expected config to remain unchanged")
	}
}
