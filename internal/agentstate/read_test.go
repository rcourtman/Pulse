package agentstate

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestReadFileCompleteBoundedAuthority(t *testing.T) {
	for _, size := range []int{0, 1, MaxFileBytes - 1, MaxFileBytes, MaxFileBytes + 1} {
		t.Run(stringSize(size), func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "token")
			want := bytes.Repeat([]byte("x"), size)
			if err := os.WriteFile(path, want, 0600); err != nil {
				t.Fatal(err)
			}
			before, err := os.Stat(path)
			if err != nil {
				t.Fatal(err)
			}
			got, err := ReadFile(path)
			if size > MaxFileBytes {
				if !errors.Is(err, ErrTooLarge) || got != nil {
					t.Fatal("oversized file released partial authority")
				}
			} else if err != nil || !bytes.Equal(got, want) {
				t.Fatalf("complete authority was changed: %v", err)
			}
			after, err := os.Stat(path)
			if err != nil || !os.SameFile(before, after) || before.Mode() != after.Mode() || !before.ModTime().Equal(after.ModTime()) {
				t.Fatal("read changed file identity or attributes")
			}
			unchanged, err := os.ReadFile(path)
			if err != nil || !bytes.Equal(unchanged, want) {
				t.Fatal("read replaced saved data")
			}
		})
	}
}

func TestReadFileKeepsAbsenceDistinctFromInvalidState(t *testing.T) {
	dir := t.TempDir()
	if data, err := ReadFile(filepath.Join(dir, "missing")); !errors.Is(err, os.ErrNotExist) || data != nil {
		t.Fatalf("missing file lost absence identity: %v", err)
	}
	if data, err := ReadFile(dir); !errors.Is(err, ErrNotRegular) || data != nil {
		t.Fatal("directory was accepted as authority")
	}
	f, err := os.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if data, err := readOpenedFile(f); !errors.Is(err, ErrNotRegular) || data != nil {
		t.Fatal("opened directory was accepted as authority")
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	if data, err := readOpenedFile(f); err == nil || data != nil {
		t.Fatal("failed descriptor stat returned authority")
	}
}

func TestReadFileFollowsProjectedSecretsAndRelocatedState(t *testing.T) {
	root := t.TempDir()
	version := filepath.Join(root, "version-1")
	if err := os.Mkdir(version, 0700); err != nil {
		t.Fatal(err)
	}
	want := []byte(" \tsynthetic-saved-token\r\n")
	if err := os.WriteFile(filepath.Join(version, "token"), want, 0400); err != nil {
		t.Fatal(err)
	}
	link := func(target, path string) {
		t.Helper()
		if err := os.Symlink(target, path); err != nil {
			if runtime.GOOS == "windows" {
				t.Skip("Windows symlink privilege unavailable")
			}
			t.Fatal(err)
		}
	}
	link("version-1", filepath.Join(root, "..data"))
	link(filepath.Join("..data", "token"), filepath.Join(root, "token"))
	alias := filepath.Join(t.TempDir(), "relocated")
	link(root, alias)
	for _, path := range []string{filepath.Join(root, "token"), filepath.Join(alias, "token")} {
		for attempt := 0; attempt < 2; attempt++ {
			got, err := ReadFile(path)
			if err != nil || !bytes.Equal(got, want) {
				t.Fatalf("linked read-only saved authority failed: %v", err)
			}
		}
	}
	// A projected volume may switch its directory link between reads.
	version2 := filepath.Join(root, "version-2")
	if err := os.Mkdir(version2, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(version2, "token"), []byte("synthetic-rotated-token"), 0400); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(root, "..data")); err != nil {
		t.Fatal(err)
	}
	link("version-2", filepath.Join(root, "..data"))
	if data, err := ReadFile(filepath.Join(alias, "token")); err != nil || string(data) != "synthetic-rotated-token" {
		t.Fatalf("projected secret rotation was not read: %v", err)
	}
	if err := os.Remove(filepath.Join(root, "..data")); err != nil {
		t.Fatal(err)
	}
	link("missing-version", filepath.Join(root, "..data"))
	if data, err := ReadFile(filepath.Join(alias, "token")); !errors.Is(err, ErrUnavailable) || errors.Is(err, os.ErrNotExist) || data != nil {
		t.Fatalf("missing projected destination became first-run absence: %v", err)
	}
}

