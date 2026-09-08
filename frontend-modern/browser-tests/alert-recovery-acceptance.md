# Alert recovery and occurrence acceptance

This is a manual evidence checklist, not a passing receipt or new product scope.
Use an authorised synthetic installation and destination; never disrupt customer
monitoring or retain credentials, destination URLs, or customer payloads.
Run full builds and browser runners through `pulse-heavy-run -- <command>`.

## Identity before interaction

Record UTC time, public source SHA, private source SHA where applicable, version,
immutable image digest or installer artifact identity, browser/version, viewport,
and installation mode. Record the actual installed identity, not just a workflow
SHA. Keep before/after upgrade identities. Mark unavailable checks **not run**.
A main-build result does not qualify a release-line candidate.

The 8 September 2026 beta.2 preparation selects public
`a3d5a65031a1206dd6cc4249e886950a7d312dbc`; this is a source target, not evidence
of publication, installation, or private-pair compatibility. Reconcile any later
candidate change before executing. Main-only recovery-feedback changes require
separate identity and results.

## Installed ordinary event and recurrence

1. On a synthetic resource, trigger a real threshold alert, not the destination's
   test-send button. From the overview, follow the failed destination into alert
   delivery details. Record the resource, alert and occurrence identifiers with
   firing time, visible state and failed-attempt time using synthetic identifiers.
2. Repair the controlled destination failure and use the supported retry path.
   Match the actual received firing message to that resource and occurrence.
   A successful HTTP response alone does not prove correct received content.
3. Acknowledge the occurrence and restore the monitored resource. Match the
   resolved message and visible resolution time to this first occurrence.
   Destination recovery alone must not be treated as resource recovery.
4. Retrigger the same alert after resolution. Record the new occurrence and
   firing time. The first occurrence must remain resolved with its own
   acknowledgement history; the second must not inherit its resolution.
5. Restart the authorised synthetic installation. Reload history in both the
   desktop inline view and narrow resource drawer. Check both occurrences,
   timestamps, acknowledgement ownership and current state against the retained
   before-restart evidence. Resolve the second occurrence and verify its receipt.
6. Where a supported synthetic lifecycle replay fixture exists, replay a delayed
   event for the first occurrence while the second is active. It must not close
   or merge the second. If no supported fixture exists, mark this subcase not run;
   do not invent a public replay endpoint or edit the database to create evidence.

Retain sanitised screenshots and destination receipts plus observed results for
each step. Separate delivery failures from history/projection failures. Do not
infer coverage of every resource type from one synthetic resource.

## Main-build feedback and assistive technology (separate result)

Use a browser with an actual screen reader and record both versions. Exercise
Retry and Dismiss failures while unrelated settings contain unsaved edits.
Confirm changed errors are announced, edits remain, and keyboard focus is not
stolen. Clear the recovery message with the keyboard and confirm a usable focus
destination and onward navigation. Repeat a failure to check that subsequent
messages are announced without duplicate atomic announcements. Record what was
heard, not merely the presence of `role=status` in the DOM.

For incident-history read failures, verify initial failure and successful retry,
then a failed refresh with cached history retained, followed by a genuinely empty
successful response. Check both inline and narrow drawer presentations. Distinguish
an unavailable read from “no incidents”.

[W3C status-message guidance](https://www.w3.org/WAI/WCAG22/Understanding/status-messages.html)
explains notification without focus changes and recommends user testing against
excessive announcements. DOM tests support this assessment but cannot establish
spoken output. These checks do not claim WCAG conformance or installed delivery
qualification on their own.

## Receipt summary

For each section report: exact identity; command or manual steps; pass/fail/not
run per step; sanitised evidence location; observed defect; unresolved dependency.
Do not close a reporter's issue or call a release qualified from this checklist
alone. Preserve adverse results when a later attempt succeeds.
