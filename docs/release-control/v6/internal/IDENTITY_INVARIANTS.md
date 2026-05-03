# Pulse Identity Invariants

This file captures the stable identity contract for v6 runtime and agent-ready
work. It is deliberately stricter than display naming: anything that can grant
access, bind billing, own an organization, or authorize an action needs a
durable key that is not silently replaced by email.

## Core Rule

Email is contact metadata. It may be used for delivery, display, lookup,
rate-limiting, and legacy migration boundaries, but it must not be the durable
principal once a stable user ID exists.

## Canonical Identifiers

1. Pulse control-plane user ID
   - Shape: `u_...` from `internal/cloudcp/registry`.
   - Owns: Pulse Account user identity and hosted tenant membership principal.
   - Email role: contact/display/delivery metadata through registry user email,
     organization `OwnerEmail`, and member `Email`.

2. Pulse account ID
   - Shape: `a_...` from `internal/cloudcp/registry`.
   - Owns: commercial account/workspace grouping, MSP/customer account scope,
     invitations, and account membership.
   - Must not be inferred from email.

3. Hosted tenant ID
   - Shape: `t_...` from `internal/cloudcp/registry`.
   - Owns: hosted workspace runtime, tenant data directory, handoff audience,
     and tenant organization scope.
   - Must not be inferred from Stripe email or SSO email.

4. Organization ID
   - Shape: org-safe identifier validated by the API/config boundary.
   - Owns: local organization metadata, RBAC/share scope, org cookie scope, and
     tenant org metadata.
   - Organization owner/member principals should be stable user IDs when the
     control plane has one; email-shaped user IDs are legacy fallback only.

5. Stripe customer and subscription IDs
   - Shape: Stripe IDs such as `cus_...` and `sub_...`.
   - Owns: billing linkage and subscription lifecycle only.
   - Stripe `customer_email` is delivery/contact evidence, not identity
     authority. It may send a magic link only when it matches existing stored
     owner/member contact metadata or a legacy email-keyed member.

6. SSO provider subject
   - Shape: provider-scoped OIDC `sub` or SAML `NameID`, paired with provider ID.
   - Owns: external identity continuity for SSO.
   - Email role: access restriction, contact, display, and legacy username
     fallback. A future SSO migration must move self-hosted RBAC from unscoped
     username/email keys to provider-scoped subject keys with a compatibility
     migration, not by changing session names silently.

7. API token ID/hash
   - Shape: token hash plus token metadata in config/auth storage.
   - Owns: API credential identity, scope, and revocation.
   - `owner_user_id` metadata must carry the authenticated principal, not an
     arbitrary email copied from a browser payload.

8. Mobile/relay device identity
   - Shape: relay/mobile device registration IDs and platform device tokens.
   - Owns: device trust, push routing, and pairing state.
   - Email must not be used as a device key.

## Runtime Requirements

1. Handoff sessions bind to the signed stable subject or `UserID`, then verify
   that subject against existing server-side organization membership.
2. Magic-link sessions bind to the stored organization principal resolved from
   contact email at verification time, not to the email embedded in the token.
3. Hosted checkout and tenant provisioning seed organization membership from
   registry user IDs. Contact email is copied separately.
4. Webhook payloads must never create organizations or memberships from email.
   They may update billing only after server-owned org/customer linkage exists.
5. Legacy email-keyed records may be accepted only at migration boundaries and
   should be canonicalized to stable user IDs when the stable ID is known.

## Current Explicit Exception

Self-hosted local auth and SSO RBAC still use the historical `username` session
field as the local principal. That is a compatibility boundary, not the desired
long-term model for SSO. Any future SSO hardening must introduce a provider-
scoped stable principal and migration path for existing RBAC assignments,
audit records, and active sessions instead of substituting email with another
display claim in-place.
