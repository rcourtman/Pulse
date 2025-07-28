import { createSignal } from 'solid-js';
import { createStore } from 'solid-js/store';
import { logger } from '@/utils/logger';

// Chart configuration based on main branch
export const CHART_CONFIG = {
  // Different sizes for different use cases
  sparkline: { width: 66, height: 16, padding: 1 }, // For I/O metrics
  mini: { width: 118, height: 20, padding: 2 },     // For usage metrics
  storage: { width: 200, height: 14, padding: 1 },  // For storage tab - default width, will be overridden dynamically
  strokeWidth: 1.5,
  
  // Get optimal render points based on screen resolution
  getRenderPoints: () => {
    const screenWidth = window.screen.width;
    const pixelRatio = window.devicePixelRatio || 1;
    const effectiveWidth = screenWidth * pixelRatio;
    
    if (effectiveWidth >= 3840) return 80;      // 4K
    else if (effectiveWidth >= 2560) return 60; // 2K
    else if (effectiveWidth >= 1920) return 40; // 1080p
    else if (effectiveWidth >= 1366) return 30; // 720p
    else return 25;                              // Small screens
  },
  
  // Smart color coding based on data values
  getSmartColor: (values: number[], metric: string): string => {
    if (!values || values.length === 0) {
      const isDarkMode = document.documentElement.classList.contains('dark');
      return isDarkMode ? '#6b7280' : '#d1d5db'; // gray-500/gray-300
    }
    
    const currentValue = values[values.length - 1];
    const maxValue = Math.max(...values);
    
    if (metric === 'cpu' || metric === 'memory' || metric === 'disk') {
      // Percentage-based metrics
      if (metric === 'cpu') {
        if (currentValue >= 90 || maxValue >= 95) return '#ef4444'; // red-500
        if (currentValue >= 80 || maxValue >= 85) return '#f59e0b'; // amber-500
      } else if (metric === 'memory') {
        if (currentValue >= 85 || maxValue >= 90) return '#ef4444'; // red-500
        if (currentValue >= 75 || maxValue >= 80) return '#f59e0b'; // amber-500
      } else if (metric === 'disk') {
        if (currentValue >= 90 || maxValue >= 95) return '#ef4444'; // red-500
        if (currentValue >= 80 || maxValue >= 85) return '#f59e0b'; // amber-500
      }
      
      const isDarkMode = document.documentElement.classList.contains('dark');
      return isDarkMode ? '#6b7280' : '#d1d5db'; // gray: normal operation
    } else if (metric === 'diskread' || metric === 'diskwrite' || metric === 'netin' || metric === 'netout') {
      // I/O metrics - use absolute thresholds
      // const maxMBps = maxValue / (1024 * 1024); // unused
      const avgValue = values.reduce((sum, v) => sum + v, 0) / values.length;
      const avgMBps = avgValue / (1024 * 1024);
      
      if (avgMBps > 50) return '#ef4444';  // red: >50 MB/s
      if (avgMBps > 10) return '#f59e0b';  // amber: >10 MB/s
      if (avgMBps > 1) return '#10b981';   // green: >1 MB/s
      
      const isDarkMode = document.documentElement.classList.contains('dark');
      return isDarkMode ? '#6b7280' : '#d1d5db'; // gray: minimal activity
    } else {
      // Unknown metric - use default color
      const isDarkMode = document.documentElement.classList.contains('dark');
      return isDarkMode ? '#6b7280' : '#d1d5db';
    }
  },
  
  // Default colors for metrics
  colors: {
    cpu: '#ef4444',      // red-500
    memory: '#3b82f6',   // blue-500
    disk: '#8b5cf6',     // violet-500
    diskread: '#3b82f6', // blue-500
    diskwrite: '#f97316', // orange-500
    netin: '#10b981',    // emerald-500
    netout: '#f59e0b'    // amber-500
  }
};

