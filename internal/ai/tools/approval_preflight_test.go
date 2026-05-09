package tools

import (
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/approval"
)

func TestClassifyApprovalCommand_BucketsKnownCommandShapes(t *testing.T) {
	cases := []struct {
		name    string
		target  string
		command string
		want    string
	}{
		{"systemctl restart canonical", "agent", "systemctl restart nginx", "service-restart"},
		{"service restart legacy", "agent", "service nginx restart", ""},
		{"service restart canonical", "agent", "service restart nginx", "service-restart"},
		{"systemctl stop", "agent", "systemctl stop postgres", "service-stop"},
		{"systemctl start", "agent", "systemctl start redis", "service-start"},
		{"systemctl reload", "agent", "systemctl reload nginx", "service-reload"},
		{"docker restart", "docker", "docker restart homepage", "container-restart"},
		{"podman restart", "agent", "podman restart pihole", "container-restart"},
		{"docker stop", "docker", "docker stop oldcontainer", "container-stop"},
		{"kubectl rollout restart", "kubernetes", "kubectl rollout restart deployment/api", "k8s-rollout-restart"},
		// Proxmox VM lifecycle classes — Pulse's primary monitored platform.
		// Each verb has distinct semantics so they map to distinct classes.
		{"qm reboot", "agent", "qm reboot 101", "proxmox-vm-reboot"},
		{"qm restart alias", "agent", "qm restart 101", "proxmox-vm-reboot"},
		{"qm stop hard halt", "agent", "qm stop 101", "proxmox-vm-stop"},
		{"qm start", "agent", "qm start 101", "proxmox-vm-start"},
		{"qm shutdown graceful", "agent", "qm shutdown 101", "proxmox-vm-shutdown"},
		// Proxmox LXC container lifecycle classes.
		{"pct reboot", "agent", "pct reboot 200", "proxmox-ct-reboot"},
		{"pct restart alias", "agent", "pct restart 200", "proxmox-ct-reboot"},
		{"pct stop hard halt", "agent", "pct stop 200", "proxmox-ct-stop"},
		{"pct start", "agent", "pct start 200", "proxmox-ct-start"},
		{"pct shutdown graceful", "agent", "pct shutdown 200", "proxmox-ct-shutdown"},
		// Negative cases: tool prefix without a recognized verb must NOT
		// classify (defends against `qm migrate`, `pct destroy`, etc.).
		{"qm migrate not a lifecycle verb", "agent", "qm migrate 101 pve-2", ""},
		{"pct destroy not a lifecycle verb", "agent", "pct destroy 200", ""},
		{"unknown free-form command", "agent", "echo hello world", ""},
		{"empty command", "agent", "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := classifyApprovalCommand(tc.target, tc.command)
			if got != tc.want {
				t.Fatalf("classifyApprovalCommand(%q, %q) = %q, want %q", tc.target, tc.command, got, tc.want)
			}
		})
	}
}

func TestApprovalCommandClassPreflightAdditions_AuthorsConcreteContextForKnownClasses(t *testing.T) {
	cases := []struct {
		name        string
		command     string
		safetyToken string // must appear somewhere in safety checks
		verifyToken string // must appear somewhere in verification steps
	}{
		{"service-restart names systemctl is-active", "systemctl restart nginx", "briefly unavailable", "systemctl is-active"},
		{"service-stop warns dependent services", "systemctl stop postgres", "dependent services", "inactive"},
		{"container-restart names docker inspect", "docker restart homepage", "briefly unavailable", "docker inspect"},
		{"k8s rollout names rollout status", "kubectl rollout restart deployment/api", "PodDisruptionBudget", "rollout status"},
		// Proxmox classes must concretely warn about the destructive vs
		// graceful split (qm stop = hard halt, qm shutdown = graceful) and
		// name the canonical `qm status` / `pct status` read-after-write
		// check the operator should see post-dispatch.
		{"qm reboot names qm status", "qm reboot 101", "ACPI shutdown", "qm status"},
		{"qm stop warns it is hard not graceful", "qm stop 101", "hard stop", "stopped"},
		{"qm shutdown names timeout fallback", "qm shutdown 101", "ACPI shutdown", "qm status"},
		{"pct stop warns it is hard not graceful", "pct stop 200", "hard stop", "stopped"},
		{"pct shutdown names lxc-attach", "pct shutdown 200", "lxc-attach", "pct status"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			safety, verify := approvalCommandClassPreflightAdditions("agent", tc.command)
			if len(safety) == 0 || len(verify) == 0 {
				t.Fatalf("expected non-empty preflight additions for %q, got safety=%v verify=%v", tc.command, safety, verify)
			}
			joinedSafety := strings.Join(safety, " ")
			if !strings.Contains(joinedSafety, tc.safetyToken) {
				t.Fatalf("safety checks missing token %q for %q: %v", tc.safetyToken, tc.command, safety)
			}
			joinedVerify := strings.Join(verify, " ")
			if !strings.Contains(joinedVerify, tc.verifyToken) {
				t.Fatalf("verification steps missing token %q for %q: %v", tc.verifyToken, tc.command, verify)
			}
		})
	}
}

