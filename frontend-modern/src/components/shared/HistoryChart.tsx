import { Component, createMemo, createUniqueId } from 'solid-js';
import {
  formatHistoryChartTooltipValue,
  getHistoryChartAccessibleDescription,
  getHistoryChartAccessibleLabel,
  type HistoryChartProps,
} from './historyChartModel';
import { HistoryChartHeader } from './HistoryChartHeader';
import { HistoryChartHoverGroup, useHistoryChartHoverGroup } from './HistoryChartHoverGroup';
import { HistoryChartOverlay } from './HistoryChartOverlay';
import { HistoryChartTooltip } from './HistoryChartTooltip';
import { useHistoryChartState } from './useHistoryChartState';

export type { HistoryChartProps } from './historyChartModel';
export { HistoryChartHoverGroup };

export const HistoryChart: Component<HistoryChartProps> = (props) => {
  let canvasRef: HTMLCanvasElement | undefined;
  let containerRef: HTMLDivElement | undefined;
  const descriptionId = `history-chart-description-${createUniqueId()}`;
  const hoverGroup = useHistoryChartHoverGroup();

  const chart = useHistoryChartState(
    props,
    {
      getCanvas: () => canvasRef,
      getContainer: () => containerRef,
    },
    hoverGroup,
  );
  const accessibleDescription = createMemo(() =>
    getHistoryChartAccessibleDescription({
      data: chart.data(),
      error: chart.error(),
      isLocked: chart.isLocked(),
      loading: chart.loading(),
      range: chart.range(),
      unit: props.unit,
    }),
  );

  return (
    <div
      class={`flex flex-col h-full ${props.compact ? '' : 'bg-surface rounded-md shadow-sm border border-border p-4'}`}
    >
      <HistoryChartHeader
        chart={chart}
        compact={props.compact}
        hideSelector={props.hideSelector}
        label={props.label}
        unit={props.unit}
      />

      <div
        class={`relative flex-1 w-full ${props.compact ? 'min-h-[120px]' : 'min-h-[200px]'}`}
        ref={containerRef}
      >
        <canvas
          ref={canvasRef}
          class="block w-full h-full cursor-crosshair rounded focus-visible:outline focus-visible:outline-2 focus-visible:outline-blue-500"
          tabIndex={0}
          onFocus={chart.handleFocus}
          onBlur={chart.handleBlur}
          onKeyDown={chart.handleKeyDown}
          role="img"
          aria-label={getHistoryChartAccessibleLabel(props.label)}
          aria-describedby={descriptionId}
          onMouseMove={chart.handleMouseMove}
          onMouseLeave={chart.handleMouseLeave}
        />
        <p id={descriptionId} class="sr-only">
          {accessibleDescription()} Use Left and Right arrow keys to inspect readings, Home and End
          for the first and last reading, and Escape to clear inspection.
        </p>
        <p class="sr-only" aria-live="polite" aria-atomic="true">
          {chart.keyboardInspecting() && chart.hoveredPoint()
            ? `${new Date(chart.hoveredPoint()!.timestamp).toLocaleString()}: ${formatHistoryChartTooltipValue(chart.hoveredPoint()!.value, props.unit)}`
            : ''}
        </p>
        <HistoryChartOverlay chart={chart} hideLock={props.hideLock} />
        <HistoryChartTooltip
          hoveredPoint={chart.hoveredPoint()}
          chartWidth={chart.chartWidth()}
          chartHeight={chart.chartHeight()}
          unit={props.unit}
        />
      </div>
    </div>
  );
};
