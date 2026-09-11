//go:build !windows

package alerts

import (
	"os"
	"path/filepath"
	"testing"
)

func TestIntentCheckpointRepairsUnsafeDestination(t *testing.T) {
	for _, kind := range []string{"permissions", "symlink"} {
		t.Run(kind, func(t *testing.T) {
			m := &Manager{alertsDir: t.TempDir()}
			save := func() {
				t.Helper()
				if err := m.SaveActiveAlerts(); err != nil {
					t.Fatal(err)
				}
			}
			save()
			path := filepath.Join(m.alertsDir, intentPendingFileName)
			if kind == "permissions" {
				if err := os.Chmod(path, 0644); err != nil {
					t.Fatal(err)
				}
			} else {
				if err := os.Rename(path, path+".target"); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(path+".target", path); err != nil {
					t.Fatal(err)
				}
			}
			save()
			info, err := os.Lstat(path)
			if err != nil {
				t.Fatal(err)
			}
			if !info.Mode().IsRegular() || info.Mode().Perm() != alertsFilePerm {
				t.Fatalf("unsafe state accepted: %v", info.Mode())
			}
		})
	}
}
