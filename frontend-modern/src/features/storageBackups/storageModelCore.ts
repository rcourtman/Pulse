import type { NormalizedHealth, StorageRecord } from './models';
import { getSourcePlatformLabel } from '@/utils/sourcePlatforms';
import { normalizeStorageSourceKey } from '@/utils/storageSources';
import {
  getStorageRecordActionSummary,
  getStorageRecordContent,
  getStorageRecordHostLabel,
  getStorageRecordImpactSummary,
  getStorageRecordIssueLabel,
  getStorageRecordIssueSummary,
  getStorageRecordNodeHints,
  getStorageRecordNodeLabel,
  getStorageRecordPlatformLabel,
  getStorageRecordProtectionLabel,
  getStorageRecordStats,
  getStorageRecordStatus,
  getStorageRecordTopologyLabel,
  getStorageRecordType,
  getStorageRecordUsagePercent,
} from './recordPresentation';

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

export const findSelectedStorageNode = (
  selectedNodeId: string,
  nodeOptions: StorageNodeOption[],
): StorageNodeOption | null => {
  if (selectedNodeId === 'all') return null;
  return nodeOptions.find((node) => node.id === selectedNodeId) || null;
};

export const matchesStorageRecordNode = (
  record: StorageRecord,
  node: StorageNodeOption | null,
): boolean => {
  if (!node) return true;
  const aliases = Array.from(
    new Set(
      [node.id, node.label, ...(node.aliases || [])]
        .map((value) => (value || '').toLowerCase().trim())
        .filter(Boolean),
    ),
  );
  const hints = getStorageRecordNodeHints(record);
  return hints.some((hint) => aliases.some((alias) => hint.includes(alias)));
};

export const buildStorageSourceOptions = (records: StorageRecord[]): string[] => {
  const values = Array.from(
    new Set(records.map((record) => normalizeStorageSourceKey(record.source.platform))),
  ).sort((a, b) => getSourcePlatformLabel(a).localeCompare(getSourcePlatformLabel(b)));
  return ['all', ...values];
};

export const matchesStorageRecordSearch = (record: StorageRecord, query: string): boolean => {
  if (!query) return true;
  const haystack = [
    record.name,
    record.category,
    record.location.label,
    record.source.platform,
    getStorageRecordPlatformLabel(record),
    getStorageRecordHostLabel(record),
    getStorageRecordTopologyLabel(record),
    getStorageRecordProtectionLabel(record),
    getStorageRecordIssueLabel(record),
    getStorageRecordIssueSummary(record),
    getStorageRecordImpactSummary(record),
    getStorageRecordActionSummary(record),
    getStorageRecordType(record),
    getStorageRecordContent(record),
    getStorageRecordStatus(record),
    ...(record.capabilities || []),
  ]
    .filter(Boolean)
    .join(' ')
    .toLowerCase();
  return haystack.includes(query);
};

export interface FilterStorageRecordsOptions {
  sourceFilter: string;
  healthFilter: 'all' | NormalizedHealth;
  selectedNode: StorageNodeOption | null;
  search: string;
}

export const filterStorageRecords = (
  records: StorageRecord[],
  options: FilterStorageRecordsOptions,
): StorageRecord[] => {
  const query = options.search.trim().toLowerCase();
  const selectedSource = normalizeStorageSourceKey(options.sourceFilter);
  return records
    .filter((record) =>
      selectedSource === 'all'
        ? true
        : normalizeStorageSourceKey(record.source.platform) === selectedSource,
    )
    .filter((record) =>
      options.healthFilter === 'all' ? true : record.health === options.healthFilter,
    )
    .filter((record) => matchesStorageRecordNode(record, options.selectedNode))
    .filter((record) => matchesStorageRecordSearch(record, query));
};

export const sortStorageRecords = (
  records: StorageRecord[],
  sortKey: StorageSortKey,
  sortDirection: 'asc' | 'desc',
): StorageRecord[] => {
  const numericCompare = (a: number, b: number): number => {
    if (a === b) return 0;
    return a < b ? -1 : 1;
  };

  return [...records].sort((a, b) => {
    let comparison = 0;

    if (sortKey === 'usage') {
      comparison = numericCompare(getStorageRecordUsagePercent(a), getStorageRecordUsagePercent(b));
    } else if (sortKey === 'priority') {
      comparison = numericCompare(a.incidentPriority || 0, b.incidentPriority || 0);
    } else if (sortKey === 'type') {
      comparison = getStorageRecordType(a).localeCompare(getStorageRecordType(b), undefined, {
        sensitivity: 'base',
      });
    } else {
      comparison = a.name.localeCompare(b.name, undefined, { sensitivity: 'base' });
    }

    if (comparison === 0) {
      comparison = a.name.localeCompare(b.name, undefined, { sensitivity: 'base' });
    }

    return sortDirection === 'asc' ? comparison : -comparison;
  });
};

export const groupStorageRecords = (
  records: StorageRecord[],
  groupBy: StorageGroupKey,
): StorageGroupedRecords[] => {
  if (groupBy === 'none') {
    return [{ key: 'All', items: records, stats: getStorageRecordStats(records) }];
  }

  const groups = new Map<string, StorageRecord[]>();

  for (const record of records) {
    const key =
      groupBy === 'type'
        ? getStorageRecordType(record)
        : groupBy === 'status'
          ? getStorageRecordStatus(record)
          : getStorageRecordNodeLabel(record);
    const normalized = key.trim() || 'unknown';
    if (!groups.has(normalized)) groups.set(normalized, []);
    groups.get(normalized)!.push(record);
  }

  return Array.from(groups.entries())
    .sort(([keyA], [keyB]) => keyA.localeCompare(keyB, undefined, { sensitivity: 'base' }))
    .map(([key, items]) => ({ key, items, stats: getStorageRecordStats(items) }));
};

export const summarizeStorageRecords = (records: StorageRecord[]): StorageSummary => {
  const totals = getStorageRecordStats(records);
  return {
    count: records.length,
    totalBytes: totals.totalBytes,
    usedBytes: totals.usedBytes,
    usagePercent: totals.usagePercent,
    byHealth: totals.byHealth,
  };
};
