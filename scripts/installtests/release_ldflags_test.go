package installtests

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentidentity"
)

func TestReleaseLdflagsServerIncludesCanonicalBuildMetadata(t *testing.T) {
	cmd := exec.Command(repoFile("scripts", "release_ldflags.sh"),
		"server",
		"--version", "6.0.0-rc.1",
		"--build-time", "2026-03-15T20:00:00Z",
		"--git-commit", "abcdef1",
		"--license-public-key", "ZmFrZS1saWNlbnNlLWtleQ==",
		"--update-public-keys", "ZmFrZS11cGRhdGUta2V5",
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
		"-X github.com/rcourtman/pulse-go-rewrite/internal/updatesignature.EmbeddedTrustedPublicKeys=ZmFrZS11cGRhdGUta2V5",
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
		"--update-public-keys", "ZmFrZS11cGRhdGUta2V5",
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("release_ldflags.sh agent failed: %v\n%s", err, string(output))
	}

	ldflags := strings.TrimSpace(string(output))
	if !strings.Contains(ldflags, "internal/agentidentity.ReleaseIdentity=[[pulse-agent-release:v1:v6.0.0]]") {
		t.Fatalf("missing executable release stamp: %s", ldflags)
	}
	if !strings.Contains(ldflags, "-X main.Version=v6.0.0") {
		t.Fatalf("agent ldflags missing normalized version: %q", ldflags)
	}
	if !strings.Contains(ldflags, "internal/updatesignature.EmbeddedTrustedPublicKeys=ZmFrZS11cGRhdGUta2V5") {
		t.Fatalf("agent ldflags missing update public keys: %q", ldflags)
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

func TestReleaseLdflagsAgentVersionGrammar(t *testing.T) {
	for _, version := range []string{"6.2.1", "v6.2.1+git.123", "6.2.0-rc.8", "0.0.0-rootful.v1.gabcdef123456", "rootful-v1.abcdef123456", "6.2", "01.2.3", strings.Repeat("1", 128) + ".2.3"} {
		_, runtimeErr := agentidentity.Version(version)
		out, err := exec.Command(repoFile("scripts", "release_ldflags.sh"), "agent", "--version", version).CombinedOutput()
		if (err == nil) != (runtimeErr == nil) {
			t.Fatalf("version %q: runtime=%v recipe=%v %s", version, runtimeErr, err, out)
		}
	}
}

func TestReleaseLdflagsQualificationVersionsUseRuntimeGrammar(t *testing.T) {
	pattern := regexp.MustCompile(`qualification_version="([^"]+)"`)
	for _, profile := range []string{"rootful", "rootless"} {
		source, err := os.ReadFile(repoFile("scripts", "run-secure-runtime-"+profile+"-qualification.sh"))
		if err != nil {
			t.Fatal(err)
		}
		match := pattern.FindStringSubmatch(string(source))
		if len(match) != 2 {
			t.Fatal("qualification version missing")
		}
		version := strings.ReplaceAll(match[1], "${SOURCE_COMMIT:0:12}", "012345678901")
		if _, err := agentidentity.Version(version); err != nil {
			t.Fatalf("%s version %q: %v", profile, version, err)
		}
		if out, err := exec.Command(repoFile("scripts", "release_ldflags.sh"), "agent", "--version", version).CombinedOutput(); err != nil {
			t.Fatalf("%s recipe: %v %s", profile, err, out)
		}
	}
}

func TestReleaseLdflagsVersionGrammarIsSealed(t *testing.T) {
	for _, name := range []string{"secure_runtime_source_manifest_v6.json", "secure_runtime_source_manifest_v7.json", "secure_runtime_rootful_source_manifest_v1.json", "secure_runtime_rootless_source_manifest_v1.json"} {
		data, err := os.ReadFile(repoFile("scripts", "release_control", name))
		if err != nil {
			t.Fatal(err)
		}
		var manifest struct {
			ExactPaths []string `json:"exact_paths"`
		}
		if err := json.Unmarshal(data, &manifest); err != nil {
			t.Fatal(err)
		}
		found := false
		for _, path := range manifest.ExactPaths {
			if path == "internal/agentidentity/version-pattern.txt" {
				found = true
			}
		}
		if !found {
			t.Fatalf("%s does not seal the shared embedded version grammar", name)
		}
	}
}

func TestReleaseLdflagsRequiresNonemptyGrammar(t *testing.T) {
	root := t.TempDir()
	scripts := filepath.Join(root, "scripts")
	grammarDir := filepath.Join(root, "internal", "agentidentity")
	for _, dir := range []string{scripts, grammarDir} {
		if err := os.MkdirAll(dir, 0700); err != nil {
			t.Fatal(err)
		}
	}
	recipe, err := os.ReadFile(repoFile("scripts", "release_ldflags.sh"))
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(scripts, "release_ldflags.sh")
	if err := os.WriteFile(path, recipe, 0700); err != nil {
		t.Fatal(err)
	}
	grammar := filepath.Join(grammarDir, "version-pattern.txt")
	for _, content := range []string{"missing", "", "\r\n"} {
		if content != "missing" {
			if err := os.WriteFile(grammar, []byte(content), 0600); err != nil {
				t.Fatal(err)
			}
		}
		output, err := exec.Command(path, "agent", "--version", "6.2.1").CombinedOutput()
		if err == nil || !strings.Contains(string(output), "agent version grammar") {
			t.Fatalf("grammar %q: %v %s", content, err, output)
		}
	}
}
