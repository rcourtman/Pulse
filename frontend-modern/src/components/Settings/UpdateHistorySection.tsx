import { Component, For, Show, createResource, createSignal } from 'solid-js';
import HistoryIcon from 'lucide-solid/icons/history';
import { UpdatesAPI, type UpdateHistoryEntry } from '@/api/updates';
import { updateStore } from '@/stores/updates';
import { Button } from '@/components/shared/Button';
import { Dialog } from '@/components/shared/Dialog';
import { LoadingSpinner } from '@/components/shared/LoadingSpinner';

const HISTORY_LIMIT = 20;

const formatVersionLabel = (version: string): string => {
  const trimmed = version.trim();
  if (!trimmed) return 'unknown';
  return trimmed.startsWith('v') ? trimmed : `v${trimmed}`;
};

const formatTimestamp = (timestamp: string): string => {
  const date = new Date(timestamp);
  return Number.isNaN(date.getTime()) ? timestamp : date.toLocaleString();
};

const actionLabel = (entry: UpdateHistoryEntry): string => {
  switch (entry.action) {
    case 'rollback':
      return 'Rollback';
    default:
      return entry.initiated_by === 'auto' ? 'Automatic update' : 'Update';
  }
};

const statusPresentation = (status: string): { label: string; className: string } => {
  switch (status) {
    case 'success':
      return { label: 'Succeeded', className: 'text-emerald-600 dark:text-emerald-400' };
    case 'failed':
      return { label: 'Failed', className: 'text-red-600 dark:text-red-400' };
    case 'in_progress':
      return { label: 'In progress', className: 'text-blue-600 dark:text-blue-400' };
    case 'rolled_back':
      return { label: 'Rolled back', className: 'text-amber-600 dark:text-amber-400' };
    case 'cancelled':
      return { label: 'Cancelled', className: 'text-muted' };
    default:
      return { label: status, className: 'text-muted' };
  }
};

// A rollback restores the backup taken before this update was applied, so it
// is only offered where that backup is still retained on disk (the backend
// clears backup_path when retention prunes it) and the update actually landed.
const canRollBack = (entry: UpdateHistoryEntry): boolean =>
  entry.action === 'update' && entry.status === 'success' && Boolean(entry.backup_path);

