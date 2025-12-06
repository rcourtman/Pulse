import { Show, For, createMemo, createSignal, onMount, onCleanup } from 'solid-js';
import { Portal } from 'solid-js/web';
import type { Disk } from '@/types/api';
import { formatBytes, formatPercent } from '@/utils/format';

interface StackedDiskBarProps {
  /** Array of disk objects - if empty/undefined, falls back to aggregate */
  disks?: Disk[];
  /** Aggregate disk data (fallback when disks array unavailable) */
  aggregateDisk?: Disk;
}

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

  // Determine if we have multiple disks or should use aggregate
  const hasMultipleDisks = createMemo(() => {
    const disks = props.disks;
    return disks && disks.length > 1;
  });

  const hasSingleDisk = createMemo(() => {
    const disks = props.disks;
    return disks && disks.length === 1;
  });

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

  // For single disk or aggregate, get the percentage
  const singleDiskPercent = createMemo(() => {
    if (hasSingleDisk() && props.disks?.[0]) {
      const disk = props.disks[0];
      if (!disk.total || disk.total <= 0) return 0;
      return (disk.used / disk.total) * 100;
    }
    if (props.aggregateDisk) {
      if (!props.aggregateDisk.total || props.aggregateDisk.total <= 0) return 0;
      return (props.aggregateDisk.used / props.aggregateDisk.total) * 100;
    }
    return 0;
  });

  // Calculate segment widths for stacked bar (each disk's used space as % of total capacity)
  const segments = createMemo(() => {
    if (!hasMultipleDisks() || !props.disks) return [];

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

  // Generate tooltip content
  const tooltipContent = createMemo(() => {
    if (hasDisks() && props.disks) {
      return props.disks.map((disk, idx) => {
        const percent = disk.total > 0 ? (disk.used / disk.total) * 100 : 0;
        const label = disk.mountpoint || disk.device || `Disk ${idx + 1}`;
        return {
          label,
          used: formatBytes(disk.used, 0),
          total: formatBytes(disk.total, 0),
          percent: formatPercent(percent),
          color: percent >= 90 ? getUsageColor(90) :
            percent >= 80 ? getUsageColor(80) :
              SEGMENT_COLORS[idx % SEGMENT_COLORS.length],
        };
      });
    }
    // Fallback for aggregate disk
    if (props.aggregateDisk && props.aggregateDisk.total > 0) {
      const percent = (props.aggregateDisk.used / props.aggregateDisk.total) * 100;
      return [{
        label: 'Total',
        used: formatBytes(props.aggregateDisk.used, 0),
        total: formatBytes(props.aggregateDisk.total, 0),
        percent: formatPercent(percent),
        color: getUsageColor(percent),
      }];
    }
    return [];
  });

  // Label for the bar
  const displayLabel = createMemo(() => {
    if (hasMultipleDisks()) {
      return formatPercent(overallPercent());
    }
    return formatPercent(singleDiskPercent());
  });

  // Sublabel showing used/total
  const displaySublabel = createMemo(() => {
    return `${formatBytes(totalUsed(), 0)}/${formatBytes(totalCapacity(), 0)}`;
  });

  // Check if sublabel fits
  const showSublabel = createMemo(() => {
    const fullText = `${displayLabel()} (${displaySublabel()})`;
    return containerWidth() >= estimateTextWidth(fullText);
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
    <div ref={containerRef} class="metric-text w-full h-4 flex items-center justify-center">
      <div
        class="relative w-full h-full overflow-hidden bg-gray-200 dark:bg-gray-600 rounded"
        onMouseEnter={handleMouseEnter}
        onMouseLeave={handleMouseLeave}
      >
        {/* Stacked segments for multiple disks */}
        <Show when={hasMultipleDisks()}>
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

        {/* Single bar for single disk or aggregate */}
        <Show when={!hasMultipleDisks()}>
          <div
            class="absolute top-0 left-0 h-full"
            style={{
              width: `${Math.min(singleDiskPercent(), 100)}%`,
              'background-color': getUsageColor(singleDiskPercent()),
            }}
          />
        </Show>

        {/* Label overlay */}
        <span class="absolute inset-0 flex items-center justify-center text-[10px] font-semibold text-gray-700 dark:text-gray-100 leading-none">
          <span class="flex items-center gap-1 whitespace-nowrap px-0.5">
            <span>{displayLabel()}</span>
            <Show when={showSublabel()}>
              <span class="metric-sublabel font-normal text-gray-500 dark:text-gray-300">
                ({displaySublabel()})
              </span>
            </Show>
            <Show when={hasMultipleDisks()}>
              <span class="text-[8px] font-normal text-gray-500 dark:text-gray-400">
                [{props.disks?.length}]
              </span>
            </Show>
          </span>
        </span>
      </div>

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
                  <div class="flex justify-between gap-3 py-0.5" classList={{ 'border-t border-gray-700/50': idx() > 0 }}>
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
                )}
              </For>
            </div>
          </div>
        </Portal>
      </Show>
    </div>
  );
}
