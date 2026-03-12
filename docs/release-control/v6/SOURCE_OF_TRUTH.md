# Pulse v6 Source Of Truth

Last updated: 2026-03-12
Status: ACTIVE

This file is the stable human governance layer for the active v6 release
profile. It is not a live progress dashboard.

Evergreen control-plane governance now lives in
`docs/release-control/CONTROL_PLANE.md`, and the active profile plus active
target are machine-selected by `docs/release-control/control_plane.json`.

Current lane scores, evidence references, and typed operational decision
records live only in `docs/release-control/v6/status.json`.
Current repo/release readiness is derived by
`python3 scripts/release_control/status_audit.py --pretty` from
`status.json`, not hand-maintained in this file. The human runbook for
trust-critical manual release gates lives in
`docs/release-control/v6/HIGH_RISK_RELEASE_VERIFICATION_MATRIX.md`.
Readiness assertion design rules live in this file.
The active assertion catalog, proof references, and executable proof commands
live in `status.json.readiness_assertions`.

## Purpose

1. Define the stable execution model for the active v6 release profile.
2. Lock profile-specific scope, release definition, and no-go rules.
3. Record only long-lived architectural and release decisions for this profile.
4. Point agents to the files that own live state and subsystem truth for this
   profile.

This file must not contain:

1. hand-maintained lane score tables
2. hand-maintained evidence lists
3. ephemeral session recaps or task logs

## Canonical Control Files

1. `docs/release-control/CONTROL_PLANE.md`
   Evergreen governance for the permanent Pulse release control plane.
2. `docs/release-control/control_plane.json`
   Machine-readable control-plane state, including the active profile and
   active target.
3. `docs/release-control/control_plane.schema.json`
   Machine-readable contract for `control_plane.json`.
4. `docs/release-control/v6/status.json`
   Live lane state, structured evidence references, and typed operational
   decision records.
5. `docs/release-control/v6/status.schema.json`
   Machine-readable contract for the `status.json` shape.
6. `docs/release-control/v6/CANONICAL_DEVELOPMENT_PROTOCOL.md`
   Repo-wide change rules for canonical work.
7. `docs/release-control/v6/subsystems/registry.json`
   Machine-readable subsystem ownership and proof requirements.
8. `docs/release-control/v6/subsystems/registry.schema.json`
   Machine-readable contract for the subsystem registry shape.
9. `docs/release-control/v6/subsystems/*.md`
   Per-subsystem contracts: truth, extension points, forbidden paths, and
   completion obligations.
10. `docs/release-control/v6/RELEASE_PROMOTION_POLICY.md`
    Canonical stable-versus-RC promotion rules, rollout criteria, and rollback
    expectations for v6 and later release lines.

## Scope

`status.json.scope.control_plane_repo` is `pulse`.
`status.json.scope.repo_catalog` is the canonical machine-readable repo map for
the active Pulse workspace.

Active repositories for v6:

1. `pulse`
   Core desktop/runtime repo and the canonical v6 release-control authority.
2. `pulse-pro`
   Financial, operational, checkout, license-server, and relay-server
   surfaces.
3. `pulse-enterprise`
   Closed-source enterprise and paid runtime features.
4. `pulse-mobile`
   Mobile client, relay pairing, approvals, and device-local auth/state.

Ignored for v6 control:

1. `pulse-5.1.x`
2. `pulse-refactor-streams`

## Release Definition

Pulse v6 is ready when these outcomes land together:

1. Unified resource model is stable and expansion-ready.
2. Product quality feels polished and trustable out of the box.
3. Commercial packaging materially improves conversion and revenue.
4. Stable or GA promotion happens only after RC validation, not as the first
   customer exposure.

## Evergreen Readiness Assertions

Pulse v6 readiness is governed by a small evergreen assertion set.
These assertions are durable release truths, not one-off launch checklist
items.
`status.json.readiness_assertions` owns the active assertion catalog, proof
references, and executable proof commands for the active release line.
`docs/release-control/control_plane.json` owns the active engineering target
that this profile is currently pursuing.
`python3 scripts/release_control/control_plane_audit.py --check` enforces that
the active target cannot stay stale once its machine-derived completion rule is
already satisfied.

Assertion design rules:

1. Keep the set small and durable.
2. Write each assertion as a binary release truth, not as a task list.
3. Map each assertion to concrete lanes, governed subsystems, or release gates.
4. Prefer machine-derived proof and generated summaries over hand-maintained
   docs.
5. If repeated manual proof becomes expensive, automate it or demote it to a
   one-time migration item instead of keeping it in the evergreen set.
6. After GA, keep using the same control plane and promote a new active target
   instead of cloning a new governance stack.
7. When a user states a durable product truth, normalize it into a readiness
   assertion, release gate, or open decision rather than leaving it as chat.
8. Treat casual user language about consistency, seamlessness, drift, bypass
   resistance, or things that "should always be true" as candidate governance
   input, not only explicit requests to add a formal assertion.

## Non-Negotiable Release Gates

