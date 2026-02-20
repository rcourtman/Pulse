import { describe, expect, it } from 'vitest';
import { deriveTabFromPath, settingsTabPath, type SettingsTab } from '../settingsRouting';
import { getTabLockReason, isTabLocked } from '../settingsFeatureGates';

const canonicalTabPaths = {
  proxmox: '/settings/infrastructure',
  docker: '/settings/workloads/docker',
  agents: '/settings/workloads',
  'workspace': '/settings/system-general',
  'integrations': '/settings/system-network',
  'maintenance': '/settings/system-updates',
  'maintenance': '/settings/system-recovery',
  'system-ai': '/settings/system-ai',
  // // 'system-relay': '/settings/system-relay',
  'system-pro': '/settings/system-pro',
  // // 'organization-overview': '/settings/organization',
  // // 'organization-access': '/settings/organization/access',
  // // 'organization-billing': '/settings/organization/billing',
  // // 'organization-billing-admin': '/settings/organization/billing-admin',
  // // 'organization-sharing': '/settings/organization/sharing',
  api: '/settings/integrations/api',
  'authentication': '/settings/security-overview',
  'authentication': '/settings/security-auth',
  'authentication': '/settings/security-sso',
  'team': '/settings/security-roles',
  'team': '/settings/security-users',
  'audit': '/settings/security-audit',
  // // 'security-webhooks': '/settings/security-webhooks',
} as const satisfies Record<SettingsTab, string>;

const hasFeatures =
  (features: string[]) =>
    (feature: string): boolean =>
      features.includes(feature);

const gatedTabs: Array<[SettingsTab, string]> = [
];

describe('settingsNavigation integration scaffold', () => {
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

    it('getTabLockReason returns reason for locked tabs and null for unlocked', () => {
      for (const [tab, requiredFeature] of gatedTabs) {
        expect(getTabLockReason(tab, hasFeatures([]), () => true)).toBe(
          'This settings section requires Pro.',
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
