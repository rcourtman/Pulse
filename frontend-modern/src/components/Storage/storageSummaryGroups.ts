import type { SummarySeriesGroupScope } from '@/components/shared/summaryCardInteraction';
import {
  buildStorageGroupRowPresentation,
  getStorageGroupPoolCountLabel,
} from '@/features/storageBackups/groupPresentation';
import { resolveStorageRecordMetricResourceId } from '@/features/storageBackups/storageMetricsIdentity';
import type { StorageGroupKey, StorageGroupedRecords } from './useStorageModel';

const normalizeStorageSummaryGroupKey = (value: string): string => value.trim();

export const buildStorageSummaryGroupId = (
  groupBy: StorageGroupKey,
  groupKey: string,
): string | null => {
  if (groupBy === 'none') {
    return null;
  }
  const normalizedGroupKey = normalizeStorageSummaryGroupKey(groupKey);
  if (!normalizedGroupKey) {
    return null;
  }
  return `storage:${groupBy}:${normalizedGroupKey}`;
};

export const buildStorageSummaryGroupScope = (
  group: StorageGroupedRecords,
  groupBy: StorageGroupKey,
): SummarySeriesGroupScope | null => {
  const id = buildStorageSummaryGroupId(groupBy, group.key);
  if (!id) {
    return null;
  }

  const seriesIds = Array.from(
    new Set(group.items.map((record) => resolveStorageRecordMetricResourceId(record)).filter(Boolean)),
  );
  if (seriesIds.length === 0) {
    return null;
  }

  const presentation = buildStorageGroupRowPresentation(group);
  const countLabel = getStorageGroupPoolCountLabel(group.items.length);

  return {
    id,
    label: `${presentation.label} (${countLabel})`,
    seriesIds,
  };
};

export const buildStorageSummaryGroupScopeMap = (
  groups: StorageGroupedRecords[],
  groupBy: StorageGroupKey,
): Map<string, SummarySeriesGroupScope> => {
  const scopes = new Map<string, SummarySeriesGroupScope>();
  for (const group of groups) {
    const scope = buildStorageSummaryGroupScope(group, groupBy);
    if (scope) {
      scopes.set(scope.id, scope);
    }
  }
  return scopes;
};
