package remoteconfig

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/rs/zerolog"
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
	makeResp := func(hostID, sig string, issued, expires time.Time) Response {
		resp := Response{
			Success: true,
			HostID:  hostID,
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
		{name: "missing host metadata", resp: makeResp("", signature, issuedAt, expiresAt), wantText: "missing host metadata"},
		{name: "host mismatch", resp: makeResp("agent-2", signature, issuedAt, expiresAt), wantText: "host mismatch"},
		{name: "missing timestamps", resp: makeResp("agent-1", signature, time.Time{}, time.Time{}), wantText: "missing timestamp"},
		{name: "expired", resp: makeResp("agent-1", signature, issuedAt.Add(-10*time.Minute), issuedAt.Add(-5*time.Minute)), wantText: "expired"},
		{name: "future", resp: makeResp("agent-1", signature, issuedAt.Add(10*time.Minute), issuedAt.Add(20*time.Minute)), wantText: "future"},
		{name: "invalid signature", resp: makeResp("agent-1", "nope", issuedAt, expiresAt), wantText: "verification failed"},
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
	client := New(Config{PulseURL: "  ", InsecureSkipVerify: true})
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
	if _, _, err := client.Fetch(context.Background()); err == nil || !strings.Contains(err.Error(), "invalid remote config client configuration") {
		t.Fatalf("expected invalid configuration error, got %v", err)
	}
}

func TestClientResolveHostIDRequestErrors(t *testing.T) {
	client := New(Config{
		PulseURL: "http://bad url",
		APIToken: "t",
		Hostname: "host",
	})
	if _, err := client.resolveHostID(context.Background()); err == nil || !strings.Contains(err.Error(), "invalid remote config client configuration") {
		t.Fatalf("expected invalid configuration error, got %v", err)
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

func TestClientFetchConfigValidation(t *testing.T) {
	tests := []struct {
		name     string
		cfg      Config
		wantText string
	}{
		{
			name: "invalid pulse URL scheme",
			cfg: Config{
				PulseURL: "ftp://example.com",
				APIToken: "token",
				AgentID:  "agent-1",
			},
			wantText: "invalid remote config client configuration",
		},
		{
			name: "missing API token",
			cfg: Config{
				PulseURL: "https://example.com",
				APIToken: "   ",
				AgentID:  "agent-1",
			},
			wantText: "API token is required",
		},
		{
			name: "missing agent ID",
			cfg: Config{
				PulseURL: "https://example.com",
				APIToken: "token",
				AgentID:  " ",
			},
			wantText: "agent ID is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := New(tt.cfg)
			_, _, err := client.Fetch(context.Background())
			if err == nil || !strings.Contains(err.Error(), tt.wantText) {
				t.Fatalf("expected error containing %q, got %v", tt.wantText, err)
			}
		})
	}
}

func TestClientResolveHostIDEscapesHostnameQuery(t *testing.T) {
	const hostname = " host with spaces/and?chars "
	var gotRawQuery string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotRawQuery = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"success":true,"host":{"id":"host-123"}}`))
	}))
	defer ts.Close()

	client := New(Config{
		PulseURL: ts.URL,
		APIToken: "token",
		Hostname: hostname,
	})
	got, err := client.resolveHostID(context.Background())
	if err != nil {
		t.Fatalf("resolveHostID error: %v", err)
	}
	if got != "host-123" {
		t.Fatalf("expected host-123, got %q", got)
	}
	if gotRawQuery != "hostname=host+with+spaces%2Fand%3Fchars" {
		t.Fatalf("unexpected hostname query encoding: %q", gotRawQuery)
	}
}

func TestClientFetchEscapesAgentIDPath(t *testing.T) {
	var gotEscapedPath string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotEscapedPath = r.URL.EscapedPath()
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"success":true,"hostId":"agent/1","config":{"settings":{"mode":"ok"}}}`))
	}))
	defer ts.Close()

	client := New(Config{
		PulseURL: ts.URL,
		APIToken: "token",
		AgentID:  "agent/1",
	})
	settings, _, err := client.Fetch(context.Background())
	if err != nil {
		t.Fatalf("Fetch error: %v", err)
	}
	if !strings.Contains(gotEscapedPath, "agent%2F1") {
		t.Fatalf("expected escaped agent ID in path, got %q", gotEscapedPath)
	}
	if settings["mode"] != "ok" {
		t.Fatalf("unexpected settings: %#v", settings)
	}
}

