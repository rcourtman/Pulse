import { describe, expect, it } from 'vitest';
import { createRoot, createSignal } from 'solid-js';
import { createStableTabList, primaryNavTabEquals, utilityNavTabEquals } from '../stableNavTabs';
import type { MobileNavBarIcon, MobileNavBarUtilityTab } from '../mobileNavBarModel';

const Icon: MobileNavBarIcon = () => null;

const utilityTab = (overrides: Partial<MobileNavBarUtilityTab> = {}): MobileNavBarUtilityTab => ({
  id: 'alerts',
  label: 'Alerts',
  route: '/alerts',
  tooltip: 'Review active alerts',
  badge: null,
  count: 3,
  countLabel: undefined,
  breakdown: { warning: 2, critical: 1 },
  icon: Icon,
  ...overrides,
});

describe('createStableTabList', () => {
  it('returns the identical array when a rebuilt list is structurally equal', () => {
    createRoot((dispose) => {
      const [tabs, setTabs] = createSignal([utilityTab()], { equals: false });
      const stable = createStableTabList(tabs, utilityNavTabEquals);
      const first = stable();
      setTabs([utilityTab()]);
      expect(stable()).toBe(first);
      expect(stable()[0]).toBe(first[0]);
      dispose();
    });
  });

  it('reuses unchanged item references when one item changes', () => {
    createRoot((dispose) => {
      const settingsTab = utilityTab({ id: 'settings', label: 'Settings', route: '/settings' });
      const [tabs, setTabs] = createSignal([utilityTab(), settingsTab], { equals: false });
      const stable = createStableTabList(tabs, utilityNavTabEquals);
      const first = stable();
      setTabs([
        utilityTab({ count: 4, breakdown: { warning: 3, critical: 1 } }),
        utilityTab({ id: 'settings', label: 'Settings', route: '/settings' }),
      ]);
      const second = stable();
      expect(second).not.toBe(first);
      expect(second[0]).not.toBe(first[0]);
      expect(second[1]).toBe(first[1]);
      dispose();
    });
  });

  it('detects length changes', () => {
    createRoot((dispose) => {
      const [tabs, setTabs] = createSignal([utilityTab()], { equals: false });
      const stable = createStableTabList(tabs, utilityNavTabEquals);
      const first = stable();
      setTabs([utilityTab(), utilityTab({ id: 'settings', route: '/settings' })]);
      const second = stable();
      expect(second).not.toBe(first);
      expect(second[0]).toBe(first[0]);
      expect(second).toHaveLength(2);
      dispose();
    });
  });
});

describe('utilityNavTabEquals', () => {
  it('compares breakdown by value', () => {
    expect(utilityNavTabEquals(utilityTab(), utilityTab())).toBe(true);
    expect(
      utilityNavTabEquals(utilityTab(), utilityTab({ breakdown: { warning: 9, critical: 1 } })),
    ).toBe(false);
    expect(utilityNavTabEquals(utilityTab({ breakdown: undefined }), utilityTab())).toBe(false);
    expect(
      utilityNavTabEquals(
        utilityTab({ breakdown: undefined }),
        utilityTab({ breakdown: undefined }),
      ),
    ).toBe(true);
  });

  it('compares scalar fields', () => {
    expect(utilityNavTabEquals(utilityTab(), utilityTab({ count: undefined }))).toBe(false);
    expect(utilityNavTabEquals(utilityTab(), utilityTab({ badge: 'update' }))).toBe(false);
  });
});

describe('primaryNavTabEquals', () => {
  it('compares all scalar fields and the icon reference', () => {
    const base = {
      id: 'proxmox',
      label: 'Proxmox',
      route: '/proxmox',
      settingsRoute: '/settings/infrastructure',
      tooltip: 'Proxmox',
      enabled: true,
      live: true,
      icon: Icon,
      alwaysShow: false,
      badge: undefined,
    };
    expect(primaryNavTabEquals(base, { ...base })).toBe(true);
    expect(primaryNavTabEquals(base, { ...base, live: false })).toBe(false);
    const OtherIcon: MobileNavBarIcon = () => null;
    expect(primaryNavTabEquals(base, { ...base, icon: OtherIcon })).toBe(false);
  });
});
