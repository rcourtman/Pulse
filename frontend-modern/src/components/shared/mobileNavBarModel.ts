import type { Component } from 'solid-js';

export type MobileNavBarIcon = Component<{ class?: string }>;

export type MobileNavBarPrimaryTab = {
  id: string;
  label: string;
  route: string;
  settingsRoute: string;
  tooltip: string;
  enabled: boolean;
  live: boolean;
  icon: MobileNavBarIcon;
  alwaysShow: boolean;
  badge?: string;
};

export type MobileNavBarUtilityTab = {
  id: 'alerts' | 'ai' | 'actions' | 'settings';
  label: string;
  route: string;
  tooltip: string;
  badge: 'update' | 'pro' | null;
  count: number | undefined;
  countLabel?: string;
  breakdown: { warning: number; critical: number } | undefined;
  icon: MobileNavBarIcon;
};

export type MobileNavBarProps = {
  activeTab: () => string | null;
  primaryTabs: () => MobileNavBarPrimaryTab[];
  utilityTabs: () => MobileNavBarUtilityTab[];
  getPrimaryHref?: (tab: MobileNavBarPrimaryTab) => string;
  onPrimaryClick: (tab: MobileNavBarPrimaryTab) => void;
  onUtilityClick: (tab: MobileNavBarUtilityTab) => void;
};

export type MobileNavBarPrimaryDestination = { kind: 'primary'; tab: MobileNavBarPrimaryTab };
export type MobileNavBarUtilityDestination = { kind: 'utility'; tab: MobileNavBarUtilityTab };
export type MobileNavBarDestination =
  MobileNavBarPrimaryDestination | MobileNavBarUtilityDestination;

export type MobileNavBarLayout = {
  platformDestinations: MobileNavBarPrimaryDestination[];
  fixedDestinations: MobileNavBarDestination[];
  overflowDestinations: MobileNavBarDestination[];
};

const MOBILE_NAV_PRIMARY_PRIORITY = [
  'home',
  'proxmox',
  'docker',
  'kubernetes',
  'truenas',
  'vmware',
  'standalone',
] as const;

const MOBILE_NAV_UTILITY_PRIORITY = ['alerts', 'ai', 'actions', 'settings'] as const;
const MOBILE_NAV_FIXED_UTILITY_IDS = new Set<MobileNavBarUtilityTab['id']>([
  'alerts',
  'ai',
  'actions',
]);

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

export function buildOrderedMobileNavPrimaryTabs(
  tabs: MobileNavBarPrimaryTab[],
): MobileNavBarPrimaryTab[] {
  return buildOrderedMobileNavTabs(tabs, MOBILE_NAV_PRIMARY_PRIORITY);
}

export function buildOrderedMobileNavUtilityTabs(
  tabs: MobileNavBarUtilityTab[],
): MobileNavBarUtilityTab[] {
  return buildOrderedMobileNavTabs(tabs, MOBILE_NAV_UTILITY_PRIORITY);
}

/**
 * Keep the bottom bar to five predictable targets at most: a dedicated
 * platform switcher, the three daily operations destinations, and More for
 * secondary utilities. Platform choice is primary navigation and must never
 * be buried in the generic overflow menu.
 */
export function buildMobileNavBarLayout(
  primaryTabs: MobileNavBarPrimaryTab[],
  utilityTabs: MobileNavBarUtilityTab[],
): MobileNavBarLayout {
  const orderedPrimaryTabs = buildOrderedMobileNavPrimaryTabs(primaryTabs);
  const orderedUtilityTabs = buildOrderedMobileNavUtilityTabs(utilityTabs);
  const fixedUtilityTabs = orderedUtilityTabs.filter((tab) =>
    MOBILE_NAV_FIXED_UTILITY_IDS.has(tab.id),
  );

  return {
    platformDestinations: orderedPrimaryTabs.map((tab): MobileNavBarPrimaryDestination => ({
      kind: 'primary',
      tab,
    })),
    fixedDestinations: fixedUtilityTabs.map((tab): MobileNavBarDestination => ({
      kind: 'utility',
      tab,
    })),
    overflowDestinations: orderedUtilityTabs
      .filter((tab) => !MOBILE_NAV_FIXED_UTILITY_IDS.has(tab.id))
      .map((tab): MobileNavBarDestination => ({ kind: 'utility', tab })),
  };
}

export function getMobileNavDestinationKey(destination: MobileNavBarDestination): string {
  return `${destination.kind}:${destination.tab.id}`;
}

export function getMobileNavDestinationHref(
  destination: MobileNavBarDestination,
  getPrimaryHref?: (tab: MobileNavBarPrimaryTab) => string,
): string {
  if (destination.kind === 'utility') return destination.tab.route;
  if (getPrimaryHref) return getPrimaryHref(destination.tab);
  return destination.tab.enabled ? destination.tab.route : destination.tab.settingsRoute;
}

export function isMobileNavDestinationActive(
  destination: MobileNavBarDestination,
  activeTab: string | null,
): boolean {
  return destination.tab.id === activeTab;
}

export function mobileNavDestinationHasBadge(destination: MobileNavBarDestination): boolean {
  if (destination.kind === 'primary') {
    return Boolean(destination.tab.badge || !destination.tab.enabled);
  }
  return Boolean(destination.tab.badge || (destination.tab.count && destination.tab.count > 0));
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

export function getMobileNavTabAriaLabel(tab: MobileNavBarUtilityTab): string {
  const badges = getMobileNavAlertBadgeCounts(tab);
  if (!badges) {
    if (tab.count && tab.count > 0) {
      return `${tab.label}: ${tab.countLabel ?? `${tab.count} items`}`;
    }
    return tab.label;
  }
  const parts: string[] = [];
  if (badges.critical > 0) {
    parts.push(`${badges.critical} critical`);
  }
  if (badges.warning > 0) {
    parts.push(`${badges.warning} warning`);
  }
  if (parts.length === 0) return tab.label;
  return `${tab.label}: ${parts.join(', ')}`;
}

export function getMobileNavTabButtonClass(options: {
  active: boolean;
  enabled?: boolean;
}): string {
  return `relative flex min-h-10 min-w-0 flex-1 select-none flex-col items-center justify-center gap-0 rounded-md px-1 py-0.5 text-[9px] font-medium transition-colors ${
    options.active ? 'bg-blue-50 text-blue-700 dark:bg-blue-900 dark:text-blue-300' : 'text-muted'
  } ${options.enabled === false ? 'opacity-70' : ''}`.trim();
}
