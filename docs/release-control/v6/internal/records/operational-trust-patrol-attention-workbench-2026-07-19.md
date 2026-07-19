# Operational Trust: Patrol Attention Workbench

Date: 2026-07-19
Specification:
`docs/release-control/v6/internal/OPERATIONAL_TRUST_IMPLEMENTATION_SPEC.md`
Phase: 3, Patrol attention workbench
Candidate: `protection-posture-attention-queue`

## Outcome

Phase 3 is a product-grade canonical read path over the Phase 1 lifecycle and
Phase 2 protection posture:

1. `internal/ai/attention.go` projects canonical alert operational records,
   evidence, transitions, and recovery-owned posture into typed attention
   items. It owns ordering, lifecycle filters, pagination, summary, and honest
   calm evaluation, not writable truth.
2. `internal/api/attention_handlers.go` exposes bounded list, summary, and
   detail routes protected by `monitoring:read`. The summary avoids recovery
   history; list posture joins are batched at 200 subjects.
3. `frontend-modern/src/features/patrol/PatrolAttentionWorkbench.tsx` is the
   primary Patrol surface. Navigation and queue counts use the same summary.
   Legacy checks, investigations, and run history are demoted to collapsed
   supporting context.
4. Selected detail carries affected and related resources, impact,
   recommended next step, typed evidence and limitations, protection posture,
   timeline, owning-resource navigation, and an explanation-only Assistant
   handoff.
5. Lifecycle failure remains unavailable, posture failure remains partial, and
   neither can become a false zero, calm, healthy, resolved, or protected
   state.
6. Historical snapshots contribute recent resolved context only. A
   non-terminal history entry without a matching active record cannot revive
   false current work. Canonical record IDs containing `/` remain valid,
   percent-encoded deep-link identities.

The queue contains no action-execution affordance in this phase. Governed
offers, approval, execution, and verification remain Phase 5 work.

## User Lens

Operator job, in the least-expert plausible user's words:

> Show me what needs attention now, why it matters, what it affects, whether it
> is protected, and what I should do next.

Live exercise:

1. The authenticated launch remains monitor-first. Patrol is one navigation
   action away.
2. Patrol answers the job in its first bordered section: one heading/count,
   six lifecycle filters, an ordered queue, and refresh.
3. One item selection reaches the deepest state. It exposes impact, resource,
   evidence, protection, timeline, next step, and supporting links.
4. A 390-by-844 viewport has no document-width overflow. Selection and a
   direct deep-link bring detail into view; close restores focus to the item.
5. Reduced-motion, keyboard activation, stable screen-reader names, calm,
   partial/unavailable, acknowledged, suppressed, stale/unknown, and recent
   resolved states are covered by browser proof.
6. The live deepest state exposed 64 repeated observations for one disk
   incident. The default detail now shows the latest three and collapses the
   remaining 61 under an explicit older-observations disclosure, preserving
   forensic evidence without turning the monitor into a raw event browser.

Default-visible element decisions:

| Element | Operator action | Decision |
| --- | --- | --- |
| Active count | Judge current workload and open Patrol | Keep |
| Lifecycle filters | Inspect work without losing acknowledged, suppressed, uncertain, or resolved history | Keep |
| Ordered item row | Choose the next issue and open evidence | Keep |
| Resource, evidence, protection, age labels | Decide urgency and whether the item is trustworthy enough to act on | Keep |
| Refresh | Request a new evaluation | Keep |
| Provider evidence and timeline | Explain one selected item | Demote to selected detail |
| Repeated older evidence observations | Inspect forensic history when needed | Demote to an explicit disclosure inside selected detail |
| Patrol run history, investigations, score | Forensic or historical context | Demote to collapsed supporting context |
| Large Assistant prompt | No action before an item is selected | Cut |
| Generic trust score/proof strip | No direct operational action | Cut from the primary queue |

Vocabulary:

- `Stale or unknown` replaces implementation vocabulary about collectors and
  explicitly says that evidence is insufficient for health.
- `Protection unknown` means no complete subject-linked provider history, not
  unprotected.
- Provider/collector identity appears only in detail.

GitHub issue comparison:

