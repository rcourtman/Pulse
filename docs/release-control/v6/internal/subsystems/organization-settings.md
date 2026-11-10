# Organization Settings Contract

## Contract Metadata

```json
{
  "subsystem_id": "organization-settings",
  "lane": "L14",
  "contract_file": "docs/release-control/v6/internal/subsystems/organization-settings.md",
  "status_file": "docs/release-control/v6/internal/status.json",
  "registry_file": "docs/release-control/v6/internal/subsystems/registry.json",
  "dependency_subsystem_ids": ["frontend-primitives"]
}
```

## Purpose

### Saved sharing decisions keep exact parties and whole-page context

Incoming and outgoing sharing rows retain the saved organization's full key
beside its display name (or once for an unnamed, unlisted or ID-named party).
Resource names and type/keys, each saved role, pending/active state, acceptance
context and text-labelled actions remain individually available. Names and
opaque keys are isolated literal text. This does not infer a grant from a label.

The existing shared grid retains its stable-key row state, accepted-row proxies,
callbacks and native table/row/cell structure. Only sharing uses this scoped
layout; no shared primitive changes.

Below the existing 34rem table-container threshold the two identity cells
share a line, followed by separate status/action areas and full-width acceptance
context. Managers keep 44px touch controls. Readers without sharing actions
keep a compact metadata rhythm instead of an empty action line. Status and
Actions retain accessible header labels. Empty inventories keep their existing
meaning, wide tables retain Requested and Access, and no record is aggregated,
truncated or moved behind hover or disclosure. Whole-page review includes the
natural first view and incoming names/roles at the same outgoing anchor, not
just each table in isolation.

Only sharing presentation changes. The state owner's accepted-row identity,
reader invalidation, confirmations, exact encoded mutation target, CSRF/org
binding, no automatic write replay and saved readback remain unchanged.
Registered sharing-panel and row-transport tests cover the visible keys across
renamed/reordered inventories, key-only fallbacks, long/crafted literal text,
read-only sessions and existing mutation/recovery flows. Actual-App fixtures
and comparable desktop/phone captures are required for the consuming UI;
these do not establish original-client, live authorization or release recovery.

### Sharing choices expose the selected saved identities

Create Share distinguishes namesake recipients by organization ID and namesake
resources by complete type and ID in the native option text. Each current
selection also has a wrapping, associated description containing its name and
separate bidi-isolated ID, so clipping in a closed native select cannot hide the
chosen identity. ID-only names are not repeated. Descriptions follow current
option membership, not leftover draft fields: withdrawn choices and explicit
manual resource entry do not retain a misleading selected-resource description.
Names and keys are presented at render time only; governed resource names in
the grant payload and all existing selection, authority, CSRF and no-write-retry
fences are unchanged. No recipient is inferred or preselected.

Phone spacing keeps the role, resource guidance, manual-entry control and
Create Share action available alongside the additional identity text; desktop
retains its existing layout. Registered sharing panel/transport tests exercise
visible-name selection, namesake/type-distinct keys, name/ID fallbacks, crafted
names, rename/reorder/withdrawal, disabled writes and uncertain-result reloads.
Comparable actual-App desktop/phone light/dark browser fixtures bind the display
and exact one-write/saved-readback behavior. This is selection clarity, not live
authorization, original-client recovery or release availability.

### Keyboard recovery after an uncertain access change

Organization Access hands lost panel focus to its existing unavailable warning
when a focused control is removed after an uncertain change. The next Tab reaches
the explicit read-only Reload access action. A reload that succeeds focuses the
accepted organization summary; a failed reload focuses the new warning. Both
targets are programmatically focusable, not additional Tab stops. Initial and
background loads and successful-mutation refreshes do not focus the summary.
Focus elsewhere is preserved, and context changes, hidden surfaces and disposal
cancel stale handoffs. Visible information, mutation/authority fences, readback
and no-write-retry behavior are unchanged. `OrganizationAccessPanel.test.tsx`
covers failure, reload, no-steal, context and cleanup paths. Consuming-App browser
proof covers keyboard recovery and exact one-write/read-only reload behavior at
desktop and phone sizes in both themes; native screen-reader speech is unmeasured.

### Saved role IDs distinguish namesake access decisions

Roles and User Access display the complete saved role ID separately from its
mutable name. Role actions, permission-list labels, edit/delete dialogs and
save/deletion outcome messages identify that same key, including when only the
ID distinguishes two roles. IDs are neither normalized nor inferred from names;
long values wrap, and visible IDs use bidi isolation. The existing role editor
still keeps its saved ID field disabled. Individual permissions, descriptions,
built-in read-only status and all existing controls remain available.

User Access shows each assigned role's ID in its inventory and role choices.
An assigned ID absent from the current catalogue is labelled as not in the role
list and is shown read-only in the editor as a saved ID kept on save. It gains
no new toggle or grant authority. Saved permissions remain explicitly distinct
from unsaved selections. Compact phone editor spacing preserves the existing
choice/context/control visibility alongside the additional identity text.

The normal frontend suite covers namesake/empty/ID-looking names, unique exact
accessible labels, unchanged keys, both namesakes assigned, preserved unlisted
IDs, and cancelled/confirmed decisions. Real-client transport regressions check
the exact encoded mutation targets, saved payload/readback, sibling isolation
and uncertain-result recovery without replaying writes. Comparable native
Settings-shell/panel desktop and phone light/dark browser fixtures establish
presentation and consuming-client behavior, not full App/IdP, original-client
recovery, installed acceptance or release availability. Auth, CSRF, tenant and
accepted-row lifetime fences and backend mutation policies are unchanged.

The additional identity information has a declared, bounded UserAssignmentsPanel
reference allowance of 320 gzip bytes (5,924 to 6,244), not a reset to the measured
7,125-byte final chunk. Its resulting 7,181-byte cap retains the existing 15% tolerance;
all other chunk references, total JS/CSS limits and preload/integrity checks stay
unchanged. Shared inventory/editor identity rendering keeps the label definition
in one place. Source build/size checks and the consuming browser proof verify
this intentional access-clarity cost; they do not establish a speed improvement.

### Organization members keep identity and access controls together on phones

The existing Organization Access member table wraps complete saved user IDs,
including long same-prefix SSO identifiers, instead of widening the phone table
and leaving Role and Remove offscreen. Local phone column sizing and compact
cell padding retain native table semantics, each saved role, every role choice
and the text-labelled removal control. Member role selectors are at least 44px
tall on phones. Desktop keeps its Added dates and existing layout; the
pre-existing phone date visibility is unchanged. Read-only sessions retain full
identities and individual role badges without exposing mutation controls.

Only presentation changes: accepted-row identity, owner restrictions, deliberate
ownership confirmation, session/CSRF/organization fences, saved readback and
no-automatic-retry requests remain unchanged. The registered
`OrganizationAccessSelections.transport.test.tsx` uses long suffix-distinct
principals through real rendered controls/client transport, exact-key role
updates, cancelled/confirmed removal and ownership decisions without changing
siblings. Comparable native Settings-shell/panel browser fixtures exercise the
same data at desktop and phone light/dark, plus read-only policy withdrawal.
This establishes source/presentation protection, not original-client, IdP or
installed recovery.

### User Access identifies the exact saved principal at every decision

User Access keeps the full saved username readable alongside display name,
email and provider in its inventory, manage dialog and access-removal
confirmation. Mutable claims are not unique: two accounts may have identical
names, email and provider labels. Action and dialog accessible names, uncertain
mutation feedback and removal completion therefore also identify the saved
username. A username already serving as the display name is not repeated.
This presents only the existing admin inventory fields; it adds no read or
write authority, metadata source or outbound disclosure.

Long identity values wrap instead of truncating or widening the phone table.
Every assigned role and both touch-sized actions remain visible with the user,
not behind a tooltip, horizontal scroll or role count. The editor retains its
saved-permissions qualifier, selected roles, Close, Cancel and Save; removal
retains session-revocation effects and the identity-provider re-creation warning.
The state model still admits exact accepted row objects and sends only the
selected username with its existing reader/org fence and no automatic retry.

