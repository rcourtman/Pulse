import { Component } from 'solid-js';
import type { HistoryChartProps } from './historyChartModel';
import { HistoryChartHeader } from './HistoryChartHeader';
import { HistoryChartOverlay } from './HistoryChartOverlay';
import { HistoryChartTooltip } from './HistoryChartTooltip';
import { useHistoryChartState } from './useHistoryChartState';

export type { HistoryChartProps } from './historyChartModel';

export const HistoryChart: Component<HistoryChartProps> = (props) => {
  let canvasRef: HTMLCanvasElement | undefined;
  let containerRef: HTMLDivElement | undefined;

  const chart = useHistoryChartState(props, {
    getCanvas: () => canvasRef,
    getContainer: () => containerRef,
  });

  return (
    <div
      class={`flex flex-col h-full ${props.compact ? '' : 'bg-surface rounded-md shadow-sm border border-border p-4'}`}
    >
      <HistoryChartHeader chart={chart} compact={props.compact} hideSelector={props.hideSelector} label={props.label} unit={props.unit} />

      <div
        class={`relative flex-1 w-full ${props.compact ? 'min-h-[120px]' : 'min-h-[200px]'}`}
        ref={containerRef}
      >
        <canvas
          ref={canvasRef}
          class="block w-full h-full cursor-crosshair"
          onMouseMove={chart.handleMouseMove}
          onMouseLeave={chart.handleMouseLeave}
        />
        <HistoryChartOverlay chart={chart} hideLock={props.hideLock} />
      </div>

      <HistoryChartTooltip hoveredPoint={chart.hoveredPoint()} unit={props.unit} />
    </div>
  );
};
