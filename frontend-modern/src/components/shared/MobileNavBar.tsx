import { For, Show } from 'solid-js';
import { type MobileNavBarProps, useMobileNavBarState } from './useMobileNavBarState';
import {
  getMobileNavAlertBadgeCounts,
  getMobileNavTabAriaLabel,
  getMobileNavTabButtonClass,
} from './mobileNavBarModel';

export type {
  MobileNavBarPlatformTab,
  MobileNavBarProps,
  MobileNavBarUtilityTab,
} from './mobileNavBarModel';

export function MobileNavBar(props: MobileNavBarProps) {
  const mobileNav = useMobileNavBarState(props);
  const tabIconClass = 'h-4 w-4 shrink-0';

  return (
    <>
      <nav class="fixed inset-x-0 bottom-0 z-40 border-t border-border bg-surface pb-safe lg:hidden">
        <div class="relative">
          <div
            ref={mobileNav.setNavRef}
            class="flex items-center gap-1 overflow-x-auto scrollbar-hide px-2 py-1.5 sm:gap-2 sm:overflow-x-visible sm:px-4 sm:justify-between"
            role="tablist"
            aria-label="Mobile navigation"
          >
            <For each={mobileNav.orderedPlatformTabs()}>
              {(platform) => {
                const Icon = platform.icon;

                return (
                  <button
                    type="button"
                    data-tab-id={platform.id}
                    onClick={() => mobileNav.handlePlatformClick(platform)}
                    title={platform.tooltip}
                    class={getMobileNavTabButtonClass({
                      active: props.activeTab() === platform.id,
                      enabled: platform.enabled,
                    })}
                  >
                    <span aria-hidden="true" class="relative flex items-center justify-center">
                      <Icon class={tabIconClass} />
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
                );
              }}
            </For>

            <For each={mobileNav.orderedUtilityTabs()}>
              {(tab) => {
                const alertBadges = () => getMobileNavAlertBadgeCounts(tab);
                const Icon = tab.icon;

                return (
                  <button
                    type="button"
                    data-tab-id={tab.id}
                    onClick={() => mobileNav.handleUtilityClick(tab)}
                    title={tab.tooltip}
                    aria-label={getMobileNavTabAriaLabel(tab)}
                    class={getMobileNavTabButtonClass({
                      active: props.activeTab() === tab.id,
                    })}
                  >
                    <span class="relative flex items-center justify-center">
                      <span aria-hidden="true" class="inline-flex items-center justify-center">
                        <Icon class={tabIconClass} />
                      </span>
                      <Show when={alertBadges()}>
                        {(badges) => (
                          <span aria-hidden="true" class="absolute -right-2 -top-1 flex items-center gap-1">
                            <Show when={badges().critical > 0}>
                              <span class="inline-flex h-4 min-w-[16px] items-center justify-center rounded-full bg-red-600 px-1 text-[10px] font-bold text-white">
                                {badges().critical}
                              </span>
                            </Show>
                            <Show when={badges().warning > 0}>
                              <span class="inline-flex h-4 min-w-[16px] items-center justify-center rounded-full bg-amber-200 px-1 text-[10px] font-semibold text-amber-900">
                                {badges().warning}
                              </span>
                            </Show>
                          </span>
                        )}
                      </Show>
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
                );
              }}
            </For>
          </div>

          <Show when={mobileNav.showLeftFade()}>
            <div class="pointer-events-none absolute inset-y-0 left-0 w-8 bg-gradient-to-r from-surface to-transparent"></div>
          </Show>
          <Show when={mobileNav.showFade()}>
            <div class="pointer-events-none absolute inset-y-0 right-0 w-8 bg-gradient-to-l from-surface to-transparent"></div>
          </Show>
        </div>
      </nav>
    </>
  );
}

export default MobileNavBar;
