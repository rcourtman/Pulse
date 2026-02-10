# Worker Prompt: W1-C — Fix SSO Gating Consistency

## Task ID: W1-C (P0-3)

## Goal

Audit every feature constant in `features.go` against intended tier placement. Fix documentation inconsistencies. Produce a single source-of-truth entitlement matrix. The code is correct (basic SSO is free); the docs are wrong.

## Context

`internal/license/features.go` includes `FeatureSSO` ("sso") in `TierFeatures[TierFree]` at line 57. This means basic SSO (OIDC) is a free feature. However, MONETIZATION.md and the landing page claim SSO is a Pro feature. The decision is: **basic SSO (OIDC) is free** — it's table stakes in 2026. Advanced SSO (SAML, multi-provider, role mapping) remains Pro-only via `FeatureAdvancedSSO`.

## Scope (Exact Files)

### 1. Feature Audit: `internal/license/features.go`

Read the full tier matrix (lines 52-140) and verify every feature is in the correct tier:

**Expected placement:**

| Feature | Constant | Free | Pro | Notes |
|---------|----------|------|-----|-------|
| AI Patrol | `ai_patrol` | ✓ | ✓ | Patrol runs free, outcomes gated by autonomy |
| AI Auto-Fix | `ai_autofix` | ✗ | ✓ | Pro-only |
| AI Alerts | `ai_alerts` | ✗ | ✓ | Pro-only |
| Kubernetes AI | `kubernetes_ai` | ✗ | ✓ | Pro-only |
| Relay | `relay` | ✗ | ✓ | Pro-only (mobile requires relay) |
| Long-term Metrics | `long_term_metrics` | ✗ | ✓ | 90-day retention |
| Update Alerts | `update_alerts` | ✗ | ✓ | Pro-only |
| Agent Profiles | `agent_profiles` | ✗ | ✓ | Pro-only |
| Basic SSO | `sso` | ✓ | ✓ | **Free** (OIDC) |
| Advanced SSO | `advanced_sso` | ✗ | ✓ | SAML, multi-provider |
| RBAC | `rbac` | ✗ | ✓ | Pro-only |
| Audit Logging | `audit_logging` | ✗ | ✓ | Events captured for all; query/export Pro |
| Advanced Reporting | `advanced_reporting` | ✗ | ✓ | Pro-only |
| Multi-Tenant | `multi_tenant` | ✗ | ✓ | MSP/Enterprise |

If any feature is in the wrong tier in `features.go`, fix it. Based on prior analysis, the code is correct — but verify.

### 2. Fix MONETIZATION.md

- Find `MONETIZATION.md` in the repo root or docs/
- Update it to reflect that basic SSO (OIDC) is free for self-hosted
- Make clear: "Advanced SSO (SAML, multi-provider, role mapping)" is Pro
- Ensure tier descriptions match `features.go` exactly

### 3. Fix upgrade_reasons: `internal/license/conversion/upgrade_reasons.go`

- Check if there's an upgrade reason for `sso` (basic SSO). If so, **remove it** — you shouldn't prompt users to upgrade for a feature they already have.
- The upgrade reason for `advanced_sso` should remain (it's Pro-only).
- Verify all other upgrade reasons match the tier matrix above.

### 4. Create Entitlement Matrix: `docs/architecture/ENTITLEMENT_MATRIX.md` (NEW FILE)

Create a canonical entitlement matrix document that serves as the single source of truth. Include:
- Every feature key from `features.go`
- Which tiers include it
- The display name
- The gating mechanism (feature key, autonomy level, etc.)
- Reference to the code location (`features.go` line numbers)

### 5. Check in-app copy

Search the frontend for any references to SSO being "Pro only" or upgrade prompts for basic SSO:
```bash
grep -r "sso" frontend-modern/src/ --include="*.ts" --include="*.tsx" -l
grep -r "SSO" frontend-modern/src/ --include="*.ts" --include="*.tsx" -l
```
If any frontend code shows upgrade prompts for basic SSO, note it (but don't fix frontend in this task — that's W2-A's job).

### 6. Tests

Run existing feature gate tests to make sure nothing broke:
```bash
go test ./internal/license/... -count=1 -v -run "TestTierFeature|TestFeature"
```

If there are code_standards_test.go tests that validate feature consistency, run those too.

## Constraints

- Do NOT change `features.go` tier placements unless they contradict the matrix above (they shouldn't)
- Do NOT modify frontend code (that's a separate task)
- Do NOT add new feature constants
- Do NOT change the upgrade_reasons URL patterns

## Acceptance Checks

```bash
# Verify no code changes broke anything
go build ./internal/license/...
go test ./internal/license/... -count=1
go test ./internal/license/conversion/... -count=1

# Verify no SSO upgrade reason for basic SSO
grep -n '"sso"' internal/license/conversion/upgrade_reasons.go
# Expected: no match (only "advanced_sso" should appear)
```

## Expected Return

```
status: done | blocked
files_changed: [list with brief why for each]
commands_run: [command + exit code for each]
summary: [what was done]
blockers: [if any]
```
