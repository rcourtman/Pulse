import { Show, For, createSignal } from 'solid-js';
import { AnimatedNumber } from '@/components/shared/AnimatedNumber';
import { Dialog } from '@/components/shared/Dialog';
import { TooltipPortal } from '@/components/shared/TooltipPortal';
import { formatPercent } from '@/utils/format';
import { type StackedMemoryBarProps } from './stackedMemoryBarModel';
import { useStackedMemoryBarState } from './useStackedMemoryBarState';

export function StackedMemoryBar(props: StackedMemoryBarProps) {
  const state = useStackedMemoryBarState(props);
  const presentation = state.presentation;
  const [detailsOpen, setDetailsOpen] = createSignal(false);
  let detailsTrigger: HTMLButtonElement | undefined;
  const swapBarWidth = () => String(Math.max(0, Math.min(presentation().swapBarPercent, 100)));
  const segmentEdge = (leftPercent: number, widthPercent: number) =>
    String(Math.max(0, Math.min(leftPercent + widthPercent, 100)));

  return (
    <div
      ref={state.setContainerRef}
      class={`metric-text relative w-full flex items-center justify-center ${props.detailsLabel ? 'h-11 sm:h-6' : 'h-4'}`}
      onMouseEnter={state.handleMouseEnter}
      onMouseLeave={state.handleMouseLeave}
      onClick={(event) => {
        // Portal dismissal must not reach a table row's separate action.
        if (props.detailsLabel) event.stopPropagation();
      }}
    >
      <div class="relative w-full h-4 overflow-hidden bg-surface-hover rounded-sm">
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
                  class="metric-fill-geometry"
                  x={String(Math.max(0, Math.min(segment.leftPercent, 100)))}
                  y="0"
                  width={String(Math.max(0, segment.widthPercent))}
                  height="100"
                  rx="3"
                  fill={segment.color}
                />
                <Show when={idx() < presentation().segments.length - 1}>
                  <line
                    class="metric-fill-divider"
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
              class="metric-fill-geometry"
              x="0"
              y="82"
              width={swapBarWidth()}
              height="18"
              rx="2"
              fill={presentation().swapBarColor}
            />
          </Show>
        </svg>

        <span class="absolute inset-0 flex items-center justify-center text-[10px] font-semibold text-base-content leading-none pointer-events-none min-w-0 overflow-hidden">
          <span class="max-w-full min-w-0 whitespace-nowrap overflow-hidden text-ellipsis rounded-xs px-1 text-center shadow-xs">
            <span>
              <Show
                when={!presentation().unavailable}
                fallback={<span class="text-muted">N/A</span>}
              >
                <AnimatedNumber value={presentation().displayPercentValue} format={formatPercent} />
              </Show>
            </span>
            <Show when={presentation().showSublabel}>
              <span class="metric-sublabel font-normal"> ({presentation().displaySublabel})</span>
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

      <Show when={props.detailsLabel}>
        {(label) => (
          <>
            <button
              ref={detailsTrigger}
              type="button"
              class="absolute inset-0 w-full cursor-pointer rounded-sm focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-blue-500"
              aria-label={label()}
              aria-haspopup="dialog"
              aria-expanded={detailsOpen()}
              onClick={(event) => {
                event.stopPropagation();
                state.tip.setShow(false);
                setDetailsOpen(true);
              }}
            />
            <Dialog
              isOpen={detailsOpen()}
              onClose={() => setDetailsOpen(false)}
              returnFocus={() => detailsTrigger}
              ariaLabel={label()}
              panelClass="max-w-sm"
            >
              <div class="overflow-y-auto p-4 text-sm">
                <h2 class="mb-3 whitespace-normal font-semibold text-base-content">{label()}</h2>
                <Show when={presentation().tooltipMessage}>
                  <p class="mb-3 whitespace-normal text-muted">{presentation().tooltipMessage}</p>
                </Show>
                <dl>
                  <For each={presentation().tooltipRows}>
                    {(row) => (
                      <div
                        class="flex flex-wrap justify-between gap-x-4 gap-y-1 py-2"
                        classList={{ 'border-t border-border': row.borderTop }}
                      >
                        <dt class={`whitespace-normal ${row.labelClass}`}>{row.label}</dt>
                        <dd class="whitespace-nowrap text-base-content">{row.value}</dd>
                      </div>
                    )}
                  </For>
                </dl>
                <button
                  type="button"
                  class="mt-3 min-h-11 w-full rounded-sm border border-border px-3 text-base-content hover:bg-surface-hover focus-visible:outline-2 focus-visible:outline-blue-500"
                  onClick={() => setDetailsOpen(false)}
                >
                  Close
                </button>
              </div>
            </Dialog>
          </>
        )}
      </Show>

      <TooltipPortal
        when={!detailsOpen() && state.tooltipVisible()}
        x={state.tip.pos().x}
        y={state.tip.pos().y}
      >
        <div class="min-w-[140px]">
          <div class="font-medium mb-1 text-base-content border-b border-border pb-1">
            {presentation().tooltipTitle}
          </div>
          <Show when={presentation().tooltipMessage}>
            <p class="mb-2 max-w-xs text-xs text-muted">{presentation().tooltipMessage}</p>
          </Show>
          <For each={presentation().tooltipRows}>
            {(row) => (
              <div
                class="flex justify-between gap-3 py-0.5"
                classList={{ 'border-t border-border': row.borderTop }}
              >
                <span class={row.labelClass}>{row.label}</span>
                <span class="whitespace-nowrap text-base-content">{row.value}</span>
              </div>
            )}
          </For>
        </div>
      </TooltipPortal>
    </div>
  );
}
