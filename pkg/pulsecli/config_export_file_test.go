package pulsecli

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

type faultConfigExportFile struct {
	*os.File
	fault      string
	failure    error
	closeErr   error
	closeCalls int
	synced     bool
}

func (f *faultConfigExportFile) Chmod(mode os.FileMode) error {
	if f.fault == "chmod" {
		return f.failure
	}
	return f.File.Chmod(mode)
}

func (f *faultConfigExportFile) Write(data []byte) (int, error) {
	if f.fault == "write" || f.fault == "short-write" {
		n, err := f.File.Write(data[:len(data)/2])
		if f.fault == "write" {
			err = errors.Join(err, f.failure)
		}
		return n, err
	}
	return f.File.Write(data)
}

func (f *faultConfigExportFile) Sync() error {
	if f.fault == "sync" {
		return f.failure
	}
	f.synced = true
	return f.File.Sync()
}

func (f *faultConfigExportFile) Close() error {
	f.closeCalls++
	err := f.File.Close()
	if f.fault == "close" {
		err = errors.Join(err, f.failure)
	}
	return errors.Join(err, f.closeErr)
}

func TestConfigExportFilePreservesDestinationOnFailure(t *testing.T) {
	for _, existing := range []bool{false, true} {
		for _, fault := range []string{"inspect", "create", "chmod", "write", "short-write", "sync", "close", "rename"} {
			name := "absent/"
			if existing {
				name = "existing/"
			}
			t.Run(name+fault, func(t *testing.T) {
				dir := t.TempDir()
				path := filepath.Join(dir, "backup.enc")
				const original = "previous complete encrypted backup"
				if existing {
					require.NoError(t, os.WriteFile(path, []byte(original), 0o600))
				}
				failure := errors.New("synthetic output failure")
				var file *faultConfigExportFile
				renames := 0
				ops := configExportFileOps{
					lstat: func(path string) (os.FileInfo, error) {
						if fault == "inspect" {
							return nil, failure
						}
						return os.Lstat(path)
					},
					createTemp: func(parent, pattern string) (configExportFile, error) {
						require.Equal(t, dir, parent, "stage on the destination filesystem")
						if fault == "create" {
							return nil, failure
						}
						f, err := os.CreateTemp(parent, pattern)
						require.NoError(t, err)
						file = &faultConfigExportFile{File: f, fault: fault, failure: failure}
						return file, nil
					},
					rename: func(from, to string) error {
						renames++
						require.True(t, file.synced)
						require.Equal(t, 1, file.closeCalls)
						require.Equal(t, path, to)
						return failure
					},
					remove: os.Remove,
				}
				err := publishConfigExportFile(path, []byte("replacement complete encrypted backup"), ops)
				if fault == "short-write" {
					require.ErrorIs(t, err, io.ErrShortWrite)
				} else {
					require.ErrorIs(t, err, failure)
				}
				data, err := os.ReadFile(path)
				if existing {
					require.NoError(t, err)
					require.Equal(t, original, string(data))
				} else {
					require.True(t, os.IsNotExist(err), "failure created an output file")
				}
				if file != nil {
					require.Equal(t, 1, file.closeCalls)
					_, err := os.Lstat(file.Name())
					require.True(t, os.IsNotExist(err), "owned temporary file was not cleaned up")
				}
				wantRenames, wantEntries := 0, 0
				if fault == "rename" {
					wantRenames = 1
				}
				if existing {
					wantEntries = 1
				}
				require.Equal(t, wantRenames, renames, "incomplete output reached publication")
				entries, err := os.ReadDir(dir)
				require.NoError(t, err)
				require.Len(t, entries, wantEntries)
			})
		}
	}
}

func TestConfigExportFileReportsCleanupFailures(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "backup.enc")
	writeErr, closeErr, removeErr := errors.New("write failed"), errors.New("close failed"), errors.New("cleanup failed")
	var file *faultConfigExportFile
	err := publishConfigExportFile(path, []byte("synthetic encrypted export"), configExportFileOps{
		lstat: os.Lstat,
		createTemp: func(dir, pattern string) (configExportFile, error) {
			f, err := os.CreateTemp(dir, pattern)
			require.NoError(t, err)
			file = &faultConfigExportFile{File: f, fault: "write", failure: writeErr, closeErr: closeErr}
			return file, nil
		},
		rename: func(string, string) error { t.Fatal("failed output was published"); return nil },
		remove: func(string) error { return removeErr },
	})
	require.ErrorIs(t, err, writeErr)
	require.ErrorIs(t, err, closeErr)
	require.ErrorIs(t, err, removeErr)
	require.Equal(t, 1, file.closeCalls)
	requirePrivateExportFile(t, file.Name())
	_, err = os.Lstat(path)
	require.True(t, os.IsNotExist(err))
	// The deliberately failed cleanup leaves only this owned private temp.
	require.NoError(t, os.Remove(file.Name()))
}

