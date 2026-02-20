import { lazy } from 'solid-js';
import type { Component } from 'solid-js';
import type { GeneralSettingsPanel as GeneralSettingsPanelType } from './GeneralSettingsPanel';
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
import type { SettingsTab } from './settingsTypes';

const GeneralSettingsPanel = lazy(() =>
  import('./GeneralSettingsPanel').then((m) => ({ default: m.GeneralSettingsPanel })),
);
const NetworkSettingsPanel = lazy(() =>
  import('./NetworkSettingsPanel').then((m) => ({ default: m.NetworkSettingsPanel })),
);
const UpdatesSettingsPanel = lazy(() =>
  import('./UpdatesSettingsPanel').then((m) => ({ default: m.UpdatesSettingsPanel })),
);

const SecurityOverviewPanel = lazy(() =>
  import('./SecurityOverviewPanel').then((m) => ({ default: m.SecurityOverviewPanel })),
);
const UserAssignmentsPanel = lazy(() => import('./UserAssignmentsPanel'));
const AuditLogPanel = lazy(() => import('./AuditLogPanel'));

export interface SettingsPanelRegistryEntry {
  component: Component<any>;
  getProps?: () => object;
}

export type SettingsDispatchableTab = Exclude<SettingsTab, 'proxmox'>;

export type SettingsPanelRegistry = Record<string, SettingsPanelRegistryEntry>;

export interface SettingsPanelRegistryContext {
  agentsPanel: Component;
  dockerPanel: Component;
  systemAiPanel: Component;
  securitySsoPanel: Component;
  getGeneralPanelProps: () => Parameters<typeof GeneralSettingsPanelType>[0];
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
  workspace: {
    component: GeneralSettingsPanel,
    getProps: context.getGeneralPanelProps,
  },
  integrations: {
    component: NetworkSettingsPanel,
    getProps: context.getNetworkPanelProps,
  },
  maintenance: {
    component: UpdatesSettingsPanel,
    getProps: context.getUpdatesPanelProps,
  },
  authentication: {
    component: SecurityOverviewPanel,
    getProps: context.getSecurityOverviewPanelProps,
  },
  team: {
    component: UserAssignmentsPanel,
  },
  audit: {
    component: AuditLogPanel,
  }
});
