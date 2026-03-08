import { useLocation, useNavigate } from '@solidjs/router';
import {
  Component,
  For,
  Index,
  Show,
  createEffect,
  createMemo,
  createSignal,
  onCleanup,
} from 'solid-js';
import { useWebSocket } from '@/App';
import { useResources } from '@/hooks/useResources';
import { Card } from '@/components/shared/Card';
import { Subtabs } from '@/components/shared/Subtabs';
import { Table, TableHeader, TableBody, TableRow, TableHead } from '@/components/shared/Table';
import { DiskList } from '@/components/Storage/DiskList';
import { EnhancedStorageBar } from '@/components/Storage/EnhancedStorageBar';
import StorageSummary from '@/components/Storage/StorageSummary';
import type { SummaryTimeRange } from '@/components/shared/summaryTimeRange';
import { useAlertsActivation } from '@/stores/alertsActivation';
import { buildStorageRecords } from '@/features/storageBackups/storageAdapters';
import { getCephHealthLabel, getCephHealthStyles } from '@/features/storageBackups/storageDomain';
import type { NormalizedHealth, StorageRecord } from '@/features/storageBackups/models';
import { useStorageRecoveryResources } from '@/hooks/useUnifiedResources';
import { buildStoragePath, parseStorageLinkSearch } from '@/routing/resourceLinks';
import { formatBytes, formatPercent } from '@/utils/format';
import { getProxmoxData } from '@/utils/resourcePlatformData';
import { useKioskMode } from '@/hooks/useKioskMode';
import { getResourceIdentityAliases } from '@/utils/resourceIdentity';
import { buildStorageSourceOptionsFromKeys } from '@/utils/storageSources';
import { useStorageRouteState } from './useStorageRouteState';
import { isCephRecord, useStorageCephModel } from './useStorageCephModel';
import { useStorageAlertState } from './useStorageAlertState';
import {
  StorageFilter,
  type StorageGroupByFilter,
  type StorageStatusFilter,
} from './StorageFilter';
import { matchesPhysicalDiskNode } from './diskResourceUtils';
import { StorageGroupRow } from './StorageGroupRow';
import { StoragePoolRow } from './StoragePoolRow';
import {
  type StorageGroupKey,
  type StorageSortKey,
  getRecordNodeLabel,
  useStorageModel,
} from './useStorageModel';

type StorageView = 'pools' | 'disks';

const STORAGE_SORT_OPTIONS: Array<{ value: StorageSortKey; label: string }> = [
  { value: 'priority', label: 'Priority' },
  { value: 'name', label: 'Name' },
  { value: 'usage', label: 'Usage %' },
  { value: 'type', label: 'Type' },
];

const normalizeHealthFilter = (value: string): 'all' | NormalizedHealth => {
  const normalized = (value || '').trim().toLowerCase();
  if (!normalized || normalized === 'all') return 'all';
  if (normalized === 'available' || normalized === 'online' || normalized === 'healthy')
    return 'healthy';
  if (normalized === 'degraded' || normalized === 'warning') return 'warning';
  if (normalized === 'critical') return 'critical';
  if (normalized === 'offline') return 'offline';
  if (normalized === 'unknown') return 'unknown';
  return 'all';
};

const normalizeSortKey = (value: string): StorageSortKey => {
  if (value === 'priority' || value === 'name' || value === 'usage' || value === 'type')
    return value;
  return 'priority';
};

const normalizeGroupKey = (value: string): StorageGroupKey => {
  if (value === 'none' || value === 'node' || value === 'type' || value === 'status') return value;
  return 'none';
};

const normalizeView = (value: string): StorageView => (value === 'disks' ? 'disks' : 'pools');

