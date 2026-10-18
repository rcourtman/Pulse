# Recovery confidence and change investigation plan

Status: TRACKED POST-RELIABILITY PLAN. Audit and sequencing only. The existing planned product target, L15 and L19 follow-ups, and demand ledger link to this sequence. Implementation remains unscheduled until reliability explicitly yields priority. This document does not activate a lane, change the reliability target, or add a release gate.

Reviewed: 2026-10-17
Roadmap tracking updated: 2026-10-18
Source baseline: `8f10dfa9438151db41ea01ccf19e58b669601e96`

Pulse should first make recovery evidence and nearby changes easier to act on. The strongest first investment is the already-requested cross-system backup task timeline. Follow that with an accessible incident-focused change investigation flow, then a consistent explanation of workload protection and its objectives. Full recovery orchestration is a later product decision.

## User jobs and scope

The least-expert plausible operator needs to answer four questions:

1. Is this workload backed up, and what does the evidence actually establish?
2. Did its secondary copy, verification, or related protection job fail?
3. What changed around the time this service became unhealthy?
4. What should I inspect or do next, and where can I do it?

This audit covers Proxmox Backups, workload recovery details, resource change history, and alert-to-investigation handoffs. It compares Pulse's current implementation with PegaProx and ProxCenter's documented capabilities. Competitor documentation establishes product scope, not comparative safety, performance, or installation-level reliability. Several ProxCenter capabilities require Enterprise. PegaProx labels automated restore-and-boot backup verification experimental.

## Current Pulse baseline

Recovery already has a useful product model. `/proxmox/backups` offers By date and Coverage. Coverage includes workload identity, posture explanation, provider collection and access evidence, and individual restore points. Snapshots remain distinguishable from independent backups. Loading, unavailable, incomplete, unknown-time, and genuinely empty results are separate states. The APIs are read-only and require monitoring read access. There is deliberately no aggregate `/recovery` page.

Protection posture includes freshness, verification, coverage and an explanation. Default backend policy uses seven-day freshness and verification windows, with verification optional. Artifact-age presentation also has seven-day and thirty-day boundaries. Backup color settings and backup-age alert thresholds are separate controls. These concepts exist already, but a workload-specific recovery objective and its relation to each displayed result need to be made explicit before claiming SLA or recovery readiness.

The browser baseline supports extending Coverage rather than replacing it. Expanding a failed workload explains that a backup exists but a source reported failure. A task-only workload remains unprotected. A verified workload shows its PBS and PVE evidence separately. The useful refinement is an earlier, shorter answer and a clearer next step. Server CPU, memory and deduplication telemetry can remain available behind expansion when they do not explain the workload's protection problem. Keep detailed posture definitions available without requiring every operator to read the full paragraph before reaching the first workload.

Backup Health reports already use retained attempt history and disclose incomplete coverage. They are configured in reporting, separate from the backup investigation journey. The general PBS resource drawer shows current job totals and active tasks. That is different from a durable ordered history of backup, sync, verify, prune and garbage-collection outcomes.

Changes already have canonical structured records, persistence, provenance, actor where supplied, related resource IDs, typed filters, and global and per-resource APIs. The generic drawer history implementation requests 25 recent changes when remote history is enabled. Its renderer supports kind, source and adapter filters, timestamps, transitions, reasons and expandable metadata. It has a total count but no continuation into older results or incident time-window control. Related-resource labels in individual change records are text chips.

The normal platform table presentation explicitly disables remote history requests and falls back to changes already present on the resource snapshot. It hides the history section when there are no embedded changes, counts, filters or errors. In the inspected Proxmox guest flow, the row opened metrics and technical details but exposed no change history. The generic renderer's capabilities must therefore not be mistaken for a dependable historical investigation route in the current table flow. This is the first problem for the change slice to resolve.

Registry change detection is narrower than full provider configuration auditing. Its configuration summary covers resource type, technology, name and custom URL, with other observed fields listed in metadata. It does not establish a complete historical diff of Proxmox CPU, memory, disks, networking, storage and firewall configuration. Actor information from a snapshot difference must remain unknown unless another authoritative source supplies it.

Alert investigation already hands resource-scoped context to Patrol and Assistant. Approval, action review and independent verification exist. These are foundations to extend, not missing systems to replace.

### Current source references

