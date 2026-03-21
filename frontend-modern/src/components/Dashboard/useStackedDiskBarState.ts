import { createMemo, createSignal, onCleanup, onMount } from 'solid-js';
import { useTooltip } from '@/hooks/useTooltip';
import {
  buildStackedDiskBarPresentation,
  type StackedDiskBarProps,
} from './stackedDiskBarModel';

export function useStackedDiskBarState(props: StackedDiskBarProps) {
  const tip = useTooltip();
  const [containerWidth, setContainerWidth] = createSignal(100);
  let containerRef: HTMLDivElement | undefined;
  let resizeObserver: ResizeObserver | undefined;

  const presentation = createMemo(() => buildStackedDiskBarPresentation(props, containerWidth()));

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

  const handleMouseEnter = (event: MouseEvent) => {
    if (presentation().tooltipContent.length > 0) {
      tip.onMouseEnter(event);
    }
  };

  return {
    handleMouseEnter,
    handleMouseLeave: tip.onMouseLeave,
    presentation,
    setContainerRef: (element: HTMLDivElement) => {
      containerRef = element;
    },
    tip,
    tooltipVisible: createMemo(() => tip.show() && presentation().tooltipContent.length > 0),
  };
}
