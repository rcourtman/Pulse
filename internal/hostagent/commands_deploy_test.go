package hostagent

import (
	"context"
	"os/exec"
	"strings"
	"testing"

	sshknownhosts "github.com/rcourtman/pulse-go-rewrite/internal/ssh/knownhosts"
	"github.com/rs/zerolog"
)

type stubKnownHostsManager struct {
	path string
}

func (m stubKnownHostsManager) Ensure(context.Context, string) error { return nil }
func (m stubKnownHostsManager) EnsureWithPort(context.Context, string, int) error {
	return nil
}
func (m stubKnownHostsManager) EnsureWithEntries(context.Context, string, int, [][]byte) error {
	return nil
}
func (m stubKnownHostsManager) Path() string { return m.path }

var _ sshknownhosts.Manager = stubKnownHostsManager{}

func TestShellescape(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"http://10.0.0.1:7655", "'http://10.0.0.1:7655'"},
		{"simple", "'simple'"},
		{"it's a test", "'it'\"'\"'s a test'"},
		{"", "''"},
		{"$(whoami)", "'$(whoami)'"},
		{"; rm -rf /", "'; rm -rf /'"},
	}
	for _, tt := range tests {
		got := shellescape(tt.input)
		if got != tt.expected {
			t.Errorf("shellescape(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestValidateNodeIP(t *testing.T) {
	valid := []string{"10.0.0.1", "192.168.1.100", "::1", "fe80::1"}
	for _, ip := range valid {
		if err := validateNodeIP(ip); err != nil {
			t.Errorf("expected valid IP %q, got error: %v", ip, err)
		}
	}

	invalid := []string{"", "not-an-ip", "10.0.0.1; rm -rf /", "example.com", "10.0.0.1:22"}
	for _, ip := range invalid {
		if err := validateNodeIP(ip); err == nil {
			t.Errorf("expected error for invalid IP %q", ip)
		}
	}
}

func TestNormalizeDeploySSHUser(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{name: "empty defaults to root", input: "", want: ""},
		{name: "trimmed user", input: " pulse-deploy ", want: "pulse-deploy"},
		{name: "underscore and dot", input: "ec2_user.ops", want: "ec2_user.ops"},
		{name: "reject host separator", input: "user@example", wantErr: true},
		{name: "reject option style", input: "-root", wantErr: true},
		{name: "reject whitespace", input: "bad user", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NormalizeDeploySSHUser(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error for %q", tt.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("NormalizeDeploySSHUser(%q): %v", tt.input, err)
			}
			if got != tt.want {
				t.Fatalf("NormalizeDeploySSHUser(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestMakeSemaphore(t *testing.T) {
	// Zero/negative defaults to 1.
	sem := makeSemaphore(0)
	if cap(sem) != 1 {
		t.Errorf("expected capacity 1 for 0, got %d", cap(sem))
	}

	sem = makeSemaphore(-1)
	if cap(sem) != 1 {
		t.Errorf("expected capacity 1 for -1, got %d", cap(sem))
	}

	sem = makeSemaphore(3)
	if cap(sem) != 3 {
		t.Errorf("expected capacity 3, got %d", cap(sem))
	}
}

func TestMarshalPreflightResult(t *testing.T) {
	data := marshalPreflightResult(true, true, false, "amd64", "")
	if data == "" {
		t.Fatal("expected non-empty JSON")
	}
	// Should be valid JSON.
	if data[0] != '{' {
		t.Errorf("expected JSON object, got: %s", data)
	}
}

func TestMarshalInstallResult(t *testing.T) {
	data := marshalInstallResult(0, "success")
	if data == "" {
		t.Fatal("expected non-empty JSON")
	}

	// Test truncation.
	longOutput := make([]byte, 5000)
	for i := range longOutput {
		longOutput[i] = 'x'
	}
	data = marshalInstallResult(1, string(longOutput))
	if len(data) > 5000 {
		t.Error("expected truncated output in result")
	}
}

func TestDeployCancelTracker(t *testing.T) {
	called := false
	cancel := func() { called = true }

	registerDeployJob("j1", cancel)

	// Cancel should call the function.
	activeDeploysMu.Lock()
	fn, ok := activeDeploys["j1"]
	activeDeploysMu.Unlock()
	if !ok {
		t.Fatal("expected job to be registered")
	}
	fn()
	if !called {
		t.Error("expected cancel function to be called")
	}

	unregisterDeployJob("j1")
	activeDeploysMu.Lock()
	_, ok = activeDeploys["j1"]
	activeDeploysMu.Unlock()
	if ok {
		t.Error("expected job to be unregistered")
	}
}

func TestRunInstallSSH_IncludesEnrollAndEnableCommands(t *testing.T) {
	origExec := execCommandContext
	defer func() { execCommandContext = origExec }()

	var capturedArgs []string
	execCommandContext = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		capturedArgs = append([]string{name}, args...)
		// Return a command that exits 0 immediately.
		return exec.Command("true")
	}

	c := &CommandClient{
		logger:        zerolog.New(zerolog.NewTestWriter(t)),
		sshKnownHosts: stubKnownHostsManager{path: "/tmp/pulse-test-known-hosts"},
	}

	exitCode, _, err := c.runInstallSSH(context.Background(), "10.0.0.1", "https://10.0.0.1:7655")
	if err != nil {
		t.Fatalf("runInstallSSH error: %v", err)
	}
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", exitCode)
	}

	// Join all args to find the inner command string.
	joined := strings.Join(capturedArgs, " ")
	if !strings.Contains(joined, "--enroll") {
		t.Error("SSH install command does not include --enroll")
	}
	if !strings.Contains(joined, "--enable-commands") {
		t.Error("SSH install command does not include --enable-commands")
	}
	if !strings.Contains(joined, "--enable-proxmox") {
		t.Error("SSH install command does not include --enable-proxmox")
	}
	if !strings.Contains(joined, "StrictHostKeyChecking=yes") {
		t.Error("SSH install command does not enforce strict host key checking")
	}
	if !strings.Contains(joined, "UserKnownHostsFile=/tmp/pulse-test-known-hosts") {
		t.Error("SSH install command does not use the managed known_hosts file")
	}
	if !strings.Contains(joined, "GlobalKnownHostsFile=/dev/null") {
		t.Error("SSH install command does not isolate global known_hosts state")
	}
	if !strings.Contains(joined, "root@10.0.0.1") {
		t.Error("SSH install command does not default to root user")
	}
}

func TestRunInstallSSH_RejectsNonLoopbackPlainHTTP(t *testing.T) {
	c := &CommandClient{
		logger:        zerolog.New(zerolog.NewTestWriter(t)),
		sshKnownHosts: stubKnownHostsManager{path: "/tmp/pulse-test-known-hosts"},
	}

	if _, _, err := c.runInstallSSH(context.Background(), "10.0.0.1", "http://10.0.0.1:7655"); err == nil {
		t.Fatal("expected non-loopback plain HTTP Pulse URL to be rejected")
	}
}

func TestRunInstallSSH_AllowsNonLoopbackPlainHTTPInInsecureMode(t *testing.T) {
	origExec := execCommandContext
	defer func() { execCommandContext = origExec }()

	var capturedArgs []string
	execCommandContext = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		capturedArgs = append([]string{name}, args...)
		return exec.Command("true")
	}

	c := &CommandClient{
		insecureSkipVerify: true,
		logger:             zerolog.New(zerolog.NewTestWriter(t)),
		sshKnownHosts:      stubKnownHostsManager{path: "/tmp/pulse-test-known-hosts"},
	}

	exitCode, _, err := c.runInstallSSH(context.Background(), "10.0.0.1", "http://10.0.0.1:7655")
	if err != nil {
		t.Fatalf("runInstallSSH error: %v", err)
	}
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", exitCode)
	}
	joined := strings.Join(capturedArgs, " ")
	if !strings.Contains(joined, "http://10.0.0.1:7655") || !strings.Contains(joined, "/install.sh") {
		t.Fatalf("SSH install command missing plain HTTP installer URL: %s", joined)
	}
}

func TestRunInstallSSH_UsesConfiguredDeploySSHUserWithSudo(t *testing.T) {
	origExec := execCommandContext
	defer func() { execCommandContext = origExec }()

	var capturedArgs []string
	execCommandContext = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		capturedArgs = append([]string{name}, args...)
		return exec.Command("true")
	}

	c := &CommandClient{
		deploySSHUser: "pulse-deploy",
		logger:        zerolog.New(zerolog.NewTestWriter(t)),
		sshKnownHosts: stubKnownHostsManager{path: "/tmp/pulse-test-known-hosts"},
	}

	exitCode, _, err := c.runInstallSSH(context.Background(), "10.0.0.1", "https://10.0.0.1:7655")
	if err != nil {
		t.Fatalf("runInstallSSH error: %v", err)
	}
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", exitCode)
	}

	joined := strings.Join(capturedArgs, " ")
	if !strings.Contains(joined, "pulse-deploy@10.0.0.1") {
		t.Fatalf("SSH install command missing configured deploy user: %s", joined)
	}
	if strings.Contains(joined, "root@10.0.0.1") {
		t.Fatalf("SSH install command unexpectedly used root user: %s", joined)
	}
	if !strings.Contains(joined, "sudo -n bash -lc") {
		t.Fatalf("SSH install command missing sudo escalation for non-root deploy user: %s", joined)
	}
}

