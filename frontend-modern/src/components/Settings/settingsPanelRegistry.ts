import type { Component } from 'solid-js';
import { SystemLogsPanel } from './SystemLogsPanel';
import { GeneralSettingsPanel } from './GeneralSettingsPanel';
import { NetworkSettingsPanel } from './NetworkSettingsPanel';
import { UpdatesSettingsPanel } from './UpdatesSettingsPanel';
import { BackupsSettingsPanel } from './BackupsSettingsPanel';
import { RelaySettingsPanel } from './RelaySettingsPanel';
import { ProLicensePanel } from './ProLicensePanel';
import OrganizationOverviewPanel from './OrganizationOverviewPanel';
import OrganizationAccessPanel from './OrganizationAccessPanel';
import OrganizationSharingPanel from './OrganizationSharingPanel';
import OrganizationBillingPanel from './OrganizationBillingPanel';
import { APIAccessPanel } from './APIAccessPanel';
import { SecurityOverviewPanel } from './SecurityOverviewPanel';
import { SecurityAuthPanel } from './SecurityAuthPanel';
import RolesPanel from './RolesPanel';
import UserAssignmentsPanel from './UserAssignmentsPanel';
import AuditLogPanel from './AuditLogPanel';
import { AuditWebhookPanel } from './AuditWebhookPanel';
import { DiagnosticsPanel } from './DiagnosticsPanel';
import { ReportingPanel } from './ReportingPanel';
import type { SettingsTab } from './settingsTypes';

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
  getGeneralPanelProps: () => Parameters<typeof GeneralSettingsPanel>[0];
  getNetworkPanelProps: () => Parameters<typeof NetworkSettingsPanel>[0];
  getUpdatesPanelProps: () => Parameters<typeof UpdatesSettingsPanel>[0];
  getBackupsPanelProps: () => Parameters<typeof BackupsSettingsPanel>[0];
  getOrganizationOverviewPanelProps: () => Parameters<typeof OrganizationOverviewPanel>[0];
  getOrganizationAccessPanelProps: () => Parameters<typeof OrganizationAccessPanel>[0];
  getOrganizationSharingPanelProps: () => Parameters<typeof OrganizationSharingPanel>[0];
  getOrganizationBillingPanelProps: () => Parameters<typeof OrganizationBillingPanel>[0];
  getApiAccessPanelProps: () => Parameters<typeof APIAccessPanel>[0];
  getSecurityOverviewPanelProps: () => Parameters<typeof SecurityOverviewPanel>[0];
  getSecurityAuthPanelProps: () => Parameters<typeof SecurityAuthPanel>[0];
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
