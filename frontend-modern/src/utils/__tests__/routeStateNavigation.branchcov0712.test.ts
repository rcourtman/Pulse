import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import {
  ROUTE_STATE_REPLACE_OPTIONS,
  createRouteStateNavigateScheduler,
} from '@/utils/routeStateNavigation';
import {
  clearPendingAppShellRestoreTop,
  readPendingAppShellRestoreTop,
} from '@/utils/appShellScrollRestoration';

describe('routeStateNavigation – branch coverage (schedule / clearScrollRestoreWork / applyScrollRestore)', () => {
  const scrollToSpy = vi.fn();
  let currentScrollX = 24;
  let currentScrollY = 320;

  beforeEach(() => {
    vi.useFakeTimers();
    clearPendingAppShellRestoreTop();
    scrollToSpy.mockReset();
    currentScrollX = 24;
    currentScrollY = 320;
    Object.defineProperty(window, 'scrollX', {
      configurable: true,
      get: () => currentScrollX,
    });
    Object.defineProperty(window, 'scrollY', {
      configurable: true,
      get: () => currentScrollY,
    });
    // Default to a non-jsdom UA so applyScrollRestore reaches window.scrollTo;
    // the jsdom-guard test overrides this locally.
    Object.defineProperty(window.navigator, 'userAgent', {
      configurable: true,
      value: 'Mozilla/5.0',
    });
    Object.defineProperty(window.history, 'scrollRestoration', {
      configurable: true,
      value: 'auto',
      writable: true,
    });
    scrollToSpy.mockImplementation((x: number, y: number) => {
      currentScrollX = x;
      currentScrollY = y;
    });
    window.scrollTo = scrollToSpy as typeof window.scrollTo;
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  describe('schedule', () => {
    it('returns early when the pending target is an empty string (!target branch)', () => {
      const navigate = vi.fn();
      const scheduler = createRouteStateNavigateScheduler(navigate, () => '/proxmox/backups');

      scheduler.schedule('');
      vi.runAllTimers();

      expect(navigate).not.toHaveBeenCalled();
      expect(scrollToSpy).not.toHaveBeenCalled();
      scheduler.cleanup();
    });

    it('fires a plain replace without scroll restoration when the target is a different pathname (restoreScroll null arm)', () => {
      const navigate = vi.fn();
      // currentPath stays on the scheduling pathname so the stale-pathname
      // guard does not drop the rewrite, but the target pathname differs —
      // driving the `isSamePathnameNavigation(...) ? {...} : null` ternary
      // into its null arm and reaching the bare navigate() at the bottom.
      const scheduler = createRouteStateNavigateScheduler(navigate, () => '/proxmox/storage');

      scheduler.schedule('/proxmox/overview?type=vm');
      vi.runAllTimers();

      expect(navigate).toHaveBeenCalledWith(
        '/proxmox/overview?type=vm',
        ROUTE_STATE_REPLACE_OPTIONS,
      );
      expect(scrollToSpy).not.toHaveBeenCalled();
      // scrollRestoration is only flipped to 'manual' inside the restoreScroll
      // block, so the null arm leaves it untouched.
      expect(window.history.scrollRestoration).toBe('auto');
      scheduler.cleanup();
    });

    it('does not capture app-shell scroll when the shell is present but already at the top', () => {
      const navigate = vi.fn();
      let currentPath = '/proxmox/overview?type=app-container';
      const scheduler = createRouteStateNavigateScheduler(navigate, () => currentPath);
      const shell = document.createElement('div');
      shell.className = 'app-scroll-shell';
      shell.scrollTop = 0;
      document.body.appendChild(shell);

      scheduler.schedule('/proxmox/overview?type=app-container&resource=agent-1');
      vi.runAllTimers();

      // shell is truthy but `shell.scrollTop > 0` is false → short-circuits past
      // schedulePendingAppShellRestoreTop.
      expect(readPendingAppShellRestoreTop()).toBe(null);

      shell.remove();
      scheduler.cleanup();
    });
  });

  describe('clearScrollRestoreWork (exercised via cleanup)', () => {
    it('clears pending scroll-restore timeouts and animation frames registered by a same-path rewrite', () => {
      const navigate = vi.fn();
      let currentPath = '/proxmox/overview?type=app-container';
      const scheduler = createRouteStateNavigateScheduler(navigate, () => currentPath);

      // Capture the exact animation-frame handle the scheduler registers so we
      // can assert clearScrollRestoreWork cancels that precise handle. Vitest's
      // fake timers hand back Node Timeout-like opaque handles (not plain
      // numbers), so we compare by identity rather than by numeric type.
      const registeredRafHandles: unknown[] = [];
      const underlyingRaf = window.requestAnimationFrame.bind(window);
      vi.spyOn(window, 'requestAnimationFrame').mockImplementation((cb: FrameRequestCallback) => {
        const handle = underlyingRaf(cb);
        registeredRafHandles.push(handle);
        return handle;
      });

      scheduler.schedule('/proxmox/overview?type=app-container&resource=agent-9');
      // Fire the outer setTimeout(0) only. This runs the restoreScroll block,
      // registering two scroll-restore timeouts (delay 0 and 120) and a rAF
      // chain, but — per the coalescing semantics — does not yet flush the
      // inner 0ms timeout. The immediate applyScrollRestore fires once.
      vi.advanceTimersByTime(0);
      expect(scrollToSpy).toHaveBeenCalledTimes(1);
      expect(registeredRafHandles).toHaveLength(1);

      const clearTimeoutSpy = vi.spyOn(window, 'clearTimeout');
      const cancelAnimSpy = vi.spyOn(window, 'cancelAnimationFrame');

      scheduler.cleanup();

      // clearScrollRestoreWork iterates the two pending timeout handles
      // (the `typeof window !== 'undefined'` → window.clearTimeout arm) and
      // cancels the pending animation frame (the window.cancelAnimationFrame arm),
      // passing the exact handle that requestAnimationFrame returned.
      expect(clearTimeoutSpy.mock.calls).toHaveLength(2);
      expect(clearTimeoutSpy.mock.calls[0]).toHaveLength(1);
      expect(clearTimeoutSpy.mock.calls[1]).toHaveLength(1);
      expect(cancelAnimSpy.mock.calls).toHaveLength(1);
      expect(cancelAnimSpy.mock.calls[0][0]).toBe(registeredRafHandles[0]);

      // The deferred scroll-restore work was cancelled, so flushing the
      // remaining timers produces no further scrollTo calls.
      vi.runAllTimers();
      expect(scrollToSpy).toHaveBeenCalledTimes(1);
    });
  });

  describe('applyScrollRestore', () => {
    it('skips window.scrollTo when running under a jsdom user agent (jsdom guard branch)', () => {
      const navigate = vi.fn();
      let currentPath = '/proxmox/overview?type=app-container';
      const scheduler = createRouteStateNavigateScheduler(navigate, () => currentPath);

      Object.defineProperty(window.navigator, 'userAgent', {
        configurable: true,
        value: 'Mozilla/5.0 (linux) AppleWebKit/537.36 (KHTML, like Gecko) jsdom/24.1.0',
      });

      scheduler.schedule('/proxmox/overview?type=app-container&resource=guest-7');
      vi.runAllTimers();

      // navigate still fires (it happens before applyScrollRestore), but every
      // applyScrollRestore invocation returns early at the jsdom guard.
      expect(navigate).toHaveBeenCalledWith(
        '/proxmox/overview?type=app-container&resource=guest-7',
        ROUTE_STATE_REPLACE_OPTIONS,
      );
      expect(scrollToSpy).not.toHaveBeenCalled();
      scheduler.cleanup();
    });
  });
});
