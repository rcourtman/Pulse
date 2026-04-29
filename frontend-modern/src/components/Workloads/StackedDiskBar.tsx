import { Show, For } from 'solid-js';
import { TooltipPortal } from '@/components/shared/TooltipPortal';
import { type StackedDiskBarProps } from './stackedDiskBarModel';
import { useStackedDiskBarState } from './useStackedDiskBarState';

export function StackedDiskBar(props: StackedDiskBarProps) {
  const state = useStackedDiskBarState(props);
  const presentation = state.presentation;
  const clampPercent = (value: number) => String(Math.max(0, Math.min(value, 100)));
  const parsePercent = (value: string) => {
    const parsed = Number.parseFloat(value);
    if (!Number.isFinite(parsed)) return '0';
    return String(Math.max(0, Math.min(parsed, 100)));
  };

  return (
    <div ref={state.setContainerRef} class={presentation().containerClass}>
      <Show
        when={presentation().miniMode && presentation().hasDisks}
        fallback={
          <div
            class="relative w-full h-full overflow-hidden bg-surface-hover rounded"
            onMouseEnter={state.handleMouseEnter}
            onMouseLeave={state.handleMouseLeave}
          >
            {/* Stacked segments for multiple disks */}
            <Show when={presentation().useStackedSegments}>
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
                        data-stacked-disk-fill="segment"
                        x={String(
                          presentation()
                            .segments.slice(0, idx())
                            .reduce((sum, item) => sum + item.widthPercent, 0),
                        )}
                        y="0"
                        width={clampPercent(segment.widthPercent)}
                        height="100"
                        rx="3"
                        fill={segment.color}
                      />
                      <Show when={idx() < presentation().segments.length - 1}>
                        <line
                          x1={String(
                            presentation()
                              .segments.slice(0, idx() + 1)
                              .reduce((sum, item) => sum + item.widthPercent, 0),
                          )}
                          x2={String(
                            presentation()
                              .segments.slice(0, idx() + 1)
                              .reduce((sum, item) => sum + item.widthPercent, 0),
                          )}
                          y1="0"
                          y2="100"
                          stroke="rgba(255,255,255,0.3)"
                          stroke-width="1"
                        />
                      </Show>
                    </>
                  )}
                </For>
              </svg>
            </Show>

            {/* Single bar for aggregate or single disk */}
            <Show when={!presentation().useStackedSegments}>
              <svg
                aria-hidden="true"
                class="absolute inset-0 h-full w-full"
                viewBox="0 0 100 100"
                preserveAspectRatio="none"
              >
                <rect
                  data-stacked-disk-fill="single"
                  x="0"
                  y="0"
                  width={clampPercent(presentation().barPercent)}
                  height="100"
                  rx="3"
                  fill={presentation().barColor}
                />
              </svg>
            </Show>

            {/* Label overlay */}
            <span class="absolute inset-0 flex items-center justify-center text-[10px] font-semibold text-base-content leading-none min-w-0 overflow-hidden">
              <span class="max-w-full min-w-0 whitespace-nowrap overflow-hidden text-ellipsis px-0.5 text-center">
                <span>{presentation().displayLabel}</span>
                <Show when={presentation().showMaxLabel}>
                  <span class="text-[8px] font-normal text-muted" title={presentation().maxLabelFull}>
                    {' '}
                    {presentation().maxLabelShort}
                  </span>
                </Show>
                <Show when={presentation().showSublabel}>
                  <span class="metric-sublabel font-normal text-muted">
                    {' '}
                    ({presentation().displaySublabel})
                  </span>
                </Show>
                <Show when={presentation().showDiskCount}>
                  <span class="text-[8px] font-normal text-muted"> [{props.disks?.length}]</span>
                </Show>
                {/* Anomaly indicator */}
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
        }
      >
        <div class="w-full" onMouseEnter={state.handleMouseEnter} onMouseLeave={state.handleMouseLeave}>
          <div class="flex items-stretch gap-1">
            <For each={presentation().miniDisks}>
              {(disk) => (
                <div class="flex min-w-0 flex-1 flex-col items-stretch gap-0.5">
                  <span class="text-[8px] text-muted truncate" title={disk.label}>
                    {disk.label}
                  </span>
                  <div class="relative h-2.5 rounded-sm bg-surface-alt overflow-hidden">
                    <svg
                      aria-hidden="true"
                      class="absolute inset-0 h-full w-full"
                      viewBox="0 0 100 100"
                      preserveAspectRatio="none"
                    >
                      <rect
                        data-stacked-disk-fill="mini"
                        x="0"
                        y="0"
                        width={clampPercent(disk.percent)}
                        height="100"
                        rx="2"
                        fill={disk.color}
                      />
                    </svg>
                  </div>
                </div>
              )}
            </For>
          </div>
        </div>
      </Show>

      {/* Tooltip for disk breakdown */}
      <TooltipPortal
        when={state.tooltipVisible()}
        x={state.tip.pos().x}
        y={state.tip.pos().y}
      >
        <div class="min-w-[140px]">
          <div class="font-medium mb-1 text-slate-300 border-b border-border pb-1">
            {presentation().tooltipTitle}
          </div>
          <For each={presentation().tooltipContent}>
            {(item, idx) => (
              <div
                class="flex flex-col gap-1 py-0.5"
                classList={{ 'border-t border-border': idx() > 0 }}
              >
                <div class="flex justify-between gap-3">
                  <span class="flex max-w-[100px] items-center gap-1 truncate text-slate-300">
                    <svg aria-hidden="true" class="h-2 w-2 shrink-0" viewBox="0 0 8 8">
                      <circle cx="4" cy="4" r="4" fill={item.color} />
                    </svg>
                    <span class="truncate">{item.label}</span>
                  </span>
                  <span class="whitespace-nowrap text-slate-300">
                    {item.percent} ({item.used}/{item.total})
                  </span>
                </div>
                <div class="relative h-1.5 w-full overflow-hidden rounded bg-surface-hover">
                  <svg
                    aria-hidden="true"
                    class="absolute inset-0 h-full w-full"
                    viewBox="0 0 100 100"
                    preserveAspectRatio="none"
                  >
                    <rect
                      data-stacked-disk-fill="tooltip"
                      x="0"
                      y="0"
                      width={parsePercent(item.percent)}
                      height="100"
                      rx="2"
                      fill={item.color}
                    />
                  </svg>
                </div>
              </div>
            )}
          </For>
        </div>
      </TooltipPortal>
    </div>
  );
}
