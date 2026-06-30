# Monitor-First Patrol Workbench Strong Version - 2026-06-25

## Decision

Pulse should present as a monitor-first operator workbench for mixed private
infrastructure, not as an AI assistant and not as a summary page that delays
monitoring. The simple user model is:

1. Monitoring pages answer "Show me my estate now."
2. Patrol answers "What did the checking loop find, and what should I do?"
3. Assistant answers "Help me understand this specific thing."
4. Coverage/posture summaries answer "Am I protected?" only when the operator
   explicitly opens that posture view or contextual expansion.
5. Patrol watches, investigates, proposes, acts within policy, verifies, and
   records outcomes underneath those surfaces.

This record expands the existing Pulse Intelligence floor. It does not replace
the current L23 contract that Patrol owns structured detection and Assistant is
contextual. It adds the stronger product target that the surfaced app must stay
simple, calm-day useful, and action-oriented.

## North Star

Pulse opens directly into monitoring, keeps Patrol as the understood checking
loop, and routes the operator from real findings/evidence to safe action.

## Strong-Version Checklist

- [x] Authenticated launch remains monitor-first: do not make a generic Home
      or posture page the default landing surface.
- [x] Patrol remains the visible destination name for the checking loop.
- [ ] Patrol groups operator work across findings, approvals, failed
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
- [ ] Calm-day monitoring and Patrol empty states feel alive and useful: they
      show protection, freshness, coverage, drift, and verification posture in
      context instead of just "nothing here."
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
2. Keep Patrol as the visible checking-loop destination while clarifying open
   findings, approvals, and verification state inside the page.
3. Preserve monitor-first launch behavior and avoid a generic Home front door.
4. Add calm-day posture only as contextual monitoring/Patrol evidence, not as
   the default landing page.
5. Demote generic Assistant entry points to contextual explain/review actions.
6. Tighten trust scaffolding and autonomy presentation across the queue.
7. Validate in browser across desktop/mobile active-work and calm-day states.

## Progress

- 2026-06-25: Product correction: do not make Home the primary landing page,
  and do not rename the Patrol destination to `Needs Attention`. Users already
  understand Patrol as the checking loop. The route, browser title, shell nav,
  command palette, and shortcut copy should lead with `Patrol`; `Open work`,
  findings, approvals, and "needs attention" remain page/status language inside
  Patrol where they explain the work.
- 2026-06-25: Implemented the correction in the desktop shell, mobile shell,
  command palette, shortcut copy, Patrol page title, and focused tests. Browser
  proof confirmed authenticated `/` lands on `/proxmox/overview`, while
  `/patrol` shows the `Patrol` title and heading on desktop and mobile.
- 2026-06-25: Simplified Patrol copy across the page header, mode control,
  current-work summary, empty states, Settings model selector, and agent
  surface manifest so Patrol consistently reads as the checking loop: check
  infrastructure, list current issues, ask according to Patrol mode, verify,
  and record the result.
- 2026-06-25: Demoted the shared Assistant shell entry from a generic chat
  opener to contextual help. The floating launcher and command palette now use
  the current route to label the action as "Ask about <view>" and open Assistant
  with factual `pulse-view` context attached. Browser proof covered desktop
  `/proxmox/overview` launcher, Assistant context strip, command palette first
  action, and the mobile launcher.
- 2026-06-25: Clarified Patrol's Open work description so it explains the next
  useful row-level step by Patrol mode: review evidence in Watch only, approve
  changes in Ask first, inspect safe fixes or automatic actions in higher
  modes, and review verification results without adding a separate proof strip.
- 2026-06-25: Added a shell-level Patrol open-work count for desktop and mobile
  navigation. The visible destination remains `Patrol`; the count is secondary
  current-work signaling driven by active Patrol findings and live Patrol
  approvals, not a renamed `Needs Attention` route or a generic Home queue.
- 2026-06-25: Made resource-backed alert investigation action-first. Active
  alert cards now lead with `Have Patrol investigate`, which starts the scoped
  manual Patrol check, while Assistant stays available as the secondary
  read-only explanation path with the alert context attached.
- 2026-06-30: Added Patrol-owned work grouping inside the `Open work` workspace
  for current signals that are not captured by a single finding title:
  approvals awaiting decisions, failed governed actions, failed latest checks,
  recurring active issues, and overdue scheduled protection. The grouping stays
  inside Patrol and does not replace issue rows, approvals, verification, or
  contextual Assistant handoffs.
- 2026-06-30: Added Patrol-owned calm-day protection posture inside the empty
  `Open work` workspace. It surfaces protection-current, latest coverage,
  schedule freshness, drift-history, and verification-waiting facts only after
  current findings, approvals, failed checks, stale protection, running checks,
  and setup failures have been ruled out, and browser proof keeps those facts
  out of the monitor-first launch page.
- 2026-06-30: Added active Patrol row scaffolding so each infrastructure
  finding can expose its problem, affected resource, consequence, checked
  evidence, next step, and verification state from existing finding, approval,
  and workflow fields. The scaffold stays attached to the issue row and does
  not create a separate proof, trust, or status strip.
- 2026-06-30: Added Proxmox overview monitor-context Patrol coverage posture.
  It appears only when Patrol has no current work, failed/latest check,
  running check, setup failure, overdue schedule, or pending approval; uses
  monitor-context labels; and keeps the Patrol empty-work `Patrol protection
  posture` list inside Patrol.

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

- Cross-source current-work grouping remains useful, but it belongs inside
  Patrol or contextual monitoring surfaces rather than a renamed top-level
  destination.
- Proxmox now has a monitor-context coverage posture. Additional monitor
  surfaces should reuse the same Patrol gating and monitor-label boundary
  before showing coverage or protection summaries.
- The status coverage gap `protection-posture-attention-queue` tracks the
  remaining product-surface work.
