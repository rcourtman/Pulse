import { Accessor, createMemo } from 'solid-js';
import type { NormalizedHealth, StorageRecord } from '@/features/storageBackups/models';
import type { ZFSPool } from '@/types/api';
import { getSourcePlatformLabel } from '@/utils/sourcePlatforms';
import { normalizeStorageSourceKey } from '@/utils/storageSources';

export type StorageSortKey = 'priority' | 'name' | 'usage' | 'type';
export type StorageGroupKey = 'node' | 'type' | 'status' | 'none';

export type StorageNodeOption = {
  id: string;
  label: string;
  instance?: string;
  aliases?: string[];
};

export interface StorageGroupStats {
  totalBytes: number;
  usedBytes: number;
  usagePercent: number;
  byHealth: Record<NormalizedHealth, number>;
}

export type StorageGroupedRecords = {
  key: string;
  items: StorageRecord[];
  stats: StorageGroupStats;
};

export type StorageSummary = {
  count: number;
  totalBytes: number;
  usedBytes: number;
  usagePercent: number;
  byHealth: Record<NormalizedHealth, number>;
};

type UseStorageModelOptions = {
  records: Accessor<StorageRecord[]>;
  search: Accessor<string>;
  sourceFilter: Accessor<string>;
  healthFilter: Accessor<'all' | NormalizedHealth>;
  selectedNodeId: Accessor<string>;
  nodeOptions: Accessor<StorageNodeOption[]>;
  sortKey: Accessor<StorageSortKey>;
  sortDirection: Accessor<'asc' | 'desc'>;
  groupBy: Accessor<StorageGroupKey>;
};

const getRecordDetails = (record: StorageRecord): Record<string, unknown> =>
  (record.details || {}) as Record<string, unknown>;

const getRecordStringDetail = (record: StorageRecord, key: string): string => {
  const value = getRecordDetails(record)[key];
  return typeof value === 'string' ? value : '';
};

const getRecordStringArrayDetail = (record: StorageRecord, key: string): string[] => {
  const value = getRecordDetails(record)[key];
  return Array.isArray(value)
    ? value.filter((item): item is string => typeof item === 'string' && item.trim().length > 0)
    : [];
};

export const getRecordNodeHints = (record: StorageRecord): string[] => {
  const details = getRecordDetails(record);
  const detailNode = typeof details.node === 'string' ? details.node : '';
  const detailParent = typeof details.parentId === 'string' ? details.parentId : '';
  const detailParentName = typeof details.parentName === 'string' ? details.parentName : '';
  const locationRoot = record.location.label.split('/')[0]?.trim() || '';
  return [
    detailNode,
    detailParent,
    detailParentName,
    ...getRecordStringArrayDetail(record, 'nodeHints'),
    locationRoot,
    record.location.label,
    record.refs?.platformEntityId,
  ]
    .map((value) => value.toLowerCase().trim())
    .filter((value) => value.length > 0);
};

export const getRecordType = (record: StorageRecord): string =>
  getRecordStringDetail(record, 'type') || record.category || 'other';

export const getRecordContent = (record: StorageRecord): string =>
  getRecordStringDetail(record, 'content');

export const getRecordStatus = (record: StorageRecord): string => {
  const status = getRecordStringDetail(record, 'status');
  if (status) return status;
  if (record.health === 'healthy') return 'available';
  if (record.health === 'warning') return 'degraded';
  if (record.health === 'offline') return 'offline';
  if (record.health === 'critical') return 'critical';
  return 'unknown';
};

export const getRecordPlatformLabel = (record: StorageRecord): string =>
  record.platformLabel?.trim() || getSourcePlatformLabel(record.source.platform);

export const getRecordHostLabel = (record: StorageRecord): string =>
  record.hostLabel?.trim() || getRecordNodeLabel(record);

export const getRecordTopologyLabel = (record: StorageRecord): string =>
  record.topologyLabel?.trim() || getRecordType(record);

export const getRecordProtectionLabel = (record: StorageRecord): string =>
  record.protectionLabel?.trim() || 'Healthy';

export const getRecordIssueLabel = (record: StorageRecord): string =>
  record.issueLabel?.trim() || 'Healthy';

export const getRecordIssueSummary = (record: StorageRecord): string =>
  record.issueSummary?.trim() || record.issueLabel?.trim() || '';

export const getRecordImpactSummary = (record: StorageRecord): string =>
  record.impactSummary?.trim() || 'No dependent resources';

export const getRecordActionSummary = (record: StorageRecord): string =>
  record.actionSummary?.trim() || 'Monitor';

export const getRecordShared = (record: StorageRecord): boolean | null => {
  const shared = getRecordDetails(record).shared;
  return typeof shared === 'boolean' ? shared : null;
};

export const getRecordNodeLabel = (record: StorageRecord): string => {
  const parentName = getRecordStringDetail(record, 'parentName');
  if (parentName.trim()) return parentName;
  const node = getRecordStringDetail(record, 'node');
  if (node.trim()) return node;
  return record.location.label || 'unassigned';
};

