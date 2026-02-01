import { Component, JSX, createSignal, createEffect, onMount, onCleanup, createMemo } from 'solid-js';
import { Show } from 'solid-js';
import { useBreakpoint } from '@/hooks/useBreakpoint';

interface ScrollableTableProps {
  children: JSX.Element;
  class?: string;
  minWidth?: string;
  /** Minimum width on mobile screens (< 768px). Defaults to '100%' for natural fit. */
  mobileMinWidth?: string;
  persistKey?: string;
}

const scrollPositions = new Map<string, number>();

export const ScrollableTable: Component<ScrollableTableProps> = (props) => {
  const [showLeftFade, setShowLeftFade] = createSignal(false);
  const [showRightFade, setShowRightFade] = createSignal(false);
  let scrollContainer: HTMLDivElement | undefined;

  const { isMobile } = useBreakpoint();

  // Dynamic minWidth based on screen size
  const effectiveMinWidth = createMemo(() => {
    if (isMobile()) {
      return props.mobileMinWidth ?? '100%';
    }
    return props.minWidth || 'auto';
  });

  const checkScroll = (scrollLeftValue?: number) => {
    if (!scrollContainer) return;

    const scrollLeft = scrollLeftValue ?? scrollContainer.scrollLeft;
    const { scrollWidth, clientWidth } = scrollContainer;
    setShowLeftFade(scrollLeft > 0);
    setShowRightFade(scrollLeft < scrollWidth - clientWidth - 1);
  };

  onMount(() => {
    const initialLeft = props.persistKey ? scrollPositions.get(props.persistKey) ?? 0 : 0;
    if (scrollContainer) {
      scrollContainer.scrollLeft = initialLeft;
    }
    checkScroll(initialLeft);

    const resizeHandler = () => checkScroll();
    window.addEventListener('resize', resizeHandler);
    onCleanup(() => {
      window.removeEventListener('resize', resizeHandler);
    });
  });

  createEffect(() => {
    if (scrollContainer) {
      const handler = () => {
        if (!scrollContainer) return;
        const left = scrollContainer.scrollLeft;
        if (props.persistKey) {
          scrollPositions.set(props.persistKey, left);
        }
        checkScroll(left);
      };
      scrollContainer.addEventListener('scroll', handler, { passive: true });
      return () => scrollContainer?.removeEventListener('scroll', handler);
    }
  });

  return (
    <div class={`relative ${props.class || ''}`}>
      {/* Left fade */}
      <Show when={showLeftFade()}>
        <div class="absolute left-0 top-0 bottom-0 w-8 bg-white dark:bg-gray-800 z-10 pointer-events-none" />
      </Show>

      {/* Scrollable container */}
      <div
        ref={scrollContainer}
        class="overflow-x-auto"
        style="scrollbar-width: none; -ms-overflow-style: none; -webkit-overflow-scrolling: touch; overscroll-behavior-x: contain;"
      >
        <style>{`
          .overflow-x-auto::-webkit-scrollbar { display: none; }
        `}</style>
        <div style={{ 'min-width': effectiveMinWidth() }}>
          {props.children}
        </div>
      </div>

      {/* Right fade */}
      <Show when={showRightFade()}>
        <div class="absolute right-0 top-0 bottom-0 w-8 bg-white dark:bg-gray-800 z-10 pointer-events-none" />
      </Show>
    </div>
  );
};
