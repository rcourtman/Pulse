import { describe, expect, it, vi } from 'vitest';
import { createSignal, type Setter } from 'solid-js';
import type { SecurityStatus } from '@/types/config';
import { createSettingsPanelRegistry } from '../settingsPanelRegistry';
import type { SettingsSystemPanels } from '../useSettingsSystemPanels';
import {
  buildSettingsPanelRegistryContext,
  type UseSettingsPanelRegistryParams,
} from '../settingsPanelRegistryContext';

const noopSetter: Setter<boolean> = () => false;

const systemPanels: SettingsSystemPanels = {
  systemGeneralPanel: () => null,
  getNetworkPanelProps: () => ({}) as ReturnType<SettingsSystemPanels['getNetworkPanelProps']>,
  getRecoveryPanelProps: () => ({}) as ReturnType<SettingsSystemPanels['getRecoveryPanelProps']>,
  getUpdatesPanelProps: () => ({}) as ReturnType<SettingsSystemPanels['getUpdatesPanelProps']>,
};

function buildParams(securityStatus: SecurityStatus | null): UseSettingsPanelRegistryParams {
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
      ({}) as ReturnType<UseSettingsPanelRegistryParams['getInfrastructurePanelProps']>,
    systemPanels,
  };
}

describe('buildSettingsPanelRegistryContext', () => {
  it('uses the authenticated principal rather than the configured administrator', () => {
    const context = buildSettingsPanelRegistryContext(
      buildParams({
        currentUsername: 'org-owner',
        authUsername: 'instance-admin',
      } as SecurityStatus),
    );
    expect(context.getOrganizationSharingPanelProps().currentUser).toBe('org-owner');
    expect(context.getOrganizationAccessPanelProps().currentUser).toBe('org-owner');
    expect(context.getOrganizationOverviewPanelProps().currentUser).toBe('org-owner');
    expect(context.getApiAccessPanelProps().currentUser).toBe('org-owner');
    expect(context.getUserAssignmentsPanelProps().currentUser).toBe('org-owner');
    expect(context.getRolesPanelProps().currentUser).toBe('org-owner');
  });

  it('does not replace an explicitly empty principal with the configured admin', () => {
    const context = buildSettingsPanelRegistryContext(
      buildParams({
        currentUsername: '',
        authUsername: 'instance-admin',
      } as SecurityStatus),
    );
    expect(context.getOrganizationSharingPanelProps().currentUser).toBe('');
    expect(context.getApiAccessPanelProps().currentUser).toBe('');
  });

  it('passes the effective current user into organization panels', () => {
    const context = buildSettingsPanelRegistryContext(
      buildParams({
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
      }),
    );

    expect(context.getOrganizationOverviewPanelProps()).toMatchObject({
      currentUser: 'proxy-admin',
    });
    expect(context.getOrganizationAccessPanelProps()).toMatchObject({ currentUser: 'proxy-admin' });
    expect(context.getOrganizationSharingPanelProps()).toMatchObject({
      currentUser: 'proxy-admin',
    });
  });

  it('falls back from proxy auth to sso session to local auth username', () => {
    const ssoContext = buildSettingsPanelRegistryContext(
      buildParams({
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
      }),
    );

    const localContext = buildSettingsPanelRegistryContext(
      buildParams({
        hasAuthentication: true,
        apiTokenConfigured: false,
        apiTokenHint: '',
        requiresAuth: true,
        credentialsEncrypted: true,
        exportProtected: true,
        hasAuditLogging: false,
        configuredButPendingRestart: false,
        authUsername: 'local-admin',
      }),
    );

    expect(ssoContext.getOrganizationSharingPanelProps()).toMatchObject({
      currentUser: 'sso-admin',
    });
    expect(localContext.getOrganizationSharingPanelProps()).toMatchObject({
      currentUser: 'local-admin',
    });
  });

  it('leaves currentUser undefined when no authenticated username is available', () => {
    const context = buildSettingsPanelRegistryContext(
      buildParams({
        hasAuthentication: true,
        apiTokenConfigured: false,
        apiTokenHint: '',
        requiresAuth: true,
        credentialsEncrypted: true,
        exportProtected: true,
        hasAuditLogging: false,
        configuredButPendingRestart: false,
      }),
    );

    expect(context.getOrganizationOverviewPanelProps()).toMatchObject({ currentUser: undefined });
    expect(context.getOrganizationAccessPanelProps()).toMatchObject({ currentUser: undefined });
    expect(context.getOrganizationSharingPanelProps()).toMatchObject({ currentUser: undefined });
    expect(context.getApiAccessPanelProps()).toMatchObject({ currentUser: undefined });
  });
  it('binds Reporting to the live reader and explicit read capability', () => {
    const [status, setStatus] = createSignal<SecurityStatus | null>(null);
    const context = buildSettingsPanelRegistryContext({
      ...buildParams(null),
      securityStatus: status,
    });
    expect(context.getReportingPanelProps()).toEqual({
      currentUser: undefined,
      accessKey: '[null,null]',
      canRead: false,
    });
    setStatus({
      currentUsername: 'report-reader',
      settingsCapabilities: { reportingRead: true },
    } as SecurityStatus);
    expect(context.getReportingPanelProps()).toEqual({
      currentUser: 'report-reader',
      accessKey: '[null,null]',
      canRead: true,
    });
    setStatus({
      currentUsername: '',
      settingsCapabilities: { reportingRead: false },
    } as SecurityStatus);
    expect(context.getReportingPanelProps()).toEqual({
      currentUser: '',
      accessKey: '[null,null]',
      canRead: false,
    });
  });
  it('normalizes Reporting scope order without hiding credential or scope changes', () => {
    const [status, setStatus] = createSignal<SecurityStatus | null>({
      currentUsername: 'operator',
      apiTokenHint: 'hint-a',
      tokenScopes: ['settings:write', 'settings:read'],
    } as SecurityStatus);
    const context = buildSettingsPanelRegistryContext({
      ...buildParams(null),
      securityStatus: status,
    });
    const key = context.getReportingPanelProps().accessKey;
    setStatus({ ...status(), tokenScopes: ['settings:read', 'settings:write'] } as SecurityStatus);
    expect(context.getReportingPanelProps().accessKey).toBe(key);
    setStatus({ ...status(), tokenScopes: ['settings:read'] } as SecurityStatus);
    expect(context.getReportingPanelProps().accessKey).not.toBe(key);
    setStatus({
      ...status(),
      apiTokenHint: 'hint-b',
      tokenScopes: ['settings:read', 'settings:write'],
    } as SecurityStatus);
    expect(context.getReportingPanelProps().accessKey).not.toBe(key);
  });
  it('binds audit webhooks to the live principal and explicit read/write capabilities', () => {
    const [status, setStatus] = createSignal<SecurityStatus | null>(null);
    const context = buildSettingsPanelRegistryContext({
      ...buildParams(null),
      securityStatus: status,
    });
    expect(context.getAuditWebhookPanelProps()).toEqual({
      currentUser: undefined,
      canRead: false,
      canManage: false,
    });
    setStatus({
      currentUsername: 'audit-reader',
      settingsCapabilities: { auditWebhooksRead: true, auditWebhooksWrite: false },
    } as SecurityStatus);
    expect(context.getAuditWebhookPanelProps()).toEqual({
      currentUser: 'audit-reader',
      canRead: true,
      canManage: false,
    });
    setStatus({
      currentUsername: 'audit-writer',
      settingsCapabilities: { auditWebhooksRead: true, auditWebhooksWrite: true },
    } as SecurityStatus);
    expect(context.getAuditWebhookPanelProps()).toEqual({
      currentUser: 'audit-writer',
      canRead: true,
      canManage: true,
    });
    setStatus({ currentUsername: '', settingsCapabilities: {} } as SecurityStatus);
    expect(context.getAuditWebhookPanelProps()).toEqual({
      currentUser: '',
      canRead: false,
      canManage: false,
    });
  });
  it('binds Roles to the live principal and explicit roles capability', () => {
    const [status, setStatus] = createSignal<SecurityStatus | null>(null);
    const context = buildSettingsPanelRegistryContext({
      ...buildParams(null),
      securityStatus: status,
    });
    expect(context.getRolesPanelProps()).toEqual({ currentUser: undefined, canManage: false });
    setStatus({
      currentUsername: 'role-reader',
      settingsCapabilities: { roles: true, users: false },
    } as SecurityStatus);
    expect(context.getRolesPanelProps()).toEqual({ currentUser: 'role-reader', canManage: true });
    setStatus({
      currentUsername: 'other-reader',
      settingsCapabilities: { roles: false, users: true },
    } as SecurityStatus);
    expect(context.getRolesPanelProps()).toEqual({ currentUser: 'other-reader', canManage: false });
  });
  it('binds User Access to the live principal and explicit users capability', () => {
    const [status, setStatus] = createSignal<SecurityStatus | null>(null);
    const context = buildSettingsPanelRegistryContext({
      ...buildParams(null),
      securityStatus: status,
    });
    expect(context.getUserAssignmentsPanelProps()).toEqual({
      currentUser: undefined,
      canManage: false,
    });
    setStatus({
      currentUsername: 'reader-one',
      settingsCapabilities: { users: true },
    } as SecurityStatus);
    expect(context.getUserAssignmentsPanelProps()).toEqual({
      currentUser: 'reader-one',
      canManage: true,
    });
    setStatus({
      currentUsername: 'reader-two',
      settingsCapabilities: { users: false },
    } as SecurityStatus);
    expect(context.getUserAssignmentsPanelProps()).toEqual({
      currentUser: 'reader-two',
      canManage: false,
    });
  });
  it('binds Audit Log to live principal, explicit capability and normalised credential scopes', () => {
    const [status, setStatus] = createSignal<SecurityStatus | null>(null);
    const context = buildSettingsPanelRegistryContext({
      ...buildParams(null),
      securityStatus: status,
    });
    expect(context.getAuditLogPanelProps()).toEqual({
      currentUser: undefined,
      accessKey: '[null,null]',
      canRead: false,
    });
    setStatus({
      currentUsername: 'audit-reader',
      apiTokenHint: 'fixture-hint',
      tokenScopes: ['settings:read', 'monitoring:read'],
      settingsCapabilities: { auditLog: true },
    } as SecurityStatus);
    expect(context.getAuditLogPanelProps()).toMatchObject({
      currentUser: 'audit-reader',
      canRead: true,
    });
    const key = context.getAuditLogPanelProps()?.accessKey;
    setStatus({ ...status(), tokenScopes: ['monitoring:read', 'settings:read'] } as SecurityStatus);
    expect(context.getAuditLogPanelProps()?.accessKey).toBe(key);
    setStatus({ ...status(), tokenScopes: ['monitoring:read'] } as SecurityStatus);
    expect(context.getAuditLogPanelProps()?.accessKey).not.toBe(key);
    setStatus({
      ...status(),
      currentUsername: '',
      settingsCapabilities: { auditLog: false },
    } as SecurityStatus);
    expect(context.getAuditLogPanelProps()).toMatchObject({ currentUser: '', canRead: false });
  });
  it('binds Diagnostics to the live principal, explicit capability and stable credential scope facts', () => {
    const [status, setStatus] = createSignal<SecurityStatus | null>(null);
    const context = buildSettingsPanelRegistryContext({
      ...buildParams(null),
      securityStatus: status,
    });
    expect(context.getDiagnosticsPanelProps()).toEqual({
      currentUser: undefined,
      accessKey: '[null,null]',
      canRead: false,
    });
    setStatus({
      currentUsername: 'diagnostics-reader',
      apiTokenHint: 'fixture-hint',
      tokenScopes: ['settings:read', 'monitoring:read'],
      settingsCapabilities: { diagnosticsRead: true },
    } as SecurityStatus);
    expect(context.getDiagnosticsPanelProps()).toMatchObject({
      currentUser: 'diagnostics-reader',
      canRead: true,
    });
    const key = context.getDiagnosticsPanelProps().accessKey;
    setStatus({ ...status(), tokenScopes: ['monitoring:read', 'settings:read'] } as SecurityStatus);
    expect(context.getDiagnosticsPanelProps().accessKey).toBe(key);
    setStatus({ ...status(), tokenScopes: ['settings:read'] } as SecurityStatus);
    expect(context.getDiagnosticsPanelProps().accessKey).not.toBe(key);
    setStatus({
      currentUsername: 'other-reader',
      settingsCapabilities: { diagnosticsRead: false },
    } as SecurityStatus);
    expect(context.getDiagnosticsPanelProps()).toMatchObject({
      currentUser: 'other-reader',
      canRead: false,
    });
  });
  it('binds System Logs to the current reader and explicit capability without treating scope order as a new reader', () => {
    const [status, setStatus] = createSignal<SecurityStatus | null>(null);
    const context = buildSettingsPanelRegistryContext({
      ...buildParams(null),
      securityStatus: status,
    });
    expect(context.getSystemLogsPanelProps()).toEqual({
      currentUser: undefined,
      accessKey: '[null,null]',
      canRead: false,
    });
    setStatus({
      currentUsername: 'reader',
      apiTokenHint: 'hint-a',
      tokenScopes: ['settings:read', 'settings:write'],
      settingsCapabilities: { systemLogsRead: true },
    } as SecurityStatus);
    const accepted = context.getSystemLogsPanelProps();
    expect(accepted).toMatchObject({ currentUser: 'reader', canRead: true });
    setStatus({ ...status(), tokenScopes: ['settings:write', 'settings:read'] } as SecurityStatus);
    expect(context.getSystemLogsPanelProps()).toEqual(accepted);
    setStatus({ ...status(), apiTokenHint: 'hint-b' } as SecurityStatus);
    expect(context.getSystemLogsPanelProps().accessKey).not.toBe(
      (accepted as { accessKey: string }).accessKey,
    );
    setStatus({ ...status(), settingsCapabilities: { systemLogsRead: false } } as SecurityStatus);
    expect(context.getSystemLogsPanelProps().canRead).toBe(false);
  });
});

