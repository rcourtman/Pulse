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

const alpine324RuntimeDigest = "be2656061b536c1883c4034790e21d0a09c5aaaf51968b89368a3c0371ea2c9a"

const node24Amd64FrontendDigest = "a34ab6d7f6f4813f4d0c5469d74685604b962fbdbf5793a7bc7b4998ecb867c8"

const go12611AlpineBuilderDigest = "ae52b5bc6ed045a9ed26ab03dc19327a802a9cc9aa8b27ed57f5c6b60579b0bb"

const go12611DevcontainerDigest = "7af425da11f4fc1a9308eb704369e3f6d1c44e35bac0f874a59ade3f20fdc07e"

func TestGoContainerBuildersUseGovernedToolchain(t *testing.T) {
	// The devcontainer has a different OS base, but must retain the module's
	// compiler patch. The server, control plane and mock share one Alpine image.
	for _, fixture := range []struct {
		path    string
		pattern string
		image   string
	}{
		{"Dockerfile", `(?m)^FROM --platform=linux/amd64 (\S+) AS backend-builder$`, "golang:1.26.11-alpine@sha256:" + go12611AlpineBuilderDigest},
		{"deploy/provider-msp/Dockerfile.control-plane", `(?m)^FROM --platform=\$BUILDPLATFORM (\S+) AS builder$`, "golang:1.26.11-alpine@sha256:" + go12611AlpineBuilderDigest},
		{"tests/integration/mock-github-server/Dockerfile", `(?m)^FROM (\S+) AS builder$`, "golang:1.26.11-alpine@sha256:" + go12611AlpineBuilderDigest},
		{".devcontainer/Dockerfile", `(?m)^FROM (\S+)$`, "golang:1.26.11@sha256:" + go12611DevcontainerDigest},
	} {
		t.Run(fixture.path, func(t *testing.T) {
			contents, err := os.ReadFile(repoFile(fixture.path))
			if err != nil {
				t.Fatal(err)
			}
			matches := regexp.MustCompile(fixture.pattern).FindAllStringSubmatch(string(contents), -1)
			if len(matches) != 1 || matches[0][1] != fixture.image {
				t.Fatalf("Go build stage = %q; expected exactly one stage using %s", matches, fixture.image)
			}
		})
	}
	module, err := os.ReadFile(repoFile("go.mod"))
	if err != nil {
		t.Fatal(err)
	}
	if !regexp.MustCompile(`(?m)^toolchain go1\.26\.11$`).Match(module) {
		t.Fatal("container compiler pins must match the governed go.mod toolchain")
	}
}

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