`RBACPaywallPanels.test.tsx` exercises two namesakes through the real panel's
selection, permission read, destructive confirmation, exact-key removal and
uncertain role update. `rbacPresentation.test.ts` covers exact identifier labels
and local/email fallback. Browser proof pairs the native Settings shell and
User Access with the same HTTP fixtures on desktop and phone light/dark,
including exact encoded PUT/DELETE targets and capability withdrawal.

### Single-role reads distinguish failed storage from absence

The existing `GET /api/admin/roles/{id}` uses the selected tenant manager's
optional error-aware role reader. A failed role-row or permission read returns
the fixed `503 rbac_store_unavailable` response, never `404 not_found` or a
successful role with incomplete permissions. The response discloses no stored
role data or storage error details. A successfully queried missing role still
returns 404; complete roles and deliberate empty permissions retain their JSON.
Legacy manager implementations without the optional reader remain supported.

`TestRBACRoleDetailRead*` in the normal API suite covers closed-store and invalid
permission-JSON failures in the default and a named organization, same-ID roles
in three tenants, empty/missing roles, error-aware partial results and legacy
compatibility. This changes no route, gate, grant, mutation, UI or inheritance
evaluation. It is source protection for a role read, not original-client recovery.

### Overlapping organization edits preserve current access

The saved-organization boundary refuses a complete snapshot superseded by
another member, invitation, share, owner, name or lifecycle change. Existing
mutations report `409 organization_changed`, without merging grants, changing
credentials or replaying the request. Creation cannot overwrite saved metadata,
and a loaded snapshot cannot recreate a deleted organization. The snapshot is
internal, JSON-excluded bookkeeping, not a new client permission or API field.

Overview, Access and Sharing consume that refusal through their existing
unconfirmed-change states and explicit saved-data Reload controls. Reload reads
current metadata; it never repeats the refused mutation. A fresh intentional
action remains available under the existing authority rules. Normal-suite
`TestOrgMutationSnapshot*` and `TestOrganizationSnapshot*` cover this server-side
boundary, including real registered session/CSRF/tenant routes. Comparable
actual-panel desktop/narrow proof consumes the native before/after response
bytes. This is saved-state protection, not original-client/IdP recovery,
historical impact measurement or a cross-process storage-coordination claim.

### Equal security-status values keep the User Access reader

User Access compares effective runtime readiness, RBAC/user-management admission
and principal values before replacing its reader. A new security-status object
with identical values preserves search, role selections, saved-permission reads,
removal confirmation and any pending assignment or removal request. Controls
remain disabled until that request's real outcome; a refresh neither rolls back
a server-applied change nor permits another write. Confirmed success obtains one
fresh inventory. An uncertain outcome retains its warning across equal refreshes
and requires **Check saved access**, a read-only recovery. Definite rejections
retain their correctable draft and local error.

Actual principal, runtime readiness, entitlement or management-capability changes,
organization rebinds (including the same ID) and disposal still withdraw and abort
immediately and suppress obsolete completions. Existing organization binding,
CSRF, no-retry transport, endpoints, JSON payloads and backend authorization are
unchanged. `UserAccessReaderLifetime.test.tsx` pins equal-value refreshes during
drafts, saved-permission reads, both mutations and their success/uncertain/refused
outcomes, plus true withdrawal. `RBACPaywallPanels.test.tsx` exercises the rendered
pending dialog, local uncertainty/focus and read-only recovery. Comparable
production-panel/client browser proof covers desktop/narrow and both themes.
`UserAccessTransport.test.tsx` keeps the real RBAC client and CSRF preparation,
preserving one save or removal across equal refreshes before and after dispatch;
it checks both confirmed outcomes and explicit read-only uncertainty recovery.
This safeguard is not original saved-account, SSO or installed recovery.

### Overview explains the current session's write restriction

When organization details are visible but session presentation policy is
read-only, Overview explains that organization details are read-only for this
session, including for the displayed owner. Only a writable session without
organization-management membership receives the existing admin-or-owner hint.
Both states keep the name input and Save disabled. Policy and principal changes
update the explanation through the existing reader lifecycle; equal refreshes
do not imply new authority. No management predicate, request, role or server
authorization changes. Registered presentation-helper and production-panel/
store/client tests cover these distinctions and restored writable-owner use;
comparable desktop/narrow browser proof retains the summary cards, controls and
membership distinctions. This is not original SSO or installed-client recovery.

### Equal-reader refreshes preserve pending organization changes

Organization Overview, Access and Sharing hold same-reader events, repeated
effective presentation-policy updates and direct reloads behind a pending mutation.
The accepted snapshot remains visible with Saving and disabled mutation controls;
a refresh cannot undo server work or establish that it is safe to submit again.
Success emits the existing organizations-changed event, obtaining one fresh
snapshot. An uncertain or mismatched mutation response keeps the existing
unavailable state and explicit Reload recovery, not an automatic write retry.
Overview retains its submitted display-name draft and Saving state across equal
security-status presentation-policy updates; after success a saved read supplies
the name. Uncertainty withdraws the editor until a saved read succeeds; Reload
checks saved details without repeating the rename. Later directory or policy
refreshes may also perform saved reads, never repeat the write. A pending
mutation may still be preparing CSRF and need not have reached the server. A
never-settled write remains pending until completion or reader withdrawal, rather
than being presented as rolled back.

Observed organization switches, reactive principal/entitlement or surface
withdrawal, read-only posture and disposal still invalidate and cancel immediately.
A silent organization-field change fails the live reader check at the next load
or completion; this repair adds no observer for that pre-existing path. Old-reader
feedback is suppressed; local cancellation does not undo a server-applied write.
Expected organization, session/CSRF binding, no-retry requests, server authority
and deliberate ownership confirmation remain unchanged. A production
`org_switched` event clears runtime capabilities even for the same ID: that
unknown-authority interval is withdrawal, not an equal-reader directory refresh.
The runtime capability reset remains intact. A failed forced capability refresh
also cancels through the existing fail-closed Community fallback.

The registered Access and Sharing panel tests exercise all ten plain mutation
paths plus ownership confirmation, successful and uncertain completion, explicit
recovery, blocked repeats, direct reloads and true authority/lifetime withdrawal.
Browser fixtures consume the production panels, state and HTTP client with
delayed safe responses at desktop/narrow viewports. These are source safeguards,
not original SSO, secret, tenant or installed-client recovery.
`OrganizationOverviewMutation.client.test.tsx` isolates equal directory, policy
and capability refreshes with the production Overview panel, stores and HTTP
client, plus mismatched responses, failed saved reads, read-only recovery and
late completion after actual principal, organization, policy, entitlement or
component withdrawal. `OrganizationOverviewPanel.test.tsx` also pins normalized
request scopes through the pending rename and resulting saved read.

### Invitation decisions identify the exact organization

Your Invitations shows each organization's display name **and** ID beside its
existing role, inviter, date and Accept/Decline controls. A missing name or one
equal to the ID shows the ID once. Same-named organizations therefore remain
distinguishable without a tooltip or disclosure. Each invitation is an accessible
group, and both actions name its organization and ID. Long names, IDs and inviter
identities wrap instead of escaping the card at narrow or desktop widths.
`getOrganizationInvitationPresentation` owns that visible and accessible copy
in the existing organization presentation helper, with its registered unit proof.

The existing current-reader, session/CSRF and no-automatic-retry transport is
unchanged: the action uses the invitation ID, not the display name or active
organization. Read-only sessions keep those decisions disabled. No membership
or permission is added by presentation alone. `OrganizationAccessPanel.test.tsx`
covers same-named decisions, exact dispatch/readback, ID fallbacks and read-only
controls. Comparable actual-panel desktop/narrow browser fixtures exercise the
real state, API and HTTP client with synthetic invitations, including keyboard
acceptance/decline, long identities and dark theme. This does not establish
original-account recovery or installed service acceptance.

### Sharing requests require an explicitly chosen recipient