it('dispatches Availability with the live principal, explicit read capability and stable scope identity', () => {
  const [status, setStatus] = createSignal<SecurityStatus | null>(null);
  const context = buildSettingsPanelRegistryContext({
    ...buildParams(null),
    securityStatus: status,
  });
  expect(context.getAvailabilityPanelProps()).toEqual({
    currentUser: undefined,
    accessKey: '[null,null]',
    canRead: false,
  });
  setStatus({
    currentUsername: 'reader-a',
    authUsername: 'configured-admin',
    apiTokenHint: 'hint-a',
    tokenScopes: ['settings:write', 'settings:read'],
    settingsCapabilities: { availabilityRead: true },
  } as SecurityStatus);
  const registry = createSettingsPanelRegistry(context);
  const accepted = registry['monitoring-availability'].getProps?.();
  expect(accepted).toEqual(context.getAvailabilityPanelProps());
  expect(accepted).toEqual({
    currentUser: 'reader-a',
    accessKey: '["hint-a",["settings:read","settings:write"]]',
    canRead: true,
  });
  setStatus({ ...status()!, tokenScopes: ['settings:read', 'settings:write'] });
  expect(context.getAvailabilityPanelProps()).toEqual(accepted);
  setStatus({ ...status()!, apiTokenHint: 'hint-b' });
  expect(context.getAvailabilityPanelProps().accessKey).not.toEqual(
    (accepted as { accessKey: string }).accessKey,
  );
  setStatus({
    ...status()!,
    currentUsername: '',
    settingsCapabilities: { availabilityRead: false },
  } as SecurityStatus);
  expect(context.getAvailabilityPanelProps()).toMatchObject({ currentUser: '', canRead: false });
});
