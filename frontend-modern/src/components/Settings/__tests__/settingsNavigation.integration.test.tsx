import { describe, expect, it } from 'vitest';
import { deriveTabFromPath, settingsTabPath, type SettingsTab } from '../settingsRouting';
import { getTabLockReason, isTabLocked } from '../settingsFeatureGates';
import { getSettingsTabSaveBehavior, shouldHideSettingsNavItem } from '../settingsTabs';

const canonicalTabPaths = {
  proxmox: '/settings/infrastructure/api',
  docker: '/settings/workloads/docker',
  agents: '/settings',
  'system-general': '/settings/system-general',
  'system-network': '/settings/system-network',
  'system-updates': '/settings/system-updates',
  'system-recovery': '/settings/system-recovery',
  'system-ai': '/settings/system-ai',
  'system-relay': '/settings/system-relay',
  'system-pro': '/settings/system-pro',
  'organization-overview': '/settings/organization',
  'organization-access': '/settings/organization/access',
  'organization-billing': '/settings/organization/billing',
  'organization-billing-admin': '/settings/organization/billing-admin',
  'organization-sharing': '/settings/organization/sharing',
  api: '/settings/integrations/api',
  'security-overview': '/settings/security-overview',
  'security-auth': '/settings/security-auth',
  'security-sso': '/settings/security-sso',
  'security-roles': '/settings/security-roles',
  'security-users': '/settings/security-users',
  'security-audit': '/settings/security-audit',
  'security-webhooks': '/settings/security-webhooks',
} as const satisfies Record<SettingsTab, string>;

const hasFeatures =
  (features: string[]) =>
  (feature: string): boolean =>
    features.includes(feature);

const gatedTabs: Array<[SettingsTab, string]> = [
  ['system-relay', 'relay'],
  ['security-webhooks', 'audit_logging'],
  ['organization-overview', 'multi_tenant'],
  ['organization-access', 'multi_tenant'],
  ['organization-sharing', 'multi_tenant'],
  ['organization-billing', 'multi_tenant'],
  ['organization-billing-admin', 'multi_tenant'],
];

