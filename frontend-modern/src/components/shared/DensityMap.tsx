import { Component, Show } from 'solid-js';
import { Portal } from 'solid-js/web';
import {
  formatDensityMapHoverTime,
  type DensityMapProps,
} from './densityMapModel';
import { useDensityMapState } from './useDensityMapState';

export type { DensityMapProps } from './densityMapModel';

export const DensityMap: Component<DensityMapProps> = (props) => {
  const densityMap = useDensityMapState(props);

  return (
    <div class="relative w-full h-full flex flex-col group">
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
