import { describe, expect, it } from 'vitest';
import {
  buildOrderedMobileNavTabs,
  getMobileNavAlertBadgeCounts,
  getMobileNavFadeState,
  getMobileNavTabAriaLabel,
  getMobileNavTabButtonClass,
} from '@/components/shared/mobileNavBarModel';
import type {
  MobileNavBarIcon,
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
        id: 'actions',
        label: 'Actions',
        count: 12,
        countLabel: '12 pending',
      });
      expect(getMobileNavTabAriaLabel(tab)).toBe('Actions: 12 pending');
    });

    it('synthesizes "N items" when countLabel is absent (countLabel ?? right arm)', () => {
      const tab = makeUtilityTab({
        id: 'actions',
        label: 'Actions',
        count: 12,
        countLabel: undefined,
      });
      expect(getMobileNavTabAriaLabel(tab)).toBe('Actions: 12 items');
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

  describe('getMobileNavTabButtonClass', () => {
    const BASE =
      'relative flex min-h-10 shrink-0 flex-col items-center gap-1 rounded-md px-2 py-1.5 text-[11px] font-medium transition-colors';
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

  describe('getMobileNavFadeState', () => {
    // Helper: jsdom reports 0 for all scroll geometry by default, so define the
    // readonly scroll metrics as configurable own properties on the instance.
    function makeScrollEl(props: {
      scrollWidth: number;
      clientWidth: number;
      scrollLeft: number;
    }): HTMLDivElement {
      const el = document.createElement('div');
      Object.defineProperty(el, 'scrollWidth', { configurable: true, value: props.scrollWidth });
      Object.defineProperty(el, 'clientWidth', { configurable: true, value: props.clientWidth });
      Object.defineProperty(el, 'scrollLeft', { configurable: true, value: props.scrollLeft });
      return el;
    }

    it('returns both fades false when the element is undefined (early-return guard)', () => {
      expect(getMobileNavFadeState(undefined)).toStrictEqual({
        showLeftFade: false,
        showRightFade: false,
      });
    });

    it('returns both fades false when there is no scrollable overflow (maxScrollLeft <= 1)', () => {
      const el = makeScrollEl({ scrollWidth: 100, clientWidth: 100, scrollLeft: 0 });
      expect(getMobileNavFadeState(el)).toStrictEqual({
        showLeftFade: false,
        showRightFade: false,
      });
    });

    it('shows only the right fade at the left edge (scrollLeft > 1 false)', () => {
      const el = makeScrollEl({ scrollWidth: 200, clientWidth: 100, scrollLeft: 0 });
      expect(getMobileNavFadeState(el)).toStrictEqual({
        showLeftFade: false,
        showRightFade: true,
      });
    });

    it('treats scrollLeft === 1 as not past the left threshold (> 1 strict boundary)', () => {
      const el = makeScrollEl({ scrollWidth: 200, clientWidth: 100, scrollLeft: 1 });
      expect(getMobileNavFadeState(el)).toStrictEqual({
        showLeftFade: false,
        showRightFade: true,
      });
    });

    it('shows both fades in the middle of the scroll range', () => {
      const el = makeScrollEl({ scrollWidth: 200, clientWidth: 100, scrollLeft: 50 });
      expect(getMobileNavFadeState(el)).toStrictEqual({
        showLeftFade: true,
        showRightFade: true,
      });
    });

    it('shows only the left fade at the far-right edge (scrollLeft < maxScrollLeft - 1 false)', () => {
      const el = makeScrollEl({ scrollWidth: 200, clientWidth: 100, scrollLeft: 100 });
      expect(getMobileNavFadeState(el)).toStrictEqual({
        showLeftFade: true,
        showRightFade: false,
      });
    });

    it('treats scrollLeft === maxScrollLeft - 1 as already at the right edge (< strict boundary)', () => {
      const el = makeScrollEl({ scrollWidth: 200, clientWidth: 100, scrollLeft: 99 });
      expect(getMobileNavFadeState(el)).toStrictEqual({
        showLeftFade: true,
        showRightFade: false,
      });
    });
  });
});
