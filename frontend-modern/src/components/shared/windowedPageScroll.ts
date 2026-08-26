const SCROLLABLE_OVERFLOW_PATTERN = /(?:auto|scroll|overlay)/;
const MIN_VERTICAL_SCROLL_RANGE_PX = 1;
const WHEEL_LINE_HEIGHT_PX = 16;

type WindowedPageScrollTarget = HTMLElement | Window;
type WindowedPageScrollListener = (event: Event) => void;

interface WindowedPageScrollEventsOptions {
  scrollTarget: WindowedPageScrollTarget;
  onScroll: WindowedPageScrollListener;
  onWheel: WindowedPageScrollListener;
  onResize: WindowedPageScrollListener;
}

export const findWindowedPageScrollContainer = (element: HTMLElement): HTMLElement | null => {
  let parent = element.parentElement;
  while (parent && parent !== document.body && parent !== document.documentElement) {
    const styles = getComputedStyle(parent);
    const hasVerticalScrollRange =
      parent.scrollHeight - parent.clientHeight > MIN_VERTICAL_SCROLL_RANGE_PX;
    if (
      SCROLLABLE_OVERFLOW_PATTERN.test(styles.overflowY) &&
      (styles.overflowY === 'scroll' || hasVerticalScrollRange)
    ) {
      return parent;
    }
    parent = parent.parentElement;
  }
  return null;
};

/**
 * True when a windowed surface sits in a hidden subtree (e.g. a prewarmed but
 * inactive platform tab). Hidden surfaces must skip viewport sync entirely:
 * their geometry reads would force the layout flush for mutations other
 * surfaces just made, billing an invisible table for the visible one's work.
 * checkVisibility reads computed style without forcing layout; browsers
 * without it simply keep syncing.
 */
export const isWindowedSurfaceHidden = (element: HTMLElement): boolean =>
  typeof element.checkVisibility === 'function' && !element.checkVisibility();

export const wheelDeltaInPixels = (event: WheelEvent, viewportHeight: number): number => {
  if (event.deltaMode === WheelEvent.DOM_DELTA_LINE) return event.deltaY * WHEEL_LINE_HEIGHT_PX;
  if (event.deltaMode === WheelEvent.DOM_DELTA_PAGE) return event.deltaY * viewportHeight;
  return event.deltaY;
};

/**
 * Canonical input lifecycle for virtualized content inside the native page scroller.
 *
 * Every listener is passive: the compositor must never wait on windowing work,
 * so a busy main thread degrades to a briefly delayed runway top-up instead of
 * a frozen wheel. The wheel listener exists only to prewarm the runway with the
 * projected delta in the same tick the gesture arrives.
 *
 * Touch intentionally has no listener here. The compositor moves the page first,
 * then the passive scroll event advances the keyed-row runway without allowing
 * scroll anchoring to consume the operator's gesture.
 */
export const bindWindowedPageScrollEvents = (
  options: WindowedPageScrollEventsOptions,
): (() => void) => {
  options.scrollTarget.addEventListener('scroll', options.onScroll, { passive: true });
  options.scrollTarget.addEventListener('wheel', options.onWheel, { passive: true });
  window.addEventListener('resize', options.onResize);

  return () => {
    options.scrollTarget.removeEventListener('scroll', options.onScroll);
    options.scrollTarget.removeEventListener('wheel', options.onWheel);
    window.removeEventListener('resize', options.onResize);
  };
};
