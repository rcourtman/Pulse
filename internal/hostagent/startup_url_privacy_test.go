package hostagent

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/pkg/securityutil"
	"github.com/rs/zerolog"
	gohost "github.com/shirou/gopsutil/v4/host"
)

// All URLs and credentials in these tests are synthetic, never reporter data.
func TestHostURLRejectionIsPrivateAndReadOnly(t *testing.T) {
	for _, tc := range []struct{ name, raw string }{
		{"userinfo", "https://synthetic-user:synthetic-password@private-fixture.invalid"},
		{"query", "https://private-fixture.invalid/?token=synthetic-query"},
		{"fragment", "https://private-fixture.invalid/#synthetic-fragment"},
		{"port", "https://private-fixture.invalid:synthetic-port"},
		{"escape", "https://private-fixture.invalid/%synthetic-escape"},
		{"missing-host", "https:///synthetic-path"},
		{"scheme", "synthetic-scheme://private-fixture.invalid"},
		{"control", "https://private-fixture.invalid/\nsynthetic-control"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			stateDir := t.TempDir()
			saved := filepath.Join(stateDir, "runtime.token")
			if err := os.WriteFile(saved, []byte("synthetic-saved-token\n"), 0600); err != nil {
				t.Fatal(err)
			}
			before, err := os.Stat(saved)
			if err != nil {
				t.Fatal(err)
			}
			var logs bytes.Buffer
			logger := zerolog.New(&logs)
			probes := 0
			collector := &mockCollector{hostInfoFn: func(context.Context) (*gohost.InfoStat, error) {
				probes++
				return nil, errors.New("host discovery must not run")
			}}
			agent, newErr := New(Config{PulseURL: tc.raw, APIToken: "synthetic-runtime-token", StateDir: stateDir,
				Logger: &logger, Collector: collector})
			if agent != nil {
				t.Fatal("invalid URL produced a host module")
			}
			result, enrollErr := Enroll(context.Background(), EnrollmentConfig{PulseURL: tc.raw,
				APIToken: "synthetic-bootstrap-token", StateDir: stateDir, Logger: &logger, Collector: collector})
			if result.APIToken != "" || result.AgentID != "" {
				t.Fatal("invalid URL produced enrollment credentials")
			}
			for _, err := range []error{newErr, enrollErr} {
				if err == nil || !strings.Contains(err.Error(), "invalid pulse URL") {
					t.Fatal("URL refusal lost its configuration context")
				}
				if strings.Contains(err.Error(), "synthetic-") || strings.Contains(err.Error(), "private-fixture.invalid") {
					t.Error("URL refusal disclosed rejected input")
				}
			}
			if probes != 0 || logs.Len() != 0 {
				t.Error("invalid URL reached discovery or logging")
			}
			entries, err := os.ReadDir(stateDir)
			if err != nil || len(entries) != 1 {
				t.Fatal("URL refusal added saved state")
			}
			data, readErr := os.ReadFile(saved)
			after, statErr := os.Stat(saved)
			if readErr != nil || statErr != nil || string(data) != "synthetic-saved-token\n" || after.Mode() != before.Mode() {
				t.Fatal("URL refusal changed saved credentials or permissions")
			}
		})
	}
}

func TestHostURLDiagnosticReusePreservesTransportPolicy(t *testing.T) {
	t.Cleanup(func() { securityutil.SetOperatorPlaintextHTTPConsent(false) })
	for _, consent := range []bool{false, true} {
		securityutil.SetOperatorPlaintextHTTPConsent(consent)
		for _, raw := range []string{
			" HTTPS://Pulse.Invalid:7655/base/ ", "https://pulse.invalid/a%2Fb///", "https://pulse.invalid/?",
			"http://localhost:7655/", "http://127.0.0.1:7655/", "http://[::1]:7655/",
			"http://192.168.1.20:7655/", "http://pulse:7655/", "http://203.0.113.10/",
			"https://user:password@pulse.invalid", "https://pulse.invalid/?token=value", "https://pulse.invalid/#fragment",
			"https://pulse.invalid:0", "https://pulse.invalid:65536", "https://pulse.invalid:bad-port",
			"ftp://pulse.invalid/", "wss://pulse.invalid/", "https:///missing-host", "pulse.invalid/base", " ",
		} {
			want, wantErr := securityutil.NormalizePulseHTTPBaseURLWithOptions(raw, securityutil.PulseURLValidationOptions{AllowLocalNetworkHTTP: true})
			got, err := normalizePulseURL(raw)
			if (err != nil) != (wantErr != nil) {
				t.Fatal("diagnostic reuse changed transport admission")
			}
			if err == nil && got != want.String() {
				t.Fatal("diagnostic reuse changed an accepted URL")
			}
		}
	}
}
