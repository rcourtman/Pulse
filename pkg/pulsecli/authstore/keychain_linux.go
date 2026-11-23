//go:build linux

package authstore

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

type linuxSecretToolStore struct{}

func defaultKeychainStore(_ string) SecretStore {
	return &linuxSecretToolStore{}
}

func (k *linuxSecretToolStore) Backend() string {
	return BackendSecretService
}

func (k *linuxSecretToolStore) Available() bool {
	if strings.TrimSpace(os.Getenv("DBUS_SESSION_BUS_ADDRESS")) == "" {
		return false
	}
	_, err := exec.LookPath("secret-tool")
	return err == nil
}

func (k *linuxSecretToolStore) Set(profile, token string) error {
	name, err := NormalizeProfileName(profile)
	if err != nil {
		return err
	}
	token = strings.TrimSpace(token)
	if token == "" {
		return fmt.Errorf("token must not be empty")
	}

	cmd := exec.Command(
		"secret-tool",
		"store",
		"--label", fmt.Sprintf("Pulse CLI (%s)", name),
		"service", keychainServiceName,
		"profile", name,
	)
	cmd.Stdin = strings.NewReader(token)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		detail := strings.TrimSpace(stderr.String())
		if detail != "" {
			return fmt.Errorf("secret-tool store failed: %s: %w", detail, err)
		}
		return fmt.Errorf("secret-tool store failed: %w", err)
	}
	return nil
}

func (k *linuxSecretToolStore) Get(profile string) (string, error) {
	name, err := NormalizeProfileName(profile)
	if err != nil {
		return "", err
	}
	cmd := exec.Command(
		"secret-tool",
		"lookup",
		"service", keychainServiceName,
		"profile", name,
	)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
			return "", fmt.Errorf("%w: %s", ErrSecretNotFound, name)
		}
		detail := strings.TrimSpace(stderr.String())
		if detail != "" {
			return "", fmt.Errorf("secret-tool lookup failed: %s: %w", detail, err)
		}
		return "", fmt.Errorf("secret-tool lookup failed: %w", err)
	}
	token := strings.TrimRight(stdout.String(), "\r\n")
	if token == "" {
		return "", fmt.Errorf("%w: %s", ErrSecretNotFound, name)
	}
	return token, nil
}

func (k *linuxSecretToolStore) Delete(profile string) error {
	name, err := NormalizeProfileName(profile)
	if err != nil {
		return err
	}
	cmd := exec.Command(
		"secret-tool",
		"clear",
		"service", keychainServiceName,
		"profile", name,
	)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		detail := strings.TrimSpace(stderr.String())
		if detail != "" {
			return fmt.Errorf("secret-tool clear failed: %s: %w", detail, err)
		}
		return fmt.Errorf("secret-tool clear failed: %w", err)
	}
	return nil
}
