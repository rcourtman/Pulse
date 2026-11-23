package pulsecli

import (
	"bytes"
	"crypto/x509"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentcapabilities"
	"github.com/rcourtman/pulse-go-rewrite/pkg/pulsecli/authstore"
)

type doctorRoundTripFunc func(*http.Request) (*http.Response, error)

func (f doctorRoundTripFunc) Do(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestDoctorPassesHealthyHTTPSInstance(t *testing.T) {
	const testToken = "pulse_tok_doctor_ok"
	var sawSurface []string

	ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sawSurface = append(sawSurface, r.Header.Get(agentcapabilities.AgentSurfaceHeader))
		switch r.URL.Path {
		case agentcapabilities.AgentCapabilitiesPath:
			_ = json.NewEncoder(w).Encode(map[string]any{
				"version": "v1",
				"capabilities": []map[string]any{
					{"name": "get_fleet_context"},
					{"name": "list_recent_changes"},
				},
			})
		case agentcapabilities.FleetContextCapabilityPath:
			if r.Header.Get(agentcapabilities.AgentAPITokenHeader) != testToken {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"resources": []map[string]any{{"id": "node-1"}, {"id": "vm-100"}, {"id": "ct-200"}},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()

	kc := newMemSecretStore(authstore.BackendKeychain, true)
	store := newTestAuthStore(t, kc)
	if _, err := store.SaveProfile("default", ts.URL, testToken, "read-only"); err != nil {
		t.Fatalf("SaveProfile() error = %v", err)
	}

	var out bytes.Buffer
	cmd := NewDoctorCommand(&DoctorDeps{
		HTTPClient: ts.Client(),
		Store:      store,
		Getenv:     func(string) string { return "" },
	})
	cmd.SetOut(&out)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"--json"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("doctor --json error = %v\noutput:\n%s", err, out.String())
	}

	if strings.Contains(out.String(), testToken) {
		t.Fatalf("doctor output must never leak raw token:\n%s", out.String())
	}
	if len(sawSurface) != 2 || sawSurface[0] != pulseCLIAgentSurface || sawSurface[1] != pulseCLIAgentSurface {
		t.Fatalf("X-Pulse-Agent-Surface headers = %v, want [%q, %q]", sawSurface, pulseCLIAgentSurface, pulseCLIAgentSurface)
	}

	var report doctorReport
	if err := json.Unmarshal(out.Bytes(), &report); err != nil {
		t.Fatalf("decode doctor JSON error = %v\n%s", err, out.String())
	}
	if !report.OK || report.ManifestVersion != "v1" || report.CapabilityCount != 2 || report.ResourceCount != 3 {
		t.Fatalf("unexpected doctor report: %+v", report)
	}
	for _, check := range report.Checks {
		if check.Status != doctorStatusPass {
			t.Fatalf("expected all checks pass, got %+v", check)
		}
	}
}

func TestDoctorWarnsOnNonLoopbackHTTPAndPassesOverall(t *testing.T) {
	kc := newMemSecretStore(authstore.BackendKeychain, true)
	store := newTestAuthStore(t, kc)
	if _, err := store.SaveProfile("default", "http://pulse.internal:7655", "tok-http", "read-only"); err != nil {
		t.Fatalf("SaveProfile() error = %v", err)
	}

	client := doctorRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		body := `{"resources":[]}`
		if req.URL.Path == agentcapabilities.AgentCapabilitiesPath {
			body = `{"version":"v1","capabilities":[{"name":"get_fleet_context"}]}`
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Status:     "200 OK",
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(body)),
		}, nil
	})

	var out bytes.Buffer
	cmd := NewDoctorCommand(&DoctorDeps{
		HTTPClient: client,
		Store:      store,
		Getenv:     func(string) string { return "" },
	})
	cmd.SetOut(&out)
	cmd.SetErr(&bytes.Buffer{})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("expected warning-only doctor to succeed, got %v", err)
	}
	if !strings.Contains(out.String(), "[WARN] transport") || !strings.Contains(out.String(), "Enable HTTPS") {
		t.Fatalf("expected transport HTTPS warning in output:\n%s", out.String())
	}
}