A fresh Organization Sharing draft starts at **Select organization**. Neither
directory order nor a sole peer selects a recipient on the operator's behalf.
Choosing a discovered resource or completing manual resource fields cannot
enable Create Share until a current destination is chosen. Direct submission
with no destination sends nothing and reports the existing required-field error.
The untouched initial field is not presented as an error.

An explicit recipient remains selected by ID across same-reader reloads,
directory reordering and failed-read recovery. Clearing the choice or losing
the destination keeps it invalid until a current recipient is chosen again;
a returning destination is not renewed consent. Organization/principal changes
clear the recipient with the existing resource draft. Current-reader admission,
session/CSRF binding, cancellation and disabled automatic retries are unchanged.

`OrganizationSharingPanel.test.tsx` exercises zero/one/multiple peers, quick-pick
and manual paths, untouched/cleared/removed drafts, explicit payloads and reader
changes. Comparable desktop/narrow actual-panel browser fixtures exercise the
state, resource discovery, OrgsAPI and HTTP client through pending-share readback.
This is prevention of an unintended **pending request**, not a change to the
target organization's existing acceptance requirement or server authority.
It does not establish original-account or installed tenant recovery.

### Ownership changes require current, deliberate confirmation

Both existing ownership paths (the member-role selector and Invite Member form)
open the same confirmation before sending any change. It names the organization
and ID, accepted existing member and current owner, and explains that the old
owner becomes an Admin. Cancel, Escape and backdrop dismissal send nothing;
the row selector continues to display its saved role while review is pending.
No additional role choices, membership or server authority are granted.

The confirmation is bound to one exact intent, read generation, organization,
principal and accepted member. Refresh, organization/principal/presentation or
entitlement withdrawal and disposal discard it; stale callbacks cannot confirm
or cancel a newer intent. The form path also rechecks its original draft. A
confirmed transfer uses the existing session/CSRF/expected-organization transport
with cancellation and no automatic retries. Only a member result naming the
confirmed target with the Owner role is accepted as a confirmed write. A failed,
unreadable, malformed or mismatched response requires the
existing explicit saved-access read, never a repeated write. The accepted saved
Owner remains visible in the disabled selector to non-owner administrators;
Owner is not offered for other members or new invitations from those readers.
An already-entered Owner invitation draft remains visible but disabled, labelled
**Owner (current owner only)**, after a saved-access read establishes ownership
was transferred. Submitting that stale draft is disabled, not silently changed
to a lesser role or replayed.

`OrganizationAccessPanel.test.tsx` and the real-client
`OrganizationOwnershipTransfer.client.test.tsx` cover both entry paths,
confirmation, truthful saved-role readback, context withdrawal and uncertain
committed writes. Comparable desktop/narrow browser fixtures exercise the actual
panel, API and client. These establish frontend safeguards, not original-account,
installed service or server-authorization acceptance.

### Audit events and signature checks withdraw with the reader

The existing Settings Audit Log passes its current principal, explicit audit
read capability and non-secret token/scope identity into the viewer. Accepted
events, counts, signature results, pending checks and batch feedback belong to
that reader, organisation and entitlement. Switching, withdrawal and disposal
clear them and cancel pending work, including work that ignores cancellation.
Returning to the same identifiers does not readmit an old result. An unchanged
status response does not invalidate a live request. Every read/check uses the
expected organisation and disabled automatic retries. Refresh replaces the page
and its verification generation; stale row objects are not actionable. Read or
verification refusal stays explicitly unavailable until an admitted refresh,
never a false empty inventory or an inherited Verified badge. Normal audit
reader/request and registry tests plus desktop/narrow actual-App browser proof
cover these boundaries. Backend audit authorization/redaction and original
administrator/IdP recovery remain separate, unchanged responsibilities.

### Audit destinations belong to their current reader

The existing Audit Webhooks panel receives the live principal and explicit
read/write capabilities from Settings. Accepted endpoint URLs, unsaved entries,
removal confirmation and inline feedback belong to that reader, selected
organisation and audit-logging entitlement. A principal, read/write authority
or entitlement change, organisation switch or disposal aborts outstanding work
and erases that state. Returning to the same principal or organisation does not
readmit an old response. Equal-reader status refreshes retain the current draft.

A new admitted reader loads configuration before any replacement. Failed and
malformed reads remain unknown, not an empty destination list. A failed refresh
within the same reader retains their last accepted destinations and draft, but
blocks edits until an explicit read-only Retry succeeds. Read-only readers can
recover without acquiring write access. Storage drift noticed at an action or
completion withdraws old state rather than dispatching that action in another
organisation. Reads and writes keep the existing expected-organisation, abort
signal and no-automatic-retry client contract; backend enforcement is unchanged.

Only accepted current inventory enables add/removal. Mutations are serialised,
and removal requires the current in-page confirmation. Refused additions retain
the draft; refused removals retain the confirmation with fixed inline feedback.
Only explicit retry repeats a write. An obsolete success/refusal/completion
cannot finish a newer read or write, restore old URLs, clear a later draft,
close another confirmation or publish global feedback. Client cancellation
cannot undo a server-applied change; recovery reads its actual configuration.

Registered `AuditWebhookPanel` and registry-context tests cover reader changes,
authority loss/recovery, malformed/refused reads, storage drift, disposal and
late read/write results. Actual App browser proof at desktop and narrow widths
exercises the producer through Settings, its registry, API client and dialogs.
Synthetic proof is not native receiver delivery, IdP or original tenant recovery.

### Failed Settings access checks withdraw the previous grant

A same-reader security-status refresh keeps the mounted panel and its draft
while checking. Once a refresh fails (including HTTP refusal, unavailable
transport or invalid JSON/status shape), Settings discards the previous status,
unmounts capability-gated panels and withdraws shared Settings capability links.
It keeps the existing safe General fallback and explains that access is unknown,
not that permission was denied or authentication disabled. Explicit Retry only
checks status again; it does not change permissions or repeat a mutation.
Pending Retry keeps withdrawn navigation hidden until the new check succeeds.

Status reads are deduplicated, cancellable and not automatically retried. Panel
disposal cancels the read and prevents late status from restoring local or shared
capabilities. A failed check does not reset the separate read-only/demo policy to
permissive defaults. Successful checks still use the served capabilities and
existing navigation rules, with no backend or authentication-policy change.
Normal `useSettingsAccess.refresh` and existing access/navigation tests exercise
failure, recovery, pending same-reader preservation and disposal. Actual-App
browser proof covers the password-dialog close producer, failed check, safe
fallback and read-only retry at desktop and narrow widths; it is not original
administrator, IdP or installed recovery.

### Role administration belongs to the current reader

The existing Roles screen binds its accepted role inventory, unsaved permissions,
editor, deletion confirmation and local action feedback to the selected
organisation, authenticated principal and explicit Roles capability. Losing any
of them, losing RBAC or disposing the panel aborts outstanding requests and
withdraws that state. Returning to the same organisation or role ID does not
readmit an earlier response. Obsolete successes, refusals and completions cannot
replace current data, finish a newer loading state, refetch or announce an old
role name to the next reader.

Reads and writes use the existing API client with an expected organisation,
cancellation signal and automatic retries disabled. Only accepted, structurally
valid inventory enables actions. An unread or malformed response is unavailable,
not an empty role list, and its inline Retry performs only a read. Built-in and
obsolete row objects remain uneditable. Save and delete are serialised; deletion
requires the current in-page confirmation, which is withdrawn on context loss.
A definite request/permission/validation rejection preserves the live draft or
deletion confirmation with fixed inline feedback. A missing response, timeout,
server error or unreadable success leaves the result unconfirmed: the old
inventory, editor and confirmation withdraw, further writes remain disabled,
and a focused notice offers an explicit read-only saved-role check. Failed or
malformed reads keep that uncertainty; only a current, valid inventory restores
actions, without replaying a write or claiming that it was rolled back. The
notice and its role name withdraw with their reader. Closing an editor erases
its draft. Saved permission strings outside the editor's shortcut
list remain visible and unchanged unless explicitly edited. Success feedback
belongs to this panel rather than a global identity-bearing notification. The
containing Settings renderer reads optional current entry props rather than
retaining a conditional accessor after a panel is withdrawn. Revocation keeps
the existing safe navigation fallback usable instead of raising the route error
boundary; a same-reader status refresh does not remount the panel or lose its
draft. Backend authorisation, endpoints and permission semantics remain unchanged.

