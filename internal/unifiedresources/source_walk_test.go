package unifiedresources

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// walkProductionGoFiles inspects the same production sources as the policy
// audits, without statting unrelated files. Other test packages can remove
// generated runtime files after a directory has been listed. WalkDir retains
// entry names/types, so that churn cannot fail a source-only audit. Directory,
// source-read and visitor errors still fail the audit; none are suppressed.
func walkProductionGoFiles(root string, visit func(string, []byte) error) error {
	return filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || filepath.Ext(path) != ".go" || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		source, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return visit(path, source)
	})
}

func writeSourceWalkFixture(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for name, source := range files {
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(source), 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

func TestProductionGoWalkIgnoresDisappearingNonSourceEntries(t *testing.T) {
	for _, ignored := range []string{"10-runtime.sidecar", "10-fixture_test.go", "10-notes.md"} {
		t.Run(ignored, func(t *testing.T) {
			root := t.TempDir()
			want := map[string]string{
				"00-trigger.go":           "package fixture\n",
				"20-nested/production.go": "package nested\n// policy evidence\n",
				"30-last.go":              "package fixture\n// final evidence\n",
			}
			writeSourceWalkFixture(t, root, want)
			writeSourceWalkFixture(t, root, map[string]string{ignored: "not production source"})
			seen := map[string]string{}
			err := walkProductionGoFiles(root, func(path string, source []byte) error {
				rel, err := filepath.Rel(root, path)
				if err != nil {
					return err
				}
				seen[filepath.ToSlash(rel)] = string(source)
				if rel == "00-trigger.go" {
					// Both walkers list siblings before visiting the first file.
					// Removal here deterministically exercises that window, with
					// no concurrent writer, database or timing-dependent sleep.
					return os.Remove(filepath.Join(root, ignored))
				}
				return nil
			})
			if err != nil {
				t.Fatalf("non-source removal must not abort source inspection: %v", err)
			}
			if !reflect.DeepEqual(seen, want) {
				t.Fatalf("inspected sources = %v, want %v", seen, want)
			}
			if _, err := os.Stat(filepath.Join(root, ignored)); !errors.Is(err, fs.ErrNotExist) {
				t.Fatalf("fixture did not remove the ignored entry: %v", err)
			}
		})
	}
}

func TestProductionGoWalkFailsOnDisappearingSourceOrDirectory(t *testing.T) {
	for _, directory := range []bool{false, true} {
		name := "10-source.go"
		if directory {
			name = "10-package"
		}
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			writeSourceWalkFixture(t, root, map[string]string{"00-trigger.go": "package fixture\n"})
			victim := filepath.Join(root, name)
			if directory {
				if err := os.Mkdir(victim, 0o700); err != nil {
					t.Fatal(err)
				}
			} else {
				writeSourceWalkFixture(t, root, map[string]string{name: "package fixture\n"})
			}
			triggered := false
			err := walkProductionGoFiles(root, func(path string, _ []byte) error {
				if filepath.Base(path) == "00-trigger.go" {
					triggered = true
					return os.Remove(victim)
				}
				return nil
			})
			if !triggered || !errors.Is(err, fs.ErrNotExist) {
				t.Fatalf("missing audit input must fail closed: triggered=%v, err=%v", triggered, err)
			}
		})
	}
}

func TestProductionGoWalkFailsOnSourceReadError(t *testing.T) {
	root := t.TempDir()
	writeSourceWalkFixture(t, root, map[string]string{
		"00-trigger.go": "package fixture\n",
		"10-source.go":  "package fixture\n",
	})
	triggered := false
	err := walkProductionGoFiles(root, func(path string, _ []byte) error {
		if filepath.Base(path) != "00-trigger.go" {
			return nil
		}
		triggered = true
		victim := filepath.Join(root, "10-source.go")
		if err := os.Remove(victim); err != nil {
			return err
		}
		// The listed entry is still a source file, but reading it now fails.
		return os.Mkdir(victim, 0o700)
	})
	if !triggered || err == nil {
		t.Fatalf("source read failure must not be ignored: triggered=%v, err=%v", triggered, err)
	}
}

func TestProductionGoWalkFailsOnMissingRoot(t *testing.T) {
	called := false
	err := walkProductionGoFiles(filepath.Join(t.TempDir(), "missing"), func(string, []byte) error {
		called = true
		return nil
	})
	if called || !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("missing scan root must fail closed: called=%v, err=%v", called, err)
	}
}

func TestProductionGoWalkPropagatesVisitorError(t *testing.T) {
	root := t.TempDir()
	writeSourceWalkFixture(t, root, map[string]string{
		"00-source.go": "package fixture\n",
		"10-source.go": "package fixture\n",
	})
	wantErr := errors.New("policy audit failed")
	visits := 0
	err := walkProductionGoFiles(root, func(string, []byte) error {
		visits++
		return wantErr
	})
	if !errors.Is(err, wantErr) || visits != 1 {
		t.Fatalf("visitor failure must stop the audit: visits=%d, err=%v", visits, err)
	}
}
