import { describe, expect, it } from 'vitest';
import { SETTINGS_NAV_GROUPS, getSettingsNavItem } from '../settingsNavCatalog';
import type { SettingsTab } from '../settingsNavigationModel';
import {
  shouldBlockSettingsRouteItem,
  shouldHideSettingsNavItem,
  type SettingsNavVisibilityContext,
} from '../settingsNavVisibility';

// Companion to infrastructureNavCapabilityGate.test.ts. Gating Infrastructure
// moved the blocked-route fallback onto "the first tab the session can reach",
// which made every remaining ungated-but-admin-only tab a landing page for
// non-admin sessions rather than merely a dead link. Availability checks was
// the first one it hit: /api/availability-targets is RequireAdmin +
// settings:read, so the session landed on a red "Admin privileges required"
// banner over an empty panel.
//
// The route side of each pairing is pinned in
// internal/api/security_status_capability_enforcement_test.go, which probes the
// endpoints with a non-admin identity and asserts the withheld capability
// matches an actual 403.

const ADMIN_ONLY_TABS: ReadonlyArray<{
  tab: SettingsTab;
  capability: keyof NonNullable<SettingsNavVisibilityContext['settingsCapabilities']>;
  routes: string[];
}> = [
  {
    tab: 'monitoring-availability',
    capability: 'availabilityRead',
    routes: ['/api/availability-targets'],
  },
  { tab: 'system-ai', capability: 'pulseIntelligenceRead', routes: ['/api/settings/ai'] },
  { tab: 'system-ai-patrol', capability: 'pulseIntelligenceRead', routes: ['/api/settings/ai'] },
  { tab: 'system-ai-assistant', capability: 'pulseIntelligenceRead', routes: ['/api/settings/ai'] },
  { tab: 'support-diagnostics', capability: 'diagnosticsRead', routes: ['/api/diagnostics'] },
  {
    tab: 'support-reporting',
    capability: 'reportingRead',
    routes: ['/api/admin/reports/catalog', '/api/admin/reports/schedules'],
  },
  {
    tab: 'support-logs',
    capability: 'systemLogsRead',
    routes: ['/api/logs/level', '/api/logs/stream'],
  },
];

const createContext = (
  overrides: Partial<SettingsNavVisibilityContext> = {},
): SettingsNavVisibilityContext => ({
  hasFeature: () => true,
  runtimeCapabilitiesLoaded: () => true,
  hostedModeEnabled: false,
  ...overrides,
});

describe('admin-only settings nav gates', () => {
  it.each(ADMIN_ONLY_TABS)(
    '$tab declares $capability (mount fetch: $routes)',
    ({ tab, capability }) => {
      expect(getSettingsNavItem(tab)?.requiredCapability).toBe(capability);
    },
  );

  it.each(ADMIN_ONLY_TABS)(
    '$tab is hidden and route-blocked when $capability is withheld',
    ({ tab, capability }) => {
      const context = createContext({
        settingsCapabilitiesResolved: true,
        settingsCapabilities: { [capability]: false },
      });

      expect(shouldHideSettingsNavItem(tab, context)).toBe(true);
      expect(shouldBlockSettingsRouteItem(tab, context)).toBe(true);
    },
  );

  it.each(ADMIN_ONLY_TABS)(
    '$tab survives for a session holding $capability',
    ({ tab, capability }) => {
      const context = createContext({
        settingsCapabilitiesResolved: true,
        settingsCapabilities: { [capability]: true },
      });

      expect(shouldHideSettingsNavItem(tab, context)).toBe(false);
      expect(shouldBlockSettingsRouteItem(tab, context)).toBe(false);
    },
  );

  it.each(ADMIN_ONLY_TABS)('$tab is not hidden before the security status resolves', ({ tab }) => {
    // Fail-open until resolved, or an admin watches half the sidebar vanish
    // and reappear on every load. Settings.tsx is what keeps the panel from
    // mounting during that window.
    const context = createContext({
      settingsCapabilitiesResolved: false,
      settingsCapabilities: null,
    });

    expect(shouldHideSettingsNavItem(tab, context)).toBe(false);
    expect(shouldBlockSettingsRouteItem(tab, context)).toBe(false);
  });

  it('leaves a reachable landing tab once every admin-only tab is blocked', () => {
    // The point of the gates is that the fallback resolves to something real.
    // If this ever empties out, a non-admin gets a settings shell with no
    // panel at all, which is a worse failure than the one being fixed.
    const withheld = createContext({
      settingsCapabilitiesResolved: true,
      settingsCapabilities: {},
      hasFeature: () => false,
    });

    const reachable = SETTINGS_NAV_GROUPS.flatMap((group) => group.items)
      .filter((item) => !shouldBlockSettingsRouteItem(item.id, withheld))
      .map((item) => item.id);

    expect(reachable).toContain('system-general');
    expect(reachable).not.toContain('monitoring-availability');
    expect(reachable).not.toContain('support-diagnostics');
  });
});