export const UpdateHistorySection: Component = () => {
  const [history, { refetch }] = createResource(() => UpdatesAPI.listUpdateHistory(HISTORY_LIMIT));
  const [confirmEntry, setConfirmEntry] = createSignal<UpdateHistoryEntry | null>(null);
  const [isStartingRollback, setIsStartingRollback] = createSignal(false);

  const startRollback = async () => {
    const entry = confirmEntry();
    if (!entry || isStartingRollback()) return;

    setIsStartingRollback(true);
    try {
      const accepted = await updateStore.rollbackUpdate({
        eventId: entry.event_id,
        fromVersion: updateStore.versionInfo()?.version || entry.version_to,
        toVersion: entry.version_from,
      });
      if (accepted) {
        // The global update progress watcher picks the rollback up from the
        // status stream and owns the restart/reload flow from here.
        setConfirmEntry(null);
      } else {
        // The store already surfaced the error toast; refresh in case the
        // entry state changed underneath us (e.g. backup pruned).
        void refetch();
      }
    } finally {
      setIsStartingRollback(false);
    }
  };

  return (
    <div class="space-y-4">
      <div>
        <h4 class="flex items-center gap-2 text-sm font-medium text-base-content">
          <HistoryIcon class="w-4 h-4" aria-hidden="true" />
          Update History
        </h4>
        <p class="mt-1 text-xs text-muted">
          Updates applied through Pulse, with rollback to the backup taken before each one.
        </p>
      </div>

      <Show
        when={!history.loading}
        fallback={
          <div class="flex items-center gap-2 text-sm text-muted">
            <LoadingSpinner size="sm" label="Loading update history" />
            Loading update history...
          </div>
        }
      >
        <Show
          when={!history.error}
          fallback={<p class="text-sm text-muted">Update history is unavailable right now.</p>}
        >
          <Show
            when={(history() ?? []).length > 0}
            fallback={
              <p class="text-sm text-muted">No updates have been applied through Pulse yet.</p>
            }
          >
            <div class="overflow-x-auto rounded-md border border-border">
              <table class="w-full min-w-[560px] table-fixed text-left text-sm">
                <colgroup>
                  <col class="w-[26%]" />
                  <col class="w-[20%]" />
                  <col class="w-[24%]" />
                  <col class="w-[15%]" />
                  <col class="w-[15%]" />
                </colgroup>
                <thead class="border-b border-border bg-surface-alt text-xs uppercase text-muted">
                  <tr>
                    <th class="px-3 py-2 font-semibold">When</th>
                    <th class="px-3 py-2 font-semibold">Action</th>
                    <th class="px-3 py-2 font-semibold">Version</th>
                    <th class="px-3 py-2 font-semibold">Result</th>
                    <th class="px-3 py-2 text-right font-semibold"></th>
                  </tr>
                </thead>
                <tbody class="divide-y divide-border">
                  <For each={history()}>
                    {(entry) => (
                      <tr>
                        <td class="truncate px-3 py-2 text-muted" title={entry.timestamp}>
                          {formatTimestamp(entry.timestamp)}
                        </td>
                        <td class="truncate px-3 py-2 text-base-content">{actionLabel(entry)}</td>
                        <td
                          class="truncate px-3 py-2 text-muted"
                          title={`${formatVersionLabel(entry.version_from)} to ${formatVersionLabel(entry.version_to)}`}
                        >
                          {formatVersionLabel(entry.version_from)} &rarr;{' '}
                          {formatVersionLabel(entry.version_to)}
                        </td>
                        <td
                          class={`truncate px-3 py-2 ${statusPresentation(entry.status).className}`}
                          title={entry.error?.message || ''}
                        >
                          {statusPresentation(entry.status).label}
                        </td>
                        <td class="px-3 py-2 text-right">
                          <Show when={canRollBack(entry)}>
                            <Button
                              variant="dangerOutline"
                              size="xs"
                              type="button"
                              onClick={() => setConfirmEntry(entry)}
                            >
                              Roll back
                            </Button>
                          </Show>
                        </td>
                      </tr>
                    )}
                  </For>
                </tbody>
              </table>
            </div>
          </Show>
        </Show>
      </Show>

      <Dialog
        isOpen={confirmEntry() !== null}
        onClose={() => {
          if (!isStartingRollback()) setConfirmEntry(null);
        }}
        panelClass="max-w-lg"
        closeOnBackdrop={!isStartingRollback()}
        ariaLabel="Confirm rollback"
      >
        <Show when={confirmEntry()}>
          {(entry) => (
            <div class="w-full">
              <div class="px-6 py-4 border-b border-border">
                <h2 class="text-lg font-semibold text-base-content">
                  Roll back to Pulse {formatVersionLabel(entry().version_from)}?
                </h2>
              </div>
              <div class="px-6 py-4 space-y-3 text-sm text-base-content">
                <p>
                  This restores the binary, configuration, and data from the backup taken before the
                  update to {formatVersionLabel(entry().version_to)} on{' '}
                  {formatTimestamp(entry().timestamp)}.
                </p>
                <p>
                  Settings and alert changes made since that backup will be reverted, and Pulse will
                  restart to complete the rollback.
                </p>
              </div>
              <div class="px-6 py-4 bg-surface-alt border-t border-border flex items-center justify-end gap-3">
                <Button
                  variant="ghost"
                  size="md"
                  type="button"
                  disabled={isStartingRollback()}
                  onClick={() => setConfirmEntry(null)}
                >
                  Cancel
                </Button>
                <Button
                  variant="danger"
                  size="md"
                  type="button"
                  disabled={isStartingRollback()}
                  onClick={() => void startRollback()}
                >
                  {isStartingRollback()
                    ? 'Starting rollback...'
                    : `Roll back to ${formatVersionLabel(entry().version_from)}`}
                </Button>
              </div>
            </div>
          )}
        </Show>
      </Dialog>
    </div>
  );
};
