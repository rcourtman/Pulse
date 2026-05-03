# Self-Hosted Pro Runtime Truth Record

- Date: `2026-04-23`
- Decision: `self-hosted-pro-runtime-truth`
- Scope:
  - `pulse`
  - `pulse-pro`
  - lanes `L3`, `L9`

## Decision

Pulse Pro in v6 does have real paid value, but the honest product story is
narrower than several legacy nouns made it sound.

The canonical v6 Pro story should be:

1. Alert-triggered root-cause analysis.
2. Safe remediation through Patrol auto-fix and higher-autonomy controls.
3. Longer operating history on self-hosted installs.
4. Included team/admin extras: RBAC, audit logging, reporting, and agent
   profiles. SSO is part of the Community tier and no longer a Pro value claim.

The following should not be treated as primary marketed Pro pillars:

1. `kubernetes_ai`
   It is still a compatibility capability and route gate, but not a coherent
   v6 product pillar.
2. `incident memory` as a distinct system capability
   Today this is mostly packaging over longer metrics history, alert history,
   and Patrol run history rather than a separate memory product.
3. `scheduled remediations`
   No first-class shipped v6 surface was found.
4. `execution audit trail`
   The real shipped surfaces are audit logging and Patrol/run history, not a
   distinct marketed feature by that name.

## Advertised vs Delivered Matrix

| Marketed value | Runtime backing | Product truth verdict | Evidence |
|---|---|---|---|
| Alert-triggered root-cause analysis | Alert investigations are wired from live alerts into Pulse Assistant and the Patrol/AI settings surface gates alert-triggered analysis behind `ai_alerts`. | `shipped and productized` | `frontend-modern/src/components/Alerts/InvestigateAlertButton.tsx`; `frontend-modern/src/features/patrol/usePatrolIntelligenceState.ts`; `internal/api/router_routes_ai_relay.go` |
| Safe remediation / Patrol auto-fix | Patrol run, approval, autonomy, and remediation actions are gated behind `ai_autofix`, and the Patrol intelligence surface exposes approval/autonomy controls. | `shipped and productized` | `frontend-modern/src/features/patrol/usePatrolIntelligenceState.ts`; `internal/api/ai_handlers.go`; `internal/api/router_routes_ai_relay.go` |
| 90-day history | Tier history is canonical in licensing, surfaced in runtime capabilities as `max_history_days`, and enforced through locked history-chart overlays. | `shipped and productized` | `pkg/licensing/features.go`; `frontend-modern/src/stores/license.ts`; `frontend-modern/src/components/shared/HistoryChartOverlay.tsx` |
| Incident memory | The real backing is longer metrics retention plus Patrol run history and alert history. No distinct memory engine, incident notebook, or learned remediation memory surfaced in v6. | `shipped but weakly packaged` | `frontend-modern/src/components/shared/HistoryChartOverlay.tsx`; `internal/api/router_routes_ai_relay.go`; `internal/api/alerts.go`; `internal/api/ai_handlers.go` |
| Remote access / mobile / push | Relay remains a dedicated, real settings surface with live configuration, status, and pairing flows, and it is explicitly license-gated. | `shipped and productized` | `frontend-modern/src/components/Settings/RelaySettingsPanel.tsx`; `frontend-modern/src/components/Settings/useRelaySettingsPanelState.ts`; `internal/api/router_routes_ai_relay.go` |
| Agent profiles | Dedicated settings panel with CRUD, assignment, and AI suggestion affordances. The surface is explicitly paywalled behind `agent_profiles`. | `shipped and productized` | `frontend-modern/src/components/Settings/AgentProfilesPanel.tsx`; `frontend-modern/src/components/Settings/useAgentProfilesPanelState.ts`; `internal/api/router_routes_registration.go` |
| SSO providers | Dedicated SSO providers surface. OIDC, SAML, and multi-provider flows are included with Community; `advanced_sso` remains only as an included compatibility capability. | `shipped and productized outside the paid bundle` | `frontend-modern/src/components/Settings/SSOProvidersPanel.tsx`; `frontend-modern/src/components/Settings/useSSOProvidersState.ts`; `internal/api/router_routes_auth_security.go` |
| RBAC | Dedicated roles surface, user/role admin routes, integrity check, and admin reset paths. | `shipped and productized` | `frontend-modern/src/components/Settings/RolesPanel.tsx`; `frontend-modern/src/components/Settings/useRolesPanelState.ts`; `internal/api/router_routes_licensing.go` |
| Audit logging | Dedicated audit log surface with filtering and signature verification, backed by gated audit endpoints. | `shipped and productized` | `frontend-modern/src/components/Settings/AuditLogPanel.tsx`; `frontend-modern/src/components/Settings/useAuditLogPanelState.ts`; `internal/api/router_routes_licensing.go` |
| PDF/CSV reporting | Dedicated reporting panel, reporting catalog, report generation, multi-report generation, and VM inventory export. | `shipped and productized` | `frontend-modern/src/components/Settings/ReportingPanel.tsx`; `frontend-modern/src/components/Settings/useReportingPanelState.ts`; `internal/api/reporting_catalog_handlers.go`; `internal/api/metrics_reporting_handlers.go`; `internal/api/reporting_inventory_handlers.go` |
| Kubernetes AI analysis | Still present as a gated API route and compatibility capability, but no first-class v6 customer-facing product surface was found. | `compatibility-only` | `internal/api/router_routes_ai_relay.go`; `internal/api/ai_handlers.go`; `frontend-modern/src/api/ai.ts` |
| Scheduled remediations | No first-class v6 UI, route family, or shared product surface was found. | `not shipped enough to advertise` | workspace audit on `pulse` and `pulse-pro` found no current v6 surface |
| Execution audit trail | The real product surfaces are audit logging plus Patrol run history. No distinct marketed feature boundary exists under this name. | `not a standalone product feature` | `frontend-modern/src/components/Settings/AuditLogPanel.tsx`; `frontend-modern/src/features/patrol/usePatrolIntelligenceState.ts`; `internal/api/router_routes_licensing.go`; `internal/api/router_routes_ai_relay.go` |

