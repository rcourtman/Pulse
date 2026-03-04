package api

// Code standards tests: these act as linter rules that run in CI.
// They scan source files for anti-patterns that indicate DRY violations
// and fail the build if someone reintroduces a consolidated pattern.

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// readGoFiles returns the contents of all non-test .go files in the api package directory.
func readGoFiles(t *testing.T) map[string]string {
	t.Helper()
	files := make(map[string]string)

	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("failed to read api directory: %v", err)
	}

	for _, entry := range entries {
		name := entry.Name()
		if filepath.Ext(name) != ".go" || strings.HasSuffix(name, "_test.go") {
			continue
		}
		data, err := os.ReadFile(name)
		if err != nil {
			t.Fatalf("failed to read %s: %v", name, err)
		}
		files[name] = string(data)
	}
	return files
}

// TestNoInline402Responses ensures handlers use WriteLicenseRequired() instead of
// manually writing HTTP 402 responses. The only files allowed to write 402 directly
// are license_handlers.go and middleware_license.go (which define the helpers).
func TestNoInline402Responses(t *testing.T) {
	allowedFiles := map[string]bool{
		"license_handlers.go":   true,
		"middleware_license.go": true,
	}

	// Match patterns like: WriteHeader(402), WriteHeader(http.StatusPaymentRequired),
	// http.Error(...402...), StatusPaymentRequired (used in a write context)
	inline402 := regexp.MustCompile(`WriteHeader\(\s*(402|http\.StatusPaymentRequired)\s*\)`)

	for name, content := range readGoFiles(t) {
		if allowedFiles[name] {
			continue
		}
		if matches := inline402.FindAllStringIndex(content, -1); len(matches) > 0 {
			for _, m := range matches {
				line := 1 + strings.Count(content[:m[0]], "\n")
				t.Errorf("%s:%d: inline 402 response — use WriteLicenseRequired() from license_handlers.go instead", name, line)
			}
		}
	}
}

// TestNoAgentHandlerMethodRedefinition ensures that SetMonitor, SetMultiTenantMonitor,
// and getMonitor are only defined on baseAgentHandlers, not on concrete handler types.
func TestNoAgentHandlerMethodRedefinition(t *testing.T) {
	// These methods should only be defined in agent_handlers_base.go
	patterns := []struct {
		re   *regexp.Regexp
		name string
	}{
		{regexp.MustCompile(`func \(h \*(?:Docker|Kubernetes|Host)AgentHandlers\) SetMonitor\b`), "SetMonitor"},
		{regexp.MustCompile(`func \(h \*(?:Docker|Kubernetes|Host)AgentHandlers\) SetMultiTenantMonitor\b`), "SetMultiTenantMonitor"},
		{regexp.MustCompile(`func \(h \*(?:Docker|Kubernetes|Host)AgentHandlers\) getMonitor\b`), "getMonitor"},
	}

	for name, content := range readGoFiles(t) {
		if name == "agent_handlers_base.go" {
			continue
		}
		for _, p := range patterns {
			if loc := p.re.FindStringIndex(content); loc != nil {
				line := 1 + strings.Count(content[:loc[0]], "\n")
				t.Errorf("%s:%d: %s() redefined on concrete handler type — it's inherited from baseAgentHandlers", name, line, p.name)
			}
		}
	}
}

// ---------------------------------------------------------------------------
// Repo Boundary Audit — Paid-Domain Code in Public Paths
// ---------------------------------------------------------------------------
//
// The pulse public repo should contain only community/core runtime code.
// Paid-domain implementations (RBAC, Audit, SSO, Reporting) are migrating to
// pulse-enterprise behind pkg/extensions contracts.
//
// This test tracks paid-domain implementation files still in internal/api/.
// The list acts as a monotonic ratchet: it can only shrink as files migrate.
// Adding new paid-domain implementation files to internal/api/ will fail CI.
//
// Allowed patterns in the public repo:
//   - pkg/extensions/*        — interface contracts (imported by enterprise)
//   - enterprise_extension_*  — binder registration (1 SetXxx func per file)
//   - pkg/auth/*              — shared auth primitives (also used by core)
//   - pkg/audit/*             — shared audit primitives (also used by core)
//
// NOT allowed (should migrate to pulse-enterprise over time):
//   - Handler implementations that serve paid-only HTTP endpoints
//   - Service implementations that execute paid-only business logic
//
// See: pulse-enterprise/docs/V6_REPO_REALIGNMENT.md

