# Pulse v6 Canonical Development Protocol

This is the canonical development protocol for the active v6 release profile.
The evergreen Pulse control plane now lives in `docs/release-control/`.

This protocol exists to make the repo constrained instead of interpretive.
Future work should be difficult to do in the wrong shape.

## Core Rule

Each important concern in Pulse v6 must have:

1. one canonical truth
2. one obvious extension path
3. explicit forbidden paths
4. completion obligations when that truth changes
5. at least one executable guardrail where practical

If a subsystem does not satisfy those five properties, it is drift-prone.

## V6 Modernization Invariants

Every governed v6 slice inherits these rules, including lanes that already
exist:

1. identify the canonical v6 model for the surface before changing it
2. route through unified resources when that is the canonical shape
3. prefer canonical shared types, APIs, and paths over lane-local variants
4. remove or isolate legacy host-era terminology, adapters, and compatibility
   shims from primary runtime paths
5. update subsystem ownership, path policies, and proof routing when runtime
   ownership moves
6. record any unavoidable modernization residual explicitly instead of treating
   local improvement on a legacy path as lane completion

These rules are retrospective.
When an existing lane is touched, reviewed, or reopened, judge it against the
same modernization bar as new lane work rather than assuming earlier work
closed it permanently.

## Required Operating Files

For v6 work, agents must treat these startup files as the execution entry
point:

1. `docs/release-control/AGENT_VALUES.md`
2. `docs/release-control/CONTROL_PLANE.md`
3. `docs/release-control/control_plane.json`
4. `docs/release-control/v6/internal/SOURCE_OF_TRUTH.md`
5. `docs/release-control/v6/internal/CANONICAL_DEVELOPMENT_PROTOCOL.md`

Use derived startup commands before escalating into the heavier machine control
files:

1. `python3 scripts/release_control/control_plane.py --agent-entrypoint --pretty`
2. `python3 scripts/release_control/agent_preflight.py --pretty`
3. `python3 scripts/release_control/status_audit.py --pretty`
4. `python3 scripts/release_control/status_lookup.py --lane <LANE_ID> --pretty`
5. `python3 scripts/release_control/status_lookup.py --assertion <ASSERTION_ID> --pretty`
6. `python3 scripts/release_control/status_lookup.py --release-gate <GATE_ID> --pretty`
7. `python3 scripts/release_control/status_lookup.py --followup <FOLLOWUP_ID> --pretty`
8. `python3 scripts/release_control/status_lookup.py --work-claim <CLAIM_ID> --pretty`
9. `python3 scripts/release_control/work_claim.py --kind ... --id ... --summary ... --agent-id ... --pretty`
10. `python3 scripts/release_control/subsystem_lookup.py <path> [<path> ...] --pretty`

Escalate into these raw machine files only when the current task genuinely
needs their full detail:

1. `docs/release-control/v6/internal/status.json`
2. `docs/release-control/v6/status.schema.json`
3. `docs/release-control/v6/internal/subsystems/registry.json`
4. `docs/release-control/v6/internal/subsystems/registry.schema.json`
5. the relevant subsystem contract in `docs/release-control/v6/internal/subsystems/`

