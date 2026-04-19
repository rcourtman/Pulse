import { Component, For, Show, createEffect, createSignal, onCleanup } from 'solid-js';
import { ConnectionsAPI, type Connection } from '@/api/connections';
import { CONNECTION_TYPE_LABELS, surfaceLabel } from './useConnectionsLedger';

interface ConnectionDetailPanelProps {
  connection: () => Connection | undefined;
  onMutated?: () => void;
  onEdit?: (connection: Connection) => void;
  onRemoved?: () => void;
}

const EDITABLE_CONNECTION_TYPES: readonly Connection['type'][] = [
  'pve',
  'pbs',
  'pmg',
  'vmware',
  'truenas',
];

const REMOVE_CONFIRM_TIMEOUT_MS = 4000;

const errorMessage = (err: unknown): string => {
  if (err instanceof Error && err.message) return err.message;
  if (typeof err === 'string' && err.trim()) return err;
  return 'Something went wrong.';
};

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

export const ConnectionDetailPanel: Component<ConnectionDetailPanelProps> = (props) => {
  const [pendingAction, setPendingAction] = createSignal<'pause' | 'remove' | null>(null);
  const [actionError, setActionError] = createSignal<string | null>(null);
  const [confirmingRemove, setConfirmingRemove] = createSignal(false);
  let confirmTimer: number | undefined;

  const clearConfirmTimer = () => {
    if (confirmTimer !== undefined) {
      window.clearTimeout(confirmTimer);
      confirmTimer = undefined;
    }
  };

  // Reset transient action state whenever the selected connection changes.
  createEffect(() => {
    props.connection();
    setPendingAction(null);
    setActionError(null);
    setConfirmingRemove(false);
    clearConfirmTimer();
  });

  onCleanup(clearConfirmTimer);

  const handlePauseToggle = async (connection: Connection) => {
    setActionError(null);
    setPendingAction('pause');
    try {
      await ConnectionsAPI.setEnabled(connection.id, !connection.enabled);
      props.onMutated?.();
    } catch (err) {
      setActionError(errorMessage(err));
    } finally {
      setPendingAction(null);
    }
  };

  const handleRemoveClick = async (connection: Connection) => {
    setActionError(null);
    if (!confirmingRemove()) {
      setConfirmingRemove(true);
      clearConfirmTimer();
      confirmTimer = window.setTimeout(() => {
        setConfirmingRemove(false);
        confirmTimer = undefined;
      }, REMOVE_CONFIRM_TIMEOUT_MS);
      return;
    }
    clearConfirmTimer();
    setConfirmingRemove(false);
    setPendingAction('remove');
    try {
      await ConnectionsAPI.remove(connection.id);
      props.onMutated?.();
      props.onRemoved?.();
    } catch (err) {
      setActionError(errorMessage(err));
    } finally {
      setPendingAction(null);
    }
  };

  return (
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
        const canPause = connection.capabilities.supportsPause;
        const canRemove = connection.type !== 'docker' && connection.type !== 'kubernetes';
        const canEdit =
          Boolean(props.onEdit) && EDITABLE_CONNECTION_TYPES.includes(connection.type);
        const pauseLabel = connection.enabled ? 'Pause' : 'Resume';
        const pauseBusy = () => pendingAction() === 'pause';
        const removeBusy = () => pendingAction() === 'remove';
        const anyBusy = () => pendingAction() !== null;
        return (
          <div class="rounded-lg border border-border bg-surface">
            <div class="px-5 py-4">
              <dl class="grid gap-4 text-sm sm:grid-cols-2">
                <div>
                  <dt class="text-xs font-semibold uppercase tracking-wide text-muted">Type</dt>
                  <dd class="mt-1 text-base-content">{typeLabel}</dd>
                </div>

                <div>
                  <dt class="text-xs font-semibold uppercase tracking-wide text-muted">Address</dt>
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
                  <dt class="text-xs font-semibold uppercase tracking-wide text-muted">Enabled</dt>
                  <dd class="mt-1 text-base-content">
                    {connection.enabled ? 'Yes' : 'No (paused)'}
                  </dd>
                </div>

                <div class="sm:col-span-2">
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

                <div>
                  <dt class="text-xs font-semibold uppercase tracking-wide text-muted">Source</dt>
                  <dd class="mt-1 text-base-content capitalize">{connection.source}</dd>
                </div>

                <Show when={connection.lastError}>
                  {(errorAccessor) => {
                    const err = errorAccessor();
                    return (
                      <div class="sm:col-span-2">
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
              </dl>
            </div>

            <Show when={canEdit || canPause || canRemove}>
              <div class="space-y-2 border-t border-border px-5 py-4">
                <Show when={actionError()}>
                  <div
                    role="alert"
                    class="rounded-md border border-rose-300 bg-rose-50 px-3 py-2 text-xs text-rose-800 dark:border-rose-900 dark:bg-rose-950 dark:text-rose-200"
                  >
                    {actionError()}
                  </div>
                </Show>
                <div class="flex flex-wrap items-center justify-end gap-2">
                  <Show when={canEdit}>
                    <button
                      type="button"
                      disabled={anyBusy()}
                      onClick={() => props.onEdit?.(connection)}
                      class="inline-flex items-center rounded-md border border-border px-3 py-1.5 text-sm font-medium text-base-content transition-colors hover:bg-surface-hover disabled:cursor-not-allowed disabled:opacity-60"
                    >
                      Edit
                    </button>
                  </Show>
                  <Show when={canPause}>
                    <button
                      type="button"
                      disabled={anyBusy()}
                      onClick={() => void handlePauseToggle(connection)}
                      class="inline-flex items-center rounded-md border border-border px-3 py-1.5 text-sm font-medium text-base-content transition-colors hover:bg-surface-hover disabled:cursor-not-allowed disabled:opacity-60"
                    >
                      {pauseBusy() ? 'Working…' : pauseLabel}
                    </button>
                  </Show>
                  <Show when={canRemove}>
                    <button
                      type="button"
                      disabled={anyBusy()}
                      onClick={() => void handleRemoveClick(connection)}
                      class={
                        confirmingRemove()
                          ? 'inline-flex items-center rounded-md bg-rose-600 px-3 py-1.5 text-sm font-medium text-white transition-colors hover:bg-rose-700 disabled:cursor-not-allowed disabled:opacity-60'
                          : 'inline-flex items-center rounded-md border border-rose-300 px-3 py-1.5 text-sm font-medium text-rose-700 transition-colors hover:bg-rose-50 disabled:cursor-not-allowed disabled:opacity-60 dark:border-rose-900 dark:text-rose-300 dark:hover:bg-rose-950'
                      }
                    >
                      {removeBusy()
                        ? 'Removing…'
                        : confirmingRemove()
                          ? 'Click again to confirm'
                          : 'Remove'}
                    </button>
                  </Show>
                </div>
                <Show when={connection.type === 'agent'}>
                  <p class="text-xs text-muted">
                    Removing stops recording this agent. Run the uninstall command on the host to
                    fully detach; history is retained.
                  </p>
                </Show>
              </div>
            </Show>
          </div>
        );
      }}
    </Show>
  );
};
