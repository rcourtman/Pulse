import { Accessor, Component, For, JSX, Setter, Show } from 'solid-js';
import ChevronRight from 'lucide-solid/icons/chevron-right';
import { Card } from '@/components/shared/Card';
import { PageHeader } from '@/components/shared/PageHeader';
import { SearchInput } from '@/components/shared/SearchInput';
import { getSettingsSearchEmptyState } from '@/utils/settingsShellPresentation';
import type { SettingsHeaderMeta, SettingsNavGroup, SettingsTab } from './settingsTypes';

interface SettingsPageShellProps {
  headerMeta: Accessor<SettingsHeaderMeta>;
  hasUnsavedChanges: Accessor<boolean>;
  activeTabSaveBehavior: Accessor<'system' | undefined>;
  saveSettings: () => void;
  discardChanges: () => void;
  isMobileMenuOpen: Accessor<boolean>;
  setIsMobileMenuOpen: Setter<boolean>;
  sidebarCollapsed: Accessor<boolean>;
  setSidebarCollapsed: Setter<boolean>;
  searchQuery: Accessor<string>;
  setSearchQuery: Setter<string>;
  filteredTabGroups: Accessor<SettingsNavGroup[]>;
  flatTabs: Accessor<SettingsNavGroup['items']>;
  activeTab: Accessor<SettingsTab>;
  setActiveTab: (tab: SettingsTab) => void;
  isPro: Accessor<boolean>;
  children: JSX.Element;
}

