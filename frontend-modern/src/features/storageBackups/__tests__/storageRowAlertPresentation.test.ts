import { describe, expect, it } from 'vitest';
import {
  getStorageRowAlertPresentation,
} from '@/features/storageBackups/storageRowAlertPresentation';

describe('storageRowAlertPresentation', () => {
  it('returns unacknowledged critical row styling canonically', () => {
    const result = getStorageRowAlertPresentation({
      alertState: {
        hasAlert: true,
        alertCount: 1,
        severity: 'critical',
        hasUnacknowledgedAlert: true,
        unacknowledgedCount: 1,
        acknowledgedCount: 0,
        hasAcknowledgedOnlyAlert: false,
      },
      parentNodeOnline: true,
      isExpanded: false,
      isResourceHighlighted: false,
    });

    expect(result.rowClass).toContain('bg-red-50');
    expect(result.rowStyle['box-shadow']).toContain('#ef4444');
    expect(result.dataAlertState).toBe('unacknowledged');
  });

  it('returns acknowledged-only row styling canonically', () => {
    const result = getStorageRowAlertPresentation({
      alertState: {
        hasAlert: true,
        alertCount: 1,
        severity: 'warning',
        hasUnacknowledgedAlert: false,
        unacknowledgedCount: 0,
        acknowledgedCount: 1,
        hasAcknowledgedOnlyAlert: true,
      },
      parentNodeOnline: true,
      isExpanded: false,
      isResourceHighlighted: false,
    });

    expect(result.rowClass).toContain('bg-surface-alt');
    expect(result.rowStyle['box-shadow']).toContain('rgba(156, 163, 175, 0.8)');
    expect(result.dataAlertState).toBe('acknowledged');
  });
});
