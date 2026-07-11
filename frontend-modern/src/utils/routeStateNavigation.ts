import { schedulePendingAppShellRestoreTop } from '@/utils/appShellScrollRestoration';

export const ROUTE_STATE_REPLACE_OPTIONS = {
  replace: true,
  scroll: false,
} as const;

const ROUTE_STATE_SCROLL_RESTORE_DIVERGENCE_PX = 24;
const ROUTE_STATE_DELIBERATE_SCROLL_SUPPRESSION_MS = 1000;
let routeStateDeliberateScrollSuppressedUntil = 0;

export const markRouteStateDeliberateScroll = (now = Date.now()): void => {
  routeStateDeliberateScrollSuppressedUntil = Math.max(
    routeStateDeliberateScrollSuppressedUntil,
    now + ROUTE_STATE_DELIBERATE_SCROLL_SUPPRESSION_MS,
  );
};

const resolvePathname = (path: string): string =>
  new URL(path, window.location.origin).pathname;

const isSamePathnameNavigation = (currentPath: string, targetPath: string): boolean =>
  resolvePathname(currentPath) === resolvePathname(targetPath);

const shouldRestorePreservedScrollValue = (current: number, restore: number): boolean => {
  if (Math.abs(current - restore) <= ROUTE_STATE_SCROLL_RESTORE_DIVERGENCE_PX) {
    return true;
  }
  return restore > ROUTE_STATE_SCROLL_RESTORE_DIVERGENCE_PX && current === 0;
};

export const createRouteStateNavigateScheduler = (
  navigate: (path: string, options: typeof ROUTE_STATE_REPLACE_OPTIONS) => void,
  readCurrentPath: () => string,
) => {
  let pendingHandle: number | null = null;
  let pendingPath: string | null = null;
  let pendingSchedulingPathname: string | null = null;
  let scrollRestoreTimeoutHandles: number[] = [];
  let scrollRestoreFrameHandles: number[] = [];

  const clearScrollRestoreWork = () => {
    scrollRestoreTimeoutHandles.forEach((handle) => {
      if (typeof window !== 'undefined') {
        window.clearTimeout(handle);
      } else {
        clearTimeout(handle);
      }
    });
    scrollRestoreTimeoutHandles = [];

    if (typeof window !== 'undefined') {
      scrollRestoreFrameHandles.forEach((handle) => window.cancelAnimationFrame(handle));
    }
    scrollRestoreFrameHandles = [];
  };

  const scheduleScrollRestoreTimeout = (callback: () => void, delay: number) => {
    const handle = window.setTimeout(() => {
      scrollRestoreTimeoutHandles = scrollRestoreTimeoutHandles.filter(
        (candidate) => candidate !== handle,
      );
      callback();
    }, delay);
    scrollRestoreTimeoutHandles.push(handle);
  };

  const scheduleScrollRestoreFrame = (callback: () => void) => {
    const handle = window.requestAnimationFrame(() => {
      scrollRestoreFrameHandles = scrollRestoreFrameHandles.filter(
        (candidate) => candidate !== handle,
      );
      callback();
    });
    scrollRestoreFrameHandles.push(handle);
  };

  const schedule = (nextPath: string) => {
    pendingPath = nextPath;
    pendingSchedulingPathname = resolvePathname(readCurrentPath());
    if (pendingHandle !== null) return;

    pendingHandle = window.setTimeout(() => {
      pendingHandle = null;
      if (typeof window === 'undefined') {
        pendingPath = null;
        pendingSchedulingPathname = null;
        return;
      }
      const target = pendingPath;
      const schedulingPathname = pendingSchedulingPathname;
      pendingPath = null;
      pendingSchedulingPathname = null;
      if (!target) return;
      const currentPath = readCurrentPath();
      if (currentPath === target) return;
      // Route-state rewrites only manage the query string of the surface that
      // scheduled them. If the pathname changed between scheduling and firing
      // (the user clicked to another section while this rewrite was pending),
      // the scheduling surface no longer owns the URL — replaying the stale
      // target would swallow the user's navigation, so drop it instead.
      if (schedulingPathname !== null && resolvePathname(currentPath) !== schedulingPathname) {
        return;
      }

      const restoreScroll = isSamePathnameNavigation(currentPath, target)
        ? { x: window.scrollX, y: window.scrollY }
        : null;

      if (restoreScroll) {
        clearScrollRestoreWork();
        const previousScrollRestoration = window.history.scrollRestoration;
        window.history.scrollRestoration = 'manual';
        const shell = document.querySelector<HTMLElement>('.app-scroll-shell');
        if (shell && shell.scrollTop > 0) {
          schedulePendingAppShellRestoreTop(shell.scrollTop);
        }
        const applyScrollRestore = () => {
          if (typeof window === 'undefined') return;
          if (/jsdom/i.test(window.navigator.userAgent)) return;
          if (routeStateDeliberateScrollSuppressedUntil > Date.now()) {
            return;
          }
          if (
            !shouldRestorePreservedScrollValue(window.scrollX, restoreScroll.x) ||
            !shouldRestorePreservedScrollValue(window.scrollY, restoreScroll.y)
          ) {
            return;
          }
          window.scrollTo(restoreScroll.x, restoreScroll.y);
        };
        navigate(target, ROUTE_STATE_REPLACE_OPTIONS);
        applyScrollRestore();
        scheduleScrollRestoreTimeout(applyScrollRestore, 0);
        scheduleScrollRestoreFrame(() => {
          applyScrollRestore();
          scheduleScrollRestoreFrame(() => {
            applyScrollRestore();
          });
        });
        scheduleScrollRestoreTimeout(() => {
          applyScrollRestore();
          if (typeof window === 'undefined') return;
          window.history.scrollRestoration = previousScrollRestoration;
        }, 120);
        return;
      }

      navigate(target, ROUTE_STATE_REPLACE_OPTIONS);
    }, 0);
  };

  const cleanup = () => {
    clearScrollRestoreWork();
    if (pendingHandle !== null) {
      window.clearTimeout(pendingHandle);
      pendingHandle = null;
    }
    pendingPath = null;
    pendingSchedulingPathname = null;
  };

  return { cleanup, schedule };
};
