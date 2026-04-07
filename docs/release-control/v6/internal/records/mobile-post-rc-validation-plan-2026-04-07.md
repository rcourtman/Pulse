# Mobile Post-RC Validation Plan Record

- Date: `2026-04-07`
- Decision: `mobile-post-rc-validation-plan`
- Scope:
  - `pulse-mobile`
  - `pulse`
  - `pulse-pro`
  - lane `L5`
  - subsystems `frontend-primitives`, `relay-runtime`

## Decision

Pulse Mobile should not widen just because the current v1 shell now feels
coherent.
The next step is a short operator validation pass, then a narrow set of
authority-building slices.

The validation cohort should be `5-10` existing Pulse users who already have a
real reason to check infrastructure away from the desk.
They should exercise the current build on the phone they would actually carry,
not a narrated prototype walk-through.

## Validation Script

Each participant should be asked to complete the same tasks:

1. Open the app and say whether anything needs attention right now.
2. Identify the next thing they would act on and explain why.
3. Open a pending approval or finding and decide what they would do.
4. Recover from a stale, disconnected, or trust-degraded state without desktop
   help.
5. Switch instances and confirm they still understand what needs attention.

The moderator should avoid explaining the intended information architecture
unless the participant is completely stuck.

## Pass Signals

The build is strong enough to widen only if most participants can:

1. Explain what `Now` is for without prompting.
2. Identify the next action quickly from the first screen.
3. Distinguish status, findings, approvals, and follow-up work without tab
   hunting.
4. Understand whether the app is trustworthy when data is stale, disconnected,
   or degraded.
5. Ask for one missing authority cue rather than asking for a full desktop
   dashboard to feel safe.

## Failure Signals

The current shape still needs redesign or trust work if participants:

1. Cannot tell whether anything is broken from the opening screen.
2. Treat the app as a settings shell, notification inbox, or incomplete
   dashboard.
3. Bounce across multiple tabs before they trust the current state.
4. Refuse to act because freshness or trust posture is unclear.
5. Ask for graphs or desktop parity just to answer a basic triage question.

## Evidence Capture

Capture the same evidence for every session:

1. Participant type and environment shape.
2. First point of hesitation on `Now`, `Instances`, `Findings`, `Approvals`,
   or `Follow-Ups`.
3. Missing information they asked for in their own words.
4. Whether the gap is:
   - missing authority cue
   - trust ambiguity
   - information architecture confusion
   - true non-goal request
5. Whether they would keep the app installed after the session and why.

## Post-Test Build Slices

If the current v1 shape passes the validation bar, widen in this order:

1. `ops-snapshot-authority`
   - Add one stronger authority layer to `Now` and `Instances`: top affected
     instance, top affected workload/resource, and last successful
     contact/freshness age.
   - Do not add graphs or broad fleet browsing.
2. `trust-repair-fast-path`
   - Tighten stale, offline, revoked, and reconnect states into one clear
     recovery path from `Now` and `Instances`.
   - Prefer direct repair actions over explanatory copy sprawl.
3. `action-confidence`
   - Strengthen approval and finding detail surfaces so the user can understand
     risk, likely outcome, and post-action state without returning to desktop.
4. `hardware-release-signoff`
   - Re-run physical-device iOS `approval-actions` and `push-routing` proof
     before any public rollout judgment.

If validation shows that users still want slightly more monitoring authority
after those slices, the next admissible expansion is:

5. `ops-snapshot-drilldown`
   - Add one compact drilldown from `Ops Snapshot` for top affected resources
     and freshness context.
   - Keep it secondary to triage; do not add a new primary tab.

## Explicit Non-Goals

The post-RC mobile follow-up should still reject:

1. Graph-first dashboard recreation.
2. Extra primary tabs to compensate for weak primary information architecture.
3. Desktop-feature parity as a mobile success criterion.

## Outcome

- `mobile-post-rc-hardening` now has a concrete execution order:
  validate with real operators, widen through narrow authority/trust slices,
  then clear the remaining hardware proof gates.
- Mobile product-ready judgment should be based on operator validation plus
  hardware trust proof, not on aesthetics or unbounded parity expansion.