// Chart data point interface
export interface ChartDataPoint {
  timestamp: number;
  value: number;
}

// Chart data for a single metric
export interface MetricChartData {
  cpu?: ChartDataPoint[];
  memory?: ChartDataPoint[];
  disk?: ChartDataPoint[];
  diskRead?: ChartDataPoint[];
  diskWrite?: ChartDataPoint[];
  networkIn?: ChartDataPoint[];
  networkOut?: ChartDataPoint[];
}

// Store for chart data
interface ChartStore {
  guestData: Record<string, MetricChartData>;
  nodeData: Record<string, MetricChartData>;
  storageData: Record<string, MetricChartData>;
  lastFetch: number;
  timeRange: string;
  stats: {
    oldestDataTimestamp?: number;
  };
}

const [chartStore, setChartStore] = createStore<ChartStore>({
  guestData: {},
  nodeData: {},
  storageData: {},
  lastFetch: 0,
  timeRange: '60', // Default 1 hour
  stats: {}
});

// Signal for current render points
const [renderPoints, setRenderPoints] = createSignal(CHART_CONFIG.getRenderPoints());

// Update render points on window resize
if (typeof window !== 'undefined') {
  window.addEventListener('resize', () => {
    const newPoints = CHART_CONFIG.getRenderPoints();
    if (newPoints !== renderPoints()) {
      setRenderPoints(newPoints);
      // Clear processed data cache when resolution changes
      processedDataCache.clear();
    }
  });
}

// Cache for processed chart data
const processedDataCache = new Map<string, {
  data: ChartDataPoint[];
  timestamp: number;
  hash: string;
}>();

// Generate quick hash of data for cache validation
function generateDataHash(data: ChartDataPoint[]): string {
  if (!data || data.length === 0) return '0';
  const first = data[0];
  const last = data[data.length - 1];
  // const middle = data[Math.floor(data.length / 2)]; // unused
  return `${data.length}-${first?.timestamp || 0}-${last?.timestamp || 0}-${first?.value?.toFixed(2) || 0}-${last?.value?.toFixed(2) || 0}`;
}

// Calculate importance scores for adaptive sampling
function calculateImportanceScores(data: ChartDataPoint[]): number[] {
  const scores = new Array(data.length);
  const values = data.map(d => d.value);
  
  for (let i = 0; i < data.length; i++) {
    let score = 0;
    
    // Rate of change
    if (i > 0 && i < data.length - 1) {
      const change = Math.abs(values[i + 1] - values[i - 1]);
      score += change;
    }
    
    // Peaks and valleys
    if (i > 0 && i < data.length - 1) {
      const isPeak = values[i] > values[i - 1] && values[i] > values[i + 1];
      const isValley = values[i] < values[i - 1] && values[i] < values[i + 1];
      if (isPeak || isValley) {
        score += Math.abs(values[i] - values[i - 1]) + Math.abs(values[i] - values[i + 1]);
      }
    }
    
    // Edge points get bonus
    if (i === 0 || i === data.length - 1) {
      score += 1000; // Ensure edges are always included
    }
    
    scores[i] = score;
  }
  
  return scores;
}

// Adaptive sampling algorithm
export function adaptiveSample(data: ChartDataPoint[], targetPoints: number): ChartDataPoint[] {
  if (data.length <= targetPoints) return data;
  
  const maxPoints = Math.max(2, targetPoints);
  
  // Calculate importance scores
  const importance = calculateImportanceScores(data);
  
  // Always include first and last points
  const selectedIndices = new Set([0, data.length - 1]);
  const remainingPoints = maxPoints - 2;
  
  if (remainingPoints <= 0) {
    return [data[0], data[data.length - 1]];
  }
  
  // Create candidates array
  const candidates = [];
  for (let i = 1; i < data.length - 1; i++) {
    candidates.push({ index: i, importance: importance[i] });
  }
  
  // Sort by importance
  candidates.sort((a, b) => b.importance - a.importance);
  
  // Add most important points up to limit
  for (let i = 0; i < Math.min(remainingPoints, candidates.length); i++) {
    selectedIndices.add(candidates[i].index);
  }
  
  // Convert to sorted array and extract data points
  const sortedIndices = Array.from(selectedIndices).sort((a, b) => a - b);
  return sortedIndices.map(i => data[i]);
}

