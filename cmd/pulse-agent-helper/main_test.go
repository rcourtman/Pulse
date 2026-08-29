package main

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/agenthelper"
)

func TestParseFlagsAcceptsOnlyLocalHelperConfiguration(t *testing.T) {
	config, err := parseFlags([]string{
		"--socket", "/run/pulse-agent/custom-helper.sock",
		"--allowed-uid", "1001",
		"--socket-gid", "1002",
		"--max-deadline", "45s",
	})
	if err != nil {
		t.Fatalf("parseFlags: %v", err)
	}
	if config.socketPath != "/run/pulse-agent/custom-helper.sock" ||
		config.allowedUID != 1001 || config.socketGID != 1002 || config.maxDeadline != 45*time.Second {
		t.Fatalf("config = %#v", config)
	}

	for _, forbidden := range [][]string{
		{"--url", "https://pulse.example"},
		{"--token", "secret"},
		{"--listen", "tcp://127.0.0.1:9999"},
		{"positional"},
	} {
		if _, err := parseFlags(forbidden); err == nil {
			t.Fatalf("network/credential/positional flags accepted: %v", forbidden)
		}
	}
}

func TestParseFlagsRejectsUnsafeValues(t *testing.T) {
	tests := [][]string{
		{"--socket", "relative.sock"},
		{"--socket", "/run/../tmp/helper.sock"},
		{"--max-deadline", "0s"},
		{"--max-deadline", "1ns"},
		{"--max-deadline", "6m"},
		{"--allowed-uid", "-2"},
		{"--socket-gid", "-2"},
		{"--allowed-uid", "4294967296"},
		{"--socket-gid", "4294967296"},
	}
	for _, args := range tests {
		if _, err := parseFlags(args); err == nil {
			t.Fatalf("unsafe flags accepted: %v", args)
		}
	}
}

func TestResolveIdentityUsesExplicitNumericValues(t *testing.T) {
	uid, gid, err := resolveIdentity(1001, 1002)
	if err != nil {
		t.Fatalf("resolveIdentity: %v", err)
	}
	if uid != 1001 || gid != 1002 {
		t.Fatalf("uid=%d gid=%d", uid, gid)
	}
}

func TestResolveIdentityRejectsOutOfRangeNumericValues(t *testing.T) {
	if _, _, err := resolveIdentity(int64(^uint32(0))+1, 1002); err == nil {
		t.Fatal("UID above uint32 accepted")
	}
	if strconv.IntSize == 32 {
		if _, _, err := resolveIdentity(1001, int64(math.MaxInt32)+1); err == nil {
			t.Fatal("GID above 32-bit int accepted")
		}
	}
}

func TestInheritedSystemdListenerRejectsMalformedActivation(t *testing.T) {
	t.Setenv("LISTEN_PID", "")
	t.Setenv("LISTEN_FDS", "")
	if listener, ok, err := inheritedSystemdListener(); err != nil || ok || listener != nil {
		t.Fatalf("no activation = listener=%v ok=%t err=%v", listener, ok, err)
	}

	t.Setenv("LISTEN_PID", "not-a-pid")
	t.Setenv("LISTEN_FDS", "1")
	if _, _, err := inheritedSystemdListener(); err == nil {
		t.Fatal("malformed LISTEN_PID accepted")
	}

	t.Setenv("LISTEN_PID", strings.TrimSpace(strings.Repeat("0", 1)))
	t.Setenv("LISTEN_FDS", "2")
	if _, _, err := inheritedSystemdListener(); err == nil {
		t.Fatal("foreign LISTEN_PID accepted")
	}
}

func TestOpenListenerRefusesToReplaceNonSocket(t *testing.T) {
	t.Setenv("LISTEN_PID", "")
	t.Setenv("LISTEN_FDS", "")
	path := filepath.Join(t.TempDir(), "helper.sock")
	if err := os.WriteFile(path, []byte("do-not-replace"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := openListener(path, os.Getgid()); err == nil {
		t.Fatal("regular file at socket path was replaced")
	}
	contents, err := os.ReadFile(path)
	if err != nil || string(contents) != "do-not-replace" {
		t.Fatalf("protected file changed: contents=%q err=%v", contents, err)
	}
}

func TestCommandRegistryWiresTypedLocalProviders(t *testing.T) {
	var _ agenthelper.SMARTProvider = localSMARTProvider{}
	var _ agenthelper.ProxmoxProvider = localProxmoxProvider{}
	registry := agenthelper.NewRegistry(localSMARTProvider{}, localProxmoxProvider{})
	// Registry availability and strict empty request schemas are exercised
	// through the public protocol by the internal server suite.
	if registry == nil {
		t.Fatal("registry is nil")
	}
}

func TestLocalProxmoxProviderAlwaysReturnsValidJSON(t *testing.T) {
	result, err := (localProxmoxProvider{}).LXCFilesystems(t.Context())
	if err != nil {
		t.Fatalf("LXCFilesystems: %v", err)
	}
	if !json.Valid(result) {
		t.Fatalf("invalid provider JSON: %q", result)
	}
}

func TestRunFailsClosedOutsideLinux(t *testing.T) {
	if runtime.GOOS == "linux" {
		t.Skip("non-Linux fail-closed assertion")
	}
	if err := run(nil); err == nil || !strings.Contains(err.Error(), "only on Linux") {
		t.Fatalf("run error = %v", err)
	}
}
