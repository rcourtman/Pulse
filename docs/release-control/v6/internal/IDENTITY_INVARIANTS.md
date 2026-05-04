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
   - Email role: access restriction, contact, display, and legacy RBAC
     migration. Self-hosted SSO sessions now use provider-scoped stable
     principals; mutable username/email/display claims may only seed
     compatibility role migration when no authoritative group mapping is
     present.

7. API token ID/hash
   - Shape: token hash plus token metadata in config/auth storage.
   - Owns: API credential identity, scope, and revocation.
   - `owner_user_id` metadata must carry the authenticated principal, not an
     arbitrary email copied from a browser payload. Token minting helpers must
     reject caller-supplied metadata that attempts to set `owner_user_id`.
   - User/admin-initiated token minting paths, including security-token
     creation/regeneration, agent install commands, deploy bootstrap and
     enrollment runtime tokens, container runtime migration, and first-run
     security setup must attach owner identity through the shared server-side
     owner helper rather than through extension metadata.

8. Mobile/relay device identity
   - Shape: relay/mobile device registration IDs and platform device tokens.
   - Owns: device trust, push routing, and pairing state.
   - Email must not be used as a device key.

## Runtime Requirements

1. Handoff sessions bind to the signed stable subject or `UserID`, then verify
   that subject against existing server-side organization membership. Blank or
   email-shaped handoff subjects are invalid; email may only help find legacy
   email-keyed tenant records after a stable subject is already present.
2. Magic-link sessions bind to the stored organization principal resolved from
   contact email at verification time, not to the email embedded in the token.
   If the matching organization owner/member has no stored principal, the
   magic-link flow must fail closed instead of synthesizing a principal from
   contact email.
3. Hosted checkout and tenant provisioning seed organization membership from
   stable Pulse user IDs. Registry-backed paths must create or resolve the
   registry user before writing hosted tenant `OwnerUserID` or member `UserID`;
   legacy public hosted signup must generate an opaque `u_...` owner principal
   and copy email only into contact metadata and magic-link delivery. No hosted
   provisioning path may synthesize durable owner/member IDs from email.
   Duplicate-signup recovery for older hosted org rows must canonicalize blank
   or email-shaped owner principals to a generated stable `u_...` owner before
   returning the existing org for magic-link delivery.
4. Webhook payloads must never create organizations or memberships from email.
   They may update billing only after server-owned org/customer linkage exists.
   Post-checkout magic-link delivery from a webhook must also resolve the
   Stripe contact email through current server-side org membership first;
   matching contact metadata with a blank owner/member principal is not enough
   to send a sign-in link.
5. Legacy email-keyed records may be accepted only at migration boundaries and
   should be canonicalized to stable user IDs when the stable ID is known.
6. Self-hosted OIDC and SAML sessions bind to an opaque principal derived from
   provider type, provider ID, and the provider subject (`sub` or `NameID`).
   RBAC may copy a legacy username/email assignment to that principal during
   compatibility migration, but the browser session and tracked active-session
   owner must be the stable SSO principal.

## Current Explicit Exception

Self-hosted local password/proxy auth still uses the historical `username`
session field as the local principal. That is a compatibility boundary for
local operators and reverse-proxy deployments. SSO no longer shares that
exception: OIDC and SAML sessions must use provider-scoped stable principals,
with legacy username/email assignments treated only as migration inputs.