| Concern | Current authority |
| --- | --- |
| Recovery journey and limitations | [Recovery guide](../../../RECOVERY.md), [backup page](../../../../frontend-modern/src/features/proxmox/ProxmoxBackupsTable.tsx), [coverage details](../../../../frontend-modern/src/features/proxmox/ProxmoxCoverageTable.tsx) |
| Posture and age semantics | [Protection posture](../../../../internal/recovery/posture.go), [backup presentation model](../../../../frontend-modern/src/features/proxmox/proxmoxBackupRecoveryModel.ts), [backup threshold controls](../../../../frontend-modern/src/components/Alerts/ThresholdsTableProxmoxBackupsSection.tsx) |
| Retained attempt reports | [Backup health model](../../../../internal/recovery/backup_health.go), [report endpoint](../../../../internal/api/reporting_backup_health.go) |
| Change truth and collection | [Canonical changes](../../../../internal/unifiedresources/changes.go), [change emission](../../../../internal/unifiedresources/change_emission.go), [presentation summary](../../../../internal/unifiedresources/change_presentation.go) |
| Change investigation UI | [Drawer state and table-mode boundary](../../../../frontend-modern/src/components/Infrastructure/useResourceDetailDrawerState.ts), [history state](../../../../frontend-modern/src/components/Infrastructure/useResourceDetailDrawerHistoryState.ts), [drawer history](../../../../frontend-modern/src/components/Infrastructure/ResourceDetailDrawerOverviewTab.tsx) |
| Alert handoff | [Investigate alert](../../../../frontend-modern/src/components/Alerts/InvestigateAlertButton.tsx), [resource query APIs](../../../../internal/api/resourceapi/resources.go) |

## Lessons from the comparison

| Job | PegaProx | ProxCenter | Useful direction for Pulse |
| --- | --- | --- | --- |
| Understand protection | Documents backup age against configured recovery point objectives and an experimental restore, boot and health-check workflow. | Lists archive verification state and time beside backup details and provider actions. | Explain freshness, provider verification and tested recovery separately, with evidence age and missing observations visible. |
| Inspect recovery evidence | Recovery plans connect readiness checks, dependencies, target resources and retained test evidence. A boot screenshot establishes a narrower fact than application health. | Brings archive details and restore actions into one backup workspace. | Keep the path from workload to evidence to the next appropriate tool short. Add recovery-test evidence before considering a recovery orchestrator. |
| See configuration changes | Documents baseline drift detection with alerts. | Documents before/after configuration diffs across guests, nodes, storage and firewall, with time and resource filters. | Extend canonical change records with safe semantic diffs and focused investigation, rather than adding an unrelated event store. |

