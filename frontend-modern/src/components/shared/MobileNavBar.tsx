import { createEffect, createMemo, createSignal, For, Show, onCleanup } from 'solid-js';
import type { JSX } from 'solid-js';

type PlatformTab = {
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

type UtilityTab = {
  id: 'alerts' | 'ai' | 'settings';
  label: string;
  route: string;
  tooltip: string;
  badge: 'update' | 'pro' | null;
  count: number | undefined;
  breakdown: { warning: number; critical: number } | undefined;
  icon: JSX.Element;
};

type MobileNavBarProps = {
  activeTab: () => string;
  platformTabs: () => PlatformTab[];
  utilityTabs: () => UtilityTab[];
  onPlatformClick: (platform: PlatformTab) => void;
  onUtilityClick: (tab: UtilityTab) => void;
};

export function MobileNavBar(props: MobileNavBarProps) {
  let navRef: HTMLDivElement | undefined;
  const [showFade, setShowFade] = createSignal(false);
  const [showLeftFade, setShowLeftFade] = createSignal(false);

  const orderedPlatformTabs = createMemo(() => {
    const tabs = props.platformTabs();
    const priority = [
      'dashboard',
      'infrastructure',
      'workloads',
      'storage',
      'recovery',
    ];
    const prioritySet = new Set(priority);
    const byId = new Map(tabs.map((tab) => [tab.id, tab]));
    const ordered: PlatformTab[] = [];
    priority.forEach((id) => {
      const tab = byId.get(id);
      if (tab) ordered.push(tab);
    });
    tabs.forEach((tab) => {
      if (!prioritySet.has(tab.id)) ordered.push(tab);
    });
    return ordered;
  });

  const orderedUtilityTabs = createMemo(() => {
    const tabs = props.utilityTabs();
    const priority = ['alerts', 'settings', 'ai'];
    const prioritySet = new Set(priority);
    const byId = new Map(tabs.map((tab) => [tab.id, tab]));
    const ordered: UtilityTab[] = [];
    priority.forEach((id) => {
      const tab = byId.get(id as UtilityTab['id']);
      if (tab) ordered.push(tab);
    });
    tabs.forEach((tab) => {
      if (!prioritySet.has(tab.id)) ordered.push(tab);
    });
    return ordered;
  });

  const updateFadeIndicator = () => {
    if (!navRef) {
      setShowFade(false);
      return;
    }
    const maxScrollLeft = navRef.scrollWidth - navRef.clientWidth;
    setShowFade(maxScrollLeft > 1 && navRef.scrollLeft < maxScrollLeft - 1);
    setShowLeftFade(navRef.scrollLeft > 1);
  };

  createEffect(() => {
    if (!navRef) return;
    updateFadeIndicator();
    const handleScroll = () => updateFadeIndicator();
    navRef.addEventListener('scroll', handleScroll, { passive: true });
    window.addEventListener('resize', handleScroll);
    onCleanup(() => {
      navRef?.removeEventListener('scroll', handleScroll);
      window.removeEventListener('resize', handleScroll);
    });
  });

  createEffect(() => {
    if (!navRef) return;
    const activeId = props.activeTab();
    if (!activeId) return;
    const activeEl = navRef.querySelector<HTMLElement>(`[data-tab-id="${activeId}"]`);
    if (!activeEl) return;
    requestAnimationFrame(() => {
      activeEl.scrollIntoView({ behavior: 'smooth', block: 'nearest', inline: 'center' });
      updateFadeIndicator();
    });
  });

  const handlePlatformClick = (platform: PlatformTab) => {
    props.onPlatformClick(platform);
  };

  const handleUtilityClick = (tab: UtilityTab) => {
    props.onUtilityClick(tab);
  };

  const renderAlertsBadge = (tab: UtilityTab) => {
    if (tab.id !== 'alerts') return null;
    if (!tab || !tab.count || tab.count <= 0) return null;
    const critical = tab.breakdown?.critical ?? 0;
    const warning = tab.breakdown?.warning ?? 0;
    return (
      <span class="absolute -right-2 -top-1 flex items-center gap-1">
        {critical > 0 && (
          <span class="inline-flex h-4 min-w-[16px] items-center justify-center rounded-full bg-red-600 px-1 text-[10px] font-bold text-white">
            {critical}
          </span>
        )}
        {warning > 0 && (
          <span class="inline-flex h-4 min-w-[16px] items-center justify-center rounded-full bg-amber-200 px-1 text-[10px] font-semibold text-amber-900">
            {warning}
          </span>
        )}
      </span>
    );
  };

  return (
    <>
      {/* Bottom navigation bar */}
      <nav
        class="fixed inset-x-0 bottom-0 z-40 border-t border-border bg-surface md:hidden pb-safe"
      >
        <div class="relative">
          <div
            ref={(el) => (navRef = el)}
            class="flex items-center gap-1 overflow-x-auto scrollbar-hide px-2 py-1.5"
            role="tablist"
            aria-label="Mobile navigation"
          >
            <For each={orderedPlatformTabs()}>
              {(platform) => (
                <button
                  type="button"
                  data-tab-id={platform.id}
                  onClick={() => handlePlatformClick(platform)}
                  title={platform.tooltip}
                  class={`relative flex min-h-10 shrink-0 flex-col items-center gap-1 rounded-md px-2 py-1.5 text-[11px] font-medium transition-colors ${props.activeTab() === platform.id
                    ? 'bg-blue-50 text-blue-700 dark:bg-blue-900 dark:text-blue-300'
                    : 'text-muted'
                    } ${platform.enabled ? '' : 'opacity-70'}`}
                >
                  <span class="relative flex items-center justify-center">
                    {platform.icon}
                  </span>
                  <span class="whitespace-nowrap">{platform.label}</span>
                  <Show when={!platform.enabled}>
                    <span class="rounded-full bg-amber-100 px-1.5 py-0.5 text-[9px] font-semibold text-amber-700 dark:bg-amber-900 dark:text-amber-200">
                      Setup
                    </span>
                  </Show>
                  <Show when={platform.badge}>
                    <span class="rounded-full bg-surface-hover px-1.5 py-0.5 text-[9px] font-semibold text-muted">
                      {platform.badge}
                    </span>
                  </Show>
                </button>
              )}
            </For>

            <For each={orderedUtilityTabs()}>
              {(tab) => (
                <button
                  type="button"
                  data-tab-id={tab.id}
                  onClick={() => handleUtilityClick(tab)}
                  title={tab.tooltip}
                  class={`relative flex min-h-10 shrink-0 flex-col items-center gap-1 rounded-md px-2 py-1.5 text-[11px] font-medium transition-colors ${props.activeTab() === tab.id
                    ? 'bg-blue-50 text-blue-700 dark:bg-blue-900 dark:text-blue-300'
                    : 'text-muted'
                    }`}
                >
                  <span class="relative flex items-center justify-center">
                    {tab.icon}
                    {renderAlertsBadge(tab)}
                  </span>
                  <span class="whitespace-nowrap">{tab.label}</span>
                  <Show when={tab.badge === 'update'}>
                    <span class="mt-0.5 h-1.5 w-1.5 rounded-full bg-red-500"></span>
                  </Show>
                  <Show when={tab.badge === 'pro'}>
                    <span class="rounded-full bg-blue-100 px-1.5 py-0.5 text-[9px] font-semibold text-blue-700 dark:bg-blue-900 dark:text-blue-300">
                      Pro
                    </span>
                  </Show>
                </button>
              )}
            </For>
          </div>

          <Show when={showLeftFade()}>
            <div class="pointer-events-none absolute inset-y-0 left-0 w-8 dark: dark:"></div>
          </Show>
          <Show when={showFade()}>
            <div class="pointer-events-none absolute inset-y-0 right-0 w-8 dark: dark:"></div>
          </Show>
        </div>
      </nav>
    </>
  );
}

export default MobileNavBar;
