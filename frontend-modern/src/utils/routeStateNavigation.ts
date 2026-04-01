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

const isSamePathnameNavigation = (currentPath: string, targetPath: string): boolean => {
  const base = window.location.origin;
  return new URL(currentPath, base).pathname === new URL(targetPath, base).pathname;
};

export const createRouteStateNavigateScheduler = (
  navigate: (path: string, options: typeof ROUTE_STATE_REPLACE_OPTIONS) => void,
  readCurrentPath: () => string,
) => {
  let pendingHandle: number | null = null;
  let pendingPath: string | null = null;

  const schedule = (nextPath: string) => {
    pendingPath = nextPath;
    if (pendingHandle !== null) return;

    pendingHandle = window.setTimeout(() => {
      pendingHandle = null;
      const target = pendingPath;
      pendingPath = null;
      if (!target) return;
      const currentPath = readCurrentPath();
      if (currentPath === target) return;

      const restoreScroll = isSamePathnameNavigation(currentPath, target)
        ? { x: window.scrollX, y: window.scrollY }
        : null;

      if (restoreScroll) {
        const previousScrollRestoration = window.history.scrollRestoration;
        window.history.scrollRestoration = 'manual';
        const applyScrollRestore = () => {
          if (/jsdom/i.test(window.navigator.userAgent)) return;
          if (routeStateDeliberateScrollSuppressedUntil > Date.now()) {
            return;
          }
          if (
            Math.abs(window.scrollX - restoreScroll.x) > ROUTE_STATE_SCROLL_RESTORE_DIVERGENCE_PX ||
            Math.abs(window.scrollY - restoreScroll.y) > ROUTE_STATE_SCROLL_RESTORE_DIVERGENCE_PX
          ) {
            return;
          }
          window.scrollTo(restoreScroll.x, restoreScroll.y);
        };
        navigate(target, ROUTE_STATE_REPLACE_OPTIONS);
        applyScrollRestore();
        window.setTimeout(applyScrollRestore, 0);
        window.requestAnimationFrame(() => {
          applyScrollRestore();
          window.requestAnimationFrame(() => {
            applyScrollRestore();
          });
        });
        window.setTimeout(() => {
          applyScrollRestore();
          window.history.scrollRestoration = previousScrollRestoration;
        }, 120);
        return;
      }

      navigate(target, ROUTE_STATE_REPLACE_OPTIONS);
    }, 0);
  };

  const cleanup = () => {
    if (pendingHandle === null) return;
    window.clearTimeout(pendingHandle);
    pendingHandle = null;
    pendingPath = null;
  };

  return { cleanup, schedule };
};
