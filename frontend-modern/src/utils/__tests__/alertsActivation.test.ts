import { beforeEach, describe, expect, it, vi } from 'vitest';
import {
  ALERTS_DETECTION_EVENT,
  isAlertsDetectionEnabled,
  setGlobalAlertsDetectionEnabled,
} from '@/utils/alertsActivation';

describe('alerts detection state', () => {
  beforeEach(() => {
    window.__pulseAlertsDetectionEnabled = null;
  });

  describe('isAlertsDetectionEnabled', () => {
    it('defaults to enabled before configuration loads', () => {
      expect(isAlertsDetectionEnabled()).toBe(true);
    });

    it('returns the explicit detection state', () => {
      setGlobalAlertsDetectionEnabled(false);
      expect(isAlertsDetectionEnabled()).toBe(false);

      setGlobalAlertsDetectionEnabled(true);
      expect(isAlertsDetectionEnabled()).toBe(true);
    });

    it('defaults to enabled when state is undefined', () => {
      window.__pulseAlertsDetectionEnabled = undefined;
      expect(isAlertsDetectionEnabled()).toBe(true);
    });
  });

  describe('setGlobalAlertsDetectionEnabled', () => {
    it('sets the global detection state', () => {
      setGlobalAlertsDetectionEnabled(false);
      expect(window.__pulseAlertsDetectionEnabled).toBe(false);
    });

    it('dispatches the detection event with the exact state', () => {
      const dispatchEventSpy = vi.spyOn(window, 'dispatchEvent');
      setGlobalAlertsDetectionEnabled(true);

      const event = dispatchEventSpy.mock.calls[0][0] as CustomEvent<boolean>;
      expect(event.type).toBe(ALERTS_DETECTION_EVENT);
      expect(event.detail).toBe(true);
    });

    it('can reset state to null', () => {
      setGlobalAlertsDetectionEnabled(false);
      setGlobalAlertsDetectionEnabled(null);
      expect(window.__pulseAlertsDetectionEnabled).toBeNull();
    });
  });
});
