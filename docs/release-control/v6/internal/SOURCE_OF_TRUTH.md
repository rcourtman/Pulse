# Pulse v6 Source Of Truth

Last updated: 2026-03-20
Status: ACTIVE

This file is the stable human governance layer for the active v6 release
profile. It is not a live progress dashboard.

Evergreen control-plane governance now lives in
`docs/release-control/CONTROL_PLANE.md`, and the active profile plus active
target are machine-selected by `docs/release-control/control_plane.json`.

Current lane scores, evidence references, coverage-gap discovery records,
candidate-lane planning records, and typed operational decision records live
only in `docs/release-control/v6/internal/status.json`.
Lane completion state, residual-gap summaries, normalized follow-up tracking,
and derived coverage scores also live only in
`docs/release-control/v6/internal/status.json`.
Non-blocking same-lane residuals that are concrete enough to name directly
live in `status.json.lane_followups`.
Product-scope lane-discovery records that reveal where the current taxonomy is
still under-modeled live in `status.json.coverage_gaps`.
Planned future lane promotions for those gaps live in
`status.json.candidate_lanes`.
Active multi-agent lease records that reserve governed work in flight live in
`status.json.work_claims`.
Current repo/release readiness is derived by
`python3 scripts/release_control/status_audit.py --pretty` from
`status.json`, not hand-maintained in this file. The human runbook for
trust-critical manual release gates lives in
`docs/release-control/v6/internal/HIGH_RISK_RELEASE_VERIFICATION_MATRIX.md`.
Use `python3 scripts/release_control/status_lookup.py` for candidate-lane,
coverage-gap, lane, readiness-assertion, release-gate, followup, and
work-claim lookups when only one governed surface is relevant.
Readiness assertion design rules live in this file.
The active assertion catalog, proof references, and executable proof commands
live in `status.json.readiness_assertions`.

## Purpose

1. Define the stable execution model for the active v6 release profile.
2. Lock profile-specific scope, release definition, and no-go rules.
3. Record only long-lived architectural and release decisions for this profile.
4. Point agents to the files that own live state and subsystem truth for this
   profile.
5. Keep startup context light by preferring derived status commands and
   targeted lookup helpers before opening large machine files in full.

This file must not contain:

1. hand-maintained lane score tables
2. hand-maintained evidence lists
3. ephemeral session recaps or task logs

## Canonical Control Files

1. `docs/release-control/AGENT_VALUES.md`
   Evergreen values-only guidance for agent behavior.
2. `docs/release-control/CONTROL_PLANE.md`
   Evergreen governance for the permanent Pulse release control plane.
3. `docs/release-control/control_plane.json`
   Machine-readable control-plane state, including the active profile and
   active target.
4. `docs/release-control/control_plane.schema.json`
   Machine-readable contract for `control_plane.json`.
5. `docs/release-control/v6/internal/status.json`
   Live lane state, lane completion records, coverage-gap discovery records,
   candidate-lane planning records, active work claims, structured evidence
   references, and typed operational decision records.
6. `docs/release-control/v6/status.schema.json`
   Machine-readable contract for the `status.json` shape.
7. `docs/release-control/v6/internal/CANONICAL_DEVELOPMENT_PROTOCOL.md`
   Repo-wide change rules for canonical work.
8. `docs/release-control/v6/internal/subsystems/registry.json`
   Machine-readable subsystem ownership and proof requirements.
9. `docs/release-control/v6/internal/subsystems/registry.schema.json`
   Machine-readable contract for the subsystem registry shape.
10. `docs/release-control/v6/internal/subsystems/*.md`
   Per-subsystem contracts: truth, extension points, forbidden paths, and
   completion obligations.
11. `docs/release-control/v6/internal/RELEASE_PROMOTION_POLICY.md`
   Canonical stable-versus-prerelease promotion rules, rollout criteria, and rollback
   expectations for v6 and later release lines.
12. `docs/release-control/v6/internal/V5_MAINTENANCE_SUPPORT_POLICY.md`
   Canonical v5 maintenance-only support policy, release-line rules, and GA
   notice requirements for the v6 cutover.
13. `docs/release-control/v6/internal/RC_TO_GA_REHEARSAL_TEMPLATE.md`
   Canonical human record shape for the non-publish RC-to-GA rehearsal run.

These machine files remain canonical, but agents should not ingest them in full
by default when a smaller derived command answers the question. Prefer
`status_audit.py --pretty`, `status_lookup.py`, and `subsystem_lookup.py`
first, then escalate into the raw files only when the current slice needs that
detail.

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

