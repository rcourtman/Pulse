# Pulse Account Portal Spec

Last updated: 2026-03-28
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

The future Pulse account surface should be one shell with task-first areas, not
separate portals for each commercial motion or section names derived from
Pulse's internal product model.

Primary areas:

1. `Overview`
   A short triage surface that answers only three questions: what needs
   attention, what is ready, and what the next obvious action is.
2. `Workspaces`
   Hosted workspace list, health summary, open-workspace handoff, lifecycle
   actions, and create-workspace entry points.
3. `Access`
   Account roster, invites, role changes, and access removal.
4. `Billing`
   Hosted billing state and Stripe billing handoff when relevant. Self-hosted
   license retrieval, refunds, privacy/data actions, and related commercial
   utilities appear here only when the account has those needs.
5. `Support`
   Escalation path only. It must not compete with primary account tasks in the
   main shell.

Conditional content:

1. Hosted billing controls appear inside `Billing` only when the account has
   hosted billing authority.
2. Self-hosted license, refund, and privacy utilities appear inside `Billing`
   only when the account has relevant self-hosted history or entitlements.
3. MSP-specific fleet scale or operator context appears inside `Workspaces`;
   it does not become a separate top-level area.

## Experience Principles

The portal must feel like one deliberate product, not a stitched-together set
of utilities.

Core rules:

1. The first screen must be triage, not a dashboard essay.
2. The overview should answer only `Needs attention`, `Ready`, and `Next
   action`.
3. Top-level navigation must be organized by user jobs, not by Pulse's
   internal hosted, license, billing, or support implementation boundaries.
4. Users must be able to complete basic tasks without learning Pulse's model
   first.
5. Hosted and self-hosted users must not land on the same undifferentiated
   wall of copy.
6. A self-hosted-only user should immediately see that hosted workspaces are
   absent by entitlement, not broken or missing.
7. A hosted Cloud or MSP user should immediately see workspace access, account
   role, and the next obvious action before lower-priority self-hosted tools.
8. Generic overflow icons are forbidden when the only behavior behind them is
   a destructive action.
9. Primary actions must be labeled with the real outcome, for example
   `Open workspace`, `Create workspace`, `Invite people`, or `Manage billing`.
10. Self-hosted commercial tools belong in the same account shell, but they
    must remain conditional secondary content inside `Billing` when hosted
    access is active.
11. Hosted and MSP accounts must expose account-level operations separately
    from individual workspace cards so operator actions are visible before a
    user starts drilling into one workspace at a time.
12. Workspace fleets must summarize health and attention state at the account
    level, not force the user to scan badges one card at a time.
13. Workspace lifecycle actions must open in an explicit management surface
    with the selected workspace context visible, not inside a hidden overflow
    menu or a blind confirm-first interaction. When that management surface or
    the create-workspace form opens below the fold, the shell must reveal it.
14. Access management must be a visible roster and invite surface, not a table
    or control hidden behind unrelated account copy.
15. Support must remain an escalation path, not a peer destination competing
    with the primary jobs a user came to do.
16. The signed-in `Overview` must be one shell-level triage surface across the
    current account set, not a repeated per-account dashboard.
17. When `Workspaces` or `Access` are unavailable, the tab must show an
    explicit unavailable state that points to the right live task instead of
    rendering blank space. The shell navigation must stay honest too: it must
    not badge or describe unavailable hosted tasks as though they are live.
18. `Access` must stay action-first: roster, invite, role change, and remove
    access are the job; view-only users may review the roster but must never
    see live controls that imply they can mutate it. The view-only roster must
    stay a review surface, not a per-row action table with fake disabled
    action state.
19. `Billing` must present one obvious billing path at a time, with hosted
    billing first when applicable and self-hosted licenses, refunds, or
    privacy as secondary job-specific paths rather than a billing essay.
20. `Support` must explain only when to escalate and what to send so it reads
    as a handoff path, not a competing task surface.
21. On phone-width layouts, top-level task navigation must collapse into a
    compact task strip so the active job stays above the fold instead of being
    buried below a desktop-style sidebar, and the strip must auto-reveal the
    active task when the user changes jobs.
22. On phone-width layouts, account identity context must collapse into a
    compact summary strip ahead of the active task instead of repeating a
    large desktop intro block before every section.
23. On phone-width layouts, opening a lower workspace job such as
    `Lifecycle` or `Create workspace` must reveal that job surface instead of
    leaving the user at the top of the workspace list.
24. `Workspaces` must default to the workspace list and task entry points, not
    an idle lifecycle explainer. The lifecycle rail should appear only when a
    lifecycle or create-workspace job is actually active.
25. `Access` must default to the hosted roster plus explicit job entry points,
    not a permanently open mutation rail. Invite, role-change, and remove
    controls should appear only when that exact access job is active.
26. The first hosted `Access` render must come from bootstrap-owned roster
    truth, not a fetch-first placeholder. The portal bootstrap/runtime
    contract must carry the current hosted roster snapshot so `Access` opens
    as a real review surface before any later refresh or mutation follow-up.
27. `Billing` must default to hosted billing plus explicit self-hosted job
    entry points, not an always-open billing dashboard. Self-hosted billing,
    license, refund, and privacy panels should appear only when that exact
    billing job is active, and opening one on phone widths must reveal the
    active panel in-frame. When no hosted account exists, `Billing` must lead
    directly with the self-hosted job picker instead of front-loading an
    empty hosted-billing block.
