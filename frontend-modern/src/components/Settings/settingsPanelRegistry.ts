import { lazy } from 'solid-js';
import type { Component } from 'solid-js';
import type { NetworkSettingsPanel as NetworkSettingsPanelType } from './NetworkSettingsPanel';
import type { UpdatesSettingsPanel as UpdatesSettingsPanelType } from './UpdatesSettingsPanel';
import type { RecoverySettingsPanel as RecoverySettingsPanelType } from './RecoverySettingsPanel';
import type OrganizationOverviewPanelType from './OrganizationOverviewPanel';
import type OrganizationAccessPanelType from './OrganizationAccessPanel';
import type OrganizationSharingPanelType from './OrganizationSharingPanel';
import type OrganizationBillingPanelType from './OrganizationBillingPanel';
import type { APIAccessPanel as APIAccessPanelType } from './APIAccessPanel';
import type { SecurityOverviewPanel as SecurityOverviewPanelType } from './SecurityOverviewPanel';
import type { SecurityAuthPanel as SecurityAuthPanelType } from './SecurityAuthPanel';
import type { RelaySettingsPanel as RelaySettingsPanelType } from './RelaySettingsPanel';
import type { AuditWebhookPanel as AuditWebhookPanelType } from './AuditWebhookPanel';
import type { SettingsTab } from './settingsTypes';

const NetworkSettingsPanel = lazy(() =>
  import('./NetworkSettingsPanel').then((m) => ({ default: m.NetworkSettingsPanel })),
);
const UpdatesSettingsPanel = lazy(() =>
  import('./UpdatesSettingsPanel').then((m) => ({ default: m.UpdatesSettingsPanel })),
);
const RecoverySettingsPanel = lazy(() =>
  import('./RecoverySettingsPanel').then((m) => ({ default: m.RecoverySettingsPanel })),
);
const RelaySettingsPanel = lazy(() =>
  import('./RelaySettingsPanel').then((m) => ({ default: m.RelaySettingsPanel })),
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

export interface SettingsPanelRegistryEntry {
  component: Component<any>;
  getProps?: () => object;
}

export type SettingsDispatchableTab = Exclude<SettingsTab, 'proxmox'>;

export type SettingsPanelRegistry = Record<SettingsDispatchableTab, SettingsPanelRegistryEntry>;

export interface SettingsPanelRegistryContext {
  agentsPanel: Component;
  systemGeneralPanel: Component;
  systemAiPanel: Component;
  systemProPanel: Component;
  securitySsoPanel: Component;
  getNetworkPanelProps: () => Parameters<typeof NetworkSettingsPanelType>[0];
  getUpdatesPanelProps: () => Parameters<typeof UpdatesSettingsPanelType>[0];
  getRecoveryPanelProps: () => Parameters<typeof RecoverySettingsPanelType>[0];
  getOrganizationOverviewPanelProps: () => Parameters<typeof OrganizationOverviewPanelType>[0];
  getOrganizationAccessPanelProps: () => Parameters<typeof OrganizationAccessPanelType>[0];
  getOrganizationSharingPanelProps: () => Parameters<typeof OrganizationSharingPanelType>[0];
  getOrganizationBillingPanelProps: () => Parameters<typeof OrganizationBillingPanelType>[0];
  getApiAccessPanelProps: () => Parameters<typeof APIAccessPanelType>[0];
  getSecurityOverviewPanelProps: () => Parameters<typeof SecurityOverviewPanelType>[0];
  getSecurityAuthPanelProps: () => Parameters<typeof SecurityAuthPanelType>[0];
  getRelayPanelProps: () => Parameters<typeof RelaySettingsPanelType>[0];
  getAuditWebhookPanelProps: () => Parameters<typeof AuditWebhookPanelType>[0];
}

export const createSettingsPanelRegistry = (
  context: SettingsPanelRegistryContext,
): SettingsPanelRegistry => ({
  agents: {
    component: context.agentsPanel,
  },
  'system-general': {
    component: context.systemGeneralPanel,
  },
  'system-network': {
    component: NetworkSettingsPanel,
    getProps: context.getNetworkPanelProps,
  },
  'system-updates': {
    component: UpdatesSettingsPanel,
    getProps: context.getUpdatesPanelProps,
  },
  'system-recovery': {
    component: RecoverySettingsPanel,
    getProps: context.getRecoveryPanelProps,
  },
  'system-ai': {
    component: context.systemAiPanel,
  },
  'system-relay': {
    component: RelaySettingsPanel,
    getProps: context.getRelayPanelProps,
  },
  'system-pro': {
    component: context.systemProPanel,
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
    getProps: context.getAuditWebhookPanelProps,
  },
});