Pulse v6 has a bridge-release direction, but the current stabilization target
stays on the monitoring-and-alerting RC floor rather than the finished private
operational broker.
For this profile, "bridge release" means the product should stop reading as a
monitoring product growing sideways only where the surfaced case is already
proven; the next surfaced target is policy-aware data governance, while the
broader resource-centric, action-ready control platform remains a future lane.
Pulse owns infrastructure context, policy, and governed action boundaries so
any sandboxed agent can use them; Pulse does not own being the sandbox where
arbitrary agent execution lives.
The retained foundation is therefore the hidden backend layer: canonical
resources and relationships, standardized agent-emitted resource/signal/change
envelopes, policy metadata and routing hooks, governed action and approval
boundaries with auditability, and first-class fleet governance.

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
7. Not all evidence classes are equal.
   High-risk release gates must declare the minimum evidence tier required for
   closure, and rehearsal evidence below that tier must remain blocking even
   when a dated record already exists.
8. When a user states a durable product truth, normalize it into a readiness
   assertion, release gate, or open decision rather than leaving it as chat.
9. Treat casual user language about consistency, seamlessness, drift, bypass
   resistance, or things that "should always be true" as candidate governance
   input, not only explicit requests to add a formal assertion.
10. Active v6-facing guidance must stay current; legacy or historical docs may
    exist only as clearly marked reference material, not as current instructions.
11. Comparable settings surfaces should be normalized into canonical
    page-shell rules when the user raises consistency drift, rather than
    treated as vague polish notes.
12. Lane taxonomy is allowed to evolve when it no longer reflects the current
    product truth.
13. If a durable product surface is materially underrepresented by the current
    lane map, record an active `status.json.coverage_gaps` entry instead of
    hiding the gap inside a broad lane, a lane followup, or a generic
    assertion.
14. Treat `status.json.coverage_gaps` as active fog-of-war records rather than
    a loose backlog: each one should cite evidence, carry an explicit coverage
    deduction, and name the intended promotion path such as a lane split, new
    lane, lane expansion, or target update.
    `coverage_gaps.status` is not cosmetic: once a lane-shaping gap
    (`new-lane`, `lane-split`, or `lane-expansion`) has a typed
    `candidate_lanes` record, move that gap to `planned`; do not mark such a
    gap `planned` until a matching `candidate_lanes` record exists.
15. When the intended lane split/addition/expansion shape is already clear,
    record it in `status.json.candidate_lanes` so lane-taxonomy evolution is
    machine-readable rather than buried in prose.
    Candidate lanes should also identify the owning control-plane target so
    future lane-expansion work is routed to an explicit governed initiative.
    `candidate_lanes` are only for lane-shaping promotion paths, so
    `target-update` coverage gaps must stay out of that surface.
16. Once a coverage gap is resolved by a lane split/addition, lane expansion,
    or target update, remove it from `coverage_gaps` instead of leaving it
    behind as historical noise.
17. Before coding begins on a governed slice, record a `status.json.work_claims`
    entry for the chosen slice, then verify it with
    `python3 scripts/release_control/agent_preflight.py --require-active-claim --agent-id <AGENT_ID>`.
    The claim is an audit trail: it records what is being worked on and
    prevents accidental overlap if the same surface is resumed or handed off.
    Expired claims no longer mark active work; remove them when encountered so
    the live claim set stays trustworthy.
18. Lane work must be evaluated against the canonical v6 model, not only the
    local state it inherits.
    For any lane or candidate lane, first identify the canonical v6 shape for
    that surface: unified-resource-backed where applicable, canonical shared
    types and APIs, explicit subsystem ownership, and legacy compatibility code
    reduced to boundary-only exceptions instead of primary flow.
    When a lane surface still routes through a pre-v6 or host-era shape, do
    not merely improve that path locally and call the lane healthier.
    Either modernize it toward the canonical v6 mechanism in the same slice or
    record the remaining modernization gap explicitly in the owning lane's
    completion state, follow-up tracking, or lane-shaping governance.
19. These modernization rules are retrospective.
    Existing lanes do not keep their floor forever just because earlier work
    landed before the control system made this explicit.
    When an existing lane is touched, reviewed, or reopened, judge it against
    the same canonical v6 modernization rules as new lane work and lower its
    claimed completeness if legacy-primary paths are still carrying the surface.
    Backward compatibility is not a blanket excuse to keep old internal
    mechanisms alive. Preserve legacy behavior only at explicit boundaries such
    as migrations, upgrades, imports, external contracts, or wire
    interoperability that still genuinely exist; otherwise retire the
    legacy-primary path instead of teaching the lane to depend on it.
