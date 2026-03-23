import type { JSX } from 'solid-js';

export type MobileNavBarPlatformTab = {
  id: string;
  label: string;
  route: string;
  settingsRoute: string;
  tooltip: string;
  enabled: boolean;
  live: boolean;
  icon: JSX.Element;
  alwaysShow: boolean;
  badge?: string;
};

export type MobileNavBarUtilityTab = {
  id: 'alerts' | 'ai' | 'operations' | 'settings';
  label: string;
  route: string;
  tooltip: string;
  badge: 'update' | 'pro' | null;
  count: number | undefined;
  breakdown: { warning: number; critical: number } | undefined;
  icon: JSX.Element;
};

export type MobileNavBarProps = {
  activeTab: () => string;
  platformTabs: () => MobileNavBarPlatformTab[];
  utilityTabs: () => MobileNavBarUtilityTab[];
  onPlatformClick: (platform: MobileNavBarPlatformTab) => void;
  onUtilityClick: (tab: MobileNavBarUtilityTab) => void;
};

const MOBILE_NAV_PLATFORM_PRIORITY = [
  'dashboard',
  'infrastructure',
  'workloads',
  'storage',
  'recovery',
] as const;

const MOBILE_NAV_UTILITY_PRIORITY = ['alerts', 'ai', 'operations', 'settings'] as const;

export function buildOrderedMobileNavTabs<T extends { id: string }>(
  tabs: T[],
  priority: readonly string[],
): T[] {
  const prioritySet = new Set(priority);
  const byId = new Map(tabs.map((tab) => [tab.id, tab]));
  const ordered: T[] = [];

  priority.forEach((id) => {
    const tab = byId.get(id);
    if (tab) ordered.push(tab);
  });

  tabs.forEach((tab) => {
    if (!prioritySet.has(tab.id)) ordered.push(tab);
  });

  return ordered;
}

export function buildOrderedMobileNavPlatformTabs(
  tabs: MobileNavBarPlatformTab[],
): MobileNavBarPlatformTab[] {
  return buildOrderedMobileNavTabs(tabs, MOBILE_NAV_PLATFORM_PRIORITY);
}

export function buildOrderedMobileNavUtilityTabs(
  tabs: MobileNavBarUtilityTab[],
): MobileNavBarUtilityTab[] {
  return buildOrderedMobileNavTabs(tabs, MOBILE_NAV_UTILITY_PRIORITY);
}

export function getMobileNavAlertBadgeCounts(
  tab: MobileNavBarUtilityTab,
): { critical: number; warning: number } | null {
  if (tab.id !== 'alerts') return null;
  if (!tab.count || tab.count <= 0) return null;

  return {
    critical: tab.breakdown?.critical ?? 0,
    warning: tab.breakdown?.warning ?? 0,
  };
}

export function getMobileNavTabButtonClass(options: {
  active: boolean;
  enabled?: boolean;
}): string {
  return `relative flex min-h-10 shrink-0 flex-col items-center gap-1 rounded-md px-2 py-1.5 text-[11px] font-medium transition-colors ${
    options.active
      ? 'bg-blue-50 text-blue-700 dark:bg-blue-900 dark:text-blue-300'
      : 'text-muted'
  } ${options.enabled === false ? 'opacity-70' : ''}`.trim();
}

export function getMobileNavFadeState(element: HTMLDivElement | undefined): {
  showLeftFade: boolean;
  showRightFade: boolean;
} {
  if (!element) {
    return { showLeftFade: false, showRightFade: false };
  }

  const maxScrollLeft = element.scrollWidth - element.clientWidth;
  return {
    showLeftFade: element.scrollLeft > 1,
    showRightFade: maxScrollLeft > 1 && element.scrollLeft < maxScrollLeft - 1,
  };
}
