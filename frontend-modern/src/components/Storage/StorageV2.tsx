import { useLocation, useNavigate } from '@solidjs/router';
import { Component, For, Show, createEffect, createMemo, createSignal, onCleanup } from 'solid-js';
import { useWebSocket } from '@/App';
import { Card } from '@/components/shared/Card';
import { StatusDot } from '@/components/shared/StatusDot';
import { DiskList } from '@/components/Storage/DiskList';
import { EnhancedStorageBar } from '@/components/Storage/EnhancedStorageBar';
import { ZFSHealthMap } from '@/components/Storage/ZFSHealthMap';
import { useAlertsActivation } from '@/stores/alertsActivation';
import { buildStorageRecordsV2 } from '@/features/storageBackupsV2/storageAdapters';
import {
  getCephHealthLabel,
  getCephHealthStyles,
} from '@/features/storageBackupsV2/storageDomain';
import { PLATFORM_BLUEPRINTS } from '@/features/storageBackupsV2/platformBlueprint';
import type { NormalizedHealth, StorageRecordV2 } from '@/features/storageBackupsV2/models';
import { useStorageBackupsResources } from '@/hooks/useUnifiedResources';
import {
  buildStoragePath,
  parseStorageLinkSearch,
} from '@/routing/resourceLinks';
import { formatBytes, formatPercent } from '@/utils/format';
import { useStorageRouteState } from './useStorageRouteState';
import { getCephClusterKeyFromRecord, isCephRecord, useStorageV2CephModel } from './useStorageV2CephModel';
import { useStorageV2AlertState } from './useStorageV2AlertState';
import {
  type StorageGroupKey,
  type StorageSortKey,
  getRecordContent,
  getRecordNodeLabel,
  getRecordShared,
  getRecordStatus,
  getRecordType,
  getRecordUsagePercent,
  getRecordZfsPool,
  sourceLabel,
  useStorageV2Model,
} from './useStorageV2Model';

const HEALTH_CLASS: Record<NormalizedHealth, string> = {
  healthy: 'text-green-700 dark:text-green-300',
  warning: 'text-yellow-700 dark:text-yellow-300',
  critical: 'text-red-700 dark:text-red-300',
  offline: 'text-gray-600 dark:text-gray-300',
  unknown: 'text-gray-500 dark:text-gray-400',
};

type StorageV2View = 'pools' | 'disks';

const STORAGE_SORT_OPTIONS: Array<{ value: StorageSortKey; label: string }> = [
  { value: 'name', label: 'Name' },
  { value: 'usage', label: 'Usage %' },
  { value: 'type', label: 'Type' },
];

