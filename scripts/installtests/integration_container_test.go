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
	if !regexpDigestPinnedImage(`alpine:3.24`, compose) {
		t.Fatal("integration seed image must use the governed Alpine line pinned by a full immutable digest")
	}
	if strings.Contains(dockerfile, "golang:1.23-alpine") || strings.Contains(compose, "image: alpine:3.20\n") {
		t.Fatal("integration containers must not restore retired toolchain or runtime lines")
	}
}

func regexpDigestPinnedImage(image string, content string) bool {
	pin := regexp.QuoteMeta(image) + `@sha256:[0-9a-f]{64}`
	// The same-run CI bundle uses this fixed override; the local default must
	// still be a complete governed pin. Do not admit arbitrary interpolation.
	pattern := regexp.MustCompile(`(?m)^[ \t]*image:[ \t]*(?:` + pin + `|\$\{PULSE_E2E_SEED_IMAGE:-` + pin + `\})[ \t]*\r?$`)
	return pattern.MatchString(content)
}

func TestIntegrationContainerImagePinForms(t *testing.T) {
	pin := "alpine:3.24@sha256:" + strings.Repeat("a", 64)
	for _, tc := range []struct {
		name  string
		value string
		want  bool
	}{
		{"literal", "image: " + pin + "\n", true},
		{"indented-literal", "    image: " + pin + "\n", true},
		{"same-run-override", "    image: ${PULSE_E2E_SEED_IMAGE:-" + pin + "}\n", true},
		{"crlf-override", "\timage:\t${PULSE_E2E_SEED_IMAGE:-" + pin + "} \r\n", true},
		{"unpinned-literal", "image: alpine:3.24\n", false},
		{"unpinned-default", "image: ${PULSE_E2E_SEED_IMAGE:-alpine:3.24}\n", false},
		{"missing-default", "image: ${PULSE_E2E_SEED_IMAGE}\n", false},
		{"retired-line", "image: ${PULSE_E2E_SEED_IMAGE:-" + strings.Replace(pin, "3.24", "3.20", 1) + "}\n", false},
		{"mutable-line", "image: ${PULSE_E2E_SEED_IMAGE:-" + strings.Replace(pin, "3.24", "latest", 1) + "}\n", false},
		{"other-variable", "image: ${OTHER_IMAGE:-" + pin + "}\n", false},
		{"unset-only-default", "image: ${PULSE_E2E_SEED_IMAGE-" + pin + "}\n", false},
		{"alternate-operator", "image: ${PULSE_E2E_SEED_IMAGE:+" + pin + "}\n", false},
		{"short-literal-digest", "image: " + pin[:len(pin)-1] + "\n", false},
		{"short-default-digest", "image: ${PULSE_E2E_SEED_IMAGE:-" + pin[:len(pin)-1] + "}\n", false},
		{"long-default-digest", "image: ${PULSE_E2E_SEED_IMAGE:-" + pin + "a}\n", false},
		{"invalid-default-digest", "image: ${PULSE_E2E_SEED_IMAGE:-" + strings.Replace(pin, "sha256:a", "sha256:g", 1) + "}\n", false},
		{"missing-close", "image: ${PULSE_E2E_SEED_IMAGE:-" + pin + "\n", false},
		{"default-suffix", "image: ${PULSE_E2E_SEED_IMAGE:-" + pin + ":other}\n", false},
		{"override-suffix", "image: ${PULSE_E2E_SEED_IMAGE:-" + pin + "}/other\n", false},
		{"multiline-value", "image:\n" + pin + "\n", false},
		{"multiline-default", "image: ${PULSE_E2E_SEED_IMAGE:-\n" + pin + "}\n", false},
		{"commented-pin", "# image: " + pin + "\nimage: alpine:latest\n", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := regexpDigestPinnedImage("alpine:3.24", tc.value); got != tc.want {
				t.Fatalf("governed seed image admission = %t, want %t", got, tc.want)
			}
		})
	}
}
