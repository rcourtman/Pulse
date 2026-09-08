# Patrol and Assistant customer journey qualification

The customer job is: "Tell me what needs my attention, explain why, and help me
deal with it without creating more work." Patrol owns the issue and investigation.
Assistant explains that same issue and uses existing governed action contracts.

The earlier r34 local redesign and named qualification matrix landed. See
[verified delivery and remaining gate](#verified-local-delivery-and-remaining-release-gate)
for that source and CI evidence. The subsequent
[shared incident-history continuation](#continued-shared-incident-history-modernization-2026-09-07)
is unlanded and its required qualification remains incomplete. Production-wide
readiness also remains open.

## Active redesign plan, 2026-09-05

The maintainer requested a whole-design assessment and an explicit goal to
complete the resulting plan. The existing customer-outcomes candidate remains
the active scope under `v6-product-lane-expansion`. This is an architecture and
qualification effort, not further cosmetic refinement of the current surface.

### Product and architecture decision

Patrol owns proactive investigation of the user's infrastructure and retained
operating intent. Assistant explains and continues that same issue and its
existing investigation/action records. An observed signal, model hypothesis,
proposed action, accepted plan, executed operation and independently verified
outcome are different facts. No transition may promote one into another merely
because a tool accepted a structurally valid record.

Keep canonical resource identity, observation provenance, retained evidence,
operator intent and existing governed action records. Keep permissions, approval,
mutual exclusion, idempotency, explicit resource/cost limits, execution and
independent postcondition verification deterministic. Interpretation, relevance,
causal diagnosis, investigation choices and action judgment belong to the model.
Every orchestration pass must identify its objective invariant. A pass whose
purpose is to manufacture or force a diagnosis from proxy counts must be removed
or replaced with better model context and an explicit model decision.

The design review reproduced a concrete trust violation in
`internal/ai/chat/agentic_investigation_budget.go`: accepted proposal rationale is
called an evidence checkpoint, inserted into the final Root Cause section, and
the completion prompt forbids downgrading it in the reviewed baseline. Acceptance establishes that a
proposal was recorded, not that its causal claim is true. This mechanism must
be corrected before the diagnosis/action journey can qualify. The delivered change removes both insertion paths and the duplicated proposal
prompt state. A full-service regression preserves the exact uncertain conclusion
in the stream, returned result and persisted session after evidence and proposal
turns. The named live investigation and outcome matrix is performed below.

### Baseline and measurement limits

The recorded 2026-09-05 telemetry review in the owning coverage gap used latest
reports from monitoring-active, multi-ping installations, excluding development
and deployment proof. It contained 127 paid installations, 71 with Patrol enabled
and 23 with Assistant calls. Fourteen reported verified resolutions came from one
installation. These are the previously recorded aggregate review, not a fresh
query made during this redesign. Paid includes all non-free tiers. Cooccurring
usage does not establish a linked successful task. Schema 17 outcome/provider/cost
fields had no adoption in that review, so those fields cannot establish present
customer effectiveness or model cost.

Local live proof has independently exposed lost evidence, incorrect history
coordinates, premature compaction, unsupported causal claims and roughly
three-minute interactive investigations. One maintainer installation is useful
reproduction evidence, not a representative customer success rate.

### Execution order and acceptance

| Step | Work | Acceptance | Current state |
|---|---|---|---|
| 1. Product contract and baseline | Map the current loop and sources of judgment. Record telemetry populations and gaps. | Every identified decision has an owner. Activity is not labelled usefulness. | Complete for this redesign scope. Contract, ownership decisions and baseline limits are recorded. |
| 2. Shared evidence | Preserve canonical risk reasons and SMART counters, source/time semantics and history across tools/turns. | Regression tests preserve unknown versus zero and all canonical evidence. Real responses can inspect the same facts as the product. | Implemented and qualified for the named shared-evidence defects. Canonical disk detail, risk and cadence pass real data-path proof. Affected package and concurrency checks pass. Integrated CI later exposed remaining query and allocation regressions. The final bounded query-reuse correction passes complete selected exact-base worker comparisons and full metrics/database and focused race checks. Final landing CI passed and PRs #1928 and #1929 merged. Historical interpretation failures and the later corrected qualification are retained in step 5. |
| 3. Diagnostic orchestration | Correct proposal-as-proof. Audit triage budgets, unmatched-signal evaluation, assessment completion and investigation cutoffs. | No code-written causal conclusion. No quality inferred from tool, flag or finding counts. Each retained pass has an objective reason. Safety boundaries and incomplete outcomes remain explicit. | Proposal promotion and capture inference were removed in c5d2f56dda. Commit 668af3fe6b removes investigation success-call floors, checkpoint instructions and generic call-count wrap-up rules. The detection slice removes contextless follow-up passes, flag/report-count policy and first-finding completion modes. Full chat and AI suites, focused API and conversation race tests pass. The later source-bound real-model/action matrix is performed and recorded below. Wider rollout qualification remains open. |
| 4. Issue through verified outcome | Follow existing issue/investigation/action records into Assistant, approval, execution and independent readback. | Accepted proposal is visibly distinct from execution and verification. Rejected or unsupported actions do not become success. Uncertainty can survive an action proposal. | Implemented and locally qualified. Canonical planning returns inside the model turn, actor/request replay is persistent, and accepted actions survive later provider failure. Approved/rejected Docker and missing-runner/VM journeys passed independent live oracles. Final r34 history, action-state, attached-context and Assistant continuation matrices pass. Core PR1960 and enterprise PR23 landed the verified source pair. |
| 5. Ground-truth qualification and landing | Extend existing qualification tooling only where necessary. Exercise healthy/unhealthy, dependency, missing-access, storage/backup and approved/rejected action cases. Inspect the final browser journey at desktop and narrow widths. | Record exact source/model/permissions, evidence, decisions, faults/misses, latency and verification. Fix in-scope failures, pass appropriate proofs and land scoped commits. | Local implementation qualification performed for the named matrix. r28 real Gemini runs cover healthy, unhealthy, dependency, storage capacity, missing access, approved and rejected actions, with independent Docker and VM observations. Later projection/streaming fixes have affected race proof and final r34 Playwright proof. Core PR1960 and enterprise PR23 landed the verified source pair. Independent customer environments, backup/restore, unattended autonomy and population reliability remain separate unqualified gates. |

Use one shared runtime and the existing qualification runner, not a second
product intelligence engine or a new parallel lifecycle. Preserve independent
negative controls and fault oracles. Recorded fixtures prove contracts and
reproducibility, not live model competence. Do not tune success wording or
scoring to make the model pass.

### Diagnostic orchestration audit

| Mechanism | Current implementation | Decision for implementation review |
|---|---|---|
| Proposal rationale inserted as Root Cause | The reviewed baseline amended the conclusion after acceptance in the agent loop and service. Both mutations are removed in the working change. | Prove uncertain prose survives accepted proposals through stream, persistence and linked Assistant display. Keep the proposal record as an attributed model decision and allow uncertainty in the diagnosis. |
| Causal-resource validator | Removed the duplicate resource graph and name/status inference from the working capture boundary. Causal attribution is optional when unknown. | Prove capability/schema validation, parameter isolation and invocation integrity remain enforced. Dependency evidence remains available to the model through canonical queries. |
| Flag-count turn ladder | Removed in the detection slice. Ordinary runs have a fixed forty-turn limit, explicit quick runs retain four. | Verify flags and inventory size cannot change the execution limit or mandate findings. |
| Unmatched-signal evaluation | Removed the separate evaluator and its signal-count report budget. | Verify the original model conclusion and usage are retained without a second diagnostic session. Ground-truth missed-fault qualification remains required. |
| Missing-finding assessment sweep | Removed the separate session. | Preserve explicit assessments in the original conversation and incomplete status for omissions. No old finding excerpt may become a fresh verdict. |
| First accepted finding ends investigation | Removed the post-write summary, continuation and repair modes. | Preserve evidence tools and original context within the run limit. Accepted decisions survive a later provider failure without hiding that failure. |
| Investigation evidence-call floor | Removed in the current slice. Seed-only and failed-read conclusions survive without forced extra calls. | Completion is not diagnostic correctness. Preserve explicit limits, failed/unavailable evidence and independent action freshness checks. |
| Generic wrap-up counters | Removed the 12/18-call tool-result instructions and four silent-turn cutoff. | Explicit run limits bound work. Counts and silence do not establish evidential sufficiency. A twenty-read regression preserves available tools, observations and the model conclusion. |
| Authority and execution boundaries | Tenant identity, capability schemas, approvals, invocation IDs, parameter redaction and independent readback. | Keep and prove unchanged when diagnostic policy is simplified. These enforce objective invariants. |

This table retains the audited removal decisions. The execution-order table and
final source-bound qualification and delivery sections record their completed
implementation proofs and the separate remaining rollout gate.

### Completion and external dependencies

The named local implementation qualification is performed and delivered.
Production-wide readiness remains open. The maintainer authorized Gemini 3.8
Flash through OpenRouter with a
US$5 key limit and one-day expiry on 2026-09-06. That supported route passes the
streaming readiness and initial live Watch and dependency cases recorded below.
The earlier Claude subscription refusal belongs to the exact synthetic
continuation request. It does not establish a blanket restriction on autonomous
monitoring. The refused request has not been retried or rephrased. Readiness is
not evidence that diagnosis, action execution or independent recovery succeeds.

Release publication and wider product readiness are separate. Independent
volunteered Pro environments are still required before claiming repeatable
customer value. Capture that rollout requirement in the owning qualification gap
rather than presenting one homelab result as completion of it.

## Implemented interaction

`Explain with Assistant` on an attention item or active alert starts an explicit
explanation using the selected evidence. Opening the drawer without choosing a
task remains context only. The request preserves the composer draft and current
conversation, uses normal send/queue/retry handling, and does not enable autonomy.
Pending initialization is cancelled when the organisation changes or the drawer
component is disposed.

Attention handoffs use the canonical finding builder when exactly one finding
is linked. Otherwise they pass bounded attention evidence and typed resource and
action references without inventing a finding identity. Server-refreshed findings,
permissions, approval records, and execution policy remain authoritative.

The drawer names the selected issue, hides unrelated workflow starters while
context is attached, and explains that discovery adds service detail. Discovery
being off does not mean inventory, metrics, and alerts are unavailable.

## Browser interaction matrix

Run against the current authenticated local development build:

```sh
node scripts/check-patrol-assistant-journey.mjs
```

The script uses synthetic attention evidence and model responses, with other
non-GET application requests blocked. It writes screenshots and a result receipt
under `tmp/patrol-assistant-journey/`. It neither qualifies model reasoning nor
executes infrastructure actions. Authentication stays in browser memory.

| Surface | States and interactions |
|---|---|
| `/patrol`, 1440, 900 and 390 pixels wide | Start review, expand evidence, focus and activate Explain with Enter, inspect issue title and response, verify selected evidence/resource and one request |
| Assistant drawer | Draft preserved across close/reopen, no inference on ordinary open, no competing workflow starters, normal failed-request display and retry, reload without resubmission |
| `/patrol` Activity, 1440, 900 and 390 pixels wide | Ordinary and mirrored-alert findings, keyboard review, completed-but-unresolved investigation, failed read and nested transcript, collapse, same linked finding and uncertainty in Assistant |
| `/alerts`, desktop and narrow | Open secondary action menu, dismiss with Escape and outside click, reopen, choose explanation, verify selected alert context and no Patrol trigger |
| Initialization regression tests | Newer context cannot be cleared by an earlier send, rejected send retains context, preparation failure reports error, organisation switch cancels old evidence |

Inspect screenshots for clipping, wrapping, readable responses, reachable input,
menu placement, and visible error/retry controls. The browser verification receipt
binds the pass to exact source hashes. Rerun affected states after source edits.

## Real outcome qualification still required

A scripted response passing the interaction matrix is not evidence that the
features deliver repeatable customer value. Qualify the following jobs on a
disposable environment with known ground truth before expanding claims or autonomy.

| Job | Useful result | Controls |
|---|---|---|
| Unhealthy service | Current evidence identifies the failed service and likely cause, then gives a supported next step | Healthy service, intentional stop, missing access, correlated dependency failure |
| Backup or capacity risk | Explains actual coverage or growth risk, what is uncertain, and a concrete next step | Healthy protection, stale evidence, unavailable backup source, capacity with no safe automatic fix |
| Supported VM/LXC change | Correct canonical targets, reviewable plan, approval before execution, independent result verification attached to the original issue | Rejected approval, unsupported operation, stale target, partial failure |

Use the existing `cmd/patrol-qualify` scenario runner and
`tests/qualification/patrol/scenarios/`, with explicit live-fault and remediation
authority for the disposable lab. Validate the catalog with:

```sh
go run ./cmd/patrol-qualify -mode validate
```

The existing Docker watch/investigation/remediation catalog provides service
cases. Backup/capacity cases and the complete VM/LXC customer journey need separate
ground-truth fixtures. The existing `ProxmoxBulkLifecycleActionScenario` checks
canonical planning but does not by itself prove execution and verification.

Record source SHA, runtime version, exact provider/model, effective permissions,
scenario ground truth, observed evidence, proposed action, approval and result,
useful/incorrect/missed diagnosis, latency and tokens/cost. A correct evidenced
"no action needed" or manual hardware replacement is a useful outcome. A tool
call or an action plan alone is not.

Compare model configurations on the same cases. Establish repeatability across
independent volunteered Pro environments before treating one successful install
as a product-wide result. Keep customer content out of default telemetry.

## Measurement boundaries

Use eligible paid cohorts and latest valid installation reports. Keep current
enablement separate from historical rolling usage. Do not divide findings by
investigations or actions when their retention and provenance differ. Do not
interpret Assistant/Patrol activity cooccurrence as a linked completed task.

Provider, cost and outcome telemetry requires schema adoption before it can be
used for assessment. Link outcomes locally to canonical findings/actions, export
only content-free aggregates, and distinguish useful diagnosis, justified no-op,
blocked access, rejected action, executed action and verified resolution.

The owning governance gap is `patrol-assistant-customer-outcome-qualification`.
This interaction repair does not close that gap or authorize release publication.

## Maintainer homelab evaluation, 2026-09-05

The local development runtime used real existing connections with mock mode
disabled, `claude-subscription:claude-opus-5` through Claude CLI 2.1.211 and the
maintainer's Max login. Assistant was read-only, automatic fixes were disabled,
and the development background-work guard stayed enabled. No injected faults
or infrastructure writes were needed. This is one maintainer environment,
not independent paid-customer qualification.

The initial fleet question completed five provider turns and fourteen read-only
tool calls in 172 seconds. The transport recorded 12 input tokens and 8,406 output
tokens. Those CLI counters exclude cached input and are not complete context
usage or billable cost evidence. The visible answer was not a qualification pass:

- The temperature tool omitted a hot Proxmox node despite canonical telemetry
  reporting roughly 94 degrees Celsius. A direct read-only sensor check confirmed
  elevated temperature. The shared tool now projects Proxmox and agent readings
  with canonical identity and source, retaining sensor timestamps when available.
- The answer treated a warning-filtered count as the total alert count.
- The resource summary reported healthy while its own observations included a
  critical memory-pressure observation and omitted the thermal evidence.
- The visible final answer stopped mid-sentence. The shared artifact guard drops
  the remainder of a turn when it recognises provider-call text. The original
  pre-sanitized final response was not retained, so the precise trigger for this
  run is unproven. Completion handling and ordinary tool-reference prose need
  a focused reproduction rather than an inferred parser fix.
- Storage-risk wording exceeded the observed evidence about recoverability, and
  shared storage was asserted as the cause without a captured topology check.

The simple native subscription transport probe passed. The separate real Patrol
readiness probe passed initial connectivity and tool/context checks but its
continuation returned a provider policy refusal. The UI misleadingly framed the
incomplete probe as a latency problem. This did not qualify Patrol or justify
bypassing the provider failure. Preserve the unresolved classification issue,
provider suitability, response completeness, evidence reconciliation and actual
Patrol outcomes under the existing customer-outcome qualification gap.

The temperature-only retest received canonical per-core, package and disk
readings from the repaired tool. Its final answer still claimed that no thermal
alert existed, although that same conversation contained an active thermal
alert. Runtime tracing identified unconditional compaction after two tool turns:
by the final answer, message context had fallen to roughly 968 estimated tokens,
far below its 128,000-token fallback window. This replaced earlier observations
with abbreviated summaries before they were needed for the conclusion.

The runtime now preserves tool observations while the full request fits the
model context window. Existing overflow handling remains authoritative. A
five-turn executable regression verifies that the final request retains the
original alert, temperature and collection timestamp from all four tool rounds.
This repairs the evidence-loss mechanism without claiming that a particular
model will always reconcile evidence correctly. Canonical lookup type mismatches,
summary health, provider readiness and complete customer outcomes remain separate
qualification gaps.

With both repairs applied, the final live temperature question completed in
181 seconds. The answer retained the per-core/package and NVMe readings, the
source timestamp, and the independently returned active thermal alert with its
threshold. It delivered a complete response and distinguished missing history
and disk-health coverage from observed sensor data. This is a bounded diagnosis
improvement, not a complete pass: hardware-cause inference from a 24-hour CPU
average remained stronger than the evidence warranted, canonical lookup type
mismatches still consumed tool calls, and the roughly three-minute answer latency
remained below the interactive product bar. No remediation was requested or run.

Browser verification exercised `/patrol` at 1440x1000 and 390x1000 with the real
provider, inspected temperature tool details expanded with Enter and collapsed
with a click, scrolled the evidence and complete answer, closed and reopened the
drawer with Escape, and reloaded the persisted session. Provider selection was
also checked at `/settings/pulse-intelligence/provider`. The local receipt binds
the two changed runtime files by SHA-256. No provider responses were mocked.

A final identical readiness retry again received the provider policy refusal.
The normal manual Patrol trigger returned HTTP 409 `patrol_readiness_not_ready`.
No Patrol run or infrastructure action was accepted. Actual Patrol evaluation
with this model remains blocked on a working supported provider path.


## Resource evidence continuation, 2026-09-05

The next real read-only run exposed independent model-facing contract defects.
A canonical resource or hostname was passed directly to the in-memory metrics
store, while `pulse_summarize` already resolved the registry's metrics target.
Physical disk queries used the operation selector `type=disks` as a hardware
format filter. The advertised `node` get type was rejected. Canonical agent
projection also dropped Proxmox CPU topology. These defects are repaired in the
shared tools, with ambiguous identity rejected and canonical response IDs kept.

The first repaired performance query returned only post-restart observations.
The production tool adapter now receives the current monitor's retained SQLite
store. Resource-scoped reads use that store and return errors rather than
silently substituting shorter in-memory history. A close/reopen regression
covers Proxmox nodes, agents, VMs and system containers. Merged points are
chronological before the existing output-size limit is applied. Unscoped
summaries and baselines retain their existing providers and remain a
modernization residual.

A subsequent live Opus 5 response retrieved pre-restart observations and the
correct delly2 NVMe disk, whose SMART assessment passed at 33 degrees Celsius.
It correctly identified that the retained series covered about 31 minutes,
not the requested full day. More retained data cannot be invented. The model
still inferred a probable wear-related reason for a warning without the risk
record, and the tool omitted explicit wear units. Disk risk explanation and
coverage reconciliation remain unqualified.

The corrected incomplete-readiness projection is browser-tested using the
backend regression result, explicitly as presentation proof rather than a new
provider qualification. No provider refusal was bypassed or repeatedly retried.
The real manual Patrol endpoint remains blocked with HTTP 409. The existing
provider policy refusal still prevents a qualified autonomous Patrol run.


The named-resource repeat also misused `current_resource` from the Patrol
page, then asked which machine was intended despite `delly2` being named.
The shared boundary error had instructed the model to ask the user before any
further targeted read. It now distinguishes the unattached shortcut from an
explicit name and directs name resolution through canonical query search.
The shortcut remains blocked until attached context exists.

Live gate testing exposed an execution defect while correcting the saved
readiness verdict: both runtime and API paths treated an unassessed mode as an
executable warning. One manual read-only test run started unexpectedly and
reassessed an existing finding before the development backend was restarted to
stop it. No infrastructure mutation was performed. It is not a qualified
Patrol outcome. The runtime now blocks a provider-failed unassessed Watch result
for every selected mode, and the API consumes that execution verdict instead
of authorizing from display warnings. Operator cancellation retains its
existing non-failure behavior. The repaired live endpoint returned HTTP 409,
and the actual saved result displays latency and Watch-only as not assessed.
No new provider probe was needed to correct that persisted result.


The final named-resource retest completed without clarification in 110 seconds.
It resolved delly2 to the correct canonical agent, reported 12 CPU cores,
retrieved chronological pre-restart metrics, and attributed the KIOXIA NVMe
SMART PASSED result to the correct host. It identified the 31-minute retained
window and roughly 44-minute age of its newest point, and did not claim that
unchecked alerts were absent. Reasoning remains imperfect: child resources
were later described as guests without workload enumeration, and coincident
metric changes were treated as a probable workload start without attribution.
This is a bounded evidence-path pass, not full diagnosis qualification.

Final browser verification used `/patrol` and
`/settings/pulse-intelligence/patrol` at 1440x1000 and 390x1000. It exercised
Assistant submission and completion, keyboard/pointer tool-result expansion
and collapse, output scrolling, final-answer pixels, Escape, session history
and reload. The actual cached readiness result and actual HTTP 409 gate were
verified after the final source changes. Local receipts bind source hashes,
transcripts and screenshots under the homelab resource-browser artifact set.


## Evidence interpretation continuation, 2026-09-05

The provider refusal is now a first-class `provider_refusal` diagnostic. The
subscription transport preserves the terminal refusal before either structured
completion or a preceding tool call can be recovered. Both process exit paths
are covered. Saved legacy evidence is corrected only from a complete terminal
JSON envelope with `stop_reason=refusal`, preserving the original time and probe
counts. This produces clear policy/support guidance without a new provider
request, weakened CLI restrictions or a verified Patrol mode. The real saved
result was inspected in desktop/mobile settings and the manual endpoint still
returned HTTP 409.

The model-facing summary no longer invokes the report narrator. Its canonical
`MetricEvidenceProvider` contract reads the report engine's retained data and
returns units, extrema, means, latest values, point counts and observation
timestamps. Extrema include recorded bucket ranges and identify their buckets.
Means and latest values can be bucket averages. The response names alerts,
findings, disk health, backups and topology as not queried, requiring their own
tools before health or causal conclusions. A 92% memory reading is returned as
evidence without a heuristic critical or healthy label. Existing export/report
narratives and health cards remain separate behavior and have not been qualified
or repaired by this change.

The first live summary investigation correctly separated high utilisation from
proven memory pressure and disclosed the short, stale retained window. It still
dismissed a recorded historical temperature maximum because the current sensor
reading was lower. That is an incorrect inference, not proof that the recorded
peak is an artefact. The summary now explicitly distinguishes preserved bucket
extrema from bucket averages and includes the extrema's bucket timestamps.
Unsupported workload-change inference and assumptions about a previously
working agent channel remain model-reasoning concerns.

A separate local fixture reproduced the shared history query coverage defect.
It wrote CPU=10 in the minute tier at 21:37 and CPU=20 in raw data at 22:16.
Both 24-hour `Store.Query` and `Store.QueryAll` returned only the older point.
A two-hour query returned the newer raw point. The preferred non-empty tier
therefore hides newer evidence. Temporal tier reconciliation, including batch
queries and aggregation semantics, is recorded in the performance-and-scalability
contract under the existing outcome gap. The evidence tool exposes its returned
coverage and cannot make the underlying query complete. This is separate from
missing observations that were never collected.

The final live read-only investigation completed in 195 seconds with ten visible
tool controls. It resolved the named resource, preserved the extrema bucket
timestamps, explained the difference between extrema and plotted averages,
disclosed the 83-point / 82-minute returned span and roughly 45-minute age,
and distinguished high utilisation from demonstrated memory pressure. The
failed host pressure read was correctly treated as unavailable evidence.
However, the answer still speculated about a sensor/startup artefact, inferred
monitoring restart from coincident timestamps, claimed continuous coverage from
minute spacing, and called the returned span a retention limit. None of those
claims was established by the tool results. Recommending simply waiting a day
is insufficient given the independently reproduced tier query defect. This
is a bounded evidence-contract pass, with diagnosis qualification still open.

Final browser inspection covered `/patrol` and
`/settings/pulse-intelligence/patrol` at 1440x1000 and 390x1000. The interaction
matrix included Assistant submission/completion, keyboard and pointer expansion
and collapse, scrollable tool outputs through their final entries, final answer
pixels, Escape, session-history selection and reload. It also covered the actual
cached provider-refusal settings and blocked manual execution, with no repeated
provider readiness probe. The final saved-session pixel pass followed a correction
to the tool's descriptive governance metadata. Runtime source hashes, transcript
and screenshots are retained in the local homelab evidence-trust artifact set.
These receipts do not qualify autonomous Patrol or infrastructure mutations.


## Retained history reconciliation, 2026-09-05

The shared metrics query now reconciles each resource/metric across retained
tiers before downsampling. Preferred buckets exclude overlapping lower-priority
observations, and uncovered times and new metrics remain visible to single,
all-series and batch consumers. Rollups preserve stored extrema through later
aggregation stages. This does not recover historical peaks already discarded
or observations that were never collected. Query-plan checks exercise the actual
shared SQL builder and require indexed overlap probes.

The original local fixture now returns both the minute and newer raw point for
24-hour single/all-series reads. Regression cases cover older fallback buckets,
internal gaps, newer raw tails, overlap precedence at minute/hour/day boundaries,
resource-family isolation, per-metric filters, downsampling after reconciliation,
and single/batch parity. Customer issue #1717 independently documents the value
of API-only Proxmox history, although its earlier canonical-ID defect is distinct
from this query-tier defect. The full issue and comments were read. It has no
supplied screenshots and this work makes no new claim about that reporter's fix.

The live read-only Assistant repeat completed in 161 seconds. Its 24-hour
summary now contained 444 CPU/memory points and 436 temperature points, ending
at 22:43:54 BST, seven seconds before the summary query. The previous run returned
83 points ending at 21:39, about 45 minutes stale. The returned observation span
is now roughly 2 hours 27 minutes, with a largest timestamp gap of 61 seconds.
This validates the recovered recent tail, not a full day of collection.

The model again correctly distinguished high utilisation from demonstrated
pressure and disclosed the unavailable host agent evidence. Diagnosis remains
unqualified. It asserted continuous collection from point spacing, inferred a
monitoring/store restart from coincident timestamps, and called two temperature
readings contradictory without ruling out change between their different
observation times. It suggested treating a rising memory floor as evidence of
a leak without workload attribution. These are model-reasoning failures, not
additional proof of a storage defect or homelab fault. No infrastructure mutation
or new refused provider-readiness probe was performed.

Playwright exercised `/patrol` Assistant submission/completion, all eleven tool
controls with keyboard and pointer expansion/collapse, inner output scrolling,
answer pixels, Escape, session selection and reload at 1440x1000 and 390x1000.
The actual cached provider-refusal state at
`/settings/pulse-intelligence/patrol` and the manual HTTP 409 gate still hold.
The local retained-coverage artifact set contains transcript, current source
hashes and final browser screenshots.


The adjacent `/proxmox` node History check found a second evidence-path defect.
The view inferred a linked Agent from discovery routing, requested `agent/delly2`,
and showed Collecting history while the real source was `node/homelab-delly2`.
The canonical unified metrics target now advertises the collector's `node`
storage family and source ID for API-only Proxmox hosts, while preserving real
Agent-source precedence. The node projection carries that target to the drawer
and takes Agent linkage only from explicit metadata. Summary tools consume the
corrected target without their former Proxmox-specific exception.

Current-build browser verification exercised the node History tab at 1h, 24h
and 7d on 1440x1000 and 390x1000 viewports. Utilisation, network and thermal
charts all contained retained points. Chart hover, range changes, Overview/History
switching and reload were checked. API-only nodes no longer show Agent-only disk
throughput or a fabricated linked-Agent label. A real linked-Agent case remains
covered by drawer regression tests. No historical data was fabricated to fill
the unavailable portion of the selected range.

Worker verification passed the metrics package with the race detector
(excluding timing-sensitive SLO tests), all query SLO tests without the race
detector, and the full unified-resources package. Focused summary/reporting
regressions, the two changed frontend suites (58 tests) and frontend type checking
also passed. The node History receipt binds its final source hashes and returned
range/point metadata separately from the model transcript.

The final repeat after the canonical node-target correction completed in 215
seconds with thirteen visible tool controls. It read 194 retained points through
22:58:55 BST at 22:59:06, approximately eleven seconds old. Point count changed
as the normal rollup replaced raw points with preferred minute buckets, while
the recent tail remained available. The answer preserved the pressure-versus-
utilisation distinction and recognised a resolved thermal alert, but still
inferred a restart, continuous collection and a previously working Agent without
sufficient evidence. It also repeated the known disk-health/wear explanation
gap. These residuals remain in the outcome qualification gap. The final
retained-canonical-coverage receipts bind the current code to Assistant
expansion, output scrolling, answer pixels and reload at both viewports, plus
the actual saved provider refusal and HTTP 409 gate.


## Redesign baseline retest, 2026-09-05

The unchanged performance question completed in 224 seconds with eleven visible
tool controls after the local disk and timestamp-context changes. Playwright
exercised `/patrol` at 1440x1000 and 390x1000, including keyboard/pointer tool
expansion, output scrolling, Escape, session selection and reload. Initial answer
pixels were inspected. Final conclusion pixels and the focused disk question
remain pending, so this is not full browser qualification for the slice.

The model explicitly repeated the timestamp caveat while still calling the
returned series unbroken and inferring that the alert start almost certainly
marked collection start. It also treated a present-time pressure read as a
definitive resolution of the historical pressure question. These are failed
diagnoses, not evidence that more caveat text is sufficient.

The changed disk tool returned SMART `PASSED`, canonical `warning`, and remaining
life 63, but neither SMART counters nor risk reasons. A direct canonical resource
API read confirmed those fields are absent upstream too. The subsequent source-status read identified `proxmox.status=stale`, which
explains a freshness warning without a hardware-risk reason. The disk tool had
omitted that field too. Preserve the canonical source status alongside health
and risk before qualifying this explanation. Do not fabricate a wear warning
from 63 percent remaining.

Artifacts remain private under the local
`tmp/patrol-outcome-telemetry/homelab/diagnostic-evidence-performance` directory.
An initial browser attempt never reached a provider because the configured SSD
temporary directory was absent. The directory was restored before the recorded
run. Installed Chrome was used because the expected cached Playwright browser
was unavailable. No refused readiness probe or infrastructure mutation occurred.

The qualification scorer also needs an explicit trust review: current root-cause
grounding checks match resource names/IDs in a named Markdown section, and other
diagnosis checks use required words. Combined with code-inserted proposal prose,
this can reward a formatted assertion without independent diagnostic support.
Keep the physical fault oracles and action readback checks, but do not treat
those text checks as semantic proof of cause.

### Regression work discovered during redesign

PR #1920 CI exposed a retained-history performance regression, including the
bounded chart query and shared store reads. Reconciliation must remain correct,
but display aggregation belongs inside SQLite rather than scanning all retained
points into Go. The working correction also reuses scan destinations. Initial paired
worker benchmarks confirm reduced allocations but still show material runtime
regressions. Performance correction and source-bound browser proof remain open.

The resource API test selected the first canonical `agent` resource and assumed
it was Agent-backed. A Proxmox-only node has the same canonical resource type,
but different history storage coordinates. The test now selects the fixture's
actual Agent identity and separately asserts the Proxmox node target.

Disk warning investigation exposed another canonical gap. Proxmox physical disks
poll every five minutes by default, but source freshness currently uses the
shorter general Proxmox threshold. Registry merge status selection also requires
a regression check for recovery from stale Proxmox data. Preserve source
freshness in the tool now, and correct cadence/recovery at the owning registry
and monitoring boundary before qualifying those warnings as useful diagnosis.

### Disk live retest, 2026-09-06 local time

The current local Assistant answered the disk question through Claude Opus 5
in about three minutes. The source freshness field was visible and correctly
identified as distinct from SMART failure. The answer still failed qualification:
it invented a midnight polling restart from coincident timestamps, treated the
remaining-life measurement as ambiguous, and hit three rejected disk-detail calls.
The tool advertised `physical-disk` for get but its handler did not support it.

The next working correction routes exact canonical disk get/health requests
through the same shared projection and labels remaining life with the explicit
`life_remaining_percent` field. The live answer is a recorded failure, not proof
of completed diagnosis. The screenshots exercised `/patrol`, Assistant, tool
expansions, keyboard activation, narrow wrapping, Escape and session reload at
1440x1000 and 390x1000. Fresh browser proof is required after these later changes.

### Canonical freshness correction under verification

Physical disk collection now carries its independent polling interval into the
canonical per-source freshness record. Staleness uses at least two expected
intervals, while retaining a longer configured source threshold. A new observation
from the highest-priority available source may refresh its resource status,
including a Proxmox-only resource that previously kept its first warning.
The targeted regression covers five- and fifteen-minute disk schedules, stale
transition, identity-preserving recovery and measured hardware risk after recovery.
The live collector exposed a further loss in the quick temperature refresh:
`physicalDiskFromReadStateView` discarded cadence before writing disk records
back to monitoring state. That conversion now preserves the typed source status
schedule. Broader registry regressions and final browser proof remain required. This does not make timestamp coincidences evidence of collector restarts.

The retained-query candidate also encountered a SQLite native fault during the
full metrics package run. The same baseline package passed. This is unresolved
until the failing context and current candidate are verified. Narrow benchmark
and query-plan success do not close this failure.

### Continued orchestration review

The assessment sweep currently receives only the retained finding title, severity,
resource and up to 500 characters of old evidence. It receives neither the main
run's observations nor their collection times. That is insufficient context for a
new present/resolved judgment. The completion obligation may remain mechanical,
but any continuation must share the actual investigation context. It must not
turn old evidence into a fresh assessment.

The investigation evidence-call floor also misclassifies a failed or
approval-blocked read as no evidence at all. Such a result establishes a limit
on available access, while a successful tool call alone establishes no diagnostic
quality. Remove the quality inference and forced start-repair turn. Preserve
explicit call/turn/time limits and let an incomplete or uncertain conclusion
record the unavailable evidence. Action authority remains independently enforced.

The unmatched-signal follow-up rebuilds a reduced evidence list after the main
model run and prohibits further investigation. Its candidate selection ranks
health/reliability/backup/connectivity/anomaly categories and caps them at twenty.
This duplicates interpretation outside the model and discards the main reasoning
context. The planned replacement is the original model assessment with complete
seed/tool evidence, measured against independent missed-fault controls.

A fresh live canonical disk read after the cadence round-trip correction returned
`online` and `expectedUpdateIntervalSeconds=300`. This verifies the actual
collector-to-canonical-resource path. Full affected package tests and the current
Assistant answer remain separate acceptance checks.

### Current disk answer and browser evidence

The 2026-09-06 00:28 BST Assistant retest completed through the configured Claude
subscription route in 2m38s, with 12,436 reported tokens. All nine evidence calls
completed, including the exact canonical disk detail request that failed before.
The answer correctly interpreted 63 percent remaining life, source recency and
the distinction between disk and node temperature evidence.

The overall answer still fails qualification. It asserted that recovery exists
because an online backup datastore has space, without reading actual backup
coverage or restore evidence. It also offered host commands without checking
that execution access exists. The conclusion should remain bounded to the
observed disk state. Latency and unnecessary fleet-wide explanation remain
product defects to address in the shared diagnostic contract.

Playwright exercised `/patrol` and the Assistant drawer at 1440x1000 and
390x1000, expanded all nine evidence calls with keyboard activation, scrolled
long outputs, collapsed them, used Escape, reloaded and reopened the retained
session. Root inspection covered actual disk-list/detail output pixels, retained
summary output and the final answer through its last paragraph. The answer and
evidence remain readable at both widths. The source-bound receipt and screenshots
are private under `tmp/patrol-outcome-telemetry/homelab/diagnostic-evidence-disk-canonical-final`.
This is interface and data-path proof, not a passing diagnostic outcome.

### Retained-performance retest and transport correction

The 00:32 BST read-only performance request completed in 3m36s with 17,677
reported tokens. It correctly reported unavailable agent access and separated
high memory occupancy from established pressure. It still called the retained
slice unbroken and inferred collector startup from coincident timestamps despite
explicitly repeating the evidence caveat. That diagnosis remains failed.

This run also exposed raw serialized routing JSON in the visible conversation.
The subscription fallback had accumulated native CLI text before the first
declared tool call and forwarded it as Assistant content. It also retained only
one call from a native batch. The working provider correction returns the entire
first declared batch in order, emits no routing-turn prose, and keeps the
structured final answer unchanged. Targeted provider tests pass. All explicit
refusal and local-tool isolation boundaries remain. Fresh browser evidence is
required after this runtime change.

The transport retest exceeded the browser's five-minute limit and was cancelled
when that browser closed. The saved session contains four ordered routing turns
with 2, 5, 4 and 3 calls respectively. Their answer content is empty, so the
protocol text leak is absent from the persisted transport result. Fourteen calls
did not produce a final answer before cancellation. This is incomplete browser
qualification and failed interactive latency, not a completed diagnostic run.
The response instruction currently demands thorough investigation and suggested
next steps even after the user's specific question can be answered. Review that
shared product instruction before collecting another equally broad run.

### Scoped response and rendering retest

The 00:56 BST performance request completed in 2m42s with 13,021 reported
tokens after the shared response prompt was scoped to the user's actual job.
Routing JSON was absent, missing agent access remained explicit and memory
pressure was correctly left unknown. The answer still overstated continuous
coverage from retained timestamps, inferred history start causes, treated an
alert on the same utilization measurement as corroboration, and mixed UTC/BST.
This remains a failed diagnostic outcome. It does not qualify autonomous Patrol.

That real answer exposed a six-column table expanding the narrow conversation.
The shared sanitized renderer now gives tables their own keyboard-scrollable
region. Playwright reopened the real session at 1440x1000, 900x1000 and 390x1000,
reached the last column, verified fixed conversation width, and exercised Escape,
reload and session reopening. Browser-only short and streaming table fixtures
confirmed preserved focus, region identity and scroll position. Actual pixels
were inspected. The source-bound frontend receipt is
`frontend-modern/browser-verification.json`; private artifacts are under
`tmp/patrol-outcome-telemetry/homelab/markdown-table-final`. These fixture results
establish rendering behavior only. No additional model calls were made.

### Final evidence-foundation proof

The affected tools, chat, providers, models, unified resources, monitoring and
resource API package suites pass on the worker. The final metrics implementation
(`store.go` SHA256 `5ef4b22506ecc73131bcd881a5bbc72d2a45f43f393f2dccbfba52dc287b18e8`)
passes the full metrics suite in 90.446s, the database suite in 0.131s and the
focused concurrent read/write race proof in 19.625s. Tests cover new/deleted
history visibility, resource isolation, bounded compiled-statement retention,
one-connection operation and transaction-bound query instrumentation.

The unchanged bounded chart benchmark measures 889.3 microseconds baseline
versus 888.8 microseconds candidate (p=.796, n=10), with no significant slowdown.
Final paired metrics benchmarks pass the existing regression checker. The
single-metric downsample case reports +10.00% (p=.005, n=10), close to the
checker's greater-than-10% threshold. Raw single-series and both multi-metric
queries have no significant latency difference. These are bounded worker
measurements, not a production latency guarantee. The earlier +20.34% chart
regression is corrected by reusing compiled presence SQL while evaluating it
inside every current read snapshot. No history or tier-presence result is cached.

One earlier metrics package run terminated inside SQLite native binding code.
Subsequent full and focused race runs passed, including the final implementation.
The original fault has no established root cause and is not labelled harmless
or explained by the successful reruns. Its private worker log is
`paired-metrics-local-20260905T235637/full-metrics-final.log`.

### Diagnostic orchestration slice, 2026-09-06

The investigation loop no longer rejects a conclusion for having no successful
read or forces extra tool calls when seed context or unavailable access supports
an honest unknown conclusion. Evidence attempts still consume the configured
limit, which remains visible as a system fact. Removed completion checkpoints
and the Assistant/Watch 12/18-call wrap-up instructions from tool results, along
with the four silent-tool-turn cutoff. Overall turns, advertised capability
boundaries, configured budgets and repeated-call/error recovery remain bounded.

Full worker chat and AI packages pass against the exact changed runtime source.
The longer-read regression executes twenty distinct reads over five silent tool
turns, retains unchanged observations, and preserves the model's conclusion.
Seed-only and failed/policy-blocked read cases retain uncertainty. These are
contract tests, not proof of real diagnosis.

Browser qualification reproduced a linked-issue defect: a finding folded under
an existing alert could not stay selected because selection searched only the
non-mirrored display groups. Selection now resolves the canonical filtered
findings, preserving the existing interactive inline surface. The final
Playwright matrix opens ordinary and mirrored findings at 1440/900/390 widths,
opens the nested failed-read transcript, collapses the review and
passes the same finding, unknown conclusion and access limit to Assistant.
Scripted inference and blocked writes make this a presentation/context proof.

PR #1920's initial worker proof passed against local source 5288b64d, but its CI
comparison against the actual base 4f9de86e failed nine benchmark comparisons,
including small single-resource retained reads. Landing is blocked while the
exact base/candidate comparison is reproduced and corrected. Do not treat the
previous narrower performance pass as complete CI qualification.

### Exact-base retained-read correction

The small retained-read path performed an extra tier-presence query and opened
an explicit transaction even when the reconciliation was already one SQLite
statement. Plain reads now reuse bounded compiled reconciliation statements.
All-metric plain reads retain index time order instead of sorting by metric.
Each output series remains chronological. Display aggregation retains its
required series grouping and same-snapshot presence optimization. No data or
tier-presence result is cached.

The corrected `store.go` SHA256 is
`41132f7eae5a987e72092aceb8813a20f3dc44986c849023ffa0654e5bfe58aa`.
Ten alternating 100 ms worker samples against exact PR base 4f9de86e report:

| Query | Base | Correction | Comparison |
|---|---:|---:|---|
| Bounded single-metric API chart | 848.6 us | 894.5 us | No significant difference, p=.436 |
| Single series across resources | 916.98 us | 47.12 us | -94.86%, p<.001 |
| Single query, 500-node load fixture | 18.78 ms | 51.50 us | -99.73%, p<.001 |
| All-metric dashboard | 309.3 us | 174.3 us | -43.66%, p<.001 |
| Batch dashboard, 10 nodes | 1.912 ms | 1.723 ms | -9.86%, p=.007 |
| Batch dashboard, 50 nodes | 8.807 ms | 8.337 ms | -5.34%, p=.023 |
| Batch dashboard, 100 nodes | 17.56 ms | 16.08 ms | -8.43%, p=.002 |
| Batch dashboard, 500 nodes | 87.56 ms | 82.34 ms | No significant difference, p=.052 |

These are worker measurements, not a production latency guarantee. An exact
query-plan reproduction shows the old single-query SQL choosing the broad
tier/time index on this worker. The corrected runtime uses the series/range
index explicitly. This explains the large worker single-query difference but
does not establish which plan the prior CI runner chose. No benchmark threshold
was relaxed. The final full metrics suite passes in 87.114s, the database
suite in 0.119s and focused concurrent read/write race proof in 19.065s. The
remaining failed CI comparisons have no significant worker latency difference:
API memory fallback, 163.6 versus 167.3 us (p=.315), and the fifty-guest chart
batch, 128.0 versus 121.3 ms (p=.190). These comparisons cover the earlier failed
cases. A fresh CI run must still qualify the final landing commit. The earlier
unexplained native SQLite fault remains an open observation.

Private worker evidence is retained under
`/opt/pulse-release-worker/pr1920-bench-4f9-c5d2/`, with source-bound
`full-metrics-second.log`, `full-db-second.log`, `race-metrics-second.log`,
`second-candidate-*` and `adj-*` artifacts.


### Detection conversation redesign

The original model conversation now owns reads, finding decisions and correction
of rejected calls within the explicit run limit. Removed the separate evaluator,
old-evidence assessment sweep, signal-count report budget and post-finding prompt
replacement. The standing prompt describes Pulse's evidence, existing alerts,
active-finding obligation and action boundaries without fixed tool sequences or
forcing symptom reports before investigation. A run with no new finding may be called all clear only when the model's
evidence supports that conclusion. Final-turn and output-limit recovery instructions
also retain missing/stale evidence and avoid repeating accepted assessments.

The 01:55 UTC ordinary Assistant recheck did not yield a final answer. Server
logs record `context canceled` at 01:57:06 UTC, before the qualification runner
was interrupted at about 01:57:44 UTC. The request's eleven tool calls and partial
session remain reproduction evidence. It is a runtime failure with an unresolved
cancellation source, not a completed diagnosis. Its source/binary correspondence
has not been established as an exact final-build proof. No autonomous request or
paid alternate-provider call was made.

The rebuilt local runtime retains the same unknown cause and failed-read evidence
through ordinary and alert-mirrored finding review and the linked Assistant.
The scripted browser matrix passes at 1440x1000, 900x1000 and 390x1000, including
nested transcript expansion/collapse, keyboard review and explanation, selected
issue identity, draft preservation, menu dismissal and error/retry. Actual pixels
were inspected. This proves presentation and context continuity, not diagnosis.
The private source-bound receipt is `tmp/patrol-assistant-journey/result.json`.
Runtime binary SHA-256 before/after the pass was
`724941045a17714460f703724528a040645c752aabe01c114fc1aae6cecca52c`.

Final affected worker proof passes: full chat 13.670s, full AI 10.216s,
Patrol/API bridge tests 24.421s and focused conversation race tests 1.043s.
The race cases cover mixed accepted/rejected finding calls, a fresh evidence read
between independent findings, provider failure after an accepted finding and
exact accepted-call idempotency. The integration case retains an omitted
assessment as active and incomplete after exactly one original model run.
Zero, one and fifteen heuristic flags do not change the run limit, trigger an
auxiliary session, manufacture findings or inflate model usage. Existing
control-mode, capability and objective-observer tests remain part of the passing
full packages. Private proof logs are `/opt/pulse-release-worker/final-affected2-chat.log`
and `final-affected3-{ai,api,race}.log`.

The subsequent ordinary Assistant check on the unchanged binary completed from
02:45:25.486 to 02:48:18.734 UTC, about 173 seconds, with thirteen visible tool
records. It distinguished high utilisation from proven pressure and preserved
failed access. It still inferred observation startup from timestamp coincidence
and asserted that a change across retained resolutions was not an aggregation
artefact without supporting evidence. The temporal conclusion therefore remains
unqualified. Completion of this request does not explain the earlier cancellation
and does not qualify autonomous Patrol.

This real run also reproduced a canonical result defect: `pulse_read` with
`action=file` returned missing-agent text but a successful result. The shared
file-read boundary now returns a tool error for absent agents and nonzero
host/container command exits, preserving the original explanation. Success
continues to require actual file content. Focused negative controls cover absent
agents and both stderr/stdout error paths. The final real Assistant check executed the exact file read once and returned
`tool_end.success=false`. The failed badge and original explanation were visible
in desktop and narrow views, survived expansion/collapse and reload, and the
model described a collection limit without claiming file evidence. The single
request ran from 02:52:16.552 to 02:52:48.461 UTC. The scripted linked-issue matrix
also passed again at 1440/900/390 widths on the final rebuilt runtime.

Final binary SHA-256 was
`a575a40a640af1aaed479b56636dc5a3205c76df6999bbb20546ccf09fe70659`,
unchanged throughout both passes. `tools_file.go` SHA-256 was
`a5585c787e8a24aa5bc694b40a6c945709ecab06d9f581105131531dd16d3ac3`.
Private receipts remain under the task's
`diagnostic-evidence-file-read-failure-final/` and the repository's
`tmp/patrol-assistant-journey/`. This qualifies the named ordinary read failure
and presentation contract, not autonomous investigation or action outcomes.

The final full tools package passes in 59.413s and focused file-read race proof
in 1.033s, covering successful reads alongside the failed-read controls.
Private worker logs are `file-read-full.log` and `file-read-race.log` under
`/opt/pulse-release-worker/`.

## Integration qualification, 2026-09-06

The detection and failed-read slice is committed as `61607333cc9e`. Integration
with main `3f74c0c27304` required a fresh browser receipt. Intermediate-width
inspection exposed the shared delivery-health card squeezing its explanation
under the action buttons. The shared card now wraps by available width and uses
an opaque semantic heading colour. Permanent fixtures include 900 pixels and
check control bounds and heading overflow.

Final worker proof passes 18 delivery-ordering cases and 12 Overview refresh
cases at 1440, 900 and 390 pixels, with Overview checked in light and dark themes.
Unavailable health, retained retry/dismiss actions, pending refresh and healthy
recovery were exercised. Final card pixels were inspected at all three widths.
Frontend lint, type checking and three affected test files pass. Focused merged
Patrol/API/adapter/Docker-result proof passes. The final local scripted
`/patrol` and `/alerts` journey passes at 1440x1000, 900x1000 and 390x1000,
including nested failed-read evidence and the linked Assistant. This qualifies
the integrated presentation and context contract only. Exact staged hook and
remote landing are subsequent delivery checks. Real diagnosis and autonomous
action outcomes remain unqualified, and the provider refusal remains enforced.

## Qualification catalogue contract, 2026-09-06

The published JSON schema omitted `health_process_stop`, although the runtime
implements it and all three approved, rejected and autonomous restart scenarios
use it. A regression against the checked-in catalogue reproduced all three
rejections. The schema now accepts that implemented injector. The regression
reads the published enum and checks actual catalogue faults rather than keeping
a second injector list. This is a fault-type compatibility check, not a full
JSON Schema validator or a live-model result.

At source base `f779bf064ab4`, the corrected catalogue validates all eleven
scenarios and both qualification packages pass on pulse-dev (4.232s and 0.005s).
The private worker log is `/opt/pulse-release-worker/patrol-schema-f779.log`.
No provider request, fault injection or infrastructure mutation was performed.

The existing live runner covers Docker health, process exit/restart and network
dependency faults. It has no missing-access or storage/backup scenario or
corresponding independent lab probe. The ordinary homelab file-read failure
proof remains useful but does not close that autonomous qualification gap.
Those cases require suitable independent ground truth through the existing
qualification framework before the overall goal can complete.

## Retained-read performance correction, 2026-09-06

Build and Test run `34008823529` for integration `f779bf064ab4` failed its
benchmark comparison against exact main `3f74c0c27304`. All other jobs passed,
and Core E2E run `34008823534` passed all eight browser shards. The benchmark
failure comprised fifteen time/allocation comparisons, including batch reads
up to 59% slower and small-read bytes about 198% higher. Earlier narrow worker
comparisons omitted the actual QueryAllBatch cases and did not establish this
integrated performance result. The initial status report calling that CI job
passed was incorrect and was explicitly corrected.

The canonical reader now reuses bounded SQL templates and numbered bindings,
checks absent preferred tiers once within the current statement, and appends
consecutive results directly to their series. It retains tier reconciliation,
current snapshots, scope isolation and output semantics. No benchmark threshold
was relaxed. An initial correction still regressed the plain raw-read case by
11% and was revised before landing.

Final production `pkg/metrics/store.go` SHA-256:
`9a66d1acea82c17ca540abe9a9ee66e089a36420620e2ebb8ca7e6d102191a8d`.
Ten alternating 100ms samples on pulse-dev, Go 1.26.8 and GOMAXPROCS=4,
compare the final implementation with exact base `3f74c0c27304`. The complete
selected Query, QueryAllBatch, RollupCandidate, fleet dashboard, history API,
chart batch and NormalizeRoute benchmark families have no statistically
significant greater-than-10% regression in time, bytes or allocations. Plain
raw reads show no significant time change. Batch reads improve 16–42%, with
bytes reduced 28–34%. Single-metric downsampling is 5.66% slower and the
1,000-point rollup candidate is 6.04% slower, both inside the unchanged gate.
The bounded history API shows no significant time change. These are selected
worker comparisons, not a claim that final remote CI has passed.

Private raw samples and benchstat outputs are under
`tmp/patrol-f779-v2-selected/` and `tmp/patrol-f779-v2-normalize/` at the
workspace root. Focused tests cover newly appearing preferred buckets after a
cached absent-tier read, current resource family, metric, window and display
step bindings, per-series overlap and batch parity. Integration with main
`6c000837e27b` brings three test-only changes and no additional runtime changes.
Full metrics and database packages pass on pulse-dev in 77.265s and 0.272s.
Focused retained coverage, fresh bindings and batch identity race proof passes
in 5.349s. The two latency-based concurrent SLO tests run in the ordinary suite
but explicitly skip under the race detector. A separate canonical hot-path
regression exercises eight concurrent query scopes and mixed display steps
through a one-connection pool without latency assertions. It passes normally
(0.043s) and under race (2.106s), with no cross-query binding leakage or lock
inversion. Its worker log is `patrol-retained-concurrent-bindings.log`.
Private full/race logs are under
`tmp/patrol-f779-final-proof/`. Exact staged hook and remote landing remain
pending. This correction does not close the outstanding real-model/action outcome gap.

## Independent service-storage oracle, 2026-09-06

The qualification framework now includes
`investigation.docker-storage-pressure`, with a driver-owned fixed 8 MiB tmpfs,
a real ENOSPC write fault and an independent filesystem-statistics probe. The
service's normal writes fail and its health degrades, while an unrelated control
stays healthy. Reversion removes only the fixed fill file. The driver verifies
the prepared container ID, exact ownership labels, filesystem type and capacity
before writing. Existing fill files and symlinks are refused.

The final qualification and CLI packages pass on pulse-dev (4.223s and 0.005s).
The initial provider-free oracle run passed in 8.478s. The final live test adds
an explicit symlink outside the scratch mount, proves the target is unchanged
after refusal and reversion, and passes in 8.65s. The catalogue contains twelve
valid manifests. These tests make no Pulse API or model request.

The independent measurements establish 8,380,416 free bytes at baseline,
zero during the fault and restored free space after reversion. The service
remains running during the fault, its health changes to unhealthy, and the
control remains healthy. Final cleanup removes both disposable containers and
their network, the second cleanup is a no-op, and pre-existing inventory is
unchanged. The worker's preloaded Alpine 3.20 image digest is
`sha256:d9e853e87e55526f6b2917df91a2115c36dd7c696a35be12163d44e6e2a4b6bc`.

Production driver `internal/ai/qualification/lab_storage.go` SHA-256 is
`9e204767ddd20496c52e2566fae5690df22f840f8aac32b3af00ddc3cffc8bc8`.
Final live-test SHA-256 is
`0350997f3dbc6cbaa1877a9942e75ce500d28ecdfeff7e16fc8d8bb2cce588d1`.
Private worker logs are `patrol-storage-qualification.log` and
`patrol-storage-symlink-final.log` under `/opt/pulse-release-worker/`.

This closes the missing service-storage fault/oracle implementation, not the
required real-model diagnosis. A normal Pulse collector and supported provider
route are still required to qualify Patrol on this scenario. Host/storage-pool
and backup faults are outside this specific fixture. Missing access must be
qualified at Pulse's source or tool boundary, not substituted with an app's
unrelated file-permission fault. The overall goal and provider-refusal boundary
remain unchanged. Final staged hook and landing for this slice remain pending.

Actual published-schema validation initially rejected the new storage manifest
and three existing action manifests because `required_summary_terms` remained
mandatory despite runtime support for equivalent-term groups. The schema now
accepts either form and retains required evidence fields. A full JSON Schema
catalogue test, with negative controls for missing/empty expectations and missing
evidence, is wired into the existing Patrol regression workflow. This is distinct
from `-mode validate`, which exercises only the Go manifest validator.

All twelve manifests now pass the published schema with the CI-pinned
`jsonschema` 4.26.0. All five schema regression tests pass, including missing
and empty summary expectations and missing evidence controls. The worker log
is `/opt/pulse-release-worker/patrol-storage-schema-final.log`. Schema SHA-256
is `01ea739e2a68dc0a7b6a0041de15f5e2edcf59f31826b02a01b51a1e2b5488d`.

The final shipped guide is byte-identical to the source and passes Playwright
at `/docs/AI_PATROL_QUALIFICATION`, 1440x1000, 900x1000 and 390x1000.
The new catalogue row, storage section, command and deepest qualification
limits were inspected as pixels. The page stays within its viewport and the
long command scrolls within its own block. Reload, keyboard navigation to the
documentation index and browser-history return pass. Private matrix, screenshots
and receipt are under `tmp/patrol-storage-docs-proof/` at the workspace root.
The served guide SHA-256 is
`1970ed5cd70e976d2acbeaf7d36a7b78b5dedf65355c50a19abdba8ca96c9770`.
No model or infrastructure action is used by this browser proof.

## Disk probe completion ordering, 2026-09-06

Build and Test run `34011979848` on `f26668aa6ddc` reports a failure in
`TestCollectDisksExcludesFreeBSDFdescfsBeforeUsage`: the expected root filesystem
read did not invoke its usage probe. The shared in-flight registry published a
result before removing the completed entry. A subsequent collection could reuse
that completed result instead of taking a fresh measurement.

A controlled regression holds the registry lock while the syscall completes.
It fails on the preceding implementation because the caller returns while its
completed probe remains discoverable. Retirement and completion publication now
share one critical section. The syscall and caller waits remain outside the lock,
and overlapping collectors still share genuinely running probes. This corrects
the shared ordering contract instead of clearing state or retrying the test.

On pulse-dev with Go 1.26.8, twenty full hostmetrics package runs pass in
16.687s and three complete race-detector runs pass in 1.864s. These include
excluded mounts, stalled mounts, recovery, shared results and cancellation.
Production source SHA-256 is
`cbf5efa6bfcc163faa061ccf7c70bb6738c4ef2f78473d7d66fa00c589f87555`.
Regression file SHA-256 is
`e7a3adfe7f2cbc36cd8c315ef71ec019fa34db45b75458fa0b2212e3533b09b7`.
Both hashes match before and after proof. The raw worker log is
`/opt/pulse-release-worker/patrol-disk-probe-completion.log`.

The preceding storage slice was committed and pushed as `618700db5e` to
PR #1928, which remains open. Its exact staged hook passed 163 tests in 127.989s with all fourteen
file hashes unchanged. This supersedes the pending-hook statements above for that
slice only. Current remote CI is not a completed pass. The f266 benchmark job
also failed and its comparison remains under investigation. The overall goal,
real-model/action qualification and provider refusal remain open and unchanged.

## Large-scope query binding correction, 2026-09-06

The disk-probe slice is committed as `b964eea767` and pushed to open PR #1928.
Its final exact staged hook passes all 163 tests in 128.317s, with six unchanged
source hashes. This supersedes the pending disk-hook status above.

Build and Test run `34011979848` compares `f26668aa6ddc` against PR base
`3347f561ec7b`, rather than the earlier `3f74c0c27304` worker baseline. Its
remaining benchmark failure is the 500-node dashboard batch read: allocation
events increase 24.73%, allocated bytes increase 0.62% and runtime has no
statistically significant change. Ten alternating 100ms samples on pulse-dev
with Go 1.26.8 and GOMAXPROCS=4 reproduce all three results. The unchanged
repository benchmark checker rejects that candidate.

The pinned SQLite driver matches each numbered parameter against argument
ordinals, converting each compared ordinal to a string. Large scopes repeatedly
pay those conversions. The allocation profile identifies that conversion path.
The correction retains shared parameter values across query branches but uses
alphabetic names and `database/sql.Named`. Every read still supplies current
values. Resource identity, retention coverage, snapshots and cached shape bounds
are unchanged. Parameter names are internal positions, never user values.

An initial source-bound experiment against the same exact baseline reduces the
500-node allocation difference to +0.26% and allocated bytes to -1.68%. Runtime
has no statistically significant difference. Ten samples per variant pass the
actual repository benchmark checker. Experiment source SHA-256 is
`b73ea7984bded06b2a3319ed3a62fbe63ee87f63fd317a40b51d451f5c21b89c`.
Query-plan, fresh snapshot/window/step, concurrent binding and large-scope
binding tests pass in 0.201s. The new large-scope test reuses one cached shape
with 500 current IDs and changed family, metric filters and time window. IDs
that resemble SQL or parameter names remain data.

Private raw experiment comparisons and profiles are under
`tmp/patrol-disk-bench-go1268/` at the workspace root. The final product source
also includes an explanatory comment. Full affected package/race proof, broader
final-source comparisons and the final staged hook remain pending. The goal
remains open for real-model diagnosis, missing-access and action-outcome
qualification. The provider refusal is unchanged.

Final product source SHA-256 is
`72a7674a67ed1114be74038fb9a3d1e9c9d0507fb0d9597995f38aec0c8cbc56`.
Final test source SHA-256 is
`76be5a315e8b6dafd4a93b09711e01854087ce9f6f5a94ddb8c2ff20b3e8f708`.
Full metrics and database packages pass on Go 1.26.8 in 77.390s and 0.203s.
Focused retained/tier/binding/batch race proof, including the new 500-resource
binding case, passes in 7.707s. Both source hashes match before and after proof.
The worker log is `/opt/pulse-release-worker/patrol-named-binding-final-tests.log`.
Broader final-source performance comparison and staged-hook qualification
remain pending.

The two latency-based concurrent SLO tests are included in the ordinary full
suite and deliberately skip under the race detector. The separate concurrent
and large-scope binding regressions execute under race without latency gates.

The final broader comparison uses ten paired 100ms rounds on pulse-dev, with
baseline before candidate in each round, against exact CI base `3347f561ec7b`.
It covers all Query, QueryAllBatch, QueryManyResources and 500-node dashboard
query/concurrent-load cases, plus history API, memory-fallback control and
workload/summary chart APIs. All forty invocations succeed. Both metrics and
API comparisons pass the unchanged repository checker for time, bytes and
allocations. The 500-node read remains +0.26% in allocation events and -1.68%
in bytes versus base, with no statistically significant runtime difference.

Base source hashes match the actual git object and final source hashes match
the product manifest above. The local and worker benchmark checker hashes are
identical. An earlier setup used the invalid package path `./pkg/api` and is
discarded. The complete final run uses `./internal/api` and an actual frontend
build artifact. Its raw worker directory is
`/opt/pulse-release-worker/patrol-wide-results-corrected/`, with the final
comparisons, gates and logs copied to `tmp/patrol-named-binding-final-proof/` at
the workspace root. This establishes the selected local performance proof,
not a completed remote CI pass. Final staged-hook qualification and landing
remain pending.


### Missing diagnostic access preserves monitored identity

The named-binding correction is committed as `1f41fa174d7d` and pushed to PR
#1928 after the exact five-file staged hook passed 163 tests in 127.438s.
This supersedes the pending-hook statement above. Remote landing remains open.

The missing-access case reproduced a shared routing defect before any new
infrastructure fault was introduced. With no connected command agents, the
resolver returned before consulting monitoring topology. Known hosts, VMs and
system containers consequently lost their kind, parent and required transport.
The regression failed for all three known targets in the original source.

The shared resolver now retains topology when no server or connection exists.
A known target cannot fall through to an unrelated agent with a colliding ID.
No-target routing still requires exactly one connected agent. The file-read,
file-write, file-append, read-only execution and retained legacy command handlers
return the existing NO_AGENT failure envelope, including known resource kind and
parent node when available. The requested operation did not run. Missing access
no longer produces a successful file-write result or unsupported installation
advice. Absence of a connection does not establish a policy denial, missing
installation, guest capability, fresh observation or healthy workload.

This concerns diagnostic command access. Advertised Proxmox lifecycle actions
retain their canonical hypervisor action authority and do not acquire an
in-guest diagnostic prerequisite. Issue #1782's full body and two comments were
read as adjacent evidence of the customer harm caused by invented prerequisites.
Its requested reporter confirmation remains outstanding. No comment was sent.

The final full tools package passes on pulse-dev with Go 1.26.8 in 59.437s.
Targeted local regression passes on Go 1.27.1 in 0.538s. Worker source hashes are
unchanged across the full-package proof. Private reproduction, source manifest
and browser-result export are under `tmp/patrol-access-routing-proof/` at the
workspace root. The export invokes actual current tool handlers with controlled
connection fixtures. Its initial connected mock lacked a GetConnectedAgents
expectation and failed before export. The corrected export passes in 0.513s.
Browser fixtures prove rendering only. They do not qualify model judgment or
real command-scope enforcement. Final browser, race and staged proofs follow.


Final focused race proof explicitly runs all five new/missing-read tests and
passes in 1.051s on Go 1.26.8. An earlier broader name pattern passed but omitted
three newly named routing tests, so it is not used to claim their race coverage.
Existing API regressions for implicit monitoring-token scope and real WebSocket
rejection without agent-exec scope also pass in 0.122s. These preserve the access
boundary. They are not a model-led missing-access investigation.

Playwright and pixel inspection pass for known disconnected target, unknown
target and ordinary failed-read controls at `/patrol`, 1440x1000, 900x1000 and
390x1000. The 18 cases cover ordinary and mirrored findings, keyboard review,
completed-but-unresolved records, readable error evidence, expanded investigation
transcripts, collapse, linked Assistant explanation and deepest failed-tool
input/output, Escape and context-only reopen, plus reload without resubmission.
Nine chat requests are intercepted, with no infrastructure writes. The backend
results are serialized from actual final-source handlers with controlled
connection/command fixtures. Model conclusions and transport are scripted.
The generic Patrol toolbar opens context-only Assistant, so the proof checks
no automatic submission there rather than expecting a persisted issue session.
One permission-control pass exhausted a five-second wait while capabilities
were still loading. Its complete matrix passes with a twenty-second request
wait. This does not establish a latency SLO. Final receipts/screenshots are in
`tmp/patrol-access-browser-proof/{known-vm,unknown-target,permission-denied}/`
at the workspace root. No frontend runtime source changed.

The final routing source hashes are:

- `internal/ai/tools/tools_control.go`: `f6ac2e05542657a1d859196b06c3a68288e08b436db6dabf4f691ded744dc751`

- `internal/ai/tools/tools_file.go`: `c42002e1bf012024e1b38aa64738e0a8d9e0609699a49362675e01e7891d763e`

- `internal/ai/tools/tools_read.go`: `643a02f70432adff75355ef3083d84ab24f43c3b811bba7730cfca2ca4fd8de7`

- `internal/ai/tools/strict_resolution_test.go`: `7f88e16933c0b91b1ef7754cb7df977501fdc7278540a6ec70c1c4283e08c62d`

- `internal/ai/tools/file_docker_test.go`: `6bb8f5df1291175b9f7c52dd863293bfe89a4b189ef3ed79daeaf2f8d7adb45b`


The missing-access slice is ready for its exact staged hook. The broader goal
remains open for real-model diagnosis and approved/rejected action outcomes.
On head `1f41fa174d7d`, governance, all eight Core E2E shards and CodeQL pass.
Build and Test run `34015148620` is still pending with no jobs, so omitted PR
checks are not treated as success. PR #1928 remains open with auto-merge enabled.


### Live dependency and restart fault contracts

The missing-access slice is committed and pushed as `58caeda69ba3` to PR #1928.
Its exact eight-file staged hook passes all 163 tests in 127.706s with unchanged
hashes. The first hook environment lacked PyYAML. The complete rerun used
PyYAML 6.0.3 and jsonschema 4.26.0, with the initial failure retained separately.
This supersedes the pending-hook statement above. Remote CI remains open.

The existing dependency and action scenarios had schema/mocked-command coverage
but no explicit live Docker oracle regression alongside the storage oracle.
`TestDockerDependencyAndRestartOraclesLive` now exercises the checked-in
investigation dependency manifest and all three approved/rejected/autonomous
service-restart manifests. It shares the existing DockerLab, explicit daemon
selection and exact run-scoped cleanup. It never contacts Pulse or a model.

On pulse-dev with Go 1.26.8 and pre-existing Alpine 3.20 image digest
`sha256:d9e853e87e55526f6b2917df91a2115c36dd7c696a35be12163d44e6e2a4b6bc`,
the live package run passes in 65.080s. The dependency case takes 12.53s, and
the three service cases take 17.51s, 17.51s and 17.52s. Each records baseline,
fault, unchanged fault after refused duplicate injection, explicit fixture
recovery and restored baseline. Stopping the dependency makes its running
client unhealthy, and starting it restores both. Stopping the service health
process leaves its container running and unhealthy until explicit restart.
Every cleanup passes, second cleanup is a no-op, and pre-existing containers,
volumes, networks and images are unchanged. No image was pulled.

This proves the fault/oracle contracts, including that observation does not
repair the fixture. Direct fixture recovery is teardown. It is not a Pulse
approval, rejected-action execution, autonomous action or verified customer
outcome. The required real-model and canonical action journeys remain open.
Raw log: `/opt/pulse-release-worker/patrol-dependency-action-oracles-live.log`,
copied to `tmp/patrol-dependency-action-oracles/` at the workspace root.
The live test source SHA-256 is
`78b72dc44231cd3ecbd0b2ee925d53a5836af3141e695152046aa032580f1e48`.
The ordinary qualification and CLI packages pass in 4.224s and 0.008s on Go 1.26.8 with the explicit live environment unset. The test hash is unchanged. Commit `173d74a8e422` is pushed to PR #1928 after the exact four-file staged hook passed all 163 tests in 129.364s with unchanged source hashes. Remote landing remains pending.


### Current ordinary Assistant diagnosis remains unqualified

One read-only ordinary Assistant request on 2026-09-06 ran from
06:49:50.441Z to 06:53:26.724Z, taking 216.283s with 15 tool calls. The
configured `claude-subscription:claude-opus-5` route returned HTTP 200. The
request explicitly used `autonomous_mode=false`. No alternate paid provider,
Patrol readiness retry or infrastructure mutation was performed. This single
request establishes neither an aggregate success rate nor a latency SLO.

The missing-access correction works in this real run. One diagnostic read
returns failed `NO_AGENT` while preserving the monitored node identity, and
the answer correctly retains that access limit without retrying the read.
Diagnosis still fails factual review. The answer quotes the tool contract that
returned timestamps cannot establish collection uptime or missing-history cause,
then claims that a coincident alert timestamp identifies observation start. It
calls 181 seven-day returned points hourly even though their returned span is
under twelve hours. It moves the 83.3% memory minimum from the previous evening
at 20:34 to around 04:20, and uses low average CPU as evidence against pressure
without direct pressure measurements. Correct tool metadata did not prevent
these unsupported interpretations. Do not count a completed request or an
accurate missing-access message as successful diagnosis.

Private receipts, persisted tool outputs, request/source/binary bindings and
factual evaluation are at workspace-relative
`tmp/patrol-current-assistant-check/`. The backend binary hash remains
`1e1c3a9c0c758a4a751afa16200c6fba793acc93fd56ea62e7eb432edcf5f6c8`
before and after the request. The Playwright run exercised the persisted answer
and expanded failed-tool input/output at `/patrol`, 1440x1000 and 390x1000,
including reload. Pixel inspection confirms readable answer and error evidence.
The test intercepted non-GET requests other than the one ordinary chat. Its
blocked readiness check caused the selected-route warning in the screenshots,
so those screenshots do not establish the route's unmodified readiness UI.

The supported alternate-provider test remains awaiting its separately billed
US$20 maximum approval. The cached subscription refusal for autonomous Patrol
remains enforced. Further prompt or orchestration rules are not justified merely
because this response ignored already explicit evidence limitations. Real-model
diagnosis, approved/rejected action outcomes and wider customer readiness remain
open qualification requirements.


### Landing benchmark follow-through

Run `34017211910` on `173d74a8e422` completed with the frontend, full API
race shard, both remaining backend shards and build/smoke checks passing. Its
only failed job was the unchanged benchmark gate, with four NormalizeSegment
time regressions against exact base `3347f561ec7bc7ae30903e64998b0f90b5fb5217`.
A ten-pair, 500ms-sample worker reproduction confirmed three segment regressions,
while full middleware time and allocations remained unchanged. Both source
files and normalized compiled instruction streams matched between base and
candidate. The binary addresses differed. Layout sensitivity is an inference
from these observations, not a proven functional defect.

The canonical route-label classifiers now inspect ASCII bytes directly instead
of decoding Unicode runes that cannot satisfy the numeric/hexadecimal checks.
Existing label precedence and non-ASCII behavior are preserved, with regression
cases for long numeric IDs, Unicode digits/names and malformed UTF-8. The full
HTTP-metrics test file passes under the race detector in 1.057s on Go 1.26.8.
Ten new alternating baseline/candidate pairs pass the existing greater-than-10%,
p-less-than-0.05 gate with 500ms samples. Segment numeric, UUID, long-token,
short-name and medium-name time changes are -32.19%, -33.59%, -38.83%, -37.06%
and -37.11%. Bytes and allocations are unchanged. Adjacent route and full
middleware benchmarks have no significant regressions. These are local
microbenchmark results, not a claim about customer-perceived application speed.

Final source hashes:

- `internal/api/http_metrics.go`: `916c27ad5ff07e00170dd94df0b75eb509dd329e34288647f5358a0759353d37`
- `internal/api/http_metrics_test.go`: `4d425559d84052a50de56286101a6b37422027629d076c845920d8d02f9aff41`

Worker comparison: `/opt/pulse-release-worker/patrol-normalize-final-bench/`.
Private copies, the failed initial comparison and exact CI failure output remain
at workspace-relative `tmp/patrol-current-assistant-check/`. The preceding
diagnostic-record commit `ba69933da352` passed its two-file staged hook, all
163 tests in 127.527s with unchanged hashes. The current runtime change still
requires its final staged hook and exact-head remote CI. Overall diagnostic and
action-outcome qualification remains open.


## Healthy and dependency Assistant qualification, 2026-09-06

Two ordinary read-only Assistant requests used the configured
`claude-subscription:claude-opus-5` route with explicit `autonomous_mode=false`.
These are single-case observations, not an autonomous Patrol pass or an estimate
of customer success, false-alarm or missed-problem rates. The earlier policy
refusal remains in force. No paid-provider request was made.

The unchanged Docker dependency manifest ran through the existing `DockerLab`
on the monitored Tower host. Only run-owned Alpine containers and their private
network were created. The existing production container was untouched. Independent
Docker observations and Pulse resource convergence established healthy client and
dependency, then a stopped dependency and running-but-unhealthy client. Fault
injection used a deliberate stop with a five-second grace period. Its exit 137
therefore does not establish OOM.

| Case | Observed result | Limit |
|---|---|---|
| Healthy client | 82.835s, seven tool calls, no failed tools. Assistant correctly recommended no action. Independent observations before and after the request retained both healthy containers and unchanged identities. | The answer incorrectly inferred no contribution to or effect from host storage pressure from empty mounts and zero sampled writes. The primary decision passes this case, but the full explanation does not. |
| Stopped dependency | 204.384s, sixteen tool calls, three failed reads. Assistant identified the stopped sibling and treated the dependency explanation as a hypothesis. It preserved the command-access limit and did not claim exit 137 proved OOM. | It excluded storage causality from zero sampled I/O, overstated what `OOMKilled=false` establishes, recommended restarting the client without first establishing that need, and claimed continued failure after dependency recovery must be a client-healthcheck fault. Those claims exceed the observations. This is partial diagnostic evidence, not a qualification pass. |

The fault case also received two `app-container not found` responses from config
reads despite successful canonical get responses. That tool capability/error
contract remains an in-scope follow-up. No post-answer Docker fault observation or
explicit recovery phase was recorded before the fixture's 25-minute deadline.
Deadline cleanup completed at 08:16:40.820Z, removed both run-owned containers and
their network, then passed a second no-op cleanup with unchanged original
inventory. The helper exited on its deadline. This proves cleanup, not a completed
recovery or action-outcome journey. Neither Assistant request mutated the lab.

Private evidence root:
`/Volumes/Development/pulse/tmp/patrol-assistant-lab-readiness/runs/asst-20260906075136356-380d4d/`.
It contains independent `oracle/` receipts, healthy/fault Pulse convergence,
complete SSE streams, persisted sessions, requests, screenshots and before/after
source/binary bindings. Both requests used unchanged runtime binary SHA256
`ebd0c2c74e4dbee330284a12a99137cfcecaecfe1ec2545c25432f06183a10c1`.
HTTP windows were 07:53:58.742Z to 07:55:21.577Z and 07:58:04.234Z to
08:01:28.618Z. Displayed token counts are not complete context or billable-cost
measurements. Browser interception blocked unrelated non-GET requests, including
route checks, producing an artificial selected-route warning. It did not test
route recovery or retry the autonomous refusal.

### Tool evidence identity correction

The captured browser responses exposed a separate reproducible evidence defect.
Concurrent starts/progress used name fallback even when invocation IDs differed.
Completing one same-name action could remove another invocation's approval card.
The shared reducer now treats supplied IDs as authoritative through start,
progress, cancellation, completion and approval cleanup. Older ID-less name
matching remains the existing compatibility path.

A second reproduction showed deep Solid store reconciliation mutating objects
shared by `toolCalls` and `streamEvents`. Removing a workflow row could change the
first query's input and output into a later alerts call. Message rendering now
keys rows by message ID and reads the immutable message through an accessor.
It keeps DOM stability without copying or mutating the evidence graph. Regression
proof includes the actual status-row-removal trigger, concurrent same-name calls,
sibling approval retention and cancellation. The affected test files pass all
167 tests, including existing message mount-stability checks.

Browser matrix: `/patrol` at 1440x1000, 900x1000 and 390x1000. Captured-response
replay checks all seven healthy and sixteen dependency tool inputs and outputs
against their exact terminal SSE records. It also exercises hover/focus, keyboard
expansion/collapse, output scrolling and restored persisted sessions. A controlled
stream fixture exercises concurrent pending tools, repeated starts, progress,
out-of-order completion, independent approval cards, cancellation and failed
completion. These are renderer/identity checks, not model or action qualification.
Private replay and state receipts live under
`/Volumes/Development/pulse/tmp/patrol-assistant-lab-readiness/identity-*`.

PR #1928 merged its earlier scope at `6d2d188867430f653e1bf9ada634fd2b90440786`.
The later diagnosis record and route-label performance correction merged through
PR #1929 at `cf98358c0eb46987a82def5776fa41db5f54210a`. Its backend, frontend,
benchmarks, governance, CodeQL and eight Core E2E shards passed. The identity
correction above is a separate scoped change and requires its own landing checks.
The redesign remains open for reliable interpretation, the config-read contract,
storage/backup, approved/rejected action outcomes and supported autonomous Patrol
qualification. Wider customer readiness still requires independent Pro environments.

### Docker measurement correction plan, 2026-09-06

The next shared-source correction distinguishes absent block-I/O observations
from measured idle zero and removes the Docker layer-size ratio from filesystem
capacity. Counter presence uses the existing rate tracker contract. Missing
reports must not reset the baseline or fabricate samples. Container layer sizes
remain descriptive metadata.

Persisted Docker-family disk series previously mixed invalid capacity ratios and
unobserved I/O zeros with real measurements. New disk observations use separate
physical series keys while public metric names remain unchanged. Retained reads
exclude ambiguous legacy disk series without deleting or relabelling them. The
shared app-container storage family also serves non-Docker providers, so new
valid capacity observations must remain supported. Non-Docker series retain
existing behavior. Explicit zero must survive every retained-read API and rollup.

Browser verification is required after the final backend build. Interaction
matrix: `/docker` at 1440x1000, 900x1000 and 390x1000, container selection,
resource drawer open/close, current metrics, history expansion, measured idle,
unavailable readings, and reload. Inspect actual pixels, scrolling, focus and
Escape dismissal. `/patrol` evidence rendering must preserve absent versus zero
in current-resource and retained-history tool results. Controlled responses may
qualify rendering but cannot qualify diagnosis. Live read-only API observations
must bind to the rebuilt backend. No autonomous subscription retry or paid-model
request is authorized by this correction.

Collection also carries optional presence for each I/O direction. Explicit zero
entries survive the report JSON. For older reports without presence, only
positive counters establish an observation, so ambiguous zeros remain unavailable
until the agent is updated or a positive baseline exists. This does not require
re-enrollment. Docker-host first-disk history and network-counter presence are
adjacent limits outside this container block-I/O correction.

The final browser matrix also covers the shared host I/O table, Docker host
Overview and Machines table/tooltip at the same three widths. Partial read/write
observations must show a missing marker for the absent direction, retain measured
zero, and remain excluded from sums used for sorting and comparison. Exercise
column selection, hover/focus, tooltip dismissal and scrolling where present.

### Docker correction qualification and scope

The implementation carries per-direction presence from collection and report
JSON into the existing rate tracker, canonical resource metrics, persisted
history and resource-to-browser conversion. REST resource adaptation also
preserves optional rates. Shared rate formatting keeps missing values distinct
from zero in Machines and Docker host details. Incomplete rates do not become
complete throughput totals for sorting or comparison. A browser-discovered
first-user column migration bug is corrected in the shared preference hook, so
showing Disk I/O survives the first reload.

Legacy workload conversion in `frontend-modern/src/hooks/useWorkloads.ts` still
uses numeric direction fields with grouped availability. Its direction-level
modernization remains a separate consumer follow-up. Docker-host first-disk
history and network presence are also outside this container measurement slice.
The correction must not be represented as complete coverage of all metrics or
all monitoring surfaces. No new model competence or autonomous action result is
claimed.

Affected Go package checks and targeted race checks ran on pulse-dev with
Go1.26.8. The changed websocket assertion now expects observed read zero with
absent write omitted. Targeted frontend suites and type checking cover optional
rates, REST conversion, sorting, formatting and column persistence. Ten paired
read-benchmark rounds used the unchanged parent store via Go overlay. The
canonical >10%, p<0.05 regression gate passed, with +0.82% timing geomean in this
scoped comparison. This is not a fleet-load or full-product performance claim.

The Pro backend was cross-built on pulse-dev from the changed source and
installed into the existing local dev runtime. Binary SHA256:
`59f05f954ff8080bd3e8f3054b2b059281c49172ee771b2e454c255241158a4a`.
No production agent was replaced. Older agents remain compatible and treat
ambiguous zero counters conservatively.

Private receipts: `/Volumes/Development/pulse/tmp/patrol-docker-observed-metrics/`.
Worker logs: `/opt/pulse-release-worker/patrol-docker-observed-proof/`.
PR1934's preceding identity correction merged at
`6b0abc3bee9ffa81f6ab298b5b67ee11369688a0` with all checks passing. The current
measurement slice passed final-source Playwright inspection on `/docker`,
`/standalone/machines` and `/patrol` at 1440x1000, 900x1000 and 390x1000.
Live history, controlled absence/idle/loading/error, partial host rates, column
persistence, nested picker dismissal, tooltip focus, and expanded Assistant
evidence were exercised. The source-bound receipt is
`frontend-modern/browser-verification.json`. The unused shared host table card
has type and selector coverage, not an active-route browser claim. Controlled
responses qualify rendering only. Landing checks remain separate from the
unperformed diagnosis, approved/rejected action and recovery qualifications.

### Configuration-read correction plan

A successful canonical container get followed by a false config `not found`
result is a source contract defect. Native configuration reads must use current
canonical inventory for identity and provider capability. Optional session
resolution preserves continuity for later actions, not proof of existence.
Explicit query restrictions must be checked before registering or refreshing a
resource. Unsupported adapters, missing configuration providers, unavailable
placement and empty provider responses remain distinct from missing inventory.
No action validation or native log-read authority changes in this slice.

Regression matrix: TrueNAS config with absent, empty and existing session
context, canonical identity across aliases, explicit query denial without a
provider call, Docker unsupported capability, genuinely missing inventory,
unavailable placement, provider failure and nil provider response. Reproduce
the failing cases before changing runtime code. Run affected Go tools checks
and focused race coverage on pulse-dev.

Browser matrix after the final build: `/patrol` Assistant tool result details
at 1440x1000, 900x1000 and 390x1000, available configuration, unsupported
capability, true missing resource and denied/provider-failed results. Exercise
open/closed details, hover and keyboard focus, Enter/Space, deepest output
scrolling, Escape, and persisted/reloaded evidence. Use captured actual tool
results to qualify rendering without claiming model diagnosis or native
provider integration. No autonomous subscription retry or separately billed
provider request is part of this correction.

### Configuration-read correction qualification

The baseline reproduced absent/empty session failures, stale session placement
and false not-found results after successful canonical gets. The corrected
read path uses canonical resource identity and current provider placement. It
checks explicit query restrictions before registration and preserves an existing
query-only session's action limits. Unsupported configuration, unavailable
provider/placement and nil provider responses carry explicit reasons and the
tool error bit. Unavailable inventory and missing read state also remain failures
rather than evidence of resource absence. Actual inventory absence remains the
existing not-found lookup result.

Fourteen focused contract cases pass with strict resolution enabled. The
existing native-config regression, full tools package and focused race proof
passed on pulse-dev with Go1.26.8. The final-source Pro binary SHA256 is
`552699cdf2e61a4ca1cea2ac5ef4e065735cbd1dbca01665456e184bd4fc3533`.
It is installed only in the existing local dev stack. No production agent or
provider configuration was changed.

Playwright passed on `/patrol` at 1440x1000, 900x1000 and 390x1000. Eight actual
tool results were replayed and inspected, including successful, unavailable,
missing, denied and failed reads. Expanded inputs/outputs, keyboard toggles,
scrolling, Escape, reload and controlled session restoration preserve exact
evidence and error state. Controlled session responses prove rendering and
reload behavior, not server persistence or a new model/native-provider result.
The source-bound browser receipt records those limits. Private artifacts are
under `/Volumes/Development/pulse/tmp/patrol-config-read-contract/` and worker
logs under `/opt/pulse-release-worker/patrol-config-read-proof/`.

PR1935's Docker correction required two legacy partial-total test expectations
to be updated in `0fcb2ee147354de770dfc4b0b9672d8c2c9dceb2`. The focused 55-test
file and scoped hook passed. Its latest CI has no failures and remains pending
completion. The configuration correction still requires its own landing checks.
Real-model retest, temporal/storage interpretation, approved and rejected
action outcomes and independent recovery proof remain open. Autonomous
subscription refusal and separately billed provider approval boundaries remain
unchanged.


### Corrected storage evidence, ordinary Assistant retest

The Docker observation and canonical configuration corrections are pushed to
PR #1935 at `355ac1f0a481d6dbc9a7ff3977bced0956711979`. Their exact staged
worker pre-commit checks passed without source changes. Remote checks remain
pending. The current configuration runtime also passed all fourteen contract
cases, the complete tools package and focused race checks.

One ordinary read-only storage assessment ran on 2026-09-06 from
12:27:05.857Z to 12:30:03.726Z, an HTTP window of 177.869s. It used the existing
`claude-subscription:claude-opus-5` route, explicit `autonomous_mode=false`,
read-only control and thirteen successful tool reads. No infrastructure change,
paid-model request or autonomous readiness retry occurred. This is a single
assessment, not a success-rate or latency estimate.

The answer identified the backup datastore at 90.6% utilisation and its active
capacity warning affecting seven workloads. It used the corrected container I/O
history, separated cumulative device counters from rates, acknowledged missing
container filesystem usage, and retained the reason for missing older history
as unknown. It did not attribute older host I/O peaks to the container whose
returned I/O window starts later. These are useful observations.

The complete diagnosis still does not qualify. Its opening assurance that the
container is not short of space contradicts the later acknowledgement that
container filesystem usage is unavailable. It treats high retained host rates
as bucket/counter artifacts without establishing that mechanism. It includes a
host CPU maximum timestamped 21:00 the previous evening in a 03:00-04:00 window.
The suggested retention explanation is not established by capacity alone, and
available PBS job reads were not performed. Its rough growth extrapolation uses
retained extrema, not a measured first-to-last slope, and must retain that limit.
Corrected observations have not established reliable interpretation.

The run also highlights a tool-context distinction to review: Docker-host
`agent_connected` describes the command connection, while telemetry may still
arrive through other collection paths. The model treated current telemetry and
that false connection flag as an unresolved inconsistency. Its storage-pools
request supplied `host`, although the tool schema only advertises that filter
for RAID and Ceph detail. That call returned all pools. Neither observation
justifies fabricating resource absence or collection downtime.

Playwright exercised `/patrol` at 1440x1000 and 390x1000, the actual answer,
all thirteen expanded tool records, keyboard activation, deepest output
scrolling, Escape, reload and the persisted session. Every displayed input and
output matches the persisted tool records. Pixel review includes the answer,
evidence and the mobile table scrolled to its rightmost state. The table's
400-pixel content is reachable inside its 309-pixel horizontal viewport.
The artificial selected-route warning comes from blocked non-GET route checks,
so this does not qualify the unmodified provider-readiness UI.

The runtime binary SHA256 stayed
`552699cdf2e61a4ca1cea2ac5ef4e065735cbd1dbca01665456e184bd4fc3533`
through the request and browser pass. Private request, source, binary, tool,
persistence, evaluation and pixel receipts are under workspace-relative
`tmp/patrol-storage-assistant-check/`. No native config action was requested,
so this assessment does not qualify model use of that corrected action.
Storage-fault ground truth, reliable diagnosis, approved/rejected actions and
independent recovery remain open. The supported autonomous provider dependency
and wider independent-Pro-environment gate remain unchanged.


### Command connection evidence correction plan

Live command connectivity and retained monitoring observations are independent
facts. The shared tool contract will name command-agent connections explicitly,
including parent-node connections, without changing routing or execution policy.
Topology built without a command-connection snapshot must omit connection flags,
execution hints and connected counts rather than manufacture false/zero values.
An observed empty snapshot still reports disconnected/zero. Assistant inventory
context must preserve the same observation boundary. Existing permission,
approval and invocation checks remain authoritative.

Regression matrix: current Docker inventory and metrics with disconnected and
connected command transport, read-only control with a connected agent, parent
node versus guest connection, topology without a connection observation versus
an observed empty set, and Assistant's seeded inventory. Run affected tools/chat
packages and focused race proof on pulse-dev.

Browser matrix after rebuilding the local Pro backend: `/patrol` at 1440x1000,
900x1000 and 390x1000, actual captured query results showing disconnected and
connected command transport beside unchanged monitored workload evidence.
Exercise tool details open/closed, keyboard focus/activation, deepest output
scrolling, Escape, reload and persisted result presentation. Inspect pixels and
bind receipts to the final source and binary. Controlled rendering proof does
not qualify model interpretation, autonomous Patrol or infrastructure actions.


### Command connection evidence qualification

The original projection failed the new regression because it labelled command
transport as generic agent connectivity and emitted connected-agent counts from
an inventory-only seed. Canonical guest search also promoted a parent-node
connection into a direct guest connection. The shared projection now retains
those distinctions. Existing host aliases remain available for non-guest
resources. No routing, approval, execution or provider policy boundary changes.

Four canonical query cases pass: no command connection, connected read-only
transport, a direct guest connection without a parent connection, and connected
transport with control enabled. Current workload state and CPU remain available
in every case and no command is executed. Separate checks prove that topology
without a command snapshot omits connection and execution hints and connected
counts, while an observed empty snapshot retains false/zero. Assistant inventory
context inherits that same unobserved state.

Final source proof on pulse-dev used Go1.26.8 and GOMAXPROCS4. The full tools
package passed in 59.456s and chat in 6.402s. Focused race checks passed in 1.048s
and 1.030s. The Pro runtime cross-build passed and the installed local binary
SHA256 is `bcaf748107211ee733a6dc0f4d17220d9b4d1ce1918c25bde27cf3d10c0d6379`.
The managed development process restarted onto that artifact and `/api/health`
reported healthy. No production agent was replaced.

Playwright exercised nine captured results at `/patrol`, 1440x1000, 900x1000
and 390x1000. Inputs, outputs and completed states match exactly before and after
controlled session reload. Hover, keyboard focus/activation, expansion/collapse,
deepest output scrolling, Escape and session selection passed. Pixel inspection
covered each distinct connection state, unchanged workload metrics and restored
mobile results. Backend and renderer hashes remained unchanged. The artificial
route-check warning and controlled persistence fixtures retain their earlier
qualification limits. No model request was part of this proof.
A read-only settings check confirms the cached `provider_refusal` still carries
its original `2026-09-05T19:46:39Z` timestamp and `patrol_capable=false`.

Private source bindings, logs, captured outputs, runtime process/health receipts
and browser proof are at workspace-relative `tmp/patrol-command-context/`.
The change still requires its scoped pre-commit and landing checks. The preceding
PR #1935 head `4d302109cee0758a132ff150935630b50114cc05` has no reported failures
but its Build and Test and Core E2E runs are pending behind live earlier runs
on the same branch. Those workflows are not restarted or cancelled.

This correction establishes the connection evidence contract, not reliable
interpretation. Native configuration-read model use, storage-fault diagnosis,
approved/rejected action outcomes and independent recovery remain open, as do
the supported autonomous provider dependency and independent-environment gate.


### Ordinary Assistant storage fault and recovery, 2026-09-06

The command-connection correction passed the exact nine-file worker pre-commit
and was pushed as `f5f440dbad18d83557104d2cf6197d8319949e44` in PR #1935.
Required CI remains in progress. This is not a release or a completed goal.

An owned DockerLab run used the checked-in storage-pressure manifest on Tower.
Independent observations established a healthy worker with 8,347,648 free bytes,
then a real ENOSPC fault with zero free bytes, a running/unhealthy worker and a
healthy control. Pulse collection converged to both states before the request.
The ordinary read-only Assistant used the configured subscription Opus 5 route.
It was not an autonomous Patrol request and did not retry the cached refusal.

The diagnosis took 114.847 seconds and ten tool calls. It identified the worker's
unhealthy state, the control's current healthy state and a failed command-route
log read. It did not retry that unavailable capability. However, it falsely
ruled out resource pressure using low CPU, memory, network and disk-read values.
Filesystem capacity was absent from its evidence and was independently full.
This is a failed diagnosis, despite its otherwise useful uncertainty statement
and suggested diagnostic read. No model-directed mutation occurred.

After the answer, the independent oracle still found zero available bytes and
an unhealthy worker. Removing only the owned fill file restored 8,220,672 bytes
and healthy status. Pulse collected recovery and resolved the health alert.
A follow-up in the same Assistant session took 91.358 seconds and five new reads.
It correctly identified current recovery and the resolved alert, distinguished
symptom recovery from an unknown cause and did not invent an intervention.
It overstated continuous control health and non-impact from sparse observations.
The recovery assessment is partial, not a complete incident explanation.
An independent post-answer check confirmed healthy worker/control, no container restart
and 7,639,040 free bytes. Both cleanup passes passed, with no second-pass work
and unchanged unrelated inventory. The disposable resources are removed.

Playwright exercised `/patrol` and Assistant at 1440x1000 and 390x1000, all fifteen
retained tool input/output pairs, keyboard expansion/collapse, deepest output
scrolling, complete answers, Escape, reload and the same retained conversation.
Rendered inputs/outputs match persisted records. Pixel inspection covered both
answers and the failed-access result on desktop and mobile. Runtime and source
hashes remained unchanged across both requests, with binary
`bcaf748107211ee733a6dc0f4d17220d9b4d1ce1918c25bde27cf3d10c0d6379`.
The route warning was an artifact of blocking non-chat POSTs in the proof browser.
The original autonomous refusal timestamp remained `2026-09-05T19:46:39Z`.
Private fixtures, source bindings, observations, screenshots and assessments
are at workspace-relative `tmp/patrol-storage-fault-case/`.

### Next canonical correction: tmpfs inventory

Before implementation, source and native inspection establish a collection gap:
Docker reports the owned scratch mount in `HostConfig.Tmpfs`, while `Mounts` is
empty. `internal/dockeragent/collect.go` copies only `Mounts`, so shared resource
queries falsely present an empty mount inventory. Preserve these native tmpfs
entries through the existing report mount type. Keep destination, type and
reported options, derive read/write from those options, preserve authoritative
existing mount records and deterministic ordering. Do not infer used/free space
from a configured size. No enrollment, permission or production agent change is
part of this collection correction.

Proof plan: reproduce the captured tmpfs-only inspect shape through the actual
collector, then cover existing mounts, overlapping representations, read-only
options and absent host configuration. Run targeted/full collector checks on
pulse-dev and verify the report through the existing shared projection. Browser
proof after the final change must exercise mount evidence in resource details
and Assistant tool results, desktop and mobile, including deepest expansion and
reload. A captured-result rendering check is not installed-agent or model
qualification. Leave those limits explicit until the new collector is exercised
through a supported installed path.

The incident lookup also needs an identity audit: the canonical container ID
returned no incident recording while the observed health alert used its legacy
Docker resource ID. This is a concrete lookup discrepancy to investigate, not
yet proof that a recording exists. Model inference from unmeasured capacity and
sparse health history remains an open quality failure. Supported autonomous
provider, approved/rejected actions and independent environments remain open.

Further shared-projection inspection before editing found that `MountInfo` drops
native mount type/options and that canonical app-container queries derive write
access from equality with the single string `ro`, misreporting compound read-only
options. The same slice must preserve type/options and the canonical `RW` boolean
through both canonical-provider and typed read-state query paths. Add a query
regression and capture its actual output for final Assistant browser proof.
This remains mount configuration evidence, not measured filesystem capacity.


### Tmpfs collection and query contract proof

The captured tmpfs-only and mixed-mount regressions failed against the previous
collector, then passed after the collection correction. The full dockeragent
package passed in 18.883s and focused race proof in 1.030s. Existing monitor report
mount propagation and discovery mount regressions passed. Both query paths
failed because type/options were lost, then passed after projection correction.
The full tools package passed in 59.473s and focused race proof in 1.030s.
A final output-only capture rerun passed in 0.013s. All proof used Go1.26.8 and
GOMAXPROCS4 on pulse-dev. The final Pro cross-build passed and the installed
local binary SHA256 is
`0a21dca4106c2ddc6873a3aca3b23378dccef35383ca00d7e9292b966de7c200`.
The managed local backend restarted and `/api/health` reported healthy.
No production collector was replaced.

Final Playwright proof used captured canonical resources at `/docker`, widths
1920, 1440, 900 and 390 with height 1080. It exercised mount summary/title,
keyboard row expansion/collapse, mobile row tapping, adjacent detail state,
mount-destination search, Escape and reloaded search state. The existing wide
mount column is truncated with a full title. Responsive details have no dedicated
mount section. This is an existing presentation limitation, not full mobile
mount inspection qualification. Assistant's complete mount evidence is readable
at `/patrol`, 1440x1000, 900x1000 and 390x1000. Both actual query projections
passed exact input/output comparison, hover/focus, keyboard expansion/collapse,
deepest scrolling, controlled session reload and reopening retained records.
Pixels were inspected on desktop and mobile. Source/binary hashes stayed fixed.
The original autonomous refusal timestamp is unchanged.

These browser fixtures qualify rendering of the corrected shared fields. They
do not qualify an installed collector, actual model interpretation of tmpfs
configuration, or durable backend persistence of those fixture sessions. The
ordinary live diagnosis/recovery records above have real server persistence
and retain their failed/partial judgments. Exact scoped hook and landing remain
required. Required model/action qualification and independent environments are
still open. The typed compatibility get path also does not accept the canonical
ID returned by its list path, so its mount regression uses an existing accepted
name. That identity residual is recorded for modernization, not silently fixed
through this mount projection.

## Canonical incident history, 2026-09-06

The tmpfs correction passed the exact worker hook and was pushed as
`6e18777d30f30b498def39d30016a697cabc4ea7` in PR #1935. That scoped
delivery does not change the failed storage diagnosis or partial recovery verdict.

The incident audit found a source-of-truth mismatch, not evidence that an existing
recording merely needed an ID alias. The legacy five-second recorder has no
production alert callback connected to its coordinator. It samples cached values
using recorder time without preserving their source measurement time. Connecting
that recorder would not supply trustworthy higher-frequency history.

The canonical resource timeline already stores observed changes, alert lifecycle
events and executed actions. Assistant handoffs use a bounded excerpt of this
same store. The shared `pulse_knowledge` incidents action now reads that
organization-pinned timeline directly, using the supplied canonical resource ID.
It does not require the resource still to exist in current inventory, infer
identity from names, include related resources implicitly, or reconstruct events
from current metrics. The response preserves canonical source, observation and
optional occurrence timestamps, state transitions and metadata. `since` filters
on observation time, and bounded results report `has_more`. Empty retained history
does not establish health. Missing or failed storage is a failed read.

Explicit legacy `window_id` lookups remain isolated archive reads, must match the
requested resource, and explain that sample timestamps do not establish source
freshness. The primary incidents action no longer uses those recordings. The
legacy recorder/coordinator startup and API active-count plumbing still exist.
Their retirement is a separate cleanup in this redesign and must preserve any
saved archives. Do not connect them as a replacement incident truth source.

Qualification uses the real SQLite resource store with a fired/resolved lifecycle,
an older excluded record, a related-resource negative control, absent occurrence
time, truncation and empty history. Unavailable/failed storage, invalid input and
archive resource isolation are separate negative controls. Captured actual tool
responses must pass the Assistant expansion, scrolling and reload matrix at
`/patrol`, 1440x1000, 900x1000 and 390x1000. This is contract and rendering proof,
not a new real-model or continuous-coverage claim.

Read-only API inspection of the actual removed storage fixture confirmed a
remaining canonical write-boundary defect. The canonical app-container timeline
returns its creation and removal, while the fired event at 13:14:28.59485Z and
resolved event at 13:17:58.634095Z remain under its legacy Docker resource ID.
The resource API includes related network changes by design. The new tool uses
direct resource history only. `recordAlertTimelineChange` passes the alert's
source ID directly to `BuildAlertTimelineChange`, and `MonitorAdapter.RecordChange`
forwards it without canonical resolution. Consequently this read-path change is
only partial incident-history remediation. The next required owning fix must
resolve event identity before persistence and preserve access to retained prior
identity records, including removed resources, through the shared identity/history
contract. It must not add a Docker string rewrite inside the Assistant tool.
The shared writer and retained-identity correction remain required in this goal.
Private raw API receipts are in `tmp/patrol-canonical-history/live-timeline.json`
and `live-legacy-alert-timeline.json`. No model call or infrastructure mutation
was made during these reads.

The history regression passes through the registered tool dispatcher. The full
tools package passed in 59.856s, the final focused capture passed in 0.036s, and the
focused race check passed in 1.189s on pulse-dev with Go1.26.8 and GOMAXPROCS4.
The final Pro build passed and was installed into the local development stack.
Its SHA256 is `4929aeb869db54122bc5525352d3126c0e9fa7c4847e3fdfa741500600b5e00d`.
The managed backend restarted healthy. Final Playwright proof passed all five
registered-tool cases at `/patrol`, 1440x1000, 900x1000 and 390x1000, including
hover/focus, Enter/Space, deepest output scrolling, Escape and controlled session
reload with exact input/output and success/failure comparison. Root inspected
actual pixels at all three widths. Source and binary hashes remained fixed.
The provider warning stayed visible and no retry, route switch or provider
request was made. This is captured-response rendering, not real-model diagnosis
or server persistence qualification. The exact scoped worker hook gates delivery
through PR #1935.


## Shared Docker history identity, 2026-09-06

The preceding incident-read correction was pushed as
`580a246981c76b401e9007f5ac65c89355b64c6d` in PR #1935. Its live retained
records established the identity split addressed here.

The canonical fix belongs to the shared monitor/store boundary. Exact full
Docker container references resolve through current registry identity. Retained
bindings survive inventory removal, and deterministic source-specific identities
allow legacy records to be found after restart. Names and abbreviated IDs are
not sufficient evidence. A small organization-scoped history alias index joins
readable records without rewriting event IDs, timestamps or metadata. Existing
canonical succession machinery was deliberately not used for these aliases
because it also moves operator state and action indexes. History matching must
not transfer authority. The alias index follows journal retention and separate
store connections read fresh bindings.

Focused regression and race proofs passed on pulse-dev with Go1.26.8 and
GOMAXPROCS4. Full unifiedresources, monitoring and tools packages passed in
37.826s, 79.866s and 59.584s. They cover real alert-manager callbacks, recovery
after inventory removal, restart, replay, same-name controls, tenant isolation,
unchanged operator/approval records and registered Assistant tool reads. Scoped
history lookup measured 0.261–0.275ms with one alias and 0.317–0.336ms with
20,000 unrelated aliases, at 6,280 bytes and 94 allocations per read. These are
worker microbenchmarks, not fleet or frontend performance qualification.

The verified worker Pro binary has SHA256
`bb6d1508a5b4d23943c37dfc42198f132c0139805dcd1891ee18aca0a9f9dd54`.
It was installed into the local development stack and restarted healthy. Both
canonical and legacy timeline API queries now return the same seven retained
records for the removed storage fixture, including the original fired/resolved
records with exact unchanged content. The complete registered-tool rendering
matrix passed at `/patrol`, 1440x1000, 900x1000 and 390x1000. Six captured cases
include migrated history, bounded and empty results, and unavailable/failed
reads. Hover/focus, Enter/Space, deepest scrolling, Escape and controlled-session
reload preserved exact inputs, outputs and completed/failed states. Pixels were
inspected at all three widths. Source and binary hashes remained fixed. Private
receipts are `tmp/patrol-history-identity/browser/receipt.json` and
`live-history-proof.json`. Controlled session responses qualify rendering, not
server persistence or model diagnosis. The cached provider refusal remained
unchanged. No model request or infrastructure fault was made. The exact scoped
worker hook remains the delivery gate for PR #1935.

This history correction does not qualify the failed ordinary storage diagnosis,
partial recovery claim, installed tmpfs collector, autonomous provider, approved
and rejected action outcomes, or independent Pro environments. The unused legacy
recorder/coordinator still needs retirement with its archives preserved.


The history identity change passed the exact thirteen-file worker hook and was
pushed as `919331d5b3f6076f8616b07eb8e9ca611f26ee52` in PR #1935. Integration
with main `11a8cc2180aae886ec7f92e2333002b57cf1b9a3` preserves both sides of
three additive subsystem-contract conflicts. The host-ingestion auto-merge
retains Docker observation corrections alongside incoming host-link provenance.
Unrelated registry indentation was restored without changing its decoded data.

The combined monitoring, unifiedresources, tools, config and models packages
passed in 85.894s, 41.931s, 59.609s, 18.381s and 0.098s. Focused history,
Assistant, host-link and lifecycle race checks passed. Frontend type checking
and four alert suites passed all 42 tests. The incoming delivery-log component
browser proof passed at 1440x900 and 390x900, including reordered success/failure,
held events, pending state and newest failure. Pixels were inspected. This is
scripted component proof, not installed notification delivery.

The final merged-source Pro binary has SHA256
`234f625cb74be3d300facfed1bb17b17e20c44c06037a1f8f4fffc1c4f49d621`.
Its local restart was healthy. The complete six-case Assistant matrix was
repeated at `/patrol`, 1440x1000, 900x1000 and 390x1000, with exact tool records,
keyboard expansion/collapse, deep scrolling and controlled-session reload.
Pixels and source/binary bindings were checked after this final build. Both
actual removed-container timeline queries still return the same seven retained
records with the original fired/resolved content. No model request, route
switch, infrastructure fault or production collector replacement was made.
The autonomous provider refusal remains enforced. Integration receipts are in
`tmp/patrol-history-integration/`. The full integration hook gates its merge
commit and push. All previously recorded model and wider-readiness gaps remain
open.

## Legacy recorder retirement, 2026-09-06

The disconnected incident coordinator, five-second cached-metrics sampler,
pre-incident buffers, archive writer and unused adapters are removed. They had
no production alert trigger. Canonical resource history remains the primary
incident evidence for Assistant. No replacement diagnosis or scheduling policy
was added.

Explicit archive lookup now requires exact organization, resource and window
binding. The reader is lazy and read-only. Saved file contents, modification
time, mode, old observations, metadata and summary values survive reads. Missing
archives, malformed files and missing windows remain distinct outcomes. Legacy
`recording` status is historical, and the response discloses that the old
`summary.duration_ms` field contains nanoseconds. An old file is never rewritten
to make its evidence appear current.

The incidents API now reports `active_count: null` with
`active_count_status: not_measured`. The retired coordinator's empty map never
established a measured zero. Its legacy incident-memory listing still needs a
canonical query design covering aliases, canonical-only events, honest bounds
and propagated projection-read errors. This is recorded as an open modernization
residual rather than treating that listing as complete.

Final-source registered archive-tool receipts pass Playwright at `/patrol`,
1440x1000, 900x1000 and 390x1000. The five cases cover a saved observation,
unavailable/malformed archives, the wrong resource and a missing window.
Verification includes hover/focus, Enter expansion, exact tool input/output,
deep scrolling, Space collapse, Escape, reload and controlled session reopening.
Actual pixels were inspected. Incoming main alert dispatch wording also passes
its isolated real Overview browser script at all three widths. These controlled
responses prove rendering, not model diagnosis, installed delivery or server
persistence.

The final worker Pro binary is
`bd29e6f27be7b3ad4cfbc37842f4da90f08c6a48c9fc23b12c9c597b67346c9b`.
After the managed local restart, canonical and legacy queries still return the
same seven retained homelab records with the original fired/resolved events
unchanged. The live incidents API reports an unmeasured count. Cached provider
refusal remains enforced. No model request, paid spend, provider retry,
production collector change or fault injection occurred in this slice.

Archive, tools, chat, AI runtime and targeted API/race checks passed on the
worker. One full API run as root invalidated its mode-bit persistence-failure
fixture. That fixture passes unchanged under the normal worker account. The
full API rerun passes under that account (286.960s), as do the incoming
startup-replay and legacy-boundary source checks. Frontend type checks and all
29 incoming alert tests pass. The exact staged hook gates landing.
Private receipts are under `tmp/patrol-archive-retirement/` in the workspace.

### Live collector storage evidence, 2026-09-06

`TestCollectContainerStorageFaultLive` calls the production Docker client and
`collectContainer` implementation against the existing storage-pressure lab.
The opt-in command, run on the worker beside its Docker Unix socket, is:

```sh
PULSE_QUALIFY_ORACLE_DOCKER_CONTEXT=default go test ./internal/dockeragent -run '^TestCollectContainerStorageFaultLive$' -count=1 -timeout=240s -v
```

The final test passed in 8.694s on Docker 29.8.0 against the runtime source tree
of `186ce504c8f0fa6e0174f3b10f3d99f6278f10fc`. Its SHA256 is
`cdf303f4de23c020690c81b0b57190c728319788cb0aaff906b0d4d9d864541e`.
Independent filesystem observations measured 8,380,416 available bytes before
the fault, zero during it and 8,380,416 after recovery. The service stayed
running. Collected and JSON-decoded health followed healthy, unhealthy, healthy,
while the unrelated control remained healthy. Docker returned zero native
`Mounts` throughout. The report retained the single tmpfs destination, type,
8 MiB configuration options and writable setting from `HostConfig.Tmpfs`.
Configured size is not a measured capacity counter in the report.

The test targets exact run-owned container IDs and labels. Existing fixture
cleanup passed, its second cleanup was a no-op and inventory matched the
pre-run snapshot. The test skips without explicit opt-in. All four existing
mount regression cases also pass. Private logs are in
`tmp/patrol-storage-collector-live/` in the workspace.

This proves live collection and report serialization only. It does not send a
report to Pulse, enroll or replace a production agent, call a provider, or
qualify approval, execution, diagnosis or model recovery. No runtime or frontend
source changed in this slice, so no new browser claim is made. Prior storage
diagnosis failures and the cached autonomous-provider refusal remain open.


## Funded Gemini qualification, 2026-09-06

The maintainer authorized `openrouter:google/gemini-3.8-flash` with a provider-side
US$5 key limit expiring on 2026-09-07. The provider key endpoint confirmed both
constraints. Credentials remain in runtime configuration, not these receipts.
Synthetic readiness passed in 9.034 seconds: three streaming tool scenarios,
two context fixtures and multi-turn continuation. This supports the readiness
claims for Watch only and Ask first. It does not qualify autonomous fixes.

The first unhealthy-container run, `q-20260906-172952-3cccbf34`, detected the
correct fault and left the healthy control alone, but failed overall. The exact
Gemini route had no price entry, and the model attempted unsupported Docker
configuration access. The shared price table now records the reviewed standard
rates of US$0.75 input and US$3.75 output per million tokens for direct Gemini
and OpenRouter. Variant routes remain unknown. These introductory rates must be
reviewed on 2027-01-01. The query capability description now explicitly names
TrueNAS as the supported app-container configuration adapter and directs Docker
collected health/mount/port/network reads to `get`. Runtime permissions and
qualification gates are unchanged.

The following runs used the worker-built Pro binary
`74464e75977caf55cda092c8cf56c24967616c8c24d770872fbea5d86e31a1dc`,
core base `b0b39f00dc6685ad9ed63e8a6e91b954338073e4` plus the pricing and
capability-description changes, and canonical enterprise base
`3d9f4e3051d38027355a2a1f36b8c7f672a09b65`. The worker archive commit
`d9cb84e15d1acc377341129bdda5c28176e7128c` has identical contents for all
87 tracked enterprise files. The existing runner created disposable
resources on Tower, waited for normal collection, and used independent fault,
recovery and cleanup oracles.

| Case / run | Result | Evidence |
|---|---|---|
| Unhealthy, `q-20260906-174546-a7a9810b` | Pass | 9.709s detection phase, two tools, no failed/duplicate calls, healthy sibling unflagged. |
| Unhealthy, `q-20260906-174708-81f8d655` | Pass | 10.541s detection phase, exact unhealthy resource found. |
| Unhealthy, `q-20260906-174758-14deaa15` | Pass | 9.395s detection phase, exact unhealthy resource found. |
| Unhealthy, `q-20260906-174853-71cf894f` | Pass | 25.673s detection phase, exact unhealthy resource found. |
| Healthy mixed, `q-20260906-180144-381874a6` | Pass | 5.200s detection phase, no false findings. |
| Dependency, `q-20260906-175058-de5e350d` | Pass | Starting from only the client symptom, identified the stopped dependency and affected client. Investigation completed in 18.970s with three evidence calls and no mutation. |
| Storage, `q-20260906-175232-3ce6fbfa` | Fail before inference | Normal collection never converged to the required resource projection. No model diagnosis was attempted. |
| Approved restart, `q-20260906-175812-0593b9b7` | Fail before approval | Detection and investigation completed, but no exact action reference existed. The broker refused because Tower's Docker command agent was disconnected. Nothing executed. |
| Rejected restart, `q-20260906-175940-bff6992a` | Fail before rejection | No exact action was available to reject. This does not qualify rejected-action handling. |

Every listed run passed cleanup, including second-cleanup no-op and unchanged
inventory. Individual Watch run estimates were about US$0.007 to US$0.014.
Those scorecard estimates cover the Patrol detection phase, not the separate
investigation calls. Provider-side aggregate spend is the budget authority for
this temporary key. The fixed route price does not turn an estimate into a
reconciled bill or establish a hard Pulse budget for unpriced history.

Live qualification exposed two additional shared contract defects. The
investigation orchestrator logged action-broker refusal but completed the
record without retaining the error, leaving the model's captured-proposal prose
visible without the later refusal. The current enterprise change retains the
original diagnosis, persists the broker refusal as a failed investigation with
`needs_attention`, and creates no action reference. The product history adapter
also projected result-bearing transcript calls back into provider request calls,
dropping observed output and success/failure. The current core change uses one
shared transcript type for stored chat and product history, preserving the
separate explicit provider projection.

The live review exposed duplicate detail IDs, duplicate unformatted conclusions,
paused history made unclickable by the scheduling switch, and narrow filter/sort
overlap. The shared finding/investigation surfaces now preserve one detail target,
render sanitized Markdown once for identical summaries, retain distinct summaries,
keep history available while paused, and wrap controls. Result-bearing tool calls
use the same expandable evidence component as Assistant. Historical calls without
a result status retain their evidence without invented success or failure. An
investigation outcome of `cannot_fix` or `needs_attention` does not identify who
resolved the finding, so the shared resolution copy no longer infers manual review.

Private run receipts and source/binary bindings are under
`tmp/patrol-gemini-38/` in the workspace. The original failed runs remain failed.
Installed storage collection and a temporary command-enabled lab agent remain
prerequisites for real storage and approved/rejected recovery qualification.
No production agent has been replaced. Full action outcomes, remaining backup
coverage and independent volunteered Pro environments remain open.


### Final refusal and evidence-retention proof

Two further approved-remediation attempts remain **failed**:
`q-20260906-181839-6cbdc711` and `q-20260906-182952-6acfb739`. Both retained the
broker error separately from the original model summary, saved `status=failed`
and `outcome=needs_attention`, and created no action reference. Both passed
cleanup. The final run used Pro binary SHA256
`24de8c9ea0020067d298c489f0d99a272b5ec4b00afab7a40dff55aabb244061`,
including the proposal-response clarification, and detected its exact unhealthy
container with no false positives. Its saved investigation is
`48a16b05-50f0-4605-847c-0a71b3435975` for finding `ca3af29ac54d540f`.
The original failed scorecards have not been reclassified as action passes.

The restored history API retains observed outputs and explicit `success=false`
for historical `pulse_read` failures. Live browser review at `/patrol` exercises
successful query output, both `ACTION_NOT_ALLOWED` and `NO_AGENT` failures,
original diagnosis, one broker error, paused history and review focus return.
The settings proof at `/settings/pulse-intelligence/patrol` checks the exact
model, reviewed rates, synthetic readiness limits and reload. The current
source-bound browser receipt records desktop, intermediate and mobile results.
GET response fixtures cover unknown historical result status only, without
claiming new persisted model evidence or action execution.

Remaining qualification requires a current installed collector and a temporary
command-enabled lab agent. A Linux amd64 agent has been built on the worker,
SHA256 `ae2ed8b97709ec6e71af979c293ca9d3634662767ec1b59629baf4933c90cf5d`,
without installing it or changing Tower credentials. Tower's separate production
reporting agent is untouched. Any agent enrollment must use the canonical scoped
installation flow, preserve explicit identity and revoke temporary execution
access after qualification. Detection success does not satisfy this prerequisite
or the remaining backup and independent-environment cases.


## Installed agent and governed action qualification, 2026-09-06

The maintainer explicitly approved a temporary update and scoped command token
for Tower's separate development agent, followed by restoration. The installed
agent artifact was `ae2ed8b97709ec6e71af979c293ca9d3634662767ec1b59629baf4933c90cf5d`.
Both host and Docker modules reported running, and the command connection
registered the same agent identity. The production agent retained PID 752388.
The tests used Ask first with manual triggers. Scheduled Patrol ended paused in
Watch only. No autonomous-mode qualification is claimed.

The first installed storage attempt, `q-20260906-192435-f0d7eebf`, failed before
inference. Inspection established a qualification-client pagination defect:
`/api/resources?limit=1000` returned a maximum of 100 records from an inventory
of 104, leaving the exact worker on page two. This was not an absence of normal
collection. The client now follows the API pages and rejects partial results
when a later page fails. A regression finds an unhealthy resource beyond the
first 100, and the complete qualification package passes. Fault oracles and
score thresholds were not weakened. The corrected runner hash is
`a27f9ab0bf4786b670db3e8a989e974c8c43c5084d9524474ee694735d2e7df9`.

| Case / run | Automated result | Reviewed outcome |
|---|---|---|
| Approved restart, `q-20260906-193013-12d25545` | Pass | Correct unhealthy container, exact finding/investigation/resource and plan-hash binding, explicit approval before execution, completed restart, independent healthy/running readback and lifecycle verification. Detection 8.893s, fault-to-remediation phase total 78.490s. |
| Rejected restart, `q-20260906-193207-7095dad5` | Pass | Exact plan rejected, no restart, independent unchanged unhealthy fault until teardown. Detection 10.838s, fault-to-decision phase total 37.795s. |
| Storage, `q-20260906-193613-556ef23d` | Pass from existing scorecard | **Fails semantic diagnosis review.** Collection converged and logs exposed ENOSPC, but the model incorrectly asserted that Tower was out of disk space and implicated its array. Only an 8 MiB container tmpfs was exhausted. |

The approved action is `act_dcc3b52e5451810e49466daf9a6fccb0`, linked to finding
`566515f71129ce73` and investigation `aae05717-c9b0-4aac-8439-ba78f46c28e9`.
The rejected action is `act_ee0b736f0430e472e896a456ba3cb6eb`, linked to finding
`a17940552206e1ca` and investigation `955f4f47-0b6f-404b-b513-70a1d226f111`.
Each case passed independent teardown, second-cleanup no-op and restored
inventory. Per-case detection estimates were $0.012123, $0.012283 and $0.014915.
These exclude investigation calls and are not provider-account spend.

Storage remains unqualified. The model had the collected tmpfs mount and its
configured size. Its canonical-resource log call failed, the fallback host and
container log call succeeded, and its `df -h` command required approval. It then
promoted unrelated host/array warnings into a definite capacity diagnosis.
The scorecard's required terms and narrow forbidden phrases missed that false
claim. Its raw pass is retained as evidence of a qualification limitation, not
accepted as product success. The next storage slice needs canonical, authorized
filesystem-capacity evidence and explicit semantic review against the bounded
fault. Do not permit arbitrary commands merely to make that case pass, add a
benchmark-specific diagnosis rule, or treat identifier/phrase matches as proof
of causal correctness. Backup coverage and independent Pro environments remain
unqualified as well.

Real outcome review exposed stale durable records: the finding and investigation
could say `fix_verified` while the embedded product record still said
`fix_queued`. Action reconciliation now refreshes that record through the same
canonical builder used at investigation completion, preserving original model
prose, evidence and retained rollback. Read-time hydration repairs existing
records even when the top-level outcome already matches. Unchanged hydration
must not republish state or repeat outcome notifications. Resolved findings keep
their exact action-history link, and Assistant handoff preserves resolved status.
Investigation completion replaces an earlier partial action projection with its
final evidence. Subsequent action transitions preserve that completed evidence,
including impact and confidence that the current finding may no longer retain.
An intermediate proof build exposed that loss of retained impact. The regression
now preserves it, while already absent historical fields remain unassessed.
Final runtime and browser verification of these corrections is recorded below.

After qualification, both original development binaries were restored separately
because they differed: runtime `e5a2b60e52e35c37f68daa348c642757b56a64b40ca1d72f4db843ce69eb5db4`,
persistent `73c224dfd750c41b2cbd883c3ce7e352071862bc6a60e59fe6ed4dd3de312bc6`.
The original protected token was restored, temporary issued tokens were revoked
and checked absent, temporary backups were removed after comparison, and no
owned fault containers remained. The restored v6.2.0-rc.8 development agent
reported fresh telemetry. Its original token lacks command scope, so its command
connection is again absent by design. Production PID 752388 remained unchanged.

Final action-history proof uses Pro Darwin arm64 binary
`859d5d2de84cfd2264caa7dbcf5f080e3b1d06b5876df2c81dc0e00872f5e779`.
Worker proof passes the API action reconciliation selection and investigation,
record, rollback and early-projection completion regressions. The full API suite
passed in 310.353s before the final evidence-preservation refinement, followed
by the final targeted regressions. The three affected action component suites
pass 30 tests. The complete qualification package and pagination regressions
also pass. The exact final staged hook gates landing.

Playwright exercises `/patrol` Activity/All and both exact `/actions?action=...`
links above at 1440, 900 and 390 by 1000. Final-content checks cover resolved
record retention, outcome agreement, safety disclosure, completed/rejected
headers, planning-time copy, absent settled execution controls, independent
verification, policy/evidence/delivery disclosures, keyboard toggles, Escape,
close controls, deep-link reload, scroll fit and retained review focus. Actual
pixels were inspected at desktop, intermediate and phone sizes. Assistant
handoff opens the same finding with completed/rejected context and read-only
control. Provider readiness POST was deliberately blocked during that rendering
proof and no prompt was sent. Earlier browser attempts encountered an
intermittent bootstrap connection screen. The complete final matrix passed
after removing redundant immediate navigations from the proof driver, without
claiming a bootstrap fix. Source bindings are in
`frontend-modern/browser-verification.json`.

### Next implementation: resource filesystem observations

PR #1935 merged as `560dbf314c4fc3744f52aa4e5a6a204cafe3aa7d`.
Its API race suite and other CI checks passed. The log-level-parser benchmark
failed twice in CI, at +10.04 and +10.23 percent. Ten alternating exact-base worker samples did not
reproduce that regression (5.449 ns versus 5.357 ns, p=.436), with unchanged
parser source and unchanged thresholds. This is not a new product-readiness claim.
The repeated CI observation remains open. No further retry was requested.

The next slice is in progress and is not qualified. Its required work is:

1. Collect filesystem capacity, available blocks and finite inode inventory at
   the resource's actual mountpoints. Preserve observation time and native
   source. Keep configuration, image layers, filesystem capacity and resource
   quotas distinct. A failed read carries no numeric usage payload.
2. Bind native Linux reads to the exact inspected container process and its
   runtime cgroup, confine path resolution to the process root, and revalidate
   container identity after collection. Remote or unattested process namespaces
   remain unavailable. No container binary or general command permission is used.
3. Bound stalled filesystem reads without accumulating repeated kernel calls.
   Carry observations through report ingestion, snapshot cloning, the unified
   resource and both shared model query paths. Replace old observations on a
   later failed or absent report instead of presenting stale usage as current.
4. Review accumulated investigation instructions that prescribe restart or peer
   investigation. Preserve objective tool/proposal/permission contracts while
   leaving diagnosis and evidence selection to the model.
5. Qualify actual storage diagnosis and recovery against the bounded tmpfs
   oracle, then check healthy, dependency, missing-access and action regressions.
   Review the complete causal claims and recommendations, independently of the
   existing lexical score. Backup and independent-environment coverage remain
   separate open requirements.

Browser matrix for the final source and current build: at 1440, 900 and 390 by
1000, inspect `/patrol` finding details, investigation messages with measured
and unavailable filesystem evidence, Assistant handoff and its final diagnosis,
and the adjacent container details. Exercise loading/error presentation,
disclosures, keyboard controls, scrolling/overflow, dismissal/focus return and
reload. The approved/rejected action-history journey must remain intact. No
browser or real-model pass is claimed by this implementation plan.

Native observer race tests passed in 1.031s and the shared report tests in
1.010s on pulse-dev with Go 1.26.8. Test binaries cross-compile for Linux arm64,
Linux 386 and Darwin arm64. Targeted collection identity, report ingestion,
stale-observation replacement and both model-query projections passed.

The first real collector fixture ran as the unprivileged worker account. Docker
API access succeeded, but opening the root-owned process namespace failed with
permission denied. No numeric usage was emitted and independent cleanup passed.
The separate positive test ran the precompiled collector fixture as UID 0,
matching Tower's native development-agent privilege, without changing any
installed service or permissions. It measured the exact 8,388,608-byte tmpfs:
8,380,416 bytes available at baseline, zero under the independent pressure fault,
and 8,380,416 after recovery. Container health changed healthy/unhealthy/healthy.
The restored-baseline sample remained healthy with 8,372,224 bytes available.
Teardown, second cleanup no-op and inventory restoration all passed.

The native fixture binary SHA-256 was
`c275a530f332237b1c1fff06e31a0194929f9929edcba4aa09423da168869b1a`.
The full root-native log SHA-256 is
`83b62f1cab9b5a6d597447b672c964bee26bd9187801dd80b0069c1251ba3e5f`,
retained at workspace `tmp/patrol-filesystem-evidence/live/native-live.log`.
This establishes native collection on that worker privilege profile. It does
not establish an installed homelab agent, model diagnosis or wider deployment
support. The prompt change and final query description still need their final
regressions and all runtime/browser/model qualification above.

The first installed runtime attempt exposed two additional qualification defects.
The native observer was incorrectly gated by `CollectDiskMetrics`, although the
unified agent disables that flag to avoid expensive image-layer sizing. Native
mountpoint observations now run independently, with a regression for that exact
configuration. No installed-agent diagnosis pass is inferred from the earlier
collector-only proof.

The real backend also stalled agent reports and resource reads while the Patrol
attention page reconstructed alert history. A goroutine capture showed history
occupying the event store's only database connection, a durable lifecycle append
waiting for it while holding the alert manager lock, and monitoring/API reads
waiting for that lock. The existing query sorted full event snapshots before
selecting each page. The owning event store now separates read-only WAL readers
from its serialized writer and selects chronological page IDs before loading
snapshots. The actual database query plans show the former temporary sort and
the replacement time-index scan plus primary-key payload reads. The new disk
regression holds a read snapshot open while requiring a durable write to finish,
rejects a mutation through the reader and verifies committed data from a new
snapshot. Event-store race tests and alert history/recovery tests passed.

Run `q-20260906-220538-71a19f74` stopped at preflight because the runner had
retained monitor mode. The corrected approval-mode run
`q-20260906-220615-f15cef02` detected the intended unhealthy resource, but its
detection took 163.581 seconds during the database contention. It was cancelled
before investigation qualification completed. Cancellation interrupted the
recovery measurement, so no recovery pass is claimed. The independent final
teardown removed both owned containers and their network, the second cleanup
was a no-op and the original inventory was restored. Tower's original runtime,
persistent binary and token were restored, the temporary command token revoked
and its absence verified, and this transaction's backups/candidate removed.
The production agent retained PID 752388 throughout. The first browser attempts
reached the slow connection state and are failures, not final browser proof.
The native-agent wiring and event-store fixes still require fresh runtime,
model, browser and delivery qualification.

A diagnostic startup stack subsequently identified a distinct canonical resource
history scan. Incident catch-up called global `GetRecentChanges` while the
resource store lacked a leading observation-time index. That query occupied the
store connection while initial registry change emission waited. The actual
database has the canonical non-null `observed_at` schema. Its query plan showed
a table scan and temporary sort. The store now creates the missing global
chronological index, with an existing-database upgrade/query-plan/ordering test.
This is a second fix requiring a new backend build and live qualification.
Legacy timestamp fallback query plans remain outside this canonical-schema
proof and need separate migration qualification.

Run `q-20260906-224128-a59214cb` passed preflight and independently established
the disposable storage fault, then was cancelled during collection convergence.
The local hot-development watcher completed an earlier build and replaced the
backend during the run, despite the later verification lock. The replacement
used Go 1.27.1 and had SHA-256
`6eb9b4e30e7fa4a11d71accbfe9b9922133e1d885b5910aec0e267a54e37fd7f`,
which differs from the pinned worker artifact. This run reached no model turn
and supplies no diagnosis evidence. Both owned containers and their network
were removed, repeat cleanup was a no-op and the original inventory was restored.
Final qualification must wait for the pinned artifact and verify that no earlier
build is still able to replace it.

The second Tower transaction was fully rolled back: original development binary
and token restored, temporary command token revoked with absence verified, owned
backups/candidate removed, and production PID 752388 unchanged. The restored
development PID was 2398103. The rollback also persisted paused Patrol and monitor
autonomy through the authenticated API.

Review of the replacement history query found that its unconditional time-index
hint also forced retained-history scans for replay watermarks and individual
alerts. A read-only empty-tail probe of the actual event database returned no
rows in both cases, but the forced time scan took 0.14939 seconds versus 0.00006
seconds for a primary-key seek in the native SQLite observer. These timings are
a diagnostic comparison, not a Go runtime performance qualification. The owning
page query now seeks durable IDs for an explicit replay watermark and uses the
alert-specific chronological index for an alert bound. Full chronological walks
retain their time-index paging. An EXPLAIN regression uses the actual composed
query and rejects full retained-history scans for both filtered cases. The
queued backend build is superseded by this source change and requires fresh
event-store/history proof before a final artifact can be qualified.

The focused actual-query plan regression passed locally on Go 1.27.1 in 0.791
seconds. This was a single non-race test for quick feedback against the warm
development cache. It does not replace the pending pinned Go 1.26.8 worker race,
history and artifact qualification.

The final worker event-store race suite passed in 3.981 seconds and the targeted
alert-history/recovery checks passed in 1.274 seconds. The resulting Darwin Pro
artifact SHA-256 is
`bef2a3cce5a18f0d2c14badc4ba14f3ffffe454777ccd83f9b641a893b80ea38`.
After deployment, the running executable inode matched that artifact. The global
resource-history index was present. Authenticated preflight confirmed Gemini
3.8 Flash in all three selectors, read-only control and paused Patrol. Resource
listing took 0.647 seconds. The first attention summary took 21.265 seconds and
a repeat took 0.968 seconds. The first-read delay remains a measured startup
latency limit, not a qualified latency improvement.

The completion audit also confirmed that the existing Proxmox bulk lifecycle
evaluation denies approval and proves planning only. VM execution and independent
verification remain required by the candidate-lane contract. The explicitly
disposable FreeBSD agent-lab VM 110 on delly is stopped and available as a bounded
fixture. Pulse identifies it as `vm-e8cc8be82e584c58`, but reports the node command
agent disconnected. The existing separate development unit is inactive and
disabled, while the production unit is active. No VM lifecycle pass is claimed
from inventory, planning or the Docker action results.

Final browser review must also cover an expired unexecuted action: Patrol's
investigation, durable finding record and Assistant context must say needs
attention, while the linked action retains its precise expired state. Original
model prose remains retained evidence. An attached finding suppresses generic
empty-chat starters and unrelated recent sessions. Exercise attached and cleared
context, empty and existing conversations, close/reopen, keyboard focus, reload,
and expired/completed/rejected action links at 1440, 900 and 390 pixels wide.
The prior storage browser pass exposed both defects and does not qualify these
repairs.

Installed native filesystem case `q-20260906-232102-5316b904` passed the runner and
independent semantic review. The model queried the actual 8,388,608-byte tmpfs
with zero free/available bytes, then obtained ENOSPC logs and attributed the
unhealthy container to that mount. Its recommendation distinguished temporary
restart of ephemeral storage from long-term size/retention changes. The case
recorded a proposal only. Recovery proof came from independent filler removal,
which restored 8,347,648 available bytes and healthy status. The healthy neighbour
remained unaffected and teardown/inventory restoration passed. Detection took
31.121 seconds of model time, collection took 24.825 seconds, and the runner
recorded 107.490 seconds end-to-end. The scorecard's US$0.01331325 is Watch cost,
not a claim about complete investigation billing.

Fresh current-prompt cases `q-20260907-073028-5b5ab1f1` (dependency) and
`q-20260907-073212-807e363b` (healthy control) passed with cleanup. Semantic review
confirmed that investigation identified the stopped upstream dependency and
proposed starting that dependency, while healthy Watch produced no finding.
Approved action `q-20260907-073416-1d33864c` retained exact finding/investigation/
action and plan-hash linkage and reached independently verified healthy recovery.
The uncertain diagnostic prose is retained, so executed recovery is not presented
as proof that the model knew the original cause.

Rejected restart `q-20260907-073601-2d6b13ae` passed with an authoritative rejected
action, no execution and unchanged inventory after teardown. Tower transaction
r3 restored its exact original runtime and persistent binaries and token,
revoked the temporary command token and removed its backups. Production agent
PID 752388 remained unchanged.

Missing-command-access replay `q-20260907-074158-110e8868` safely detected the
unhealthy container, retained needs-attention and did not execute. It failed
the approved-remediation scenario, as expected for unavailable execution, and
is not a remediation pass. Investigation reported the absent agent and unknown
internal cause, but still captured a restart proposal that the canonical broker
subsequently refused. The stored submission error says no action was created.
This exposes a remaining contract gap: action availability needs to reach the
model before its conclusion, so it can explain the unavailable recovery path
without relying on a later orchestration failure. Do not describe this as a
qualified seamless missing-access journey. Fixture teardown and independent
safety/recovery oracles passed.

VM110 qualification on 2026-09-07 remains incomplete. The existing development
unit pointed to an old control-plane address and had an invalid token. Each
attempt restored the original binary/token and inactive, disabled unit. A
temporary runtime-only service override pointed the bounded test at this Mac,
and fresh command registration then passed. Read-only Assistant correctly
withheld control. Under temporarily enabled approval-required control, Gemini
created exact-target start plan `act_72dde1ef1b7b67884e66f6c4608faae8` and
explicitly said it had not executed. The first plan attempt returned SQLITE_BUSY
and the model retried successfully. The test then approved the canonical plan.
Execution returned HTTP500 with the durable action left executing and its
attempt receipt-pending. No lifecycle pass is claimed. VM110 was independently
confirmed stopped after cleanup, the original agent and read-only control were
restored, all temporary tokens and runtime overrides were removed, and production
agent PID1565 stayed unchanged. The owning durable action execution/reconciliation
contract needs investigation before another lifecycle qualification. This is
a product failure found by qualification, not evidence of successful recovery.

The Mac hot-dev verification lock from the previous session had expired before
the morning source edits. A local automatic build replaced the previous runtime
at 07:33:58 UTC. Morning Docker case receipts still prove their observed
behavior but must not be attributed to the earlier pinned Darwin artifact. A
fresh live lock and explicit worker artifact deployment now bind final browser
qualification. The last restart also exposed a multi-minute bootstrap delay.
Healthy request timings after startup do not qualify that bootstrap latency.

The stranded VM qualification action was closed through the canonical operator
force-fail endpoint after independent stopped-state and inactive-agent checks.
The audit retains an inconclusive failed outcome. It was not deleted, retried
or converted into successful execution.

Final browser artifact `e223f46accfc35102d671dd21793b5508bd64cd1dac4784c0d41c0f50b8f55b2`
contains native filesystem/history changes, expired-outcome hydration and
action-presence-based Patrol history, plus contextual Assistant starter
visibility. The pinned worker API regression passes, including expired
hydration without invented verification. Frontend type-check and 417 tests
covering Assistant, FindingsPanel and ApprovalSection pass. The artifact is
source-bound to the final runtime changes, while subsequent edits add tests and
qualification documentation only. Full staged-hook verification remains the
last landing check.

Final Playwright interaction matrices passed at 1440x1000, 900x1000 and
390x1000 on `/patrol` and exact expired/completed/rejected `/actions?action=...`
links. Pixel review covered settled dialog placement, nested tool-output
scrolling, native filesystem provenance/counters, retained action history,
independent verification, keyboard disclosures, reload, Escape and explicit
close. Attached empty Assistant context hides unrelated starters. New session
clears context and restores welcome/recent sessions. Loading an existing
session keeps the transcript accessible. Source hashes and exact routes are in
`frontend-modern/browser-verification.json`. Browser proof covers these named
changes, not whole-product readiness.

Existing-session inspection exposed another canonical orchestration residual:
`internal/ai/chat/agentic.go` replaces retained assistant prose with an internal
FSM verification instruction even though it withholds that instruction from
the live callback. A planned `pulse_control` action triggered this write gate
despite no execution. The next orchestration slice must use actual action
execution/verification facts and keep internal provider instructions out of
customer transcripts. This is not qualified by the context-visibility fix.

A subsequent live check on 2026-09-07 invalidated the earlier assumption that
history paging alone was sufficient. `/api/ai/patrol/attention/summary` exceeded
30 seconds. A goroutine capture showed 107 full-history walkers, with two reading
retained snapshot payloads and the others waiting for the bounded reader pool.
The same complete history fold was being repeated independently for every poll.
The current change shares a derived chronological fold at a durable event-ID
boundary, reads only its new tail, and rebuilds after retention, delayed older
events or store replacement. Tombstones and live overlays retain their original
semantics. New regression, rebuild and current-runtime browser proof are required
before this slice lands. The previous artifact and browser receipt remain
historical evidence rather than proof of this additional change.

The follow-up history change passed the full eventlog race suite (3.827 seconds),
focused history/projection/migration/parity race tests (4.048 seconds), and then
the complete alerts package race suite (23.785 seconds) on the non-root worker.
The replacement Darwin Pro artifact is
`1f1f71d2fd77b89f43a980d1e990a4010d2ec467502551555d5ab702f1aa71e6`.
Source hashes bind all six changed alerts files and the unchanged final frontend.

After installation, the first attention summary completed in 4.695 seconds.
Eighteen authenticated requests with at most six concurrent callers all returned
HTTP200. The first six took 4.457 to 4.514 seconds, the remaining twelve took
0.319 to 0.705 seconds, and the final stack check found zero history walkers.
This is bounded functional recovery evidence, not a performance benchmark.
The interactive Mac load was above five and wider load qualification remains a
worker responsibility. No fault, provider call or infrastructure action was
needed for this concurrency check.

Both complete Playwright scripts passed again against that replacement artifact:
`/patrol` storage evidence, expired action and Assistant attached/new/existing
states, plus completed and rejected action handoffs. Viewports were 1440x1000,
900x1000 and 390x1000. Current pixels were reviewed for native measurement output,
nested scrolling, action state, independent verification, drawer content, policy
expansion, reachable controls and dialog placement. Reload, keyboard expansion,
Escape and close checks passed. The current browser receipt supersedes the earlier
artifact binding for this slice. Missing-access continuity, VM dispatch and
Assistant orchestration failures remain separate open qualification defects.

### Typed Proxmox runner qualification, 2026-09-07

The previous filesystem/history slice was committed as
`3a4a3fd62bb8d36660f6f0756ed8b58338f39d83` and pushed to PR #1951 with
auto-merge enabled. Exact staged pre-commit and pre-push hooks passed on the
non-root worker against tree `90e4b6e3e59d6bcd514f0f5af1ff3d69bc8adb8a`.
All required checks passed and PR #1951 merged as
`09ab5c2d0ae6e02fcc5280853782a8142646e40f`. The advisory benchmark reported
four NormalizeSegment microbenchmark regressions against unchanged source.
That observation is not a failure of the required feature qualification.

The VM110 execution error was recovered from the backend log: the server
rejected the generic command session because Proxmox guest lifecycle requires
a typed action runner. This is separate from the earlier SQLite plan-write
failure. Planning and binding previously accepted generic connection presence,
while the real execution path checked the credential-bound role. Existing
Proxmox API tests exercised a shell fallback instead of that real typed path.

The current source resolves a unique, currently admitted typed runner for the
owning node and tenant. It rejects legacy, revoked, pending, fenced and
ambiguous sessions and requires durable receipt support before plan persistence.
The dispatch guard checks the actual selected connection. The API shell
fallback is removed and its tests use typed payloads. Mutation completion no
longer substitutes for readback evidence, and a status-only reboot read remains
inconclusive without independent uptime evidence. The full agentexec race suite
and focused Proxmox API race cases pass on the worker (9.062 and 3.295 seconds).

The rebuilt development server artifact is
`b4aa4f9c47e3740ac6c409e91d9ab84beb25c0dff3b8641ce04872929df58004`, with
native Linux runner
`42ab7aa4f7c0c2f825586d5ae536254b8282924cb056cd4d65fce53a19c43b44`.
The live VM110 qualification is in progress. Two fixture setup attempts stopped
before runner registration: an executable placed on Delly's noexec `/run`, then
a temporary service name that did not satisfy the canonical
`pulse-agent-runner.service` containment dependency. Both attempts restored the
stopped VM, revoked the temporary credential and removed the private tunnel.
These are test setup failures, not successful execution proofs. Runtime
restrictions were retained. No production agent was changed.

With the canonical service name, the runner passed containment and WebSocket
registration, then activation returned HTTP403. A single isolated diagnostic
request recovered the exact reason: `Action runner bearer credential required`.
The managed development runtime enables admin bypass, which returned a generic
admin context before extracting the supplied bearer token. The exact-token
activation endpoint correctly refused that missing identity. The shared auth
path now preserves normal validation and scopes whenever explicit credentials
are present. This does not weaken activation or change production auth mode.
The third fixture and diagnostic credentials were revoked and all temporary
services and tunnels removed. Final rebuilt qualification remains pending.

The auth regression group passed with race detection (7.863 seconds), including
explicit-token development bypass, runner issuance/activation and Basic auth.
Six direct Basic-auth fixtures now reset their shared lockout state. The two
successful-auth fixtures also initialize and close their own session store.
Earlier grouped failures came from process-global lockout contamination, and
isolated successful-auth cases exposed missing session initialization. Runtime
lockout behavior was not changed.

Artifact `c1233ccf5e1fc7872f9e30c1e372019d8037468603ad92fdd62f0d0d9882fa0b`
registered and activated the exact runner credential successfully. Restarting
the dev server had removed the previous live monitored-host identity, so the
qualification fixture now starts a separate monitoring-only collector with a
fresh report and report/config scopes, then issues the separate action-runner
credential. Stale retained resource metadata does not establish that prerequisite.
Both temporary services and credentials are removed at cleanup.

The first full typed run produced Gemini plan
`act_9c97005e3eca1a957bb172e7d123fdda` in session
`c6627f40-8087-4d70-bc79-52cfd86560fb` for exactly VM110 on Delly. It consumed
51,259 input and 861 output tokens and reported $0.041673. Canonical approval
and execution completed the start, with an independent Proxmox API observation
of running state and a separate SSH `qm status` confirmation. The restoration
stop `act_2ab4ded219d68854368b445f08234939` likewise completed and independently
confirmed stopped state. All temporary services, credentials and tunnels were
removed, the original production agent PID1565 was retained and control returned
to read-only.

That run exposed a shared evidence label saying `Agent observed` for independent
Proxmox API evidence. The shared decision packet now uses `Observed` while
retaining the named observer and separate receipt time. Nine existing decision
packet tests and the worker production frontend build passed. The replacement
bundled artifact is
`7504f38902ce90edd9dbf961a78c2ab4fc7a21ac232e9447c984ff89ca79c525`.
A final repeated lifecycle and browser matrix is in progress. The earlier stop
browser pass was interrupted by login rate limiting because the fixture signed
in per viewport. Browser contexts now reuse their authenticated session, with
no product auth or rate-limit relaxation. The redundant read/verification turn
following a planned Assistant control remains an open orchestration defect.

The final-label repeat invalidated a claim of reliable repeated VM execution.
Plan `act_c59346a6cfcddaee4e70f0870e2ec16e` was approved, but result projection
returned HTTP500 with `normalize Proxmox agent readback evidence: invalid action
evidence: receivedAt predates observedAt`. The durable audit remained executing
with a receipt-pending attempt. The fixture cleanup restored stopped state and
removed both temporary services, credentials and tunnel. The exact action was
then force-failed with inconclusive execution truth through the operator API.
It was not retried or relabelled successful.

A bounded SSH time sample measured Delly between 44.95 and 205.19 milliseconds
ahead of the Mac, without changing either clock. The shared evidence normalizer
incorrectly required receiver wall time to follow observer wall time across
machines. Canonical evidence now preserves both original UTC timestamps and
binds them into the digest. Capability-specific freshness checks remain required.
Docker and host-update projectors no longer clamp a valid observation to receipt
time. The Proxmox path captures the actual receipt boundary before its independent
observation and bounds agent readback freshness separately. Stale or excessive
clock skew leaves verification inconclusive without discarding completed execution.
The final shared action-result/evidence race selection passed in 2.339 seconds.
The final API cross-clock, typed Proxmox, Docker/Proxmox contract and explicit
authentication selection passed with race detection in 9.287 seconds on
`pulse-dev`. This followed the independent release preflight through the normal
allocator, without a Mac heavy-test fallback or lock bypass.

The final enterprise development binary has SHA256
`c53f2e54e7ea510fc3abfbfbe164e74e363d3b1560aaa36a62df9fb1aaaf2c7d`.
Its source manifest matches the final runtime and frontend files. Gemini session
`22bcd9ee-52ba-4733-8b6c-5f1e8d2bb32f` prepared one exact VM110 start plan,
using 88,539 input tokens and 1,106 output tokens at $0.07055175. Start action
`act_9ec8d575b777a5f8fc70561376a52381` and restoration stop action
`act_3384576233bf55aa3182537ad02d81d4` both completed with independently
confirmed Proxmox state. Their retained native receipts are terminal, report
completed mutation and preserve original running/stopped readback timestamps.

Playwright passed pending and completed review states for both actions at
1440x1000, 900x1000 and 390x1000. Final pixels were inspected, including expanded
independent evidence, keyboard focus, nested scrolling and reachable footers.
Keyboard disclosures, Escape, explicit close, exact-link reopen and reload all
passed. Independent evidence now says `Observed`, while the delivery record's
agent-specific timestamp retains its accurate label. The same final build passed
retained completed, rejected and expired Docker action review smoke at all three
widths. Approvals and execution used the exact plan hashes through the canonical
API. Browser writes were limited to login.

Cleanup restored stopped VM110, preserved production agent PID1565, removed both
temporary services and their private directories, revoked both credentials,
closed the tunnel and restored read-only control. A fresh plan then returned
HTTP409 `action_runner_unavailable`, matching stopped-resource readiness.
Worker receipts are in workspace `tmp/patrol-runner-readiness/runtime/clock-final-build/`.
Live, native receipt and browser artifacts are in
`tmp/patrol-runner-readiness/{vm-transaction.json,browser/}`. This qualifies the
named typed runner and clock-fix slice, not the full redesign or autonomous modes.
The action-review regression fixture now includes a real independent observation
with distinct observer and receiver clocks, rather than an empty evidence list.

### Next canonical planning boundary

The remaining missing-access defect is broader than an absent prompt hint.
`ProposalCatalog` returns static capability definitions. `ProposalCapture`
validates and holds a proposal, but the actual broker plans it only after the
model has finished. `ProposalCapture.Outcome` also converts any unsuccessful
proposal attempt followed by a no-action conclusion into an investigation error.
Those rules prevent the model from incorporating canonical planning refusal and
can override a valid decision to stop. Assistant's separate verification FSM
then mistakes its own successful plan submission for executed infrastructure.

The next implementation should put canonical planning results into the model's
actual tool turn. Separate broker planning from policy-authorized execution,
using the existing action lifecycle for both. A proposal tool should return the
persisted plan reference or exact planning refusal before the model concludes.
Current availability belongs beside the capability schema, with the canonical
planner rechecking it. Keep the trusted finding/investigation/proposal identity,
non-sensitive parameters, tenant authority and idempotency. Canonical action
identity must replace the duplicate request-local proposal fingerprint/state
machine. A later provider error must not erase an already persisted plan.
Policy-authorized execution remains a separate lifecycle transition with its
current authority recheck, and its eventual outcome is reconciled through the
same linked action. The investigation profile must not acquire unrestricted
command execution or approval authority.

Remove the generic Assistant resolve/write/verify policy machine rather than
reclassifying protected controls as reads. Registered invocation permissions and
the canonical action lifecycle already enforce the real authority boundaries.
Preserve actual assistant prose and tool results in the transcript. Model-owned
investigation can select further evidence without a tool-count proxy forcing
another turn or manufacturing verification.

The final-label repeat provides direct evidence for this change. Session
`c601dcaf-2666-4357-95a9-14149ae84e2f` first correctly explained that VM110's
start plan awaited approval. The harness then forced another model turn to
verify a write that had not executed. The model queried the stopped VM and its
history, adding a second conclusion about the unchanged state. Remove the
semantic lifecycle-request gate and first-tool-before-question counters in the
same orchestration review. Tool availability, current evidence and the user's
request should inform model judgment. Keep explicit resource identity,
permission checks, approved action binding and configured spend limits as
mechanical boundaries.

Required proof includes live missing-runner refusal visible before the final
conclusion, unavailable and healthy no-action conclusions without false failure,
a capability becoming unavailable between lookup and planning, repeated same
plan identity without duplicate action, provider failure after a persisted plan,
unchanged approval/control/tenant restrictions, retained original assistant prose,
and an approved or rejected plan continuing through the shared action history.
This is the next implementation plan, not a claim that those changes exist.

Qualification must distinguish an expected planning or access refusal from a
defective tool result. The current scorer fails every nonzero failed-tool count,
and its summary-term, resource-name and evidence-count checks do not establish
semantic diagnosis accuracy. Add scenario-owned expected-refusal proof without
waiving unexpected errors. Retain semantic review against independent ground
truth, as the earlier storage false diagnosis demonstrated. Population-level
diagnosis, false-alarm, missed-problem and end-to-end latency rates remain unknown
from adoption and outcome buckets alone.

### Canonical planning and model-owned continuation, 2026-09-07

This is the active implementation slice after the typed-runner handoff. The
runner and clock-evidence change is committed as `2a7019b0fa02587446e55af3838603d1c6924dba`
in PR #1955, merged as `62f6931c1fc2e46511876139ea20904752ea208c`
on 2026-09-07 at 11:29 UTC. Its worker, disposable VM and browser proof is
complete, and the local default branch includes the merge. Its historical proof
does not qualify the changes below.

Required boundaries (the independent continuation removal is being qualified
first, followed by canonical planning and linked progression):

1. Expose current executor-owned readiness beside canonical capability schemas.
   Recheck admission inside the canonical planner. Unknown readiness stays
   unknown and no catalog lookup grants execution authority.
2. Persist a canonical action plan during the proposal tool call and return its
   exact identity or refusal to the model. Remove the request-local proposal
   fingerprint, ambiguity and failed-attempt judgment machine. Keep trusted
   finding/investigation identity, sensitive-parameter refusal and canonical
   idempotency. Classify plan persistence as a Pulse-state write, with only that
   named write permitted to the investigation profile.
3. Keep policy-authorized progression separate from planning. Continue the same
   action reference through investigation, Assistant, decision and independently
   verified outcome. A provider failure cannot erase a persisted action or
   convert it into executed recovery.
4. Remove generic resolve/write/verify gating, semantic lifecycle-request
   detection and first-tool-before-question counters. Remove forced completion
   based on proposal acceptance, successful writes or failed-tool counts.
   Preserve explicit run/evidence/spend budgets, cancellation, invocation policy,
   canonical target binding and independent action verification.
5. Qualify expected refusals against scenario-owned truth without permitting
   unexpected failures. Mechanical keyword/count checks are not semantic
   diagnosis proof. Run affected regression and race suites on the worker, then
   inspect actual real-model conclusions and independent lab outcomes.

The final-build interaction matrix covers `/patrol`, contextual Assistant and
exact `/actions?action=...` links at 1440x1000, 900x1000 and 390x1000. Exercise
unknown cause and unavailable execution, pending approval, rejected and completed
actions, retained provider failure, attached context, new/existing sessions,
expanded tool evidence, reload, keyboard focus, Escape, outside dismissal,
scrolling and focus return. Inspect pixels and the persisted transcript after
the last source change. Record model, source hashes, permission posture, known
faults and healthy controls, latency, cost limits, action IDs and cleanup.

Required negative regressions include readiness loss between lookup and plan,
same-plan replay without a duplicate action, conflicting requests without
erasing an existing plan, provider failure after persistence, a valid no-action
conclusion after refusal, and unchanged tenant/control/approval restrictions.
Live qualification must include healthy, unhealthy, dependency, missing-access,
storage-or-backup, approved and rejected cases. Existing passes are historical
until their affected contracts are requalified. The subscription refusal remains
untouched. Wider rollout still requires independent volunteered environments.

Measurement interpretation is explicit: the previously recorded 127 paid
installations, 71 Patrol-enabled installations and 23 Assistant users measure
adoption. Fourteen verified resolutions from one installation do not establish
a population success rate. Useful-diagnosis, false-alarm, missed-problem and
end-to-end latency population baselines are unknown. Named lab observations
and reviewed action postconditions provide local evidence with their own
denominators, never substitutes for those missing population measurements.

### Continuation removal in progress, 2026-09-07

The first independent source slice removes the generic Assistant workflow
state machine, semantic lifecycle-request correction, first-read-before-question
counter, repeated-call counter and three-error forced stop. It also removes
inferred self-correction counters. Explicit budgets, cancellation, canonical
invocation/target policy and approval/execution verification are retained.
Preparing a plan leaves investigation tools available and never creates a
synthetic verification episode. Streamed prose is preserved in saved history.

The first worker compile exposed three leftover references in tests and one
unused local variable. These were repaired. The next package run was explicitly
aborted after its old interaction corpus waited for an unanswered first-turn
question. Its stack confirmed `executeQuestionTool`, not a runtime deadlock.
That scenario now supplies an answer through `Service.AnswerQuestion` and has a
10-second context deadline. Neither failed attempt counts as qualification.
The fresh chat/tools package run, current-build real-model proof and browser
matrix are still pending. Canonical planning in the tool turn and the remaining
live scenario matrix are not implemented by this slice.

Preliminary real-model evidence from the first continuation build is not final
qualification. Gemini session `3a64ec04-fdd4-491c-a160-eab905554c02` selected the
canonical VM, attempted planning, received the actual missing-runner refusal and
explained it without claiming execution. The run took 7.813 seconds and cost
$0.017526 (21,618 input and 350 output tokens). A proof-script session read used
the wrong URL suffix, then recovered `/messages` without repeating inference.

The disposable VM plan in session `f778f25e-ddce-4b35-9563-b791134cf26d` took
9.552 seconds and remained pending approval without a synthetic verification
turn. Its prose nevertheless implied that approval would automatically execute
the admin-class VM action. Actual Actions UI requires a separate Run for this
class. The typed control result only said Pulse owned the remaining workflow.
It now returns the complete canonical plan, an exact action URL and explicit
`execution_requested: false`, with factual separation of approval, execution
and independently recorded outcome. This is a tool-context correction, not a
harness rule judging or rewriting the model's answer.

The first rejected-state browser check also failed because the local embedded
frontend directory copied into that build predated the current frontend source.
The browser showed the rejected action in History but lacked the current review
header. Rebuild embedded assets from the exported source before repeating the
entire affected matrix. This packaging failure is not counted as a UI pass.
Action `act_268983bb9d1f1705c9fd871419ddcf73` was rejected without execution,
VM110 stayed stopped, both temporary services and tokens were removed, and
read-only control was restored. Final-source proof remains pending.

The second bundled build bound 4,843 source/module/frontend files and passed
chat/tools race suites plus action lifecycle regression. Its missing-runner
session `bd322787-3c1d-450b-82c1-7129e3ac0298` explained the refusal in 8.795
seconds. Saved-history browser inspection found another real semantic defect:
the canonical plan call was labelled `run command`, and backend progress called
it execution. The shared tool presentation and live progress now explicitly
identify preparation of an action plan. Permission classification is unchanged.
This source change requires fresh build and browser proof before landing.

Two browser-script assumptions were also corrected without changing the
product: Assistant is explicitly reopened after reload, and the full-width
mobile panel is dismissed through its close control or Escape because no
backdrop pixels are exposed. Desktop backdrop dismissal and keyboard focus
return remain applicable checks. Script failures do not count as passes.

#### Rejected local qualification, final continuation source r4

Binary `ed9aba14161fba582ab39ec129969f8ffb49a84d49bf0f48648736e80a713dd8`
matched 4,843 source/module/frontend files. Worker chat race tests passed in
13.757 seconds, both tool-presentation files passed 62 tests and the frontend
build passed. An earlier repeated build was rejected because a transfer put six
changed files under a nested directory. Byte comparisons found that mismatch
before installation, and the duplicate files were removed after correct transfer.

Missing-runner session `dbf9e9f2-e923-46de-aef9-b3a44aef4fb2` took 12.763
seconds, returned the actual planning refusal and created no action. Saved
history at `/patrol` rendered the plan attempt accurately at 1440x1000,
900x1000 and 390x1000. Expanded evidence, keyboard opening, Escape/focus return,
reopening, reload and applicable backdrop/close dismissal passed.

Session `e29a3169-475d-46d9-ac4a-2f7547cb1cc1` prepared action
`act_3df7160a0611c2628fbbeca8099b681e` in 8.563 seconds. Its rejection caused
no execution, independently checked through Proxmox. Assistant explained the
rejection in 5.861 seconds and created no further action. When explicitly asked
again, it prepared `act_b8a46526da0b24adf6c0c6af195e88cb` in 5.158 seconds.
The latter was approved, executed and independently verified running. Actions
review passed pending, rejected and completed states at all three widths,
including expanded policy, observer and delivery details, dismissal and reload.

The Assistant continuation **failed**. Asked for the completed action's outcome,
it queried cached resource inventory and the resource timeline, then stated
that the action was never approved or executed and the VM had remained offline.
The canonical action record and independent Proxmox observer contradicted that
claim. The absence of a current action-audit read capability was a real shared
evidence-access gap. Passing execution and browser assertions did not qualify
that model conclusion. The response took 7.992 seconds.

The next source revision adds `pulse_query action=action` with an exact
`action_id`. It reads the tenant-pinned canonical audit, retaining full plan
risk/context and canonical `ActionResultV2` observation provenance, while
excluding request parameters, credential bindings and raw driver output. Tool
context names the difference between recorded action outcome, inventory and
incomplete resource history. The model owns whether and how to investigate
those facts. Generic progress no longer infers infrastructure execution from
a write classification. Requalify the entire affected continuation matrix.

Restoration action `act_be3a853728b4e872ea44b7f179ebe22f` independently
verified VM110 stopped. Both temporary services, credentials and the reverse
SSH tunnel were removed. Read-only control was restored and the production
agent was unchanged. Receipts and failed transcripts remain in workspace
`tmp/patrol-planning-continuation/vm-transaction-r4-outcome-access-failure.json`.

#### Action outcome access qualification, r5

Binary `6854b7155caafd289cd7f2ff1ad975f952e2f9204e870f90c0793c0dc5f1a65a`
matched 4,845 source/module/frontend files. Chat race proof passed in 13.848
seconds. The tools race suite passed in 62.414 seconds after a new fixture was
corrected to include its required actor identity. Final action-read regression
passed after the recorded-decision projection was added. Both presentation
files passed 63 tests, and the frontend build passed.

The previously failed session read the canonical completed action and its
independent observer evidence correctly in 6.972 seconds, without creating an
action. A fresh missing-runner session
`211cbaa5-fbd3-49b5-b622-8d7f416b35d2` returned the actual refusal in 11.787
seconds. Fresh VM session `4bb71619-a39d-475f-ad2e-5996417cc3c6` took 12.074
seconds to prepare `act_43dd318fa8577d79167a6f66b2460907`, 8.799 seconds to
explain its recorded rejection, 7.712 seconds to prepare the explicitly requested
replacement `act_9bdd65533a8c0d1e526845012a931b10`, and 5.965 seconds to explain
its approved, executed and independently confirmed running outcome. The latter
two explanation turns chose the canonical action query. Neither explanation
created another plan. These are five named successful model turns plus one
retrospective outcome read, not population success or latency estimates.

Restoration `act_31ea9abb7591c83f466055a4eacd5abe` independently confirmed
VM110 stopped. Both fixture services and tokens, the reverse tunnel and the
temporary control-level change were restored. This proves local execution and
model continuation for this bounded VM fixture only.

The saved-history browser pass exposed clipped inline action URLs at 390 pixels.
The shared Assistant markdown styling now wraps inline code, and canonical plan
and outcome tool cards expose a native `Review action` link derived from their
bound action ID rather than a model-authored destination. Opening it closes the
Assistant overlay before Actions review. This frontend-only correction requires
fresh browser qualification. It does not change the already checked model or
backend source. Existing sessions remain available through Recent Assistant
sessions after reload. The browser receipt must explicitly resume and re-read
that saved session, rather than count the transient empty bootstrap as a pass.

#### Final Assistant presentation proof, r6

The final bundled binary is
`db840916eb4adec3c8c0d916bda4dd7d7ce7588ed5fc9c4be4ba9f8d1a1584d4`.
Its manifest binds 4,845 source/module/frontend files. Compared with r5, only
`MessageItem.tsx`, `ToolExecutionBlock.tsx` and `toolPresentation.ts` changed.
Model-facing Go source is byte-identical to the r5 real-model and disposable-VM
proof. Final targeted frontend verification passed 292 tests in four files,
and the final bundled frontend build passed. A lint attempt against a plain
source export failed because the planning-docs check requires Git. It is not a
passing hook receipt. Landing hooks must run against the exact staged tree in
a real worker checkout.

At `/patrol`, the missing-runner and verified-VM sessions were reopened from
Recent Assistant sessions at 1440x1000, 900x1000 and 390x1000. The browser
verified the exact session read on initial resume and again after reload,
expanded every tool card, scrolled the saved conversation and inspected actual
pixels. Inline action identifiers and URLs now wrap within the mobile message
area. Keyboard launch, Escape and focus return, reopening, desktop backdrop
dismissal and mobile close dismissal passed. The verified-VM native `Review
action` link closed Assistant and opened the exact completed action review at
all three widths. The browser waits for the dialog opening animation before
capturing its pixels. Earlier captures taken mid-animation are not evidence of
a stable rendered state.

The script and screenshot receipts are retained under workspace
`tmp/patrol-planning-continuation/assistant-browser`, with labels
`no-runner-r6` and `verified-vm-r6`. The temporary VM fixture remains stopped,
its credentials and services removed, and development control remains read-only.
This closes the affected Assistant continuation and presentation slice only.
Patrol canonical planning during the model turn, proposal-capture retirement,
request identity binding and the complete fresh diagnostic scenario matrix
remain required work. The subscription-provider refusal remains preserved.
No population reliability estimate or production-wide readiness follows from
this one-maintainer fixture.

Final r6 action-review regression also passed the rejected start, completed
start and completed restoration-stop deep links at all three widths. Policy,
independent observer and delivery disclosures, nested scrolling, keyboard
activation, Escape, explicit close and persisted reload were exercised.

The first exact-tree landing hook rejected stale public architecture claims
about the removed look-before-asking counter. Both Assistant architecture pages
and their shipped mirrors now describe the canonical permission and evidence
boundaries. The corresponding drift test retains the real tool-kind and
concurrency contracts and removes the retired state-machine checks. Playwright
opened `/docs/ASSISTANT_SAFETY` and `/docs/ASSISTANT_ARCHITECTURE` against the
current Vite build at 1440x1000 and 390x1000, checked linked navigation, reload,
end-of-document scrolling, actual pixels and absence of horizontal overflow.
Receipts are in `tmp/patrol-planning-continuation/docs-browser`. The pinned
worker formatter also restored indentation in an unchanged preflight helper.
That formatting-only difference does not change the qualified runtime behavior.

### Canonical request identity follow-through, 2026-09-07

The Assistant continuation/evidence slice is committed as
`e37353f937cbd665ab5f593843f67ed67dbc5748` and submitted in PR #1957 with automatic
merge requested. Its exact staged tree
`07e84669e69de86f98fcbdba5866f311ddf194bf` passed the full worker pre-commit hook.
The preceding pre-push lint/typecheck receipt covers identical runtime source,
with only the final qualification prose and browser receipt added afterwards.
Heavy hooks ran on the worker, and the Mac commit/push reused that proof rather
than starting the forbidden local workload. Merge state must be checked before
claiming the change is on remote main.

The next shared change binds each nonempty action request ID to the trusted
actor and the first persisted canonical plan. Current action IDs include mutable
resource/policy snapshots, so matching the action ID alone permits one request
to create another plan after drift. The proposed root fix checks request identity
in the canonical store, atomically with creation, without adding a diagnostic
state machine. SQLite obtains its writer lock before checking competing accepted
requests and rolls back tentative replay rows. Original plan, expiry, decisions
and outcome remain authoritative. Conflicting intent or origin refuses the
replay, and an explicit new request ID is required for a new plan. Unbound legacy
records retain their prior action-ID semantics. Tests must cover concurrent
store instances, reopen, changed intent, actor isolation and replay without a
live resource registry. This work is in progress and is not yet qualified.

The store proof also covers distinct inputs that collide only after redaction.
Those are not treated as matching intent. If the persisted request has lost
information to redaction, exact replay equality remains unknown and refuses,
without creating a replacement or deleting the original action. This is an
explicit limit. Normal non-sensitive replay remains deterministic.

Assistant planning now derives its request identity from the trusted saved user
turn and provider invocation ID. Replaying that invocation retains its request
identity, while a new user turn can request a new action. Calls without a bound
invocation identity retain fresh-request semantics. The adapter no longer makes
a separate capability-admission decision before the canonical planner, which
owns both persisted replay and admission of a new plan. These additional runtime
changes require final-source regression and live qualification before landing.

Before live qualification of this request/planning slice, exercise:

- First canonical plan and its exact pending-approval review.
- Same actor/request after resource or policy drift, retaining original plan,
  expiry, decisions and outcome without a second action or execution.
- Changed intent under the same request, returning the stable HTTP 409
  `action_request_conflict`, with original history unchanged.
- An explicitly new request and a new Assistant user turn, each able to create
  its own plan under current admission policy.
- Rejected and independently completed actions reopened by exact ID, including
  Assistant continuation, persisted session reload and native review links.
- Desktop, intermediate and narrow layouts, expanded policy/observer/delivery
  details, keyboard activation, Escape, dismissal, focus return and scrolling.

The core broker now has a separate plan-only entrypoint, sharing the canonical
planning implementation while omitting policy progression. Wiring that entrypoint
into model-visible investigation turns, removing proposal capture as an outcome
channel, retaining persisted action references across provider failure and
repeating the full diagnostic matrix remain unfinished. This addition alone is
not the completed Patrol planning migration.


### Canonical planning implementation in progress, 2026-09-07 13:50 UTC

The next source slice replaces the investigation proposal capture state machine
with a core-owned plan-only callback. The tool returns the persisted canonical
plan and action identity during the model turn. Planning no longer forces a
prose-only next turn. Later conflicting calls preserve the accepted action.
Completed tool observations are attached at planning time and later reads do not
rewrite that accepted origin. Provider failure retains the known action in the
investigation and does not request policy progression. These changes are not yet
qualified or landed. Tests that asserted proposal ambiguity erased history are
being replaced with preservation, concurrency and refusal-continuation tests.

The request identity r16 targeted race proof passed in actionlifecycle,
unifiedresources, agentcapabilities and API. The full actionlifecycle race suite
passed in 6.940 seconds. The full API race command reached its 600-second total
package timeout with the named security test only just starting. This is not a
passing full-package receipt. An isolated security-test run and the r17 invocation
identity proof are queued through the shared heavy-work allocator behind release
preflight. No allocator lock or provider refusal has been bypassed.

Remaining executable steps: compile and run changed core and enterprise boundary
regressions, repair any failures, build a source-bound development artifact on the
worker, repeat the documented action interaction matrix at desktop/intermediate/
mobile widths, run the unhealthy/healthy/dependency/missing-access/storage and
approved/rejected real-model disposable-lab scenarios, retain exact results and
cleanup receipts, run landing hooks and land scoped changes. The local completion
gate remains open. Wider customer rollout remains separately unqualified.

### Qualification export correction, 2026-09-07 14:10 UTC

The first enterprise r18 compile used a stale shared core export. Its setup
replaced the scratch export root while a core proof was queued. That compile
also ran outside the required heavy-work allocator and failed before tests.
It is not qualification evidence. The stale queued proofs and a duplicate
isolated API invocation were cancelled. The original isolated API test remains
queued. No release-preflight process was interrupted.

The corrected r19 commands use the shared allocator, the non-root maintainer
identity and checksum preconditions for 33 core source/document paths and three
enterprise files. Source transfer and formatting completed before queueing.
These checks bind the forthcoming receipts to the tested content, but they do
not replace the still-required live model, disposable-lab and browser proof.

The canonical sensitive-parameter guard now runs in the action lifecycle after
request replay lookup, so an accepted plan can still be read when the live
capability registry becomes unavailable. The Patrol broker sets that guard from
trusted core context. Missing request identity refuses before any action is
created. Model-supplied sensitive values remain prohibited.


### Canonical planning regression status, 2026-09-07 14:24 UTC

The r21 checksum-bound targeted race command passed actionlifecycle, unified
resources, tools, chat, API, agent capabilities and public contracts. The full
enterprise investigation race suite passed in 1.135 seconds. The previously
named API security test passed in the targeted API command (5.240 seconds for
the complete selected API set). This does not retroactively pass the earlier
600-second full API package run.

The r22 full chat, agent-capability and public-contract race suites passed.
Two tools tests still depended on the retired adapter-level capability veto.
They now exercise the real canonical lifecycle for VMware refusal. The r23 full
tools race suite passed, and a Go 1.26.8 Darwin development build succeeded with
SHA256 c00d6557fa8d5e5637d315e09e4535c3da0d5245cc2f9d6bce903fbd532cf15c.
That artifact has not been installed or live-qualified. A final approval-context
projection repair was identified afterwards and requires the next build.

The proposal-attempt counter, ambiguity/integrity outcome state machine and
its separate error transport have been removed from the primary contracts.
A planning refusal is an ordinary tool result. A persisted action remains in
history after a later refusal or provider failure. Normal provider failures
remain errors and cannot become completed diagnosis outcomes.

Delivery PR 1957 is still pending. Its benchmark check reported five route
normalization microbenchmarks above the configured threshold. A paired exact
base/candidate worker reproduction reproduced three slowdowns despite unchanged
normalization source. The compiled-code comparison remains under investigation.
No failed check has been overridden. Required live scenario and Playwright
qualification for the current planning source remain unperformed.


### 2026-09-07 current planning proof and qualification handoff

The r24/r25 targeted approval projection and complete enterprise investigation
race suites passed on Go 1.26.8. Source binding found three stale export files
before live installation: the browser receipt, static Safety document and a
repository documentation test. These were corrected explicitly. No live proof
is claimed for either intermediate artifact.

The r26 qualification package race suite passed in 7.983 seconds and the full
enterprise investigation race suite passed in 1.133 seconds. The qualification
scorer now retains failed tool calls as telemetry without treating any refusal
as an automatic diagnosis failure. Scenario-owned truth and independent action
oracles remain required. Existing keyword checks are mechanical transcript
checks, not semantic evidence of useful diagnosis or recommendation safety.
Each new live result still requires review against the independent fixture
observations and exact action/outcome records.

The task-specific r26 browser interaction matrix is retained at workspace
`tmp/patrol-planning-continuation/live-r26/interaction-matrix.md`. It covers
Assistant history, exact action continuation, pending/rejected/completed action
states, evidence expansions, keyboard dismissal and focus return, reload, and
1440/900/390 widths. Required current-source real-model and browser qualification
remains unperformed at this entry. The subscription-provider refusal remains
preserved. Wider independent-customer rollout evidence is a separate open gate.


The installed r26 binary is
`0e390e8f8362bcc7dc10b51ee5eb5cf6a2232b2f736b38ab681eae7162838a78`.
All 4,850 source/module/frontend file hashes matched its worker export before
installation. Runtime reports Go 1.26.8 and version
`0.0.0-dev-pro+patrol-planning-r26`. Its generic version endpoint still reports
release/stable metadata, so artifact SHA and source manifest are the proof
identity, not those generic labels.

Real Gemini missing-runner session `6fe14195-e9dd-416e-9e96-3bb500359cf4`
completed in 11.636 seconds and correctly reported the unavailable host typed
runner. It made three planning attempts, including an unnecessary numeric-ID
lookup miss. No action succeeded and no guest-agent prerequisite was invented.
The inventory tool exposed an unavailable disk percentage as -100 for the stopped
VM. That value did not drive the conclusion, but remains an explicit metric
projection defect to resolve before claiming general diagnostic data quality.

Real Gemini session `83407929-bec4-493e-99ce-20295725ca86` prepared rejected
start `act_68699d9b3caed37f85faaaaf9c70ff3e` in 9.578 seconds and explained its
unexecuted terminal result in 7.499 seconds. A distinct approved start
`act_42404c31949a96963aa3e6794bace969` was prepared in 5.867 seconds. Pulse
independently observed the VM running through Proxmox, and Assistant queried
that exact action and explained approval, successful execution and independent
verification in 6.645 seconds. Neither explanation created another plan.
Restoration stop `act_2265d24450898a678ae3a1dc34034faa` independently confirmed
stopped. Runner, both temporary tokens and tunnel were removed, control was
restored and the production agent remained unchanged.

Playwright exercised the pending/rejected/completed action records at
1440/900/390 and Assistant history, seven tool expansions, reload and the native
exact-action review link. Pixel inspection confirmed the settled desktop
Assistant header and narrow action outcome were usable. An initial screenshot
during scrolling clipped the header, so the affected pass was repeated after
scroll settlement. These passes qualify the named VM journey only. Docker
healthy/unhealthy/dependency/storage and Patrol-origin plan continuation remain
required. Local receipts and the independent interpretation are under workspace
`tmp/patrol-planning-continuation/live-r26/`.


### Current canonical planning qualification, r28 to r30

The r28 binary SHA256 is
`c515516beb61fe902f2c3d1fdd50f4f9bf2185cc33f15d529027d93b2eeaba54`.
It contains the canonical unavailable-disk correction and the revised investigation
prompt. Unavailable disk usage is absent rather than -100 or a fabricated zero.
The prompt distinguishes the observed failure mechanism from an unobserved
origin. It does not manufacture a cause from an accepted plan or require
per-call gap narration. Targeted core metric and tool checks and the complete
enterprise investigation race suite passed on Go 1.26.8.

All following cases used the authorized `openrouter:google/gemini-3.8-flash`
route, disposable fixtures and independent fault/revert/cleanup oracles. Each is
one current repetition, not a statistically representative reliability sample.
Earlier failed attempts remain failed. Scorer grounding fields are keyword
checks. The interpretation below comes from reviewing the model conclusion
against the observed fixture and canonical action records.

| Case and run | Observed result | Watch / complete scenario seconds |
|---|---|---|
| Healthy `q-20260907-150813-a08381f0` | No finding on healthy controls. No injected fault, so recall is not applicable despite the scorer's vacuous value of 1. | 5.803 / 20.296 |
| Unhealthy `q-20260907-150920-a22e598e` | Identified the one unhealthy container and left the healthy sibling alone. Correctly left its internal cause unknown and named the next diagnostic read. Watch-only case. | 11.948 / 47.460 |
| Dependency `q-20260907-150204-5cd9ff3b` | Followed the client symptom to the stopped dependency. Distinguished the observed outage mechanism from the unknown reason it stopped. Canonical start plan `act_488eb1b9e5fb2d79e361bc163206ae19` required approval. No product execution or recovery is claimed. | 11.370 / 52.437 |
| Storage `q-20260907-150004-1bbdf26d` | Identified an exhausted 8 MiB tmpfs, zero free space and ENOSPC. Distinguished unknown origin and the data-loss implication of clearing volatile storage. Plan `act_d2f137411342c995d840d529a70fbf95` remained unexecuted and later expired. Fixture reversion independently restored capacity and health, which is lab recovery rather than a verified product action. | 10.244 / 50.562 |
| Approved `q-20260907-150350-f75852dd` | Identified the failed PID health probe without inventing why the process died. Action `act_fd2d0bdab1930e2d3a75faf080b8ba7a` was approved, executed and independently observed healthy through the Docker daemon. Finding `cd8d247cf83bce25` retained the linked investigation and verified outcome. | 10.232 / 89.633 |
| Rejected `q-20260907-150613-16ba31eb` | Identified the same observed failure mechanism and retained uncertainty. Action `act_369c466aea179f93fc69aa04a710689a` was rejected. Independent observation confirmed the fault remained, with no execution promoted into success. | 9.388 / 39.133 |

The five injected Docker faults were found with zero measured misses and zero
extra findings in these exact runs. This does not estimate population recall or
false-alarm rates. Scenario totals include collection and workflow waiting, not
just model latency. Watch estimates range from US$0.007354 to US$0.015235 and
exclude separate investigation calls. They are not complete journey spend.
Provider-side key limits remain the budget authority. Population useful diagnosis,
false alarms, missed problems, journey latency and verified-outcome rates remain
unknown beyond the honest adoption baseline above.

Missing-runner session `ecd14a20-6855-466e-942c-02fee00f30ef` completed in
9.336 seconds, reported the unavailable typed runner and did not create a
successful action or invent an in-guest agent prerequisite. VM session
`baf00cce-0f5c-4f7b-88d3-7040559c5ae3` prepared rejected start
`act_c007cc3882625e10ef84c28f571590ab`, explained its unexecuted outcome,
prepared approved start `act_cb9cfb3d10031015a063bfcb1f46e90b`, and explained
its independent Proxmox running observation. These four turns took 10.273,
7.068, 5.533 and 6.304 seconds. Both explanations queried the existing exact
action and created no replacement. Restoration stop
`act_96826d092451c76c3063d2a7e270549d` independently confirmed stopped.
Playwright exercised pending, rejected, completed start and completed stop at
1440, 900 and 390 pixels. Those backend outcomes remain source-bound to r28.

All Docker fixtures passed independent cleanup, repeated cleanup no-op and
unchanged-inventory checks. VM110 was restored stopped, temporary runner,
collector and command tokens were revoked, and the tunnel stopped. Tower's
temporary collector and command token were removed and its original development
collector restored at 15:12 UTC. Both production agents remained unchanged.
Patrol is paused and control is read-only. Raw local receipts are in workspace
`tmp/patrol-planning-continuation/live-r28/` and the Tower restoration transaction
in `live-r26/tower-agent-transaction-r3.json`.

The r29 frontend correction stopped projecting unknown legacy destructive risk
as false, removed a synthetic legacy fix, and distinguishes a completed canonical
action from an approval. It passed type checking and 143 focused frontend tests.
Real linked Assistant session `46a5bb5c-5d51-45f8-8856-ca65cdb5034f` read the
approved action once and correctly explained the independent historical outcome,
unobserved process-exit cause and later fixture removal. Browser pixels failed:
chunk-boundary spaces and newlines were lost. The shared handoff-policy sanitizer
trimmed every content delta. The r30 change preserves whitespace through policy
redaction and tests concatenated streamed and stored text while retaining
redaction. Final current-source browser proof is required before this is qualified.

PR #1957 merged as `a66b8e11d7ca9ed5660ffd8725a1461661ca2fdf` at 14:24:04 UTC.
Its benchmark check failed. No manual override was issued by this task, and that
failure is not reclassified as a pass. The remaining implementation is a separate
scoped delivery. Local main was safely fast-forwarded to `7e34b00d4f` with task
changes preserved. The worker export received the exact 31 newly committed
files before the r30 build. Qualification of the affected final browser journey
and verified landing remain open. Independent volunteered environments remain a
separate wider rollout gate. The subscription-provider refusal is preserved.


### Verified-history replay correction, r31

The r30 real-model explanation was readable and correctly cited the canonical
Docker observation time. It also exposed a separate persisted-history defect:
`UpdateInvestigation` and `UpdateInvestigationOutcome` assigned `time.Now()` to
an already resolved finding and appended another verification event when the
same result was replayed. Startup reconciliation could therefore move the
finding resolution time without a new recovery. The canonical action observer
time remained intact.

Both writers now share one verified-resolution projection. It preserves an
existing resolution timestamp, appends verification only for a new outcome or
an unresolved finding, and allows a genuinely regressed finding to resolve again.
Unchanged non-resolution status/outcome replays do not append fake transitions.
Regression coverage checks both writers, repeated replay, active counts and a
new resolution after regression. Previously rewritten timestamps are not guessed
back into history. Their exact recovery evidence remains the canonical action's
independent observation. A current-runtime restart/read comparison and repeated
browser qualification remain required for this correction.


The follow-through separated the stores: the Patrol source still held the correct
15:05:29 UTC resolution, while the unified finding supplied to Assistant had a
later timestamp. The router first projects the source finding and then invokes
`UnifiedStore.Resolve`, which unconditionally replaced that projected timestamp.
The r32 correction makes this shared resolver preserve a recorded resolution and
still timestamp a genuinely reopened finding. A regression covers that exact
projection-plus-resolve sequence. The r31 writer fix remains necessary for
idempotent source updates, but was not alone the complete fix for the observed
Assistant discrepancy. Full affected r31 race suites passed, including chat,
unified resources, lifecycle, tools, qualification and public contracts.


### Final browser follow-through, r33

The r32 source-bound runtime is
`ed1b38e9744f613b52e747ecc53fb669cdad3210bce7cf619c39919cc047cfb8`.
An authenticated before/after restart comparison proved the timestamp repair:
Patrol retained `2026-09-07T16:05:29.047446+01:00`, while the unified record
changed from the incorrect startup time `16:26:35.673544+01:00` back to that
exact source time. There was still exactly one source verification event.
The canonical source could repair this projection without guessing history.

Real Assistant session `93998cc0-cf4c-4ca3-8011-ff27dde0f227` explained the
approved outcome at 15:05:29 UTC and retained uncertainty about the original
process exit. Storage continuation `9d92a098-9f6f-4b05-8f37-e9cfea8c7425`
read the exact expired action, distinguished no execution from recovery,
identified the measured tmpfs exhaustion and attributed finding closure to
later resource removal rather than the unexecuted plan. These are semantic
continuation results, not another fault-injection repetition.

Pixel review exposed an additional desktop defect after streaming: the end
anchor's `scrollIntoView` could scroll the outer document and leave the docked
Assistant outside the viewport. The shared transcript component now scrolls its
own container for both streaming and Latest. It still respects a reader who has
scrolled away from live output. Existing scrolling regressions now check the
owned container and no ancestor scrolling. The final matrix adds streaming,
manual scroll-away, Latest, document position, reachable header/composer and
resize while open. This is required browser qualification, not a screenshot-only
cosmetic claim.

### Scope reconciliation

The current local matrix satisfies the requested storage-or-backup branch with
an actual capacity fault. Backup coverage, restore correctness and independent
backup-source failure remain unqualified and cannot inherit that result. The
old incident-memory listing and typed compatibility-ID adapter remain isolated
modernization residuals. Primary diagnosis uses canonical resource queries and
history. Their earlier limitations are not represented as corrected here.
Unsupported-filter failures from earlier transcripts remain historical failures,
while current named canonical queries pass the recorded scenarios. This does not
qualify every filter or legacy adapter. The fixed retained-history contention has
functional concurrency proof, not a worker performance benchmark. Current
scenario and Assistant timings are observations, not latency SLO qualification.
Unattended action execution and broader autonomy remain outside the approved and
rejected operator-mediated lab proof. Independent volunteered environments remain
the wider rollout gate. These residuals stay in the owning coverage gap rather
than being silently treated as product-wide readiness.


The fresh-stream matrix then distinguished wheel overscroll from programmatic
scrolling. At the transcript bottom, an 800-pixel wheel event moved the outer
page from scrollY 41 to 841 while the transcript stayed at its maximum. The
shared transcript now also contains overscroll, preventing end-of-conversation
wheel input from chaining into the page. The failed r33 browser runs remain
failed. The r34 affected matrix must repeat after this final frontend change.


### Final source-bound qualification, r34, 2026-09-07

The final bundled runtime is `0.0.0-dev-pro+patrol-planning-r34`, SHA256
`e1ae053e656b28af9ba0e7541a47bdeb56303d1d7912edc57b2dac2f94620bd1`.
The source binding matched 4,853 Go, module and frontend files. Relative to the
r28 fault matrix, the only later non-test Go changes are the shared whitespace
redactor, chat presentation redaction, Patrol resolution writer and unified
resolution writer. Planning, investigation prompts, execution and capability
logic are identical. The r28 fault matrix is not misrepresented as a new r34
repetition. The changed writers have package race proof and current-runtime
restart, real-model continuation and browser follow-through.

Worker proof passed the affected AI findings tests and full chat, unified
resources, action lifecycle, agent capabilities, tools, qualification and public
contract race suites in r31. The r32 unified-store race suite passed after the
final timestamp correction. Final r34 frontend type checking, 178 tests across
five affected files, Vite build and the full enterprise investigation race suite
passed. The earlier full API timeout remains a timeout, alongside the narrower
passing API/contract proofs. The earlier PR1957 benchmark failure is still not a
pass and no benchmark claim is added here.

Fresh r34 Assistant session `6a0b9f67-97fc-4966-9674-1060df61cf7b` explained
finding `cd8d247cf83bce25` and exact action
`act_fd2d0bdab1930e2d3a75faf080b8ba7a`. It retained the independent recovery
observation at 15:05:29 UTC, uncertainty about the initial process death and the
separate later fixture removal. Model response time was 11.970 seconds and the
whole browser interaction took 32.815 seconds. No new action was requested.

At `/patrol`, final Playwright covered 1440x1000, 900x1000 and 390x1000:
approved, rejected and expired-storage finding reviews, all investigation tool
cards, deeply scrolled filesystem measurements, Discuss with Assistant,
attached finding context, explicit new-session context clearing, resuming saved
context, exact Actions links, terminal controls, reload and Escape. Storage
retained the expired plan and its risk. Rejection remained distinct from an
executed recovery. The approved action retained independent verification.
Actual desktop, intermediate and narrow pixels were inspected.

Fresh streamed approved explanation and saved rejected/storage explanations
passed wheel scrolling at the transcript end, scrolling away, Latest, resize
while open, unchanged outer document position and reachable header/composer at
all three widths. Saved missing-access session
`ecd14a20-6855-466e-942c-02fee00f30ef` and VM action/outcome session
`baf00cce-0f5c-4f7b-88d3-7040559c5ae3` were selected through the session-history
picker, their exact message reads checked, all tool details expanded and the
same three-width scrolling matrix passed without model submissions. The
missing-access explanation retains the actual missing typed-runner refusal.
It does not invent a guest-agent requirement or claim an accepted action.

The first history scripts crossed asynchronous initial row replacement and
failed on detached locators. A network-idle wait also timed out on this live
monitoring page. The final script waits for the explicit history response,
opens the review through the user control, closes it, checks returned focus and
reopens with Enter. A separate 12-second check retained the exact row node,
keyboard focus and open selection. These failed script attempts are preserved,
not counted as product passes. Both shipped Assistant documentation routes also
passed desktop/narrow linked navigation, reload, deepest scroll and no overflow.

Receipts live under workspace `tmp/patrol-planning-continuation/patrol-browser`:
`approved-explained-r34`, `approved-history-r34`, `storage-history-r34d`,
`rejected-history-r34b`, `rejected-resumed-r34`, `storage-resumed-r34`,
`missing-access-saved-r34b`, `vm-outcomes-saved-r34` and `review-refresh-r34`.
Documentation receipts are in `docs-browser-r34`. The committed browser receipt
binds the three changed frontend runtime files to exact content hashes.

The named local implementation matrix is performed. This does not complete the
production-wide readiness gate. Independent volunteered customer environments,
population diagnosis/false-alarm/miss/latency evidence, backup/restore and wider
unattended autonomy remain unqualified. Local delivery still requires scoped
core and enterprise commits through their repository workflow. Preserve the
explicit subscription-provider refusal with no retry, rephrasing or bypass.


Final r34 action review also reopened VM rejected Start, independently verified
Start and independently verified restoration Stop at 1440, 900 and 390 widths.
Policy/evidence/delivery disclosures, deepest outcome, Escape, explicit close,
reopen and reload passed with login as the only write. Exact-action receipts are
in `patrol-browser/vm-actions-r34`.

The first exact staged-tree hook passed sensitivity/gitleaks, source formatting,
documentation mirrors, browser binding and governance staging, then correctly
stopped because the registry's explicit store-proof list did not name the new
`action_request_identity_test.go`. The owning proof map now includes that actual
persistence/concurrency regression and the new policy-redaction regression.
No contract-neutral or completion override was used. The full hook must pass on
the revised exact tree before commit.


### Core delivery check correction, 2026-09-07

Core PR1960's initial head `c501376843431e1812abd42da16888b462b41dc3`
passed full frontend tests, both non-API backend race shards, all eight E2E
shards, governance, security and paired benchmarks. Its API race suite ran
1607.764 seconds and reported one failing test:
`TestContract_AssistantFindingContextUsesModelOnlyHandoff`. The guard expected
thirteen spaces before `runResult.ModelTurns`, although gofmt correctly changed
field alignment after the result structure changed. No runtime assertion or
race failure was reported by that shard.

The static wiring guard now normalizes whitespace before comparing its required
snippets. It still requires the same identifiers and wiring, and its forbidden
adapter assertions remain intact. The correction changes only the test, not
model behavior or the qualified runtime. The exact failing test and repository
checks must pass before this delivery is counted complete. The failed API job is
[101809873027](https://github.com/rcourtman/Pulse/actions/runs/34143221580/job/101809873027),
with its raw log retained in workspace
`tmp/patrol-planning-continuation/core-api-c501.log`.


### Verified local delivery and remaining release gate

Core [PR1960](https://github.com/rcourtman/Pulse/pull/1960) merged as
`a42e3800d9a1b5ea185823469e891f44bc698c2c`, carrying implementation commit
`c501376843431e1812abd42da16888b462b41dc3`. Its pre-merge qualification check rollup
contained 28 successful checks on final head
`17aa972f357292415322eba09d607cc9fe3f3fe3`, including the correction to
the contract test's whitespace comparison. The full Build and Test run is
[34146504729](https://github.com/rcourtman/Pulse/actions/runs/34146504729), and
Core E2E is
[34146504686](https://github.com/rcourtman/Pulse/actions/runs/34146504686).
Initial head c5013768 failed the API source-contract assertion described above.
Its other checks, including its paired benchmark, passed. The exact failing
contract test subsequently passed under the race detector in 1.064 seconds.
The complete API race suite then passed in 1997.577 seconds on final head
17aa972f, recorded in job 101819688313 and workspace
`tmp/patrol-planning-continuation/core-api-17aa.log`.
These current results do not rewrite PR1957's earlier failed benchmark.
The initial c5013768 paired benchmark gate uses >10% and p<0.05. NormalizeSegment
long-token, short-name and medium-name timings increased 8.55%, 6.69% and
8.38% respectively (n=10), below that gate's threshold. Passing the gate is
not a claim of zero timing change or a production latency SLO.

The first benchmark attempt on final head 17aa972f failed only
`ParseLogLevel/#00`, 4.886 ns versus 5.422 ns, +10.99% (p=0.000, n=10),
on an AMD EPYC 9V74. Auto-merge was disabled while that failure was examined.
The preceding c5013768 CI run on an Intel Xeon 6973P-C measured 2.339 ns
versus 2.504 ns, +7.03%, below the gate. Neither the parser nor its benchmark
source changed between base and candidate. Their compiled parser instruction
sequence was identical apart from relocated addresses, which does not prove
identical microarchitectural timing.

A worker reproduction compared exact base 7e34b00d and candidate 17aa972f
on one Intel i5-10600 with Go 1.26.8 and GOMAXPROCS=4, ten alternating pairs
at each of 100 ms and 1 s. Empty-input results were 5.637 ns versus 5.402 ns
(-4.17%, p=0.010) and 5.482 ns versus 5.358 ns (p=0.149). No log-level
subcase crossed the regression gate. Load was 2.09/2.22/1.41 before and 1.50/1.99/1.52 after.
This did not reproduce the CI failure. It justified one controlled rerun of
the failed benchmark job with the same source, threshold and sample policy.
The rerun, job 101827584247 on an AMD EPYC 7763, measured 6.488 ns versus
7.126 ns, +9.84% (p=0.000, n=10), below the unchanged >10% gate. This is
a measured timing increase, not zero regression. That later passing check
does not erase the first failed measurement or establish its precise cause.
Raw comparisons and disassembly are retained in
workspace `tmp/patrol-planning-continuation/loglevel-17aa/`, and the failed CI
log is `core-benchmarks-17aa.log` beside that directory. The passing retry
log is `core-benchmarks-17aa-retry.log` in the same workspace directory.
No padding, parser
optimization or threshold exception was introduced to force the result.

Enterprise [PR23](https://github.com/rcourtman/pulse-enterprise/pull/23) merged
as `b9fa43dcf0ee743652b20d1a866da8ca9c82cdbd`, carrying
`b3d122751db7d5380f18956a1c9865b81b5d0123`. Its main-branch
[Build & Test 34143454705](https://github.com/rcourtman/pulse-enterprise/actions/runs/34143454705)
passed. `PULSE_TEST_REVISION` pins the companion to the exact public-core
implementation commit above. The full investigation race suite also passed on
the worker after checking that the pin matched the actual core checkout.

The configured pre-commit hook passed on exact tree
`6d43182ea31783feaa50493e91a21b084a9edaad`, unchanged before and after the hook.
The new proof-file mappings and their two existing lookup assertions were
corrected before that pass. All source formatting was unchanged. The Mac's
redundant pre-push lint attempt was stopped. The same frontend lint and type
checking had already passed on the worker, where heavy verification belongs.
The corrected contract test and its progress record passed the same complete
hook on tree `477e03b4b493b41fd52f75cbf5c273b87bdfbd05`, unchanged before and
after the hook. The later non-runtime browser receipt and contract-test source
are the only changes among the 4,853 recorded inputs. The exact committed
public/private comparison is retained in workspace
`tmp/patrol-planning-continuation/delivered-source-equivalence.json`. Delivered
runtime production source and the installed r34 binary remain byte-identical
to final browser proof.

The local implementation and the named regression, real-model, disposable-lab
and Playwright matrix are performed and delivered. The earlier historical
paragraphs preserve failed attempts and then-current open work. This section and
the execution-order table state the current local disposition.

**Classification: release_gate.** Production-wide readiness remains unqualified
until evidence extends beyond this maintainer homelab to independent volunteered
customer environments, with actual diagnoses, false alarms, missed problems,
latency and verified outcomes reviewed against known conditions. Deployment
count or a passing lab score cannot satisfy that gate. Backup/restore, unattended
autonomy, all-filter compatibility and population latency SLOs also do not
inherit the named local proof. The canonical coverage gap and candidate remain
open. The next qualification step is an explicitly volunteered independent
environment
with its source/model/permissions recorded, operator-confirmed known conditions,
negative controls where safe, retained investigation and action identities,
end-to-end timings, and independent outcome observation. Review actual
conclusions against those conditions, record misses and false alarms alongside
successes, and retain unavailable evidence as unknown. This requires the relevant
operator access and consent. Do not manufacture it from adoption telemetry or
repeat maintainer-lab runs as a substitute. No production rollout or release was
performed. The explicit subscription-provider refusal remains preserved without
retry or bypass.

## Continued shared incident-history modernization, 2026-09-07

The maintainer requested continuation after the named local redesign matrix
landed. The independent-environment question remains open. This slice addresses
the recorded incident-memory query residual and does not satisfy the wider
readiness gate by extending the same homelab evidence.

The reviewed continuation baseline selected legacy memory shells before consulting
canonical history. This omits canonical-only alert occurrences and aliases whose
shell identifiers differ from the query. Projection readers also discard store
errors, scan an infrastructure-wide 256-event window before selecting an alert,
and can combine events from repeated occurrences of the same alert. The owning
fix belongs in the shared incident query/projection and canonical history query,
not in a single HTTP handler or model-written summary.

Executable order and acceptance:

1. Reproduce canonical-only, alias, projection-error, bounded-history and repeated
   occurrence cases with actual memory/SQLite history stores. Record provenance
   and observation/occurrence time expectations before changing projection.
2. Read canonical history first through its shared query contract, apply query
   filters before limits, retain explicit bounds and unavailable evidence, and
   preserve legacy notes as attributed history. Keep separate alert occurrences
   separate. Do not create a diagnosis, recovery or current-health verdict from
   missing records or a completed query.
3. Route resource listings, alert timeline reads and model context through that
   shared result. Propagate read failures to API callers and disclose unavailable
   context to the model. Preserve canonical event provenance and existing exact
   alert/resource authority. No new diagnosis orchestration or provider retry
   policy is authorized by this change.
4. Run targeted regressions and affected complete race suites on the worker.
   Inspect the affected live Alerts timeline and Assistant journey with
   Playwright after the final build at 1440x1000, 900x1000 and 390x1000. Cover
   loading, empty, failed reads, repeated occurrences, expanded event details,
   filters, dismissal/focus return, reload and deep scrolling. Inspect pixels as
   well as DOM state. If model-visible context changes, qualify the affected
   real-model history explanation through an authorized provider route and
   preserve any unavailable-provider limit explicitly.
5. Update the owning contracts and this record with exact source, proofs and
   remaining limits, then land the verified scope through the repository
   workflow. Local proof and independent customer qualification stay distinct.

Current state: the incident-history continuation has final r10 source acceptance
on integrated main `977afdd9559c0e9d5859f4c79bcc48e889381bba` plus the scoped
changes. Affected regressions and final browser interaction/pixel checks pass.
Three funded Astra explanations passed, with both earlier Gemini failures
retained. The actual note survives reload and its text reaches Assistant.
Repository commit checks and scoped landing remain pending. Both the old
candidate and matching unchanged base timed out in the full API race suite,
so no passing whole-suite receipt is claimed. The subscription-provider refusal
is preserved without retry or bypass, and wider readiness remains open.

### Incident-history candidate r1: implementation and proof in progress

The shared `IncidentStore.QueryIncidents` now reads canonical evidence before
selecting incident rows. Alert metadata filters and observation bounds are
applied before limits in SQLite and memory, with matching aggregate counts.
Both stores order by observation time and event ID. The earlier memory behavior
used reverse insertion order, which disagreed with SQLite for late observations.
The relationship-aware store and monitoring replay fixtures now assert the
canonical observation order while retaining every expected event.

Explicit firing times partition repeated occurrences. The alert identifier and
firing time determine canonical-only occurrence identity. Repeated observations
of the same firing do not rename it, and two firings 500 milliseconds apart stay
separate. Exact-start selection chooses the closest occurrence within the
existing one-second shell matching tolerance. A capped query does not attach
ambiguous later events to an earlier saved occurrence. Missing starts stay
unknown. Source event IDs, observed/occurred times, actor, adapter, confidence,
metadata and related resource identities remain on the returned evidence.

Canonical-only incidents can retain an operator note without copying canonical
lifecycle events into a second durable history. Saved notes and snapshots carry
their own source attribution. History aliases are read selectors only and do not
change action authority. API reads return service-unavailable on canonical read
failure, and monitoring skips reconciliation instead of fabricating a fallback
when that read fails. Assistant's incident summaries disclose unavailable or
truncated history and do not claim current health from historical lifecycle.

User job: “What happened last time, and is this the same problem?” The live r34
baseline was opened at `/alerts`, History, then a timeline at 1440 × 1000. This is
three navigation/control actions. The expanded timeline has twelve event filter
controls plus the Assistant handoff before its event content. The current slice
keeps the established timeline interaction and places new forensic provenance
behind one “Evidence details” disclosure. Missing timestamps are shown as
unavailable, and a truncated query receives an explicit notice. The inspected
issue #1782 supports current-evidence grounding and preservation of the governed
action path. It does not establish customer demand for extra timeline decoration.
No issue comment was sent.

Current proof receipts under the worker's `patrol-incident-history` directory:

- New filter regression first failed compilation because the filter fields did
  not exist. It then passed against both real stores. The full store race suite
  initially exposed the reverse-insertion expectation described above.
- Focused memory and incident HTTP race tests passed. The full affected suites
  passed: memory 2.217 s, unified resources 72.877 s, alerting 3.691 s,
  monitoring 212.854 s and runtime 24.119 s. An earlier monitoring run failed
  its reverse-insertion expectation, before that expectation was corrected.
- Frontend type checking passed. Nine tests across the timeline panel and event
  card passed, followed by the current frontend build. An initial dependency
  symlink attempt failed Vite module resolution before running tests. A fresh
  task-local `npm ci` resolved that environment failure.
- Cross-build verified 5,985 core and 60 enterprise source hashes with no
  mismatches. Candidate `0.0.0-dev-pro+incident-history-r1` has SHA-256
  `1cd63e63b767a7a02334a89ead3b8f34941b1371fdd764646ae6f689df7455c0`.

This is a candidate under qualification, not a completed continuation. The full
API race suite, final-build Playwright matrix, affected real-model explanation,
final review and repository landing are still pending at this checkpoint. The
previously authorized Gemini key had a US$5 limit and expiry on 2026-09-07.
No provider call has been made in this continuation, and the Claude subscription
refusal remains untouched. Independent customer-environment consent/evidence is
still unavailable, so the wider rollout gate remains open independently of this
local implementation work.

### Incident-history r1 live failures and r2 corrections

The r1 live timeline of disposable run `q-20260907-150920-a22e598e` failed
qualification. The alert-history row was resolved, but its timeline was open.
The canonical resource timeline contained both records: firing
`85ddd9f5-0368-5740-a041-eb68d2f131c6` at 15:10:01.025055Z and resolution
`f3c13163-33dd-5b10-9b37-ef8c8a1bc37b` at 15:10:30.96527Z. Two legacy shells
had the same alert identifier and explicit firing time. Projection assigned
firing to one and resolution to the other. The shared query now merges those
shells before projecting canonical events, retains a stable existing ID and
all local notes, and accepts either saved ID as a read selector. Regression
covers both aliases and the combined lifecycle. API snapshot fallback was not
used to mask this defect.

A separate live backup timeline displayed `warning 0.0 >= 0.0`. The shared event
summary had invented an inequality from numeric fields without retaining the
source condition. Source messages now own fired-event descriptions. Where no
message exists, the fallback names only the alert type and level. Regression
covers source conditions, including a below-threshold comparison, and neutral
fallbacks for CPU and backup incidents.

Read-failure inspection also exposed an error-visibility gap. A timeline refresh
could retain cached evidence while hiding its error, and resource history could
show an empty-state claim after its error toast disappeared. Both views now
retain visible read failure and a retry control. Resource-history state preserves
cached evidence separately from loading and failure, and clears failure only
when a new read succeeds. Tests exercise failed initial read, successful retry,
failed cached refresh and successful empty read.

The r1 Playwright script also had a harness error: Escape cleared its search,
so its single-row expectation saw 138 timeline buttons. Removing that unintended
search reset repaired the script. This is separate from the reproduced product
failures above. No final r2 browser or real-model pass is claimed here.

The saved non-secret provider-limit receipt confirms the authorized key expired
at `2026-09-07T17:23:20.631Z`. This was verified after that timestamp, without
calling the provider. Its initial US$5 balance is not a current spend balance.
The affected r2 real-model history explanation requires a valid funded route
and remains unperformed. The subscription-provider refusal is not an alternative
route and has not been retried.

Remaining affected model qualification is a read-only Assistant turn from the
repaired historical lab incident. Ask: “Explain what happened in this occurrence,
what evidence records its resolution, and what we can and cannot conclude about
its current health. Do not change anything.” Verify the response against the
independent canonical firing/resolution records above, its saved note and exact
occurrence ID. The answer must distinguish observation from occurrence time,
recorded resolution from present health, and historical evidence from an action
outcome. No action proposal, execution or provider substitution is needed. A
second turn with unavailable history must say that the evidence could not be
read, without converting failure into no incidents or a healthy result. Retain
session IDs, provider/model, exact runtime hash, latency and actual spend. A
scripted fixture response does not satisfy this real-model check.

The maintainer supplied a replacement OpenRouter key in this continuation and
authorized its use. The key metadata endpoint reports a valid paid key with
zero initial usage and no provider-side limit or expiry. The prior US$5
qualification ceiling is retained as a task budget, not claimed as a provider
enforcement boundary. Only the two read-only history turns above are planned.
Authenticated local settings confirmed Patrol disabled, control level read-only,
and `openrouter:google/gemini-3.8-flash` selected for chat and Patrol before the
credential update. The update succeeded through the ordinary settings endpoint.
An initial helper request omitted its CSRF header and was rejected before the
update. The corrected request supplied the normal session CSRF token. No
subscription request or model call was made during configuration.

Narrow exploration reproduced an additional handoff defect at 390 × 1000.
The incident drawer remained above Assistant after Discuss, hiding the
continuation. The shared incident handoff now exposes the same explicit callback
pattern used by finding handoffs. Mobile timeline and resource-history drawers
close after the incident context has been handed to Assistant. Desktop inline
panels remain governed by their existing controls. Mounted tests cover both
mobile source views and preserve the same incident ID, status and read-only
context. Final browser qualification must repeat the drawer transition and
inspect the reachable Assistant composer, dismissal and focus.

The first callback attempt still failed because the app's shared blocking-dialog
guard correctly closed Assistant while the source drawer was mounted. The final
transition captures the incident context, closes the source drawer, then opens
Assistant in the next microtask, matching the existing command-palette handoff.
The mounted regression checks that the dialog stack is no longer blocking when
Assistant opens. Narrow exploration then passed without a model request. This
exploration used the r1 backend and does not qualify its known split lifecycle.

### Candidate r2 qualification checkpoint

Final r2 source passed memory and unified-resource race suites in 2.225 s and
72.169 s, plus focused incident alerting/API/monitoring race checks in 1.203 s,
2.576 s and 1.770 s. Frontend type checking and 62 tests in eight files passed,
followed by the frontend build and verification of 5,987 core and 60 enterprise
source hashes. Binary SHA-256:
`7c82377ca4cc58f0f3ab3e3d64bd2edac7692309596416ce0f70e836d731726a`.
The local built runtime confirmed the previously split lab occurrence as
resolved with both canonical lifecycle records.

The broad r1 API race run failed. It reported
`TestServerInfoEndpointReportsDevelopment` expecting development mode, then hit
its one-hour deadline in `TestAuthenticatedEndpointsRequireToken`, which had
spent 39m6s in the running test. The timeout stack is in mock unified-resource
fixture expansion during per-resource metric-window evaluation and router
construction. This is not a passing broad API receipt. The worker source export
has no Git directory. A fresh exact-base Git checkout passed the same two tests
in isolation in 62.298 s after its required frontend embed build. The initial
baseline attempt lacked generated embed assets and failed setup. Candidate
comparison in that Git checkout remains pending at this checkpoint. Neither
isolated result can establish that the full suite is free of shared-state or
performance problems.

R2 browser qualification also found that Assistant's composer registration and
focus happened only at mount. Reopening from a mobile drawer left focus on the
underlying alert search, so Escape cleared the search rather than dismissing
Assistant. Candidate r3 moves registration and focus into Assistant's shared
open lifecycle and clears the registration on close. A mounted regression
starts closed and checks two separate composer instances across reopening.
Final build, complete browser repetition and both funded model explanations
remain pending. No full-task completion is claimed.

### Candidate r3 browser failure and r4 correction

The isolated candidate API comparison passed the same two tests in 63.937 s.
The complete Assistant component file passed 211 tests. These receipts do not
replace the failed broad API race run above.

R3 was built and deployed locally with SHA-256
`f4ec960416cff6fa5c779119689d04f2842e38aee6494e23a4f1649d4f7e1d01`.
The repeated live matrix still failed narrow Escape dismissal. Browser inspection
identified a second focus owner in the mobile alert list. Its normal drawer-close
callback restores focus twice through animation frames, overriding the shared
Assistant open lifecycle. R4 gives the source drawer an explicit handoff callback
that closes the investigation without scheduling return focus. Ordinary drawer
dismissal retains its existing focus restoration. The regression now mounts the
parent mobile list and waits through both animation frames before checking the
destination focus. Its first run exposed an incomplete test fixture, which omitted
the required alert type. That fixture was corrected before repeating qualification.

Opening Assistant also sends ordinary provider-readiness requests. The browser
receipt's chat/session submission counter excludes those probes and must not be
interpreted as a count of all provider traffic. No history explanation has been
submitted at this checkpoint. The subscription refusal remains unchanged.

R4 passed 218 frontend tests across the full Assistant file and both mobile
investigation files, type checking and the frontend build. Runtime SHA-256:
`bad20d2dee980abf826809755689538ae8c1a5ea113656619e7d5d92741af366`.
The complete live `/alerts` history matrix passed at 1440, 900 and 390 × 1000,
including both Assistant handoffs, Escape, evidence expansion, cached read
failure/retry and reload. Controlled-state scripts initially selected notification
toasts or hidden desktop copies of mobile elements. Those locator failures were
corrected without changing product source.

The first funded history explanation was a failed qualification, despite correctly
distinguishing alert closure from current health and verified remediation.
Session `efdc8896-747c-4516-8b43-d1e697495c00`, Gemini 3.8 Flash through OpenRouter,
HTTP 200, 15.806 s, 28,070 input and 1,622 output tokens, recorded session cost
US$0.027135. The model read inventory and canonical history, then asserted no governed action record existed without reading actions. The
initial network-removal criticism did not account for automatically attached
related-resource history and is corrected in the r6 review below. R5 corrects the shared history tool's existing
read-scope description: resolution records closure rather than workload recovery,
related-resource IDs identify relationships rather than additional event targets,
and this read does not query action records. These are factual source boundaries,
not deterministic diagnosis or response-scoring rules. The failed transcript is
retained locally. A fresh-session rerun is required before the unavailable-history
turn. The additional turn remains within the existing US$5 task ceiling.

Provider-readiness inspection confirms that this OpenRouter route performs an
authenticated key-metadata GET, not a model completion
(`internal/ai/providers/openai.go`, `TestConnection` / `testOpenRouterKey`).
The first funded session increased the local one-day estimated usage total by
US$0.02794875, including US$0.00081375 for its automatic session title. These are
Pulse pricing estimates. Actual provider billing and remaining balance have not
been independently reconciled after that turn.

### R5 outcome and R6 qualification plan

R5 tool regression passed in 1.640 s and the final source hashes matched.
Runtime SHA-256:
`f43652b8c939238b5dcb7848e68c99d72def8ade316234010d6767784e99447f`.
The live three-viewport history matrix and controlled desktop/mobile loading,
initial failure, retry, empty, partial and unknown-time cases passed again.
The controlled cases substitute HTTP responses, not a real database outage.

The second funded Gemini history explanation also failed factual qualification.
Session `c2f40f27-6d7c-40f6-8e3a-549ac5232071`, HTTP 200, 18.406 s,
40,235 input and 1,630 output tokens, estimated session cost US$0.03628875.
Including its automatic title, the local estimated usage increase was
US$0.037116. It stopped claiming the related network was removed, but still
asserted no action plan or verified execution existed without reading action
records. Both failed attempts remain in the denominator. Their total estimated
increment including titles is US$0.06506475. Neither is a passing history
explanation or evidence of population-wide effectiveness.

The request inspection exposed a separate shared projection defect: saved shells
retained a legacy resource ID and stale risk fields despite canonical alert
evidence. R6 gives explicit canonical alert event kinds ownership of resource
identity, type, severity and message, while retaining the saved incident ID and
notes. A related command's execution host cannot retarget the incident. The new
single regression passed locally in 0.495 s after correcting its fixture's
collection type. Full memory regression and the rebuilt runtime remain required.

The full candidate API race run in the disposable Git checkout failed at its
30-minute deadline, with `TestAuthenticatedEndpointsRequireToken` running 6m40s.
The package elapsed time was 1802.152 s. Its stack traverses mock fixture cloning,
unified snapshot construction, per-resource metric-window evaluation and router
construction. Those stack functions are not modified by this incident query
slice. This is an unresolved broad performance/qualification limit, not a pass
and not proof that the entire failure is unrelated to all candidate effects.
No further broad rerun was queued ahead of the release rehearsal.

The next two history checks use the exact funded route
`openrouter:openai/gpt-6-astra`, in fresh sessions with Patrol disabled and control
read-only. The provider's public Models API lists function calling and the route's
prices. [Official model documentation](https://developers.openai.com/api/docs/models/gpt-6-astra)
identifies it as a model for complex reasoning and end-to-end work. The route was
selected as a stronger-model qualification, not to erase Gemini's failures or
claim that its earlier seven-case lab matrix transfers to this model.

The exact OpenRouter price row was reviewed on 2026-09-07: US$10 input / US$50
output per million tokens below its `min_prompt_tokens=272000` override, and
US$20 / US$75 at that override. Cache discounts are omitted for conservative
budget estimation. Unreviewed variants remain unknown. A targeted boundary and
alias regression passed locally in 0.364 s. These are estimated list-price costs,
not actual billing. The aggregate qualification ceiling remains US$5. The
subscription-provider refusal is unchanged. No GPT-6 Astra explanation has been
submitted at this checkpoint.

The local 30-day estimated budget was temporarily set to US$5 before any Astra
explanation, with a pre-existing estimated total of US$1.45530525. This is stricter
than a separate US$5 allowance for this continuation. The existing budget check
runs between model turns and cannot cap an already in-flight provider charge.
The prior budget was zero and the prior chat route was Gemini 3.8 Flash. Restore
those temporary qualification settings after the two checks, retaining the
user-authorized replacement credential in encrypted runtime configuration.

A matching full API race baseline comparison is queued after the candidate r6
proof/build, behind the release rehearsal. It uses unchanged base
`1a822d164b3119724b01615da217618edf7e37e5`, a disposable Git checkout, the same
Go 1.26.8 toolchain, non-root execution, GOMAXPROCS=4 and 30-minute timeout.
This is a pending diagnostic comparison, not a passing receipt or a relaxation
of the candidate failure.

### R6 final source qualification in progress

The release rehearsal released the shared worker at 23:28:46 UTC. The normally
queued r6 job completed with exit 0. The full memory race suite passed in
2.189 s and the full cost race suite in 1.148 s. Focused alerting and monitoring
race checks passed in 1.222 s and 1.496 s. The first filtered memory invocation
reported no tests, so it is not counted as proof. The subsequent full memory
invocation is the passing receipt. All 5,987 core and 60 enterprise source
hashes matched. The built r6 runtime SHA-256 is
`3c875fcfe1826c4ef50fe6f116d49269c1111dc569b09a0e80175d13f9421f92`.
It was hash-checked and installed on the local development stack. Final browser,
funded-model and saved-note qualification remains pending at this checkpoint.

The r6 live matrix passed at 1440, 900 and 390 × 1000 with the canonical resource
ID and type in the incident and Assistant handoff. Controlled desktop/mobile
loading, initial error, retry to empty, partial-history and unknown-time cases
also passed. Pixels confirmed visible evidence and errors, reachable handoffs
and the mobile composer. This proof preceded the resize defect described below.

Both funded Astra turns passed factual review. History session
`1b881e3f-57b1-4fcd-be64-ee8e46885394` completed HTTP 200 in 18.507 s,
with reported 8,054 input and 456 output tokens, estimated session cost
US$0.10334 and total usage increase US$0.11216 including its title. It explained
the recorded closure, kept present health and remediation unknown, and said an
action record was not supplied rather than asserting none existed. Its network
removal statement is supported by separate canonical record
`8539f198-82b0-4720-81c1-abdf871e60b1`, targeted at
`docker-network-7f19c579093f3cd0`. The shared server handoff automatically reads
related resource history with a five-record limit. This record was within that
input. Tool calls alone are not the complete model context. The earlier r4
network criticism is therefore withdrawn as an unsupported qualification
finding. Both Gemini attempts still fail on their independent unsupported
claims about action-record absence.

Unavailable-history session `8365f1d5-e20b-4317-b297-3200f37ad67c` completed
HTTP 200 in 8.957 s, reported 6,843 input and 100 output tokens, estimated
session cost US$0.07343 and total increase US$0.07883 including its title. It
stated only that canonical evidence could not be read and left incidents,
health, actions, cause and transience unknown. Neither Astra turn called tools
or changed infrastructure. The unavailable case uses the actual formatter's
error text in controlled handoff context, not a live database outage. Total
estimated spend across all four turns and titles is US$0.25605475. Actual
provider billing remains unreconciled. Two failed explanations remain in the
four-attempt denominator. This is not a population success-rate estimate.

Some earlier saved SSE text had mojibake because the browser receipt helper used
response.text decoding. Actual pixels rendered punctuation correctly. Subsequent
capture explicitly decodes response.body as UTF-8. No output-rewriting product
patch was added. After both turns, the temporary chat model was restored to
Gemini 3.8 Flash and budget to zero, with Patrol still disabled and read-only.
Zero budget is omitted by the settings JSON, which initially confused the
restoration assertion. A subsequent read verified the restored settings.

### R7 responsive investigation correction in progress

One actual note was saved to the owned historical lab occurrence and survived
read-error retry and desktop reload. Shrinking that open desktop timeline to
390 pixels exposed a product defect: its card said Hide, but the timeline was
hidden and no mobile drawer existed. Two read-only attempts confirmed it without
writing another note. The local mobile drawer selection had diverged from the
shared expansion state. The mobile layout now projects existing shared resource
or timeline selection when entering the phone layout and leaves that state
intact when returning to desktop. Its action label reflects the actual drawer,
including when other desktop rows remain expanded. Both timeline and resource
resize regressions pass in the seven-test mobile-list suite. R7 build and full
affected browser repetition remain required. The responsive correction alone left the model input unchanged. A subsequent
review found that the handoff discarded operator note text, retaining only its
"Note added" summary. R7 now preserves the attributed note in shared incident
model formatting and the Assistant handoff, while excluding unrelated details
and raw command output. The new memory regression passed in 0.437 s and the
combined handoff/mobile-list checks passed 11 tests in 1.58 s. One additional
funded history-and-note explanation is required after the r7 build, within the
existing US$5 ceiling. The two r6 outcomes remain recorded without replacing
that final changed-context check.

### Integration with updated main, 2026-09-07

Main advanced from `1a822d164b3119724b01615da217618edf7e37e5` to
`203b50a46ece5e911d6f5f6ea46bbeef218aeab5` during qualification. The candidate
was saved to a task-local archive and explicit-path Git stash, main was
fast-forwarded, and the work was reapplied. Upstream lifecycle replay identity,
unchanged-checkpoint suppression and their regressions are retained. The
superseded legacy projection helper remains removed in favour of QueryIncidents.
Owning contract additions from both changes are preserved.

The upstream cross-resource occurrence regression failed on the initial merge
in both the two-minute and 500-millisecond cases. An unrelated resource's shell
was shortening the selected alert occurrence. The shared query now resolves
resource history identities, separates saved-shell grouping by resource, adopts
the exact canonical firing target before assigning subsequent evidence, and
excludes unrelated-resource boundaries for alert events. The five focused
integrated regression groups passed in 0.491 s. Full integrated source proof and
r7 qualification remain required. The old-base API comparison only diagnoses the
earlier full-suite timeout and cannot qualify the integrated candidate.

The matching clean old-base full API race comparison also failed its 30-minute
bound. The checkout was exactly `1a822d164b3119724b01615da217618edf7e37e5`
and stayed clean. It ran as the normal worker user through the allocator with
Go 1.26.8 and GOMAXPROCS=4. The package failed after 1802.195 s, with
`TestRecoveryEndpointRequiresDirectLoopback` running for 7 m 46 s at timeout.
The earlier candidate failed after 1802.152 s with a different active test,
`TestAuthenticatedEndpointsRequireToken`. The worker log is
`/opt/pulse-release-worker/patrol-incident-history/base-api-proof.log`.
This establishes that the unchanged old base also cannot finish that broad
race command within the same bound. It does not attribute every candidate
effect to the base, and neither run is a passing whole-API-suite receipt.
The integrated candidate still requires its scoped proof and repository checks.

### R7 integrated source qualification, 2026-09-08

The normal worker r7 command completed with exit 0. All 5,990 core and 60
enterprise source hashes matched. Complete affected race suites passed:
incident memory 2.252 s, unified resources 72.880 s, alerting 3.598 s and
monitoring 206.311 s. Frontend type checking and all 51 tests in the four
handoff/mobile investigation files passed. The frontend build completed in
27.50 s. The resulting development runtime is
`0.0.0-dev-pro+incident-history-r7`, with core base
`203b50a46ece5e911d6f5f6ea46bbeef218aeab5` plus the scoped changes and enterprise
`b9fa43dcf0ee743652b20d1a866da8ca9c82cdbd`. Its SHA-256 is
`341b9d2e3c685d0280a321453546a5a02321d8ba9ce923f99a51a961da9c8340`.
The same binary and frontend assets were installed on the local development
stack. Final browser, changed-context model and repository checks remain
required at this checkpoint.

The remaining r7 scoped race proof also passed: complete shared AI 23.613 s,
tools 62.645 s and cost 1.148 s, then the selected incident API handlers in
2.015 s. It ran from the exact integrated Git checkout under the normal worker
allocator. These passes do not replace the failed broad API runs.

R7 live history and controlled-state Playwright scripts passed at their planned
widths, including composer focus after both handoffs. Pixel inspection then
found the saved operator note clipped on the 900-pixel desktop timeline because
event text inherited the table's no-wrap style. R8 corrects the shared event
card's wrapping and preserves note line breaks. This is a real browser failure,
not a passing visual receipt. The affected matrix must repeat on r8. The initial
resize command also failed in its invocation wrapper before the script ran,
while node's syntax check of the actual script passed. That invocation provides
no product evidence. No additional operator note was written.
The direct resize invocation then reached the real r7 UI and failed the new
saved-note overflow assertion, independently reproducing the clipped text.

R8 type checking, 51 frontend tests and the 26.50-second build passed. Its
runtime hash is `90d912e8cea77ffd108be0046430cec1e90d4403900861fcd8c6aa1ae8265152`.
Live and controlled-state scripts passed. The resize matrix retained the note,
draft and expanded events across 390/900/767/768-pixel transitions, but closing
the resource drawer with Escape cleared the background search and lost focus.
The first assertion reported 22 Resource buttons. A second read-only capture
confirmed the search had become empty and focus moved to body, so this is a
product defect rather than merely a selector ambiguity. The shared type-to-search
registry treated an inert modal background as eligible for keyboard shortcuts.
Two new regressions failed before the fix. R9 excludes inert inputs from both
visible and prepared search targets using the shared dialog's existing DOM
ownership boundary. No new dialog lifecycle or lane-local keyboard handler is
added. The final browser and funded model check remain pending.

The corrected shared search, Dialog and SearchInput suites passed 36 tests in
1.54 s locally. The normal worker repeated those 36 tests, type checking and
the frontend build (28.14 s), then built r9 with exit 0 and matching 5,990/60
source hashes. The installed runtime SHA-256 is
`7bad38f8da70a89b56b6693f41af5fa3176c4375bf10b5b1f4958aaa8fc929ba`.
The shared keyboard hook now has explicit frontend-primitives registry ownership.
Three existing presentation expectation files were updated after this build
and passed 118 tests in 1.36 s. Those later edits are test-only and do not change
the built runtime source. The integrated r7 backend proofs remain applicable
because r8 and r9 changed only frontend presentation and keyboard ownership.

All three r9 browser scripts passed, including note wrapping, retained draft,
saved-note reload, modal typing, retained search, Escape and focus return at
1440/900/390 and the 767/768 breakpoint. The subsequent funded history-and-note
turn passed factual review. Session `81c92125-cdc3-4611-931a-687e6548bf65`
completed HTTP 200 in 18.668 s with reported 8,130 input and 456 output tokens.
Its estimated session cost was US$0.1041, plus US$0.0088 for the title, for
US$0.1129 total. Five continuation turns and titles now total US$0.36895475
estimated, with three passing explanations and both Gemini failures retained.
Actual provider billing is not reconciled, and this is not a population rate.
The answer attributed the historical closure, retained current-health and
action-outcome uncertainty, and correctly described the operator note as a
display/persistence check with no infrastructure change or verified recovery.
It called no tools and performed no infrastructure action. Temporary model and
budget settings were restored to Gemini 3.8 Flash and zero, with Patrol disabled
and read-only. The subscription-provider refusal remains untouched.

The same model run exposed a further responsive defect: shrinking a desktop
conversation reopened its source history drawer above Assistant. The earlier
matrix had resized the source before handoff, not the open destination. Two
regressions reproduced this for timeline and resource history. R10 keeps the
existing Assistant destination active when projecting history state on a layout
change. It adds no model-visible context and needs no additional funded turn.
The affected browser matrix, including resizing the open conversation and
reading the saved response, must pass before source-bound acceptance and landing.

### Final incident-history source acceptance, r10

R10 passed 43 affected frontend tests and the 27.07-second worker build with
matching 5,990 core and 60 enterprise source hashes. Its installed runtime hash
is `2e1958d0856c812d22870144cf228509f2a4a3696258a533d34802442f500edb`.
All four final functional browser scripts passed, followed by actual pixel
inspection. `/alerts` was exercised at 1440, 900 and 390 × 1000, with source
and destination resize transitions also covering 767 and 768 pixels. The saved
funded response was resumed by exact session ID and read at every width without
another model submission. Assistant remains visible above the retained source,
the composer is usable, and the operator-note explanation is readable.

The final interaction matrix includes canonical occurrence selection, native
Evidence details with keyboard focus and activation, All/None filters, note
draft enable/clear, actual saved-note reload, source/destination resizing,
expanded resource events, cached and initial read errors, Retry/Refresh, empty
and partial history, unknown timestamps, modal typing, Escape and return focus.
Long notes wrap at the intermediate desktop width. Error toasts are not the only
failure indication. Controlled HTTP variants qualify rendering, not a real
database outage. Final acceptance is bound to exact frontend bytes in
`frontend-modern/browser-verification.json`. Earlier visual and factual failures
remain recorded above. The final helpers are executable at
`/Volumes/Development/pulse/tmp/patrol-incident-history/verify-live.mjs`,
`verify-states.mjs`, `verify-resize.mjs` and `verify-saved-explanation.mjs`.
Their receipt directories are `browser`, `browser-states`, `browser-resize` and
`browser-saved-explanation` under that task directory.

Repository commit checks and scoped landing are the remaining local delivery
steps. The broader customer-outcome gap stays open. These receipts do not
qualify independent customer environments, unattended autonomy, backup restore,
population false-alarm/miss rates or latency SLOs. The failed broad API race
runs remain an explicit test limit, despite passing affected package and handler
proof. The final history explanation qualifies the funded Astra route and does
not erase the two Gemini failures or transfer the earlier named lab matrix to
another model.

Commit preflight initially rejected missing dependent agent-lifecycle and
storage-recovery contract updates and missing explicit proof mappings. Those
contracts now state the historical-evidence boundary, and the registry maps the
actual incident query and lifecycle regression files to their owning paths.
Additional adapter assertions verify filtered-query forwarding, alias identities
and unavailable reads. The targeted query tests passed in 0.573 s. These are
test/governance changes only, leaving the r10 runtime and browser hashes intact.
The staged canonical completion and registry guards then passed without a
contract-neutral bypass.

The final upstream integration advances the base to
`977afdd9559c0e9d5859f4c79bcc48e889381bba`. It adds checkpoint and guest-memory
tests plus contract text, with no runtime change. Comparison against the 5,990
source manifest found only three test files and the browser receipt different.
All 14 accepted frontend content hashes remain identical. The newly integrated
checkpoint assertion initially compared fresh query timestamps as durable
incident state and failed. It now compares stable projection identity/events
separately from refreshed query bounds, while preserving coverage semantics and
the no-checkpoint-replacement assertion. The targeted occurrence test passed
in 0.450 s after this test-only integration.

The first full worker hook stopped at missing private repository evidence roots.
The existing filesystem-proof copies of pulse-pro and pulse-mobile and the
current task enterprise source export supply those roots through the supported
repository-root overrides. Status audit then reported no errors or warnings.
This repairs the proof environment and does not suppress the audit.

Registry audit also requires sibling Git checkouts. Initial sibling links made
that audit pass, but the cross-repository absolute-path helper correctly rejected
paths resolved outside its workspace. Real disposable clones of the existing
private filesystem-proof checkouts replace those links, with no audit bypass.
Its audit and contract audit pass. The full hook then caught one expected-file
fixture missing the newly registered monitoring regression. Updating that fixture
to include the actual proof file preserves the guard's exact mapping assertion.
All 130 completion-helper tests passed in 2.645 s. This is test-only scope.