20. Guardrail-only work is support work, not lane advancement.
    Contract ratchets, proof-routing cleanup, registry tightening, and
    guardrail-only tests may strengthen confidence, but they must not be
    treated as substantive lane movement by themselves.
    If `status.json` advances a lane's score, status, or completion state, the
    same slice should normally include an owned runtime or product-surface
    delta for that lane unless the remaining gap is explicitly governance-only.
21. Large machine control files are canonical databases, not startup prompt
    payload.
    Treat them as canonical databases, not startup prompt payload.
    Agents should prefer executable summaries and targeted lookup commands over
    rereading `status.json` or `registry.json` in full on ordinary turns.
    Escalate into the raw files only when a lane, assertion, release gate,
    work claim, or subsystem boundary actually requires full-detail inspection.
22. Legacy compatibility must be named as a boundary exception, not assumed as
    a primary v6 design goal.
    If a touched surface still needs old-path support, make the boundary
    obligation explicit in the owning contract or lane residual. Otherwise,
    treat continuing investment in that old internal path as drift, not
    prudence.
23. Claimed lane work should prefer the largest coherent same-surface slice.
    Within an active claimed lane, prefer the largest coherent same-surface slice.
    When one behavior arc on one governed surface is already clearly in scope,
    group the remaining same-surface work into the largest slice that still
    has one coherent proof story.
    Do not fragment a single modernization or canonicalization arc into many
    tiny commits just because each residue item can be named separately. Split
    only when there is a real risk boundary, concept boundary, or proof
    boundary.
24. Lane scores, missing evidence items, and proof counts are signals, not the
    work target.
    Use them to find the remaining runtime, product, ownership, or governance
    gap that still keeps the lane below floor.
    Do not frame a slice as "get two more evidence items" or "raise the lane
    score" unless the remaining gap is explicitly governance-only.
    For ordinary lane work, identify the real same-lane gap first, then add
    the proof that demonstrates the gap is actually closed.
25. Worktrees are available for isolated mutating slices when explicit isolation
    is needed.
    A shared mutable checkout is fine for normal single-session work.
    Use `worktree_base.py`, `worktree_claim.py`, and `worktree_finish.py` when
    a slice requires its own isolated hooks, staged scope, and dirty state —
    for example when a subagent needs to mutate independently and land back
    cleanly.

## Non-Negotiable Release Gates

1. Do not ship with open trust-critical P0s.
2. Do not ship when paywalls and runtime gates disagree.
3. Do not ship hosted Pulse flows that break signup, auth, provision, hosted
   runtime access, billing/admin visibility, or revocation.
4. Do not ship multi-tenant support if tenant isolation, organization scope,
   tenant-scoped runtime state, or cross-org sharing can leak outside the
   intended tenant boundary.
5. Do not ship MSP support if one provider account cannot safely onboard,
   manage, and separate multiple client tenants from one control surface.
6. Do not ship if API tokens can exceed assigned user, org, or scope
   boundaries, survive revocation, or silently widen authority through legacy
   alias handling.
7. Do not ship if a low-privilege user can view, mutate, or destroy beyond the
   permissions granted by their effective org membership and role.
8. Do not ship if grandfathered recurring continuity, cancellation revocation,
   and later re-entry pricing can drift across Stripe, Pulse runtime, and
   customer-visible billing state.
9. Do not ship if Pulse Mobile can keep stale access, lose approval state, or
   fail pairing, reconnect, or auth-transition recovery against a real
   instance.
10. Do not ship if relay registration, reconnect, stale-session recovery, or
    disconnect drain can strand clients in resume loops, dead sessions, or
    lost inflight work.
11. Do not ship upgrades that reset paid state, licensing continuity, or
    first-session flow.
12. Do not ship if comparable settings surfaces drift away from the canonical
    page-shell contract and still present inconsistent top-level framing,
    header chrome, or section treatment without explicit justification.
13. Do not keep polishing strong lanes while weak lanes remain behind.
14. Do not treat `status.json` lane scores reaching target as sufficient release approval by themselves; open operational decisions, machine-derived unresolved readiness assertions, and unresolved release gates still apply.
15. Do not promote v6 to stable or GA without an exercised prerelease, a real
    release-pipeline proof run, a recorded rollback target plus exact
    reinstall command, and a written v5 maintenance-only support policy.
