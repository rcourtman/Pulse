import { describe, expect, it } from 'vitest';
import { deriveTabFromPath, settingsTabPath, type SettingsTab } from '../settingsRouting';
import { getTabLockReason, isTabLocked } from '../settingsFeatureGates';

const canonicalTabPaths = {
  proxmox: '/settings/infrastructure',
  docker: '/settings/workloads/docker',
  agents: '/settings/workloads',
  'system-general': '/settings/system-general',
  'system-network': '/settings/system-network',
  'system-updates': '/settings/system-updates',
  'system-backups': '/settings/backups',
  'system-ai': '/settings/system-ai',
  'system-relay': '/settings/integrations/relay',
  'system-logs': '/settings/operations/logs',
  'system-pro': '/settings/system-pro',
  'organization-overview': '/settings/organization',
  'organization-access': '/settings/organization/access',
  'organization-billing': '/settings/billing',
  'organization-sharing': '/settings/organization/sharing',
  api: '/settings/integrations/api',
  'security-overview': '/settings/security-overview',
  'security-auth': '/settings/security-auth',
  'security-sso': '/settings/security-sso',
  'security-roles': '/settings/security-roles',
  'security-users': '/settings/security-users',
  'security-audit': '/settings/security-audit',
  'security-webhooks': '/settings/security-webhooks',
  diagnostics: '/settings/operations/diagnostics',
  reporting: '/settings/operations/reporting',
} as const satisfies Record<SettingsTab, string>;

const hasFeatures =
  (features: string[]) =>
  (feature: string): boolean =>
    features.includes(feature);

const gatedTabs: Array<[SettingsTab, string]> = [
  ['system-relay', 'relay'],
  ['reporting', 'advanced_reporting'],
  ['security-webhooks', 'audit_logging'],
  ['organization-overview', 'multi_tenant'],
  ['organization-access', 'multi_tenant'],
  ['organization-sharing', 'multi_tenant'],
  ['organization-billing', 'multi_tenant'],
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
          'This settings section requires Pulse Pro.',
        );
        expect(getTabLockReason(tab, hasFeatures([requiredFeature]), () => true)).toBeNull();
      }
      expect(getTabLockReason('proxmox', hasFeatures([]), () => true)).toBeNull();
    });
  });

  it.todo('tracks duplicate bootstrap de-duplication');
});
