package agentupdate

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"runtime/debug"
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentidentity"
	"github.com/rcourtman/pulse-go-rewrite/internal/agentidentity/identitytest"
	"github.com/rcourtman/pulse-go-rewrite/internal/securityutil"
)

func TestDownloadedIdentityVersionOutput(t *testing.T) {
	target, err := agentidentity.CurrentTarget()
	if err != nil {
		t.Fatal(err)
	}
	for _, output := range []string{"v6.2.1\n", "6.2.1\n", "v6.2.0-rc.8\n", "v6.2.10\n", "banner v6.2.1\n", "v6.2.1\nv6.2.1\n", strings.Repeat("x", 1024)} {
		path := identitytest.Build(t, identitytest.Options{Version: "v6.2.1", VersionOutput: output})
		u := New(Config{})
		err := u.verifyDownloadedIdentity(context.Background(), path, "6.2.1", target)
		wantOK := output == "v6.2.1\n" || output == "6.2.1\n"
		if !wantOK && err != nil {
			reason := "reports"
			if len(output) > 256 {
				reason = "invalid downloaded executable identity output"
			}
			if !strings.Contains(err.Error(), reason) {
				t.Fatalf("expected %q, got %v", reason, err)
			}
		}
		if (err == nil) != wantOK {
			t.Fatalf("output %q: %v", output, err)
		}
	}
}

func assertDownloadedIdentityRefusalAtomic(t *testing.T, o identitytest.Options, reason string) {
	t.Helper()
	originalRestart, originalExit := restartProcessFn, exitFn
	restarts, exits := 0, 0
	restartProcessFn = func(string) error { restarts++; return nil }
	exitFn = func(int) { exits++ }
	t.Cleanup(func() { restartProcessFn, exitFn = originalRestart, originalExit })
	fixture := identitytest.Build(t, o)
	binary, err := os.ReadFile(fixture)
	if err != nil {
		t.Fatal(err)
	}
	dir, execPath := writeTempExec(t)
	state := filepath.Join(dir, "state")
	if err := os.Mkdir(state, 0700); err != nil {
		t.Fatal(err)
	}
	sentinel := filepath.Join(state, "previous-version")
	if err := os.WriteFile(sentinel, []byte("preserved"), 0600); err != nil {
		t.Fatal(err)
	}
	continuity := filepath.Join(dir, ".pulse-update-info")
	oldInfo := []byte("preserved executable continuity")
	if err := os.WriteFile(continuity, oldInfo, 0600); err != nil {
		t.Fatal(err)
	}
	u := New(Config{PulseURL: "http://localhost", AgentName: "pulse-agent", CurrentVersion: "v6.1.0", StateDir: state})
	u.client = recoveryUpdateClient("6.2.1", binary, 200, "")
	selfTest := false
	u.selfTestFn = func(context.Context, string) error { selfTest = true; return nil }
	err = u.performUpdateWithExecPathForVersion(context.Background(), execPath, "6.2.1")
	if err == nil || !strings.Contains(err.Error(), reason) {
		t.Fatalf("expected refusal %q, got %v", reason, err)
	}
	if selfTest || restarts != 0 || exits != 0 {
		t.Fatalf("activation effects: self-test=%v restart=%d exit=%d", selfTest, restarts, exits)
	}
	current, err := os.ReadFile(execPath)
	if err != nil || string(current) != "old-binary" {
		t.Fatal("running executable changed")
	}
	saved, err := os.ReadFile(sentinel)
	if err != nil || string(saved) != "preserved" {
		t.Fatal("saved state changed")
	}
	saved, err = os.ReadFile(continuity)
	if err != nil || !bytes.Equal(saved, oldInfo) {
		t.Fatal("executable continuity metadata changed")
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 3 {
		t.Fatalf("unexpected rename/staging files: %v %v", entries, err)
	}
	entries, err = os.ReadDir(state)
	if err != nil || len(entries) != 1 {
		t.Fatal("update state was created")
	}
}

func TestDownloadedIdentityRefusalIsAtomic(t *testing.T) {
	for _, platform := range []goOS{goOSLinux, goOSWindows} {
		t.Run(string(platform), func(t *testing.T) {
			originalOS, originalInfo := runtimeGOOS, readBuildInfoFn
			t.Cleanup(func() { runtimeGOOS, readBuildInfoFn = originalOS, originalInfo })
			runtimeGOOS = platform
			readBuildInfoFn = func() (*debug.BuildInfo, bool) {
				return &debug.BuildInfo{Settings: []debug.BuildSetting{{Key: "GOOS", Value: string(platform)}, {Key: "GOARCH", Value: "amd64"}}}, true
			}
			for _, tc := range []struct {
				o      identitytest.Options
				reason string
			}{
				{identitytest.Options{Version: "v6.2.0-rc.8", Target: string(platform) + "-amd64"}, "stale build"},
				{identitytest.Options{Version: "v6.2.10", Target: string(platform) + "-amd64"}, "stale build"},
				{identitytest.Options{Version: "v6.2.0", Target: string(platform) + "-amd64", Extra: "v6.2.1"}, "stale build"},
				{identitytest.Options{Version: "v6.2.1", Target: string(platform) + "-arm64"}, "executable target"},
			} {
				assertDownloadedIdentityRefusalAtomic(t, tc.o, tc.reason)
			}
		})
	}
}

// Runtime-output controls execute the actual host target. The simulated
// Windows/Linux cases above cover static rejection without foreign execution.
func TestDownloadedRuntimeIdentityRefusalIsAtomic(t *testing.T) {
	target, err := agentidentity.CurrentTarget()
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		o      identitytest.Options
		reason string
	}{
		{identitytest.Options{Version: "v6.2.1", Target: target, VersionOutput: "v6.2.0\n"}, "reports"},
		{identitytest.Options{Version: "v6.2.1", Target: target, IdentityOutput: "[[pulse-agent-release:v1:v6.2.0]]\n"}, "release identity does not match"},
		{identitytest.Options{Version: "v6.2.1", Target: target, IdentityOutput: "\n"}, "release identity does not match"},
	} {
		assertDownloadedIdentityRefusalAtomic(t, tc.o, tc.reason)
	}
}

