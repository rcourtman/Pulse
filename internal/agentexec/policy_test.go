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
		{"blocked sudo with flags", "sudo -u root rm -rf /", PolicyBlock},
		{"blocked proc environ", "cat /proc/self/environ", PolicyBlock},
		{"blocked proc kcore", "cat /proc/kcore", PolicyBlock},
		{"auto approve", "df -h", PolicyAllow},
		{"auto approve safe proc meminfo", "cat /proc/meminfo", PolicyAllow},
		{"auto approve ipv4 ping probe", "ping -c 1 -W 1 10.0.0.1", PolicyAllow},
		{"auto approve ipv6 ping probe", "ping -c 1 -W 1 2001:db8::1", PolicyAllow},
		{"require approval", "systemctl restart nginx", PolicyRequireApproval},
		{"generic proc read requires approval", "cat /proc/1/status", PolicyRequireApproval},
		{"hostname ping requires approval", "ping -c 1 -W 1 example.com", PolicyRequireApproval},
		{"unknown defaults to approval", "echo hello", PolicyRequireApproval},
		{"sudo with flags remains conservative", "sudo -u root df -h", PolicyRequireApproval},
		{"compound command requires approval", "df -h && echo ok", PolicyRequireApproval},
		{"compound ping requires approval", "ping -c 1 -W 1 10.0.0.1; echo ok", PolicyRequireApproval},
		{"find delete requires approval", "find /var -type f -delete", PolicyRequireApproval},

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
	if !p.IsBlocked("sudo -u root rm -rf /") {
		t.Fatalf("expected sudo -u root rm -rf / to be blocked")
	}
	if !p.IsBlocked("cat /proc/self/environ") {
		t.Fatalf("expected cat /proc/self/environ to be blocked")
	}
	if !p.NeedsApproval("echo hello") {
		t.Fatalf("expected echo hello to require approval by default")
	}
	if !p.NeedsApproval("cat /proc/1/status") {
		t.Fatalf("expected cat /proc/1/status to require approval")
	}
	if !p.IsAutoApproved("df -h") {
		t.Fatalf("expected df -h to be auto approved")
	}
	if !p.IsAutoApproved("cat /proc/meminfo") {
		t.Fatalf("expected cat /proc/meminfo to be auto approved")
	}
	if !p.IsAutoApproved("ping -c 1 -W 1 10.0.0.1") {
		t.Fatalf("expected single-target ping probe to be auto approved")
	}
}
