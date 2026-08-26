import { createMemo, createSignal, onCleanup, onMount } from 'solid-js';
import { buildMetricBarPresentation, type MetricBarProps } from './metricBarModel';

export function useMetricBarState(props: MetricBarProps) {
  const [containerWidth, setContainerWidth] = createSignal(100);
  let containerRef: HTMLDivElement | undefined;
  let resizeObserver: ResizeObserver | undefined;

  const presentation = createMemo(() => buildMetricBarPresentation(props, containerWidth()));

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
    presentation,
    setContainerRef: (element: HTMLDivElement) => {
      containerRef = element;
    },
  };
}
