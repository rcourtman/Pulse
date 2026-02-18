import { useLocation, useNavigate } from '@solidjs/router';
import { Component, For, Show, createEffect, createMemo, createSignal, onCleanup } from 'solid-js';
import { useWebSocket } from '@/App';
import { useResources } from '@/hooks/useResources';
import { Card } from '@/components/shared/Card';
import { DiskList } from '@/components/Storage/DiskList';
import { EnhancedStorageBar } from '@/components/Storage/EnhancedStorageBar';
import { StorageHero } from '@/components/Storage/StorageHero';
import { useAlertsActivation } from '@/stores/alertsActivation';
import { buildStorageRecords } from '@/features/storageBackups/storageAdapters';
import {
  getCephHealthLabel,
  getCephHealthStyles,
} from '@/features/storageBackups/storageDomain';
import type { NormalizedHealth, StorageRecord } from '@/features/storageBackups/models';
import { useStorageRecoveryResources } from '@/hooks/useUnifiedResources';
import {
  buildStoragePath,
  parseStorageLinkSearch,
} from '@/routing/resourceLinks';
import { formatBytes, formatPercent } from '@/utils/format';
import { getProxmoxData } from '@/utils/resourcePlatformData';
import { useStorageRouteState } from './useStorageRouteState';
import { isCephRecord, useStorageCephModel } from './useStorageCephModel';
import { useStorageAlertState } from './useStorageAlertState';
import { useStorageHeroTrend } from './useStorageHeroTrend';
import { StorageFilter, type StorageGroupByFilter, type StorageStatusFilter } from './StorageFilter';
import { StorageGroupRow } from './StorageGroupRow';
import { StoragePoolRow } from './StoragePoolRow';
import {
  type StorageGroupKey,
  type StorageSortKey,
  getRecordNodeLabel,
  sourceLabel,
  useStorageModel,
} from './useStorageModel';

type StorageView = 'pools' | 'disks';

const STORAGE_SORT_OPTIONS: Array<{ value: StorageSortKey; label: string }> = [
  { value: 'name', label: 'Name' },
  { value: 'usage', label: 'Usage %' },
  { value: 'type', label: 'Type' },
];

const normalizeHealthFilter = (value: string): 'all' | NormalizedHealth => {
  const normalized = (value || '').trim().toLowerCase();
  if (!normalized || normalized === 'all') return 'all';
  if (normalized === 'available' || normalized === 'online' || normalized === 'healthy') return 'healthy';
  if (normalized === 'degraded' || normalized === 'warning') return 'warning';
  if (normalized === 'critical') return 'critical';
  if (normalized === 'offline') return 'offline';
  if (normalized === 'unknown') return 'unknown';
  return 'all';
};

const normalizeSortKey = (value: string): StorageSortKey => {
  if (value === 'usage' || value === 'type') return value;
  return 'name';
};

const normalizeGroupKey = (value: string): StorageGroupKey => {
  if (value === 'type' || value === 'status') return value;
  return 'node';
};

const normalizeView = (value: string): StorageView => (value === 'disks' ? 'disks' : 'pools');

const normalizeSortDirection = (value: string): 'asc' | 'desc' =>
  value === 'desc' ? 'desc' : 'asc';

const getStorageMetaBoolean = (record: StorageRecord, key: 'isCeph' | 'isZfs'): boolean | null => {
  const details = (record.details || {}) as Record<string, unknown>;
  const value = details[key];
  return typeof value === 'boolean' ? value : null;
};

const isRecordCeph = (record: StorageRecord): boolean => {
  const isCephMeta = getStorageMetaBoolean(record, 'isCeph');
  if (isCephMeta !== null) return isCephMeta;
  return isCephRecord(record);
};