// paidDomainFiles is the set of non-test .go files in internal/api/ that
// contain paid-domain implementation code. Each entry is a file that should
// eventually migrate to pulse-enterprise.
//
// Last audited: 2026-02-27 (14 files).
var paidDomainFiles = map[string]string{
	// RBAC handlers (5 files)
	"access_control_handlers.go": "RBAC",
	"access_admin_handlers.go":   "RBAC",
	"access_admin_recovery.go":   "RBAC",
	"access_metrics_handlers.go": "RBAC",
	"access_tenant_provider.go":  "RBAC",

	// Audit handlers (1 file)
	"activity_audit_handlers.go": "Audit",

	// SSO/OIDC/SAML handlers and services (5 files)
	"identity_sso_handlers.go": "SSO",
	"oidc_handlers.go":         "SSO",
	"oidc_service.go":          "SSO",
	"saml_handlers.go":         "SSO",
	"saml_service.go":          "SSO",

	// Reporting handlers (2 files)
	"metrics_reporting_handlers.go": "Reporting",
	"reporting_runtime_snapshot.go": "Reporting",
}

const paidDomainFileCeiling = 13

// TestPaidDomainBoundaryAudit enforces two rules:
//
//  1. No NEW paid-domain implementation files may be added to internal/api/.
//     New paid-domain code must go in pulse-enterprise behind pkg/extensions.
//  2. The total count of tracked files can only decrease (migration ratchet).
//
// When a file is migrated to pulse-enterprise, remove it from paidDomainFiles
// and lower paidDomainFileCeiling accordingly.
func TestPaidDomainBoundaryAudit(t *testing.T) {
	goFiles := readGoFiles(t) // non-test .go files in internal/api/

	// Verify every tracked file still exists (catch stale entries).
	for name, domain := range paidDomainFiles {
		if _, ok := goFiles[name]; !ok {
			t.Errorf("paid-domain file %s (%s) is tracked but no longer exists — remove it from paidDomainFiles and lower paidDomainFileCeiling", name, domain)
		}
	}

	// Count tracked files that exist.
	existingCount := 0
	for name := range paidDomainFiles {
		if _, ok := goFiles[name]; ok {
			existingCount++
		}
	}

	// Ratchet: total must not exceed ceiling.
	if existingCount > paidDomainFileCeiling {
		t.Errorf("paid-domain file count %d exceeds ceiling %d — new paid-domain files must go in pulse-enterprise", existingCount, paidDomainFileCeiling)
	}
	if existingCount < paidDomainFileCeiling {
		t.Logf("paid-domain file count %d is below ceiling %d — lower paidDomainFileCeiling to lock in this improvement", existingCount, paidDomainFileCeiling)
	}

	// Detect untracked paid-domain files by matching known naming patterns.
	// These patterns identify files that implement paid-feature HTTP handlers
	// or business logic (not binders, not tests, not shared infrastructure).
	paidPatterns := []*regexp.Regexp{
		regexp.MustCompile(`^access_`),                // RBAC handlers (access_*)
		regexp.MustCompile(`^rbac_`),                  // RBAC handlers (rbac_*)
		regexp.MustCompile(`^activity_audit`),         // Audit handlers
		regexp.MustCompile(`^audit_`),                 // Audit handlers (audit_*)
		regexp.MustCompile(`^(?:oidc|saml)_`),         // SSO protocol impls
		regexp.MustCompile(`^identity_sso`),           // SSO identity handlers
		regexp.MustCompile(`^sso_`),                   // SSO handlers (sso_*)
		regexp.MustCompile(`^(?:metrics_)?reporting`), // Reporting handlers
		regexp.MustCompile(`^reporting_runtime`),      // Reporting runtime
	}

	// Enterprise extension binder files are the correct pattern — they
	// contain only SetXxx registration functions, not implementations.
	// Use an exact allowlist to prevent new paid implementations from
	// hiding behind the enterprise_extension_ prefix.
	knownBinderFiles := map[string]bool{
		"enterprise_extension_ai_alert_analysis.go": true,
		"enterprise_extension_ai_autofix.go":        true,
		"enterprise_extension_ai_investigation.go":  true,
		"enterprise_extension_audit_admin.go":       true,
		"enterprise_extension_rbac_admin.go":        true,
		"enterprise_extension_reporting_admin.go":   true,
		"enterprise_extension_sso_admin.go":         true,
	}

	for name := range goFiles {
		// Skip already-tracked files.
		if _, tracked := paidDomainFiles[name]; tracked {
			continue
		}

		// Skip known binder files (exact allowlist, not prefix).
		if knownBinderFiles[name] {
			continue
		}

		// Flag unknown enterprise_extension_ files (not in the exact allowlist).
		if strings.HasPrefix(name, "enterprise_extension_") {
			t.Errorf("unknown enterprise extension binder %s — add it to knownBinderFiles if it is a valid binder, or move implementation code to pulse-enterprise", name)
			continue
		}

		// Check against paid-domain patterns.
		for _, re := range paidPatterns {
			if re.MatchString(name) {
				t.Errorf("untracked paid-domain file %s matches pattern %s — new paid-domain implementations must go in pulse-enterprise (if this is a false positive, add an exemption)", name, re.String())
				break
			}
		}
	}
}

