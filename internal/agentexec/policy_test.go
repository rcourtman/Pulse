package agentexec

import "testing"

func TestCompilePatternsIgnoresInvalidRegex(t *testing.T) {
	res := compilePatterns([]string{"^df(\\s|$)", "["})
	if len(res) != 1 {
		t.Fatalf("expected 1 compiled regex, got %d", len(res))
	}
}

func TestDefaultPolicyEvaluate(t *testing.T) {
	p := DefaultPolicy()

	cases := []struct {
		name    string
		command string
		want    PolicyDecision
	}{
		{"blocked", "rm -rf /", PolicyBlock},
		{"blocked sudo", "sudo rm -rf /", PolicyBlock},
		{"auto approve", "df -h", PolicyAllow},
		{"require approval", "systemctl restart nginx", PolicyRequireApproval},
		{"unknown defaults to approval", "echo hello", PolicyRequireApproval},
		{"sudo with flags remains conservative", "sudo -u root df -h", PolicyRequireApproval},

		// Proxmox VM control - should require approval, not be blocked
		{"qm reboot requires approval", "qm reboot 201", PolicyRequireApproval},
		{"qm shutdown requires approval", "qm shutdown 201", PolicyRequireApproval},
		{"pct reboot requires approval", "pct reboot 100", PolicyRequireApproval},
		{"pct shutdown requires approval", "pct shutdown 100", PolicyRequireApproval},

		// Host-level system commands should be blocked
		{"bare reboot blocked", "reboot", PolicyBlock},
		{"bare shutdown blocked", "shutdown", PolicyBlock},
		{"shutdown now blocked", "shutdown now", PolicyBlock},
		{"reboot now blocked", "reboot -f", PolicyBlock},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := p.Evaluate(tc.command); got != tc.want {
				t.Fatalf("Evaluate(%q) = %q, want %q", tc.command, got, tc.want)
			}
		})
	}
}

func TestPolicyHelpers(t *testing.T) {
	p := DefaultPolicy()
	if !p.IsBlocked("rm -rf /") {
		t.Fatalf("expected rm -rf / to be blocked")
	}
	if !p.NeedsApproval("echo hello") {
		t.Fatalf("expected echo hello to require approval by default")
	}
	if !p.IsAutoApproved("df -h") {
		t.Fatalf("expected df -h to be auto approved")
	}
}
