package updates

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestInstallShAdapter_DownloadBinary(t *testing.T) {
	tarball := filepath.Join(t.TempDir(), "pulse.tar.gz")
	writeTarGz(t, tarball, map[string]string{
		"bin/pulse": "binary",
	})

	data, err := os.ReadFile(tarball)
	if err != nil {
		t.Fatalf("read tarball: %v", err)
	}
	sum := sha256.Sum256(data)
	checksum := hex.EncodeToString(sum[:])

	dir := t.TempDir()
	curl := filepath.Join(dir, "curl")
	script := strings.Join([]string{
		"#!/bin/sh",
		`out=""`,
		`url=""`,
		`while [ "$#" -gt 0 ]; do`,
		`  if [ "$1" = "-o" ]; then`,
		`    out="$2"`,
		`    shift 2`,
		`    continue`,
		`  fi`,
		`  url="$1"`,
		`  shift`,
		`done`,
		`if echo "$url" | grep -q ".sha256$"; then`,
		`  echo "$PULSE_TEST_CHECKSUM  pulse.tar.gz" > "$out"`,
		`else`,
		`  cat "$PULSE_TEST_TARBALL" > "$out"`,
		`fi`,
		``,
	}, "\n")
	if err := os.WriteFile(curl, []byte(script), 0755); err != nil {
		t.Fatalf("write curl stub: %v", err)
	}

	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("PULSE_TEST_TARBALL", tarball)
	t.Setenv("PULSE_TEST_CHECKSUM", checksum)

	adapter := &InstallShAdapter{}
	binaryPath, err := adapter.downloadBinary(context.Background(), "1.2.3")
	if err != nil {
		t.Fatalf("downloadBinary error: %v", err)
	}
	t.Cleanup(func() {
		_ = os.RemoveAll(filepath.Dir(filepath.Dir(filepath.Dir(binaryPath))))
	})

	payload, err := os.ReadFile(binaryPath)
	if err != nil {
		t.Fatalf("read binary: %v", err)
	}
	if string(payload) != "binary" {
		t.Fatalf("unexpected binary content: %q", string(payload))
	}
}

func TestInstallShAdapter_WaitForHealth(t *testing.T) {
	dir := t.TempDir()
	curl := filepath.Join(dir, "curl")
	script := "#!/bin/sh\nexit 0\n"
	if err := os.WriteFile(curl, []byte(script), 0755); err != nil {
		t.Fatalf("write curl stub: %v", err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

	adapter := &InstallShAdapter{}
	if err := adapter.waitForHealth(context.Background(), 200*time.Millisecond); err != nil {
		t.Fatalf("waitForHealth error: %v", err)
	}
}

func TestInstallShAdapter_ServiceCommands(t *testing.T) {
	dir := t.TempDir()
	systemctl := filepath.Join(dir, "systemctl")
	script := "#!/bin/sh\nexit 0\n"
	if err := os.WriteFile(systemctl, []byte(script), 0755); err != nil {
		t.Fatalf("write systemctl stub: %v", err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

	adapter := &InstallShAdapter{}
	if err := adapter.stopService(context.Background(), "pulse"); err != nil {
		t.Fatalf("stopService error: %v", err)
	}
	if err := adapter.startService(context.Background(), "pulse"); err != nil {
		t.Fatalf("startService error: %v", err)
	}
}

func TestInstallShAdapter_RestoreConfig(t *testing.T) {
	backupDir := filepath.Join(t.TempDir(), "backup")
	targetDir := filepath.Join(t.TempDir(), "target")
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		t.Fatalf("mkdir backup: %v", err)
	}
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		t.Fatalf("mkdir target: %v", err)
	}
	if err := os.WriteFile(filepath.Join(backupDir, "config.txt"), []byte("ok"), 0600); err != nil {
		t.Fatalf("write backup file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(targetDir, "old.txt"), []byte("old"), 0600); err != nil {
		t.Fatalf("write target file: %v", err)
	}

	adapter := &InstallShAdapter{}
	if err := adapter.restoreConfig(context.Background(), backupDir, targetDir); err != nil {
		t.Fatalf("restoreConfig error: %v", err)
	}

	if _, err := os.Stat(filepath.Join(targetDir, "config.txt")); err != nil {
		t.Fatalf("expected restored file: %v", err)
	}
	if _, err := os.Stat(filepath.Join(targetDir, "old.txt")); err == nil {
		t.Fatal("expected old file to be removed")
	}
}

func TestInstallShAdapter_InstallBinary(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	if err := os.WriteFile(source, []byte("payload"), 0600); err != nil {
		t.Fatalf("write source: %v", err)
	}

	targetDir := filepath.Join(dir, "bin")
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		t.Fatalf("mkdir target: %v", err)
	}
	target := filepath.Join(targetDir, "pulse")
	if err := os.WriteFile(target, []byte("old"), 0600); err != nil {
		t.Fatalf("write target: %v", err)
	}

	chownDir := t.TempDir()
	chown := filepath.Join(chownDir, "chown")
	if err := os.WriteFile(chown, []byte("#!/bin/sh\nexit 0\n"), 0755); err != nil {
		t.Fatalf("write chown stub: %v", err)
	}
	t.Setenv("PATH", chownDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	adapter := &InstallShAdapter{}
	if err := adapter.installBinary(context.Background(), source, target); err != nil {
		t.Fatalf("installBinary error: %v", err)
	}

	if _, err := os.Stat(target + ".pre-rollback"); err != nil {
		t.Fatalf("expected backup file: %v", err)
	}
	content, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read target: %v", err)
	}
	if string(content) != "payload" {
		t.Fatalf("unexpected target content: %q", string(content))
	}
}
