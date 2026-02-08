import { useLocation, useNavigate } from '@solidjs/router';
import { Component, For, Show, createEffect, createMemo, createSignal, untrack } from 'solid-js';
import { useWebSocket } from '@/App';
import { Card } from '@/components/shared/Card';
import { StatusDot } from '@/components/shared/StatusDot';
import { DiskList } from '@/components/Storage/DiskList';
import { EnhancedStorageBar } from '@/components/Storage/EnhancedStorageBar';
import { ZFSHealthMap } from '@/components/Storage/ZFSHealthMap';
import { buildStorageRecordsV2 } from '@/features/storageBackupsV2/storageAdapters';
import { PLATFORM_BLUEPRINTS } from '@/features/storageBackupsV2/platformBlueprint';
import type { NormalizedHealth, StorageRecordV2 } from '@/features/storageBackupsV2/models';
import { useStorageBackupsResources } from '@/hooks/useUnifiedResources';
import {
  STORAGE_QUERY_PARAMS,
  STORAGE_V2_PATH,
  buildStorageV2Path,
  parseStorageLinkSearch,
} from '@/routing/resourceLinks';
import type { CephCluster, ZFSPool } from '@/types/api';
import { formatBytes, formatPercent } from '@/utils/format';

const HEALTH_CLASS: Record<NormalizedHealth, string> = {
  healthy: 'text-green-700 dark:text-green-300',
  warning: 'text-yellow-700 dark:text-yellow-300',
  critical: 'text-red-700 dark:text-red-300',
  offline: 'text-gray-600 dark:text-gray-300',
  unknown: 'text-gray-500 dark:text-gray-400',
};

type StorageV2View = 'pools' | 'disks';
type StorageSortKey = 'name' | 'usage' | 'type';
type StorageGroupKey = 'node' | 'type' | 'status';

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

const getRecordDetails = (record: StorageRecordV2): Record<string, unknown> =>
  (record.details || {}) as Record<string, unknown>;

const getRecordStringDetail = (record: StorageRecordV2, key: string): string => {
  const value = getRecordDetails(record)[key];
  return typeof value === 'string' ? value : '';
};

const sourceLabel = (value: string): string =>
  value
    .split('-')
    .filter(Boolean)
    .map((part) => part.charAt(0).toUpperCase() + part.slice(1))
    .join(' ');

const getRecordNodeHints = (record: StorageRecordV2): string[] => {
  const details = getRecordDetails(record);
  const detailNode = typeof details.node === 'string' ? details.node : '';
  const detailParent = typeof details.parentId === 'string' ? details.parentId : '';
  const locationRoot = record.location.label.split('/')[0]?.trim() || '';
  return [detailNode, detailParent, locationRoot, record.location.label]
    .map((value) => value.toLowerCase().trim())
    .filter((value) => value.length > 0);
};

const getRecordType = (record: StorageRecordV2): string =>
  getRecordStringDetail(record, 'type') || record.category || 'other';

const getRecordContent = (record: StorageRecordV2): string => getRecordStringDetail(record, 'content');

const getRecordStatus = (record: StorageRecordV2): string => {
  const status = getRecordStringDetail(record, 'status');
  if (status) return status;
  if (record.health === 'healthy') return 'available';
  if (record.health === 'warning') return 'degraded';
  if (record.health === 'offline') return 'offline';
  if (record.health === 'critical') return 'critical';
  return 'unknown';
};

const getRecordShared = (record: StorageRecordV2): boolean | null => {
  const shared = getRecordDetails(record).shared;
  return typeof shared === 'boolean' ? shared : null;
};

const getRecordNodeLabel = (record: StorageRecordV2): string => {
  const node = getRecordStringDetail(record, 'node');
  if (node.trim()) return node;
  return record.location.label || 'unassigned';
};

const getRecordUsagePercent = (record: StorageRecordV2): number => {
  if (typeof record.capacity.usagePercent === 'number' && Number.isFinite(record.capacity.usagePercent)) {
    return record.capacity.usagePercent;
  }
  const total = record.capacity.totalBytes || 0;
  const used = record.capacity.usedBytes || 0;
  if (total <= 0) return 0;
  return (used / total) * 100;
};

