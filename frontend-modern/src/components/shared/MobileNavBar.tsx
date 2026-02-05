import { createEffect, createMemo, createSignal, For, Show, onCleanup } from 'solid-js';
import type { JSX } from 'solid-js';
import ServerIcon from 'lucide-solid/icons/server';
import BoxesIcon from 'lucide-solid/icons/boxes';
import BellIcon from 'lucide-solid/icons/bell';
import SettingsIcon from 'lucide-solid/icons/settings';
import MoreHorizontalIcon from 'lucide-solid/icons/more-horizontal';
import XIcon from 'lucide-solid/icons/x';

type PlatformTab = {
  id: string;
  label: string;
  route: string;
  settingsRoute: string;
  tooltip: string;
  enabled: boolean;
  icon: JSX.Element;
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
  const [drawerOpen, setDrawerOpen] = createSignal(false);
  const [touchStartX, setTouchStartX] = createSignal<number | null>(null);
  const [touchStartY, setTouchStartY] = createSignal<number | null>(null);

  const alertsTab = createMemo(() => props.utilityTabs().find((tab) => tab.id === 'alerts'));
  const settingsTab = createMemo(() => props.utilityTabs().find((tab) => tab.id === 'settings'));
  const infrastructureTab = createMemo(() =>
    props.platformTabs().find((tab) => tab.id === 'infrastructure'),
  );
  const workloadsTab = createMemo(() =>
    props.platformTabs().find((tab) => tab.id === 'workloads'),
  );

  const morePlatformTabs = createMemo(() =>
    props.platformTabs().filter((tab) => !['infrastructure', 'workloads'].includes(tab.id)),
  );
  const moreUtilityTabs = createMemo(() =>
    props.utilityTabs().filter((tab) => !['alerts', 'settings'].includes(tab.id)),
  );

  createEffect(() => {
    if (!drawerOpen()) return;
    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'Escape') {
        setDrawerOpen(false);
      }
    };
    document.addEventListener('keydown', handleKeyDown);
    onCleanup(() => document.removeEventListener('keydown', handleKeyDown));
  });

  const handlePlatformClick = (platform: PlatformTab) => {
    props.onPlatformClick(platform);
    setDrawerOpen(false);
  };

  const handleUtilityClick = (tab: UtilityTab) => {
    props.onUtilityClick(tab);
    setDrawerOpen(false);
  };

  const handleTouchStart: JSX.EventHandlerUnion<HTMLDivElement, TouchEvent> = (event) => {
    const touch = event.touches[0];
    if (!touch) return;
    setTouchStartX(touch.clientX);
    setTouchStartY(touch.clientY);
  };

  const handleTouchEnd: JSX.EventHandlerUnion<HTMLDivElement, TouchEvent> = (event) => {
    const touch = event.changedTouches[0];
    if (!touch || touchStartX() === null || touchStartY() === null) {
      setTouchStartX(null);
      setTouchStartY(null);
      return;
    }
    const deltaX = touch.clientX - touchStartX()!;
    const deltaY = touch.clientY - touchStartY()!;
    if (deltaX > 60 && Math.abs(deltaY) < 40) {
      setDrawerOpen(false);
    }
    setTouchStartX(null);
    setTouchStartY(null);
  };

  const renderAlertsBadge = () => {
    const tab = alertsTab();
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
      <nav class="fixed inset-x-0 bottom-0 z-40 border-t border-gray-200 bg-white/95 backdrop-blur dark:border-gray-700 dark:bg-gray-900/95 md:hidden">
        <div class="flex items-center justify-around px-2 py-2">
          <button
            type="button"
            onClick={() => infrastructureTab() && handlePlatformClick(infrastructureTab()!)}
            class={`flex flex-1 flex-col items-center gap-1 rounded-lg px-2 py-1.5 text-[11px] font-medium transition-colors ${props.activeTab() === 'infrastructure'
              ? 'text-blue-600 dark:text-blue-400'
              : 'text-gray-500 dark:text-gray-400'
              }`}
          >
            <ServerIcon class="h-5 w-5" />
            <span>Infra</span>
          </button>

          <button
            type="button"
            onClick={() => workloadsTab() && handlePlatformClick(workloadsTab()!)}
            class={`flex flex-1 flex-col items-center gap-1 rounded-lg px-2 py-1.5 text-[11px] font-medium transition-colors ${props.activeTab() === 'workloads'
              ? 'text-blue-600 dark:text-blue-400'
              : 'text-gray-500 dark:text-gray-400'
              }`}
          >
            <BoxesIcon class="h-5 w-5" />
            <span>Workloads</span>
          </button>

          <button
            type="button"
            onClick={() => alertsTab() && handleUtilityClick(alertsTab()!)}
            class={`relative flex flex-1 flex-col items-center gap-1 rounded-lg px-2 py-1.5 text-[11px] font-medium transition-colors ${props.activeTab() === 'alerts'
              ? 'text-blue-600 dark:text-blue-400'
              : 'text-gray-500 dark:text-gray-400'
              }`}
          >
            <span class="relative">
              <BellIcon class="h-5 w-5" />
              {renderAlertsBadge()}
            </span>
            <span>Alerts</span>
          </button>

          <Show when={settingsTab()}>
            <button
              type="button"
              onClick={() => settingsTab() && handleUtilityClick(settingsTab()!)}
              class={`flex flex-1 flex-col items-center gap-1 rounded-lg px-2 py-1.5 text-[11px] font-medium transition-colors ${props.activeTab() === 'settings'
                ? 'text-blue-600 dark:text-blue-400'
                : 'text-gray-500 dark:text-gray-400'
                }`}
            >
              <SettingsIcon class="h-5 w-5" />
              <span>Settings</span>
            </button>
          </Show>

          <button
            type="button"
            onClick={() => setDrawerOpen((prev) => !prev)}
            class={`flex flex-1 flex-col items-center gap-1 rounded-lg px-2 py-1.5 text-[11px] font-medium transition-colors ${drawerOpen() || !['infrastructure', 'workloads', 'alerts', 'settings'].includes(props.activeTab())
              ? 'text-blue-600 dark:text-blue-400'
              : 'text-gray-500 dark:text-gray-400'
              }`}
          >
            <MoreHorizontalIcon class="h-5 w-5" />
            <span>More</span>
          </button>
        </div>
      </nav>

      {/* Drawer overlay */}
      <div
        class={`fixed inset-0 z-50 md:hidden ${drawerOpen() ? 'pointer-events-auto' : 'pointer-events-none'}`}
        aria-hidden={!drawerOpen()}
      >
        <div
          class={`absolute inset-0 bg-black/50 transition-opacity duration-200 ${drawerOpen() ? 'opacity-100' : 'opacity-0'}`}
          onClick={() => setDrawerOpen(false)}
        />
        <div
          class={`absolute inset-y-0 right-0 w-80 max-w-[90vw] bg-white shadow-xl transition-transform duration-200 dark:bg-gray-900 ${drawerOpen() ? 'translate-x-0' : 'translate-x-full'}`}
          role="dialog"
          aria-label="Mobile navigation menu"
          onTouchStart={handleTouchStart}
          onTouchEnd={handleTouchEnd}
        >
          <div class="flex items-center justify-between border-b border-gray-200 px-4 py-4 dark:border-gray-700">
            <div class="text-sm font-semibold text-gray-900 dark:text-gray-100">Menu</div>
            <button
              type="button"
              onClick={() => setDrawerOpen(false)}
              class="rounded-lg p-1.5 text-gray-400 transition-colors hover:bg-gray-100 hover:text-gray-600 dark:hover:bg-gray-800 dark:hover:text-gray-300"
              aria-label="Close menu"
            >
              <XIcon class="h-5 w-5" />
            </button>
          </div>

          <div class="h-full overflow-y-auto px-4 pb-20 pt-4">
            <Show when={morePlatformTabs().length > 0}>
              <div class="text-xs font-semibold uppercase tracking-wide text-gray-500 dark:text-gray-400">
                More Destinations
              </div>
              <div class="mt-3 space-y-2">
                <For each={morePlatformTabs()}>
                  {(platform) => (
                    <button
                      type="button"
                      onClick={() => handlePlatformClick(platform)}
                      title={platform.tooltip}
                      class={`flex w-full items-center justify-between rounded-lg border border-gray-200 px-3 py-2 text-sm font-medium transition-colors dark:border-gray-700 ${platform.enabled
                        ? 'text-gray-700 hover:bg-gray-100 dark:text-gray-200 dark:hover:bg-gray-800'
                        : 'text-gray-400 hover:bg-gray-100 dark:text-gray-500 dark:hover:bg-gray-800'
                        }`}
                    >
                      <span class="flex items-center gap-2">
                        {platform.icon}
                        <span>{platform.label}</span>
                      </span>
                      <span class="flex items-center gap-2">
                        <Show when={!platform.enabled}>
                          <span class="rounded-full bg-amber-100 px-2 py-0.5 text-[10px] font-semibold text-amber-700 dark:bg-amber-900/40 dark:text-amber-200">
                            Setup
                          </span>
                        </Show>
                        <Show when={platform.badge}>
                          <span class="rounded-full bg-gray-200 px-2 py-0.5 text-[10px] font-semibold text-gray-600 dark:bg-gray-700 dark:text-gray-300">
                            {platform.badge}
                          </span>
                        </Show>
                      </span>
                    </button>
                  )}
                </For>
              </div>
            </Show>

            <Show when={moreUtilityTabs().length > 0}>
              <div class="mt-6 text-xs font-semibold uppercase tracking-wide text-gray-500 dark:text-gray-400">
                System
              </div>
              <div class="mt-3 space-y-2">
                <For each={moreUtilityTabs()}>
                  {(tab) => (
                    <button
                      type="button"
                      onClick={() => handleUtilityClick(tab)}
                      title={tab.tooltip}
                      class="flex w-full items-center justify-between rounded-lg border border-gray-200 px-3 py-2 text-sm font-medium text-gray-700 transition-colors hover:bg-gray-100 dark:border-gray-700 dark:text-gray-200 dark:hover:bg-gray-800"
                    >
                      <span class="flex items-center gap-2">
                        {tab.icon}
                        <span>{tab.label}</span>
                      </span>
                      <span class="flex items-center gap-2">
                        <Show when={tab.id === 'alerts' && tab.count && tab.count > 0}>
                          <span class="rounded-full bg-red-600 px-2 py-0.5 text-[10px] font-semibold text-white">
                            {tab.count}
                          </span>
                        </Show>
                        <Show when={tab.badge === 'update'}>
                          <span class="h-2 w-2 rounded-full bg-red-500"></span>
                        </Show>
                        <Show when={tab.badge === 'pro'}>
                          <span class="rounded-full bg-blue-100 px-2 py-0.5 text-[10px] font-semibold text-blue-700 dark:bg-blue-900/40 dark:text-blue-300">
                            Pro
                          </span>
                        </Show>
                      </span>
                    </button>
                  )}
                </For>
              </div>
            </Show>
          </div>
        </div>
      </div>
    </>
  );
}

export default MobileNavBar;
