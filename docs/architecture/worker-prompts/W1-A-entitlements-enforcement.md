# Worker Prompt: W1-A — Fix Hosted Entitlements Enforcement

## Task ID: W1-A (P0-0)

## Goal

Fix three methods in `internal/license/license.go` that ignore the evaluator when `license==nil`, and fix the entitlements endpoint to use evaluator capabilities when available. This is the #1 blocking bug for all hosted work — without it, hosted tenants always get free-tier behavior regardless of their billing state.

## The Bug

When a hosted tenant has billing state granting Pro capabilities (via `Service.SetEvaluator()`), but no JWT license key (which is normal for hosted — they don't use license keys), three methods fall through to free-tier behavior:

1. **`HasFeature()`** at line 367-368: When `s.evaluator != nil` but `s.license == nil`, returns `TierHasFeature(TierFree, feature)` — ignoring the evaluator entirely.
2. **`SubscriptionState()`** at line 462-464: When `s.license == nil`, returns `SubStateExpired` — never checks evaluator.
3. **`Status()`** at line 486-488: When `s.license == nil`, returns free-tier status — never consults evaluator.

## Scope (Exact Files)

### 1. `internal/license/license.go`

**Fix `HasFeature()` (lines 355-395):**
- In the evaluator branch (line 361-378), when `s.license == nil` AND `s.evaluator != nil`:
  - Delegate to `s.evaluator.HasCapability(feature)` instead of returning `TierHasFeature(TierFree, feature)`
  - The existing code at line 377 already does this for the happy path — the bug is specifically the nil-license early return at line 368

**Fix `SubscriptionState()` (lines 455-473):**
- Before the `s.license == nil` check at line 462, add an evaluator check:
  - If `s.evaluator != nil`, return `s.evaluator.SubscriptionState()`
  - Only fall through to `SubStateExpired` if evaluator is also nil

**Fix `Status()` (lines 476-521):**
- When `s.license == nil` but `s.evaluator != nil`:
  - Query evaluator for capabilities and subscription state
  - Populate `LicenseStatus.Features` from evaluator capabilities (not `TierFeatures[TierFree]`)
  - Set `Valid` based on evaluator subscription state (active/trial = valid; expired/suspended = invalid)
  - Set appropriate `Tier` (could use "pro" or derive from evaluator — Pro is fine since hosted tenants with active billing are effectively Pro)

### 2. `internal/api/entitlement_handlers.go`

**Fix `HandleEntitlements()` (lines 77-96):**
- Currently at line 89-92, builds payload from `svc.Status()` which returns features based on license tier
- When evaluator is available, the entitlements payload should reflect evaluator capabilities
- After fixing `Status()` above to respect evaluator, this may work automatically — but verify
- If `Status()` changes are not sufficient, add explicit evaluator path: check `svc.Evaluator() != nil`, if so build capabilities from `evaluator.HasCapability()` for each known feature key

### 3. `internal/license/license_test.go` (new or extend)

**Add regression test matrix covering all 4 combinations:**

```
1. license==nil, evaluator==nil       → free tier (HasFeature returns TierHasFeature(TierFree, f))
2. license==nil, evaluator!=nil       → evaluator drives (HOSTED PATH — the bug fix)
3. license!=nil, evaluator==nil       → JWT drives (SELF-HOSTED PATH)
4. license!=nil, evaluator!=nil       → JWT takes precedence (HYBRID)
```

For each combination, test:
- `HasFeature("ai_autofix")` — Pro-only feature
- `HasFeature("ai_patrol")` — Free feature
- `SubscriptionState()` — correct state string
- `Status()` — correct Features list and Valid flag

## Constraints

- Do NOT change the evaluator interface (`internal/license/entitlements/evaluator.go`)
- Do NOT change the billing state struct
- Do NOT touch frontend code
- JWT license must still take precedence when both license and evaluator are set
- Free-tier behavior when NEITHER license nor evaluator is set must be preserved exactly
- Thread safety: `HasFeature`, `Status`, `SubscriptionState` all hold `s.mu.RLock()` — maintain this

## Key Interfaces You'll Need

```go
// Evaluator interface (entitlements/evaluator.go)
type Evaluator interface {
    HasCapability(capability string) bool
    SubscriptionState() string
    Limits() map[string]int64
    Refresh() error
}

// Service.Evaluator() returns the evaluator (license.go)
func (s *Service) Evaluator() *entitlements.Evaluator

// Service.SetEvaluator() sets the evaluator (license.go)
func (s *Service) SetEvaluator(e *entitlements.Evaluator)
```

## Acceptance Checks

```bash
# Must pass
go build ./internal/license/...
go build ./internal/api/...
go test ./internal/license/... -count=1 -v -run "TestHasFeature|TestSubscriptionState|TestStatus|TestEvaluator"
go test ./internal/api/... -count=1 -v -run "TestEntitlement"

# Must also still pass (existing tests)
go test ./internal/license/... -count=1
go test ./internal/api/... -count=1 -run "TestHostedLifecycle"
```

## Expected Return

```
status: done | blocked
files_changed: [list with brief why for each]
commands_run: [command + exit code for each]
summary: [what was done]
blockers: [if any]
```
