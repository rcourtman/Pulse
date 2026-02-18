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

  const orderedPlatformTabs = createMemo(() => {
    const tabs = props.platformTabs();
    const priority = [
      'infrastructure',
      'workloads',
      'storage',
      'storage-v2',
      'backups',
      'backups-v2',
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
        class="fixed inset-x-0 bottom-0 z-40 border-t border-gray-200 bg-white/95 backdrop-blur dark:border-gray-700 dark:bg-gray-900/95 md:hidden"
        style="padding-bottom: env(safe-area-inset-bottom, 0px)"
      >
        <div class="relative">
          <div
            ref={(el) => (navRef = el)}
            class="flex items-center gap-1 overflow-x-auto scrollbar-hide px-2 py-2"
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
                  class={`relative flex shrink-0 flex-col items-center gap-1 rounded-lg px-2 py-1 text-[10px] font-medium transition-colors ${props.activeTab() === platform.id
                    ? 'text-blue-600 dark:text-blue-400'
                    : 'text-gray-500 dark:text-gray-400'
                    } ${platform.enabled ? '' : 'opacity-70'}`}
                >
                  <span class="relative flex items-center justify-center">
                    {platform.icon}
                  </span>
                  <span class="whitespace-nowrap">{platform.label}</span>
                  <Show when={!platform.enabled}>
                    <span class="rounded-full bg-amber-100 px-1.5 py-0.5 text-[9px] font-semibold text-amber-700 dark:bg-amber-900/40 dark:text-amber-200">
                      Setup
                    </span>
                  </Show>
                  <Show when={platform.badge}>
                    <span class="rounded-full bg-gray-200 px-1.5 py-0.5 text-[9px] font-semibold text-gray-600 dark:bg-gray-700 dark:text-gray-300">
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
                  class={`relative flex shrink-0 flex-col items-center gap-1 rounded-lg px-2 py-1 text-[10px] font-medium transition-colors ${props.activeTab() === tab.id
                    ? 'text-blue-600 dark:text-blue-400'
                    : 'text-gray-500 dark:text-gray-400'
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
                    <span class="rounded-full bg-blue-100 px-1.5 py-0.5 text-[9px] font-semibold text-blue-700 dark:bg-blue-900/40 dark:text-blue-300">
                      Pro
                    </span>
                  </Show>
                </button>
              )}
            </For>
          </div>

          <Show when={showFade()}>
            <div class="pointer-events-none absolute inset-y-0 right-0 w-10 bg-gradient-to-l from-white via-white/80 to-transparent dark:from-gray-900/95 dark:via-gray-900/70"></div>
          </Show>
        </div>
      </nav>
    </>
  );
}

export default MobileNavBar;