const normalizeSortDirection = (value: string): 'asc' | 'desc' =>
  value === 'asc' ? 'asc' : 'desc';

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
  const { state, activeAlerts, connected, initialDataReceived, reconnecting, reconnect } =
    useWebSocket();
  const { byType } = useResources();

  const kioskMode = useKioskMode();

  const nodes = createMemo(() => byType('agent'));
  const physicalDisks = createMemo(() => byType('physical_disk'));
  const cephResources = createMemo(() => byType('ceph'));
  const storageRecoveryResources = useStorageRecoveryResources();
  const alertsActivation = useAlertsActivation();
  const alertsEnabled = createMemo(() => alertsActivation.activationState() === 'active');

  const [summaryTimeRange, setSummaryTimeRange] = createSignal<SummaryTimeRange>('1h');
  const [search, setSearch] = createSignal('');
  const [sourceFilter, setSourceFilter] = createSignal('all');
  const [healthFilter, setHealthFilter] = createSignal<'all' | NormalizedHealth>('all');
  const [view, setView] = createSignal<StorageView>('pools');
  const [selectedNodeId, setSelectedNodeId] = createSignal('all');
  const [sortKey, setSortKey] = createSignal<StorageSortKey>('priority');
  const [sortDirection, setSortDirection] = createSignal<'asc' | 'desc'>('desc');
  const [groupBy, setGroupBy] = createSignal<StorageGroupKey>('none');
  const [expandedGroups, setExpandedGroups] = createSignal<Set<string>>(new Set());
  const [expandedPoolId, setExpandedPoolId] = createSignal<string | null>(null);
  const [highlightedRecordId, setHighlightedRecordId] = createSignal<string | null>(null);
  const [handledResourceId, setHandledResourceId] = createSignal<string | null>(null);
  let highlightTimer: number | undefined;

  const adapterResources = createMemo(() => {
    const unifiedResources = storageRecoveryResources.resources();
    return unifiedResources;
  });

  const records = createMemo(() => buildStorageRecords({ state, resources: adapterResources() }));
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
      aliases: getResourceIdentityAliases(node),
    }));
  });

  const diskNodeOptions = createMemo(() => {
    return nodeOptions().filter((node) =>
      physicalDisks().some((disk) =>
        matchesPhysicalDiskNode(disk, {
          id: node.id,
          name: node.label,
          instance: node.instance,
        }),
      ),
    );
  });

  const activeNodeOptions = createMemo(() =>
    view() === 'disks' ? diskNodeOptions() : nodeOptions(),
  );

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

  const { sourceOptions, filteredRecords, groupedRecords } = useStorageModel({
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
        if (!next.has(key)) {
          next.add(key);
          changed = true;
        }
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

  const storageFilterGroupBy = (): StorageGroupByFilter => {
    const current = groupBy();
    return current === 'type' || current === 'status' || current === 'none' ? current : 'node';
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

  const sourceFilterOptions = createMemo(() => buildStorageSourceOptionsFromKeys(sourceOptions()));

  const isWaitingForData = createMemo(
    () =>
      storageRecoveryResources.loading() &&
      filteredRecords().length === 0 &&
      !connected() &&
      !initialDataReceived(),
  );
  const isDisconnectedAfterLoad = createMemo(
    () => !connected() && initialDataReceived() && !reconnecting(),
  );
  const isLoadingPools = createMemo(
    () =>
      storageRecoveryResources.loading() && view() === 'pools' && filteredRecords().length === 0,
  );
  const hasFetchError = createMemo(() => Boolean(storageRecoveryResources.error()));

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
        write: (value) => (value !== 'none' ? value : null),
      },
      sort: {
        get: sortKey,
        set: setSortKey,
        read: (parsed) => normalizeSortKey(parsed.sort),
        write: (value) => (value !== 'priority' ? value : null),
      },
      order: {
        get: sortDirection,
        set: setSortDirection,
        read: (parsed) => normalizeSortDirection(parsed.order),
        write: (value) => (value !== 'desc' ? value : null),
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

  createEffect(() => {
    if (selectedNodeId() === 'all') return;
    if (activeNodeOptions().some((node) => node.id === selectedNodeId())) return;
    setSelectedNodeId('all');
  });

  onCleanup(() => {
    if (highlightTimer) window.clearTimeout(highlightTimer);
  });

  return (
    <div class="space-y-4">
      <StorageSummary
        poolCount={filteredRecords().length}
        diskCount={(() => {
          const nodeId = selectedNodeId();
          if (nodeId === 'all') return physicalDisks().length;
          const node = nodeOptions().find((n) => n.id === nodeId);
          if (!node) return physicalDisks().length;
          return physicalDisks().filter((d) =>
            matchesPhysicalDiskNode(d, { id: node.id, name: node.label, instance: node.instance }),
          ).length;
        })()}
        timeRange={summaryTimeRange()}
        onTimeRangeChange={setSummaryTimeRange}
        nodeId={selectedNodeId()}
      />

      <Show
        when={
          view() === 'pools' &&
          cephSummaryStats().clusters.length > 0 &&
          filteredRecords().some(isRecordCeph)
        }
      >
        <Card padding="md" tone="card">
          <div class="flex flex-wrap items-center justify-between gap-3">
            <div class="space-y-0.5">
              <div class="text-xs font-semibold uppercase tracking-wide text-muted">
                Ceph Summary
              </div>
              <div class="text-sm text-muted">
                {cephSummaryStats().clusters.length} cluster
                {cephSummaryStats().clusters.length !== 1 ? 's' : ''} detected
              </div>
            </div>
            <div class="text-right">
              <div class="text-sm font-semibold text-base-content">
                {formatBytes(cephSummaryStats().totalBytes)}
              </div>
              <div class="text-[11px] text-muted">
                {formatPercent(cephSummaryStats().usagePercent)} used
              </div>
            </div>
          </div>
          <div class="mt-3 grid gap-3 sm:grid-cols-2">
            <For each={cephSummaryStats().clusters}>
              {(cluster) => (
                <div class="rounded-md border border-border bg-surface p-3">
                  <div class="flex items-start justify-between gap-2">
                    <div class="min-w-0">
                      <div class="text-sm font-semibold text-base-content truncate">
                        {cluster.name || 'Ceph Cluster'}
                      </div>
                      <Show when={cluster.healthMessage}>
                        <div
                          class="text-[11px] text-muted truncate max-w-[240px]"
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

      <Show when={!kioskMode()}>
        <Subtabs
          value={view()}
          onChange={(value) => setView(value as StorageView)}
          ariaLabel="Storage view"
          tabs={[
            { value: 'pools', label: 'Pools' },
            { value: 'disks', label: 'Physical Disks' },
          ]}
        />

        <StorageFilter
          search={search}
          setSearch={setSearch}
          groupBy={view() === 'pools' ? storageFilterGroupBy : undefined}
          setGroupBy={view() === 'pools' ? (value) => setGroupBy(value) : undefined}
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
              <select
                value={selectedNodeId()}
                onChange={(event) => setSelectedNodeId(event.currentTarget.value)}
                class="px-2 py-1 text-xs border border-border rounded-md bg-surface text-base-content focus:ring-2 focus:ring-blue-500 focus:border-blue-500"
                aria-label="Node"
              >
                <option value="all">{view() === 'disks' ? 'All Disk Hosts' : 'All Nodes'}</option>
                <For each={activeNodeOptions()}>
                  {(node) => <option value={node.id}>{node.label}</option>}
                </For>
              </select>
              <div class="h-5 w-px bg-surface-hover hidden sm:block"></div>
            </>
          }
        />
      </Show>

      <Show when={reconnecting()}>
        <Card padding="sm" tone="warning">
          <div class="flex items-center justify-between gap-3">
            <span class="text-xs text-amber-800 dark:text-amber-200">
              Reconnecting to backend data stream…
            </span>
            <button
              type="button"
              onClick={() => reconnect()}
              class="rounded border border-amber-300 bg-amber-100 px-2 py-1 text-xs font-medium text-amber-800 hover:bg-amber-200 dark:border-amber-700 dark:bg-amber-900 dark:text-amber-200 dark:hover:bg-amber-900"
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
              class="rounded border border-amber-300 bg-amber-100 px-2 py-1 text-xs font-medium text-amber-800 hover:bg-amber-200 dark:border-amber-700 dark:bg-amber-900 dark:text-amber-200 dark:hover:bg-amber-900"
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

      <Card padding="none" tone="card" class="overflow-hidden">
        <div class="border-b border-border bg-surface-hover px-3 py-2 text-[11px] font-semibold uppercase tracking-wide text-muted">
          {view() === 'pools' ? 'Storage Pools' : 'Physical Disks'}
        </div>
        <Show when={view() === 'disks'}>
          <div class="p-2">
            <DiskList
              disks={physicalDisks()}
              nodes={nodes()}
              selectedNode={selectedNodeId() === 'all' ? null : selectedNodeId()}
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
                  <div class="p-6 text-sm text-muted">
                    No storage records match the current filters.
                  </div>
                }
              >
                <div class="overflow-x-auto">
                  <Table class="w-full text-xs">
                    <TableHeader>
                      <TableRow class="bg-surface-alt text-muted border-b border-border">
                        <TableHead class="px-1.5 sm:px-2 py-0.5 text-left text-[11px] sm:text-xs font-medium uppercase tracking-wider">
                          Storage
                        </TableHead>
                        <TableHead class="px-1.5 sm:px-2 py-0.5 text-left text-[11px] sm:text-xs font-medium uppercase tracking-wider">
                          Source
                        </TableHead>
                        <TableHead class="hidden xl:table-cell px-1.5 sm:px-2 py-0.5 text-left text-[11px] sm:text-xs font-medium uppercase tracking-wider">
                          Type
                        </TableHead>
                        <TableHead class="px-1.5 sm:px-2 py-0.5 text-left text-[11px] sm:text-xs font-medium uppercase tracking-wider">
                          Host
                        </TableHead>
                        <TableHead class="px-1.5 sm:px-2 py-0.5 text-left text-[11px] sm:text-xs font-medium uppercase tracking-wider md:min-w-[180px]">
                          Protection
                        </TableHead>
                        <TableHead class="px-1.5 sm:px-2 py-0.5 text-left text-[11px] sm:text-xs font-medium uppercase tracking-wider md:min-w-[190px] xl:min-w-[220px]">
                          Usage
                        </TableHead>
                        <TableHead class="px-1.5 sm:px-2 py-0.5 text-left text-[11px] sm:text-xs font-medium uppercase tracking-wider hidden lg:table-cell">
                          Impact
                        </TableHead>
                        <TableHead class="px-1.5 sm:px-2 py-0.5 text-left text-[11px] sm:text-xs font-medium uppercase tracking-wider">
                          Primary Issue
                        </TableHead>
                        <TableHead class="px-1.5 sm:px-2 py-0.5 w-10" />
                      </TableRow>
                    </TableHeader>
                    <TableBody class="divide-y divide-border">
                      {/* Outer <For> uses string keys — strings compare by value so DOM is stable across data updates */}
                      <For each={groupedRecords().map((g) => g.key)}>
                        {(groupKey) => {
                          const group = createMemo(
                            () => groupedRecords().find((g) => g.key === groupKey)!,
                          );
                          const groupItems = createMemo(() => group().items);
                          return (
                            <>
                              <Show when={groupBy() !== 'none'}>
                                <StorageGroupRow
                                  group={group()}
                                  groupBy={groupBy()}
                                  expanded={expandedGroups().has(groupKey)}
                                  onToggle={() => toggleGroup(groupKey)}
                                />
                              </Show>
                              <Show when={expandedGroups().has(groupKey)}>
                                {/* Inner <Index> tracks by position — updates props reactively instead of recreating DOM */}
                                <Index each={groupItems()}>
                                  {(record) => {
                                    const isExpanded = () => expandedPoolId() === record().id;
                                    const alertState = createMemo(() =>
                                      getRecordAlertState(record().id),
                                    );
                                    const nodeLabel = createMemo(() =>
                                      getRecordNodeLabel(record()).trim().toLowerCase(),
                                    );
                                    const parentNodeOnline = createMemo(() => {
                                      const label = nodeLabel();
                                      if (!label) return true;
                                      const nodeStatus = nodeOnlineByLabel().get(label);
                                      return nodeStatus === undefined ? true : nodeStatus;
                                    });
                                    const showAlertHighlight = createMemo(
                                      () =>
                                        alertState().hasUnacknowledgedAlert && parentNodeOnline(),
                                    );
                                    const hasAcknowledgedOnlyAlert = createMemo(
                                      () =>
                                        alertState().hasAcknowledgedOnlyAlert && parentNodeOnline(),
                                    );
                                    const isResourceHighlighted = () =>
                                      highlightedRecordId() === record().id;
                                    const rowClass = createMemo(() => {
                                      const classes = [
                                        'transition-all duration-200',
                                        'hover:bg-surface-hover',
                                      ];

                                      if (showAlertHighlight()) {
                                        classes.push(
                                          alertState().severity === 'critical'
                                            ? 'bg-red-50 dark:bg-red-950'
                                            : 'bg-yellow-50 dark:bg-yellow-950',
                                        );
                                      } else if (isResourceHighlighted()) {
                                        classes.push(
                                          'bg-blue-50 dark:bg-blue-900 ring-1 ring-blue-300 dark:ring-blue-600',
                                        );
                                      } else if (hasAcknowledgedOnlyAlert()) {
                                        classes.push('bg-surface-alt');
                                      }

                                      if (isExpanded()) {
                                        classes.push('bg-surface-alt');
                                      }

                                      return classes.join(' ');
                                    });

                                    const rowStyle = createMemo(() => {
                                      if (showAlertHighlight()) {
                                        return {
                                          'box-shadow': `inset 4px 0 0 0 ${
                                            alertState().severity === 'critical'
                                              ? '#ef4444'
                                              : '#eab308'
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
                                        record={record()}
                                        expanded={isExpanded()}
                                        groupExpanded={expandedGroups().has(groupKey)}
                                        onToggleExpand={() =>
                                          setExpandedPoolId((current) =>
                                            current === record().id ? null : record().id,
                                          )
                                        }
                                        rowClass={rowClass()}
                                        rowStyle={rowStyle()}
                                        physicalDisks={physicalDisks()}
                                        alertDataAttrs={{
                                          'data-row-id': record().id,
                                          'data-alert-state': showAlertHighlight()
                                            ? 'unacknowledged'
                                            : hasAcknowledgedOnlyAlert()
                                              ? 'acknowledged'
                                              : 'none',
                                          'data-alert-severity': alertState().severity || 'none',
                                          'data-resource-highlighted': isResourceHighlighted()
                                            ? 'true'
                                            : 'false',
                                        }}
                                      />
                                    );
                                  }}
                                </Index>
                              </Show>
                            </>
                          );
                        }}
                      </For>
                    </TableBody>
                  </Table>
                </div>
              </Show>
            }
          >
            <div class="p-6 text-sm text-muted">Loading storage resources...</div>
          </Show>
        </Show>
      </Card>
    </div>
  );
};

export default Storage;
