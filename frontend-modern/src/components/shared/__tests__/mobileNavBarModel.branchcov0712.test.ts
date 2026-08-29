import { describe, expect, it } from 'vitest';
import {
  buildMobileNavBarLayout,
  buildOrderedMobileNavTabs,
  getMobileNavAlertBadgeCounts,
  getMobileNavDestinationHref,
  getMobileNavTabAriaLabel,
  getMobileNavTabButtonClass,
} from '@/components/shared/mobileNavBarModel';
import type {
  MobileNavBarIcon,
  MobileNavBarPrimaryTab,
  MobileNavBarUtilityTab,
} from '@/components/shared/mobileNavBarModel';

// The icon is never rendered by these model-only tests; a cast stub satisfies
// the Component type without pulling solid-js into a plain .ts file.
const noopIcon = (() => null) as unknown as MobileNavBarIcon;

// Defaults are tuned to the alerts-scoped functions so their tests stay minimal;
// non-alerts tests override `id` explicitly.
function makeUtilityTab(overrides: Partial<MobileNavBarUtilityTab> = {}): MobileNavBarUtilityTab {
  return {
    id: 'alerts',
    label: 'Alerts',
    route: '/alerts',
    tooltip: 'Alerts',
    badge: null,
    count: undefined,
    breakdown: undefined,
    icon: noopIcon,
    ...overrides,
  };
}

function makePrimaryTab(
  id: string,
  overrides: Partial<MobileNavBarPrimaryTab> = {},
): MobileNavBarPrimaryTab {
  return {
    id,
    label: id,
    route: `/${id}`,
    settingsRoute: '/settings/infrastructure',
    tooltip: id,
    enabled: true,
    live: true,
    icon: noopIcon,
    alwaysShow: true,
    ...overrides,
  };
}