// TestNoRawBroadcastStateInAgentHandlers ensures agent handler files use
// h.broadcastState(ctx) instead of the raw go h.wsHub.BroadcastState(...) pattern.
func TestNoRawBroadcastStateInAgentHandlers(t *testing.T) {
	agentFiles := map[string]bool{
		"docker_agents.go":     true,
		"kubernetes_agents.go": true,
		"host_agents.go":       true,
	}

	raw := regexp.MustCompile(`go h\.wsHub\.BroadcastState\(`)

	for name, content := range readGoFiles(t) {
		if !agentFiles[name] {
			continue
		}
		if matches := raw.FindAllStringIndex(content, -1); len(matches) > 0 {
			for _, m := range matches {
				line := 1 + strings.Count(content[:m[0]], "\n")
				t.Errorf("%s:%d: raw BroadcastState call — use h.broadcastState(r.Context()) instead", name, line)
			}
		}
	}
}

// TestNoRawHTTPErrorInSettingsHandlers ensures settings handler files use
// writeErrorResponse() instead of http.Error(). Raw http.Error() returns
// plain text bodies that break frontend JSON parsing during first-session
// error states, causing console errors and broken UX.
//
// Settings handlers must return structured JSON via writeErrorResponse()
// so the frontend can parse error codes and show appropriate error UI.
func TestNoRawHTTPErrorInSettingsHandlers(t *testing.T) {
	settingsFiles := map[string]bool{
		"system_settings.go":           true,
		"subscription_entitlements.go": true,
	}

	rawHTTPError := regexp.MustCompile(`http\.Error\(`)

	for name, content := range readGoFiles(t) {
		if !settingsFiles[name] {
			continue
		}
		if matches := rawHTTPError.FindAllStringIndex(content, -1); len(matches) > 0 {
			for _, m := range matches {
				line := 1 + strings.Count(content[:m[0]], "\n")
				t.Errorf("%s:%d: raw http.Error() call — use writeErrorResponse() for structured JSON errors", name, line)
			}
		}
	}
}

// TestPkgLicensingImportBoundary ensures that only licensing_bridge.go may
// import pkg/licensing directly. All other files in internal/api/ must use
// bridge wrappers (type aliases, constants, and functions defined in
// licensing_bridge.go) instead of importing pkg/licensing directly.
//
// This prevents tight coupling between business logic handlers and the
// licensing package, keeping the boundary clean and auditable.
func TestPkgLicensingImportBoundary(t *testing.T) {
	// Only licensing_bridge.go is allowed to import pkg/licensing.
	allowedFiles := map[string]bool{
		"licensing_bridge.go": true,
	}

	importPattern := regexp.MustCompile(`"github\.com/rcourtman/pulse-go-rewrite/pkg/licensing(?:/[^"]*)?"\s*$`)

	for name, content := range readGoFiles(t) {
		if allowedFiles[name] {
			continue
		}
		for i, line := range strings.Split(content, "\n") {
			if importPattern.MatchString(line) {
				t.Errorf("%s:%d: direct pkg/licensing import — use bridge wrappers from licensing_bridge.go instead", name, i+1)
			}
		}
	}
}

