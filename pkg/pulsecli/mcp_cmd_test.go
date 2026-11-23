package pulsecli

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentcapabilities"
	"github.com/rcourtman/pulse-go-rewrite/pkg/pulsecli/authstore"
)

type cliTestSecretStore struct {
	backend   string
	available bool
	secrets   map[string]string
}

func newCLITestSecretStore(backend string) *cliTestSecretStore {
	return &cliTestSecretStore{
		backend:   backend,
		available: true,
		secrets:   map[string]string{},
	}
}

func (s *cliTestSecretStore) Backend() string { return s.backend }
func (s *cliTestSecretStore) Available() bool { return s.available }
func (s *cliTestSecretStore) Set(profile, token string) error {
	s.secrets[profile] = token
	return nil
}
func (s *cliTestSecretStore) Get(profile string) (string, error) {
	val, ok := s.secrets[profile]
	if !ok {
		return "", authstore.ErrSecretNotFound
	}
	return val, nil
}
func (s *cliTestSecretStore) Delete(profile string) error {
	delete(s.secrets, profile)
	return nil
}

func newCLITestAuthStore(t *testing.T) *authstore.Store {
	t.Helper()
	store, err := authstore.NewStore(authstore.StoreOptions{
		ConfigDir: t.TempDir(),
		Keychain:  newCLITestSecretStore(authstore.BackendKeychain),
		FileStore: newCLITestSecretStore(authstore.BackendFile),
	})
	if err != nil {
		t.Fatalf("authstore.NewStore: %v", err)
	}
	return store
}

func TestPulseMCPCommandUsesSavedAuthStoreProfileWithoutEnvToken(t *testing.T) {
	var seenSurface string
	var seenToken string
	manifest := agentcapabilities.CanonicalManifest()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case agentcapabilities.AgentCapabilitiesPath:
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(manifest)
		case agentcapabilities.FleetContextCapabilityPath:
			seenSurface = r.Header.Get(agentcapabilities.AgentSurfaceHeader)
			seenToken = r.Header.Get(agentcapabilities.AgentAPITokenHeader)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"resources":[],"generatedAt":"2026-11-23T10:00:00Z"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	store := newCLITestAuthStore(t)
	if _, err := store.SaveProfile("homelab", server.URL, "keychain-saved-token", "read-only"); err != nil {
		t.Fatalf("SaveProfile: %v", err)
	}

	stdin := strings.NewReader(
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}` + "\n" +
			`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"get_fleet_context","arguments":{}}}` + "\n",
	)
	var stdout, stderr bytes.Buffer

	root := NewClientRootCommand(CommandSpec{Use: "pulse"}, ClientDeps{
		MCP: &MCPDeps{
			HTTPClient: server.Client(),
			Store:      store,
			Getenv:     func(string) string { return "" },
			In:         stdin,
			Out:        &stdout,
			ErrOut:     &stderr,
		},
	})
	root.SetArgs([]string{"mcp", "--profile", "homelab"})

	if err := root.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("pulse mcp failed: %v\nstderr=%s", err, stderr.String())
	}
	if seenSurface != agentcapabilities.AgentSurfacePulseMCP {
		t.Fatalf("X-Pulse-Agent-Surface = %q, want %q", seenSurface, agentcapabilities.AgentSurfacePulseMCP)
	}
	if seenToken != "keychain-saved-token" {
		t.Fatalf("X-API-Token = %q, want saved profile token", seenToken)
	}

	lines := strings.Split(strings.TrimSpace(stdout.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 JSON-RPC responses, got %d:\n%s", len(lines), stdout.String())
	}
	if !strings.Contains(lines[0], `"protocolVersion"`) {
		t.Fatalf("initialize response missing protocolVersion: %s", lines[0])
	}
	if !strings.Contains(lines[1], `"resources"`) || strings.Contains(lines[1], `"isError":true`) {
		t.Fatalf("tools/call response unexpected: %s", lines[1])
	}
}

func TestPulseMCPCommandFailsCleanlyWhenNoCredentialsAvailable(t *testing.T) {
	manifest := agentcapabilities.CanonicalManifest()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == agentcapabilities.AgentCapabilitiesPath {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(manifest)
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	store := newCLITestAuthStore(t)
	var stderr bytes.Buffer
	err := RunMCPStdio(context.Background(), RunMCPStdioOptions{
		BaseURL:        server.URL,
		BaseURLSet:     true,
		Store:          store,
		Getenv:         func(string) string { return "" },
		ManifestClient: server.Client(),
		HTTPClient:     server.Client(),
		In:             strings.NewReader(""),
		Out:            &bytes.Buffer{},
		ErrOut:         &stderr,
	})
	if err == nil {
		t.Fatal("expected missing token error")
	}
	for _, want := range []string{"PULSE_API_TOKEN", "pulse auth login", "current manifest scopes:"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error %q missing %q", err.Error(), want)
		}
	}
}
