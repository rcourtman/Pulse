import { For, Show } from 'solid-js';
import { getPlatformIcon } from '@/features/platformPage/platformIcon';
import { FilterBar, type FilterDef } from '@/components/shared/FilterBar';
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
    const Icon = definition.icon;
    return {
      value: definition.value,
      label: (
        <span class="inline-flex items-center gap-2">
          <span aria-hidden="true">
            <Icon class="h-4 w-4" />
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

  return (
    <div class="space-y-4">
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
        <div class="rounded-md border border-blue-200 bg-blue-50 dark:border-blue-800 dark:bg-blue-900 p-3 relative group">
          <button
            type="button"
            onClick={state.dismissHelpBanner}
            class="absolute right-1 top-1 inline-flex min-h-11 min-w-11 items-center justify-center rounded-md p-1 text-blue-500 transition-colors hover:bg-blue-100 hover:text-blue-700 dark:text-blue-400 dark:hover:bg-blue-900 dark:hover:text-blue-200 sm:right-2 sm:top-2 sm:min-h-0 sm:min-w-0 sm:opacity-0 sm:group-hover:opacity-100"
            title={state.getAlertThresholdsHelpDismissLabel()}
            aria-label={state.getAlertThresholdsHelpDismissLabel()}
          >
            <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path
                stroke-linecap="round"
                stroke-linejoin="round"
                stroke-width="2"
                d="M6 18L18 6M6 6l12 12"
              />
            </svg>
          </button>
          <div class="flex items-start gap-2 pr-6">
            <svg
              class="w-5 h-5 text-blue-600 dark:text-blue-400 flex-shrink-0 mt-0.5"
              fill="none"
              stroke="currentColor"
              viewBox="0 0 24 24"
            >
              <path
                stroke-linecap="round"
                stroke-linejoin="round"
                stroke-width="2"
                d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"
              />
            </svg>
            <div class="text-sm text-blue-900 dark:text-blue-100">
              <p class="font-medium">{state.getAlertThresholdsHelpBanner().title}</p>
              <p class="mt-1">{state.getAlertThresholdsHelpBanner().toggleGuidance}</p>
              <p>{state.getAlertThresholdsHelpBanner().inheritanceGuidance}</p>
              <p>{state.getAlertThresholdsHelpBanner().bulkGuidance}</p>
              <p class="text-blue-600 dark:text-blue-400">
                {state.getAlertThresholdsHelpBanner().collapseHint}
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
                    class="inline-flex min-h-11 items-center gap-2 rounded-full border border-sky-200 bg-surface px-3 py-1.5 text-xs font-medium text-base-content transition-colors hover:border-sky-400 hover:bg-sky-100 focus:outline-none focus-visible:ring-2 focus-visible:ring-sky-500 sm:min-h-0 dark:border-sky-700 dark:hover:bg-sky-900"
                    onClick={() => state.revealCustomOverrides(item.key)}
                    aria-label={`Show ${item.label}: ${state.getAlertThresholdsOverrideCountLabel(item.overrides)}`}
                  >
                    <span>{item.label}</span>
                    <span class="rounded-full bg-sky-100 px-2 py-0.5 text-sky-700 dark:bg-sky-900 dark:text-sky-200">
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
        <div class="flex justify-end gap-2">
          <button
            type="button"
            onClick={state.expandAll}
            class="min-h-11 rounded px-2 py-1 text-xs transition-colors hover:bg-surface-hover hover:text-muted sm:min-h-0"
          >
            Expand all
          </button>
          <span class="text-muted">|</span>
          <button
            type="button"
            onClick={state.collapseAll}
            class="min-h-11 rounded px-2 py-1 text-xs transition-colors hover:bg-surface-hover hover:text-muted sm:min-h-0"
          >
            Collapse all
          </button>
        </div>
      </Show>

      <div class="space-y-6">
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
