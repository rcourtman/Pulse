import { describe, expect, it } from 'vitest';
import { getAlertStyles } from '@/utils/alerts';
import type { Alert } from '@/types/api';

describe('getAlertStyles', () => {
  const createAlert = (overrides: Partial<Alert> = {}): Alert => ({
    id: 'alert-1',
    type: 'warning',
    level: 'warning',
    resourceId: 'resource-1',
    resourceName: 'Test Resource',
    node: 'node1',
    instance: 'qemu',
    message: 'Test alert',
    value: 80,
    threshold: 70,
    startTime: '2024-01-01T00:00:00Z',
    acknowledged: false,
    ...overrides,
  });

  const createActiveAlerts = (...alerts: Alert[]): Record<string, Alert> => {
    return alerts.reduce(
      (acc, alert) => {
        acc[alert.id] = alert;
        return acc;
      },
      {} as Record<string, Alert>,
    );
  };

  describe('when alerts are disabled', () => {
    it('returns no alert styles when alertsEnabled is false', () => {
      const alerts = createActiveAlerts(createAlert({ level: 'critical' }));
      const result = getAlertStyles('resource-1', alerts, false);

      expect(result.hasAlert).toBe(false);
      expect(result.alertCount).toBe(0);
      expect(result.severity).toBeNull();
      expect(result.rowClass).toBe('');
    });

    it('returns alert styles when alertsEnabled is undefined (defaults to enabled)', () => {
      const alerts = createActiveAlerts(createAlert({ level: 'critical' }));
      const result = getAlertStyles('resource-1', alerts, undefined);

      expect(result.hasAlert).toBe(true);
    });
  });

  describe('critical alerts', () => {
    it('returns critical styles for critical alert', () => {
      const alerts = createActiveAlerts(createAlert({ level: 'critical' }));
      const result = getAlertStyles('resource-1', alerts, true);

      expect(result.hasAlert).toBe(true);
      expect(result.alertCount).toBe(1);
      expect(result.severity).toBe('critical');
      expect(result.rowClass).toContain('bg-red-50');
      expect(result.indicatorClass).toContain('bg-red-500');
      expect(result.badgeClass).toContain('bg-red-100');
    });

    it('returns critical styles when both critical and warning alerts exist', () => {
      const alerts = createActiveAlerts(
        createAlert({ id: 'alert-1', level: 'critical' }),
        createAlert({ id: 'alert-2', level: 'warning' }),
      );
      const result = getAlertStyles('resource-1', alerts, true);

      expect(result.severity).toBe('critical');
    });
  });

  describe('warning alerts', () => {
    it('returns warning styles for warning alert', () => {
      const alerts = createActiveAlerts(createAlert({ level: 'warning' }));
      const result = getAlertStyles('resource-1', alerts, true);

      expect(result.hasAlert).toBe(true);
      expect(result.severity).toBe('warning');
      expect(result.rowClass).toContain('bg-yellow-50');
      expect(result.indicatorClass).toContain('bg-yellow-500');
      expect(result.badgeClass).toContain('bg-yellow-100');
    });
  });

  describe('alert counts', () => {
    it('counts multiple alerts for same resource', () => {
      const alerts = createActiveAlerts(
        createAlert({ id: 'alert-1', level: 'warning' }),
        createAlert({ id: 'alert-2', level: 'warning' }),
        createAlert({ id: 'alert-3', level: 'critical' }),
      );
      const result = getAlertStyles('resource-1', alerts, true);

      expect(result.alertCount).toBe(3);
    });

    it('counts unacknowledged alerts separately', () => {
      const alerts = createActiveAlerts(
        createAlert({ id: 'alert-1', acknowledged: false }),
        createAlert({ id: 'alert-2', acknowledged: true }),
        createAlert({ id: 'alert-3', acknowledged: false }),
      );
      const result = getAlertStyles('resource-1', alerts, true);

      expect(result.unacknowledgedCount).toBe(2);
      expect(result.acknowledgedCount).toBe(1);
      expect(result.hasUnacknowledgedAlert).toBe(true);
    });

    it('detects acknowledged-only alerts', () => {
      const alerts = createActiveAlerts(
        createAlert({ id: 'alert-1', acknowledged: true }),
        createAlert({ id: 'alert-2', acknowledged: true }),
      );
      const result = getAlertStyles('resource-1', alerts, true);

      expect(result.hasAcknowledgedOnlyAlert).toBe(true);
      expect(result.hasUnacknowledgedAlert).toBe(false);
    });
  });

  describe('powered-off alerts', () => {
    it('detects powered-off alert type', () => {
      const alerts = createActiveAlerts(createAlert({ id: 'alert-1', type: 'powered-off' }));
      const result = getAlertStyles('resource-1', alerts, true);

      expect(result.hasPoweredOffAlert).toBe(true);
      expect(result.hasNonPoweredOffAlert).toBe(false);
    });

    it('detects non-powered-off alert type', () => {
      const alerts = createActiveAlerts(createAlert({ id: 'alert-1', type: 'high-cpu' }));
      const result = getAlertStyles('resource-1', alerts, true);

      expect(result.hasPoweredOffAlert).toBe(false);
      expect(result.hasNonPoweredOffAlert).toBe(true);
    });

    it('detects both powered-off and non-powered-off alerts', () => {
      const alerts = createActiveAlerts(
        createAlert({ id: 'alert-1', type: 'powered-off' }),
        createAlert({ id: 'alert-2', type: 'high-cpu' }),
      );
      const result = getAlertStyles('resource-1', alerts, true);

      expect(result.hasPoweredOffAlert).toBe(true);
      expect(result.hasNonPoweredOffAlert).toBe(true);
    });
  });

  describe('resource filtering', () => {
    it('only returns alerts for specified resource', () => {
      const alerts = createActiveAlerts(
        createAlert({ id: 'alert-1', resourceId: 'resource-1', level: 'critical' }),
        createAlert({ id: 'alert-2', resourceId: 'resource-2', level: 'critical' }),
      );
      const result = getAlertStyles('resource-1', alerts, true);

      expect(result.alertCount).toBe(1);
      expect(result.severity).toBe('critical');
    });

    it('returns empty styles when no alerts for resource', () => {
      const alerts = createActiveAlerts(createAlert({ id: 'alert-1', resourceId: 'resource-2' }));
      const result = getAlertStyles('resource-1', alerts, true);

      expect(result.hasAlert).toBe(false);
      expect(result.alertCount).toBe(0);
      expect(result.severity).toBeNull();
      expect(result.rowClass).toBe('');
    });
  });

  describe('empty alerts', () => {
    it('returns empty styles for empty alerts object', () => {
      const alerts = createActiveAlerts();
      const result = getAlertStyles('resource-1', alerts, true);

      expect(result.hasAlert).toBe(false);
      expect(result.alertCount).toBe(0);
      expect(result.severity).toBeNull();
    });
  });
});