func TestMissingLinkedAuthorityIsNotFirstRunAbsence(t *testing.T) {
	for _, shape := range []string{"file-link", "directory-link", "nested-directory-link", "unclean-directory-link", "link-chain"} {
		t.Run(shape, func(t *testing.T) {
			root := t.TempDir()
			target := filepath.Join(root, "unavailable")
			linkPath := filepath.Join(root, "saved-state")
			path := linkPath
			want := []byte("synthetic-retained-authority\n")
			switch shape {
			case "directory-link":
				path = filepath.Join(linkPath, "token")
			case "nested-directory-link":
				path = filepath.Join(linkPath, "nested", "token")
			case "unclean-directory-link":
				path = linkPath + string(os.PathSeparator) + ".." + string(os.PathSeparator) + "token"
			case "link-chain":
				if err := os.Symlink("unavailable", filepath.Join(root, "intermediate")); err != nil {
					if runtime.GOOS == "windows" {
						t.Skip("Windows symlink privilege unavailable")
					}
					t.Fatal(err)
				}
				target = filepath.Join(root, "intermediate")
			}
			directoryTarget := shape != "file-link" && shape != "link-chain"
			if directoryTarget {
				// Windows chooses the link kind from the existing target. Make
				// the directory link first, then model its target going missing.
				if err := os.Mkdir(target, 0700); err != nil {
					t.Fatal(err)
				}
			}
			if err := os.Symlink(filepath.Base(target), linkPath); err != nil {
				if runtime.GOOS == "windows" {
					t.Skip("Windows symlink privilege unavailable")
				}
				t.Fatal(err)
			}
			if directoryTarget {
				if err := os.Remove(target); err != nil {
					t.Fatal(err)
				}
			}
			before, err := os.Lstat(linkPath)
			if err != nil {
				t.Fatal(err)
			}
			for attempt := 0; attempt < 2; attempt++ {
				if data, err := ReadFile(path); data != nil || !errors.Is(err, ErrUnavailable) || errors.Is(err, os.ErrNotExist) {
					t.Fatalf("missing linked authority became absence or data: %v", err)
				}
			}
			after, err := os.Lstat(linkPath)
			gotTarget, linkErr := os.Readlink(linkPath)
			if err != nil || linkErr != nil || !os.SameFile(before, after) || before.Mode() != after.Mode() || !before.ModTime().Equal(after.ModTime()) || gotTarget != filepath.Base(target) {
				t.Fatal("refusal changed the retained link")
			}
			// Restore the missing target, not the saved link or another identity.
			restored := filepath.Join(root, "unavailable")
			if shape == "directory-link" {
				restored = filepath.Join(restored, "token")
			} else if shape == "nested-directory-link" {
				restored = filepath.Join(restored, "nested", "token")
			} else if shape == "unclean-directory-link" {
				if err := os.Mkdir(restored, 0700); err != nil {
					t.Fatal(err)
				}
				restored = filepath.Join(root, "token")
			}
			if err := os.MkdirAll(filepath.Dir(restored), 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(restored, want, 0600); err != nil {
				t.Fatal(err)
			}
			if data, err := ReadFile(path); err != nil || !bytes.Equal(data, want) {
				t.Fatalf("restoring the linked target did not restore authority: %v", err)
			}
		})
	}
}

func TestMissingStateBeneathLiveDirectoryLinksStillAllowsFirstStart(t *testing.T) {
	root := t.TempDir()
	alias := filepath.Join(t.TempDir(), "relocated")
	if err := os.Symlink(root, alias); err != nil {
		if runtime.GOOS == "windows" {
			t.Skip("Windows symlink privilege unavailable")
		}
		t.Fatal(err)
	}
	for _, path := range []string{filepath.Join(root, "token"), filepath.Join(root, "nested", "token"), filepath.Join(alias, "token"), filepath.Join(alias, "nested", "token")} {
		if data, err := ReadFile(path); data != nil || !errors.Is(err, os.ErrNotExist) || errors.Is(err, ErrUnavailable) {
			t.Fatalf("real first-run absence was refused: %v", err)
		}
	}
	// A saved file which reappears after the missing Stat observation cannot be
	// classified as absent using that stale observation. Other errors retain
	// their identity, and no raw missing-path error is wrapped as unavailable.
	path := filepath.Join(root, "token")
	if err := os.WriteFile(path, []byte("synthetic-reappeared"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := classifyMissingFile(path, os.ErrNotExist); !errors.Is(err, ErrUnavailable) || errors.Is(err, os.ErrNotExist) {
		t.Fatal("reappeared state was classified as absent")
	}
	if err := classifyMissingFile(path, os.ErrPermission); !errors.Is(err, os.ErrPermission) {
		t.Fatal("unrelated read error lost its identity")
	}
}

type observedReader struct {
	remaining int
	read      int
	failure   error
}

func (r *observedReader) Read(p []byte) (int, error) {
	if r.remaining == 0 {
		if r.failure != nil {
			return 0, r.failure
		}
		return 0, io.EOF
	}
	n := min(len(p), r.remaining)
	for i := range p[:n] {
		p[i] = 'x'
	}
	r.read += n
	r.remaining -= n
	return n, nil
}

func TestReadBoundedNeverAcceptsPrefixOrReadsBeyondOverflowByte(t *testing.T) {
	for _, size := range []int64{0, MaxFileBytes - 1, MaxFileBytes, MaxFileBytes + 1} {
		t.Run(stringSize(int(size)), func(t *testing.T) {
			r := &observedReader{remaining: 4 * MaxFileBytes}
			data, err := readBounded(r, size)
			wantRead := MaxFileBytes + 1
			if size > MaxFileBytes {
				wantRead = 0
			}
			if !errors.Is(err, ErrTooLarge) || data != nil || r.read != wantRead {
				t.Fatalf("oversize/growing read: read=%d, want %d, error=%v", r.read, wantRead, err)
			}
		})
	}
	wantErr := errors.New("synthetic-private-read-failure")
	r := &observedReader{remaining: 24, failure: wantErr}
	if data, err := readBounded(r, 24); !errors.Is(err, wantErr) || data != nil {
		t.Fatal("partial data from a failed read became authority")
	}
}

func stringSize(n int) string {
	// Avoid putting any actual authority data in test labels.
	return fmt.Sprintf("bytes-%d", n)
}
