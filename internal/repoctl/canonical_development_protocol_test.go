package repoctl

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"slices"
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

func assertContainsNone(t *testing.T, rel string, content string, forbidden []string) {
	t.Helper()
	for _, item := range forbidden {
		if strings.Contains(content, item) {
			t.Fatalf("%s contains retired content %q", rel, item)
		}
	}
}

func assertRepoFileMissing(t *testing.T, rel string) {
	t.Helper()

	path := filepath.Join("..", "..", rel)
	_, err := os.Stat(path)
	if err == nil {
		t.Fatalf("%s should be removed", rel)
	}
	if !os.IsNotExist(err) {
		t.Fatalf("failed to stat %s: %v", rel, err)
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
		"\"version\": 10",
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
		"pkg/licensing/service.go",
		"pkg/licensing/grant_refresh.go",
		"pkg/licensing/revocation_poll.go",
		"pkg/licensing/license_server_client.go",
		"pkg/licensing/persistence.go",
		"pkg/licensing/activation_store.go",
		"pkg/licensing/trial_activation.go",
		"pkg/licensing/conversion_",
		"pkg/licensing/public_key.go",
		"pkg/licensing/trial_start.go",
		"internal/api/licensing_",
		"internal/api/payments_",
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
		"structured evidence references",
	})

	source := readRepoFile(t, "docs/release-control/v6/SOURCE_OF_TRUTH.md")
	assertContainsAll(t, "docs/release-control/v6/SOURCE_OF_TRUTH.md", source, []string{
		"CANONICAL_DEVELOPMENT_PROTOCOL.md",
		"docs/release-control/v6/subsystems/",
		"## Development Governance",
	})
}