Sources: [PegaProx feature scope](https://github.com/PegaProx/project-pegaprox#-features), [PegaProx recovery plans](https://docs.pegaprox.com/backup-ha-dr/site-recovery/), [ProxCenter backups](https://docs.proxcenter.io/operations/backups/), [ProxCenter change tracking](https://docs.proxcenter.io/operations/changes/). ProxCenter documents a five-minute configuration snapshot interval and possible loss of changes reverted between polls. Pulse should disclose equivalent collection limits wherever they apply.

## Demand and reliability prerequisites

The task timeline already has an accepted, bounded implementation contract in [BACKUP_TASK_TIMELINE_CONTRACT.md](../../../BACKUP_TASK_TIMELINE_CONTRACT.md). Its demand ledger entry is **Cross-system backup task timeline and outcomes** in `pulse-pro/FEATURE_REQUESTS.md`. This plan refines that work's priority and dependencies. It does not create a competing specification.

- [Issue 1812](https://github.com/rcourtman/Pulse/issues/1812) requests observed task timing and outcomes across a complex PVE/PBS estate. Treat this as evidence for the bounded task view, not blanket demand for arbitrary Linux schedules, lifecycle reconstruction, or maintenance orchestration.
- [Issue 1815](https://github.com/rcourtman/Pulse/issues/1815) contains a deliberate failed secondary-sync retest. The primary backup remained successful while the secondary sync failed. The stuck-running defect was closed after the reporter's retest. Showing the separate failed copy is the remaining product opportunity.
- [Issue 2292](https://github.com/rcourtman/Pulse/issues/2292) was closed after the reporter confirmed v6.5.0 corrected the backup badge. Preserve its identity and newest-backup regressions. Do not relist that resolved defect as a new gap.
- [Issue 2465](https://github.com/rcourtman/Pulse/issues/2465) records missed backup evaluation and a subsequent correction that retains existing alerts and exposes evaluation failure. Successful monitoring and visible failure remain prerequisites, not deferred feature work.
- [Issue 2587](https://github.com/rcourtman/Pulse/issues/2587) already has a PBS shortcut correction reported in v6.6.0-rc.1. Verify current behavior and delivery before scheduling more navigation work for it.

Reconciled on 18 October 2026: the #1815 backup-running correction `8c639846969c4f174aba874e7841df0ba0122de8` is included in published stable v6.5.0. Its stable-inclusion prerequisite is met. Current reliability work remains the active target, with remaining work and the manual exit evidence recorded in [the existing L1 delivery follow-up](records/maintenance-release-delivery-and-observation-2026-10-18.md). Trust defects found in matching, collection, evaluation, persistence or updates return to that work instead of being disguised as post-reliability enhancements.

## Priority and dependency order

Priority numbers below order post-reliability work. They are not release severities or commitments to dates. Effort is relative and includes backend truth, browser behavior, failure states and regression proof.

| Order | Outcome | Effort | Existing owner | Dependency |
| --- | --- | --- | --- | --- |
| 1 | Cross-system backup task outcomes | Medium | L15 storage and recovery, with monitoring and API contracts | Reliability prerequisite and the existing task timeline contract |
| 2 | Incident-focused investigation of nearby changes | Medium | L19 resource change intelligence, with alerts and Patrol | Existing canonical change history and incident/resource identity |
| 3 | One understandable workload protection assessment | Medium | L15, with alerts and reporting | Audit existing policy and identity authorities, reuse task evidence from 1 |
| 4 | Safe provider configuration diffs | Medium to large | L19, with monitoring adapters and security/privacy | Evidence contract and investigation flow from 2 |
| 5 | Retained recovery-test evidence and bounded readiness | Large, demand-dependent | L15, with L20 only if execution is later admitted | Protection assessment from 3, explicit test workflow and customer demand |

Orders 1 and 2 can be designed independently once reliability work permits. Land narrow slices with their own acceptance proof. Avoid a combined recovery-and-events redesign.

### 1 Cross-system backup task outcomes

**User outcome:** From Backups, identify a failed or incomplete secondary protection job without losing the truth that the primary backup succeeded.

Implement the existing contract through collection normalization, durable task storage, a bounded read API and a Tasks view beside By date and Coverage. Use provider-instance plus UPID identity and separate parent multi-guest tasks from child evidence. Keep task outcomes distinct from restorable artifacts. Start with backup, sync, verify, prune and garbage collection. Preserve source/destination identity only when directly evidenced.

**Acceptance:** A successful primary backup followed by failed PBS sync shows both outcomes and the affected destination. Running-to-terminal state survives polling, server restart and history pagination without duplicates. Permission-limited, truncated, disabled and not-yet-observed scopes cannot produce a clean success summary. Desktop shows useful task timing, narrow layout uses a readable activity list. Filters and time range survive reload and a shared URL. No elapsed-time-only stuck verdict, schedule inference or provider mutation enters this slice.

**Why first:** This is the strongest independent customer evidence, already scoped and already supported by collectors. It adds cross-source value that neither individual provider UI supplies for the whole estate.

### 2 Incident-focused investigation of nearby changes

**User outcome:** From an alert or unhealthy resource, inspect relevant changes around onset, follow a dependency and return to the same investigation context.

First provide an explicit way to request retained history from the platform row or alert context. Preserve the table's bounded network behavior. Fetch history for the investigation the operator opens, rather than enabling remote history fan-out across every row.

Build a focused investigation state on existing resource timelines. Preserve the incident, focal resource, bounded time range and filters in navigation. Support continuation beyond the latest 25 records and show the returned range, total and any retention or collection limits. Use canonical relationships to move between affected resources. Turn related-resource chips into scoped navigation where the resource is visible.

Default to a concise event summary and an evidenced next step. Move adapter details, confidence mechanics and raw metadata into expansion. Keep observed time separate from occurrence time. Explain why a related event is included. Temporal proximity alone cannot establish causality, and an empty observed window cannot establish that nothing changed.

**Acceptance:** An alert opens an anchored time window, an operator follows host to guest or container without losing it, inspects older events, and returns. Shared links restore that context. Event and time filters, pagination, access denial, missing related resources and expired history have explicit states. Test one host/VM/container chain and one backup/sync incident on desktop and narrow viewports. Patrol and Assistant receive the same resource and time scope without requiring a model call merely to browse evidence.

**Why second:** The current row flow does not itself retrieve retained changes. Resolve access to the existing evidence before expanding either the configuration audit or a broad lifecycle timeline. This extends the existing L19 residual.

### 3 One understandable workload protection assessment

**User outcome:** Explain what is protected, against which expectation, and what is still unproven, without reconciling badges, coverage, alerts and reports manually.

First document which canonical policy drives posture, artifact age, dashboard colors, alert thresholds and reports. Preserve legitimately different questions, but label them. Reuse existing resource identity, recovery posture and alert policy ownership. Do not add a competing frontend score or second protection state machine.

Show a short assessment with expandable evidence: latest unambiguously matched backup, expected maximum data age where configured, verification scope and age, secondary-copy outcome where observed, and recovery-test evidence when available. If no objective is configured, state the freshness rule being applied. A recently successful task alone cannot become an independent restore point. A snapshot alone cannot become a backup. Unreadable or ambiguous evidence remains unknown.

Make deletion protection explicit wherever an archive's retention lock shares the word Protected with workload posture. A retained archive, a recent backup, a verified archive and a successfully tested application recovery are four different facts. Prefer those concrete descriptions over another overall confidence badge.

**Acceptance:** The same workload and policy produce compatible explanations in the guest badge, Coverage, detail, alert and Backup Health report. Tests cover reused VMIDs across instances, namespaces, migration, deleted workloads, unknown clocks, failed collection and successful primary backup with failed secondary sync. Distinguish provider archive verification, successful boot and application verification. Each failing or unknown facet offers the appropriate evidence, connection repair, alert setting or native-provider link. Establish one policy extension point before adding per-workload or tag-based objectives.

**Why third:** Pulse already has much of the evidence. Consistency and a clear explanation create more immediate value than a new generic recovery dashboard. Introduce configurable objectives through the canonical policy only after its existing meanings are clear.

### 4 Safe provider configuration diffs

**User outcome:** See exactly which relevant configuration value changed, with its source and attribution limits.

Start with an agreed allowlist of provider-owned fields that matter to an observed investigation, such as guest CPU/memory allocation, disk attachment and network attachment. Extend canonical structured changes with bounded before/after values and source snapshot identity. Define redaction before persistence. Do not store full arbitrary configuration or raw API payloads. Record actor only from an authoritative task/audit source. Read failures and newly admitted resources establish baselines, not fabricated changes.

**Acceptance:** An allocation or attachment change produces one understandable diff after restart and duplicate collection. Unchanged snapshots and failed reads produce none. Secrets and unbounded values cannot enter the record or export. Poll interval and baseline/retention limits are visible. A filterable semantic diff is available from the incident flow. Actor is clearly unknown when the provider does not supply it. Broaden field coverage from actual user investigations rather than attempting every provider setting at once.

**Why fourth:** This is ProxCenter's clearest documented advantage in the change flow. It requires collector and evidence work, not merely a new timeline layout.

### 5 Retained recovery-test evidence and bounded readiness

**User outcome:** Know when a recovery procedure was last exercised, which recovery point it used, what was checked, and what remains untested.

Begin with evidence from an explicit external or operator-run test attached to the canonical workload and exact recovery point. Record test time, target, checks, outcome and evidence provenance. Distinguish operator attestation from independently observed checks. Reuse existing service availability checks when they genuinely observe the recovered service. Show unavailable evidence without inventing a readiness score.

**Acceptance:** A test receipt identifies its backup and target, survives restart, and ages visibly. Replaced or removed recovery points do not retain a current-looking test result. A boot screenshot cannot imply database consistency or application health. Reports describe the same checks and limitations as the workload view. Before any automated test execution, separately establish isolated networking, target capacity, encryption-key access, approval, cancellation, cleanup and failure recovery through existing action governance.

**Why later:** PegaProx makes tested recovery tangible, but its backup verification workflow is experimental. Pulse should establish demand and evidence semantics before accepting restore, boot or failover side effects.

## Explicit deferrals

- Full site failover, automatic failback, rolling updates and DRS are separate operational products with substantial execution obligations.
- A universal infrastructure lifecycle page remains subject to its existing demand-ledger disposition. The focused incident journey does not authorize reconstructing events Pulse never observed.
- Arbitrary Linux timer discovery, schedule optimization, provider schedule editing and ordered maintenance runbooks remain outside the bounded task timeline contract.
- A new archive browser, dashboard score or parallel event database does not resolve the gaps above.

## Governance and activation

The existing roadmap owns pickup of this sequence:

| Canonical record | Tracked scope |
| --- | --- |
| Planned target `v6-product-lane-expansion` in [control_plane.json](../../control_plane.json) | Links this plan for consideration after `v6-release-reliability` explicitly yields priority. The reliability target remains active. |
| L15 follow-up `storage-recovery-post-rc-hardening` in [status.json](status.json) | Orders 1, 3 and 5, with the existing task timeline contract and a separate decision before recovery-test execution. |
| L19 follow-up `resource-change-intelligence-post-rc-hardening` in [status.json](status.json) | Orders 2 and 4, starting with access to retained history before richer configuration diffs. |
| Existing demand entries in `pulse-pro/FEATURE_REQUESTS.md` | Cross-system backup task timeline and outcomes links order 1. Infrastructure lifecycle event timeline distinguishes the bounded L19 investigation work from its broader, still-watching request. |

L15 and L19 keep their existing planned follow-ups and bounded-residual lane states. L20 governs any later expansion of executable actions. Demand statuses and signal counts are unchanged. The audit and roadmap direction are not additional independent customer signals.

When reliability work is explicitly ready to yield priority under [the recorded manual exit decision](records/maintenance-release-delivery-and-observation-2026-10-18.md#exit-decision-and-feature-pickup), activate one bounded slice through the existing target and demand process. Reconcile the relevant ledger entry and contract, reserve its claim, and name its source owner and proof obligations before implementation. This tracked sequence is not evidence that the reliability target has completed and is not independent authority to start every item.

## Browser audit evidence

The baseline was built from the source revision above with locked frontend dependencies and the ordinary backend build, then exercised on an isolated mock-data server. Desktop was 1440 by 1000 and narrow was 390 by 844. This establishes current UI behavior with controlled evidence, not live provider correctness, restore safety or release qualification. The development binary's version label was not used as release evidence.

| Journey or state | Browser observation | Planning consequence |
| --- | --- | --- |
| Backups to Coverage and expanded workload | Coverage has seven desktop columns. From Proxmox Overview, Backups, Coverage and one workload expansion reach the evidence in three actions. The source distinction and failure explanation are already useful. | Keep this entry point and model. Put the protection answer ahead of secondary server telemetry. |
| Failed, verified, task-only and unknown workload evidence | Expanded `checkout-web-02`, `postgres-primary-01`, `smtp-relay-01` and `backup-orchestrator-01`. A task marked OK did not make task-only evidence protected. A verified backup and local snapshot were separately identified. | Preserve these distinctions in task history, assessment and reporting. |
| By date | The date view lists PBS snapshots, PVE files and guest snapshots separately, with creation time and individual artifact state. | Extend the investigation path instead of adding a duplicate archive browser. Clarify artifact protection wording. |
| Narrow Coverage | The page stayed within 390 pixels and row-tap expansion worked. The server table and explanatory paragraph preceded workload results. Mock-only banners also consumed space, so their height is not a production layout finding. | Prefer a compact explanation and progressive detail. Keep touch row activation and keyboard disclosure behavior. |
| Posture unavailable | A deliberately intercepted 503 preserved restore-point evidence, marked workload posture unknown and said Pulse would not guess protection. | Retain this fail-closed behavior. |
| PVE and PBS inventory unavailable | Deliberately intercepted 503 responses produced an incomplete-inventory explanation and Refresh action, rather than an empty success view. | Preserve the distinction between no evidence and a failed read. |
| Selected Proxmox guest | `/proxmox?resource=vm-239701ced1f966f0` opened the seeded guest's current state and metrics. No retained-history request or change-history section appeared in the inspected row. The separate source check confirms table-mode remote history is disabled and snapshot evidence is only a fallback. | Add an explicit, bounded route to retained changes. Do not infer that the workload had no changes. |
| Alert History and incident evidence | `/alerts/history` opened a real incident panel with lifecycle filters, a timestamped event and expandable source/record evidence. This answers the incident's own lifecycle, not a time-aligned history of surrounding provider configuration changes. | Reuse the incident anchor and evidence disclosure in the focused change investigation. |

The generic drawer's 25-record cap, filter mechanics and plain related-resource chips are source-confirmed findings. They were not qualified as a working historical browser journey through the selected guest row. That distinction is why an accessible investigation entry point ranks ahead of richer diffs. Permission, retention and real provider behavior still require implementation-specific acceptance evidence when a slice is built. No production restore, provider configuration change or model-run investigation was performed.
