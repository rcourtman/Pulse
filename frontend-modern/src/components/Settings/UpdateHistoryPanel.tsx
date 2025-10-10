import { createSignal, onMount, For, Show, createMemo } from 'solid-js';
import { UpdatesAPI, type UpdateHistoryEntry } from '@/api/updates';
import { Card } from '@/components/shared/Card';
import { SectionHeader } from '@/components/shared/SectionHeader';

export function UpdateHistoryPanel() {
  const [history, setHistory] = createSignal<UpdateHistoryEntry[]>([]);
  const [loading, setLoading] = createSignal(true);
  const [error, setError] = createSignal<string | null>(null);
  const [filterStatus, setFilterStatus] = createSignal<string>('all');

  const filteredHistory = createMemo(() => {
    const filter = filterStatus();
    if (filter === 'all') return history();
    return history().filter((entry) => entry.status === filter);
  });

  const loadHistory = async () => {
    setLoading(true);
    setError(null);
    try {
      const data = await UpdatesAPI.getUpdateHistory(50); // Get last 50 updates
      setHistory(data);
    } catch (err) {
      console.error('Failed to load update history:', err);
      setError('Failed to load update history');
    } finally {
      setLoading(false);
    }
  };

  onMount(() => {
    loadHistory();
  });

  const getStatusBadge = (status: string) => {
    switch (status) {
      case 'success':
        return (
          <span class="px-2 py-1 text-xs font-medium bg-green-100 dark:bg-green-900/30 text-green-800 dark:text-green-200 rounded">
            Success
          </span>
        );
      case 'failed':
        return (
          <span class="px-2 py-1 text-xs font-medium bg-red-100 dark:bg-red-900/30 text-red-800 dark:text-red-200 rounded">
            Failed
          </span>
        );
      case 'in_progress':
        return (
          <span class="px-2 py-1 text-xs font-medium bg-blue-100 dark:bg-blue-900/30 text-blue-800 dark:text-blue-200 rounded">
            In Progress
          </span>
        );
      case 'rolled_back':
        return (
          <span class="px-2 py-1 text-xs font-medium bg-orange-100 dark:bg-orange-900/30 text-orange-800 dark:text-orange-200 rounded">
            Rolled Back
          </span>
        );
      case 'cancelled':
        return (
          <span class="px-2 py-1 text-xs font-medium bg-gray-100 dark:bg-gray-900/30 text-gray-800 dark:text-gray-200 rounded">
            Cancelled
          </span>
        );
      default:
        return <span class="px-2 py-1 text-xs font-medium bg-gray-100 dark:bg-gray-900/30 text-gray-800 dark:text-gray-200 rounded">{status}</span>;
    }
  };

  const formatDate = (timestamp: string) => {
    const date = new Date(timestamp);
    return date.toLocaleString();
  };

  const formatDuration = (durationMs: number) => {
    if (durationMs === 0) return '-';
    const seconds = Math.floor(durationMs / 1000);
    const minutes = Math.floor(seconds / 60);
    if (minutes > 0) {
      return `${minutes}m ${seconds % 60}s`;
    }
    return `${seconds}s`;
  };

  return (
    <div class="space-y-6">
      <SectionHeader title="Update History" />

      <Card>
        <div class="p-6">
          {/* Filter Controls */}
          <div class="mb-4 flex items-center gap-2">
            <label class="text-sm font-medium text-gray-700 dark:text-gray-300">
              Filter by status:
            </label>
            <select
              value={filterStatus()}
              onChange={(e) => setFilterStatus(e.currentTarget.value)}
              class="px-3 py-1.5 text-sm border border-gray-300 dark:border-gray-600 rounded bg-white dark:bg-gray-800 text-gray-900 dark:text-gray-100"
            >
              <option value="all">All</option>
              <option value="success">Success</option>
              <option value="failed">Failed</option>
              <option value="in_progress">In Progress</option>
              <option value="rolled_back">Rolled Back</option>
              <option value="cancelled">Cancelled</option>
            </select>
          </div>

          {/* Loading State */}
          <Show when={loading()}>
            <div class="text-center py-8 text-gray-500 dark:text-gray-400">
              Loading update history...
            </div>
          </Show>

          {/* Error State */}
          <Show when={error()}>
            <div class="text-center py-8">
              <div class="text-red-600 dark:text-red-400">{error()}</div>
              <button
                onClick={loadHistory}
                class="mt-2 px-4 py-2 text-sm font-medium text-white bg-blue-600 hover:bg-blue-700 rounded"
              >
                Retry
              </button>
            </div>
          </Show>

          {/* Empty State */}
          <Show when={!loading() && !error() && filteredHistory().length === 0}>
            <div class="text-center py-8 text-gray-500 dark:text-gray-400">
              No update history available
            </div>
          </Show>

          {/* History Table */}
          <Show when={!loading() && !error() && filteredHistory().length > 0}>
            <div class="overflow-x-auto">
              <table class="min-w-full divide-y divide-gray-200 dark:divide-gray-700">
                <thead class="bg-gray-50 dark:bg-gray-800">
                  <tr>
                    <th class="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">
                      Timestamp
                    </th>
                    <th class="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">
                      Action
                    </th>
                    <th class="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">
                      Version
                    </th>
                    <th class="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">
                      Status
                    </th>
                    <th class="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">
                      Duration
                    </th>
                    <th class="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">
                      Deployment
                    </th>
                  </tr>
                </thead>
                <tbody class="bg-white dark:bg-gray-900 divide-y divide-gray-200 dark:divide-gray-700">
                  <For each={filteredHistory()}>
                    {(entry) => (
                      <tr class="hover:bg-gray-50 dark:hover:bg-gray-800">
                        <td class="px-4 py-3 whitespace-nowrap text-sm text-gray-900 dark:text-gray-100">
                          {formatDate(entry.timestamp)}
                        </td>
                        <td class="px-4 py-3 whitespace-nowrap text-sm">
                          <span class="capitalize">{entry.action}</span>
                        </td>
                        <td class="px-4 py-3 whitespace-nowrap text-sm font-mono text-gray-700 dark:text-gray-300">
                          {entry.version_from} → {entry.version_to}
                        </td>
                        <td class="px-4 py-3 whitespace-nowrap text-sm">
                          {getStatusBadge(entry.status)}
                        </td>
                        <td class="px-4 py-3 whitespace-nowrap text-sm text-gray-600 dark:text-gray-400">
                          {formatDuration(entry.duration_ms)}
                        </td>
                        <td class="px-4 py-3 whitespace-nowrap text-sm text-gray-600 dark:text-gray-400">
                          <span class="capitalize">{entry.deployment_type}</span>
                        </td>
                      </tr>
                    )}
                  </For>
                </tbody>
              </table>
            </div>

            {/* Details Section - Could be expanded with more info */}
            <div class="mt-4 text-sm text-gray-500 dark:text-gray-400">
              Showing {filteredHistory().length} {filteredHistory().length === 1 ? 'entry' : 'entries'}
            </div>
          </Show>
        </div>
      </Card>
    </div>
  );
}
