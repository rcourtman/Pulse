# Pulse Release Control Plane

This directory is the evergreen governance layer for Pulse release control.

It is not tied to one release line. It defines the long-lived control-plane
model that active and future release profiles reuse.

## Purpose

1. Keep one canonical governance system across releases.
2. Keep one canonical values layer for agent behavior instead of pushing
   detailed operating burden into prompts.
3. Separate evergreen control-plane rules from release-specific profile state.
4. Make the active release profile explicit and machine-resolvable.
5. Make the active engineering target explicit so the objective can change
   without replacing the control plane itself.
6. Prevent future releases from cloning a new `v7`, `v8`, or `v9` governance
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
9. High-risk closure must respect evidence quality.
   A passed release gate is not enough by itself. `status.json.release_gates`
   must declare the minimum evidence tier needed for closure, and the audit
   layer must treat lower-tier evidence as rehearsal only rather than as
   high-confidence completion.
10. Direction changes must be normalized.
   When the user states a durable product truth or changes the current product
   priority, the agent must classify that direction as a readiness assertion,
   release gate, open decision, or active-target update instead of leaving it
   as informal chat.
   Treat casual statements about consistency, seamlessness, drift, bypass
   resistance, or things that "should always be true" as candidate governance
   input, not only explicit "add an assertion" requests.
11. Lane coverage gaps must be normalized.
   When an agent discovers a durable product surface that is materially
   underrepresented by the current lane map, it must record that gap in the
   active profile instead of burying it inside a broad lane, lane followup, or
   assertion.
   Repeated or durable coverage gaps must be promoted into a lane split,
   lane addition, or active-target update once the right owning surface is
   clear.
   When that promotion path is clear enough to name a `candidate_lane`, the
   candidate should also point at the owning control-plane target so future
   lane-expansion work has an explicit destination rather than only prose.
12. Work claims are the audit trail for active governed slices.
   Before coding begins on a governed slice, record a `work_claim` against
   the lane, blocker, coverage gap, candidate lane, or other governed item
   being worked on.
   The claim records what is being worked on and prevents accidental overlap
   if work is resumed or handed off.
   Claims carry expiry so abandoned claims age out instead of freezing the
   map indefinitely; remove expired claims when encountered.
13. Lane work must follow canonical v6 modernization rules.
   Choosing the right lane is not enough by itself. Once a lane or candidate
   lane is selected, the agent must check that surface against the canonical
   v6 architecture rather than only improving the local pre-v6 or legacy path.
   That includes preferring the unified resource model where it is the
   canonical shape, removing or isolating legacy host-era terminology and
   compatibility shims from primary paths, using canonical v6 types and APIs,
   and updating subsystem ownership plus proof routing when runtime ownership
   changes.
   These modernization rules are retrospective as well as forward-looking:
   existing lanes that were previously improved in a legacy-shaped way must be
   re-judged against the canonical v6 model when they are touched again, and
   they must not be treated as complete simply because earlier local work
   landed.
14. Prompt guidance should stay principle-led.
   Prompt-like guidance should express posture, values, and entry-point
   heuristics, while the control plane, active profile, subsystem contracts,
   audits, and guardrails carry the detailed operating rules.
   If an agent still needs repeated procedural prompt wording to behave, the
   canonical system should be strengthened instead of growing the prompt.
15. Governance routing should be quiet by default.
   Resolving the entry bundle, mapping a request into a lane or governed slice,
   and deciding whether a claim or contract update is required are normally
   internal behaviors. Surface those mechanics only when they materially affect
   blockers, scope, cross-repo impact, or the user's next decision.
16. Use worktrees when a mutating slice needs physical isolation.
   Claims and lookup tools reduce logical overlap, but they do not isolate
   hooks, formatters, staged reads, or unrelated dirty state. Use a dedicated
   worktree when a slice requires that isolation — a shared checkout is fine
   for normal single-session work.
   The canonical helper trio is `worktree_base.py` to ensure the clean landing
   worktree exists, `worktree_claim.py` to start isolated work, and
   `worktree_finish.py` to land a finished isolated slice back onto the clean
   base-branch worktree.
