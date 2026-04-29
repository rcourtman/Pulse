# Paid Feature Claim Proof Matrix

Date: 2026-04-29

## Decision

Self-hosted v6 paid claims are release-gated by an executable proof bundle. The bundle verifies that Community, Relay, and Pro claims line up across the canonical licensing contract, history entitlement enforcement, frontend plan copy, public checkout/pricing, download authorization, and Relay runtime entitlement behavior.

## Proof Command

```bash
python3 scripts/release_control/paid_feature_claims_proof.py --json
```

## Required Coverage

- Community keeps core monitoring free and does not introduce monitored-system volume caps.
- Relay claims only remote web access, mobile pairing, push notifications, and 14-day metric history.
- Pro preserves Relay capabilities and adds operator extras: root-cause analysis, safe remediation workflows, 90-day metric history, RBAC, audit logging, reporting, SAML SSO, and agent profiles.
- History claims are enforced by the runtime metrics-history API, not only shown in copy.
- Public pricing, checkout, download, and relay-server entitlement behavior remain consistent with those claims.
