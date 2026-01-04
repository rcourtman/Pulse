package main

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rs/zerolog"
)

func TestParseLogLevel(t *testing.T) {
	tests := []struct {
		input string
		want  zerolog.Level
	}{
		{"trace", zerolog.TraceLevel},
		{"debug", zerolog.DebugLevel},
		{"DEBUG", zerolog.DebugLevel},
		{"info", zerolog.InfoLevel},
		{"warn", zerolog.WarnLevel},
		{"warning", zerolog.WarnLevel},
		{"error", zerolog.ErrorLevel},
		{"fatal", zerolog.FatalLevel},
		{"panic", zerolog.PanicLevel},
		{"disabled", zerolog.Disabled},
		{"none", zerolog.Disabled},
		{"unknown", zerolog.InfoLevel},
		{"", zerolog.InfoLevel},
	}
	for _, tt := range tests {
		if got := parseLogLevel(tt.input); got != tt.want {
			t.Errorf("parseLogLevel(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestDropPrivileges(t *testing.T) {
	// Save original functions
	oldGeteuid := osGeteuid
	oldResolve := resolveUserSpecFunc
	oldSetgroups := unixSetgroups
	oldSetgid := unixSetgid
	oldSetuid := unixSetuid
	defer func() {
		osGeteuid = oldGeteuid
		resolveUserSpecFunc = oldResolve
		unixSetgroups = oldSetgroups
		unixSetgid = oldSetgid
		unixSetuid = oldSetuid
	}()

	// Mock for root user
	osGeteuid = func() int { return 0 }
	resolveUserSpecFunc = func(u string) (*userSpec, error) {
		return &userSpec{name: u, uid: 1000, gid: 1000, groups: []int{1000}}, nil
	}
	unixSetgroups = func(g []int) error { return nil }
	unixSetgid = func(g int) error { return nil }
	unixSetuid = func(u int) error { return nil }

	// Test success path
	spec, err := dropPrivileges("testuser")
	if err != nil {
		t.Errorf("dropPrivileges failed: %v", err)
	}
	if spec == nil {
		t.Fatal("expected spec, got nil")
	}
	if spec.uid != 1000 {
		t.Errorf("expected uid 1000, got %d", spec.uid)
	}

	// Test non-root (should return nil, nil)
	osGeteuid = func() int { return 1000 }
	spec, err = dropPrivileges("testuser")
	if err != nil {
		t.Errorf("unexpected error for non-root: %v", err)
	}
	if spec != nil {
		t.Error("expected nil spec for non-root")
	}
}

func TestResolveUserSpec_PasswdFallback(t *testing.T) {
	// Mock passwd file
	tmpDir := t.TempDir()
	pPath, _ := os.CreateTemp(tmpDir, "passwd")
	pPath.WriteString("testuser:x:1001:1001::/home/testuser:/bin/sh\n")
	pPath.Close()

	origPath := passwdPath
	defer func() { passwdPath = origPath }()
	passwdPath = pPath.Name()

	// Test
	spec, err := lookupUserFromPasswd("testuser")
	if err != nil {
		t.Fatalf("lookup failed: %v", err)
	}
	if spec.uid != 1001 || spec.gid != 1001 {
		t.Errorf("mismatch: %+v", spec)
	}

	// Test not found
	_, err = lookupUserFromPasswd("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent user")
	}
}

func TestEnsureSSHKeypair(t *testing.T) {
	tmpDir := t.TempDir()
	proxy := &Proxy{sshKeyPath: tmpDir}

	// Mock exec for ssh-keygen
	origExec := execCommandFunc
	defer func() { execCommandFunc = origExec }()
	execCommandFunc = func(name string, arg ...string) *exec.Cmd {
		args := strings.Join(arg, " ")
		if strings.Contains(args, "ssh-keygen") {
			// Parse shell command to find -f
			// cmd string is in arg[1] (after -c)
			if len(arg) > 1 {
				cmdStr := arg[1]
				parts := strings.Fields(cmdStr)
				for i, p := range parts {
					if p == "-f" && i+1 < len(parts) {
						path := parts[i+1]
						os.WriteFile(path, []byte("priv"), 0600)
						os.WriteFile(path+".pub", []byte("pub"), 0644)
					}
				}
			}
			return mockExecCommand("")
		}
		return mockExecCommand("")
	}

	// First run: generate
	if err := proxy.ensureSSHKeypair(); err != nil {
		t.Fatalf("ensureSSHKeypair failed: %v", err)
	}

	if _, err := os.Stat(filepath.Join(tmpDir, "id_ed25519")); err != nil {
		t.Error("private key not created")
	}

	// Second run: existing
	// Restore exec to fail if called (should not be called)
	execCommandFunc = func(name string, arg ...string) *exec.Cmd {
		return errorExecCommand("should not be called")
	}
	if err := proxy.ensureSSHKeypair(); err != nil {
		t.Fatalf("ensureSSHKeypair existing failed: %v", err)
	}
}

type mockListener struct {
	net.Listener
	closed bool
}

func (m *mockListener) Close() error {
	m.closed = true
	return nil
}

func (m *mockListener) Accept() (net.Conn, error) {
	// Block until closed
	select {}
}

func (m *mockListener) Addr() net.Addr {
	return &net.UnixAddr{Name: "/tmp/sock", Net: "unix"}
}

func TestProxy_StartStop(t *testing.T) {
	tmpDir := t.TempDir()
	sshDir := filepath.Join(tmpDir, "ssh")
	socketPath := filepath.Join(tmpDir, "sock")

	// Mock net.Listen
	origListen := netListen
	defer func() { netListen = origListen }()
	listenCalled := false
	netListen = func(network, address string) (net.Listener, error) {
		listenCalled = true
		return &mockListener{}, nil
	}

	// Mock exec for key gen
	origExec := execCommandFunc
	defer func() { execCommandFunc = origExec }()
	execCommandFunc = func(name string, arg ...string) *exec.Cmd {
		args := strings.Join(arg, " ")
		if strings.Contains(args, "ssh-keygen") {
			// Create dummy key files
			for i, a := range arg {
				if a == "-f" && i+1 < len(arg) {
					os.MkdirAll(filepath.Dir(arg[i+1]), 0755)
					os.WriteFile(arg[i+1], []byte("priv"), 0600)
					os.WriteFile(arg[i+1]+".pub", []byte("pub"), 0644)
				}
			}
			return mockExecCommand("")
		}
		return mockExecCommand("")
	}

	proxy := &Proxy{
		sshKeyPath: sshDir,
		socketPath: socketPath,
		metrics:    NewProxyMetrics("test"),
	}

	if err := proxy.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	if !listenCalled {
		t.Error("net.Listen not called")
	}

	// Check directories created
	if _, err := os.Stat(sshDir); err != nil {
		t.Error("ssh dir not created")
	}

	// Stop
	proxy.Stop()
	// Should close listener -> our mock doesn't block Stop.

	// Check socket removed (Start code removes it first)
	// But our mock listener doesn't create file.
	// The Start() function calls os.RemoveAll(p.socketPath).
}

// Helpers for http_server_test.go which I might have deleted if they were in main_test.go
func mockExecCommand(output string) *exec.Cmd {
	cs := []string{"-test.run=TestHelperProcess", "--", output}
	cmd := exec.Command(os.Args[0], cs...)
	cmd.Env = []string{"GO_WANT_HELPER_PROCESS=1", "GO_HELPER_OUTPUT=" + output}
	return cmd
}

func errorExecCommand(msg string) *exec.Cmd {
	cs := []string{"-test.run=TestHelperProcess", "--", msg}
	cmd := exec.Command(os.Args[0], cs...)
	cmd.Env = []string{"GO_WANT_HELPER_PROCESS=1", "GO_HELPER_ERROR=" + msg}
	return cmd
}

func TestHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}

	if msg := os.Getenv("GO_HELPER_ERROR"); msg != "" {
		fmt.Fprint(os.Stderr, msg)
		os.Exit(1)
	}

	output := os.Getenv("GO_HELPER_OUTPUT")
	fmt.Fprint(os.Stdout, output)
	os.Exit(0)
}
