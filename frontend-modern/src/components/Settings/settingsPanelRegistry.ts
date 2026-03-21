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
import type { InfrastructureWorkspace as InfrastructureWorkspaceType } from './InfrastructureWorkspace';
import type { SettingsTab } from './settingsNavigationModel';
import { SETTINGS_PANEL_REGISTRY_LOADERS } from './settingsPanelRegistryLoaders';

export interface SettingsPanelRegistryEntry {
  component: Component<any>;
  getProps?: () => object;
}

export type SettingsDispatchableTab = Exclude<SettingsTab, 'proxmox'>;

export type SettingsPanelRegistry = Record<SettingsDispatchableTab, SettingsPanelRegistryEntry>;

export interface SettingsPanelRegistryContext {
  getInfrastructurePanelProps: () => Parameters<typeof InfrastructureWorkspaceType>[0];
  systemGeneralPanel: Component;
  systemAiPanel: Component;
  systemBillingPanel: Component;
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
  'infrastructure-operations': {
    component: SETTINGS_PANEL_REGISTRY_LOADERS.InfrastructureWorkspace,
    getProps: context.getInfrastructurePanelProps,
  },
  'system-general': {
    component: context.systemGeneralPanel,
  },
  'system-network': {
    component: SETTINGS_PANEL_REGISTRY_LOADERS.NetworkSettingsPanel,
    getProps: context.getNetworkPanelProps,
  },
  'system-updates': {
    component: SETTINGS_PANEL_REGISTRY_LOADERS.UpdatesSettingsPanel,
    getProps: context.getUpdatesPanelProps,
  },
  'system-recovery': {
    component: SETTINGS_PANEL_REGISTRY_LOADERS.RecoverySettingsPanel,
    getProps: context.getRecoveryPanelProps,
  },
  'system-ai': {
    component: context.systemAiPanel,
  },
  'system-relay': {
    component: SETTINGS_PANEL_REGISTRY_LOADERS.RelaySettingsPanel,
    getProps: context.getRelayPanelProps,
  },
  'system-billing': {
    component: context.systemBillingPanel,
  },
  'organization-overview': {
    component: SETTINGS_PANEL_REGISTRY_LOADERS.OrganizationOverviewPanel,
    getProps: context.getOrganizationOverviewPanelProps,
  },
  'organization-access': {
    component: SETTINGS_PANEL_REGISTRY_LOADERS.OrganizationAccessPanel,
    getProps: context.getOrganizationAccessPanelProps,
  },
  'organization-sharing': {
    component: SETTINGS_PANEL_REGISTRY_LOADERS.OrganizationSharingPanel,
    getProps: context.getOrganizationSharingPanelProps,
  },
  'organization-billing': {
    component: SETTINGS_PANEL_REGISTRY_LOADERS.OrganizationBillingPanel,
    getProps: context.getOrganizationBillingPanelProps,
  },
  'organization-billing-admin': {
    component: SETTINGS_PANEL_REGISTRY_LOADERS.BillingAdminPanel,
  },
  api: {
    component: SETTINGS_PANEL_REGISTRY_LOADERS.APIAccessPanel,
    getProps: context.getApiAccessPanelProps,
  },
  'security-overview': {
    component: SETTINGS_PANEL_REGISTRY_LOADERS.SecurityOverviewPanel,
    getProps: context.getSecurityOverviewPanelProps,
  },
  'security-auth': {
    component: SETTINGS_PANEL_REGISTRY_LOADERS.SecurityAuthPanel,
    getProps: context.getSecurityAuthPanelProps,
  },
  'security-sso': {
    component: context.securitySsoPanel,
  },
  'security-roles': {
    component: SETTINGS_PANEL_REGISTRY_LOADERS.RolesPanel,
  },
  'security-users': {
    component: SETTINGS_PANEL_REGISTRY_LOADERS.UserAssignmentsPanel,
  },
  'security-audit': {
    component: SETTINGS_PANEL_REGISTRY_LOADERS.AuditLogPanel,
  },
  'security-webhooks': {
    component: SETTINGS_PANEL_REGISTRY_LOADERS.AuditWebhookPanel,
    getProps: context.getAuditWebhookPanelProps,
  },
});
