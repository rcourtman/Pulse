import { Component, Show } from 'solid-js';
import { Portal } from 'solid-js/web';
import {
  formatDensityMapHoverTime,
  hasDensityMapFocusActivity,
  type DensityMapProps,
} from './densityMapModel';
import { useDensityMapState } from './useDensityMapState';

export type { DensityMapProps } from './densityMapModel';

export const DensityMap: Component<DensityMapProps> = (props) => {
  const densityMap = useDensityMapState(props);
  const interactionState = () => props.interactionState ?? 'default';
  const formatDetailValue = (value: number | null) =>
    value === null ? 'No sample' : densityMap.formatValue(value);

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
      <div class="mx-1 mb-1 flex h-7 items-center overflow-hidden">
        <Show
          when={densityMap.focusDetail()}
          fallback={
            <div class="w-full truncate text-[9px] font-medium uppercase tracking-[0.12em] text-slate-500/80">
              Top activity overview
            </div>
          }
        >
          {(detail) => (
            <div
              class="flex w-full min-w-0 items-center gap-2 text-[10px]"
              data-density-map-focus-detail="true"
              data-density-map-focus-empty={hasDensityMapFocusActivity(detail()) ? 'false' : 'true'}
              data-density-map-focus-series-id={detail().seriesId}
            >
              <div class="flex min-w-0 flex-1 items-center gap-1.5">
                <span
                  class="h-2 w-2 shrink-0 rounded-full"
                  style={{ background: detail().seriesColor }}
                />
                <span class="truncate font-semibold uppercase tracking-[0.1em] text-slate-600 dark:text-slate-300">
                  {detail().seriesName}
                </span>
                <Show when={detail().sparklinePath}>
                  {(path) => (
                    <svg
                      viewBox="0 0 64 22"
                      class="h-4 w-12 shrink-0 overflow-visible text-slate-400"
                      aria-hidden="true"
                    >
                      <path
                        d={path()}
                        fill="none"
                        stroke={detail().seriesColor}
                        stroke-linecap="round"
                        stroke-linejoin="round"
                        stroke-width="1.75"
                      />
                    </svg>
                  )}
                </Show>
              </div>
              <Show
                when={hasDensityMapFocusActivity(detail())}
                fallback={
                  <div class="shrink-0 rounded-full bg-slate-100 px-2 py-1 text-[9px] font-medium uppercase tracking-[0.08em] text-slate-500 dark:bg-slate-800 dark:text-slate-300">
                    {props.focusEmptyStateLabel ?? 'No activity in range'}
                  </div>
                }
              >
                <div class="flex shrink-0 items-center gap-3 leading-none">
                  <div class="flex flex-col items-start">
                    <span class="text-[9px] uppercase tracking-wide text-slate-500 dark:text-slate-400">
                      Latest
                    </span>
                    <span class="mt-0.5 font-semibold text-slate-900 dark:text-slate-50">
                      {formatDetailValue(detail().currentValue)}
                    </span>
                  </div>
                  <div class="flex flex-col items-start">
                    <span class="text-[9px] uppercase tracking-wide text-slate-500 dark:text-slate-400">
                      Peak
                    </span>
                    <span class="mt-0.5 font-semibold text-slate-900 dark:text-slate-50">
                      {formatDetailValue(detail().peakValue)}
                    </span>
                  </div>
                </div>
              </Show>
            </div>
          )}
        </Show>
      </div>

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

      <Portal>
        <Show when={densityMap.hoveredState()}>
          {(hover) => (
            <div
              class="fixed pointer-events-none text-xs rounded px-2 py-1.5 shadow-lg border border-slate-600"
              style={{
                left: `${hover().tooltipX}px`,
                top: `${hover().tooltipY - 6}px`,
                transform: 'translate(-50%, -100%)',
                'z-index': '9999',
                'background-color': 'rgb(15, 23, 42)',
                color: 'rgb(248, 250, 252)',
              }}
            >
              <div class="font-medium text-center mb-1 text-slate-300">
                {formatDensityMapHoverTime(hover().timestamp)}
              </div>
              <div class="flex items-center gap-1.5 leading-tight">
                <span class="w-2 h-2 rounded-sm" style={{ background: hover().seriesColor }} />
                <span class="font-semibold text-white">{hover().seriesName}</span>
                <span class="ml-2 font-medium text-emerald-400">
                  {densityMap.formatValue(hover().value)}
                </span>
              </div>
            </div>
          )}
        </Show>
      </Portal>
    </div>
  );
};

export default DensityMap;
