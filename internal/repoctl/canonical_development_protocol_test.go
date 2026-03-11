package repoctl

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
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

func sourceOfTruthLastUpdated(t *testing.T, content string) string {
	t.Helper()

	re := regexp.MustCompile(`(?m)^Last updated: ([0-9]{4}-[0-9]{2}-[0-9]{2})$`)
	match := re.FindStringSubmatch(content)
	if len(match) != 2 {
		t.Fatalf("SOURCE_OF_TRUTH.md missing parsable Last updated line")
	}
	return match[1]
}

func statusJSON(t *testing.T) map[string]any {
	t.Helper()

	content := readRepoFile(t, "docs/release-control/v6/status.json")
	var payload map[string]any
	if err := json.Unmarshal([]byte(content), &payload); err != nil {
		t.Fatalf("failed to parse status.json: %v", err)
	}
	return payload
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
		"\"version\": 8",
		"\"subsystems\":",
		"\"verification\":",
		"\"allow_same_subsystem_tests\":",
		"\"test_prefixes\":",
		"\"exact_files\":",
		"\"path_policies\":",
		"\"require_explicit_path_policy_coverage\":",
		"pkg/licensing/evaluator.go",
		"pkg/licensing/token_source.go",
		"pkg/licensing/entitlement_payload.go",
		"pkg/licensing/hosted_subscription.go",
		"pkg/licensing/billing_state_normalization.go",
		"pkg/licensing/database_source.go",
		"pkg/licensing/features.go",
		"pkg/licensing/stripe_subscription.go",
		"pkg/licensing/models.go",
		"pkg/licensing/activation_types.go",
		"internal/cloudcp/entitlements/service.go",
		"internal/cloudcp/registry/registry.go",
		"internal/cloudcp/stripe/provisioner.go",
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

func TestStatusJSONStaysInSyncWithSourceOfTruth(t *testing.T) {
	source := readRepoFile(t, "docs/release-control/v6/SOURCE_OF_TRUTH.md")
	sourceUpdated := sourceOfTruthLastUpdated(t, source)
	status := statusJSON(t)

	updatedAt, ok := status["updated_at"].(string)
	if !ok {
		t.Fatalf("status.json missing string updated_at")
	}
	if updatedAt != sourceUpdated {
		t.Fatalf("status.json updated_at = %q, want %q", updatedAt, sourceUpdated)
	}
}

func TestStatusJSONSourcePrecedenceIncludesCanonicalGovernanceFiles(t *testing.T) {
	status := statusJSON(t)

	raw, ok := status["source_precedence"].([]any)
	if !ok {
		t.Fatalf("status.json missing source_precedence list")
	}

	var got []string
	for _, entry := range raw {
		value, ok := entry.(string)
		if !ok {
			t.Fatalf("status.json source_precedence contains non-string entry")
		}
		got = append(got, value)
	}

	wantPrefix := []string{
		"docs/release-control/v6/SOURCE_OF_TRUTH.md",
		"docs/release-control/v6/status.json",
		"docs/release-control/v6/CANONICAL_DEVELOPMENT_PROTOCOL.md",
		"docs/release-control/v6/subsystems/registry.json",
	}
	if len(got) < len(wantPrefix) {
		t.Fatalf("status.json source_precedence too short: %v", got)
	}
	for i, want := range wantPrefix {
		if got[i] != want {
			t.Fatalf("status.json source_precedence[%d] = %q, want %q", i, got[i], want)
		}
	}
}

func TestCanonicalCompletionGuardIsWiredIntoPreCommit(t *testing.T) {
	hook := readRepoFile(t, ".husky/pre-commit")
	assertContainsAll(t, ".husky/pre-commit", hook, []string{
		"canonical_completion_guard.py",
		"Running canonical completion guard...",
		"Running governance guardrail tests...",
		"go test ./internal/repoctl -count=1",
	})

	script := readRepoFile(t, "scripts/release_control/canonical_completion_guard.py")
	assertContainsAll(t, "scripts/release_control/canonical_completion_guard.py", script, []string{
		"SUBSYSTEM_REGISTRY",
		"load_subsystem_rules",
		"build_verification_requirements",
		"check_staged_contracts",
		"verification",
		"path_policies",
		"test_prefixes",
		"docs/release-control/v6/subsystems/",
	})
}

func TestCanonicalGovernanceRunsInCI(t *testing.T) {
	workflow := readRepoFile(t, ".github/workflows/canonical-governance.yml")
	assertContainsAll(t, ".github/workflows/canonical-governance.yml", workflow, []string{
		"name: Canonical Governance",
		"python3 scripts/release_control/canonical_completion_guard.py --files-from-stdin",
		"go test ./internal/repoctl -count=1",
		"python3 scripts/release_control/canonical_completion_guard_test.py",
	})
}
