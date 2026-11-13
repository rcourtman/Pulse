package config

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/crypto"
	"github.com/rcourtman/pulse-go-rewrite/pkg/auth"
	"github.com/stretchr/testify/require"
)

// The backup deliberately differs from the primary. Backing up the primary
// before a rejected save would silently destroy this older recovery point.
type tokenBackupFixture struct {
	p                       *ConfigPersistence
	primary, backup         []byte
	primaryInfo, backupInfo os.FileInfo
	hasBackup               bool
}

func backupTestToken(t *testing.T, raw string) APITokenRecord {
	t.Helper()
	record, err := NewAPITokenRecord(raw, "Saved monitoring", []string{ScopeMonitoringRead})
	require.NoError(t, err)
	record.ID = "saved-monitor"
	record.CreatedAt = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	expires := time.Date(2099, 1, 1, 0, 0, 0, 0, time.UTC)
	record.ExpiresAt = &expires
	record.OrgID = "client-a"
	record.Metadata = map[string]string{"owner_user_id": "synthetic-owner", "bound_agent_id": "saved-agent"}
	return *record
}

func newTokenBackupFixture(t *testing.T, encrypted, hasBackup bool) *tokenBackupFixture {
	t.Helper()
	f := &tokenBackupFixture{p: NewConfigPersistence(t.TempDir()), hasBackup: hasBackup}
	if !encrypted {
		f.p.crypto = nil
	}
	if hasBackup {
		require.NoError(t, f.p.SaveAPITokens([]APITokenRecord{backupTestToken(t, "synthetic-older-bearer")}))
	}
	require.NoError(t, f.p.SaveAPITokens([]APITokenRecord{backupTestToken(t, "synthetic-current-bearer")}))
	var err error
	f.primary, err = os.ReadFile(f.p.apiTokensFile)
	require.NoError(t, err)
	f.primaryInfo, err = os.Stat(f.p.apiTokensFile)
	require.NoError(t, err)
	if hasBackup {
		f.backup, err = os.ReadFile(f.p.apiTokensFile + ".backup")
		require.NoError(t, err)
		f.backupInfo, err = os.Stat(f.p.apiTokensFile + ".backup")
		require.NoError(t, err)
		require.NotEqual(t, f.primary, f.backup)
	}
	return f
}

func (f *tokenBackupFixture) requireUnchanged(t *testing.T) {
	t.Helper()
	requireAPITokenFileUnchanged(t, f.p.apiTokensFile, f.primary, f.primaryInfo)
	f.requireBackupUnchanged(t)
	temps, err := filepath.Glob(filepath.Join(f.p.configDir, ".api_tokens.json.*.tmp"))
	require.NoError(t, err)
	require.Empty(t, temps, "failed save left an owned temporary file")
}

func (f *tokenBackupFixture) requireBackupUnchanged(t *testing.T) {
	t.Helper()
	if f.hasBackup {
		requireAPITokenFileUnchanged(t, f.p.apiTokensFile+".backup", f.backup, f.backupInfo)
	} else {
		_, err := os.Stat(f.p.apiTokensFile + ".backup")
		require.True(t, os.IsNotExist(err), "failed save created a recovery file")
	}
}

func requireBackupTestBearer(t *testing.T, p *ConfigPersistence, raw string) {
	t.Helper()
	tokens, err := p.LoadAPITokens()
	require.NoError(t, err)
	require.Equal(t, []APITokenRecord{backupTestToken(t, raw)}, tokens)
	cfg := &Config{APITokens: tokens}
	cfg.SortAPITokens()
	for _, candidate := range []string{"synthetic-older-bearer", "synthetic-current-bearer", "synthetic-next-bearer"} {
		require.Equal(t, candidate == raw, cfg.IsValidAPIToken(candidate), "unexpected restored credential set")
	}
	require.True(t, tokens[0].CanAccessOrg("client-a"))
	require.False(t, tokens[0].CanAccessOrg("client-b"))
	require.True(t, tokens[0].HasScope(ScopeMonitoringRead))
	require.False(t, tokens[0].HasScope(ScopeSettingsWrite))
}

func (f *tokenBackupFixture) requireRecovery(t *testing.T) {
	t.Helper()
	f.p.SetFileSystem(defaultFileSystem{})
	requireBackupTestBearer(t, f.p, "synthetic-current-bearer")
	if f.hasBackup {
		// Restore only this disposable fixture's retained backup, then consume
		// it through the real loader and bearer checks, not just a byte hash.
		require.NoError(t, os.WriteFile(f.p.apiTokensFile, f.backup, 0600))
		requireBackupTestBearer(t, f.p, "synthetic-older-bearer")
	}
}

