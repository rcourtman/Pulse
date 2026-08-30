import { Accessor, Component, For, JSX, Setter, Show } from 'solid-js';
import ChevronRight from 'lucide-solid/icons/chevron-right';
import { Card } from '@/components/shared/Card';
import { PageHeader } from '@/components/shared/PageHeader';
import { SearchInput } from '@/components/shared/SearchInput';
import {
  getSettingsSearchEmptyState,
  getSettingsShellCopy,
  getSettingsUnsavedChangesBanner,
} from '@/utils/settingsShellPresentation';
import type { SettingsHeaderMeta, SettingsNavGroup, SettingsTab } from './settingsNavigationModel';
import { isInfrastructureSettingsTab } from './settingsNavigationModel';

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
  const shellCopy = () => getSettingsShellCopy();
  const unsavedChangesBanner = () => getSettingsUnsavedChangesBanner();
  const infrastructureWorkspaceActive = () => isInfrastructureSettingsTab(props.activeTab());
  const isSidebarItemActive = (itemId: SettingsTab) =>
    itemId === 'infrastructure-systems'
      ? isInfrastructureSettingsTab(props.activeTab())
      : props.activeTab() === itemId;

  return (
    <div data-settings-shell class="min-w-0 max-w-full space-y-0 lg:space-y-6">
      <div class="hidden lg:block">
        <PageHeader title={props.headerMeta().title} description={props.headerMeta().description} />
      </div>

      <Show when={props.hasUnsavedChanges() && props.activeTabSaveBehavior() === 'system'}>
        <div class="mb-3 border-l-4 border-amber-500 bg-amber-50 p-3 shadow-sm dark:border-amber-400 dark:bg-amber-900 sm:rounded-r-lg sm:p-4 lg:mb-0">
          <div class="flex flex-col items-start justify-between gap-3 sm:flex-row sm:items-center sm:gap-4">
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
                <p class="font-semibold text-amber-900 dark:text-amber-100">
                  {unsavedChangesBanner().title}
                </p>
                <p class="text-sm text-amber-700 dark:text-amber-200 mt-0.5">
                  {unsavedChangesBanner().description}
                </p>
              </div>
            </div>
            <div class="flex w-full gap-2 sm:w-auto sm:gap-3">
              <button
                type="button"
                class="flex-1 sm:flex-initial px-5 py-2.5 text-sm font-medium bg-amber-600 text-white rounded-md hover:bg-amber-700 shadow-sm transition-colors"
                onClick={props.saveSettings}
              >
                {unsavedChangesBanner().saveLabel}
              </button>
              <button
                type="button"
                class="px-4 py-2.5 text-sm font-medium text-amber-700 dark:text-amber-200 hover:underline transition-colors"
                onClick={props.discardChanges}
              >
                {unsavedChangesBanner().discardLabel}
              </button>
            </div>
          </div>
        </div>
      </Show>

      <Card
        padding="none"
        border={false}
        class="relative flex min-w-0 max-w-full overflow-visible border-y border-border max-sm:rounded-none sm:border lg:min-h-[600px] lg:flex-row lg:overflow-hidden"
      >
        <nav
          data-settings-navigation
          class={`${props.isMobileMenuOpen() ? 'flex w-full flex-col' : 'hidden lg:flex lg:flex-col'} ${props.sidebarCollapsed() ? 'lg:w-16 lg:min-w-[4rem] lg:max-w-[4rem] lg:basis-[4rem]' : 'lg:w-72 lg:min-w-[18rem] lg:max-w-[18rem] lg:basis-[18rem]'} relative z-10 max-h-[calc(100dvh-8rem)] flex-shrink-0 overflow-y-auto overscroll-contain border-b border-border bg-surface transition-all duration-200 lg:max-h-none lg:overflow-visible lg:border-b-0 lg:border-r lg:bg-transparent lg:align-top`}
          aria-label={shellCopy().navigationAriaLabel}
        >
          <div
            class={`${props.sidebarCollapsed() ? 'px-2' : 'px-3 lg:px-4'} space-y-4 py-3 transition-all duration-200 lg:sticky lg:top-0 lg:space-y-5 lg:py-5`}
          >
            <Show when={!props.sidebarCollapsed()}>
              <div class="flex min-h-11 items-center justify-between border-b border-border pb-2 lg:min-h-0">
                <div
                  role="heading"
                  aria-level="1"
                  class="text-base font-semibold tracking-tight text-base-content lg:text-sm"
                >
                  {shellCopy().navigationTitle}
                </div>
                <button
                  type="button"
                  onClick={() => props.setIsMobileMenuOpen(false)}
                  class="inline-flex min-h-11 min-w-11 items-center justify-center rounded-md text-muted transition-colors active:bg-surface-hover lg:hidden"
                  aria-label={shellCopy().mobileCloseLabel}
                >
                  <svg
                    class="h-5 w-5"
                    fill="none"
                    viewBox="0 0 24 24"
                    stroke="currentColor"
                    stroke-width="2"
                  >
                    <path stroke-linecap="round" stroke-linejoin="round" d="M6 18 18 6M6 6l12 12" />
                  </svg>
                </button>
                <button
                  type="button"
                  onClick={() => props.setSidebarCollapsed(true)}
                  class="hidden rounded-md p-1 transition-colors hover:bg-surface-hover lg:inline-flex"
                  aria-label={shellCopy().collapseSidebarLabel}
                  aria-controls="settings-sidebar-menu"
                  aria-expanded="true"
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
                class="hidden w-full rounded-md p-2 transition-colors hover:bg-surface-hover lg:block"
                aria-label={shellCopy().expandSidebarLabel}
                aria-controls="settings-sidebar-menu"
                aria-expanded="false"
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
                <div class="pb-1 lg:px-2 lg:pb-2">
                  <SearchInput
                    value={props.searchQuery}
                    onChange={props.setSearchQuery}
                    placeholder={shellCopy().searchPlaceholder}
                    class="w-full"
                    captureBackspace
                    clearOnEscape
                    shortcutHint={shellCopy().searchShortcutHint}
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
                  <div class="mb-4 lg:mb-2 lg:space-y-2">
                    <Show
                      when={
                        !props.sidebarCollapsed() &&
                        !(group.items.length === 1 && group.items[0]?.label === group.label)
                      }
                    >
                      <p class="mb-1.5 px-2 text-[11px] font-semibold uppercase tracking-wider text-muted lg:mb-0 lg:px-0 lg:text-xs lg:font-[500]">
                        {group.label}
                      </p>
                    </Show>
                    <div class="flex flex-col divide-y divide-border-subtle overflow-hidden rounded-md border border-border-subtle bg-surface-alt lg:space-y-1.5 lg:divide-y-0 lg:overflow-visible lg:rounded-none lg:border-0 lg:bg-transparent">
                      <For each={group.items}>
                        {(item) => {
                          const isActive = () => isSidebarItemActive(item.id);
                          return (
                            <button
                              type="button"
                              aria-current={isActive() ? 'page' : undefined}
                              disabled={item.disabled}
                              class={`group flex min-h-12 w-full items-center ${props.sidebarCollapsed() ? 'justify-center' : 'justify-between'} lg:min-h-0 lg:rounded-md ${props.sidebarCollapsed() ? 'px-2 py-2.5' : 'px-3 py-2.5 lg:px-3 lg:py-2'} text-sm font-medium transition-colors ${item.disabled ? 'cursor-not-allowed text-muted opacity-60' : isActive() ? 'bg-surface text-blue-600 dark:text-blue-300 lg:bg-blue-50 lg:dark:bg-blue-900 lg:dark:text-blue-200' : 'hover:text-base-content active:bg-surface-hover lg:hover:bg-surface-hover lg:active:bg-transparent'}`}
                              onClick={() => {
                                if (item.disabled) return;
                                props.setActiveTab(item.id);
                                props.setIsMobileMenuOpen(false);
                              }}
                              title={props.sidebarCollapsed() ? item.label : undefined}
                            >
                              <div class="flex w-full items-center gap-3 lg:gap-2.5">
                                <div
                                  class={`flex h-8 w-8 items-center justify-center rounded-md lg:h-auto lg:w-auto lg:rounded-none ${isActive() ? 'bg-blue-100 text-blue-600 dark:bg-blue-900 dark:text-blue-400 lg:bg-transparent' : 'bg-surface text-muted lg:bg-transparent lg:text-inherit'}`}
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
        </nav>

        <div
          data-settings-content
          class={`min-w-0 flex-1 overflow-visible lg:overflow-hidden ${props.isMobileMenuOpen() ? 'hidden lg:block' : 'block'}`}
        >
          <Show when={props.flatTabs().length > 0}>
            <div class="sticky top-0 z-40 flex min-h-12 items-center border-b border-border-subtle bg-surface/95 px-2 backdrop-blur lg:hidden">
              <button
                type="button"
                onClick={() => {
                  props.setSidebarCollapsed(false);
                  props.setIsMobileMenuOpen(true);
                }}
                class="flex min-h-11 items-center gap-1 rounded-md px-2 py-1.5 text-sm font-medium text-blue-600 transition-colors active:bg-blue-50 dark:text-blue-400 dark:active:bg-blue-900"
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
                {shellCopy().mobileBackLabel}
              </button>
              <div
                role="heading"
                aria-level="1"
                class="ml-auto truncate pr-2 text-sm font-semibold text-base-content"
              >
                <Show keyed when={props.flatTabs().find((tab) => tab.id === props.activeTab())}>
                  {(tab) => tab.label}
                </Show>
              </div>
            </div>
          </Show>

          <div
            data-settings-content-body
            class={`min-w-0 ${infrastructureWorkspaceActive() ? 'p-0 sm:p-4 lg:p-5' : 'py-3 sm:p-6 lg:p-8'}`}
          >
            {props.children}
          </div>
        </div>
      </Card>
    </div>
  );
};