func TestApprovalCommandClassPreflightAdditions_ReturnsEmptyForUnknownClass(t *testing.T) {
	// Unknown command shapes must not be padded with fabricated safety or
	// verification copy. Default preflight content stands on its own.
	safety, verify := approvalCommandClassPreflightAdditions("agent", "echo hello world")
	if safety != nil || verify != nil {
		t.Fatalf("expected nil additions for unknown command, got safety=%v verify=%v", safety, verify)
	}
}

func TestVerificationCommandForCommand_DerivesPerClassReadAfterWriteCheck(t *testing.T) {
	cases := []struct {
		name    string
		target  string
		command string
		wantCmd string
		wantOk  bool
	}{
		{"systemctl restart", "agent", "systemctl restart nginx", "systemctl is-active 'nginx'", true},
		{"systemctl reload with .service suffix", "agent", "systemctl reload my.service", "systemctl is-active 'my.service'", true},
		{"systemctl stop", "agent", "systemctl stop postgres", "systemctl is-active 'postgres'", true},
		// Docker/podman classes are intentionally excluded from broker-level
		// verification — pulse_docker runs its own docker inspect check at
		// the tool layer, so adding a broker-level dispatch would double-run.
		{"docker restart deferred to tool layer", "docker", "docker restart homepage", "", false},
		{"podman restart deferred to tool layer", "agent", "podman restart pihole", "", false},
		// Proxmox classes are intentionally excluded from broker-level
		// verification — pulse_control's verifyGuestAction already runs
		// `qm status` and `pct status` at the tool layer, so adding a
		// parallel broker-level dispatch would double-run the same check.
		// The preflight copy still names the verification narrative; only
		// the broker-side derivation is suppressed.
		{"qm reboot deferred to pulse_control", "agent", "qm reboot 101", "", false},
		{"qm stop deferred to pulse_control", "agent", "qm stop 101", "", false},
		{"qm shutdown deferred to pulse_control", "agent", "qm shutdown 101", "", false},
		{"pct stop deferred to pulse_control", "agent", "pct stop 200", "", false},
		{"pct shutdown deferred to pulse_control", "agent", "pct shutdown 200", "", false},
		{"unknown command", "agent", "echo hello", "", false},
		{"systemctl with single-quote in unit", "agent", `systemctl restart nasty'name`, "systemctl is-active 'nasty'\\''name'", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cmd, ok := VerificationCommandForCommand(tc.target, tc.command)
			if ok != tc.wantOk {
				t.Fatalf("VerificationCommandForCommand(%q, %q) ok = %v, want %v", tc.target, tc.command, ok, tc.wantOk)
			}
			if cmd != tc.wantCmd {
				t.Fatalf("VerificationCommandForCommand(%q, %q) cmd = %q, want %q", tc.target, tc.command, cmd, tc.wantCmd)
			}
		})
	}
}

func TestApprovalPreflight_MergesClassAdditionsIntoSafetyAndVerification(t *testing.T) {
	// End-to-end check that approvalPreflight surfaces the per-class
	// additions to the operator without losing the default safety /
	// verification content.
	req := &approval.ApprovalRequest{
		ID:         "approval-systemctl-restart",
		Command:    "systemctl restart nginx",
		TargetType: "agent",
		TargetID:   "agent-1",
		TargetName: "edge-1",
		Context:    "restart nginx after Patrol detected stale config",
	}
	preflight := approvalPreflight(req)
	if preflight == nil {
		t.Fatalf("expected preflight, got nil")
	}

	joinedSafety := strings.Join(preflight.SafetyChecks, " | ")
	if !strings.Contains(joinedSafety, "Approval is scoped to the current organization") {
		t.Fatalf("default safety check missing: %v", preflight.SafetyChecks)
	}
	if !strings.Contains(joinedSafety, "briefly unavailable") {
		t.Fatalf("class-specific safety check missing: %v", preflight.SafetyChecks)
	}

	joinedVerify := strings.Join(preflight.VerificationSteps, " | ")
	if !strings.Contains(joinedVerify, "Persist unified action audit lifecycle") {
		t.Fatalf("default verification step missing: %v", preflight.VerificationSteps)
	}
	if !strings.Contains(joinedVerify, "systemctl is-active") {
		t.Fatalf("class-specific verification step missing: %v", preflight.VerificationSteps)
	}
}