func TestSourceOfTruthStaysStableAndNonOperational(t *testing.T) {
	rel := "docs/release-control/v6/SOURCE_OF_TRUTH.md"
	content := readRepoFile(t, rel)
	assertContainsAll(t, rel, content, []string{
		"## Purpose",
		"## Canonical Control Files",
		"## Scope",
		"## Release Definition",
		"## Non-Negotiable Release Gates",
		"## Locked Decisions",
		"## Development Governance",
		"## Source Domains",
		"It is not a live progress dashboard.",
		"Current lane scores, evidence references, and open operational decisions live",
	})
	assertContainsNone(t, rel, content, []string{
		"## Priority Engine",
		"### Lane Scoring Rubrics",
		"## Session Contract",
		"## Current Lane Snapshot",
		"| Lane ID | Lane | Target |",
		"Evidence:",
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

func TestStatusJSONLaneEvidenceReferencesAreStructured(t *testing.T) {
	status := statusJSON(t)

	scope, ok := status["scope"].(map[string]any)
	if !ok {
		t.Fatalf("status.json missing scope object")
	}

	rawRepos, ok := scope["active_repos"].([]any)
	if !ok {
		t.Fatalf("status.json scope missing active_repos list")
	}

	activeRepos := make(map[string]struct{}, len(rawRepos))
	for _, raw := range rawRepos {
		repo, ok := raw.(string)
		if !ok {
			t.Fatalf("status.json active_repos contains non-string entry")
		}
		activeRepos[repo] = struct{}{}
	}

	policy, ok := status["evidence_reference_policy"].(map[string]any)
	if !ok {
		t.Fatalf("status.json missing evidence_reference_policy object")
	}
	if got, ok := policy["format"].(string); !ok || got != "repo-qualified-relative-paths" {
		t.Fatalf("status.json evidence_reference_policy.format = %#v, want %q", policy["format"], "repo-qualified-relative-paths")
	}
	if got, ok := policy["local_repo"].(string); !ok || got != "pulse" {
		t.Fatalf("status.json evidence_reference_policy.local_repo = %#v, want %q", policy["local_repo"], "pulse")
	}

	rawKinds, ok := policy["allowed_kinds"].([]any)
	if !ok {
		t.Fatalf("status.json evidence_reference_policy.allowed_kinds missing list")
	}
	var allowedKinds []string
	for _, raw := range rawKinds {
		kind, ok := raw.(string)
		if !ok {
			t.Fatalf("status.json evidence_reference_policy.allowed_kinds contains non-string entry")
		}
		allowedKinds = append(allowedKinds, kind)
	}

	lanes, ok := status["lanes"].([]any)
	if !ok || len(lanes) == 0 {
		t.Fatalf("status.json missing lanes list")
	}

	seenIDs := make(map[string]struct{}, len(lanes))
	for _, rawLane := range lanes {
		lane, ok := rawLane.(map[string]any)
		if !ok {
			t.Fatalf("status.json lanes contains non-object entry")
		}

		laneID, ok := lane["id"].(string)
		if !ok || laneID == "" {
			t.Fatalf("status.json lane missing id")
		}
		if _, exists := seenIDs[laneID]; exists {
			t.Fatalf("status.json lane id %q duplicated", laneID)
		}
		seenIDs[laneID] = struct{}{}

		rawEvidence, ok := lane["evidence"].([]any)
		if !ok || len(rawEvidence) == 0 {
			t.Fatalf("status.json lane %q missing evidence list", laneID)
		}

		for _, rawEvidenceRef := range rawEvidence {
			ref, ok := rawEvidenceRef.(map[string]any)
			if !ok {
				t.Fatalf("status.json lane %q contains legacy non-object evidence entry", laneID)
			}

			repo, ok := ref["repo"].(string)
			if !ok || repo == "" {
				t.Fatalf("status.json lane %q evidence missing repo", laneID)
			}
			if _, ok := activeRepos[repo]; !ok {
				t.Fatalf("status.json lane %q evidence repo %q is not in active_repos", laneID, repo)
			}

			path, ok := ref["path"].(string)
			if !ok || path == "" {
				t.Fatalf("status.json lane %q evidence missing path", laneID)
			}
			if filepath.IsAbs(path) {
				t.Fatalf("status.json lane %q evidence path %q must not be absolute", laneID, path)
			}
			cleaned := filepath.ToSlash(filepath.Clean(path))
			if cleaned != path {
				t.Fatalf("status.json lane %q evidence path %q must be clean relative path %q", laneID, path, cleaned)
			}
			if strings.HasPrefix(path, "../") || strings.Contains(path, "/../") {
				t.Fatalf("status.json lane %q evidence path %q must not escape repo root", laneID, path)
			}

			kind, ok := ref["kind"].(string)
			if !ok || !slices.Contains(allowedKinds, kind) {
				t.Fatalf("status.json lane %q evidence kind %#v not allowed", laneID, ref["kind"])
			}

			if repo != "pulse" {
				continue
			}

			fullPath := filepath.Join("..", "..", path)
			info, err := os.Stat(fullPath)
			if err != nil {
				t.Fatalf("status.json lane %q evidence path %q missing in pulse repo: %v", laneID, path, err)
			}
			switch kind {
			case "file":
				if !info.Mode().IsRegular() {
					t.Fatalf("status.json lane %q evidence path %q should be file", laneID, path)
				}
			case "dir":
				if !info.IsDir() {
					t.Fatalf("status.json lane %q evidence path %q should be dir", laneID, path)
				}
			}
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

func TestLegacyReleaseControlOrchestratorIsRemoved(t *testing.T) {
	for _, rel := range []string{
		"docs/release-control/v6/AUTOMATION_LOOP.md",
		"docs/release-control/v6/loop.config.json",
		"scripts/release_control/e2e_test.sh",
		"scripts/release_control/loopctl.sh",
		"scripts/release_control/orchestrator.py",
		"scripts/release_control/orchestrator_unit_test.py",
	} {
		assertRepoFileMissing(t, rel)
	}

	readme := readRepoFile(t, "docs/release-control/v6/README.md")
	assertContainsNone(t, "docs/release-control/v6/README.md", readme, []string{
		"loop.config.json",
		"AUTOMATION_LOOP.md",
	})

	source := readRepoFile(t, "docs/release-control/v6/SOURCE_OF_TRUTH.md")
	assertContainsNone(t, "docs/release-control/v6/SOURCE_OF_TRUTH.md", source, []string{
		"## Product Review Sweep",
		"## Parallel Execution",
		"loop.config.json",
		"loopctl.sh",
	})

	status := statusJSON(t)
	if _, ok := status["automation_state"]; ok {
		t.Fatalf("status.json should not carry legacy automation_state")
	}
	if got, ok := status["execution_model"].(string); !ok || got != "direct-repo-sessions" {
		t.Fatalf("status.json execution_model = %#v, want %q", status["execution_model"], "direct-repo-sessions")
	}
}
