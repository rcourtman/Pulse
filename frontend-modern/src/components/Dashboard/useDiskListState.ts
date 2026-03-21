import { createMemo } from 'solid-js';

import { getDashboardGuestDiskStatusMessage } from '@/utils/dashboardGuestPresentation';

import { buildDashboardDiskPresentation, type DiskListProps } from './diskListModel';

export function useDiskListState(props: DiskListProps) {
  const hasDisks = createMemo(() => props.disks.length > 0);
  const diskStatusTooltip = createMemo(() =>
    getDashboardGuestDiskStatusMessage(props.diskStatusReason),
  );
  const diskPresentation = createMemo(() =>
    props.disks.map((disk, index) => buildDashboardDiskPresentation(disk, index)),
  );

  return {
    diskPresentation,
    diskStatusTooltip,
    hasDisks,
  };
}
