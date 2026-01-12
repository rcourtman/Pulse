import type { Component } from 'solid-js';
import { Show, createMemo, createSignal, createEffect, onMount, onCleanup } from 'solid-js';
import { createStore } from 'solid-js/store';
import { useNavigate } from '@solidjs/router';
import type { DockerHost } from '@/types/api';
import { Card } from '@/components/shared/Card';
import { EmptyState } from '@/components/shared/EmptyState';
import { DockerFilter, type DockerViewMode } from './DockerFilter';
import { DockerHostSummaryTable, type DockerHostSummary } from './DockerHostSummaryTable';
import { DockerUnifiedTable } from './DockerUnifiedTable';
import { DockerClusterServicesTable } from './DockerClusterServicesTable';
import { hasSwarmClusters } from './swarmClusterHelpers';
import { useWebSocket } from '@/App';
import { useDebouncedValue } from '@/hooks/useDebouncedValue';
import { usePersistentSignal } from '@/hooks/usePersistentSignal';
import { formatBytes, formatRelativeTime } from '@/utils/format';
import { DockerMetadataAPI, type DockerMetadata } from '@/api/dockerMetadata';
import { DockerHostMetadataAPI, type DockerHostMetadata } from '@/api/dockerHostMetadata';
import { logger } from '@/utils/logger';
import { STORAGE_KEYS } from '@/utils/localStorage';
import { DEGRADED_HEALTH_STATUSES, OFFLINE_HEALTH_STATUSES } from '@/utils/status';
import { MonitoringAPI } from '@/api/monitoring';
import { showSuccess, showError, showToast } from '@/utils/toast';
import { isKioskMode } from '@/utils/url';

type DockerMetadataRecord = Record<string, DockerMetadata>;
type DockerHostMetadataRecord = Record<string, DockerHostMetadata>;

interface DockerHostsProps {
  hosts: DockerHost[];
  activeAlerts?: Record<string, unknown> | any;
}