17. Lane-expansion targets must steer slice selection.
   When the active target is a lane-expansion or bridge-foundation target and
   `status_audit.py --pretty` exposes an `available_candidate_lane_queue`,
   agents should choose from that queue before local cleanup, bounded
   stabilization residue, or presentation-only polish unless the user
   explicitly overrides the priority or a release-blocking surface needs
   immediate containment.
   Support work is still valid, but it should normally advance the selected
   candidate lane or the active blocker rather than displacing it.
18. Candidate-lane work must be first-class, not prose-only.
   If `status.json.candidate_lanes` and `status.json.coverage_gaps` are the
   real remaining work, the control-plane entrypoint, lookup helpers, and
   current-state text must treat them as primary governed surfaces rather than
   forcing agents to infer them indirectly from broad audit output.
19. Shared-checkout claim flow must be executable.
   If `status.json.work_claims` is the governed overlap boundary, the control
   plane must expose a first-class shared-checkout helper for reserving,
   renewing, and releasing exactly one governed slice instead of leaving that
   path as prose-only guidance.
   Support-only slices still need that claim boundary, and any same-lane
   residual must be normalized before the claim is replaced or released.

## Canonical Files

1. `docs/release-control/AGENT_VALUES.md`
   Stable human values layer for agent behavior.
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
Active targets narrow execution focus; they do not define the entire product
map.
If the active profile discovers product scope that the current lane taxonomy
does not model cleanly, that gap should still be recorded even when it sits
outside the active target floor.
For release targets, `rc_ready` means a governed prerelease build can be
cut, while `release_ready` means stable or GA promotion readiness after
prerelease validation.
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
   It is an intentionally human-held stabilization target with `proof_scope:
   none`, and default slice selection should stay centered on the monitoring
   floor unless the user explicitly overrides it or the governed target
   changes again.
3. `v6-product-lane-expansion` remains planned and is still blocked on the
   broader surfaced product case being proven.
   Its candidate-lane surface remains available in `available_candidate_lane_queue`
   plus the linked `candidate_lanes` and `coverage_gaps`.
4. `v6-ga-promotion` remains planned and is still blocked on exercised
   promotion proof when that target is resumed.
5. `v6-rc-cut` is complete and remains the predecessor release-cut target.
6. Its files remain under `docs/release-control/v6/`.
7. The existing v6 control surfaces are still live, but they now sit underneath
   an evergreen Pulse control plane rather than pretending to be the whole
   long-term system.
8. Until the explicit post-GA branch cutover happens, both prerelease and
   stable v6 promotions resolve to `pulse/v6` via `control_plane.json`.
9. `AGENT_VALUES.md` is the evergreen values-only entry point; prompts should
   stay close to that layer and delegate detailed behavior to the governed
   control-plane and profile-specific files.
10. `python3 scripts/release_control/control_plane.py --agent-entrypoint --pretty`
    is the executable entrypoint for that ordered guidance bundle; agents
    should prefer it over reconstructing the bundle ad hoc.
    That entrypoint should lead with lightweight startup files plus derived
    commands such as `status_audit.py --pretty` and targeted lookup helpers.
    `status_lookup.py` is the id-scoped helper for candidate lanes, coverage
    gaps, lanes, assertions, gates, followups, and work claims.
    `work_claim.py` is the shared-checkout claim helper for reserving or
    releasing exactly one governed slice before mutation.
    `worktree_base.py` ensures the canonical clean landing worktree exists for
    the base branch.
    `worktree_claim.py` is the canonical helper for pairing a mutating claim
    with a dedicated worktree when a slice requires physical isolation.
    `worktree_finish.py` is the canonical helper for landing an isolated
    finished slice back onto the base branch worktree.
    It should not require agents to ingest raw `status.json` or `registry.json`
    in full at startup unless the current task genuinely needs to drill into
    those surfaces.
