package unifiedresources

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

// SourceSpecificID returns the deterministic ID used for non-host resources when the registry
// does not have a canonical identity to key off of.
//
// This matches the ResourceRegistry's internal ID derivation:
// stable := fmt.Sprintf("%s:%s", source, sourceID)
// id := fmt.Sprintf("%s-%s", resourceType, hex(sha256(stable)[:8]))
func SourceSpecificID(resourceType ResourceType, source DataSource, sourceID string) string {
	stable := fmt.Sprintf("%s:%s", source, sourceID)
	hash := sha256.Sum256([]byte(stable))
	return fmt.Sprintf("%s-%s", resourceType, hex.EncodeToString(hash[:8]))
}
