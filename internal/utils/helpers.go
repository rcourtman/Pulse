package utils

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/google/uuid"
)

// GenerateID generates a unique ID with the given prefix
func GenerateID(prefix string) string {
	return fmt.Sprintf("%s-%s", prefix, uuid.NewString())
}

// WriteJSONResponse writes a JSON response to the http.ResponseWriter
func WriteJSONResponse(w http.ResponseWriter, data interface{}) error {
	w.Header().Set("Content-Type", "application/json")
	// Use Marshal instead of Encoder for better performance with large payloads
	jsonData, err := json.Marshal(data)
	if err != nil {
		return err
	}
	_, err = w.Write(jsonData)
	return err
}

// ParseBool interprets common boolean strings, returning true for typical truthy values.
func ParseBool(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "yes", "y", "on":
		return true
	default:
		return false
	}
}

// GetenvTrim returns the environment variable value with surrounding whitespace removed.
func GetenvTrim(key string) string {
	return strings.TrimSpace(os.Getenv(key))
}

// NormalizeVersion strips the "v" prefix from version strings for comparison.
// This normalizes versions like "v4.33.1" to "4.33.1" so that version strings
// from different sources (agent vs server) can be compared consistently.
func NormalizeVersion(version string) string {
	return strings.TrimPrefix(strings.TrimSpace(version), "v")
}

// CompareVersions compares two semver-like version strings.
// Returns:
//
//	1 if a > b (a is newer)
//	0 if a == b
//
// -1 if a < b (b is newer)
//
// Handles versions like "4.33.1", "v4.33.1", "4.33" gracefully.
func CompareVersions(a, b string) int {
	// Normalize both versions
	a = NormalizeVersion(a)
	b = NormalizeVersion(b)

	// Split into parts
	partsA := strings.Split(a, ".")
	partsB := strings.Split(b, ".")

	// Compare each part numerically
	maxLen := len(partsA)
	if len(partsB) > maxLen {
		maxLen = len(partsB)
	}

	for i := 0; i < maxLen; i++ {
		var numA, numB int

		if i < len(partsA) {
			fmt.Sscanf(partsA[i], "%d", &numA)
		}
		if i < len(partsB) {
			fmt.Sscanf(partsB[i], "%d", &numB)
		}

		if numA > numB {
			return 1
		}
		if numA < numB {
			return -1
		}
	}

	return 0
}
