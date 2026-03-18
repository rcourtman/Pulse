import type { BackupStatus } from '@/utils/format';

export interface DashboardGuestBackupStatusPresentation {
  color: string;
  bgColor: string;
  icon: 'check' | 'warning' | 'x';
}

const BACKUP_STATUS_PRESENTATION: Record<BackupStatus, DashboardGuestBackupStatusPresentation> = {
  fresh: {
    color: 'text-green-600 dark:text-green-400',
    bgColor: 'bg-green-100 dark:bg-green-900',
    icon: 'check',
  },
  stale: {
    color: 'text-yellow-600 dark:text-yellow-400',
    bgColor: 'bg-yellow-100 dark:bg-yellow-900',
    icon: 'warning',
  },
  critical: {
    color: 'text-red-600 dark:text-red-400',
    bgColor: 'bg-red-100 dark:bg-red-900',
    icon: 'x',
  },
  never: {
    color: 'text-muted',
    bgColor: 'bg-surface-alt',
    icon: 'x',
  },
};

export function getDashboardGuestBackupStatusPresentation(
  status: BackupStatus,
): DashboardGuestBackupStatusPresentation {
  return BACKUP_STATUS_PRESENTATION[status];
}

export function getDashboardGuestBackupTooltip(
  status: BackupStatus,
  ageFormatted?: string | null,
): string {
  if (status === 'never') {
    return 'No backup found';
  }
  return `Last backup: ${ageFormatted || 'Unknown'}`;
}

export function getDashboardGuestNetworkEmptyState(): string {
  return 'No IP assigned';
}

export function getDashboardGuestDiskStatusMessage(reason?: string): string {
  switch (reason) {
    case 'agent-not-running':
      return 'Guest agent not running. Install and start qemu-guest-agent in the VM.';
    case 'agent-timeout':
      return 'Guest agent timeout. Agent may need to be restarted.';
    case 'permission-denied':
      return 'Permission denied. Check that your Pulse user/token has VM.Monitor permission (PVE 8) or VM.GuestAgent.Audit permission (PVE 9).';
    case 'agent-disabled':
      return 'Guest agent is disabled in VM configuration. Enable it in VM Options.';
    case 'no-filesystems':
      return 'No filesystems found. VM may be booting or using a Live ISO.';
    case 'special-filesystems-only':
      return 'Only special filesystems detected (ISO/squashfs). This is normal for Live systems.';
    case 'agent-error':
      return 'Error communicating with guest agent.';
    case 'no-data':
      return 'No disk data available from Proxmox API.';
    default:
      return 'Disk stats unavailable. Guest agent may not be installed.';
  }
}
