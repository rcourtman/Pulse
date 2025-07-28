import { createSignal } from 'solid-js';

interface ChartDimensions {
  [key: string]: number;
}

// Global store for chart dimensions
const [dimensions, setDimensions] = createSignal<ChartDimensions>({});

// Cache to persist dimensions between re-renders
const dimensionCache: ChartDimensions = {};

// Single ResizeObserver instance shared by all charts
let resizeObserver: ResizeObserver | null = null;
const observedElements = new Map<Element, string>();

// Debounce timer
let resizeTimeout: number | undefined;

// Initialize the shared ResizeObserver
function initResizeObserver() {
  if (!resizeObserver && typeof window !== 'undefined') {
    resizeObserver = new ResizeObserver((entries) => {
      // Clear existing timeout
      if (resizeTimeout) {
        clearTimeout(resizeTimeout);
      }

      // Debounce resize events by 50ms
      resizeTimeout = window.setTimeout(() => {
        requestAnimationFrame(() => {
          const updates: ChartDimensions = {};
          
          for (const entry of entries) {
            const chartId = observedElements.get(entry.target);
            if (chartId) {
              const width = Math.floor(entry.contentRect.width);
              // Only update if width actually changed
              if (dimensions()[chartId] !== width) {
                updates[chartId] = width;
              }
            }
          }

          // Batch update all dimensions at once
          if (Object.keys(updates).length > 0) {
            // Update cache
            Object.assign(dimensionCache, updates);
            setDimensions(prev => ({ ...prev, ...updates }));
          }
        });
      }, 50);
    });
  }
}

// Register a chart element for observation
export function observeChart(element: HTMLElement, chartId: string) {
  initResizeObserver();
  
  if (resizeObserver && element) {
    // Store the association
    observedElements.set(element, chartId);
    resizeObserver.observe(element);
    
    // Get initial dimension - use cached value if available to prevent animation
    const cachedWidth = dimensionCache[chartId];
    const width = cachedWidth || Math.floor(element.offsetWidth);
    dimensionCache[chartId] = width;
    setDimensions(prev => ({ ...prev, [chartId]: width }));
    
    // Return cleanup function
    return () => {
      if (resizeObserver) {
        resizeObserver.unobserve(element);
        observedElements.delete(element);
        
        // Don't clean up dimension entry - keep it cached
        // This prevents re-animation on re-render
        // setDimensions(prev => {
        //   const next = { ...prev };
        //   delete next[chartId];
        //   return next;
        // });
      }
    };
  }
  
  return () => {};
}

// Get dimension for a specific chart
export function getChartDimension(chartId: string): number {
  return dimensions()[chartId] || 0;
}

// Subscribe to dimension changes for a specific chart
export function subscribeToChartDimension(chartId: string) {
  return () => dimensions()[chartId];
}

// Clean up on app unmount
if (typeof window !== 'undefined') {
  window.addEventListener('unload', () => {
    if (resizeObserver) {
      resizeObserver.disconnect();
      resizeObserver = null;
    }
    if (resizeTimeout) {
      clearTimeout(resizeTimeout);
    }
  });
}