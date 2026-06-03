import { Show, For } from 'solid-js';
import { AnimatedNumber } from '@/components/shared/AnimatedNumber';
import { TooltipPortal } from '@/components/shared/TooltipPortal';
import { formatPercent } from '@/utils/format';
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
  const invertPercent = (value: number) => String(100 - Math.max(0, Math.min(value, 100)));

  return (
    <div ref={state.setContainerRef} class={presentation().containerClass}>
      <Show
        when={presentation().verticalBarsMode}
        fallback={
          <Show
            when={presentation().inlineDiskMode && presentation().hasDisks}
            fallback={
              <div
                data-stacked-disk-trigger
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
                            class="metric-fill-geometry"
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
                              class="metric-fill-divider"
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
                      class="metric-fill-geometry"
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
                    <span>
                      <AnimatedNumber
                        value={presentation().displayPercentValue}
                        format={formatPercent}
                      />
                    </span>
                    <Show when={presentation().showMaxLabel}>
                      <span
                        class="text-[8px] font-normal text-muted"
                        title={presentation().maxLabelFull}
                      >
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
                      <span
                        class="text-[8px] font-normal text-muted"
                        title={`${props.disks?.length ?? 0} disks`}
                      >
                        {' '}
                        [{props.disks?.length}]
                      </span>
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
            <div
              data-stacked-disk-trigger
              class="h-full w-full"
              onMouseEnter={state.handleMouseEnter}
              onMouseLeave={state.handleMouseLeave}
            >
              <div class="flex h-full items-stretch gap-0.5">
                <For each={presentation().miniDisks}>
                  {(disk) => (
                    <div
                      class="relative min-w-0 flex-1 overflow-hidden rounded-sm bg-surface-alt"
                      title={disk.title}
                    >
                      <svg
                        aria-hidden="true"
                        class="absolute inset-0 h-full w-full"
                        viewBox="0 0 100 100"
                        preserveAspectRatio="none"
                      >
                        <rect
                          data-stacked-disk-fill="inline"
                          class="metric-fill-geometry"
                          x="0"
                          y="0"
                          width={clampPercent(disk.percent)}
                          height="100"
                          rx="2"
                          fill={disk.color}
                        />
                      </svg>
                      <span class="absolute inset-0 flex min-w-0 items-center justify-center overflow-hidden px-px text-center text-[9px] font-semibold leading-none text-base-content">
                        <span class="min-w-0 truncate">{disk.inlineText}</span>
                      </span>
                    </div>
                  )}
                </For>
              </div>
            </div>
          </Show>
        }
      >
        {/* Vertical micro-bars: one per disk, fill height = utilization, color = threshold */}
        <div
          data-stacked-disk-trigger
          class="h-full w-full"
          onMouseEnter={state.handleMouseEnter}
          onMouseLeave={state.handleMouseLeave}
        >
          <div class="flex h-full items-end justify-center gap-[3px]">
            <For each={presentation().verticalBars}>
              {(bar) => (
                <div
                  class="group/disk relative h-full w-[5px] shrink-0 cursor-help overflow-hidden rounded-sm bg-surface-hover ring-1 ring-transparent transition-[transform,box-shadow,ring-color] duration-100 hover:scale-y-[1.08] hover:ring-base-content/30"
                  title={bar.title}
                >
                  <div
                    data-stacked-disk-fill="vertical"
                    class="absolute inset-0 rounded-sm brightness-100 transition-[filter] duration-100 group-hover/disk:brightness-125"
                  >
                    <svg
                      aria-hidden="true"
                      class="absolute inset-0 h-full w-full"
                      viewBox="0 0 100 100"
                      preserveAspectRatio="none"
                    >
                      <rect
                        class="metric-fill-geometry"
                        x="0"
                        y={invertPercent(bar.fillPercent)}
                        width="100"
                        height={clampPercent(bar.fillPercent)}
                        rx="2"
                        fill={bar.color}
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
        maxWidth={420}
      >
        <div class="w-[360px] max-w-full">
          <div class="font-medium mb-1 text-slate-300 border-b border-border pb-1">
            {presentation().tooltipTitle}
          </div>
          <For each={presentation().tooltipContent}>
            {(item, idx) => (
              <div
                class="flex flex-col gap-1 py-0.5"
                classList={{ 'border-t border-border': idx() > 0 }}
              >
                <div class="grid grid-cols-[minmax(0,1fr)_auto] gap-x-3 gap-y-0.5">
                  <span class="flex min-w-0 items-start gap-1.5 text-slate-300">
                    <svg aria-hidden="true" class="mt-0.5 h-2 w-2 shrink-0" viewBox="0 0 8 8">
                      <circle cx="4" cy="4" r="4" fill={item.color} />
                    </svg>
                    <span class="min-w-0 break-all leading-snug">{item.label}</span>
                  </span>
                  <span class="whitespace-nowrap text-right font-medium text-base-content">
                    {item.percent}
                  </span>
                  <span class="col-span-2 text-right text-[9px] leading-none text-muted">
                    {item.used}/{item.total}
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
                      class="metric-fill-geometry"
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
