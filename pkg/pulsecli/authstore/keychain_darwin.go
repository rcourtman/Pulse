//go:build darwin

package authstore

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

type darwinKeychainStore struct{}

func defaultKeychainStore(_ string) SecretStore {
	return &darwinKeychainStore{}
}

func (k *darwinKeychainStore) Backend() string {
	return BackendKeychain
}

func (k *darwinKeychainStore) Available() bool {
	_, err := exec.LookPath("security")
	return err == nil
}

func (k *darwinKeychainStore) Set(profile, token string) error {
	name, err := NormalizeProfileName(profile)
	if err != nil {
		return err
	}
	token = strings.TrimSpace(token)
	if token == "" {
		return fmt.Errorf("token must not be empty")
	}

	// Pass the secret over stdin to `security -i` as hex (-X) so the raw token
	// never appears in process argv or requires shell quoting.
	commandLine := fmt.Sprintf(
		"add-generic-password -U -a %s -s %s -X %s\n",
		keychainAccountName(name),
		keychainServiceName,
		hex.EncodeToString([]byte(token)),
	)
	cmd := exec.Command("security", "-i")
	cmd.Stdin = strings.NewReader(commandLine)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		detail := strings.TrimSpace(stderr.String())
		if detail != "" {
			return fmt.Errorf("macOS keychain set failed: %s: %w", detail, err)
		}
		return fmt.Errorf("macOS keychain set failed: %w", err)
	}
	return nil
}

func (k *darwinKeychainStore) Get(profile string) (string, error) {
	name, err := NormalizeProfileName(profile)
	if err != nil {
		return "", err
	}
	cmd := exec.Command(
		"security",
		"find-generic-password",
		"-a", keychainAccountName(name),
		"-s", keychainServiceName,
		"-w",
	)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && exitErr.ExitCode() == 44 {
			return "", fmt.Errorf("%w: %s", ErrSecretNotFound, name)
		}
		detail := strings.TrimSpace(stderr.String())
		if detail != "" {
			return "", fmt.Errorf("macOS keychain lookup failed: %s: %w", detail, err)
		}
		return "", fmt.Errorf("macOS keychain lookup failed: %w", err)
	}
	token := strings.TrimRight(stdout.String(), "\r\n")
	if token == "" {
		return "", fmt.Errorf("%w: %s", ErrSecretNotFound, name)
	}
	return token, nil
}

func (k *darwinKeychainStore) Delete(profile string) error {
	name, err := NormalizeProfileName(profile)
	if err != nil {
		return err
	}
	cmd := exec.Command(
		"security",
		"delete-generic-password",
		"-a", keychainAccountName(name),
		"-s", keychainServiceName,
	)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && exitErr.ExitCode() == 44 {
			return nil
		}
		detail := strings.TrimSpace(stderr.String())
		if detail != "" {
			return fmt.Errorf("macOS keychain delete failed: %s: %w", detail, err)
		}
		return fmt.Errorf("macOS keychain delete failed: %w", err)
	}
	return nil
}
