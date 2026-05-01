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

	rendered := strings.ReplaceAll(string(content), installerSSHPublicKeyPlaceholder, installerSSHPublicKey)
	outputPath := filepath.Join(outputDir, name)
	if err := os.WriteFile(outputPath, []byte(rendered), 0o755); err != nil {
		return fmt.Errorf("write %s: %w", name, err)
	}
	return nil
}

func fail(message string) {
	fmt.Fprintln(os.Stderr, message)
	os.Exit(1)
}
