import { describe, expect, it } from 'vitest';
import {
  DASHBOARD_RECOVERY_EMPTY_STATE,
  DASHBOARD_RECOVERY_STALE_MESSAGE,
} from '@/utils/dashboardRecoveryPresentation';

describe('dashboardRecoveryPresentation', () => {
  it('returns canonical recovery dashboard copy', () => {
    expect(DASHBOARD_RECOVERY_EMPTY_STATE).toBe('No recovery data available');
    expect(DASHBOARD_RECOVERY_STALE_MESSAGE).toBe('Last recovery point over 24 hours ago');
  });
});