func requirePrivateExportFile(t *testing.T, path string) {
	t.Helper()
	info, err := os.Lstat(path)
	require.NoError(t, err)
	require.True(t, info.Mode().IsRegular())
	if runtime.GOOS != "windows" {
		require.Equal(t, os.FileMode(0o600), info.Mode().Perm())
	}
}

func TestConfigExportFilePublishesOnlyAfterClose(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "backup.enc")
	require.NoError(t, os.WriteFile(path, []byte("original"), 0o600))
	var file *faultConfigExportFile
	data := []byte("complete replacement")
	err := publishConfigExportFile(path, data, configExportFileOps{
		lstat: os.Lstat,
		createTemp: func(parent, pattern string) (configExportFile, error) {
			require.Equal(t, dir, parent)
			f, err := os.CreateTemp(parent, pattern)
			require.NoError(t, err)
			file = &faultConfigExportFile{File: f}
			requirePrivateExportFile(t, f.Name())
			return file, nil
		},
		rename: func(from, to string) error {
			require.Equal(t, path, to)
			require.Equal(t, file.Name(), from)
			require.True(t, file.synced)
			require.Equal(t, 1, file.closeCalls)
			before, err := os.ReadFile(path)
			require.NoError(t, err)
			require.Equal(t, "original", string(before), "old backup changed before publication")
			staged, err := os.ReadFile(from)
			require.NoError(t, err)
			require.Equal(t, data, staged)
			return os.Rename(from, to)
		},
		remove: func(string) error { t.Fatal("successful publication was removed"); return nil },
	})
	require.NoError(t, err)
	requirePrivateExportFile(t, path)
	got, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Equal(t, data, got)
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	require.Len(t, entries, 1)
}

func TestConfigExportFileDoesNotFollowSubstitutedFinalSymlink(t *testing.T) {
	dir := t.TempDir()
	path, target := filepath.Join(dir, "backup.enc"), filepath.Join(dir, "unrelated-file")
	require.NoError(t, os.WriteFile(path, []byte("original backup"), 0o600))
	require.NoError(t, os.WriteFile(target, []byte("unrelated original bytes"), 0o600))
	err := publishConfigExportFile(path, []byte("complete encrypted export"), configExportFileOps{
		lstat: os.Lstat,
		createTemp: func(parent, pattern string) (configExportFile, error) {
			// Replace the inspected regular destination before publication.
			require.NoError(t, os.Remove(path))
			if err := os.Symlink(target, path); err != nil {
				t.Skipf("symlinks unavailable: %v", err)
			}
			return os.CreateTemp(parent, pattern)
		},
		rename: os.Rename,
		remove: os.Remove,
	})
	require.NoError(t, err)
	data, err := os.ReadFile(target)
	require.NoError(t, err)
	require.Equal(t, "unrelated original bytes", string(data))
	requirePrivateExportFile(t, path)
	data, err = os.ReadFile(path)
	require.NoError(t, err)
	require.Equal(t, "complete encrypted export", string(data))
}

func TestConfigExportFileConcurrentPublicationIsComplete(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("same-directory rename atomicity is a Unix filesystem guarantee")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "backup.enc")
	const writers = 4
	values := make([][]byte, writers+1)
	for i := range values {
		values[i] = bytes.Repeat([]byte{byte('a' + i)}, 128*1024)
	}
	require.NoError(t, writeConfigExportFile(path, values[0]))
	start, done := make(chan struct{}), make(chan struct{})
	results := make(chan error, writers)
	var wg sync.WaitGroup
	for i := range writers {
		wg.Go(func() {
			<-start
			results <- writeConfigExportFile(path, values[i+1])
		})
	}
	go func() { wg.Wait(); close(done) }()
	defer func() { <-done }()
	close(start)
	reads := 0
	for {
		got, err := os.ReadFile(path)
		require.NoError(t, err)
		complete := false
		for _, value := range values {
			complete = complete || bytes.Equal(value, got)
		}
		require.True(t, complete, "reader observed a truncated or mixed backup")
		reads++
		select {
		case <-done:
			for range writers {
				require.NoError(t, <-results)
			}
			requirePrivateExportFile(t, path)
			entries, err := os.ReadDir(dir)
			require.NoError(t, err)
			require.Len(t, entries, 1)
			t.Logf("observed %d whole-file reads across %d independent writers", reads, writers)
			return
		default:
		}
	}
}
