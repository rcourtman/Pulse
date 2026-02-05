import { Component, For, Show, createMemo } from 'solid-js';
import { useNavigate } from '@solidjs/router';
import { useWebSocket } from '@/App';
import { Card } from '@/components/shared/Card';
import { EmptyState } from '@/components/shared/EmptyState';
import { SectionHeader } from '@/components/shared/SectionHeader';
import { StatusDot } from '@/components/shared/StatusDot';
import { formatRelativeTime } from '@/utils/format';
import type { StatusIndicator } from '@/utils/status';
import type { PMGInstance } from '@/types/api';

const parseTimestamp = (value?: string) => {
  if (!value) return undefined;
  const parsed = Date.parse(value);
  return Number.isNaN(parsed) ? undefined : parsed;
};

const formatStatusLabel = (value?: string) => {
  if (!value) return 'Unknown';
  return value.replace(/_/g, ' ').replace(/\b\w/g, (char) => char.toUpperCase());
};

const getPMGStatusIndicator = (instance: PMGInstance): StatusIndicator => {
  const status = `${instance.connectionHealth || ''} ${instance.status || ''}`.toLowerCase();
  if (status.includes('healthy') || status.includes('online')) {
    return { variant: 'success', label: 'Healthy' };
  }
  if (status.includes('degraded') || status.includes('warning')) {
    return { variant: 'warning', label: 'Degraded' };
  }
  if (status.includes('offline') || status.includes('error') || status.includes('disconnected')) {
    return { variant: 'danger', label: 'Offline' };
  }
  return {
    variant: 'muted',
    label: formatStatusLabel(instance.connectionHealth || instance.status),
  };
};

