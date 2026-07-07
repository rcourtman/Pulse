# OIDC Scope And Group Authorization Fix Spec

## Status

Resolved on 2026-07-07; see Resolution. Reporter retest still requires a
release artifact containing both fixes.

Primary issue: #1535

Related issues: #1528, #1533

Governed owners:

- `api-contracts`: SSO provider API payloads, OIDC login initialization, callback authorization, session identity.
- `frontend-primitives`: Settings -> Security -> Single Sign-On provider configuration.

This document is a handoff spec. It is not a solution design.

## User-Visible Failure

Settings-configured OIDC can complete part of the login flow, but group-based authorization and role mapping still fail for reporters using v6.0.4 through v6.0.5-rc.3.

The visible failure reported on #1535 after v6.0.5-rc.3:

- the displayed username/session label is no longer the internal `sso:oidc:...` principal
- group authorization still fails with `Your account is not part of an authorized group to use Pulse.`
- the observed OIDC authorization request scope is only `openid profile email`
- the reporter expects a configured group claim to be available for group role mapping

Reported IdPs:

- Pocket ID
- Authentik

## What Is Already Fixed

The following commits are present on `origin/main` and in the v6.0.5 RC line:

- `caa9b41834` `Fix OIDC provider detail persistence`
  - Fixes SSO provider API/detail persistence for nested OIDC fields, groups claim, allowed groups, and group role mappings when the payload supplies them.
  - References #1521.
- `1c8a9346ef` `Fix legacy OIDC SSO discovery and CSP nonce`
  - Restores the saved/legacy OIDC discovery path and SSO button behavior.
  - References #1533.
- `eb99d7a6b3` `Fix SSO session display labels`
  - Keeps the provider-scoped SSO principal as the stable session owner while displaying the IdP username/email/display claim in app chrome.
  - References #1535.

These commits do not finish group-scope authorization for Settings-configured OIDC.

## Current Evidence

Current `origin/main` evidence:

- `internal/api/identity_sso_handlers.go`
  - OIDC provider detail responses expose nested OIDC scopes.
  - Create/update handlers can persist supplied OIDC scopes, groups claim, allowed groups, and group role mappings.
- `internal/api/sso_handlers_crud_test.go`
  - API tests prove a payload containing `["openid", "profile", "email", "groups"]` can round-trip through provider detail and persistence.
- `frontend-modern/src/components/Settings/ssoProvidersModel.ts`
  - The form model has `oidcScopes`.
  - The empty form default is `openid profile email`.
  - The payload builder sends `oidc.scopes` from `form.oidcScopes`.
- `frontend-modern/src/components/Settings/SSOProvidersPanel.tsx`
  - The OIDC create/edit UI exposes issuer, client, secret, redirect/logout, groups claim, allowed groups, allowed domains, allowed emails, and group role mappings.
  - The OIDC create/edit UI does not render an editable OIDC scopes field.
- `internal/api/oidc_handlers.go`
  - Login initialization falls back to `openid profile email` when provider scopes are empty.
  - Group restriction and group role mapping depend on the configured groups claim being present in the OIDC claims.
  - Group claim extraction already accepts arrays and comma/space-separated strings.

Commit history evidence:

- No commits from `v6.0.0..origin/main` touch `frontend-modern/src/components/Settings/SSOProvidersPanel.tsx`.
- No commits from `v6.0.0..origin/main` touch `frontend-modern/src/components/Settings/ssoProvidersModel.ts`.

## Expected Product Behavior

An administrator configuring OIDC through Settings must be able to view and edit the exact OIDC scopes Pulse uses for the authorization request.

The configured scopes must be the same scopes that:

- are saved by provider create/update
- are returned by provider detail
- are shown again when the provider is reopened for editing
- are used when Pulse builds the OIDC authorization request

For providers with no custom scopes configured, existing behavior remains:

- default scopes are `openid profile email`
- existing providers continue to work without requiring manual reconfiguration

For providers that require an extra group scope before returning group claims:

- a Settings-configured provider can request that scope
- Pulse can receive the configured group claim
- allowed-group checks use that claim
- group role mappings use that claim
- a matching group grants the mapped Pulse role
- a non-matching or missing group still fails closed

The v6 SSO identity invariant remains:

- the provider-scoped subject is the stable SSO principal
- `preferred_username`, email, or display name must not become the canonical session owner
- local-user linking by `preferred_username` must not be reintroduced as the authorization model
- app chrome should continue to display the IdP user-facing claim instead of the internal provider-scoped principal

## Non-Goals

This fix does not need to redesign SSO.

This fix does not need to change:

- SAML behavior
- proxy auth
- local username/password auth
- paid-tier gating
- the provider-scoped SSO principal model
- the visible v6 Settings navigation model

The callback/auth-routing failure reported in #1533 is related OIDC fallout, but it is distinct from the missing group-scope path unless evidence proves a shared root cause.

## Not Fixed If

The issue is not fixed if any of the following remain true:

- the API accepts custom OIDC scopes, but the Settings UI cannot configure them
- the Settings UI has a groups claim field, but the authorization request still omits the configured group scope
- Pulse globally adds `groups` to every OIDC provider without preserving admin-configured scope intent
- role mapping works only when the IdP happens to return groups under the default `openid profile email` request
- role mapping depends on matching a local Pulse user by `preferred_username`
- the display label regresses to `sso:oidc:...`
- existing providers with no custom scopes break
- reporter retest is requested before a release artifact actually contains the fix

## Required Proof

A complete fix needs proof for these outcomes:

- A Settings-created OIDC provider can save a non-default scope set such as `openid profile email groups`.
- Reopening that provider in Settings shows the same scope set.
- The authorization request generated for that provider includes the saved scope set.
- An OIDC callback containing the configured groups claim grants the mapped role.
- An OIDC callback without a matching group still fails closed.
- Existing providers with empty or missing scopes still use `openid profile email`.
- The #1535 display-label fix remains intact.
- The #1533 SSO button/discovery fix remains intact.
- Tests cover both backend payload persistence and the Settings UI path that a normal administrator uses.
- Browser proof exercises the Settings -> Security -> Single Sign-On create/edit path, not only source-level payload builders.

## Resolution (2026-07-07)

Two defects, two repos:

- repos/pulse `87aac4e57` adds the editable Scopes field to the Settings
  OIDC create/edit modal (the form model already round-tripped
  `oidc.scopes`; the panel never rendered an input for it), plus model and
  panel tests covering the create/edit round-trip.
- pulse-enterprise `689100c` fixes the deeper root cause. SSO admin
  endpoints are overridden by the enterprise binder
  (`pulse-enterprise/internal/ssoadmin/hooks.go`), and its provider detail
  GET used a local flat serialization that drops nested OIDC scopes, the
  groups claim, and group role mappings. Because SSO is license gated,
  every real install reads provider detail through that override, so
  reopening a provider showed defaults and the next save reset the saved
  scopes. The OSS-side persistence/detail fixes in `caa9b41834` never
  executed on licensed builds. Detail reads now delegate to the core
  handler, which returns the canonical nested payload.

Verified live against a dev enterprise build: a provider created with
`openid profile email groups` shows the same set when reopened, and the
`/api/oidc/{id}/login` redirect carries
`scope=openid+profile+email+groups`. Persistence and the authorization
request were already correct end to end; only the detail read path was
lossy.
