package installtests

import (
	"regexp"
	"testing"
)

// assertDigestPinnedDockerBase allows automated digest refreshes while still
// rejecting shortened, malformed, or mutable base-image references.
func assertDigestPinnedDockerBase(t *testing.T, dockerfile, prefix string) {
	t.Helper()
	pattern := regexp.MustCompile(regexp.QuoteMeta(prefix) + `[0-9a-f]{64}(?:\s|$)`)
	if !pattern.MatchString(dockerfile) {
		t.Fatalf("Dockerfile base image must use a full immutable digest: %s<64 lowercase hex characters>", prefix)
	}
}