func TestAPITokenBackupSurvivesPreparationFailure(t *testing.T) {
	for _, encrypted := range []bool{false, true} {
		encoding := "plaintext"
		failures := []string{"excess-role-scopes", "missing-role-scope", "unknown-role", "created-time", "expiry-time", "usage-time"}
		if encrypted {
			encoding = "encrypted"
			failures = append(failures, "encryption")
		}
		for _, hasBackup := range []bool{false, true} {
			backupState := "no-backup"
			if hasBackup {
				backupState = "retained-backup"
			}
			for _, failure := range failures {
				t.Run(encoding+"/"+backupState+"/"+failure, func(t *testing.T) {
					f := newTokenBackupFixture(t, encrypted, hasBackup)
					record := backupTestToken(t, "synthetic-next-bearer")
					badTime := time.Date(10000, 1, 1, 0, 0, 0, 0, time.UTC)
					wantError := "validate api token"
					switch failure {
					case "excess-role-scopes":
						record.Metadata[auth.RuntimeRoleMetadataKey] = auth.RuntimeRoleMonitoringCollector
						record.Scopes = []string{ScopeAgentReport, ScopeAgentConfigRead, ScopeSettingsWrite}
					case "missing-role-scope":
						record.Metadata[auth.RuntimeRoleMetadataKey] = auth.RuntimeRoleMonitoringCollector
						record.Scopes = []string{ScopeAgentReport}
					case "unknown-role":
						record.Metadata[auth.RuntimeRoleMetadataKey] = "synthetic-unknown-role"
					case "created-time":
						record.CreatedAt, wantError = badTime, "marshal api token records"
					case "expiry-time":
						record.ExpiresAt, wantError = &badTime, "marshal api token records"
					case "usage-time":
						record.LastUsedAt, wantError = &badTime, "marshal api token records"
					case "encryption":
						// An uninitialized manager deterministically fails encryption;
						// no host key, random source or encryption policy is modified.
						manager := f.p.crypto
						f.p.crypto = &crypto.CryptoManager{}
						defer func() { f.p.crypto = manager }()
						wantError = "encrypt api tokens"
					}
					before := record.Clone()
					err := f.p.SaveAPITokens([]APITokenRecord{record})
					require.ErrorContains(t, err, wantError)
					require.NotContains(t, err.Error(), record.Hash)
					require.Equal(t, before, record, "failed preparation mutated its caller")
					f.requireUnchanged(t)
				})
			}
		}
	}
}

// Target one pathname, so the backup itself can succeed while the primary
// fails. Global write failures would conceal the premature-backup regression.
type tokenBackupPathFaultFS struct {
	FileSystem
	target           string
	fault            *atomicConfigFaultFS
	creates, renames []string
}

func (fs *tokenBackupPathFaultFS) CreateTemp(dir, pattern string) (TemporaryFile, error) {
	fs.creates = append(fs.creates, pattern)
	if pattern == "."+filepath.Base(fs.target)+".*.tmp" {
		return fs.fault.CreateTemp(dir, pattern)
	}
	return fs.FileSystem.CreateTemp(dir, pattern)
}

func (fs *tokenBackupPathFaultFS) Rename(from, to string) error {
	fs.renames = append(fs.renames, to)
	if to == fs.target {
		return fs.fault.Rename(from, to)
	}
	return fs.FileSystem.Rename(from, to)
}

func TestAPITokenBackupSurvivesPrimaryWriteFailure(t *testing.T) {
	for _, encrypted := range []bool{false, true} {
		encoding := "plaintext"
		if encrypted {
			encoding = "encrypted"
		}
		for _, hasBackup := range []bool{false, true} {
			backupState := "no-backup"
			if hasBackup {
				backupState = "retained-backup"
			}
			for _, failure := range []string{"create", "chmod", "write", "short write", "close", "rename"} {
				t.Run(encoding+"/"+backupState+"/"+failure, func(t *testing.T) {
					f := newTokenBackupFixture(t, encrypted, hasBackup)
					fault := &atomicConfigFaultFS{FileSystem: defaultFileSystem{}, fault: failure, failure: errors.New("synthetic primary failure")}
					fs := &tokenBackupPathFaultFS{FileSystem: defaultFileSystem{}, target: f.p.apiTokensFile, fault: fault}
					f.p.SetFileSystem(fs)
					err := f.p.SaveAPITokens([]APITokenRecord{backupTestToken(t, "synthetic-next-bearer")})
					if failure == "short write" {
						require.ErrorIs(t, err, io.ErrShortWrite)
					} else {
						require.ErrorIs(t, err, fault.failure)
					}
					f.requireUnchanged(t)
					require.NotContains(t, fs.creates, ".api_tokens.json.backup.*.tmp", "failed primary attempted a backup publication")
					f.requireRecovery(t)
				})
			}
		}
	}
}

