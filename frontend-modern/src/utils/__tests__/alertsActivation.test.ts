import { describe, expect, it, vi, beforeEach, afterEach } from 'vitest';
import {
  setGlobalActivationState,
  isAlertsActivationEnabled,
  ALERTS_ACTIVATION_EVENT,
} from '@/utils/alertsActivation';
import type { ActivationState } from '@/types/alerts';

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

    it('returns false when state is inactive', () => {
      setGlobalActivationState('inactive');
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
      setGlobalActivationState('inactive');
      expect(window.__pulseAlertsActivationState).toBe('inactive');
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
