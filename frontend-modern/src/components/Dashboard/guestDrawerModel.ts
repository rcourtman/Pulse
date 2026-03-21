import type { WorkloadGuest } from '@/types/workloads';

import { formatBytes } from '@/utils/format';
import { resolveWorkloadType } from '@/utils/workloads';

type Guest = WorkloadGuest;

export interface GuestDrawerProps {
  guest: Guest;
  onClose: () => void;
  customUrl?: string;
  onCustomUrlChange?: (guestId: string, url: string) => void;
}

export type GuestDrawerTab = 'overview' | 'discovery';

export interface GuestDrawerBackupPresentation {
  ageClass: string;
  ageLabel: string;
  dateLabel: string;
}

export const isGuestDrawerVM = (guest: Guest): boolean => resolveWorkloadType(guest) === 'vm';

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
    ageLabel:
      daysSince === 0 ? 'Today' : daysSince === 1 ? 'Yesterday' : `${daysSince}d ago`,
    dateLabel: backupDate.toLocaleDateString(),
  };
};
