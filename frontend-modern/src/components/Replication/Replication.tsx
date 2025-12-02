import type { Component } from 'solid-js';
import { Show, For, createMemo } from 'solid-js';
import { ProxmoxSectionNav } from '@/components/Proxmox/ProxmoxSectionNav';
import { useWebSocket } from '@/App';
import { Card } from '@/components/shared/Card';
import { EmptyState } from '@/components/shared/EmptyState';
import { formatAbsoluteTime, formatRelativeTime } from '@/utils/format';
import { StatusDot } from '@/components/shared/StatusDot';
import { getReplicationJobStatusIndicator } from '@/utils/status';

function formatDuration(durationSeconds?: number, durationHuman?: string): string {
  if (durationHuman && durationHuman.trim()) return durationHuman;
  if (!durationSeconds || durationSeconds <= 0) return '';

  const hours = Math.floor(durationSeconds / 3600)
    .toString()
    .padStart(2, '0');
  const minutes = Math.floor((durationSeconds % 3600) / 60)
    .toString()
    .padStart(2, '0');
  const seconds = Math.floor(durationSeconds % 60)
    .toString()
    .padStart(2, '0');

  return `${hours}:${minutes}:${seconds}`;
}



function formatRate(limit?: number): string {
  if (!limit || limit <= 0) return '—';
  return `${limit.toFixed(0)} MB/s`;
}

