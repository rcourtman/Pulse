import { describe, expect, it, vi, beforeEach } from 'vitest';
import {
  setGlobalActivationState,
  isAlertsActivationEnabled,
  ALERTS_ACTIVATION_EVENT,
} from '@/utils/alertsActivation';

describe('alertsActivation', () => {
  beforeEach(() => {
    // Reset global state before each test
    if (typeof window !== 'undefined') {
      window.__pulseAlertsActivationState = null;
    }
  });

  describe('isAlertsActivationEnabled', () => {
    it('returns true by default in browser when no state is set', () => {
      const result = isAlertsActivationEnabled();
      expect(result).toBe(true);
    });

    it('returns true when state is active', () => {
      setGlobalActivationState('active');
      const result = isAlertsActivationEnabled();
      expect(result).toBe(true);
    });

    it('returns false when state is not active', () => {
      setGlobalActivationState('snoozed');
      const result = isAlertsActivationEnabled();
      expect(result).toBe(false);
    });

    it('returns true when state is undefined (default)', () => {
      window.__pulseAlertsActivationState = undefined;
      const result = isAlertsActivationEnabled();
      expect(result).toBe(true);
    });
  });

  describe('setGlobalActivationState', () => {
    it('sets the global activation state', () => {
      setGlobalActivationState('pending_review');
      expect(window.__pulseAlertsActivationState).toBe('pending_review');
    });

    it('dispatches custom event when state changes', () => {
      const dispatchEventSpy = vi.spyOn(window, 'dispatchEvent');
      setGlobalActivationState('active');

      expect(dispatchEventSpy).toHaveBeenCalled();
      const event = dispatchEventSpy.mock.calls[0][0] as CustomEvent;
      expect(event.type).toBe(ALERTS_ACTIVATION_EVENT);
      expect(event.detail).toBe('active');
    });

    it('can set state to null', () => {
      setGlobalActivationState('active');
      setGlobalActivationState(null);
      expect(window.__pulseAlertsActivationState).toBeNull();
    });
  });
});
