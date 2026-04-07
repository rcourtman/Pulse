# Organization Settings Contract

## Contract Metadata

```json
{
  "subsystem_id": "organization-settings",
  "lane": "L14",
  "contract_file": "docs/release-control/v6/internal/subsystems/organization-settings.md",
  "status_file": "docs/release-control/v6/internal/status.json",
  "registry_file": "docs/release-control/v6/internal/subsystems/registry.json",
  "dependency_subsystem_ids": []
}
```

## Purpose

Own organization role/share semantics and the canonical settings surfaces that
let users review organization metadata, manage membership, assign roles, and
create cross-organization shares.

## Canonical Files

1. `frontend-modern/src/api/orgs.ts`
2. `frontend-modern/src/api/rbac.ts`
3. `frontend-modern/src/components/Settings/OrganizationAccessLoadingState.tsx`
4. `frontend-modern/src/components/Settings/OrganizationAccessManagementSection.tsx`
5. `frontend-modern/src/components/Settings/OrganizationAccessMembersSection.tsx`
6. `frontend-modern/src/components/Settings/OrganizationAccessPanel.tsx`
7. `frontend-modern/src/components/Settings/OrganizationIncomingSharesSection.tsx`
8. `frontend-modern/src/components/Settings/OrganizationOutgoingSharesSection.tsx`
9. `frontend-modern/src/components/Settings/OrganizationOverviewDetailsSection.tsx`
10. `frontend-modern/src/components/Settings/OrganizationOverviewLoadingState.tsx`
11. `frontend-modern/src/components/Settings/OrganizationOverviewMembersSection.tsx`
12. `frontend-modern/src/components/Settings/OrganizationOverviewPanel.tsx`
13. `frontend-modern/src/components/Settings/OrganizationSharingCreateSection.tsx`
14. `frontend-modern/src/components/Settings/OrganizationSharingLoadingState.tsx`
15. `frontend-modern/src/components/Settings/OrganizationSharingPanel.tsx`
16. `frontend-modern/src/components/Settings/RBACFeatureGateSection.tsx`
17. `frontend-modern/src/components/Settings/RolesEditorDialog.tsx`
18. `frontend-modern/src/components/Settings/RolesPanel.tsx`
19. `frontend-modern/src/components/Settings/useOrganizationAccessPanelState.ts`
20. `frontend-modern/src/components/Settings/useOrganizationOverviewPanelState.ts`
21. `frontend-modern/src/components/Settings/useOrganizationSharingPanelState.ts`
22. `frontend-modern/src/components/Settings/UserAssignmentsDialog.tsx`
23. `frontend-modern/src/components/Settings/UserAssignmentsPanel.tsx`
24. `frontend-modern/src/components/Settings/useRBACFeatureGateState.ts`
25. `frontend-modern/src/components/Settings/useRolesPanelState.ts`
26. `frontend-modern/src/components/Settings/useUserAssignmentsPanelState.ts`
27. `frontend-modern/src/utils/organizationRolePresentation.ts`
28. `frontend-modern/src/utils/organizationSettingsPresentation.ts`
29. `frontend-modern/src/utils/orgUtils.ts`
30. `internal/api/access_control_handlers.go`
31. `internal/api/enterprise_extension_rbac_admin.go`
32. `internal/api/org_handlers.go`
33. `internal/api/org_lifecycle_handlers.go`
34. `internal/models/organization.go`

## Shared Boundaries

1. `frontend-modern/src/api/orgs.ts` shared with `api-contracts`: the organization frontend client is both an organization settings control surface and a canonical API payload contract boundary.
2. `frontend-modern/src/api/rbac.ts` shared with `api-contracts`: the RBAC frontend client is both an organization settings control surface and a canonical API payload contract boundary.
3. `internal/api/access_control_handlers.go` shared with `api-contracts`: RBAC role and user-assignment handlers are both an organization settings control surface and a canonical API payload contract boundary.
4. `internal/api/enterprise_extension_rbac_admin.go` shared with `api-contracts`: RBAC admin extension endpoints are both an organization settings control surface and a canonical API payload contract boundary.
5. `internal/api/org_handlers.go` shared with `api-contracts`: organization management handlers are both an organization settings control surface and a canonical API payload contract boundary.
6. `internal/api/org_lifecycle_handlers.go` shared with `api-contracts`: organization lifecycle handlers are both an organization settings control surface and a canonical API payload contract boundary.

## Extension Points

