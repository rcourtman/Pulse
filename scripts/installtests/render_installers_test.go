package installtests

import (
	"encoding/base64"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
)

const renderedInstallerKeyPlaceholder = "__PULSE_INSTALLER_SSH_PUBLIC_KEY__"

func renderInstallersForTest(sourceDir, outputDir string, keyArgs ...string) ([]byte, error) {
	args := append([]string{"run", "./scripts/render_installers.go", "--source-dir", sourceDir, "--output-dir", outputDir}, keyArgs...)
	cmd := exec.Command("go", args...)
	cmd.Dir = repoFile()
	return cmd.CombinedOutput()
}

func renderedInstallShellFunction(t *testing.T, script, name string) string {
	t.Helper()
	match := regexp.MustCompile(`(?m)^` + regexp.QuoteMeta(name) + `\(\) \{\n(?s:.*?)^\}`).FindString(script)
	if match == "" {
		t.Fatalf("could not find %s in rendered install.sh", name)
	}
	return match
}

func renderedInstallPowerShellFunction(t *testing.T, script, name string) string {
	t.Helper()
	match := regexp.MustCompile(`(?m)^function ` + regexp.QuoteMeta(name) + ` \{\n(?s:.*?)^\}`).FindString(script)
	if match == "" {
		t.Fatalf("could not find %s in rendered install.ps1", name)
	}
	return match
}