// Process chart data with caching and adaptive sampling
export function processChartData(
  serverData: ChartDataPoint[],
  chartType: 'mini' | 'sparkline' = 'mini',
  guestId: string,
  metric: string
): ChartDataPoint[] {
  if (!serverData || serverData.length === 0) {
    return [];
  }
  
  // Generate cache key
  const cacheKey = `${guestId}-${metric}-${chartType}`;
  const dataHash = generateDataHash(serverData);
  
  // Check cache
  const cached = processedDataCache.get(cacheKey);
  if (cached && cached.hash === dataHash && (Date.now() - cached.timestamp < 30000)) {
    return cached.data;
  }
  
  let targetPoints = renderPoints();
  
  // Sparklines need fewer points
  if (chartType === 'sparkline') {
    targetPoints = Math.round(targetPoints * 0.6);
  }
  
  let processedData: ChartDataPoint[];
  if (serverData.length <= targetPoints) {
    processedData = serverData;
  } else {
    processedData = adaptiveSample(serverData, targetPoints);
  }
  
  // Cache the result
  processedDataCache.set(cacheKey, {
    data: processedData,
    timestamp: Date.now(),
    hash: dataHash
  });
  
  // Clean up old cache entries
  if (processedDataCache.size > 200) {
    const now = Date.now();
    const maxAge = 60000; // 1 minute
    
    for (const [key, value] of processedDataCache.entries()) {
      if (now - value.timestamp > maxAge) {
        processedDataCache.delete(key);
      }
    }
  }
  
  return processedData;
}

// Fetch chart data from API
export async function fetchChartData(timeRange: string) {
  try {
    const response = await fetch(`/api/charts?range=${timeRange}`);
    if (!response.ok) {
      throw new Error(`HTTP ${response.status}`);
    }
    
    const data = await response.json();
    
    // Calculate time offset between server and browser
    const serverTime = data.timestamp || Date.now();
    const browserTime = Date.now();
    const timeOffset = browserTime - serverTime;
    
    // Adjust all timestamps to compensate for time offset
    if (data.data) {
      for (const guestId in data.data) {
        for (const metric in data.data[guestId]) {
          if (Array.isArray(data.data[guestId][metric])) {
            data.data[guestId][metric] = data.data[guestId][metric].map((point: any) => ({
              ...point,
              timestamp: point.timestamp + timeOffset
            }));
          }
        }
      }
    }
    
    // Adjust node data timestamps
    if (data.nodeData) {
      for (const nodeId in data.nodeData) {
        for (const metric in data.nodeData[nodeId]) {
          if (Array.isArray(data.nodeData[nodeId][metric])) {
            data.nodeData[nodeId][metric] = data.nodeData[nodeId][metric].map((point: any) => ({
              ...point,
              timestamp: point.timestamp + timeOffset
            }));
          }
        }
      }
    }
    
    // Adjust storage data timestamps
    if (data.storageData) {
      for (const storageId in data.storageData) {
        for (const metric in data.storageData[storageId]) {
          if (Array.isArray(data.storageData[storageId][metric])) {
            data.storageData[storageId][metric] = data.storageData[storageId][metric].map((point: any) => ({
              ...point,
              timestamp: point.timestamp + timeOffset
            }));
          }
        }
      }
    }
    
    // Update store
    setChartStore({
      guestData: data.data || {},
      nodeData: data.nodeData || {},
      storageData: data.storageData || {},
      lastFetch: Date.now(),
      timeRange,
      stats: {
        oldestDataTimestamp: data.stats?.oldestDataTimestamp
      }
    });
    
    // Don't clear the entire cache - let individual entries expire naturally
    // This prevents all charts from recalculating at once
    // processedDataCache.clear();
    
    return { guestData: data.data, nodeData: data.nodeData, storageData: data.storageData };
  } catch (error) {
    logger.error('Failed to fetch chart data', error);
    return null;
  }
}

