import { describe, expect, it } from 'vitest';
import {
  getDashboardGuestBackupStatusPresentation,
  getDashboardGuestBackupTooltip,
  getDashboardGuestDiskStatusMessage,
  getDashboardGuestNetworkEmptyState,
} from '@/utils/dashboardGuestPresentation';

describe('dashboardGuestPresentation', () => {
  it('returns canonical guest backup status presentation', () => {
    expect(getDashboardGuestBackupStatusPresentation('fresh')).toEqual({
      color: 'text-green-600 dark:text-green-400',
      bgColor: 'bg-green-100 dark:bg-green-900',
      icon: 'check',
    });
    expect(getDashboardGuestBackupStatusPresentation('never')).toEqual({
      color: 'text-muted',
      bgColor: 'bg-surface-alt',
      icon: 'x',
    });
  });

  it('returns canonical guest backup tooltip copy', () => {
    expect(getDashboardGuestBackupTooltip('never')).toBe('No backup found');
    expect(getDashboardGuestBackupTooltip('stale', '3d')).toBe('Last backup: 3d');
  });

  it('returns canonical guest network and disk fallback copy', () => {
    expect(getDashboardGuestNetworkEmptyState()).toBe('No IP assigned');
    expect(getDashboardGuestDiskStatusMessage('no-filesystems')).toBe(
      'No filesystems found. VM may be booting or using a Live ISO.',
    );
    expect(getDashboardGuestDiskStatusMessage()).toBe(
      'Disk stats unavailable. Guest agent may not be installed.',
    );
  });
});
