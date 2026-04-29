import { describe, expect, it } from 'vitest';
import {
  getWorkloadsGuestBackupStatusPresentation,
  getWorkloadsGuestBackupTooltip,
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
      color: 'text-muted',
      bgColor: 'bg-surface-alt',
      icon: 'x',
    });
  });

  it('returns canonical guest backup tooltip copy', () => {
    expect(getWorkloadsGuestBackupTooltip('never')).toBe('No backup found');
    expect(getWorkloadsGuestBackupTooltip('stale', '3d')).toBe('Last backup: 3d');
  });

  it('returns canonical guest network and disk fallback copy', () => {
    expect(getWorkloadsGuestNetworkEmptyState()).toBe('No IP assigned');
    expect(getWorkloadGuestDiskStatusMessage('no-filesystems')).toBe(
      'No filesystems found. VM may be booting or using a Live ISO.',
    );
    expect(getWorkloadGuestDiskStatusMessage()).toBe(
      'Disk stats unavailable. Guest agent may not be installed.',
    );
  });
});
