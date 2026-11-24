package installtests

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestRenderHomebrewFormulaRendersVerifiedDigests(t *testing.T) {
	releaseDir := t.TempDir()
	outPath := filepath.Join(t.TempDir(), "Formula", "pulse.rb")

	fixtures := map[string]string{
		"pulse-cli-v6.9.0-darwin-arm64.tar.gz": "darwin-arm64-bytes",
		"pulse-cli-v6.9.0-darwin-amd64.tar.gz": "darwin-amd64-bytes",
		"pulse-cli-v6.9.0-linux-arm64.tar.gz":  "linux-arm64-bytes",
		"pulse-cli-v6.9.0-linux-amd64.tar.gz":  "linux-amd64-bytes",
	}
	expectedDigests := make(map[string]string, len(fixtures))
	for name, body := range fixtures {
		if err := os.WriteFile(filepath.Join(releaseDir, name), []byte(body), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
		sum := sha256.Sum256([]byte(body))
		expectedDigests[name] = hex.EncodeToString(sum[:])
	}

	cmd := exec.Command(
		"bash",
		repoFile("scripts", "render-homebrew-formula.sh"),
		"--version", "v6.9.0",
		"--release-dir", releaseDir,
		"--output", outPath,
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("render-homebrew-formula.sh failed: %v\n%s", err, out)
	}

	renderedBytes, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read rendered formula: %v", err)
	}
	rendered := string(renderedBytes)

	if strings.Contains(rendered, "{{") {
		t.Fatalf("rendered formula still contains template placeholders:\n%s", rendered)
	}
	for _, needle := range []string{
		`version "6.9.0"`,
		`pulse-cli-v6.9.0-darwin-arm64.tar.gz`,
		`pulse-cli-v6.9.0-darwin-amd64.tar.gz`,
		`pulse-cli-v6.9.0-linux-arm64.tar.gz`,
		`pulse-cli-v6.9.0-linux-amd64.tar.gz`,
		expectedDigests["pulse-cli-v6.9.0-darwin-arm64.tar.gz"],
		expectedDigests["pulse-cli-v6.9.0-darwin-amd64.tar.gz"],
		expectedDigests["pulse-cli-v6.9.0-linux-arm64.tar.gz"],
		expectedDigests["pulse-cli-v6.9.0-linux-amd64.tar.gz"],
	} {
		if !strings.Contains(rendered, needle) {
			t.Fatalf("rendered formula missing %q:\n%s", needle, rendered)
		}
	}
}

func TestRenderHomebrewFormulaRejectsPrereleaseAndMissingArchive(t *testing.T) {
	releaseDir := t.TempDir()
	outPath := filepath.Join(t.TempDir(), "pulse.rb")

	preCmd := exec.Command(
		"bash",
		repoFile("scripts", "render-homebrew-formula.sh"),
		"--version", "v6.9.0-rc.4",
		"--release-dir", releaseDir,
		"--output", outPath,
	)
	if out, err := preCmd.CombinedOutput(); err == nil {
		t.Fatalf("expected prerelease version to be rejected, got success:\n%s", out)
	}

	missingCmd := exec.Command(
		"bash",
		repoFile("scripts", "render-homebrew-formula.sh"),
		"--version", "6.9.0",
		"--release-dir", releaseDir,
		"--output", outPath,
	)
	if out, err := missingCmd.CombinedOutput(); err == nil {
		t.Fatalf("expected missing tarballs to fail closed, got success:\n%s", out)
	}
}

func TestPublishHomebrewTapWorkflowContract(t *testing.T) {
	raw, err := os.ReadFile(repoFile(".github", "workflows", "publish-homebrew-tap.yml"))
	if err != nil {
		t.Fatalf("read publish-homebrew-tap.yml: %v", err)
	}
	var parsed map[string]any
	if err := yaml.Unmarshal(raw, &parsed); err != nil {
		t.Fatalf("publish-homebrew-tap.yml is not valid YAML: %v", err)
	}
	text := string(raw)
	for _, needle := range []string{
		"scripts/render-homebrew-formula.sh",
		"Formula/pulse.rb",
		"checksums.txt",
		"HOMEBREW_TAP_TOKEN",
		"rcourtman/homebrew-tap",
	} {
		if !strings.Contains(text, needle) {
			t.Fatalf("publish-homebrew-tap.yml missing %q", needle)
		}
	}
}