1. Add or change organization role and share semantics through `internal/models/organization.go`
2. Add or change organization access, overview, sharing, RBAC feature-gating, role-management, or user-assignment presentation through `frontend-modern/src/components/Settings/OrganizationAccessPanel.tsx`, `frontend-modern/src/components/Settings/OrganizationAccessLoadingState.tsx`, `frontend-modern/src/components/Settings/OrganizationAccessManagementSection.tsx`, `frontend-modern/src/components/Settings/OrganizationAccessMembersSection.tsx`, `frontend-modern/src/components/Settings/OrganizationOverviewPanel.tsx`, `frontend-modern/src/components/Settings/OrganizationOverviewLoadingState.tsx`, `frontend-modern/src/components/Settings/OrganizationOverviewDetailsSection.tsx`, `frontend-modern/src/components/Settings/OrganizationOverviewMembersSection.tsx`, `frontend-modern/src/components/Settings/OrganizationSharingPanel.tsx`, `frontend-modern/src/components/Settings/OrganizationSharingCreateSection.tsx`, `frontend-modern/src/components/Settings/OrganizationSharingLoadingState.tsx`, `frontend-modern/src/components/Settings/OrganizationOutgoingSharesSection.tsx`, `frontend-modern/src/components/Settings/OrganizationIncomingSharesSection.tsx`, `frontend-modern/src/components/Settings/useOrganizationAccessPanelState.ts`, `frontend-modern/src/components/Settings/useOrganizationOverviewPanelState.ts`, `frontend-modern/src/components/Settings/useOrganizationSharingPanelState.ts`, `frontend-modern/src/components/Settings/RBACFeatureGateSection.tsx`, `frontend-modern/src/components/Settings/RolesPanel.tsx`, `frontend-modern/src/components/Settings/RolesEditorDialog.tsx`, `frontend-modern/src/components/Settings/useRBACFeatureGateState.ts`, `frontend-modern/src/components/Settings/useRolesPanelState.ts`, `frontend-modern/src/components/Settings/UserAssignmentsPanel.tsx`, `frontend-modern/src/components/Settings/UserAssignmentsDialog.tsx`, and `frontend-modern/src/components/Settings/useUserAssignmentsPanelState.ts`
3. Route organization and RBAC frontend transport changes through `frontend-modern/src/api/orgs.ts` and `frontend-modern/src/api/rbac.ts`
4. Keep backend organization management and lifecycle handlers aligned through `internal/api/org_handlers.go` and `internal/api/org_lifecycle_handlers.go`
5. Keep RBAC role, assignment, and admin recovery transport aligned through `internal/api/access_control_handlers.go` and `internal/api/enterprise_extension_rbac_admin.go`

## Forbidden Paths

1. Duplicating organization role normalization or badge styling outside the canonical organization presentation helpers
2. Reintroducing organization settings copy, validation, or empty-state strings directly inside feature panels instead of the shared organization presentation helpers
3. Letting share-role, RBAC role-assignment, or membership semantics drift between `internal/models/organization.go` and the governed organization settings surfaces

## Completion Obligations

1. Update the organization model, settings surfaces, and proof files together when role/share semantics move
2. Keep organization settings copy and validation inside the canonical organization presentation helpers
3. Keep the shared organization and RBAC transport proof routes explicit in `registry.json`; default fallback proof routing is not allowed for this subsystem
4. Update this contract whenever a new organization settings, role-management, or organization-domain helper entry point becomes canonical runtime surface area

## Current State

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
Incoming share visibility is part of that same boundary as well: a recipient
must only see inbound shares whose requested `accessRole` is satisfied by the
user's effective membership role in the target organization, using the shared
role comparator instead of handler-local owner/admin shortcuts.
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
That same settings surface now also inherits the runtime-versus-commercial
licensing split. Organization settings may consume runtime capability truth
from the shared runtime-capabilities contract, but billing identity and
upgrade posture stay outside this subsystem on the dedicated commercial
surfaces, and any public-demo suppression must come from the shared resolved
`presentationPolicy` rather than settings-local demo checks or billing
entitlement probes.
The organization sharing surface now also sources resource quick-pick labels
from the shared preferred resource display helper, so governed resources do
not fall back to raw names inside share creation.
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
The RBAC settings area now follows the same extracted-owner pattern as the
other modernized settings surfaces: `RBACFeatureGateSection.tsx` owns the
shared paywall CTA rendering, `useRBACFeatureGateState.ts` owns the shared
license and free-trial runtime, `useRolesPanelState.ts` plus
`RolesEditorDialog.tsx` own the roles runtime split, and
`useUserAssignmentsPanelState.ts` plus `UserAssignmentsDialog.tsx` own the
user-assignment runtime split. `RolesPanel.tsx` and `UserAssignmentsPanel.tsx`
remain the canonical render shells for those governed RBAC surfaces.
That RBAC gate now also depends on the shared commercial navigation contract:
`RBACFeatureGateSection.tsx` may request the canonical `rbac` destination from
the shared license boundary, but it must render that destination through the
`frontend-primitives` typed upgrade link owner instead of assuming
organization-settings paywalls always leave the app in a new tab.
That shared RBAC free-trial runtime must also preserve backend denial reasons
through the canonical upgrade presentation helper instead of collapsing every
trial-start conflict into a generic already-used message. Organization settings
paywalls should only map the explicit canonical trial helper outputs, not
re-interpret status codes locally.
The RBAC feature-gate state now also depends on the shared
`frontend-modern/src/utils/trialStartAction.ts` owner for hosted handoff and
success/error orchestration. Organization settings paywalls must not keep a
lane-local `startProTrial()` branch once that shared helper covers the same
runtime contract.
That same RBAC paywall surface now also depends on the runtime-versus-
commercial license split: RBAC enablement must stay on the runtime capability
store, while free-trial eligibility and upgrade routing stay on the canonical
commercial-posture store backed by `/api/license/commercial-posture`.
Organization settings must not collapse those two concerns back into one
payload just because the same paywall shell renders both.
That same posture split now also fixes RBAC bootstrap ownership.
`frontend-modern/src/components/Settings/useRBACFeatureGateState.ts` may
consume the resolved commercial-posture store for trial and upgrade copy, but
it must not issue its own mount-time `loadCommercialPosture()` read.
Authenticated-shell bootstrap belongs to
`frontend-modern/src/useAppRuntimeState.ts`, with only the governed first-run
setup completion surface allowed to bootstrap posture outside that shell.
