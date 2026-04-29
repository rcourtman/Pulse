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

  const presentation = createMemo(() =>
    buildStackedMemoryBarPresentation(props, containerWidth()),
  );

  onMount(() => {
    if (!containerRef) {
      return;
    }

    setContainerWidth(containerRef.offsetWidth);

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

