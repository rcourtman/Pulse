import { Component, For, Show } from 'solid-js';
import { Dialog } from '@/components/shared/Dialog';
import type { Connection } from '@/api/connections';
import { CONNECTION_TYPE_LABELS, surfaceLabel } from './useConnectionsLedger';

interface ConnectionDetailDrawerProps {
  connection: () => Connection | undefined;
  onClose: () => void;
}

const formatLastSeen = (value: string | null): string => {
  if (!value) return 'No activity yet';
  const ts = Date.parse(value);
  if (Number.isNaN(ts)) return value;
  return new Date(ts).toLocaleString();
};

const formatErrorAt = (value: string): string => {
  const ts = Date.parse(value);
  if (Number.isNaN(ts)) return value;
  return new Date(ts).toLocaleString();
};

export const ConnectionDetailDrawer: Component<ConnectionDetailDrawerProps> = (props) => {
  return (
    <Dialog
      isOpen={Boolean(props.connection())}
      onClose={props.onClose}
      layout="drawer-right"
      panelClass="max-w-[560px]"
      ariaLabel="Connection details"
    >
      <Show when={props.connection()}>
        {(accessor) => {
          const connection = accessor();
          const typeLabel = CONNECTION_TYPE_LABELS[connection.type] ?? connection.type;
          const activeScopeKeys = Object.keys(connection.scope ?? {}).filter(
            (key) => connection.scope?.[key],
          );
          const inactiveScopeKeys = (connection.surfaces ?? []).filter(
            (key) => !activeScopeKeys.includes(key),
          );
          return (
            <div class="flex h-full flex-col">
              <div class="flex items-start justify-between gap-4 border-b border-border px-5 py-4">
                <div class="min-w-0 space-y-1">
                  <div class="break-words text-base font-semibold text-base-content">
                    {connection.name || connection.address || connection.id}
                  </div>
                  <div class="text-xs text-muted">{typeLabel}</div>
                </div>
                <button
                  type="button"
                  onClick={props.onClose}
                  class="inline-flex items-center rounded-md border border-border px-2.5 py-1 text-xs font-medium text-base-content transition-colors hover:bg-surface-hover"
                >
                  Close
                </button>
              </div>

              <div class="flex-1 overflow-y-auto px-5 py-4">
                <dl class="space-y-4 text-sm">
                  <div>
                    <dt class="text-xs font-semibold uppercase tracking-wide text-muted">
                      Address
                    </dt>
                    <dd class="mt-1 break-all text-base-content">{connection.address || '—'}</dd>
                  </div>

                  <div>
                    <dt class="text-xs font-semibold uppercase tracking-wide text-muted">State</dt>
                    <dd class="mt-1 text-base-content">
                      <div class="font-medium capitalize">{connection.state}</div>
                      <Show when={connection.stateReason}>
                        <div class="mt-0.5 text-xs text-muted">{connection.stateReason}</div>
                      </Show>
                    </dd>
                  </div>

                  <div>
                    <dt class="text-xs font-semibold uppercase tracking-wide text-muted">
                      Enabled
                    </dt>
                    <dd class="mt-1 text-base-content">
                      {connection.enabled ? 'Yes' : 'No (paused)'}
                    </dd>
                  </div>

                  <div>
                    <dt class="text-xs font-semibold uppercase tracking-wide text-muted">
                      Surfaces collected
                    </dt>
                    <dd class="mt-1 flex flex-wrap gap-1.5">
                      <Show
                        when={activeScopeKeys.length > 0}
                        fallback={
                          <span class="text-xs text-muted">
                            No surfaces currently enabled for collection.
                          </span>
                        }
                      >
                        <For each={activeScopeKeys}>
                          {(key) => (
                            <span class="inline-flex items-center rounded-full border border-border bg-surface px-2 py-0.5 text-xs font-medium text-base-content">
                              {surfaceLabel(key)}
                            </span>
                          )}
                        </For>
                      </Show>
                    </dd>
                    <Show when={inactiveScopeKeys.length > 0}>
                      <dd class="mt-2 flex flex-wrap gap-1.5">
                        <For each={inactiveScopeKeys}>
                          {(key) => (
                            <span class="inline-flex items-center rounded-full border border-dashed border-border bg-surface-alt px-2 py-0.5 text-xs font-medium text-muted">
                              {surfaceLabel(key)} (paused)
                            </span>
                          )}
                        </For>
                      </dd>
                    </Show>
                  </div>

                  <div>
                    <dt class="text-xs font-semibold uppercase tracking-wide text-muted">
                      Last activity
                    </dt>
                    <dd class="mt-1 text-base-content">{formatLastSeen(connection.lastSeen)}</dd>
                  </div>

                  <Show when={connection.lastError}>
                    {(errorAccessor) => {
                      const err = errorAccessor();
                      return (
                        <div>
                          <dt class="text-xs font-semibold uppercase tracking-wide text-muted">
                            Last error
                          </dt>
                          <dd class="mt-1 space-y-1">
                            <div class="break-words text-rose-700 dark:text-rose-300">
                              {err.message}
                            </div>
                            <div class="text-xs text-muted">{formatErrorAt(err.at)}</div>
                          </dd>
                        </div>
                      );
                    }}
                  </Show>

                  <div>
                    <dt class="text-xs font-semibold uppercase tracking-wide text-muted">
                      Source
                    </dt>
                    <dd class="mt-1 text-base-content capitalize">{connection.source}</dd>
                  </div>
                </dl>
              </div>
            </div>
          );
        }}
      </Show>
    </Dialog>
  );
};