// Render the installers with the release command line, then run the rendered
// install.sh and install.ps1 verification functions against real ssh-keygen
// signatures. Pinning the key must not also rewrite the sentinel that the
// pinned-key check compares it with: a rendered installer that compares the
// key with itself skips every signature check.
func TestRenderInstallersKeepSignatureVerificationEnabled(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("rendered install.sh verification requires bash and ssh-keygen")
	}
	sourceDir, err := filepath.Abs(repoFile("scripts"))
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	key := filepath.Join(dir, "release-key")
	if out, err := exec.Command("ssh-keygen", "-q", "-t", "ed25519", "-N", "", "-C", "pulse-installer", "-f", key).CombinedOutput(); err != nil {
		t.Fatalf("generate release key: %v\n%s", err, out)
	}
	publicKeyBytes, err := os.ReadFile(key + ".pub")
	if err != nil {
		t.Fatal(err)
	}
	publicKey := strings.TrimSpace(string(publicKeyBytes))

	pinnedDir := filepath.Join(dir, "pinned")
	if out, err := renderInstallersForTest(sourceDir, pinnedDir, "--installer-ssh-public-key", publicKey); err != nil {
		t.Fatalf("render pinned installers: %v\n%s", err, out)
	}
	unpinnedDir := filepath.Join(dir, "unpinned")
	if out, err := renderInstallersForTest(sourceDir, unpinnedDir, "--installer-ssh-public-key", "", "--allow-empty-installer-ssh-public-key"); err != nil {
		t.Fatalf("render unpinned installers: %v\n%s", err, out)
	}
	read := func(parts ...string) string {
		t.Helper()
		content, err := os.ReadFile(filepath.Join(parts...))
		if err != nil {
			t.Fatal(err)
		}
		return string(content)
	}

	for name, pin := range map[string]string{
		"install.sh":  `PINNED_INSTALLER_SSH_PUBLIC_KEY="` + publicKey + `"`,
		"install.ps1": `$PinnedInstallerSshPublicKey = "` + publicKey + `"`,
	} {
		rendered := read(pinnedDir, name)
		if strings.Count(rendered, pin) != 1 {
			t.Errorf("rendered %s does not pin the release key exactly once", name)
		}
		if strings.Count(rendered, renderedInstallerKeyPlaceholder) != 1 {
			t.Errorf("rendering %s rewrote the unpinned-key sentinel", name)
		}
	}
	renderedPS1 := read(pinnedDir, "install.ps1")
	if !strings.Contains(renderedPS1, `($PinnedInstallerSshPublicKey -ne "`+renderedInstallerKeyPlaceholder+`")`) {
		t.Error("rendered install.ps1 no longer compares the pin with the unpinned-key sentinel")
	}

	payloadDir := filepath.Join(dir, "payload dir with spaces")
	if err := os.MkdirAll(payloadDir, 0o700); err != nil {
		t.Fatal(err)
	}
	payload := filepath.Join(payloadDir, "pulse-agent")
	signedBytes := []byte{0x00, 0x01, 0x0a, 0x0d, 0xff, 's', 'i', 'g', 'n', 'e', 'd', 0x00, 0xfe}
	tamperedBytes := []byte{0x00, 0x01, 0x0a, 0x0d, 0xff, 't', 'a', 'm', 'p', 'e', 'r', 0x00, 0xfe}
	if err := os.WriteFile(payload, signedBytes, 0o600); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("ssh-keygen", "-q", "-Y", "sign", "-f", key, "-n", "pulse-install", payload).CombinedOutput(); err != nil {
		t.Fatalf("sign payload: %v\n%s", err, out)
	}
	signatureBytes, err := os.ReadFile(payload + ".sig")
	if err != nil {
		t.Fatal(err)
	}
	signature := base64.StdEncoding.EncodeToString(signatureBytes)

	const exitUnpinned, exitSignatureFailed = 94, 93
	verify := func(t *testing.T, renderedDir string, content []byte, sig string) int {
		t.Helper()
		rendered := read(renderedDir, "install.sh")
		pin := regexp.MustCompile(`(?m)^PINNED_INSTALLER_SSH_PUBLIC_KEY=.*$`).FindString(rendered)
		if pin == "" {
			t.Fatal("rendered install.sh has no installer key pin")
		}
		harness := `set -euo pipefail
TMP_FILES=()
EXIT_SIGNATURE_FAILED=93
INSTALL_SIGNATURE_IDENTITY=pulse-installer
INSTALL_SIGNATURE_NAMESPACE=pulse-install
` + pin + `
fail() { printf '%s\n' "$1" >&2; exit "$2"; }
log_info() { :; }
json_event() { :; }
`
		for _, name := range []string{"has_pinned_installer_signature_key", "decode_base64_to_file", "verify_download_signature"} {
			harness += renderedInstallShellFunction(t, rendered, name) + "\n"
		}
		harness += `has_pinned_installer_signature_key || exit 94
verify_download_signature "$1" "$2"
`
		if err := os.WriteFile(payload, content, 0o600); err != nil {
			t.Fatal(err)
		}
		cmd := exec.Command("bash", "-c", harness, "rendered-installer-signature", payload, sig)
		cmd.Env = append(os.Environ(), "TMPDIR="+t.TempDir())
		out, err := cmd.CombinedOutput()
		var exit *exec.ExitError
		if err != nil && !errors.As(err, &exit) {
			t.Fatalf("run rendered verification: %v\n%s", err, out)
		}
		return cmd.ProcessState.ExitCode()
	}
	for _, tc := range []struct {
		name, renderedDir string
		content           []byte
		signature         string
		want              int
	}{
		{"signed", pinnedDir, signedBytes, signature, 0},
		{"changed after signing", pinnedDir, tamperedBytes, signature, exitSignatureFailed},
		{"unsigned", pinnedDir, signedBytes, "", exitSignatureFailed},
		{"no release key", unpinnedDir, signedBytes, "", exitUnpinned},
	} {
		t.Run("install.sh/"+tc.name, func(t *testing.T) {
			if got := verify(t, tc.renderedDir, tc.content, tc.signature); got != tc.want {
				t.Fatalf("rendered install.sh verification exited %d, want %d", got, tc.want)
			}
		})
	}

	if pwshPath, lookErr := exec.LookPath("pwsh"); lookErr == nil {
		verifyPS1 := func(t *testing.T, renderedDir string, content []byte, sig string) (int, string) {
			t.Helper()
			rendered := read(renderedDir, "install.ps1")
			pin := regexp.MustCompile(`(?m)^\$PinnedInstallerSshPublicKey = .*$`).FindString(rendered)
			if pin == "" {
				t.Fatal("rendered install.ps1 has no installer key pin")
			}
			harnessPath := filepath.Join(t.TempDir(), "verify.ps1")
			harness := `param([string]$FilePath, [string]$SignatureHeader)
$ErrorActionPreference = "Stop"
$script:TempFiles = @()
$InstallerSignatureIdentity = "pulse-installer"
$InstallerSignatureNamespace = "pulse-install"
` + pin + "\n"
			for _, name := range []string{"Test-HasPinnedInstallerSignatureKey", "Get-SshKeygenPath", "Invoke-InstallerSignatureVerification"} {
				harness += renderedInstallPowerShellFunction(t, rendered, name) + "\n"
			}
			harness += `try {
    if (-not (Test-HasPinnedInstallerSignatureKey)) {
        exit 94
    }
    Invoke-InstallerSignatureVerification -FilePath $FilePath -SignatureHeader $SignatureHeader
    exit 0
} catch {
    [Console]::Error.WriteLine($_.Exception.Message)
    exit 93
} finally {
    foreach ($f in $script:TempFiles) {
        Remove-Item -Force $f -ErrorAction SilentlyContinue
    }
}
`
			if err := os.WriteFile(harnessPath, []byte(harness), 0o600); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(payload, content, 0o600); err != nil {
				t.Fatal(err)
			}
			cmd := exec.Command(pwshPath, "-NoProfile", "-File", harnessPath, "-FilePath", payload, "-SignatureHeader", sig)
			out, err := cmd.CombinedOutput()
			var exit *exec.ExitError
			if err != nil && !errors.As(err, &exit) {
				t.Fatalf("run rendered install.ps1 verification: %v\n%s", err, out)
			}
			return cmd.ProcessState.ExitCode(), string(out)
		}
		for _, tc := range []struct {
			name, renderedDir string
			content           []byte
			signature         string
			want              int
		}{
			{"signed", pinnedDir, signedBytes, signature, 0},
			{"changed after signing", pinnedDir, tamperedBytes, signature, exitSignatureFailed},
			{"unsigned", pinnedDir, signedBytes, "", exitSignatureFailed},
			{"malformed signature", pinnedDir, signedBytes, base64.StdEncoding.EncodeToString([]byte("not a valid sshsig")), exitSignatureFailed},
			{"no release key", unpinnedDir, signedBytes, "", exitUnpinned},
		} {
			t.Run("install.ps1/"+tc.name, func(t *testing.T) {
				if got, out := verifyPS1(t, tc.renderedDir, tc.content, tc.signature); got != tc.want {
					t.Fatalf("rendered install.ps1 verification exited %d, want %d\n%s", got, tc.want, out)
				}
			})
		}
	}
}

