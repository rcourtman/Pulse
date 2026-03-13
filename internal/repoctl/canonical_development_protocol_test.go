package repoctl

import (
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
)

func useStagedGovernanceReads() bool {
	return os.Getenv("PULSE_READ_STAGED_GOVERNANCE") == "1"
}

func repoRootPath(t *testing.T) string {
	t.Helper()

	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("failed to resolve pulse repo root: %v", err)
	}
	return root
}

func readRepoFile(t *testing.T, rel string) string {
	t.Helper()

	if useStagedGovernanceReads() {
		cmd := exec.Command("git", "show", ":"+rel)
		cmd.Dir = repoRootPath(t)
		data, err := cmd.Output()
		if err != nil {
			t.Fatalf("failed to read staged %s: %v", rel, err)
		}
		return string(data)
	}

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

	if useStagedGovernanceReads() {
		cmd := exec.Command("git", "cat-file", "-e", ":"+rel)
		cmd.Dir = repoRootPath(t)
		err := cmd.Run()
		if err == nil {
			t.Fatalf("%s should be removed from the staged commit", rel)
		}
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) {
			t.Fatalf("failed to inspect staged %s: %v", rel, err)
		}
		return
	}

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

func statusLaneIDs(t *testing.T, status map[string]any) map[string]struct{} {
	t.Helper()

	rawLanes, ok := status["lanes"].([]any)
	if !ok {
		t.Fatalf("status.json missing lanes list")
	}

	ids := make(map[string]struct{}, len(rawLanes))
	for _, rawLane := range rawLanes {
		lane, ok := rawLane.(map[string]any)
		if !ok {
			t.Fatalf("status.json lanes contains non-object entry")
		}
		id, ok := lane["id"].(string)
		if !ok || id == "" {
			t.Fatalf("status.json lane missing id")
		}
		ids[id] = struct{}{}
	}
	return ids
}

func repoRootForEvidence(t *testing.T, repo string) string {
	t.Helper()

	envKey := "PULSE_REPO_ROOT_" + strings.ToUpper(strings.ReplaceAll(repo, "-", "_"))
	if value := os.Getenv(envKey); value != "" {
		return value
	}

	currentRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("failed to resolve pulse repo root: %v", err)
	}
	if repo == "pulse" {
		return currentRoot
	}
	return filepath.Join(filepath.Dir(currentRoot), repo)
}

func shouldSkipCrossRepoEvidenceExistenceCheck(repo string) bool {
	if repo == "pulse" {
		return false
	}
	if os.Getenv("GITHUB_ACTIONS") != "true" {
		return false
	}
	envKey := "PULSE_REPO_ROOT_" + strings.ToUpper(strings.ReplaceAll(repo, "-", "_"))
	return os.Getenv(envKey) == ""
}

func validateEvidenceRefs(t *testing.T, activeRepos map[string]struct{}, allowedKinds []string, rawEvidence []any, context string) {
	t.Helper()

	for _, rawEvidenceRef := range rawEvidence {
		ref, ok := rawEvidenceRef.(map[string]any)
		if !ok {
			t.Fatalf("%s contains legacy non-object evidence entry", context)
		}

		repo, ok := ref["repo"].(string)
		if !ok || repo == "" {
			t.Fatalf("%s evidence missing repo", context)
		}
		if _, ok := activeRepos[repo]; !ok {
			t.Fatalf("%s evidence repo %q is not in active_repos", context, repo)
		}

		path, ok := ref["path"].(string)
		if !ok || path == "" {
			t.Fatalf("%s evidence missing path", context)
		}
		if filepath.IsAbs(path) {
			t.Fatalf("%s evidence path %q must not be absolute", context, path)
		}
		cleaned := filepath.ToSlash(filepath.Clean(path))
		if cleaned != path {
			t.Fatalf("%s evidence path %q must be clean relative path %q", context, path, cleaned)
		}
		if strings.HasPrefix(path, "../") || strings.Contains(path, "/../") {
			t.Fatalf("%s evidence path %q must not escape repo root", context, path)
		}

		kind, ok := ref["kind"].(string)
		if !ok || !slices.Contains(allowedKinds, kind) {
			t.Fatalf("%s evidence kind %#v not allowed", context, ref["kind"])
		}

		repoRoot := repoRootForEvidence(t, repo)
		if info, err := os.Stat(repoRoot); err != nil || !info.IsDir() {
			if shouldSkipCrossRepoEvidenceExistenceCheck(repo) {
				t.Logf("%s evidence repo %q not present in this CI checkout; skipping existence check", context, repo)
				continue
			}
			t.Fatalf("%s evidence repo root for %q missing at %q (set %s to override): %v", context, repo, repoRoot, "PULSE_REPO_ROOT_"+strings.ToUpper(strings.ReplaceAll(repo, "-", "_")), err)
		}

		fullPath := filepath.Join(repoRoot, path)
		info, err := os.Stat(fullPath)
		if err != nil {
			t.Fatalf("%s evidence path %q missing in repo %q: %v", context, path, repo, err)
		}
		switch kind {
		case "file":
			if !info.Mode().IsRegular() {
				t.Fatalf("%s evidence path %q in repo %q should be file", context, path, repo)
			}
		case "dir":
			if !info.IsDir() {
				t.Fatalf("%s evidence path %q in repo %q should be dir", context, path, repo)
			}
		}
	}
}

