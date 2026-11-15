package api

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"
)

type authSnapshotFixture struct {
	dir         string
	name        string
	save        func()
	revoke      func()
	checkLive   func(*testing.T)
	checkReload func(*testing.T)
	raw         []string
}

func newAuthSnapshotFixture(t *testing.T, kind string) authSnapshotFixture {
	t.Helper()
	return newAuthSnapshotFixtureAt(t, kind, t.TempDir())
}

func newAuthSnapshotFixtureAt(t *testing.T, kind, dir string) authSnapshotFixture {
	t.Helper()
	const target, control = "synthetic-snapshot-target", "synthetic-snapshot-control"
	f := authSnapshotFixture{dir: dir}
	switch kind {
	case "sessions":
		s := &SessionStore{sessions: make(map[string]*SessionData), dataPath: dir}
		s.CreateSession(target, time.Hour, "fixture", "127.0.0.1", "target-user")
		s.CreateSession(control, time.Hour, "fixture", "127.0.0.1", "control-user")
		f.name, f.save = "sessions.json", s.save
		f.revoke = func() { s.DeleteSession(target) }
		f.checkLive = func(t *testing.T) {
			t.Helper()
			if s.ValidateSession(target) || !s.ValidateSession(control) {
				t.Error("live withdrawal changed its target boundary")
			}
		}
		f.checkReload = func(t *testing.T) {
			t.Helper()
			loaded := &SessionStore{sessions: make(map[string]*SessionData), dataPath: dir}
			loaded.load()
			if loaded.ValidateSession(target) || !loaded.ValidateSession(control) {
				t.Error("restart restored withdrawn authority or lost the independent session")
			}
		}
		f.raw = []string{target, control}
	case "csrf":
		s := &CSRFTokenStore{tokens: make(map[string][]*CSRFToken), dataPath: dir}
		targetOne := s.GenerateCSRFToken(target)
		targetTwo := s.GenerateCSRFToken(target)
		controlToken := s.GenerateCSRFToken(control)
		f.name, f.save = "csrf_tokens.json", s.save
		f.revoke = func() { s.DeleteCSRFToken(target) }
		f.checkLive = func(t *testing.T) {
			t.Helper()
			if s.ValidateCSRFToken(target, targetOne) || s.ValidateCSRFToken(target, targetTwo) || !s.ValidateCSRFToken(control, controlToken) {
				t.Error("live withdrawal changed its complete token-set boundary")
			}
		}
		f.checkReload = func(t *testing.T) {
			t.Helper()
			loaded := &CSRFTokenStore{tokens: make(map[string][]*CSRFToken), dataPath: dir}
			loaded.load()
			if loaded.ValidateCSRFToken(target, targetOne) || loaded.ValidateCSRFToken(target, targetTwo) || !loaded.ValidateCSRFToken(control, controlToken) {
				t.Error("restart restored withdrawn CSRF authority or lost the independent token")
			}
		}
		f.raw = []string{target, control, targetOne, targetTwo, controlToken}
	default:
		t.Fatalf("unknown auth snapshot fixture %q", kind)
	}
	return f
}

func TestAuthSnapshotPublicationLinkedStorage(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX directory/destination link control")
	}
	for _, kind := range []string{"sessions", "csrf"} {
		t.Run(kind, func(t *testing.T) {
			dir := t.TempDir()
			linked := filepath.Join(t.TempDir(), "linked-data")
			if err := os.Symlink(dir, linked); err != nil {
				t.Fatal(err)
			}
			f := newAuthSnapshotFixtureAt(t, kind, linked)
			path := filepath.Join(linked, f.name)
			original, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			target := filepath.Join(t.TempDir(), "previous-snapshot")
			if err := os.Rename(path, target); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(target, path); err != nil {
				t.Fatal(err)
			}
			f.revoke()
			f.checkLive(t)
			f.checkReload(t)
			checkPrivateAuthSnapshot(t, f)
			if got, err := os.ReadFile(target); err != nil || !bytes.Equal(got, original) {
				t.Fatal("publication followed the canonical link instead of replacing it")
			}
			if got, err := os.Readlink(linked); err != nil || got != dir {
				t.Fatal("publication changed the configured data-directory link")
			}
		})
	}
}

