import { Show, For, createMemo, createSignal, onMount, onCleanup } from 'solid-js';
import { Portal } from 'solid-js/web';
import type { Disk } from '@/types/api';
import { formatBytes, formatPercent } from '@/utils/format';
import type { AnomalyReport } from '@/types/aiIntelligence';

interface StackedDiskBarProps {
  /** Array of disk objects - if empty/undefined, falls back to aggregate */
  disks?: Disk[];
  /** Aggregate disk data (fallback when disks array unavailable) */
  aggregateDisk?: Disk;
  /** Display mode for multi-disk hosts */
  mode?: 'stacked' | 'aggregate' | 'mini';
  /** Baseline anomaly if detected */
  anomaly?: AnomalyReport | null;
}

// Anomaly severity colors
const anomalySeverityClass: Record<string, string> = {
  critical: 'text-red-400',
  high: 'text-orange-400',
  medium: 'text-yellow-400',
  low: 'text-blue-400',
};

// Color palette for disk segments - distinct colors for visual differentiation
const SEGMENT_COLORS = [
  'rgba(34, 197, 94, 0.6)',   // green
  'rgba(59, 130, 246, 0.6)',  // blue
  'rgba(168, 85, 247, 0.6)',  // purple
  'rgba(249, 115, 22, 0.6)',  // orange
  'rgba(236, 72, 153, 0.6)',  // pink
  'rgba(20, 184, 166, 0.6)',  // teal
];

// Get color based on usage percentage
function getUsageColor(percentage: number): string {
  if (percentage >= 90) return 'rgba(239, 68, 68, 0.6)';  // red
  if (percentage >= 80) return 'rgba(234, 179, 8, 0.6)';  // yellow
  return 'rgba(34, 197, 94, 0.6)'; // green
}

// Estimate text width for label fitting
const estimateTextWidth = (text: string): number => {
  return text.length * 5.5 + 8;
};

