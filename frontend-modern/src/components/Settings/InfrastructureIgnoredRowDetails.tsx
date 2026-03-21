import type { Accessor, Component } from 'solid-js';
import {
  getReconnectActionLabel,
  getCapabilitySurfaceLabel,
  type UnifiedAgentRow,
} from './infrastructureOperationsModel';
import { useInfrastructureOperationsContext } from './useInfrastructureOperationsState';

export interface InfrastructureIgnoredRowDetailsProps {
  rowAccessor: Accessor<UnifiedAgentRow>;
}

export const InfrastructureIgnoredRowDetails: Component<InfrastructureIgnoredRowDetailsProps> = (
  props,
) => {
  const state = useInfrastructureOperationsContext();
  const row = () => props.rowAccessor();
  const pendingAction = () => state.getPendingInventoryAction(row().rowKey);
  const isAllowingReconnect = () => pendingAction() === 'allow-reconnect';
  const reconnectLabel = () => getReconnectActionLabel(row());
  const blockedId = () =>
    row().dockerActionId || row().kubernetesActionId || row().agentActionId || row().id;

  return (
    <div
      id={`ignored-details-${row().rowKey}`}
      class="flex h-full flex-col overflow-y-auto bg-amber-50/70 dark:bg-amber-950/30"
    >
      <div class="border-b border-amber-200 bg-amber-100/80 px-4 py-4 dark:border-amber-800 dark:bg-amber-900/30">
        <div class="flex items-start justify-between gap-4">
          <div class="min-w-0">
            <div class="text-[11px] font-semibold uppercase tracking-[0.18em] text-amber-900 dark:text-amber-100">
              Selected ignored item
            </div>
            <div class="mt-2 text-lg font-semibold text-base-content">{row().name}</div>
            <div class="mt-2 text-xs font-medium uppercase tracking-wide text-amber-800 dark:text-amber-200">
              Ignored by Pulse
            </div>
            <div class="mt-2 text-sm text-amber-950 dark:text-amber-100">
              Pulse is blocking reports from this surface.
            </div>
          </div>
          <button
            type="button"
            onClick={() => state.setSelectedIgnoredRowKey(null)}
            class="rounded-md p-1 hover:bg-amber-200/70 hover:text-base-content dark:hover:bg-amber-800/50"
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
        <div class="rounded-lg border border-amber-200/80 bg-white/70 px-4 py-4 dark:border-amber-800/80 dark:bg-amber-950/20">
          <div class="text-xs font-semibold uppercase tracking-wide text-muted">Ignored surface</div>
          <div class="mt-3 overflow-hidden rounded-md border border-amber-200/80 dark:border-amber-800/80">
            <div class="grid grid-cols-[minmax(0,1.2fr)_minmax(0,1.5fr)_minmax(0,1fr)_auto] gap-0 border-b border-amber-200/80 bg-white/80 px-3 py-2 text-[11px] font-semibold uppercase tracking-wide text-amber-900 dark:border-amber-800/80 dark:bg-amber-950/30 dark:text-amber-100">
              <div>Ignored surface</div>
              <div>What Pulse is ignoring</div>
              <div>ID</div>
              <div class="text-right">Recovery</div>
            </div>
            <div class="grid grid-cols-[minmax(0,1.2fr)_minmax(0,1.5fr)_minmax(0,1fr)_auto] gap-0 bg-transparent px-3 py-2 text-xs">
              <div class="pr-3 font-medium text-base-content">
                {row().capabilities.map(getCapabilitySurfaceLabel).join(', ')}
              </div>
              <div class="pr-3 text-muted">
                Pulse will ignore new reports for this surface until reconnect is allowed.
              </div>
              <div class="pr-3 text-muted">
                <span class="font-mono text-base-content">{blockedId()}</span>
              </div>
              <div class="text-right text-[11px] text-muted">Ready to return</div>
            </div>
          </div>
        </div>

        <div class="rounded-lg border border-amber-200/80 bg-white/70 px-4 py-4 dark:border-amber-800/80 dark:bg-amber-950/20">
          <div class="text-xs font-semibold uppercase tracking-wide text-muted">Recovery action</div>
          <div class="mt-2 text-xs text-muted">Allow this blocked ID to report again.</div>
          <div class="mt-4 flex flex-col gap-3">
            <button
              onClick={() =>
                row().capabilities.includes('docker')
                  ? void state.handleAllowDockerReconnect(row())
                  : row().capabilities.includes('kubernetes')
                    ? void state.handleAllowKubernetesReconnect(row())
                    : void state.handleAllowHostReconnect(row())
              }
              disabled={Boolean(pendingAction())}
              class="inline-flex min-h-10 sm:min-h-9 items-center justify-center rounded-md bg-white px-3 py-2 text-sm font-medium text-blue-600 shadow-sm ring-1 ring-border hover:bg-blue-50 hover:text-blue-900 disabled:cursor-not-allowed disabled:opacity-60 dark:bg-slate-900 dark:text-blue-400 dark:ring-slate-700 dark:hover:bg-blue-900 dark:hover:text-blue-300"
            >
              {isAllowingReconnect() ? 'Allowing reconnect…' : reconnectLabel()}
            </button>
            <div class="text-xs text-muted">
              This only changes Pulse. It does not reinstall software.
            </div>
          </div>
        </div>
      </div>
    </div>
  );
};

export default InfrastructureIgnoredRowDetails;
