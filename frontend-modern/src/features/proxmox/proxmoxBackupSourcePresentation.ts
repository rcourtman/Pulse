import type { MetadataBadgeTone } from '@/components/shared/MetadataBadge';

export const PROXMOX_BACKUP_SOURCE_KINDS = ['pbs', 'archive', 'snapshot'] as const;

export type ProxmoxBackupSourceKind = (typeof PROXMOX_BACKUP_SOURCE_KINDS)[number];

export interface ProxmoxBackupSourcePresentation {
  badgeTone: MetadataBadgeTone;
  badgeLabel: string;
  compactFilterLabel: string;
  coverageColumnLabel: string;
  coverageFallbackLabel: string;
  detailFallbackLabel: string;
  filterAriaLabel: string;
  filterLabel: string;
  filterTitle: string;
  sourceTitle: string;
  stateFallbackLabel: string;
  timelineLabel: string;
  timelineSegmentClassName: string;
  timelineSwatchClassName: string;
}

// "Snapshot" means one thing on the Backups page: a guest snapshot, which is
// not a separate backup and never counts as protection. PBS calls its own
// restore points snapshots too, but beside guest snapshots that reads as the
// same thing, so PBS items are named PBS backups here.
const SOURCE_PRESENTATION: Record<ProxmoxBackupSourceKind, ProxmoxBackupSourcePresentation> = {
  pbs: {
    badgeTone: 'sky',
    badgeLabel: 'PBS',
    compactFilterLabel: 'PBS',
    coverageColumnLabel: 'Latest PBS',
    coverageFallbackLabel: 'No PBS backup',
    detailFallbackLabel: 'PBS backup',
    filterAriaLabel: 'PBS backups from Proxmox Backup Server',
    filterLabel: 'PBS backups',
    // The PBS UI calls these backup snapshots; say so where it helps, not in
    // the labels.
    filterTitle: 'Direct inventory from Proxmox Backup Server, which calls these backup snapshots',
    sourceTitle: 'Direct inventory from Proxmox Backup Server',
    stateFallbackLabel: 'Backup',
    timelineLabel: 'PBS backups',
    timelineSegmentClassName: 'bg-cyan-500',
    timelineSwatchClassName: 'bg-cyan-500',
  },
  archive: {
    badgeTone: 'info',
    badgeLabel: 'PVE file',
    compactFilterLabel: 'PVE files',
    coverageColumnLabel: 'Latest PVE file',
    coverageFallbackLabel: 'No PVE backup file',
    detailFallbackLabel: 'PVE backup file',
    filterAriaLabel: 'PVE backup files found on Proxmox VE storage',
    filterLabel: 'PVE backup files',
    filterTitle: 'vzdump backup files or volumes reported by Proxmox VE storage',
    sourceTitle: 'PVE backup files reported by Proxmox VE storage',
    stateFallbackLabel: 'Backup file',
    timelineLabel: 'PVE backup files',
    timelineSegmentClassName: 'bg-blue-500',
    timelineSwatchClassName: 'bg-blue-500',
  },
  snapshot: {
    badgeTone: 'indigo',
    badgeLabel: 'Snapshot',
    compactFilterLabel: 'Snapshots',
    coverageColumnLabel: 'Latest snapshot',
    coverageFallbackLabel: 'No guest snapshot',
    detailFallbackLabel: 'Guest snapshot',
    filterAriaLabel: 'Guest snapshots from Proxmox VE',
    filterLabel: 'Guest snapshots',
    filterTitle: 'Proxmox VE guest snapshots',
    sourceTitle: 'Proxmox VE guest snapshot',
    stateFallbackLabel: 'Snapshot',
    timelineLabel: 'Guest snapshots',
    timelineSegmentClassName: 'bg-violet-500',
    timelineSwatchClassName: 'bg-violet-500',
  },
};

export function getProxmoxBackupSourcePresentation(
  kind: ProxmoxBackupSourceKind,
): ProxmoxBackupSourcePresentation {
  return SOURCE_PRESENTATION[kind];
}

export function getProxmoxArchiveSourceTitle(isPbsStorage: boolean): string {
  if (isPbsStorage) {
    return 'Backup volume reported by Proxmox VE from a PBS-backed storage target';
  }
  return SOURCE_PRESENTATION.archive.sourceTitle;
}
