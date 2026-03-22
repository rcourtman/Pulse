import { Show } from 'solid-js';
import { SearchInput } from '@/components/shared/SearchInput';
import Server from 'lucide-solid/icons/server';
import Mail from 'lucide-solid/icons/mail';
import Users from 'lucide-solid/icons/users';
import Boxes from 'lucide-solid/icons/boxes';
import { BulkEditDialog } from './BulkEditDialog';
import { ThresholdsTableAgentsTab } from './ThresholdsTableAgentsTab';
import { ThresholdsTableDockerTab } from './ThresholdsTableDockerTab';
import { ThresholdsTablePMGTab } from './ThresholdsTablePMGTab';
import { ThresholdsTableProxmoxTab } from './ThresholdsTableProxmoxTab';
import type { ThresholdsTableProps } from '@/features/alerts/thresholds/types';
import { useThresholdsTableState } from '@/features/alerts/thresholds/hooks/useThresholdsTableState';

export function ThresholdsTable(props: ThresholdsTableProps) {
  const state = useThresholdsTableState(props);

  return (
    <div class="space-y-4">
      <div class="relative">
        <SearchInput
          value={state.searchTerm}
          onChange={state.setSearchTerm}
          placeholder={state.getAlertThresholdsSearchPlaceholder()}
          class="w-full"
          onBeforeAutoFocus={() => Boolean(state.editingId())}
          focusOnShortcut
          clearOnEscape
          shortcutHint="Ctrl+F"
        />
      </div>

      <Show when={!state.helpBannerDismissed()}>
        <div class="rounded-md border border-blue-200 bg-blue-50 dark:border-blue-800 dark:bg-blue-900 p-3 relative group">
          <button
            type="button"
            onClick={state.dismissHelpBanner}
            class="absolute top-2 right-2 p-1 rounded-md text-blue-400 hover:text-blue-600 dark:text-blue-500 dark:hover:text-blue-300 hover:bg-blue-100 dark:hover:bg-blue-900 opacity-0 group-hover:opacity-100 transition-opacity"
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
              <span class="font-medium">{state.getAlertThresholdsHelpBanner().title}</span> Set any
              threshold to{' '}
              <code class="px-1 py-0.5 bg-blue-100 dark:bg-blue-900 rounded text-xs font-mono">
                {state.getAlertThresholdsHelpBanner().disableValue}
              </code>{' '}
              to disable alerts for that metric. Click on disabled thresholds showing{' '}
              <span class="italic">{state.getAlertThresholdsHelpBanner().reenableLabel}</span> to
              re-enable them. Resources with custom settings show a{' '}
              <span class="inline-flex items-center px-1.5 py-0.5 bg-blue-100 dark:bg-blue-900 text-blue-700 dark:text-blue-300 rounded text-xs">
                {state.getAlertThresholdsHelpBanner().customBadgeLabel}
              </span>{' '}
              badge.{' '}
              <span class="text-blue-600 dark:text-blue-400">
                {state.getAlertThresholdsHelpBanner().collapseHint}
              </span>
            </div>
          </div>
        </div>
      </Show>

      <div class="border-b border-border">
        <nav class="-mb-px flex gap-4 sm:gap-6" aria-label="Tabs">
          <button
            type="button"
            onClick={() => state.handleTabClick('proxmox')}
            class={`py-3 px-1 border-b-2 font-medium text-sm transition-colors cursor-pointer flex items-center gap-1.5 ${state.activeTab() === 'proxmox' ? 'border-blue-500 text-blue-600 dark:text-blue-400' : 'border-transparent text-muted hover:text-base-content hover:border-slate-300'}`}
          >
            <Server class="w-4 h-4" />
            <span class="hidden sm:inline">Proxmox / PBS</span>
            <span class="sm:hidden">Proxmox</span>
          </button>
          <button
            type="button"
            onClick={() => state.handleTabClick('pmg')}
            class={`py-3 px-1 border-b-2 font-medium text-sm transition-colors cursor-pointer flex items-center gap-1.5 ${state.activeTab() === 'pmg' ? 'border-blue-500 text-blue-600 dark:text-blue-400' : 'border-transparent text-muted hover:text-base-content hover:border-slate-300'}`}
          >
            <Mail class="w-4 h-4" />
            <span class="hidden sm:inline">Mail Gateway</span>
            <span class="sm:hidden">Mail</span>
          </button>
          <button
            type="button"
            onClick={() => state.handleTabClick('agents')}
            class={`py-3 px-1 border-b-2 font-medium text-sm transition-colors cursor-pointer flex items-center gap-1.5 ${state.activeTab() === 'agents' ? 'border-blue-500 text-blue-600 dark:text-blue-400' : 'border-transparent text-muted hover:text-base-content hover:border-slate-300'}`}
          >
            <Users class="w-4 h-4" />
            <span>Agents</span>
          </button>
          <button
            type="button"
            onClick={() => state.handleTabClick('docker')}
            class={`py-3 px-1 border-b-2 font-medium text-sm transition-colors cursor-pointer flex items-center gap-1.5 ${state.activeTab() === 'docker' ? 'border-blue-500 text-blue-600 dark:text-blue-400' : 'border-transparent text-muted hover:text-base-content hover:border-slate-300'}`}
          >
            <Boxes class="w-4 h-4" />
            <span>Containers</span>
          </button>
        </nav>
      </div>

      <Show when={state.activeTab() === 'proxmox'}>
        <div class="flex justify-end gap-2">
          <button
            type="button"
            onClick={state.expandAll}
            class="text-xs px-2 py-1 hover:text-muted hover:bg-surface-hover rounded transition-colors"
          >
            Expand all
          </button>
          <span class="text-muted">|</span>
          <button
            type="button"
            onClick={state.collapseAll}
            class="text-xs px-2 py-1 hover:text-muted hover:bg-surface-hover rounded transition-colors"
          >
            Collapse all
          </button>
        </div>
      </Show>

      <div class="space-y-6">
        <Show when={state.activeTab() === 'proxmox'}>
          <ThresholdsTableProxmoxTab state={state} tableProps={props} />
        </Show>

        <Show when={state.activeTab() === 'pmg'}>
          <ThresholdsTablePMGTab state={state} tableProps={props} />
        </Show>

        <Show when={state.activeTab() === 'agents'}>
          <ThresholdsTableAgentsTab state={state} tableProps={props} />
        </Show>

        <Show when={state.activeTab() === 'docker'}>
          <ThresholdsTableDockerTab state={state} tableProps={props} />
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