The startup files answer values, active target, profile selection, release
priority, and stable operating rules.
`AGENT_VALUES.md` is the values-only layer: it should stay small and
principle-led, and prompt-like guidance should stay close to that layer rather
than restating the detailed operating manual.
`python3 scripts/release_control/control_plane.py --agent-entrypoint --pretty`
is the executable way to print this ordered guidance bundle when discovery or
repo entrypoint is not obvious.
`python3 scripts/release_control/agent_preflight.py --pretty` is the first
machine check after the control-plane bundle; it keeps the branch and target
check visible before any mutation.
After reserving a governed claim, rerun
`python3 scripts/release_control/agent_preflight.py --require-active-claim --agent-id <AGENT_ID>`
to verify that the claim is actually persisted before editing.
That bundle should stay command-first: use `status_audit.py --pretty` and the
`status_lookup.py` selectors to answer current-state questions before opening
raw `status.json` or `registry.json`.
`python3 scripts/release_control/work_claim.py --kind ... --id ... --summary ... --agent-id ... --write`
is the executable way to reserve a governed work claim once the slice is
chosen.
In normal user-facing work, that resolution should stay quiet unless it
materially changes the plan, the scope boundary, or the blocker story.
`CONTROL_PLANE.md` and `control_plane.json` own the evergreen governance model
and the active target.
`scripts/release_control/control_plane_audit.py --check` is the stale-target
guard for that evergreen layer.
`SOURCE_OF_TRUTH.md` owns profile-specific governance, scope, locked decisions,
and the readiness-assertion design rules.
`status.json` owns live lane state, lane-to-subsystem ownership, lane
completion records, the active readiness assertion catalog, readiness
derivation rules, executable proof commands, structured evidence references,
typed lane/subsystem decision records, active work claims, and the canonical machine-readable
workspace repo catalog.
Lane scores reaching target in `status.json` mean the tracked architecture and
governance work for those lanes is at target; they are not, by themselves,
approval to cut an RC or promote stable/GA if phase-blocking readiness
assertions, `open_decisions`, or `release_gates` still remain.
When the agent starts from a sibling repo or the shared workspace root, these
same files are still authoritative; resolve them from `pulse/docs/release-control/v6/`
rather than recreating release control in the current repo.
Use `python3 scripts/release_control/status_audit.py --pretty` for the current
machine-derived repo/governance, `rc_ready`, and `release_ready` summary. Use
`docs/release-control/v6/internal/HIGH_RISK_RELEASE_VERIFICATION_MATRIX.md` as the
human runbook for clearing the manual high-risk gates represented in
`status.json.release_gates`.
Use `python3 scripts/release_control/status_lookup.py` for id-scoped lane,
assertion, release-gate, followup, and work-claim views instead of printing
the whole status audit when only one governed surface matters.
`status.schema.json` owns the machine-readable status contract.
`subsystems/registry.schema.json` owns the machine-readable subsystem registry
contract, including explicit shared-runtime ownership declarations for
intentionally multi-owner files.
The protocol, subsystem registry, and subsystem contracts answer how work must
be done.
Readiness assertions are not a parallel checklist. They are the release truths
written on top of those governed subsystem surfaces; `status.json.readiness_assertions`
declares the active assertion catalog, proof references, and executable proof
commands, while this protocol and `SOURCE_OF_TRUTH.md` define how that layer
must behave.
If the agent still needs repeated detailed prompt instruction to follow this
system, that is a governance-system gap to fix here, not a reason to keep
growing prompt prose.
Likewise, if the agent keeps narrating setup or governance plumbing instead of
using it quietly, that is a behavior-layer gap to fix here rather than a cue
to accept process-diary output as normal.

`scripts/release_control/status_audit.py --check` is the machine audit entry
point for validating live lane evidence references, readiness assertion proof
linkage, typed decision records, and derived evidence health. It also enforces
canonical list ordering inside `status.json` so repo scope, lanes, readiness
assertions, evidence references, and decision timelines do not drift into
noisy, hand-arranged variants.
`scripts/release_control/registry_audit.py --check` is the machine audit entry
point for validating subsystem ownership, proof routing, registry lane
bindings, canonical ordering for unordered registry lists, and path-policy
reachability under first-match precedence.
`scripts/release_control/contract_audit.py --check` is the machine audit entry
point for validating structured contract metadata, section presence/order,
registry/status linkage, explicit cross-subsystem dependency declarations, and
canonical path references and shared-boundary declarations inside subsystem
contracts.

## Subsystem Contracts

Each major subsystem contract must define:

1. `Contract Metadata`
2. `Purpose`
3. `Canonical Files`
4. `Shared Boundaries`
5. `Extension Points`
6. `Forbidden Paths`
7. `Completion Obligations`
8. `Current State`

Current required subsystem contracts:

