//go:build ignore

package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const installerSSHPublicKeyPlaceholder = "__PULSE_INSTALLER_SSH_PUBLIC_KEY__"

// installerSSHPublicKeyPins are the assignments that receive the pinned key.
// Each installer repeats the placeholder once more, as the sentinel its
// pinned-key check compares the assignment with. That occurrence must reach
// the rendered installer unchanged: rewritten, the check compares the key with
// itself and download signatures are never verified.
var installerSSHPublicKeyPins = map[string]string{
	"install.sh":  `PINNED_INSTALLER_SSH_PUBLIC_KEY="` + installerSSHPublicKeyPlaceholder + `"`,
	"install.ps1": `$PinnedInstallerSshPublicKey = "` + installerSSHPublicKeyPlaceholder + `"`,
}

func main() {
	sourceDir := flag.String("source-dir", "", "directory containing install.sh and install.ps1")
	outputDir := flag.String("output-dir", "", "directory to write rendered installers into")
	installerSSHPublicKey := flag.String("installer-ssh-public-key", "", "OpenSSH public key to pin into rendered installers")
	allowEmptyInstallerSSHPublicKey := flag.Bool("allow-empty-installer-ssh-public-key", false, "allow unsigned/dev installer rendering without a pinned installer SSH public key")
	flag.Parse()

	if strings.TrimSpace(*sourceDir) == "" || strings.TrimSpace(*outputDir) == "" {
		fail("source-dir and output-dir are required")
	}
	if strings.TrimSpace(*installerSSHPublicKey) == "" && !*allowEmptyInstallerSSHPublicKey {
		fail("installer-ssh-public-key is required for rendered installers")
	}

	if err := os.MkdirAll(*outputDir, 0o755); err != nil {
		fail(fmt.Sprintf("create output dir: %v", err))
	}

	for _, name := range []string{"install.sh", "install.ps1"} {
		if err := renderInstaller(*sourceDir, *outputDir, name, strings.TrimSpace(*installerSSHPublicKey)); err != nil {
			fail(err.Error())
		}
	}
}

func renderInstaller(sourceDir, outputDir, name, installerSSHPublicKey string) error {
	sourcePath := filepath.Join(sourceDir, name)
	content, err := os.ReadFile(sourcePath)
	if err != nil {
		return fmt.Errorf("read %s: %w", name, err)
	}

	rendered, err := pinInstallerSSHPublicKey(name, string(content), installerSSHPublicKey)
	if err != nil {
		return err
	}
	outputPath := filepath.Join(outputDir, name)
	if err := os.WriteFile(outputPath, []byte(rendered), 0o755); err != nil {
		return fmt.Errorf("write %s: %w", name, err)
	}
	return nil
}

func pinInstallerSSHPublicKey(name, template, installerSSHPublicKey string) (string, error) {
	pin := installerSSHPublicKeyPins[name]
	// Refuse a template whose pin or sentinel changed rather than guess which
	// occurrence is which.
	if pin == "" || strings.Count(template, pin) != 1 || strings.Count(template, installerSSHPublicKeyPlaceholder) != 2 {
		return "", fmt.Errorf("render %s: installer signature pin template changed", name)
	}
	pinned := strings.Replace(pin, installerSSHPublicKeyPlaceholder, installerSSHPublicKey, 1)
	return strings.Replace(template, pin, pinned, 1), nil
}

func fail(message string) {
	fmt.Fprintln(os.Stderr, message)
	os.Exit(1)
}
