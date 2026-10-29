import { resolveBackupThresholds, type BackupStatus, type BackupThresholds } from '@/utils/format';

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
    // The legacy key means no completion time in current inventory, not
    // independently established absence of every backup or restore point.
    color: 'text-yellow-600 dark:text-yellow-400',
    bgColor: 'bg-yellow-100 dark:bg-yellow-900/25',
    icon: 'warning',
  },
  unknown: {
    color: 'text-yellow-600 dark:text-yellow-400',
    bgColor: 'bg-yellow-100 dark:bg-yellow-900/25',
    icon: 'warning',
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

const formatBackupAgeLimit = (hours: number): string => {
  if (hours >= 24 && hours % 24 === 0) {
    const days = hours / 24;
    return `${days} day${days === 1 ? '' : 's'}`;
  }
  return `${hours} hour${hours === 1 ? '' : 's'}`;
};

// Which of the operator's own age limits coloured a completed backup, so an
// amber shield says why instead of leaving the user to find the Recovery
// thresholds (#2858). Amber starts after Fresh Hours for stale and overdue
// alike, so both name that field; Stale Hours only marks overdue on top.
// Statuses without an age have no rule to name.
export function getWorkloadsGuestBackupAgeRule(
  status: BackupStatus,
  thresholds?: BackupThresholds,
): string | null {
  const { freshHours, staleHours } = resolveBackupThresholds(thresholds);
  switch (status) {
    case 'fresh':
      return `Fresh: no more than ${formatBackupAgeLimit(freshHours)} old (Fresh Hours).`;
    case 'stale':
      return `Shown amber: more than ${formatBackupAgeLimit(freshHours)} old (Fresh Hours).`;
    case 'overdue':
      return `Shown amber: more than ${formatBackupAgeLimit(freshHours)} old (Fresh Hours). Also past Stale Hours (${formatBackupAgeLimit(staleHours)}).`;
    default:
      return null;
  }
}

export function getWorkloadsGuestBackupTooltip(
  status: BackupStatus,
  ageFormatted?: string | null,
  backupRunning?: boolean,
): string {
  const base =
    status === 'never'
      ? 'No completed backup reported'
      : status === 'unknown'
        ? ageFormatted || 'Backup age unavailable'
        : `Last backup: ${ageFormatted || 'Unknown'}`;
  if (backupRunning) {
    return `Backup running now · ${base.toLowerCase()}`;
  }
  return base;
}

export function getWorkloadsGuestProtectionPresentation(options: {
  ageLabel?: string | null;
  ageClass?: string | null;
}): WorkloadsGuestProtectionPresentation {
  if (!options.ageLabel) {
    return { label: 'No completed backup reported', tone: 'warning' };
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
        return 'Proxmox could not query the guest agent. This does not prove it is absent or stopped. Defer setup and live probes during backups or a guest incident.';
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
      case 'agent-completion-unverified':
        return 'Guest reads paused because completion of the earlier request could not be verified. Pulse will check again after the cooldown. Defer live diagnostics and guest-agent restarts during backups or a guest incident.';
      case 'agent-redirect':
        return 'Guest reads paused because the endpoint returned a redirect. Pulse did not follow it. Completion of the original guest request is uncertain. Defer live diagnostics and guest-agent restarts during backups or a guest incident.';
      case 'agent-transport-unverified':
        return 'Guest reads deferred because Pulse cannot verify that the configured connection avoids resending guest requests. This guest request was not sent. Defer connection changes and live probes during backups or a guest incident.';
      case 'agent-capacity':
        return 'Guest reads deferred because Pulse has reached its guest-read capacity. Pulse will check again on a later poll.';
      case 'invalid-guest-key':
        return 'Guest reads unavailable because the VM identity is invalid.';
      case 'permission-denied':
        return "Permission denied. If this VM is an intended monitoring target, review the configured account and token's effective read permissions: VM.Monitor (PVE 8) or VM.GuestAgent.Audit (PVE 9+). Do not widen intentionally scoped access just to clear this error. A denied read does not establish guest or filesystem health. Defer access changes and live probes during backups or a guest incident.";
      case 'agent-disabled':
        return 'Proxmox reports the guest agent disabled in VM configuration. Review guest-OS-specific setup outside backups or guest incidents.';
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
        return 'Guest filesystem usage is unavailable. The cause is unknown. Use guest-local filesystem tools. Defer setup and live probes during backups or a guest incident.';
    }
  })();

  return carriedForward ? `Using last known disk stats. ${message}` : message;
}

export interface WorkloadGuestDiskReadSource {
  status?: string;
  /** Proxmox's reason for its own guest filesystem read. */
  diskStatusReason?: string;
  /** `disks` came from the linked Pulse agent because Proxmox reported none. */
  disksFromAgent?: boolean;
  agentStale?: boolean;
}

export type WorkloadGuestDiskRead =
  | { state: 'current'; message: null; needsAction: false }
  | {
      state: 'last-known' | 'unavailable';
      /** Why the reading is not current. */
      message: string;
      /** A broken read the user can fix, rather than a setup gap. */
      needsAction: boolean;
    };

// Disk-read reasons that point at something broken the user can fix.
const DISK_READ_ACTIONABLE_REASONS = new Set(['permission-denied', 'agent-error']);

// One freshness rule for a guest's filesystem rows and aggregate disk metric,
// shared by the workload table cell, the drawer and its History. Proxmox
// records no guest disk usage without guest filesystems, so when the linked
// Pulse agent's filesystems fill in, the registry keeps the agent's disk
// metric too and the reading follows that agent, not the Proxmox reason.
export function getWorkloadGuestDiskRead(
  guest: WorkloadGuestDiskReadSource,
  isVM: boolean,
): WorkloadGuestDiskRead {
  const reason = isVM ? guest.diskStatusReason : undefined;
  // Power-state updates can arrive without a filesystem-read reason, while
  // the canonical resource still carries earlier numbers. A stopped VM has
  // no current in-guest reading, regardless of which producer supplied it.
  // LXC platform storage remains independent of guest runtime power state.
  if (isVM && (guest.status === 'stopped' || reason === 'vm-stopped')) {
    return {
      state: 'unavailable',
      message: getWorkloadGuestDiskStatusMessage('vm-stopped'),
      needsAction: false,
    };
  }
  if (guest.disksFromAgent) {
    if (guest.status === 'stopped') {
      return {
        state: 'unavailable',
        message: isVM
          ? getWorkloadGuestDiskStatusMessage('vm-stopped')
          : 'Filesystem usage is unavailable.',
        needsAction: false,
      };
    }
    return guest.agentStale
      ? {
          state: 'last-known',
          message: 'Using last known disk stats. The Pulse Agent in this guest stopped reporting.',
          needsAction: false,
        }
      : { state: 'current', message: null, needsAction: false };
  }
  if (!reason) return { state: 'current', message: null, needsAction: false };
  return {
    state: reason.startsWith('prev-') ? 'last-known' : 'unavailable',
    message: getWorkloadGuestDiskStatusMessage(reason),
    needsAction: DISK_READ_ACTIONABLE_REASONS.has(reason.replace(/^prev-/, '')),
  };
}
