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

This boundary is deliberate: preserving an explicit reporter declaration is
safe to automate, while deciding whether two observations are one root cause is
a product and technical judgment.
