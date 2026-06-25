# Protection Posture And Attention Queue Strong Version - 2026-06-25

## Decision

Pulse should present as an operator workbench for mixed private infrastructure,
not as an AI assistant. The simple user model is:

1. Home answers "Am I protected?"
2. Needs Attention answers "What needs me?"
3. Explore/dashboard surfaces answer "Let me look around."
4. Assistant answers "Help me understand this specific thing."
5. Patrol watches, investigates, proposes, acts within policy, verifies, and
   records outcomes underneath those surfaces.

This record expands the existing Pulse Intelligence floor. It does not replace
the current L23 contract that Patrol owns structured detection and Assistant is
contextual. It adds the stronger product target that the surfaced app must stay
simple, calm-day useful, and action-oriented.

## North Star

Pulse watches my estate, tells me whether I am protected, and shows only the
things I need to act on.

## Strong-Version Checklist

- [ ] Home is a calm-day protection posture surface, not a marketing page and
      not a Patrol status console.
- [ ] Home shows coverage/freshness posture for backups, agents, alerts,
      storage, recent changes, approvals, and last verification.
- [ ] Needs Attention is a first-class primary destination for open work.
- [ ] Needs Attention groups operator work across findings, approvals, failed
      checks, stale protection, risky drift, recurring issues, and unresolved
      incidents.
- [ ] Each item uses plain language: problem, affected thing, why it matters,
      what Pulse checked, recommended next step, and current verification state.
- [ ] Assistant is opened from context-specific actions such as "Ask about this"
      or "Explain evidence"; it is not the primary front door for operations.
- [ ] Patrol remains visible as the engine and policy owner, but users do not
      need to choose between Patrol, Assistant, Findings, or Intelligence to
      know what to do.
- [ ] Explore/dashboard pages remain available for browsing infrastructure;
      they do not try to become copies of provider consoles.
- [ ] Object-detail handoffs either explain the read-only boundary or route to
      the action queue/Patrol, never to an empty generic composer.
- [ ] The autonomy dial is conservative by default and explicit: Watch only,
      Ask first, Safe auto-fix, Autopilot.
- [ ] Recommendations expose trust scaffolding: evidence, confidence/reason,
      scope or blast radius, proposed action, approval requirement,
      verification result, and attempt history.
- [ ] Calm-day empty states feel alive and useful: they show protection,
      freshness, coverage, drift, and verification posture instead of just
      "nothing here."
- [ ] Mobile and desktop use the same mental model: status/protection first,
      next action second, contextual explanation third.
- [ ] Settings, docs, command palette, and onboarding stop promoting generic
      Assistant chat as the main operations experience.
- [ ] Browser proof exercises the live surfaces in both active-work and
      calm-day states before the strong version is considered complete.

## Non-Goals

- Do not remove Assistant.
- Do not remove monitoring dashboards.
- Do not hide evidence or history behind automation.
- Do not rename everything without changing the user journey.
- Do not build a generic AIOps inbox that ignores Pulse's mixed private estate
  moat: governed, trusted action across tools that cannot otherwise be
  consolidated.

## Implementation Order

1. Capture the product target in governance and tests so context compaction
   cannot erase it.
2. Reframe existing Patrol/current-work copy toward Needs Attention language
   without breaking the current L23 floor.
3. Build the calm-day Home posture model and wire it to real runtime data.
4. Promote Needs Attention as the primary work queue.
5. Demote generic Assistant entry points to contextual explain/review actions.
6. Tighten trust scaffolding and autonomy presentation across the queue.
7. Validate in browser across desktop/mobile active-work and calm-day states.

## Evidence

- GitHub issue #1234 asks for centralized management of Patrol finding notes
  and recommendations.
- GitHub issue #1244 asks for a shortcut to list Patrol findings at a glance.
- GitHub issue #1078 shows the risk of Assistant hijacking normal monitoring
  exploration when it becomes the primary interaction model.
- The existing L23 floor already establishes Patrol as structured detection and
  Assistant as contextual handoff; this record adds the stronger surfaced
  product shape needed for ordinary users.

## Residuals

- This record does not implement the new Home or Needs Attention surfaces.
- The exact nav labels remain product decisions, but the job names are fixed:
  protected posture, needs attention, and exploration.
- The status coverage gap `protection-posture-attention-queue` tracks the
  remaining product-surface work.