1. `docs/release-control/v6/internal/subsystems/alerts.md`
2. `docs/release-control/v6/internal/subsystems/api-contracts.md`
3. `docs/release-control/v6/internal/subsystems/cloud-paid.md`
4. `docs/release-control/v6/internal/subsystems/frontend-primitives.md`
5. `docs/release-control/v6/internal/subsystems/monitoring.md`
6. `docs/release-control/v6/internal/subsystems/organization-settings.md`
7. `docs/release-control/v6/internal/subsystems/patrol-intelligence.md`
8. `docs/release-control/v6/internal/subsystems/performance-and-scalability.md`
9. `docs/release-control/v6/internal/subsystems/security-privacy.md`
10. `docs/release-control/v6/internal/subsystems/unified-resources.md`

If a major subsystem is refactored and does not have one of these files, the
work is incomplete.

The machine-readable ownership map for those subsystem contracts lives in
`docs/release-control/v6/internal/subsystems/registry.json`.
Each contract must also carry structured metadata that binds the markdown file
to its registry subsystem id, owning lane in `status.json`, and exact declared
cross-subsystem dependencies implied by its canonical-file and extension-point
references.
If a runtime file is intentionally owned by multiple subsystems, that overlap
must be declared explicitly in `registry.json` and mirrored in the contract's
`Shared Boundaries` section; accidental overlap is not an allowed registry
state. Shared-boundary entries must use the exact registry-derived sentence
shape rather than freeform wording.

## Task Completion Protocol

Every substantial task must finish by checking these questions:

1. Did I change a canonical contract?
   If yes: update the relevant subsystem contract.
2. Did I change live lane state, lane completion records, coverage-gap records, readiness derivation rules, readiness assertion proof routes, evidence references, or typed operational decision records?
   If yes: update `status.json`.
   This includes `lane_followups` when a bounded residual becomes concrete
   enough to name directly without making it a blocker.
   It also includes `coverage_gaps` when the current lane map no longer
   models a durable product surface cleanly enough.
   It also includes `candidate_lanes` when the intended lane split, lane
   addition, or lane expansion shape is clear enough to name explicitly.
3. Did I change stable governance, scope, evergreen readiness assertions, or lock a new architectural/release decision?
   If yes: update `SOURCE_OF_TRUTH.md`.
4. Did I replace an old path with a new canonical path?
   If yes: add or tighten a guardrail so the old path cannot silently return.
5. Did I add a new extension point?
   If yes: record exactly where future work must attach to it.
6. Did I leave compatibility code in runtime paths?
   If yes: either remove it now or explicitly classify it as a boundary-only exception.
7. Did the user state a durable product truth or change the current priority?
   If yes: classify it as a readiness assertion, release gate, open decision,
   or active-target update and record it in the owning control file instead of
   leaving it as chat.
   Treat casual phrases about consistency, seamlessness, drift, bypass
   resistance, or things that "should always be true" as candidate governance
   input, not only explicit requests to update the control system.
8. Did the user casually raise a new quality bar or product invariant?
   If yes: before ending the task, either normalize it into the control plane
   or explain clearly why existing governance already covers it.
8a. Am I about to narrate canonical setup that the user does not need?
    If the information does not materially change the next action, blocker
    state, or scope boundary, keep it internal and continue the work.
9. Did I learn that the current lane map is hiding a durable product surface?
   If yes: record a `coverage_gaps` entry with evidence, an explicit coverage
   impact deduction, and the intended promotion path instead of burying the
   discovery inside generic residual text.
   When the intended destination is already clear, also add or update the
   matching `candidate_lanes` record so the lane-expansion plan is typed,
   machine-readable, and pointed at the owning control-plane target.
   Once a lane-shaping gap (`new-lane`, `lane-split`, or `lane-expansion`)
   has a typed `candidate_lanes` record, move that `coverage_gaps` entry to
   `planned`; do not mark such a gap `planned` before the matching
   `candidate_lanes` record exists.
   `target-update` coverage gaps must stay out of `candidate_lanes`.