const toZfsPool = (value: unknown): ZFSPool | null => {
  if (!value || typeof value !== 'object') return null;
  const candidate = value as Partial<ZFSPool>;
  if (typeof candidate.state !== 'string' || !Array.isArray(candidate.devices)) return null;
  return candidate as ZFSPool;
};

const getRecordZfsPool = (record: StorageRecordV2): ZFSPool | null =>
  toZfsPool(getRecordDetails(record).zfsPool);

const isCephType = (type?: string): boolean => {
  const value = (type || '').toLowerCase();
  return value === 'rbd' || value === 'cephfs' || value === 'ceph';
};

const isCephRecord = (record: StorageRecordV2): boolean => {
  if (isCephType(getRecordType(record))) return true;
  return record.capabilities.includes('replication') && record.source.platform.includes('proxmox');
};

const getCephClusterKeyFromRecord = (record: StorageRecordV2): string => {
  const details = getRecordDetails(record);
  const parent = typeof details.parentId === 'string' ? details.parentId : '';
  return record.refs?.platformEntityId || parent || record.location.label || record.source.platform;
};

const getCephHealthLabel = (health?: string): string => {
  if (!health) return 'CEPH';
  const normalized = health.toUpperCase();
  return normalized.startsWith('HEALTH_') ? normalized.replace('HEALTH_', '') : normalized;
};

const getCephHealthStyles = (health?: string): string => {
  const normalized = (health || '').toUpperCase();
  if (normalized === 'HEALTH_OK') {
    return 'bg-green-100 text-green-700 dark:bg-green-900/60 dark:text-green-300 border border-green-200 dark:border-green-800';
  }
  if (normalized === 'HEALTH_WARN' || normalized === 'HEALTH_WARNING') {
    return 'bg-yellow-100 text-yellow-700 dark:bg-yellow-900/60 dark:text-yellow-200 border border-yellow-300 dark:border-yellow-800';
  }
  if (normalized === 'HEALTH_ERR' || normalized === 'HEALTH_ERROR' || normalized === 'HEALTH_CRIT') {
    return 'bg-red-100 text-red-700 dark:bg-red-900/60 dark:text-red-200 border border-red-300 dark:border-red-800';
  }
  return 'bg-blue-100 text-blue-700 dark:bg-blue-900/50 dark:text-blue-200 border border-blue-200 dark:border-blue-700';
};

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

