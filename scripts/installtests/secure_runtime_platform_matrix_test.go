package installtests

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

func TestSecureRuntimePlatformMatrixRemainsExplicitAndShipped(t *testing.T) {
	canonical, err := os.ReadFile(repoFile("docs", "AGENT_SECURITY.md"))
	if err != nil {
		t.Fatalf("read canonical agent security documentation: %v", err)
	}
	shipped, err := os.ReadFile(repoFile("frontend-modern", "public", "docs", "AGENT_SECURITY.md"))
	if err != nil {
		t.Fatalf("read shipped agent security documentation: %v", err)
	}
	if !bytes.Equal(canonical, shipped) {
		t.Fatal("shipped Agent Security documentation differs from the canonical source")
	}

	content := string(canonical)
	required := []string{
		"### Safe-profile support and qualification matrix",
		"Standard Linux systemd host telemetry and collector update",
		"Linux SMART telemetry",
		"Proxmox node-local LXC filesystem telemetry",
		"Rootful Docker or Podman inventory",
		"Collector-owned rootless Docker or Podman",
		"Separate runner package update and package-cache cleanup",
		"Separate runner Proxmox guest and container lifecycle/update actions",
		"Appliance, non-systemd, Windows, and macOS host-agent profiles",
		"Implemented, unqualified",
		"collectionMode: typed-helper-summary",
		"currently explicit rather than the installer default.",
		"Residual owner and removal condition",
	}
	for _, marker := range required {
		if !strings.Contains(content, marker) {
			t.Errorf("secure-runtime platform matrix is missing %q", marker)
		}
	}
}
