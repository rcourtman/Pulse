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
   The active profile also owns the governed prerelease and stable release
   branches for that line, so workflows and local release tooling must resolve
   branch requirements from the control plane instead of hard-coding `main`.
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
8. Executable proof runs must follow the active target.
   Local hooks and CI should resolve readiness-assertion proof scope from the
   active target completion rule instead of hard-coding only repo-ready or
   future-phase release-ready proof surfaces.
   Manual targets may declare an explicit proof scope when they should still
   pull machine proofs, or `none` when the hold is intentionally human-owned
   and should not emit derivation warnings.
9. Direction changes must be normalized.
   When the user states a durable product truth or changes the current product
   priority, the agent must classify that direction as a readiness assertion,
   release gate, open decision, or active-target update instead of leaving it
   as informal chat.
   Treat casual statements about consistency, seamlessness, drift, bypass
   resistance, or things that "should always be true" as candidate governance
   input, not only explicit "add an assertion" requests.

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
6. the governed prerelease and stable release branches for that profile

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
For release targets, `rc_ready` means a governed prerelease candidate can be
cut, while `release_ready` means stable or GA promotion readiness after RC
validation.
Executable readiness proof runs should follow that same phase boundary: the
active target should pull in only the readiness blocking levels required for
its completion rule, not future-phase proof suites by default.
When a target is intentionally human-held, declare that explicitly in
`control_plane.json` rather than relying on a derivation failure to signal it.
Do not wait for a special governance prompt before checking whether informal
user language should update the control plane.

## Current State

1. v6 is the current active release profile.
2. `v6-rc-stabilization` is the current active engineering target.
   It is intentionally manual and declares `proof_scope: none`, so the repo
   reports a human-held stabilization state without pretending that proof-scope
   derivation failed.
3. `v6-rc-cut` is complete and remains the immediate predecessor target.
4. `v6-ga-promotion` is a planned follow-on target, not the current objective.
5. Its files remain under `docs/release-control/v6/`.
6. The existing v6 control surfaces are still live, but they now sit underneath
   an evergreen Pulse control plane rather than pretending to be the whole
   long-term system.
7. Until the explicit post-GA branch cutover happens, both prerelease and
   stable v6 promotions resolve to `pulse/v6` via `control_plane.json`.
