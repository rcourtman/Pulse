package installtests

import (
	"os"
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

const node24Amd64FrontendDigest = "ebfe2f90462722a7a4de65e91990e97fe0d401c70e0e762c5b53302f905ec1c1"

func TestNode24FrontendBuilderDigestIsAligned(t *testing.T) {
	pattern := regexp.MustCompile(`(?m)^FROM --platform=linux/amd64 node:24-alpine@sha256:([0-9a-f]{64}) AS frontend-builder$`)
	for _, path := range []string{
		repoFile("Dockerfile"),
		repoFile("deploy", "provider-msp", "Dockerfile.control-plane"),
	} {
		contents, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read Node 24 builder Dockerfile %s: %v", path, err)
		}
		matches := pattern.FindStringSubmatch(string(contents))
		if len(matches) != 2 {
			t.Fatalf("%s must pin its amd64 Node 24 frontend builder by a full immutable digest", path)
		}
		if matches[1] != node24Amd64FrontendDigest {
			t.Fatalf("%s Node 24 frontend builder digest is %s; expected %s", path, matches[1], node24Amd64FrontendDigest)
		}
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
