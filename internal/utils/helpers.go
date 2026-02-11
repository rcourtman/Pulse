package utils

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
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

// NormalizeVersion normalizes version strings for comparison by:
// 1. Stripping whitespace
// 2. Removing the "v" prefix (e.g., "v4.33.1" -> "4.33.1")
// 3. Stripping build metadata after "+" (e.g., "4.36.2+git.14.dirty" -> "4.36.2")
//
// Per semver spec, build metadata MUST be ignored when determining version precedence.
// This fixes issues where dirty builds like "4.36.2+git.14.g469307d6.dirty" would
// incorrectly be treated as newer than "4.36.2", causing infinite update loops.
func NormalizeVersion(version string) string {
	v := strings.TrimPrefix(strings.TrimSpace(version), "v")
	// Strip build metadata (everything after +)
	// Per semver: build metadata MUST be ignored when determining version precedence
	if idx := strings.Index(v, "+"); idx != -1 {
		v = v[:idx]
	}
	return v
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

	verA := parseComparableVersion(a)
	verB := parseComparableVersion(b)

	if verA.major != verB.major {
		if verA.major > verB.major {
			return 1
		}
		return -1
	}
	if verA.minor != verB.minor {
		if verA.minor > verB.minor {
			return 1
		}
		return -1
	}
	if verA.patch != verB.patch {
		if verA.patch > verB.patch {
			return 1
		}
		return -1
	}

	return comparePrerelease(verA.prerelease, verB.prerelease)
}

type comparableVersion struct {
	major      int
	minor      int
	patch      int
	prerelease []string
}

func parseComparableVersion(version string) comparableVersion {
	core := version
	prerelease := ""
	if idx := strings.Index(core, "-"); idx != -1 {
		prerelease = core[idx+1:]
		core = core[:idx]
	}

	parts := strings.Split(core, ".")
	getPart := func(index int) int {
		if index >= len(parts) {
			return 0
		}
		return parseVersionNumber(parts[index])
	}

	parsed := comparableVersion{
		major: getPart(0),
		minor: getPart(1),
		patch: getPart(2),
	}

	if prerelease == "" {
		return parsed
	}
	for _, identifier := range strings.Split(prerelease, ".") {
		identifier = strings.TrimSpace(identifier)
		if identifier == "" {
			continue
		}
		parsed.prerelease = append(parsed.prerelease, identifier)
	}

	return parsed
}

func parseVersionNumber(part string) int {
	part = strings.TrimSpace(part)
	if part == "" {
		return 0
	}

	digitsEnd := 0
	for digitsEnd < len(part) && part[digitsEnd] >= '0' && part[digitsEnd] <= '9' {
		digitsEnd++
	}
	if digitsEnd == 0 {
		return 0
	}

	value, err := strconv.Atoi(part[:digitsEnd])
	if err != nil {
		return 0
	}
	return value
}

func comparePrerelease(a, b []string) int {
	// Stable versions (no prerelease) are always newer than prereleases.
	if len(a) == 0 && len(b) == 0 {
		return 0
	}
	if len(a) == 0 {
		return 1
	}
	if len(b) == 0 {
		return -1
	}

	maxLen := len(a)
	if len(b) > maxLen {
		maxLen = len(b)
	}

	for i := 0; i < maxLen; i++ {
		if i >= len(a) {
			return -1
		}
		if i >= len(b) {
			return 1
		}

		aPart := a[i]
		bPart := b[i]
		aNum, aIsNum := parseNumericIdentifier(aPart)
		bNum, bIsNum := parseNumericIdentifier(bPart)

		switch {
		case aIsNum && bIsNum:
			if aNum > bNum {
				return 1
			}
			if aNum < bNum {
				return -1
			}
		case aIsNum && !bIsNum:
			return -1
		case !aIsNum && bIsNum:
			return 1
		default:
			if aPart > bPart {
				return 1
			}
			if aPart < bPart {
				return -1
			}
		}
	}

	return 0
}

func parseNumericIdentifier(identifier string) (int, bool) {
	if identifier == "" {
		return 0, false
	}
	for _, ch := range identifier {
		if ch < '0' || ch > '9' {
			return 0, false
		}
	}

	value, err := strconv.Atoi(identifier)
	if err != nil {
		return 0, false
	}
	return value, true
}