func TestWriteTokenSSH_UsesConfiguredDeploySSHUserWithSudo(t *testing.T) {
	origExec := execCommandContext
	defer func() { execCommandContext = origExec }()

	var capturedArgs []string
	execCommandContext = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		capturedArgs = append([]string{name}, args...)
		return exec.Command("true")
	}

	c := &CommandClient{
		deploySSHUser: "pulse-deploy",
		logger:        zerolog.New(zerolog.NewTestWriter(t)),
		sshKnownHosts: stubKnownHostsManager{path: "/tmp/pulse-test-known-hosts"},
	}

	if err := c.writeTokenSSH(context.Background(), "10.0.0.1", "token"); err != nil {
		t.Fatalf("writeTokenSSH error: %v", err)
	}

	joined := strings.Join(capturedArgs, " ")
	if !strings.Contains(joined, "pulse-deploy@10.0.0.1") {
		t.Fatalf("SSH token write missing configured deploy user: %s", joined)
	}
	if !strings.Contains(joined, "sudo -n bash -lc") {
		t.Fatalf("SSH token write missing sudo escalation for non-root deploy user: %s", joined)
	}
	if !strings.Contains(joined, "/run/pulse-agent/bootstrap.token") {
		t.Fatalf("SSH token write missing bootstrap token target path: %s", joined)
	}
}
