import { getStorageRecordNodeLabel } from './recordPresentation';
import { getStorageRowAlertPresentation } from './storageRowAlertPresentation';
import type { StorageAlertRowState } from './storageAlertState';
import type { StorageRecord } from './models';
import type { StorageGroupedRecords, StorageGroupKey } from '@/components/Storage/useStorageModel';

export type StoragePoolsTableGroupModel = StorageGroupedRecords & {
  expanded: boolean;
  showHeader: boolean;
};

export type StoragePoolsTableRowModel = {
  expanded: boolean;
  parentNodeOnline: boolean;
  rowClass: string;
  // Reason the row is highlighted; shown in the State cell so the colour is
  // never unexplained. Null when the row carries no open alert.
  alertHeadline: string | null;
  alertHeadlineCompact: string | null;
  alertHeadlineClass: string;
  alertDataAttrs: {
    'data-row-id': string;
    'data-alert-state': string;
    'data-alert-severity': string;
    'data-resource-highlighted': string;
  };
};

export const buildStoragePoolsTableGroups = (
  groupedRecords: StorageGroupedRecords[],
  groupBy: StorageGroupKey,
  expandedGroups: Set<string>,
): StoragePoolsTableGroupModel[] =>
  groupedRecords.map((group) => ({
    ...group,
    expanded: expandedGroups.has(group.key),
    showHeader: groupBy !== 'none',
  }));

export const buildStoragePoolsTableRowModel = (
  record: StorageRecord,
  options: {
    expandedPoolId: string | null;
    highlightedRecordId: string | null;
    nodeOnlineByLabel: Map<string, boolean>;
    getRecordAlertState: (recordId: string) => StorageAlertRowState;
  },
): StoragePoolsTableRowModel => {
  const expanded = options.expandedPoolId === record.id;
  const nodeLabel = getStorageRecordNodeLabel(record).trim().toLowerCase();
  const nodeStatus = nodeLabel ? options.nodeOnlineByLabel.get(nodeLabel) : undefined;
  const parentNodeOnline = nodeStatus === undefined ? true : nodeStatus;
  const alertState = options.getRecordAlertState(record.id);
  const rowAlertPresentation = getStorageRowAlertPresentation({
    alertState,
    parentNodeOnline,
    isExpanded: expanded,
    isResourceHighlighted: options.highlightedRecordId === record.id,
  });

  return {
    expanded,
    parentNodeOnline,
    rowClass: rowAlertPresentation.rowClass,
    alertHeadline:
      rowAlertPresentation.dataAlertState === 'unacknowledged'
        ? (alertState.headline ?? null)
        : null,
    alertHeadlineCompact:
      rowAlertPresentation.dataAlertState === 'unacknowledged'
        ? (alertState.headlineCompact ?? alertState.headline ?? null)
        : null,
    alertHeadlineClass:
      alertState.severity === 'critical'
        ? 'text-red-700 dark:text-red-300 font-medium'
        : 'text-amber-700 dark:text-amber-300 font-medium',
    alertDataAttrs: {
      'data-row-id': record.id,
      'data-alert-state': rowAlertPresentation.dataAlertState,
      'data-alert-severity': rowAlertPresentation.dataAlertSeverity,
      'data-resource-highlighted': rowAlertPresentation.dataResourceHighlighted,
    },
  };
};
