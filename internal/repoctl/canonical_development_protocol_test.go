package repoctl

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func readRepoFile(t *testing.T, rel string) string {
	t.Helper()

	path := filepath.Join("..", "..", rel)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read %s: %v", rel, err)
	}
	return string(data)
}

func assertContainsAll(t *testing.T, rel string, content string, required []string) {
	t.Helper()
	for _, item := range required {
		if !strings.Contains(content, item) {
			t.Fatalf("%s missing required content %q", rel, item)
		}
	}
}

func TestCanonicalDevelopmentProtocolExists(t *testing.T) {
	rel := "docs/release-control/v6/CANONICAL_DEVELOPMENT_PROTOCOL.md"
	content := readRepoFile(t, rel)
	assertContainsAll(t, rel, content, []string{
		"# Pulse v6 Canonical Development Protocol",
		"## Core Rule",
		"## Required Operating Files",
		"## Subsystem Contracts",
		"## Task Completion Protocol",
		"## Guardrails",
		"## Boundary Rule",
	})
}

func TestSubsystemContractsExistWithRequiredSections(t *testing.T) {
	requiredContracts := []string{
		"docs/release-control/v6/subsystems/alerts.md",
		"docs/release-control/v6/subsystems/monitoring.md",
		"docs/release-control/v6/subsystems/unified-resources.md",
		"docs/release-control/v6/subsystems/cloud-paid.md",
		"docs/release-control/v6/subsystems/api-contracts.md",
		"docs/release-control/v6/subsystems/frontend-primitives.md",
		"docs/release-control/v6/subsystems/performance-and-scalability.md",
	}

	requiredSections := []string{
		"## Purpose",
		"## Canonical Files",
		"## Extension Points",
		"## Forbidden Paths",
		"## Completion Obligations",
		"## Current State",
	}

	for _, rel := range requiredContracts {
		content := readRepoFile(t, rel)
		assertContainsAll(t, rel, content, requiredSections)
	}
}

func TestSubsystemRegistryExistsAndReferencesContracts(t *testing.T) {
	rel := "docs/release-control/v6/subsystems/registry.json"
	content := readRepoFile(t, rel)
	assertContainsAll(t, rel, content, []string{
		"\"version\": 1",
		"\"subsystems\":",
		"docs/release-control/v6/subsystems/alerts.md",
		"docs/release-control/v6/subsystems/monitoring.md",
		"docs/release-control/v6/subsystems/unified-resources.md",
		"docs/release-control/v6/subsystems/cloud-paid.md",
		"docs/release-control/v6/subsystems/api-contracts.md",
		"docs/release-control/v6/subsystems/frontend-primitives.md",
		"docs/release-control/v6/subsystems/performance-and-scalability.md",
	})
}

func TestV6ControlDocsReferenceCanonicalDevelopmentProtocol(t *testing.T) {
	readme := readRepoFile(t, "docs/release-control/v6/README.md")
	assertContainsAll(t, "docs/release-control/v6/README.md", readme, []string{
		"CANONICAL_DEVELOPMENT_PROTOCOL.md",
		"subsystems/*.md",
	})

	source := readRepoFile(t, "docs/release-control/v6/SOURCE_OF_TRUTH.md")
	assertContainsAll(t, "docs/release-control/v6/SOURCE_OF_TRUTH.md", source, []string{
		"CANONICAL_DEVELOPMENT_PROTOCOL.md",
		"docs/release-control/v6/subsystems/",
		"## Development Governance",
	})
}

func TestCanonicalCompletionGuardIsWiredIntoPreCommit(t *testing.T) {
	hook := readRepoFile(t, ".husky/pre-commit")
	assertContainsAll(t, ".husky/pre-commit", hook, []string{
		"canonical_completion_guard.py",
		"Running canonical completion guard...",
	})

	script := readRepoFile(t, "scripts/release_control/canonical_completion_guard.py")
	assertContainsAll(t, "scripts/release_control/canonical_completion_guard.py", script, []string{
		"SUBSYSTEM_REGISTRY",
		"load_subsystem_rules",
		"check_staged_contracts",
		"docs/release-control/v6/subsystems/",
	})
}
