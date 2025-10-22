import { Component, JSX, createSignal, createEffect, onMount, onCleanup } from 'solid-js';
import { Show } from 'solid-js';

interface ScrollableTableProps {
  children: JSX.Element;
  class?: string;
  minWidth?: string;
}

export const ScrollableTable: Component<ScrollableTableProps> = (props) => {
  const [showLeftFade, setShowLeftFade] = createSignal(false);
  const [showRightFade, setShowRightFade] = createSignal(false);
  let scrollContainer: HTMLDivElement | undefined;

  const checkScroll = () => {
    if (!scrollContainer) return;

    const { scrollLeft, scrollWidth, clientWidth } = scrollContainer;
    setShowLeftFade(scrollLeft > 0);
    setShowRightFade(scrollLeft < scrollWidth - clientWidth - 1);
  };

  onMount(() => {
    checkScroll();
    window.addEventListener('resize', checkScroll);
    onCleanup(() => {
      window.removeEventListener('resize', checkScroll);
    });
  });

  createEffect(() => {
    if (scrollContainer) {
      scrollContainer.addEventListener('scroll', checkScroll);
      return () => scrollContainer?.removeEventListener('scroll', checkScroll);
    }
  });

  return (
    <div class={`relative ${props.class || ''}`}>
      {/* Left fade */}
      <Show when={showLeftFade()}>
        <div class="absolute left-0 top-0 bottom-0 w-8 bg-gradient-to-r from-white dark:from-gray-800 to-transparent z-10 pointer-events-none" />
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
        <div style={{ 'min-width': props.minWidth || 'auto' }}>{props.children}</div>
      </div>

      {/* Right fade */}
      <Show when={showRightFade()}>
        <div class="absolute right-0 top-0 bottom-0 w-8 bg-gradient-to-l from-white dark:from-gray-800 to-transparent z-10 pointer-events-none" />
      </Show>
    </div>
  );
};
