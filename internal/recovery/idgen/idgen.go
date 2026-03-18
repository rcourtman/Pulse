package idgen

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"time"
)

// StableID generates deterministic IDs from a set of parts.
// It is used to keep recovery point IDs stable across ingests (and across mock requests).
func StableID(parts ...string) string {
	h := sha256.New()
	for _, part := range parts {
		h.Write([]byte(strings.TrimSpace(part)))
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))
}

// TimeKey returns a stable RFC3339Nano key for ordering/dedup across runs.
// It prefers primary, falling back to fallback.
func TimeKey(primary, fallback *time.Time) string {
	if primary != nil && !primary.IsZero() {
		return primary.UTC().Format(time.RFC3339Nano)
	}
	if fallback != nil && !fallback.IsZero() {
		return fallback.UTC().Format(time.RFC3339Nano)
	}
	return ""
}

// PtrTime converts a time into a UTC time pointer (nil if zero).
func PtrTime(t time.Time) *time.Time {
	if t.IsZero() {
		return nil
	}
	tt := t.UTC()
	return &tt
}
