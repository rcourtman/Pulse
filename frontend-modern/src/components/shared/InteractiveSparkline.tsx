import { Component, For, Show } from 'solid-js';
import { Portal } from 'solid-js/web';
import {
  formatInteractiveSparklineHoverTime,
  type InteractiveSparklineProps,
} from './interactiveSparklineModel';
import { useInteractiveSparklineState } from './useInteractiveSparklineState';

export type {
  InteractiveSparklineProps,
  InteractiveSparklineSeries,
} from './interactiveSparklineModel';

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
        <div class="absolute inset-y-0 left-0 w-7 pointer-events-none">
          <For each={sparkline.axisTicks()}>
            {(tick) => (
              <span
                class="absolute left-0 text-[8px] leading-none text-muted transition-all duration-300 ease-out"
                style={{
                  top: tick.top,
                  transform:
                    tick.anchor === 'top'
                      ? 'translateY(0)'
                      : tick.anchor === 'bottom'
                        ? 'translateY(-100%)'
                        : 'translateY(-50%)',
                }}
              >
                {tick.label}
              </span>
            )}
          </For>
        </div>
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
                  {(cursorX) => (
                    <line
                      x1={cursorX()}
                      y1={Math.max(0, (sparkline.activeHoverState()?.minY ?? 4) - 4)}
                      x2={cursorX()}
                      y2={sparkline.vbH}
                      stroke="currentColor"
                      stroke-opacity="0.45"
                      stroke-width="1"
                      stroke-dasharray="3 3"
                      vector-effect="non-scaling-stroke"
                    />
                  )}
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
                          style={{ transition: 'opacity 90ms linear, stroke-width 90ms linear' }}
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
        </div>
      </div>
      <div
        class="relative pointer-events-none ml-7 mr-3"
        style={{ height: `${sparkline.xAxisBandPx}px` }}
      >
        <For each={sparkline.xAxisTicks()}>
          {(tick) => (
            <span
              class="absolute top-[2px] text-[9px] font-medium leading-none text-muted transition-all duration-300 ease-out"
              style={{
                left: `${tick.left}%`,
                transform:
                  tick.anchor === 'start'
                    ? 'translateX(0)'
                    : tick.anchor === 'end'
                      ? 'translateX(-100%)'
                      : 'translateX(-50%)',
              }}
            >
              {tick.label}
            </span>
          )}
        </For>
      </div>

      <Portal>
        <Show when={sparkline.hoveredState()}>
          {(hover) => (
            <div
              class="fixed pointer-events-none text-xs rounded px-2 py-1.5 shadow-lg border border-slate-600"
              style={{
                left: `${hover().tooltipX}px`,
                top: `${hover().tooltipY}px`,
                transform: 'translate(-50%, -100%)',
                'z-index': '9999',
                'background-color': 'rgb(15, 23, 42)',
                color: 'rgb(248, 250, 252)',
              }}
            >
              <div class="font-medium text-center mb-1">
                {formatInteractiveSparklineHoverTime(hover().timestamp)}
              </div>
              <For each={hover().values}>
                {(entry) => (
                  <div
                    class={`flex items-center gap-1.5 leading-tight ${
                      props.highlightNearestSeriesOnHover &&
                      hover().focusedTooltip &&
                      hover().highlightedSeriesIndex !== null
                        ? hover().highlightedSeriesIndex === entry.seriesIndex
                          ? 'rounded px-1'
                          : 'opacity-40'
                        : ''
                    }`}
                    style={
                      props.highlightNearestSeriesOnHover &&
                      hover().focusedTooltip &&
                      hover().highlightedSeriesIndex === entry.seriesIndex
                        ? { 'background-color': 'rgba(255,255,255,0.1)' }
                        : {}
                    }
                  >
                    <span class="w-1.5 h-1.5 rounded-full" style={{ background: entry.color }} />
                    <span style={{ color: 'rgb(203, 213, 225)' }}>{entry.name}</span>
                    <span class="ml-auto font-medium" style={{ color: 'rgb(248, 250, 252)' }}>
                      {sparkline.formatValue(entry.value)}
                    </span>
                  </div>
                )}
              </For>
              <Show when={hover().totalValues > hover().values.length}>
                <div class="text-[10px] mt-0.5" style={{ color: 'rgb(148, 163, 184)' }}>
                  +{hover().totalValues - hover().values.length} more series
                </div>
              </Show>
            </div>
          )}
        </Show>
      </Portal>
    </div>
  );
};

export default InteractiveSparkline;
