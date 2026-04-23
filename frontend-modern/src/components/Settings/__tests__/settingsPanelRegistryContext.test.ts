import { describe, expect, it, vi } from 'vitest';
import type { Setter } from 'solid-js';
import type { SecurityStatus } from '@/types/config';
import type { SettingsSystemPanels } from '../useSettingsSystemPanels';
import {
  buildSettingsPanelRegistryContext,
  type UseSettingsPanelRegistryParams,
} from '../settingsPanelRegistryContext';

const noopSetter: Setter<boolean> = () => false;

const systemPanels: SettingsSystemPanels = {
  systemGeneralPanel: () => null,
  getNetworkPanelProps: () => ({} as ReturnType<SettingsSystemPanels['getNetworkPanelProps']>),
  getRecoveryPanelProps: () => ({} as ReturnType<SettingsSystemPanels['getRecoveryPanelProps']>),
  getUpdatesPanelProps: () => ({} as ReturnType<SettingsSystemPanels['getUpdatesPanelProps']>),
};

function buildParams(
  securityStatus: SecurityStatus | null,
): UseSettingsPanelRegistryParams {
  return {
    securityStatus: () => securityStatus,
    securityStatusLoading: () => false,
    organizationMonitoredSystemUsage: () => 0,
    organizationGuestUsage: () => 0,
    loadSecurityStatus: vi.fn(async () => undefined),
    showQuickSecuritySetup: () => false,
    setShowQuickSecuritySetup: noopSetter,
    showQuickSecurityWizard: () => false,
    setShowQuickSecurityWizard: noopSetter,
    showPasswordModal: () => false,
    setShowPasswordModal: noopSetter,
    hideLocalLogin: () => false,
    hideLocalLoginLocked: () => false,
    savingHideLocalLogin: () => false,
    handleHideLocalLoginChange: vi.fn(async () => undefined),
    versionInfo: () => null,
    getInfrastructurePanelProps: () =>
      ({} as ReturnType<UseSettingsPanelRegistryParams['getInfrastructurePanelProps']>),
    systemPanels,
  };
}

describe('buildSettingsPanelRegistryContext', () => {
  it('passes the effective current user into organization panels', () => {
    const context = buildSettingsPanelRegistryContext(buildParams({
      hasAuthentication: true,
      apiTokenConfigured: false,
      apiTokenHint: '',
      requiresAuth: true,
      credentialsEncrypted: true,
      exportProtected: true,
      hasAuditLogging: false,
      configuredButPendingRestart: false,
      authUsername: 'local-admin',
      ssoSessionUsername: 'sso-admin',
      proxyAuthUsername: 'proxy-admin',
    }));

    expect(context.getOrganizationOverviewPanelProps()).toMatchObject({ currentUser: 'proxy-admin' });
    expect(context.getOrganizationAccessPanelProps()).toMatchObject({ currentUser: 'proxy-admin' });
    expect(context.getOrganizationSharingPanelProps()).toMatchObject({ currentUser: 'proxy-admin' });
  });

  it('falls back from proxy auth to sso session to local auth username', () => {
    const ssoContext = buildSettingsPanelRegistryContext(buildParams({
      hasAuthentication: true,
      apiTokenConfigured: false,
      apiTokenHint: '',
      requiresAuth: true,
      credentialsEncrypted: true,
      exportProtected: true,
      hasAuditLogging: false,
      configuredButPendingRestart: false,
      authUsername: 'local-admin',
      ssoSessionUsername: 'sso-admin',
    }));

    const localContext = buildSettingsPanelRegistryContext(buildParams({
      hasAuthentication: true,
      apiTokenConfigured: false,
      apiTokenHint: '',
      requiresAuth: true,
      credentialsEncrypted: true,
      exportProtected: true,
      hasAuditLogging: false,
      configuredButPendingRestart: false,
      authUsername: 'local-admin',
    }));

    expect(ssoContext.getOrganizationSharingPanelProps()).toMatchObject({ currentUser: 'sso-admin' });
    expect(localContext.getOrganizationSharingPanelProps()).toMatchObject({ currentUser: 'local-admin' });
  });

  it('leaves currentUser undefined when no authenticated username is available', () => {
    const context = buildSettingsPanelRegistryContext(buildParams({
      hasAuthentication: true,
      apiTokenConfigured: false,
      apiTokenHint: '',
      requiresAuth: true,
      credentialsEncrypted: true,
      exportProtected: true,
      hasAuditLogging: false,
      configuredButPendingRestart: false,
    }));

    expect(context.getOrganizationOverviewPanelProps()).toMatchObject({ currentUser: undefined });
    expect(context.getOrganizationAccessPanelProps()).toMatchObject({ currentUser: undefined });
    expect(context.getOrganizationSharingPanelProps()).toMatchObject({ currentUser: undefined });
  });
});
