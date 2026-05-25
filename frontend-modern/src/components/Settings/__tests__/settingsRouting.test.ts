import { describe, expect, it } from 'vitest';
import {
  agentKeyFromPlatformType,
  DEFAULT_SETTINGS_TAB,
  deriveTabFromPath,
  deriveTabFromQuery,
  isRetiredSettingsCompatibilityPath,
  isRouteableSettingsLocation,
  isRouteableSettingsPath,
  resolveCanonicalSettingsPath,
  settingsAgentLabel,
  settingsAgentNodeLabel,
  settingsAgentPlatformType,
  settingsTabPath,
  type SettingsTab,
} from '../settingsNavigationModel';
import { isFeatureLocked, isTabLocked } from '../settingsFeatureGates';

const canonicalTabPaths = {
  'infrastructure-systems': '/settings/infrastructure',
  'monitoring-availability': '/settings/monitoring/availability',
  'system-general': '/settings/system-general',
  'system-network': '/settings/system-network',
  'system-updates': '/settings/system-updates',
  'system-recovery': '/settings/system-recovery',
  'system-ai': '/settings/system-ai',
  'system-relay': '/settings/system-relay',
  'system-billing': '/settings/system/billing/plan',
  'support-diagnostics': '/settings/support/diagnostics',
  'support-reporting': '/settings/support/reporting',
  'support-logs': '/settings/support/logs',
  'organization-overview': '/settings/organization',
  'organization-access': '/settings/organization/access',
  'organization-billing': '/settings/organization/billing',
  'organization-billing-admin': '/settings/organization/billing-admin',
  'organization-sharing': '/settings/organization/sharing',
  api: '/settings/security/api',
  'security-overview': '/settings/security-overview',
  'security-data-handling': '/settings/security-data-handling',
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

describe('settingsNavigationModel', () => {
  it('uses infrastructure systems as the default /settings landing tab', () => {
    expect(DEFAULT_SETTINGS_TAB).toBe('infrastructure-systems');
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
    expect(resolveCanonicalSettingsPath('/settings')).toBe('/settings/infrastructure');
    expect(resolveCanonicalSettingsPath('/settings/workloads')).toBeNull();
    expect(resolveCanonicalSettingsPath('/settings/workloads/docker')).toBeNull();
    expect(resolveCanonicalSettingsPath('/settings/support')).toBe('/settings/support/diagnostics');
    expect(resolveCanonicalSettingsPath('/settings/system-updates')).toBe(
      '/settings/system-updates',
    );
    expect(resolveCanonicalSettingsPath('/settings/infrastructure')).toBe(
      '/settings/infrastructure',
    );
    expect(resolveCanonicalSettingsPath('/settings/monitoring')).toBe(
      '/settings/monitoring/availability',
    );
    expect(resolveCanonicalSettingsPath('/settings/monitoring/availability')).toBe(
      '/settings/monitoring/availability',
    );
    expect(resolveCanonicalSettingsPath('/settings/infrastructure/install')).toBeNull();
    expect(resolveCanonicalSettingsPath('/settings/infrastructure/platforms')).toBeNull();
    expect(resolveCanonicalSettingsPath('/settings/infrastructure/platforms/truenas')).toBeNull();
    expect(resolveCanonicalSettingsPath('/settings/infrastructure/operations')).toBeNull();
    expect(resolveCanonicalSettingsPath('/settings/infrastructure/api')).toBeNull();
    expect(resolveCanonicalSettingsPath('/settings/infrastructure/api/pve')).toBeNull();
    expect(resolveCanonicalSettingsPath('/settings/infrastructure/proxmox')).toBeNull();
    expect(resolveCanonicalSettingsPath('/settings/integrations/api')).toBeNull();
    expect(resolveCanonicalSettingsPath('/settings/operations')).toBeNull();
    expect(resolveCanonicalSettingsPath('/settings/operations/reporting')).toBeNull();
    expect(resolveCanonicalSettingsPath('/settings/operations/logs')).toBeNull();
    expect(resolveCanonicalSettingsPath('/settings/system/billing')).toBe(
      '/settings/system/billing/plan',
    );
    expect(resolveCanonicalSettingsPath('/settings/system-pro')).toBeNull();
    expect(resolveCanonicalSettingsPath('/not-settings')).toBeNull();
  });

  it('keeps retired infrastructure settings aliases out of routeable settings paths', () => {
    const retiredPaths = [
      '/settings/workloads',
      '/settings/workloads/docker',
      '/settings/workloads/docker/agents',
      '/settings/infrastructure/install',
      '/settings/infrastructure/platforms',
      '/settings/infrastructure/platforms/proxmox/pve',
      '/settings/infrastructure/api/pve',
      '/settings/infrastructure/proxmox',
      '/settings/infrastructure/truenas',
      '/settings/infrastructure/vmware',
      '/settings/operations',
      '/settings/operations/reporting',
      '/settings/operations/logs',
      '/settings/integrations/api',
      '/settings/system-pro',
    ];

    for (const path of retiredPaths) {
      expect(isRetiredSettingsCompatibilityPath(path)).toBe(true);
      expect(isRouteableSettingsPath(path)).toBe(false);
    }

    expect(isRouteableSettingsPath('/settings')).toBe(true);
    expect(isRouteableSettingsPath('/settings/infrastructure')).toBe(true);
    expect(isRouteableSettingsPath('/settings/monitoring/availability')).toBe(true);
    expect(isRouteableSettingsPath('/settings/security/api')).toBe(true);
    expect(isRouteableSettingsPath('/settings/unknown')).toBe(false);
  });

  it('rejects invalid route-owned add query states instead of mounting the wrong panel', () => {
    expect(isRouteableSettingsLocation('/settings/infrastructure', '?add=agent')).toBe(true);
    expect(isRouteableSettingsLocation('/settings/infrastructure', '?add=availability')).toBe(
      false,
    );
    expect(isRouteableSettingsLocation('/settings/monitoring/availability', '?add=target')).toBe(
      true,
    );
    expect(
      isRouteableSettingsLocation('/settings/monitoring/availability', '?add=availability'),
    ).toBe(false);
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

  it('maps plan and usage billing subroutes back to the billing tab', () => {
    expect(deriveTabFromPath('/settings/system/billing/plan')).toBe('system-billing');
    expect(deriveTabFromPath('/settings/system/billing/usage')).toBe('system-billing');
  });

  it('maps query deep-links contract values', () => {
    const queryCases: Array<[string, SettingsTab | null]> = [
      ['?tab=infrastructure', 'infrastructure-systems'],
      ['?tab=availability', 'monitoring-availability'],
      ['?tab=monitoring-availability', 'monitoring-availability'],
      ['?tab=system-ai', 'system-ai'],
      ['?tab=system-relay', 'system-relay'],
      ['?tab=system-billing', 'system-billing'],
      ['?tab=diagnostics', 'support-diagnostics'],
      ['?tab=reporting', 'support-reporting'],
      ['?tab=logs', 'support-logs'],
      ['?tab=system-recovery', 'system-recovery'],
      ['?tab=organization-overview', 'organization-overview'],
      ['?tab=organization-billing', 'organization-billing'],
      ['?tab=security-overview', 'security-overview'],
      ['?tab=data-handling', 'security-data-handling'],
      ['?tab=resource-privacy', 'security-data-handling'],
      ['?tab=workloads', null],
      ['?tab=install', null],
      ['?tab=docker', null],
      ['?tab=system-pro', null],
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
    expect(isTabLocked('infrastructure-systems', hasFeatures([]), () => true)).toBe(false);
  });

  it('maps settings agent keys to platform types and labels', () => {
    expect(settingsAgentPlatformType('pve')).toBe('proxmox-pve');
    expect(settingsAgentLabel('pve')).toBe('Proxmox VE');
    expect(settingsAgentNodeLabel('pve')).toBe('Proxmox VE node');

    expect(settingsAgentPlatformType('pbs')).toBe('proxmox-pbs');
    expect(settingsAgentLabel('pbs')).toBe('Proxmox Backup Server');
    expect(settingsAgentNodeLabel('pbs')).toBe('Proxmox Backup Server');

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

  it('maps support routes back to support tabs without legacy operations aliases', () => {
    expect(deriveTabFromPath('/settings/support/diagnostics')).toBe('support-diagnostics');
    expect(deriveTabFromPath('/settings/support/reporting')).toBe('support-reporting');
    expect(deriveTabFromPath('/settings/support/logs')).toBe('support-logs');
    expect(isRouteableSettingsPath('/settings/operations')).toBe(false);
    expect(isRouteableSettingsPath('/settings/operations/reporting')).toBe(false);
    expect(isRouteableSettingsPath('/settings/operations/logs')).toBe(false);
  });
});
