import { lazy } from 'solid-js';
import type { Component } from 'solid-js';
import type { GeneralSettingsPanel as GeneralSettingsPanelType } from './GeneralSettingsPanel';
import type { NetworkSettingsPanel as NetworkSettingsPanelType } from './NetworkSettingsPanel';
import type { UpdatesSettingsPanel as UpdatesSettingsPanelType } from './UpdatesSettingsPanel';
import type { BackupsSettingsPanel as BackupsSettingsPanelType } from './BackupsSettingsPanel';
import type OrganizationOverviewPanelType from './OrganizationOverviewPanel';
import type OrganizationAccessPanelType from './OrganizationAccessPanel';
import type OrganizationSharingPanelType from './OrganizationSharingPanel';
import type OrganizationBillingPanelType from './OrganizationBillingPanel';
import type { APIAccessPanel as APIAccessPanelType } from './APIAccessPanel';
import type { SecurityOverviewPanel as SecurityOverviewPanelType } from './SecurityOverviewPanel';
import type { SecurityAuthPanel as SecurityAuthPanelType } from './SecurityAuthPanel';
import type { SettingsTab } from './settingsTypes';

const SystemLogsPanel = lazy(() =>
  import('./SystemLogsPanel').then((m) => ({ default: m.SystemLogsPanel })),
);
const GeneralSettingsPanel = lazy(() =>
  import('./GeneralSettingsPanel').then((m) => ({ default: m.GeneralSettingsPanel })),
);
const NetworkSettingsPanel = lazy(() =>
  import('./NetworkSettingsPanel').then((m) => ({ default: m.NetworkSettingsPanel })),
);
const UpdatesSettingsPanel = lazy(() =>
  import('./UpdatesSettingsPanel').then((m) => ({ default: m.UpdatesSettingsPanel })),
);
const BackupsSettingsPanel = lazy(() =>
  import('./BackupsSettingsPanel').then((m) => ({ default: m.BackupsSettingsPanel })),
);
const RelaySettingsPanel = lazy(() =>
  import('./RelaySettingsPanel').then((m) => ({ default: m.RelaySettingsPanel })),
);
const ProLicensePanel = lazy(() =>
  import('./ProLicensePanel').then((m) => ({ default: m.ProLicensePanel })),
);
const OrganizationOverviewPanel = lazy(() => import('./OrganizationOverviewPanel'));
const OrganizationAccessPanel = lazy(() => import('./OrganizationAccessPanel'));
const OrganizationSharingPanel = lazy(() => import('./OrganizationSharingPanel'));
const OrganizationBillingPanel = lazy(() => import('./OrganizationBillingPanel'));
const BillingAdminPanel = lazy(() => import('./BillingAdminPanel'));
const APIAccessPanel = lazy(() =>
  import('./APIAccessPanel').then((m) => ({ default: m.APIAccessPanel })),
);
const SecurityOverviewPanel = lazy(() =>
  import('./SecurityOverviewPanel').then((m) => ({ default: m.SecurityOverviewPanel })),
);
const SecurityAuthPanel = lazy(() =>
  import('./SecurityAuthPanel').then((m) => ({ default: m.SecurityAuthPanel })),
);
const RolesPanel = lazy(() => import('./RolesPanel'));
const UserAssignmentsPanel = lazy(() => import('./UserAssignmentsPanel'));
const AuditLogPanel = lazy(() => import('./AuditLogPanel'));
const AuditWebhookPanel = lazy(() =>
  import('./AuditWebhookPanel').then((m) => ({ default: m.AuditWebhookPanel })),
);
const DiagnosticsPanel = lazy(() =>
  import('./DiagnosticsPanel').then((m) => ({ default: m.DiagnosticsPanel })),
);
const ReportingPanel = lazy(() =>
  import('./ReportingPanel').then((m) => ({ default: m.ReportingPanel })),
);

export interface SettingsPanelRegistryEntry {
  component: Component<any>;
  getProps?: () => object;
}

