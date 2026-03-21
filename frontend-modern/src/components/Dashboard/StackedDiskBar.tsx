import { Show, For } from 'solid-js';
import { TooltipPortal } from '@/components/shared/TooltipPortal';
import { type StackedDiskBarProps } from './stackedDiskBarModel';
import { useStackedDiskBarState } from './useStackedDiskBarState';

export function StackedDiskBar(props: StackedDiskBarProps) {
  const state = useStackedDiskBarState(props);
  const presentation = state.presentation;

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
              <div class="absolute top-0 left-0 h-full w-full flex">
                <For each={presentation().segments}>
                  {(segment, idx) => (
                    <div
                      class="h-full"
                      style={{
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
              </div>
            </Show>

            {/* Single bar for aggregate or single disk */}
            <Show when={!presentation().useStackedSegments}>
              <div
                class="absolute top-0 left-0 h-full"
                style={{
                  width: `${presentation().barPercent}%`,
                  'background-color': presentation().barColor,
                }}
              />
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
          <div
            class="grid gap-1"
            style={{
              'grid-template-columns': `repeat(${presentation().miniDisks.length}, minmax(0, 1fr))`,
            }}
          >
            <For each={presentation().miniDisks}>
              {(disk) => (
                <div class="flex flex-col items-stretch gap-0.5">
                  <span class="text-[8px] text-muted truncate" title={disk.label}>
                    {disk.label}
                  </span>
                  <div class="relative h-2.5 rounded-sm bg-surface-alt overflow-hidden">
                    <div
                      class="h-full"
                      style={{
                        width: `${Math.min(disk.percent, 100)}%`,
                        'background-color': disk.color,
                      }}
                    />
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
                  <span class="truncate max-w-[100px]" style={{ color: item.color }}>
                    {item.label}
                  </span>
                  <span class="whitespace-nowrap text-slate-300">
                    {item.percent} ({item.used}/{item.total})
                  </span>
                </div>
                <div class="h-1.5 w-full rounded bg-surface-hover overflow-hidden">
                  <div
                    class="h-full"
                    style={{
                      width: item.percent,
                      'background-color': item.color,
                    }}
                  />
                </div>
              </div>
            )}
          </For>
        </div>
      </TooltipPortal>
    </div>
  );
}
