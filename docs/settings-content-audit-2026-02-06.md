# Settings Content Audit (Page-by-Page)

Date: 2026-02-06  
Focus: Cohesion of language, intent, and visual style across all settings pages in the unified resource model.

## Method
- Reviewed every active settings tab and its primary panel content.
- Marked each page as `Keep`, `Adjust`, `Merge Candidate`, or `Deprecate Candidate`.
- Verified technical health with `frontend-modern` type-check and build.
- Notes on "working" are static/runtime-confidence checks, not full manual UI QA for every path.

## Decisions

| Page | Decision | Why |
|---|---|---|
| Infrastructure (`proxmox` in `Settings.tsx`) | Keep | Core connection management for PVE/PBS/PMG is still required and functional. |
| Workloads (`UnifiedAgents`) | Keep + Adjusted | Required for unified agent lifecycle; copy updated to align with resource model and intent. |
| Docker (inline in `Settings.tsx`) | Keep | Valid operational setting; still narrow in scope but relevant. |
| API Access (`APIAccessPanel`) | Keep + Adjusted | Required; header copy tightened for consistency and clarity. |
| Diagnostics (`DiagnosticsPanel`) | Keep | Operationally important; content scope is still valid. |
| Reporting (`ReportingPanel`) | Keep + Adjusted | Required and aligned to unified resources; restyled to match settings design language. |
| General (`GeneralSettingsPanel`) | Keep + Adjusted | Required; description now reflects actual scope (appearance + cadence). |
| Network (`NetworkSettingsPanel`) | Keep + Adjusted | Required; description now reflects discovery + network controls comprehensively. |
| Updates (`UpdatesSettingsPanel`) | Keep + Adjusted | Required; language tightened and consistent with other panels. |
| Backups (`BackupsSettingsPanel`) | Keep + Adjusted | Required; description now reflects polling + export/import responsibilities. |
| AI (`AISettings`) | Keep + Adjusted | Required; minor copy consistency fix. |
| Remote Access (`RelaySettingsPanel`) | Keep + Adjusted | Required; dual panel descriptions unified. |
| Pulse Pro (`ProLicensePanel`) | Keep | Required for feature visibility and activation flow. |
| System Logs (`SystemLogsPanel`) | Keep + Adjusted | Required; wording standardized with panel behavior. |
| Security Overview (`SecurityOverviewPanel`) | Keep | Required; posture summary remains clear and valuable. |
| Authentication (`SecurityAuthPanel`) | Keep + Adjusted | Required; wording standardized. |
| Single Sign-On (`SSOProvidersPanel` + legacy `OIDCPanel`) | Keep + Merge Candidate | SSO providers are primary; legacy OIDC panel remains useful for backward compatibility but should be retirement-tracked. |
| Roles (`RolesPanel`) | Keep | Required for RBAC. |
| User Access (`UserAssignmentsPanel`) | Keep | Required for RBAC assignments. |
| Audit Log (`AuditLogPanel`) | Keep | Required for compliance/security workflows. |
| Audit Webhooks (`AuditWebhookPanel`) | Keep + Adjusted | Required and Pro-gated; content/style was inconsistent and has now been normalized. |

## Changes Applied in This Pass

1. Standardized content style and copy in major outliers:
   - `frontend-modern/src/components/Settings/ReportingPanel.tsx`
   - `frontend-modern/src/components/Settings/AuditWebhookPanel.tsx`

2. Tightened page descriptions for cohesion:
   - `frontend-modern/src/components/Settings/APIAccessPanel.tsx`
   - `frontend-modern/src/components/Settings/GeneralSettingsPanel.tsx`
   - `frontend-modern/src/components/Settings/NetworkSettingsPanel.tsx`
   - `frontend-modern/src/components/Settings/UpdatesSettingsPanel.tsx`
   - `frontend-modern/src/components/Settings/BackupsSettingsPanel.tsx`
   - `frontend-modern/src/components/Settings/RelaySettingsPanel.tsx`
   - `frontend-modern/src/components/Settings/SecurityAuthPanel.tsx`
   - `frontend-modern/src/components/Settings/SystemLogsPanel.tsx`
   - `frontend-modern/src/components/Settings/UnifiedAgents.tsx`
   - `frontend-modern/src/components/Settings/AISettings.tsx`

3. Added explicit Pro gating fallback in `AuditWebhookPanel` for direct-link resilience.

## Follow-Up Work (Recommended)

1. Split `Settings.tsx` infrastructure blocks into reusable section components (PVE/PBS/PMG).
2. Track and eventually remove legacy `OIDCPanel` once migration parity is guaranteed.
3. Add route smoke tests for every settings tab path and deep link.
4. Add snapshot/content tests for panel title/description cohesion to prevent drift.
