# RC Publication Judgment Packet

- Date: `2026-04-09`
- Decision: `rc-publication-judgment`
- Active target: `v6-rc-cut`
- Active branch: `pulse/v6-release`
- Result: `blocked`

## Scope

This packet prepares the explicit owner judgment for publishing a governed Pulse
v6 RC. It does not make the product approval decision.

The current release-control target still uses the `rc_ready` completion rule.
GA or stable promotion remains out of scope until a real prerelease has shipped
and `rc-to-ga-promotion-readiness` is cleared under the later release-ready
phase.

## Governance Snapshot

Commands run from `/Volumes/Development/pulse/repos/pulse`:

1. `python3 scripts/release_control/agent_preflight.py --pretty`
   - Result: pass.
   - Branch matched `pulse/v6-release`.
   - Active target was `v6-rc-cut`.
2. `python3 scripts/release_control/status_audit.py --pretty`
   - Result: pass.
   - `repo_ready=True`.
   - `rc_ready=False`.
   - `release_ready=False`.
   - All 16 lanes were at the target floor.
   - All rc-ready release gates were recorded as passed.
   - The only governed rc-ready blocker remained this open decision:
     `rc-publication-judgment`.
3. `python3 scripts/release_control/release_promotion_policy_test.py`
   - Result: pass.
   - This only proves the GA promotion policy guardrails; it does not clear
     stable or GA promotion.

## Active-Target Proof Run

Command:

```bash
python3 scripts/release_control/readiness_assertion_guard.py --active-target
```

The first active-target run found a stale `RA16` proof expectation:

1. `pulse-api-cancellation-boundary`: pass.
2. `pulse-v5-recurring-upgrade-migration`: pass.
3. `frontend-grandfathered-license-presentation`: fail.
4. `pulse-pro-public-checkout-reentry-guard`: pass.

The failure was an exact presentation snapshot that had not been extended after
the self-hosted Pulse Pro billing presentation gained canonical demo-hidden and
policy-loading copy. Runtime behavior did not need to change.

The snapshot was updated in
`frontend-modern/src/utils/__tests__/licensePresentation.test.ts`, then the
direct proof was rerun:

```bash
python3 scripts/release_control/internal/commercial_cancellation_reactivation_proof.py
```

Result:

1. `pulse-api-cancellation-boundary`: pass.
2. `pulse-v5-recurring-upgrade-migration`: pass.
3. `frontend-grandfathered-license-presentation`: pass.
4. `pulse-pro-public-checkout-reentry-guard`: pass.

## Current Blocker

After the `RA16` proof repair, the full active-target guard was rerun. It
progressed through the earlier RC proof slices and then blocked at `RA17`:

```text
BLOCKED: readiness assertion proof failed for RA17:mobile-relay-auth-approvals-proof (exit 1)
```

The isolated `RA17` proof showed this failing subcheck:

```text
enterprise-approval-handlers
cwd: /Volumes/Development/pulse/repos/pulse-enterprise
command: go test ./internal/aiautofix -run 'TestHandleListApprovals|TestHandleApproveAndExecuteInvestigationFix|TestHandleApprove' -count=1
detail: go mod tidy
```

The same command run directly in `pulse-enterprise` returned:

```text
go: updates to go.mod needed; to update it:
	go mod tidy
```

The `pulse-enterprise` repo already had unrelated dirty files in
`cmd/pulse-enterprise/main.go` and `cmd/pulse-enterprise/main_test.go` before
this packet was written. Those changes appear to belong to another parallel
slice, so this packet does not run `go mod tidy` or edit `pulse-enterprise`.

The mobile portions of the `RA17` proof passed:

1. `mobile-api-client`: pass.
2. `mobile-relay-runtime`: pass.
3. `mobile-secure-persistence-and-approvals`: pass.
4. `mobile-wire-protocol`: pass.

The relay reconnect/drain proof was run separately:

```bash
python3 scripts/release_control/readiness_assertion_guard.py --assertion RA18
```

Result: pass.

## Judgment Outcome

Do not clear `rc-publication-judgment` from this packet.

The release-control metadata is close to the RC floor, but the current
workspace cannot complete the active-target proof while the enterprise approval
handler slice requires `go mod tidy`. Publishing an RC from this state would
turn a parallel worktree hygiene problem into an ambiguous release decision.

## Required Unblock Steps

1. Resolve the `pulse-enterprise` mod-tidy requirement in the owning
   enterprise slice.
2. Return all active repos needed for the RC proof to a clean, intentional
   state.
3. Rerun:

```bash
python3 scripts/release_control/readiness_assertion_guard.py --active-target
python3 scripts/release_control/status_audit.py --pretty
```

4. If both commands pass, ask the owner to make the explicit product judgment
   on `rc-publication-judgment`.
5. Keep `rc-to-ga-promotion-readiness` blocked until a real prerelease has
   shipped and the later GA promotion rehearsal exists.
