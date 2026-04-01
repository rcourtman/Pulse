import { Component, Show } from 'solid-js';
import {
  formatDensityMapHoverTime,
  hasDensityMapFocusActivity,
  type DensityMapProps,
} from './densityMapModel';
import { TooltipPortal } from './TooltipPortal';
import { useDensityMapState } from './useDensityMapState';

export type { DensityMapProps } from './densityMapModel';

export const DensityMap: Component<DensityMapProps> = (props) => {
  const densityMap = useDensityMapState(props);
  const interactionState = () => props.interactionState ?? 'default';
  const formatDetailValue = (value: number | null) =>
    value === null ? 'No sample' : densityMap.formatValue(value);
  const hoveredFocusDetail = () => (densityMap.hoveredState() ? densityMap.focusDetail() : null);

  return (
    <div
      class={`relative w-full h-full flex flex-col group transition-opacity duration-150 ease-out ${
        interactionState() === 'inactive' ? 'opacity-35' : 'opacity-100'
      }`.trim()}
      data-summary-chart-kind="density-map"
      data-active-hover-timestamp={densityMap.activeHoverTimestamp() ?? ''}
      data-highlight-series-id={props.highlightSeriesId ?? ''}
      data-highlight-series-active={densityMap.externalSeriesIndex() !== null ? 'true' : 'false'}
      data-rendered-series-count={densityMap.chartData().series.length}
      data-summary-chart-state={interactionState()}
    >
      <div class="flex-1 relative min-h-0 bg-transparent flex">
        {/* Y-axis: typically in a density map we might just show "Top Nodes" */}
        <div class="absolute left-0 top-0 h-full w-full pointer-events-none flex flex-col justify-between py-1 opacity-0 group-hover:opacity-100 transition-opacity">
          {/* We could overlay series names here, but it might get messy. Let's rely on tooltip. */}
        </div>

        <div class="h-full ml-1 mr-1 flex-1">
          <canvas
            ref={densityMap.setCanvasRef}
            class="w-full h-full cursor-crosshair block"
            onMouseMove={densityMap.handleMouseMove}
            onMouseLeave={densityMap.handleMouseLeave}
          />
        </div>
      </div>

      {/* X-axis labels */}
      <div class="relative h-4 mt-1 pointer-events-none mx-1">
        <span class="absolute left-0 top-0 text-[9px] font-medium leading-none text-slate-500 transition-all duration-300">
          -{props.rangeLabel || props.timeRange}
        </span>
        <span class="absolute right-0 top-0 text-[9px] font-medium leading-none text-slate-500 transition-all duration-300">
          now
        </span>
      </div>

      <Show when={densityMap.hoveredState()}>
        {(hover) => (
          <TooltipPortal when={true} x={hover().tooltipX} y={hover().tooltipY + 2}>
            <div class="min-w-[152px] max-w-[236px]" data-density-map-tooltip="true">
              <div class="mb-1 grid grid-cols-[auto_minmax(0,1fr)_auto] items-center gap-1.5 border-b border-border pb-1 text-[10px] leading-none">
                <span class="h-2 w-2 rounded-sm" style={{ background: hover().seriesColor }} />
                <span class="min-w-0 truncate font-semibold tracking-[0.12em] text-base-content">
                  {hover().seriesName}
                </span>
                <span class="whitespace-nowrap text-muted">
                  {formatDensityMapHoverTime(hover().timestamp)}
                </span>
              </div>
              <div class="flex items-center gap-2.5">
                <div class="flex items-baseline gap-1">
                  <span class="text-[9px] uppercase tracking-wide text-muted">Current</span>
                  <span class="whitespace-nowrap text-[11px] font-semibold text-emerald-400">
                    {densityMap.formatValue(hover().value)}
                  </span>
                </div>
                <Show
                  when={hoveredFocusDetail()}
                  fallback={
                    <div class="flex items-baseline gap-1">
                      <span class="text-[9px] uppercase tracking-wide text-muted">Peak</span>
                      <span class="whitespace-nowrap text-[11px] font-semibold text-base-content">
                        No sample
                      </span>
                    </div>
                  }
                >
                  {(detail) => (
                    <Show
                      when={hasDensityMapFocusActivity(detail())}
                      fallback={
                        <div class="flex items-baseline gap-1">
                          <span class="text-[9px] uppercase tracking-wide text-muted">Peak</span>
                          <span class="text-[11px] font-semibold text-base-content">
                            {props.focusEmptyStateLabel ?? 'No activity in range'}
                          </span>
                        </div>
                      }
                    >
                      <div class="flex items-baseline gap-1">
                        <span class="text-[9px] uppercase tracking-wide text-muted">Peak</span>
                        <span class="whitespace-nowrap text-[11px] font-semibold text-base-content">
                          {formatDetailValue(detail().peakValue)}
                        </span>
                      </div>
                    </Show>
                  )}
                </Show>
              </div>
              <Show when={hoveredFocusDetail()?.sparklinePath}>
                {(path) => (
                  <div class="mt-1 flex justify-end">
                    <svg
                      viewBox="0 0 64 22"
                      class="h-4 w-[72px] overflow-visible"
                      aria-hidden="true"
                      data-density-map-tooltip-sparkline="true"
                    >
                      <path
                        d={path()}
                        fill="none"
                        stroke={hover().seriesColor}
                        stroke-linecap="round"
                        stroke-linejoin="round"
                        stroke-width="1.75"
                      />
                    </svg>
                  </div>
                )}
              </Show>
            </div>
          </TooltipPortal>
        )}
      </Show>
    </div>
  );
};

export default DensityMap;
