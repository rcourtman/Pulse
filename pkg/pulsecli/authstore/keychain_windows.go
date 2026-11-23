//go:build windows

package authstore

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type windowsDPAPIStore struct {
	dir string
}

func defaultKeychainStore(configDir string) SecretStore {
	return &windowsDPAPIStore{dir: filepath.Join(configDir, "dpapi")}
}

func (k *windowsDPAPIStore) Backend() string {
	return BackendDPAPI
}

func (k *windowsDPAPIStore) Available() bool {
	return k.powershellPath() != ""
}

func (k *windowsDPAPIStore) powershellPath() string {
	for _, candidate := range []string{"powershell.exe", "powershell", "pwsh.exe", "pwsh"} {
		if path, err := exec.LookPath(candidate); err == nil {
			return path
		}
	}
	return ""
}

func (k *windowsDPAPIStore) secretPath(name string) string {
	return filepath.Join(k.dir, name+".dpapi")
}

func (k *windowsDPAPIStore) Set(profile, token string) error {
	name, err := NormalizeProfileName(profile)
	if err != nil {
		return err
	}
	token = strings.TrimSpace(token)
	if token == "" {
		return fmt.Errorf("token must not be empty")
	}
	ps := k.powershellPath()
	if ps == "" {
		return ErrKeychainUnavailable
	}

	const script = `Add-Type -AssemblyName System.Security; ` +
		`$s = [Console]::In.ReadToEnd(); ` +
		`$b = [Text.Encoding]::UTF8.GetBytes($s); ` +
		`$e = [Security.Cryptography.ProtectedData]::Protect($b, $null, [Security.Cryptography.DataProtectionScope]::CurrentUser); ` +
		`[Console]::Out.Write([Convert]::ToBase64String($e))`

	cmd := exec.Command(ps, "-NoProfile", "-NonInteractive", "-Command", script)
	cmd.Stdin = strings.NewReader(token)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		detail := strings.TrimSpace(stderr.String())
		if detail != "" {
			return fmt.Errorf("windows DPAPI protect failed: %s: %w", detail, err)
		}
		return fmt.Errorf("windows DPAPI protect failed: %w", err)
	}

	encoded := strings.TrimSpace(stdout.String())
	if encoded == "" {
		return fmt.Errorf("windows DPAPI protect produced empty ciphertext")
	}
	return writeFileAtomic(k.secretPath(name), []byte(encoded+"\n"), 0o600)
}

func (k *windowsDPAPIStore) Get(profile string) (string, error) {
	name, err := NormalizeProfileName(profile)
	if err != nil {
		return "", err
	}
	raw, err := os.ReadFile(k.secretPath(name))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", fmt.Errorf("%w: %s", ErrSecretNotFound, name)
		}
		return "", fmt.Errorf("read windows DPAPI secret: %w", err)
	}
	ps := k.powershellPath()
	if ps == "" {
		return "", ErrKeychainUnavailable
	}

	const script = `Add-Type -AssemblyName System.Security; ` +
		`$c = [Console]::In.ReadToEnd().Trim(); ` +
		`$b = [Convert]::FromBase64String($c); ` +
		`$u = [Security.Cryptography.ProtectedData]::Unprotect($b, $null, [Security.Cryptography.DataProtectionScope]::CurrentUser); ` +
		`[Console]::Out.Write([Text.Encoding]::UTF8.GetString($u))`

	cmd := exec.Command(ps, "-NoProfile", "-NonInteractive", "-Command", script)
	cmd.Stdin = strings.NewReader(strings.TrimSpace(string(raw)))
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		detail := strings.TrimSpace(stderr.String())
		if detail != "" {
			return "", fmt.Errorf("windows DPAPI unprotect failed: %s: %w", detail, err)
		}
		return "", fmt.Errorf("windows DPAPI unprotect failed: %w", err)
	}
	token := strings.TrimRight(stdout.String(), "\r\n")
	if token == "" {
		return "", fmt.Errorf("%w: %s", ErrSecretNotFound, name)
	}
	return token, nil
}

func (k *windowsDPAPIStore) Delete(profile string) error {
	name, err := NormalizeProfileName(profile)
	if err != nil {
		return err
	}
	if err := os.Remove(k.secretPath(name)); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove windows DPAPI secret: %w", err)
	}
	return nil
}
