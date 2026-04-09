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

## Enterprise Follow-Up

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
slice, so the dependency fix stayed limited to the module graph only.

`pulse-enterprise` was repaired with:

```text
abdcd2d chore(deps): tidy enterprise module graph
```

That commit touched only `go.mod` and `go.sum`.

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

The isolated `RA17` proof was then rerun:

```bash
python3 scripts/release_control/internal/mobile_relay_auth_approvals_proof.py --json
```

Result: all subchecks passed, including `enterprise-approval-handlers`.

## Proof Refresh

After the packet was first recorded, the in-progress Pulse monitored-system
enforcement/admission slice advanced and the full active-target proof was
rerun:

```bash
python3 scripts/release_control/readiness_assertion_guard.py --active-target
```

Result: pass.

The rerun completed the full governed active-target proof, including the
earlier `RA3`/`RA4` entitlement-gating slice, the repaired enterprise-backed
`RA17` mobile approvals proof, relay reconnect/drain proof, commercial
cancellation/reactivation proof, quickstart/browser proof, and the remaining
rc-ready assertion set.

`python3 scripts/release_control/status_audit.py --pretty` still reports:

1. `repo_ready=True`.
2. `rc_ready=False`.
3. `release_ready=False`.
4. All rc-ready release gates passed.
5. `rc-publication-judgment` as the only remaining rc-ready blocker.

## Judgment Outcome

Do not clear `rc-publication-judgment` from this packet.

The release-control proof floor is now satisfied, but prerelease publication is
still an explicit owner product decision rather than an automatic consequence of
passing proof. This packet is now ready for that judgment; it does not make the
judgment itself.

## Required Unblock Steps

1. Review this packet as the explicit RC publication decision input.
2. If the owner judges the current candidate ready for a real governed RC,
   resolve `rc-publication-judgment` explicitly instead of treating proof
   completion as implicit approval.
3. If the owner does not approve publication yet, keep
   `rc-publication-judgment` open and record the reason as a product decision,
   not a missing proof.
4. Keep `rc-to-ga-promotion-readiness` blocked until a real prerelease has
   shipped and the later GA promotion rehearsal exists.
