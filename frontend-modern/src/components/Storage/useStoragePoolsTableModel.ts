import { createMemo } from 'solid-js';
import {
  buildStoragePoolsTableGroups,
  buildStoragePoolsTableRowModel,
} from '@/features/storageBackups/storagePoolsTablePresentation';
import type { StorageAlertRowState } from '@/features/storageBackups/storageAlertState';
import type { StorageGroupedRecords, StorageGroupKey } from './useStorageModel';

type UseStoragePoolsTableModelOptions = {
  groupedRecords: () => StorageGroupedRecords[];
  groupBy: () => StorageGroupKey;
  expandedGroups: () => Set<string>;
  expandedPoolId: () => string | null;
  highlightedRecordId: () => string | null;
  nodeOnlineByLabel: () => Map<string, boolean>;
  getRecordAlertState: (recordId: string) => StorageAlertRowState;
  setExpandedPoolId: (value: string | null | ((current: string | null) => string | null)) => void;
};

export const useStoragePoolsTableModel = (options: UseStoragePoolsTableModelOptions) => {
  const groups = createMemo(() =>
    buildStoragePoolsTableGroups(
      options.groupedRecords(),
      options.groupBy(),
      options.expandedGroups(),
    ),
  );

  const buildRowModel = (_recordId: string, record: Parameters<typeof buildStoragePoolsTableRowModel>[0]) =>
    buildStoragePoolsTableRowModel(record, {
      expandedPoolId: options.expandedPoolId(),
      highlightedRecordId: options.highlightedRecordId(),
      nodeOnlineByLabel: options.nodeOnlineByLabel(),
      getRecordAlertState: options.getRecordAlertState,
    });

  const togglePool = (recordId: string) => {
    options.setExpandedPoolId((current) => (current === recordId ? null : recordId));
  };

  return {
    groups,
    buildRowModel,
    togglePool,
  };
};
