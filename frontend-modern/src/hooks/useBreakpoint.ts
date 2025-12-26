import { createSignal, onMount, onCleanup, createMemo, Accessor } from 'solid-js';

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

/**
 * Reactive hook for tracking viewport width and breakpoints.
 *
 * Use this for conditional rendering based on screen size,
 * which is more performant than rendering and hiding with CSS.
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
  // Initialize with current window width (or 0 for SSR)
  const initialWidth = typeof window !== 'undefined' ? window.innerWidth : 0;
  const [width, setWidth] = createSignal(initialWidth);

  onMount(() => {
    // Debounce resize events to avoid excessive re-renders
    let resizeTimeout: number | undefined;

    const handleResize = () => {
      if (resizeTimeout) {
        window.cancelAnimationFrame(resizeTimeout);
      }
      resizeTimeout = window.requestAnimationFrame(() => {
        setWidth(window.innerWidth);
      });
    };

    // Set initial width in case it changed between SSR and mount
    setWidth(window.innerWidth);

    window.addEventListener('resize', handleResize, { passive: true });

    onCleanup(() => {
      window.removeEventListener('resize', handleResize);
      if (resizeTimeout) {
        window.cancelAnimationFrame(resizeTimeout);
      }
    });
  });

  const breakpoint = createMemo(() => getBreakpointName(width()));

  const isAtLeast = (bp: Breakpoint): boolean => {
    return width() >= BREAKPOINTS[bp];
  };

  const isBelow = (bp: Breakpoint): boolean => {
    return width() < BREAKPOINTS[bp];
  };

  const isVisible = (priority: ColumnPriority): boolean => {
    const minBreakpoint = PRIORITY_BREAKPOINTS[priority];
    return width() >= BREAKPOINTS[minBreakpoint];
  };

  const isMobile = createMemo(() => width() < BREAKPOINTS.md);
  const isTablet = createMemo(() => width() >= BREAKPOINTS.md && width() < BREAKPOINTS.xl);
  const isDesktop = createMemo(() => width() >= BREAKPOINTS.xl);

  return {
    width,
    breakpoint,
    isAtLeast,
    isBelow,
    isVisible,
    isMobile,
    isTablet,
    isDesktop,
  };
}

/**
 * Get the Tailwind CSS class for hiding an element below a given breakpoint
 */
export function getVisibilityClass(priority: ColumnPriority, display: 'flex' | 'block' | 'table-cell' | 'grid' | 'inline' = 'flex'): string {
  const bp = PRIORITY_BREAKPOINTS[priority];
  if (bp === 'xs') {
    // Always visible, no hiding class needed
    return display;
  }
  return `hidden ${bp}:${display}`;
}

/**
 * Get the minimum width for a priority level
 */
export function getPriorityMinWidth(priority: ColumnPriority): number {
  return BREAKPOINTS[PRIORITY_BREAKPOINTS[priority]];
}
