# Pulse Monetization (Self-Hosted + Hosted)

This document describes what Pulse sells and how feature entitlements map to license tiers.

Source of truth for runtime gating is `internal/license/features.go` (tier -> feature keys). If this file
and any marketing/in-app copy disagree, the code wins and the copy must be updated.

## Tier Overview

Pulse uses capability keys (feature flags) to gate paid functionality. Some capabilities are additionally
gated by non-license mechanisms (for example, AI autonomy level).

### Community (Free, Self-Hosted)

Included capabilities:
- `ai_patrol` (Pulse Patrol runs on BYOK; advanced autonomy outcomes are separately gated)
- `sso` (Basic SSO via OIDC)
- `update_alerts`

Notes:
- Basic SSO (OIDC) is free. Advanced SSO remains paid via `advanced_sso`.

### Pro Intelligence (Monthly / Annual / Lifetime)

Included capabilities:
- `ai_patrol`
- `ai_alerts`
- `ai_autofix`
- `kubernetes_ai`
- `agent_profiles`
- `update_alerts`
- `relay`
- `sso` (Basic OIDC)
- `advanced_sso` (SAML, multi-provider, role mapping)
- `rbac`
- `audit_logging`
- `advanced_reporting`
- `long_term_metrics`

Notes:
- Audit events may be captured regardless of tier, but access to query/export is gated behind `audit_logging`.
- Long-range metrics history beyond the free window is gated by `long_term_metrics`.

### MSP

MSP includes all Pro capabilities plus:
- `unlimited` (volume/instance limit removal)

Notes:
- `multi_user` and `white_label` exist as capability keys but are not currently included in the MSP tier in code.

### Enterprise

Enterprise includes all Pro capabilities plus:
- `unlimited`
- `multi_user`
- `white_label`
- `multi_tenant`

Notes:
- Multi-tenant is also guarded by a feature flag (`PULSE_MULTI_TENANT_ENABLED=true`) in addition to licensing.

## Capability Keys

Canonical list and tier mapping lives in `docs/architecture/ENTITLEMENT_MATRIX.md`.

