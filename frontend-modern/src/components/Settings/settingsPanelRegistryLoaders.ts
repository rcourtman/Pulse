import { lazy } from 'solid-js';

export const SETTINGS_PANEL_REGISTRY_LOADERS = {
  APIAccessPanel: lazy(() =>
    import('./APIAccessPanel').then((m) => ({ default: m.APIAccessPanel })),
  ),
  AuditLogPanel: lazy(() => import('./AuditLogPanel')),
  AuditWebhookPanel: lazy(() =>
    import('./AuditWebhookPanel').then((m) => ({ default: m.AuditWebhookPanel })),
  ),
  BillingAdminPanel: lazy(() => import('./BillingAdminPanel')),
  InfrastructureWorkspace: lazy(() =>
    import('./InfrastructureWorkspace').then((m) => ({ default: m.InfrastructureWorkspace })),
  ),
  NetworkSettingsPanel: lazy(() =>
    import('./NetworkSettingsPanel').then((m) => ({ default: m.NetworkSettingsPanel })),
  ),
  OrganizationAccessPanel: lazy(() => import('./OrganizationAccessPanel')),
  OrganizationBillingPanel: lazy(() => import('./OrganizationBillingPanel')),
  OrganizationOverviewPanel: lazy(() => import('./OrganizationOverviewPanel')),
  OrganizationSharingPanel: lazy(() => import('./OrganizationSharingPanel')),
  RecoverySettingsPanel: lazy(() =>
    import('./RecoverySettingsPanel').then((m) => ({ default: m.RecoverySettingsPanel })),
  ),
  RelaySettingsPanel: lazy(() =>
    import('./RelaySettingsPanel').then((m) => ({ default: m.RelaySettingsPanel })),
  ),
  RolesPanel: lazy(() => import('./RolesPanel')),
  SecurityAuthPanel: lazy(() =>
    import('./SecurityAuthPanel').then((m) => ({ default: m.SecurityAuthPanel })),
  ),
  SecurityOverviewPanel: lazy(() =>
    import('./SecurityOverviewPanel').then((m) => ({ default: m.SecurityOverviewPanel })),
  ),
  UpdatesSettingsPanel: lazy(() =>
    import('./UpdatesSettingsPanel').then((m) => ({ default: m.UpdatesSettingsPanel })),
  ),
  UserAssignmentsPanel: lazy(() => import('./UserAssignmentsPanel')),
} as const;
