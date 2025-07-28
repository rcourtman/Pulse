import { Component, createMemo, createSignal, onCleanup } from 'solid-js';
import { CHART_CONFIG, processChartData, formatTimeAgo, formatChartValue } from '@/stores/charts';
import type { ChartDataPoint } from '@/stores/charts';
import { showTooltip, hideTooltip } from '@/components/shared/Tooltip';

interface SparklineProps {
  data: number[] | ChartDataPoint[];
  width?: number;
  height?: number;
  color?: string;
  strokeWidth?: number;
  filled?: boolean;
  metric?: string;
  guestId?: string;
  chartType?: 'mini' | 'sparkline' | 'storage';
  showTooltip?: boolean;
  responsive?: boolean;
  forceGray?: boolean;
}

const Sparkline: Component<SparklineProps> = (props) => {
  const chartType = () => props.chartType || 'mini';
  const config = () => CHART_CONFIG[chartType()];
  const width = () => props.width || config().width;
  const height = () => props.height || config().height;
  const strokeWidth = () => props.strokeWidth || CHART_CONFIG.strokeWidth;
  const metric = () => props.metric || 'generic';
  
  let svgRef: SVGSVGElement | undefined;
  let tooltipTimeout: number | undefined;
  const [isHovering, setIsHovering] = createSignal(false);
  const [hoverPoint, setHoverPoint] = createSignal<{x: number, y: number, value: number, timestamp?: number} | null>(null);
  const [, setTooltipContent] = createSignal<string>('');
  const [, setTooltipPosition] = createSignal<{x: number, y: number}>({x: 0, y: 0});
  
  // Convert data to ChartDataPoint format if needed
  const chartData = createMemo(() => {
    const rawData = props.data || [];
    
    // If already ChartDataPoint format, use as-is
    if (rawData.length > 0 && typeof rawData[0] === 'object' && 'timestamp' in rawData[0]) {
      return rawData as ChartDataPoint[];
    }
    
    // Convert number array to ChartDataPoint array
    return (rawData as number[]).map((value, index) => ({
      timestamp: Date.now() - (rawData.length - index - 1) * 5000, // Assume 5s intervals
      value
    }));
  });
  
  // Process data with adaptive sampling if needed
  const processedData = createMemo(() => {
    const data = chartData();
    // Skip processing if we don't have much data
    if (data.length <= 10) return data;
    
    if (!props.guestId || !props.metric) return data;
    
    // For storage charts, just return the data as-is
    const type = chartType();
    if (type === 'storage') return data;
    
    return processChartData(data, type as 'mini' | 'sparkline', props.guestId, props.metric);
  });
  
  // Get smart color based on data
  const smartColor = createMemo(() => {
    if (props.color) return props.color;
    
    // Force gray for dashboard charts
    if (props.forceGray) {
      const isDarkMode = document.documentElement.classList.contains('dark');
      return isDarkMode ? '#6b7280' : '#d1d5db'; // gray-500/gray-300
    }
    
    const data = processedData();
    const values = data.map(d => d.value);
    return CHART_CONFIG.getSmartColor(values, metric());
  });
  
  // Generate SVG path from data points
  const pathData = createMemo(() => {
    const data = processedData();
    
    if (data.length < 2) return { line: '', area: '', points: [], minValue: 0, maxValue: 0 };
    
    const w = width();
    const h = height();
    const padding = config().padding;
    
    // Extract values for scaling
    const values = data.map(d => d.value);
    const minValue = Math.min(...values);
    const maxValue = Math.max(...values);
    // const valueRange = maxValue - minValue; // unused
    
    // Smart scaling: include 0 for percentage metrics if low values
    let scalingMin = minValue;
    let scalingMax = maxValue;
    
    const isPercentageMetric = metric() === 'cpu' || metric() === 'memory' || metric() === 'disk';
    if (isPercentageMetric && minValue < 20) {
      scalingMin = 0;
    } else if (!isPercentageMetric && minValue < maxValue * 0.01) {
      scalingMin = 0;
    }
    
    const scalingRange = scalingMax - scalingMin || 1;
    
    // Calculate dimensions
    const chartAreaWidth = w - 2 * padding;
    const chartAreaHeight = h - 2 * padding;
    const yScale = chartAreaHeight / scalingRange;
    const xStep = chartAreaWidth / Math.max(1, data.length - 1);
    
    // Generate points with coordinates
    const points = data.map((point, i) => {
      const x = padding + i * xStep;
      const y = h - padding - (scalingRange > 0 ? (point.value - scalingMin) * yScale : chartAreaHeight / 2);
      return { x, y, value: point.value, timestamp: point.timestamp };
    });
    
    // Build line path - simplified without toFixed for performance
    const lineData = points.map((point, index) => 
      `${index === 0 ? 'M' : 'L'} ${point.x} ${point.y}`
    ).join(' ');
    
    const baseY = h - padding;
    const areaData = props.filled ? 
      `M ${points[0].x} ${baseY} ` + 
      points.map(p => `L ${p.x} ${p.y}`).join(' ') + 
      ` L ${points[points.length - 1].x} ${baseY} Z` 
      : '';
    
    return { line: lineData, area: areaData, points, minValue, maxValue };
  });
  
  // Generate stable gradient ID based on guest/metric
  const gradientId = createMemo(() => {
    if (props.guestId && props.metric) {
      return `gradient-${props.guestId}-${props.metric}`.replace(/[^a-zA-Z0-9-]/g, '-');
    }
    return `gradient-${Math.random().toString(36).substr(2, 9)}`;
  });
  
  // Handle mouse events
  const handleMouseMove = (e: MouseEvent) => {
    if (!svgRef || !props.showTooltip) return;
    
    const rect = svgRef.getBoundingClientRect();
    const x = (e.clientX - rect.left) * (width() / rect.width);
    
    const data = pathData();
    if (!data.points || data.points.length === 0) return;
    
    // Find closest point
    const chartAreaWidth = width() - 2 * config().padding;
    const relativeX = Math.max(0, Math.min(chartAreaWidth, x - config().padding));
    
    let closestIndex = 0;
    let closestDistance = Infinity;
    
    data.points.forEach((point, i) => {
      const distance = Math.abs(point.x - config().padding - relativeX);
      if (distance < closestDistance) {
        closestDistance = distance;
        closestIndex = i;
      }
    });
    
    const point = data.points[closestIndex];
    if (point) {
      setHoverPoint(point);
      
      // Update tooltip content
      const value = formatChartValue(point.value, metric());
      const timeAgo = point.timestamp ? formatTimeAgo(point.timestamp) : '';
      let content = `${value}`;
      if (timeAgo) content += `<br><small>${timeAgo}</small>`;
      
      // Add range info
      if (data.minValue !== data.maxValue) {
        const minFormatted = formatChartValue(data.minValue, metric());
        const maxFormatted = formatChartValue(data.maxValue, metric());
        content += `<br><small>Range: ${minFormatted} - ${maxFormatted}</small>`;
      }
      
      setTooltipContent(content);
      setTooltipPosition({ x: e.clientX, y: e.clientY });
      
      // Show tooltip using global system
      showTooltip(content, e.clientX, e.clientY);
    }
  };
  
  const handleMouseEnter = () => {
    setIsHovering(true);
  };
  
  const handleMouseLeave = () => {
    setIsHovering(false);
    setHoverPoint(null);
    if (tooltipTimeout) {
      clearTimeout(tooltipTimeout);
    }
    
    // Hide tooltip using global system
    hideTooltip();
  };
  
  // Cleanup on unmount
  onCleanup(() => {
    if (tooltipTimeout) {
      clearTimeout(tooltipTimeout);
    }
  });
  
  return (
    <svg
      ref={svgRef}
      width={width()}
      height={height()}
      class="sparkline"
      style={{ 
        display: 'block', 
        cursor: props.showTooltip ? 'crosshair' : 'default'
      }}
      onMouseMove={handleMouseMove}
      onMouseEnter={handleMouseEnter}
      onMouseLeave={handleMouseLeave}
    >
      {props.filled && (
        <defs>
          <linearGradient id={gradientId()} x1="0%" y1="0%" x2="0%" y2="100%">
            <stop offset="0%" style={`stop-color:${smartColor()};stop-opacity:0.3`} />
            <stop offset="100%" style={`stop-color:${smartColor()};stop-opacity:0.1`} />
          </linearGradient>
        </defs>
      )}
      
      {/* Area fill (if enabled) */}
      {props.filled && pathData().area && (
        <path
          d={pathData().area}
          fill={`url(#${gradientId()})`}
        />
      )}
      
      {/* Main line */}
      <path
        d={pathData().line}
        fill="none"
        stroke={isHovering() ? (document.documentElement.classList.contains('dark') ? '#ffffff' : '#000000') : smartColor()}
        stroke-width={strokeWidth()}
        stroke-linecap="round"
        stroke-linejoin="round"
        vector-effect="non-scaling-stroke"
      />
      
      {/* Hover indicator */}
      {isHovering() && hoverPoint() && (
        <g class="hover-indicator-group">
          <circle
            cx={hoverPoint()!.x}
            cy={hoverPoint()!.y}
            r="2"
            fill={document.documentElement.classList.contains('dark') ? '#000000' : '#ffffff'}
            stroke={document.documentElement.classList.contains('dark') ? '#ffffff' : '#000000'}
            stroke-width="1.5"
            style={{ 'pointer-events': 'none' }}
          />
        </g>
      )}
      
      {/* Invisible overlay for better mouse detection */}
      {props.showTooltip && (
        <rect
          width={width()}
          height={height()}
          fill="transparent"
          style={{ cursor: 'crosshair' }}
        />
      )}
    </svg>
  );
};

export default Sparkline;