1. Do not ship with open trust-critical P0s.
2. Do not ship when paywalls and runtime gates disagree.
3. Do not ship hosted flows that break signup, auth, provision, or revocation.
4. Do not ship upgrades that reset paid state, licensing continuity, or first-session flow.
5. Do not keep polishing strong lanes while weak lanes remain behind.
6. Do not treat `status.json` lane scores reaching target as sufficient release approval by themselves; open operational decisions, machine-derived unresolved readiness assertions, and unresolved release gates still apply.
7. Do not promote v6 to stable or GA without an exercised RC, a real release-pipeline proof run, a recorded rollback target, and a written v5 maintenance-only support policy.

## Locked Decisions

1. Release-control execution is direct and repo-aware. The old orchestrator and
   loop tooling are retired.
2. v6 GA is gated by `L1`, `L2`, `L3`, `L5`, `L6`, `L7`, `L8`, `L9`, `L10`,
   `L11`, and `L12`. `L4` remains post-GA track work and is not a GA floor
   gate.
3. Trial authority for v6 is SaaS-controlled.
   `POST /api/license/trial/start` must initiate hosted signup only; the local
   runtime may redeem signed trial activations but must not mint local trial
   state directly.
4. v5 to v6 commercial migration must preserve unresolved paid-license state
   and downgrade safety.
5. Cloud and MSP Stripe `price_*` IDs are operational fill-in items, not
   architectural blockers.
6. Stable or GA promotion for v6 must come from an exercised RC and stay
   blocked until the RC-to-GA promotion gate and v5 maintenance-policy
   decision are both resolved.
7. v6 and later releases use a promotion model, not a direct broad-rollout
   model.
   `stable` must receive only promoted, already-validated builds, `rc` is the
   opt-in preview channel, and unattended auto-update exposure remains
   `stable`-only unless a new channel policy is explicitly adopted.
8. Once v6 reaches stable or GA, v5 enters a 90-day maintenance-only window:
   critical security issues, critical correctness/data-loss issues, and safe
   migration blockers only. After that window, v5 is unsupported.

## Cross-Repo Contracts

These contracts must not drift:

1. Licensing contract:
   `pulse/pkg/licensing` semantics vs license-server plans and gates.
2. Relay grant contract:
   license-server issued grants vs relay-server acceptance.
3. Relay protocol contract:
   `pulse/internal/relay`, `pulse-pro/relay-server`, and
   `pulse-mobile/src/relay` wire compatibility.
4. Pricing contract:
   architecture pricing docs vs Stripe and checkout wiring in `pulse-pro`.

## Development Governance

For canonical subsystem work:

1. Read `docs/release-control/CONTROL_PLANE.md`,
   `docs/release-control/control_plane.json`, this file, and
   `docs/release-control/v6/status.json` first.
   If the agent starts outside the `pulse` repo, resolve those files from
   `pulse/docs/release-control/v6/` under the shared workspace root rather than
   inventing a parallel control layer in the current repo.
2. Then read `docs/release-control/v6/CANONICAL_DEVELOPMENT_PROTOCOL.md`.
3. Then read `docs/release-control/v6/subsystems/registry.json`.
4. Then read the relevant subsystem contract under
   `docs/release-control/v6/subsystems/`.
5. Update `status.json` when live lane state, readiness derivation rules,
   assertion proof routes, evidence references, or typed operational decision
   records change.
6. Update this file only when stable governance, scope, locked decisions, or
   the readiness-assertion design rules change.
7. When a canonical path replaces an old path, add or tighten a guardrail so
   the old path cannot silently return.
8. When the active target's machine-derived completion rule is satisfied,
   update `docs/release-control/control_plane.json` in the same task or stop
   and promote the next target before continuing under stale scope.
9. When the user changes Pulse's priority or says what the product should focus
   on next, classify that as an active-target update or another control-plane
   change instead of leaving it as informal discussion.
10. Do not wait for a special governance prompt.
    If the user casually says something should be consistent, seamless,
    difficult to bypass, free of drift, or always true, decide whether it
    belongs in a readiness assertion, release gate, open decision, or active
    target before ending the task.

For readiness assertion work:

1. Update this file only when the readiness-assertion design rules change.
2. Update `status.json.readiness_assertions` when the active assertion catalog
   or proof references/proof commands change.
3. Route manual assertion proof through
   `HIGH_RISK_RELEASE_VERIFICATION_MATRIX.md` whenever the assertion needs a
   trust-critical release gate instead of a one-off checklist.

## Source Domains

If conflicts appear, resolve by domain:

1. `docs/release-control/v6/CANONICAL_DEVELOPMENT_PROTOCOL.md`,
   `docs/release-control/v6/subsystems/registry.json`, and the relevant
   subsystem contract own implementation rules.
2. `docs/release-control/v6/status.json` owns live lane state, the active
   readiness assertion catalog, readiness derivation rules, executable proof
   commands, structured evidence references, and typed operational decision
   records.
3. `docs/release-control/v6/status.schema.json` owns the machine-readable shape
   contract for `status.json`.
4. `docs/release-control/v6/subsystems/registry.schema.json` owns the
   machine-readable shape contract for the subsystem registry.
5. `docs/release-control/CONTROL_PLANE.md` and
   `docs/release-control/control_plane.json` own evergreen governance, active
   profile selection, and active target selection.
6. This file owns profile-specific governance, repo scope, release gates,
   readiness assertion design rules, and locked decisions for v6.
7. Supporting architecture and release docs are evidence only. They do not
   override the files above.
