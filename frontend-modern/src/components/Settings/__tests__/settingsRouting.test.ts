import { describe, expect, it } from 'vitest';
import {
  agentKeyFromPlatformType,
  DEFAULT_SETTINGS_TAB,
  deriveAgentFromPath,
  deriveTabFromPath,
  deriveTabFromQuery,
  resolveCanonicalSettingsPath,
  settingsAgentLabel,
  settingsAgentNodeLabel,
  settingsAgentPath,
  settingsAgentPlatformType,
  settingsTabPath,
  type SettingsTab,
} from '../settingsRouting';
import { isFeatureLocked, isTabLocked } from '../settingsFeatureGates';

const canonicalTabPaths = {
  proxmox: '/settings/infrastructure/proxmox',
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
  api: '/settings/security/api',
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
    expect(resolveCanonicalSettingsPath('/settings/workloads')).toBe('/settings');
    expect(resolveCanonicalSettingsPath('/settings/workloads/docker')).toBe('/settings');
    expect(resolveCanonicalSettingsPath('/settings/system-updates')).toBe(
      '/settings/system-updates',
    );
    expect(resolveCanonicalSettingsPath('/settings/infrastructure')).toBe('/settings');
    expect(resolveCanonicalSettingsPath('/settings/infrastructure/pve')).toBe(
      '/settings/infrastructure/proxmox/pve',
    );
    expect(resolveCanonicalSettingsPath('/settings/infrastructure/api')).toBe(
      '/settings/infrastructure/proxmox',
    );
    expect(resolveCanonicalSettingsPath('/settings/infrastructure/api/pve')).toBe(
      '/settings/infrastructure/proxmox/pve',
    );
    expect(resolveCanonicalSettingsPath('/settings/integrations/api')).toBe(
      '/settings/security/api',
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
      ['?tab=docker', 'agents'],
      ['?tab=system-ai', 'system-ai'],
      ['?tab=system-relay', 'system-relay'],
      ['?tab=system-pro', 'system-pro'],
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
      ['/settings/infrastructure/proxmox/pve', 'pve'],
      ['/settings/infrastructure/proxmox/pbs', 'pbs'],
      ['/settings/infrastructure/proxmox/pmg', 'pmg'],
      ['/settings/infrastructure', null],
      ['/settings', null],
    ];
    for (const [path, expectedAgent] of agentCases) {
      expect(deriveAgentFromPath(path)).toBe(expectedAgent);
    }
  });

  it('maps settings agent keys to canonical platform types, labels, and paths', () => {
    expect(settingsAgentPath('pve')).toBe('/settings/infrastructure/proxmox/pve');
    expect(settingsAgentPlatformType('pve')).toBe('proxmox-pve');
    expect(settingsAgentLabel('pve')).toBe('Proxmox VE');
    expect(settingsAgentNodeLabel('pve')).toBe('Proxmox VE node');

    expect(settingsAgentPath('pbs')).toBe('/settings/infrastructure/proxmox/pbs');
    expect(settingsAgentPlatformType('pbs')).toBe('proxmox-pbs');
    expect(settingsAgentLabel('pbs')).toBe('Proxmox Backup Server');
    expect(settingsAgentNodeLabel('pbs')).toBe('Proxmox Backup Server');

    expect(settingsAgentPath('pmg')).toBe('/settings/infrastructure/proxmox/pmg');
    expect(settingsAgentPlatformType('pmg')).toBe('proxmox-pmg');
    expect(settingsAgentLabel('pmg')).toBe('Proxmox Mail Gateway');
    expect(settingsAgentNodeLabel('pmg')).toBe('Proxmox Mail Gateway');
  });

  it('derives settings agent keys from canonical and legacy platform aliases', () => {
    expect(agentKeyFromPlatformType('proxmox-pve')).toBe('pve');
    expect(agentKeyFromPlatformType('proxmox-pbs')).toBe('pbs');
    expect(agentKeyFromPlatformType('proxmox-pmg')).toBe('pmg');
    expect(agentKeyFromPlatformType('proxmox')).toBe('pve');
    expect(agentKeyFromPlatformType('pbs')).toBe('pbs');
    expect(agentKeyFromPlatformType('pmg')).toBe('pmg');
    expect(agentKeyFromPlatformType('agent')).toBeNull();
  });

  it('treats proxmox deep links as infrastructure aliases', () => {
    expect(deriveTabFromPath('/settings/infrastructure/proxmox')).toBe('agents');
    expect(deriveTabFromPath('/settings/infrastructure/proxmox/pve')).toBe('agents');
    expect(deriveTabFromPath('/settings/infrastructure/api/pve')).toBe('agents');
  });
});