func TestDownloadedIdentityPrivilegedRefusalIsAtomic(t *testing.T) {
	original := runtimeGOOS
	t.Cleanup(func() { runtimeGOOS = original })
	runtimeGOOS = goOSLinux
	path := identitytest.Build(t, identitytest.Options{Version: "v6.2.0-rc.8", Target: "linux-amd64"})
	binary, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	helper := &fakePrivilegedUpdate{root: t.TempDir()}
	if err := securityutil.HardenPrivatePath(helper.root, 0700); err != nil {
		t.Fatal(err)
	}
	u := New(Config{PrivilegedUpdate: helper, CurrentVersion: "v6.1.0", StateDir: helper.root})
	called := false
	u.selfTestFn = func(context.Context, string) error { called = true; return nil }
	err = u.performPrivilegedUpdate(context.Background(), "/usr/local/bin/pulse-agent", bytes.NewReader(binary), int64(len(binary)), checksum(binary), "signed-update", "6.2.1", "linux-amd64")
	if err == nil || called {
		t.Fatalf("identity refusal %v self-test %v", err, called)
	}
	if !reflect.DeepEqual(helper.events, []string{"quarantine", "cleanup"}) {
		t.Fatalf("helper mutated: %v", helper.events)
	}
	if pending, err := LoadPendingPrivilegedUpdate(helper.root); err != nil || pending != nil {
		t.Fatalf("pending state changed: %v %v", pending, err)
	}
}

func TestDownloadedRuntimeReleaseIdentity(t *testing.T) {
	target, err := agentidentity.CurrentTarget()
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ output, reason string }{
		{"\n", "release identity does not match"},
		{"[[pulse-agent-release:v1:v6.2.0]]\n", "release identity does not match"},
		{"banner [[pulse-agent-release:v1:v6.2.1]]\n", "release identity does not match"},
		{strings.Repeat("x", 1024), "invalid downloaded executable identity output"},
		{"[[pulse-agent-release:v1:v6.2.1]]\n", ""},
	} {
		path := identitytest.Build(t, identitytest.Options{Version: "v6.2.1", IdentityOutput: tc.output})
		if err := agentidentity.Validate(path, "6.2.1", target); err != nil {
			t.Fatalf("runtime control failed static admission: %v", err)
		}
		err := New(Config{}).verifyDownloadedIdentity(context.Background(), path, "6.2.1", target)
		if tc.reason == "" {
			if err != nil {
				t.Fatal(err)
			}
		} else if err == nil || !strings.Contains(err.Error(), tc.reason) {
			t.Fatalf("output %q: expected %q, got %v", tc.output, tc.reason, err)
		}
	}
	path := identitytest.Build(t, identitytest.Options{Version: "v6.2.1"})
	if err := New(Config{}).verifyDownloadedIdentity(context.Background(), path, "6.2.1", ""); err == nil || !strings.Contains(err.Error(), "missing expected executable release target") {
		t.Fatalf("absent target: %v", err)
	}
}

func TestVersionOutputCannotBypassBoundThroughCopy(t *testing.T) {
	cancelled := false
	output := &versionOutput{cancel: func() { cancelled = true }}
	// Hide the source WriteTo method so io.Copy can choose a destination
	// ReaderFrom fast path. An embedded bytes.Buffer exposed an unbounded one.
	reader := struct{ io.Reader }{strings.NewReader(strings.Repeat("x", 1024))}
	if _, err := io.Copy(output, reader); err == nil {
		t.Fatal("oversized copy accepted")
	}
	if !cancelled || !output.overflow || output.Len() > 256 {
		t.Fatalf("unbounded output: cancelled=%v overflow=%v length=%d", cancelled, output.overflow, output.Len())
	}
}

func TestDownloadFailurePreservesIdentityConflict(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("arch") != "" {
			w.WriteHeader(http.StatusConflict)
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	_, execPath := writeTempExec(t)
	updater := New(Config{PulseURL: server.URL, AgentName: "pulse-agent", CurrentVersion: "6.1.0"})
	err := updater.performUpdateWithExecPathForVersion(context.Background(), execPath, "6.2.1")
	if err == nil || !strings.Contains(err.Error(), "409") || !strings.Contains(err.Error(), "404") {
		t.Fatalf("lost candidate refusal: %v", err)
	}
}

func TestInvalidAdvertisedVersionRefusesBeforeDownload(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { requests++; w.WriteHeader(500) }))
	defer server.Close()
	_, path := writeTempExec(t)
	for _, version := range []string{"6.2.1.0", "06.2.1", ""} {
		helper := &fakePrivilegedUpdate{root: t.TempDir()}
		updater := New(Config{PulseURL: server.URL, AgentName: "pulse-agent", CurrentVersion: "6.1.0", PrivilegedUpdate: helper})
		err := updater.performUpdateWithExecPathForVersion(context.Background(), path, version)
		if err == nil || !strings.Contains(err.Error(), "invalid advertised executable version") {
			t.Fatalf("%q: %v", version, err)
		}
		if requests != 0 || len(helper.events) != 0 {
			t.Fatalf("pre-admission effects: requests=%d helper=%v", requests, helper.events)
		}
		data, err := os.ReadFile(path)
		if err != nil || string(data) != "old-binary" {
			t.Fatal("executable changed")
		}
	}
}
