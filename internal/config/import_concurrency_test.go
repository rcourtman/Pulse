package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/notifications"
	"github.com/stretchr/testify/require"
)

type pausedConfigImport struct {
	done    chan struct{}
	err     error
	release func()
}

// Hold the existing metadata publication gate, not an artificial persistence
// success. All archive sections have been staged when the SSO file appears.
func pauseConfigImport(t *testing.T, cp *ConfigPersistence, archive string) *pausedConfigImport {
	t.Helper()
	store := cp.GetGuestMetadataStore()
	store.writeMu.Lock()
	var once sync.Once
	attempt := &pausedConfigImport{done: make(chan struct{}), release: func() { once.Do(store.writeMu.Unlock) }}
	go func() {
		attempt.err = cp.ImportConfig(archive, "synthetic-import-validation-passphrase")
		close(attempt.done)
	}()
	t.Cleanup(func() {
		attempt.release()
		select {
		case <-attempt.done:
		case <-time.After(10 * time.Second):
			t.Error("import did not join after its metadata gate was released")
		}
	})
	require.Eventually(t, func() bool {
		paths, err := filepath.Glob(filepath.Join(cp.configDir, ".import-staging-*", filepath.Base(cp.ssoFile)+".tmp-*"))
		return err == nil && len(paths) == 1
	}, 10*time.Second, time.Millisecond, "import did not stage its final configuration section")
	return attempt
}

