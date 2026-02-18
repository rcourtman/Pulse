package recovery

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

const (
	subjectKeyResourcePrefix = "res:"
	subjectKeyExternalPrefix = "ext:"
)

// SubjectKeyForPoint returns a stable grouping key for rollups. When a recovery point can be linked
// to a unified resource, the key is "res:<resource-id>". Otherwise it is a hashed external key
// derived from provider + external ref fields.
func SubjectKeyForPoint(p RecoveryPoint) string {
	return SubjectKey(p.Provider, p.SubjectResourceID, p.SubjectRef)
}

// SubjectKey builds a stable grouping key from the point provider + subject identifiers.
func SubjectKey(provider Provider, subjectResourceID string, subjectRef *ExternalRef) string {
	rid := strings.TrimSpace(subjectResourceID)
	if rid != "" {
		return subjectKeyResourcePrefix + rid
	}
	return externalKey(provider, subjectRef)
}

// RepositoryKey builds a stable grouping key for the repository/destination side (best-effort).
func RepositoryKey(provider Provider, repositoryResourceID string, repositoryRef *ExternalRef) string {
	rid := strings.TrimSpace(repositoryResourceID)
	if rid != "" {
		return subjectKeyResourcePrefix + rid
	}
	return externalKey(provider, repositoryRef)
}

func externalKey(provider Provider, ref *ExternalRef) string {
	if ref == nil {
		return ""
	}

	// Use the strongest identifier available in a predictable order.
	typeValue := strings.TrimSpace(ref.Type)
	if typeValue == "" {
		return ""
	}

	h := sha256.New()
	write := func(s string) {
		h.Write([]byte(strings.TrimSpace(s)))
		h.Write([]byte{0})
	}

	write(subjectKeyExternalPrefix)
	write(string(provider))
	write(typeValue)
	write(ref.UID)
	write(ref.ID)
	write(ref.Namespace)
	write(ref.Name)
	write(ref.Class)

	// Extra is intentionally excluded: it is often high-entropy and non-stable.
	return subjectKeyExternalPrefix + hex.EncodeToString(h.Sum(nil))
}

// RollupResourceID returns the public-facing rollup ID:
// - linked resources return the raw unified resource ID
// - external refs return the external key
func RollupResourceID(subjectKey string) string {
	subjectKey = strings.TrimSpace(subjectKey)
	if strings.HasPrefix(subjectKey, subjectKeyResourcePrefix) {
		return strings.TrimPrefix(subjectKey, subjectKeyResourcePrefix)
	}
	return subjectKey
}