The role list keeps every saved `action:resource` permission visible without an
editor, hover, expansion or horizontal scrolling at phone widths, including
built-in roles that cannot be edited. Phones place the complete list beneath
its role identity; desktop keeps the Permissions column. Wildcards and saved
strings beyond the editor shortcuts remain exact, long identities and grants
wrap, and an accepted empty permission list says "No permissions" for that
role, not that its users have no access. Identity, description, built-in
read-only distinction and edit/delete controls remain available alongside the
grants. The existing inventory-withdrawal boundary covers both projections.
Registered RBAC panel tests cover the complete saved list and empty state;
comparable desktop/narrow browser proof checks actual visibility, wrapping,
all row controls and unchanged saved values through the editor.

Normal `RolesReaderLifetime`, RBAC API, paywall and registry tests cover
read/write races, revoked authority, licence loss, disposal, stale row IDs,
malformed reads, exact request binding, definite rejections and unconfirmed
create/update/delete recovery. Browser proof
exercises actual App/Settings/Role consumers at desktop and narrow widths using
safe synthetic HTTP producers; it is not installed administrator or IdP recovery.


### Overview reads and name changes belong to the selected organization

Overview clears its accepted metadata, member list and unsaved name when a
selection, current user, presentation gate or lifetime changes. All reads carry
one cancellation signal and the expected organization through the existing
session-preferred API client, with automatic retries disabled. Only the current
read generation may publish a matching organization and a complete member list;
a late success, failure or completion cannot replace a newer selection or end
its loading state. A failed read is an inline unavailable state, never an empty
member list, and offers an explicit read-only reload.

Renaming requires the current accepted snapshot, owner/admin authority and a
non-read-only session. Dispatch binds the exact organization even across a CSRF
wait and permits only one current write, without automatic write retries. A
withdrawn or disposed completion cannot change data or notify a later selection.
An unconfirmed write clears the editable snapshot and requires a saved read;
reload does not repeat the rename. Successful writes notify the existing
organization-change bus and refresh the saved snapshot. Normal-suite Overview,
OrganizationReadLifetimes and organization-client tests cover those boundaries;
desktop/narrow browser proof exercises the actual panels and API client with
safe HTTP fixtures, not an installed tenant or SSO recovery claim.


### Saved permissions belong to one open user-access dialog

User Access displays only an admitted saved-permissions read for the principal
currently being managed. Opening or closing a dialog invalidates earlier reads,
including earlier reads for the same principal; an obsolete success, failure or
completion cannot replace data or finish loading in a newer dialog. Disposal
and loss of the existing RBAC capability invalidate the pending read too.
Pending or failed reads never retain another user's permission badges or
claim that the selected user has no access. A persistent, fixed-text failure
notice offers an explicit read-only retry that preserves unsaved role selections
and permits at most one current read. It never retries a role update or removal.

The saved-permissions section is not a simulation of unsaved role choices.
Only an accepted empty response describes empty saved permissions; it does not
predict access after save. Normal-suite `RBACPaywallPanels.test.tsx` covers
cross-user and same-user reopen races, loading and failure, explicit retry,
draft preservation, accepted empty results and unchanged exact-principal writes.
Desktop and narrow browser proof exercises the real User Access route and API
client with safe HTTP fixtures. Backend authorization and the existing RBAC
gate remain the authority; a successful read grants no write capability.

A member test announces and reveals its result beside that exact member rather
than leaving feedback in the off-screen global footer. The normal primary
Test connection result stays in the footer; both remain endpoint-qualified
and do not update background or agent health. The member-section test and
browser interaction proof check the locally scoped live result.

### Proxmox endpoint trust in the existing connection editor

The primary SSL fingerprint edits only its authority/base path. Each peer shows
the server-projected `tlsEndpoint`, its own effective pin and, when applicable,
unknown first-use state. A deliberate peer edit or CA-validation action submits
only that endpoint; unchanged members are not cleared or assigned the primary
pin. Explicit CA validation keeps Verify SSL enabled. Unknown first-use policy
is not described as saved CA validation.

Single-endpoint tests use stored credentials and optional draft TLS choices,
not unsaved addresses or credentials. They do not save trust, mark background
or agent health recovered, or claim other endpoints were tested. Draft edits
invalidate earlier results, including a pending result from an older draft.
After a successful trust save the configured-node owner reads back the actual
server policy before reopening; write-only trust payloads are not cached as
configuration. Disabled collection scope and secrets remain unchanged.

Normal `nodeModalModel`, `useNodeModalState.connectionTest` and
`NodeModalClusterMembersSection` tests cover partial trust submission, explicit
empty choices, unknown members, saved-credential probes and stale-result
invalidation. Desktop/narrow browser proof consumes actual source-produced
node and probe responses through `NodeCredentialSlot` and the configured-node
state owner. These fixtures do not establish native IdP, agent or appliance
recovery.

Own organization role/share semantics and the canonical settings surfaces that
let users review organization metadata, manage membership, assign roles, and
create, review, and approve cross-organization shares.

## Canonical Files

1a. `docs/RBAC.md`
1b. `frontend-modern/public/docs/RBAC.md`
1. `frontend-modern/src/api/orgs.ts`
2. `frontend-modern/src/api/rbac.ts`
3. `frontend-modern/src/components/Settings/OrganizationAccessLoadingState.tsx`
4. `frontend-modern/src/components/Settings/OrganizationAccessManagementSection.tsx`
5. `frontend-modern/src/components/Settings/OrganizationAccessInvitationsSection.tsx`
6. `frontend-modern/src/components/Settings/OrganizationAccessMembersSection.tsx`
7. `frontend-modern/src/components/Settings/OrganizationAccessPanel.tsx`
8. `frontend-modern/src/components/Settings/OrganizationIncomingSharesSection.tsx`
9. `frontend-modern/src/components/Settings/OrganizationOutgoingSharesSection.tsx`
10. `frontend-modern/src/components/Settings/OrganizationOverviewDetailsSection.tsx`
11. `frontend-modern/src/components/Settings/OrganizationOverviewLoadingState.tsx`
12. `frontend-modern/src/components/Settings/OrganizationOverviewMembersSection.tsx`
13. `frontend-modern/src/components/Settings/OrganizationOverviewPanel.tsx`
14. `frontend-modern/src/components/Settings/OrganizationSharingCreateSection.tsx`
15. `frontend-modern/src/components/Settings/OrganizationSharingLoadingState.tsx`
16. `frontend-modern/src/components/Settings/OrganizationSharingPanel.tsx`
17. `frontend-modern/src/components/Settings/RBACFeatureGateSection.tsx`
18. `frontend-modern/src/components/Settings/RolesEditorDialog.tsx`
19. `frontend-modern/src/components/Settings/RolesPanel.tsx`
20. `frontend-modern/src/components/Settings/settingsPanelRegistryContext.tsx`
21. `frontend-modern/src/components/Settings/useOrganizationAccessPanelState.ts`
22. `frontend-modern/src/components/Settings/useOrganizationOverviewPanelState.ts`
23. `frontend-modern/src/components/Settings/useOrganizationSharingPanelState.ts`
24. `frontend-modern/src/components/Settings/UserAssignmentsDialog.tsx`
25. `frontend-modern/src/components/Settings/UserAssignmentsPanel.tsx`
26. `frontend-modern/src/components/Settings/useRBACFeatureGateState.ts`
27. `frontend-modern/src/components/Settings/useRolesPanelState.ts`
28. `frontend-modern/src/components/Settings/useUserAssignmentsPanelState.ts`
29. `frontend-modern/src/utils/organizationRolePresentation.ts`
30. `frontend-modern/src/utils/organizationSettingsPresentation.ts`
31. `frontend-modern/src/utils/orgUtils.ts`
32. `frontend-modern/src/types/rbac.ts`
33. `frontend-modern/src/utils/rbacPresentation.ts`
34. `internal/api/access_control_handlers.go`
35. `internal/api/enterprise_extension_rbac_admin.go`
36. `internal/api/org_handlers.go`
37. `internal/api/org_lifecycle_handlers.go`
38. `internal/models/organization.go`

