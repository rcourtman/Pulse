import { Show, For } from 'solid-js';
import { TooltipPortal } from '@/components/shared/TooltipPortal';
import { type StackedMemoryBarProps } from './stackedMemoryBarModel';
import { useStackedMemoryBarState } from './useStackedMemoryBarState';

export function StackedMemoryBar(props: StackedMemoryBarProps) {
  const state = useStackedMemoryBarState(props);
  const presentation = state.presentation;

  return (
    <div ref={state.setContainerRef} class="metric-text w-full h-4 flex items-center justify-center">
      <div
        class="relative w-full h-full overflow-hidden bg-surface-hover rounded"
        onMouseEnter={state.handleMouseEnter}
        onMouseLeave={state.handleMouseLeave}
      >
        <For each={presentation().segments}>
          {(segment, idx) => (
            <div
              class="absolute top-0 h-full transition-all duration-300"
              style={{
                left: `${segment.leftPercent}%`,
                width: `${segment.widthPercent}%`,
                'background-color': segment.color,
                'border-right':
                  idx() < presentation().segments.length - 1
                    ? '1px solid rgba(255,255,255,0.3)'
                    : 'none',
              }}
            />
          )}
        </For>

        <Show when={presentation().showSwapBar}>
          <div
            class="absolute bottom-0 left-0 h-[3px] w-full bg-purple-500"
            style={{ width: `${presentation().swapBarPercent}%` }}
          />
        </Show>

        <span class="absolute inset-0 flex items-center justify-center text-[10px] font-semibold text-base-content leading-none pointer-events-none min-w-0 overflow-hidden">
          <span class="max-w-full min-w-0 whitespace-nowrap overflow-hidden text-ellipsis px-0.5 text-center">
            <span>{presentation().displayLabel}</span>
            <Show when={presentation().showSublabel}>
              <span class="metric-sublabel font-normal text-muted">
                {' '}
                ({presentation().displaySublabel})
              </span>
            </Show>
            <Show when={presentation().anomalyDescription && presentation().anomalyRatio}>
              <span
                class={`ml-0.5 font-bold animate-pulse ${presentation().anomalyClass}`}
                title={presentation().anomalyDescription}
              >
                {presentation().anomalyRatio}
              </span>
            </Show>
          </span>
        </span>
      </div>

      <TooltipPortal when={state.tooltipVisible()} x={state.tip.pos().x} y={state.tip.pos().y}>
        <div class="min-w-[140px]">
          <div class="font-medium mb-1 text-slate-300 border-b border-border pb-1">
            {presentation().tooltipTitle}
          </div>
          <For each={presentation().tooltipRows}>
            {(row) => (
              <div
                class="flex justify-between gap-3 py-0.5"
                classList={{ 'border-t border-border': row.borderTop }}
              >
                <span class={row.labelClass}>{row.label}</span>
                <span class="whitespace-nowrap text-slate-300">{row.value}</span>
              </div>
            )}
          </For>
        </div>
      </TooltipPortal>
    </div>
  );
}
