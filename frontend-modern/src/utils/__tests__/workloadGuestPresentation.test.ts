import { describe, expect, it } from 'vitest';
import {
  getWorkloadsGuestBackupStatusPresentation,
  getWorkloadsGuestBackupTooltip,
  getWorkloadsGuestProtectionPresentation,
  getWorkloadGuestDiskStatusMessage,
  getWorkloadsGuestNetworkEmptyState,
} from '@/utils/workloadGuestPresentation';

describe('workloadGuestPresentation', () => {
  it('returns canonical guest backup status presentation', () => {
    expect(getWorkloadsGuestBackupStatusPresentation('fresh')).toEqual({
      color: 'text-green-600 dark:text-green-400',
      bgColor: 'bg-green-100 dark:bg-green-900',
      icon: 'check',
    });
    expect(getWorkloadsGuestBackupStatusPresentation('never')).toEqual({
      color: 'text-red-600 dark:text-red-400',
      bgColor: 'bg-red-100 dark:bg-red-900',
      icon: 'x',
    });
    expect(getWorkloadsGuestBackupStatusPresentation('overdue')).toEqual({
      color: 'text-yellow-600 dark:text-yellow-400',
      bgColor: 'bg-yellow-100 dark:bg-yellow-900',
      icon: 'warning',
    });
  });

  it('returns canonical guest backup tooltip copy', () => {
    expect(getWorkloadsGuestBackupTooltip('never')).toBe('No backup found');
    expect(getWorkloadsGuestBackupTooltip('stale', '3d')).toBe('Last backup: 3d');
  });

  it('returns canonical compact protection context for object drawers', () => {
    expect(getWorkloadsGuestProtectionPresentation({ backupInProgress: true })).toEqual({
      label: 'Backup running',
      tone: 'success',
    });
    expect(getWorkloadsGuestProtectionPresentation({})).toEqual({
      label: 'No backup found',
      tone: 'danger',
    });
    expect(
      getWorkloadsGuestProtectionPresentation({
        ageLabel: '3d ago',
        ageClass: 'text-yellow-600',
      }),
    ).toEqual({ label: '3d ago', tone: 'warning' });
  });

  it('presents a running backup as its own state, keeping the completed age', () => {
    expect(getWorkloadsGuestBackupStatusPresentation('running')).toEqual({
      color: 'text-blue-600 dark:text-blue-400',
      bgColor: 'bg-blue-100 dark:bg-blue-900',
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
});
