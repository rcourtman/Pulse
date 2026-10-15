import { describe, expect, it } from 'vitest';
import { guestDiskDeferrals } from '@/components/Workloads/__fixtures__/guestDiskDeferrals';
import {
  getWorkloadsGuestBackupAgeRule,
  getWorkloadsGuestBackupStatusPresentation,
  getWorkloadsGuestBackupTooltip,
  getWorkloadsGuestProtectionPresentation,
  getWorkloadGuestDiskRead,
  getWorkloadGuestDiskStatusMessage,
  getWorkloadsGuestNetworkEmptyState,
  supportsWorkloadBackupEvidence,
} from '@/utils/workloadGuestPresentation';

describe('workloadGuestPresentation', () => {
  it.each([
    { type: 'qemu' },
    { type: 'lxc' },
    { type: 'vm', platformType: 'proxmox-pve' },
    { type: 'system-container', platformScopes: ['proxmox-pve', 'docker'] },
  ])('preserves reported and legacy Proxmox backup coverage (%j)', (guest) => {
    expect(supportsWorkloadBackupEvidence(guest)).toBe(true);
  });

  it.each([
    { type: 'vm', platformType: 'vmware-vsphere' },
    { type: 'qemu', platformScopes: ['vmware-vsphere'] },
    { type: 'system-container', platformScopes: ['incus'] },
    { type: 'system-container', platformScopes: ['truenas'] },
    { type: 'app-container', platformScopes: ['proxmox-pve', 'docker'] },
    { type: 'docker' },
    { type: 'pod' },
  ])('does not infer backup absence for integrations without that evidence (%j)', (guest) => {
    expect(supportsWorkloadBackupEvidence(guest)).toBe(false);
  });

  it.each([undefined, '', 'prev-agent-busy'])(
    'does not promote a stopped VM’s retained numbers without a current read (%s)',
    (diskStatusReason) => {
      for (const disksFromAgent of [undefined, true]) {
        expect(
          getWorkloadGuestDiskRead({ status: 'stopped', diskStatusReason, disksFromAgent }, true),
        ).toEqual({
          state: 'unavailable',
          message: getWorkloadGuestDiskStatusMessage('vm-stopped'),
          needsAction: false,
        });
      }
    },
  );

  it('keeps stopped LXC platform storage separate from in-guest VM readings', () => {
    expect(getWorkloadGuestDiskRead({ status: 'stopped' }, false).state).toBe('current');
  });

  it.each(['paused', 'suspended'])(
    'does not call a %s VM filesystem report current, including a still-leased agent',
    (status) => {
      for (const disksFromAgent of [undefined, true]) {
        expect(getWorkloadGuestDiskRead({ status, disksFromAgent }, true)).toEqual({
          state: 'last-known',
          message: `Guest filesystem readings are not current while the VM is ${status}. Any displayed values are last known.`,
          needsAction: false,
        });
      }
      // Host-reported LXC storage need not stop being current with execution.
      expect(getWorkloadGuestDiskRead({ status }, false).state).toBe('current');
      // Do not replace an explicit absence/failure or its existing advice.
      expect(
        getWorkloadGuestDiskRead({ status, diskStatusReason: 'permission-denied' }, true),
      ).toEqual({
        state: 'unavailable',
        message: getWorkloadGuestDiskStatusMessage('permission-denied'),
        needsAction: true,
      });
      expect(
        getWorkloadGuestDiskRead({ status, diskStatusReason: 'prev-agent-busy' }, true).message,
      ).toBe(getWorkloadGuestDiskStatusMessage('prev-agent-busy'));
      expect(
        getWorkloadGuestDiskRead({ status, disksFromAgent: true, agentStale: true }, true).message,
      ).toContain('Pulse Agent in this guest stopped reporting');
    },
  );

  it.each(['permission-denied', 'prev-permission-denied'])(
    'keeps %s separate from permission expansion and guest health',
    (reason) => {
      const message = getWorkloadGuestDiskStatusMessage(reason);
      expect(message).toContain('If this VM is an intended monitoring target');
      expect(message).toContain("configured account and token's effective read permissions");
      expect(message).toContain('VM.Monitor (PVE 8) or VM.GuestAgent.Audit (PVE 9+)');
      expect(message).toContain(
        'Do not widen intentionally scoped access just to clear this error',
      );
      expect(message).toContain('A denied read does not establish guest or filesystem health');
      expect(message).toContain(
        'Defer access changes and live probes during backups or a guest incident',
      );
      expect(message.startsWith('Using last known disk stats.')).toBe(reason.startsWith('prev-'));
      expect(message).not.toMatch(/Administrator|VM\.GuestAgent\.(Exec|FileWrite)|curl|qm agent/);
    },
  );

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

  it('distinguishes uncertain completion and redirects from a request refused before dispatch', () => {
    const completion = getWorkloadGuestDiskStatusMessage('agent-completion-unverified');
    expect(completion).toContain('completion of the earlier request could not be verified');
    expect(completion).toContain('after the cooldown');
    const redirect = getWorkloadGuestDiskStatusMessage('agent-redirect');
    expect(redirect).toContain('Pulse did not follow it');
    expect(redirect).toContain('Completion of the original guest request is uncertain');
    for (const message of [completion, redirect]) {
      expect(message).not.toContain('This guest request was not sent');
    }
    const transport = getWorkloadGuestDiskStatusMessage('agent-transport-unverified');
    expect(transport).toContain('configured connection avoids resending guest requests');
    expect(transport).toContain('This guest request was not sent');
    expect(transport).not.toMatch(/completion.*uncertain|after the cooldown/);
    for (const message of [completion, redirect, transport]) {
      expect(message).toContain('backups or a guest incident');
      expect(message).not.toMatch(/Install|disable verification|retry now|qm agent|curl/);
    }
  });

  it.each(['agent-not-running', 'agent-disabled', 'future-private-error', undefined])(
    'keeps %s out of installation, activation and guessed OS advice',
    (reason) => {
      for (const current of [reason, `prev-${reason ?? 'unknown'}`]) {
        const text = getWorkloadGuestDiskStatusMessage(current);
        expect(text).not.toMatch(
          /Install and start|Enable it in VM Options|may not be installed|qemu-guest-agent/,
        );
        expect(text).toMatch(/backup/);
        expect(text).toMatch(/guest incident/);
        expect(text).not.toContain('future-private-error');
      }
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
      'Guest filesystem usage is unavailable. The cause is unknown. Use guest-local filesystem tools. Defer setup and live probes during backups or a guest incident.',
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
  it('names the age limit that coloured a completed backup, in days when whole', () => {
    const defaults = { freshHours: 24, staleHours: 72 };
    expect(getWorkloadsGuestBackupAgeRule('fresh', defaults)).toBe(
      'Fresh: no more than 1 day old (Fresh Hours).',
    );
    expect(getWorkloadsGuestBackupAgeRule('stale', defaults)).toBe(
      'Shown amber: more than 1 day old (Fresh Hours).',
    );
    // Overdue is amber for the same reason as stale: Fresh Hours decides the
    // colour, so raising Stale Hours would not postpone it.
    expect(getWorkloadsGuestBackupAgeRule('overdue', defaults)).toBe(
      'Shown amber: more than 1 day old (Fresh Hours). Also past Stale Hours (3 days).',
    );
    // A three-day backup cycle (#2858) sets Fresh Hours to 72.
    expect(getWorkloadsGuestBackupAgeRule('stale', { freshHours: 72, staleHours: 96 })).toBe(
      'Shown amber: more than 3 days old (Fresh Hours).',
    );
    expect(getWorkloadsGuestBackupAgeRule('fresh', { freshHours: 36, staleHours: 96 })).toBe(
      'Fresh: no more than 36 hours old (Fresh Hours).',
    );
    // Unset limits fall back to the same defaults the shield is judged by.
    expect(getWorkloadsGuestBackupAgeRule('overdue')).toBe(
      'Shown amber: more than 1 day old (Fresh Hours). Also past Stale Hours (3 days).',
    );
    expect(getWorkloadsGuestBackupAgeRule('never', defaults)).toBeNull();
    expect(getWorkloadsGuestBackupAgeRule('unknown', defaults)).toBeNull();
  });
});
