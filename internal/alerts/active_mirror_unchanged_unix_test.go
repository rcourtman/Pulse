//go:build !windows

package alerts

import (
	"os"
	"path/filepath"
	"testing"
)

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
