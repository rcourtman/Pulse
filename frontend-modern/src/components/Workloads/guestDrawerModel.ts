import type { WorkloadGuest } from '@/types/workloads';
import type { HistoryTimeRange, ResourceType as HistoryResourceType } from '@/api/charts';

import { formatBytes } from '@/utils/format';
import { getCanonicalWorkloadId, resolveWorkloadType } from '@/utils/workloads';

type Guest = WorkloadGuest;

export interface GuestDrawerProps {
  guest: Guest;
  onClose: () => void;
  customUrl?: string;
  onCustomUrlChange?: (guestId: string, url: string) => void;
}

export type GuestDrawerTab = 'overview' | 'history' | 'discovery';

export interface GuestDrawerHistoryTarget {
  resourceType: HistoryResourceType;
  resourceId: string;
}

export interface GuestDrawerHistoryChartConfig {
  metric: string;
  label: string;
  unit: string;
  color: string;
}

export interface GuestDrawerBackupPresentation {
  ageClass: string;
  ageLabel: string;
  dateLabel: string;
}

export const isGuestDrawerVM = (guest: Guest): boolean => resolveWorkloadType(guest) === 'vm';

export const GUEST_DRAWER_HISTORY_DEFAULT_RANGE: HistoryTimeRange = '24h';

export const GUEST_DRAWER_HISTORY_CHARTS: GuestDrawerHistoryChartConfig[] = [
  { metric: 'cpu', label: 'CPU', unit: '%', color: '#8b5cf6' },
  { metric: 'memory', label: 'Memory', unit: '%', color: '#f59e0b' },
  { metric: 'disk', label: 'Disk', unit: '%', color: '#10b981' },
  { metric: 'netin', label: 'Network In', unit: 'B/s', color: '#10b981' },
  { metric: 'netout', label: 'Network Out', unit: 'B/s', color: '#fb923c' },
  { metric: 'diskread', label: 'Disk Read', unit: 'B/s', color: '#3b82f6' },
  { metric: 'diskwrite', label: 'Disk Write', unit: 'B/s', color: '#f59e0b' },
];

export const getGuestDrawerHistoryTarget = (guest: Guest): GuestDrawerHistoryTarget | null => {
  const resourceId = getCanonicalWorkloadId(guest).trim();
  if (!resourceId) return null;

  const workloadType = resolveWorkloadType(guest);
  switch (workloadType) {
    case 'vm':
      return { resourceType: 'vm', resourceId };
    case 'system-container':
      return { resourceType: 'system-container', resourceId };
    case 'app-container':
      return { resourceType: 'app-container', resourceId };
    case 'pod':
      return { resourceType: 'pod', resourceId };
    default:
      return null;
  }
};

export const hasGuestDrawerOsInfo = (guest: Guest): boolean =>
  (guest.osName?.length ?? 0) > 0 || (guest.osVersion?.length ?? 0) > 0;

export const getGuestDrawerAgentLabel = (guest: Guest): string => {
  const version = (guest.agentVersion || '').trim();
  if (!version) return '';
  return isGuestDrawerVM(guest) ? `QEMU ${version}` : version;
};

export const getGuestDrawerAgentTitle = (guest: Guest): string => {
  const version = (guest.agentVersion || '').trim();
  if (!version) return '';
  return isGuestDrawerVM(guest) ? `QEMU guest agent ${version}` : version;
};

export const getGuestDrawerMemoryExtraLines = (guest: Guest): string[] | undefined => {
  if (!guest.memory) return undefined;

  const lines: string[] = [];
  const total = guest.memory.total ?? 0;
  if (guest.memory.balloon && guest.memory.balloon > 0 && guest.memory.balloon !== total) {
    lines.push(`Balloon: ${formatBytes(guest.memory.balloon)}`);
  }
  if (guest.memory.swapTotal && guest.memory.swapTotal > 0) {
    const swapUsed = guest.memory.swapUsed ?? 0;
    lines.push(`Swap: ${formatBytes(swapUsed)} / ${formatBytes(guest.memory.swapTotal)}`);
  }
  return lines.length > 0 ? lines : undefined;
};

export const hasGuestDrawerFilesystemDetails = (guest: Guest): boolean =>
  Array.isArray(guest.disks) && guest.disks.length > 0;

export const getGuestDrawerNetworkInterfaces = (guest: Guest) => guest.networkInterfaces || [];

export const normalizeGuestDrawerTags = (tags: Guest['tags']): string[] => {
  if (Array.isArray(tags)) {
    return tags.map((tag) => tag.trim()).filter((tag) => tag.length > 0);
  }
  if (typeof tags === 'string') {
    return tags
      .split(',')
      .map((tag) => tag.trim())
      .filter((tag) => tag.length > 0);
  }
  return [];
};

export const getGuestDrawerBackupPresentation = (
  lastBackup: string | number | Date,
  now: Date = new Date(),
): GuestDrawerBackupPresentation => {
  const backupDate = new Date(lastBackup);
  const daysSince = Math.floor((now.getTime() - backupDate.getTime()) / (1000 * 60 * 60 * 24));
  const isOld = daysSince > 7;
  const isCritical = daysSince > 30;

  return {
    ageClass: isCritical
      ? 'text-red-600 dark:text-red-400'
      : isOld
        ? 'text-amber-600 dark:text-amber-400'
        : 'text-green-600 dark:text-green-400',
    ageLabel: daysSince === 0 ? 'Today' : daysSince === 1 ? 'Yesterday' : `${daysSince}d ago`,
    dateLabel: backupDate.toLocaleDateString(),
  };
};
