package bootstrap

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/crypto"
	internalauth "github.com/rcourtman/pulse-go-rewrite/pkg/auth"
)

const (
	TokenFilename          = ".bootstrap_token"
	persistedFormatVersion = 2
)

type persistedToken struct {
	Version         int       `json:"version"`
	TokenCiphertext string    `json:"token_ciphertext"`
	TokenHash       string    `json:"token_hash"`
	CreatedAt       time.Time `json:"created_at,omitempty"`
}

func ResolvePath(dataPath string) (string, error) {
	clean := strings.TrimSpace(dataPath)
	if clean == "" {
		return "", errors.New("data path required for bootstrap token")
	}
	return filepath.Join(clean, TokenFilename), nil
}

func Load(dataPath string) (token string, path string, migrated bool, err error) {
	path, err = ResolvePath(dataPath)
	if err != nil {
		return "", "", false, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return "", path, false, err
	}

	token, migrated, err = loadFromBytes(strings.TrimSpace(dataPath), path, data)
	if err != nil {
		return "", path, migrated, err
	}
	return token, path, migrated, nil
}

func LoadOrCreate(dataPath string, generate func() (string, error)) (token string, created bool, path string, migrated bool, err error) {
	clean := strings.TrimSpace(dataPath)
	if clean == "" {
		return "", false, "", false, errors.New("data path required for bootstrap token")
	}

	if err := os.MkdirAll(clean, 0o700); err != nil {
		return "", false, "", false, fmt.Errorf("ensure data path: %w", err)
	}

	token, path, migrated, err = Load(clean)
	if err == nil {
		return token, false, path, migrated, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return "", false, path, migrated, err
	}

	token, err = generate()
	if err != nil {
		return "", false, path, false, fmt.Errorf("generate bootstrap token: %w", err)
	}

	path, err = Persist(clean, token, time.Now().UTC())
	if err != nil {
		return "", false, path, false, err
	}
	return token, true, path, false, nil
}

func Persist(dataPath, token string, createdAt time.Time) (string, error) {
	clean := strings.TrimSpace(dataPath)
	token = strings.TrimSpace(token)
	if clean == "" {
		return "", errors.New("data path required for bootstrap token")
	}
	if token == "" {
		return "", errors.New("bootstrap token file is empty")
	}

	if err := os.MkdirAll(clean, 0o700); err != nil {
		return "", fmt.Errorf("ensure data path: %w", err)
	}

	path, err := ResolvePath(clean)
	if err != nil {
		return "", err
	}

	manager, err := crypto.NewCryptoManagerAt(clean)
	if err != nil {
		return path, fmt.Errorf("prepare bootstrap token crypto: %w", err)
	}

	ciphertext, err := manager.EncryptString(token)
	if err != nil {
		return path, fmt.Errorf("encrypt bootstrap token: %w", err)
	}

	record := persistedToken{
		Version:         persistedFormatVersion,
		TokenCiphertext: ciphertext,
		TokenHash:       internalauth.HashAPIToken(token),
		CreatedAt:       createdAt.UTC(),
	}
	data, err := json.Marshal(record)
	if err != nil {
		return path, fmt.Errorf("marshal bootstrap token: %w", err)
	}

	tmpFile := path + ".tmp"
	if err := os.WriteFile(tmpFile, data, 0o600); err != nil {
		return path, fmt.Errorf("persist bootstrap token: %w", err)
	}
	if err := os.Rename(tmpFile, path); err != nil {
		_ = os.Remove(tmpFile)
		return path, fmt.Errorf("persist bootstrap token: %w", err)
	}
	if err := os.Chmod(path, 0o600); err != nil {
		return path, fmt.Errorf("harden bootstrap token file: %w", err)
	}

	return path, nil
}

func loadFromBytes(dataPath, path string, data []byte) (token string, migrated bool, err error) {
	trimmed := strings.TrimSpace(string(data))
	if trimmed == "" {
		return "", false, errors.New("bootstrap token file is empty")
	}

	if strings.HasPrefix(trimmed, "{") {
		token, err = loadEncryptedRecord(dataPath, trimmed)
		return token, false, err
	}

	if _, err := Persist(dataPath, trimmed, time.Now().UTC()); err != nil {
		return "", false, err
	}
	return trimmed, true, nil
}

func loadEncryptedRecord(dataPath, raw string) (string, error) {
	var record persistedToken
	if err := json.Unmarshal([]byte(raw), &record); err != nil {
		return "", fmt.Errorf("read existing bootstrap token: %w", err)
	}
	if strings.TrimSpace(record.TokenCiphertext) == "" {
		return "", errors.New("bootstrap token file is empty")
	}

	manager, err := crypto.NewCryptoManagerAt(dataPath)
	if err != nil {
		return "", fmt.Errorf("prepare bootstrap token crypto: %w", err)
	}
	token, err := manager.DecryptString(record.TokenCiphertext)
	if err != nil {
		return "", fmt.Errorf("decrypt bootstrap token: %w", err)
	}
	token = strings.TrimSpace(token)
	if token == "" {
		return "", errors.New("bootstrap token file is empty")
	}
	if expected := strings.TrimSpace(record.TokenHash); expected != "" && expected != internalauth.HashAPIToken(token) {
		return "", errors.New("bootstrap token hash mismatch")
	}
	return token, nil
}