describe('mobileNavBarModel.branchcov2', () => {
  describe('getMobileNavAlertBadgeCounts', () => {
    it('returns null for a non-alerts tab via the id guard (true arm)', () => {
      const tab = makeUtilityTab({ id: 'settings', count: 9 });
      expect(getMobileNavAlertBadgeCounts(tab)).toBeNull();
    });

    it('returns null when count is undefined (!tab.count falsy arm)', () => {
      expect(getMobileNavAlertBadgeCounts(makeUtilityTab({ count: undefined }))).toBeNull();
    });

    it('returns null when count is 0 (!tab.count falsy arm, zero is falsy)', () => {
      expect(getMobileNavAlertBadgeCounts(makeUtilityTab({ count: 0 }))).toBeNull();
    });

    it('returns null when count is negative (tab.count <= 0 arm)', () => {
      expect(getMobileNavAlertBadgeCounts(makeUtilityTab({ count: -4 }))).toBeNull();
    });

    it('returns zeros when breakdown is undefined (?. ?? 0 right arm for both fields)', () => {
      const tab = makeUtilityTab({ count: 7, breakdown: undefined });
      expect(getMobileNavAlertBadgeCounts(tab)).toStrictEqual({ critical: 0, warning: 0 });
    });

    it('returns the breakdown counts when both severities are defined (?. left arm)', () => {
      const tab = makeUtilityTab({ count: 9, breakdown: { critical: 4, warning: 5 } });
      expect(getMobileNavAlertBadgeCounts(tab)).toStrictEqual({ critical: 4, warning: 5 });
    });

    it('preserves an explicit zero critical via ?? (proves ?? is not ||)', () => {
      const tab = makeUtilityTab({ count: 9, breakdown: { critical: 0, warning: 3 } });
      expect(getMobileNavAlertBadgeCounts(tab)).toStrictEqual({ critical: 0, warning: 3 });
    });

    it('preserves an explicit zero warning via ?? (proves ?? is not ||)', () => {
      const tab = makeUtilityTab({ count: 9, breakdown: { critical: 2, warning: 0 } });
      expect(getMobileNavAlertBadgeCounts(tab)).toStrictEqual({ critical: 2, warning: 0 });
    });

    it('falls back to 0 when breakdown.critical is undefined despite breakdown being present', () => {
      const tab = makeUtilityTab({
        count: 5,
        breakdown: {
          critical: undefined,
          warning: 2,
        } as unknown as { critical: number; warning: number },
      });
      expect(getMobileNavAlertBadgeCounts(tab)).toStrictEqual({ critical: 0, warning: 2 });
    });
  });

  describe('getMobileNavTabAriaLabel', () => {
    it('returns the bare label for a non-alerts tab with no count (badges null + count falsy)', () => {
      const tab = makeUtilityTab({ id: 'settings', label: 'Settings', count: undefined });
      expect(getMobileNavTabAriaLabel(tab)).toBe('Settings');
    });

    it('returns the bare label when count is 0 (count && count > 0 false arm)', () => {
      const tab = makeUtilityTab({ id: 'settings', label: 'Settings', count: 0 });
      expect(getMobileNavTabAriaLabel(tab)).toBe('Settings');
    });

    it('uses countLabel when count > 0 (countLabel ?? right-side fallback NOT taken)', () => {
      const tab = makeUtilityTab({
        id: 'ai',
        label: 'Patrol',
        count: 12,
        countLabel: '12 pending',
      });
      expect(getMobileNavTabAriaLabel(tab)).toBe('Patrol: 12 pending');
    });

    it('synthesizes "N items" when countLabel is absent (countLabel ?? right arm)', () => {
      const tab = makeUtilityTab({
        id: 'ai',
        label: 'Patrol',
        count: 12,
        countLabel: undefined,
      });
      expect(getMobileNavTabAriaLabel(tab)).toBe('Patrol: 12 items');
    });

    it('returns the bare label for an alerts tab whose breakdown is all zeros (parts.length === 0 arm)', () => {
      const tab = makeUtilityTab({
        id: 'alerts',
        label: 'Alerts',
        count: 5,
        breakdown: { critical: 0, warning: 0 },
      });
      expect(getMobileNavTabAriaLabel(tab)).toBe('Alerts');
    });

    it('lists only critical when warning is 0 (warning push guard false)', () => {
      const tab = makeUtilityTab({
        id: 'alerts',
        label: 'Alerts',
        count: 4,
        breakdown: { critical: 3, warning: 0 },
      });
      expect(getMobileNavTabAriaLabel(tab)).toBe('Alerts: 3 critical');
    });

    it('lists only warning when critical is 0 (critical push guard false)', () => {
      const tab = makeUtilityTab({
        id: 'alerts',
        label: 'Alerts',
        count: 4,
        breakdown: { critical: 0, warning: 2 },
      });
      expect(getMobileNavTabAriaLabel(tab)).toBe('Alerts: 2 warning');
    });

    it('joins both severities with a comma when both are > 0', () => {
      const tab = makeUtilityTab({
        id: 'alerts',
        label: 'Alerts',
        count: 10,
        breakdown: { critical: 3, warning: 7 },
      });
      expect(getMobileNavTabAriaLabel(tab)).toBe('Alerts: 3 critical, 7 warning');
    });
  });

  describe('buildOrderedMobileNavTabs', () => {
    it('returns an empty array when given no tabs', () => {
      expect(buildOrderedMobileNavTabs([], ['a', 'b'])).toStrictEqual([]);
    });

    it('reorders tabs to follow the priority list (priority id present -> push branch)', () => {
      const tabs = [{ id: 'b' }, { id: 'a' }, { id: 'c' }];
      expect(buildOrderedMobileNavTabs(tabs, ['a', 'b', 'c'])).toStrictEqual([
        { id: 'a' },
        { id: 'b' },
        { id: 'c' },
      ]);
    });

    it('skips priority ids absent from tabs (byId.get -> undefined -> if(tab) false arm)', () => {
      const tabs = [{ id: 'a' }];
      expect(buildOrderedMobileNavTabs(tabs, ['a', 'x', 'y'])).toStrictEqual([{ id: 'a' }]);
    });

    it('appends non-priority tabs after priority ones in original input order', () => {
      const tabs = [{ id: 'extra1' }, { id: 'a' }, { id: 'extra2' }];
      expect(buildOrderedMobileNavTabs(tabs, ['a'])).toStrictEqual([
        { id: 'a' },
        { id: 'extra1' },
        { id: 'extra2' },
      ]);
    });

    it('keeps all tabs in input order when none match the priority set', () => {
      const tabs = [{ id: 'x' }, { id: 'y' }];
      expect(buildOrderedMobileNavTabs(tabs, ['a', 'b'])).toStrictEqual([{ id: 'x' }, { id: 'y' }]);
    });

    it('preserves the full tab object (generic T) through reordering', () => {
      const tabs = [
        { id: 'kubernetes', kind: 'primary' },
        { id: 'docker', kind: 'primary' },
      ];
      expect(buildOrderedMobileNavTabs(tabs, ['docker', 'kubernetes'])).toStrictEqual([
        { id: 'docker', kind: 'primary' },
        { id: 'kubernetes', kind: 'primary' },
      ]);
    });
  });

  describe('buildMobileNavBarLayout', () => {
    it('keeps platforms in a dedicated switcher and daily operations fixed', () => {
      const layout = buildMobileNavBarLayout(
        [makePrimaryTab('standalone'), makePrimaryTab('docker'), makePrimaryTab('proxmox')],
        [
          makeUtilityTab({ id: 'settings' }),
          makeUtilityTab({ id: 'actions' }),
          makeUtilityTab({ id: 'ai' }),
          makeUtilityTab({ id: 'alerts' }),
        ],
      );

      expect(layout.platformDestinations.map((destination) => destination.tab.id)).toEqual([
        'proxmox',
        'docker',
        'standalone',
      ]);
      expect(layout.fixedDestinations.map((destination) => destination.tab.id)).toEqual([
        'alerts',
        'ai',
        'actions',
      ]);
      expect(layout.overflowDestinations.map((destination) => destination.tab.id)).toEqual([
        'settings',
      ]);
      expect(layout.overflowDestinations.map((destination) => destination.kind)).toEqual([
        'utility',
      ]);
    });

    it('does not invent an overflow destination when all supplied tabs are fixed', () => {
      const layout = buildMobileNavBarLayout(
        [makePrimaryTab('proxmox')],
        [makeUtilityTab({ id: 'alerts' }), makeUtilityTab({ id: 'ai' })],
      );

      expect(layout.platformDestinations.map((destination) => destination.tab.id)).toEqual([
        'proxmox',
      ]);
      expect(layout.fixedDestinations.map((destination) => destination.tab.id)).toEqual([
        'alerts',
        'ai',
      ]);
      expect(layout.overflowDestinations).toEqual([]);
    });
  });

  describe('getMobileNavDestinationHref', () => {
    it('uses the utility route directly', () => {
      expect(
        getMobileNavDestinationHref({ kind: 'utility', tab: makeUtilityTab({ route: '/alerts' }) }),
      ).toBe('/alerts');
    });

    it('uses the canonical route for an enabled primary destination by default', () => {
      expect(getMobileNavDestinationHref({ kind: 'primary', tab: makePrimaryTab('proxmox') })).toBe(
        '/proxmox',
      );
    });

    it('sends an unconfigured primary destination to infrastructure settings by default', () => {
      expect(
        getMobileNavDestinationHref({
          kind: 'primary',
          tab: makePrimaryTab('docker', { enabled: false }),
        }),
      ).toBe('/settings/infrastructure');
    });

    it('prefers the shell route resolver so remembered state is exposed in the href', () => {
      const tab = makePrimaryTab('proxmox');
      expect(
        getMobileNavDestinationHref(
          { kind: 'primary', tab },
          (primary) => `${primary.route}?status=running`,
        ),
      ).toBe('/proxmox?status=running');
    });
  });

  describe('getMobileNavTabButtonClass', () => {
    const BASE =
      'relative flex min-h-10 min-w-0 flex-1 select-none flex-col items-center justify-center gap-0 rounded-md px-1 py-0.5 text-[9px] font-medium transition-colors';
    const ACTIVE = 'bg-blue-50 text-blue-700 dark:bg-blue-900 dark:text-blue-300';
    const MUTED = 'text-muted';

    it('emits the active palette when active, with no opacity when enabled is omitted', () => {
      expect(getMobileNavTabButtonClass({ active: true })).toBe(`${BASE} ${ACTIVE}`);
    });

    it('emits text-muted when inactive (active ternary false arm)', () => {
      expect(getMobileNavTabButtonClass({ active: false })).toBe(`${BASE} ${MUTED}`);
    });

    it('appends opacity-70 when enabled is strictly false on an active tab', () => {
      expect(getMobileNavTabButtonClass({ active: true, enabled: false })).toBe(
        `${BASE} ${ACTIVE} opacity-70`,
      );
    });

    it('appends opacity-70 when enabled is strictly false on an inactive tab', () => {
      expect(getMobileNavTabButtonClass({ active: false, enabled: false })).toBe(
        `${BASE} ${MUTED} opacity-70`,
      );
    });

    it('does not append opacity-70 when enabled is true (strict === false check)', () => {
      expect(getMobileNavTabButtonClass({ active: true, enabled: true })).toBe(`${BASE} ${ACTIVE}`);
    });
  });
});
