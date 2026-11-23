package pulsecli

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentcapabilities"
	"github.com/rcourtman/pulse-go-rewrite/pkg/pulsecli/authstore"
	"github.com/spf13/cobra"
)

type memSecretStore struct {
	backend   string
	available bool
	tokens    map[string]string
}

func newMemSecretStore(backend string, available bool) *memSecretStore {
	return &memSecretStore{
		backend:   backend,
		available: available,
		tokens:    map[string]string{},
	}
}

func (m *memSecretStore) Backend() string { return m.backend }
func (m *memSecretStore) Available() bool { return m.available }
func (m *memSecretStore) Set(profile, token string) error {
	if !m.available {
		return authstore.ErrKeychainUnavailable
	}
	m.tokens[profile] = token
	return nil
}
func (m *memSecretStore) Get(profile string) (string, error) {
	tok, ok := m.tokens[profile]
	if !ok {
		return "", authstore.ErrSecretNotFound
	}
	return tok, nil
}
func (m *memSecretStore) Delete(profile string) error {
	delete(m.tokens, profile)
	return nil
}

func newTestAuthStore(t *testing.T, kc *memSecretStore) *authstore.Store {
	t.Helper()
	store, err := authstore.NewStore(authstore.StoreOptions{
		ConfigDir: t.TempDir(),
		Keychain:  kc,
		Now:       func() time.Time { return time.Date(2026, 11, 23, 12, 0, 0, 0, time.UTC) },
	})
	if err != nil {
		t.Fatalf("authstore.NewStore() error = %v", err)
	}
	return store
}

func TestResolveTargetPrecedence(t *testing.T) {
	kc := newMemSecretStore(authstore.BackendKeychain, true)
	store := newTestAuthStore(t, kc)
	if _, err := store.SaveProfile("default", "https://default.example.com", "tok-default", "read-only"); err != nil {
		t.Fatalf("SaveProfile(default) error = %v", err)
	}
	if _, err := store.SaveProfile("staging", "https://staging.example.com", "tok-staging", "propose"); err != nil {
		t.Fatalf("SaveProfile(staging) error = %v", err)
	}

	// 1. Saved default profile when env & flags are empty.
	res, err := ResolveTargetDetailed(TargetResolverDeps{
		Store:  store,
		Getenv: func(string) string { return "" },
	}, "", "")
	if err != nil {
		t.Fatalf("ResolveTargetDetailed(default) error = %v", err)
	}
	if res.BaseURL != "https://default.example.com" || res.Token != "tok-default" || res.URLSource != "profile" || res.TokenSource != "profile" {
		t.Fatalf("default resolution = %+v", res)
	}

	// 2. Named profile beats default profile.
	res, err = ResolveTargetDetailed(TargetResolverDeps{
		Store:  store,
		Getenv: func(string) string { return "" },
	}, "", "staging")
	if err != nil {
		t.Fatalf("ResolveTargetDetailed(staging) error = %v", err)
	}
	if res.BaseURL != "https://staging.example.com" || res.Token != "tok-staging" || res.ProfileName != "staging" {
		t.Fatalf("staging resolution = %+v", res)
	}

	// 3. --api-url flag beats saved profile URL while keeping profile token.
	res, err = ResolveTargetDetailed(TargetResolverDeps{
		Store:  store,
		Getenv: func(string) string { return "" },
	}, "https://flag.example.com/", "staging")
	if err != nil {
		t.Fatalf("ResolveTargetDetailed(flag) error = %v", err)
	}
	if res.BaseURL != "https://flag.example.com" || res.URLSource != "flag" || res.Token != "tok-staging" || res.TokenSource != "profile" {
		t.Fatalf("flag resolution = %+v", res)
	}

	// 4. Env vars beat both --api-url flag and saved profile.
	envMap := map[string]string{
		"PULSE_API_URL":   "https://env.example.com/",
		"PULSE_API_TOKEN": "tok-env",
	}
	res, err = ResolveTargetDetailed(TargetResolverDeps{
		Store:  store,
		Getenv: func(k string) string { return envMap[k] },
	}, "https://flag.example.com", "")
	if err != nil {
		t.Fatalf("ResolveTargetDetailed(env) error = %v", err)
	}
	if res.BaseURL != "https://env.example.com" || res.Token != "tok-env" || res.URLSource != "env" || res.TokenSource != "env" {
		t.Fatalf("env resolution = %+v", res)
	}

	// 5. Explicit missing profile fails cleanly.
	if _, err := ResolveTargetDetailed(TargetResolverDeps{
		Store:  store,
		Getenv: func(string) string { return "" },
	}, "", "nonexistent"); err == nil {
		t.Fatal("expected error for nonexistent profile, got nil")
	}
}