export function StackedDiskBar(props: StackedDiskBarProps) {
  const [containerWidth, setContainerWidth] = createSignal(100);
  const [showTooltip, setShowTooltip] = createSignal(false);
  const [tooltipPos, setTooltipPos] = createSignal({ x: 0, y: 0 });
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

  // Format anomaly ratio for display
  const anomalyRatio = createMemo(() => {
    if (!props.anomaly || props.anomaly.baseline_mean === 0) return null;
    const ratio = props.anomaly.current_value / props.anomaly.baseline_mean;
    if (ratio >= 2) return `${ratio.toFixed(1)}x`;
    if (ratio >= 1.5) return '↑↑';
    return '↑';
  });

  // Determine if we have multiple disks or should use aggregate
  const hasMultipleDisks = createMemo(() => {
    const disks = props.disks;
    return disks && disks.length > 1;
  });

  const aggregateMode = createMemo(() => props.mode === 'aggregate');
  const miniMode = createMemo(() => props.mode === 'mini');
  const useStackedSegments = createMemo(() => hasMultipleDisks() && !aggregateMode() && !miniMode());

  // Calculate total capacity across all disks
  const totalCapacity = createMemo(() => {
    if (!props.disks || props.disks.length === 0) {
      return props.aggregateDisk?.total ?? 0;
    }
    return props.disks.reduce((sum, d) => sum + (d.total || 0), 0);
  });

  const totalUsed = createMemo(() => {
    if (!props.disks || props.disks.length === 0) {
      return props.aggregateDisk?.used ?? 0;
    }
    return props.disks.reduce((sum, d) => sum + (d.used || 0), 0);
  });

  const overallPercent = createMemo(() => {
    const total = totalCapacity();
    if (total <= 0) return 0;
    return (totalUsed() / total) * 100;
  });

  const barPercent = createMemo(() => Math.min(overallPercent(), 100));

  // Calculate segment widths for stacked bar (each disk's used space as % of total capacity)
  const segments = createMemo(() => {
    if (!useStackedSegments() || !props.disks) return [];

    const total = totalCapacity();
    if (total <= 0) return [];

    return props.disks.map((disk, idx) => {
      const usedPercent = (disk.used / total) * 100;
      const diskPercent = disk.total > 0 ? (disk.used / disk.total) * 100 : 0;
      // Use warning/critical colors for high usage, otherwise use the color palette
      const color = diskPercent >= 90 ? getUsageColor(90) :
        diskPercent >= 80 ? getUsageColor(80) :
          SEGMENT_COLORS[idx % SEGMENT_COLORS.length];
      return {
        disk,
        widthPercent: Math.min(usedPercent, 100),
        diskUsagePercent: diskPercent,
        color,
        index: idx,
      };
    });
  });

  // Check if we have any disk details to show
  const hasDisks = createMemo(() => {
    return props.disks && props.disks.length > 0;
  });

  const miniDisks = createMemo(() => {
    if (!props.disks) return [];
    return props.disks.map((disk, idx) => {
      const percent = disk.total > 0 ? (disk.used / disk.total) * 100 : 0;
      const label = disk.mountpoint || disk.device || `Disk ${idx + 1}`;
      return {
        label,
        percent,
        color: getUsageColor(percent),
      };
    });
  });

  const maxDiskInfo = createMemo(() => {
    if (!props.disks || props.disks.length === 0) return null;
    let maxPercent = -1;
    let maxLabel = '';
    for (const disk of props.disks) {
      const percent = disk.total > 0 ? (disk.used / disk.total) * 100 : 0;
      if (percent > maxPercent) {
        maxPercent = percent;
        maxLabel = disk.mountpoint || disk.device || 'Disk';
      }
    }
    if (maxPercent < 0) return null;
    return { percent: maxPercent, label: maxLabel };
  });

  const maxLabelShort = createMemo(() => {
    const info = maxDiskInfo();
    if (!info) return '';
    return `max ${formatPercent(info.percent)}`;
  });

  const maxLabelFull = createMemo(() => {
    const info = maxDiskInfo();
    if (!info) return '';
    if (info.label) return `Max ${formatPercent(info.percent)} (${info.label})`;
    return `Max ${formatPercent(info.percent)}`;
  });

  const barColor = createMemo(() => {
    const info = maxDiskInfo();
    if (aggregateMode() && hasMultipleDisks() && info) {
      return getUsageColor(info.percent);
    }
    return getUsageColor(overallPercent());
  });

  // Generate tooltip content
  const tooltipContent = createMemo(() => {
    const useUsageColors = aggregateMode() || miniMode();
    if (hasDisks() && props.disks) {
      return props.disks.map((disk, idx) => {
        const percent = disk.total > 0 ? (disk.used / disk.total) * 100 : 0;
        const label = disk.mountpoint || disk.device || `Disk ${idx + 1}`;
        return {
          label,
          used: formatBytes(disk.used),
          total: formatBytes(disk.total),
          percent: formatPercent(percent),
          color: useUsageColors
            ? getUsageColor(percent)
            : percent >= 90
              ? getUsageColor(90)
              : percent >= 80
                ? getUsageColor(80)
                : SEGMENT_COLORS[idx % SEGMENT_COLORS.length],
        };
      });
    }
    // Fallback for aggregate disk
    if (props.aggregateDisk && props.aggregateDisk.total > 0) {
      const percent = (props.aggregateDisk.used / props.aggregateDisk.total) * 100;
      return [{
        label: 'Total',
        used: formatBytes(props.aggregateDisk.used),
        total: formatBytes(props.aggregateDisk.total),
        percent: formatPercent(percent),
        color: getUsageColor(percent),
      }];
    }
    return [];
  });

  // Label for the bar
  const displayLabel = createMemo(() => {
    return formatPercent(overallPercent());
  });

  // Sublabel showing used/total
  const displaySublabel = createMemo(() => {
    return `${formatBytes(totalUsed())}/${formatBytes(totalCapacity())}`;
  });

  const showMaxLabel = createMemo(() => {
    if (!aggregateMode() || !hasMultipleDisks()) return false;
    const shortLabel = maxLabelShort();
    if (!shortLabel) return false;
    const fullText = `${displayLabel()} ${shortLabel}`;
    return containerWidth() >= estimateTextWidth(fullText);
  });

  // Check if sublabel fits
  const showSublabel = createMemo(() => {
    const baseText = `${displayLabel()}${showMaxLabel() ? ` ${maxLabelShort()}` : ''}`;
    const fullText = `${baseText} (${displaySublabel()})`;
    return containerWidth() >= estimateTextWidth(fullText);
  });

  const containerClass = createMemo(() => {
    return miniMode() && hasDisks()
      ? 'metric-text w-full'
      : 'metric-text w-full h-4 flex items-center justify-center';
  });

  const handleMouseEnter = (e: MouseEvent) => {
    if (tooltipContent().length > 0) {
      const rect = (e.currentTarget as HTMLElement).getBoundingClientRect();
      setTooltipPos({ x: rect.left + rect.width / 2, y: rect.top });
      setShowTooltip(true);
    }
  };

  const handleMouseLeave = () => {
    setShowTooltip(false);
  };

  return (
    <div ref={containerRef} class={containerClass()}>
      <Show
        when={miniMode() && hasDisks()}
        fallback={
          <div
            class="relative w-full h-full overflow-hidden bg-gray-200 dark:bg-gray-600 rounded"
            onMouseEnter={handleMouseEnter}
            onMouseLeave={handleMouseLeave}
          >
            {/* Stacked segments for multiple disks */}
            <Show when={useStackedSegments()}>
              <div class="absolute top-0 left-0 h-full w-full flex">
                <For each={segments()}>
                  {(segment, idx) => (
                    <div
                      class="h-full"
                      style={{
                        width: `${segment.widthPercent}%`,
                        'background-color': segment.color,
                        'border-right': idx() < segments().length - 1 ? '1px solid rgba(255,255,255,0.3)' : 'none',
                      }}
                    />
                  )}
                </For>
              </div>
            </Show>

            {/* Single bar for aggregate or single disk */}
            <Show when={!useStackedSegments()}>
              <div
                class="absolute top-0 left-0 h-full"
                style={{
                  width: `${barPercent()}%`,
                  'background-color': barColor(),
                }}
              />
            </Show>

            {/* Label overlay */}
            <span class="absolute inset-0 flex items-center justify-center text-[10px] font-semibold text-gray-700 dark:text-gray-100 leading-none">
              <span class="flex items-center gap-1 whitespace-nowrap px-0.5">
                <span>{displayLabel()}</span>
                <Show when={showMaxLabel()}>
                  <span
                    class="text-[8px] font-normal text-gray-500 dark:text-gray-400"
                    title={maxLabelFull()}
                  >
                    {maxLabelShort()}
                  </span>
                </Show>
                <Show when={showSublabel()}>
                  <span class="metric-sublabel font-normal text-gray-500 dark:text-gray-300">
                    ({displaySublabel()})
                  </span>
                </Show>
                <Show when={useStackedSegments()}>
                  <span class="text-[8px] font-normal text-gray-500 dark:text-gray-400">
                    [{props.disks?.length}]
                  </span>
                </Show>
                {/* Anomaly indicator */}
                <Show when={props.anomaly && anomalyRatio()}>
                  <span
                    class={`ml-0.5 font-bold animate-pulse ${anomalySeverityClass[props.anomaly!.severity] || 'text-yellow-400'}`}
                    title={props.anomaly!.description}
                  >
                    {anomalyRatio()}
                  </span>
                </Show>
              </span>
            </span>
          </div>
        }
      >
        <div class="w-full" onMouseEnter={handleMouseEnter} onMouseLeave={handleMouseLeave}>
          <div
            class="grid gap-1"
            style={{
              'grid-template-columns': `repeat(${miniDisks().length}, minmax(0, 1fr))`,
            }}
          >
            <For each={miniDisks()}>
              {(disk) => (
                <div class="flex flex-col items-stretch gap-0.5">
                  <span class="text-[8px] text-gray-500 dark:text-gray-400 truncate" title={disk.label}>
                    {disk.label}
                  </span>
                  <div class="relative h-2.5 rounded-sm bg-gray-300/70 dark:bg-gray-500/70 overflow-hidden">
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
      <Show when={showTooltip() && tooltipContent().length > 0}>
        <Portal mount={document.body}>
          <div
            class="fixed z-[9999] pointer-events-none"
            style={{
              left: `${tooltipPos().x}px`,
              top: `${tooltipPos().y - 8}px`,
              transform: 'translate(-50%, -100%)',
            }}
          >
            <div class="bg-gray-900 dark:bg-gray-800 text-white text-[10px] rounded-md shadow-lg px-2 py-1.5 min-w-[140px] border border-gray-700">
              <div class="font-medium mb-1 text-gray-300 border-b border-gray-700 pb-1">
                {hasMultipleDisks() ? 'Disk Breakdown' : 'Disk Usage'}
              </div>
              <For each={tooltipContent()}>
                {(item, idx) => (
                  <div class="flex flex-col gap-1 py-0.5" classList={{ 'border-t border-gray-700/50': idx() > 0 }}>
                    <div class="flex justify-between gap-3">
                      <span
                        class="truncate max-w-[100px]"
                        style={{ color: item.color }}
                      >
                        {item.label}
                      </span>
                      <span class="whitespace-nowrap text-gray-300">
                        {item.percent} ({item.used}/{item.total})
                      </span>
                    </div>
                    <div class="h-1.5 w-full rounded bg-gray-700/70 overflow-hidden">
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
          </div>
        </Portal>
      </Show>
    </div>
  );
}
