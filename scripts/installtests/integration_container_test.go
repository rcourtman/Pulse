package installtests

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

func TestIntegrationContainersUseGovernedImmutableBases(t *testing.T) {
	dockerfileBytes, err := os.ReadFile(repoFile("tests", "integration", "mock-github-server", "Dockerfile"))
	if err != nil {
		t.Fatalf("read mock GitHub Dockerfile: %v", err)
	}
	dockerfile := string(dockerfileBytes)
	assertDigestPinnedDockerStage(t, dockerfile, `FROM golang:1.26.7-alpine@sha256:`, ` AS builder`)
	assertDigestPinnedDockerStage(t, dockerfile, `FROM alpine:3.24@sha256:`, ``)

	composeBytes, err := os.ReadFile(repoFile("tests", "integration", "docker-compose.test.yml"))
	if err != nil {
		t.Fatalf("read integration compose file: %v", err)
	}
	compose := string(composeBytes)
	if !regexpDigestPinnedImage(`alpine:3.24`, compose) {
		t.Fatal("integration seed image must use the governed Alpine line pinned by a full immutable digest")
	}
	if strings.Contains(dockerfile, "golang:1.23-alpine") || strings.Contains(compose, "image: alpine:3.20\n") {
		t.Fatal("integration containers must not restore retired toolchain or runtime lines")
	}
}

func regexpDigestPinnedImage(image string, content string) bool {
	pattern := regexp.MustCompile(`(?m)^\s*image:\s*` + regexp.QuoteMeta(image) + `@sha256:[0-9a-f]{64}\s*$`)
	return pattern.MatchString(content)
}