16. Do not declare a lane or blocker complete just because the first passing
    check landed. If obvious same-lane gaps still keep the outcome from being
    coherent, trustworthy, or realistically shippable, that lane is not done
    yet.
17. Lanes that are at target but intentionally not closed must record a
    bounded residual in `status.json` with a short rationale and explicit
    tracking references to the governing follow-up surface.
    Pair lane `status` and `completion.state` coherently: `complete` goes with
    `target-met`, `bounded-residual` goes with `partial`, and `open` must not
    be paired with `target-met`.
    `partial` means measurable progress, so it must not sit at zero score.
    `not-started` means zero score and `open`. `blocked` remains an `open`
    lane below target rather than a residual or complete state.
    Blocked lanes must declare typed same-lane blocker references to unresolved
    readiness assertions, release gates, or open decisions.
    `completion.tracking` is only for bounded residuals; open or complete lanes
    should keep that list empty until the lane reaches its current floor and
    the remaining work becomes a governed residual.
    Those references must belong to that same lane and must still be
    unresolved rather than pointing at unrelated governance objects or
    already-passed assertions, gates, or completed targets.
    Bounded residual tracking must use a lane followup, readiness assertion,
    release gate, or open decision rather than a broad target reference.
    `lane_followups` are active residual records, not a loose backlog: each one
    should stay referenced by the owning lane's `bounded-residual`
    `completion.tracking`.
    Once a lane followup is no longer active residual work, remove it from
    `lane_followups` instead of leaving it behind with a completed status.
18. Do not treat a lane as healthy just because it has local fixes on a legacy path.
    If the canonical v6 route for that surface now runs through unified
    resources, canonical shared types, explicit subsystem ownership, or a
    modernized proof path, the lane is still below floor until that migration
    is made or the remaining gap is governed explicitly.
19. Do not treat missing evidence count or a score delta as the task itself.
    If the real remaining gap is runtime, product, ownership, or behavior
    coherence, fix that gap and let the score/evidence catch up as proof.
    Only treat evidence-count closure itself as the task when the remaining
    gap is explicitly governance-only.
20. Do not run parallel mutating agents out of one dirty checkout and call
    that acceptable coordination.
    Claims reduce overlap, but they do not isolate hooks, formatters, staged
    reads, or unrelated dirt. Parallel mutation should use separate worktrees
    so each agent sees one slice's git state at a time.

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
   blocked until the RC-to-GA promotion gate is cleared and the published v5
   maintenance-policy notice is ready.
7. v6 and later releases use a promotion model, not a direct broad-rollout
   model.
   `stable` must receive only promoted, already-validated builds, `rc` is the
   opt-in preview channel, and unattended auto-update exposure remains
   `stable`-only unless a new channel policy is explicitly adopted.
8. Once v6 reaches stable or GA, v5 enters a 90-day maintenance-only window:
   critical security issues, critical correctness/data-loss issues, and safe
   migration blockers only. The exact end-of-support date must be published in
   the GA release notice. After that window, v5 is unsupported.
9. Paid Pulse Pro v5 customers keep their existing recurring price through the
   v6 pricing change until they cancel. Renewal and entitlement continuity
   must preserve that grandfathered price state; any return after cancellation
   must use current v6 pricing.
10. Pulse Mobile does not need desktop parity to stop blocking the v6 RC line.
    The mobile usefulness floor for RC is narrower: preserve at least one
    trusted paired instance across relaunches, expose relay/runtime state
    clearly in the main shell, fail closed into a recoverable disconnected
    state on stale or revoked access, and keep live approvals useful and
    recoverable. Broader parity and expansion remain post-RC scope.
11. The minimum required update set for canonical work is a floor, not a lane
    closure rule. Agents should push the current lane to a coherent,
    defensible stop point and complete the next obvious same-lane work when it
    is necessary for trustworthy results, then normalize any remaining valid
    gap instead of calling the lane done by default.