// Empty array constant to avoid recreating arrays
const EMPTY_ARRAY: ChartDataPoint[] = [];

// Get chart data for a specific guest and metric
export function getGuestChartData(guestId: string, metric: string): ChartDataPoint[] {
  const guestData = chartStore.guestData[guestId];
  if (!guestData) return EMPTY_ARRAY;
  
  const metricData = guestData[metric as keyof MetricChartData];
  return metricData || EMPTY_ARRAY;
}

// Get chart data for a specific node and metric
export function getNodeChartData(nodeId: string, metric: string): ChartDataPoint[] {
  const nodeData = chartStore.nodeData[nodeId];
  if (!nodeData) return EMPTY_ARRAY;
  
  const metricData = nodeData[metric as keyof MetricChartData];
  return metricData || EMPTY_ARRAY;
}

// Get chart data for a specific storage and metric
export function getStorageChartData(storageId: string, metric: string): ChartDataPoint[] {
  const storageData = chartStore.storageData[storageId];
  if (!storageData) return EMPTY_ARRAY;
  
  const metricData = storageData[metric as keyof MetricChartData];
  return metricData || EMPTY_ARRAY;
}

// Check if we should fetch new data
export function shouldFetchChartData(): boolean {
  const CHART_FETCH_INTERVAL = 5000; // 5 seconds
  return !chartStore.lastFetch || (Date.now() - chartStore.lastFetch) > CHART_FETCH_INTERVAL;
}

// Format time ago for tooltips
export function formatTimeAgo(timestamp: number): string {
  const now = Date.now();
  const diffMs = now - timestamp;
  
  if (diffMs < 0) return '0s ago';
  
  const diffMinutes = Math.floor(diffMs / 60000);
  const diffSeconds = Math.floor((diffMs % 60000) / 1000);
  
  if (diffMinutes >= 60) {
    const hours = Math.floor(diffMinutes / 60);
    const minutes = diffMinutes % 60;
    
    if (hours >= 24) {
      const days = Math.floor(hours / 24);
      const remainingHours = hours % 24;
      
      if (remainingHours > 0) {
        return `${days}d ${remainingHours}h ${minutes}m ago`;
      } else if (minutes > 0) {
        return `${days}d ${minutes}m ago`;
      } else {
        return `${days}d ago`;
      }
    }
    
    return `${hours}h ${minutes}m ago`;
  } else if (diffMinutes > 0) {
    return `${diffMinutes}m ${diffSeconds}s ago`;
  } else {
    return `${diffSeconds}s ago`;
  }
}

// Format value for tooltip display
export function formatChartValue(value: number, metric: string): string {
  if (metric === 'cpu' || metric === 'memory' || metric === 'disk') {
    return Math.round(value) + '%';
  } else if (metric === 'diskread' || metric === 'diskwrite' || metric === 'netin' || metric === 'netout') {
    // Format as speed
    const mbps = value / (1024 * 1024);
    if (mbps >= 100) return `${Math.round(mbps)} MB/s`;
    if (mbps >= 10) return `${Math.round(mbps * 10) / 10} MB/s`;
    if (mbps >= 1) return `${Math.round(mbps * 100) / 100} MB/s`;
    
    const kbps = value / 1024;
    if (kbps >= 1) return `${Math.round(kbps)} KB/s`;
    
    return `${Math.round(value)} B/s`;
  } else {
    // Default formatting
    return String(Math.round(value));
  }
}

// Export store and utilities
export { chartStore, renderPoints };