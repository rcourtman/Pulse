//go:build !windows

package alerts

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// Equal size and safe permissions do not establish checkpoint identity. A
// valid but stale record must be repaired even when metadata looks unchanged.
func TestActiveMirrorRepairsSameSizeStaleContent(t *testing.T) {
	dir := t.TempDir()
	m := &Manager{alertsDir: dir}
	alerts := []*Alert{{ID: "a", Acknowledged: true, AckUser: "operator"}}
	save := func() {
		t.Helper()
		if err := m.writeActiveAlertsRecoveryMirror(alerts); err != nil {
			t.Fatal(err)
		}
	}
	save()
	path := filepath.Join(dir, "active-alerts.json")
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	stale := bytes.Replace(want, []byte("operator"), []byte("previous"), 1)
	if bytes.Equal(stale, want) || len(stale) != len(want) {
		t.Fatal("fixture must change content without changing size")
	}
	if err := os.WriteFile(path, stale, alertsFilePerm); err != nil {
		t.Fatal(err)
	}
	save()
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatal("same-size stale acknowledgement survived checkpoint")
	}
	repaired, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	save()
	after, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if !os.SameFile(repaired, after) {
		t.Fatal("repaired checkpoint did not return to unchanged fast path")
	}
}

func TestActiveMirrorUnchangedRepairsUnsafeDestination(t *testing.T) {
	for _, kind := range []string{"permissions", "symlink"} {
		t.Run(kind, func(t *testing.T) {
			dir := t.TempDir()
			m := &Manager{alertsDir: dir}
			alerts := []*Alert{{ID: "a"}}
			save := func() {
				t.Helper()
				if err := m.writeActiveAlertsRecoveryMirror(alerts); err != nil {
					t.Fatal(err)
				}
			}
			save()
			path := filepath.Join(dir, "active-alerts.json")
			target := path + ".target"
			if kind == "permissions" {
				if err := os.Chmod(path, 0644); err != nil {
					t.Fatal(err)
				}
			} else {
				if err := os.Rename(path, target); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(target, path); err != nil {
					t.Fatal(err)
				}
			}
			save()
			info, err := os.Lstat(path)
			if err != nil {
				t.Fatal(err)
			}
			if !info.Mode().IsRegular() || info.Mode().Perm() != alertsFilePerm {
				t.Fatalf("unsafe mirror accepted: %v", info.Mode())
			}
			if kind == "symlink" {
				original, err := os.Stat(target)
				if err != nil {
					t.Fatal(err)
				}
				if os.SameFile(original, info) {
					t.Fatal("symlink target reused")
				}
			}
		})
	}
}
