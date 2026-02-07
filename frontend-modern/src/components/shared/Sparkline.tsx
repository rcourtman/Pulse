/**
 * Sparkline Component
 *
 * Lightweight canvas-based sparkline chart for displaying metric trends.
 * Optimized for rendering many sparklines simultaneously in tables.
 */

import { onCleanup, createEffect, createSignal, Component, Show, createMemo, onMount } from 'solid-js';
import { Portal } from 'solid-js/web';
import type { MetricPoint } from '@/api/charts';
import { scheduleSparkline, setupCanvasDPR } from '@/utils/canvasRenderQueue';
import { downsampleLTTB, calculateOptimalPoints } from '@/utils/downsample';
import { getMetricColorHex } from '@/utils/metricThresholds';


/** Compact inline lock badge shown in sparkline rows when the selected range requires Pro. */
export const SparklineLockBadge: Component = () => (
  <span class="flex items-center gap-0.5 text-[9px] text-gray-400 dark:text-gray-500">
    <svg class="w-2.5 h-2.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
      <rect x="3" y="11" width="18" height="11" rx="2" ry="2" />
      <path d="M7 11V7a5 5 0 0 1 10 0v4" />
    </svg>
    <span>Pro</span>
  </span>
);

interface SparklineProps {
  data: MetricPoint[];
  metric: 'cpu' | 'memory' | 'disk';
  width?: number;
  height?: number;
  color?: string;
  thresholds?: {
    warning: number;  // Yellow threshold
    critical: number; // Red threshold
  };
}

