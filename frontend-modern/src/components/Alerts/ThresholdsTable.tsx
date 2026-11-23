import { For, Show, type Component } from 'solid-js';
import { getPlatformIcon } from '@/features/platformPage/platformIcon';
import { FilterBar, type FilterDef } from '@/components/shared/FilterBar';
import { LoadingSpinner } from '@/components/shared/LoadingSpinner';
import { Subtabs, type SubtabOption } from '@/components/shared/Subtabs';
import { useBreakpoint } from '@/hooks/useBreakpoint';
import { BulkEditDialog } from './BulkEditDialog';
import { ThresholdsTableAgentsTab } from './ThresholdsTableAgentsTab';
import { ThresholdsTableDockerTab } from './ThresholdsTableDockerTab';
import { ThresholdsTableKubernetesTab } from './ThresholdsTableKubernetesTab';
import { ThresholdsTableProxmoxTab } from './ThresholdsTableProxmoxTab';
import { ThresholdsTableTrueNASTab } from './ThresholdsTableTrueNASTab';
import { ThresholdsTableVMwareTab } from './ThresholdsTableVMwareTab';
import type { ThresholdsTableProps } from '@/features/alerts/thresholds/types';
import type { ThresholdsActiveTab } from '@/features/alerts/thresholds/tableTypes';
import { useThresholdsTableState } from '@/features/alerts/thresholds/hooks/useThresholdsTableState';

const thresholdPlatformDefinitions = [
  { value: 'proxmox', label: 'Proxmox', icon: getPlatformIcon('proxmox') },
  { value: 'docker', label: 'Docker', icon: getPlatformIcon('docker') },
  { value: 'kubernetes', label: 'Kubernetes', icon: getPlatformIcon('kubernetes') },
  { value: 'truenas', label: 'TrueNAS', icon: getPlatformIcon('truenas') },
  { value: 'vmware', label: 'vSphere', icon: getPlatformIcon('vmware') },
  { value: 'systems', label: 'Machines', icon: getPlatformIcon('systems') },
] as const;