const Storage: Component = () => {
  const navigate = useNavigate();
  const location = useLocation();
  const { state, activeAlerts, connected, initialDataReceived, reconnecting, reconnect } = useWebSocket();
  const { byType } = useResources();
  const nodes = createMemo(() => byType('node'));
  const physicalDisks = createMemo(() => byType('physical_disk'));
  const cephResources = createMemo(() => byType('ceph'));
  const storageBackupsResources = useStorageRecoveryResources();
  const alertsActivation = useAlertsActivation();
  const alertsEnabled = createMemo(() => alertsActivation.activationState() === 'active');

  const [search, setSearch] = createSignal('');
  const [sourceFilter, setSourceFilter] = createSignal('all');
  const [healthFilter, setHealthFilter] = createSignal<'all' | NormalizedHealth>('all');
  const [view, setView] = createSignal<StorageView>('pools');
  const [selectedNodeId, setSelectedNodeId] = createSignal('all');
  const [sortKey, setSortKey] = createSignal<StorageSortKey>('name');
  const [sortDirection, setSortDirection] = createSignal<'asc' | 'desc'>('asc');
  const [groupBy, setGroupBy] = createSignal<StorageGroupKey>('node');
  const [expandedGroups, setExpandedGroups] = createSignal<Set<string>>(new Set());
  const [expandedPoolId, setExpandedPoolId] = createSignal<string | null>(null);
  const [highlightedRecordId, setHighlightedRecordId] = createSignal<string | null>(null);
  const [handledResourceId, setHandledResourceId] = createSignal<string | null>(null);
  let highlightTimer: number | undefined;

  const adapterResources = createMemo(() => {
    const unifiedResources = storageBackupsResources.resources();
    return unifiedResources;
  });

  const records = createMemo(() => buildStorageRecords({ state, resources: adapterResources() }));
  const storageTrend = useStorageHeroTrend(records);
  const activeAlertsAccessor = () => {
    if (typeof activeAlerts === 'function') {
      return (activeAlerts as () => unknown)();
    }
    return activeAlerts;
  };
  const { getRecordAlertState } = useStorageAlertState({
    records,
    activeAlerts: activeAlertsAccessor,
    alertsEnabled,
  });

  const nodeOptions = createMemo(() => {
    return nodes().map((node) => ({
      id: node.id,
      label: node.name,
      instance: getProxmoxData(node)?.instance,
    }));
  });

  const nodeOnlineByLabel = createMemo(() => {
    const map = new Map<string, boolean>();
    nodes().forEach((node) => {
      const key = (node.name || '').trim().toLowerCase();
      if (!key) return;
      const hasNodeStatus = typeof node.status === 'string' && node.status.trim().length > 0;
      const hasNodeUptime = typeof node.uptime === 'number';
      if (!hasNodeStatus && !hasNodeUptime) {
        return;
      }
      map.set(key, node.status === 'online' && (node.uptime || 0) > 0);
    });
    return map;
  });

  const { sourceOptions, selectedNode, filteredRecords, groupedRecords, summary } = useStorageModel({
    records,
    search,
    sourceFilter,
    healthFilter,
    selectedNodeId,
    nodeOptions,
    sortKey,
    sortDirection,
    groupBy,
  });

  // Default all groups to expanded on first load; new groups auto-expand
  createEffect(() => {
    const allKeys = groupedRecords().map((g) => g.key);
    setExpandedGroups((prev) => {
      if (prev.size === 0) return new Set(allKeys);
      // Add any newly-appearing groups as expanded
      const next = new Set(prev);
      let changed = false;
      for (const key of allKeys) {
        if (!next.has(key)) { next.add(key); changed = true; }
      }
      return changed ? next : prev;
    });
  });

  const toggleGroup = (key: string) => {
    setExpandedGroups((prev) => {
      const next = new Set(prev);
      if (next.has(key)) next.delete(key);
      else next.add(key);
      return next;
    });
  };

  const { cephSummaryStats } = useStorageCephModel({
    records,
    cephResources,
  });

  // Health breakdown is already computed from filteredRecords() inside useStorageModel (summary().byHealth).
  // Reuse that source to avoid an extra pass and guarantee consistent counts across the page.
  const healthBreakdown = createMemo(() => summary().byHealth);

  const storageFilterGroupBy = (): StorageGroupByFilter => {
    const current = groupBy();
    return current === 'type' || current === 'status' ? current : 'node';
  };

  const storageFilterStatus = (): StorageStatusFilter => {
    const current = healthFilter();
    if (current === 'all') return 'all';
    if (current === 'healthy') return 'available';
    return current;
  };

  const setStorageFilterStatus = (value: StorageStatusFilter) => {
    if (value === 'all') {
      setHealthFilter('all');
      return;
    }
    if (value === 'available') {
      setHealthFilter('healthy');
      return;
    }
    setHealthFilter(value);
  };

  const sourceFilterOptions = createMemo(() => {
    const toneForKey = (key: string) => {
      switch (key) {
        case 'proxmox':
        case 'proxmox-pve':
          return { label: 'PVE', tone: 'blue' as const };
        case 'pbs':
        case 'proxmox-pbs':
          return { label: 'PBS', tone: 'emerald' as const };
        case 'ceph':
          return { label: 'Ceph', tone: 'violet' as const };
        case 'kubernetes':
          return { label: 'K8s', tone: 'cyan' as const };
        case 'pmg':
          return { label: 'PMG', tone: 'blue' as const };
        default:
          return { label: sourceLabel(key), tone: 'slate' as const };
      }
    };

    return sourceOptions().map((key) => {
      if (key === 'all') return { key: 'all', label: 'All Sources', tone: 'slate' as const };
      const preset = toneForKey(key);
      return { key, ...preset };
    });
  });

  const isWaitingForData = createMemo(
    () =>
      storageBackupsResources.loading() &&
      filteredRecords().length === 0 &&
      !connected() &&
      !initialDataReceived(),
  );
  const isDisconnectedAfterLoad = createMemo(() => !connected() && initialDataReceived() && !reconnecting());
  const isLoadingPools = createMemo(
    () => storageBackupsResources.loading() && view() === 'pools' && filteredRecords().length === 0,
  );
  const hasFetchError = createMemo(() => Boolean(storageBackupsResources.error()));

  const isActiveStorageRoute = () => location.pathname === '/storage';

  createEffect(() => {
    const { resource } = parseStorageLinkSearch(location.search);
    if (!resource || resource === handledResourceId()) return;

    const match = records().find((record) => record.id === resource || record.name === resource);
    if (!match) return;

    if (isRecordCeph(match)) {
      setExpandedPoolId(match.id);
    }

    setHighlightedRecordId(match.id);
    setHandledResourceId(resource);

    if (highlightTimer) window.clearTimeout(highlightTimer);
    highlightTimer = window.setTimeout(() => setHighlightedRecordId(null), 2000);
  });

  useStorageRouteState({
    location,
    navigate,
    buildPath: buildStoragePath,
    isReadEnabled: isActiveStorageRoute,
    isWriteEnabled: isActiveStorageRoute,
    useCurrentPathForNavigation: true,
    fields: {
      tab: {
        get: view,
        set: setView,
        read: (parsed) => normalizeView(parsed.tab),
        write: (value) => (value !== 'pools' ? value : null),
      },
      source: {
        get: sourceFilter,
        set: setSourceFilter,
        read: (parsed) => parsed.source || 'all',
        write: (value) => (value !== 'all' ? value : null),
      },
      status: {
        get: healthFilter,
        set: setHealthFilter,
        read: (parsed) => normalizeHealthFilter(parsed.status),
        write: (value) => (value !== 'all' ? value : null),
      },
      node: {
        get: selectedNodeId,
        set: setSelectedNodeId,
        read: (parsed) => parsed.node || 'all',
        write: (value) => (value !== 'all' ? value : null),
      },
      group: {
        get: groupBy,
        set: setGroupBy,
        read: (parsed) => normalizeGroupKey(parsed.group),
        write: (value) => (value !== 'node' ? value : null),
      },
      sort: {
        get: sortKey,
        set: setSortKey,
        read: (parsed) => normalizeSortKey(parsed.sort),
        write: (value) => (value !== 'name' ? value : null),
      },
      order: {
        get: sortDirection,
        set: setSortDirection,
        read: (parsed) => normalizeSortDirection(parsed.order),
        write: (value) => (value !== 'asc' ? value : null),
      },
      query: {
        get: search,
        set: setSearch,
        read: (parsed) => parsed.query,
        write: (value) => value.trim() || null,
      },
    },
  });

  createEffect(() => {
    if (view() !== 'pools') {
      setExpandedPoolId(null);
    }
  });

  onCleanup(() => {
    if (highlightTimer) window.clearTimeout(highlightTimer);
  });

  return (
    <div class="space-y-4">
      <StorageHero
        summary={summary()}
        healthBreakdown={healthBreakdown()}
        diskCount={physicalDisks().length}
        trend={storageTrend.trend()}
      />

      <Show
        when={
          view() === 'pools' &&
          cephSummaryStats().clusters.length > 0 &&
          filteredRecords().some(isRecordCeph)
        }
      >
        <Card padding="md" tone="glass">
          <div class="flex flex-wrap items-center justify-between gap-3">
            <div class="space-y-0.5">
              <div class="text-xs font-semibold uppercase tracking-wide text-gray-500 dark:text-gray-400">
                Ceph Summary
              </div>
              <div class="text-sm text-gray-600 dark:text-gray-300">
                {cephSummaryStats().clusters.length} cluster
                {cephSummaryStats().clusters.length !== 1 ? 's' : ''} detected
              </div>
            </div>
            <div class="text-right">
              <div class="text-sm font-semibold text-gray-900 dark:text-gray-100">
                {formatBytes(cephSummaryStats().totalBytes)}
              </div>
              <div class="text-[11px] text-gray-500 dark:text-gray-400">
                {formatPercent(cephSummaryStats().usagePercent)} used
              </div>
            </div>
          </div>
          <div class="mt-3 grid gap-3 sm:grid-cols-2">
            <For each={cephSummaryStats().clusters}>
              {(cluster) => (
                <div class="rounded-lg border border-gray-200/70 dark:border-gray-700/70 bg-white/60 dark:bg-gray-800/40 p-3">
                  <div class="flex items-start justify-between gap-2">
                    <div class="min-w-0">
                      <div class="text-sm font-semibold text-gray-900 dark:text-gray-100 truncate">
                        {cluster.name || 'Ceph Cluster'}
                      </div>
                      <Show when={cluster.healthMessage}>
                        <div
                          class="text-[11px] text-gray-500 dark:text-gray-400 truncate max-w-[240px]"
                          title={cluster.healthMessage}
                        >
                          {cluster.healthMessage}
                        </div>
                      </Show>
                    </div>
                    <span
                      class={`px-1.5 py-0.5 rounded text-[10px] font-medium ${getCephHealthStyles(
                        cluster.health,
                      )}`}
                    >
                      {getCephHealthLabel(cluster.health)}
                    </span>
                  </div>
                  <div class="mt-2">
                    <EnhancedStorageBar
                      used={cluster.usedBytes}
                      free={cluster.availableBytes}
                      total={cluster.totalBytes}
                    />
                  </div>
                </div>
              )}
            </For>
          </div>
        </Card>
      </Show>

      <StorageFilter
        search={search}
        setSearch={setSearch}
        groupBy={view() === 'pools' ? storageFilterGroupBy : undefined}
        setGroupBy={view() === 'pools' ? ((value) => setGroupBy(value)) : undefined}
        sortKey={sortKey}
        setSortKey={(value) => setSortKey(normalizeSortKey(value))}
        sortDirection={sortDirection}
        setSortDirection={setSortDirection}
        sortOptions={STORAGE_SORT_OPTIONS}
        sortDisabled={view() !== 'pools'}
        statusFilter={storageFilterStatus}
        setStatusFilter={setStorageFilterStatus}
        sourceFilter={sourceFilter}
        setSourceFilter={setSourceFilter}
        sourceOptions={sourceFilterOptions()}
        leadingFilters={
          <>
            <div class="inline-flex rounded-lg bg-gray-100 dark:bg-gray-700 p-0.5" role="group" aria-label="View">
              <button
                type="button"
                onClick={() => setView('pools')}
                aria-pressed={view() === 'pools'}
                class={`px-2.5 py-1 text-xs font-medium rounded-md transition-all ${view() === 'pools'
                  ? 'bg-white dark:bg-gray-800 text-gray-900 dark:text-gray-100 shadow-sm'
                  : 'text-gray-600 dark:text-gray-400 hover:text-gray-900 dark:hover:text-gray-100'
                  }`}
              >
                Pools
              </button>
              <button
                type="button"
                onClick={() => setView('disks')}
                aria-pressed={view() === 'disks'}
                class={`px-2.5 py-1 text-xs font-medium rounded-md transition-all ${view() === 'disks'
                  ? 'bg-white dark:bg-gray-800 text-gray-900 dark:text-gray-100 shadow-sm'
                  : 'text-gray-600 dark:text-gray-400 hover:text-gray-900 dark:hover:text-gray-100'
                  }`}
              >
                Physical Disks
              </button>
            </div>
            <div class="h-5 w-px bg-gray-200 dark:bg-gray-600 hidden sm:block"></div>
            <select
              value={selectedNodeId()}
              onChange={(event) => setSelectedNodeId(event.currentTarget.value)}
              class="px-2 py-1 text-xs border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-800 text-gray-700 dark:text-gray-200 focus:ring-2 focus:ring-blue-500/20 focus:border-blue-500"
              aria-label="Node"
            >
              <option value="all">All Nodes</option>
              <For each={nodeOptions()}>{(node) => <option value={node.id}>{node.label}</option>}</For>
            </select>
            <div class="h-5 w-px bg-gray-200 dark:bg-gray-600 hidden sm:block"></div>
          </>
        }
      />

      <Show when={reconnecting()}>
        <Card padding="sm" tone="warning">
          <div class="flex items-center justify-between gap-3">
            <span class="text-xs text-amber-800 dark:text-amber-200">Reconnecting to backend data stream…</span>
            <button
              type="button"
              onClick={() => reconnect()}
              class="rounded border border-amber-300 bg-amber-100 px-2 py-1 text-xs font-medium text-amber-800 hover:bg-amber-200 dark:border-amber-700 dark:bg-amber-900/40 dark:text-amber-200 dark:hover:bg-amber-900/60"
            >
              Retry now
            </button>
          </div>
        </Card>
      </Show>

      <Show when={hasFetchError()}>
        <Card padding="sm" tone="warning">
          <div class="text-xs text-amber-800 dark:text-amber-200">
            Unable to refresh storage resources. Showing latest available data.
          </div>
        </Card>
      </Show>

      <Show when={isDisconnectedAfterLoad()}>
        <Card padding="sm" tone="warning">
          <div class="flex items-center justify-between gap-3">
            <span class="text-xs text-amber-800 dark:text-amber-200">
              Storage data stream disconnected. Data may be stale.
            </span>
            <button
              type="button"
              onClick={() => reconnect()}
              class="rounded border border-amber-300 bg-amber-100 px-2 py-1 text-xs font-medium text-amber-800 hover:bg-amber-200 dark:border-amber-700 dark:bg-amber-900/40 dark:text-amber-200 dark:hover:bg-amber-900/60"
            >
              Reconnect
            </button>
          </div>
        </Card>
      </Show>

      <Show when={isWaitingForData()}>
        <Card padding="sm" tone="warning">
          <div class="text-xs text-amber-800 dark:text-amber-200">
            Waiting for storage data from connected platforms.
          </div>
        </Card>
      </Show>

      <Card padding="none" class="overflow-hidden">
        <Show when={view() === 'disks'}>
          <div class="p-2">
            <DiskList
              disks={physicalDisks()}
              nodes={nodes()}
              selectedNode={selectedNode()?.id || null}
              searchTerm={search()}
            />
          </div>
        </Show>
        <Show when={view() === 'pools'}>
          <Show
            when={isLoadingPools()}
            fallback={
              <Show
                when={groupedRecords().length > 0}
                fallback={
                  <div class="p-6 text-sm text-gray-600 dark:text-gray-300">
                    No storage records match the current filters.
                  </div>
                }
              >
                <div class="overflow-x-auto">
                  <table class="w-full text-xs">
                    <thead>
                      <tr class="border-b border-gray-200 bg-gray-50 text-left text-[10px] uppercase tracking-wide text-gray-500 dark:border-gray-700 dark:bg-gray-800/60 dark:text-gray-400">
                        <th class="px-1.5 sm:px-2 py-1">Name</th>
                        <Show when={groupBy() !== 'node'}>
                          <th class="px-1.5 sm:px-2 py-1">Node</th>
                        </Show>
                        <th class="px-1.5 sm:px-2 py-1">Type</th>
                        <th class="px-1.5 sm:px-2 py-1 min-w-[180px]">Capacity</th>
                        <th class="px-1.5 sm:px-2 py-1 w-[120px] hidden md:table-cell">Trend</th>
                        <th class="px-1.5 sm:px-2 py-1">Health</th>
                        <th class="px-1.5 sm:px-2 py-1 w-10" />
                      </tr>
                    </thead>
                    <tbody class="divide-y divide-gray-200 dark:divide-gray-700">
                      <For each={groupedRecords()}>
                        {(group) => (
                          <>
                            <StorageGroupRow
                              group={group}
                              groupBy={groupBy()}
                              expanded={expandedGroups().has(group.key)}
                              onToggle={() => toggleGroup(group.key)}
                            />
                            <Show when={expandedGroups().has(group.key)}>
                            <For each={group.items}>
                              {(record) => {
                                const isExpanded = () => expandedPoolId() === record.id;
                                const alertState = createMemo(() => getRecordAlertState(record.id));
                                const nodeLabel = getRecordNodeLabel(record).trim().toLowerCase();
                                const parentNodeOnline = createMemo(() => {
                                  if (!nodeLabel) return true;
                                  const nodeStatus = nodeOnlineByLabel().get(nodeLabel);
                                  return nodeStatus === undefined ? true : nodeStatus;
                                });
                                const showAlertHighlight = createMemo(
                                  () => alertState().hasUnacknowledgedAlert && parentNodeOnline(),
                                );
                                const hasAcknowledgedOnlyAlert = createMemo(
                                  () => alertState().hasAcknowledgedOnlyAlert && parentNodeOnline(),
                                );
                                const isResourceHighlighted = () => highlightedRecordId() === record.id;
                                const rowClass = createMemo(() => {
                                  const classes = [
                                    'transition-all duration-200',
                                    'hover:bg-gray-50 dark:hover:bg-gray-800/30',
                                  ];

                                  if (showAlertHighlight()) {
                                    classes.push(
                                      alertState().severity === 'critical'
                                        ? 'bg-red-50 dark:bg-red-950/30'
                                        : 'bg-yellow-50 dark:bg-yellow-950/20',
                                    );
                                  } else if (isResourceHighlighted()) {
                                    classes.push('bg-blue-50/60 dark:bg-blue-900/20 ring-1 ring-blue-300 dark:ring-blue-600');
                                  } else if (hasAcknowledgedOnlyAlert()) {
                                    classes.push('bg-gray-50/40 dark:bg-gray-800/40');
                                  }

                                  if (isExpanded()) {
                                    classes.push('bg-gray-50 dark:bg-gray-800/40');
                                  }

                                  return classes.join(' ');
                                });

                                const rowStyle = createMemo(() => {
                                  if (showAlertHighlight()) {
                                    return {
                                      'box-shadow': `inset 4px 0 0 0 ${
                                        alertState().severity === 'critical' ? '#ef4444' : '#eab308'
                                      }`,
                                    };
                                  }
                                  if (hasAcknowledgedOnlyAlert()) {
                                    return {
                                      'box-shadow': 'inset 4px 0 0 0 rgba(156, 163, 175, 0.8)',
                                    };
                                  }
                                  return {} as Record<string, string>;
                                });

                                return (
                                  <StoragePoolRow
                                    record={record}
                                    groupBy={groupBy()}
                                    expanded={isExpanded()}
                                    groupExpanded={expandedGroups().has(group.key)}
                                    onToggleExpand={() =>
                                      setExpandedPoolId((current) =>
                                        current === record.id ? null : record.id,
                                      )
                                    }
                                    rowClass={rowClass()}
                                    rowStyle={rowStyle()}
                                    physicalDisks={physicalDisks()}
                                    alertDataAttrs={{
                                      'data-row-id': record.id,
                                      'data-alert-state': showAlertHighlight()
                                        ? 'unacknowledged'
                                        : hasAcknowledgedOnlyAlert()
                                          ? 'acknowledged'
                                          : 'none',
                                      'data-alert-severity': alertState().severity || 'none',
                                      'data-resource-highlighted': isResourceHighlighted() ? 'true' : 'false',
                                    }}
                                  />
                                );
                              }}
                            </For>
                            </Show>
                          </>
                        )}
                      </For>
                    </tbody>
                  </table>
                </div>
              </Show>
            }
          >
            <div class="p-6 text-sm text-gray-600 dark:text-gray-300">Loading storage resources...</div>
          </Show>
        </Show>
      </Card>
    </div>
  );
};

export default Storage;