12. Unified-resource change history is the canonical durable backend timeline.
    Alert incident memory may retain investigation-local notes, analysis,
    commands, runbooks, and alert lifecycle breadcrumbs, but it must remain a
    derived incident projection rather than a competing source of truth for
    durable resource history.

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
   `docs/release-control/AGENT_VALUES.md`,
   `docs/release-control/control_plane.json`, this file, and
   `docs/release-control/v6/internal/status.json` first.
   Use `python3 scripts/release_control/control_plane.py --agent-entrypoint --pretty`
   when you need the canonical ordered entry bundle instead of reconstructing
   it manually.
   If the agent starts outside the `pulse` repo, resolve those files from
   `pulse/docs/release-control/v6/` under the shared workspace root rather than
   inventing a parallel control layer in the current repo.
2. Then read `docs/release-control/v6/internal/CANONICAL_DEVELOPMENT_PROTOCOL.md`.
3. Then read `docs/release-control/v6/internal/subsystems/registry.json`.
4. Then read the relevant subsystem contract under
   `docs/release-control/v6/internal/subsystems/`.
5. Update `status.json` when live lane state, readiness derivation rules,
   lane completion records, coverage-gap records, candidate-lane planning
   records, assertion proof routes, evidence references, or typed operational
   decision records change.
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
11. Do not stop at the first narrow success inside a lane.
    When the next obvious same-lane work is still required for a coherent and
    trustworthy result, complete it in the same slice when feasible.
12. Keep that standard bounded.
    If the remaining work clearly belongs to another lane, another active
    target, a larger redesign, or a separately governed open decision, record
    that state explicitly instead of expanding the current slice without end.
13. If the current lane map no longer models a durable product surface cleanly,
    record that discovery in `status.json.coverage_gaps` with evidence and an
    intended promotion path instead of burying it in generic residual text.
14. Treat lane completion, lane coverage, and coverage planning as separate
    derived signals.
    `status_audit.py --pretty` may derive a lane-completion score from
    `current_score/target_score`, a lane-coverage score from unresolved
    `coverage_gaps`, a coverage-planning score from unresolved
    `coverage_gaps` that still lack explicit `candidate_lanes`, and a
    conservative governed-surface score as the lower of completion and
    coverage.
    Those scores are decision aids, not proof that unknown work does not
    exist.
15. Treat active work claiming as its own live control surface.
    `status.json.work_claims` should hold only live lease-style claims, not a
    historical log, and `status_audit.py --pretty` should make active claims,
    expired claims, claim conflicts, and the `available_candidate_lane_queue`
    visible enough that agents can avoid overlapping picks.
    Use `scripts/release_control/work_claim.py` as the default reservation
    path rather than hand-editing claim objects when the system can reserve
    the slice directly.
16. Keep prompt-like guidance values-led.
    If a rule needs detailed repeated reminder, prefer strengthening the
    control plane, active profile, subsystem contracts, audits, or guardrails
    over expanding prompt prose into a second operating manual.
17. Keep governance routing quiet by default.
    Resolving the canonical entry bundle, choosing the owning lane, and
    deciding whether a claim or governance update is required should usually
    happen in the background. Surface those mechanics only when they
    materially change blockers, scope, cross-repo impact, or the user's next
    decision.

For readiness assertion work:

1. Update this file only when the readiness-assertion design rules change.
2. Update `status.json.readiness_assertions` when the active assertion catalog
   or proof references/proof commands change.
3. Route manual assertion proof through
   `HIGH_RISK_RELEASE_VERIFICATION_MATRIX.md` whenever the assertion needs a
   trust-critical release gate instead of a one-off checklist.

## Source Domains

If conflicts appear, resolve by domain:

1. `docs/release-control/v6/internal/CANONICAL_DEVELOPMENT_PROTOCOL.md`,
   `docs/release-control/v6/internal/subsystems/registry.json`, and the relevant
   subsystem contract own implementation rules.
2. `docs/release-control/v6/internal/status.json` owns live lane state, the active
   readiness assertion catalog, readiness derivation rules, executable proof
   commands, coverage-gap discovery records, candidate-lane planning records,
   structured evidence references, and typed operational decision records.
3. `docs/release-control/v6/status.schema.json` owns the machine-readable shape
   contract for `status.json`.
4. `docs/release-control/v6/internal/subsystems/registry.schema.json` owns the
   machine-readable shape contract for the subsystem registry.
5. `docs/release-control/AGENT_VALUES.md` owns evergreen agent values, while
   `docs/release-control/CONTROL_PLANE.md` and
   `docs/release-control/control_plane.json` own evergreen governance, active
   profile selection, and active target selection.
6. This file owns profile-specific governance, repo scope, release gates,
   readiness assertion design rules, and locked decisions for v6.
7. Supporting architecture and release docs are evidence only. They do not
   override the files above.
