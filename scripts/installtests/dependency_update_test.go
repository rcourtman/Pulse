package installtests

import (
	"regexp"
	"testing"
	"time"
)

const (
	alpineRuntimeLine       = "3.24"
	alpineRuntimeSupportEnd = "2028-06-01"
	nodeToolchainLine       = "24"
	nodeToolchainSupportEnd = "2028-04-30"
	containerSupportLead    = 180 * 24 * time.Hour
)

// assertDigestPinnedDockerStage allows automated digest refreshes while still
// rejecting shortened, malformed, mutable, or decoy base-image references for
// the named production stage.
func assertDigestPinnedDockerStage(t *testing.T, dockerfile, prefix, suffix string) {
	t.Helper()
	pattern := regexp.MustCompile(
		`(?m)^` + regexp.QuoteMeta(prefix) + `[0-9a-f]{64}` + regexp.QuoteMeta(suffix) + `$`,
	)
	if !pattern.MatchString(dockerfile) {
		t.Fatalf(
			"Dockerfile stage must use a full immutable digest: %s<64 lowercase hex characters>%s",
			prefix,
			suffix,
		)
	}
}

// TestGovernedContainerBaseSupportWindow turns the upstream lifecycle date
// into an advance operational signal. Digest automation keeps a selected line
// patched, but it cannot move a deliberately governed major/minor tag.
func TestGovernedContainerBaseSupportWindow(t *testing.T) {
	for _, policy := range []struct {
		name       string
		line       string
		supportEnd string
	}{
		{name: "Alpine", line: alpineRuntimeLine, supportEnd: alpineRuntimeSupportEnd},
		{name: "Node.js", line: nodeToolchainLine, supportEnd: nodeToolchainSupportEnd},
	} {
		supportEnd, err := time.Parse("2006-01-02", policy.supportEnd)
		if err != nil {
			t.Fatalf("parse %s %s support end: %v", policy.name, policy.line, err)
		}
		remaining := time.Until(supportEnd)
		if remaining < containerSupportLead {
			t.Fatalf(
				"%s %s has less than 180 days of normal support remaining (support ends %s); select and qualify a newer line",
				policy.name,
				policy.line,
				policy.supportEnd,
			)
		}
	}
}