10. Am I stopping because the lane is actually coherent, or only because the
   first passing check landed?
   If the result still has obvious same-lane gaps that make it feel partial,
   fragile, or not realistically shippable, continue the slice or record the
   remaining blocker/open decision before stopping.
10b. Am I iterating on a baseline that is not good enough?
    If a customer-facing surface is still prototype-grade, confusing, or
    obviously below the intended product bar, do not keep refining the same
    shape. Step back to the owning product model, IA, trust boundary, or
    architecture first.
10a. Am I treating support work like product progress?
    Guardrail-only proof routing, contract wording, registry cleanup, and
    audit/test ratchets can support a lane, but they do not count as
    substantive lane movement by themselves.
    If the lane state is being advanced in `status.json`, the same slice
    should normally also change an owned runtime or product surface for that
    lane unless the remaining lane gap is explicitly governance-only.
10aa. Am I treating score or evidence deltas as the task instead of as
    diagnostics?
    Missing evidence items, current-vs-target score, and proof counts are
    signals that help locate the real same-lane gap.
10ab. Have I checked real browser truth for a customer-facing surface?
    For important customer-facing UI, code-level proof is not enough by
    itself. If the surface is meant to be trusted or used normally, exercise
    it in-browser and judge whether it actually deserves more iteration in its
    current shape.
    They are not the work item by themselves unless the remaining gap is
    explicitly governance-only.
    For ordinary lane work, identify the runtime, product, ownership, or
    behavior-coherence gap first, then prove that gap is closed.
10ab. Did I check this lane against the canonical v6 model rather than only the
    local path it already had?
    Confirm whether the surface should now run through unified resources,
    canonical shared types and APIs, explicit subsystem ownership, or retired
    legacy terminology and shims.
    If the lane still depends on a pre-v6-primary path, do not stop at local
    cleanup; modernize it or record the remaining modernization gap explicitly.
10ac. Am I preserving legacy internals when only boundary compatibility is
    justified?
    Legacy support is allowed only for explicit boundaries such as migrations,
    upgrades, imports, external contracts, or wire compatibility that still
    genuinely exist.
    If I am improving an old internal path just to keep it alive alongside the
    canonical v6 path, that is drift. Retire it or record the exception
    explicitly as boundary-only compatibility rather than treating it as normal
    lane progress.
11. Is the remaining work still part of this lane?
    If not, stop cleanly and normalize it into the next target, another lane,
    or an explicit open decision instead of letting this task expand forever.
12. If I am recording a bounded residual, do the tracking references point to
    the same lane and stay genuinely unresolved?
    Pair lane `status` and `completion.state` coherently: `complete` goes with
    `target-met`, `bounded-residual` goes with `partial`, and `open` must not
    be paired with `target-met`.
    `partial` means measurable progress, so it must not sit at zero score.
    `not-started` means zero score and `open`. `blocked` remains an `open`
    lane below target rather than a residual or complete state.
    Blocked lanes must declare typed same-lane blocker references to unresolved
    readiness assertions, release gates, or open decisions.
    `completion.tracking` is only for bounded residuals; if the lane is still
    `open` or already `complete`, leave that list empty until the lane reaches
    its current floor and the remaining work becomes a governed residual.
    Do not keep a lane in `bounded-residual` by pointing at already-passed
    assertions, cleared release gates, or completed targets.
    Bounded residual tracking must use a lane followup, readiness assertion,
    release gate, or open decision rather than a broad target reference.
    Treat `lane_followups` as active residual records rather than loose backlog
    notes: each one should stay referenced by the owning lane's
    `bounded-residual` `completion.tracking`.
    Once a lane followup is no longer active residual work, remove it from
    `lane_followups` instead of leaving it behind with a completed status.

This is the minimum update set for canonical work:

1. implementation
2. verification
3. contract update
4. guardrail/ratchet update when an old path is retired