func validateProofCommands(t *testing.T, rawProofCommands any, context string) int {
	t.Helper()

	commands, ok := rawProofCommands.([]any)
	if !ok || len(commands) == 0 {
		t.Fatalf("%s missing proof_commands list", context)
	}

	seenIDs := make(map[string]struct{}, len(commands))
	ids := make([]string, 0, len(commands))
	for _, rawCommand := range commands {
		command, ok := rawCommand.(map[string]any)
		if !ok {
			t.Fatalf("%s proof_commands contains non-object entry", context)
		}

		id, ok := command["id"].(string)
		if !ok || strings.TrimSpace(id) == "" {
			t.Fatalf("%s proof_commands contains command without id", context)
		}
		if _, exists := seenIDs[id]; exists {
			t.Fatalf("%s proof_commands duplicates id %q", context, id)
		}
		seenIDs[id] = struct{}{}
		ids = append(ids, id)

		rawRun, ok := command["run"].([]any)
		if !ok || len(rawRun) == 0 {
			t.Fatalf("%s proof_command %q missing run list", context, id)
		}
		for _, rawArg := range rawRun {
			arg, ok := rawArg.(string)
			if !ok || strings.TrimSpace(arg) == "" {
				t.Fatalf("%s proof_command %q run contains invalid entry", context, id)
			}
		}

		if rawCwd, ok := command["cwd"]; ok {
			cwd, ok := rawCwd.(string)
			if !ok || strings.TrimSpace(cwd) == "" {
				t.Fatalf("%s proof_command %q has invalid cwd %#v", context, id, rawCwd)
			}
			if filepath.IsAbs(cwd) {
				t.Fatalf("%s proof_command %q cwd %q must not be absolute", context, id, cwd)
			}
			cleaned := filepath.ToSlash(filepath.Clean(cwd))
			if cleaned != cwd || strings.HasPrefix(cwd, "../") || strings.Contains(cwd, "/../") {
				t.Fatalf("%s proof_command %q cwd %q must be clean relative path", context, id, cwd)
			}
		}
	}

	sortedIDs := slices.Clone(ids)
	slices.Sort(sortedIDs)
	if !slices.Equal(ids, sortedIDs) {
		t.Fatalf("%s proof_commands must be sorted by command id", context)
	}

	return len(commands)
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
		"contract_audit.py --check",
		"Did the user state a durable product truth or change the current priority?",
		"consistency, seamlessness, drift, bypass",
		"Did the user casually raise a new quality bar or product invariant?",
		"status.json.lanes[*].completion",
		"references must also reference the same lane",
		"already-passed",
		"temporary fallback",
		"lane followup",
	})
}

func TestSubsystemContractsExistWithRequiredSections(t *testing.T) {
	requiredContracts := []string{
		"docs/release-control/v6/subsystems/alerts.md",
		"docs/release-control/v6/subsystems/api-contracts.md",
		"docs/release-control/v6/subsystems/cloud-paid.md",
		"docs/release-control/v6/subsystems/frontend-primitives.md",
		"docs/release-control/v6/subsystems/monitoring.md",
		"docs/release-control/v6/subsystems/organization-settings.md",
		"docs/release-control/v6/subsystems/patrol-intelligence.md",
		"docs/release-control/v6/subsystems/performance-and-scalability.md",
		"docs/release-control/v6/subsystems/unified-resources.md",
	}

	requiredSections := []string{
		"## Contract Metadata",
		"## Purpose",
		"## Canonical Files",
		"## Shared Boundaries",
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
		"\"version\": 12",
		"\"shared_ownerships\":",
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
		"docs/release-control/v6/subsystems/api-contracts.md",
		"docs/release-control/v6/subsystems/cloud-paid.md",
		"docs/release-control/v6/subsystems/frontend-primitives.md",
		"docs/release-control/v6/subsystems/monitoring.md",
		"docs/release-control/v6/subsystems/organization-settings.md",
		"docs/release-control/v6/subsystems/patrol-intelligence.md",
		"docs/release-control/v6/subsystems/performance-and-scalability.md",
		"docs/release-control/v6/subsystems/unified-resources.md",
	})
}

func TestSubsystemRegistrySchemaExistsAndDeclaresOwnershipContract(t *testing.T) {
	rel := "docs/release-control/v6/subsystems/registry.schema.json"
	content := readRepoFile(t, rel)
	assertContainsAll(t, rel, content, []string{
		"\"$schema\": \"https://json-schema.org/draft/2020-12/schema\"",
		"\"title\": \"Pulse v6 Subsystem Registry Schema\"",
		"\"verification\"",
		"\"path_policy\"",
		"\"shared_ownership\"",
		"\"lane\"",
		"\"owned_prefixes\"",
	})
}

func TestReleaseControlPlaneFilesExist(t *testing.T) {
	docRel := "docs/release-control/CONTROL_PLANE.md"
	doc := readRepoFile(t, docRel)
	assertContainsAll(t, docRel, doc, []string{
		"# Pulse Release Control Plane",
		"control_plane.json",
		"active target",
		"v6 is the current active release profile",
		"control_plane_audit.py --check",
		"Direction changes must be normalized",
		"Do not wait for a special governance prompt",
		"stable or GA promotion readiness",
		"v6-rc-stabilization",
		"should always be true",
	})

	jsonRel := "docs/release-control/control_plane.json"
	jsonContent := readRepoFile(t, jsonRel)
	assertContainsAll(t, jsonRel, jsonContent, []string{
		"\"system\": \"pulse-release-control\"",
		"\"active_profile_id\": \"v6\"",
		"\"active_target_id\": \"v6-rc-stabilization\"",
		"\"id\": \"v6-rc-cut\"",
		"\"status\": \"completed\"",
		"\"id\": \"v6-rc-stabilization\"",
		"\"status\": \"active\"",
		"\"completion_rule\": \"manual\"",
		"\"proof_scope\": \"none\"",
		"\"id\": \"v6-ga-promotion\"",
		"\"status\": \"planned\"",
		"\"completion_rule\": \"rc_ready\"",
		"\"completion_rule\": \"release_ready\"",
		"\"prerelease_branch\": \"pulse/v6\"",
		"\"stable_branch\": \"pulse/v6\"",
	})

	schemaRel := "docs/release-control/control_plane.schema.json"
	schemaContent := readRepoFile(t, schemaRel)
	assertContainsAll(t, schemaRel, schemaContent, []string{
		"\"title\": \"Pulse Release Control Plane Schema\"",
		"\"active_profile_id\"",
		"\"active_target_id\"",
		"\"targets\"",
		"\"completion_rule\"",
		"\"prerelease_branch\"",
		"\"stable_branch\"",
	})
}

