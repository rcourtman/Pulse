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
  rowStyle: Record<string, string>;
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
  const rowAlertPresentation = getStorageRowAlertPresentation({
    alertState: options.getRecordAlertState(record.id),
    parentNodeOnline,
    isExpanded: expanded,
    isResourceHighlighted: options.highlightedRecordId === record.id,
  });

  return {
    expanded,
    parentNodeOnline,
    rowClass: rowAlertPresentation.rowClass,
    rowStyle: rowAlertPresentation.rowStyle,
    alertDataAttrs: {
      'data-row-id': record.id,
      'data-alert-state': rowAlertPresentation.dataAlertState,
      'data-alert-severity': rowAlertPresentation.dataAlertSeverity,
      'data-resource-highlighted': rowAlertPresentation.dataResourceHighlighted,
    },
  };
};
