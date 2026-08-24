import type { BackupStatus } from '@/utils/format';

// The age-derived BackupStatus plus the overlay state for a backup that is
// running right now. 'running' never comes from getBackupInfo (which only
// knows ages); callers overlay it from the guest's backupInProgress flag.
export type WorkloadsGuestBackupDisplayStatus = BackupStatus | 'running';

export interface WorkloadsGuestBackupStatusPresentation {
  color: string;
  bgColor: string;
  icon: 'check' | 'warning' | 'x' | 'running';
}

export interface WorkloadsGuestProtectionPresentation {
  label: string;
  tone: 'success' | 'warning' | 'danger';
}

const BACKUP_STATUS_PRESENTATION: Record<
  WorkloadsGuestBackupDisplayStatus,
  WorkloadsGuestBackupStatusPresentation
> = {
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
  overdue: {
    color: 'text-yellow-600 dark:text-yellow-400',
    bgColor: 'bg-yellow-100 dark:bg-yellow-900',
    icon: 'warning',
  },
  never: {
    color: 'text-red-600 dark:text-red-400',
    bgColor: 'bg-red-100 dark:bg-red-900',
    icon: 'x',
  },
  running: {
    color: 'text-blue-600 dark:text-blue-400',
    bgColor: 'bg-blue-100 dark:bg-blue-900',
    icon: 'running',
  },
};

export function getWorkloadsGuestBackupStatusPresentation(
  status: WorkloadsGuestBackupDisplayStatus,
): WorkloadsGuestBackupStatusPresentation {
  return BACKUP_STATUS_PRESENTATION[status];
}

export function getWorkloadsGuestBackupTooltip(
  status: BackupStatus,
  ageFormatted?: string | null,
  backupRunning?: boolean,
): string {
  const base =
    status === 'never' ? 'No completed backup found' : `Last backup: ${ageFormatted || 'Unknown'}`;
  if (backupRunning) {
    return `Backup running now · ${base.toLowerCase()}`;
  }
  if (status === 'never') {
    return 'No backup found';
  }
  return base;
}

export function getWorkloadsGuestProtectionPresentation(options: {
  backupInProgress?: boolean;
  ageLabel?: string | null;
  ageClass?: string | null;
}): WorkloadsGuestProtectionPresentation {
  if (options.backupInProgress) {
    return { label: 'Backup running', tone: 'success' };
  }
  if (!options.ageLabel) {
    return { label: 'No backup found', tone: 'danger' };
  }
  return {
    label: options.ageLabel,
    tone: options.ageClass?.includes('green')
      ? 'success'
      : options.ageClass?.includes('red')
        ? 'danger'
        : 'warning',
  };
}

export function getWorkloadsGuestNetworkEmptyState(): string {
  return 'No IP assigned';
}

export function getWorkloadGuestDiskStatusMessage(reason?: string): string {
  const carriedForward = reason?.startsWith('prev-') ?? false;
  const normalizedReason = carriedForward ? reason?.slice(5) : reason;

  const message = (() => {
    switch (normalizedReason) {
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
  })();

  return carriedForward ? `Using last known disk stats. ${message}` : message;
}
