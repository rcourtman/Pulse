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
	assertDigestPinnedDockerStage(t, dockerfile, `FROM golang:1.26.8-alpine@sha256:`, ` AS builder`)
	assertDigestPinnedDockerStage(t, dockerfile, `FROM alpine:3.24@sha256:`, ``)

	composeBytes, err := os.ReadFile(repoFile("tests", "integration", "docker-compose.test.yml"))
	if err != nil {
		t.Fatalf("read integration compose file: %v", err)
	}
	compose := string(composeBytes)
	if !integrationSeedUsesGovernedDefault(`alpine:3.24`, compose) {
		t.Fatal("integration seed image must use the governed Alpine line pinned by a full immutable digest")
	}
	if strings.Contains(dockerfile, "golang:1.23-alpine") || strings.Contains(compose, "image: alpine:3.20\n") {
		t.Fatal("integration containers must not restore retired toolchain or runtime lines")
	}
}

func integrationSeedUsesGovernedDefault(image string, content string) bool {
	// The same-run image bundle supplies the CI override. The local fallback
	// must still be the governed immutable image, not an arbitrary variable or
	// an unpinned default. Bundle admission is checked by its connected suite.
	pinned := regexp.QuoteMeta(image) + `@sha256:[0-9a-f]{64}`
	pattern := regexp.MustCompile(`(?m)^[\t ]*image:[\t ]*(?:` + pinned + `|\$\{PULSE_E2E_SEED_IMAGE:-` + pinned + `\})[\t ]*$`)
	return pattern.MatchString(content)
}

func TestIntegrationSeedGovernedDefaultAdmission(t *testing.T) {
	pinned := "alpine:3.24@sha256:" + strings.Repeat("a", 64)
	for _, content := range []string{
		"    image: " + pinned + "\n",
		"    image: ${PULSE_E2E_SEED_IMAGE:-" + pinned + "}\n",
	} {
		if !integrationSeedUsesGovernedDefault("alpine:3.24", content) {
			t.Fatalf("refused governed seed default: %q", content)
		}
	}
	for name, content := range map[string]string{
		"mutable":           "image: alpine:3.24",
		"mutable default":   "image: ${PULSE_E2E_SEED_IMAGE:-alpine:3.24}",
		"retired line":      "image: ${PULSE_E2E_SEED_IMAGE:-alpine:3.20@sha256:" + strings.Repeat("a", 64) + "}",
		"short digest":      "image: ${PULSE_E2E_SEED_IMAGE:-" + strings.TrimSuffix(pinned, "a") + "}",
		"foreign variable":  "image: ${OTHER_IMAGE:-" + pinned + "}",
		"missing default":   "image: ${PULSE_E2E_SEED_IMAGE}",
		"missing brace":     "image: ${PULSE_E2E_SEED_IMAGE:-" + pinned,
		"extra suffix":      "image: ${PULSE_E2E_SEED_IMAGE:-" + pinned + "}suffix",
		"extra brace":       "image: " + pinned + "}",
		"wrong service key": "other_image: " + pinned,
	} {
		t.Run(name, func(t *testing.T) {
			if integrationSeedUsesGovernedDefault("alpine:3.24", content) {
				t.Fatalf("accepted invalid seed default: %q", content)
			}
		})
	}
}