export function ThresholdsTable(props: ThresholdsTableProps) {
  const state = useThresholdsTableState(props);
  const { isMobile } = useBreakpoint();
  const platformTabs: SubtabOption[] = thresholdPlatformDefinitions.map((definition) => {
    const Icon = definition.icon as Component<{ class?: string; title?: string }>;
    return {
      value: definition.value,
      label: (
        <span class="inline-flex items-center gap-2">
          <span aria-hidden="true">
            <Icon class="h-4 w-4" title="" />
          </span>
          <span>{definition.label}</span>
        </span>
      ),
    };
  });

  const filters = (): FilterDef[] => [
    {
      id: 'alerts-overrides',
      label: 'Overrides',
      group: 'properties',
      value: state.overrideFilter,
      setValue: state.setOverrideFilter,
      defaultValue: 'all',
      options: () => [
        { value: 'all', label: 'All resources' },
        { value: 'custom', label: 'Custom only' },
        { value: 'disabled', label: 'Disabled only' },
      ],
    },
  ];
  const customOverrideItems = () => state.summaryItems().filter((item) => item.overrides > 0);
  const showResourcesLoadingSkeleton = () =>
    Boolean(props.resourcesLoading?.()) && props.allResources.length === 0;

  return (
    <div class="space-y-3">
      <Subtabs
        value={state.activeTab()}
        onChange={(value) => state.handleTabClick(value as ThresholdsActiveTab)}
        tabs={platformTabs}
        ariaLabel="Threshold platform"
      />

      <FilterBar
        role="group"
        ariaLabel="Alert threshold filters"
        isMobile={isMobile}
        search={{
          value: state.searchTerm,
          setValue: state.setSearchTerm,
          placeholder: state.getAlertThresholdsSearchPlaceholder(),
          clearOnEscape: true,
          onBeforeAutoFocus: () => Boolean(state.editingId()),
        }}
        filters={filters()}
        showClearAll={() =>
          state.searchTerm().trim().length > 0 || state.overrideFilter() !== 'all'
        }
        onClearAll={() => {
          state.setSearchTerm('');
          state.setOverrideFilter('all');
        }}
      />

      <Show when={!state.helpBannerDismissed()}>
        <div class="relative rounded-md border border-blue-200 bg-blue-50 px-3 py-2 dark:border-blue-800 dark:bg-blue-900/25">
          <button
            type="button"
            onClick={state.dismissHelpBanner}
            class="absolute right-1 top-1 inline-flex min-h-11 min-w-11 items-center justify-center rounded-md p-1 text-blue-600 transition-colors hover:bg-blue-100 hover:text-blue-800 dark:text-blue-300 dark:hover:bg-blue-900/40 dark:hover:text-blue-100 sm:right-2 sm:top-1.5 sm:min-h-0 sm:min-w-0"
            title={state.getAlertThresholdsHelpDismissLabel()}
            aria-label={state.getAlertThresholdsHelpDismissLabel()}
          >
            <svg class="h-4 w-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path
                stroke-linecap="round"
                stroke-linejoin="round"
                stroke-width="2"
                d="M6 18L18 6M6 6l12 12"
              />
            </svg>
          </button>
          <div class="flex items-start gap-2 pr-8">
            <svg
              class="mt-0.5 h-4 w-4 shrink-0 text-blue-600 dark:text-blue-400"
              fill="none"
              stroke="currentColor"
              viewBox="0 0 24 24"
              aria-hidden="true"
            >
              <path
                stroke-linecap="round"
                stroke-linejoin="round"
                stroke-width="2"
                d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"
              />
            </svg>
            <div class="space-y-0.5 text-xs leading-5 text-blue-900 dark:text-blue-100">
              <p class="font-medium">{state.getAlertThresholdsHelpBanner().title}</p>
              <p>
                {state.getAlertThresholdsHelpBanner().toggleGuidance}{' '}
                {state.getAlertThresholdsHelpBanner().inheritanceGuidance}
              </p>
              <p>
                {state.getAlertThresholdsHelpBanner().bulkGuidance}{' '}
                <span class="text-blue-700 dark:text-blue-300">
                  {state.getAlertThresholdsHelpBanner().collapseHint}
                </span>
              </p>
            </div>
          </div>
        </div>
      </Show>

      <Show when={customOverrideItems().length > 0}>
        <section
          class="rounded-lg border border-sky-200 bg-sky-50/70 p-3 dark:border-sky-800 dark:bg-sky-950/30"
          aria-label={state.getAlertThresholdsOverridesPresentation().title}
        >
          <div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
            <div>
              <h3 class="text-sm font-semibold text-base-content">
                {state.getAlertThresholdsOverridesPresentation().title}
              </h3>
              <p class="mt-0.5 text-xs text-muted">
                {state.getAlertThresholdsOverridesPresentation().description}
              </p>
            </div>
            <div class="flex flex-wrap gap-2">
              <For each={customOverrideItems()}>
                {(item) => (
                  <button
                    type="button"
                    class="inline-flex min-h-11 items-center gap-2 rounded-full border border-sky-200 bg-surface px-3 py-1.5 text-xs font-medium text-base-content transition-colors hover:border-sky-400 hover:bg-sky-100 focus:outline-hidden focus-visible:ring-2 focus-visible:ring-sky-500 sm:min-h-0 dark:border-sky-700 dark:hover:bg-sky-900/25"
                    onClick={() => state.revealCustomOverrides(item.key)}
                    aria-label={`Show ${item.label}: ${state.getAlertThresholdsOverrideCountLabel(item.overrides)}`}
                  >
                    <span>{item.label}</span>
                    <span class="rounded-full bg-sky-100 px-2 py-0.5 text-sky-700 dark:bg-sky-900/25 dark:text-sky-200">
                      {state.getAlertThresholdsOverrideCountLabel(item.overrides)}
                    </span>
                  </button>
                )}
              </For>
            </div>
          </div>
        </section>
      </Show>

      <Show when={state.summaryItems().length > 0}>
        <div class="flex items-center justify-end gap-2">
          <button
            type="button"
            onClick={state.expandAll}
            class="min-h-11 rounded-sm px-2 py-1 text-xs transition-colors hover:bg-surface-hover hover:text-muted sm:min-h-0"
          >
            Expand all
          </button>
          <span class="select-none text-xs text-muted" aria-hidden="true">
            |
          </span>
          <button
            type="button"
            onClick={state.collapseAll}
            class="min-h-11 rounded-sm px-2 py-1 text-xs transition-colors hover:bg-surface-hover hover:text-muted sm:min-h-0"
          >
            Collapse all
          </button>
        </div>
      </Show>

      <Show when={showResourcesLoadingSkeleton()}>
        <div
          role="status"
          aria-live="polite"
          data-testid="thresholds-resources-loading"
          class="space-y-3 rounded-lg border border-border bg-surface p-4"
        >
          <div class="flex items-center gap-2 text-sm font-medium text-base-content">
            <LoadingSpinner size="sm" />
            <span>Loading monitored resources...</span>
          </div>
          <div class="space-y-2" aria-hidden="true">
            <div class="h-10 w-full animate-pulse rounded-md bg-surface-alt" />
            <div class="h-10 w-full animate-pulse rounded-md bg-surface-alt" />
            <div class="h-10 w-3/4 animate-pulse rounded-md bg-surface-alt" />
          </div>
        </div>
      </Show>

      <div class="space-y-5">
        <Show when={state.activeTab() === 'proxmox'}>
          <ThresholdsTableProxmoxTab state={state} tableProps={props} />
        </Show>

        <Show when={state.activeTab() === 'docker'}>
          <ThresholdsTableDockerTab state={state} tableProps={props} />
        </Show>

        <Show when={state.activeTab() === 'kubernetes'}>
          <ThresholdsTableKubernetesTab state={state} tableProps={props} />
        </Show>

        <Show when={state.activeTab() === 'truenas'}>
          <ThresholdsTableTrueNASTab state={state} tableProps={props} />
        </Show>

        <Show when={state.activeTab() === 'vmware'}>
          <ThresholdsTableVMwareTab state={state} tableProps={props} />
        </Show>

        <Show when={state.activeTab() === 'systems'}>
          <ThresholdsTableAgentsTab state={state} tableProps={props} />
        </Show>
      </div>

      <BulkEditDialog
        isOpen={state.isBulkEditDialogOpen()}
        onClose={() => state.setIsBulkEditDialogOpen(false)}
        selectedIds={state.bulkEditIds()}
        columns={state.bulkEditColumns()}
        onSave={state.handleSaveBulkEdit}
      />
    </div>
  );
}
