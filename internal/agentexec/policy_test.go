package agentexec

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

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

func TestDefaultPolicyVerifyWindow(t *testing.T) {
	p := DefaultPolicy()
	if p.VerifyWindow != DefaultVerifyWindow {
		t.Fatalf("default verify window = %v, want %v", p.VerifyWindow, DefaultVerifyWindow)
	}
}

func TestNormalizeVerifyWindowBounds(t *testing.T) {
	cases := []struct {
		in   time.Duration
		want time.Duration
	}{
		{0, DefaultVerifyWindow},
		{-1 * time.Second, DefaultVerifyWindow},
		{30 * time.Second, 30 * time.Second},
		{DefaultVerifyWindow, DefaultVerifyWindow},
		{MaxVerifyWindow, MaxVerifyWindow},
		{MaxVerifyWindow + time.Second, MaxVerifyWindow},
		{1 * time.Hour, MaxVerifyWindow},
	}
	for _, tc := range cases {
		if got := NormalizeVerifyWindow(tc.in); got != tc.want {
			t.Fatalf("NormalizeVerifyWindow(%v) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

func TestCommandPolicyNormalizeBoundsInPlace(t *testing.T) {
	p := &CommandPolicy{VerifyWindow: time.Hour}
	p.Normalize()
	if p.VerifyWindow != MaxVerifyWindow {
		t.Fatalf("Normalize did not clamp verify_window: got %v, want %v", p.VerifyWindow, MaxVerifyWindow)
	}

	q := &CommandPolicy{VerifyWindow: 0}
	q.Normalize()
	if q.VerifyWindow != DefaultVerifyWindow {
		t.Fatalf("Normalize did not default verify_window: got %v, want %v", q.VerifyWindow, DefaultVerifyWindow)
	}
}

func TestCommandPolicyJSONRoundtripVerifyWindow(t *testing.T) {
	original := &CommandPolicy{
		AutoApprove:     []string{"^df$"},
		RequireApproval: []string{"^systemctl\\s+restart"},
		Blocked:         []string{"^reboot$"},
		VerifyWindow:    90 * time.Second,
	}

	raw, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(raw), `"verify_window":"1m30s"`) {
		t.Fatalf("verify_window should serialize as a duration string; got %s", raw)
	}

	var decoded CommandPolicy
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if decoded.VerifyWindow != 90*time.Second {
		t.Fatalf("roundtripped verify_window = %v, want %v", decoded.VerifyWindow, 90*time.Second)
	}
}

func TestCommandPolicyJSONUnmarshalAppliesBounds(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want time.Duration
	}{
		{"empty defaults to default window", `{}`, DefaultVerifyWindow},
		{"zero string defaults to default", `{"verify_window":"0s"}`, DefaultVerifyWindow},
		{"over-cap clamps to max", `{"verify_window":"1h"}`, MaxVerifyWindow},
		{"reasonable value passes through", `{"verify_window":"5m"}`, 5 * time.Minute},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var p CommandPolicy
			if err := json.Unmarshal([]byte(tc.raw), &p); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			if p.VerifyWindow != tc.want {
				t.Fatalf("verify_window = %v, want %v", p.VerifyWindow, tc.want)
			}
		})
	}
}

func TestCommandPolicyJSONUnmarshalRejectsInvalidDuration(t *testing.T) {
	var p CommandPolicy
	if err := json.Unmarshal([]byte(`{"verify_window":"not-a-duration"}`), &p); err == nil {
		t.Fatalf("expected error for invalid verify_window")
	}
}