func TestDoctorFailsWithActionableFixes(t *testing.T) {
	t.Run("missing credentials", func(t *testing.T) {
		kc := newMemSecretStore(authstore.BackendKeychain, true)
		store := newTestAuthStore(t, kc)

		var out bytes.Buffer
		cmd := NewDoctorCommand(&DoctorDeps{
			Store:  store,
			Getenv: func(string) string { return "" },
		})
		cmd.SetOut(&out)
		cmd.SetErr(&bytes.Buffer{})
		cmd.SilenceUsage = true
		cmd.SilenceErrors = true

		if err := cmd.Execute(); err == nil {
			t.Fatal("expected error when no credentials are configured")
		}
		if !strings.Contains(out.String(), "[FAIL] credentials") || !strings.Contains(out.String(), "pulse auth login") {
			t.Fatalf("expected actionable credentials fix:\n%s", out.String())
		}
	})

	t.Run("401 unauthorized token", func(t *testing.T) {
		client := doctorRoundTripFunc(func(req *http.Request) (*http.Response, error) {
			if req.URL.Path == agentcapabilities.AgentCapabilitiesPath {
				return &http.Response{
					StatusCode: http.StatusOK,
					Status:     "200 OK",
					Header:     make(http.Header),
					Body:       io.NopCloser(strings.NewReader(`{"version":"v1","capabilities":[{"name":"get_fleet_context"}]}`)),
				}, nil
			}
			return &http.Response{
				StatusCode: http.StatusUnauthorized,
				Status:     "401 Unauthorized",
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(`{"error":"unauthorized"}`)),
			}, nil
		})

		var out bytes.Buffer
		cmd := NewDoctorCommand(&DoctorDeps{
			HTTPClient: client,
			Getenv: func(k string) string {
				if k == "PULSE_API_URL" {
					return "https://pulse.example.com"
				}
				if k == "PULSE_API_TOKEN" {
					return "expired-token"
				}
				return ""
			},
		})
		cmd.SetOut(&out)
		cmd.SetErr(&bytes.Buffer{})
		cmd.SilenceUsage = true
		cmd.SilenceErrors = true

		if err := cmd.Execute(); err == nil {
			t.Fatal("expected error on 401 token")
		}
		if !strings.Contains(out.String(), "[FAIL] fleet_read") || !strings.Contains(out.String(), externalAgentsNavLabel) {
			t.Fatalf("expected actionable 401 fix:\n%s", out.String())
		}
	})

	t.Run("403 missing monitoring:read scope", func(t *testing.T) {
		client := doctorRoundTripFunc(func(req *http.Request) (*http.Response, error) {
			if req.URL.Path == agentcapabilities.AgentCapabilitiesPath {
				return &http.Response{
					StatusCode: http.StatusOK,
					Status:     "200 OK",
					Header:     make(http.Header),
					Body:       io.NopCloser(strings.NewReader(`{"version":"v1","capabilities":[{"name":"get_fleet_context"}]}`)),
				}, nil
			}
			return &http.Response{
				StatusCode: http.StatusForbidden,
				Status:     "403 Forbidden",
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(`{"error":"forbidden"}`)),
			}, nil
		})

		var out bytes.Buffer
		cmd := NewDoctorCommand(&DoctorDeps{
			HTTPClient: client,
			Getenv: func(k string) string {
				if k == "PULSE_API_URL" {
					return "https://pulse.example.com"
				}
				if k == "PULSE_API_TOKEN" {
					return "limited-token"
				}
				return ""
			},
		})
		cmd.SetOut(&out)
		cmd.SetErr(&bytes.Buffer{})
		cmd.SilenceUsage = true
		cmd.SilenceErrors = true

		if err := cmd.Execute(); err == nil {
			t.Fatal("expected error on 403 token")
		}
		if !strings.Contains(out.String(), "monitoring:read") {
			t.Fatalf("expected monitoring:read guidance:\n%s", out.String())
		}
	})

	t.Run("TLS certificate failure", func(t *testing.T) {
		client := doctorRoundTripFunc(func(req *http.Request) (*http.Response, error) {
			return nil, x509.UnknownAuthorityError{}
		})

		var out bytes.Buffer
		cmd := NewDoctorCommand(&DoctorDeps{
			HTTPClient: client,
			Getenv: func(k string) string {
				if k == "PULSE_API_URL" {
					return "https://self-signed.example.com"
				}
				if k == "PULSE_API_TOKEN" {
					return "tok-tls"
				}
				return ""
			},
		})
		cmd.SetOut(&out)
		cmd.SetErr(&bytes.Buffer{})
		cmd.SilenceUsage = true
		cmd.SilenceErrors = true

		if err := cmd.Execute(); err == nil {
			t.Fatal("expected error on TLS certificate failure")
		}
		if !strings.Contains(out.String(), "[FAIL] manifest") || !strings.Contains(out.String(), "TLS certificate") {
			t.Fatalf("expected TLS certificate guidance:\n%s", out.String())
		}
	})
}
