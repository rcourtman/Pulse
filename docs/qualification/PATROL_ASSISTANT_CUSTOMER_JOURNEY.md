# Patrol and Assistant customer journey qualification

The customer job is: "Tell me what needs my attention, explain why, and help me
deal with it without creating more work." Patrol owns the issue and investigation.
Assistant explains that same issue and uses existing governed action contracts.

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