const Replication: Component = () => {
  const { state, connected, reconnecting, reconnect } = useWebSocket();

  const replicationJobs = createMemo(() => {
    const jobs = state.replicationJobs ?? [];
    return [...jobs].sort((a, b) => {
      if (a.instance !== b.instance) return a.instance.localeCompare(b.instance);
      if ((a.guestName || '') !== (b.guestName || '')) {
        return (a.guestName || '').localeCompare(b.guestName || '');
      }
      return (a.jobId || '').localeCompare(b.jobId || '');
    });
  });

  return (
    <div class="space-y-3">
      <ProxmoxSectionNav current="replication" />

      <Show when={connected()} fallback={
        <Card padding="lg" tone="danger">
          <EmptyState
            icon={
              <svg class="h-12 w-12 text-red-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 8v4m0 4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
              </svg>
            }
            title="Connection lost"
            description={
              reconnecting()
                ? 'Attempting to reconnect…'
                : 'Unable to connect to the backend server'
            }
            tone="danger"
            actions={
              !reconnecting() ? (
                <button
                  onClick={() => reconnect()}
                  class="mt-2 inline-flex items-center px-4 py-2 text-xs font-medium rounded bg-red-600 text-white hover:bg-red-700 transition-colors"
                >
                  Reconnect now
                </button>
              ) : undefined
            }
          />
        </Card>
      }>
        <Show
          when={replicationJobs().length > 0}
          fallback={
            <Card padding="lg">
              <EmptyState
                icon={
                  <svg class="h-12 w-12 text-blue-500" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M4 4h16M4 10h16M4 16h16" />
                  </svg>
                }
                title="No replication jobs detected"
                description="Replication jobs will appear here once configured in Proxmox."
              />
            </Card>
          }
        >
          <Card padding="none" tone="glass">
            <div class="overflow-x-auto">
              <table class="min-w-[1000px] w-full divide-y divide-gray-200 dark:divide-gray-700">
                <thead class="bg-gray-50 dark:bg-gray-900/40">
                  <tr class="text-left text-xs font-semibold uppercase tracking-wide text-gray-600 dark:text-gray-300">
                    <th class="px-4 py-3">Guest</th>
                    <th class="px-4 py-3">Job</th>
                    <th class="px-4 py-3">Source → Target</th>
                    <th class="px-4 py-3">Last Sync</th>
                    <th class="px-4 py-3">Next Sync</th>
                    <th class="px-4 py-3">Status</th>
                    <th class="px-4 py-3">Failures</th>
                    <th class="px-4 py-3">Rate</th>
                  </tr>
                </thead>
                <tbody class="divide-y divide-gray-200 dark:divide-gray-800 text-sm text-gray-700 dark:text-gray-200">
                  <For each={replicationJobs()}>
                    {(job) => {
                      const indicator = getReplicationJobStatusIndicator(job);
                      return (
                        <tr class="hover:bg-gray-50/80 dark:hover:bg-gray-900/40 transition-colors">
                          <td class="px-4 py-3">
                            <div class="font-medium text-gray-900 dark:text-gray-100 truncate max-w-[200px]">
                              {job.guestName || `VM ${job.guestId ?? ''}`}
                            </div>
                            <div class="text-xs text-gray-500 dark:text-gray-400">
                              {job.instance} · ID {job.guestId ?? '—'} · {job.guestNode || job.sourceNode || 'Unknown node'}
                            </div>
                          </td>
                          <td class="px-4 py-3">
                            <div class="font-medium">{job.jobId || '—'}</div>
                            <div class="text-xs text-gray-500 dark:text-gray-400">
                              {job.type ? `${job.type.toUpperCase()} · ` : ''}{job.schedule || '*/15'}
                            </div>
                          </td>
                          <td class="px-4 py-3">
                            <div class="text-sm">
                              {job.sourceNode || '—'}
                              <span class="mx-1 text-gray-400">→</span>
                              {job.targetNode || '—'}
                            </div>
                            <div class="text-xs text-gray-500 dark:text-gray-400">
                              {job.sourceStorage || 'local'} → {job.targetStorage || 'remote'}
                            </div>
                          </td>
                          <td class="px-4 py-3">
                            <Show when={job.lastSyncTime}>
                              <div class="font-medium">{formatAbsoluteTime(job.lastSyncTime!)}</div>
                              <div class="text-xs text-gray-500 dark:text-gray-400">
                                {formatRelativeTime(job.lastSyncTime!)}
                                <Show when={job.lastSyncDurationSeconds || job.lastSyncDurationHuman}>
                                  <span class="mx-1">·</span>
                                  {formatDuration(job.lastSyncDurationSeconds, job.lastSyncDurationHuman)}
                                </Show>
                              </div>
                            </Show>
                            <Show when={!job.lastSyncTime}>
                              <span class="text-gray-400">Never</span>
                            </Show>
                          </td>
                          <td class="px-4 py-3">
                            <Show when={job.nextSyncTime}>
                              <div class="font-medium">{formatAbsoluteTime(job.nextSyncTime!)}</div>
                              <div class="text-xs text-gray-500 dark:text-gray-400">
                                {formatRelativeTime(job.nextSyncTime!)}
                              </div>
                            </Show>
                            <Show when={!job.nextSyncTime}>
                              <span class="text-gray-400">—</span>
                            </Show>
                          </td>
                          <td class="px-4 py-3">
                            <div class="flex items-center gap-2">
                              <StatusDot
                                variant={indicator.variant}
                                title={indicator.label}
                                ariaLabel={indicator.label}
                                size="sm"
                              />
                              <span class="text-sm font-medium text-gray-700 dark:text-gray-300">
                                {indicator.label}
                              </span>
                            </div>
                            <Show when={job.error}>
                              <div class="mt-1 text-xs text-red-500 dark:text-red-400 line-clamp-2">
                                {job.error}
                              </div>
                            </Show>
                          </td>
                          <td class="px-4 py-3">
                            <span class="text-sm font-medium">{job.failCount ?? 0}</span>
                          </td>
                          <td class="px-4 py-3">
                            <span class="text-sm">{formatRate(job.rateLimitMbps)}</span>
                          </td>
                        </tr>
                      );
                    }}
                  </For>
                </tbody>
              </table>
            </div>
          </Card>
        </Show>
      </Show>
    </div>
  );
};

export default Replication;
