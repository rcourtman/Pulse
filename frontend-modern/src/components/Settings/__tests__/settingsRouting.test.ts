import { describe, expect, it } from 'vitest';
import {
  DEFAULT_SETTINGS_TAB,
  deriveAgentFromPath,
  deriveTabFromPath,
  deriveTabFromQuery,
  resolveCanonicalSettingsPath,
  settingsTabPath,
  type SettingsTab,
} from '../settingsRouting';
import { isFeatureLocked, isTabLocked } from '../settingsFeatureGates';

const canonicalTabPaths = {
  proxmox: '/settings/infrastructure/api',
  docker: '/settings/workloads/docker',
  agents: '/settings/workloads',
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

describe('settingsRouting', () => {
  it('uses Unified Agents as the default /settings landing tab', () => {
    expect(DEFAULT_SETTINGS_TAB).toBe('agents');
  });

  it('returns canonical paths for every tab id', () => {
    for (const [tab, path] of Object.entries(canonicalTabPaths)) {
      expect(settingsTabPath(tab as SettingsTab)).toBe(path);
    }
  });

  it('falls back to /settings/{tab} for unknown tab patterns', () => {
    expect(settingsTabPath('future-tab' as SettingsTab)).toBe('/settings/future-tab');
  });

  it('resolves only canonical settings paths', () => {
    expect(resolveCanonicalSettingsPath('/settings/system-updates')).toBe(
      '/settings/system-updates',
    );
    expect(resolveCanonicalSettingsPath('/settings/infrastructure')).toBe('/settings/workloads');
    expect(resolveCanonicalSettingsPath('/settings/infrastructure/pve')).toBe(
      '/settings/infrastructure/api/pve',
    );
    expect(resolveCanonicalSettingsPath('/not-settings')).toBeNull();
  });

  it('maps organization routing contracts', () => {
    const organizationCases: Array<[string, SettingsTab]> = [
      ['/settings/organization', 'organization-overview'],
      ['/settings/organization/access', 'organization-access'],
      ['/settings/organization/sharing', 'organization-sharing'],
      ['/settings/organization/billing', 'organization-billing'],
    ];
    for (const [path, expectedTab] of organizationCases) {
      expect(deriveTabFromPath(path)).toBe(expectedTab);
    }
  });

  it('maps query deep-links contract values', () => {
    const queryCases: Array<[string, SettingsTab | null]> = [
      ['?tab=infrastructure', 'agents'],
      ['?tab=agents', 'agents'],
      ['?tab=workloads', 'agents'],
      ['?tab=proxmox', 'proxmox'],
      ['?tab=docker', 'docker'],
      ['?tab=system-recovery', 'system-recovery'],
      ['?tab=organization-overview', 'organization-overview'],
      ['?tab=organization-billing', 'organization-billing'],
      ['?tab=security-overview', 'security-overview'],
      ['?tab=unknown', null],
    ];
    for (const [query, expectedTab] of queryCases) {
      expect(deriveTabFromQuery(query)).toBe(expectedTab);
    }
  });

  it('evaluates feature-lock behavior contracts', () => {
    expect(isFeatureLocked(undefined, hasFeatures([]), () => true)).toBe(false);
    expect(isFeatureLocked([], hasFeatures([]), () => true)).toBe(false);
    expect(isFeatureLocked(['relay'], hasFeatures([]), () => false)).toBe(false);
    expect(isFeatureLocked(['relay'], hasFeatures(['relay']), () => true)).toBe(false);
    expect(isFeatureLocked(['relay'], hasFeatures([]), () => true)).toBe(true);
    expect(isFeatureLocked(['relay', 'audit_logging'], hasFeatures(['relay']), () => true)).toBe(
      true,
    );
  });

  it('locks gated tabs based on features and license state', () => {
    const gatedTabs: Array<[SettingsTab, string]> = [
      ['system-relay', 'relay'],
      ['security-webhooks', 'audit_logging'],
      ['organization-overview', 'multi_tenant'],
      ['organization-access', 'multi_tenant'],
      ['organization-sharing', 'multi_tenant'],
      ['organization-billing', 'multi_tenant'],
    ];
    for (const [tab, requiredFeature] of gatedTabs) {
      expect(isTabLocked(tab, hasFeatures([]), () => true)).toBe(true);
      expect(isTabLocked(tab, hasFeatures([requiredFeature]), () => true)).toBe(false);
      expect(isTabLocked(tab, hasFeatures([]), () => false)).toBe(false);
    }
    expect(isTabLocked('proxmox', hasFeatures([]), () => true)).toBe(false);
  });

  it('maps deriveAgentFromPath contracts for canonical infrastructure routes', () => {
    const agentCases: Array<[string, 'pve' | 'pbs' | 'pmg' | null]> = [
      ['/settings/infrastructure/api/pve', 'pve'],
      ['/settings/infrastructure/api/pbs', 'pbs'],
      ['/settings/infrastructure/api/pmg', 'pmg'],
      ['/settings/infrastructure', null],
      ['/settings/workloads', null],
    ];
    for (const [path, expectedAgent] of agentCases) {
      expect(deriveAgentFromPath(path)).toBe(expectedAgent);
    }
  });
});
