import { createMemo } from 'solid-js';

import { getWorkloadGuestDiskStatusMessage } from '@/utils/workloadGuestPresentation';

import { buildWorkloadsDiskPresentation, type DiskListProps } from './diskListModel';

export function useDiskListState(props: DiskListProps) {
  const hasDisks = createMemo(() => props.disks.length > 0);
  const diskStatusTooltip = createMemo(() =>
    getWorkloadGuestDiskStatusMessage(props.diskStatusReason),
  );
  const diskPresentation = createMemo(() =>
    props.disks.map((disk, index) => buildWorkloadsDiskPresentation(disk, index)),
  );

  return {
    diskPresentation,
    diskStatusTooltip,
    hasDisks,
  };
}
