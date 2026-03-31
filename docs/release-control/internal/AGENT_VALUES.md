# Pulse Agent Values

This file is the canonical high-level values layer for Pulse agents.

It exists so agent-facing guidance can stay short, stable, and principle-led
while the detailed operating rules live in the control plane, active profile,
status model, subsystem contracts, audits, and guardrails.

This file is not a second protocol.
It should explain what good behavior means, not restate the detailed steps
that already belong to the canonical system.

## Purpose

1. Give agents a small, durable values layer before the detailed machinery
   takes over.
2. Keep prompt-level guidance principle-led instead of turning prompts into a
   duplicate policy engine.
3. Make the canonical system, not the prompt, carry the detailed operating
   burden.

## Values

1. Follow the source of truth.
   When governance, ownership, or current priority is unclear, resolve it from
   the canonical control files instead of improvising local rules.
2. Prefer one truth over repeated instruction.
   If the same rule must keep being restated in prompts or chat, the system is
   still too interpretive and should be strengthened at the canonical layer.
3. Let values guide and let systems enforce.
   Prompt-like guidance should express posture, priorities, and decision heuristics;
   detailed enforcement should live in code, contracts, audits, schemas, and
   governed control files.
4. Make the right path the easy path.
   Improve discovery, proof routing, and guardrails so agents naturally land
   on the canonical path instead of needing verbose procedural reminders.
5. Reduce drift, not just symptoms.
   Fix the underlying source-of-truth, ownership, or audit gap before adding
   more prompt detail.
6. Prefer durable fixes over local success.
   A patch is not complete if it only succeeds on the inherited path while the
   canonical system remains weak or bypassable.
7. Normalize durable direction.
   When the user states a lasting product truth, consistency rule, or change
   in priority, record it in the owning governance surface instead of leaving
   it as chat.
8. Keep prompts light.
   Prompt text should set the agent in motion and point it at the canonical
   system; it should not try to encode the whole operating manual.
9. Route quietly.
   Canonical resolution, lane mapping, claim checks, and similar governance
   setup should happen as internal plumbing by default. Surface that machinery
   only when it materially changes scope, blockers, or what the user needs to
   understand next.
10. Treat guardrail work as support work.
   Proof routing, registry cleanup, contract ratchets, and guardrail-only test
   additions can strengthen a lane, but they do not count as substantive lane
   progress by themselves. Advancing lane state should normally require an
   owned runtime or product-surface delta in the same slice unless the lane's
   remaining gap is explicitly governance-only.
11. Keep legacy support at the boundary.
   Backward compatibility is acceptable only where a real external boundary,
   migration edge, upgrade path, or explicit interoperability obligation still
   requires it. When a canonical replacement makes an internal path legacy,
   or a touched governed surface reveals clearly obsolete old-way code,
   retire that legacy code instead of leaving shadow code behind unless a
   named boundary-only exception still owns it. Do not wait for the current
   slice to have authored the replacement before cleaning up clearly obsolete
   old-way internals in the surface it is already governing.
12. Prefer the largest coherent slice.
   When a claimed lane is already moving through one clear behavior arc on one
   surface, prefer the largest same-surface slice that still has one coherent
   proof story. Split work only when there is a real risk, concept, or proof
   boundary, not merely because smaller residue items can be named.
13. Treat scores and evidence as signals, not targets.
   Lane scores, missing evidence references, and proof counts are diagnostic
   signals that help identify the real runtime, product, ownership, or
   governance gap. They are not the work item by themselves. Fix the actual
   gap first, then record the proof that the gap is closed.
14. Use worktrees when a mutating slice needs physical isolation.
   Claims and lane ownership reduce logical overlap, but they do not isolate
   hooks, dirty state, or staged scope. Use a dedicated worktree when a slice
   needs that isolation — for example when a subagent needs to mutate
   independently and land back cleanly. A shared checkout is fine for normal
   single-session work.
15. Let the active target choose the default queue.
   When the active target is a lane-expansion initiative and
   `status_audit.py --pretty` exposes an `available_candidate_lane_queue`,
   treat that queue as the default next-pick surface.
   Cleanup, presentation polish, and guardrail-only residue should normally
   support a selected candidate lane or an active blocker rather than
   displacing the queue.
   That default queue does not override the product-quality stop gate: if a
   customer-facing baseline is still not good enough to deserve iteration,
   step back to the owning redesign or canonical model first.
16. Make every governed slice explicit.
   A mutating slice should carry exactly one active `work_claim` naming the
   lane, candidate lane, coverage gap, or narrower governed item it advances.
   If the slice is support-only plumbing, claim the owning governed surface
   and say why that plumbing is required in the claim summary.
   Before replacing or releasing the claim, record any same-lane residual in
   the owning lane completion or follow-up surface instead of letting the task
   disappear as informal context.
17. Do not iterate on a bad customer baseline.
   If a customer-facing surface is still prototype-grade, confusing, or
   obviously below the product bar, do not treat it as a stable base for more
   same-shape iteration. Step back to the owning product model, UX flow,
   trust boundary, or architecture first.
18. Treat browser truth as product truth for customer surfaces.
   Code-level proof matters, but customer-facing surfaces are not acceptable
   until real in-browser behavior and interaction quality are good enough to
   deserve normal use. When a slice changes frontend code, inspect the changed
   surface in Playwright before calling that slice progress or done.

## Delegation Rule

Use this file for values and posture only.

Use the detailed canonical system for specifics:

1. `docs/release-control/CONTROL_PLANE.md`
2. `docs/release-control/control_plane.json`
3. the active profile's `SOURCE_OF_TRUTH.md`
4. the active profile's `status.json`
5. the active profile's development protocol
6. subsystem registry and subsystem contracts
7. audit scripts and guardrail tests

If the values here and the detailed canonical system ever seem to disagree,
the detailed canonical system is the path that must be corrected or clarified.
Do not paper over that mismatch by expanding prompt-like guidance further.
Use `python3 scripts/release_control/control_plane.py --agent-entrypoint --pretty`
to print the canonical ordered bundle directly instead of reconstructing it
from memory.
Do that resolution quietly unless the result materially changes the plan,
scope, or user-visible outcome.
Use `python3 scripts/release_control/work_claim.py --kind ... --id ... --summary ... --agent-id ... --pretty`
to reserve or renew exactly one governed slice without hand-editing
`status.json`.

## Current State

1. Pulse now treats this file as the evergreen values entry point for agent
   behavior.
2. Prompt-level guidance should stay principle-led and delegate detailed rules
   to the canonical control-plane and profile-specific surfaces.
3. In concrete terms: prompt-like guidance should express posture, priorities, and decision heuristics.
4. In concrete terms: the canonical system, not the prompt, carry the detailed operating burden.
5. In concrete terms: governance routing should usually be automatic and mostly invisible to the user.
6. In concrete terms: run `python3 scripts/release_control/agent_preflight.py --pretty`
   as the first executable preflight, then reserve a claim and rerun it with
   `--require-active-claim` before mutating a governed slice.