// Actual failed directory/temp creation and rename leave the old canonical
// bytes and live withdrawal alone. Only a later successful save is durable.
func TestAuthSnapshotPublicationFailureRecovery(t *testing.T) {
	for _, kind := range []string{"sessions", "csrf"} {
		for _, fault := range []string{"directory", "create", "rename"} {
			t.Run(kind+"/"+fault, func(t *testing.T) {
				if fault == "create" && (runtime.GOOS == "windows" || os.Geteuid() == 0) {
					t.Skip("ordinary-user POSIX directory permissions")
				}
				f := newAuthSnapshotFixture(t, kind)
				path := filepath.Join(f.dir, f.name)
				original, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				var restore func()
				switch fault {
				case "directory":
					if err := os.Rename(f.dir, f.dir+".saved"); err != nil {
						t.Fatal(err)
					}
					restore = func() {
						if err := os.Remove(f.dir); err != nil && !os.IsNotExist(err) {
							t.Error(err)
						}
						if err := os.Rename(f.dir+".saved", f.dir); err != nil {
							t.Error(err)
						}
					}
					if err := os.WriteFile(f.dir, []byte("synthetic directory blocker"), 0600); err != nil {
						restore()
						t.Fatal(err)
					}
				case "create":
					if err := os.Chmod(f.dir, 0500); err != nil {
						t.Fatal(err)
					}
					restore = func() {
						if err := os.Chmod(f.dir, 0700); err != nil {
							t.Error(err)
						}
					}
				case "rename":
					if err := os.Rename(path, path+".saved"); err != nil {
						t.Fatal(err)
					}
					if err := os.Mkdir(path, 0700); err != nil {
						_ = os.Rename(path+".saved", path)
						t.Fatal(err)
					}
					restore = func() {
						if err := os.Remove(path); err != nil {
							t.Error(err)
						}
						if err := os.Rename(path+".saved", path); err != nil {
							t.Error(err)
						}
					}
				}
				var once sync.Once
				repair := func() { once.Do(restore) }
				t.Cleanup(repair)
				f.revoke()
				f.checkLive(t)
				repair()
				if got, err := os.ReadFile(path); err != nil || !bytes.Equal(got, original) {
					t.Fatal("failed publication changed original saved authority")
				}
				checkNoOwnedAuthTemps(t, f.dir, f.name)
				f.save()
				f.checkReload(t)
				checkPrivateAuthSnapshot(t, f)
			})
		}
	}
}

func checkPrivateAuthSnapshot(t *testing.T, f authSnapshotFixture) {
	t.Helper()
	path := filepath.Join(f.dir, f.name)
	info, err := os.Lstat(path)
	if err != nil {
		t.Fatal(err)
	}
	if !info.Mode().IsRegular() {
		t.Error("published authority is not a regular file")
	}
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0600 {
		t.Error("published authority is not owner-private")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, raw := range f.raw {
		if bytes.Contains(data, []byte(raw)) {
			t.Error("snapshot disclosed a raw credential")
		}
	}
	leftovers, err := filepath.Glob(filepath.Join(f.dir, "."+f.name+".*.tmp"))
	if err != nil || len(leftovers) != 0 {
		t.Error("publication left an owned temporary snapshot")
	}
}

// A previously interrupted/unrelated .tmp path is not this save's descriptor.
// Exercise the actual two store consumers, not only a standalone file helper.
func TestAuthSnapshotPublicationIgnoresLegacyTemps(t *testing.T) {
	for _, kind := range []string{"sessions", "csrf"} {
		for _, stale := range []string{"permissive-file", "symlink", "directory"} {
			t.Run(kind+"/"+stale, func(t *testing.T) {
				if runtime.GOOS == "windows" && stale != "directory" {
					t.Skip("POSIX mode and symlink control")
				}
				f := newAuthSnapshotFixture(t, kind)
				legacy := filepath.Join(f.dir, f.name+".tmp")
				const sentinel = "unrelated synthetic bytes"
				var target string
				switch stale {
				case "permissive-file":
					if err := os.WriteFile(legacy, []byte(sentinel), 0644); err != nil {
						t.Fatal(err)
					}
					if err := os.Chmod(legacy, 0644); err != nil {
						t.Fatal(err)
					}
					if err := os.Chmod(filepath.Join(f.dir, f.name), 0644); err != nil {
						t.Fatal(err)
					}
				case "symlink":
					target = filepath.Join(t.TempDir(), "unrelated")
					if err := os.WriteFile(target, []byte(sentinel), 0644); err != nil {
						t.Fatal(err)
					}
					if err := os.Symlink(target, legacy); err != nil {
						t.Fatal(err)
					}
				case "directory":
					if err := os.Mkdir(legacy, 0700); err != nil {
						t.Fatal(err)
					}
				}
				f.revoke()
				f.checkLive(t)
				checkPrivateAuthSnapshot(t, f)
				f.checkReload(t)
				switch stale {
				case "permissive-file":
					if data, err := os.ReadFile(legacy); err != nil || string(data) != sentinel {
						t.Error("save changed or removed an unowned temporary file")
					}
				case "symlink":
					if data, err := os.ReadFile(target); err != nil || string(data) != sentinel {
						t.Error("save followed a temporary symlink into an unrelated file")
					}
					if got, err := os.Readlink(legacy); err != nil || got != target {
						t.Error("save changed or removed an unowned temporary symlink")
					}
				case "directory":
					if info, err := os.Lstat(legacy); err != nil || !info.IsDir() {
						t.Error("save changed or removed an unowned temporary directory")
					}
				}
			})
		}
	}
}