const STORAGE_GROUP_OPTIONS: Array<{ value: StorageGroupKey; label: string }> = [
  { value: 'node', label: 'Node' },
  { value: 'type', label: 'Type' },
  { value: 'status', label: 'Status' },
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

const normalizeView = (value: string): StorageV2View => (value === 'disks' ? 'disks' : 'pools');

const normalizeSortDirection = (value: string): 'asc' | 'desc' =>
  value === 'desc' ? 'desc' : 'asc';

const getStorageMetaBoolean = (record: StorageRecordV2, key: 'isCeph' | 'isZfs'): boolean | null => {
  const details = (record.details || {}) as Record<string, unknown>;
  const value = details[key];
  return typeof value === 'boolean' ? value : null;
};

const isRecordCeph = (record: StorageRecordV2): boolean => {
  const isCephMeta = getStorageMetaBoolean(record, 'isCeph');
  if (isCephMeta !== null) return isCephMeta;
  return isCephRecord(record);
};

const StorageV2: Component = () => {
  const navigate = useNavigate();
  const location = useLocation();
  const { state, activeAlerts, connected, initialDataReceived, reconnecting, reconnect } = useWebSocket();
  const storageBackupsResources = useStorageBackupsResources();
  const alertsActivation = useAlertsActivation();
  const alertsEnabled = createMemo(() => alertsActivation.activationState() === 'active');

  const [search, setSearch] = createSignal('');
  const [sourceFilter, setSourceFilter] = createSignal('all');
  const [healthFilter, setHealthFilter] = createSignal<'all' | NormalizedHealth>('all');
  const [view, setView] = createSignal<StorageV2View>('pools');
  const [selectedNodeId, setSelectedNodeId] = createSignal('all');
  const [sortKey, setSortKey] = createSignal<StorageSortKey>('name');
  const [sortDirection, setSortDirection] = createSignal<'asc' | 'desc'>('asc');
  const [groupBy, setGroupBy] = createSignal<StorageGroupKey>('node');
  const [expandedCephRecordId, setExpandedCephRecordId] = createSignal<string | null>(null);
  const [highlightedRecordId, setHighlightedRecordId] = createSignal<string | null>(null);
  const [handledResourceId, setHandledResourceId] = createSignal<string | null>(null);
  let highlightTimer: number | undefined;

  const adapterResources = createMemo(() => {
    const unifiedResources = storageBackupsResources.resources();
    return unifiedResources;
  });

  const records = createMemo(() => buildStorageRecordsV2({ state, resources: adapterResources() }));
  const activeAlertsAccessor = () => {
    if (typeof activeAlerts === 'function') {
      return (activeAlerts as () => unknown)();
    }
    return activeAlerts;
  };
  const { getRecordAlertState } = useStorageV2AlertState({
    records,
    activeAlerts: activeAlertsAccessor,
    alertsEnabled,
  });

  const nodeOptions = createMemo(() => {
    const nodes = state.nodes || [];
    return nodes.map((node) => ({ id: node.id, label: node.name, instance: node.instance }));
  });

  const nodeOnlineByLabel = createMemo(() => {
    const map = new Map<string, boolean>();
    (state.nodes || []).forEach((node) => {
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

  const { sourceOptions, selectedNode, filteredRecords, groupedRecords, summary } = useStorageV2Model({
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

  const { cephSummaryStats, resolveCephCluster, getCephSummaryText, getCephPoolsText } = useStorageV2CephModel({
    records,
    cephClusters: () => state.cephClusters,
  });

  const nextPlatforms = createMemo(() =>
    PLATFORM_BLUEPRINTS.filter((platform) => platform.stage === 'next').map((platform) => platform.label),
  );

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
  const hasV2FetchError = createMemo(() => Boolean(storageBackupsResources.error()));

  const isActiveStorageRoute = () => location.pathname === '/storage';

  createEffect(() => {
    const { resource } = parseStorageLinkSearch(location.search);
    if (!resource || resource === handledResourceId()) return;

    const match = records().find((record) => record.id === resource || record.name === resource);
    if (!match) return;

    if (isRecordCeph(match)) {
      setExpandedCephRecordId(match.id);
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
      setExpandedCephRecordId(null);
    }
  });

  onCleanup(() => {
    if (highlightTimer) window.clearTimeout(highlightTimer);
  });

  return (
    <div class="space-y-4">
      <Card padding="md" tone="glass">
        <div class="flex flex-wrap items-center justify-between gap-3">
          <div>
            <h2 class="text-sm font-semibold text-gray-900 dark:text-gray-100">Storage</h2>
            <p class="text-xs text-gray-600 dark:text-gray-400">
              Storage capacity and health across connected platforms.
            </p>
          </div>
          <div class="text-xs text-gray-500 dark:text-gray-400">
            Next platforms: {nextPlatforms().join(', ')}
          </div>
        </div>
      </Card>

      <Card padding="md">
        <div class="grid gap-3 sm:grid-cols-4">
          <div>
            <div class="text-[11px] uppercase text-gray-500 dark:text-gray-400">Records</div>
            <div class="text-lg font-semibold text-gray-900 dark:text-gray-100">{summary().count}</div>
          </div>
          <div>
            <div class="text-[11px] uppercase text-gray-500 dark:text-gray-400">Total Capacity</div>
            <div class="text-lg font-semibold text-gray-900 dark:text-gray-100">
              {formatBytes(summary().totalBytes)}
            </div>
          </div>
          <div>
            <div class="text-[11px] uppercase text-gray-500 dark:text-gray-400">Used</div>
            <div class="text-lg font-semibold text-gray-900 dark:text-gray-100">
              {formatBytes(summary().usedBytes)}
            </div>
          </div>
          <div>
            <div class="text-[11px] uppercase text-gray-500 dark:text-gray-400">Usage</div>
            <div class="text-lg font-semibold text-gray-900 dark:text-gray-100">
              {formatPercent(summary().usagePercent)}
            </div>
          </div>
        </div>
      </Card>

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

      <Card padding="md">
        <div class="grid gap-2 md:grid-cols-[minmax(140px,1fr)_minmax(180px,1fr)_minmax(120px,1fr)_minmax(140px,1fr)_minmax(140px,1fr)_minmax(140px,1fr)_minmax(130px,1fr)_auto]">
          <select
            value={view()}
            onChange={(event) => setView(event.currentTarget.value as StorageV2View)}
            class="rounded border border-gray-300 bg-white px-3 py-2 text-sm text-gray-900 outline-none focus:border-blue-500 dark:border-gray-700 dark:bg-gray-900 dark:text-gray-100"
            aria-label="View"
          >
            <option value="pools">Pools</option>
            <option value="disks">Physical Disks</option>
          </select>
          <select
            value={selectedNodeId()}
            onChange={(event) => setSelectedNodeId(event.currentTarget.value)}
            class="rounded border border-gray-300 bg-white px-3 py-2 text-sm text-gray-900 outline-none focus:border-blue-500 dark:border-gray-700 dark:bg-gray-900 dark:text-gray-100"
            aria-label="Node"
          >
            <option value="all">all nodes</option>
            <For each={nodeOptions()}>{(node) => <option value={node.id}>{node.label}</option>}</For>
          </select>
          <select
            value={sourceFilter()}
            onChange={(event) => setSourceFilter(event.currentTarget.value)}
            class="rounded border border-gray-300 bg-white px-3 py-2 text-sm text-gray-900 outline-none focus:border-blue-500 dark:border-gray-700 dark:bg-gray-900 dark:text-gray-100"
            aria-label="Source"
          >
            <For each={sourceOptions()}>
              {(option) => <option value={option}>{option === 'all' ? 'all sources' : sourceLabel(option)}</option>}
            </For>
          </select>
          <select
            value={healthFilter()}
            onChange={(event) => setHealthFilter(event.currentTarget.value as 'all' | NormalizedHealth)}
            class="rounded border border-gray-300 bg-white px-3 py-2 text-sm text-gray-900 outline-none focus:border-blue-500 dark:border-gray-700 dark:bg-gray-900 dark:text-gray-100"
            aria-label="Health"
          >
            <option value="all">all health states</option>
            <option value="healthy">healthy</option>
            <option value="warning">warning</option>
            <option value="critical">critical</option>
            <option value="offline">offline</option>
            <option value="unknown">unknown</option>
          </select>
          <select
            value={groupBy()}
            onChange={(event) => setGroupBy(event.currentTarget.value as StorageGroupKey)}
            class="rounded border border-gray-300 bg-white px-3 py-2 text-sm text-gray-900 outline-none focus:border-blue-500 dark:border-gray-700 dark:bg-gray-900 dark:text-gray-100"
            aria-label="Group By"
            disabled={view() !== 'pools'}
          >
            <For each={STORAGE_GROUP_OPTIONS}>
              {(option) => <option value={option.value}>{`group: ${option.label.toLowerCase()}`}</option>}
            </For>
          </select>
          <select
            value={sortKey()}
            onChange={(event) => setSortKey(event.currentTarget.value as StorageSortKey)}
            class="rounded border border-gray-300 bg-white px-3 py-2 text-sm text-gray-900 outline-none focus:border-blue-500 dark:border-gray-700 dark:bg-gray-900 dark:text-gray-100"
            aria-label="Sort By"
            disabled={view() !== 'pools'}
          >
            <For each={STORAGE_SORT_OPTIONS}>
              {(option) => <option value={option.value}>{`sort: ${option.label.toLowerCase()}`}</option>}
            </For>
          </select>
          <div class="flex items-center justify-start md:justify-center">
            <button
              type="button"
              onClick={() => setSortDirection((current) => (current === 'asc' ? 'desc' : 'asc'))}
              class="inline-flex h-10 w-10 items-center justify-center rounded border border-gray-300 bg-white text-gray-600 hover:bg-gray-50 dark:border-gray-700 dark:bg-gray-900 dark:text-gray-300 dark:hover:bg-gray-800 disabled:opacity-50"
              aria-label="Sort Direction"
              disabled={view() !== 'pools'}
            >
              <svg
                class={`h-4 w-4 transition-transform ${sortDirection() === 'asc' ? 'rotate-180' : ''}`}
                viewBox="0 0 24 24"
                fill="none"
                stroke="currentColor"
                stroke-width="2"
              >
                <path stroke-linecap="round" stroke-linejoin="round" d="M8 9l4-4 4 4m0 6l-4 4-4-4" />
              </svg>
            </button>
          </div>
          <input
            type="text"
            value={search()}
            onInput={(event) => setSearch(event.currentTarget.value)}
            placeholder="Search name, location, source, capability..."
            aria-label="Search"
            class="rounded border border-gray-300 bg-white px-3 py-2 text-sm text-gray-900 outline-none focus:border-blue-500 md:col-span-3 dark:border-gray-700 dark:bg-gray-900 dark:text-gray-100"
          />
        </div>
      </Card>

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

      <Show when={hasV2FetchError()}>
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
              disks={state.physicalDisks || []}
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
                  <table class="w-full text-sm">
                    <thead>
                      <tr class="border-b border-gray-200 bg-gray-50 text-left text-xs uppercase tracking-wide text-gray-500 dark:border-gray-700 dark:bg-gray-800/60 dark:text-gray-400">
                        <th class="px-3 py-2">Name</th>
                        <th class="px-3 py-2">Node</th>
                        <th class="px-3 py-2">Type</th>
                        <th class="px-3 py-2">Content</th>
                        <th class="px-3 py-2">Status</th>
                        <th class="px-3 py-2">Shared</th>
                        <th class="px-3 py-2">Used</th>
                        <th class="px-3 py-2">Free</th>
                        <th class="px-3 py-2">Total</th>
                        <th class="px-3 py-2 min-w-[180px]">Usage</th>
                        <th class="px-3 py-2">Health</th>
                        <th class="px-3 py-2 w-10" />
                      </tr>
                    </thead>
                    <tbody class="divide-y divide-gray-200 dark:divide-gray-700">
                      <For each={groupedRecords()}>
                        {(group) => (
                          <>
                            <tr class="bg-gray-50 dark:bg-gray-900/40">
                              <td colSpan={12} class="px-3 py-1.5 text-[12px] font-semibold text-slate-700 dark:text-slate-300">
                                {groupBy() === 'node'
                                  ? `Node: ${group.key}`
                                  : groupBy() === 'type'
                                    ? `Type: ${group.key}`
                                    : `Status: ${group.key}`}{' '}
                                <span class="ml-1 text-[10px] font-normal text-slate-400 dark:text-slate-500">
                                  {group.items.length} {group.items.length === 1 ? 'record' : 'records'}
                                </span>
                              </td>
                            </tr>
                            <For each={group.items}>
                              {(record) => {
                                const zfsPool = getRecordZfsPool(record);
                                const cephCluster = resolveCephCluster(record);
                                const isCeph = isRecordCeph(record);
                                const isExpanded = () => expandedCephRecordId() === record.id;
                                const alertState = createMemo(() => getRecordAlertState(record.id));
                                const status = getRecordStatus(record);
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
                                const totalBytes = record.capacity.totalBytes || 0;
                                const usedBytes = record.capacity.usedBytes || 0;
                                const freeBytes =
                                  record.capacity.freeBytes ?? (totalBytes > 0 ? Math.max(totalBytes - usedBytes, 0) : 0);
                                const usagePercent = getRecordUsagePercent(record);
                                const hasZfsIssues = Boolean(
                                  zfsPool &&
                                    (zfsPool.state !== 'ONLINE' ||
                                      zfsPool.readErrors > 0 ||
                                      zfsPool.writeErrors > 0 ||
                                      zfsPool.checksumErrors > 0),
                                );
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

                                  if (isCeph && isExpanded()) {
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
                                  <>
                                    <tr
                                      class={rowClass()}
                                      style={rowStyle()}
                                      data-row-id={record.id}
                                      data-alert-state={
                                        showAlertHighlight()
                                          ? 'unacknowledged'
                                          : hasAcknowledgedOnlyAlert()
                                            ? 'acknowledged'
                                            : 'none'
                                      }
                                      data-alert-severity={alertState().severity || 'none'}
                                      data-resource-highlighted={isResourceHighlighted() ? 'true' : 'false'}
                                    >
                                      <td
                                        class={`${
                                          showAlertHighlight() || hasAcknowledgedOnlyAlert() ? 'px-2' : 'px-3'
                                        } py-2 text-gray-900 dark:text-gray-100`}
                                      >
                                        <div class="flex items-center gap-1.5 min-w-0">
                                          <span class="truncate max-w-[220px]" title={record.name}>
                                            {record.name}
                                          </span>
                                          <Show when={zfsPool && zfsPool.devices.length > 0}>
                                            <span class="mx-1">
                                              <ZFSHealthMap pool={zfsPool!} />
                                            </span>
                                          </Show>
                                          <Show when={zfsPool && zfsPool.state !== 'ONLINE'}>
                                            <span
                                              class={`px-1.5 py-0.5 rounded text-[10px] font-medium ${zfsPool?.state === 'DEGRADED'
                                                ? 'bg-yellow-100 text-yellow-700 dark:bg-yellow-900 dark:text-yellow-300'
                                                : 'bg-red-100 text-red-700 dark:bg-red-900 dark:text-red-300'
                                                }`}
                                            >
                                              {zfsPool?.state}
                                            </span>
                                          </Show>
                                          <Show
                                            when={
                                              zfsPool &&
                                              (zfsPool.readErrors > 0 ||
                                                zfsPool.writeErrors > 0 ||
                                                zfsPool.checksumErrors > 0)
                                            }
                                          >
                                            <span class="px-1.5 py-0.5 rounded text-[10px] font-medium bg-red-100 text-red-700 dark:bg-red-900 dark:text-red-300">
                                              ERRORS
                                            </span>
                                          </Show>
                                          <Show
                                            when={
                                              isCeph &&
                                              cephCluster?.health &&
                                              cephCluster.health.toUpperCase() !== 'HEALTH_UNKNOWN'
                                            }
                                          >
                                            <span
                                              class={`px-1.5 py-0.5 rounded text-[10px] font-medium ${getCephHealthStyles(
                                                cephCluster?.health,
                                              )}`}
                                              title={cephCluster?.healthMessage}
                                            >
                                              {getCephHealthLabel(cephCluster?.health)}
                                            </span>
                                          </Show>
                                        </div>
                                      </td>
                                      <td class="px-3 py-2 text-gray-700 dark:text-gray-300">{getRecordNodeLabel(record)}</td>
                                      <td class="px-3 py-2 text-gray-700 dark:text-gray-300">{getRecordType(record)}</td>
                                      <td class="px-3 py-2 text-gray-700 dark:text-gray-300">{getRecordContent(record) || '-'}</td>
                                      <td class="px-3 py-2">
                                        <div class="flex items-center gap-1.5 whitespace-nowrap">
                                          <StatusDot
                                            variant={
                                              status === 'available' || status === 'online' ? 'success' : 'danger'
                                            }
                                            size="xs"
                                          />
                                          <span class="text-xs text-gray-700 dark:text-gray-300">{status}</span>
                                        </div>
                                      </td>
                                      <td class="px-3 py-2 text-gray-700 dark:text-gray-300">
                                        {getRecordShared(record) === null ? '-' : getRecordShared(record) ? 'yes' : 'no'}
                                      </td>
                                      <td class="px-3 py-2 text-gray-700 dark:text-gray-300">
                                        {totalBytes > 0 || usedBytes > 0 ? formatBytes(usedBytes) : 'n/a'}
                                      </td>
                                      <td class="px-3 py-2 text-gray-700 dark:text-gray-300">
                                        {totalBytes > 0 || freeBytes > 0 ? formatBytes(freeBytes) : 'n/a'}
                                      </td>
                                      <td class="px-3 py-2 text-gray-700 dark:text-gray-300">
                                        {totalBytes > 0 ? formatBytes(totalBytes) : 'n/a'}
                                      </td>
                                      <td class="px-3 py-2 text-gray-700 dark:text-gray-300 min-w-[180px]">
                                        <EnhancedStorageBar
                                          used={usedBytes}
                                          total={Math.max(totalBytes, 0)}
                                          free={Math.max(freeBytes, 0)}
                                          zfsPool={zfsPool || undefined}
                                        />
                                        <div class="mt-1 text-[11px] text-gray-500 dark:text-gray-400">
                                          {formatPercent(usagePercent)}
                                        </div>
                                      </td>
                                      <td class={`px-3 py-2 font-medium ${HEALTH_CLASS[record.health]}`}>
                                        {record.health}
                                      </td>
                                      <td class="px-3 py-2 text-right">
                                        <Show
                                          when={isCeph}
                                          fallback={<span class="text-xs text-gray-400 dark:text-gray-500">-</span>}
                                        >
                                          <button
                                            type="button"
                                            onClick={() =>
                                              setExpandedCephRecordId((current) =>
                                                current === record.id ? null : record.id,
                                              )
                                            }
                                            class="rounded border border-gray-300 px-2 py-1 text-[11px] text-gray-700 hover:bg-gray-100 dark:border-gray-700 dark:text-gray-300 dark:hover:bg-gray-800"
                                            aria-label={`Toggle Ceph details for ${record.name}`}
                                          >
                                            {isExpanded() ? 'Hide' : 'Show'}
                                          </button>
                                        </Show>
                                      </td>
                                    </tr>
                                    <Show when={isCeph && isExpanded()}>
                                      <tr class="text-[11px] border-t border-gray-200 bg-gray-50/60 text-gray-700 dark:border-gray-700 dark:bg-gray-900/30 dark:text-gray-300">
                                        <td colSpan={12} class="px-4 py-3">
                                          <div class="grid gap-3 md:grid-cols-2">
                                            <div class="rounded-lg border border-gray-200 bg-white/80 p-4 shadow-sm dark:border-gray-600/60 dark:bg-gray-900/30">
                                              <div class="flex items-center gap-2 text-xs font-semibold text-gray-700 dark:text-gray-200">
                                                <span>Ceph Cluster</span>
                                                <span>{cephCluster?.name || getCephClusterKeyFromRecord(record)}</span>
                                                <Show when={cephCluster?.health}>
                                                  <span
                                                    class={`px-1.5 py-0.5 rounded text-[10px] font-medium ${getCephHealthStyles(
                                                      cephCluster?.health,
                                                    )}`}
                                                  >
                                                    {getCephHealthLabel(cephCluster?.health)}
                                                  </span>
                                                </Show>
                                              </div>
                                              <Show when={getCephSummaryText(record, cephCluster)}>
                                                <div class="mt-2 text-[12px] text-gray-600 dark:text-gray-300">
                                                  {getCephSummaryText(record, cephCluster)}
                                                </div>
                                              </Show>
                                              <Show when={cephCluster?.healthMessage}>
                                                <div class="mt-2 text-[11px] text-gray-500 dark:text-gray-400">
                                                  {cephCluster?.healthMessage}
                                                </div>
                                              </Show>
                                            </div>
                                            <div class="rounded-lg border border-gray-200 bg-white/80 p-4 shadow-sm dark:border-gray-600/60 dark:bg-gray-900/30 text-gray-600 dark:text-gray-300">
                                              <div class="text-xs font-semibold text-gray-700 dark:text-gray-200">Pools</div>
                                              <div class="mt-2 text-[12px]">
                                                {getCephPoolsText(record, cephCluster) || 'No pool data available'}
                                              </div>
                                            </div>
                                          </div>
                                        </td>
                                      </tr>
                                    </Show>
                                    <Show when={hasZfsIssues}>
                                      <tr class="bg-yellow-50 dark:bg-yellow-950/20 border-l-4 border-yellow-500">
                                        <td colSpan={12} class="p-2">
                                          <div class="text-xs space-y-1">
                                            <div class="flex items-center gap-2">
                                              <span class="font-semibold text-yellow-700 dark:text-yellow-400">
                                                ZFS Pool Status:
                                              </span>
                                              <span
                                                class={`px-1.5 py-0.5 rounded text-xs font-medium ${zfsPool?.state === 'ONLINE'
                                                  ? 'bg-green-100 text-green-700 dark:bg-green-900 dark:text-green-300'
                                                  : zfsPool?.state === 'DEGRADED'
                                                    ? 'bg-yellow-100 text-yellow-700 dark:bg-yellow-900 dark:text-yellow-300'
                                                    : 'bg-red-100 text-red-700 dark:bg-red-900 dark:text-red-300'
                                                  }`}
                                              >
                                                {zfsPool?.state}
                                              </span>
                                              <Show
                                                when={
                                                  zfsPool &&
                                                  (zfsPool.readErrors > 0 ||
                                                    zfsPool.writeErrors > 0 ||
                                                    zfsPool.checksumErrors > 0)
                                                }
                                              >
                                                <span class="text-red-600 dark:text-red-400">
                                                  Errors: {zfsPool?.readErrors} read, {zfsPool?.writeErrors} write,{' '}
                                                  {zfsPool?.checksumErrors} checksum
                                                </span>
                                              </Show>
                                            </div>
                                          </div>
                                        </td>
                                      </tr>
                                    </Show>
                                  </>
                                );
                              }}
                            </For>
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

export default StorageV2;
