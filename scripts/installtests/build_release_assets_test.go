package installtests

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildReleaseUsesV6InstallScripts(t *testing.T) {
	content, err := os.ReadFile(repoFile("scripts", "build-release.sh"))
	if err != nil {
		t.Fatalf("read build-release.sh: %v", err)
	}

	script := string(content)
	required := []string{
		`cp scripts/install.sh "$RELEASE_DIR/install.sh"`,
		`[ -f "scripts/install.ps1" ] && cp "scripts/install.ps1" "$RELEASE_DIR/install.ps1"`,
	}
	for _, needle := range required {
		if !strings.Contains(script, needle) {
			t.Fatalf("build-release.sh missing required release asset copy: %s", needle)
		}
	}

	if strings.Contains(script, `cp install.sh "$RELEASE_DIR/install.sh"`) {
		t.Fatal("build-release.sh still copies the legacy root install.sh into release assets")
	}
}

func TestCreateReleaseUploadsPowerShellInstaller(t *testing.T) {
	content, err := os.ReadFile(repoFile(".github", "workflows", "create-release.yml"))
	if err != nil {
		t.Fatalf("read create-release.yml: %v", err)
	}

	workflow := string(content)
	required := []string{
		`gh release upload "${TAG}" release/install.sh --clobber`,
		`if [ -f release/install.ps1 ]; then`,
		`gh release upload "${TAG}" release/install.ps1 --clobber`,
	}
	for _, needle := range required {
		if !strings.Contains(workflow, needle) {
			t.Fatalf("create-release.yml missing required installer upload step: %s", needle)
		}
	}
}

func repoFile(parts ...string) string {
	root := filepath.Join("..", "..")
	segments := append([]string{root}, parts...)
	return filepath.Join(segments...)
}
