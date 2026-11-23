package authstore

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

type fakeSecretStore struct {
	backend   string
	available bool
	setErr    error
	getErr    error
	tokens    map[string]string
}

func newFakeSecretStore(backend string, available bool) *fakeSecretStore {
	return &fakeSecretStore{
		backend:   backend,
		available: available,
		tokens:    map[string]string{},
	}
}

func (f *fakeSecretStore) Backend() string { return f.backend }
func (f *fakeSecretStore) Available() bool { return f.available }

func (f *fakeSecretStore) Set(profile, token string) error {
	if !f.available {
		return ErrKeychainUnavailable
	}
	if f.setErr != nil {
		return f.setErr
	}
	f.tokens[profile] = token
	return nil
}

func (f *fakeSecretStore) Get(profile string) (string, error) {
	if f.getErr != nil {
		return "", f.getErr
	}
	tok, ok := f.tokens[profile]
	if !ok {
		return "", ErrSecretNotFound
	}
	return tok, nil
}

func (f *fakeSecretStore) Delete(profile string) error {
	delete(f.tokens, profile)
	return nil
}

func TestStoreUsesKeychainAndNeverWritesTokenToProfilesJSON(t *testing.T) {
	dir := t.TempDir()
	kc := newFakeSecretStore(BackendKeychain, true)
	fixedNow := time.Date(2026, 11, 23, 12, 0, 0, 0, time.UTC)

	store, err := NewStore(StoreOptions{
		ConfigDir: dir,
		Keychain:  kc,
		Now:       func() time.Time { return fixedNow },
	})
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}

	const secretToken = "pulse_tok_super_secret_12345"
	profile, err := store.SaveProfile("", "https://pulse.example.com/", secretToken, "read-only")
	if err != nil {
		t.Fatalf("SaveProfile() error = %v", err)
	}
	if profile.URL != "https://pulse.example.com" {
		t.Fatalf("profile.URL = %q, want trailing slash trimmed", profile.URL)
	}
	if profile.TokenBackend != BackendKeychain {
		t.Fatalf("profile.TokenBackend = %q, want %q", profile.TokenBackend, BackendKeychain)
	}
	if !profile.UpdatedAt.Equal(fixedNow) {
		t.Fatalf("profile.UpdatedAt = %v, want %v", profile.UpdatedAt, fixedNow)
	}

	// profiles.json must never contain the raw token and must be 0600 on Unix.
	rawProfiles, err := os.ReadFile(store.ProfilesPath())
	if err != nil {
		t.Fatalf("ReadFile(profiles.json) error = %v", err)
	}
	if strings.Contains(string(rawProfiles), secretToken) {
		t.Fatalf("profiles.json leaked raw token:\n%s", string(rawProfiles))
	}
	if runtime.GOOS != "windows" {
		info, err := os.Stat(store.ProfilesPath())
		if err != nil {
			t.Fatalf("Stat(profiles.json) error = %v", err)
		}
		if got := info.Mode().Perm(); got != 0o600 {
			t.Fatalf("profiles.json perms = %o, want 600", got)
		}
	}

	// Fallback credentials file should not even exist when keychain succeeded.
	if _, err := os.Stat(store.CredentialsPath()); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("credentials.json should not exist when keychain succeeded, stat err = %v", err)
	}

	name, loaded, token, err := store.GetToken("")
	if err != nil {
		t.Fatalf("GetToken() error = %v", err)
	}
	if name != DefaultProfileName {
		t.Fatalf("resolved profile = %q, want %q", name, DefaultProfileName)
	}
	if loaded.URL != "https://pulse.example.com" {
		t.Fatalf("loaded URL = %q", loaded.URL)
	}
	if token != secretToken {
		t.Fatalf("loaded token = %q, want %q", token, secretToken)
	}
}

func TestStoreFallsBackTo0600FileWhenKeychainUnavailableOrFails(t *testing.T) {
	cases := []struct {
		name string
		kc   *fakeSecretStore
	}{
		{
			name: "keychain unavailable",
			kc:   newFakeSecretStore(BackendKeychain, false),
		},
		{
			name: "keychain locked or errored on set",
			kc: &fakeSecretStore{
				backend:   BackendSecretService,
				available: true,
				setErr:    errors.New("keyring locked"),
				tokens:    map[string]string{},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			store, err := NewStore(StoreOptions{
				ConfigDir: dir,
				Keychain:  tc.kc,
			})
			if err != nil {
				t.Fatalf("NewStore() error = %v", err)
			}

			const secretToken = "pulse_tok_headless_fallback"
			profile, err := store.SaveProfile("homelab", "https://pulse.lan:7655", secretToken, "propose")
			if err != nil {
				t.Fatalf("SaveProfile() error = %v", err)
			}
			if profile.TokenBackend != BackendFile {
				t.Fatalf("TokenBackend = %q, want %q", profile.TokenBackend, BackendFile)
			}

			// profiles.json still must not contain the raw token.
			rawProfiles, err := os.ReadFile(store.ProfilesPath())
			if err != nil {
				t.Fatalf("ReadFile(profiles.json) error = %v", err)
			}
			if strings.Contains(string(rawProfiles), secretToken) {
				t.Fatalf("profiles.json leaked raw token")
			}

			if runtime.GOOS != "windows" {
				info, err := os.Stat(store.CredentialsPath())
				if err != nil {
					t.Fatalf("Stat(credentials.json) error = %v", err)
				}
				if got := info.Mode().Perm(); got != 0o600 {
					t.Fatalf("credentials.json perms = %o, want 600", got)
				}
			}

			name, _, gotToken, err := store.GetToken("homelab")
			if err != nil {
				t.Fatalf("GetToken() error = %v", err)
			}
			if name != "homelab" || gotToken != secretToken {
				t.Fatalf("GetToken() = (%q, %q), want (homelab, %q)", name, gotToken, secretToken)
			}
		})
	}
}

