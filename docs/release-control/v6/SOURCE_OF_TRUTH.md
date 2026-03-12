# Pulse v6 Source Of Truth

Last updated: 2026-03-12
Status: ACTIVE

This file is the stable human governance layer for Pulse v6.
It is not a live progress dashboard.

Current lane scores, evidence references, and typed operational decision
records live only in `docs/release-control/v6/status.json`.
`status.json.readiness.repo_ready` and `status.json.readiness.release_ready`
are the canonical machine-visible distinction between repo readiness and
release readiness. The human runbook for trust-critical manual release gates
lives in `docs/release-control/v6/HIGH_RISK_RELEASE_VERIFICATION_MATRIX.md`.
Stable readiness assertion definitions live in this file; live readiness
assertion state and proof references live in
`status.json.readiness_assertions`.

## Purpose

1. Define the stable execution model for v6 work.
2. Lock repo scope, release definition, and no-go rules.
3. Record only long-lived architectural and release decisions.
4. Point agents to the files that own live state and subsystem truth.

This file must not contain:

1. hand-maintained lane score tables
2. hand-maintained evidence lists
3. ephemeral session recaps or task logs

## Canonical Control Files

1. `docs/release-control/v6/status.json`
   Live lane state, structured evidence references, and typed operational
   decision records.
2. `docs/release-control/v6/status.schema.json`
   Machine-readable contract for the `status.json` shape.
3. `docs/release-control/v6/CANONICAL_DEVELOPMENT_PROTOCOL.md`
   Repo-wide change rules for canonical work.
4. `docs/release-control/v6/subsystems/registry.json`
   Machine-readable subsystem ownership and proof requirements.
5. `docs/release-control/v6/subsystems/registry.schema.json`
   Machine-readable contract for the subsystem registry shape.
6. `docs/release-control/v6/subsystems/*.md`
   Per-subsystem contracts: truth, extension points, forbidden paths, and
   completion obligations.

## Scope

Active repositories for v6:

1. `pulse`
2. `pulse-pro`
3. `pulse-enterprise`
4. `pulse-mobile`

Ignored for v6 control:

1. `pulse-5.1.x`
2. `pulse-refactor-streams`

## Release Definition

Pulse v6 is ready when these outcomes land together:

1. Unified resource model is stable and expansion-ready.
2. Product quality feels polished and trustable out of the box.
3. Commercial packaging materially improves conversion and revenue.

## Evergreen Readiness Assertions

Pulse v6 readiness is governed by a small evergreen assertion set.
These assertions are durable release truths, not one-off launch checklist
items. This file owns the stable assertion definitions.
`status.json.readiness_assertions` owns the live assertion state, proof
references, and blocking status for the active release line.

Assertion design rules:

1. Keep the set small and durable.
2. Write each assertion as a binary release truth, not as a task list.
3. Map each assertion to concrete lanes, governed subsystems, or release gates.
4. If repeated manual proof becomes expensive, automate it or demote it to a
   one-time migration item instead of keeping it in the evergreen set.

Current v6 assertion set:

1. `RA1` repo-ready invariant:
   No governed surfaces that should use the unified resource model may keep
   shipping on legacy resource paths or response shapes.
2. `RA2` release-ready journey:
   A new user can complete Pulse Pro Relay signup through paid activation
   without manual operator intervention or ambiguous provisioning steps.
3. `RA3` release-ready invariant:
   After first successful entitlement activation, Pulse preserves paid state
   across normal sessions and supported upgrades without repeated license
   entry.
4. `RA4` release-ready trust gate:
   Typical end users should not be able to trivially unlock Pulse Pro features
   by removing client-only checks while server, hosted, or signed-entitlement
   enforcement remains absent.
5. `RA5` release-ready invariant:
   Non-paid users do not see paid-only navigation or pages unless the surface
   is explicitly promotional.
6. `RA6` release-ready journey:
   Supported upgrades preserve core state, entitlements, and first-session
   continuity without manual repair work.

## Non-Negotiable Release Gates

1. Do not ship with open trust-critical P0s.
2. Do not ship when paywalls and runtime gates disagree.
3. Do not ship hosted flows that break signup, auth, provision, or revocation.
4. Do not ship upgrades that reset paid state, licensing continuity, or first-session flow.
5. Do not keep polishing strong lanes while weak lanes remain behind.
6. Do not treat `status.json` lane scores reaching target as sufficient release approval by themselves; open operational decisions, unresolved readiness assertions, and unresolved release gates still apply.

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

1. Read this file and `docs/release-control/v6/status.json` first.
2. Then read `docs/release-control/v6/CANONICAL_DEVELOPMENT_PROTOCOL.md`.
3. Then read `docs/release-control/v6/subsystems/registry.json`.
4. Then read the relevant subsystem contract under
   `docs/release-control/v6/subsystems/`.
5. Update `status.json` when live lane state, live readiness assertion state,
   evidence references, or typed operational decision records change.
6. Update this file only when stable governance, scope, locked decisions, or
   evergreen readiness assertions change.
7. When a canonical path replaces an old path, add or tighten a guardrail so
   the old path cannot silently return.

For readiness assertion work:

1. Update this file when the stable assertion set or assertion wording changes.
2. Update `status.json.readiness_assertions` when live proof state or blocking
   status changes.
3. Route manual assertion proof through
   `HIGH_RISK_RELEASE_VERIFICATION_MATRIX.md` whenever the assertion needs a
   trust-critical release gate instead of a one-off checklist.

## Source Domains

If conflicts appear, resolve by domain:

1. `docs/release-control/v6/CANONICAL_DEVELOPMENT_PROTOCOL.md`,
   `docs/release-control/v6/subsystems/registry.json`, and the relevant
   subsystem contract own implementation rules.
2. `docs/release-control/v6/status.json` owns live lane state, live readiness
   assertion state, structured evidence references, and typed operational
   decision records.
3. `docs/release-control/v6/status.schema.json` owns the machine-readable shape
   contract for `status.json`.
4. `docs/release-control/v6/subsystems/registry.schema.json` owns the
   machine-readable shape contract for the subsystem registry.
5. This file owns stable governance, repo scope, release gates, evergreen
   readiness assertion definitions, and locked decisions.
6. Supporting architecture and release docs are evidence only. They do not
   override the files above.