export const DockerHosts: Component<DockerHostsProps> = (props) => {
  const navigate = useNavigate();
  const { initialDataReceived, reconnecting, connected, reconnect } = useWebSocket();

  // Load docker metadata from localStorage or API
  const loadInitialDockerMetadata = (): DockerMetadataRecord => {
    try {
      const cached = localStorage.getItem(STORAGE_KEYS.DOCKER_METADATA);
      if (cached) {
        return JSON.parse(cached);
      }
    } catch (err) {
      logger.warn('Failed to parse cached docker metadata', err);
    }
    return {};
  };

  // Load docker host metadata from localStorage
  const loadInitialDockerHostMetadata = (): DockerHostMetadataRecord => {
    try {
      const cached = localStorage.getItem(STORAGE_KEYS.DOCKER_METADATA + '_hosts');
      if (cached) {
        return JSON.parse(cached);
      }
    } catch (err) {
      logger.warn('Failed to parse cached docker host metadata', err);
    }
    return {};
  };

  const [dockerMetadata, setDockerMetadata] = createSignal<DockerMetadataRecord>(
    loadInitialDockerMetadata(),
  );

  const [dockerHostMetadata, setDockerHostMetadata] = createSignal<DockerHostMetadataRecord>(
    loadInitialDockerHostMetadata(),
  );

  const sortedHosts = createMemo(() => {
    const hosts = props.hosts || [];
    return [...hosts].sort((a, b) => {
      const aName = a.customDisplayName || a.displayName || a.hostname || a.id || '';
      const bName = b.customDisplayName || b.displayName || b.hostname || b.id || '';
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
  const [groupingMode, setGroupingMode] = usePersistentSignal<DockerViewMode>('dockerGroupingMode', 'grouped', {
    deserialize: (v) => (['grouped', 'flat', 'cluster'].includes(v) ? v as DockerViewMode : 'grouped'),
  });

  // Detect if any Swarm clusters exist (2+ hosts sharing a clusterId)
  const hasSwarmClustersDetected = createMemo(() => hasSwarmClusters(sortedHosts()));

  // Kiosk mode - hide filter panel for clean dashboard display
  const kioskMode = createMemo(() => isKioskMode());

  const clampPercent = (value: number | undefined | null) => {
    if (value === undefined || value === null || Number.isNaN(value)) return 0;
    if (!Number.isFinite(value)) return 0;
    if (value < 0) return 0;
    if (value > 100) return 100;
    return value;
  };

  // Cache for stable summary objects to prevent re-animations
  const summaryCache = new Map<string, [DockerHostSummary, any]>();

  const hostSummaries = createMemo<DockerHostSummary[]>(() => {
    const usedKeys = new Set<string>();

    const result = sortedHosts().map((host) => {
      const totalContainers = host.containers?.length ?? 0;
      const runningContainers =
        host.containers?.filter((container) => container.state?.toLowerCase() === 'running').length ?? 0;
      const stoppedContainers =
        host.containers?.filter((container) =>
          ['exited', 'stopped', 'created'].includes(container.state?.toLowerCase() || '')
        ).length ?? 0;
      // Count anything that isn't running or stopped/created as an error/warning state (restarting, dead, paused, etc)
      const errorContainers = totalContainers - runningContainers - stoppedContainers;

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
        memoryTotal > 0 ? `${formatBytes(memoryUsed, 0)} / ${formatBytes(memoryTotal, 0)}` : undefined;

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
          diskLabel = `${formatBytes(totals.used, 0)} / ${formatBytes(totals.total, 0)}`;
        }
      }

      const uptimeSeconds = host.uptimeSeconds ?? 0;
      const lastSeenRelative = host.lastSeen ? formatRelativeTime(host.lastSeen) : '—';
      const lastSeenAbsolute = host.lastSeen ? new Date(host.lastSeen).toLocaleString() : '';

      const newSummary: DockerHostSummary = {
        host,
        cpuPercent,
        memoryPercent,
        memoryLabel,
        diskPercent,
        diskLabel,
        runningPercent,
        runningCount: runningContainers,
        stoppedCount: stoppedContainers,
        errorCount: errorContainers,
        totalCount: totalContainers,
        uptimeSeconds,
        lastSeenRelative,
        lastSeenAbsolute,
      };

      const key = host.id;
      usedKeys.add(key);

      let entry = summaryCache.get(key);
      if (!entry) {
        entry = createStore(newSummary);
        summaryCache.set(key, entry);
      } else {
        const [_, setState] = entry;
        setState(newSummary);
      }
      return entry[0];
    });

    // Prune cache
    for (const key of summaryCache.keys()) {
      if (!usedKeys.has(key)) {
        summaryCache.delete(key);
      }
    }

    return result;
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

  onMount(() => {
    document.addEventListener('keydown', handleKeyDown);

    // Load docker metadata from API
    DockerMetadataAPI.getAllMetadata()
      .then((metadata) => {
        setDockerMetadata(metadata || {});
        try {
          localStorage.setItem(STORAGE_KEYS.DOCKER_METADATA, JSON.stringify(metadata || {}));
        } catch (err) {
          logger.warn('Failed to cache docker metadata', err);
        }
      })
      .catch((err) => {
        logger.debug('Failed to load docker metadata', err);
      });

    // Load docker host metadata from API
    DockerHostMetadataAPI.getAllMetadata()
      .then((metadata) => {
        setDockerHostMetadata(metadata || {});
        try {
          localStorage.setItem(STORAGE_KEYS.DOCKER_METADATA + '_hosts', JSON.stringify(metadata || {}));
        } catch (err) {
          logger.warn('Failed to cache docker host metadata', err);
        }
      })
      .catch((err) => {
        logger.debug('Failed to load docker host metadata', err);
      });
  });
  onCleanup(() => document.removeEventListener('keydown', handleKeyDown));

  // Handler to update docker host custom URL
  const handleHostCustomUrlUpdate = (hostId: string, url: string) => {
    const trimmedUrl = url.trim();
    const nextUrl = trimmedUrl === '' ? undefined : trimmedUrl;

    setDockerHostMetadata((prev) => {
      const updated = { ...prev };
      if (nextUrl === undefined) {
        // Remove URL but keep other metadata fields
        if (updated[hostId]) {
          const { customUrl: _removed, ...rest } = updated[hostId];
          if (Object.keys(rest).length === 0 || (Object.keys(rest).length === 1 && !rest.customDisplayName && !rest.notes?.length)) {
            delete updated[hostId];
          } else {
            updated[hostId] = rest;
          }
        }
      } else {
        updated[hostId] = {
          ...(prev[hostId] || {}),
          customUrl: nextUrl,
        };
      }

      // Cache to localStorage
      try {
        localStorage.setItem(STORAGE_KEYS.DOCKER_METADATA + '_hosts', JSON.stringify(updated));
      } catch (err) {
        logger.warn('Failed to cache docker host metadata', err);
      }

      return updated;
    });
  };

  // Handler to update docker resource custom URL
  const handleCustomUrlUpdate = (resourceId: string, url: string) => {
    const trimmedUrl = url.trim();
    const nextUrl = trimmedUrl === '' ? undefined : trimmedUrl;

    setDockerMetadata((prev) => {
      const updated = { ...prev };
      if (nextUrl === undefined) {
        delete updated[resourceId];
      } else {
        updated[resourceId] = {
          ...(prev[resourceId] || { id: resourceId }),
          customUrl: nextUrl,
        };
      }

      // Cache to localStorage
      try {
        localStorage.setItem(STORAGE_KEYS.DOCKER_METADATA, JSON.stringify(updated));
      } catch (err) {
        logger.warn('Failed to cache docker metadata', err);
      }

      return updated;
    });
  };

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
    if (status === 'offline') return OFFLINE_HEALTH_STATUSES.has(normalized);
    if (status === 'degraded') return DEGRADED_HEALTH_STATUSES.has(normalized);
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
    return { type: 'host-status' as const, value: status };
  });

  const handleHostSelect = (hostId: string) => {
    setSelectedHostId((current) => (current === hostId ? null : hostId));
  };

  const updateableContainers = createMemo(() => {
    const containers: { hostId: string; containerId: string; containerName: string }[] = [];
    sortedHosts().forEach((host) => {
      if (!hostMatchesStatus(host)) return;
      host.containers?.forEach((c) => {
        if (c.updateStatus?.updateAvailable) {
          containers.push({
            hostId: host.id,
            containerId: c.id,
            containerName: c.name || c.id,
          });
        }
      });
    });
    return containers;
  });

  // Track batch update status: key is hostId:containerId
  const [batchUpdateState, setBatchUpdateState] = createStore<Record<string, 'updating' | 'queued' | 'error'>>({});

  const handleUpdateAll = async () => {
    const targets = updateableContainers();
    if (targets.length === 0) return;

    // Initial toast
    showToast(
      'info',
      'Batch Update Started',
      `Preparing to update ${targets.length} containers...`,
      10000,
    );

    // Mark all as updating
    targets.forEach(t => {
      setBatchUpdateState(`${t.hostId}:${t.containerId}`, 'updating');
    });

    let successCount = 0;
    let failCount = 0;

    // Process in chunks of 5 to avoid overloading the browser/network
    const chunkSize = 5;
    for (let i = 0; i < targets.length; i += chunkSize) {
      const chunk = targets.slice(i, i + chunkSize);

      await Promise.all(chunk.map(async (target) => {
        const key = `${target.hostId}:${target.containerId}`;
        try {
          await MonitoringAPI.updateDockerContainer(
            target.hostId,
            target.containerId,
            target.containerName,
          );
          setBatchUpdateState(key, 'queued');
          successCount++;
        } catch (err) {
          failCount++;
          setBatchUpdateState(key, 'error');
          logger.error(`Failed to trigger update for ${target.containerName}`, err);
        }
      }));
    }

    if (failCount === 0) {
      showSuccess(`Successfully queued updates for all ${targets.length} containers.`);
      // Clear success states after delay
      setTimeout(() => {
        targets.forEach(t => {
          const key = `${t.hostId}:${t.containerId}`;
          if (batchUpdateState[key] === 'queued') {
            setBatchUpdateState(key, undefined as any);
          }
        });
      }, 5000);
    } else if (successCount === 0) {
      showError(`Failed to queue any updates. Check console for details.`);
    } else {
      showToast('warning', 'Batch Update Completed', `Queued ${successCount} updates. ${failCount} failed.`);
    }
  };

  const handleCheckUpdates = async (hostId: string) => {
    try {
      await MonitoringAPI.checkDockerUpdates(hostId);
      showSuccess('Update check triggered. The host will refresh container information shortly.');
    } catch (err) {
      showError(`Failed to trigger update check: ${err instanceof Error ? err.message : String(err)}`);
    }
  };

  // Get the command status for the selected host to show checking indicator
  const selectedHostCommandStatus = createMemo(() => {
    const hostId = selectedHostId();
    if (!hostId) return undefined;

    const host = props.hosts.find(h => h.id === hostId);
    if (!host?.command) return undefined;

    // Only show status for check_updates commands
    if (host.command.type !== 'check_updates') return undefined;

    return host.command.status;
  });

  const renderFilter = () => (
    <DockerFilter
      search={search}
      setSearch={setSearch}
      statusFilter={statusFilter}
      setStatusFilter={setStatusFilter}
      groupingMode={groupingMode}
      setGroupingMode={setGroupingMode}
      hasSwarmClusters={hasSwarmClustersDetected()}
      onReset={() => {
        setSearch('');
        setSelectedHostId(null);
        setStatusFilter('all');
        setGroupingMode('grouped');
      }}
      searchInputRef={(el) => {
        searchInputRef = el;
      }}
      updateAvailableCount={updateableContainers().length}
      onUpdateAll={handleUpdateAll}
      onCheckUpdates={handleCheckUpdates}
      activeHostId={selectedHostId()}
      checkingUpdatesStatus={selectedHostCommandStatus()}
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
            title={reconnecting() ? 'Reconnecting to container agents...' : 'Loading container data...'}
            description={
              reconnecting()
                ? 'Re-establishing metrics from the monitoring service.'
                : connected()
                  ? 'Waiting for the first container update.'
                  : 'Connecting to the monitoring service.'
            }
          />
        </Card>
      </Show>

      {/* Disconnected State */}
      <Show when={!connected() && !isLoading()}>
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
      </Show>

      <Show when={!isLoading()}>
        <Show
          when={sortedHosts().length > 0}
          fallback={
            <>
              <Show when={!kioskMode()}>{renderFilter()}</Show>
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
                  title="No container runtimes reporting"
                  description="Deploy the Pulse container agent (Docker or Podman) on at least one host to light up this tab. As soon as an agent reports in, runtime metrics appear automatically."
                  actions={
                    <button
                      type="button"
                      onClick={() => navigate('/settings/docker')}
                      class="inline-flex items-center gap-2 rounded-lg bg-blue-600 px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-blue-700"
                    >
                      <span>Set up container agent</span>
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
              dockerHostMetadata={dockerHostMetadata()}
              onHostCustomUrlUpdate={handleHostCustomUrlUpdate}
            />
          </Show>

          <Show when={!kioskMode()}>{renderFilter()}</Show>

          <Show
            when={groupingMode() === 'cluster'}
            fallback={
              <DockerUnifiedTable
                hosts={sortedHosts()}
                searchTerm={debouncedSearch()}
                statsFilter={statsFilter()}
                selectedHostId={selectedHostId}
                dockerMetadata={dockerMetadata()}
                dockerHostMetadata={dockerHostMetadata()}
                onCustomUrlUpdate={handleCustomUrlUpdate}
                batchUpdateState={batchUpdateState}
                groupingMode={groupingMode() === 'flat' ? 'flat' : 'grouped'}
              />
            }
          >
            <DockerClusterServicesTable
              hosts={sortedHosts()}
              searchTerm={debouncedSearch()}
            />
          </Show>
        </Show>
      </Show>
    </div>
  );
};
