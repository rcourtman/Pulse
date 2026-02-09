import { Accessor, createMemo } from 'solid-js';
import type { NormalizedHealth, StorageRecord } from '@/features/storageBackups/models';
import type { ZFSPool } from '@/types/api';

export type StorageSortKey = 'name' | 'usage' | 'type';
export type StorageGroupKey = 'node' | 'type' | 'status' | 'none';

export type StorageNodeOption = {
  id: string;
  label: string;
  instance?: string;
};

export type StorageGroupedRecords = {
  key: string;
  items: StorageRecord[];
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

export const sourceLabel = (value: string): string =>
  value
    .split('-')
    .filter(Boolean)
    .map((part) => part.charAt(0).toUpperCase() + part.slice(1))
    .join(' ');

export const getRecordNodeHints = (record: StorageRecord): string[] => {
  const details = getRecordDetails(record);
  const detailNode = typeof details.node === 'string' ? details.node : '';
  const detailParent = typeof details.parentId === 'string' ? details.parentId : '';
  const locationRoot = record.location.label.split('/')[0]?.trim() || '';
  return [detailNode, detailParent, locationRoot, record.location.label]
    .map((value) => value.toLowerCase().trim())
    .filter((value) => value.length > 0);
};

export const getRecordType = (record: StorageRecord): string =>
  getRecordStringDetail(record, 'type') || record.category || 'other';

export const getRecordContent = (record: StorageRecord): string => getRecordStringDetail(record, 'content');

export const getRecordStatus = (record: StorageRecord): string => {
  const status = getRecordStringDetail(record, 'status');
  if (status) return status;
  if (record.health === 'healthy') return 'available';
  if (record.health === 'warning') return 'degraded';
  if (record.health === 'offline') return 'offline';
  if (record.health === 'critical') return 'critical';
  return 'unknown';
};

export const getRecordShared = (record: StorageRecord): boolean | null => {
  const shared = getRecordDetails(record).shared;
  return typeof shared === 'boolean' ? shared : null;
};

export const getRecordNodeLabel = (record: StorageRecord): string => {
  const node = getRecordStringDetail(record, 'node');
  if (node.trim()) return node;
  return record.location.label || 'unassigned';
};

export const getRecordUsagePercent = (record: StorageRecord): number => {
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

export const getRecordZfsPool = (record: StorageRecord): ZFSPool | null =>
  toZfsPool(getRecordDetails(record).zfsPool);

export const useStorageModel = (options: UseStorageModelOptions) => {
  const selectedNode = createMemo(() => {
    if (options.selectedNodeId() === 'all') return null;
    return options.nodeOptions().find((node) => node.id === options.selectedNodeId()) || null;
  });

  const matchesSelectedNode = (record: StorageRecord): boolean => {
    const node = selectedNode();
    if (!node) return true;
    const nodeName = node.label.toLowerCase().trim();
    const nodeInstance = (node.instance || '').toLowerCase().trim();
    const hints = getRecordNodeHints(record);
    return hints.some((hint) => hint.includes(nodeName) || (nodeInstance && hint.includes(nodeInstance)));
  };

  const sourceOptions = createMemo(() => {
    const values = Array.from(new Set(options.records().map((record) => record.source.platform))).sort((a, b) =>
      sourceLabel(a).localeCompare(sourceLabel(b)),
    );
    return ['all', ...values];
  });

  const filteredRecords = createMemo(() => {
    const query = options.search().trim().toLowerCase();
    return options.records()
      .filter((record) => (options.sourceFilter() === 'all' ? true : record.source.platform === options.sourceFilter()))
      .filter((record) => (options.healthFilter() === 'all' ? true : record.health === options.healthFilter()))
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

  const sortedRecords = createMemo(() => {
    const numericCompare = (a: number, b: number): number => {
      if (a === b) return 0;
      return a < b ? -1 : 1;
    };

    return [...filteredRecords()].sort((a, b) => {
      let comparison = 0;

      if (options.sortKey() === 'usage') {
        comparison = numericCompare(getRecordUsagePercent(a), getRecordUsagePercent(b));
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
      return [{ key: 'All', items: sortedRecords() }];
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
      .map(([key, items]) => ({ key, items }));
  });

  const summary = createMemo<StorageSummary>(() => {
    const list = filteredRecords();
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

  return {
    sourceOptions,
    selectedNode,
    filteredRecords,
    sortedRecords,
    groupedRecords,
    summary,
  };
};
