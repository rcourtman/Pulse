import { Accessor, createMemo } from 'solid-js';
import type { StorageRecord } from '@/features/storageBackups/models';
import type { PhysicalDiskAlertResourceIdResolver } from '@/features/storageBackups/diskPresentation';
import {
  buildStoragePoolDetailConfigRows,
  buildStoragePoolDetailTopologyRows,
  buildStoragePoolDetailZfsSummary,
  getStoragePoolLinkedDisks,
  resolveStoragePoolDetailChartTarget,
} from '@/features/storageBackups/storagePoolDetailPresentation';
import type { GuestDrawerHistoryTarget } from '@/components/Workloads/guestDrawerModel';
import type { Resource } from '@/types/resource';

type UseStoragePoolDetailModelOptions = {
  record: Accessor<StorageRecord>;
  physicalDisks: Accessor<Resource[]>;
  getDiskAlertResourceIds?: PhysicalDiskAlertResourceIdResolver;
};

export const useStoragePoolDetailModel = (options: UseStoragePoolDetailModelOptions) => {
  // The pool's capacity history: its canonical metrics target, else its own
  // id. Snapshot replacement makes a new target with the same coordinates;
  // the drawer chart's query only re-reads on a real change of coordinates.
  const historyTarget = createMemo((): GuestDrawerHistoryTarget => {
    const target = resolveStoragePoolDetailChartTarget(options.record());
    return {
      resourceType: target.resourceType,
      resourceId: target.resourceId || options.record().id,
    };
  });
  const configRows = createMemo(() => buildStoragePoolDetailConfigRows(options.record()));
  const zfsSummary = createMemo(() => buildStoragePoolDetailZfsSummary(options.record()));
  const linkedDisks = createMemo(() =>
    getStoragePoolLinkedDisks(
      options.record(),
      options.physicalDisks(),
      options.getDiskAlertResourceIds,
    ),
  );
  const topologyRows = createMemo(() =>
    buildStoragePoolDetailTopologyRows(options.record(), linkedDisks()),
  );

  return {
    historyTarget,
    configRows,
    topologyRows,
    zfsSummary,
    linkedDisks,
  };
};
