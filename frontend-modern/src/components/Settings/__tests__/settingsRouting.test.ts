import { describe, expect, it } from 'vitest';
import {
  deriveAgentFromPath,
  deriveTabFromPath,
  deriveTabFromQuery,
  resolveCanonicalSettingsPath,
  settingsTabPath,
  type SettingsTab,
} from '../settingsRouting';
import { isFeatureLocked, isTabLocked } from '../settingsFeatureGates';

const canonicalTabPaths = {
  proxmox: '/settings/infrastructure',
  docker: '/settings/workloads/docker',
  agents: '/settings/workloads',
  'system-general': '/settings/system-general',
  'system-network': '/settings/system-network',
  'system-updates': '/settings/system-updates',
  'system-backups': '/settings/system-backups',
  'system-ai': '/settings/system-ai',
  'system-relay': '/settings/system-relay',
  'system-logs': '/settings/operations/logs',
  'system-pro': '/settings/system-pro',
  'organization-overview': '/settings/organization',
  'organization-access': '/settings/organization/access',
  'organization-billing': '/settings/organization/billing',
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

describe('settingsRouting', () => {
  it('returns canonical paths for every tab id', () => {
    for (const [tab, path] of Object.entries(canonicalTabPaths)) {
      expect(settingsTabPath(tab as SettingsTab)).toBe(path);
    }
  });

  it('falls back to /settings/{tab} for unknown tab patterns', () => {
    expect(settingsTabPath('future-tab' as SettingsTab)).toBe('/settings/future-tab');
  });

  it('maps all listed legacy aliases and redirects', () => {
    const cases: Array<[string, SettingsTab]> = [
      ['/settings/proxmox', 'proxmox'],
      ['/settings/agent-hub', 'proxmox'],
      ['/settings/docker', 'docker'],
      ['/settings/storage', 'proxmox'],
      ['/settings/hosts', 'agents'],
      ['/settings/host-agents', 'agents'],
      ['/settings/servers', 'proxmox'],
      ['/settings/linuxServers', 'agents'],
      ['/settings/windowsServers', 'agents'],
      ['/settings/macServers', 'agents'],
      ['/settings/agents', 'agents'],
      ['/settings/pve', 'proxmox'],
      ['/settings/pbs', 'proxmox'],
      ['/settings/pmg', 'proxmox'],
      ['/settings/containers', 'docker'],
    ];
    for (const [path, expectedTab] of cases) {
      expect(deriveTabFromPath(path)).toBe(expectedTab);
    }
    expect(deriveTabFromPath('/settings/not-a-real-tab')).toBe('proxmox');
  });

  it('canonicalizes legacy settings routes', () => {
    const canonicalCases: Array<[string, string]> = [
      ['/settings/backups', '/settings/system-backups'],
      ['/settings/integrations/relay', '/settings/system-relay'],
      ['/settings/billing', '/settings/organization/billing'],
      ['/settings/api', '/settings/integrations/api'],
      ['/settings/diagnostics', '/settings/operations/diagnostics'],
      ['/settings/reporting', '/settings/operations/reporting'],
      ['/settings/security', '/settings/security-overview'],
      ['/settings/pve', '/settings/infrastructure/pve'],
      ['/settings/pbs', '/settings/infrastructure/pbs'],
      ['/settings/pmg', '/settings/infrastructure/pmg'],
      ['/settings/docker', '/settings/workloads/docker'],
      ['/settings/hosts', '/settings/workloads'],
    ];

    for (const [path, expected] of canonicalCases) {
      expect(resolveCanonicalSettingsPath(path)).toBe(expected);
    }

    expect(resolveCanonicalSettingsPath('/settings/system-updates')).toBe(
      '/settings/system-updates',
    );
    expect(resolveCanonicalSettingsPath('/not-settings')).toBeNull();
  });

  it('maps organization routing contracts', () => {
    const organizationCases: Array<[string, SettingsTab]> = [
      ['/settings/organization', 'organization-overview'],
      ['/settings/organization/access', 'organization-access'],
      ['/settings/organization/sharing', 'organization-sharing'],
      ['/settings/billing', 'organization-billing'],
      ['/settings/plan', 'organization-billing'],
      ['/settings/organization/billing', 'organization-billing'],
    ];
    for (const [path, expectedTab] of organizationCases) {
      expect(deriveTabFromPath(path)).toBe(expectedTab);
    }
  });

  it('maps query deep-links contract values', () => {
    const queryCases: Array<[string, SettingsTab | null]> = [
      ['?tab=infrastructure', 'proxmox'],
      ['?tab=workloads', 'agents'],
      ['?tab=docker', 'docker'],
      ['?tab=backups', 'system-backups'],
      ['?tab=organization', 'organization-overview'],
      ['?tab=billing', 'organization-billing'],
      ['?tab=security', 'security-overview'],
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
      ['reporting', 'advanced_reporting'],
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

  it('maps deriveAgentFromPath contracts including legacy and storage routes', () => {
    const agentCases: Array<[string, 'pve' | 'pbs' | 'pmg' | null]> = [
      ['/settings/infrastructure/pve', 'pve'],
      ['/settings/infrastructure/pbs', 'pbs'],
      ['/settings/infrastructure/pmg', 'pmg'],
      ['/settings/pve', 'pve'],
      ['/settings/pbs', 'pbs'],
      ['/settings/pmg', 'pmg'],
      ['/settings/storage', 'pbs'],
      ['/settings/workloads', null],
    ];
    for (const [path, expectedAgent] of agentCases) {
      expect(deriveAgentFromPath(path)).toBe(expectedAgent);
    }
  });
});
