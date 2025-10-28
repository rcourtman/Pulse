import type { Component } from 'solid-js';
import { Show, createMemo, createSignal, createEffect, onMount, onCleanup } from 'solid-js';
import { useNavigate } from '@solidjs/router';
import type { DockerHost } from '@/types/api';
import { Card } from '@/components/shared/Card';
import { EmptyState } from '@/components/shared/EmptyState';
import { DockerFilter } from './DockerFilter';
import { DockerHostSummaryTable, type DockerHostSummary } from './DockerHostSummaryTable';
import { DockerUnifiedTable } from './DockerUnifiedTable';
import { useWebSocket } from '@/App';
import { useDebouncedValue } from '@/hooks/useDebouncedValue';
import { formatBytes, formatRelativeTime } from '@/utils/format';

const OFFLINE_HOST_STATUSES = new Set(['offline', 'error', 'unreachable', 'down', 'disconnected']);
const DEGRADED_HOST_STATUSES = new Set([
  'degraded',
  'warning',
  'maintenance',
  'partial',
  'initializing',
  'unknown',
]);

interface DockerHostsProps {
  hosts: DockerHost[];
  activeAlerts?: Record<string, unknown> | any;
}

export const DockerHosts: Component<DockerHostsProps> = (props) => {
  const navigate = useNavigate();
  const { initialDataReceived, reconnecting, connected } = useWebSocket();

  const sortedHosts = createMemo(() => {
    const hosts = props.hosts || [];
    return [...hosts].sort((a, b) => {
      const aName = a.displayName || a.hostname || a.id || '';
      const bName = b.displayName || b.hostname || b.id || '';
      return aName.localeCompare(bName);
    });
  });

  const isLoading = createMemo(() => {
    if (typeof initialDataReceived === 'function') {
      const hostCount = Array.isArray(props.hosts) ? props.hosts.length : 0;
      return !initialDataReceived() && hostCount === 0;
    }
    return false;
  });

  const [search, setSearch] = createSignal('');
  const debouncedSearch = useDebouncedValue(search, 250);
  const [selectedHostId, setSelectedHostId] = createSignal<string | null>(null);
  const [statusFilter, setStatusFilter] = createSignal<'all' | 'online' | 'degraded' | 'offline'>('all');

  const clampPercent = (value: number | undefined | null) => {
    if (value === undefined || value === null || Number.isNaN(value)) return 0;
    if (!Number.isFinite(value)) return 0;
    if (value < 0) return 0;
    if (value > 100) return 100;
    return value;
  };

  const hostSummaries = createMemo<DockerHostSummary[]>(() => {
    return sortedHosts().map((host) => {
      const totalContainers = host.containers?.length ?? 0;
      const runningContainers =
        host.containers?.filter((container) => container.state?.toLowerCase() === 'running').length ?? 0;
      const runningPercent = totalContainers > 0 ? clampPercent((runningContainers / totalContainers) * 100) : 0;

      const cpuPercent = clampPercent(host.cpuUsagePercent ?? 0);

      const memoryUsed = host.memory?.used ?? 0;
      const memoryTotal = host.memory?.total ?? host.totalMemoryBytes ?? 0;
      const memoryPercent = host.memory?.usage
        ? clampPercent(host.memory.usage)
        : memoryTotal > 0
          ? clampPercent((memoryUsed / memoryTotal) * 100)
          : 0;
      const memoryLabel =
        memoryTotal > 0 ? `${formatBytes(memoryUsed)} / ${formatBytes(memoryTotal)}` : undefined;

      let diskPercent = 0;
      let diskLabel: string | undefined;
      if (host.disks && host.disks.length > 0) {
        const totals = host.disks.reduce(
          (acc, disk) => {
            acc.used += disk.used ?? 0;
            acc.total += disk.total ?? 0;
            return acc;
          },
          { used: 0, total: 0 },
        );
        if (totals.total > 0) {
          diskPercent = clampPercent((totals.used / totals.total) * 100);
          diskLabel = `${formatBytes(totals.used)} / ${formatBytes(totals.total)}`;
        }
      }

      const uptimeSeconds = host.uptimeSeconds ?? 0;
      const lastSeenRelative = host.lastSeen ? formatRelativeTime(host.lastSeen) : '—';
      const lastSeenAbsolute = host.lastSeen ? new Date(host.lastSeen).toLocaleString() : '';

      return {
        host,
        cpuPercent,
        memoryPercent,
        memoryLabel,
        diskPercent,
        diskLabel,
        runningPercent,
        runningCount: runningContainers,
        totalCount: totalContainers,
        uptimeSeconds,
        lastSeenRelative,
        lastSeenAbsolute,
      };
    });
  });

  let searchInputRef: HTMLInputElement | undefined;

  const focusSearchInput = () => {
    queueMicrotask(() => searchInputRef?.focus());
  };

  const handleKeyDown = (event: KeyboardEvent) => {
    const target = event.target as HTMLElement;

    if (target.tagName === 'INPUT' || target.tagName === 'TEXTAREA' || target.isContentEditable) {
      return;
    }

    if (event.ctrlKey || event.metaKey || event.altKey) {
      return;
    }

    if (event.key.length === 1 && searchInputRef) {
      event.preventDefault();
      focusSearchInput();
      setSearch((prev) => prev + event.key);
    }
  };

  onMount(() => document.addEventListener('keydown', handleKeyDown));
  onCleanup(() => document.removeEventListener('keydown', handleKeyDown));

  createEffect(() => {
    const hostId = selectedHostId();
    if (!hostId) {
      return;
    }
    if (!sortedHosts().some((host) => host.id === hostId)) {
      setSelectedHostId(null);
    }
  });

  const hostMatchesStatus = (host: DockerHost) => {
    const status = statusFilter();
    if (status === 'all') return true;
    const normalized = host.status?.toLowerCase() ?? '';
    if (status === 'online') return normalized === 'online';
    if (status === 'offline') return OFFLINE_HOST_STATUSES.has(normalized);
    if (status === 'degraded') {
      return DEGRADED_HOST_STATUSES.has(normalized) || normalized === 'degraded';
    }
    return true;
  };

  const filteredHostSummaries = createMemo(() => {
    const summaries = hostSummaries();
    if (statusFilter() === 'all') return summaries;
    return summaries.filter((summary) => hostMatchesStatus(summary.host));
  });

  createEffect(() => {
    const hostId = selectedHostId();
    if (!hostId) return;
    if (!filteredHostSummaries().some((summary) => summary.host.id === hostId)) {
      setSelectedHostId(null);
    }
  });

  const statsFilter = createMemo(() => {
    const status = statusFilter();
    if (status === 'all') return null;
    return { type: 'host-status', value: status };
  });

  const handleHostSelect = (hostId: string) => {
    setSelectedHostId((current) => (current === hostId ? null : hostId));
  };

  const renderFilter = () => (
    <DockerFilter
      search={search}
      setSearch={setSearch}
      statusFilter={statusFilter}
      setStatusFilter={setStatusFilter}
      onReset={() => {
        setSearch('');
        setSelectedHostId(null);
        setStatusFilter('all');
      }}
      searchInputRef={(el) => {
        searchInputRef = el;
      }}
    />
  );

  return (
    <div class="space-y-0">
      <Show when={isLoading()}>
        <Card padding="lg">
          <EmptyState
            icon={
              <svg class="h-12 w-12 animate-spin text-blue-500" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4" />
                <path
                  class="opacity-75"
                  fill="currentColor"
                  d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
                />
              </svg>
            }
            title={reconnecting() ? 'Reconnecting to Docker agents...' : 'Loading Docker data...'}
            description={
              reconnecting()
                ? 'Re-establishing metrics from the monitoring service.'
                : connected()
                  ? 'Waiting for the first Docker update.'
                  : 'Connecting to the monitoring service.'
            }
          />
        </Card>
      </Show>

      <Show when={!isLoading()}>
        <Show
          when={sortedHosts().length > 0}
          fallback={
            <>
              {renderFilter()}
              <Card padding="lg">
                <EmptyState
                  icon={
                    <svg class="h-12 w-12 text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                      <path
                        stroke-linecap="round"
                        stroke-linejoin="round"
                        stroke-width="2"
                        d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z"
                      />
                    </svg>
                  }
                  title="No Docker hosts configured"
                  description="Deploy the Pulse Docker agent on at least one Docker host to light up this tab. As soon as an agent reports in, container metrics appear automatically."
                  actions={
                    <button
                      type="button"
                      onClick={() => navigate('/settings/docker')}
                      class="inline-flex items-center gap-2 rounded-lg bg-blue-600 px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-blue-700"
                    >
                      <span>Set up Docker agent</span>
                      <svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 5l7 7-7 7" />
                      </svg>
                    </button>
                  }
                />
              </Card>
            </>
          }
        >
          <Show when={hostSummaries().length > 0}>
            <DockerHostSummaryTable
              summaries={filteredHostSummaries}
              selectedHostId={selectedHostId}
              onSelect={handleHostSelect}
            />
          </Show>

          {renderFilter()}

          <DockerUnifiedTable
            hosts={sortedHosts()}
            searchTerm={debouncedSearch()}
            statsFilter={statsFilter()}
            selectedHostId={selectedHostId}
          />
        </Show>
      </Show>
    </div>
  );
};