func TestAuthLoginRejectsTokenFlagOnArgv(t *testing.T) {
	kc := newMemSecretStore(authstore.BackendKeychain, true)
	store := newTestAuthStore(t, kc)
	cmd := NewAuthCommand(&AuthDeps{Store: store})
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	cmd.SetArgs([]string{"login", "https://pulse.example.com", "--token", "leaked-on-argv"})

	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "unknown flag: --token") {
		t.Fatalf("expected unknown --token flag error, got %v", err)
	}
}

func TestAuthLoginStdinValidatesAndSavesProfile(t *testing.T) {
	const testToken = "pulse_tok_valid_read_only"
	var sawSurfaceManifest, sawSurfaceFleet, sawTokenHeader string

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case agentcapabilities.AgentCapabilitiesPath:
			sawSurfaceManifest = r.Header.Get(agentcapabilities.AgentSurfaceHeader)
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"version": "v1",
				"capabilities": []map[string]any{
					{"name": "get_fleet_context", "method": "GET", "path": "/api/agent/fleet-context"},
				},
			})
		case agentcapabilities.FleetContextCapabilityPath:
			sawSurfaceFleet = r.Header.Get(agentcapabilities.AgentSurfaceHeader)
			sawTokenHeader = r.Header.Get(agentcapabilities.AgentAPITokenHeader)
			if sawTokenHeader != testToken {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"resources": []map[string]any{
					{"id": "pve-1"},
					{"id": "lxc-101"},
				},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()

	kc := newMemSecretStore(authstore.BackendKeychain, true)
	store := newTestAuthStore(t, kc)

	var stdout, stderr bytes.Buffer
	cmd := NewAuthCommand(&AuthDeps{
		HTTPClient: ts.Client(),
		Store:      store,
	})
	cmd.SetIn(strings.NewReader(testToken + "\n"))
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"login", ts.URL, "--token-stdin", "--profile", "homelab", "--scope", "propose"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("auth login error = %v (stderr=%s)", err, stderr.String())
	}

	if sawSurfaceManifest != pulseCLIAgentSurface || sawSurfaceFleet != pulseCLIAgentSurface {
		t.Fatalf("X-Pulse-Agent-Surface headers = (%q, %q), want %q", sawSurfaceManifest, sawSurfaceFleet, pulseCLIAgentSurface)
	}
	if sawTokenHeader != testToken {
		t.Fatalf("X-API-Token header = %q, want %q", sawTokenHeader, testToken)
	}

	if strings.Contains(stdout.String(), testToken) || strings.Contains(stderr.String(), testToken) {
		t.Fatalf("auth login output must never echo raw token:\nstdout=%s\nstderr=%s", stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), "2 visible resources") {
		t.Fatalf("auth login summary missing resource count:\n%s", stdout.String())
	}

	// Verify stored token and clean profiles.json.
	name, prof, tok, err := store.GetToken("homelab")
	if err != nil {
		t.Fatalf("store.GetToken(homelab) error = %v", err)
	}
	if name != "homelab" || tok != testToken || prof.Scope != "propose" || prof.TokenBackend != authstore.BackendKeychain {
		t.Fatalf("stored profile/token mismatch: name=%q prof=%+v tok=%q", name, prof, tok)
	}
	rawProfiles, err := os.ReadFile(store.ProfilesPath())
	if err != nil {
		t.Fatalf("ReadFile(profiles.json) error = %v", err)
	}
	if strings.Contains(string(rawProfiles), testToken) {
		t.Fatalf("profiles.json leaked raw token")
	}
}