func TestClientResolveHostIDPreventsQueryInjection(t *testing.T) {
	const hostname = "known&admin=true"
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/agents/host/lookup" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if got := r.URL.Query().Get("hostname"); got != hostname {
			t.Fatalf("expected hostname %q, got %q", hostname, got)
		}
		if got := r.URL.Query().Get("admin"); got != "" {
			t.Fatalf("expected no injected admin query, got %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":false}`))
	}))
	defer ts.Close()

	client := New(Config{PulseURL: ts.URL, APIToken: "t", Hostname: hostname})
	if got, err := client.resolveHostID(context.Background()); err != nil || got != "" {
		t.Fatalf("expected empty host ID, got %q err=%v", got, err)
	}
}

func TestClientFetchLogsStructuredContextWhenSignatureMissing(t *testing.T) {
	var logs bytes.Buffer
	logger := zerolog.New(&logs)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
			"success": true,
			"hostId": "agent-1",
			"config": {
				"commandsEnabled": true,
				"settings": {
					"interval": "1m"
				}
			}
		}`))
	}))
	defer ts.Close()

	client := New(Config{
		PulseURL: ts.URL,
		APIToken: "token-123",
		AgentID:  "agent-1",
		Logger:   logger,
	})

	_, _, err := client.Fetch(context.Background())
	if err != nil {
		t.Fatalf("Fetch failed: %v", err)
	}

	entry := decodeLastLogEntry(t, &logs)
	if entry["component"] != "remote_config_client" {
		t.Fatalf("expected component remote_config_client, got %#v", entry["component"])
	}
	if entry["action"] != "missing_signature_skip_verification" {
		t.Fatalf("expected action missing_signature_skip_verification, got %#v", entry["action"])
	}
	if entry["host_id"] != "agent-1" {
		t.Fatalf("expected host_id agent-1, got %#v", entry["host_id"])
	}
}

func TestClientResolveHostIDLogsStructuredContextOnStatusError(t *testing.T) {
	var logs bytes.Buffer
	logger := zerolog.New(&logs)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	client := New(Config{
		PulseURL: ts.URL,
		APIToken: "token",
		Hostname: "known",
		Logger:   logger,
	})

	if _, err := client.resolveHostID(context.Background()); err == nil {
		t.Fatal("expected error for host lookup server failure")
	}

	entry := decodeLastLogEntry(t, &logs)
	if entry["component"] != "remote_config_client" {
		t.Fatalf("expected component remote_config_client, got %#v", entry["component"])
	}
	if entry["action"] != "host_lookup_non_success_status" {
		t.Fatalf("expected action host_lookup_non_success_status, got %#v", entry["action"])
	}
	if entry["status_code"] != float64(http.StatusInternalServerError) {
		t.Fatalf("expected status_code %d, got %#v", http.StatusInternalServerError, entry["status_code"])
	}
}

func decodeLastLogEntry(t *testing.T, logs *bytes.Buffer) map[string]interface{} {
	t.Helper()

	raw := strings.TrimSpace(logs.String())
	if raw == "" {
		t.Fatal("expected structured log entry, got none")
	}

	lines := strings.Split(raw, "\n")
	last := strings.TrimSpace(lines[len(lines)-1])
	if last == "" {
		t.Fatal("expected non-empty structured log line")
	}

	var entry map[string]interface{}
	if err := json.Unmarshal([]byte(last), &entry); err != nil {
		t.Fatalf("failed to decode structured log entry %q: %v", last, err)
	}

	return entry
}
