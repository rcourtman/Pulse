# Issue Triage and Topic Integrity

Pulse issue triage must preserve every actionable topic a reporter contributes.
Resolving the primary defect does not dispose of secondary bugs, feature requests,
documentation gaps, or operator workflows described in the same report.

## Intake contract

Issue forms ask for one primary outcome and provide an **Additional actionable
topics** field. Entering anything other than `None` applies the
`needs-decomposition` label automatically. This is a queue-integrity signal,
not a statement that every topic will be built.

Reporters may still write free-form issues, edit form output, or discover a
second topic during discussion. Triage owns decomposition in those cases; it
must not require the reporter to refile information they already supplied.

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
