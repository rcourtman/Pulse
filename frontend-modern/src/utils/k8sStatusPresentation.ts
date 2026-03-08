import type { StatusIndicator } from '@/utils/status';

export interface NamespaceCounts {
  total: number;
  online: number;
  warning: number;
  offline: number;
  unknown: number;
}

export function getNamespaceCountsIndicator(counts: NamespaceCounts): StatusIndicator {
  if (counts.offline > 0) {
    return { variant: 'danger', label: 'Offline' };
  }
  if (counts.warning > 0) {
    return { variant: 'warning', label: 'Warning' };
  }
  if (counts.online > 0) {
    return { variant: 'success', label: 'Online' };
  }
  return { variant: 'muted', label: 'Unknown' };
}