func TestRenderInstallersRefuseChangedSignaturePinTemplate(t *testing.T) {
	installSH, err := os.ReadFile(repoFile("scripts", "install.sh"))
	if err != nil {
		t.Fatal(err)
	}
	installPS1, err := os.ReadFile(repoFile("scripts", "install.ps1"))
	if err != nil {
		t.Fatal(err)
	}
	const publicKey = "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIMZd/DaH+BldzOkq1A8KVTcFk73nAyrE8aJOyf7i00jm pulse-installer"
	for name, change := range map[string]func(string) string{
		"sentinel removed": func(script string) string {
			return strings.Replace(script, `!= "`+renderedInstallerKeyPlaceholder+`"`, `!= "unpinned"`, 1)
		},
		"pin removed": func(script string) string {
			return strings.Replace(script, `PINNED_INSTALLER_SSH_PUBLIC_KEY="`+renderedInstallerKeyPlaceholder+`"`, `PINNED_INSTALLER_SSH_PUBLIC_KEY=""`, 1)
		},
		"placeholder added": func(script string) string {
			return script + "\n# " + renderedInstallerKeyPlaceholder + "\n"
		},
	} {
		t.Run(name, func(t *testing.T) {
			changed := change(string(installSH))
			if changed == string(installSH) {
				t.Fatal("fixture did not change install.sh")
			}
			sourceDir, outputDir := t.TempDir(), t.TempDir()
			if err := os.WriteFile(filepath.Join(sourceDir, "install.sh"), []byte(changed), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(sourceDir, "install.ps1"), installPS1, 0o755); err != nil {
				t.Fatal(err)
			}
			out, err := renderInstallersForTest(sourceDir, outputDir, "--installer-ssh-public-key", publicKey)
			if err == nil || !strings.Contains(string(out), "installer signature pin template changed") {
				t.Fatalf("renderer accepted a changed install.sh template: err=%v output=%s", err, out)
			}
			if _, statErr := os.Stat(filepath.Join(outputDir, "install.sh")); !os.IsNotExist(statErr) {
				t.Fatalf("renderer wrote install.sh from a changed template: %v", statErr)
			}
		})
	}
}