28. `Support` must stay honest to account shape. Self-hosted-only accounts
    must reduce `Support` to the billing escalation path only; hosted
    workspace or access failure routes and task buttons must not render
    without hosted accounts.
29. The portal bootstrap/runtime contract must carry explicit truth for
    whether self-hosted commercial history is relevant to the signed-in
    account. Hosted-only accounts must not render self-hosted licenses,
    refunds, privacy utilities, or self-hosted escalation copy by default.
30. Hosted view-only users must see permission-honest task copy. `Workspaces`,
    `Access`, and hosted `Billing` may not advertise create, roster-mutation,
    or hosted-billing actions when the current account role cannot perform
    them; those surfaces must say that an owner or admin is required.
31. The compact account-context strip must also stay permission-honest. It
    must describe what the current user can actually do on the account, not
    restate the account's full hosted capability set when access or billing
    changes require an owner or admin.
32. Hosted `Support` must stay permission-honest too. View-only hosted users
    may be sent back to `Workspaces`, `Access`, or `Billing` only as review
    and owner/admin handoff paths; `Support` must not imply they can perform
    hosted lifecycle, access-mutation, or hosted-billing changes themselves
    before escalation.
33. `Overview` must keep billing cues honest to account shape and permission.
    Hosted-only accounts may not mention self-hosted billing utilities by
    default, and hosted view-only roles must say when owner/admin authority is
    still required to open hosted billing.
34. User-facing role labels must stay on product vocabulary. The portal may
    describe account role as `Owner`, `Admin`, `Tech`, or `Read-only`, but it
    must not leak internal identifiers such as `read_only` or legacy aliases
    such as `member`.
35. `Overview` must also keep `Next action` permission-honest for hosted
    view-only accounts. When no workspace is ready, the primary route must
    stay on reviewable `Workspaces` or `Access` surfaces before any blocked
    hosted billing or owner/admin-only mutation path.
36. Task surfaces must keep failure copy on owned user jobs. The portal may
    not leak raw transport strings such as `Network error.` into `Access`,
    `Workspaces`, or `Billing`; each failure must stay on the task-specific
    action the user was trying to complete.
37. `Overview` must keep `Ready` honest when no hosted workspace exists yet.
    Hosted accounts with zero workspaces may not tell the user to review
    current workspace state; they must say that nothing is ready yet and that
    the first hosted workspace still needs owner/admin creation before routine
    work can start.
38. `Overview` must keep `Needs attention` honest when only suspended
    workspaces remain. The shell may not imply active work is ready merely
    because a suspended workspace exists; suspended-only states must say that
    no active workspace is ready for routine use right now.

## Screen Model

The signed-in shell should be treated as four first-class states:

1. `Self-hosted account`
   No hosted workspaces. The page should lead with the relevant billing and
   recovery actions and clear messaging that no hosted workspace access is
   attached to this account.
2. `Hosted customer account`
   One or more hosted workspaces. The page should lead with workspace access,
   hosted billing, and account access controls.
3. `MSP operator account`
   Multi-workspace hosted account. The page should lead with the workspace
   fleet, management actions, and operator access controls.
4. `Mixed account`
   Hosted access plus self-hosted commercial history. The page should still
   lead with hosted access, while the self-hosted commercial tools remain
   available inside `Billing` when relevant.

Each signed-in state should render:

1. a concise overview band that answers `Needs attention`, `Ready`, and `Next
   action`
   The overview band is shell-level triage, not one repeated card per account.
2. account cards with role, account kind, workspace totals, and account-level
   fleet status
3. an explicit `Workspaces` area for open, create, and lifecycle actions
4. an explicit `Access` area for roster, invites, role changes, and removals
   `Access` copy should stay terse and task-led; the section exists to do the
   roster job, not to teach Pulse's internal role model at length.
5. an explicit `Billing` area that leads with hosted billing when applicable
   and nests self-hosted commercial utilities only when relevant
   `Billing` should default to a single obvious task picker, not a broad
   dashboard of overlapping billing explanations, and the active self-hosted
   task panel should stay hidden until the user opens that exact billing job.
6. a `Support` area that is present only as an escalation path
   `Support` should collapse to failed-path routing plus the minimum escalation
   packet needed for handoff. Self-hosted-only accounts must collapse further
   to the self-hosted billing escalation path only and must not surface hosted
   workspace or access routes. Hosted view-only accounts must keep those hosted
   routes in review-plus-owner/admin-handoff language rather than implying live
   mutation authority.
7. explicit action groups, not anonymous menu affordances
8. explicit unavailable-state panels for top-level tasks that are not active
   for the current account shape
9. a compact narrow-screen task switcher that preserves task-first navigation
   without letting navigation chrome dominate the page before the active task,
   while keeping the active task visibly in-frame when the strip scrolls
10. a compact narrow-screen account context strip that keeps account identity,
    role, workspace count, and billing state visible without pushing the live
    task below the fold

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

Workspace health in Pulse Account must distinguish three states explicitly:

1. `healthy`
2. `checking` when no completed health check exists yet
3. `unhealthy` when the latest health check failed

The portal must not label a failed health check as `checking`.

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

1. one named `Pulse Account` shell and task-first IA centered on `Overview`,
   `Workspaces`, `Access`, `Billing`, and `Support`
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
7. an overview model that makes the next obvious action explicit instead of
   teaching the user Pulse's internal account model before they can act

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
