//go:build linux

package alerts

import (
	"os"
	"syscall"
	"testing"
	"time"
)

// Skipping the JSON rename must not dirty the containing directory's metadata
// through an unconditional chmod on every checkpoint.
func TestActiveMirrorUnchangedPreservesDirectoryMetadata(t *testing.T) {
	dir := t.TempDir()
	m := &Manager{alertsDir: dir}
	alerts := []*Alert{{ID: "a"}}
	save := func() {
		t.Helper()
		if err := m.writeActiveAlertsRecoveryMirror(alerts); err != nil {
			t.Fatal(err)
		}
	}
	ctime := func() syscall.Timespec {
		t.Helper()
		info, err := os.Stat(dir)
		if err != nil {
			t.Fatal(err)
		}
		return info.Sys().(*syscall.Stat_t).Ctim
	}
	save()
	before := ctime()
	time.Sleep(20 * time.Millisecond)
	save()
	if after := ctime(); after != before {
		t.Fatalf("unchanged checkpoint changed directory ctime: %v -> %v", before, after)
	}
	// Still repair a directory made accessible to other users.
	if err := os.Chmod(dir, 0755); err != nil {
		t.Fatal(err)
	}
	save()
	info, err := os.Stat(dir)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != alertsDirPerm {
		t.Fatalf("directory mode = %v", info.Mode())
	}
}
