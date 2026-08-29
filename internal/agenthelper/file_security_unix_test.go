//go:build unix

package agenthelper

import (
	"os"
	"path/filepath"
	"testing"
)

func TestStrictRootOwnedFileRejectsUnprivilegedOwner(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("test process is root")
	}
	path := filepath.Join(t.TempDir(), "artifact")
	if err := os.WriteFile(path, []byte("artifact"), 0o600); err != nil {
		t.Fatal(err)
	}
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	if err := StrictRootOwnedFile(file); err == nil {
		t.Fatal("unprivileged artifact owner accepted")
	}
}