This minimum update set is a floor.
It does not mean the task should stop at the first minimally valid patch when
the next obvious same-lane work is still required for a coherent outcome.
When a lane reaches its current target but still has intentionally bounded
residual work, the same slice must record that residual in
`status.json.lanes[*].completion` with a short rationale and explicit tracking
references to the governing lane followup, readiness assertion, release gate,
or open decision that owns the follow-up.
When feature work fits an existing lane, reopen that lane's completion state or
score instead of pretending the map is still complete.
Existing lanes are not grandfathered out of the modernization rules.
If a previously touched lane still relies on a non-canonical v6 path, lower
its claimed completeness and continue from the canonical model rather than
treating the earlier local improvement as sufficient.
Do not preserve legacy-primary internal flows under the label of
compatibility. Legacy support belongs at explicit boundaries only; when the
primary internal mechanism is still old-path-first, the lane remains
incomplete unless that exception is explicitly governed.
Within an active claimed lane, prefer the largest coherent same-surface slice
that still has one proof story.
Do not break one modernization or canonicalization arc into many tiny residue
commits unless there is a real risk boundary, concept boundary, or proof
boundary that makes the split necessary.
When feature work does not fit any existing lane cleanly, record a
`coverage_gaps` entry so the lane map itself can evolve.
When the durable future lane shape is clear, express that promotion path in
`status.json.candidate_lanes` instead of leaving it only in prose.
Before starting governed work in a multi-agent session, record a lease-style
`status.json.work_claims` entry for the governed item you are taking and do
not pick an item that already has an active claim.
Once the slice is chosen, record that claim immediately before deeper
investigation, targeted test execution, or code changes on that slice.
Prefer `scripts/release_control/work_claim.py` for that reservation flow
rather than hand-editing `status.json`.
Support-only slices are not exempt from this rule: claim the owning lane or
the narrower governed item they unblock, and say in the claim summary why the
plumbing is necessary for that governed surface.
When the slice is mutating and another mutating agent is already active,
prefer `scripts/release_control/worktree_base.py` first so the canonical clean
landing worktree for the base branch exists,
then prefer `scripts/release_control/worktree_claim.py` so the claim is reserved
and the branch/worktree are isolated in one step.
When an isolated mutating slice is finished and the base branch worktree is
clean, prefer `scripts/release_control/worktree_finish.py` to land the scoped
commit(s) back onto the base branch worktree instead of leaving manual merge
cleanup for later.
If you are continuing straight from one governed slice to another, replace the old claim with the new claim in the same `status.json` edit so the live claim
set never passes through a temporary unclaimed window.
If you are stopping without finishing the lane, record the remaining same-lane
residual in `status.json.lanes[*].completion`, `status.json.lane_followups`,
`status.json.coverage_gaps`, or another owning governed surface before you replace or release the claim.
Treat a broad lane claim as blocking narrower same-lane blocker or follow-up
claims until that broad claim is released or expires.
Treat lane completion, lane coverage, and coverage planning as separate
derived signals.
`status_audit.py --pretty` may derive a lane-completion score from
`current_score/target_score`, a lane-coverage score from unresolved
`coverage_gaps`, a coverage-planning score from unresolved `coverage_gaps`
that still lack explicit `candidate_lanes`, and a conservative
governed-surface score as the lower of completion and coverage.
Treat those scores and any missing evidence counts as diagnostic summaries,
not as the task itself.
Use them to choose the real remaining gap on the owned surface rather than
optimizing directly for score movement or evidence-count closure unless the
remaining work is explicitly governance-only.
When lane-expansion work is the intended target, that same audit output may
also derive a ranked `candidate_lane_queue` so agents can take the highest
impact valid candidate lane first.
That same audit output may also derive `available_candidate_lane_queue`,
active-versus-expired `work_claims`, and claim-conflict records so agents have
an executable "do not overlap" surface instead of only prose.
Those claim surfaces reduce logical overlap.
For lane followups, readiness assertions, release gates, and open decisions,
those tracking references must also reference the same lane; unrelated
governance objects are not valid residual placeholders.

