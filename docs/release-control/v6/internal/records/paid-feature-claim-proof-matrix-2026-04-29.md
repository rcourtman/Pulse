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