const Services: Component = () => {
  const navigate = useNavigate();
  const { state, connected, reconnecting, reconnect, initialDataReceived } = useWebSocket();

  const pmgInstances = createMemo(() => state.pmg || []);
  const sortedInstances = createMemo(() => {
    return [...pmgInstances()].sort((a, b) => {
      const aStatus = (a.status || '').toLowerCase();
      const bStatus = (b.status || '').toLowerCase();
      const aOnline = aStatus === 'healthy' || aStatus === 'online';
      const bOnline = bStatus === 'healthy' || bStatus === 'online';
      if (aOnline !== bOnline) return aOnline ? -1 : 1;
      return a.name.localeCompare(b.name);
    });
  });

  return (
    <div class="space-y-3">
      <SectionHeader
        label="Services"
        title="Mail Gateway Instances"
        description="PMG instances connected to Pulse."
        size="sm"
      />

      {/* Loading State */}
      <Show when={connected() && !initialDataReceived()}>
        <Card padding="lg">
          <EmptyState
            icon={
              <div class="mx-auto flex h-12 w-12 items-center justify-center">
                <svg class="h-8 w-8 animate-spin text-gray-400" fill="none" viewBox="0 0 24 24">
                  <circle
                    class="opacity-25"
                    cx="12"
                    cy="12"
                    r="10"
                    stroke="currentColor"
                    stroke-width="4"
                  />
                  <path
                    class="opacity-75"
                    fill="currentColor"
                    d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
                  />
                </svg>
              </div>
            }
            title="Loading mail gateway data..."
            description="Connecting to monitoring service"
          />
        </Card>
      </Show>

      {/* Disconnected State */}
      <Show when={!connected()}>
        <Card padding="lg" tone="danger">
          <EmptyState
            icon={
              <svg
                class="h-12 w-12 text-red-400"
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
              >
                <path
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  stroke-width="2"
                  d="M12 8v4m0 4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"
                />
              </svg>
            }
            title="Connection lost"
            description={reconnecting() ? 'Attempting to reconnect…' : 'Unable to connect to the backend server'}
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
      </Show>

      {/* Empty State */}
      <Show when={connected() && initialDataReceived() && pmgInstances().length === 0}>
        <Card padding="lg">
          <EmptyState
            icon={
              <svg
                class="h-12 w-12 text-gray-400"
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
              >
                <path
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  stroke-width="2"
                  d="M9.75 12h4.5m-4.5 3.75h4.5M12 6.75h.008M4.5 19.5h15a2.25 2.25 0 002.25-2.25V6.75A2.25 2.25 0 0019.5 4.5h-15A2.25 2.25 0 002.25 6.75v10.5A2.25 2.25 0 004.5 19.5z"
                />
              </svg>
            }
            title="No mail gateways configured"
            description="Add a PMG instance in Settings → Proxmox to see service status and recent activity."
            actions={
              <button
                type="button"
                onClick={() => navigate('/settings')}
                class="inline-flex items-center px-3 py-1.5 border border-transparent text-xs font-medium rounded-md text-white bg-blue-600 hover:bg-blue-700 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-blue-500"
              >
                Go to Settings
              </button>
            }
          />
        </Card>
      </Show>

      {/* Services Table */}
      <Show when={connected() && initialDataReceived() && pmgInstances().length > 0}>
        <Card padding="none" tone="glass" class="overflow-hidden">
          <div class="overflow-x-auto" style="scrollbar-width: none; -ms-overflow-style: none;">
            <style>{`
              .overflow-x-auto::-webkit-scrollbar { display: none; }
            `}</style>
            <table class="w-full">
              <thead>
                <tr class="bg-gray-50 dark:bg-gray-700/50 text-gray-600 dark:text-gray-300 border-b border-gray-200 dark:border-gray-600">
                  <th class="px-3 py-2 text-left text-[11px] sm:text-xs font-medium uppercase tracking-wider">
                    Service
                  </th>
                  <th class="px-3 py-2 text-left text-[11px] sm:text-xs font-medium uppercase tracking-wider">
                    Status
                  </th>
                  <th class="hidden sm:table-cell px-3 py-2 text-left text-[11px] sm:text-xs font-medium uppercase tracking-wider">
                    Version
                  </th>
                  <th class="hidden md:table-cell px-3 py-2 text-left text-[11px] sm:text-xs font-medium uppercase tracking-wider">
                    Nodes
                  </th>
                  <th class="px-3 py-2 text-left text-[11px] sm:text-xs font-medium uppercase tracking-wider">
                    Last Seen
                  </th>
                </tr>
              </thead>
              <tbody class="divide-y divide-gray-200 dark:divide-gray-700">
                <For each={sortedInstances()}>
                  {(instance) => {
                    const statusIndicator = getPMGStatusIndicator(instance);
                    const lastSeen = parseTimestamp(instance.lastSeen);
                    const nodeCount = instance.nodes ? instance.nodes.length : null;
                    return (
                      <tr class="hover:bg-gray-50/80 dark:hover:bg-gray-700/30 transition-colors">
                        <td class="px-3 py-2">
                          <div class="min-w-0">
                            <div class="text-sm font-medium text-gray-900 dark:text-gray-100 truncate">
                              {instance.name}
                            </div>
                            <div class="text-[11px] text-gray-500 dark:text-gray-400 truncate">
                              {instance.host}
                            </div>
                          </div>
                        </td>
                        <td class="px-3 py-2">
                          <div class="flex items-center gap-2">
                            <StatusDot
                              variant={statusIndicator.variant}
                              size="sm"
                              ariaLabel={statusIndicator.label}
                            />
                            <span class="text-xs text-gray-700 dark:text-gray-300">
                              {statusIndicator.label}
                            </span>
                          </div>
                        </td>
                        <td class="hidden sm:table-cell px-3 py-2 text-xs text-gray-600 dark:text-gray-300">
                          {instance.version || '—'}
                        </td>
                        <td class="hidden md:table-cell px-3 py-2 text-xs text-gray-600 dark:text-gray-300">
                          {nodeCount === null ? '—' : nodeCount}
                        </td>
                        <td class="px-3 py-2 text-xs text-gray-600 dark:text-gray-300">
                          {lastSeen ? formatRelativeTime(lastSeen) : '—'}
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
    </div>
  );
};

export default Services;