## Evidence Considered

1. `pkg/licensing/features.go` defines the canonical v6 self-hosted Pro
   capability bundle and confirms that Community / Relay / Pro monitored-system
   limits are all uncapped.
2. `pulse-pro/license-server/public_pricing.go` already markets Pro as
   root-cause analysis, safe remediation, 90-day history, and included
   team extras.
3. `frontend-modern/src/components/Alerts/InvestigateAlertButton.tsx` and
   `frontend-modern/src/features/patrol/usePatrolIntelligenceState.ts` show
   that Pro investigation and remediation are surfaced as operator jobs, not
   just hidden backend capabilities.
4. `frontend-modern/src/components/Settings/RelaySettingsPanel.tsx`,
   `AgentProfilesPanel.tsx`, `SSOProvidersPanel.tsx`, `RolesPanel.tsx`,
   `AuditLogPanel.tsx`, and `ReportingPanel.tsx` show that the main admin
   extras are all real product surfaces with explicit Pro gating.
5. `internal/api/router_routes_ai_relay.go`,
   `internal/api/router_routes_auth_security.go`, and
   `internal/api/router_routes_licensing.go` show that those surfaces are also
   backed by real route-level license gates rather than copy-only affordances.
6. The remaining `kubernetes_ai` surface is still real at the API level
   (`/api/ai/kubernetes/analyze`), but the audit did not find a corresponding
   first-class v6 product surface that justifies marketing it as a headline
   Pro feature.
7. The audit did not find a first-class current v6 surface for `scheduled
   remediations`, and it did not find a distinct standalone product feature
   that maps cleanly to the legacy phrase `execution audit trail`.

## Outcome

- Pulse Pro should continue to be sold on operator value, not monitoring volume.
- The honest v6 Pro package is:
  - root-cause analysis
  - safe remediation
  - 90-day operating history
  - team/admin extras
- `incident memory` should be treated as packaging shorthand, not as a distinct
  technical feature, until Pulse ships a clearer incident-memory product.
- `kubernetes_ai` should remain compatibility-only in customer-facing v6
  packaging unless it is elevated into a broader, coherent operator workflow.
- The next product decision is not another copy pass. It is whether to:
  1. deepen the current Pro pillars until they feel stronger, especially around
     incident memory, or
  2. add another real Pro capability worth paying for.
