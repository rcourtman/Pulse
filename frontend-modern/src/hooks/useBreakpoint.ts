import { createSignal, onMount, onCleanup, createMemo, createRoot, Accessor } from 'solid-js';

/**
 * Tailwind CSS breakpoint values (in pixels)
 * These match Tailwind's default breakpoints
 */
export const BREAKPOINTS = {
  xs: 400,
  sm: 640,
  md: 768,
  lg: 1024,
  xl: 1280,
  '2xl': 1536,
} as const;

export type Breakpoint = keyof typeof BREAKPOINTS;

/**
 * Column priority tiers mapped to breakpoints
 * - essential: Always visible (xs and up)
 * - primary: Visible on small screens and up (sm: 640px+)
 * - secondary: Visible on medium screens and up (md: 768px+)
 * - supplementary: Visible on large screens and up (lg: 1024px+)
 * - detailed: Visible on extra large screens and up (xl: 1280px+)
 */
export type ColumnPriority = 'essential' | 'primary' | 'secondary' | 'supplementary' | 'detailed';

export const PRIORITY_BREAKPOINTS: Record<ColumnPriority, Breakpoint> = {
  essential: 'xs',
  primary: 'sm',
  secondary: 'md',
  supplementary: 'lg',
  detailed: 'xl',
};

/**
 * Get the current breakpoint name based on window width
 */
function getBreakpointName(width: number): Breakpoint {
  if (width >= BREAKPOINTS['2xl']) return '2xl';
  if (width >= BREAKPOINTS.xl) return 'xl';
  if (width >= BREAKPOINTS.lg) return 'lg';
  if (width >= BREAKPOINTS.md) return 'md';
  if (width >= BREAKPOINTS.sm) return 'sm';
  return 'xs';
}

export interface UseBreakpointReturn {
  /** Current window width in pixels */
  width: Accessor<number>;
  /** Current breakpoint name (xs, sm, md, lg, xl, 2xl) */
  breakpoint: Accessor<Breakpoint>;
  /** Check if current width is at least the given breakpoint */
  isAtLeast: (bp: Breakpoint) => boolean;
  /** Check if current width is below the given breakpoint */
  isBelow: (bp: Breakpoint) => boolean;
  /** Check if a column with the given priority should be visible */
  isVisible: (priority: ColumnPriority) => boolean;
  /** Convenience booleans for common checks */
  isMobile: Accessor<boolean>;
  isTablet: Accessor<boolean>;
  isDesktop: Accessor<boolean>;
}

const readViewportWidth = (): number => (typeof window !== 'undefined' ? window.innerWidth : 0);

/**
 * One viewport width shared by every useBreakpoint() caller. Workload rows call
 * the hook once or twice each, so a per-call listener made every window resize
 * run one callback per row. A single rAF-debounced listener feeds this signal
 * while at least one caller is mounted.
 */
const [viewportWidth, setViewportWidth] = createSignal(readViewportWidth());
let mountedCallers = 0;
let resizeFrame: number | undefined;

const handleViewportResize = () => {
  if (resizeFrame !== undefined) {
    window.cancelAnimationFrame(resizeFrame);
  }
  resizeFrame = window.requestAnimationFrame(() => {
    resizeFrame = undefined;
    setViewportWidth(window.innerWidth);
  });
};

function retainViewportWidthListener(): () => void {
  setViewportWidth(window.innerWidth);
  mountedCallers += 1;
  if (mountedCallers === 1) {
    window.addEventListener('resize', handleViewportResize, { passive: true });
  }

  return () => {
    mountedCallers -= 1;
    if (mountedCallers > 0) return;
    window.removeEventListener('resize', handleViewportResize);
    if (resizeFrame !== undefined) {
      window.cancelAnimationFrame(resizeFrame);
      resizeFrame = undefined;
    }
  };
}

const sharedBreakpoint = createRoot((): UseBreakpointReturn => {
  const breakpoint = createMemo(() => getBreakpointName(viewportWidth()));

  const isAtLeast = (bp: Breakpoint): boolean => {
    return viewportWidth() >= BREAKPOINTS[bp];
  };

  const isBelow = (bp: Breakpoint): boolean => {
    return viewportWidth() < BREAKPOINTS[bp];
  };

  const isVisible = (priority: ColumnPriority): boolean => {
    const minBreakpoint = PRIORITY_BREAKPOINTS[priority];
    return viewportWidth() >= BREAKPOINTS[minBreakpoint];
  };

  const isMobile = createMemo(() => viewportWidth() < BREAKPOINTS.md);
  const isTablet = createMemo(
    () => viewportWidth() >= BREAKPOINTS.md && viewportWidth() < BREAKPOINTS.xl,
  );
  const isDesktop = createMemo(() => viewportWidth() >= BREAKPOINTS.xl);

  return {
    width: viewportWidth,
    breakpoint,
    isAtLeast,
    isBelow,
    isVisible,
    isMobile,
    isTablet,
    isDesktop,
  };
});

/**
 * Reactive hook for tracking viewport width and breakpoints.
 *
 * Use this for conditional rendering based on screen size,
 * which is more performant than rendering and hiding with CSS.
 * Every caller reads the same shared width; mounting a caller never adds
 * another window resize listener.
 *
 * @example
 * ```tsx
 * const { isAtLeast, isMobile, isVisible } = useBreakpoint();
 *
 * return (
 *   <Show when={isAtLeast('md')} fallback={<MobileView />}>
 *     <DesktopView />
 *   </Show>
 * );
 * ```
 */
export function useBreakpoint(): UseBreakpointReturn {
  // The shared width goes stale while no caller is mounted (and for up to a
  // frame after a resize), so a new caller starts from the live width.
  setViewportWidth(readViewportWidth());

  onMount(() => {
    onCleanup(retainViewportWidthListener());
  });

  return sharedBreakpoint;
}
