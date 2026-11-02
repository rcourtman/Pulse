package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/pkg/securityutil"
)

func TestRunValidateURLIsReadOnly(t *testing.T) {
	t.Cleanup(func() { securityutil.SetOperatorPlaintextHTTPConsent(false) })
	for _, tc := range []struct {
		name, url string
		insecure  bool
		wantError bool
	}{
		{name: "https", url: "https://pulse.example.invalid/base"},
		{name: "loopback", url: "http://127.0.0.1:7655"},
		{name: "private", url: "http://192.168.1.20:7655"},
		{name: "local-hostname", url: "http://pulse:7655"},
		{name: "public", url: "http://203.0.113.10", wantError: true},
		{name: "insecure-is-not-http-consent", url: "http://203.0.113.10", insecure: true, wantError: true},
		{name: "unresolved", url: "http://pulse.example.invalid", wantError: true},
		{name: "userinfo", url: "https://user:secret@pulse.example.invalid", wantError: true},
		{name: "wrong-scheme", url: "ftp://pulse.example.invalid", wantError: true},
		{name: "missing", wantError: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			stateDir := t.TempDir()
			files := map[string]string{"agent-id": "existing-id\n", "token": "existing-token\n", "runtime.token": "existing-runtime\n", "connection.env": "existing-connection\n", "install.sh": "existing-recovery\n"}
			for name, value := range files {
				if err := os.WriteFile(filepath.Join(stateDir, name), []byte(value), 0600); err != nil {
					t.Fatal(err)
				}
			}
			if err := os.Chmod(stateDir, 0750); err != nil {
				t.Fatal(err)
			}
			getenv := func(key string) string {
				if key == "PULSE_INSECURE_SKIP_VERIFY" && tc.insecure {
					return "true"
				}
				return ""
			}
			// Missing credential/observer files and enrollment must not be read or
			// run. A configured log must not be created even on invalid input.
			err := run(context.Background(), []string{"--validate-url", "--url", tc.url, "--state-dir", stateDir,
				"--token-file", filepath.Join(stateDir, "absent-token"), "--observers-file", filepath.Join(stateDir, "absent-observers"),
				"--log-file", filepath.Join(stateDir, "absent-log"), "--enroll"}, getenv)
			if (err != nil) != tc.wantError || (err != nil && !strings.Contains(err.Error(), "invalid pulse URL")) {
				t.Fatalf("URL admission error=%v, want rejection=%t", err, tc.wantError)
			}
			entries, err := os.ReadDir(stateDir)
			if err != nil || len(entries) != len(files) {
				t.Fatalf("URL check added state/log files: %v, %v", entries, err)
			}
			info, err := os.Stat(stateDir)
			if err != nil || info.Mode().Perm() != 0750 {
				t.Fatalf("URL check changed state directory permissions: %v, %v", info, err)
			}
			for name, value := range files {
				path := filepath.Join(stateDir, name)
				got, err := os.ReadFile(path)
				info, statErr := os.Stat(path)
				if err != nil || statErr != nil || string(got) != value || info.Mode().Perm() != 0600 {
					t.Fatalf("URL check changed %s: %q, %v, %v", name, got, err, statErr)
				}
			}
		})
	}
}

func TestURLValidationKeepsExplicitPlaintextConsentSeparate(t *testing.T) {
	t.Cleanup(func() { securityutil.SetOperatorPlaintextHTTPConsent(false) })
	for _, allow := range []bool{false, true} {
		cfg, err := loadConfig([]string{"--validate-url", "--url", "http://203.0.113.10", "--insecure"}, func(key string) string {
			if key == "PULSE_AGENT_ALLOW_PLAINTEXT_HTTP" && allow {
				return "true"
			}
			return ""
		})
		if err != nil || !cfg.ValidateURLOnly || cfg.AllowPlaintextHTTP != allow {
			t.Fatalf("explicit URL-check consent: %+v, %v", cfg, err)
		}
	}
}
