# Mobile Product Purpose GA Blocker - 2026-04-24

## Classification

- Type: release gate
- Blocking level: release-ready
- Lane: L5 Mobile go-live readiness

## Trigger

During physical iPad proof review on 2026-04-24, the product owner stated that the actual Pulse Mobile experience does not make clear what the app brings to a normal self-hosted Pulse user or what the app is for.

The product owner further clarified that normal self-hosted users are unlikely to think of Pulse Mobile as a control center for approving commands. Pulse is understood first as a monitoring application, so a mobile app that does not lead with monitoring will confuse users even if approval and recovery flows are technically useful.

The product owner then paused the monitoring-first assumption as well: because the web app has already been optimized for mobile, Pulse Mobile may not need to duplicate monitoring for broad self-hosted users. The product role may instead be business or multi-tenant operations, Relay-backed remote instance access, notification continuity, approvals, or another narrower job. The mobile product purpose is therefore an open decision, not merely a UX polish issue.

## Judgment

The current mobile candidate may be technically release-capable, including physical-device pairing, APNs delivery, tap-through routing, approval actions, reconnect, and store configuration evidence, but that does not make it product GA-ready.

Pulse Mobile public rollout is blocked until the in-app experience itself explains and proves the companion-app value for self-hosted operators. The target product shape must be monitoring-first: current estate status, live/stale/offline signal, active alerts or findings, and clear "nothing needs attention" states should be the visible center of gravity. Approval handling and recovery from notification or deep-link context are valuable secondary jobs, but the app must not read as a command-approval console or internal control center.

Because the owner has not yet chosen the intended mobile audience and job, the monitoring-first shape is a candidate direction rather than a locked design. The product decision must be made before implementation work treats the current app structure as GA-bound.

## Exit Criteria

- The app has an explicit product role rather than feeling like a thin or unexplained subset of desktop Pulse.
- Monitoring is the primary visible mobile value: a paired self-hosted user can quickly see current Pulse health, attention state, and whether the phone's view is fresh enough to trust.
- Approval, command, and recovery surfaces are positioned as secondary actions that follow from monitoring context rather than as the apparent reason the app exists.
- Empty and all-clear states still feel useful as monitoring states, not like dead tabs waiting for approvals or commands.
- First-run, unpaired, paired, empty, alert, approval, and recovery states make the next useful action obvious to a normal self-hosted operator.
- The first screen after pairing communicates current estate state and why opening mobile is useful.
- A physical-device walkthrough demonstrates that a user can understand the app purpose and primary jobs without release-team narration.
- Technical readiness evidence remains current on the candidate being promoted.

## Open Product Decision

Before GA, choose the Pulse Mobile product role and audience:

- Broad self-hosted companion: monitoring-first mobile Pulse for users who want native push, quick status, and light triage.
- Relay remote-access app: native shell for securely reaching a self-hosted instance through Pulse Relay when the site is not directly reachable.
- Business or multi-tenant operator app: MSP/team-facing estate triage, customer switching, incident context, and approvals.
- Notification/action utility: a narrow app primarily for push delivery, deep links, identity, and safe approval handoff back to the web app.
- No GA mobile app yet: keep current mobile work as internal/TestFlight infrastructure until a customer-visible mobile job is proven.
