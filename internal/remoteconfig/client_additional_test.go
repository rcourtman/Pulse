package remoteconfig

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestClientFetchWithSignature(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	t.Setenv("PULSE_AGENT_CONFIG_PUBLIC_KEYS", base64.StdEncoding.EncodeToString(pub))

	issuedAt := time.Now().UTC()
	expiresAt := issuedAt.Add(5 * time.Minute)
	commands := true
	settings := map[string]interface{}{"interval": "1m"}

	payload := SignedConfigPayload{
		HostID:          "agent-1",
		IssuedAt:        issuedAt,
		ExpiresAt:       expiresAt,
		CommandsEnabled: &commands,
		Settings:        settings,
	}
	signature, err := SignConfigPayload(payload, priv)
	if err != nil {
		t.Fatalf("SignConfigPayload: %v", err)
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/agents/host/agent-1/config" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		resp := Response{
			Success: true,
			HostID:  "agent-1",
		}
		resp.Config.CommandsEnabled = &commands
		resp.Config.Settings = settings
		resp.Config.IssuedAt = issuedAt
		resp.Config.ExpiresAt = expiresAt
		resp.Config.Signature = signature
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	client := New(Config{
		PulseURL: ts.URL,
		APIToken: "token-123",
		AgentID:  "agent-1",
	})

	gotSettings, gotCommands, err := client.Fetch(context.Background())
	if err != nil {
		t.Fatalf("Fetch error: %v", err)
	}
	if gotCommands == nil || *gotCommands != true {
		t.Fatalf("expected commands enabled, got %v", gotCommands)
	}
	if gotSettings["interval"] != "1m" {
		t.Fatalf("unexpected settings: %#v", gotSettings)
	}
}

func TestClientFetchSignatureFailures(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	t.Setenv("PULSE_AGENT_CONFIG_PUBLIC_KEYS", base64.StdEncoding.EncodeToString(pub))

	settings := map[string]interface{}{"interval": "1m"}
	makeResp := func(sig string, issued, expires time.Time) Response {
		resp := Response{
			Success: true,
			HostID:  "agent-1",
		}
		resp.Config.Settings = settings
		resp.Config.Signature = sig
		resp.Config.IssuedAt = issued
		resp.Config.ExpiresAt = expires
		return resp
	}

	issuedAt := time.Now().UTC()
	expiresAt := issuedAt.Add(5 * time.Minute)
	payload := SignedConfigPayload{
		HostID:    "agent-1",
		IssuedAt:  issuedAt,
		ExpiresAt: expiresAt,
		Settings:  settings,
	}
	signature, err := SignConfigPayload(payload, priv)
	if err != nil {
		t.Fatalf("SignConfigPayload: %v", err)
	}

	tests := []struct {
		name     string
		resp     Response
		wantText string
	}{
		{name: "missing timestamps", resp: makeResp(signature, time.Time{}, time.Time{}), wantText: "missing timestamp"},
		{name: "expired", resp: makeResp(signature, issuedAt.Add(-10*time.Minute), issuedAt.Add(-5*time.Minute)), wantText: "expired"},
		{name: "future", resp: makeResp(signature, issuedAt.Add(10*time.Minute), issuedAt.Add(20*time.Minute)), wantText: "future"},
		{name: "invalid signature", resp: makeResp("nope", issuedAt, expiresAt), wantText: "verification failed"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_ = json.NewEncoder(w).Encode(tt.resp)
			}))
			defer ts.Close()

			client := New(Config{PulseURL: ts.URL, APIToken: "t", AgentID: "agent-1"})
			_, _, err := client.Fetch(context.Background())
			if err == nil || !strings.Contains(err.Error(), tt.wantText) {
				t.Fatalf("expected error containing %q, got %v", tt.wantText, err)
			}
		})
	}
}

func TestClientFetchHostLookupAndErrors(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/agents/host/lookup":
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"success":true,"host":{"id":"host-9"}}`))
		case "/api/agents/host/host-9/config":
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"success":true,"hostId":"host-9","config":{"settings":{"mode":"ok"}}}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer ts.Close()

	client := New(Config{
		PulseURL: ts.URL,
		APIToken: "t",
		AgentID:  "agent-1",
		Hostname: "known",
	})
	settings, _, err := client.Fetch(context.Background())
	if err != nil {
		t.Fatalf("Fetch error: %v", err)
	}
	if settings["mode"] != "ok" {
		t.Fatalf("unexpected settings: %#v", settings)
	}

	redirect := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/other", http.StatusFound)
	}))
	defer redirect.Close()

	client = New(Config{PulseURL: redirect.URL, APIToken: "t", AgentID: "agent-1"})
	if _, _, err := client.Fetch(context.Background()); err == nil {
		t.Fatalf("expected redirect error")
	}

	badJSON := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("{bad"))
	}))
	defer badJSON.Close()

	client = New(Config{PulseURL: badJSON.URL, APIToken: "t", AgentID: "agent-1"})
	if _, _, err := client.Fetch(context.Background()); err == nil {
		t.Fatalf("expected decode error")
	}
}

func TestClientNewDefaultsAndHostLookupNotFound(t *testing.T) {
	client := New(Config{InsecureSkipVerify: true})
	if client.cfg.PulseURL != "http://localhost:7655" {
		t.Fatalf("unexpected default PulseURL: %s", client.cfg.PulseURL)
	}

	transport, ok := client.httpClient.Transport.(*http.Transport)
	if !ok || transport.TLSClientConfig == nil || !transport.TLSClientConfig.InsecureSkipVerify {
		t.Fatalf("expected insecure TLS config")
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()

	client = New(Config{
		PulseURL: ts.URL,
		APIToken: "t",
		Hostname: "missing",
	})
	if got, err := client.resolveHostID(context.Background()); err != nil || got != "" {
		t.Fatalf("expected empty host ID, got %q err=%v", got, err)
	}
}

func TestClientFetchResolveHostIDError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/lookup") {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()

	client := New(Config{
		PulseURL: ts.URL,
		APIToken: "t",
		AgentID:  "agent-1",
		Hostname: "known",
	})
	if _, _, err := client.Fetch(context.Background()); err == nil {
		t.Fatalf("expected resolve host error")
	}
}

type errorRoundTripper struct{}

func (errorRoundTripper) RoundTrip(*http.Request) (*http.Response, error) {
	return nil, context.Canceled
}

func TestClientFetchInvalidURL(t *testing.T) {
	client := New(Config{
		PulseURL: "http://bad url",
		APIToken: "t",
		AgentID:  "agent-1",
	})
	if _, _, err := client.Fetch(context.Background()); err == nil || !strings.Contains(err.Error(), "create request") {
		t.Fatalf("expected create request error, got %v", err)
	}
}

func TestClientResolveHostIDRequestErrors(t *testing.T) {
	client := New(Config{
		PulseURL: "http://bad url",
		APIToken: "t",
		Hostname: "host",
	})
	if _, err := client.resolveHostID(context.Background()); err == nil || !strings.Contains(err.Error(), "create host lookup request") {
		t.Fatalf("expected request error, got %v", err)
	}

	client = New(Config{
		PulseURL: "http://example.com",
		APIToken: "t",
		Hostname: "host",
	})
	client.httpClient = &http.Client{Transport: errorRoundTripper{}}
	if _, err := client.resolveHostID(context.Background()); err == nil || !strings.Contains(err.Error(), "host lookup request") {
		t.Fatalf("expected transport error, got %v", err)
	}
}
