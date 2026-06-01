export const PROXMOX_BACKUP_SOURCE_KINDS = ['pbs', 'archive', 'snapshot'] as const;

export type ProxmoxBackupSourceKind = (typeof PROXMOX_BACKUP_SOURCE_KINDS)[number];

export interface ProxmoxBackupSourcePresentation {
  badgeClassName: string;
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

const SOURCE_PRESENTATION: Record<ProxmoxBackupSourceKind, ProxmoxBackupSourcePresentation> = {
  pbs: {
    badgeClassName: 'bg-cyan-100 text-cyan-700 dark:bg-cyan-900/40 dark:text-cyan-200',
    badgeLabel: 'PBS',
    compactFilterLabel: 'PBS',
    coverageColumnLabel: 'PBS',
    coverageFallbackLabel: 'No PBS snapshot',
    detailFallbackLabel: 'PBS snapshot',
    filterAriaLabel: 'PBS snapshots from Proxmox Backup Server',
    filterLabel: 'PBS snapshots',
    filterTitle: 'Direct inventory from Proxmox Backup Server',
    sourceTitle: 'Direct inventory from Proxmox Backup Server',
    stateFallbackLabel: 'Snapshot',
    timelineLabel: 'PBS snapshots',
    timelineSegmentClassName: 'bg-cyan-500',
    timelineSwatchClassName: 'bg-cyan-500',
  },
  archive: {
    badgeClassName: 'bg-blue-100 text-blue-700 dark:bg-blue-900/40 dark:text-blue-200',
    badgeLabel: 'PVE file',
    compactFilterLabel: 'PVE files',
    coverageColumnLabel: 'PVE files',
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
    badgeClassName: 'bg-violet-100 text-violet-700 dark:bg-violet-900/40 dark:text-violet-200',
    badgeLabel: 'Snapshot',
    compactFilterLabel: 'Snapshots',
    coverageColumnLabel: 'Snapshots',
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
