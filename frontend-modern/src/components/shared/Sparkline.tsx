/**
 * Sparkline Component
 *
 * Lightweight canvas-based sparkline chart for displaying metric trends.
 * Optimized for rendering many sparklines simultaneously in tables.
 */

import { onCleanup, createEffect, createSignal, Component, Show } from 'solid-js';
import { Portal } from 'solid-js/web';
import type { MetricSnapshot } from '@/stores/metricsHistory';
import { scheduleSparkline } from '@/utils/canvasRenderQueue';

interface SparklineProps {
  data: MetricSnapshot[];
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
  let canvasRef: HTMLCanvasElement | undefined;
  let unregister: (() => void) | null = null;

  // Hover state for tooltip
  const [hoveredPoint, setHoveredPoint] = createSignal<{
    value: number;
    timestamp: number;
    x: number;
    y: number;
  } | null>(null);

  // If width is 0, use container width; otherwise use provided width
  const width = () => {
    if (props.width === 0 && canvasRef?.parentElement) {
      return canvasRef.parentElement.clientWidth;
    }
    return props.width || 120;
  };
  const height = () => props.height || 24;

  // Default thresholds based on metric type
  const getDefaultThresholds = () => {
    switch (props.metric) {
      case 'cpu':
        return { warning: 80, critical: 90 };
      case 'memory':
        return { warning: 75, critical: 85 };
      case 'disk':
        return { warning: 80, critical: 90 };
      default:
        return { warning: 75, critical: 90 };
    }
  };

  const thresholds = () => props.thresholds || getDefaultThresholds();

  // Get color based on latest value and thresholds
  const getColor = (value: number): string => {
    if (props.color) return props.color;

    const t = thresholds();
    if (value >= t.critical) return '#ef4444'; // red-500
    if (value >= t.warning) return '#eab308';  // yellow-500
    return '#22c55e'; // green-500
  };

  // Get color with opacity matching progress bars (60% for consistency)
  const getColorWithOpacity = (value: number): string => {
    const baseColor = getColor(value);
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

    const data = props.data;
    const w = width();
    const h = height();
    const metric = props.metric;

    // Set canvas size (accounting for device pixel ratio for sharp rendering)
    const dpr = window.devicePixelRatio || 1;
    canvas.width = w * dpr;
    canvas.height = h * dpr;
    // Only set explicit width style if a fixed width was provided
    // Otherwise let CSS handle the width (w-full class)
    if (props.width !== 0) {
      canvas.style.width = `${w}px`;
    }
    canvas.style.height = `${h}px`;
    ctx.scale(dpr, dpr);

    // Clear canvas
    ctx.clearRect(0, 0, w, h);

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

    // Extract values for the selected metric
    const values = data.map(d => d[metric]);

    // Get latest value for color
    const latestValue = values[values.length - 1] || 0;
    const color = getColor(latestValue);
    const colorWithOpacity = getColorWithOpacity(latestValue);

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

    // Draw threshold reference lines
    const t = thresholds();
    ctx.strokeStyle = '#94a3b8'; // gray-400
    ctx.lineWidth = 0.5;
    ctx.globalAlpha = 0.3;
    ctx.setLineDash([2, 2]);

    // Warning threshold line
    const warningY = h - ((t.warning - minValue) / (maxValue - minValue)) * h;
    ctx.beginPath();
    ctx.moveTo(0, warningY);
    ctx.lineTo(w, warningY);
    ctx.stroke();

    // Critical threshold line
    const criticalY = h - ((t.critical - minValue) / (maxValue - minValue)) * h;
    ctx.beginPath();
    ctx.moveTo(0, criticalY);
    ctx.lineTo(w, criticalY);
    ctx.stroke();

    ctx.setLineDash([]);
    ctx.globalAlpha = 1;

    // Draw gradient fill
    const gradient = ctx.createLinearGradient(0, 0, 0, h);
    gradient.addColorStop(0, `${color}40`); // 25% opacity at top
    gradient.addColorStop(1, `${color}10`); // 6% opacity at bottom

    ctx.fillStyle = gradient;
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

    // Draw current value dot with opacity
    if (points.length > 0) {
      const lastPoint = points[points.length - 1];
      ctx.fillStyle = colorWithOpacity;
      ctx.beginPath();
      ctx.arc(lastPoint.x, lastPoint.y, 2, 0, Math.PI * 2);
      ctx.fill();
    }
  };

  // Redraw when data or dimensions change
  createEffect(() => {
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
    if (!canvasRef || !props.data || props.data.length === 0) return;

    const rect = canvasRef.getBoundingClientRect();
    const mouseX = e.clientX - rect.left;
    const w = width();

    const data = props.data;
    const values = data.map(d => d[props.metric]);
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

  // Format timestamp for tooltip
  const formatTime = (timestamp: number) => {
    const date = new Date(timestamp);
    return date.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' });
  };

  return (
    <>
      <div class="relative block w-full" style={{ height: `${height()}px` }}>
        <canvas
          ref={canvasRef}
          class="block cursor-crosshair w-full"
          style={{
            height: `${height()}px`,
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
