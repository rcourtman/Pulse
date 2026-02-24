import { Show, For, createMemo, createSignal, onMount, onCleanup } from 'solid-js';
import { useTooltip } from '@/hooks/useTooltip';
import { TooltipPortal } from '@/components/shared/TooltipPortal';
import {
  formatBytes,
  formatPercent,
  estimateTextWidth,
  formatAnomalyRatio,
  ANOMALY_SEVERITY_CLASS,
} from '@/utils/format';
import { getMetricColorRgba } from '@/utils/metricThresholds';
import type { AnomalyReport } from '@/types/aiIntelligence';

interface StackedMemoryBarProps {
  used: number;
  total: number;
  percentOnly?: number;
  swapUsed?: number;
  swapTotal?: number;
  balloon?: number;
  resourceId?: string;
  anomaly?: AnomalyReport | null; // Baseline anomaly if detected
}

// Colors for memory segments
const MEMORY_COLORS = {
  active: 'rgba(34, 197, 94, 0.6)', // green (base, overridden by threshold)
  balloon: 'rgba(59, 130, 246, 0.6)', // blue
  swap: 'rgba(168, 85, 247, 0.6)', // purple
};

export function StackedMemoryBar(props: StackedMemoryBarProps) {
  const anomalyRatio = createMemo(() => formatAnomalyRatio(props.anomaly));
  const utilizationPercent = createMemo(() => {
    if (props.total > 0) {
      return (props.used / props.total) * 100;
    }
    if (Number.isFinite(props.percentOnly)) {
      return Math.max(0, Math.min(props.percentOnly as number, 100));
    }
    return 0;
  });

  const tip = useTooltip();
  const [containerWidth, setContainerWidth] = createSignal(100);
  let containerRef: HTMLDivElement | undefined;

  onMount(() => {
    if (!containerRef) return;
    setContainerWidth(containerRef.offsetWidth);

    const observer = new ResizeObserver((entries) => {
      for (const entry of entries) {
        setContainerWidth(entry.contentRect.width);
      }
    });

    observer.observe(containerRef);
    onCleanup(() => observer.disconnect());
  });

  // Calculate segments
  const segments = createMemo(() => {
    if (props.total <= 0) {
      const percent = utilizationPercent();
      if (percent <= 0) return [];
      return [
        { type: 'Utilization', bytes: 0, percent, color: getMetricColorRgba(percent, 'memory') },
      ];
    }

    const balloon = props.balloon || 0;

    // Proxmox balloon semantics:
    // - balloon = 0: ballooning not enabled/configured
    // - balloon = total (maxmem): ballooning configured but at max (no actual ballooning)
    // - balloon < total: active ballooning, guest limited to 'balloon' bytes
    //
    // Only show balloon segment when actual ballooning is in effect
    const hasActiveBallooning = balloon > 0 && balloon < props.total;

    // Used memory is what the guest is actually consuming
    const usedPercent = (props.used / props.total) * 100;

    if (hasActiveBallooning) {
      // With active ballooning:
      // - Green: actual used memory
      // - Yellow: balloon limit marker (shows where the guest is capped)
      // The balloon limit shows as a segment from used to balloon
      const balloonLimitPercent = Math.max(0, (balloon / props.total) * 100 - usedPercent);

      const segs = [
        {
          type: 'Active',
          bytes: props.used,
          percent: usedPercent,
          color: getMetricColorRgba(usedPercent, 'memory'),
        },
      ];

      // Only show balloon segment if there's room between used and balloon limit
      if (balloonLimitPercent > 0 && balloon > props.used) {
        segs.push({
          type: 'Balloon',
          bytes: balloon - props.used,
          percent: balloonLimitPercent,
          color: MEMORY_COLORS.balloon,
        });
      }

      return segs.filter((s) => s.bytes > 0);
    }

    // No active ballooning - show used memory with threshold-based coloring
    return [
      {
        type: 'Active',
        bytes: props.used,
        percent: usedPercent,
        color: getMetricColorRgba(usedPercent, 'memory'),
      },
    ].filter((s) => s.bytes > 0);
  });

  const swapPercent = createMemo(() => {
    if (!props.swapTotal || props.swapTotal <= 0) return 0;
    return ((props.swapUsed || 0) / props.swapTotal) * 100;
  });

  const hasSwap = createMemo(() => (props.swapTotal || 0) > 0);

  const displayLabel = createMemo(() => {
    return formatPercent(utilizationPercent());
  });

  const displaySublabel = createMemo(() => {
    if (props.total <= 0) return '';
    return `${formatBytes(props.used)}/${formatBytes(props.total)}`;
  });

  const showSublabel = createMemo(() => {
    if (!displaySublabel()) return false;
    const fullText = `${displayLabel()} (${displaySublabel()})`;
    return containerWidth() >= estimateTextWidth(fullText);
  });

  return (
    <div ref={containerRef} class="metric-text w-full h-4 flex items-center justify-center">
      <div
        class="relative w-full h-full overflow-hidden bg-surface-hover rounded"
        onMouseEnter={tip.onMouseEnter}
        onMouseLeave={tip.onMouseLeave}
      >
        {/* Stacked segments - use absolute positioning like MetricBar for correct width scaling */}
        <For each={segments()}>
          {(segment, idx) => {
            // Calculate left offset as sum of previous segments
            const leftOffset = () => {
              let offset = 0;
              const segs = segments();
              for (let i = 0; i < idx(); i++) {
                offset += segs[i].percent;
              }
              return offset;
            };
            return (
              <div
                class="absolute top-0 h-full transition-all duration-300"
                style={{
                  left: `${leftOffset()}%`,
                  width: `${segment.percent}%`,
                  'background-color': segment.color,
                  'border-right':
                    idx() < segments().length - 1 ? '1px solid rgba(255,255,255,0.3)' : 'none',
                }}
              />
            );
          }}
        </For>

        {/* Swap Indicator (Thin line at bottom if swap is used) */}
        <Show when={hasSwap() && (props.swapUsed || 0) > 0}>
          <div
            class="absolute bottom-0 left-0 h-[3px] w-full bg-purple-500"
            style={{ width: `${Math.min(swapPercent(), 100)}%` }}
          />
        </Show>

        {/* Label overlay */}
        <span class="absolute inset-0 flex items-center justify-center text-[10px] font-semibold text-base-content leading-none pointer-events-none min-w-0 overflow-hidden">
          <span class="max-w-full min-w-0 whitespace-nowrap overflow-hidden text-ellipsis px-0.5 text-center">
            <span>{displayLabel()}</span>
            <Show when={showSublabel()}>
              <span class="metric-sublabel font-normal text-muted"> ({displaySublabel()})</span>
            </Show>
            {/* Anomaly indicator */}
            <Show when={props.anomaly && anomalyRatio()}>
              <span
                class={`ml-0.5 font-bold animate-pulse ${ANOMALY_SEVERITY_CLASS[props.anomaly!.severity] || 'text-yellow-400'}`}
                title={props.anomaly!.description}
              >
                {anomalyRatio()}
              </span>
            </Show>
          </span>
        </span>
      </div>

      {/* Tooltip */}
      <TooltipPortal when={tip.show()} x={tip.pos().x} y={tip.pos().y}>
        <div class="min-w-[140px]">
          <div class="font-medium mb-1 text-slate-300 border-b border-border pb-1">
            Memory Composition
          </div>

          <Show when={props.total > 0}>
            <div class="flex justify-between gap-3 py-0.5">
              <span class="text-green-400">Used</span>
              <span class="whitespace-nowrap text-slate-300">{formatBytes(props.used)}</span>
            </div>
          </Show>

          <Show
            when={props.total > 0 && (props.balloon || 0) > 0 && (props.balloon || 0) < props.total}
          >
            <div class="flex justify-between gap-3 py-0.5 border-t border-border">
              <span class="text-blue-400">Balloon Limit</span>
              <span class="whitespace-nowrap text-slate-300">
                {formatBytes(props.balloon || 0)}
              </span>
            </div>
          </Show>

          <Show
            when={props.total > 0}
            fallback={
              <div class="flex justify-between gap-3 py-0.5 border-t border-border">
                <span class="text-blue-300">Utilization</span>
                <span class="whitespace-nowrap text-slate-300">{displayLabel()}</span>
              </div>
            }
          >
            <div class="flex justify-between gap-3 py-0.5 border-t border-border">
              <span class="text-slate-400">Free</span>
              <span class="whitespace-nowrap text-slate-300">
                {formatBytes(props.total - props.used)}
              </span>
            </div>
          </Show>

          <Show when={props.total > 0 && hasSwap()}>
            <div class="mt-1 pt-1 border-t border-slate-600">
              <div class="flex justify-between gap-3 py-0.5">
                <span class="text-purple-400">Swap</span>
                <span class="whitespace-nowrap text-slate-300">
                  {formatBytes(props.swapUsed || 0)} / {formatBytes(props.swapTotal || 0)}
                </span>
              </div>
            </div>
          </Show>
        </div>
      </TooltipPortal>
    </div>
  );
}
