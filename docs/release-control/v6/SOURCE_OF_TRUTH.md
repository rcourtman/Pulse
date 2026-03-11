# Pulse v6 Source Of Truth

Last updated: 2026-03-11
Status: ACTIVE

This file is the stable human governance layer for Pulse v6.
It is not a live progress dashboard.

Current lane scores, evidence references, and typed operational decision
records live only in `docs/release-control/v6/status.json`.

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
   Live lane state, structured evidence references, and open operational
   decision records.
2. `docs/release-control/v6/CANONICAL_DEVELOPMENT_PROTOCOL.md`
   Repo-wide change rules for canonical work.
3. `docs/release-control/v6/subsystems/registry.json`
   Machine-readable subsystem ownership and proof requirements.
4. `docs/release-control/v6/subsystems/*.md`
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

## Non-Negotiable Release Gates

1. Do not ship with open trust-critical P0s.
2. Do not ship when paywalls and runtime gates disagree.
3. Do not ship hosted flows that break signup, auth, provision, or revocation.
4. Do not keep polishing strong lanes while weak lanes remain behind.

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
5. Update `status.json` when live lane state, evidence references, or open
   operational decision records change.
6. Update this file only when stable governance, scope, or locked decisions
   change.
7. When a canonical path replaces an old path, add or tighten a guardrail so
   the old path cannot silently return.

## Source Domains

If conflicts appear, resolve by domain:

1. `docs/release-control/v6/CANONICAL_DEVELOPMENT_PROTOCOL.md`,
   `docs/release-control/v6/subsystems/registry.json`, and the relevant
   subsystem contract own implementation rules.
2. `docs/release-control/v6/status.json` owns live lane state, structured
   evidence references, and typed operational decision records.
3. This file owns stable governance, repo scope, release gates, and locked
   decisions.
4. Supporting architecture and release docs are evidence only. They do not
   override the files above.