1. [#1244](https://github.com/rcourtman/Pulse/issues/1244) asks for findings
   “listed at a glance”; the queue is the primary at-a-glance destination.
2. [#1234](https://github.com/rcourtman/Pulse/issues/1234) says recommendations
   are “really difficult to find”; selected detail keeps the next step beside
   impact and evidence.
3. [#1580](https://github.com/rcourtman/Pulse/issues/1580) reports confusion
   after acknowledging stale backup alerts; lifecycle filters keep
   acknowledged and stale/unknown distinct and inspectable.
4. [#1553](https://github.com/rcourtman/Pulse/issues/1553) identifies recovery
   notification noise without a preceding alert; the queue consumes the
   canonical lifecycle rather than notification delivery.
5. [#1056](https://github.com/rcourtman/Pulse/issues/1056) asks not to be
   bothered by intentionally archived backups; suppressed work leaves the
   default active queue but remains inspectable.
6. [#1215](https://github.com/rcourtman/Pulse/issues/1215) reports incorrect
   Patrol reachability information; evidence freshness, completeness,
   confidence, and limiting caveats are visible before Assistant explanation.
7. [#1223](https://github.com/rcourtman/Pulse/issues/1223) and
   [#1131](https://github.com/rcourtman/Pulse/issues/1131) document mobile
   scaling failures; phone-width deepest-state proof is now explicit.

Verdict: `product`. The primary surface answers the operator job without a
second lifecycle, generic object browser, proof strip, or Assistant-first
detour.

## Proof

Backend:

```text
go test ./internal/alerts ./internal/ai ./internal/api \
  -run 'OperationalContract|Attention|PatrolMetrics|RouteInventory' -count=1
```

Focused migration and route regressions additionally prove that orphaned
non-terminal history is excluded, resolved history remains available, and
canonical IDs containing encoded slashes open their detail route.

Frontend:

```text
npm run type-check
npm run lint
npm exec vitest run \
  src/api/__tests__/patrolAttention.test.ts \
  src/features/patrol/__tests__/PatrolAttentionWorkbench.test.tsx \
  src/__tests__/App.architecture.test.ts \
  src/__tests__/AppLayout.test.tsx \
  src/pages/__tests__/AIIntelligence.test.tsx \
  src/routing/__tests__/resourceLinks.test.ts
bash scripts/tests/test-hot-dev-runtime.sh
```

Browser:

```text
PLAYWRIGHT_BASE_URL=http://127.0.0.1:5173 \
  npx playwright test \
  tests/91-operational-trust-attention-workbench.spec.ts \
  --project=chromium
```

The browser proof renders the real frontend and uses bounded deterministic API
evidence without a live credential or mutable first-run setup. It passes active
work, every Phase 3 lifecycle filter, evidence/protection/timeline detail,
navigation-count consistency, keyboard/focus, narrow viewport, reduced motion,
calm, and unavailable-without-false-health.

`npm test -- tests/91-operational-trust-attention-workbench.spec.ts
--project=chromium` was not the correct managed-runtime invocation because that
wrapper requires Docker, which was not running. A broader `npm run dev:verify`
also encountered the pre-existing first-session setup-wizard helper timeout in
`16-dev-runtime-recovery.spec.ts`; the Phase 3 feature proof above avoids that
unrelated setup mutation and passes directly against the managed runtime.

The managed live runtime was then rebuilt from the same Development-SSD source
tree. Its live contract reported one current active item, a current
non-calm coverage state, and the migrated generic next step. The selected disk
item deep-linked successfully on a 390-by-844 viewport with a 390-pixel
document width; its default evidence section showed the latest 3 of 64
observations with 61 older observations collapsed.

Performance proof:

- `internal/ai/attention_performance_test.go` projects 10,000 lifecycle records
  and proves a bounded 200-item page.
- The summary route performs no posture-store read.
- The list performs one bounded posture batch and no per-item fetch.

## Remaining Specification Work

This record accepts Phase 3 only. It does not close the overall operational
trust goal, candidate lane, or coverage gap. Availability attachment (Phase
4), governed actions and verification (Phase 5), and rollout hardening (Phase
6) remain required before the specification's 14 completion criteria can
close.
