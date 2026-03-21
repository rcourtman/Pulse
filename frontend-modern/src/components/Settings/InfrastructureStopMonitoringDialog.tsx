import type { Component } from 'solid-js';
import { For, Show } from 'solid-js';
import { Dialog } from '@/components/shared/Dialog';
import { getStopMonitoringSurfaces } from './infrastructureOperationsModel';
import { useInfrastructureOperationsContext } from './useInfrastructureOperationsState';

export const InfrastructureStopMonitoringDialog: Component = () => {
  const state = useInfrastructureOperationsContext();

  return (
    <Dialog
      isOpen={Boolean(state.stopMonitoringDialog())}
      onClose={() => {
        if (!state.stopMonitoringDialog()) return;
        const row = state.stopMonitoringDialog()!.row;
        if (state.getPendingInventoryAction(row.rowKey)) return;
        state.setStopMonitoringDialog(null);
      }}
      panelClass="max-w-lg"
      closeOnBackdrop={
        !state.stopMonitoringDialog() ||
        !state.getPendingInventoryAction(state.stopMonitoringDialog()!.row.rowKey)
      }
      ariaLabel="Confirm stop monitoring"
    >
      <Show when={state.stopMonitoringDialog()}>
        {(dialog) => {
          const row = () => dialog().row;
          const pending = () => state.getPendingInventoryAction(row().rowKey) === 'stop-monitoring';
          const isKubernetes = () =>
            row().capabilities.includes('kubernetes') && !row().capabilities.includes('agent');
          const affectedSurfaces = () => getStopMonitoringSurfaces(row());

          return (
            <div class="flex max-h-[90vh] flex-col">
              <div class="border-b border-border px-6 py-4">
                <h2 class="text-lg font-semibold text-base-content">Stop monitoring?</h2>
                <p class="mt-1 text-sm text-muted">
                  Pulse will remove{' '}
                  <span class="font-medium text-base-content">{dialog().subject}</span> from
                  active reporting.
                </p>
              </div>
              <div class="space-y-4 px-6 py-4">
                <div class="rounded-md border border-amber-200 bg-amber-50 px-4 py-3 text-sm text-amber-900 dark:border-amber-700 dark:bg-amber-900 dark:text-amber-100">
                  <p class="font-medium">{dialog().scopeLabel} will stop in Pulse.</p>
                  <p class="mt-1 text-xs opacity-90">
                    The remote system keeps running. Pulse will ignore future reports and move this
                    item into Ignored by Pulse until you allow reconnect.
                  </p>
                </div>
                <Show when={affectedSurfaces().length > 0}>
                  <div class="rounded-md border border-border bg-surface-hover px-4 py-3 text-sm text-muted">
                    <p class="font-medium text-base-content">
                      Pulse will stop these reporting surfaces
                    </p>
                    <div class="mt-3 grid gap-2">
                      <For each={affectedSurfaces()}>
                        {(surface) => (
                          <div class="rounded-md border border-border bg-surface px-3 py-2">
                            <div class="text-sm font-medium text-base-content">{surface.label}</div>
                            <div class="mt-1 text-xs text-muted">{surface.detail}</div>
                            <Show when={surface.idLabel && surface.idValue}>
                              <div class="mt-2 text-[11px] text-muted">
                                {surface.idLabel}:{' '}
                                <span class="font-mono text-base-content">{surface.idValue}</span>
                              </div>
                            </Show>
                          </div>
                        )}
                      </For>
                    </div>
                  </div>
                </Show>
                <div class="rounded-md border border-border bg-surface-hover px-4 py-3 text-sm text-muted">
                  <p class="font-medium text-base-content">What stays unchanged</p>
                  <p class="mt-1 text-xs">
                    {isKubernetes()
                      ? 'The cluster itself is not uninstalled or shut down.'
                      : 'The host, containers, and installed agent binaries are not uninstalled or shut down.'}
                  </p>
                </div>
              </div>
              <div class="flex flex-col-reverse gap-2 border-t border-border px-6 py-4 sm:flex-row sm:justify-end">
                <button
                  type="button"
                  onClick={() => state.setStopMonitoringDialog(null)}
                  disabled={pending()}
                  class="inline-flex min-h-10 sm:min-h-9 items-center justify-center rounded-md border border-border px-4 py-2 text-sm font-medium text-base-content hover:bg-surface-hover disabled:cursor-not-allowed disabled:opacity-60"
                >
                  Cancel
                </button>
                <button
                  type="button"
                  onClick={() =>
                    isKubernetes()
                      ? void state.handleRemoveKubernetesCluster(row())
                      : void state.handleRemoveAgent(row())
                  }
                  disabled={pending()}
                  class="inline-flex min-h-10 sm:min-h-9 items-center justify-center rounded-md bg-red-600 px-4 py-2 text-sm font-medium text-white hover:bg-red-700 disabled:cursor-not-allowed disabled:opacity-60"
                >
                  {pending() ? 'Stopping…' : 'Confirm stop monitoring'}
                </button>
              </div>
            </div>
          );
        }}
      </Show>
    </Dialog>
  );
};

export default InfrastructureStopMonitoringDialog;
