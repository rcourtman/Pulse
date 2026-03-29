package securityutil

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"strings"
)

// HashedStorageName derives an opaque, fixed-width storage filename stem from an external identifier.
func HashedStorageName(id string) string {
	sum := sha256.Sum256([]byte(id))
	return hex.EncodeToString(sum[:])
}

// JoinStorageLeaf joins an already-owned storage directory with a validated leaf filename.
func JoinStorageLeaf(dir, leaf string) (string, error) {
	trimmedDir := strings.TrimSpace(dir)
	if trimmedDir == "" {
		return "", fmt.Errorf("storage directory is required")
	}

	name := strings.TrimSpace(leaf)
	if name == "" {
		return "", fmt.Errorf("storage leaf is required")
	}
	if name == "." || name == ".." {
		return "", fmt.Errorf("storage leaf must not be '.' or '..'")
	}
	if filepath.Base(name) != name {
		return "", fmt.Errorf("storage leaf must not contain path separators")
	}
	if strings.ContainsAny(name, `/\`) {
		return "", fmt.Errorf("storage leaf must not contain path separators")
	}

	return filepath.Join(trimmedDir, name), nil
}