export type SettingsDispatchableTab = Exclude<SettingsTab, 'proxmox'>;

export type SettingsPanelRegistry = Record<SettingsDispatchableTab, SettingsPanelRegistryEntry>;

export interface SettingsPanelRegistryContext {
  agentsPanel: Component;
  dockerPanel: Component;
  systemAiPanel: Component;
  securitySsoPanel: Component;
  getGeneralPanelProps: () => Parameters<typeof GeneralSettingsPanelType>[0];
  getNetworkPanelProps: () => Parameters<typeof NetworkSettingsPanelType>[0];
  getUpdatesPanelProps: () => Parameters<typeof UpdatesSettingsPanelType>[0];
  getBackupsPanelProps: () => Parameters<typeof BackupsSettingsPanelType>[0];
  getOrganizationOverviewPanelProps: () => Parameters<typeof OrganizationOverviewPanelType>[0];
  getOrganizationAccessPanelProps: () => Parameters<typeof OrganizationAccessPanelType>[0];
  getOrganizationSharingPanelProps: () => Parameters<typeof OrganizationSharingPanelType>[0];
  getOrganizationBillingPanelProps: () => Parameters<typeof OrganizationBillingPanelType>[0];
  getApiAccessPanelProps: () => Parameters<typeof APIAccessPanelType>[0];
  getSecurityOverviewPanelProps: () => Parameters<typeof SecurityOverviewPanelType>[0];
  getSecurityAuthPanelProps: () => Parameters<typeof SecurityAuthPanelType>[0];
}

export const createSettingsPanelRegistry = (
  context: SettingsPanelRegistryContext,
): SettingsPanelRegistry => ({
  agents: {
    component: context.agentsPanel,
  },
  docker: {
    component: context.dockerPanel,
  },
  'system-logs': {
    component: SystemLogsPanel,
  },
  'system-general': {
    component: GeneralSettingsPanel,
    getProps: context.getGeneralPanelProps,
  },
  'system-network': {
    component: NetworkSettingsPanel,
    getProps: context.getNetworkPanelProps,
  },
  'system-updates': {
    component: UpdatesSettingsPanel,
    getProps: context.getUpdatesPanelProps,
  },
  'system-backups': {
    component: BackupsSettingsPanel,
    getProps: context.getBackupsPanelProps,
  },
  'system-ai': {
    component: context.systemAiPanel,
  },
  'system-relay': {
    component: RelaySettingsPanel,
  },
  'system-pro': {
    component: ProLicensePanel,
  },
  'organization-overview': {
    component: OrganizationOverviewPanel,
    getProps: context.getOrganizationOverviewPanelProps,
  },
  'organization-access': {
    component: OrganizationAccessPanel,
    getProps: context.getOrganizationAccessPanelProps,
  },
  'organization-sharing': {
    component: OrganizationSharingPanel,
    getProps: context.getOrganizationSharingPanelProps,
  },
  'organization-billing': {
    component: OrganizationBillingPanel,
    getProps: context.getOrganizationBillingPanelProps,
  },
  'organization-billing-admin': {
    component: BillingAdminPanel,
  },
  api: {
    component: APIAccessPanel,
    getProps: context.getApiAccessPanelProps,
  },
  'security-overview': {
    component: SecurityOverviewPanel,
    getProps: context.getSecurityOverviewPanelProps,
  },
  'security-auth': {
    component: SecurityAuthPanel,
    getProps: context.getSecurityAuthPanelProps,
  },
  'security-sso': {
    component: context.securitySsoPanel,
  },
  'security-roles': {
    component: RolesPanel,
  },
  'security-users': {
    component: UserAssignmentsPanel,
  },
  'security-audit': {
    component: AuditLogPanel,
  },
  'security-webhooks': {
    component: AuditWebhookPanel,
  },
  diagnostics: {
    component: DiagnosticsPanel,
  },
  reporting: {
    component: ReportingPanel,
  },
});
