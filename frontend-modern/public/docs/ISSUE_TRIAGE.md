# Issue Triage and Topic Integrity

Pulse issue triage must preserve every actionable topic a reporter contributes.
Resolving the primary defect does not dispose of secondary bugs, feature requests,
documentation gaps, or operator workflows described in the same report.

## Intake contract

Issue forms ask for one primary outcome and provide an **Additional actionable
topics** field. Declaring a topic when opening an issue or newly adding one to
this field applies the `needs-decomposition` label automatically. This is a
queue-integrity signal, not a statement that every topic will be built.
The synchronizer never removes this label, nor restores it after a maintainer
has cleared it when an unrelated edit or reopen occurs. A later `None` in the
form does not prove that topics raised in comments were dispositioned; triage
removes the label only after the linked dispositions below are complete.

Reporters may still write free-form issues, edit form output, or discover a
second topic during discussion. Triage owns decomposition in those cases; it
must not require the reporter to refile information they already supplied.

The bug and pre-release forms ask for the version on the failing running
instance, not merely the version before an upgrade. If installation failed
before Pulse started, the attempted version or release asset is the relevant
version evidence; "unknown" is more honest than claiming a running version.
For failed installs, retain the public installer/helper source when supplied,
and distinguish a third-party helper failure from an official Pulse installer
failure. Never request a command line containing a token or other secret. A
running image reference applies only to container installs; bare-metal and LXC
reporters must not have to attest to one. Ask for evidence relevant to the
symptom: logs or sanitized diagnostics for connection and data failures, and
screenshots or exact error text for visual failures. Do not make a reporter
invent logs where none apply.

For upgrades and regressions, keep the affected Pulse server version, agent
version and monitored platform release separate. The pre-release form covers
upgrades from stable v6 as well as earlier previews. Its optional **Last known
working Pulse version** is a baseline only when the reporter observed the same
behaviour working; an upgrade's starting version alone does not establish that.
An API-only connection can have no agent. Accept "unknown" or an omitted optional
field, and use existing evidence; do not ask for a downgrade, restart, reinstall
or re-enrolment to obtain version context. Existing reports need no refile.

The optional **OS / environment** field in both bug forms also distinguishes
where readings come from: the platform API, a Pulse agent, or both. Retain the
affected platform release, including TrueNAS SCALE versus CORE, separately from
the Pulse and agent versions and the server OS. Different collection paths can
supply different readings for the same target; a working agent view does not
prove that the API view recovered. Use existing settings and observations;
blank or "unknown" is valid. Do not request diagnostics, a probe, restart or
connection change just to fill this context. Existing reports need no refile.

Keep the Pulse server's installation separate from the affected target. A
Docker-hosted Pulse server can monitor a VM, LXC or NAS; its installation type
does not make Docker commands relevant to that target. Read later comments as
well as the original title and screenshots before choosing diagnostics. When a
report shifts to a different target, retain both topics rather than assuming
the new symptom is the original defect or a duplicate.

For reports involving several hosts or clusters, preserve which nodes share a
cluster, which belong to independent installations, whether node names or guest
IDs (VMIDs) repeat, and whether they share a backup destination. Display names
and VMIDs are not globally unique; a distinct Pulse display name alone does not
prove that every reading is isolated. Use consistent aliases for private names
and addresses across the description, screenshots and logs, preserving which
values repeat: cluster A/node 1 and standalone B/node 1 are distinct targets with
the same native node name. Do not infer that a backup or agent belongs to one
installation from a matching name or VMID alone, or classify a new report as a
duplicate just because those identifiers overlap.

Use relationships already supplied in the full thread and attachments. Unknown
relationships remain unknown; do not ask for public hostnames, addresses or a
configuration dump, or add, rename, remove or re-enrol anything to obtain this
context. Existing reports need no refile. Retain every affected surface (for
example, node errors, backup status and Docker monitoring); recovery in one does
not establish recovery in the others.

Keep the connection Host/URL, Pulse display name, native node/guest name and
agent-reported hostname separate. A full connection hostname does not establish
that guest or agent names are fully qualified. If the supplied evidence leaves
that distinction consequential, ask only for the remembered short/full form
and whether full domain suffixes differ, not the literal names; unknown is valid.
Retain which guest has an agent and whether it remained connected when its
Docker inventory disappeared. Only one guest having an agent does not make a
shared guest name unique across the platform inventory. An ambiguous automatic
link is not proof that agent reporting stopped. Do not install another agent,
change a link or recreate the addition just to diagnose it; a repair for distinct
full hostnames alone does not establish a fix for repeated short names.

Missing readings and an unresponsive workload need different investigation and
recovery paths. Use the reporter's existing observations of the workload's usual
UI, not Pulse's displayed connection status, to distinguish them where possible.
Responsiveness is not proof that every filesystem is writable or the workload
is healthy. Accept "unknown" when it cannot safely be determined; do not ask for
another update, backup or guest-agent probe to fill the gap. Existing reports
need no refile: ask only for a consequential distinction not already supplied.

Some failures cannot safely be reproduced: an update may have changed a
container despite a failed banner, an alert storm may send more notifications,
or another run may bring down a host. Accept the original sequence, observed
result, time and sanitized evidence; do not require a second attempt before
triage. Where the installed state is uncertain, check it before suggesting
another action. A diagnostics export is useful only when Pulse is running and
collecting it is safe.

