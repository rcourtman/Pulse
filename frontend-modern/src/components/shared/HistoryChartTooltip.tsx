import { Component, Show } from 'solid-js';
import { Portal } from 'solid-js/web';
import { formatHistoryChartTooltipValue, type HistoryChartHoverPoint } from './historyChartModel';

interface HistoryChartTooltipProps {
  hoveredPoint: HistoryChartHoverPoint | null;
  unit?: string;
}

export const HistoryChartTooltip: Component<HistoryChartTooltipProps> = (props) => {
  return (
    <Portal>
      <Show when={props.hoveredPoint}>
        {(point) => (
          <div
            class="fixed pointer-events-none text-xs rounded px-2 py-1 shadow-lg border border-slate-600 z-[9999]"
            style={{
              left: `${point().x}px`,
              top: `${point().y}px`,
              transform: 'translateX(-50%)',
              'background-color': 'rgb(15, 23, 42)',
              color: 'rgb(248, 250, 252)',
            }}
          >
            <div class="font-medium text-center mb-0.5">
              {new Date(point().timestamp).toLocaleString()}
            </div>
            <div style={{ color: 'rgb(203, 213, 225)' }}>
              {formatHistoryChartTooltipValue(point().value, props.unit)}
            </div>
          </div>
        )}
      </Show>
    </Portal>
  );
};
