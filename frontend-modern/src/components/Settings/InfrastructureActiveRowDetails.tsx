import type { Accessor, Component } from 'solid-js';
import { For, Show } from 'solid-js';
import { copyToClipboard } from '@/utils/clipboard';
import { formatAbsoluteTime, formatRelativeTime } from '@/utils/format';
import { notificationStore } from '@/stores/notifications';
import {
  getAgentCapabilityBadgeClass,
  getAgentCapabilityLabel,
} from '@/utils/agentCapabilityPresentation';
import {
  getUnifiedAgentClipboardCopyErrorMessage,
  getUnifiedAgentClipboardCopySuccessMessage,
} from '@/utils/unifiedAgentInventoryPresentation';
import {
  createSurfaceScopedRow,
  getRowSurfaceBreakdown,
  type UnifiedAgentRow,
} from './infrastructureOperationsModel';
import { useInfrastructureOperationsContext } from './useInfrastructureOperationsState';

export interface InfrastructureActiveRowDetailsProps {
  rowAccessor: Accessor<UnifiedAgentRow>;
}

export const InfrastructureActiveRowDetails: Component<InfrastructureActiveRowDetailsProps> = (
  props,
) => {
  const state = useInfrastructureOperationsContext();
  const row = () => props.rowAccessor();
  const isKubernetes = () =>
    row().capabilities.includes('kubernetes') && !row().capabilities.includes('agent');
  const resolvedAgentId = () => row().agentId || '';
  const assignment = () =>
    resolvedAgentId() ? state.assignmentByAgent().get(resolvedAgentId()) : undefined;
  const isScopeUpdating = () =>
    resolvedAgentId() ? Boolean(state.pendingScopeUpdates()[resolvedAgentId()]) : false;
  const agentName = () => row().displayName || row().hostname || row().name;
  const surfaces = () => getRowSurfaceBreakdown(row());

  return (
    <div id={`agent-details-${row().rowKey}`} class="flex h-full flex-col overflow-y-auto">
      <div class="border-b border-border bg-surface-alt px-4 py-4">
        <div class="flex items-start justify-between gap-4">
          <div class="min-w-0 space-y-3">
            <div>
              <div class="text-[11px] font-semibold uppercase tracking-[0.18em] text-muted">
                Selected reporting item
              </div>
              <div class="mt-2 text-lg font-semibold text-base-content">{row().name}</div>
              <Show when={row().displayName && row().hostname && row().displayName !== row().hostname}>
                <div class="mt-1 text-xs text-muted">{row().hostname}</div>
              </Show>
              <div class="mt-2 text-sm text-base-content">
                Use surface controls to stop specific reporting without removing the machine.
              </div>
            </div>
            <div class="flex flex-wrap items-center gap-2 text-xs">
              <For each={row().capabilities}>
                {(cap) => (
                  <span
                    class={`inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium ${getAgentCapabilityBadgeClass(cap)}`}
                  >
                    {getAgentCapabilityLabel(cap)}
                  </span>
                )}
              </For>
              <Show when={row().isOutdatedBinary}>
                <span class="inline-flex items-center rounded-full bg-amber-100 px-2 py-0.5 text-xs font-medium text-amber-800 dark:bg-amber-900 dark:text-amber-200">
                  Outdated
                </span>
              </Show>
              <Show when={row().linkedNodeId}>
                <span class="inline-flex items-center rounded-full bg-blue-100 px-2 py-0.5 text-xs font-medium text-blue-800 dark:bg-blue-900 dark:text-blue-300">
                  Linked
                </span>
              </Show>
            </div>
          </div>
          <button
            type="button"
            onClick={() => state.setExpandedRowKey(null)}
            class="rounded-md p-1 hover:bg-surface-hover hover:text-base-content"
            aria-label="Close"
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
        </div>
      </div>

      <div class="space-y-4 p-4 text-sm text-muted">
        <div class="rounded-lg border border-border bg-surface-alt px-4 py-4">
          <div class="text-xs font-semibold uppercase tracking-wide text-muted">Machine overview</div>
          <div class="mt-3 grid gap-3 md:grid-cols-2 xl:grid-cols-1">
            <div class="space-y-2 text-xs text-muted">
              <div>
                Item ID: <span class="font-mono text-base-content">{row().id}</span>
              </div>
              <Show when={row().agentActionId && row().agentActionId !== row().id}>
                <div>
                  Agent ID: <span class="font-mono text-base-content">{row().agentActionId}</span>
                </div>
              </Show>
              <Show when={row().dockerActionId && row().dockerActionId !== row().id}>
                <div>
                  Container Agent ID:{' '}
                  <span class="font-mono text-base-content">{row().dockerActionId}</span>
                </div>
              </Show>
              <Show when={row().kubernetesActionId && row().kubernetesActionId !== row().id}>
                <div>
                  Cluster ID:{' '}
                  <span class="font-mono text-base-content">{row().kubernetesActionId}</span>
                </div>
              </Show>
              <Show when={row().agentId && row().agentId !== row().id}>
                <div>
                  Reporting agent ID:{' '}
                  <span class="font-mono text-base-content">{row().agentId}</span>
                </div>
              </Show>
              <Show when={row().linkedNodeId}>
                <div>
                  Linked node ID:{' '}
                  <span class="font-mono text-base-content">{row().linkedNodeId}</span>
                </div>
              </Show>
            </div>
            <div class="space-y-2 text-xs text-muted">
              <Show when={row().lastSeen}>
                <div>
                  Last seen {formatRelativeTime(row().lastSeen!)} ({formatAbsoluteTime(row().lastSeen!)})
                </div>
              </Show>
              <Show when={row().scope.category !== 'na'}>
                <div>
                  <div class="mb-1">Scope profile</div>
                  <Show
                    when={resolvedAgentId()}
                    fallback={
                      <span class="text-base-content" title={row().scope.detail}>
                        {row().scope.label}
                      </span>
                    }
                  >
                    <Show
                      when={isKubernetes()}
                      fallback={
                        <Show
                          when={state.profiles().length > 0}
                          fallback={
                            <span class="text-base-content" title={row().scope.detail}>
                              {row().scope.label}
                            </span>
                          }
                        >
                          <div class="flex items-center gap-2">
                            <select
                              value={assignment()?.profile_id || ''}
                              onChange={(event) => {
                                const nextValue = event.currentTarget.value;
                                const currentValue = assignment()?.profile_id || '';
                                if (nextValue === currentValue) {
                                  return;
                                }
                                void state.updateScopeAssignment(
                                  resolvedAgentId(),
                                  nextValue || null,
                                  agentName(),
                                );
                              }}
                              disabled={isScopeUpdating()}
                              class="min-h-10 sm:min-h-9 rounded-md border border-border bg-surface px-2.5 py-1.5 text-sm text-base-content shadow-sm focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500 disabled:cursor-not-allowed disabled:opacity-60 dark:focus:border-blue-400 dark:focus:ring-blue-800"
                            >
                              <option value="">Default (Auto-detect)</option>
                              <Show
                                when={
                                  assignment()?.profile_id &&
                                  !state.profileById().has(assignment()!.profile_id)
                                }
                              >
                                <option value={assignment()!.profile_id}>
                                  {state.getProfileOptionLabel(assignment()!.profile_id)}
                                </option>
                              </Show>
                              <For each={state.profiles()}>
                                {(profile) => (
                                  <option value={profile.id}>
                                    {state.getProfileOptionLabel(profile.id)}
                                  </option>
                                )}
                              </For>
                            </select>
                            <Show when={isScopeUpdating()}>
                              <span class="text-[10px] text-muted">Updating…</span>
                            </Show>
                          </div>
                        </Show>
                      }
                    >
                      <span class="text-base-content" title={row().scope.detail}>
                        {row().scope.label}
                      </span>
                    </Show>
                  </Show>
                </div>
              </Show>
              <Show
                when={
                  row().kubernetesInfo &&
                  (row().kubernetesInfo?.server ||
                    row().kubernetesInfo?.context ||
                    row().kubernetesInfo?.tokenName)
                }
              >
                <div class="space-y-1 pt-1">
                  <div class="text-[11px] font-semibold uppercase tracking-wide text-muted">
                    Kubernetes connection
                  </div>
                  <Show when={row().kubernetesInfo?.server}>
                    <div>
                      Server: <span class="text-base-content">{row().kubernetesInfo?.server}</span>
                    </div>
                  </Show>
                  <Show when={row().kubernetesInfo?.context}>
                    <div>
                      Context: <span class="text-base-content">{row().kubernetesInfo?.context}</span>
                    </div>
                  </Show>
                  <Show when={row().kubernetesInfo?.tokenName}>
                    <div>
                      Token: <span class="text-base-content">{row().kubernetesInfo?.tokenName}</span>
                    </div>
                  </Show>
                </div>
              </Show>
            </div>
          </div>
          <Show when={assignment()}>
            <div class="mt-3 border-t border-border pt-3">
              <div class="text-[11px] text-amber-600 dark:text-amber-400">
                Restart required to apply scope changes.
              </div>
              <button
                type="button"
                onClick={() => void state.handleResetScope(resolvedAgentId(), agentName() || resolvedAgentId())}
                class="mt-2 text-left text-xs text-blue-600 hover:text-blue-800 dark:text-blue-400 dark:hover:text-blue-300"
              >
                Reset to default
              </button>
            </div>
          </Show>
        </div>

        <Show when={surfaces().length > 0}>
          <div class="rounded-lg border border-border bg-surface-alt px-4 py-4">
            <div class="flex flex-col gap-1">
              <div class="text-xs font-semibold uppercase tracking-wide text-muted">Surface controls</div>
              <div class="text-xs text-muted">
                Stop a specific surface. Other surfaces keep reporting.
              </div>
            </div>
            <div class="mt-3 overflow-hidden rounded-md border border-border">
              <div class="grid grid-cols-[minmax(0,1.1fr)_minmax(0,1.35fr)_minmax(0,0.9fr)_auto] gap-0 border-b border-border bg-surface px-3 py-2 text-[11px] font-semibold uppercase tracking-wide text-muted">
                <div>Surface</div>
                <div>What Pulse receives</div>
                <div>ID</div>
                <div class="text-right">Control</div>
              </div>
              <For each={surfaces()}>
                {(surface) => (
                  <div class="grid grid-cols-[minmax(0,1.1fr)_minmax(0,1.35fr)_minmax(0,0.9fr)_auto] gap-0 border-b border-border bg-surface-alt px-3 py-2 text-xs last:border-b-0">
                    <div class="pr-3 font-medium text-base-content">{surface.label}</div>
                    <div class="pr-3 text-muted">{surface.detail}</div>
                    <div class="text-muted">
                      <Show
                        when={surface.idLabel && surface.idValue}
                        fallback={<span class="text-muted">Not separately addressed</span>}
                      >
                        <div class="space-y-1">
                          <div class="text-[11px]">{surface.idLabel}</div>
                          <div class="font-mono text-base-content">{surface.idValue}</div>
                        </div>
                      </Show>
                    </div>
                    <div class="pl-3 text-right">
                      <Show
                        when={
                          surface.key === 'docker' ||
                          surface.key === 'agent' ||
                          surface.key === 'kubernetes'
                        }
                        fallback={<span class="text-[11px] text-muted">Managed with host telemetry</span>}
                      >
                        <button
                          type="button"
                          data-row-action
                          onClick={(event) => {
                            event.stopPropagation();
                            state.openStopMonitoringDialog(
                              createSurfaceScopedRow(
                                row(),
                                surface.key as 'agent' | 'docker' | 'kubernetes',
                              ),
                            );
                          }}
                          class="inline-flex min-h-9 items-center rounded-md px-2.5 py-1.5 text-xs font-medium text-red-600 hover:bg-red-50 hover:text-red-900 dark:text-red-400 dark:hover:bg-red-900 dark:hover:text-red-300"
                        >
                          Stop this surface
                        </button>
                      </Show>
                    </div>
                  </div>
                )}
              </For>
            </div>
          </div>
        </Show>

        <div class="rounded-lg border border-border bg-surface-alt px-4 py-4">
          <div class="text-xs font-semibold uppercase tracking-wide text-muted">Machine actions</div>
          <div class="mt-2 text-xs text-muted">Machine-level utilities.</div>
          <div class="mt-4 flex flex-col gap-2">
            <Show when={!isKubernetes()}>
              <button
                type="button"
                onClick={async () => {
                  const success = await copyToClipboard(
                    state.getPlatformUninstallCommand(row().upgradePlatform, row()),
                  );
                  if (success) {
                    notificationStore.success(getUnifiedAgentClipboardCopySuccessMessage());
                  } else {
                    notificationStore.error(getUnifiedAgentClipboardCopyErrorMessage());
                  }
                }}
                class="rounded-md border border-border px-3 py-2 text-left text-xs text-slate-600 hover:bg-surface hover:text-base-content"
              >
                Copy uninstall command
              </button>
            </Show>
            <Show when={row().isOutdatedBinary}>
              <button
                type="button"
                onClick={async () => {
                  const success = await copyToClipboard(state.getUpgradeCommand(row()));
                  if (success) {
                    notificationStore.success(getUnifiedAgentClipboardCopySuccessMessage());
                  } else {
                    notificationStore.error(getUnifiedAgentClipboardCopyErrorMessage());
                  }
                }}
                class="rounded-md border border-amber-200 bg-amber-50 px-3 py-2 text-left text-xs text-amber-700 hover:bg-amber-100 hover:text-amber-900 dark:border-amber-800 dark:bg-amber-950/40 dark:text-amber-300 dark:hover:bg-amber-900/60 dark:hover:text-amber-200"
              >
                Copy upgrade command
              </button>
            </Show>
            <div class="rounded-md border border-blue-200 bg-blue-50 px-3 py-3 text-xs text-blue-900 dark:border-blue-800 dark:bg-blue-950/40 dark:text-blue-100">
              Use surface controls above to stop reporting without uninstalling.
            </div>
          </div>
        </div>
      </div>
    </div>
  );
};

export default InfrastructureActiveRowDetails;
