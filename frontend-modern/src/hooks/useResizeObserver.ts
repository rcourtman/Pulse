import { createSignal, onMount, onCleanup, type Accessor } from 'solid-js';

export interface UseResizeObserverResult {
  /** Current width of the observed element */
  width: Accessor<number>;
  /** Current height of the observed element */
  height: Accessor<number>;
  /** Ref setter for the element to observe */
  setRef: (el: HTMLElement | undefined) => void;
}

/**
 * Hook to track element dimensions using ResizeObserver.
 * Provides reactive width and height signals that update on resize.
 *
 * @param initialWidth - Initial width value (default: 100)
 * @param initialHeight - Initial height value (default: 0)
 * @returns Object with width, height signals and a ref setter
 *
 * @example
 * ```tsx
 * function MyComponent() {
 *   const { width, setRef } = useResizeObserver();
 *
 *   return (
 *     <div ref={setRef}>
 *       Width: {width()}px
 *     </div>
 *   );
 * }
 * ```
 */
export function useResizeObserver(
  initialWidth = 100,
  initialHeight = 0
): UseResizeObserverResult {
  const [width, setWidth] = createSignal(initialWidth);
  const [height, setHeight] = createSignal(initialHeight);
  let elementRef: HTMLElement | undefined;
  let observer: ResizeObserver | undefined;

  const setRef = (el: HTMLElement | undefined) => {
    elementRef = el;
  };

  onMount(() => {
    if (!elementRef) return;

    // Set initial dimensions
    setWidth(elementRef.offsetWidth);
    setHeight(elementRef.offsetHeight);

    // Create observer
    observer = new ResizeObserver((entries) => {
      for (const entry of entries) {
        setWidth(entry.contentRect.width);
        setHeight(entry.contentRect.height);
      }
    });

    observer.observe(elementRef);
  });

  onCleanup(() => {
    observer?.disconnect();
  });

  return { width, height, setRef };
}