For performance reports, retain what a reading measures before comparing it:
Pulse process, container or whole host; units, measurement window and uptime;
and the relevant CPU allocation or memory limit, fleet, polling and open
dashboards. A process-start CPU average is not a recent window, and database
size is not a write rate. The evidence field asks for these distinctions only
where known, without making new collection a condition of reporting. Existing
screenshots or an unavailable reading are valid evidence. Do not request raw
profiles, heap dumps, databases or full process command lines in a public
thread; use locally reviewed counter summaries, without restarting, creating
load or changing polling or retention just to measure.

For VM or LXC memory disagreements, retain where each existing reading appears
(overview, History, alert, Patrol or another tool), whether the comparison was
measured inside the affected guest or on its hypervisor node, and the original
times, units and selected memory source or sample age if already known. Use
consistent private aliases; unknown is valid, and existing reports need no
refile. Compare the same target and episode before inferring a discrepancy.
Available memory, buff/cache and a process's RSS measure different things:
RSS alone is not total guest usage, and a cache-inclusive hypervisor footprint
does not establish guest pressure. A Proxmox VM API connection may still obtain QEMU guest-agent (QGA) readings
without a Pulse agent.

Keep the displayed number, its qualification and the resulting finding separate.
An unexplained or retained reading is not measured zero or verified recovery.
Today's live memory source cannot establish the source of a historical sample;
do not reattribute History or a past warning from the current view. Reconcile
the full thread before asking only for a consequential remaining distinction.
Do not request commands, Diagnostics or guest-agent probes, an agent install,
memory or workload changes, or another backup just to fill this context.

For notification reports, distinguish an already-observed **Test** result from
ordinary alert delivery: single, grouped/digest or resolved. A successful Test
does not establish ordinary delivery or correct identity. Retain the destination
type, built-in or custom template, original time and redacted error or missing
host/resource context where already known; consistent private aliases preserve
which hosts or resources repeat. Keep an alert appearing in Pulse, its queued
attempt and the recipient's actual message as separate observations. Delivery
can succeed while the subject or body still identifies the wrong resource.

Use existing messages and queue details, including earlier comments, rather
than asking for the same facts again. Queued messages can retain older settings;
a current configuration screenshot does not establish the settings of an older
attempt. Unknown evidence stays unknown, and existing reports need no refile.
Do not request another Test, induced alert, queue retry/replay, queue clearing or
notification-setting changes just to complete a report. Keep destination
addresses, webhook URLs, chat IDs, tokens and full notification settings or
payloads private, even when an error or screenshot contains them. A missing
notification alone does not prove that alert evaluation failed; retain both
symptoms when the thread supplies evidence for each.

Treat webhook `response body withheld` or `details withheld` messages as
intentional privacy boundaries, not missing evidence to request from a reporter.
Use the supplied HTTP status, failure class and time; a response-read failure
does not prove the receiver rejected the request. A Test's bounded response byte
count is not the sent payload size or proof of receipt. If a provider-specific
distinction matters, use an existing, locally reviewed error code or redacted
explanation, never a raw response, Debug capture or replay. Historical errors
and other exports are not retroactively scrubbed; the full entry remains private.

## Required disposition

Before removing `needs-decomposition` or declaring a mixed report triaged:

1. Enumerate each independently actionable topic in the issue body and comments.
2. Keep the original issue focused on its primary reproducible problem.
3. Give every other topic one linked disposition:
   - a new or existing issue for a distinct defect or independently actionable
     product request;
   - a demand-ledger entry for evidence that is useful but not yet build-ready;
   - a Discussion or support path when there is no reproducible defect or
     product decision to track;
   - an explicit decline, with the reason, when the topic conflicts with Pulse's
     product or safety boundaries.
4. Use GitHub sub-issue relationships when separate issues share the same source
   report and the authenticated mutation path supports them. A plain backlink
   remains in each child body so the evidence survives clients that do not
   render sub-issues and remains the fallback for bounded bot identities.
5. Post a concise topic-to-disposition summary on the source issue. Never say a
   topic is “recorded” without naming where it is recoverable.

Decomposition does not multiply demand. Every child points to the same reporter
and source thread, and the demand ledger counts that as one signal per capability.
Do not copy credentials, private diagnostics, or personal data into a child
issue; summarize only the minimum sanitized evidence needed to preserve the
operator problem.

## Automation boundary

The label synchronizer detects the structured form field deterministically and
fails quietly for legacy forms. It does not use keyword heuristics to invent
topics or create issues automatically. A maintainer or triage agent reviews the
source context, chooses the correct destination, creates links, and is
accountable for the disposition.

Before changing metadata, the synchronizer reads the current issue body and
labels, rather than classifying a queued event's older report. A failed read
stops the job without a stale-data fallback; closed issues and pull requests are
not changed. It applies only individual label additions and removals, preserving
unrelated and community-owned retest labels. Only a still-current declaration
event can add `needs-decomposition`; a delayed event cannot restore a task that
the maintainer has since cleared. Comments and superseded declarations still
need whole-thread triage, not an automatic disposition.

Version classification does not need the latest release and cannot establish
relevant-fix availability. The synchronizer makes no release lookup, public
comment, retest request or closure.

An explicit **Pulse version** field is authoritative even in a legacy inline
or standalone report. An unknown or incomplete value must not be replaced by
an upgrade's starting version in the title, a nearby agent/platform version or
a hidden template example. A standalone legacy field uses only its first
visible value, not later fields, logs or prose. A `needs-version-info` label is
not proof that the full thread lacks the version; read existing evidence before
asking only for a consequential remaining distinction. No refile or new
diagnostics are required by version classification.

This boundary is deliberate: preserving an explicit reporter declaration is
safe to automate, while deciding whether two observations are one root cause is
a product and technical judgment.