func TestStoreMultiProfileLifecycleAndMigrationToKeychain(t *testing.T) {
	dir := t.TempDir()
	kc := newFakeSecretStore(BackendKeychain, false)

	store, err := NewStore(StoreOptions{
		ConfigDir: dir,
		Keychain:  kc,
	})
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}

	if _, err := store.SaveProfile("default", "https://prod.example.com", "tok-prod", "read-only"); err != nil {
		t.Fatalf("SaveProfile(default) error = %v", err)
	}
	if _, err := store.SaveProfile("staging", "https://staging.example.com/pulse/", "tok-staging", "propose"); err != nil {
		t.Fatalf("SaveProfile(staging) error = %v", err)
	}

	// Enable keychain and re-save "default": stale file token must be cleared.
	kc.available = true
	prof, err := store.SaveProfile("default", "https://prod.example.com", "tok-prod-v2", "read-only")
	if err != nil {
		t.Fatalf("re-save default error = %v", err)
	}
	if prof.TokenBackend != BackendKeychain {
		t.Fatalf("re-saved backend = %q, want %q", prof.TokenBackend, BackendKeychain)
	}
	fileStore := NewFileSecretStore(store.CredentialsPath())
	if _, err := fileStore.Get("default"); !errors.Is(err, ErrSecretNotFound) {
		t.Fatalf("expected stale default token removed from credentials.json, got err = %v", err)
	}
	if tok, err := fileStore.Get("staging"); err != nil || tok != "tok-staging" {
		t.Fatalf("staging file token = (%q, %v), want (tok-staging, nil)", tok, err)
	}

	// Switch default to staging.
	if err := store.SetDefaultProfile("staging"); err != nil {
		t.Fatalf("SetDefaultProfile(staging) error = %v", err)
	}
	defName, profiles, err := store.ListProfiles()
	if err != nil {
		t.Fatalf("ListProfiles() error = %v", err)
	}
	if defName != "staging" || len(profiles) != 2 {
		t.Fatalf("ListProfiles() = (%q, len=%d), want (staging, 2)", defName, len(profiles))
	}

	// Deleting staging promotes "default" back as default and cleans up empty credentials.json.
	if err := store.DeleteProfile("staging"); err != nil {
		t.Fatalf("DeleteProfile(staging) error = %v", err)
	}
	if _, err := os.Stat(store.CredentialsPath()); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected empty credentials.json to be removed, stat err = %v", err)
	}
	defName, _, err = store.ListProfiles()
	if err != nil {
		t.Fatalf("ListProfiles() error = %v", err)
	}
	if defName != "default" {
		t.Fatalf("default after deleting staging = %q, want default", defName)
	}

	// Deleting unknown profile returns ErrProfileNotFound.
	if err := store.DeleteProfile("missing"); !errors.Is(err, ErrProfileNotFound) {
		t.Fatalf("DeleteProfile(missing) = %v, want ErrProfileNotFound", err)
	}
}

func TestNormalizeProfileNameAndURL(t *testing.T) {
	if got, err := NormalizeProfileName(""); err != nil || got != DefaultProfileName {
		t.Fatalf("NormalizeProfileName('') = (%q, %v), want (%q, nil)", got, err, DefaultProfileName)
	}
	for _, valid := range []string{"default", "prod-1", "lab_01.home"} {
		if _, err := NormalizeProfileName(valid); err != nil {
			t.Fatalf("NormalizeProfileName(%q) unexpected error: %v", valid, err)
		}
	}
	for _, invalid := range []string{"../evil", "-leading", "has space", "a/b", strings.Repeat("a", 65)} {
		if _, err := NormalizeProfileName(invalid); err == nil {
			t.Fatalf("NormalizeProfileName(%q) expected error, got nil", invalid)
		}
	}

	if got, err := NormalizeURL("HTTPS://Pulse.Example.com/sub/"); err != nil || got != "https://Pulse.Example.com/sub" {
		t.Fatalf("NormalizeURL() = (%q, %v)", got, err)
	}
	for _, bad := range []string{"", "ftp://example.com", "https://user:pass@example.com", "https://example.com?x=1", "https://example.com#frag", "example.com"} {
		if _, err := NormalizeURL(bad); err == nil {
			t.Fatalf("NormalizeURL(%q) expected error, got nil", bad)
		}
	}
}

func TestDefaultConfigDirRespectsEnvOverrides(t *testing.T) {
	custom := filepath.Join(t.TempDir(), "custom-pulse")
	t.Setenv("PULSE_CONFIG_DIR", custom)
	got, err := DefaultConfigDir()
	if err != nil {
		t.Fatalf("DefaultConfigDir() error = %v", err)
	}
	if got != custom {
		t.Fatalf("DefaultConfigDir() = %q, want %q", got, custom)
	}

	t.Setenv("PULSE_CONFIG_DIR", "")
	xdg := filepath.Join(t.TempDir(), "xdg")
	t.Setenv("XDG_CONFIG_HOME", xdg)
	got, err = DefaultConfigDir()
	if err != nil {
		t.Fatalf("DefaultConfigDir() error = %v", err)
	}
	want := filepath.Join(xdg, "pulse")
	if got != want {
		t.Fatalf("DefaultConfigDir() = %q, want %q", got, want)
	}
}