## Shared Boundaries

Organisation overview, access and sharing use the authenticated currentUsername for owner/member comparisons, including local sessions scoped to a non-default organisation. They must not infer the current principal from the privileged configured-administrator field when currentUsername is present. Existing owner/admin membership checks and server-side enforcement remain authoritative.

1. `frontend-modern/src/api/orgs.ts` shared with `api-contracts`: the organization frontend client is both an organization settings control surface and a canonical API payload contract boundary.
2. `frontend-modern/src/api/rbac.ts` shared with `api-contracts`: the RBAC frontend client is both an organization settings control surface and a canonical API payload contract boundary.
3. `internal/api/access_control_handlers.go` shared with `api-contracts`: RBAC role and user-assignment handlers are both an organization settings control surface and a canonical API payload contract boundary.
4. `internal/api/enterprise_extension_rbac_admin.go` shared with `api-contracts`: RBAC admin extension endpoints are both an organization settings control surface and a canonical API payload contract boundary.
5. `internal/api/org_handlers.go` shared with `api-contracts`: organization management handlers are both an organization settings control surface and a canonical API payload contract boundary.
6. `internal/api/org_lifecycle_handlers.go` shared with `api-contracts`: organization lifecycle handlers are both an organization settings control surface and a canonical API payload contract boundary.

## Extension Points

1. Add or change organization role and share semantics through `internal/models/organization.go`
2. Add or change organization access, overview, sharing, RBAC feature-gating, role-management, or user-assignment presentation through `frontend-modern/src/components/Settings/OrganizationAccessPanel.tsx`, `frontend-modern/src/components/Settings/OrganizationAccessLoadingState.tsx`, `frontend-modern/src/components/Settings/OrganizationAccessManagementSection.tsx`, `frontend-modern/src/components/Settings/OrganizationAccessInvitationsSection.tsx`, `frontend-modern/src/components/Settings/OrganizationAccessMembersSection.tsx`, `frontend-modern/src/components/Settings/OrganizationOverviewPanel.tsx`, `frontend-modern/src/components/Settings/OrganizationOverviewLoadingState.tsx`, `frontend-modern/src/components/Settings/OrganizationOverviewDetailsSection.tsx`, `frontend-modern/src/components/Settings/OrganizationOverviewMembersSection.tsx`, `frontend-modern/src/components/Settings/OrganizationSharingPanel.tsx`, `frontend-modern/src/components/Settings/OrganizationSharingCreateSection.tsx`, `frontend-modern/src/components/Settings/OrganizationSharingLoadingState.tsx`, `frontend-modern/src/components/Settings/OrganizationOutgoingSharesSection.tsx`, `frontend-modern/src/components/Settings/OrganizationIncomingSharesSection.tsx`, `frontend-modern/src/components/Settings/settingsPanelRegistryContext.tsx`, `frontend-modern/src/components/Settings/useOrganizationAccessPanelState.ts`, `frontend-modern/src/components/Settings/useOrganizationOverviewPanelState.ts`, `frontend-modern/src/components/Settings/useOrganizationSharingPanelState.ts`, `frontend-modern/src/components/Settings/RBACFeatureGateSection.tsx`, `frontend-modern/src/components/Settings/RolesPanel.tsx`, `frontend-modern/src/components/Settings/RolesEditorDialog.tsx`, `frontend-modern/src/components/Settings/useRBACFeatureGateState.ts`, `frontend-modern/src/components/Settings/useRolesPanelState.ts`, `frontend-modern/src/components/Settings/UserAssignmentsPanel.tsx`, `frontend-modern/src/components/Settings/UserAssignmentsDialog.tsx`, and `frontend-modern/src/components/Settings/useUserAssignmentsPanelState.ts`
   Organization and RBAC tables may own their rows, columns, action cells, and
   domain copy, but their table scroll shell and embedded-frame treatment must
   come from `frontend-primitives` through `PulseDataGrid`/`Table`; do not add
   organization-local `overflow-x-auto`, negative-margin, or side-border
   compensation around grids.
   Their phone projection must keep the identity, current access/state, and
   available row action needed for an immediate decision. Added/requested
   timestamps and other secondary review metadata are desktop detail columns;
   actual role grants are decision information and remain visible on phones. Do
   not squeeze these decision fields into an unreadable six-column phone grid.
   Organization role and share-status badges follow the same primitive
   boundary: role/status semantics remain organization-owned, but visible badge
   shell, tone vocabulary, and label rendering must compose
   `OrganizationRoleBadge` / `OrganizationShareStatusBadge` over the shared
   `MetadataBadge` primitive instead of using `roleBadgeClass`,
   `statusBadgeClass`, or local pill span classes in each section.
3. Route organization and RBAC frontend transport changes through `frontend-modern/src/api/orgs.ts`, `frontend-modern/src/api/rbac.ts`, `frontend-modern/src/types/rbac.ts`, and `frontend-modern/src/utils/rbacPresentation.ts`
4. Keep backend organization management and lifecycle handlers aligned through `internal/api/org_handlers.go` and `internal/api/org_lifecycle_handlers.go`
   Organization resource shares may reference the canonical Docker / Podman
   and Kubernetes inventory resource types exposed by `/api/resources`, but the
   organization layer only validates the bounded resource-type vocabulary. It
   must not reinterpret a Docker image/volume/network/task/Swarm-node/Swarm
   secret/Swarm config row or Kubernetes
   Service/controller/storage/networking/config/event row, including
   ReplicaSets, EndpointSlices, NetworkPolicies, StorageClasses, ConfigMaps,
   Secrets, ServiceAccounts, Roles, ClusterRoles, RoleBindings,
   ClusterRoleBindings, ResourceQuotas, LimitRanges, PodDisruptionBudgets, or
   HorizontalPodAutoscalers, as a broader organization role, agent enrollment
   authority, secret-sharing channel, config-sharing channel, or command
   capability.
5. Keep RBAC role, assignment, and admin recovery transport aligned through `internal/api/access_control_handlers.go` and `internal/api/enterprise_extension_rbac_admin.go`

## Forbidden Paths

1. Duplicating organization role normalization or badge styling outside the canonical organization presentation helpers and shared organization badge wrappers
2. Reintroducing organization settings copy, validation, or empty-state strings directly inside feature panels instead of the shared organization presentation helpers
3. Letting share-role, RBAC role-assignment, or membership semantics drift between `internal/models/organization.go` and the governed organization settings surfaces

## Completion Obligations

1. Update the organization model, settings surfaces, and proof files together when role/share semantics move
2. Keep organization settings copy and validation inside the canonical organization presentation helpers
3. Keep organization role and share-status badge rendering on `OrganizationRoleBadge` / `OrganizationShareStatusBadge` and the shared metadata-badge registry rules
4. Keep the shared organization and RBAC transport proof routes explicit in `registry.json`; default fallback proof routing is not allowed for this subsystem
5. Update this contract whenever a new organization settings, role-management, or organization-domain helper entry point becomes canonical runtime surface area
6. Keep RBAC feature-gate copy on the organization presentation owner. RBAC
   may describe capability availability on paid self-hosted and hosted plans,
   but must not use Pro-badge titles or local browser upgrade telemetry in
   `RBACFeatureGateSection.tsx` or `useRBACFeatureGateState.ts`.
7. Keep organization overview, access, and sharing loading placeholders on the
   shared `SettingsLoadingSkeleton` primitive. Organization settings owns the
   membership, role, and sharing meaning of those loading surfaces, while
   frontend-primitives owns the skeleton animation, fill tokens, and table/card
   placeholder shells.
8. Keep RBAC role and user-assignment loading indicators on the shared
   `LoadingSpinner` primitive. Organization settings owns RBAC loading
   semantics; frontend-primitives owns spinner size, tone, and accessible
   status behavior.