func TestV6ControlDocsReferenceCanonicalDevelopmentProtocol(t *testing.T) {
	readme := readRepoFile(t, "docs/release-control/v6/README.md")
	assertContainsAll(t, "docs/release-control/v6/README.md", readme, []string{
		"CONTROL_PLANE.md",
		"control_plane.json",
		"CANONICAL_DEVELOPMENT_PROTOCOL.md",
		"HIGH_RISK_RELEASE_VERIFICATION_MATRIX.md",
		"status.schema.json",
		"registry.schema.json",
		"subsystems/*.md",
		"structured evidence references",
		"ready to cut an RC or approve stable/GA",
		"control_plane_audit.py --check",
		"status_audit.py --pretty",
		"status.json.readiness_assertions",
		"proof_commands",
	})

	source := readRepoFile(t, "docs/release-control/v6/SOURCE_OF_TRUTH.md")
	assertContainsAll(t, "docs/release-control/v6/SOURCE_OF_TRUTH.md", source, []string{
		"CONTROL_PLANE.md",
		"control_plane.json",
		"CANONICAL_DEVELOPMENT_PROTOCOL.md",
		"HIGH_RISK_RELEASE_VERIFICATION_MATRIX.md",
		"status.schema.json",
		"registry.schema.json",
		"docs/release-control/v6/subsystems/",
		"## Evergreen Readiness Assertions",
		"## Development Governance",
		"Do not treat `status.json` lane scores reaching target as sufficient release approval by themselves",
		"Do not promote v6 to stable or GA without an exercised RC",
		"v5 maintenance-only support policy",
		"control_plane_audit.py --check",
		"When a user states a durable product truth",
		"classify that as an active-target update or another control-plane",
		"Do not wait for a special governance prompt",
		"Lanes that are at target but intentionally not closed must record a",
		"Those references must belong to that same lane",
		"consistency, seamlessness, drift, bypass",
		"status_audit.py --pretty",
		"status.json.readiness_assertions",
	})

	matrix := readRepoFile(t, "docs/release-control/v6/HIGH_RISK_RELEASE_VERIFICATION_MATRIX.md")
	assertContainsAll(t, "docs/release-control/v6/HIGH_RISK_RELEASE_VERIFICATION_MATRIX.md", matrix, []string{
		"status.json.release_gates",
		"hosted-signup-billing-replay",
		"paid-feature-entitlement-gating",
		"rc-to-ga-promotion-readiness",
		"relay-registration-reconnect-drain",
		"mobile-relay-auth-approvals",
		"msp-provider-tenant-management",
		"organization-user-scope-and-rbac",
		"api-token-scope-and-assignment",
		"upgrade-state-and-entitlement-preservation",
	})
}

