import type { Component } from 'solid-js';
import { For, Show } from 'solid-js';
import Users from 'lucide-solid/icons/users';
import { Dialog } from '@/components/shared/Dialog';
import { PulseDataGrid } from '@/components/shared/PulseDataGrid';
import { SearchField } from '@/components/shared/SearchField';
import SettingsPanel from '@/components/shared/SettingsPanel';
import { formatAbsoluteTime, formatRelativeTime } from '@/utils/format';
import { getMonitoringStoppedEmptyState, getRemovedUnifiedAgentItemLabel } from '@/utils/unifiedAgentInventoryPresentation';
import type { AgentCapability } from '@/utils/agentCapabilityPresentation';
import type { ScopeCategory } from './infrastructureOperationsModel';
import { getCapabilitySurfaceLabel } from './infrastructureOperationsModel';
import { InfrastructureActiveRowDetails } from './InfrastructureActiveRowDetails';
import { InfrastructureIgnoredRowDetails } from './InfrastructureIgnoredRowDetails';
import { useInfrastructureOperationsContext } from './useInfrastructureOperationsState';

export const InfrastructureInventorySection: Component = () => {
  const state = useInfrastructureOperationsContext();

  return (
    <div class="space-y-6">
      <div class="rounded-md border border-border bg-surface-alt px-4 py-3 text-sm">
        <p class="text-base-content">{state.inventoryStatusSummaryText()}</p>
      </div>

      <SettingsPanel
        title="Reporting now"
        description="Systems, platforms, and runtimes currently checking in to Pulse."
        icon={<Users class="h-5 w-5" strokeWidth={2} />}
        bodyClass="space-y-4"
      >
        <div class="rounded-md border border-border bg-surface-alt px-4 py-3">
          <p class="text-sm font-medium text-base-content">{state.reportingCoverageSummaryText()}</p>
          <p class="mt-2 text-xs text-muted">
            This workspace does not list every asset Pulse has discovered. It focuses on systems
            plus platform-integrated runtimes that are actively checking in right now.
          </p>
        </div>

        <div class="rounded-md border border-emerald-200 bg-emerald-50 px-4 py-4 dark:border-emerald-800 dark:bg-emerald-950/40">
          <p class="text-xs font-semibold uppercase tracking-wide text-emerald-800 dark:text-emerald-300">
            Active reporting
          </p>
          <p class="mt-2 text-2xl font-semibold text-emerald-900 dark:text-emerald-100">
            {state.filteredActiveRows().length}
          </p>
          <p class="mt-2 text-sm text-emerald-900 dark:text-emerald-200">
            Item{state.filteredActiveRows().length === 1 ? '' : 's'} actively checking in to Pulse.
          </p>
        </div>

        <Show when={state.hasLinkedAgents()}>
          <div class="flex items-start gap-2 rounded-md border border-blue-200 bg-blue-50 px-3 py-2 dark:border-blue-800 dark:bg-blue-900">
            <svg
              class="mt-0.5 h-4 w-4 shrink-0 text-blue-500 dark:text-blue-400"
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
              stroke-width="2"
            >
              <path
                stroke-linecap="round"
                stroke-linejoin="round"
                d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"
              />
            </svg>
            <p class="text-xs text-blue-700 dark:text-blue-300">
              <span class="font-medium">{state.linkedAgents().length}</span> agent
              {state.linkedAgents().length > 1 ? 's are' : ' is'} linked to Proxmox node
              {state.linkedAgents().length > 1 ? 's' : ''} and flagged with a{' '}
              <span class="font-medium text-blue-700 dark:text-blue-300">Linked</span> badge.
            </p>
          </div>
        </Show>

        <Show when={state.hasOutdatedAgents()}>
          <div class="rounded-md border border-amber-200 bg-amber-50 px-4 py-3 dark:border-amber-700 dark:bg-amber-900">
            <div class="flex items-start gap-3">
              <svg
                class="mt-0.5 h-5 w-5 shrink-0 text-amber-500 dark:text-amber-400"
                viewBox="0 0 20 20"
                fill="currentColor"
              >
                <path
                  fill-rule="evenodd"
                  d="M8.485 2.495c.673-1.167 2.357-1.167 3.03 0l6.28 10.875c.673 1.167-.17 2.625-1.516 2.625H3.72c-1.347 0-2.189-1.458-1.515-2.625L8.485 2.495zM10 5a.75.75 0 01.75.75v3.5a.75.75 0 01-1.5 0v-3.5A.75.75 0 0110 5zm0 9a1 1 0 100-2 1 1 0 000 2z"
                  clip-rule="evenodd"
                />
              </svg>
              <div class="flex-1 space-y-1">
                <p class="text-sm font-medium text-amber-800 dark:text-amber-200">
                  {state.outdatedAgents().length} outdated agent
                  {state.outdatedAgents().length > 1 ? ' binaries' : ' binary'} detected
                </p>
                <p class="text-sm text-amber-700 dark:text-amber-300">
                  Older standalone agent binaries are deprecated. Expand a row to copy the upgrade
                  command.
                </p>
              </div>
            </div>
          </div>
        </Show>

        <div class="space-y-3">
          <div class="min-w-[220px] space-y-1">
            <label for="agent-filter-search" class="text-xs font-medium text-muted">
              Search reporting items
            </label>
            <SearchField
              placeholder="Search name, hostname, or ID"
              value={state.filterSearch()}
              onChange={state.setFilterSearch}
              class="w-full"
              inputClass="min-h-10 sm:min-h-9 px-3 py-2 sm:py-1.5 shadow-sm focus:ring-1"
            />
          </div>

          <div class="flex flex-wrap items-end gap-3">
            <div class="pb-2 text-xs font-medium uppercase tracking-wide text-muted">
              Refine results
            </div>
            <div class="space-y-1">
              <label for="agent-filter-capability" class="text-xs font-medium text-muted">
                Capability
              </label>
              <select
                id="agent-filter-capability"
                value={state.filterCapability()}
                onChange={(event) =>
                  state.setFilterCapability(event.currentTarget.value as 'all' | AgentCapability)
                }
                class="min-h-10 sm:min-h-9 rounded-md border border-border bg-surface px-2.5 py-2 text-sm text-base-content shadow-sm focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500 dark:focus:border-blue-400 dark:focus:ring-blue-800 sm:py-1.5"
              >
                <option value="all">All capabilities</option>
                <option value="agent">Agent</option>
                <option value="docker">Docker</option>
                <option value="kubernetes">Kubernetes</option>
                <option value="proxmox">Proxmox</option>
                <option value="pbs">PBS</option>
                <option value="pmg">PMG</option>
                <option value="truenas">TrueNAS</option>
              </select>
            </div>
            <div class="space-y-1">
              <label for="agent-filter-scope" class="text-xs font-medium text-muted">
                Scope
              </label>
              <select
                id="agent-filter-scope"
                value={state.filterScope()}
                onChange={(event) =>
                  state.setFilterScope(
                    event.currentTarget.value as 'all' | Exclude<ScopeCategory, 'na'>,
                  )
                }
                class="min-h-10 sm:min-h-9 rounded-md border border-border bg-surface px-2.5 py-2 text-sm text-base-content shadow-sm focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500 dark:focus:border-blue-400 dark:focus:ring-blue-800 sm:py-1.5"
              >
                <option value="all">All scopes</option>
                <option value="default">Default</option>
                <option value="profile">Profile assigned</option>
                <option value="ai-managed">Patrol-managed</option>
              </select>
            </div>
            <button
              type="button"
              onClick={state.resetFilters}
              disabled={!state.hasFilters()}
              class={`min-h-10 sm:min-h-9 rounded-md px-3 py-2 text-sm font-medium transition-colors ${
                state.hasFilters()
                  ? 'text-base-content hover:bg-surface-alt'
                  : 'cursor-not-allowed text-slate-400'
              }`}
            >
              Clear
            </button>
          </div>
        </div>

        <div class="flex flex-wrap items-center justify-between gap-3 text-xs text-muted">
          <span>
            Showing {state.filteredActiveRows().length} of {state.activeRows().length} active
            records.
          </span>
          <Show when={state.filteredMonitoringStoppedRows().length > 0}>
            <span>{state.filteredMonitoringStoppedRows().length} item(s) are currently ignored by Pulse.</span>
          </Show>
        </div>

        <div class="rounded-md border border-border bg-surface-hover px-4 py-3 text-sm text-muted">
          Stop monitoring removes an item from active reporting and moves it into the Ignored by
          Pulse list. The remote system keeps running; Pulse simply ignores new reports until you
          allow reconnect.
        </div>

        <Show when={state.inventoryActionNotice()}>
          {(notice) => (
            <div
              class={`rounded-md border px-4 py-3 text-sm ${
                notice().tone === 'success'
                  ? 'border-emerald-200 bg-emerald-50 text-emerald-900 dark:border-emerald-700 dark:bg-emerald-900 dark:text-emerald-100'
                  : 'border-blue-200 bg-blue-50 text-blue-900 dark:border-blue-700 dark:bg-blue-900 dark:text-blue-100'
              }`}
            >
              <div class="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
                <div class="space-y-1">
                  <p class="font-semibold">{notice().title}</p>
                  <p class="text-xs opacity-90">{notice().detail}</p>
                </div>
                <div class="flex items-center gap-2">
                  <Show when={notice().showRecoveryQueueLink}>
                    <button
                      type="button"
                      onClick={state.scrollToRecoveryQueue}
                      class="inline-flex min-h-10 sm:min-h-9 items-center rounded-md px-2.5 py-1.5 text-xs font-medium underline"
                    >
                      View ignored items
                    </button>
                  </Show>
                  <button
                    type="button"
                    onClick={() => state.setInventoryActionNotice(null)}
                    class="inline-flex min-h-10 sm:min-h-9 items-center rounded-md px-2.5 py-1.5 text-xs font-medium underline"
                    aria-label="Dismiss inventory action message"
                  >
                    Dismiss
                  </button>
                </div>
              </div>
            </div>
          )}
        </Show>

        <div class="grid gap-4">
          <div class="overflow-hidden rounded-xl border border-border bg-surface shadow-sm">
            <div class="border-b border-border bg-surface-alt px-4 py-3">
              <div class="text-[11px] font-semibold uppercase tracking-[0.18em] text-muted">
                Browse reporting items
              </div>
              <div class="mt-2 text-sm text-base-content">
                Select a reporting item to open its details drawer.
              </div>
            </div>
            <PulseDataGrid
              data={state.filteredActiveRows()}
              emptyState={
                state.hasFilters()
                  ? 'No reporting items match the current filters.'
                  : 'Nothing is actively reporting to Pulse yet.'
              }
              desktopMinWidth="960px"
              columns={state.reportingColumns}
              keyExtractor={(row) => row.rowKey}
              onRowClick={(row) => state.toggleAgentDetails(row.rowKey)}
            />
          </div>

          <Dialog
            isOpen={Boolean(state.selectedActiveRow())}
            onClose={() => state.setExpandedRowKey(null)}
            layout="drawer-right"
            panelClass="max-w-[760px]"
            ariaLabel="Reporting item details"
          >
            <Show when={state.selectedActiveRow()}>
              {(rowAccessor) => <InfrastructureActiveRowDetails rowAccessor={rowAccessor} />}
            </Show>
          </Dialog>
        </div>
      </SettingsPanel>

      <Show when={state.showMonitoringStoppedSection()}>
        <div ref={state.setRecoveryQueueSectionRef}>
          <SettingsPanel
            title="Ignored by Pulse"
            description="Items you explicitly told Pulse to ignore stay out of live reporting until reconnect is allowed."
            icon={<Users class="h-5 w-5" strokeWidth={2} />}
            bodyClass="space-y-4"
          >
            <Show
              when={state.filteredMonitoringStoppedRows().length > 0}
              fallback={
                <div class="rounded-md border border-dashed border-border px-4 py-6 text-sm text-muted">
                  {getMonitoringStoppedEmptyState(state.hasFilters())}
                </div>
              }
            >
              <div class="grid gap-4">
                <div class="overflow-hidden rounded-lg border border-amber-200 bg-amber-50/70 dark:border-amber-800 dark:bg-amber-950/30">
                  <div class="border-b border-amber-200 px-4 py-3 dark:border-amber-800">
                    <div class="text-[11px] font-semibold uppercase tracking-[0.18em] text-amber-900 dark:text-amber-100">
                      Browse ignored items
                    </div>
                    <div class="mt-2 text-sm text-amber-950 dark:text-amber-100">
                      Select an ignored item to open its recovery drawer.
                    </div>
                  </div>
                  <div class="divide-y divide-amber-200/80 dark:divide-amber-800/80">
                    <For each={state.filteredMonitoringStoppedRows()}>
                      {(row) => {
                        const pendingAction = () => state.getPendingInventoryAction(row.rowKey);
                        const isSelected = () => state.selectedIgnoredRowKey() === row.rowKey;

                        return (
                          <button
                            type="button"
                            onClick={() => state.setSelectedIgnoredRowKey(row.rowKey)}
                            class={`flex w-full flex-col gap-2 px-4 py-3 text-left transition-colors ${
                              isSelected()
                                ? 'bg-amber-100/80 ring-1 ring-inset ring-amber-300 dark:bg-amber-900/40 dark:ring-amber-700'
                                : 'hover:bg-amber-100/50 dark:hover:bg-amber-900/20'
                            }`}
                          >
                            <div class="flex flex-wrap items-center justify-between gap-3">
                              <div class="min-w-0">
                                <div class="flex flex-wrap items-center gap-2">
                                  <h4 class="truncate text-sm font-semibold text-base-content">
                                    {row.name}
                                  </h4>
                                  <span class="inline-flex items-center rounded-full bg-white/80 px-2 py-0.5 text-[11px] font-medium uppercase tracking-wide text-amber-800 dark:bg-amber-900/60 dark:text-amber-200">
                                    {getRemovedUnifiedAgentItemLabel(row)}
                                  </span>
                                </div>
                                <div class="mt-1 text-xs text-muted">
                                  {row.capabilities.map(getCapabilitySurfaceLabel).join(', ')}
                                </div>
                              </div>
                              <div class="text-[11px] text-muted">
                                {pendingAction() === 'allow-reconnect'
                                  ? 'Reconnect in progress'
                                  : 'Select to review'}
                              </div>
                            </div>
                            <div class="flex flex-wrap gap-x-3 gap-y-1 text-xs text-muted">
                              <Show
                                when={
                                  row.displayName && row.hostname && row.displayName !== row.hostname
                                }
                              >
                                <span>Hostname: {row.hostname}</span>
                              </Show>
                              <span>
                                Stopped{' '}
                                {row.removedAt
                                  ? `${formatRelativeTime(row.removedAt)} (${formatAbsoluteTime(row.removedAt)})`
                                  : 'at an unknown time'}
                              </span>
                            </div>
                          </button>
                        );
                      }}
                    </For>
                  </div>
                </div>

                <Dialog
                  isOpen={Boolean(state.selectedIgnoredRow())}
                  onClose={() => state.setSelectedIgnoredRowKey(null)}
                  layout="drawer-right"
                  panelClass="max-w-[720px]"
                  ariaLabel="Ignored item details"
                >
                  <Show when={state.selectedIgnoredRow()}>
                    {(rowAccessor) => <InfrastructureIgnoredRowDetails rowAccessor={rowAccessor} />}
                  </Show>
                </Dialog>
              </div>
            </Show>
          </SettingsPanel>
        </div>
      </Show>
    </div>
  );
};

export default InfrastructureInventorySection;
