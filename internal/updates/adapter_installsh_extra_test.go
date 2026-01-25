package updates

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func writeCurlStub(t *testing.T, dir string) {
	t.Helper()

	cpPath, err := exec.LookPath("cp")
	if err != nil {
		t.Fatalf("find cp: %v", err)
	}

	script := fmt.Sprintf(`#!/bin/sh
set -e
out=""
url=""
while [ "$#" -gt 0 ]; do
  case "$1" in
    -o) out="$2"; shift 2;;
    *) url="$1"; shift;;
  esac
done
if [ -z "$out" ]; then
  exit 1
fi
if echo "$url" | grep -q "\.sha256$"; then
  exec %s "$PULSE_TEST_SHA" "$out"
fi
exec %s "$PULSE_TEST_FILE" "$out"
`, cpPath, cpPath)

	writeStub(t, dir, "curl", script)
}

func TestInstallShAdapterDetectServiceName(t *testing.T) {
	stubDir := t.TempDir()
	writeStub(t, stubDir, "systemctl", `#!/bin/sh
if [ "$1" = "is-active" ] && [ "$2" = "pulse-backend" ]; then
  echo "active"
  exit 0
fi
exit 1
`)
	t.Setenv("PATH", stubDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	adapter := &InstallShAdapter{}
	name, err := adapter.detectServiceName()
	if err != nil {
		t.Fatalf("detectServiceName error: %v", err)
	}
	if name != "pulse-backend" {
		t.Fatalf("expected pulse-backend, got %s", name)
	}

	writeStub(t, stubDir, "systemctl", `#!/bin/sh
echo "inactive"
exit 0
`)
	name, err = adapter.detectServiceName()
	if err != nil {
		t.Fatalf("detectServiceName error: %v", err)
	}
	if name != "pulse" {
		t.Fatalf("expected default pulse, got %s", name)
	}
}

func TestInstallShAdapterDownloadInstallScript(t *testing.T) {
	content := []byte("#!/bin/sh\necho ok\n")
	sum := sha256.Sum256(content)

	tmpDir := t.TempDir()
	scriptPath := filepath.Join(tmpDir, "install.sh")
	if err := os.WriteFile(scriptPath, content, 0600); err != nil {
		t.Fatalf("write install.sh: %v", err)
	}
	checksumPath := filepath.Join(tmpDir, "install.sh.sha256")
	if err := os.WriteFile(checksumPath, []byte(fmt.Sprintf("%x  install.sh\n", sum)), 0600); err != nil {
		t.Fatalf("write checksum: %v", err)
	}

	stubDir := t.TempDir()
	writeCurlStub(t, stubDir)
	t.Setenv("PATH", stubDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("PULSE_TEST_FILE", scriptPath)
	t.Setenv("PULSE_TEST_SHA", checksumPath)

	adapter := NewInstallShAdapter(nil)
	got, err := adapter.downloadInstallScript(context.Background())
	if err != nil {
		t.Fatalf("downloadInstallScript error: %v", err)
	}
	if got != string(content) {
		t.Fatalf("unexpected script content: %q", got)
	}

	if err := os.WriteFile(checksumPath, []byte("deadbeef"), 0600); err != nil {
		t.Fatalf("write checksum: %v", err)
	}
	if _, err := adapter.downloadInstallScript(context.Background()); err == nil {
		t.Fatalf("expected checksum error")
	}
}

func TestInstallShAdapterDownloadBinary(t *testing.T) {
	extractDir := t.TempDir()
	tarball := filepath.Join(extractDir, "pulse-v1.2.3-linux-amd64.tar.gz")
	writeTarGz(t, tarball, map[string]string{
		"bin/pulse": "binary",
	})

	data, err := os.ReadFile(tarball)
	if err != nil {
		t.Fatalf("read tarball: %v", err)
	}
	sum := sha256.Sum256(data)
	checksumPath := tarball + ".sha256"
	if err := os.WriteFile(checksumPath, []byte(fmt.Sprintf("%x  %s\n", sum, filepath.Base(tarball))), 0600); err != nil {
		t.Fatalf("write checksum: %v", err)
	}

	stubDir := t.TempDir()
	writeCurlStub(t, stubDir)
	t.Setenv("PATH", stubDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("PULSE_TEST_FILE", tarball)
	t.Setenv("PULSE_TEST_SHA", checksumPath)

	adapter := &InstallShAdapter{}
	binaryPath, err := adapter.downloadBinary(context.Background(), "1.2.3")
	if err != nil {
		t.Fatalf("downloadBinary error: %v", err)
	}
	got, err := os.ReadFile(binaryPath)
	if err != nil {
		t.Fatalf("read binary: %v", err)
	}
	if string(got) != "binary" {
		t.Fatalf("unexpected binary content: %q", string(got))
	}

	if err := os.WriteFile(checksumPath, []byte("deadbeef"), 0600); err != nil {
		t.Fatalf("write checksum: %v", err)
	}
	if _, err := adapter.downloadBinary(context.Background(), "1.2.3"); err == nil {
		t.Fatalf("expected checksum mismatch error")
	}
}

func TestInstallShAdapterReadLastLines(t *testing.T) {
	adapter := &InstallShAdapter{}
	if adapter.readLastLines(filepath.Join(t.TempDir(), "missing"), 2) != "" {
		t.Fatalf("expected empty for missing file")
	}

	path := filepath.Join(t.TempDir(), "log.txt")
	if err := os.WriteFile(path, []byte("a\nb\nc\n"), 0600); err != nil {
		t.Fatalf("write log: %v", err)
	}
	got := adapter.readLastLines(path, 2)
	if got != "b\nc" {
		t.Fatalf("unexpected lines: %q", got)
	}
	if adapter.readLastLines(path, 0) != "" {
		t.Fatalf("expected empty for zero lines")
	}
}

func TestInstallShAdapterRestoreConfigAndInstallBinary(t *testing.T) {
	cpPath, err := exec.LookPath("cp")
	if err != nil {
		t.Fatalf("find cp: %v", err)
	}
	stubDir := t.TempDir()
	writeStub(t, stubDir, "cp", fmt.Sprintf("#!/bin/sh\nexec %s \"$@\"\n", cpPath))
	writeStub(t, stubDir, "chown", "#!/bin/sh\nexit 0\n")
	t.Setenv("PATH", stubDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	adapter := &InstallShAdapter{}

	srcDir := filepath.Join(t.TempDir(), "backup")
	destDir := filepath.Join(t.TempDir(), "config")
	if err := os.MkdirAll(srcDir, 0755); err != nil {
		t.Fatalf("mkdir src: %v", err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "app.conf"), []byte("ok"), 0600); err != nil {
		t.Fatalf("write src file: %v", err)
	}
	if err := os.MkdirAll(destDir, 0755); err != nil {
		t.Fatalf("mkdir dest: %v", err)
	}

	if err := adapter.restoreConfig(context.Background(), srcDir, destDir); err != nil {
		t.Fatalf("restoreConfig error: %v", err)
	}
	if _, err := os.Stat(filepath.Join(destDir, "app.conf")); err != nil {
		t.Fatalf("expected config restored: %v", err)
	}

	srcBinary := filepath.Join(t.TempDir(), "pulse")
	if err := os.WriteFile(srcBinary, []byte("bin"), 0755); err != nil {
		t.Fatalf("write source binary: %v", err)
	}
	targetBinary := filepath.Join(t.TempDir(), "pulse")
	if err := os.WriteFile(targetBinary, []byte("old"), 0755); err != nil {
		t.Fatalf("write target binary: %v", err)
	}

	if err := adapter.installBinary(context.Background(), srcBinary, targetBinary); err != nil {
		t.Fatalf("installBinary error: %v", err)
	}
	if _, err := os.Stat(targetBinary + ".pre-rollback"); err != nil {
		t.Fatalf("expected backup binary: %v", err)
	}
}

func TestInstallShAdapterWaitForHealth(t *testing.T) {
	stubDir := t.TempDir()
	writeStub(t, stubDir, "curl", "#!/bin/sh\nexit 0\n")
	t.Setenv("PATH", stubDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	adapter := &InstallShAdapter{}
	if err := adapter.waitForHealth(context.Background(), time.Second); err != nil {
		t.Fatalf("waitForHealth success error: %v", err)
	}

	writeStub(t, stubDir, "curl", "#!/bin/sh\nexit 1\n")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := adapter.waitForHealth(ctx, time.Second); err == nil || !strings.Contains(err.Error(), "canceled") {
		t.Fatalf("expected context error, got %v", err)
	}

	if err := adapter.waitForHealth(context.Background(), 0); err == nil {
		t.Fatalf("expected timeout error")
	}
}

func TestInstallShAdapterExecuteRollbackDownloadError(t *testing.T) {
	stubDir := t.TempDir()
	writeStub(t, stubDir, "systemctl", "#!/bin/sh\nexit 0\n")
	writeStub(t, stubDir, "curl", "#!/bin/sh\nexit 1\n")
	t.Setenv("PATH", stubDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	adapter := &InstallShAdapter{}
	entry := &UpdateHistoryEntry{BackupPath: t.TempDir()}
	if err := adapter.executeRollback(context.Background(), entry, "1.2.3"); err == nil {
		t.Fatalf("expected download error")
	}
}
