# Pulse Patrol deep dive

How a Patrol run actually works, for readers who want more than the overview
in [AI features](AI.md).

The short version is that Patrol pairs deterministic triage and verification
with a single model-owned agentic analysis loop. The model owns diagnosis and
finding decisions during the run, while deterministic checks bound triage,
stale-finding reconciliation, and fix verification.

## A run, end to end

A Patrol analysis cycle moves through the following stages. Each is described
in more detail below.

1. A pre-flight budget check (`CheckBudget("patrol")`), which fails fast before
   any context is built.
2. Guest intelligence gathering (`gatherGuestIntelligence`, bounded to five
   seconds), covering discovery and reachability probing.
3. Deterministic triage (`runDeterministicTriageState`), which computes flags,
   capacity forecasts, and quiet-state context for the run.
4. Focused seed context assembly (`buildTriageSeedSectionsState` and
   `assembleSeedWithinBudget`), with automatic tighter-budget retries if the
   provider reports a context-window error.
5. Core active-finding snapshotting (`establishCoreFindingSnapshot`) and the
   agentic tool loop (`ExecutePatrolStream`), where the model calls tools,
   files findings, and records explicit assessments.
6. Post-run reconciliation in `internal/ai/patrol_run.go`: flagging omitted
   active-finding assessments, stamping deterministic capacity forecasts,
   folding alert mirrors, auto-reconciling stale findings, cleaning up old
   resolved findings, recovering stuck or timed-out investigations, and
   reviewing active alerts.

## Budget check first

The budget check runs before context assembly and before a chat service is
acquired. A run that cannot afford to complete is stopped at the cheapest
possible point rather than after the expensive work.

## Guest intelligence

Discovery and reachability probing run before the seed context is built, with a
five-second timeout, so the context the model receives already reflects which
guests are reachable.

## Deterministic triage and seed context

Triage narrows the surface first, and the seed context is then built from what
triage returned rather than from everything Pulse knows. This keeps the
model's starting context focused on what looks interesting.

Quiet infrastructure still goes through the configured model. Patrol does not
skip the model when nothing looks wrong, because "nothing is wrong" is itself
a judgement worth making explicitly.

The turn budget for the agentic loop is a fixed execution limit, independent of
heuristic flags or inventory size: `patrolDetectionMaxTurns` in
`internal/ai/patrol_ai.go` grants `4` turns for an explicit quick run
(`PatrolDepthQuick`) and `40` turns otherwise.

## The agentic loop and explicit assessments

The model runs an agentic loop with tool access. Findings and assessments are
recorded through tool calls during the loop rather than parsed out of prose
afterwards, so a finding exists because the model deliberately filed it.

The original conversation owns diagnosis and finding decisions. Patrol does not
launch a separate follow-up model session to infer missing verdicts from old
finding excerpts or signal counts. When an active seeded finding finishes a run
without an accepted verdict, `patrolMissingAssessmentIDs` leaves that omission
explicit and records an incomplete-assessment error on the run.

## Deterministic signal detection and verification

Deterministic signal detection lives in `DetectSignals` and
`SignalThresholdsFromPatrol` in `internal/ai/patrol_signals.go`. It scans
completed tool call output for known problem signals without depending on model
judgement.

The signal types are:

| Signal | Meaning |
|---|---|
| `smart_failure` | A disk reporting SMART failure |
| `high_cpu` | Sustained CPU above the threshold |
| `high_memory` | Sustained memory above the threshold |
| `high_disk` | Disk or pool usage above the threshold |
| `backup_failed` | A backup job that failed |
| `backup_stale` | No backup inside the staleness window |
| `guest_unreachable` | A guest that failed reachability probing |

Thresholds come from your own alert configuration rather than from a separate
set of numbers, so detection agrees with what Pulse would alert on.
`SignalThresholdsFromPatrol` maps your configured alert thresholds onto the
signal thresholds, falling back per field to these defaults when a value is
not configured.

| Threshold | Default |
|---|---|
| Storage warning | 75% |
| Storage critical | 95% |
| High CPU | 70% |
| High memory | 80% |
| Backup staleness | 48 hours |

Backup staleness has no user-facing setting and always uses the default.

In the live Patrol runtime, `DetectSignals` powers deterministic fix
verification (`verifyBySignals` in `internal/ai/patrol_findings.go` for
`smart-failure` and `backup-failed`), alongside direct runtime-state verifiers
declared in `hasDeterministicVerifierForKey` (`backup-stale`, `cpu-high`,
`memory-high`, `disk-high`, and `guest-unreachable`). Both the resolve gate and
stale-finding reconciliation consult `hasDeterministicVerifierForKey` before
accepting a finding as resolved. Offline Patrol quality evaluations in
`internal/ai/eval/patrol_quality.go` also use `DetectSignals`.

## Investigation

A finding may be investigated after it is filed. Investigation is a separate
loop from the run that produced the finding, coordinated through the
`InvestigationOrchestrator` contract in `pkg/aicontracts/interfaces.go`.

Whether a finding is investigated at all depends on your autonomy level and on
the finding's own state. `Finding.ShouldInvestigate` in
`internal/ai/findings.go` is the gate, and it refuses in every one of these
cases.

- Autonomy is unset or `monitor`. Investigation only runs at `approval`,
  `assisted`, or `full`.
- The finding is resolved, dismissed, suppressed, or snoozed.
- The severity is not warning or critical. Info and watch findings are not
  investigated.
- A fix is already queued for approval, so the next move belongs to you.
- The finding reached a terminal outcome, such as fix verified, resolved, fix
  rejected, cannot fix, or needs attention.
- An investigation is already running, or the finding has already been
  investigated three times (`maxInvestigationAttempts`).
- The last investigation is still inside its cooldown (`investigationCooldown`
  is one hour by default, `investigationFixRetryCooldown` is twenty minutes
  after a failed fix or failed verification, and `investigationTimeoutCooldown`
  is ten minutes for timeout outcomes). Failed runs that produced no conclusion
  (empty outcome, timed out, or interrupted) and `fix_verification_unknown`
  carry no additional cooldown so the scheduled Patrol cadence owns the next
  assessment while keeping the three-attempt cap.

Patrol also recovers investigations that were left running and retries ones
that timed out. See `recoverStuckInvestigations` and
`retryTimedOutInvestigations` in `internal/ai/patrol_findings.go`.

## Why it is built this way

Model judgement is good at explaining and correlating across mixed
infrastructure, while deterministic checks are reliable for triage context,
stale-finding cleanup, and post-fix verification. Keeping diagnosis inside the
original conversation avoids hidden second-pass guesses when a model omits an
assessment, while deterministic verifiers keep resolution claims grounded in
actual runtime state.

## Related reading

- [AI features](AI.md) for the overview and configuration.
- [Patrol autonomy](AI_AUTONOMY.md) for what each autonomy level permits.
- [Patrol qualification](AI_PATROL_QUALIFICATION.md) for the evidence and
  release-claim rules.
