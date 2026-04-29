# Paid Feature Claim Proof Matrix

Date: 2026-04-29

## Decision

Self-hosted v6 paid claims are release-gated by an executable proof bundle. The bundle verifies that Community, Relay, and Pro claims line up across public customer-facing copy, the canonical licensing contract, history entitlement enforcement, frontend plan copy, Pro admin-extra runtime behavior, public checkout/pricing, download authorization, and Relay runtime entitlement behavior.

## Proof Command

```bash
python3 scripts/release_control/paid_feature_claims_proof.py --json
```

## Required Coverage

- Community keeps core monitoring free and does not introduce monitored-system volume caps.
- Relay claims only remote web access, mobile pairing, push notifications, and 14-day metric history.
- Pro preserves Relay capabilities and adds operator extras: root-cause analysis, safe remediation workflows, 90-day metric history, RBAC, audit logging, reporting, SAML SSO, and agent profiles.
- Public app, docs, and site copy must not reintroduce old self-hosted monitoring-limit, higher-limit, default-trial, hosted-model-credit, or bundled-Patrol-credit claims.
- Relay must not be marketed as including a customer-specific `*.pulserelay.pro` URL until that product surface exists; v6 GA Relay is the standard outbound relay service.
- History claims are enforced by the runtime metrics-history API, not only shown in copy.
- Pro admin extras are backed by concrete core and enterprise runtime tests for RBAC, audit logging, reporting, SAML SSO, and agent profile behavior.
- Public pricing, checkout, download, and relay-server entitlement behavior remain consistent with those claims.

## Final Paid-User Value Audit

Verdict: Pulse Pro v6 is not a rug pull for existing paid Pulse Pro users, provided the public story stays focused on the features that are actually delivered and proved. The v6 paid package should be described as optional operating value on top of free self-hosted monitoring, not as a monitoring-capacity tier.

What paid users keep or gain in v6:

- Existing v5 recurring Pro customers keep the existing recurring price while subscription continuity is maintained.
- Legacy paid installs can migrate into the v6 activation model without buying monitoring capacity again.
- Self-hosted monitoring and child-resource volume are not metered under the current v6 self-hosted packaging.
- Relay-capable paid users have the real Relay/mobile/push capability set.
- Pro users have 90-day history, alert-triggered root-cause analysis, safe remediation workflows, and the admin extras bundle: Advanced SSO, RBAC, audit logging, PDF/CSV reporting, and centralized agent profiles.

What must not be sold as v6 Pro value:

- monitored-system volume or "higher monitoring limits"
- a self-hosted Pro trial CTA
- hosted AI/model credits or hosted Patrol quickstart credits
- customer-specific Relay URLs
- Kubernetes AI, scheduled remediations, incident memory, or execution audit trails as headline Pro features unless they are separately productized and proved

## Claim-To-Proof Checklist

| Paid claim | Runtime/source proof | Surface proof |
|---|---|---|
| Core self-hosted monitoring stays free and unmetered | `pkg/licensing` tier contracts and `internal/api` entitlement payload tests omit self-hosted volume caps for Community, Relay, Pro, and legacy continuity. | `tests/integration/tests/58-self-hosted-trial-rate-limit-ui.spec.ts` and `tests/integration/tests/59-self-hosted-plans-entitlement-summary.spec.ts` prove the app does not present monitored-system cap upsells in normal self-hosted flows or paid plan summaries. |
| Relay provides remote web access, mobile pairing, push, and 14-day history | `pkg/licensing` grants `relay`, `mobile_app`, `push_notifications`, and `long_term_metrics`; `internal/api` enforces tier-aware metrics history; `pulse-pro/relay-server` rejects missing Relay entitlement. | Relay copy in `docs/PULSE_PRO.md`, `frontend-modern/src/utils/selfHostedPlans.ts`, `pulse-pro/landing-page/index.html`, and `pulse-pro/license-server/public_pricing.go` is covered by the static copy audit. |
| Pro adds root-cause analysis and safe remediation workflows | Licensing grants `ai_alerts` and `ai_autofix`; `internal/api` and core AI tests cover gated analysis/remediation paths. | Plan copy and Pro value proof list the features as primary Pro capabilities; stale hosted-model/trial claims are forbidden by the copy audit. |
| Pro includes 90-day history | `TierHistoryDays[pro] == 90`, `max_history_days` is emitted in entitlements, and metrics-history API tests enforce history ranges. | `HistoryChart` and `ProLicensePanel` frontend tests verify entitlement-aware history presentation. |
| Pro includes admin extras | API and enterprise package tests cover Advanced SSO, RBAC, audit logging, reporting, and agent profile behavior. | `tests/integration/tests/59-self-hosted-plans-entitlement-summary.spec.ts` proves the advertised Pro settings sections and SAML/agent-profile controls are reachable when Pro capabilities are active. |
| Existing v5 Pro customers keep paid continuity | Migration and auto-exchange tests prove legacy Pro/Lifetime plans migrate to active v6 entitlements. | The self-hosted plan summary foregrounds grandfathered price continuity and explains that monitoring volume is not metered under current v6 policy. |

## Release Recommendation

Stop adding new self-hosted Pro features before GA. The current package is defensible if the release keeps the narrow, honest story:

1. Community is the free monitoring product.
2. Relay is the optional convenience/support-style tier for remote access, mobile pairing, push, and 14-day history.
3. Pro is the operator tier for investigation, safe remediation, 90-day history, and admin/compliance extras.

The next Pro work after GA should deepen these pillars rather than adding unrelated paid surfaces. Good candidates are better incident packages, stronger reporting, clearer remediation review/audit workflows, and richer team/admin controls.

## Fresh Proof Commands

```bash
python3 scripts/release_control/paid_feature_claims_proof.py
PULSE_E2E_SKIP_DOCKER=1 npm --prefix tests/integration test -- tests/59-self-hosted-plans-entitlement-summary.spec.ts --project=chromium
```
