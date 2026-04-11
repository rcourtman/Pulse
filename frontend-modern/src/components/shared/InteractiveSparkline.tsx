import { Component, For, Show } from 'solid-js';
import {
  formatInteractiveSparklineHoverTime,
  type InteractiveSparklineHoverState,
  type InteractiveSparklineProps,
} from './interactiveSparklineModel';
import { useInteractiveSparklineState } from './useInteractiveSparklineState';

export type {
  InteractiveSparklineProps,
  InteractiveSparklineSeries,
} from './interactiveSparklineModel';

const verticalTextBaseline = (anchor: 'top' | 'middle' | 'bottom') =>
  anchor === 'top' ? 'hanging' : anchor === 'bottom' ? 'text-bottom' : 'middle';

const horizontalTextAnchor = (anchor: 'start' | 'middle' | 'end') => anchor;

const tooltipWidth = (hover: InteractiveSparklineHoverState) => (hover.focusedTooltip ? 112 : 138);

const tooltipHeight = (hover: InteractiveSparklineHoverState) =>
  22 + hover.values.length * 16 + (hover.totalValues > hover.values.length ? 14 : 0);

export const InteractiveSparkline: Component<InteractiveSparklineProps> = (props) => {
  let chartSurfaceRef: Element | undefined;
  let canvasRef: HTMLCanvasElement | undefined;
  let canvasHostRef: HTMLDivElement | undefined;
  const interactionState = () => props.interactionState ?? 'default';
  const inactiveSeriesColor = () =>
    typeof document !== 'undefined' && document.documentElement.classList.contains('dark')
      ? 'rgb(148, 163, 184)'
      : 'rgb(100, 116, 139)';

  const sparkline = useInteractiveSparklineState(props, {
    getCanvas: () => canvasRef,
    getCanvasHost: () => canvasHostRef,
    getChartSurface: () => chartSurfaceRef,
  });

  return (
    <div
      class={`w-full h-full min-h-[88px] flex flex-col transition-opacity duration-150 ease-out ${
        interactionState() === 'inactive' ? 'opacity-35' : 'opacity-100'
      }`.trim()}
      data-highlight-series-id={props.highlightSeriesId ?? ''}
      data-highlight-series-active={sparkline.externalSeriesIndex() !== null ? 'true' : 'false'}
      data-active-hover-timestamp={sparkline.activeHoverTimestamp() ?? ''}
      data-active-hover-cursor-x={sparkline.activeHoverCursorX() ?? ''}
      data-active-series-display={sparkline.activeSeriesDisplay()}
      data-rendered-series-count={sparkline.renderedSeriesCount()}
      data-summary-chart-state={interactionState()}
    >
      <div class="relative flex-1 min-h-0">
        <div class="h-full ml-7 mr-3" ref={canvasHostRef}>
          <Show
            when={sparkline.shouldUseCanvas()}
            fallback={
              <svg
                ref={(element) => {
                  chartSurfaceRef = element;
                }}
                class="w-full h-full cursor-crosshair"
                viewBox={`0 0 ${sparkline.vbW} ${sparkline.vbH}`}
                preserveAspectRatio="none"
                onMouseMove={sparkline.handleMouseMove}
                onMouseLeave={sparkline.handleMouseLeave}
                onClick={sparkline.handleClick}
              >
                <Show when={sparkline.chartData().validSeries.length === 1}>
                  <defs>
                    <linearGradient id="single-series-area" x1="0" y1="0" x2="0" y2="1">
                      <stop
                        offset="0%"
                        stop-color={sparkline.chartData().validSeries[0].color}
                        stop-opacity="0.25"
                      />
                      <stop
                        offset="100%"
                        stop-color={sparkline.chartData().validSeries[0].color}
                        stop-opacity="0"
                      />
                    </linearGradient>
                  </defs>
                </Show>
                <For each={sparkline.gridLineY()}>
                  {(y, index) => (
                    <line
                      x1="0"
                      y1={y}
                      x2={sparkline.vbW}
                      y2={y}
                      stroke="currentColor"
                      stroke-opacity={index() === 1 ? '0.1' : '0.06'}
                      stroke-width="0.5"
                    />
                  )}
                </For>

                <For each={sparkline.gridLineX()}>
                  {(x) => (
                    <line
                      x1={x}
                      y1="0"
                      x2={x}
                      y2={sparkline.vbH}
                      stroke="currentColor"
                      stroke-opacity="0.04"
                      stroke-width="0.5"
                    />
                  )}
                </For>

                <Show when={sparkline.activeHoverCursorX() !== null}>
                  <line
                    x1={sparkline.activeHoverCursorX() ?? 0}
                    y1={0}
                    x2={sparkline.activeHoverCursorX() ?? 0}
                    y2={sparkline.vbH}
                    stroke="currentColor"
                    stroke-opacity="0.45"
                    stroke-width="1"
                    stroke-dasharray="3 3"
                    vector-effect="non-scaling-stroke"
                  />
                </Show>

                <For each={sparkline.chartData().paths}>
                  {(pathData) => (
                    <Show when={sparkline.shouldRenderSeries(pathData.seriesIndex)}>
                      <g>
                        <Show when={pathData.areaPath}>
                          <path
                            d={pathData.areaPath}
                            fill="url(#single-series-area)"
                            stroke="none"
                          />
                        </Show>
                        <path
                          d={pathData.path}
                          fill="none"
                          stroke={(() => {
                            const active = sparkline.activeEmphasisSeriesIndex();
                            if (active !== null && active !== pathData.seriesIndex) {
                              return inactiveSeriesColor();
                            }
                            return pathData.color;
                          })()}
                          stroke-width={sparkline.lineWidthForSeries(pathData.seriesIndex)}
                          stroke-linecap="round"
                          stroke-linejoin="round"
                          opacity={sparkline.opacityForSeries(pathData.seriesIndex)}
                          class="transition-all duration-100 ease-linear"
                          vector-effect="non-scaling-stroke"
                        />
                      </g>
                    </Show>
                  )}
                </For>
              </svg>
            }
          >
            <canvas
              ref={(element) => {
                canvasRef = element;
                chartSurfaceRef = element;
              }}
              class="w-full h-full cursor-crosshair block"
              onMouseMove={sparkline.handleMouseMove}
              onMouseLeave={sparkline.handleMouseLeave}
              onClick={sparkline.handleClick}
            />
          </Show>
          <svg
            class="pointer-events-none absolute inset-0 h-full w-full overflow-visible"
            viewBox={`0 0 ${sparkline.vbW} ${sparkline.vbH}`}
            preserveAspectRatio="none"
            aria-hidden="true"
          >
            <Show when={sparkline.hoveredState()}>
              {(hover) => (
                <foreignObject
                  x={hover().tooltipX - tooltipWidth(hover()) / 2}
                  y={hover().tooltipY - tooltipHeight(hover())}
                  width={tooltipWidth(hover())}
                  height={tooltipHeight(hover())}
                  overflow="visible"
                >
                  <div
                    data-sparkline-tooltip="true"
                    class="h-full w-full rounded-md border border-border bg-surface px-2 py-1.5 text-[10px] text-base-content shadow-lg"
                  >
                    <div class="mb-1 text-center font-medium text-base-content">
                      {formatInteractiveSparklineHoverTime(hover().timestamp)}
                    </div>
                    <For each={hover().values}>
                      {(entry) => (
                        <div
                          class={`flex items-center gap-1.5 leading-tight ${
                            props.highlightNearestSeriesOnHover &&
                            hover().focusedTooltip &&
                            hover().highlightedSeriesIndex === entry.seriesIndex
                              ? 'rounded px-1 bg-slate-400/15'
                              : ''
                          }`}
                        >
                          <svg class="h-2 w-2 shrink-0" viewBox="0 0 8 8" aria-hidden="true">
                            <circle cx="4" cy="4" r="4" fill={entry.color} />
                          </svg>
                          <span class="text-muted">{entry.name}</span>
                          <span class="ml-auto font-medium text-base-content">
                            {sparkline.formatValue(entry.value)}
                          </span>
                        </div>
                      )}
                    </For>
                    <Show when={hover().totalValues > hover().values.length}>
                      <div class="mt-0.5 text-[10px] text-muted">
                        +{hover().totalValues - hover().values.length} more series
                      </div>
                    </Show>
                  </div>
                </foreignObject>
              )}
            </Show>
          </svg>
        </div>
        <div class="absolute inset-y-0 left-0 w-7 pointer-events-none">
          <svg
            class="h-full w-full overflow-visible text-muted"
            viewBox={`0 0 28 ${sparkline.vbH}`}
            preserveAspectRatio="none"
            aria-hidden="true"
          >
            <For each={sparkline.axisTicks()}>
              {(tick) => (
                <text
                  x="0"
                  y={tick.y}
                  fill="currentColor"
                  font-size="8"
                  class="transition-all duration-300 ease-out"
                  dominant-baseline={verticalTextBaseline(tick.anchor)}
                >
                  {tick.label}
                </text>
              )}
            </For>
          </svg>
        </div>
      </div>
      <div class="relative pointer-events-none ml-7 mr-3 h-4">
        <svg
          class="h-full w-full overflow-visible text-muted"
          viewBox={`0 0 ${sparkline.vbW} ${sparkline.xAxisBandPx}`}
          preserveAspectRatio="none"
          aria-hidden="true"
        >
          <For each={sparkline.xAxisTicks()}>
            {(tick) => (
              <text
                x={tick.x}
                y="2"
                fill="currentColor"
                font-size="9"
                font-weight="500"
                class="transition-all duration-300 ease-out"
                text-anchor={horizontalTextAnchor(tick.anchor)}
                dominant-baseline="hanging"
              >
                {tick.label}
              </text>
            )}
          </For>
        </svg>
      </div>
    </div>
  );
};

export default InteractiveSparkline;