export const SettingsPageShell: Component<SettingsPageShellProps> = (props) => {
  return (
    <div class="space-y-6">
      <div class="px-1">
        <PageHeader title={props.headerMeta().title} description={props.headerMeta().description} />
      </div>

      <Show when={props.hasUnsavedChanges() && props.activeTabSaveBehavior() === 'system'}>
        <div class="bg-amber-50 dark:bg-amber-900 border-l-4 border-amber-500 dark:border-amber-400 rounded-r-lg shadow-sm p-4">
          <div class="flex flex-col sm:flex-row items-start sm:items-center justify-between gap-4">
            <div class="flex items-start gap-3">
              <svg
                class="w-5 h-5 text-amber-600 dark:text-amber-400 flex-shrink-0 mt-0.5"
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
                stroke-width="2"
              >
                <path
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z"
                />
              </svg>
              <div>
                <p class="font-semibold text-amber-900 dark:text-amber-100">Unsaved changes</p>
                <p class="text-sm text-amber-700 dark:text-amber-200 mt-0.5">
                  Your changes will be lost if you navigate away
                </p>
              </div>
            </div>
            <div class="flex w-full sm:w-auto gap-3">
              <button
                type="button"
                class="flex-1 sm:flex-initial px-5 py-2.5 text-sm font-medium bg-amber-600 text-white rounded-md hover:bg-amber-700 shadow-sm transition-colors"
                onClick={props.saveSettings}
              >
                Save Changes
              </button>
              <button
                type="button"
                class="px-4 py-2.5 text-sm font-medium text-amber-700 dark:text-amber-200 hover:underline transition-colors"
                onClick={props.discardChanges}
              >
                Discard
              </button>
            </div>
          </div>
        </div>
      </Show>

      <Card padding="none" class="relative flex lg:flex-row overflow-hidden min-h-[600px]">
        <div
          class={`${props.isMobileMenuOpen() ? 'flex flex-col w-full' : 'hidden lg:flex lg:flex-col'} ${props.sidebarCollapsed() ? 'lg:w-16 lg:min-w-[4rem] lg:max-w-[4rem] lg:basis-[4rem]' : 'lg:w-72 lg:min-w-[18rem] lg:max-w-[18rem] lg:basis-[18rem]'} relative border-b border-border lg:border-b-0 lg:border-r lg:align-top flex-shrink-0 transition-all duration-200 bg-surface lg:bg-transparent z-10`}
          aria-label="Settings navigation"
          aria-expanded={!props.sidebarCollapsed()}
        >
          <div
            class={`sticky top-0 ${props.sidebarCollapsed() ? 'px-2' : 'px-4'} py-5 space-y-5 transition-all duration-200`}
          >
            <Show when={!props.sidebarCollapsed()}>
              <div class="flex items-center justify-between pb-2 border-b border-border">
                <h2 class="text-sm font-semibold text-base-content">Settings</h2>
                <button
                  type="button"
                  onClick={() => props.setSidebarCollapsed(true)}
                  class="p-1 rounded-md hover:bg-surface-hover transition-colors"
                  aria-label="Collapse sidebar"
                >
                  <svg class="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path
                      stroke-linecap="round"
                      stroke-linejoin="round"
                      stroke-width="2"
                      d="M11 19l-7-7 7-7m8 14l-7-7 7-7"
                    />
                  </svg>
                </button>
              </div>
            </Show>
            <Show when={props.sidebarCollapsed()}>
              <button
                type="button"
                onClick={() => props.setSidebarCollapsed(false)}
                class="w-full p-2 rounded-md hover:bg-surface-hover transition-colors"
                aria-label="Expand sidebar"
              >
                <svg class="w-5 h-5 mx-auto" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path
                    stroke-linecap="round"
                    stroke-linejoin="round"
                    stroke-width="2"
                    d="M13 5l7 7-7 7M5 5l7 7-7 7"
                  />
                </svg>
              </button>
            </Show>
            <div id="settings-sidebar-menu" class="space-y-4">
              <Show when={!props.sidebarCollapsed()}>
                <div class="px-2 pb-2">
                  <SearchInput
                    value={props.searchQuery}
                    onChange={props.setSearchQuery}
                    placeholder="Search settings..."
                    class="w-full"
                    captureBackspace
                    clearOnEscape
                    shortcutHint="Any key"
                  />
                </div>
              </Show>

              <Show
                when={
                  props.searchQuery().trim().length > 0 && props.filteredTabGroups().length === 0
                }
              >
                <div class="py-4 px-4 text-center text-sm text-muted">
                  {getSettingsSearchEmptyState(props.searchQuery()).text}
                </div>
              </Show>

              <For each={props.filteredTabGroups()}>
                {(group) => (
                  <div class="mb-6 lg:mb-2 lg:space-y-2">
                    <Show when={!props.sidebarCollapsed()}>
                      <p class="px-4 lg:px-0 mb-2 lg:mb-0 text-[13px] lg:text-xs font-[500] uppercase tracking-wider text-muted">
                        {group.label}
                      </p>
                    </Show>
                    <div class="lg:bg-transparent border-y lg:border-none divide-y lg:divide-y-0 divide-border-subtle flex flex-col lg:space-y-1.5">
                      <For each={group.items}>
                        {(item) => {
                          const isActive = () => props.activeTab() === item.id;
                          return (
                            <button
                              type="button"
                              aria-current={isActive() ? 'page' : undefined}
                              disabled={item.disabled}
                              class={`group flex w-full items-center ${props.sidebarCollapsed() ? 'justify-center' : 'justify-between'} lg:rounded-md ${props.sidebarCollapsed() ? 'px-2 py-2.5' : 'px-4 py-3.5 lg:px-3 lg:py-2'} text-[15px] lg:text-sm font-medium transition-colors ${item.disabled ? 'opacity-60 cursor-not-allowed text-muted' : isActive() ? 'lg:bg-blue-50 text-blue-600 dark:lg:bg-blue-900 dark:text-blue-300 lg:dark:text-blue-200 bg-surface' : ' lg:hover:bg-surface-hover hover:text-base-content active:bg-surface-hover lg:active:bg-transparent'}`}
                              onClick={() => {
                                if (item.disabled) return;
                                props.setActiveTab(item.id);
                                props.setIsMobileMenuOpen(false);
                              }}
                              title={props.sidebarCollapsed() ? item.label : undefined}
                            >
                              <div class="flex items-center gap-3.5 lg:gap-2.5 w-full">
                                <div
                                  class={`flex items-center justify-center rounded-md lg:rounded-none w-8 h-8 lg:w-auto lg:h-auto ${isActive() ? 'bg-blue-100 dark:bg-blue-900 lg:bg-transparent text-blue-600 dark:text-blue-400' : 'bg-surface-alt lg:bg-transparent text-muted lg:text-inherit'}`}
                                >
                                  <item.icon
                                    class="w-5 h-5 lg:w-4 lg:h-4"
                                    {...(item.iconProps || {})}
                                  />
                                </div>
                                <Show when={!props.sidebarCollapsed()}>
                                  <span
                                    class={`truncate flex-1 text-left ${isActive() ? 'font-semibold lg:font-medium' : ''}`}
                                  >
                                    {item.label}
                                  </span>
                                  <Show when={item.badge && !props.isPro()}>
                                    <span class="ml-auto px-1.5 py-0.5 text-[10px] font-bold uppercase tracking-wider bg-indigo-500 text-white rounded-md shadow-none">
                                      {item.badge}
                                    </span>
                                  </Show>
                                  <ChevronRight class="w-4 h-4 lg:hidden text-muted ml-1 flex-shrink-0" />
                                </Show>
                              </div>
                            </button>
                          );
                        }}
                      </For>
                    </div>
                  </div>
                )}
              </For>
            </div>
          </div>
        </div>

        <div
          class={`flex-1 overflow-hidden ${props.isMobileMenuOpen() ? 'hidden lg:block' : 'block animate-slideInRight lg:animate-none'}`}
        >
          <Show when={props.flatTabs().length > 0}>
            <div class="lg:hidden sticky top-0 z-40 bg-surface/95 border-b border-border-subtle px-3 py-2.5 flex items-center shadow-none">
              <button
                type="button"
                onClick={() => props.setIsMobileMenuOpen(true)}
                class="flex items-center gap-1.5 text-blue-600 dark:text-blue-400 font-medium active:bg-blue-50 dark:active:bg-blue-900 px-2 py-1.5 rounded-md transition-colors"
              >
                <svg
                  class="h-5 w-5 -ml-1 flex-shrink-0"
                  fill="none"
                  stroke="currentColor"
                  stroke-width="2.5"
                  viewBox="0 0 24 24"
                >
                  <path stroke-linecap="round" stroke-linejoin="round" d="M15 19l-7-7 7-7" />
                </svg>
                Settings
              </button>
              <div class="ml-auto font-semibold text-base-content pr-3">
                <Show when={props.flatTabs().find((tab) => tab.id === props.activeTab())}>
                  {(tab) => tab().label}
                </Show>
              </div>
            </div>
          </Show>

          <div class="p-4 sm:p-6 lg:p-8">{props.children}</div>
        </div>
      </Card>
    </div>
  );
};
