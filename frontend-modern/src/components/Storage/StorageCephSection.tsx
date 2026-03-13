import { Component, Show } from 'solid-js';
import StorageCephSummaryCard from '@/components/Storage/StorageCephSummaryCard';
import type { CephSummaryStats } from '@/features/storageBackups/cephSummaryPresentation';
import type { StorageRecord } from '@/features/storageBackups/models';
import { useStorageCephSectionModel } from './useStorageCephSectionModel';

type StorageCephSectionProps = {
  view: () => 'pools' | 'disks';
  summary: () => CephSummaryStats | null;
  filteredRecords: () => StorageRecord[];
  isCephRecord: (record: StorageRecord) => boolean;
};

export const StorageCephSection: Component<StorageCephSectionProps> = (props) => {
  const model = useStorageCephSectionModel({
    view: props.view,
    summary: props.summary,
    filteredRecords: props.filteredRecords,
    isCephRecord: props.isCephRecord,
  });

  return (
    <Show when={model.showSummary()}>
      <StorageCephSummaryCard summary={props.summary()!} />
    </Show>
  );
};

export default StorageCephSection;
