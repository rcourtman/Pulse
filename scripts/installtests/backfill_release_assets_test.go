package installtests

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestBackfillReleaseAssetsRewritesIntegrityMetadataLocally(t *testing.T) {
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash not installed")
	}
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go not installed")
	}
	if _, err := exec.LookPath("ssh-keygen"); err != nil {
		t.Skip("ssh-keygen not installed")
	}

	tag := "v0.0.0-test"
	releaseDir := t.TempDir()

	writeReleaseFile(t, releaseDir, "pulse-v0.0.0-test.tar.gz", "payload")
	writeReleaseFile(t, releaseDir, "install.sh", "#!/bin/sh\necho test\n")
	writeHistoricalChecksums(t, releaseDir, []string{
		"install.sh",
		"pulse-v0.0.0-test.tar.gz",
	})

	syftStub := filepath.Join(t.TempDir(), "syft")
	if err := os.WriteFile(syftStub, []byte(`#!/bin/sh
set -eu
out=""
for arg in "$@"; do
  case "$arg" in
    spdx-json=*)
      out=${arg#spdx-json=}
      ;;
  esac
done
[ -n "$out" ] || exit 1
printf '{"spdxVersion":"SPDX-2.3","files":[]}\n' > "$out"
`), 0o755); err != nil {
		t.Fatalf("write syft stub: %v", err)
	}

	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate signing key: %v", err)
	}
	encodedPrivateKey := base64.StdEncoding.EncodeToString(privateKey)

	runBackfill := func() {
		cmd := exec.Command("bash", "scripts/backfill-release-assets.sh",
			"--tag", tag,
			"--release-dir", releaseDir,
			"--skip-download",
			"--skip-upload",
		)
		repoRootCmd := exec.Command("go", "env", "GOMOD")
		gomodOutput, err := repoRootCmd.Output()
		if err != nil {
			t.Fatalf("resolve repo root via go env GOMOD: %v", err)
		}
		repoRoot := filepath.Dir(strings.TrimSpace(string(gomodOutput)))
		repoRoot, err = filepath.Abs(repoRoot)
		if err != nil {
			t.Fatalf("resolve repo root: %v", err)
		}
		cmd.Dir = repoRoot
		cmd.Env = append(os.Environ(),
			"PULSE_UPDATE_SIGNING_KEY="+encodedPrivateKey,
			"PULSE_RELEASE_SBOM_TOOL="+syftStub,
		)
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("backfill-release-assets.sh failed: %v\n%s", err, output)
		}
	}

	runBackfill()
	runBackfill()

	checksumsPath := filepath.Join(releaseDir, "checksums.txt")
	checksumsBytes, err := os.ReadFile(checksumsPath)
	if err != nil {
		t.Fatalf("read checksums.txt: %v", err)
	}

	releaseSBOM := "pulse-v0.0.0-test-release.sbom.spdx.json"
	lines := strings.Split(strings.TrimSpace(string(checksumsBytes)), "\n")
	if len(lines) != 3 {
		t.Fatalf("expected 3 checksummed artifacts after backfill, got %d:\n%s", len(lines), checksumsBytes)
	}

	if _, err := os.Stat(filepath.Join(releaseDir, releaseSBOM)); err != nil {
		t.Fatalf("missing generated release SBOM: %v", err)
	}
	if _, err := os.Stat(filepath.Join(releaseDir, "checksums.txt.sshsig")); err != nil {
		t.Fatalf("missing checksums.txt.sshsig: %v", err)
	}
	if _, err := os.Stat(filepath.Join(releaseDir, "checksums.txt.sig")); err != nil {
		t.Fatalf("missing checksums.txt.sig: %v", err)
	}

	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) != 2 {
			t.Fatalf("malformed checksums line %q", line)
		}

		sum := fields[0]
		filename := fields[1]
		artifactPath := filepath.Join(releaseDir, filename)
		data, err := os.ReadFile(artifactPath)
		if err != nil {
			t.Fatalf("read %s: %v", filename, err)
		}
		actualSum := fmt.Sprintf("%x", sha256.Sum256(data))
		if actualSum != sum {
			t.Fatalf("checksum mismatch for %s: got %s want %s", filename, actualSum, sum)
		}

		shaFileBytes, err := os.ReadFile(artifactPath + ".sha256")
		if err != nil {
			t.Fatalf("read %s.sha256: %v", filename, err)
		}
		if strings.TrimSpace(string(shaFileBytes)) != line {
			t.Fatalf("unexpected %s.sha256 contents: %q", filename, shaFileBytes)
		}

		if _, err := os.Stat(artifactPath + ".sig"); err != nil {
			t.Fatalf("missing %s.sig: %v", filename, err)
		}
		if _, err := os.Stat(artifactPath + ".sshsig"); err != nil {
			t.Fatalf("missing %s.sshsig: %v", filename, err)
		}
	}
}

func writeHistoricalChecksums(t *testing.T, releaseDir string, filenames []string) {
	t.Helper()

	lines := make([]string, 0, len(filenames))
	for _, filename := range filenames {
		data, err := os.ReadFile(filepath.Join(releaseDir, filename))
		if err != nil {
			t.Fatalf("read %s: %v", filename, err)
		}
		line := fmt.Sprintf("%x  %s", sha256.Sum256(data), filename)
		lines = append(lines, line)
		if err := os.WriteFile(filepath.Join(releaseDir, filename+".sha256"), []byte(line+"\n"), 0o644); err != nil {
			t.Fatalf("write %s.sha256: %v", filename, err)
		}
	}

	if err := os.WriteFile(filepath.Join(releaseDir, "checksums.txt"), []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatalf("write checksums.txt: %v", err)
	}
}

func writeReleaseFile(t *testing.T, releaseDir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(releaseDir, name), []byte(content), 0o755); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}
