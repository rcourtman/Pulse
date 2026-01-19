package updates

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstallShAdapter_DetectServiceName(t *testing.T) {
	dir := t.TempDir()
	systemctl := filepath.Join(dir, "systemctl")
	script := `#!/bin/sh
if [ "$1" = "is-active" ] && [ "$2" = "pulse-backend" ]; then
  echo "active"
  exit 0
fi
echo "inactive"
exit 0
`
	if err := os.WriteFile(systemctl, []byte(script), 0755); err != nil {
		t.Fatalf("write systemctl: %v", err)
	}

	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

	adapter := &InstallShAdapter{}
	name, err := adapter.detectServiceName()
	if err != nil {
		t.Fatalf("detectServiceName error: %v", err)
	}
	if name != "pulse-backend" {
		t.Fatalf("expected pulse-backend, got %q", name)
	}
}

func TestInstallShAdapter_DownloadInstallScript(t *testing.T) {
	content := "echo hi"
	sum := sha256.Sum256([]byte(content))
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
		`  echo "` + checksum + `  install.sh" > "$out"`,
		`else`,
		`  printf '%s' "` + content + `" > "$out"`,
		`fi`,
		``,
	}, "\n")
	if err := os.WriteFile(curl, []byte(script), 0755); err != nil {
		t.Fatalf("write curl: %v", err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

	adapter := &InstallShAdapter{installScriptURL: "http://example/install.sh"}
	out, err := adapter.downloadInstallScript(context.Background())
	if err != nil {
		t.Fatalf("downloadInstallScript error: %v", err)
	}
	if out != content {
		t.Fatalf("unexpected script content: %q", out)
	}
}
