import type { NormalizedHealth, StorageHealthFilter, StorageRecord } from './models';
import { normalizeStorageSourceKey, orderStorageSourceKeys } from '@/utils/storageSources';
import { matchesStorageNodeTerms, parseStorageSearchQuery } from './storageSearchQuery';
import type { StorageCapacityDeltaPresentation } from './storageCapacityDeltaPresentation';
import { resolveStorageRecordMetricResourceId } from './storageMetricsIdentity';
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
import { getCompactStoragePoolProtectionLabel, getStoragePoolStateLabel } from './rowPresentation';

export type StorageSortKey =
  'priority' | 'name' | 'state' | 'source' | 'type' | 'host' | 'protection' | 'usage' | 'growth';
export type StorageGroupKey = 'node' | 'type' | 'status' | 'none';

export type StorageSortContext = {
  growthBySeriesId?: ReadonlyMap<string, StorageCapacityDeltaPresentation>;
};

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
  const values = orderStorageSourceKeys(
    records.map((record) => normalizeStorageSourceKey(record.source.platform)),
  ).filter((key) => key !== 'all');
  return ['all', ...values];
};

export const matchesStorageRecordSearch = (record: StorageRecord, query: string): boolean => {
  if (!query) return true;
  const parsed = parseStorageSearchQuery(query);
  if (!matchesStorageNodeTerms(getStorageRecordNodeHints(record), parsed.nodeTerms)) {
    return false;
  }
  if (parsed.freeTerms.length === 0) return true;
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
  return parsed.freeTerms.every((term) => haystack.includes(term));
};

export interface FilterStorageRecordsOptions {
  sourceFilter: string;
  healthFilter: StorageHealthFilter;
  selectedNode: StorageNodeOption | null;
  search: string;
}

export const matchesStorageHealthFilter = (
  health: NormalizedHealth,
  filter: StorageHealthFilter,
): boolean => {
  if (filter === 'all') return true;
  if (filter === 'attention') {
    return health === 'warning' || health === 'critical' || health === 'offline';
  }
  return health === filter;
};

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
    .filter((record) => matchesStorageHealthFilter(record.health, options.healthFilter))
    .filter((record) => matchesStorageRecordNode(record, options.selectedNode))
    .filter((record) => matchesStorageRecordSearch(record, query));
};

export const sortStorageRecords = (
  records: StorageRecord[],
  sortKey: StorageSortKey,
  sortDirection: 'asc' | 'desc',
  context: StorageSortContext = {},
): StorageRecord[] => {
  const numericCompare = (a: number, b: number): number => {
    if (a === b) return 0;
    return a < b ? -1 : 1;
  };

  const textCompare = (a: string, b: string): number =>
    a.localeCompare(b, undefined, { sensitivity: 'base', numeric: true });

  const growthDeltaBytes = (record: StorageRecord): number | null =>
    context.growthBySeriesId?.get(resolveStorageRecordMetricResourceId(record))?.deltaBytes ?? null;

  return [...records].sort((a, b) => {
    let comparison = 0;

    if (sortKey === 'usage') {
      comparison = numericCompare(getStorageRecordUsagePercent(a), getStorageRecordUsagePercent(b));
    } else if (sortKey === 'growth') {
      const left = growthDeltaBytes(a);
      const right = growthDeltaBytes(b);
      if (left === null && right !== null) return 1;
      if (left !== null && right === null) return -1;
      comparison = left === null || right === null ? 0 : numericCompare(left, right);
    } else if (sortKey === 'priority') {
      comparison = numericCompare(a.incidentPriority || 0, b.incidentPriority || 0);
    } else if (sortKey === 'state') {
      comparison = textCompare(getStoragePoolStateLabel(a), getStoragePoolStateLabel(b));
    } else if (sortKey === 'source') {
      comparison = textCompare(getStorageRecordPlatformLabel(a), getStorageRecordPlatformLabel(b));
    } else if (sortKey === 'type') {
      comparison = textCompare(getStorageRecordTopologyLabel(a), getStorageRecordTopologyLabel(b));
    } else if (sortKey === 'host') {
      comparison = textCompare(getStorageRecordHostLabel(a), getStorageRecordHostLabel(b));
    } else if (sortKey === 'protection') {
      comparison = textCompare(
        getCompactStoragePoolProtectionLabel(a),
        getCompactStoragePoolProtectionLabel(b),
      );
    } else {
      comparison = textCompare(a.name, b.name);
    }

    if (comparison === 0) {
      comparison = textCompare(a.name, b.name);
    }

    return sortDirection === 'asc' ? comparison : -comparison;
  });
};

export const groupStorageRecords = (
  records: StorageRecord[],
  groupBy: StorageGroupKey,
): StorageGroupedRecords[] => {
  if (records.length === 0) {
    return [];
  }

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
