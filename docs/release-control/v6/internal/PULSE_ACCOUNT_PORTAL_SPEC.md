# Pulse Account Portal Spec

Last updated: 2026-03-26
Status: ACTIVE

## Purpose

Define the canonical customer and operator account surface for Pulse once the
current fragmented cloud, licensing, billing, recovery, and MSP account
surfaces are promoted into a dedicated governed lane.

This spec exists to stop three kinds of drift:

1. Cloud and MSP account work growing as a control-plane-only portal with no
   coherent self-hosted account story.
2. Self-hosted commercial support accreting as one-off utility pages instead
   of a real account surface.
3. Relay, Mobile, Cloud, and licensing account actions being presented as
   separate portals rather than one Pulse account model with product-specific
   areas.

## Current Product Truth

Pulse already has real customer-account and operator-account surfaces, but they
are split across different products and repos:

1. `internal/cloudcp/portal/page.go` and `internal/cloudcp/portal/handlers.go`
   provide a real hosted browser portal for Cloud and MSP accounts.
2. `internal/cloudcp/account/tenant_handlers.go` and `internal/cloudcp/routes.go`
   provide authenticated account-member, workspace, and billing actions.
3. `pulse-pro/landing-page/manage.html`,
   `pulse-pro/landing-page/retrieve-license.html`,
   `pulse-pro/landing-page/refund.html`,
   `pulse-pro/landing-page/data.html`, and
   `pulse-pro/landing-page/thanks.html` provide public commercial utility
   surfaces for the current self-hosted track.
4. Hosted Cloud and MSP now have public explanatory pages, but those pages are
   not themselves the account surface.

That means Pulse has account plumbing, not yet one coherent Pulse account
product surface.

## Canonical Product Definition

`Pulse Account` is the single authenticated commercial and lifecycle control
surface for Pulse customers and operators.

It owns:

1. identity for commercial/account actions
2. billing and subscription state
3. self-hosted licenses, activations, and recovery
4. hosted Cloud tenant access and lifecycle
5. MSP workspace and membership administration
6. support and compliance actions that belong to the commercial account, not to
   one Pulse runtime instance

It does not own:

1. in-product Pulse runtime settings
2. relay pairing or mobile device state as a standalone portal
3. tenant-local monitoring, AI, alerting, storage, or other runtime product
   workflows that belong inside Pulse itself

## Canonical User Model

The canonical commercial identity hierarchy is:

1. `user`
   One human identity that can sign in to account-scoped commercial surfaces.
2. `account`
   The commercial ownership unit. An account can be a Cloud customer, an MSP,
   or another commercial owner shape that holds billing and memberships.
3. `workspace` / `tenant`
   A hosted Pulse runtime owned by an account.
4. `license`
   A self-hosted commercial entitlement owned by an account.
5. `membership`
   A role binding between a user and an account.

One user may belong to multiple accounts. One account may own multiple hosted
workspaces and multiple self-hosted licenses. MSP is therefore an account kind
with stronger workspace-management needs, not a completely separate portal.

Portal authentication must follow that same commercial identity model. `Pulse
Account` sign-in cannot be limited to hosted-tenant members only; the portal
magic-link path must accept both:

1. hosted Cloud/MSP identities that already resolve through the control-plane
   tenant/account registry
2. self-hosted commercial identities that resolve through the shared
   license/commercial account surface even when they have no hosted tenant

When the portal explicitly requests a portal-target magic link, the resulting
verification flow must create a control-plane session and return the user to
`/portal` rather than forcing a tenant handoff redirect.

## Canonical Information Architecture

The future Pulse account surface should be one shell with product-aware areas,
not separate portals for each commercial motion.

Primary areas:

1. `Overview`
   Account identity, current plan state, high-level service status, and next
   actions.
2. `Cloud`
   Hosted tenant list, health summary, open-workspace handoff, tenant lifecycle,
   invites, and account-scoped hosted billing actions.
3. `Licenses`
   Self-hosted license retrieval, activation context, entitlement status,
   renewal state, and recovery actions.
4. `Billing`
   Subscription state, invoices, payment method, usage-facing commercial facts,
   and Stripe billing portal handoff when still needed.
5. `People`
   Memberships, roles, invitations, and account access.
6. `Recovery & Support`
   Refund, data request, recovery-email, and commercial support actions.

Conditional areas:

1. `MSP Workspaces`
   Only when the account kind is MSP or otherwise multi-workspace by contract.
2. `Organization / Tenant Admin`
   Only for hosted accounts that need browser-side workspace lifecycle actions.

## Product-Specific Boundaries

### Self-hosted Pulse

Self-hosted Pulse keeps runtime settings, activation notices, and local billing
status inside the product instance, but the durable customer-account actions
move toward Pulse Account:

1. license retrieval
2. subscription management
3. refunds and data requests
4. account-level billing history
5. future license inventory and seat/entitlement visibility

### Pulse Cloud

Pulse Cloud uses Pulse Account as its primary customer control surface.

It owns:

1. hosted tenant list
2. billing state
3. workspace open/handoff
4. tenant create/delete/suspend lifecycle
5. account membership and invites

The hosted tenant Pulse runtime remains the product runtime, not the account
portal.

### MSP

MSP is not a separate portal brand. It is a Pulse Account shape with stronger
multi-workspace and operator controls.

It adds:

1. customer workspace lifecycle
2. workspace switching
3. per-workspace health summary
4. account roles suitable for owner/admin/tech/read-only workflows

### Pulse Relay and Pulse Mobile

Pulse Relay does not get a standalone portal. Relay is a capability inside
Pulse Mobile, self-hosted Pulse, and Pulse Cloud.

Pulse Account may show:

1. whether a plan includes relay/mobile capability
2. hosted billing or upgrade implications for relay/mobile usage

It must not become a separate Relay administration product unless Relay is
later sold as a standalone service.

## Transition Rules

The current public utility pages remain valid transitional surfaces while v5 is
the live public commercial track, but they are not the desired steady-state
shape.

Transition rule:

1. existing utility pages may remain as entry points or compatibility shims
2. new commercial/account workflows should prefer the Pulse Account shell
3. utility pages should shrink toward redirects or lightweight recovery
   handoffs once equivalent Pulse Account areas exist

## Forbidden Drift

Do not:

1. build a separate Relay portal
2. build separate Cloud, MSP, and self-hosted account shells that duplicate
   billing, identity, and recovery logic
3. add new one-off commercial utility pages when the workflow belongs in Pulse
   Account
4. let the hosted control-plane portal evolve without a self-hosted license and
   recovery story
5. move runtime product settings out of Pulse and into the account portal just
   because the account shell exists

## v6 Scope And Phasing

The full Pulse Account portal is not an RC or GA floor gate for v6. That
matches the current resolved decision that full hosted MSP portal expansion is
post-GA.

But it is the canonical next product-shaping lane for commercial coherence.

### Current v6 floor

Accepted as sufficient for RC and GA:

1. Cloud/MSP control-plane portal exists
2. self-hosted recovery and billing utilities exist
3. commercial surfaces are functional but fragmented

### Candidate lane target

The `customer-account-portal` lane should deliver:

1. one named `Pulse Account` shell and IA
2. shared identity and navigation across hosted account actions and
   self-hosted commercial actions
3. canonical ownership boundaries for billing, licenses, hosted tenants,
   memberships, and recovery
4. de-duplication of fragmented public utility flows where a real authenticated
   account area is the better product shape
5. a renderer-owned frontend bootstrap contract for the account shell, so a
   maintained frontend can consume canonical account state without scraping
   ad-hoc DOM attributes or hardcoded production URLs
6. a maintained bundled frontend source tree and sync-proof path inside
   `internal/cloudcp/portal`, so the account shell does not regress into
   handwritten embedded asset drift

### Current frontend seam

The current `/portal` surface now renders one machine-owned application shell
for both signed-out and signed-in users. That shell emits a
`pulse-account-bootstrap` JSON script tag, and the authenticated runtime can
refresh from `/api/portal/bootstrap`. Together, those two surfaces are the
canonical frontend state seam for:

1. account identity context
2. hosted account and workspace summaries
3. public-site URLs plus same-origin portal route paths for commercial actions,
   so the browser shell can stay behind the control-plane CSP instead of
   calling shared license APIs cross-origin
4. signed-out versus signed-in shell state, so login, session expiry, and
   authenticated account runtime all inherit one owned page contract instead of
   separate server-rendered templates
5. the canonical bootstrap route path and stable workspace summary fields, so
   the frontend can render and refresh account/workspace state from one owned
   contract instead of depending on server-rendered DOM structure

The portal package also owns a dedicated bootstrap JSON handler shape for the
same contract, so route wiring can promote the shell toward a maintained
frontend/API split without inventing a second state model.

New frontend work should extend that contract deliberately instead of adding
one-off data attributes or baking production hostnames into static assets. The
maintained frontend source now lives under `internal/cloudcp/portal/frontend/`,
is embedded from `internal/cloudcp/portal/dist/`, and is guarded by
`internal/cloudcp/portal/frontend_sync_test.go` plus the package-local
typecheck/build steps, so Pulse Account frontend work should extend that source
tree and rebuild the committed bundle instead of editing embedded script or CSS
blobs directly. Coordination between account-shell modules should stay inside
that owned runtime boundary as well, rather than drifting back to
document-wide custom events or browser-global runtime objects.

### Post-lane follow-on

Reasonable later expansions include:

1. richer invoice/history views
2. support case history
3. broader audit/compliance export surfaces
4. deeper MSP customer/customer-contact management

## Ownership

The owning governed subsystem is `cloud-paid`.

Why:

1. the portal is a commercial/account boundary first
2. it spans Cloud, MSP, billing, licensing, and recovery
3. the existing control-plane portal and self-hosted utility surfaces already
   sit inside cloud-paid-adjacent ownership

This is a lane-expansion / new-lane shape above current `L3` and `L4`, not a
reason to fork commercial governance into another subsystem.