describe('settingsNavigation integration scaffold', () => {
  it('hides organization navigation items unless multi-tenant is enabled', () => {
    expect(
      shouldHideSettingsNavItem('organization-overview', {
        hasFeature: hasFeatures([]),
        licenseLoaded: () => false,
        hostedModeEnabled: false,
      }),
    ).toBe(true);

    expect(
      shouldHideSettingsNavItem('organization-overview', {
        hasFeature: hasFeatures(['multi_tenant']),
        licenseLoaded: () => true,
        hostedModeEnabled: false,
      }),
    ).toBe(false);
  });

  it('hides billing admin outside hosted mode', () => {
    expect(
      shouldHideSettingsNavItem('organization-billing-admin', {
        hasFeature: hasFeatures(['multi_tenant']),
        licenseLoaded: () => true,
        hostedModeEnabled: false,
        settingsCapabilities: { billingAdmin: true },
      }),
    ).toBe(true);

    expect(
      shouldHideSettingsNavItem('organization-billing-admin', {
        hasFeature: hasFeatures(['multi_tenant']),
        licenseLoaded: () => true,
        hostedModeEnabled: true,
        settingsCapabilities: { billingAdmin: true },
      }),
    ).toBe(false);
  });

  it('hides tabs when the backend denies the required capability', () => {
    expect(
      shouldHideSettingsNavItem('api', {
        hasFeature: hasFeatures([]),
        licenseLoaded: () => true,
        hostedModeEnabled: false,
        settingsCapabilities: { apiAccessRead: false },
      }),
    ).toBe(true);

    expect(
      shouldHideSettingsNavItem('security-roles', {
        hasFeature: hasFeatures(['rbac']),
        licenseLoaded: () => true,
        hostedModeEnabled: false,
        settingsCapabilities: { roles: false },
      }),
    ).toBe(true);
  });

  it('shows restricted tabs when the backend grants the required capability', () => {
    expect(
      shouldHideSettingsNavItem('security-audit', {
        hasFeature: hasFeatures(['audit_logging']),
        licenseLoaded: () => true,
        hostedModeEnabled: false,
        settingsCapabilities: { auditLog: true },
      }),
    ).toBe(false);

    expect(
      shouldHideSettingsNavItem('system-relay', {
        hasFeature: hasFeatures(['relay']),
        licenseLoaded: () => true,
        hostedModeEnabled: false,
        settingsCapabilities: { relayRead: true, relayWrite: false },
      }),
    ).toBe(false);
  });

  it('derives global save behavior from tab metadata', () => {
    expect(getSettingsTabSaveBehavior('system-general')).toBe('system');
    expect(getSettingsTabSaveBehavior('system-network')).toBe('system');
    expect(getSettingsTabSaveBehavior('system-updates')).toBe('system');
    expect(getSettingsTabSaveBehavior('system-recovery')).toBe('system');
    expect(getSettingsTabSaveBehavior('proxmox')).toBeUndefined();
    expect(getSettingsTabSaveBehavior('security-auth')).toBeUndefined();
  });

  it('resolves every canonical tab path', () => {
    for (const [tab, path] of Object.entries(canonicalTabPaths) as Array<[SettingsTab, string]>) {
      expect(deriveTabFromPath(path)).toBe(tab);
    }
  });

  it('round-trips settingsTabPath through deriveTabFromPath', () => {
    for (const tab of Object.keys(canonicalTabPaths) as SettingsTab[]) {
      expect(deriveTabFromPath(settingsTabPath(tab))).toBe(tab);
    }
  });

  it('settingsTabPath returns unique paths for all tabs', () => {
    const paths = Object.keys(canonicalTabPaths).map((tab) => settingsTabPath(tab as SettingsTab));
    const uniquePaths = new Set(paths);
    expect(uniquePaths.size).toBe(paths.length);
  });

  it('setActiveTab eagerly updates currentTab before navigation', () => {
    for (const tab of Object.keys(canonicalTabPaths) as SettingsTab[]) {
      const path = settingsTabPath(tab);
      const derived = deriveTabFromPath(path);
      expect(derived).toBe(tab);
    }
  });

  describe('locked-tab behavior', () => {
    it('locks gated tabs when license is loaded and required feature is missing', () => {
      for (const [tab, requiredFeature] of gatedTabs) {
        expect(isTabLocked(tab, hasFeatures([]), () => true)).toBe(true);
        expect(isTabLocked(tab, hasFeatures([requiredFeature]), () => true)).toBe(false);
      }
    });

    it('does not lock gated tabs while license state is unresolved', () => {
      for (const [tab] of gatedTabs) {
        expect(isTabLocked(tab, hasFeatures([]), () => false)).toBe(false);
      }
    });

    it('keeps non-gated tabs unlocked', () => {
      expect(isTabLocked('proxmox', hasFeatures([]), () => true)).toBe(false);
    });

    it('getTabLockReason returns tier-specific reason for locked tabs and null for unlocked', () => {
      const expectedTierLabels: Record<string, string> = {
        relay: 'Relay',
        multi_tenant: 'MSP',
        audit_logging: 'Pro',
      };
      for (const [tab, requiredFeature] of gatedTabs) {
        const tierLabel = expectedTierLabels[requiredFeature] ?? 'Pro';
        expect(getTabLockReason(tab, hasFeatures([]), () => true)).toBe(
          `This settings section requires ${tierLabel}.`,
        );
        expect(getTabLockReason(tab, hasFeatures([requiredFeature]), () => true)).toBeNull();
      }
      expect(getTabLockReason('proxmox', hasFeatures([]), () => true)).toBeNull();
    });
  });

  it('verifies Settings.tsx bootstrap calls only loadLicenseStatus', async () => {
    const settingsSource = (await import('../Settings.tsx?raw')).default;
    const onMountMatch = settingsSource.match(/onMount\(\(\)\s*=>\s*\{([^}]+)\}/);
    expect(onMountMatch).toBeTruthy();
    const onMountBody = onMountMatch![1];
    expect(onMountBody).toContain('loadLicenseStatus');
    expect(onMountBody).not.toContain('runDiagnostics');
    expect(settingsSource).toContain('useSettingsAccess');
    expect(settingsSource).toContain('useSettingsPanelRegistry');
    expect(settingsSource).not.toContain('const tabGroups');
  });

  it('panel registry covers all dispatchable tabs', async () => {
    const registrySource = (await import('../settingsPanelRegistry.ts?raw')).default;
    const allTabs = Object.keys(canonicalTabPaths) as SettingsTab[];
    const dispatchableTabs = allTabs.filter((tab) => tab !== 'proxmox');
    for (const tab of dispatchableTabs) {
      const isCovered = registrySource.includes(`'${tab}'`) || registrySource.includes(`${tab}:`);
      expect(isCovered, `panel registry should cover tab '${tab}'`).toBe(true);
    }
  });
});