// ---------------------------------------------------------------------------
// Phase 3 Governance — Repo Realignment Boundary Locks
// ---------------------------------------------------------------------------
//
// These tests enforce Phase 3 of the repo realignment plan
// (pulse-enterprise/docs/V6_REPO_REALIGNMENT.md):
//
//   "Lock branch/release gates: no paid implementation changes merged to
//    pulse after cut-over."
//
// The tests below are the Go-native equivalents of the enforcement modes
// in scripts/audit-private-boundary.sh, ensuring these rules run as part
// of `go test ./...` without requiring a separate CI script step.

// readConsumerGoFilesFromDir returns the contents of all non-test .go files
// in the given directory (relative to internal/api/).
func readConsumerGoFilesFromDir(t *testing.T, relDir string) map[string]string {
	t.Helper()
	dir := filepath.Join("..", relDir)
	files := make(map[string]string)
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		if filepath.Ext(path) != ".go" || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			t.Fatalf("failed to read %s: %v", path, readErr)
		}
		files[path] = string(data)
		return nil
	})
	if err != nil {
		t.Fatalf("failed to walk %s: %v", relDir, err)
	}
	return files
}

// TestNoInternalLicenseImportFromAPIProd ensures that no production file in
// internal/api/ directly imports internal/license (root or sub-packages).
// All license access must go through pkg/licensing via licensing_bridge.go.
//
// This is the Go-native equivalent of audit-private-boundary.sh
// --enforce-api-imports.
func TestNoInternalLicenseImportFromAPIProd(t *testing.T) {
	importPattern := regexp.MustCompile(`"github\.com/rcourtman/pulse-go-rewrite/internal/license(?:/[^"]*)?"\s*`)

	for name, content := range readGoFiles(t) {
		for i, line := range strings.Split(content, "\n") {
			if importPattern.MatchString(line) {
				t.Errorf("%s:%d: direct internal/license import — use pkg/licensing bridge wrappers instead", name, i+1)
			}
		}
	}
}

// TestNoInternalLicenseImportFromConsumerPackages ensures that no production
// file outside internal/api/ and internal/license/ imports internal/license
// (root package). Consumer packages must use pkg/licensing contracts.
//
// Scans all internal/ subdirectories (excluding api/ and license/), plus
// cmd/ and pkg/ at the repo root. This covers all runtime Go code; the
// shell-based audit-private-boundary.sh --enforce-nonapi-imports provides
// broader repo-wide coverage including non-runtime paths.
func TestNoInternalLicenseImportFromConsumerPackages(t *testing.T) {
	// Exempt directories under internal/:
	//   api     — covered by TestNoInternalLicenseImportFromAPIProd
	//   license — intra-package imports are fine
	exempt := map[string]bool{
		"api":     true,
		"license": true,
	}

	importPattern := regexp.MustCompile(`"github\.com/rcourtman/pulse-go-rewrite/internal/license"\s*`)

	// Scan all internal/ subdirectories (dynamically discovered).
	internalDir := filepath.Join("..")
	entries, err := os.ReadDir(internalDir)
	if err != nil {
		t.Fatalf("failed to read internal/ directory: %v", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() || exempt[entry.Name()] {
			continue
		}
		for path, content := range readConsumerGoFilesFromDir(t, entry.Name()) {
			for i, line := range strings.Split(content, "\n") {
				if importPattern.MatchString(line) {
					t.Errorf("%s:%d: direct internal/license import — consumer packages must use pkg/licensing contracts", path, i+1)
				}
			}
		}
	}

	// Also scan cmd/ and pkg/ at the repo root.
	repoRoot := filepath.Join("..", "..")
	for _, topDir := range []string{"cmd", "pkg"} {
		dir := filepath.Join(repoRoot, topDir)
		if _, statErr := os.Stat(dir); os.IsNotExist(statErr) {
			continue
		}
		files := make(map[string]string)
		walkErr := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() {
				return nil
			}
			if filepath.Ext(path) != ".go" || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			data, readErr := os.ReadFile(path)
			if readErr != nil {
				t.Fatalf("failed to read %s: %v", path, readErr)
			}
			files[path] = string(data)
			return nil
		})
		if walkErr != nil {
			t.Fatalf("failed to walk %s: %v", topDir, walkErr)
		}
		for path, content := range files {
			for i, line := range strings.Split(content, "\n") {
				if importPattern.MatchString(line) {
					t.Errorf("%s:%d: direct internal/license import — use pkg/licensing contracts instead", path, i+1)
				}
			}
		}
	}
}