func TestAuthLoginInteractivePromptAndUnauthorizedRejection(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case agentcapabilities.AgentCapabilitiesPath:
			_ = json.NewEncoder(w).Encode(map[string]any{
				"version":      "v1",
				"capabilities": []map[string]any{{"name": "get_fleet_context"}},
			})
		case agentcapabilities.FleetContextCapabilityPath:
			if r.Header.Get(agentcapabilities.AgentAPITokenHeader) != "good-tok" {
				http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"resources": []any{}})
		default:
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()

	kc := newMemSecretStore(authstore.BackendKeychain, true)
	store := newTestAuthStore(t, kc)

	// Bad token via interactive hidden prompt must fail and NOT save profile.
	cmd := NewAuthCommand(&AuthDeps{
		HTTPClient:   ts.Client(),
		Store:        store,
		IsTerminal:   func(int) bool { return true },
		ReadPassword: func(int) ([]byte, error) { return []byte("bad-tok"), nil },
	})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"login", ts.URL})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "401") {
		t.Fatalf("expected 401 auth error, got %v", err)
	}
	if _, _, err := store.GetProfile(""); !errors.Is(err, authstore.ErrProfileNotFound) {
		t.Fatalf("failed login must not save profile, got err = %v", err)
	}

	// Good token via interactive hidden prompt succeeds.
	cmd2 := NewAuthCommand(&AuthDeps{
		HTTPClient:   ts.Client(),
		Store:        store,
		IsTerminal:   func(int) bool { return true },
		ReadPassword: func(int) ([]byte, error) { return []byte("good-tok"), nil },
	})
	var out bytes.Buffer
	cmd2.SetOut(&out)
	cmd2.SetErr(&bytes.Buffer{})
	cmd2.SetArgs([]string{"login", ts.URL})
	if err := cmd2.Execute(); err != nil {
		t.Fatalf("interactive auth login error = %v", err)
	}
	if _, _, tok, err := store.GetToken("default"); err != nil || tok != "good-tok" {
		t.Fatalf("stored interactive token = (%q, %v), want (good-tok, nil)", tok, err)
	}
}

func TestAuthStatusLogoutAndInheritedRootFlags(t *testing.T) {
	kc := newMemSecretStore(authstore.BackendKeychain, true)
	store := newTestAuthStore(t, kc)
	if _, err := store.SaveProfile("default", "https://prod.example.com", "secret-prod-token", "read-only"); err != nil {
		t.Fatalf("SaveProfile(default) error = %v", err)
	}
	if _, err := store.SaveProfile("staging", "https://staging.example.com", "secret-staging-token", "propose"); err != nil {
		t.Fatalf("SaveProfile(staging) error = %v", err)
	}

	root := &cobra.Command{Use: "pulse"}
	root.PersistentFlags().String("profile", "", "profile")
	root.PersistentFlags().Bool("json", false, "json")
	root.AddCommand(NewAuthCommand(&AuthDeps{
		Store:  store,
		Getenv: func(string) string { return "" },
	}))

	var statusOut bytes.Buffer
	root.SetOut(&statusOut)
	root.SetErr(&bytes.Buffer{})
	root.SetArgs([]string{"--profile", "staging", "--json", "auth", "status"})
	if err := root.Execute(); err != nil {
		t.Fatalf("auth status error = %v", err)
	}
	if strings.Contains(statusOut.String(), "secret-prod-token") || strings.Contains(statusOut.String(), "secret-staging-token") {
		t.Fatalf("auth status leaked secret token:\n%s", statusOut.String())
	}

	var parsed authStatusOutput
	if err := json.Unmarshal(statusOut.Bytes(), &parsed); err != nil {
		t.Fatalf("decode auth status JSON error = %v\n%s", err, statusOut.String())
	}
	if parsed.ActiveProfile != "staging" || parsed.ActiveURL != "https://staging.example.com" || len(parsed.Profiles) != 2 {
		t.Fatalf("unexpected auth status output: %+v", parsed)
	}

	// Logout staging.
	var logoutOut bytes.Buffer
	logoutCmd := NewAuthCommand(&AuthDeps{Store: store})
	logoutCmd.SetOut(&logoutOut)
	logoutCmd.SetErr(&bytes.Buffer{})
	logoutCmd.SetArgs([]string{"logout", "--profile", "staging"})
	if err := logoutCmd.Execute(); err != nil {
		t.Fatalf("auth logout error = %v", err)
	}
	if _, _, err := store.GetProfile("staging"); !errors.Is(err, authstore.ErrProfileNotFound) {
		t.Fatalf("expected staging profile deleted, got err = %v", err)
	}
}
