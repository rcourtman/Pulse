package main

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentupdate"
	"github.com/rcourtman/pulse-go-rewrite/internal/dockeragent"
	"github.com/rcourtman/pulse-go-rewrite/internal/hostagent"
	"github.com/rcourtman/pulse-go-rewrite/internal/kubernetesagent"
	"github.com/rcourtman/pulse-go-rewrite/pkg/securityutil"
)

func TestRunRejectsURLBeforeLoggingOrStateChange(t *testing.T) {
	origHost, origDocker, origKube, origUpdater := newHostAgent, newDockerAgent, newKubeAgent, newUpdater
	t.Cleanup(func() {
		newHostAgent, newDockerAgent, newKubeAgent, newUpdater = origHost, origDocker, origKube, origUpdater
		securityutil.SetOperatorPlaintextHTTPConsent(false)
	})
	started := 0
	newHostAgent = func(hostagent.Config) (Runnable, error) { started++; return &mockRunnable{}, nil }
	newDockerAgent = func(dockeragent.Config) (RunnableCloser, error) { started++; return &mockRunnableCloser{}, nil }
	newKubeAgent = func(kubernetesagent.Config) (Runnable, error) { started++; return &mockRunnable{}, nil }
	newUpdater = func(cfg agentupdate.Config) *agentupdate.Updater {
		started++
		cfg.Disabled = true
		return agentupdate.New(cfg)
	}
	for _, enroll := range []bool{false, true} {
		for _, tc := range []struct{ name, raw string }{
			{"userinfo", "https://synthetic-user:synthetic-password@private-fixture.invalid"},
			{"query", "https://private-fixture.invalid/?token=synthetic-query"},
			{"fragment", "https://private-fixture.invalid/#synthetic-fragment"},
			{"port", "https://private-fixture.invalid:synthetic-port"},
			{"escape", "https://private-fixture.invalid/%synthetic-escape"},
			{"missing-host", "https:///synthetic-path"},
			{"scheme", "synthetic-scheme://private-fixture.invalid"},
			{"control", "https://private-fixture.invalid/\nsynthetic-control"},
			{"public-http", "http://203.0.113.10/synthetic-path"},
		} {
			t.Run(tc.name+"/enroll-"+strconv.FormatBool(enroll), func(t *testing.T) {
				stateDir := t.TempDir()
				files := map[string]string{"agent-id": "synthetic-saved-id\n", "token": "synthetic-saved-token\n",
					"runtime.token": "synthetic-saved-runtime\n", "connection.env": "synthetic-saved-connection\n", "install.sh": "synthetic-saved-recovery\n"}
				modes := make(map[string]os.FileMode)
				for name, data := range files {
					path := filepath.Join(stateDir, name)
					if err := os.WriteFile(path, []byte(data), 0600); err != nil {
						t.Fatal(err)
					}
					info, err := os.Stat(path)
					if err != nil {
						t.Fatal(err)
					}
					modes[name] = info.Mode()
				}
				if err := os.Chmod(stateDir, 0750); err != nil {
					t.Fatal(err)
				}
				before, err := os.Stat(stateDir)
				if err != nil {
					t.Fatal(err)
				}
				started = 0
				ctx, cancel := context.WithCancel(context.Background())
				cancel() // Baseline must also finish without contacting a service.
				err = run(ctx, []string{"--url", tc.raw, "--state-dir", stateDir,
					"--agent-id", "synthetic-runtime-id", "--token-file", filepath.Join(stateDir, "token"),
					"--enable-host", "--enable-docker", "--enable-kubernetes", "--disable-auto-update", "--health-addr", "",
					"--log-file", filepath.Join(stateDir, "unexpected.log"), "--enroll=" + strconv.FormatBool(enroll)}, func(string) string { return "" })
				if err == nil || !strings.Contains(err.Error(), "invalid pulse URL") {
					t.Error("startup did not reject the URL with correction context")
				} else if strings.Contains(err.Error(), "synthetic-") || strings.Contains(err.Error(), "private-fixture.invalid") || strings.Contains(err.Error(), "203.0.113.10") {
					t.Error("startup refusal disclosed rejected input")
				}
				if started != 0 {
					t.Error("invalid URL reached an updater or module constructor")
				}
				entries, err := os.ReadDir(stateDir)
				if err != nil || len(entries) != len(files) {
					t.Error("invalid URL created log or state files")
				}
				after, err := os.Stat(stateDir)
				if err != nil || after.Mode() != before.Mode() {
					t.Error("invalid URL changed state directory permissions")
				}
				for name, want := range files {
					path := filepath.Join(stateDir, name)
					data, readErr := os.ReadFile(path)
					info, statErr := os.Stat(path)
					if readErr != nil || statErr != nil || string(data) != want || info.Mode() != modes[name] {
						t.Error("invalid URL changed saved identity, credentials, recovery files or permissions")
					}
				}
			})
		}
	}
}

func TestRunURLAdmissionPreservesAcceptedSelfTest(t *testing.T) {
	t.Cleanup(func() { securityutil.SetOperatorPlaintextHTTPConsent(false) })
	for _, tc := range []struct {
		name, raw string
		plaintext bool
	}{
		{"https", " HTTPS://Pulse.Invalid:7655/base/ ", false},
		{"loopback", "http://127.0.0.1:7655", false},
		{"private-http", "http://192.168.1.20:7655/base", false},
		{"explicit-http", "http://203.0.113.10/base", true},
		{"default", "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			stateDir := t.TempDir()
			err := run(context.Background(), []string{"--url", tc.raw, "--self-test", "--state-dir", stateDir,
				"--allow-plaintext-http=" + strconv.FormatBool(tc.plaintext)}, func(string) string { return "" })
			if err != nil {
				t.Fatal("accepted URL no longer passes startup self-test")
			}
			entries, err := os.ReadDir(stateDir)
			if err != nil || len(entries) != 0 {
				t.Fatal("self-test started writing runtime state")
			}
		})
	}
}

func TestSelfTestDoesNotRequireRuntimeURLAdmission(t *testing.T) {
	// The updater launches this local executable probe without the service's
	// --allow-plaintext-http flag. It must not become a new transport-policy
	// gate; the separate --validate-url mode checks that policy.
	t.Cleanup(func() { securityutil.SetOperatorPlaintextHTTPConsent(false) })
	for _, raw := range []string{
		"http://203.0.113.10/synthetic-path",
		"https://synthetic-user:synthetic-password@private-fixture.invalid",
	} {
		stateDir := t.TempDir()
		err := run(context.Background(), []string{"--self-test", "--url", raw, "--state-dir", stateDir}, func(string) string { return "" })
		if err != nil {
			t.Fatal("local self-test acquired a runtime URL requirement")
		}
		entries, err := os.ReadDir(stateDir)
		if err != nil || len(entries) != 0 {
			t.Fatal("local self-test wrote runtime state")
		}
	}
}
