package installtests

import (
	"os/exec"
	"strings"
	"testing"
)

func TestReleaseLdflagsServerIncludesCanonicalBuildMetadata(t *testing.T) {
	cmd := exec.Command(repoFile("scripts", "release_ldflags.sh"),
		"server",
		"--version", "6.0.0-rc.1",
		"--build-time", "2026-03-15T20:00:00Z",
		"--git-commit", "abcdef1",
		"--license-public-key", "ZmFrZS1saWNlbnNlLWtleQ==",
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("release_ldflags.sh server failed: %v\n%s", err, string(output))
	}

	ldflags := strings.TrimSpace(string(output))
	required := []string{
		"-X main.Version=v6.0.0-rc.1",
		"-X main.BuildTime=2026-03-15T20:00:00Z",
		"-X main.GitCommit=abcdef1",
		"-X github.com/rcourtman/pulse-go-rewrite/internal/updates.BuildVersion=v6.0.0-rc.1",
		"-X github.com/rcourtman/pulse-go-rewrite/internal/dockeragent.Version=v6.0.0-rc.1",
		"-X github.com/rcourtman/pulse-go-rewrite/pkg/licensing.EmbeddedPublicKey=ZmFrZS1saWNlbnNlLWtleQ==",
		"-X github.com/rcourtman/pulse-go-rewrite/internal/license.EmbeddedPublicKey=ZmFrZS1saWNlbnNlLWtleQ==",
	}
	for _, needle := range required {
		if !strings.Contains(ldflags, needle) {
			t.Fatalf("server ldflags missing %q in %q", needle, ldflags)
		}
	}
}

func TestReleaseLdflagsAgentNormalizesVersionWithoutServerFields(t *testing.T) {
	cmd := exec.Command(repoFile("scripts", "release_ldflags.sh"),
		"agent",
		"--version", "6.0.0",
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("release_ldflags.sh agent failed: %v\n%s", err, string(output))
	}

	ldflags := strings.TrimSpace(string(output))
	if !strings.Contains(ldflags, "-X main.Version=v6.0.0") {
		t.Fatalf("agent ldflags missing normalized version: %q", ldflags)
	}
	disallowed := []string{
		"internal/updates.BuildVersion",
		"main.BuildTime",
		"main.GitCommit",
		"internal/dockeragent.Version",
	}
	for _, needle := range disallowed {
		if strings.Contains(ldflags, needle) {
			t.Fatalf("agent ldflags unexpectedly contained %q in %q", needle, ldflags)
		}
	}
}
