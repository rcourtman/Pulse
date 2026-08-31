package installtests

import (
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

const productionMCPReleaseKey = "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIMZd/DaH+BldzOkq1A8KVTcFk73nAyrE8aJOyf7i00jm pulse-installer"

func TestInstallMCPRequiresSignedChecksumEvidence(t *testing.T) {
	for _, command := range []string{"bash", "ssh-keygen"} {
		if _, err := exec.LookPath(command); err != nil {
			t.Skipf("%s not installed", command)
		}
	}

	binaryName := "pulse-mcp-" + runtime.GOOS + "-" + runtime.GOARCH
	tests := []struct {
		name         string
		manifest     func(binary []byte) string
		invalidSig   bool
		missingAsset string
		wantSuccess  bool
		wantOutput   string
	}{
		{
			name: "valid signed manifest",
			manifest: func(binary []byte) string {
				digest := sha256.Sum256(binary)
				return fmt.Sprintf("%x  %s\n", digest, binaryName)
			},
			wantSuccess: true,
			wantOutput:  "release signature verified",
		},
		{
			name: "manifest unavailable",
			manifest: func(binary []byte) string {
				digest := sha256.Sum256(binary)
				return fmt.Sprintf("%x  %s\n", digest, binaryName)
			},
			missingAsset: "checksums.txt",
			wantOutput:   "could not fetch checksums.txt; refusing unverified install",
		},
		{
			name: "signature unavailable",
			manifest: func(binary []byte) string {
				digest := sha256.Sum256(binary)
				return fmt.Sprintf("%x  %s\n", digest, binaryName)
			},
			missingAsset: "checksums.txt.sshsig",
			wantOutput:   "could not fetch checksums.txt.sshsig; refusing unverified install",
		},
		{
			name: "signature invalid",
			manifest: func(binary []byte) string {
				digest := sha256.Sum256(binary)
				return fmt.Sprintf("%x  %s\n", digest, binaryName)
			},
			invalidSig: true,
			wantOutput: "cryptographic signature verification failed for checksums.txt",
		},
		{
			name: "binary omitted",
			manifest: func(binary []byte) string {
				digest := sha256.Sum256(binary)
				return fmt.Sprintf("%x  another-file\n", digest)
			},
			wantOutput: "checksums.txt must contain exactly one valid SHA256 entry",
		},
		{
			name: "binary duplicated",
			manifest: func(binary []byte) string {
				digest := sha256.Sum256(binary)
				return fmt.Sprintf("%x  %s\n%x  %s\n", digest, binaryName, digest, binaryName)
			},
			wantOutput: "checksums.txt must contain exactly one valid SHA256 entry",
		},
		{
			name: "digest mismatch",
			manifest: func(_ []byte) string {
				return strings.Repeat("0", 64) + "  " + binaryName + "\n"
			},
			wantOutput: "sha256 mismatch for " + binaryName,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmp := t.TempDir()
			fixtureDir := filepath.Join(tmp, "fixtures")
			binDir := filepath.Join(tmp, "bin")
			installDir := filepath.Join(tmp, "install")
			for _, dir := range []string{fixtureDir, binDir, installDir} {
				if err := os.MkdirAll(dir, 0o755); err != nil {
					t.Fatalf("mkdir %s: %v", dir, err)
				}
			}

			binary := []byte("test pulse-mcp executable\n")
			writeTestFile(t, filepath.Join(fixtureDir, binaryName), binary, 0o644)
			manifestPath := filepath.Join(fixtureDir, "checksums.txt")
			writeTestFile(t, manifestPath, []byte(tt.manifest(binary)), 0o644)

			privateKeyPath := filepath.Join(tmp, "release-key")
			runTestCommand(t, exec.Command("ssh-keygen", "-q", "-t", "ed25519", "-N", "", "-C", "pulse-installer", "-f", privateKeyPath))
			publicKeyBytes, err := os.ReadFile(privateKeyPath + ".pub")
			if err != nil {
				t.Fatalf("read test public key: %v", err)
			}
			publicKey := strings.TrimSpace(string(publicKeyBytes))

			sign := exec.Command("ssh-keygen", "-q", "-Y", "sign", "-f", privateKeyPath, "-n", "pulse-install", manifestPath)
			runTestCommand(t, sign)
			if err := os.Rename(manifestPath+".sig", filepath.Join(fixtureDir, "checksums.txt.sshsig")); err != nil {
				t.Fatalf("rename signature: %v", err)
			}
			if tt.invalidSig {
				writeTestFile(t, filepath.Join(fixtureDir, "checksums.txt.sshsig"), []byte("not a signature\n"), 0o644)
			}

			scriptBytes, err := os.ReadFile(repoFile("scripts", "install-mcp.sh"))
			if err != nil {
				t.Fatalf("read installer: %v", err)
			}
			script := strings.Replace(string(scriptBytes), productionMCPReleaseKey, publicKey, 1)
			if script == string(scriptBytes) {
				t.Fatal("installer no longer contains the expected pinned production key")
			}
			scriptPath := filepath.Join(tmp, "install-mcp.sh")
			writeTestFile(t, scriptPath, []byte(script), 0o755)

			curlStub := `#!/bin/sh
set -eu
output=''
url=''
while [ "$#" -gt 0 ]; do
  case "$1" in
    -o) output="$2"; shift 2 ;;
    http*) url="$1"; shift ;;
    *) shift ;;
  esac
done
name="${url##*/}"
if [ -n "${MISSING_ASSET:-}" ] && [ "$name" = "$MISSING_ASSET" ]; then
  exit 22
fi
cp "$FIXTURE_DIR/$name" "$output"
`
			writeTestFile(t, filepath.Join(binDir, "curl"), []byte(curlStub), 0o755)

			cmd := exec.Command("bash", scriptPath)
			cmd.Env = append(os.Environ(),
				"PATH="+binDir+string(os.PathListSeparator)+os.Getenv("PATH"),
				"HOME="+tmp,
				"FIXTURE_DIR="+fixtureDir,
				"MISSING_ASSET="+tt.missingAsset,
				"PULSE_MCP_BIN_DIR="+installDir,
				"PULSE_MCP_VERSION=v-test",
			)
			output, runErr := cmd.CombinedOutput()
			if tt.wantSuccess && runErr != nil {
				t.Fatalf("installer failed: %v\n%s", runErr, output)
			}
			if !tt.wantSuccess && runErr == nil {
				t.Fatalf("installer unexpectedly succeeded:\n%s", output)
			}
			if !strings.Contains(string(output), tt.wantOutput) {
				t.Fatalf("installer output missing %q:\n%s", tt.wantOutput, output)
			}

			installedPath := filepath.Join(installDir, "pulse-mcp")
			_, statErr := os.Stat(installedPath)
			if tt.wantSuccess && statErr != nil {
				t.Fatalf("verified binary was not installed: %v", statErr)
			}
			if !tt.wantSuccess && !os.IsNotExist(statErr) {
				t.Fatalf("unverified binary reached install destination: stat err=%v", statErr)
			}
		})
	}
}