func TestStatusSchemaExistsAndDeclaresTypedStatusContract(t *testing.T) {
	rel := "docs/release-control/v6/status.schema.json"
	content := readRepoFile(t, rel)
	assertContainsAll(t, rel, content, []string{
		"\"$schema\": \"https://json-schema.org/draft/2020-12/schema\"",
		"\"title\": \"Pulse v6 Status Schema\"",
		"\"readiness\"",
		"\"readiness_assertion\"",
		"\"proof_command\"",
		"\"proof_commands\"",
		"\"blocking_level\"",
		"\"proof_type\"",
		"\"open_decision\"",
		"\"resolved_decision\"",
		"\"lane_ids\"",
		"\"direct-repo-sessions\"",
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
		"## Evergreen Readiness Assertions",
		"## Non-Negotiable Release Gates",
		"## Locked Decisions",
		"## Development Governance",
		"## Source Domains",
		"It is not a live progress dashboard.",
		"Current lane scores, evidence references, and typed operational decision",
		"Lane completion state, residual-gap summaries, and normalized follow-up",
		"status.json.lane_followups",
		"Those references must belong to that same lane",
		"already-passed assertions, gates, or completed targets",
		"temporary fallback",
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

func TestStatusJSONHasStrictTopLevelSchema(t *testing.T) {
	status := statusJSON(t)

	requiredStringFields := map[string]string{
		"version":              "6.0",
		"updated_at":           sourceOfTruthLastUpdated(t, readRepoFile(t, "docs/release-control/v6/SOURCE_OF_TRUTH.md")),
		"execution_model":      "direct-repo-sessions",
		"source_of_truth_file": "docs/release-control/v6/SOURCE_OF_TRUTH.md",
	}
	for field, want := range requiredStringFields {
		got, ok := status[field].(string)
		if !ok {
			t.Fatalf("status.json missing string %s", field)
		}
		if got != want {
			t.Fatalf("status.json %s = %q, want %q", field, got, want)
		}
	}

	if _, ok := status["priority_engine"].(map[string]any); !ok {
		t.Fatalf("status.json missing priority_engine object")
	}
	if _, ok := status["open_decisions"].([]any); !ok {
		t.Fatalf("status.json missing open_decisions list")
	}
	if _, ok := status["lane_followups"].([]any); !ok {
		t.Fatalf("status.json missing lane_followups list")
	}
	if _, ok := status["resolved_decisions"].([]any); !ok {
		t.Fatalf("status.json missing resolved_decisions list")
	}
	if assertions, ok := status["readiness_assertions"].([]any); !ok || len(assertions) == 0 {
		t.Fatalf("status.json readiness_assertions must be a non-empty list")
	}
	readiness, ok := status["readiness"].(map[string]any)
	if !ok {
		t.Fatalf("status.json missing readiness object")
	}
	if gates, ok := status["release_gates"].([]any); !ok || len(gates) == 0 {
		t.Fatalf("status.json release_gates must be a non-empty list")
	}
	if got, ok := readiness["repo_ready_rule"].(string); !ok || got != "all lanes target-met and evidence-present plus all repo-ready assertions passed" {
		t.Fatalf("status.json readiness.repo_ready_rule = %#v, want %q", readiness["repo_ready_rule"], "all lanes target-met and evidence-present plus all repo-ready assertions passed")
	}
	if got, ok := readiness["rc_ready_rule"].(string); !ok || got != "repo_ready plus all rc-ready assertions passed plus zero rc-ready open_decisions plus all rc-ready release_gates passed" {
		t.Fatalf("status.json readiness.rc_ready_rule = %#v, want %q", readiness["rc_ready_rule"], "repo_ready plus all rc-ready assertions passed plus zero rc-ready open_decisions plus all rc-ready release_gates passed")
	}
	if got, ok := readiness["release_ready_rule"].(string); !ok || got != "rc_ready plus all release-ready assertions passed plus zero release-ready open_decisions plus all release-ready release_gates passed" {
		t.Fatalf("status.json readiness.release_ready_rule = %#v, want %q", readiness["release_ready_rule"], "rc_ready plus all release-ready assertions passed plus zero release-ready open_decisions plus all release-ready release_gates passed")
	}
	if _, ok := readiness["repo_ready"]; ok {
		t.Fatalf("status.json readiness must not hand-maintain repo_ready")
	}
	if _, ok := readiness["rc_ready"]; ok {
		t.Fatalf("status.json readiness must not hand-maintain rc_ready")
	}
	if _, ok := readiness["release_ready"]; ok {
		t.Fatalf("status.json readiness must not hand-maintain release_ready")
	}
	if _, ok := readiness["rc_blockers"]; ok {
		t.Fatalf("status.json readiness must not hand-maintain rc_blockers")
	}
	if _, ok := readiness["release_blockers"]; ok {
		t.Fatalf("status.json readiness must not hand-maintain release_blockers")
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

	want := []string{
		"docs/release-control/v6/SOURCE_OF_TRUTH.md",
		"docs/release-control/v6/status.json",
		"docs/release-control/v6/status.schema.json",
		"docs/release-control/v6/CANONICAL_DEVELOPMENT_PROTOCOL.md",
		"docs/release-control/v6/subsystems/registry.json",
		"docs/release-control/v6/subsystems/registry.schema.json",
	}
	if len(got) != len(want) {
		t.Fatalf("status.json source_precedence = %v, want %v", got, want)
	}
	for i, expected := range want {
		if got[i] != expected {
			t.Fatalf("status.json source_precedence[%d] = %q, want %q", i, got[i], expected)
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
		if _, ok := lane["name"].(string); !ok {
			t.Fatalf("status.json lane %q missing string name", laneID)
		}
		targetScore, ok := lane["target_score"].(float64)
		if !ok {
			t.Fatalf("status.json lane %q missing numeric target_score", laneID)
		}
		currentScore, ok := lane["current_score"].(float64)
		if !ok {
			t.Fatalf("status.json lane %q missing numeric current_score", laneID)
		}
		if targetScore < 0 || targetScore > 10 || currentScore < 0 || currentScore > 10 {
			t.Fatalf("status.json lane %q score out of range", laneID)
		}
		if currentScore > targetScore {
			t.Fatalf("status.json lane %q current_score %.0f exceeds target_score %.0f", laneID, currentScore, targetScore)
		}
		if statusValue, ok := lane["status"].(string); !ok || !slices.Contains([]string{"not-started", "partial", "target-met", "blocked"}, statusValue) {
			t.Fatalf("status.json lane %q has invalid status %#v", laneID, lane["status"])
		}
		completion, ok := lane["completion"].(map[string]any)
		if !ok {
			t.Fatalf("status.json lane %q missing completion object", laneID)
		}
		completionState, ok := completion["state"].(string)
		if !ok || !slices.Contains([]string{"open", "bounded-residual", "complete"}, completionState) {
			t.Fatalf("status.json lane %q has invalid completion.state %#v", laneID, completion["state"])
		}
		if completionSummary, ok := completion["summary"].(string); !ok || strings.TrimSpace(completionSummary) == "" {
			t.Fatalf("status.json lane %q missing non-empty completion.summary", laneID)
		}
		rawTracking, ok := completion["tracking"].([]any)
		if !ok {
			t.Fatalf("status.json lane %q missing completion.tracking list", laneID)
		}
		for _, rawTrackingRef := range rawTracking {
			trackingRef, ok := rawTrackingRef.(map[string]any)
			if !ok {
				t.Fatalf("status.json lane %q completion.tracking contains non-object entry", laneID)
			}
			kind, ok := trackingRef["kind"].(string)
			if !ok || !slices.Contains([]string{"target", "lane-followup", "readiness-assertion", "release-gate", "open-decision"}, kind) {
				t.Fatalf("status.json lane %q has invalid completion.tracking kind %#v", laneID, trackingRef["kind"])
			}
			id, ok := trackingRef["id"].(string)
			if !ok || strings.TrimSpace(id) == "" {
				t.Fatalf("status.json lane %q completion.tracking missing non-empty id", laneID)
			}
		}

		validateEvidenceRefs(t, activeRepos, allowedKinds, rawEvidence, "status.json lane "+laneID)
	}
}

func TestStatusJSONScopeDeclaresControlPlaneAndRepoCatalog(t *testing.T) {
	status := statusJSON(t)

	scope, ok := status["scope"].(map[string]any)
	if !ok {
		t.Fatalf("status.json missing scope object")
	}

	rawActiveRepos, ok := scope["active_repos"].([]any)
	if !ok || len(rawActiveRepos) == 0 {
		t.Fatalf("status.json scope missing active_repos list")
	}
	var activeRepos []string
	for _, raw := range rawActiveRepos {
		repo, ok := raw.(string)
		if !ok || strings.TrimSpace(repo) == "" {
			t.Fatalf("status.json active_repos contains invalid entry")
		}
		activeRepos = append(activeRepos, repo)
	}

	controlPlaneRepo, ok := scope["control_plane_repo"].(string)
	if !ok || controlPlaneRepo != "pulse" {
		t.Fatalf("status.json scope.control_plane_repo = %#v, want %q", scope["control_plane_repo"], "pulse")
	}

	rawCatalog, ok := scope["repo_catalog"].([]any)
	if !ok || len(rawCatalog) != len(activeRepos) {
		t.Fatalf("status.json scope.repo_catalog must mirror active_repos")
	}

	var catalogIDs []string
	for _, raw := range rawCatalog {
		entry, ok := raw.(map[string]any)
		if !ok {
			t.Fatalf("status.json scope.repo_catalog contains non-object entry")
		}
		repoID, ok := entry["id"].(string)
		if !ok || strings.TrimSpace(repoID) == "" {
			t.Fatalf("status.json scope.repo_catalog entry missing id")
		}
		visibility, ok := entry["visibility"].(string)
		if !ok || !slices.Contains([]string{"public", "private"}, visibility) {
			t.Fatalf("status.json scope.repo_catalog %q has invalid visibility %#v", repoID, entry["visibility"])
		}
		purpose, ok := entry["purpose"].(string)
		if !ok || strings.TrimSpace(purpose) == "" {
			t.Fatalf("status.json scope.repo_catalog %q missing purpose", repoID)
		}
		catalogIDs = append(catalogIDs, repoID)
	}

	if !slices.Equal(activeRepos, catalogIDs) {
		t.Fatalf("status.json scope.repo_catalog ids = %v, want %v", catalogIDs, activeRepos)
	}
	if !slices.Contains(catalogIDs, controlPlaneRepo) {
		t.Fatalf("status.json scope.repo_catalog missing control-plane repo %q", controlPlaneRepo)
	}
}

func TestStatusJSONReadinessAssertionsAreTypedRecords(t *testing.T) {
	status := statusJSON(t)
	laneIDs := statusLaneIDs(t, status)

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

	rawGates, ok := status["release_gates"].([]any)
	if !ok {
		t.Fatalf("status.json missing release_gates list")
	}
	releaseGateIDs := make(map[string]struct{}, len(rawGates))
	releaseGateBlockingLevels := make(map[string]string, len(rawGates))
	for _, raw := range rawGates {
		gate, ok := raw.(map[string]any)
		if !ok {
			t.Fatalf("status.json release_gates contains non-object entry")
		}
		id, ok := gate["id"].(string)
		if !ok || id == "" {
			t.Fatalf("status.json release_gate missing id")
		}
		releaseGateIDs[id] = struct{}{}
		blockingLevel, ok := gate["blocking_level"].(string)
		if !ok || !slices.Contains([]string{"rc-ready", "release-ready"}, blockingLevel) {
			t.Fatalf("status.json release_gate %q has invalid blocking_level %#v", id, gate["blocking_level"])
		}
		releaseGateBlockingLevels[id] = blockingLevel
	}

	rawAssertions, ok := status["readiness_assertions"].([]any)
	if !ok || len(rawAssertions) == 0 {
		t.Fatalf("status.json readiness_assertions must be a non-empty list")
	}

	validKinds := []string{"invariant", "journey", "trust-gate"}
	validBlockingLevels := []string{"repo-ready", "rc-ready", "release-ready"}
	validProofTypes := []string{"automated", "manual", "hybrid"}
	seenIDs := make(map[string]struct{}, len(rawAssertions))

	for _, rawAssertion := range rawAssertions {
		assertion, ok := rawAssertion.(map[string]any)
		if !ok {
			t.Fatalf("status.json readiness_assertions contains non-object entry")
		}

		id, ok := assertion["id"].(string)
		if !ok || id == "" {
			t.Fatalf("status.json readiness_assertion missing id")
		}
		if _, exists := seenIDs[id]; exists {
			t.Fatalf("status.json readiness_assertion id %q duplicated", id)
		}
		seenIDs[id] = struct{}{}

		if summary, ok := assertion["summary"].(string); !ok || strings.TrimSpace(summary) == "" {
			t.Fatalf("status.json readiness_assertion %q missing summary", id)
		}
		if kind, ok := assertion["kind"].(string); !ok || !slices.Contains(validKinds, kind) {
			t.Fatalf("status.json readiness_assertion %q has invalid kind %#v", id, assertion["kind"])
		}
		blockingLevel, ok := assertion["blocking_level"].(string)
		if !ok || !slices.Contains(validBlockingLevels, blockingLevel) {
			t.Fatalf("status.json readiness_assertion %q has invalid blocking_level %#v", id, assertion["blocking_level"])
		}
		proofType, ok := assertion["proof_type"].(string)
		if !ok || !slices.Contains(validProofTypes, proofType) {
			t.Fatalf("status.json readiness_assertion %q has invalid proof_type %#v", id, assertion["proof_type"])
		}

		rawLaneIDs, ok := assertion["lane_ids"].([]any)
		if !ok || len(rawLaneIDs) == 0 {
			t.Fatalf("status.json readiness_assertion %q missing lane_ids", id)
		}
		for _, rawLaneID := range rawLaneIDs {
			laneID, ok := rawLaneID.(string)
			if !ok || laneID == "" {
				t.Fatalf("status.json readiness_assertion %q lane_ids contains invalid entry", id)
			}
			if _, ok := laneIDs[laneID]; !ok {
				t.Fatalf("status.json readiness_assertion %q references unknown lane_id %q", id, laneID)
			}
		}

		rawSubsystemIDs, ok := assertion["subsystem_ids"].([]any)
		if !ok {
			t.Fatalf("status.json readiness_assertion %q missing subsystem_ids", id)
		}
		for _, rawSubsystemID := range rawSubsystemIDs {
			subsystemID, ok := rawSubsystemID.(string)
			if !ok || subsystemID == "" {
				t.Fatalf("status.json readiness_assertion %q subsystem_ids contains invalid entry", id)
			}
		}

		rawReleaseGateIDs, ok := assertion["release_gate_ids"].([]any)
		if !ok {
			t.Fatalf("status.json readiness_assertion %q missing release_gate_ids", id)
		}
		for _, rawReleaseGateID := range rawReleaseGateIDs {
			releaseGateID, ok := rawReleaseGateID.(string)
			if !ok || releaseGateID == "" {
				t.Fatalf("status.json readiness_assertion %q release_gate_ids contains invalid entry", id)
			}
			if _, ok := releaseGateIDs[releaseGateID]; !ok {
				t.Fatalf("status.json readiness_assertion %q references unknown release_gate_id %q", id, releaseGateID)
			}
			if releaseGateBlockingLevels[releaseGateID] != blockingLevel {
				t.Fatalf("status.json readiness_assertion %q links release_gate_id %q with blocking_level %q, want %q", id, releaseGateID, releaseGateBlockingLevels[releaseGateID], blockingLevel)
			}
		}
		if proofType == "automated" && len(rawReleaseGateIDs) != 0 {
			t.Fatalf("status.json readiness_assertion %q proof_type automated must not carry release_gate_ids", id)
		}
		if proofType != "automated" && len(rawReleaseGateIDs) == 0 {
			t.Fatalf("status.json readiness_assertion %q proof_type %q must carry release_gate_ids", id, proofType)
		}
		if rawProofCommands, ok := assertion["proof_commands"]; ok {
			if count := validateProofCommands(t, rawProofCommands, "status.json readiness_assertion "+id); count == 0 {
				t.Fatalf("status.json readiness_assertion %q proof_commands must not be empty", id)
			}
		} else if proofType == "automated" {
			t.Fatalf("status.json readiness_assertion %q proof_type automated must carry proof_commands", id)
		}

		rawEvidence, ok := assertion["evidence"].([]any)
		if !ok || len(rawEvidence) == 0 {
			t.Fatalf("status.json readiness_assertion %q missing evidence list", id)
		}
		validateEvidenceRefs(t, activeRepos, allowedKinds, rawEvidence, "status.json readiness_assertion "+id)
	}
}

func TestStatusJSONOpenDecisionsAreTypedRecords(t *testing.T) {
	status := statusJSON(t)
	laneIDs := statusLaneIDs(t, status)

	rawDecisions, ok := status["open_decisions"].([]any)
	if !ok {
		t.Fatalf("status.json missing open_decisions list")
	}

	seenIDs := make(map[string]struct{}, len(rawDecisions))
	dateRe := regexp.MustCompile(`^[0-9]{4}-[0-9]{2}-[0-9]{2}$`)
	for _, rawDecision := range rawDecisions {
		decision, ok := rawDecision.(map[string]any)
		if !ok {
			t.Fatalf("status.json open_decisions contains non-object entry")
		}

		id, ok := decision["id"].(string)
		if !ok || id == "" {
			t.Fatalf("status.json open_decision missing id")
		}
		if _, exists := seenIDs[id]; exists {
			t.Fatalf("status.json open_decision id %q duplicated", id)
		}
		seenIDs[id] = struct{}{}

		if summary, ok := decision["summary"].(string); !ok || strings.TrimSpace(summary) == "" {
			t.Fatalf("status.json open_decision %q missing summary", id)
		}
		if owner, ok := decision["owner"].(string); !ok || strings.TrimSpace(owner) == "" {
			t.Fatalf("status.json open_decision %q missing owner", id)
		}
		if blockingLevel, ok := decision["blocking_level"].(string); !ok || !slices.Contains([]string{"rc-ready", "release-ready"}, blockingLevel) {
			t.Fatalf("status.json open_decision %q has invalid blocking_level %#v", id, decision["blocking_level"])
		}
		if statusValue, ok := decision["status"].(string); !ok || !slices.Contains([]string{"open", "blocked", "owner-action"}, statusValue) {
			t.Fatalf("status.json open_decision %q has invalid status %#v", id, decision["status"])
		}
		openedAt, ok := decision["opened_at"].(string)
		if !ok || !dateRe.MatchString(openedAt) {
			t.Fatalf("status.json open_decision %q missing valid opened_at date", id)
		}
		rawLaneIDs, ok := decision["lane_ids"].([]any)
		if !ok || len(rawLaneIDs) == 0 {
			t.Fatalf("status.json open_decision %q missing lane_ids", id)
		}
		if rawSubsystemIDs, ok := decision["subsystem_ids"].([]any); !ok {
			t.Fatalf("status.json open_decision %q missing subsystem_ids", id)
		} else {
			for _, rawSubsystemID := range rawSubsystemIDs {
				subsystemID, ok := rawSubsystemID.(string)
				if !ok || subsystemID == "" {
					t.Fatalf("status.json open_decision %q subsystem_ids contains invalid entry", id)
				}
			}
		}
		for _, rawLaneID := range rawLaneIDs {
			laneID, ok := rawLaneID.(string)
			if !ok || laneID == "" {
				t.Fatalf("status.json open_decision %q lane_ids contains invalid entry", id)
			}
			if _, ok := laneIDs[laneID]; !ok {
				t.Fatalf("status.json open_decision %q references unknown lane_id %q", id, laneID)
			}
		}
	}
}

func TestStatusJSONLaneFollowupsAreTypedRecords(t *testing.T) {
	status := statusJSON(t)
	laneIDs := statusLaneIDs(t, status)

	rawFollowups, ok := status["lane_followups"].([]any)
	if !ok {
		t.Fatalf("status.json missing lane_followups list")
	}

	seenIDs := make(map[string]struct{}, len(rawFollowups))
	dateRe := regexp.MustCompile(`^[0-9]{4}-[0-9]{2}-[0-9]{2}$`)
	for _, rawFollowup := range rawFollowups {
		followup, ok := rawFollowup.(map[string]any)
		if !ok {
			t.Fatalf("status.json lane_followups contains non-object entry")
		}

		id, ok := followup["id"].(string)
		if !ok || id == "" {
			t.Fatalf("status.json lane_followup missing id")
		}
		if _, exists := seenIDs[id]; exists {
			t.Fatalf("status.json lane_followup id %q duplicated", id)
		}
		seenIDs[id] = struct{}{}

		if summary, ok := followup["summary"].(string); !ok || strings.TrimSpace(summary) == "" {
			t.Fatalf("status.json lane_followup %q missing summary", id)
		}
		if owner, ok := followup["owner"].(string); !ok || strings.TrimSpace(owner) == "" {
			t.Fatalf("status.json lane_followup %q missing owner", id)
		}
		if statusValue, ok := followup["status"].(string); !ok || !slices.Contains([]string{"planned", "active", "parked", "done"}, statusValue) {
			t.Fatalf("status.json lane_followup %q has invalid status %#v", id, followup["status"])
		}
		recordedAt, ok := followup["recorded_at"].(string)
		if !ok || !dateRe.MatchString(recordedAt) {
			t.Fatalf("status.json lane_followup %q missing valid recorded_at date", id)
		}
		rawLaneIDs, ok := followup["lane_ids"].([]any)
		if !ok || len(rawLaneIDs) != 1 {
			t.Fatalf("status.json lane_followup %q must carry exactly one lane_id", id)
		}
		if rawSubsystemIDs, ok := followup["subsystem_ids"].([]any); !ok {
			t.Fatalf("status.json lane_followup %q missing subsystem_ids", id)
		} else {
			for _, rawSubsystemID := range rawSubsystemIDs {
				subsystemID, ok := rawSubsystemID.(string)
				if !ok || subsystemID == "" {
					t.Fatalf("status.json lane_followup %q subsystem_ids contains invalid entry", id)
				}
			}
		}
		for _, rawLaneID := range rawLaneIDs {
			laneID, ok := rawLaneID.(string)
			if !ok || laneID == "" {
				t.Fatalf("status.json lane_followup %q lane_ids contains invalid entry", id)
			}
			if _, ok := laneIDs[laneID]; !ok {
				t.Fatalf("status.json lane_followup %q references unknown lane_id %q", id, laneID)
			}
		}
	}
}

func TestStatusJSONResolvedDecisionsAreTypedRecords(t *testing.T) {
	status := statusJSON(t)
	laneIDs := statusLaneIDs(t, status)

	rawDecisions, ok := status["resolved_decisions"].([]any)
	if !ok || len(rawDecisions) == 0 {
		t.Fatalf("status.json missing resolved_decisions list")
	}

	seenIDs := make(map[string]struct{}, len(rawDecisions))
	dateRe := regexp.MustCompile(`^[0-9]{4}-[0-9]{2}-[0-9]{2}$`)
	validKinds := []string{"architecture", "contract", "governance", "migration", "pricing", "release-policy"}
	for _, rawDecision := range rawDecisions {
		decision, ok := rawDecision.(map[string]any)
		if !ok {
			t.Fatalf("status.json resolved_decisions contains non-object entry")
		}

		id, ok := decision["id"].(string)
		if !ok || id == "" {
			t.Fatalf("status.json resolved_decision missing id")
		}
		if _, exists := seenIDs[id]; exists {
			t.Fatalf("status.json resolved_decision id %q duplicated", id)
		}
		seenIDs[id] = struct{}{}

		if summary, ok := decision["summary"].(string); !ok || strings.TrimSpace(summary) == "" {
			t.Fatalf("status.json resolved_decision %q missing summary", id)
		}
		if kind, ok := decision["kind"].(string); !ok || !slices.Contains(validKinds, kind) {
			t.Fatalf("status.json resolved_decision %q has invalid kind %#v", id, decision["kind"])
		}
		decidedAt, ok := decision["decided_at"].(string)
		if !ok || !dateRe.MatchString(decidedAt) {
			t.Fatalf("status.json resolved_decision %q missing valid decided_at date", id)
		}
		rawLaneIDs, ok := decision["lane_ids"].([]any)
		if !ok || len(rawLaneIDs) == 0 {
			t.Fatalf("status.json resolved_decision %q missing lane_ids", id)
		}
		if rawSubsystemIDs, ok := decision["subsystem_ids"].([]any); !ok {
			t.Fatalf("status.json resolved_decision %q missing subsystem_ids", id)
		} else {
			for _, rawSubsystemID := range rawSubsystemIDs {
				subsystemID, ok := rawSubsystemID.(string)
				if !ok || subsystemID == "" {
					t.Fatalf("status.json resolved_decision %q subsystem_ids contains invalid entry", id)
				}
			}
		}
		for _, rawLaneID := range rawLaneIDs {
			laneID, ok := rawLaneID.(string)
			if !ok || laneID == "" {
				t.Fatalf("status.json resolved_decision %q lane_ids contains invalid entry", id)
			}
			if _, ok := laneIDs[laneID]; !ok {
				t.Fatalf("status.json resolved_decision %q references unknown lane_id %q", id, laneID)
			}
		}
	}
}

func TestCanonicalCompletionGuardIsWiredIntoPreCommit(t *testing.T) {
	hook := readRepoFile(t, ".husky/pre-commit")
	assertContainsAll(t, ".husky/pre-commit", hook, []string{
		"governance_stage_guard.py",
		"Running governance stage guard...",
		"staged_commit_shape_guard.py",
		"Running staged commit shape guard...",
		"Running control plane audit...",
		"control_plane_audit.py --check --staged",
		"canonical_completion_guard.py",
		"Running canonical completion guard...",
		"Running status audit...",
		"status_audit.py --check",
		"Running registry audit...",
		"registry_audit.py --check",
		"Running contract audit...",
		"contract_audit.py --check",
		"Running governance guardrail tests...",
		"go test ./internal/repoctl -count=1",
		"Running readiness assertion guard...",
		"readiness_assertion_guard.py --staged --active-target --proof-type automated",
		"canonical_completion_guard_test.py",
		"control_plane_audit_test.py",
		"contract_audit_test.py",
		"format_staged_go_test.py",
		"governance_stage_guard_test.py",
		"release_promotion_policy_support_test.py",
		"registry_audit_test.py",
		"readiness_assertion_guard_test.py",
		"staged_commit_shape_guard_test.py",
		"status_audit_test.py",
		"subsystem_lookup_test.py",
		"format_staged_go.py",
	})
	assertContainsNone(t, ".husky/pre-commit", hook, []string{
		"gofmt -w -s .",
		"git diff --cached --diff-filter=d --name-only -z | xargs -0 -r git add",
	})

	script := readRepoFile(t, "scripts/release_control/canonical_completion_guard.py")
	assertContainsAll(t, "scripts/release_control/canonical_completion_guard.py", script, []string{
		"DEFAULT_CONTROL_PLANE",
		"SUBSYSTEM_REGISTRY",
		"load_subsystem_rules",
		"build_verification_requirements",
		"check_staged_contracts",
		"verification",
		"path_policies",
		"test_prefixes",
	})

	statusAudit := readRepoFile(t, "scripts/release_control/status_audit.py")
	assertContainsAll(t, "scripts/release_control/status_audit.py", statusAudit, []string{
		"ACTIVE_PROFILE_ID",
		"ACTIVE_TARGET",
		"STATUS_PATH",
		"STATUS_SCHEMA_PATH",
		"repo_root_for_name",
		"audit_status_payload",
		"readiness_assertions",
		"release_gates",
		"lane_followups",
		"open_decisions",
		"resolved_decisions",
		"subsystem_ids",
		"completion_state",
		"derived_status",
		"--check",
	})

	controlPlaneAudit := readRepoFile(t, "scripts/release_control/control_plane_audit.py")
	assertContainsAll(t, "scripts/release_control/control_plane_audit.py", controlPlaneAudit, []string{
		"validate_control_plane_payload",
		"current_status_report",
		"completion_met",
		"active target",
		"--check",
	})

	assertionGuard := readRepoFile(t, "scripts/release_control/readiness_assertion_guard.py")
	assertContainsAll(t, "scripts/release_control/readiness_assertion_guard.py", assertionGuard, []string{
		"DEFAULT_CONTROL_PLANE",
		"STATUS_REL",
		"active_target_blocking_levels",
		"selected_proof_commands",
		"run_selected_proof_commands",
		"--active-target",
		"--blocking-level",
		"--proof-type",
		"proof_commands",
	})

	registryAudit := readRepoFile(t, "scripts/release_control/registry_audit.py")
	assertContainsAll(t, "scripts/release_control/registry_audit.py", registryAudit, []string{
		"DEFAULT_CONTROL_PLANE",
		"REGISTRY_PATH",
		"REGISTRY_SCHEMA_PATH",
		"audit_registry_payload",
		"require_explicit_path_policy_coverage",
		"tracked_repo_files",
		"--check",
	})

	contractAudit := readRepoFile(t, "scripts/release_control/contract_audit.py")
	assertContainsAll(t, "scripts/release_control/contract_audit.py", contractAudit, []string{
		"DEFAULT_CONTROL_PLANE",
		"CONTRACTS_DIR",
		"TEMPLATE_REL",
		"Contract Metadata",
		"audit_contract_payload",
		"contract metadata",
		"--check",
	})

	lookup := readRepoFile(t, "scripts/release_control/subsystem_lookup.py")
	assertContainsAll(t, "scripts/release_control/subsystem_lookup.py", lookup, []string{
		"lookup_paths",
		"verification_requirement",
		"lane_context",
		"status_summary",
		"open_decisions",
		"subsystem_ids",
		"--files-from-stdin",
		"subsystem_matches_path",
	})

	stageGuard := readRepoFile(t, "scripts/release_control/governance_stage_guard.py")
	assertContainsAll(t, "scripts/release_control/governance_stage_guard.py", stageGuard, []string{
		"CONTROL_PLANE_REL",
		"WORKTREE_SENSITIVE_PREFIXES",
		"WORKTREE_SENSITIVE_EXACT_FILES",
		"STAGED_EXECUTION_EXACT_FILES",
		"internal/repoctl/",
		"scripts/release_control/",
		".husky/pre-commit",
		"release_promotion_policy_test.py",
		"unstaged_governance_paths",
		"BLOCKED: unstaged governance file changes detected.",
	})
}

func TestCanonicalGovernanceRunsInCI(t *testing.T) {
	workflow := readRepoFile(t, ".github/workflows/canonical-governance.yml")
	assertContainsAll(t, ".github/workflows/canonical-governance.yml", workflow, []string{
		"name: Canonical Governance",
		"repository: rcourtman/pulse-pro",
		"repository: rcourtman/pulse-enterprise",
		"repository: rcourtman/pulse-mobile",
		"PULSE_REPO_ROOT_PULSE_PRO",
		"PULSE_REPO_ROOT_PULSE_ENTERPRISE",
		"PULSE_REPO_ROOT_PULSE_MOBILE",
		"python3 scripts/release_control/canonical_completion_guard.py --files-from-stdin",
		"python3 scripts/release_control/status_audit.py --check",
		"python3 scripts/release_control/control_plane_audit.py --check",
		"python3 scripts/release_control/registry_audit.py --check",
		"python3 scripts/release_control/contract_audit.py --check",
		"python3 scripts/release_control/readiness_assertion_guard.py --active-target --proof-type automated",
		"python3 scripts/release_control/readiness_assertion_guard.py --active-target --proof-type hybrid",
		"go test ./internal/repoctl -count=1",
		"python3 scripts/release_control/canonical_completion_guard_test.py",
		"python3 scripts/release_control/control_plane_audit_test.py",
		"python3 scripts/release_control/contract_audit_test.py",
		"python3 scripts/release_control/governance_stage_guard_test.py",
		"python3 scripts/release_control/registry_audit_test.py",
		"python3 scripts/release_control/readiness_assertion_guard_test.py",
		"python3 scripts/release_control/status_audit_test.py",
		"python3 scripts/release_control/subsystem_lookup_test.py",
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
