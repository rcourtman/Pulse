package config

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

type atomicConfigFaultFS struct {
	FileSystem
	fault       string
	failure     error
	cleanupErr  error
	file        *atomicConfigFaultFile
	renameCalls int
}

func (fs *atomicConfigFaultFS) CreateTemp(dir, pattern string) (TemporaryFile, error) {
	if fs.fault == "create" {
		return nil, fs.failure
	}
	f, err := fs.FileSystem.CreateTemp(dir, pattern)
	if err != nil {
		return nil, err
	}
	fs.file = &atomicConfigFaultFile{TemporaryFile: f, fs: fs}
	return fs.file, nil
}

func (fs *atomicConfigFaultFS) Rename(from, to string) error {
	fs.renameCalls++
	if fs.fault == "rename" {
		return fs.failure
	}
	return fs.FileSystem.Rename(from, to)
}

func (fs *atomicConfigFaultFS) Remove(path string) error {
	if fs.cleanupErr != nil {
		return fs.cleanupErr
	}
	return fs.FileSystem.Remove(path)
}

type atomicConfigFaultFile struct {
	TemporaryFile
	fs         *atomicConfigFaultFS
	closeCalls int
}

func (f *atomicConfigFaultFile) Chmod(perm os.FileMode) error {
	if f.fs.fault == "chmod" {
		return f.fs.failure
	}
	return f.TemporaryFile.Chmod(perm)
}

func (f *atomicConfigFaultFile) Write(data []byte) (int, error) {
	if f.fs.fault == "write" {
		// A partial write must never replace the previous primary or backup.
		n, _ := f.TemporaryFile.Write(data[:len(data)/2])
		return n, f.fs.failure
	}
	if f.fs.fault == "short write" {
		return f.TemporaryFile.Write(data[:len(data)/2])
	}
	return f.TemporaryFile.Write(data)
}

func (f *atomicConfigFaultFile) Close() error {
	f.closeCalls++
	err := f.TemporaryFile.Close()
	if f.fs.fault == "close" {
		return errors.Join(err, f.fs.failure)
	}
	return err
}

func TestAtomicConfigWritePreservesStateOnFailure(t *testing.T) {
	for _, fault := range []string{"create", "chmod", "write", "short write", "close", "rename"} {
		t.Run(fault, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "config.enc")
			require.NoError(t, os.WriteFile(path, []byte("previous complete state"), 0o600))
			failure := errors.New("synthetic " + fault + " failure")
			fs := &atomicConfigFaultFS{FileSystem: defaultFileSystem{}, fault: fault, failure: failure}
			err := writeConfigFileAtomic(fs, path, []byte("replacement complete state"), 0o600)
			if fault == "short write" {
				require.ErrorIs(t, err, io.ErrShortWrite)
			} else {
				require.ErrorIs(t, err, failure)
			}
			raw, err := os.ReadFile(path)
			require.NoError(t, err)
			require.Equal(t, "previous complete state", string(raw))
			if fs.file != nil {
				require.Equal(t, 1, fs.file.closeCalls)
				_, err = os.Stat(fs.file.Name())
				require.True(t, os.IsNotExist(err), "owned temporary file must be cleaned")
			}
			if fault != "rename" {
				require.Zero(t, fs.renameCalls, "incomplete files must not be published")
			}
			entries, err := os.ReadDir(dir)
			require.NoError(t, err)
			require.Len(t, entries, 1)
		})
	}
}

func TestAtomicConfigWriteReportsCleanupFailure(t *testing.T) {
	writeErr, cleanupErr := errors.New("synthetic write failure"), errors.New("synthetic cleanup failure")
	fs := &atomicConfigFaultFS{FileSystem: defaultFileSystem{}, fault: "write", failure: writeErr, cleanupErr: cleanupErr}
	path := filepath.Join(t.TempDir(), "config.enc")
	err := writeConfigFileAtomic(fs, path, []byte("synthetic state"), 0o600)
	require.ErrorIs(t, err, writeErr)
	require.ErrorIs(t, err, cleanupErr)
	require.Equal(t, 1, fs.file.closeCalls)
	_, err = os.Stat(path)
	require.True(t, os.IsNotExist(err), "failed write must not create a primary")
}

