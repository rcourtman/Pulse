# Recovery feedback qualification — 7 September 2026

Run from the repository root:

```sh
pulse-heavy-run -- node scripts/check-recovery-feedback.mjs
```

The final run passed 12 cases: real OverviewTab and DestinationsTab, shared
Card/Button/ToastContainer, 1440/900/390px, light and dark. Source digests and
the result are adjacent; the exact changed-source receipt is
frontend-modern/browser-verification.json. Screenshots show retained feedback
after the final toast expires. Representative narrow, intermediate and desktop
screenshots were inspected for readable feedback and reachable controls.

The harness exercises keyboard Retry/Dismiss and native confirmation,
cancellation without mutation, superseding actions, ten-second error-toast
expiry (accelerated browser clock plus exit-animation timer), successful
mutation followed by failed health read, subsequent health refresh, and
keyboard clear with retained focus. On Destinations a healthy refresh removes
the health warning without removing action-failure information. Unit tests
add healthy reconciliation independently of the available Overview controls,
callback throw/rejection, and older completion versus clear/newer ownership.

54 focused tests passed across useNotificationDeliveryHealth,
AlertDeliveryHealthCard, useAlertDestinationsTabState,
OverviewTab.deliveryactions and OverviewTab.emptystate. TypeScript and
changed-file ESLint passed. No full suite, build, installed notification
delivery, recipient receipt, app-shell navigation, or screen-reader testing
is claimed.

## Failed harness attempts, retained rather than counted as passes

1. Initial run timed out waiting for Retry. The API route stub matched
   /src/api modules as well as application API requests, leaving a blank page.
2. Diagnostic rerun confirmed an empty body and no pageerror event; failed.png
   retains that image. The in-flight script had loaded before the correction.
3. With the route constrained to the fixture origin's /api/ prefix, Overview
   rendered and passed, but the destination Refresh selector matched both
   health and activity controls.
4. A remaining healthy-reconciliation Refresh selector had the same ambiguity.
   Both selectors now identify the health card, not the first matching button.
5. The full destination render exposed a missing required email 'to' array in
   the fixture (undefined.join). The fixture now supplies complete email and
   Apprise settings and a correctly shaped empty delivery log.

The final rerun passed all cases with zero page errors. No runtime source was
changed to suppress these harness failures.

## Evidence and boundaries

The current demand-ledger named bet, “Recovery feedback without a reading
deadline” (pulse-pro FEATURE_REQUESTS.md, introduced by 9c25eaf1), authorises
this main-only scope. Fresh W3C guidance read on 7 September distinguishes
temporary information with an untimed equivalent from information lost at
expiry:
https://www.w3.org/WAI/WCAG22/Understanding/timing-adjustable.html
Its status technique supports an existing status container and explicit
atomic announcement semantics:
https://www.w3.org/WAI/WCAG22/Techniques/aria/ARIA22

This is not a WCAG verdict or a fix for the reporter's HTTP503. Feedback is
view-local: clear, a newer confirmed action, leaving the view or reload can
remove it. Queue acceptance is not delivery receipt. The optional activity
callback is tested separately; the current destination log loader already
handles its own read failures.
