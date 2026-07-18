import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import {
  ALERTS_DETECTION_EVENT,
  isAlertsDetectionEnabled,
  setGlobalAlertsDetectionEnabled,
} from '@/utils/alertsActivation';

describe('setGlobalAlertsDetectionEnabled branch coverage', () => {
  beforeEach(() => {
    window.__pulseAlertsDetectionEnabled = null;
  });

  afterEach(() => {
    vi.restoreAllMocks();
    vi.unstubAllGlobals();
  });

  it('dispatches a fresh CustomEvent for true, false, and null', () => {
    const dispatchSpy = vi.spyOn(window, 'dispatchEvent');
    const seen: Array<{ type: string; detail: boolean | null; isCustomEvent: boolean }> = [];
    dispatchSpy.mockImplementation((event: Event) => {
      seen.push({
        type: event.type,
        detail: (event as CustomEvent<boolean | null>).detail,
        isCustomEvent: event instanceof CustomEvent,
      });
      return true;
    });

    const cases: Array<boolean | null> = [true, false, null];
    for (const value of cases) {
      setGlobalAlertsDetectionEnabled(value);
      expect(window.__pulseAlertsDetectionEnabled).toBe(value);
    }

    expect(seen).toStrictEqual(
      cases.map((detail) => ({
        type: ALERTS_DETECTION_EVENT,
        detail,
        isCustomEvent: true,
      })),
    );
    expect(dispatchSpy).toHaveBeenCalledTimes(cases.length);
  });

  it('does not mutate browser state in an SSR module instance', async () => {
    window.__pulseAlertsDetectionEnabled = false;
    const dispatchSpy = vi.spyOn(window, 'dispatchEvent');
    const callsBefore = dispatchSpy.mock.calls.length;

    vi.resetModules();
    vi.stubGlobal('window', undefined);
    try {
      const mod = await import('@/utils/alertsActivation');
      mod.setGlobalAlertsDetectionEnabled(true);
      mod.setGlobalAlertsDetectionEnabled(null);
    } finally {
      vi.unstubAllGlobals();
    }

    expect(window.__pulseAlertsDetectionEnabled).toBe(false);
    expect(dispatchSpy.mock.calls.length).toBe(callsBefore);
  });
});

describe('isAlertsDetectionEnabled branch coverage', () => {
  beforeEach(() => {
    window.__pulseAlertsDetectionEnabled = null;
  });

  afterEach(() => {
    vi.restoreAllMocks();
    vi.unstubAllGlobals();
  });

  it('returns false only for an explicit disabled state', () => {
    setGlobalAlertsDetectionEnabled(false);
    expect(isAlertsDetectionEnabled()).toBe(false);
  });

  it('returns true for explicit enabled, undefined, and null states', () => {
    setGlobalAlertsDetectionEnabled(true);
    expect(isAlertsDetectionEnabled()).toBe(true);

    window.__pulseAlertsDetectionEnabled = undefined;
    expect(isAlertsDetectionEnabled()).toBe(true);

    window.__pulseAlertsDetectionEnabled = null;
    expect(isAlertsDetectionEnabled()).toBe(true);
  });

  it('returns true in an SSR module instance', async () => {
    vi.resetModules();
    vi.stubGlobal('window', undefined);
    try {
      const mod = await import('@/utils/alertsActivation');
      expect(mod.isAlertsDetectionEnabled()).toBe(true);
    } finally {
      vi.unstubAllGlobals();
    }
  });
});