func TestAPITokenBackupFollowsSuccessfulSave(t *testing.T) {
	for _, encrypted := range []bool{false, true} {
		encoding := "plaintext"
		if encrypted {
			encoding = "encrypted"
		}
		for _, inventory := range []string{"populated", "empty", "nil"} {
			t.Run(encoding+"/"+inventory, func(t *testing.T) {
				f := newTokenBackupFixture(t, encrypted, true)
				fs := &tokenBackupPathFaultFS{FileSystem: defaultFileSystem{}}
				f.p.SetFileSystem(fs)
				var next []APITokenRecord
				if inventory == "populated" {
					next = []APITokenRecord{backupTestToken(t, "synthetic-next-bearer")}
				} else if inventory == "empty" {
					next = []APITokenRecord{}
				}
				require.NoError(t, f.p.SaveAPITokens(next))
				require.Equal(t, []string{f.p.apiTokensFile, f.p.apiTokensFile + ".backup"}, fs.renames)
				backup, err := os.ReadFile(f.p.apiTokensFile + ".backup")
				require.NoError(t, err)
				require.Equal(t, f.primary, backup, "backup must be the exact previous encoding, not the replacement")
				requirePrivateConfigFile(t, f.p.apiTokensFile)
				requirePrivateConfigFile(t, f.p.apiTokensFile+".backup")
				if inventory == "populated" {
					requireBackupTestBearer(t, f.p, "synthetic-next-bearer")
				} else {
					tokens, err := f.p.LoadAPITokens()
					require.NoError(t, err)
					require.Empty(t, tokens, "explicit withdrawal must still persist")
				}
			})
		}
	}
}

func TestAPITokenBackupFailureKeepsCommittedSave(t *testing.T) {
	for _, failure := range []string{"create", "chmod", "write", "short write", "close", "rename"} {
		t.Run(failure, func(t *testing.T) {
			f := newTokenBackupFixture(t, true, true)
			fs := &tokenBackupPathFaultFS{FileSystem: defaultFileSystem{}, target: f.p.apiTokensFile + ".backup", fault: &atomicConfigFaultFS{FileSystem: defaultFileSystem{}, fault: failure, failure: errors.New("synthetic backup failure")}}
			f.p.SetFileSystem(fs)
			require.NoError(t, f.p.SaveAPITokens([]APITokenRecord{backupTestToken(t, "synthetic-next-bearer")}), "best-effort backup failure cannot roll back a committed credential set")
			f.requireBackupUnchanged(t)
			requireBackupTestBearer(t, f.p, "synthetic-next-bearer")
			temps, err := filepath.Glob(filepath.Join(f.p.configDir, ".api_tokens.json.*.tmp"))
			require.NoError(t, err)
			require.Empty(t, temps)
		})
	}
}

func TestAPITokenBackupSurvivesFailedRepairOfIncompletePrimary(t *testing.T) {
	for _, failure := range []string{"validation", "rename"} {
		t.Run(failure, func(t *testing.T) {
			f := newTokenBackupFixture(t, true, true)
			// Simulate an interrupted external replacement, with the older
			// recovery point now the only loadable saved inventory.
			f.primary = []byte("[")
			require.NoError(t, os.WriteFile(f.p.apiTokensFile, f.primary, 0600))
			var err error
			f.primaryInfo, err = os.Stat(f.p.apiTokensFile)
			require.NoError(t, err)
			record := backupTestToken(t, "synthetic-next-bearer")
			if failure == "validation" {
				record.Metadata[auth.RuntimeRoleMetadataKey] = auth.RuntimeRoleMonitoringCollector
			} else {
				f.p.SetFileSystem(&tokenBackupPathFaultFS{FileSystem: defaultFileSystem{}, target: f.p.apiTokensFile, fault: &atomicConfigFaultFS{FileSystem: defaultFileSystem{}, fault: "rename", failure: os.ErrPermission}})
			}
			require.Error(t, f.p.SaveAPITokens([]APITokenRecord{record}))
			f.requireUnchanged(t)
			f.p.SetFileSystem(defaultFileSystem{})
			_, err = f.p.LoadAPITokens()
			require.Error(t, err, "an incomplete primary is not recovered automatically")
			require.NoError(t, os.WriteFile(f.p.apiTokensFile, f.backup, 0600))
			requireBackupTestBearer(t, f.p, "synthetic-older-bearer")
		})
	}
}

func TestAPITokenBackupRejectedSaveStagesNothing(t *testing.T) {
	f := newTokenBackupFixture(t, true, true)
	tx, err := newImportTransaction(f.p.configDir)
	require.NoError(t, err)
	t.Cleanup(tx.Cleanup)
	f.p.beginTransaction(tx)
	defer f.p.endTransaction(tx)
	record := backupTestToken(t, "synthetic-next-bearer")
	record.Metadata[auth.RuntimeRoleMetadataKey] = auth.RuntimeRoleMonitoringCollector
	require.Error(t, f.p.SaveAPITokens([]APITokenRecord{record}))
	f.requireUnchanged(t)
	require.Empty(t, tx.staged, "rejected preparation must not stage a later recovery-file replacement")
}
