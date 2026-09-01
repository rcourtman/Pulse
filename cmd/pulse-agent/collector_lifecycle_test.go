package main

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/collectorlifecycle"
	internalsecurity "github.com/rcourtman/pulse-go-rewrite/internal/securityutil"
)

func TestCollectorLifecycleCommandReducesAuthorityWithFileBearer(t *testing.T) {
	const bearer = "file-only-collector-bearer"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost || request.URL.Path != "/api/agents/collector/reduce-authority" {
			t.Errorf("request = %s %s", request.Method, request.URL.Path)
		}
		if got := request.Header.Get("Authorization"); got != "Bearer "+bearer {
			t.Errorf("Authorization = %q", got)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()
	tokenFile := writeCollectorLifecycleToken(t, bearer)
	var stdout, stderr bytes.Buffer
	err := runCollectorLifecycleCommand(context.Background(), collectorReduceAuthorityCommand, []string{
		"--url", server.URL,
		"--token-file", tokenFile,
		"--token-owner-uid", collectorLifecycleTestOwnerUID(),
		"--agent-id", "agent-1",
		"--hostname", "host.local",
	}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("runCollectorLifecycleCommand: %v (stderr %q)", err, stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
}

func TestCollectorLifecycleCommandRejectsBearerArgument(t *testing.T) {
	const bearer = "must-not-enter-argv"
	var stdout, stderr bytes.Buffer
	err := runCollectorLifecycleCommand(context.Background(), collectorReduceAuthorityCommand, []string{
		"--url", "http://127.0.0.1:7655",
		"--token", bearer,
		"--agent-id", "agent-1",
		"--hostname", "host.local",
	}, &stdout, &stderr)
	if err == nil || !strings.Contains(err.Error(), "flag provided but not defined: -token") {
		t.Fatalf("error = %v, want raw bearer flag rejection", err)
	}
}

func TestCollectorLifecycleCommandPrintsAuthoritativeLastSeen(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":true,"agent":{"id":"agent-1","hostname":"host.local","lastSeen":"2026-08-30T12:00:01.123456789Z"}}`))
	}))
	defer server.Close()
	var stdout, stderr bytes.Buffer
	err := runCollectorLifecycleCommand(context.Background(), collectorVerifyRegistrationCommand, []string{
		"--url", server.URL,
		"--token-file", writeCollectorLifecycleToken(t, "collector-bearer"),
		"--token-owner-uid", collectorLifecycleTestOwnerUID(),
		"--agent-id", "agent-1",
		"--previous-last-seen", "2026-08-30T12:00:01Z",
	}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("runCollectorLifecycleCommand: %v (stderr %q)", err, stderr.String())
	}
	if got := strings.TrimSpace(stdout.String()); got != "2026-08-30T12:00:01.123456789Z" {
		t.Fatalf("stdout = %q", got)
	}
}

func TestCollectorLifecycleCommandSafelyReadsAgentIdentity(t *testing.T) {
	identityFile := writeCollectorLifecycleToken(t, "agent-file-bound")
	var stdout, stderr bytes.Buffer
	err := runCollectorLifecycleCommand(context.Background(), collectorReadAgentIDCommand, []string{
		"--agent-id-file", identityFile,
		"--token-owner-uid", collectorLifecycleTestOwnerUID(),
	}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("runCollectorLifecycleCommand: %v (stderr %q)", err, stderr.String())
	}
	if got := strings.TrimSpace(stdout.String()); got != "agent-file-bound" {
		t.Fatalf("stdout = %q", got)
	}
}

func TestCollectorLifecycleCommandSafelyReadsCollectorToken(t *testing.T) {
	tokenFile := writeCollectorLifecycleToken(t, "collector-file-bound")
	var stdout, stderr bytes.Buffer
	err := runCollectorLifecycleCommand(context.Background(), collectorReadTokenCommand, []string{
		"--token-file", tokenFile,
		"--token-owner-uid", collectorLifecycleTestOwnerUID(),
	}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("runCollectorLifecycleCommand: %v (stderr %q)", err, stderr.String())
	}
	if got := strings.TrimSpace(stdout.String()); got != "collector-file-bound" {
		t.Fatalf("stdout = %q", got)
	}
}

func TestCollectorLifecycleCommandDownloadsInstallerThroughPublicTransport(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/install.sh" || request.Header.Get("Authorization") != "" {
			t.Errorf("request path=%q authorization=%q", request.URL.Path, request.Header.Get("Authorization"))
		}
		w.Header().Set("X-Signature-SSHSIG", "installer-signature")
		_, _ = w.Write([]byte("#!/usr/bin/env bash\necho secure\n"))
	}))
	defer server.Close()
	outputPath := filepath.Join(t.TempDir(), "installer.tmp")
	if err := os.WriteFile(outputPath, nil, 0600); err != nil {
		t.Fatal(err)
	}
	if err := internalsecurity.HardenPrivatePath(outputPath, 0600); err != nil {
		t.Fatalf("harden installer output: %v", err)
	}
	var stdout, stderr bytes.Buffer
	err := runCollectorLifecycleCommand(context.Background(), collectorDownloadInstallerCommand, []string{
		"--url", server.URL,
		"--output", outputPath,
	}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("runCollectorLifecycleCommand: %v (stderr %q)", err, stderr.String())
	}
	if got := strings.TrimSpace(stdout.String()); got != "installer-signature" {
		t.Fatalf("signature stdout = %q", got)
	}
	if body, err := os.ReadFile(outputPath); err != nil || string(body) != "#!/usr/bin/env bash\necho secure\n" {
		t.Fatalf("installer body=%q err=%v", body, err)
	}
}

func TestCollectorLifecycleCommandExitCodeDistinguishesRejectedCredential(t *testing.T) {
	if got := collectorLifecycleExitCode(nil); got != 0 {
		t.Fatalf("nil exit code = %d", got)
	}
	if got := collectorLifecycleExitCode(collectorlifecycle.ErrRegistrationPending); got != 1 {
		t.Fatalf("pending exit code = %d", got)
	}
	if got := collectorLifecycleExitCode(errors.Join(errors.New("lookup failed"), collectorlifecycle.ErrCredentialRejected)); got != 2 {
		t.Fatalf("rejected exit code = %d", got)
	}
}

func writeCollectorLifecycleToken(t *testing.T, bearer string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "collector.token")
	if err := os.WriteFile(path, []byte(bearer), 0600); err != nil {
		t.Fatal(err)
	}
	if err := internalsecurity.HardenPrivatePath(path, 0600); err != nil {
		t.Fatalf("harden collector lifecycle token: %v", err)
	}
	return path
}
