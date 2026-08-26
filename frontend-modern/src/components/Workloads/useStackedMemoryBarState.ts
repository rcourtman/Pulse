import { createMemo, createSignal, onCleanup, onMount } from 'solid-js';
import { useTooltip } from '@/hooks/useTooltip';
import {
  buildStackedMemoryBarPresentation,
  type StackedMemoryBarProps,
} from './stackedMemoryBarModel';

export function useStackedMemoryBarState(props: StackedMemoryBarProps) {
  const tip = useTooltip();
  const [containerWidth, setContainerWidth] = createSignal(100);
  let containerRef: HTMLDivElement | undefined;
  let resizeObserver: ResizeObserver | undefined;

  const presentation = createMemo(() => buildStackedMemoryBarPresentation(props, containerWidth()));

  onMount(() => {
    if (!containerRef) {
      return;
    }

    // Width comes only from the ResizeObserver: its initial delivery fires in
    // the same frame as observe(), after layout and before paint. A sync
    // offsetWidth read here would force a full table reflow per mounted bar,
    // which the virtualized tables pay on every runway top-up while scrolling.
    resizeObserver = new ResizeObserver((entries) => {
      for (const entry of entries) {
        setContainerWidth(entry.contentRect.width);
      }
    });
    resizeObserver.observe(containerRef);
  });

  onCleanup(() => {
    resizeObserver?.disconnect();
  });

  return {
    handleMouseEnter: tip.onMouseEnter,
    handleMouseLeave: tip.onMouseLeave,
    presentation,
    setContainerRef: (element: HTMLDivElement) => {
      containerRef = element;
    },
    tip,
    tooltipVisible: createMemo(() => tip.show() && presentation().tooltipRows.length > 0),
  };
}
