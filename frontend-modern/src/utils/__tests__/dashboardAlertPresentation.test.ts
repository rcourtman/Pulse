import { describe, expect, it } from 'vitest';
import {
  DASHBOARD_ALERTS_EMPTY_STATE,
  getDashboardAlertSummaryText,
  getDashboardAlertTone,
} from '@/utils/dashboardAlertPresentation';

describe('dashboardAlertPresentation', () => {
  it('prefers danger tone when critical alerts are active', () => {
    expect(getDashboardAlertTone({ activeCritical: 2, activeWarning: 5 })).toBe('danger');
  });

  it('uses warning tone when only warnings are active', () => {
    expect(getDashboardAlertTone({ activeCritical: 0, activeWarning: 3 })).toBe('warning');
  });

  it('falls back to default tone when no alerts are active', () => {
    expect(getDashboardAlertTone({ activeCritical: 0, activeWarning: 0 })).toBe('default');
  });

  it('returns canonical summary copy', () => {
    expect(DASHBOARD_ALERTS_EMPTY_STATE).toBe('No active alerts');
    expect(getDashboardAlertSummaryText({ activeCritical: 2, activeWarning: 3 })).toBe(
      '2 critical · 3 warning',
    );
  });
});
