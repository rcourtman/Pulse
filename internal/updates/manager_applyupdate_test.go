package updates

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func writeStub(t *testing.T, dir, name, script string) {
	t.Helper()

	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(script), 0755); err != nil {
		t.Fatalf("write stub %s: %v", name, err)
	}
}

func copyFile(src, dest string, mode os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(out, in)
	closeErr := out.Close()
	if copyErr != nil {
		return copyErr
	}
	if closeErr != nil {
		return closeErr
	}
	if mode != 0 {
		return os.Chmod(dest, mode)
	}
	return nil
}

func TestManagerApplyUpdateFilesMissingBinary(t *testing.T) {
	manager := &Manager{}

	if err := manager.applyUpdateFiles(t.TempDir()); err == nil {
		t.Fatal("expected error for missing pulse binary")
	}
}

func TestManagerApplyUpdateFilesCopiesPulseBinary(t *testing.T) {
	manager := &Manager{}

	cpPath, err := exec.LookPath("cp")
	if err != nil {
		t.Fatalf("find cp: %v", err)
	}
	mvPath, err := exec.LookPath("mv")
	if err != nil {
		t.Fatalf("find mv: %v", err)
	}

	stubDir := t.TempDir()
	writeStub(t, stubDir, "cp", fmt.Sprintf("#!/bin/sh\nexec %s \"$@\"\n", cpPath))
	writeStub(t, stubDir, "mv", fmt.Sprintf("#!/bin/sh\nexec %s \"$@\"\n", mvPath))
	writeStub(t, stubDir, "chown", "#!/bin/sh\nexit 0\n")
	t.Setenv("PATH", stubDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	binaryPath, err := os.Executable()
	if err != nil {
		t.Fatalf("executable path: %v", err)
	}
	info, err := os.Stat(binaryPath)
	if err != nil {
		t.Fatalf("stat executable: %v", err)
	}
	backup := filepath.Join(t.TempDir(), "orig-binary")
	if err := copyFile(binaryPath, backup, info.Mode()); err != nil {
		t.Fatalf("backup binary: %v", err)
	}
	t.Cleanup(func() {
		if err := copyFile(backup, binaryPath, info.Mode()); err != nil {
			t.Fatalf("restore binary: %v", err)
		}
	})

	extractDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(extractDir, "pulse"), []byte("newbinary"), 0755); err != nil {
		t.Fatalf("write pulse: %v", err)
	}
	if err := manager.applyUpdateFiles(extractDir); err != nil {
		t.Fatalf("applyUpdateFiles root pulse: %v", err)
	}
	data, err := os.ReadFile(binaryPath)
	if err != nil {
		t.Fatalf("read replaced binary: %v", err)
	}
	if string(data) != "newbinary" {
		t.Fatalf("unexpected root binary contents: %q", string(data))
	}

	extractDir = t.TempDir()
	if err := os.MkdirAll(filepath.Join(extractDir, "bin"), 0755); err != nil {
		t.Fatalf("mkdir bin: %v", err)
	}
	if err := os.WriteFile(filepath.Join(extractDir, "bin", "pulse"), []byte("newbinary2"), 0755); err != nil {
		t.Fatalf("write pulse bin: %v", err)
	}
	if err := manager.applyUpdateFiles(extractDir); err != nil {
		t.Fatalf("applyUpdateFiles bin pulse: %v", err)
	}
	data, err = os.ReadFile(binaryPath)
	if err != nil {
		t.Fatalf("read replaced binary (bin): %v", err)
	}
	if string(data) != "newbinary2" {
		t.Fatalf("unexpected bin binary contents: %q", string(data))
	}
}