9. Keep organization overview, invitation, member, share, RBAC role creation,
   role edit/delete row actions, and user-access management actions on the
   shared `Button` / `ActionIconButton` primitives. Organization settings owns
   role, membership, sharing, feature-gate availability, and row-action
   behavior; frontend-primitives owns primary, danger-outline, success-ghost,
   danger-ghost, ghost, accent, danger, focus, disabled, and settings-action
   chrome.
10. Keep role and assignment panels fail closed on store errors: clear stale
    rows, disable mutations, retain a visible retry notice, and do not render
    the normal empty state until a healthy response has been received.

11. Keep the canonical and shipped RBAC guides explicit that SSO
    authentication is identity, not instance-administrator authority. An
    SSO-only deployment must map a trusted IdP group to the built-in `admin`
    role (or retain a configured local administrator); `operator`, `viewer`,
    and unassigned sessions remain non-administrative even when no local
    administrator exists. Preserve this boundary in
    `frontend-modern/src/utils/__tests__/docsLinks.test.ts` together with exact
    canonical/public guide parity.

12. A rejected legacy RBAC import must not destroy the store. The import is
    transactional and leaves the legacy files in place, so a failure leaves the
    database un-migrated rather than half-migrated, which denies access rather
    than granting it. Failing manager construction instead takes the whole org's
    RBAC surface offline including `ResetAdminRole`, the operator's only way
    back, on a conflict an ordinary v5 upgrade can produce. The store therefore
    stays live and records the failure on `SQLiteManager.MigrationError`.
    Management routes still fail closed with `rbac_store_unavailable` through
    the handler manager accessors; recovery reaches the provider directly and is
    deliberately exempt. Regression coverage:
    `TestAdminRecoverySurvivesFailedLegacyRBACMigration` in
    `internal/api/contract_test.go`,
    `TestRBACHandlersReportLegacyMigrationFailure` in
    `internal/api/rbac_handlers_test.go`, and the rejection cases in
    `pkg/auth/sqlite_manager_test.go`.

## Current State

### AI Service Context settings panel removed (2026-10-11)

`settingsPanelRegistryContext.tsx` and the settings registry no longer carry
the `system-ai-discovery` panel: the AI Service Context page left with AI
Discovery. The `/settings/pulse-intelligence/discovery` path and the legacy
`?tab=system-ai-discovery` value still resolve to Assistant settings, so old
links land on a live page. Organization-scoped settings are unaffected.

This subsystem now sits under the dedicated security, identity, and privacy
lane so role boundaries, organization membership semantics, and cross-org
sharing behavior stay governed as a first-class trust surface.

