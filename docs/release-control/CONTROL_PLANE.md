# Pulse Release Control Plane

This directory is the evergreen governance layer for Pulse release control.

It is not tied to one release line. It defines the long-lived control-plane
model that active and future release profiles reuse.

## Purpose

1. Keep one canonical governance system across releases.
2. Separate evergreen control-plane rules from release-specific profile state.
3. Make the active release profile explicit and machine-resolvable.
4. Make the active engineering target explicit so the objective can change
   without replacing the control plane itself.
5. Prevent future releases from cloning a new `v7`, `v8`, or `v9` governance
   stack when the system itself should remain continuous.

## Control-Plane Rules

1. Governance is continuous.
   Repo hardening, subsystem ownership, proof routing, drift guards, and
   readiness assertion mechanics are permanent parts of Pulse engineering.
2. Release profiles are temporary.
   Open decisions, manual release gates, migration obligations, and
   release-specific readiness concerns belong to the active profile, not to the
   evergreen control plane.
3. There must be one active profile at a time.
   `control_plane.json` is the machine authority for which release profile is
   currently active and where its files live.
4. Profile changes must reuse the same control-plane machinery.
   Future releases should switch or add profiles rather than fork the guardrail
   system.
5. The active target must never be implicit.
   `control_plane.json` must always declare the current target, even when the
   target changes from release hardening to stabilization, polish, or feature
   delivery.
6. The control plane must stay self-policing.
   Audit scripts, completion guards, and repo tests must derive the active
   profile from `control_plane.json` instead of hard-coding version-specific
   paths.
7. A completed active target must not linger.
   When a machine-derivable target completion rule is satisfied,
   `control_plane_audit.py --check` must fail until the current target is
   marked complete and a new active target is promoted.
8. Direction changes must be normalized.
   When the user states a durable product truth or changes the current product
   priority, the agent must classify that direction as a readiness assertion,
   release gate, open decision, or active-target update instead of leaving it
   as informal chat.

## Canonical Files

1. `docs/release-control/CONTROL_PLANE.md`
   Stable human governance for the evergreen control plane.
2. `docs/release-control/control_plane.json`
   Machine-readable control-plane state, including the active profile and
   active target.
3. `docs/release-control/control_plane.schema.json`
   Machine-readable contract for `control_plane.json`.

## Active Profile Rule

The active release profile owns:

1. profile-specific source of truth
2. live lane and readiness state
3. active release gates and open decisions
4. subsystem registry and contracts
5. release-specific runbooks and checklists

The evergreen control plane owns:

1. the profile selection mechanism
2. the active engineering target selection mechanism
3. the continuous governance model
4. the requirement that tooling resolves the active profile instead of assuming
   one fixed version forever

## Active Target Rule

The control plane must always declare one active target.

Targets may change over time, for example:

1. release hardening
2. post-release stabilization
3. polish
4. a named feature initiative
5. maintenance or reliability work

The target should change without rebuilding the governance system.
Agents should update the active target when direction changes, and the audits
should fail when a target with a machine-derivable completion rule is already
complete but still marked active.
Durable truths that are not target changes should be normalized into
readiness assertions, release gates, or open decisions rather than copied into
the target text.

## Current State

1. v6 is the current active release profile.
2. `v6-release` is the current active engineering target.
3. Its files remain under `docs/release-control/v6/`.
4. The existing v6 control surfaces are still live, but they now sit underneath
   an evergreen Pulse control plane rather than pretending to be the whole
   long-term system.