func TestInstallMCPPowerShellFailsClosed(t *testing.T) {
	path := repoFile("scripts", "install-mcp.ps1")
	assertFileContainsAll(t, path,
		"$PinnedReleaseSshPublicKey = 'ssh-ed25519 ",
		"Get-SshKeygenPath",
		"Assert-ChecksumManifestSignature $manifestPath $signaturePath",
		"could not fetch checksums.txt; refusing unverified install",
		"could not fetch checksums.txt.sshsig; refusing unverified install",
		"checksums.txt must contain exactly one valid SHA256 entry",
		"sha256 mismatch for ${binaryName}",
	)
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read PowerShell installer: %v", err)
	}
	for _, insecure := range []string{"PULSE_MCP_NO_VERIFY", "NoVerify", "skipping verification"} {
		if strings.Contains(string(content), insecure) {
			t.Fatalf("PowerShell installer retains fail-open control %q", insecure)
		}
	}
}

func writeTestFile(t *testing.T, path string, content []byte, mode os.FileMode) {
	t.Helper()
	if err := os.WriteFile(path, content, mode); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func runTestCommand(t *testing.T, cmd *exec.Cmd) {
	t.Helper()
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("%s failed: %v\n%s", strings.Join(cmd.Args, " "), err, output)
	}
}
