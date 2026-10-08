import { describe, expect, it } from 'vitest';
import { guestDiskDeferrals } from '@/components/Workloads/__fixtures__/guestDiskDeferrals';
import {
  getWorkloadsGuestBackupStatusPresentation,
  getWorkloadsGuestBackupTooltip,
  getWorkloadsGuestProtectionPresentation,
  getWorkloadGuestDiskRead,
  getWorkloadGuestDiskStatusMessage,
  getWorkloadsGuestNetworkEmptyState,
} from '@/utils/workloadGuestPresentation';

describe('workloadGuestPresentation', () => {
  it.each(guestDiskDeferrals)(
    'explains %s with and without retained disk evidence',
    (reason, message) => {
      expect(getWorkloadGuestDiskStatusMessage(reason)).toBe(message);
      expect(getWorkloadGuestDiskStatusMessage(`prev-${reason}`)).toBe(
        `Using last known disk stats. ${message}`,
      );
      expect(message).not.toContain('may not be installed');
      expect(message).not.toContain('may need to be restarted');
    },
  );

  it('returns canonical guest backup status presentation', () => {
    expect(getWorkloadsGuestBackupStatusPresentation('fresh')).toEqual({
      color: 'text-green-600 dark:text-green-400',
      bgColor: 'bg-green-100 dark:bg-green-900/25',
      icon: 'check',
    });
    expect(getWorkloadsGuestBackupStatusPresentation('never')).toEqual({
      color: 'text-red-600 dark:text-red-400',
      bgColor: 'bg-red-100 dark:bg-red-900/25',
      icon: 'x',
    });
    expect(getWorkloadsGuestBackupStatusPresentation('overdue')).toEqual({
      color: 'text-yellow-600 dark:text-yellow-400',
      bgColor: 'bg-yellow-100 dark:bg-yellow-900/25',
      icon: 'warning',
    });
  });

  it('returns canonical guest backup tooltip copy', () => {
    expect(getWorkloadsGuestBackupTooltip('never')).toBe('No backup found');
    expect(getWorkloadsGuestBackupTooltip('stale', '3d')).toBe('Last backup: 3d');
  });

  it('keeps an unknown backup age cautionary and distinct from no backup', () => {
    expect(getWorkloadsGuestBackupStatusPresentation('unknown')).toEqual({
      color: 'text-yellow-600 dark:text-yellow-400',
      bgColor: 'bg-yellow-100 dark:bg-yellow-900/25',
      icon: 'warning',
    });
    const reason = 'Backup time unavailable: invalid timestamp.';
    expect(getWorkloadsGuestBackupTooltip('unknown', reason)).toBe(reason);
    expect(getWorkloadsGuestBackupTooltip('unknown', reason, true)).toBe(
      `Backup running now · ${reason.toLowerCase()}`,
    );
  });

  it('returns canonical compact protection context for object drawers', () => {
    expect(getWorkloadsGuestProtectionPresentation({})).toEqual({
      label: 'No completed backup found',
      tone: 'danger',
    });
    expect(
      getWorkloadsGuestProtectionPresentation({
        ageLabel: '3d ago',
        ageClass: 'text-yellow-600',
      }),
    ).toEqual({ label: '3d ago', tone: 'warning' });
    expect(
      getWorkloadsGuestProtectionPresentation({
        ageLabel: 'Today',
        ageClass: 'text-green-600',
      }),
    ).toEqual({ label: 'Today', tone: 'success' });
  });

  it('presents a running backup as its own state, keeping the completed age', () => {
    expect(getWorkloadsGuestBackupStatusPresentation('running')).toEqual({
      color: 'text-blue-600 dark:text-blue-400',
      bgColor: 'bg-blue-100 dark:bg-blue-900/25',
      icon: 'running',
    });
    // While a backup runs, the tooltip still reports the last COMPLETED
    // backup age - a started backup must never read as a finished one.
    expect(getWorkloadsGuestBackupTooltip('stale', '3d', true)).toBe(
      'Backup running now · last backup: 3d',
    );
    expect(getWorkloadsGuestBackupTooltip('never', undefined, true)).toBe(
      'Backup running now · no completed backup found',
    );
  });

  it('returns canonical guest network and disk fallback copy', () => {
    expect(getWorkloadsGuestNetworkEmptyState()).toBe('No IP assigned');
    expect(getWorkloadGuestDiskStatusMessage('no-filesystems')).toBe(
      'No filesystems found. VM may be booting or using a Live ISO.',
    );
    expect(getWorkloadGuestDiskStatusMessage()).toBe(
      'Disk stats unavailable. Guest agent may not be installed.',
    );
    expect(getWorkloadGuestDiskStatusMessage('prev-no-filesystems')).toBe(
      'Using last known disk stats. No filesystems found. VM may be booting or using a Live ISO.',
    );
  });

  it("judges a linked agent's filesystems by that agent, not the Proxmox read reason", () => {
    const agent = { diskStatusReason: 'agent-not-running', disksFromAgent: true };
    expect(getWorkloadGuestDiskRead({ ...agent, status: 'running' }, true)).toEqual({
      state: 'current',
      message: null,
      needsAction: false,
    });
    expect(
      getWorkloadGuestDiskRead({ ...agent, status: 'running', agentStale: true }, true),
    ).toEqual({
      state: 'last-known',
      message: 'Using last known disk stats. The Pulse Agent in this guest stopped reporting.',
      needsAction: false,
    });
    // A stopped guest has no current filesystems from either source, with or
    // without the Proxmox vm-stopped reason.
    for (const guest of [
      { ...agent, status: 'stopped', agentStale: true },
      { ...agent, status: 'running', diskStatusReason: 'vm-stopped' },
    ]) {
      expect(getWorkloadGuestDiskRead(guest, true)).toEqual({
        state: 'unavailable',
        message: 'Guest filesystem stats unavailable while the VM is stopped.',
        needsAction: false,
      });
    }
    expect(getWorkloadGuestDiskRead({ disksFromAgent: true, status: 'stopped' }, false)).toEqual({
      state: 'unavailable',
      message: 'Filesystem usage is unavailable.',
      needsAction: false,
    });
  });

  it('keeps Proxmox read reasons for its own guest filesystems', () => {
    expect(getWorkloadGuestDiskRead({ status: 'running' }, true).state).toBe('current');
    expect(getWorkloadGuestDiskRead({ diskStatusReason: 'prev-vm-locked' }, true)).toEqual({
      state: 'last-known',
      message: getWorkloadGuestDiskStatusMessage('prev-vm-locked'),
      needsAction: false,
    });
    expect(getWorkloadGuestDiskRead({ diskStatusReason: 'agent-not-running' }, true)).toEqual({
      state: 'unavailable',
      message: getWorkloadGuestDiskStatusMessage('agent-not-running'),
      needsAction: false,
    });
    expect(
      getWorkloadGuestDiskRead({ diskStatusReason: 'prev-agent-error' }, true).needsAction,
    ).toBe(true);
    expect(
      getWorkloadGuestDiskRead({ diskStatusReason: 'permission-denied' }, true).needsAction,
    ).toBe(true);
    // Only a VM carries a Proxmox guest read reason.
    expect(getWorkloadGuestDiskRead({ diskStatusReason: 'prev-vm-locked' }, false).state).toBe(
      'current',
    );
    // An agent's staleness says nothing about Proxmox's own filesystems.
    expect(getWorkloadGuestDiskRead({ agentStale: true }, true).state).toBe('current');
  });
});
