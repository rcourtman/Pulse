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
    bgColor: 'bg-green-100 dark:bg-green-900/25',
    icon: 'check',
  },
  stale: {
    color: 'text-yellow-600 dark:text-yellow-400',
    bgColor: 'bg-yellow-100 dark:bg-yellow-900/25',
    icon: 'warning',
  },
  overdue: {
    color: 'text-yellow-600 dark:text-yellow-400',
    bgColor: 'bg-yellow-100 dark:bg-yellow-900/25',
    icon: 'warning',
  },
  never: {
    color: 'text-red-600 dark:text-red-400',
    bgColor: 'bg-red-100 dark:bg-red-900/25',
    icon: 'x',
  },
  running: {
    color: 'text-blue-600 dark:text-blue-400',
    bgColor: 'bg-blue-100 dark:bg-blue-900/25',
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
  ageLabel?: string | null;
  ageClass?: string | null;
}): WorkloadsGuestProtectionPresentation {
  if (!options.ageLabel) {
    return { label: 'No completed backup found', tone: 'danger' };
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
        return 'Guest request timed out. Completion is uncertain. Do not restart the guest agent during a backup.';
      case 'vm-locked':
        return 'Guest reads paused while Proxmox reports a VM operation lock, such as a backup. Pulse will check again on a later poll.';
      case 'lock-unverified':
        return 'Guest reads deferred because Pulse cannot verify that the VM is unlocked. Pulse will check again on a later poll.';
      case 'agent-busy':
        return 'Guest reads deferred while an earlier guest request is still in progress. Pulse will check again on a later poll.';
      case 'agent-cooldown':
        return 'Guest reads paused after an earlier request did not complete reliably. Pulse will check again after the cooldown.';
      case 'agent-response-incomplete':
        return 'Guest reads paused because the previous response was incomplete. Pulse will check again on a later poll.';
      case 'agent-capacity':
        return 'Guest reads deferred because Pulse has reached its guest-read capacity. Pulse will check again on a later poll.';
      case 'invalid-guest-key':
        return 'Guest reads unavailable because the VM identity is invalid.';
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
      case 'vm-stopped':
        return 'Guest filesystem stats unavailable while the VM is stopped.';
      case 'no-status':
        return 'Guest filesystem stats unavailable because Pulse could not read the VM status from Proxmox.';
      default:
        return 'Disk stats unavailable. Guest agent may not be installed.';
    }
  })();

  return carriedForward ? `Using last known disk stats. ${message}` : message;
}
