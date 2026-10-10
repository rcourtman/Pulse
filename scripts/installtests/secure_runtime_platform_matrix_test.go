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
		"systemd does not enforce `RestrictAddressFamilies` on native 32-bit x86",
		"Do not\nremove `PrivateNetwork` to work around a telemetry failure.",
		"helper's private network namespace",
		"operation v2 preserves the collected",
		// The helper now avoids abstract IPC rather than relaxing its private
		// network. Check the replacement route and its remaining limits, not
		// the retired matrix wording that called every such host unavailable.
		"Configs are read through the pmxcfs mount",
		"cgroup v2 and `/proc`",
		"pins the guest's init, re-checks its identity",
		"the caller selects no VMID, path, or command",
		"whenever discovery cannot establish a guest's state",
		"Does not yet justify Proxmox host-agent parity or a default change",
	}
	for _, marker := range required {
		if !strings.Contains(content, marker) {
			t.Errorf("secure-runtime platform matrix is missing %q", marker)
		}
	}
}