const StorageV2: Component = () => {
  const navigate = useNavigate();
  const location = useLocation();
  const { state, connected, initialDataReceived, reconnecting, reconnect } = useWebSocket();
  const storageBackupsResources = useStorageBackupsResources();

  const [search, setSearch] = createSignal('');
  const [sourceFilter, setSourceFilter] = createSignal('all');
  const [healthFilter, setHealthFilter] = createSignal<'all' | NormalizedHealth>('all');
  const [view, setView] = createSignal<StorageV2View>('pools');
  const [selectedNodeId, setSelectedNodeId] = createSignal('all');
  const [sortKey, setSortKey] = createSignal<StorageSortKey>('name');
  const [sortDirection, setSortDirection] = createSignal<'asc' | 'desc'>('asc');
  const [groupBy, setGroupBy] = createSignal<StorageGroupKey>('node');
  const [expandedCephRecordId, setExpandedCephRecordId] = createSignal<string | null>(null);

  const adapterResources = createMemo(() => {
    const unifiedResources = storageBackupsResources.resources();
    return unifiedResources.length > 0 ? unifiedResources : (state.resources || []);
  });

  const records = createMemo<StorageRecordV2[]>(() =>
    buildStorageRecordsV2({ state, resources: adapterResources() }),
  );

  const sourceOptions = createMemo(() => {
    const values = Array.from(new Set(records().map((record) => record.source.platform))).sort((a, b) =>
      sourceLabel(a).localeCompare(sourceLabel(b)),
    );
    return ['all', ...values];
  });

  const nodeOptions = createMemo(() => {
    const nodes = state.nodes || [];
    return nodes.map((node) => ({ id: node.id, label: node.name, instance: node.instance }));
  });

  const selectedNode = createMemo(() => {
    if (selectedNodeId() === 'all') return null;
    return nodeOptions().find((node) => node.id === selectedNodeId()) || null;
  });

  const matchesSelectedNode = (record: StorageRecordV2): boolean => {
    const node = selectedNode();
    if (!node) return true;
    const nodeName = node.label.toLowerCase().trim();
    const nodeInstance = (node.instance || '').toLowerCase().trim();
    const hints = getRecordNodeHints(record);
    return hints.some((hint) => hint.includes(nodeName) || (nodeInstance && hint.includes(nodeInstance)));
  };

  const filtered = createMemo(() => {
    const query = search().trim().toLowerCase();
    return records()
      .filter((record) => (sourceFilter() === 'all' ? true : record.source.platform === sourceFilter()))
      .filter((record) => (healthFilter() === 'all' ? true : record.health === healthFilter()))
      .filter((record) => matchesSelectedNode(record))
      .filter((record) => {
        if (!query) return true;
        const haystack = [
          record.name,
          record.category,
          record.location.label,
          record.source.platform,
          getRecordType(record),
          getRecordContent(record),
          getRecordStatus(record),
          ...(record.capabilities || []),
        ]
          .filter(Boolean)
          .join(' ')
          .toLowerCase();
        return haystack.includes(query);
      });
  });

  const sorted = createMemo(() => {
    const numericCompare = (a: number, b: number): number => {
      if (a === b) return 0;
      return a < b ? -1 : 1;
    };

    return [...filtered()].sort((a, b) => {
      let comparison = 0;

      if (sortKey() === 'usage') {
        comparison = numericCompare(getRecordUsagePercent(a), getRecordUsagePercent(b));
      } else if (sortKey() === 'type') {
        comparison = getRecordType(a).localeCompare(getRecordType(b), undefined, {
          sensitivity: 'base',
        });
      } else {
        comparison = a.name.localeCompare(b.name, undefined, { sensitivity: 'base' });
      }

      if (comparison === 0) {
        comparison = a.name.localeCompare(b.name, undefined, { sensitivity: 'base' });
      }

      return sortDirection() === 'asc' ? comparison : -comparison;
    });
  });

  const grouped = createMemo(() => {
    const groups = new Map<string, StorageRecordV2[]>();

    for (const record of sorted()) {
      const key =
        groupBy() === 'type'
          ? getRecordType(record)
          : groupBy() === 'status'
            ? getRecordStatus(record)
            : getRecordNodeLabel(record);
      const normalized = key.trim() || 'unknown';
      if (!groups.has(normalized)) groups.set(normalized, []);
      groups.get(normalized)!.push(record);
    }

    return Array.from(groups.entries())
      .sort(([keyA], [keyB]) => keyA.localeCompare(keyB, undefined, { sensitivity: 'base' }))
      .map(([key, items]) => ({ key, items }));
  });

  const summary = createMemo(() => {
    const list = filtered();
    const totals = list.reduce(
      (acc, record) => {
        const total = record.capacity.totalBytes || 0;
        const used = record.capacity.usedBytes || 0;
        acc.total += total;
        acc.used += used;
        acc.byHealth[record.health] = (acc.byHealth[record.health] || 0) + 1;
        return acc;
      },
      {
        total: 0,
        used: 0,
        byHealth: {
          healthy: 0,
          warning: 0,
          critical: 0,
          offline: 0,
          unknown: 0,
        } as Record<NormalizedHealth, number>,
      },
    );
    const usagePercent = totals.total > 0 ? (totals.used / totals.total) * 100 : 0;
    return {
      count: list.length,
      totalBytes: totals.total,
      usedBytes: totals.used,
      usagePercent,
      byHealth: totals.byHealth,
    };
  });

  const visibleCephClusters = createMemo<CephCluster[]>(() => {
    const explicit = state.cephClusters || [];
    if (explicit.length > 0) return explicit;

    const summaryByKey = new Map<
      string,
      { total: number; used: number; available: number; records: number; nodes: Set<string> }
    >();

    records().forEach((record) => {
      if (!isCephRecord(record)) return;
      const key = getCephClusterKeyFromRecord(record);
      const current =
        summaryByKey.get(key) ||
        ({ total: 0, used: 0, available: 0, records: 0, nodes: new Set<string>() } as const);
      const totalBytes = Math.max(0, record.capacity.totalBytes || 0);
      const usedBytes = Math.max(0, record.capacity.usedBytes || 0);
      const freeBytes = Math.max(0, record.capacity.freeBytes ?? totalBytes - usedBytes);

      summaryByKey.set(key, {
        total: current.total + totalBytes,
        used: current.used + usedBytes,
        available: current.available + freeBytes,
        records: current.records + 1,
        nodes: new Set([...current.nodes, getRecordNodeLabel(record)]),
      });
    });

    return Array.from(summaryByKey.entries()).map(([instance, item], index) => {
      const usagePercent = item.total > 0 ? (item.used / item.total) * 100 : 0;
      const numOsds = Math.max(1, item.records * 2);
      const numMons = Math.min(3, Math.max(1, item.nodes.size));
      return {
        id: `derived-ceph-${index}`,
        instance,
        name: `${instance} Ceph`,
        health: 'HEALTH_UNKNOWN',
        healthMessage: 'Derived from storage metrics - live Ceph telemetry unavailable.',
        totalBytes: item.total,
        usedBytes: item.used,
        availableBytes: item.available,
        usagePercent,
        numMons,
        numMgrs: numMons > 1 ? 2 : 1,
        numOsds,
        numOsdsUp: numOsds,
        numOsdsIn: numOsds,
        numPGs: Math.max(128, item.records * 64),
        pools: undefined,
        services: undefined,
        lastUpdated: Date.now(),
      } as CephCluster;
    });
  });

  const cephClusterByKey = createMemo<Record<string, CephCluster>>(() => {
    const map: Record<string, CephCluster> = {};
    visibleCephClusters().forEach((cluster) => {
      [cluster.instance, cluster.id, cluster.name].forEach((key) => {
        if (key) map[key] = cluster;
      });
    });
    return map;
  });

  const cephSummaryStats = createMemo(() => {
    const clusters = visibleCephClusters();
    const totals = clusters.reduce(
      (acc, cluster) => {
        acc.total += Math.max(0, cluster.totalBytes || 0);
        acc.used += Math.max(0, cluster.usedBytes || 0);
        acc.available += Math.max(0, cluster.availableBytes || 0);
        return acc;
      },
      { total: 0, used: 0, available: 0 },
    );
    const usagePercent = totals.total > 0 ? (totals.used / totals.total) * 100 : 0;
    return {
      clusters,
      totalBytes: totals.total,
      usedBytes: totals.used,
      availableBytes: totals.available,
      usagePercent,
    };
  });

  const resolveCephCluster = (record: StorageRecordV2): CephCluster | null => {
    const key = getCephClusterKeyFromRecord(record);
    return cephClusterByKey()[key] || null;
  };

  const getCephSummaryText = (record: StorageRecordV2, cluster: CephCluster | null): string => {
    if (cluster && Number.isFinite(cluster.totalBytes)) {
      const total = Math.max(0, cluster.totalBytes || 0);
      const used = Math.max(0, cluster.usedBytes || 0);
      const percent = total > 0 ? (used / total) * 100 : 0;
      const parts = [`${formatBytes(used)} / ${formatBytes(total)} (${formatPercent(percent)})`];
      if (Number.isFinite(cluster.numOsds) && Number.isFinite(cluster.numOsdsUp)) {
        parts.push(`OSDs ${cluster.numOsdsUp}/${cluster.numOsds}`);
      }
      if (Number.isFinite(cluster.numPGs) && cluster.numPGs > 0) {
        parts.push(`PGs ${cluster.numPGs.toLocaleString()}`);
      }
      return parts.join(' • ');
    }

    const total = Math.max(0, record.capacity.totalBytes || 0);
    const used = Math.max(0, record.capacity.usedBytes || 0);
    if (total <= 0) return '';
    const percent = (used / total) * 100;
    return `${formatBytes(used)} / ${formatBytes(total)} (${formatPercent(percent)})`;
  };

  const getCephPoolsText = (record: StorageRecordV2, cluster: CephCluster | null): string => {
    if (cluster?.pools?.length) {
      return cluster.pools
        .slice(0, 2)
        .map((pool) => {
          const total = Math.max(1, pool.storedBytes + pool.availableBytes);
          const percent = (pool.storedBytes / total) * 100;
          return `${pool.name}: ${formatPercent(percent)}`;
        })
        .join(', ');
    }

    const percent = getRecordUsagePercent(record);
    return `${record.name}: ${formatPercent(percent)}`;
  };

  const nextPlatforms = createMemo(() =>
    PLATFORM_BLUEPRINTS.filter((platform) => platform.stage === 'next').map((platform) => platform.label),
  );

  const isWaitingForData = createMemo(
    () =>
      storageBackupsResources.loading() &&
      filtered().length === 0 &&
      !connected() &&
      !initialDataReceived(),
  );
  const isDisconnectedAfterLoad = createMemo(() => !connected() && initialDataReceived() && !reconnecting());
  const isLoadingPools = createMemo(
    () => storageBackupsResources.loading() && view() === 'pools' && filtered().length === 0,
  );
  const hasV2FetchError = createMemo(() => Boolean(storageBackupsResources.error()));

  const isActiveStorageRoute = () =>
    location.pathname === STORAGE_V2_PATH || location.pathname === '/storage';

  createEffect(() => {
    if (!isActiveStorageRoute()) return;

    const parsed = parseStorageLinkSearch(location.search);

    const nextView = normalizeView(parsed.tab);
    if (nextView !== untrack(view)) setView(nextView);

    const nextSource = parsed.source || 'all';
    if (nextSource !== untrack(sourceFilter)) setSourceFilter(nextSource);

    const nextHealth = normalizeHealthFilter(parsed.status);
    if (nextHealth !== untrack(healthFilter)) setHealthFilter(nextHealth);

    const nextNode = parsed.node || 'all';
    if (nextNode !== untrack(selectedNodeId)) setSelectedNodeId(nextNode);

    const nextGroup = normalizeGroupKey(parsed.group);
    if (nextGroup !== untrack(groupBy)) setGroupBy(nextGroup);

    const nextSort = normalizeSortKey(parsed.sort);
    if (nextSort !== untrack(sortKey)) setSortKey(nextSort);

    const nextOrder = normalizeSortDirection(parsed.order);
    if (nextOrder !== untrack(sortDirection)) setSortDirection(nextOrder);

    if (parsed.query !== untrack(search)) setSearch(parsed.query);
  });

  createEffect(() => {
    if (!isActiveStorageRoute()) return;

    const managedPath = buildStorageV2Path({
      tab: view() !== 'pools' ? view() : null,
      group: groupBy() !== 'node' ? groupBy() : null,
      source: sourceFilter() !== 'all' ? sourceFilter() : null,
      status: healthFilter() !== 'all' ? healthFilter() : null,
      node: selectedNodeId() !== 'all' ? selectedNodeId() : null,
      query: search().trim() || null,
      sort: sortKey() !== 'name' ? sortKey() : null,
      order: sortDirection() !== 'asc' ? sortDirection() : null,
    });

    const [, managedSearch = ''] = managedPath.split('?');
    const managedParams = new URLSearchParams(managedSearch);
    const params = new URLSearchParams(location.search);

    Object.values(STORAGE_QUERY_PARAMS).forEach((key) => params.delete(key));
    managedParams.forEach((value, key) => params.set(key, value));

    const basePath = location.pathname;
    const nextSearch = params.toString();
    const nextPath = nextSearch ? `${basePath}?${nextSearch}` : basePath;
    const currentPath = `${location.pathname}${location.search || ''}`;

    if (nextPath !== currentPath) navigate(nextPath, { replace: true });
  });

  createEffect(() => {
    if (view() !== 'pools') {
      setExpandedCephRecordId(null);
    }
  });

  return (
    <div class="space-y-4">
      <Card padding="md" tone="glass">
        <div class="flex flex-wrap items-center justify-between gap-3">
          <div>
            <h2 class="text-sm font-semibold text-gray-900 dark:text-gray-100">Storage V2 Preview</h2>
            <p class="text-xs text-gray-600 dark:text-gray-400">
              Source-agnostic storage view model with capability-first normalization.
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

      <Show when={view() === 'pools' && cephSummaryStats().clusters.length > 0 && filtered().some(isCephRecord)}>
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
            Unable to refresh v2 storage resources. Showing latest available data.
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
                when={grouped().length > 0}
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
                      <For each={grouped()}>
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
                                const isCeph = isCephRecord(record);
                                const isExpanded = () => expandedCephRecordId() === record.id;
                                const status = getRecordStatus(record);
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

                                return (
                                  <>
                                    <tr class="hover:bg-gray-50 dark:hover:bg-gray-800/30">
                                      <td class="px-3 py-2 text-gray-900 dark:text-gray-100">
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