func (a *pausedConfigImport) finish(t *testing.T, shouldFail bool) {
	t.Helper()
	a.release()
	select {
	case <-a.done:
		if shouldFail {
			require.Error(t, a.err)
		} else {
			require.NoError(t, a.err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("import did not complete after releasing metadata publication")
	}
}

func requireNoConfigImportArtifacts(t *testing.T, cp *ConfigPersistence) {
	t.Helper()
	entries, err := os.ReadDir(cp.configDir)
	require.NoError(t, err)
	for _, entry := range entries {
		require.NotContains(t, entry.Name(), ".import-", "completed import retained its staged state")
	}
}

func TestImportConfigDoesNotAbsorbConcurrentSaves(t *testing.T) {
	for _, fails := range []bool{false, true} {
		result := "commit"
		if fails {
			result = "rollback"
		}
		for _, section := range []string{"tokens", "email", "sso"} {
			t.Run(result+"/"+section, func(t *testing.T) {
				cp := seedImportValidationState(t)
				if fails {
					// system.json is late in sorted publication order. Earlier
					// files are actually applied, then restored by the transaction.
					require.NoError(t, os.Remove(cp.systemFile))
					require.NoError(t, os.Mkdir(cp.systemFile, 0700))
				}
				archive := metadataImportArchive(t, map[string]*GuestMetadata{"imported:202": {Description: "Imported metadata"}})
				attempt := pauseConfigImport(t, cp, archive)
				before, err := os.ReadFile(cp.apiTokensFile)
				require.NoError(t, err)
				storeBefore := cp.GetGuestMetadataStore().GetAll()
				token, err := NewAPITokenRecord("synthetic-concurrent-reader", "Concurrent reader", []string{ScopeMonitoringRead})
				require.NoError(t, err)
				token.OrgID = "client-a"
				settings := NewSSOConfig()
				settings.AllowMultipleProviders = true
				settings.DefaultProviderID = "concurrent-idp"
				settings.Providers = []SSOProvider{{
					ID: "concurrent-idp", Name: "Saved sign-in", Type: SSOProviderTypeOIDC,
					OIDC: &OIDCProviderConfig{IssuerURL: "https://idp.invalid", ClientID: "saved-client", ClientSecret: "synthetic-concurrent-idp-secret"},
				}}
				started, saved := make(chan struct{}), make(chan struct{})
				var saveErr error
				go func() {
					close(started)
					switch section {
					case "tokens":
						saveErr = cp.SaveAPITokens([]APITokenRecord{*token})
					case "email":
						saveErr = cp.SaveEmailConfig(notifications.EmailConfig{SMTPHost: "concurrent.invalid", Password: "synthetic-concurrent-mail-secret"})
					case "sso":
						saveErr = cp.SaveSSOConfig(settings)
					}
					close(saved)
				}()
				t.Cleanup(func() {
					attempt.release()
					select {
					case <-saved:
					case <-time.After(10 * time.Second):
						t.Error("concurrent saver did not join")
					}
				})
				<-started
				select {
				case <-saved:
					t.Error("ordinary save acknowledged before the import committed or rolled back")
				case <-time.After(100 * time.Millisecond):
					// Bounded negative observation; the joined positive checks
					// below establish actual publication, not just blocking.
				}
				attempt.finish(t, fails)
				select {
				case <-saved:
					require.NoError(t, saveErr)
				case <-time.After(10 * time.Second):
					t.Fatal("ordinary save remained blocked after import completion")
				}
				switch section {
				case "tokens":
					loaded, err := cp.LoadAPITokens()
					require.NoError(t, err)
					require.Equal(t, []APITokenRecord{*token}, loaded, "acknowledged token save was lost to the import")
					cfg := &Config{APITokens: loaded}
					cfg.SortAPITokens()
					require.True(t, cfg.IsValidAPIToken("synthetic-concurrent-reader"))
					require.True(t, loaded[0].CanAccessOrg("client-a"))
					require.False(t, loaded[0].CanAccessOrg("client-b"))
					require.False(t, loaded[0].HasScope(ScopeSettingsWrite))
					backup, err := os.ReadFile(cp.apiTokensFile + ".backup")
					require.NoError(t, err)
					if fails {
						require.Equal(t, before, backup, "ordinary save must retain the restored previous bytes")
					} else {
						plain, err := cp.crypto.Decrypt(backup)
						require.NoError(t, err)
						var previous []APITokenRecord
						require.NoError(t, json.Unmarshal(plain, &previous))
						require.Len(t, previous, 1)
						require.Equal(t, "imported-token", previous[0].ID, "ordinary save backed up pre-import rather than committed credentials")
					}
				case "email":
					loaded, err := cp.LoadEmailConfig()
					require.NoError(t, err)
					require.Equal(t, "concurrent.invalid", loaded.SMTPHost, "acknowledged mail save was lost to the import")
					require.Equal(t, "synthetic-concurrent-mail-secret", loaded.Password)
				case "sso":
					loaded, err := cp.LoadSSOConfig()
					require.NoError(t, err)
					require.Equal(t, settings, loaded, "acknowledged sign-in save was lost to the import")
				}
				if fails {
					require.Equal(t, storeBefore, cp.GetGuestMetadataStore().GetAll(), "failed import changed live metadata")
					if section != "tokens" {
						after, err := os.ReadFile(cp.apiTokensFile)
						require.NoError(t, err)
						require.Equal(t, before, after, "failed import changed unrelated saved credentials")
					}
				} else {
					require.Equal(t, "Imported metadata", cp.GetGuestMetadataStore().Get("imported:202").Description)
				}
				requireNoConfigImportArtifacts(t, cp)
			})
		}
	}
}

func TestImportConfigSerializesConcurrentImports(t *testing.T) {
	cp := seedImportValidationState(t)
	first := metadataImportArchive(t, map[string]*GuestMetadata{"first:101": {Description: "First import"}})
	second := metadataImportArchive(t, map[string]*GuestMetadata{"second:202": {Description: "Second import"}})
	attempt := pauseConfigImport(t, cp, first)
	done := make(chan struct{})
	var secondErr error
	go func() {
		secondErr = cp.ImportConfig(second, "synthetic-import-validation-passphrase")
		close(done)
	}()
	t.Cleanup(func() {
		attempt.release()
		select {
		case <-done:
		case <-time.After(10 * time.Second):
			t.Error("second import did not join")
		}
	})
	select {
	case <-done:
		t.Fatalf("second import completed ahead of the blocked first import: %v", secondErr)
	case <-time.After(250 * time.Millisecond):
	}
	paths, err := filepath.Glob(filepath.Join(cp.configDir, ".import-staging-*"))
	require.NoError(t, err)
	require.Len(t, paths, 1, "concurrent imports shared or overlapped active transaction state")
	attempt.finish(t, false)
	select {
	case <-done:
		require.NoError(t, secondErr)
	case <-time.After(10 * time.Second):
		t.Fatal("second import did not complete after the first")
	}
	require.Nil(t, cp.GetGuestMetadataStore().Get("first:101"))
	require.Equal(t, "Second import", cp.GetGuestMetadataStore().Get("second:202").Description)
	requireNoConfigImportArtifacts(t, cp)
}

func TestImportConfigDoesNotLockOtherScopes(t *testing.T) {
	cp := seedImportValidationState(t)
	other := seedImportValidationState(t)
	archive := metadataImportArchive(t, map[string]*GuestMetadata{"scope:101": {Description: "Scope-local import"}})
	attempt := pauseConfigImport(t, cp, archive)
	done := make(chan struct{})
	var otherErr error
	go func() {
		otherErr = other.ImportConfig(archive, "synthetic-import-validation-passphrase")
		close(done)
	}()
	t.Cleanup(func() {
		attempt.release()
		select {
		case <-done:
		case <-time.After(10 * time.Second):
			t.Error("other scope's import did not join")
		}
	})
	select {
	case <-done:
		require.NoError(t, otherErr)
	case <-time.After(10 * time.Second):
		t.Fatal("one scope's blocked import stopped an independent scope")
	}
	require.Equal(t, "Scope-local import", other.GetGuestMetadataStore().Get("scope:101").Description)
	require.Nil(t, cp.GetGuestMetadataStore().Get("scope:101"), "uncommitted metadata became live")
	attempt.finish(t, false)
	requireNoConfigImportArtifacts(t, cp)
	requireNoConfigImportArtifacts(t, other)
}
