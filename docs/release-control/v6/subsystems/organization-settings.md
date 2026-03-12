# Organization Settings Contract

## Contract Metadata

```json
{
  "subsystem_id": "organization-settings",
  "lane": "L6",
  "contract_file": "docs/release-control/v6/subsystems/organization-settings.md",
  "status_file": "docs/release-control/v6/status.json",
  "registry_file": "docs/release-control/v6/subsystems/registry.json",
  "dependency_subsystem_ids": [
    "api-contracts"
  ]
}
```

## Purpose

Own organization role/share semantics and the canonical settings surfaces that
let users review organization metadata, manage membership, and create
cross-organization shares.

## Canonical Files

1. `internal/models/organization.go`
2. `frontend-modern/src/components/Settings/OrganizationAccessPanel.tsx`
3. `frontend-modern/src/components/Settings/OrganizationOverviewPanel.tsx`
4. `frontend-modern/src/components/Settings/OrganizationSharingPanel.tsx`
5. `frontend-modern/src/utils/orgUtils.ts`
6. `frontend-modern/src/utils/organizationRolePresentation.ts`
7. `frontend-modern/src/utils/organizationSettingsPresentation.ts`

## Shared Boundaries

1. None.

## Extension Points

1. Add or change organization role and share semantics through `internal/models/organization.go`
2. Add or change organization access, overview, or sharing presentation through `frontend-modern/src/components/Settings/OrganizationAccessPanel.tsx`, `frontend-modern/src/components/Settings/OrganizationOverviewPanel.tsx`, and `frontend-modern/src/components/Settings/OrganizationSharingPanel.tsx`
3. Route organization transport changes through `frontend-modern/src/api/orgs.ts`
4. Keep backend organization handler changes aligned through `internal/api/org_handlers.go` and `internal/api/org_lifecycle_handlers.go`

## Forbidden Paths

1. Duplicating organization role normalization or badge styling outside the canonical organization presentation helpers
2. Reintroducing organization settings copy, validation, or empty-state strings directly inside feature panels instead of the shared organization presentation helpers
3. Letting share-role or membership semantics drift between `internal/models/organization.go` and the governed organization settings surfaces

## Completion Obligations

1. Update the organization model, settings surfaces, and proof files together when role/share semantics move
2. Keep organization settings copy and validation inside the canonical organization presentation helpers
3. Update this contract whenever a new organization settings entry point or organization-domain helper becomes canonical runtime surface area

## Current State

Organization overview, access, and sharing had been sitting outside the
governed subsystem map even though they define real runtime expectations around
membership management and cross-organization resource sharing. This contract
now makes that boundary explicit while leaving transport payload ownership in
`api-contracts`.
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
