/**
 * Branch-coverage tests for alertsActivation.ts.
 *
 * Scope: ONLY `setGlobalActivationState` and `isAlertsActivationEnabled`.
 * The sibling alertsActivation.test.ts exercises the browser happy-path for
 * both functions; this file drives every remaining branch:
 *
 * - `setGlobalActivationState`:
 *   - The `if (!isBrowser) return` early-return arm (SSR), verified by
 *     reloading the module while `window` is undefined and asserting that
 *     `window.__pulseAlertsActivationState` is left untouched and no event
 *     is dispatched.
 *   - The browser dispatch arm for every value of `ActivationState | null`,
 *     asserting each dispatched event is a genuine `CustomEvent` instance,
 *     its `type` equals `ALERTS_ACTIVATION_EVENT`, and its `detail` equals
 *     the exact value passed (including `null`), and that each call
 *     dispatches a fresh event with no dedup.
 *
 * - `isAlertsActivationEnabled`:
 *   - The `if (!isBrowser) return true` SSR arm.
 *   - The `state === 'pending_review'` case flowing through the
 *     `return state === 'active'` false arm (sibling test only covers
 *     'snoozed' for that arm).
 *   - The `state === undefined || state === null` early-true arm via both
 *     inputs independently.
 *
 * The SSR arms are reached with `vi.resetModules()` + a dynamic `import()`
 * while `window` is stubbed to `undefined`, so the module-level `isBrowser`
 * const is re-evaluated to `false`. The stub is always cleared via
 * `vi.unstubAllGlobals()` in `afterEach` (and inside `finally` blocks).
 */
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import type { ActivationState } from '@/types/alerts';
import {
  ALERTS_ACTIVATION_EVENT,
  isAlertsActivationEnabled,
  setGlobalActivationState,
} from '@/utils/alertsActivation';

describe('setGlobalActivationState (branch coverage)', () => {
  beforeEach(() => {
    window.__pulseAlertsActivationState = null;
  });

  afterEach(() => {
    vi.restoreAllMocks();
    vi.unstubAllGlobals();
  });

  it('writes the exact value to window.__pulseAlertsActivationState for every ActivationState and null', () => {
    const cases: Array<ActivationState | null> = [
      'pending_review',
      'active',
      'snoozed',
      null,
    ];
    for (const c of cases) {
      setGlobalActivationState(c);
      expect(window.__pulseAlertsActivationState).toBe(c);
    }
  });

  it('dispatches a fresh genuine CustomEvent whose detail equals each value passed (all ActivationState and null)', () => {
    const dispatchSpy = vi.spyOn(window, 'dispatchEvent');
    const seen: Array<{
      type: string;
      detail: ActivationState | null;
      isCustomEvent: boolean;
    }> = [];
    dispatchSpy.mockImplementation((event: Event) => {
      seen.push({
        type: event.type,
        detail: (event as CustomEvent<ActivationState | null>).detail,
        isCustomEvent: event instanceof CustomEvent,
      });
      return true;
    });

    const cases: Array<ActivationState | null> = [
      'pending_review',
      'active',
      'snoozed',
      null,
    ];
    for (const c of cases) {
      setGlobalActivationState(c);
    }

    expect(seen).toStrictEqual(
      cases.map((c) => ({
        type: ALERTS_ACTIVATION_EVENT,
        detail: c,
        isCustomEvent: true,
      })),
    );
    // No dedup: exactly one dispatch per call.
    expect(dispatchSpy).toHaveBeenCalledTimes(cases.length);
  });

  it('early-returns in an SSR context (window undefined at module load) without mutating state or dispatching', async () => {
    // Sentinel: the SSR call must NOT overwrite this.
    window.__pulseAlertsActivationState = 'active';
    const dispatchSpy = vi.spyOn(window, 'dispatchEvent');
    const callsBefore = dispatchSpy.mock.calls.length;

    vi.resetModules();
    vi.stubGlobal('window', undefined);
    try {
      // Fresh module instance: module-level `isBrowser` is re-evaluated to
      // `false` because `typeof window === 'undefined'` at load time.
      const mod = await import('@/utils/alertsActivation');
      mod.setGlobalActivationState(null);
      mod.setGlobalActivationState('snoozed');
    } finally {
      vi.unstubAllGlobals();
    }

    // window is restored: sentinel must be unchanged, and no new dispatch
    // may have occurred during the SSR calls.
    expect(window.__pulseAlertsActivationState).toBe('active');
    expect(dispatchSpy.mock.calls.length).toBe(callsBefore);
  });
});

describe('isAlertsActivationEnabled (branch coverage)', () => {
  beforeEach(() => {
    window.__pulseAlertsActivationState = null;
  });

  afterEach(() => {
    vi.restoreAllMocks();
    vi.unstubAllGlobals();
  });

  it('returns false for pending_review (false arm of `state === "active"`, not exercised by sibling test)', () => {
    setGlobalActivationState('pending_review');
    expect(isAlertsActivationEnabled()).toBe(false);
  });

  it('returns false for snoozed (false arm of `state === "active"`)', () => {
    setGlobalActivationState('snoozed');
    expect(isAlertsActivationEnabled()).toBe(false);
  });

  it('returns true for active (true arm of `state === "active"`)', () => {
    setGlobalActivationState('active');
    expect(isAlertsActivationEnabled()).toBe(true);
  });

  it('returns true when window.__pulseAlertsActivationState is explicitly undefined (left || operand true)', () => {
    window.__pulseAlertsActivationState = undefined;
    expect(isAlertsActivationEnabled()).toBe(true);
  });

  it('returns true when window.__pulseAlertsActivationState is null (right || operand true)', () => {
    window.__pulseAlertsActivationState = null;
    expect(isAlertsActivationEnabled()).toBe(true);
  });

  it('returns true in an SSR context (window undefined at module load) via the !isBrowser early return', async () => {
    vi.resetModules();
    vi.stubGlobal('window', undefined);
    try {
      const mod = await import('@/utils/alertsActivation');
      expect(mod.isAlertsActivationEnabled()).toBe(true);
    } finally {
      vi.unstubAllGlobals();
    }
  });
});