type atomicConfigConcurrentFS struct {
	FileSystem
	mu    sync.Mutex
	paths []string
}

func (fs *atomicConfigConcurrentFS) CreateTemp(dir, pattern string) (TemporaryFile, error) {
	f, err := fs.FileSystem.CreateTemp(dir, pattern)
	if err != nil {
		return nil, err
	}
	fs.mu.Lock()
	fs.paths = append(fs.paths, f.Name())
	fs.mu.Unlock()
	return f, nil
}

func TestAtomicConfigIndependentWritersPublishCompleteFiles(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.enc")
	fs := &atomicConfigConcurrentFS{FileSystem: defaultFileSystem{}}
	const writers = 12
	start := make(chan struct{})
	results := make(chan error, writers)
	values := make([][]byte, writers)
	for i := range writers {
		values[i] = bytes.Repeat([]byte{byte('a' + i)}, 65536)
		go func() {
			<-start
			results <- writeConfigFileAtomic(fs, path, values[i], 0o600)
		}()
	}
	close(start)
	var failures []error
	for range writers {
		failures = append(failures, <-results)
	}
	require.NoError(t, errors.Join(failures...))
	raw, err := os.ReadFile(path)
	require.NoError(t, err)
	complete := false
	for _, value := range values {
		complete = complete || bytes.Equal(value, raw)
	}
	require.True(t, complete, "final state must be one complete write, never mixed")
	requirePrivateConfigFile(t, path)
	seen := make(map[string]bool)
	for _, tmp := range fs.paths {
		require.False(t, seen[tmp], "independent writes must own different temporary files")
		seen[tmp] = true
		require.Equal(t, dir, filepath.Dir(tmp))
	}
	require.Len(t, seen, writers)
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	require.Len(t, entries, 1)
}

func TestAtomicConfigWritesRemainStagedDuringImport(t *testing.T) {
	cp := NewConfigPersistence(t.TempDir())
	records := []APITokenRecord{{ID: "fixture-id", Hash: "previous-hash", Scopes: []string{"monitoring:read"}}}
	require.NoError(t, cp.SaveAPITokens(records))
	original, err := os.ReadFile(cp.apiTokensFile)
	require.NoError(t, err)
	env := filepath.Join(cp.configDir, ".env")
	originalEnv := []byte("PULSE_AUTH_PASS=synthetic-private-value\nUPDATE_CHANNEL=beta\n")
	require.NoError(t, os.WriteFile(env, originalEnv, 0o600))
	tx, err := newImportTransaction(cp.configDir)
	require.NoError(t, err)
	defer tx.Cleanup()
	cp.beginTransaction(tx)
	defer cp.endTransaction(tx)
	records[0].Hash = "replacement-hash"
	require.NoError(t, cp.SaveAPITokens(records))
	require.NoError(t, cp.SaveSystemSettings(SystemSettings{UpdateChannel: "stable"}))
	for path, want := range map[string][]byte{cp.apiTokensFile: original, env: originalEnv} {
		got, err := os.ReadFile(path)
		require.NoError(t, err)
		require.Equal(t, want, got, "import must not change live state before commit")
	}
	_, err = os.Stat(cp.apiTokensFile + ".backup")
	require.True(t, os.IsNotExist(err), "even backup changes must remain staged")
	require.Len(t, tx.staged, 4)
	for _, staged := range tx.staged {
		requirePrivateConfigFile(t, staged)
	}
	require.NoError(t, tx.Commit())
	cp.endTransaction(tx)
	loaded, err := cp.LoadAPITokens()
	require.NoError(t, err)
	require.Equal(t, "replacement-hash", loaded[0].Hash)
	backup, err := os.ReadFile(cp.apiTokensFile + ".backup")
	require.NoError(t, err)
	require.Equal(t, original, backup)
	rawEnv, err := os.ReadFile(env)
	require.NoError(t, err)
	require.Contains(t, string(rawEnv), "UPDATE_CHANNEL=stable\n")
	require.Contains(t, string(rawEnv), "PULSE_AUTH_PASS=synthetic-private-value\n")
}