This protocol is enforced locally at commit time by the canonical completion
guard in `scripts/release_control/canonical_completion_guard.py` and by the
governance guardrail tests in `internal/repoctl`.
Local pre-commit governance checks must evaluate staged v6 control-file content
rather than unstaged working-tree noise.
Executable readiness assertion proofs must also follow the active target phase
from `docs/release-control/control_plane.json`; repo-ready proof runs alone are
not sufficient once the active target advances to an RC or GA promotion phase.
Local pre-commit must also reject any unstaged working-tree edits for
hook-sensitive governance files under `docs/release-control/v6/`,
`scripts/release_control/`, `internal/repoctl/`, `.husky/pre-commit`, and
`.github/workflows/canonical-governance.yml`, because those local checks still
execute or structurally read the working-tree versions. Partial staging is one
instance of this broader prohibition.
Local formatter steps must also stay scoped to staged files so the hook does not
mutate unrelated dirty worktree state.
When formatting staged Go files, the formatter must operate on staged blobs in
the git index rather than rewriting the whole repo or restaging whole files.

For runtime subsystem changes, the same commit must now include:

1. the matching subsystem contract update
2. any dependent subsystem contract whose `Canonical Files`, `Shared Boundaries`, or `Extension Points` explicitly reference a touched runtime path
3. at least one matching verification artifact update

A staged contract file only counts when the staged diff changes a substantive
contract section such as `Purpose`, `Canonical Files`, `Shared Boundaries`, `Extension Points`,
`Forbidden Paths`, `Completion Obligations`, or `Current State`. Metadata-only
edits are not sufficient completion proof for runtime changes.

Verification artifacts are subsystem-specific. The allowed proof classes are
defined in `docs/release-control/v6/internal/subsystems/registry.json` and may include
explicit guardrail files, contract tests, benchmark/SLO/query-plan artifacts,
approved test-prefix matches, non-test contract/type files, or same-subsystem
tests only when the registry explicitly allows them.

Cross-subsystem contract dependencies are not advisory. If a touched runtime
path is named in another subsystem contract's `Canonical Files`,
`Shared Boundaries`, or `Extension Points`, the canonical completion guard now requires that dependent
contract to be staged in the same slice.

`status.json` evidence references must use repo-qualified relative paths.
Absolute machine-local paths are forbidden.

When a subsystem defines ordered `path_policies`, each touched runtime file must
satisfy the first matching proof policy for that file. The v6 registry requires
explicit path-policy coverage for every governed subsystem, and default
subsystem verification is no longer a supported governed path. New owned
runtime files must therefore be added to a concrete proof route instead of
inheriting subsystem-default verification.

## Guardrails

Canonical architecture is not considered real until the repo can enforce it.

Preferred guardrail types:

1. code standards tests that ban deprecated paths
2. contract tests that lock payload/wire shapes
3. query-plan and performance tests for hot paths
4. parity tests during migrations
5. drift tests for generated or embedded artifacts

Documentation alone is not sufficient when a rule can be made executable.

The canonical completion guard is the default repo-level enforcement point for
subsystem contract updates and proof-of-change verification updates.
That enforcement must run both in local hooks and in CI; bypassable local-only
governance is not sufficient.

## How To Extend Pulse

Before adding new behavior, the author must answer:

1. Which subsystem contract owns this change?
2. Which canonical files are allowed to carry the truth?
3. Which extension point should be used?
4. Which paths are forbidden for this kind of change?
5. Which tests or guardrails must change with it?

If those answers are not obvious in under a minute, the subsystem still needs
architectural hardening.

Use `python3 scripts/release_control/subsystem_lookup.py <path> [<path> ...]`
to ask the repo which subsystem, contract, proof route, live lane context,
relevant decision records, and dependent contract-update obligations apply to a
file set before editing.

## Boundary Rule

Compatibility is allowed only at explicit boundaries:

1. migration loaders
2. persisted upgrade readers
3. temporary API translation layers with a removal plan

Compatibility is not allowed to become the runtime source of truth.

## Expected End State

Pulse v6 should be:

1. canonical by default
2. extension-oriented instead of patch-oriented
3. enforced by tests and guardrails
4. obvious for both humans and agents to continue correctly