export const getRecordUsagePercent = (record: StorageRecord): number => {
  if (
    typeof record.capacity.usagePercent === 'number' &&
    Number.isFinite(record.capacity.usagePercent)
  ) {
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

export const getRecordZfsPool = (record: StorageRecord): ZFSPool | null =>
  toZfsPool(getRecordDetails(record).zfsPool);

const computeGroupStats = (items: StorageRecord[]): StorageGroupStats => {
  const seenShared = new Set<string>();
  const totals = items.reduce(
    (acc, record) => {
      acc.byHealth[record.health] = (acc.byHealth[record.health] || 0) + 1;

      const isShared = getRecordShared(record) === true;
      if (isShared) {
        const sharedKey = `${record.source.platform}|${record.name}`;
        if (seenShared.has(sharedKey)) {
          return acc;
        }
        seenShared.add(sharedKey);
      }

      acc.total += record.capacity.totalBytes || 0;
      acc.used += record.capacity.usedBytes || 0;
      return acc;
    },
    {
      total: 0,
      used: 0,
      byHealth: { healthy: 0, warning: 0, critical: 0, offline: 0, unknown: 0 } as Record<
        NormalizedHealth,
        number
      >,
    },
  );
  return {
    totalBytes: totals.total,
    usedBytes: totals.used,
    usagePercent: totals.total > 0 ? (totals.used / totals.total) * 100 : 0,
    byHealth: totals.byHealth,
  };
};

export const useStorageModel = (options: UseStorageModelOptions) => {
  const selectedNode = createMemo(() => {
    if (options.selectedNodeId() === 'all') return null;
    return options.nodeOptions().find((node) => node.id === options.selectedNodeId()) || null;
  });

  const matchesSelectedNode = (record: StorageRecord): boolean => {
    const node = selectedNode();
    if (!node) return true;
    const aliases = Array.from(
      new Set(
        [node.id, node.label, ...(node.aliases || [])]
          .map((value) => (value || '').toLowerCase().trim())
          .filter(Boolean),
      ),
    );
    const hints = getRecordNodeHints(record);
    return hints.some((hint) => aliases.some((alias) => hint.includes(alias)));
  };

  const sourceOptions = createMemo(() => {
    const values = Array.from(
      new Set(options.records().map((record) => normalizeStorageSourceKey(record.source.platform))),
    ).sort((a, b) => getSourcePlatformLabel(a).localeCompare(getSourcePlatformLabel(b)));
    return ['all', ...values];
  });

  const filteredRecords = createMemo(() => {
    const query = options.search().trim().toLowerCase();
    const selectedSource = normalizeStorageSourceKey(options.sourceFilter());
    return options
      .records()
      .filter((record) =>
        selectedSource === 'all'
          ? true
          : normalizeStorageSourceKey(record.source.platform) === selectedSource,
      )
      .filter((record) =>
        options.healthFilter() === 'all' ? true : record.health === options.healthFilter(),
      )
      .filter((record) => matchesSelectedNode(record))
      .filter((record) => {
        if (!query) return true;
        const haystack = [
          record.name,
          record.category,
          record.location.label,
          record.source.platform,
          getRecordPlatformLabel(record),
          getRecordHostLabel(record),
          getRecordTopologyLabel(record),
          getRecordProtectionLabel(record),
          getRecordIssueLabel(record),
          getRecordIssueSummary(record),
          getRecordImpactSummary(record),
          getRecordActionSummary(record),
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

  const sortedRecords = createMemo(() => {
    const numericCompare = (a: number, b: number): number => {
      if (a === b) return 0;
      return a < b ? -1 : 1;
    };

    return [...filteredRecords()].sort((a, b) => {
      let comparison = 0;

      if (options.sortKey() === 'usage') {
        comparison = numericCompare(getRecordUsagePercent(a), getRecordUsagePercent(b));
      } else if (options.sortKey() === 'priority') {
        comparison = numericCompare(a.incidentPriority || 0, b.incidentPriority || 0);
      } else if (options.sortKey() === 'type') {
        comparison = getRecordType(a).localeCompare(getRecordType(b), undefined, {
          sensitivity: 'base',
        });
      } else {
        comparison = a.name.localeCompare(b.name, undefined, { sensitivity: 'base' });
      }

      if (comparison === 0) {
        comparison = a.name.localeCompare(b.name, undefined, { sensitivity: 'base' });
      }

      return options.sortDirection() === 'asc' ? comparison : -comparison;
    });
  });

  const groupedRecords = createMemo<StorageGroupedRecords[]>(() => {
    if (options.groupBy() === 'none') {
      const items = sortedRecords();
      return [{ key: 'All', items, stats: computeGroupStats(items) }];
    }

    const groups = new Map<string, StorageRecord[]>();

    for (const record of sortedRecords()) {
      const key =
        options.groupBy() === 'type'
          ? getRecordType(record)
          : options.groupBy() === 'status'
            ? getRecordStatus(record)
            : getRecordNodeLabel(record);
      const normalized = key.trim() || 'unknown';
      if (!groups.has(normalized)) groups.set(normalized, []);
      groups.get(normalized)!.push(record);
    }

    return Array.from(groups.entries())
      .sort(([keyA], [keyB]) => keyA.localeCompare(keyB, undefined, { sensitivity: 'base' }))
      .map(([key, items]) => ({ key, items, stats: computeGroupStats(items) }));
  });

  const summary = createMemo<StorageSummary>(() => {
    const list = filteredRecords();
    const seenShared = new Set<string>();
    const totals = list.reduce(
      (acc, record) => {
        acc.byHealth[record.health] = (acc.byHealth[record.health] || 0) + 1;

        const isShared = getRecordShared(record) === true;
        if (isShared) {
          const sharedKey = `${record.source.platform}|${record.name}`;
          if (seenShared.has(sharedKey)) {
            return acc;
          }
          seenShared.add(sharedKey);
        }

        const total = record.capacity.totalBytes || 0;
        const used = record.capacity.usedBytes || 0;
        acc.total += total;
        acc.used += used;
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

  return {
    sourceOptions,
    selectedNode,
    filteredRecords,
    sortedRecords,
    groupedRecords,
    summary,
  };
};
