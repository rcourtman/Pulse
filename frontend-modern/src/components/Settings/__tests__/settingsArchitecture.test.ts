import { describe, expect, it } from 'vitest';
import settingsSource from '../Settings.tsx?raw';
import settingsShellSource from '../SettingsPageShell.tsx?raw';
import infrastructureWorkspaceSource from '../InfrastructureWorkspace.tsx?raw';
import apiAccessPanelSource from '../APIAccessPanel.tsx?raw';
import auditLogPanelSource from '../AuditLogPanel.tsx?raw';
import auditWebhookPanelSource from '../AuditWebhookPanel.tsx?raw';
import billingAdminPanelSource from '../BillingAdminPanel.tsx?raw';
import generalSettingsPanelSource from '../GeneralSettingsPanel.tsx?raw';
import aiSettingsPanelSource from '../AISettings.tsx?raw';
import networkSettingsPanelSource from '../NetworkSettingsPanel.tsx?raw';
import updatesSettingsPanelSource from '../UpdatesSettingsPanel.tsx?raw';
import recoverySettingsPanelSource from '../RecoverySettingsPanel.tsx?raw';
import relaySettingsPanelSource from '../RelaySettingsPanel.tsx?raw';
import proLicensePanelSource from '../ProLicensePanel.tsx?raw';
import organizationOverviewPanelSource from '../OrganizationOverviewPanel.tsx?raw';
import organizationAccessPanelSource from '../OrganizationAccessPanel.tsx?raw';
import organizationSharingPanelSource from '../OrganizationSharingPanel.tsx?raw';
import organizationBillingPanelSource from '../OrganizationBillingPanel.tsx?raw';
import securityOverviewPanelSource from '../SecurityOverviewPanel.tsx?raw';
import securityAuthPanelSource from '../SecurityAuthPanel.tsx?raw';
import ssoProvidersPanelSource from '../SSOProvidersPanel.tsx?raw';
import rolesPanelSource from '../RolesPanel.tsx?raw';
import userAssignmentsPanelSource from '../UserAssignmentsPanel.tsx?raw';
import { SETTINGS_HEADER_META } from '../settingsHeaderMeta';

const extractedModules = [
  '../settingsTypes.ts',
  '../settingsTabs.ts',
  '../DockerRuntimeSettingsCard.tsx',
  '../settingsHeaderMeta.ts',
  '../settingsFeatureGates.ts',
  '../BackupTransferDialogs.tsx',
  '../InfrastructureWorkspace.tsx',
  '../ProxmoxSettingsPanel.tsx',
  '../SettingsDialogs.tsx',
  '../SettingsPageShell.tsx',
  '../useDiscoverySettingsState.ts',
  '../useSettingsAccess.ts',
  '../useSettingsShellState.ts',
  '../useSettingsNavigation.ts',
  '../useSettingsPanelRegistry.tsx',
  '../useSystemSettingsState.ts',
  '../useInfrastructureSettingsState.ts',
  '../useBackupTransferFlow.ts',
  '../settingsPanelRegistry.ts',
] as const;

const requiredImportSources = [
  './settingsTabs',
  './SettingsDialogs',
  './SettingsPageShell',
  './useBackupTransferFlow',
  './useDiscoverySettingsState',
  './useInfrastructureSettingsState',
  './useSettingsAccess',
  './useSettingsPanelRegistry',
  './useSettingsShellState',
  './useSystemSettingsState',
  './useSettingsNavigation',
] as const;

const topLevelSettingsPanelSources = {
  APIAccessPanel: apiAccessPanelSource,
  AuditLogPanel: auditLogPanelSource,
  AuditWebhookPanel: auditWebhookPanelSource,
  BillingAdminPanel: billingAdminPanelSource,
  GeneralSettingsPanel: generalSettingsPanelSource,
  AISettings: aiSettingsPanelSource,
  InfrastructureWorkspace: infrastructureWorkspaceSource,
  NetworkSettingsPanel: networkSettingsPanelSource,
  UpdatesSettingsPanel: updatesSettingsPanelSource,
  RecoverySettingsPanel: recoverySettingsPanelSource,
  RelaySettingsPanel: relaySettingsPanelSource,
  ProLicensePanel: proLicensePanelSource,
  OrganizationOverviewPanel: organizationOverviewPanelSource,
  OrganizationAccessPanel: organizationAccessPanelSource,
  OrganizationSharingPanel: organizationSharingPanelSource,
  OrganizationBillingPanel: organizationBillingPanelSource,
  SecurityOverviewPanel: securityOverviewPanelSource,
  SecurityAuthPanel: securityAuthPanelSource,
  SSOProvidersPanel: ssoProvidersPanelSource,
  RolesPanel: rolesPanelSource,
  UserAssignmentsPanel: userAssignmentsPanelSource,
} as const;

const canonicalShellTitleExpectations = [
  {
    tab: 'system-general',
    title: 'General',
    source: generalSettingsPanelSource,
  },
  {
    tab: 'system-network',
    title: 'Network',
    source: networkSettingsPanelSource,
  },
  {
    tab: 'system-ai',
    title: 'AI Services',
    source: aiSettingsPanelSource,
  },
  {
    tab: 'system-pro',
    title: 'Pulse Pro',
    source: proLicensePanelSource,
  },
  {
    tab: 'organization-billing',
    title: 'Billing & Plan',
    source: organizationBillingPanelSource,
  },
  {
    tab: 'security-overview',
    title: 'Security Overview',
    source: securityOverviewPanelSource,
  },
  {
    tab: 'security-auth',
    title: 'Authentication',
    source: securityAuthPanelSource,
  },
  {
    tab: 'security-sso',
    title: 'Single Sign-On Providers',
    source: ssoProvidersPanelSource,
  },
] as const;

