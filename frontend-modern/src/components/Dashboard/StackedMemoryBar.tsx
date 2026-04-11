import { Show, For } from 'solid-js';
import { TooltipPortal } from '@/components/shared/TooltipPortal';
import { type StackedMemoryBarProps } from './stackedMemoryBarModel';
import { useStackedMemoryBarState } from './useStackedMemoryBarState';

export function StackedMemoryBar(props: StackedMemoryBarProps) {
  const state = useStackedMemoryBarState(props);
  const presentation = state.presentation;
  const swapBarWidth = () => String(Math.max(0, Math.min(presentation().swapBarPercent, 100)));
  const segmentEdge = (leftPercent: number, widthPercent: number) =>
    String(Math.max(0, Math.min(leftPercent + widthPercent, 100)));

  return (
    <div ref={state.setContainerRef} class="metric-text w-full h-4 flex items-center justify-center">
      <div
        class="relative w-full h-full overflow-hidden bg-surface-hover rounded"
        onMouseEnter={state.handleMouseEnter}
        onMouseLeave={state.handleMouseLeave}
      >
        <svg
          aria-hidden="true"
          class="absolute inset-0 h-full w-full"
          viewBox="0 0 100 100"
          preserveAspectRatio="none"
        >
          <For each={presentation().segments}>
            {(segment, idx) => (
              <>
                <rect
                  data-stacked-memory-segment="true"
                  x={String(Math.max(0, Math.min(segment.leftPercent, 100)))}
                  y="0"
                  width={String(Math.max(0, segment.widthPercent))}
                  height="100"
                  rx="3"
                  fill={segment.color}
                />
                <Show when={idx() < presentation().segments.length - 1}>
                  <line
                    x1={segmentEdge(segment.leftPercent, segment.widthPercent)}
                    x2={segmentEdge(segment.leftPercent, segment.widthPercent)}
                    y1="0"
                    y2="100"
                    stroke="rgba(255,255,255,0.3)"
                    stroke-width="1"
                  />
                </Show>
              </>
            )}
          </For>
          <Show when={presentation().showSwapBar}>
            <rect
              data-stacked-memory-swap="true"
              x="0"
              y="82"
              width={swapBarWidth()}
              height="18"
              rx="2"
              fill="rgb(168 85 247)"
            />
          </Show>
        </svg>

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
