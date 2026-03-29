package securityutil

import (
	"crypto/sha256"
	"encoding/hex"
)

// HashedStorageName derives an opaque, fixed-width storage filename stem from an external identifier.
func HashedStorageName(id string) string {
	sum := sha256.Sum256([]byte(id))
	return hex.EncodeToString(sum[:])
}