describe('Settings architecture guardrails', () => {
  it('keeps extracted settings modules present on disk', () => {
    const settingsModuleFiles = {
      ...import.meta.glob('../*.ts'),
      ...import.meta.glob('../*.tsx'),
    };

    for (const modulePath of extractedModules) {
      expect(
        Object.prototype.hasOwnProperty.call(settingsModuleFiles, modulePath),
        `${modulePath} should exist and remain externalized`,
      ).toBe(true);
    }
  });

  it('imports extracted architecture modules from Settings.tsx', () => {
    const importSources = Array.from(
      settingsSource.matchAll(/import[\s\S]*?from\s+['"]([^'"]+)['"];?/g),
      (match) => match[1],
    );

    for (const source of requiredImportSources) {
      expect(importSources, `Settings.tsx should import ${source}`).toContain(source);
    }
  });

  it('routes dispatchable settings tabs through the extracted panel registry', async () => {
    const registrySource = (await import('../settingsPanelRegistry.ts?raw')).default;
    const accessHookSource = (await import('../useSettingsAccess.ts?raw')).default;
    const panelRegistryHookSource = (await import('../useSettingsPanelRegistry.tsx?raw')).default;
    const shellHookSource = (await import('../useSettingsShellState.ts?raw')).default;

    expect(registrySource).toContain('createSettingsPanelRegistry');
    expect(registrySource).toContain('InfrastructureWorkspace');
    expect(registrySource).toContain("'security-webhooks'");
    expect(accessHookSource).toContain('shouldHideSettingsNavItem');
    expect(accessHookSource).toContain('tabFeatureRequirements');
    expect(panelRegistryHookSource).toContain('createSettingsPanelRegistry');
    expect(shellHookSource).toContain('SETTINGS_HEADER_META');
    expect(settingsSource).toContain('useSettingsPanelRegistry');
    expect(settingsSource).toContain('useSettingsAccess');
    expect(settingsSource).toContain('useSettingsShellState');
    expect(settingsSource).toContain('activeSettingsPanelEntry');
    expect(settingsSource).toContain('<Dynamic component={entry().component}');
    expect(settingsSource).not.toContain('<ProxmoxSettingsPanel');
    expect(settingsSource).not.toContain("<Show when={activeTab() === 'system-general'}>");
    expect(settingsSource).not.toContain("<Show when={activeTab() === 'security-webhooks'}>");
  });

  it('does not re-inline extracted tab and header metadata definitions', () => {
    expect(settingsSource).not.toMatch(/\b(?:const|let|var)\s+baseTabGroups\s*=/);
    expect(settingsSource).not.toMatch(/\b(?:const|let|var)\s+SETTINGS_HEADER_META\s*=/);
    expect(settingsSource).not.toMatch(/\b(?:const|let|var)\s+tabFeatureRequirements\s*=/);
  });

  it('keeps Settings.tsx below the monolith guardrail ceiling', () => {
    // If this test fails, the change should be decomposed into the appropriate
    // extracted module rather than increasing the limit. Exceptions require
    // explicit discussion.
    const maxSettingsLines = 4500;
    const settingsLineCount = settingsSource.trimEnd().split(/\r?\n/).length;

    expect(settingsLineCount).toBeLessThanOrEqual(maxSettingsLines);
  });

  it('uses lazy() imports for panel components in settingsPanelRegistry', async () => {
    const source = (await import('../settingsPanelRegistry.ts?raw')).default;

    expect(source).toContain('lazy(');

    const staticImports = Array.from(
      source.matchAll(
        /^import\s+(?!type\b)(?!{[^}]*}\s+from\s+'solid-js').*from\s+'\.\/\w+Panel'/gm,
      ),
    );
    expect(staticImports.length).toBe(0);
  });

  it('keeps page-level settings header chrome inside SettingsPageShell', () => {
    expect(settingsShellSource).toContain('<PageHeader');
    expect(settingsShellSource).toContain('getSettingsSearchEmptyState');
    expect(settingsShellSource).not.toContain('No settings found for "');
    expect(infrastructureWorkspaceSource).not.toContain('<PageHeader');
    expect(infrastructureWorkspaceSource).not.toMatch(/<h[12][^>]*>/);
    expect(infrastructureWorkspaceSource).not.toContain('Add and manage infrastructure');
    expect(infrastructureWorkspaceSource).not.toContain('tracking-[0.22em]');
  });

  it('routes every top-level settings surface through the canonical SettingsPanel framing', () => {
    for (const [panelName, source] of Object.entries(topLevelSettingsPanelSources)) {
      expect(source, `${panelName} should use SettingsPanel`).toContain('SettingsPanel');
      expect(source, `${panelName} should not introduce page-level header chrome`).not.toContain(
        '<PageHeader',
      );
    }
  });

  it('keeps shell titles aligned with the leading settings panel on key top-level surfaces', () => {
    for (const { tab, title, source } of canonicalShellTitleExpectations) {
      expect(SETTINGS_HEADER_META[tab].title, `${tab} should keep its shell title canonical`).toBe(
        title,
      );
      expect(
        source,
        `${tab} should keep its leading SettingsPanel title aligned with the shell title`,
      ).toContain(`title="${title}"`);
    }
  });
});
