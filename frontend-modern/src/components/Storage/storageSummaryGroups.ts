import type { StorageGroupKey } from './useStorageModel';

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