Organization overview, access, sharing, roles, and user-assignment surfaces had
been sitting outside the governed subsystem map even though they define real
runtime expectations around membership management, least-privilege role
assignment, and cross-organization resource sharing. This contract now makes
that boundary explicit across both the settings surfaces and the shared
organization/RBAC transport files, instead of leaving those runtime paths as
`api-contracts`-only ownership.
Canonical organization role ordering is now part of that owned model as well:
`owner` outranks `admin`, which outranks `editor`, which outranks `viewer`, and
runtime checks must resolve that ordering through shared organization-role
comparison helpers rather than ad hoc equality checks scattered across model
and handler code.
That same hierarchy governs inbound share visibility and organization
management gates: a user may only see or accept a share when their effective
membership role satisfies the share's requested access role, and admin-capable
operations must continue to derive from the canonical role comparator instead
of duplicating owner/admin special cases.
The comparator itself is now part of the owned runtime boundary: helpers such
as `CanUserManage` and any share-filtering logic must route through the shared
organization-role ordering function so viewer/editor/admin/owner semantics stay
identical across model checks, handler authorization, and settings-surface
presentation.
That same canonical comparator now governs live membership transitions too:
promoting or demoting a member must immediately change whether that user can
manage organization settings, and organization listing must not leak non-member
tenants just because another org with the same user exists in the system.
That same owned membership boundary now requires explicit invitation acceptance
for new self-hosted org access. `internal/api/org_handlers.go` may update an
existing member's role immediately, but inviting a new `UserID` must persist a
pending invitation until that same authenticated user accepts it through the
canonical invitation transport. Owner transfer must fail closed unless the
target user is already an accepted member, and the access panel must surface
both the current user's inbox and manager-visible pending invitations through
the dedicated invitation section owner rather than binding membership as soon as
an admin types a username.
Incoming share visibility is part of that same boundary as well: a recipient
must only see inbound shares whose requested `accessRole` is satisfied by the
user's effective membership role in the target organization, using the shared
role comparator instead of handler-local owner/admin shortcuts.
Cross-organization sharing itself now follows an explicit target-consent
contract instead of unilateral source-side grant semantics. Creating a share
from one organization must persist a `pending` request, not live access, until
an owner or admin in the target organization accepts it through the canonical
incoming-share transport. Pending incoming shares remain visible only to those
target-org managers, while accepted shares remain visible to members whose
effective role satisfies the share's `accessRole`.
That same contract also governs share mutations after acceptance. Changing an
accepted share's requested `accessRole` must clear the old acceptance metadata
and move the share back to `pending`, so a source org cannot silently widen a
previously approved grant without renewed target-org consent.
Hosted organization membership and billing routes now also follow this owned
semantics boundary: for hosted tenant orgs, `internal/api/org_handlers.go`
must authorize organization operations from the seeded org membership and the
hosted subscription state rather than requiring the self-hosted
`multi_tenant` feature flag. Provisioned hosted workspaces must therefore keep
`org.OwnerUserID` aligned with the authenticated creator when that actor is
known, so organization-owner checks stay consistent across runtime auth and the
settings surfaces. The organization settings panels now also normalize org
scope through `frontend-modern/src/utils/orgScope.ts` instead of carrying
their own `getOrgID() || 'default'` fallbacks, so access, overview, and
sharing views stay aligned with the shared multi-tenant org context contract.
That same membership model separates durable identity from contact metadata:
`OwnerUserID` and member `UserID` are the canonical principal keys when stable
control-plane user IDs exist, while `OwnerEmail` and member `Email` carry
delivery/display email. Email-shaped owner/member IDs remain only as legacy
fallback and migration input, and hosted handoff may canonicalize those records
to stable user IDs without changing the stored role.
Live organization authorization must use strict user-ID checks from
`internal/models/organization.go`: `internal/api/authorization.go`,
`internal/api/org_handlers.go`, `internal/api/cloud_org_admin_auth.go`, and
settings-scope admin checks may authorize only against `OwnerUserID` or member
`UserID`. The legacy email-aware organization helpers remain migration and
delivery compatibility only, so a stable-principal org cannot be entered by a
session whose user string merely matches `OwnerEmail` or member `Email`.
Email-delivery flows that need to mint sessions, including hosted magic links,
must resolve contact email back through this organization model before
authorizing. `internal/models/organization.go` owns the email-to-principal
helper so handlers do not duplicate membership lookup or accidentally bind
sessions to email. A matching owner/member contact email without a stored
`OwnerUserID` or member `UserID` is not a principal; the helper must fail
closed instead of manufacturing a session key from email.
Before that organization lookup can lead to one-time token persistence, the
public hosted magic-link route must first resolve a valid canonical hosted
external URL. Unavailable URL configuration therefore returns the same generic
response for registered and unknown contact emails without touching either
supported token store, while configured URL success still reaches this stable
principal resolver. Stripe completion applies the same ordering before its
best-effort organization-member delivery and must not let an optional URL
failure disturb billing or event idempotency.
If one contact email matches multiple distinct stored principals, the model
must also fail closed instead of selecting a principal by owner/member row
order.
Organization-principal canonicalization may use email only when the stored
owner/member principal is itself legacy email-keyed. A contact email on an
already-stable owner/member must not authorize, replace, or migrate a different
stable subject.
That same org-control surface also treats owner transfer as a re-auth-bound
operation. Existing membership remains a prerequisite for the target user, and
the acting owner must present a fresh browser session minted through the
canonical login flow before `internal/api/org_handlers.go` will promote a new
owner, instead of trusting any still-valid long-lived session cookie.
That same settings surface now also inherits the runtime-versus-commercial
licensing split. Organization settings may consume runtime capability truth
from the shared runtime-capabilities contract, but billing identity and
upgrade posture stay outside this subsystem on the dedicated commercial
surfaces, and any public-demo suppression must come from the shared resolved
`presentationPolicy` rather than settings-local demo checks or billing
entitlement probes.
That resolved policy also governs organization settings discoverability and
bootstrap. `frontend-modern/src/components/Settings/useSettingsAccess.ts`,
`frontend-modern/src/components/Settings/settingsNavCatalog.ts`, and the
owned organization overview/access/sharing panel states must fail closed until
presentation policy resolves, then stay hidden in public-demo posture even if
the hosted runtime still carries a seeded default organization for transport
scope.
The organization sharing surface now also sources resource quick-pick labels
from the shared preferred resource display helper, so governed resources do
not fall back to raw names inside share creation.
Organization share resource-type normalization is API-contract-owned but
organization-settings consumes the resulting allowlist. Native provider
resource types such as TrueNAS `network-share` may be shared only after the
unified-resource and API contracts define them as canonical resource payloads;
organization settings must not invent a local alias or a separate share-only
taxonomy for those resources.
Organization settings empty, unavailable, and load-error states are part of
that same presentation boundary: the shared organization presentation helpers
must describe server capability and settings availability directly, rather than
falling back to generic `feature not available` or transport-style `failed to
load` copy.
The same helper also owns organization action notifications and confirmations:
success and failure messages for renaming, membership changes, and sharing
operations should stay specific and customer-facing, not terse operator jargon
or bare transport wording.
The organization access surface now follows that extracted-owner pattern too:
the panel is the shell, `useOrganizationAccessPanelState.ts` owns the
membership runtime, and the loading, management, and members views each live
in dedicated section owners instead of collapsing API lifecycle, permission
gates, and table rendering into one file.
The organization overview surface now follows that same extracted-owner
pattern: the panel is the shell, `useOrganizationOverviewPanelState.ts` owns
the org/member load and display-name runtime, and the loading, details, and
membership views each live in dedicated section owners instead of mixing
summary cards, form actions, and table rendering into the shell.
The sharing surface now follows the same extracted-owner pattern: the panel is
the shell, `useOrganizationSharingPanelState.ts` owns the API-backed runtime,
and the loading, create, outgoing, and incoming views each live in dedicated
section owners instead of staying collapsed into one file.
That same shell contract now also depends on the shared settings panel registry
context. `frontend-modern/src/components/Settings/settingsPanelRegistryContext.tsx`
must pass the effective authenticated username from `SecurityStatus` into the
organization overview, access, and sharing panels so membership-derived owner
and admin actions render consistently for local auth, proxy-auth, and SSO
sessions. Non-organization settings composition, including Pulse Intelligence
panel splits, may share the same registry context only when it does not fork or
reorder that current-user projection for organization overview, access, or
sharing panels.
The RBAC settings area now follows the same extracted-owner pattern as the
other modernized settings surfaces: `RBACFeatureGateSection.tsx` owns the
shared paywall CTA rendering, `useRBACFeatureGateState.ts` owns the shared
license and free-trial runtime, `useRolesPanelState.ts` plus
`RolesEditorDialog.tsx` own the roles runtime split, and
`useUserAssignmentsPanelState.ts` plus `UserAssignmentsDialog.tsx` own the
user-assignment runtime split. `RolesPanel.tsx` and `UserAssignmentsPanel.tsx`
remain the canonical render shells for those governed RBAC surfaces.
The User Access surface must present bounded SSO display-name, email, and
provider metadata ahead of the opaque stable authorization principal while
keeping that principal available for exact operator diagnosis. Search covers
both presentation metadata and the stable principal. User removal requires an
explicit confirmation, calls the canonical deprovision endpoint, and explains
that Pulse revokes assignments and active sessions but does not disable the
upstream IdP account; a later authorized SSO login may recreate the record.
That RBAC gate now also depends on the shared commercial navigation contract:
`RBACFeatureGateSection.tsx` may request the canonical `rbac` destination from
the shared license boundary, but it must render that destination through the
`frontend-primitives` typed upgrade link owner instead of assuming
organization-settings paywalls always leave the app in a new tab.
RBAC paid handoff must stay neutral in ordinary self-hosted settings flows.
Organization settings paywalls may route to explicit plan, activation, recovery,
support, or hosted handoff surfaces where presentation policy allows, but they
must not reintroduce in-app trial-start helpers, trial-specific denial copy, or
lane-local `startProTrial()` branches.
That same RBAC paywall surface now also depends on the runtime-versus-
commercial license split: RBAC enablement must stay on the runtime capability
store, while commercial routing stays on the canonical upgrade destination
helper backed by the shared commercial boundary. Organization settings must
not collapse those concerns back into one payload just because the same
paywall shell renders both, and RBAC feature gates must not start trials
directly.
That same posture split now also fixes RBAC bootstrap ownership.
`frontend-modern/src/components/Settings/useRBACFeatureGateState.ts` may
consume the resolved commercial-posture store for trial and upgrade copy, but
it must not issue its own mount-time `loadCommercialPosture()` read.
Authenticated-shell bootstrap belongs to
`frontend-modern/src/useAppRuntimeState.ts`, with only the governed first-run
setup completion surface allowed to bootstrap posture outside that shell.
RBAC paywall state should not read raw commercial posture fields locally or
offer direct trial-start actions from non-billing feature gates.
Under the free-first self-hosted v6 policy, those RBAC gates must also honor
`presentationPolicy.hideUpgrade`: ordinary self-hosted users should not see
RBAC trial CTAs, hard-sell upgrade copy, or paid-only Roles/Users/Audit
navigation by default. Direct routes may recover to the canonical free
settings surface, and entitled or hosted-mode installs may still render the
governed RBAC surfaces.
The hidden-upgrade posture also applies to locked-card titles and bodies
themselves: RBAC gates may show neutral unavailable-capability copy, but must
not keep `(Pro)`, plan-tier labels, or paid-action wording after the CTA is
suppressed.

Community SSO owns mapping identity-provider groups to the built-in `admin`,
`operator`, and `viewer` roles. That login-time mapping must remain available
without the `rbac` capability; the paid RBAC boundary begins at custom-role
creation and manual user-assignment administration. `docs/RBAC.md` and its
shipped `frontend-modern/public/docs/RBAC.md` mirror must state this boundary
identically, and the docs-links proof must pin both the byte synchronization
and the Community-versus-Pro wording.

Organization Settings is a browser-session administration surface even when
first-run also leaves a generated API token in browser storage. Its API client
must prefer the active session cookie for organization reads, invitations,
membership, sharing, ownership, and billing administration, while preserving
token fallback only when no session exists. Mobile proof must render Overview,
Access, Sharing, and Billing with populated multitenant fixtures at supported
phone widths; hosted Billing Admin requires a real hosted-mode capability
runtime and expanded-row proof rather than being counted through a redirect.

Governed actions now bind the authenticated actor to the request organization
and re-evaluate current membership plus granular plan/approve/execute RBAC at
the lifecycle decision and execution boundaries. No caller-supplied requester,
browser scope shortcut, or detached token can substitute for current tenant
authority. This Phase B1 slice defines the canonical per-action requirement and
supports quorum/separation semantics with durable decision CAS, but it does not
claim provider/MSP delegation, provider inheritance, tenant policy-default UI,
distributed budgets, or multi-actor UI; those remain governed L20 residuals
rather than organization-settings-local policy models.

Patrol Autopilot acknowledgement is tenant-bound backend authority, not an
organization-settings-local consent flag. The persisted actor organization,
activation organization, and evaluated request organization must match; a
self-digesting foreign revocation or activation cannot be attributed to or
activate the victim tenant. Each tenant's atomic AI config owns its history,
while provider/MSP inheritance, delegated acknowledgement, tenant-default UI,
and multi-actor administration remain explicit residual product work.

