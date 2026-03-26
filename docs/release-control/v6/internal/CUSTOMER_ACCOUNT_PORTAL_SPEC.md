# Customer Account Portal Spec

Last updated: 2026-03-25
Status: PLANNED
Governance surface: `status.json.coverage_gaps.customer-account-portal-surface`
Candidate lane: `customer-account-portal`

## Intent

Pulse v6 now has real customer-facing commercial surfaces across self-hosted
licensing, hosted tenants, MSP provider workflows, billing, refund/recovery
utilities, and account-scoped control-plane actions.

Those surfaces work, but they do not yet form one coherent authenticated Pulse
account experience.

The purpose of the customer account portal lane is to promote those fragmented
surfaces into one governed product area:

- one account identity
- one commercial home
- one place to see licenses, hosted tenants, billing, and recovery actions
- one operator surface that can expand into MSP administration cleanly

## Product Sentence

Pulse Account is the canonical customer and operator portal for commercial
Pulse: self-hosted licenses, Pulse Cloud tenants, billing state, recovery
actions, and MSP administration all converge there instead of living as
disconnected utility pages and local admin fragments.

## Why This Is A Separate Lane

This is not just a UI cleanup.

It crosses:

- commercial identity and login state
- self-hosted licensing and activation recovery
- hosted tenant lifecycle and account-scoped control-plane actions
- MSP customer and workspace administration
- billing, invoices, refunds, and recovery/support actions

Those concerns already exist in runtime and operations, but they are split
across in-product settings, hosted account handlers, and public utility pages.
That is a real product surface gap, not a copy problem.

## Current Truth

Today Pulse has:

- self-serve utility pages such as subscription management, license retrieval,
  refund, and data request
- hosted/account-scoped runtime entry points and tenant handlers
- hosted organization billing and cloud pricing surfaces
- MSP provider account and tenant-management behavior

What it does not yet have is one coherent authenticated account portal that
joins those pieces together.

## Goals

1. Give customers one canonical account home for commercial Pulse.
2. Unify self-hosted licensing and hosted tenant ownership under one account
   mental model.
3. Let hosted customers see and manage their Pulse Cloud tenant state from the
   same account surface as billing.
4. Let MSP operators work from a first-class operator portal rather than a set
   of narrow admin fragments.
5. Absorb current public recovery/utility pages into a coherent account flow
   over time instead of keeping them as the long-term primary UX.

## Non-Goals

1. A standalone Relay portal. Relay remains a capability within Mobile, Cloud,
   and self-hosted product surfaces.
2. Making full hosted/MSP portal depth an RC or GA blocker for Pulse v6. The
   current governed release policy already keeps the full portal expansion
   post-GA.
3. Replacing all in-product billing/admin surfaces immediately if they still
   serve as the best runtime-local control surface.
4. Turning every support or recovery workflow into a heavyweight app before the
   core account model is coherent.

## Users

### 1. Self-Hosted Customer

Needs:

- see current license/subscription state
- recover activation/license details
- manage billing and subscription continuity
- understand entitlement limits and plan state

### 2. Hosted Pulse Cloud Customer

Needs:

- see owned tenants
- enter the hosted tenant runtime
- see hosted billing and plan state
- recover account access and understand tenant ownership

### 3. MSP Operator

Needs:

- see provider account state
- view and manage multiple client/customer environments
- understand plan/billing context without mixing MSP and self-hosted language
- operate from a provider-grade control surface

## Canonical Information Architecture

The future portal should converge on this shape:

### 1. Home

- account summary
- active subscriptions and licenses
- owned hosted tenants
- outstanding recovery/billing/action-needed state

### 2. Licenses

- self-hosted licenses and activation state
- entitlement summary
- continuity / renewal / cancellation state
- migration guidance where relevant

### 3. Pulse Cloud

- hosted tenants
- tenant status and entry points
- organization/account linkage
- hosted account-scoped actions

### 4. Billing

- subscriptions
- invoices and payment method context
- tax / VAT / receipt surfaces
- refund and cancellation/re-entry surfaces

### 5. MSP

- provider account summary
- customers / workspaces / tenant list
- operator-scoped admin actions
- clear separation from normal self-hosted customer flows

### 6. Recovery And Support

- license retrieval
- account verification flows
- data request/export/delete
- transition path away from isolated standalone utilities

## Transitional Mapping From Current Surfaces

These are interim surfaces, not the long-term portal:

- `pulse-pro/landing-page/manage.html`
- `pulse-pro/landing-page/retrieve-license.html`
- `pulse-pro/landing-page/refund.html`
- `pulse-pro/landing-page/data.html`
- `pulse` hosted account handlers and billing/admin panels
- `pulse` MSP provider account and tenant-management handlers

The v6 portal lane should treat those as migration sources, not as the final
product shape.

## V6 Scope

The proper v6 lane scope is:

1. Define the account identity and navigation model clearly.
2. Establish one authenticated account shell / entry surface.
3. Unify the first customer-critical actions:
   - license/subscription visibility
   - hosted tenant visibility
   - billing / recovery entry points
4. Keep direct utility-page compatibility while the portal absorbs them.
5. Keep runtime-local settings pages where they are still the right control
   surface, but stop treating them as the entire account experience.

## Post-GA Expansion

These are valid follow-ons after the first coherent portal lands:

- deeper hosted tenant lifecycle controls
- richer MSP operator/customer hierarchies
- support inbox and guided recovery workflows
- broader invoice/tax/export surfaces
- more opinionated cross-product notifications and account action center

## Ownership Boundary

This lane should stay owned by the `cloud-paid` subsystem unless the governance
map later proves it needs a separate subsystem.

Repo split:

- `pulse`: authenticated runtime/account APIs, hosted account handlers,
  tenant/admin surfaces, in-product billing/account presentation
- `pulse-pro`: public commercial edge, self-serve utility/recovery pages,
  checkout/license commercial account plumbing

`pulse-mobile` is a consumer of this account model, not the owner of it.

## Release Policy

This lane is a real product gap, but it is not an RC floor blocker.

That matches the existing governed policy already recorded in
`status.json.resolved_decisions.ga-floor-policy`: full hosted/MSP portal depth
is post-GA rather than a GA floor gate.

The right action is not to pretend the gap does not exist.
It is to track it as a deliberate planned lane with a coherent first scope.
