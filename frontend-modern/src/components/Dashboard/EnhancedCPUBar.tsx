import { Show } from 'solid-js';
import { TooltipPortal } from '@/components/shared/TooltipPortal';
import { type EnhancedCPUBarProps } from './enhancedCpuBarModel';
import { useEnhancedCPUBarState } from './useEnhancedCPUBarState';

export function EnhancedCPUBar(props: EnhancedCPUBarProps) {
  const state = useEnhancedCPUBarState(props);
  const presentation = state.presentation;
  const barPercent = () => {
    const parsed = Number.parseFloat(presentation().barWidth);
    if (!Number.isFinite(parsed)) return '0';
    return String(Math.max(0, Math.min(parsed, 100)));
  };

  return (
    <div class="metric-text w-full h-4 flex items-center justify-center">
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
          <rect
            data-enhanced-cpu-fill="true"
            x="0"
            y="0"
            width={barPercent()}
            height="100"
            rx="3"
            fill={presentation().barFill}
          />
        </svg>

        <span class="absolute inset-0 flex items-center justify-center text-[10px] font-semibold text-base-content leading-none pointer-events-none">
          {presentation().displayUsage}
          <Show when={props.cores}>
            <span class="hidden sm:inline font-normal text-muted ml-1">({props.cores})</span>
          </Show>
          <Show when={presentation().hasAnomaly}>
            <span
              class={`ml-1 font-bold animate-pulse ${presentation().anomalyClass}`}
              title={presentation().anomalyDescription}
            >
              {presentation().anomalyRatio}
            </span>
          </Show>
        </span>
      </div>

      <TooltipPortal when={state.tooltipVisible()} x={state.tip.pos().x} y={state.tip.pos().y}>
        <div class="min-w-[160px]">
          <div class="font-medium mb-1 text-slate-300 border-b border-border pb-1">CPU Details</div>

          <Show when={props.model}>
            <div class="text-[9px] text-slate-400 mb-1.5 truncate max-w-[200px]">{props.model}</div>
          </Show>

          <div class="flex justify-between gap-3 py-0.5">
            <span class="text-slate-400">Usage</span>
            <span class={`font-medium ${presentation().tooltipUsageClass}`}>
              {presentation().displayUsage}
            </span>
          </div>

          <Show when={presentation().displayLoadAverage !== undefined}>
            <div class="flex justify-between gap-3 py-0.5">
              <span class="text-slate-400">Load (1m)</span>
              <span class="font-medium text-base-content">{presentation().displayLoadAverage}</span>
            </div>
          </Show>
        </div>
      </TooltipPortal>
    </div>
  );
}