The RBAC settings transport consumes the security-owned canonical persistence
boundary rather than constructing a settings-local manager.
`internal/api/access_tenant_provider.go` isolates organization databases and
only the default organization may consume v5 file-based migration input.
Role and assignment panels must receive healthy empty collections as `[]`, but
store initialization, migration, and read failures stay visible as
`rbac_store_unavailable`; the settings layer must not reinterpret corruption
as an organization with no roles or users. Local identities and opaque stable
SSO principals are both valid assignment subjects, including principals with
an intentionally empty role set.

The Pulse Intelligence settings panel that hosts Pulse MCP setup is the
External agents panel since 2026-10-22: `settingsPanelRegistryContext.tsx`
registers `AgentIntegrationsPanel` under the `system-ai-agents` tab at
`/settings/pulse-intelligence/agents`, replacing the retired Assistant
settings page. The old `system-ai-assistant` tab id and
`/settings/pulse-intelligence/assistant` path redirect there, so saved links
keep working. Organization-scoped settings ownership, RBAC gating, and the
settings shell's organization presentation rules are unchanged.

### User Access reader lifetime

The existing User Access directory, role choices, saved-permission read, drafts
and removal confirmations belong to one mounted authenticated reader and
organisation. `useUserAssignmentsPanelState.ts` withdraws them on principal,
users-capability, RBAC-entitlement or organisation changes and disposal, aborts
pending requests and rejects obsolete completions even after leaving and
returning to the same context. The next admitted reader loads a fresh directory.
A failed or malformed directory/role list is unknown, not an empty configuration;
Retry is read-only. Only current inventory rows may open a confirmation, and
only one confirmed assignment/removal write may run. No obsolete completion may
announce success, close a new reader's editor or reload old accounts. Server
authorisation, IdP deprovisioning and existing self-modification protection remain
unchanged. Registered reader-lifetime, paywall-panel and RBAC transport tests
cover these rules; synthetic desktop/narrow browser proof is not installed
administrator or native IdP recovery.

User Access action feedback is owned by that same reader, not the global toast
queue: success notices withdraw on context change or a new confirmation, and
definite rejections stay beside the current save/removal controls without
discarding the draft. Directory failure has one persistent read-only Retry notice. Account
principals must not survive an organisation switch in an old notification.
Paired directory/role reads are aborted when their combined read settles so a
failed sibling does not leave an unowned pending request. The registered
reader-lifetime/paywall tests and actual App browser witness cover this bound.

### Unconfirmed user changes require a saved-access check

A lost response, server failure or unreadable success after updating a user's
roles or removing their access does not prove that the server rolled back.
User Access withdraws its accepted directory, role catalogue, saved permissions,
draft and removal confirmation. It neither replays the mutation nor presents
an empty directory. A local, focused warning says the change may already have
been applied and offers **Check saved access**, an explicit read-only refresh.
Only accepted current user and role inventories clear that warning and enable
further changes. Failed or malformed recovery keeps access unknown; concurrent
checks coalesce. Definite client rejections retain their correction path and
the existing self-removal explanation. Reader withdrawal erases the warning
and invalidates late permission, mutation and recovery responses.

`UserAccessReaderLifetime.test.tsx` and `RBACPaywallPanels.test.tsx` cover
committed-but-unconfirmed updates/removals, recovery failure and serialisation,
stale callbacks, delayed reads, definite rejection and consuming-panel focus.
Paired desktop/narrow browser captures exercise the real panel, state, RBAC API
and client with safe HTTP fixtures. This changes no grant, endpoint, entitlement,
authorization or IdP semantics and is not original saved-account recovery.

### User Access search is not directory absence

Only an accepted current directory may supply User Access result counts. An
identity search trims surrounding whitespace and matches the existing display
name, email, provider and stable principal fields case-insensitively. A populated
directory with no matching rows says **No matching users**, reports the visible
and total counts, and offers **Show all users**. Clearing that filter restores
the same accepted rows and keyboard focus to search without another API read or
write. Only a genuinely empty accepted directory shows the first-login/SSO setup
guidance. Pending, failed, unconfirmed or withdrawn readers show neither counts
nor a search-empty recovery action, and retain their existing recovery path.

`RBACPaywallPanels.test.tsx` exercises identity matching, padded/whitespace-only
queries, result counts, local clear/focus, genuine empty state and reader
withdrawal. Existing `UserAccessReaderLifetime.test.tsx` continues to cover
unknown inventories and action authority. Comparable desktop/narrow browser
captures cover the real panel, grid, API client and current-reader state using
safe HTTP fixtures. Search changes no account, role, grant or request authority;
this is not original saved-account or installed recovery.

### User Access requests retain their confirmed context

RBAC directory/role reads, saved-permission reads and user assignment/removal
operations accept client-only `RBACAdminRequestOptions`. User Access supplies an
abort signal, exact expected organisation and `retry: false` to the canonical
API client, including its CSRF preparation boundary. A withdrawn confirmation
must not dispatch into the default or a new organisation or automatically replay
a refusal. Endpoint paths, encoded principal keys, JSON payloads, authentication
and server permission checks are unchanged. `api/__tests__/rbac.test.ts` and
`UserAccessReaderLifetime.test.tsx` pin forwarding and the consuming lifecycle.


### Reporting workspaces belong to the current reader

The existing Data & Reports panel keys its whole workspace, resource-picker
searches and lazy schedule editor to the current organisation, authenticated
principal, non-secret credential hint/sorted scope set, reporting-read admission
and reporting entitlement/readiness. An equal security-status refresh preserves
the reader's drafts. A changed reader withdraws schedules, recipients, selections,
searches, titles and local feedback; restoring access requires fresh reads and
never revives the old editor.

Every reporting request retains its original `expectedOrgID`, abort signal and
`retry: false`. Disposal and current-reader checks suppress late catalog/list/
mutation results, follow-up requests, feedback and downloads, including a response
whose blob finishes after withdrawal. Aborting a client does not undo a server
write or recall an already downloaded file. Backend permission/scope checks,
report generation, scheduled delivery and retention policy are unchanged; the
client fingerprint is not a new authorisation grant or a write-capability promise.

Schedule actions require an accepted current inventory and current row identity,
are mutually excluded, and recheck admission after a confirmation. Failed or
malformed reads are unknown, not an empty schedule directory; Refresh explicitly
retries them. Refused saves retain the editor for deliberate retry, with feedback
inside this reader rather than global toasts or duplicate raw console output.
Registered `ReportingPanel.reader.test.tsx`, `useReportingPanelState.test.ts`,
`ReportingPanel.test.tsx` and `settingsPanelRegistryContext.test.ts` cover lifetime,
late body completion, same-reader refresh, refused-save retry, malformed inventory,
confirmation, route props and existing reporting/delivery behaviour. Actual App
Settings/API-client/browser proof remains required for the consuming surface.

### Default organization in shared-process multi-organization mode

With `PULSE_MULTI_TENANT_ENABLED=true` on a self-hosted (not hosted) runtime,
the default organization is open only to instance administrators and to
explicit members in its `org.json`, loaded strictly. Other signed-in users no
longer reach it by header, cookie or socket. Instance administrators manage
its members from Organization access: an add takes effect at once, members
are viewers, and membership never grants organization management. The first
start in this mode seeds RBAC-assigned users who belong to no other
organization and records `default-org-access-migration.json`. A signed-in
user with no usable organization sees their pending invitations at bootstrap.
Single-tenant and hosted runtimes keep the open default organization.
`TestOrgHandlers_DefaultOrgMembersInSharedMultiOrgMode` and the
`org_access_policy_test.go` suite cover these rules.

Organization admission applies to every verified principal, including local
HTTP Basic credentials that are verified only after tenant admission. The
configured local administrator reaches a tenant organization by header or
cookie only as a member of that organization, whether it presents a session or
Basic credentials, and minting an organization-bound token, reading tenant
configuration or live state, and configuration transfer all follow that rule.
Instance administration still grants the default organization. Configuration
import and export additionally require management of the tenant, not
membership. `basic_tenant_authorization_test.go` and
`TestConfigTransferVerifiedUsersRequireTenantManagement` cover these rules.
