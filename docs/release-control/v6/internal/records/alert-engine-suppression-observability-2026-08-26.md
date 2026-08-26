# Alert engine suppression observability and lifecycle trust

Recorded: 2026-08-26
Origin: full alerting audit (interactive session, 2026-08-26) covering
`internal/alerts`, `internal/notifications`, the Alerts UI, and ~60
alert-related GitHub issues.

## Gap

Three related defects in the alerting trust surface:

1. **Suppression decisions are unobservable.** Eight-plus noise mechanisms
   (cooldown, max-per-hour, suppression window, minimum delta, flapping,
   grouping, quiet hours, activation state) can hold or drop a notification,
   and no surface records which one did, when, or why. The delivery log
   records attempts only. Users experiencing held notifications read the
   silence as breakage (issues #1159, #1444, #937, #980 are this class).

2. **The delivery-diagnosis endpoint has no consumer.**
   `GET /api/alerts/delivery-diagnosis` projects exactly why a given active
   alert would or would not notify (`AlertDeliveryDiagnosis` in
   `internal/alerts/notification_policy.go`), and no frontend code calls it.
   The most common alerting trust question — "why didn't I get notified?" —
   is answerable by the backend and invisible to users.

3. **Alert lifecycle transitions have no durable event trail.** History
   stores merged alert snapshots, not transitions; several lifecycle
   regressions shipped undetected because the end-to-end contract
   (config save ⇒ zero notifications #1682; N grouped alerts ⇒ N rendered
   lines #1683; suppressed firing ⇒ no resolve #1553) is asserted nowhere
   durable and observable.

## Proposed resolution

Phase 0 of `docs/ALERT_ENGINE_EVOLUTION.md`: an additive append-only alert
event log (SQLite, alongside existing behavior — no lifecycle changes),
manager wiring that records transition and suppression events with reasons,
an API to read them, and UI surfacing of the delivery diagnosis plus
held-notification rows. Later phases (shadow reducer, family-by-family
cutover, rule model) are scoped in the same doc and are not part of this
gap's resolution.