export const Sparkline: Component<SparklineProps> = (props) => {
  let containerRef: HTMLDivElement | undefined;
  let canvasRef: HTMLCanvasElement | undefined;
  let unregister: (() => void) | null = null;
  const [measuredWidth, setMeasuredWidth] = createSignal(0);

  // Hover state for tooltip
  const [hoveredPoint, setHoveredPoint] = createSignal<{
    value: number;
    timestamp: number;
    x: number;
    y: number;
  } | null>(null);

  // If width is 0, use container width; otherwise use provided width
  const width = () => {
    if (props.width === 0) {
      const observed = measuredWidth();
      if (observed > 0) return observed;
      if (containerRef) return containerRef.clientWidth;
      if (canvasRef?.parentElement) return canvasRef.parentElement.clientWidth;
      return 0;
    }
    return props.width || 120;
  };
  const height = () => props.height || 16;

  onMount(() => {
    if (props.width !== 0 || !containerRef) return;

    setMeasuredWidth(containerRef.clientWidth);
    const observer = new ResizeObserver((entries) => {
      const entry = entries[0];
      if (!entry) return;
      const nextWidth = Math.max(0, Math.round(entry.contentRect.width));
      if (nextWidth !== measuredWidth()) {
        setMeasuredWidth(nextWidth);
      }
    });
    observer.observe(containerRef);

    onCleanup(() => observer.disconnect());
  });

  // Downsample data using LTTB algorithm for optimal rendering
  // This reduces ~2880 points to ~60-80 points (1 per 1.5 pixels)
  const downsampledData = createMemo(() => {
    const data = props.data;
    if (!data || data.length === 0) return data;

    const w = width();
    const optimalPoints = calculateOptimalPoints(w, 'sparkline');

    // Only downsample if we have significantly more points than needed
    if (data.length <= optimalPoints * 1.5) {
      return data;
    }

    return downsampleLTTB(data, optimalPoints);
  });

  // Get color based on latest value and thresholds
  const resolveColor = (value: number): string => {
    if (props.color) return props.color;

    // If custom thresholds were provided, use them directly
    if (props.thresholds) {
      const t = props.thresholds;
      if (value >= t.critical) return '#ef4444';
      if (value >= t.warning) return '#eab308';
      return '#22c55e';
    }

    return getMetricColorHex(value, props.metric);
  };

  // Get color with opacity matching progress bars (60% for consistency)
  const resolveColorWithOpacity = (value: number): string => {
    const baseColor = resolveColor(value);
    // Convert hex to rgba with 0.6 opacity (matching progress bar 60%)
    const r = parseInt(baseColor.slice(1, 3), 16);
    const g = parseInt(baseColor.slice(3, 5), 16);
    const b = parseInt(baseColor.slice(5, 7), 16);
    return `rgba(${r}, ${g}, ${b}, 0.85)`; // Slightly more opaque for visibility
  };

  const drawSparkline = () => {
    if (!canvasRef) return;

    const canvas = canvasRef;
    const ctx = canvas.getContext('2d');
    if (!ctx) return;

    // Use downsampled data for efficient rendering
    const data = downsampledData();
    const w = width();
    const h = height();
    if (w <= 1 || h <= 0) return;

    setupCanvasDPR(canvas, ctx, w, h);

    // Detect dark mode for reference line color
    const isDark = document.documentElement.classList.contains('dark');

    // Draw subtle reference lines at 25%, 50%, 75% to help understand scale
    // These provide visual anchors so users can distinguish 17% from 70%
    ctx.strokeStyle = isDark ? 'rgba(255, 255, 255, 0.08)' : 'rgba(0, 0, 0, 0.06)';
    ctx.lineWidth = 1;
    ctx.setLineDash([]);
    [0.25, 0.5, 0.75].forEach(pct => {
      const y = h - (pct * h);
      ctx.beginPath();
      ctx.moveTo(0, y);
      ctx.lineTo(w, y);
      ctx.stroke();
    });

    if (!data || data.length === 0) {
      // No data - show empty state
      ctx.strokeStyle = '#d1d5db'; // gray-300
      ctx.lineWidth = 1;
      ctx.setLineDash([2, 2]);
      ctx.beginPath();
      ctx.moveTo(0, h / 2);
      ctx.lineTo(w, h / 2);
      ctx.stroke();
      ctx.setLineDash([]);
      return;
    }

    // Extract values
    const values = data.map(d => d.value);

    // Get latest value for color
    const latestValue = values[values.length - 1] || 0;
    const color = resolveColor(latestValue);
    const colorWithOpacity = resolveColorWithOpacity(latestValue);

    // Find min/max for scaling
    const minValue = 0;  // Always anchor at 0
    const maxValue = Math.max(100, ...values); // Use 100 as minimum max

    // Calculate points
    const points: Array<{ x: number; y: number }> = [];
    const xStep = w / Math.max(values.length - 1, 1);

    values.forEach((value, i) => {
      const x = i * xStep;
      // Invert y because canvas coordinates are top-down
      const y = h - ((value - minValue) / (maxValue - minValue)) * h;
      points.push({ x, y });
    });

    // Draw solid fill
    ctx.fillStyle = `${color}5A`; // 35% opacity solid fill
    ctx.beginPath();
    ctx.moveTo(points[0].x, h); // Start at bottom left
    points.forEach(p => ctx.lineTo(p.x, p.y));
    ctx.lineTo(points[points.length - 1].x, h); // Close at bottom right
    ctx.closePath();
    ctx.fill();

    // Draw line with opacity matching progress bars
    ctx.strokeStyle = colorWithOpacity;
    ctx.lineWidth = 1.5;
    ctx.lineJoin = 'round';
    ctx.lineCap = 'round';
    ctx.beginPath();
    points.forEach((p, i) => {
      if (i === 0) {
        ctx.moveTo(p.x, p.y);
      } else {
        ctx.lineTo(p.x, p.y);
      }
    });
    ctx.stroke();
  };

  // Redraw when data or dimensions change
  createEffect(() => {
    // Track downsampled data for efficient re-rendering
    void downsampledData();
    void props.metric;
    void width();
    void height();
    void measuredWidth();


    // Unregister previous draw callback if it exists
    if (unregister) {
      unregister();
    }

    // Schedule draw via shared render queue
    unregister = scheduleSparkline(drawSparkline);
  });

  onCleanup(() => {
    if (unregister) {
      unregister();
    }
  });

  // Handle mouse move to find nearest point
  const handleMouseMove = (e: MouseEvent) => {
    const data = downsampledData();
    if (!canvasRef || !data || data.length === 0) return;

    const rect = canvasRef.getBoundingClientRect();
    const mouseX = e.clientX - rect.left;
    const w = width();
    if (w <= 1) return;

    const values = data.map(d => d.value);
    const xStep = w / Math.max(values.length - 1, 1);

    // Find nearest point
    let nearestIndex = Math.round(mouseX / xStep);
    nearestIndex = Math.max(0, Math.min(nearestIndex, values.length - 1));

    const value = values[nearestIndex];
    const timestamp = data[nearestIndex].timestamp;

    // Calculate absolute viewport position for portal
    setHoveredPoint({
      value,
      timestamp,
      x: rect.left + nearestIndex * xStep,  // Absolute x position
      y: rect.top - 45,  // Position above the sparkline (45px above top edge)
    });
  };


  const handleMouseLeave = () => {
    setHoveredPoint(null);
  };

  // Format timestamp for tooltip (24-hour clock)
  const formatTime = (timestamp: number) => {
    const date = new Date(timestamp);
    return date.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit', hour12: false });
  };

  return (
    <>
      <div
        ref={containerRef}
        class="relative block w-full overflow-hidden"
        style={{ height: `${height()}px`, 'max-width': '100%' }}
      >
        <canvas
          ref={canvasRef}
          class="block cursor-crosshair"
          style={{
            height: `${height()}px`,
            'max-width': '100%',
          }}
          onMouseMove={handleMouseMove}
          onMouseLeave={handleMouseLeave}
        />
      </div>
      <Portal>
        <Show when={hoveredPoint()}>
          {(point) => (
            <div
              class="fixed pointer-events-none bg-gray-900 dark:bg-gray-800 text-white text-xs rounded px-2 py-1 shadow-lg border border-gray-700"
              style={{
                left: `${point().x}px`,
                top: `${point().y}px`,
                transform: 'translateX(-50%)',
                'z-index': '9999',
              }}
            >
              <div class="font-medium">{point().value.toFixed(1)}%</div>
              <div class="text-gray-400 text-[10px]">{formatTime(point().timestamp)}</div>
            </div>
          )}
        </Show>
      </Portal>
    </>
  );
};
