package licensing

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// ActivationStateFileName is the name of the encrypted activation state file.
const ActivationStateFileName = "activation.enc"

// SaveActivationState encrypts and persists the activation state to disk.
func (p *Persistence) SaveActivationState(state *ActivationState) error {
	if state == nil {
		return fmt.Errorf("activation state cannot be nil")
	}

	// Ensure we have a persistent encryption key.
	newKey, err := p.ensurePersistentKey()
	if err != nil {
		return fmt.Errorf("ensure persistent encryption key: %w", err)
	}
	if newKey != "" {
		p.encryptionKey = newKey
	}

	jsonData, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("marshal activation state: %w", err)
	}

	encrypted, err := p.encrypt(jsonData)
	if err != nil {
		return fmt.Errorf("encrypt activation state: %w", err)
	}

	if err := ensurePersistenceOwnerOnlyDir(p.configDir); err != nil {
		return fmt.Errorf("secure config directory: %w", err)
	}

	statePath := filepath.Join(p.configDir, ActivationStateFileName)
	encoded := base64.StdEncoding.EncodeToString(encrypted)

	if err := writeOwnerOnlyPersistenceFileAtomic(statePath, []byte(encoded)); err != nil {
		return fmt.Errorf("write activation state file: %w", err)
	}

	return nil
}

// LoadActivationState reads and decrypts the activation state from disk.
// Returns nil, nil if no activation state file exists.
func (p *Persistence) LoadActivationState() (*ActivationState, error) {
	statePath := filepath.Join(p.configDir, ActivationStateFileName)

	encoded, err := readBoundedPersistenceRegularFile(statePath, maxLicenseFileSize)
	if err != nil {
		if isMissingPersistencePathError(err) {
			return nil, nil // No activation state saved
		}
		return nil, fmt.Errorf("read activation state file: %w", err)
	}

	encrypted, err := base64.StdEncoding.DecodeString(string(encoded))
	if err != nil {
		return nil, fmt.Errorf("decode activation state file: %w", err)
	}

	// Try to decrypt with current encryption key.
	decrypted, err := p.decrypt(encrypted)

	// Fall back to machine-id if the current key doesn't work.
	if err != nil && p.machineID != p.encryptionKey {
		decrypted, err = p.decryptWithKey(encrypted, p.deriveKeyFrom(p.machineID))
	}
	if err != nil {
		return nil, fmt.Errorf("decrypt activation state: %w", err)
	}

	var state ActivationState
	if err := json.Unmarshal(decrypted, &state); err != nil {
		return nil, fmt.Errorf("unmarshal activation state: %w", err)
	}

	return &state, nil
}

// ClearActivationState removes the activation state file from disk.
func (p *Persistence) ClearActivationState() error {
	statePath := filepath.Join(p.configDir, ActivationStateFileName)
	err := os.Remove(statePath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("delete activation state file: %w", err)
	}
	return nil
}

// ActivationStateExists checks if an activation state file exists on disk.
func (p *Persistence) ActivationStateExists() bool {
	statePath := filepath.Join(p.configDir, ActivationStateFileName)
	info, err := os.Lstat(statePath)
	if err != nil {
		return false
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return false
	}
	return info.Mode().IsRegular()
}
