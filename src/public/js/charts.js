PulseApp.charts = (() => {
    // Screen-resolution-based points for visual quality (not time coverage)
    function getOptimalRenderPoints() {
        const screenWidth = window.screen.width;
        const pixelRatio = window.devicePixelRatio || 1;
        const effectiveWidth = screenWidth * pixelRatio;
        
        // Scale points for visual quality - everyone sees same time span
        if (effectiveWidth >= 3840) {
            return 80;   // 4K: good detail for 1-hour trends
        } else if (effectiveWidth >= 2560) {
            return 60;   // 2K: good detail for 1-hour trends
        } else if (effectiveWidth >= 1920) {
            return 40;   // 1080p: adequate detail for 1-hour trends
        } else if (effectiveWidth >= 1366) {
            return 30;   // 720p: basic detail for 1-hour trends
        } else {
            return 25;   // Small screens: minimal but functional
        }
    }

    const CHART_CONFIG = {
        // Different sizes for different use cases
        sparkline: { width: 66, height: 16, padding: 1 }, // For I/O metrics
        mini: { width: 118, height: 20, padding: 2 },       // For usage metrics
        renderPoints: getOptimalRenderPoints(),
        strokeWidth: 1.5, // Slightly thicker for better visibility
        // Smart color coding based on data values
        getSmartColor: (values, metric) => {
            if (!values || values.length === 0) {
                // Theme-adaptive gray for "unimportant" state
                const isDarkMode = document.documentElement.classList.contains('dark');
                return isDarkMode ? '#6b7280' : '#d1d5db'; // dark: gray-500, light: gray-300
            }
            
            // Get current (latest) value and recent peak for color determination
            const currentValue = values[values.length - 1];
            const maxValue = Math.max(...values);
            
            if (metric === 'cpu' || metric === 'memory' || metric === 'disk') {
                // Percentage-based metrics - consider both current and recent peaks
                if (metric === 'cpu') {
                    // Show color if current is high OR there was a recent significant spike
                    if (currentValue >= 90 || maxValue >= 95) return '#ef4444';      // red: current high or recent spike
                    if (currentValue >= 80 || maxValue >= 85) return '#f59e0b';      // amber: elevated or recent activity
                    // Theme-adaptive gray for normal operation
                    const isDarkMode = document.documentElement.classList.contains('dark');
                    return isDarkMode ? '#6b7280' : '#d1d5db';     // gray: normal operation
                } else if (metric === 'memory') {
                    // Memory pressure - be more conservative due to its critical nature
                    if (currentValue >= 85 || maxValue >= 90) return '#ef4444';      // red: current high or recent spike
                    if (currentValue >= 75 || maxValue >= 80) return '#f59e0b';      // amber: elevated or recent pressure
                    // Theme-adaptive gray for healthy
                    const isDarkMode = document.documentElement.classList.contains('dark');
                    return isDarkMode ? '#6b7280' : '#d1d5db';     // gray: healthy
                } else if (metric === 'disk') {
                    // Disk can run higher before concerning, but spikes still noteworthy
                    if (currentValue >= 90 || maxValue >= 95) return '#ef4444';      // red: current full or recent spike
                    if (currentValue >= 80 || maxValue >= 85) return '#f59e0b';      // amber: getting full or recent activity
                    // Theme-adaptive gray for plenty of space
                    const isDarkMode = document.documentElement.classList.contains('dark');
                    return isDarkMode ? '#6b7280' : '#d1d5db';     // gray: plenty of space
                }
            } else {
                // I/O metrics - use absolute thresholds based on real-world values
                const maxValue = Math.max(...values);
                if (maxValue === 0) {
                    // Theme-adaptive gray for no activity
                    const isDarkMode = document.documentElement.classList.contains('dark');
                    return isDarkMode ? '#6b7280' : '#d1d5db';     // gray (no activity)
                }
                
                // Convert to MB/s for consistent thresholds (assume values are in bytes/s)
                const maxMBps = maxValue / (1024 * 1024);
                const avgValue = values.reduce((sum, v) => sum + v, 0) / values.length;
                const avgMBps = avgValue / (1024 * 1024);
                
                // Use absolute thresholds that make sense for I/O activity
                if (avgMBps > 50) return '#ef4444';            // red: >50 MB/s (high activity)
                if (avgMBps > 10) return '#f59e0b';            // amber: >10 MB/s (moderate activity)  
                if (avgMBps > 1) return '#10b981';             // green: >1 MB/s (normal activity)
                // Theme-adaptive gray for minimal activity
                const isDarkMode = document.documentElement.classList.contains('dark');
                return isDarkMode ? '#6b7280' : '#d1d5db';     // gray: <1 MB/s (minimal activity)
            }
        },
        colors: {
            cpu: '#ef4444',     // red-500
            memory: '#3b82f6',  // blue-500
            disk: '#8b5cf6',    // violet-500
            diskread: '#3b82f6',  // blue-500 (read operations - data flowing in)
            diskwrite: '#f97316', // orange-500 (write operations - data flowing out)
            netin: '#10b981',   // emerald-500 (network download - data coming in)
            netout: '#f59e0b'   // amber-500 (network upload - data going out)
        }
    };

    // Update render points if screen resolution changes (e.g., moving between monitors)
    let currentRenderPoints = CHART_CONFIG.renderPoints;
    function checkResolutionChange() {
        const newOptimalPoints = getOptimalRenderPoints();
        if (newOptimalPoints !== currentRenderPoints) {
            currentRenderPoints = newOptimalPoints;
            CHART_CONFIG.renderPoints = newOptimalPoints;
            // Clear cache and force refresh
            chartDataCache = null;
            nodeChartDataCache = null;
            chartCache.clear();
            // Trigger chart refresh if needed
            if (chartDataCache || nodeChartDataCache) {
                updateAllCharts();
            }
        }
    }

    // Listen for resolution changes (moving between monitors, etc.)
    window.addEventListener('resize', checkResolutionChange);

    let chartCache = new Map();
    let chartDataCache = null;
    let nodeChartDataCache = null;
    let lastChartFetch = 0;
    const CHART_FETCH_INTERVAL = 5000; // More responsive: every 5 seconds
    
    // Processing cache to avoid redundant downsampling
    let processedDataCache = new Map(); // Key: `${guestId}-${metric}-${chartType}`, Value: { data, timestamp, hash }
    let lastProcessedTimestamp = 0;
    
    // Performance optimization: batch updates with requestAnimationFrame
    let pendingChartUpdates = new Set();
    let updateRAF = null;
    
    // Visibility tracking for charts
    let visibleCharts = new Set();
    let visibilityObserver = null;

    function formatValue(value, metric) {
        if (metric === 'cpu' || metric === 'memory' || metric === 'disk') {
            return Math.round(value) + '%';
        } else {
            return PulseApp.utils ? PulseApp.utils.formatSpeed(value) : `${Math.round(value)} B/s`;
        }
    }

    function getTimeAgo(timestamp) {
        const now = Date.now();
        const diffMs = now - timestamp;
        
        // Handle edge cases - for future timestamps, show as 0s ago
        if (diffMs < 0) {
            return '0s ago';
        }
        
        const diffMinutes = Math.floor(diffMs / 60000);
        const diffSeconds = Math.floor((diffMs % 60000) / 1000);
        
        // Format based on duration
        if (diffMinutes >= 60) {
            // Show hours and minutes (no seconds)
            const hours = Math.floor(diffMinutes / 60);
            const minutes = diffMinutes % 60;
            return `${hours}h ${minutes}m ago`;
        } else if (diffMinutes > 0) {
            // Show minutes and seconds
            return `${diffMinutes}m ${diffSeconds}s ago`;
        } else {
            // Show only seconds
            return `${diffSeconds}s ago`;
        }
    }

    function createOrUpdateChart(containerId, data, metric, chartType = 'mini', guestId) {
        const container = document.getElementById(containerId);
        if (!container) {
            return null; // Container doesn't exist, skip silently
        }

        const config = CHART_CONFIG[chartType];

        if (!data || data.length < 2) {
            container.innerHTML = `<div class="text-[9px] text-gray-400 text-center leading-4">${metric.toUpperCase()}</div>`;
            return null;
        }

        // Use smart downsampling with caching
        const chartData = processChartData(data, chartType, guestId, metric);
        
        // Get smart color based on data values
        const values = chartData.map(d => d.value);
        const color = CHART_CONFIG.getSmartColor(values, metric);
        
        // Always check if SVG exists and create if needed
        let svg = container.querySelector('svg');
        let isNewChart = !svg;
        
        if (!svg) {
            // Create new SVG with proper sizing
            svg = document.createElementNS('http://www.w3.org/2000/svg', 'svg');
            svg.setAttribute('width', '100%');
            svg.setAttribute('height', '100%');
            svg.setAttribute('viewBox', `0 0 ${config.width} ${config.height}`);
            svg.setAttribute('preserveAspectRatio', 'none');
            svg.setAttribute('class', 'mini-chart');
            svg.style.cursor = 'crosshair';

            // Create gradient definition for fill
            const defs = document.createElementNS('http://www.w3.org/2000/svg', 'defs');
            const gradient = document.createElementNS('http://www.w3.org/2000/svg', 'linearGradient');
            const gradientId = `gradient-${containerId}`;
            gradient.setAttribute('id', gradientId);
            gradient.setAttribute('x1', '0%');
            gradient.setAttribute('y1', '0%');
            gradient.setAttribute('x2', '0%');
            gradient.setAttribute('y2', '100%');
            
            // Gradient stops (will be updated with actual color)
            const stop1 = document.createElementNS('http://www.w3.org/2000/svg', 'stop');
            stop1.setAttribute('offset', '0%');
            stop1.setAttribute('class', 'gradient-start');
            stop1.style.transition = 'stop-color 0.3s ease-out';
            
            const stop2 = document.createElementNS('http://www.w3.org/2000/svg', 'stop');
            stop2.setAttribute('offset', '100%');
            stop2.setAttribute('stop-opacity', '0.1');
            stop2.setAttribute('class', 'gradient-end');
            stop2.style.transition = 'stop-color 0.3s ease-out';
            
            gradient.appendChild(stop1);
            gradient.appendChild(stop2);
            defs.appendChild(gradient);
            svg.appendChild(defs);

            // Create chart group
            const chartGroup = document.createElementNS('http://www.w3.org/2000/svg', 'g');
            chartGroup.setAttribute('class', 'chart-group');
            
            // Create filled area first (so it's behind the line)
            const area = document.createElementNS('http://www.w3.org/2000/svg', 'path');
            area.setAttribute('fill', `url(#${gradientId})`);
            area.setAttribute('class', 'chart-area');
            // Add smooth transitions for area changes
            area.style.transition = 'd 0.3s ease-out';
            
            // Create line path
            const path = document.createElementNS('http://www.w3.org/2000/svg', 'path');
            path.setAttribute('stroke', color);
            path.setAttribute('stroke-width', CHART_CONFIG.strokeWidth);
            path.setAttribute('fill', 'none');
            path.setAttribute('vector-effect', 'non-scaling-stroke');
            path.setAttribute('stroke-linecap', 'round');
            path.setAttribute('stroke-linejoin', 'round');
            path.setAttribute('class', 'chart-line');
            // Add smooth transitions for path and color changes
            path.style.transition = 'd 0.3s ease-out, stroke 0.3s ease-out';
            
            chartGroup.appendChild(area);
            chartGroup.appendChild(path);
            svg.appendChild(chartGroup);

            // Add hover detection
            addHoverInteraction(svg, chartData, metric, config);
            
            container.innerHTML = '';
            container.appendChild(svg);
            
            // Observe for visibility tracking
            observeChartVisibility(container);
        }

        // Update the chart with new color
        updateChartPath(svg, chartData, config, metric, isNewChart, color);
        return svg;
    }

    function addHoverInteraction(svg, chartData, metric, config) {
        // Create invisible overlay for mouse detection
        const overlay = document.createElementNS('http://www.w3.org/2000/svg', 'rect');
        overlay.setAttribute('width', config.width);
        overlay.setAttribute('height', config.height);
        overlay.setAttribute('fill', 'transparent');
        overlay.setAttribute('class', 'chart-overlay');
        overlay.style.cursor = 'crosshair';

        // Shared function to show tooltip
        function showTooltipForPosition(event, clientX, clientY) {
            const currentData = overlay._chartData || chartData;
            const currentMetric = overlay._metric || metric;
            const currentConfig = overlay._config || config;
            const minValue = overlay._minValue;
            const maxValue = overlay._maxValue;
            
            if (!currentData || currentData.length === 0) return;
            
            const rect = svg.getBoundingClientRect();
            const x = (clientX - rect.left) * (currentConfig.width / rect.width);
            
            // Find closest data point based on visual position
            const chartAreaWidth = currentConfig.width - 2 * currentConfig.padding;
            const relativeX = Math.max(0, Math.min(chartAreaWidth, x - currentConfig.padding));
            
            // Since points are evenly spaced visually, we can use index-based lookup
            // but we need to account for the fact that the chart spreads points evenly
            const xScale = chartAreaWidth / Math.max(1, currentData.length - 1);
            
            // Find the closest point by comparing x positions
            let closestIndex = 0;
            let closestDistance = Infinity;
            
            for (let i = 0; i < currentData.length; i++) {
                const pointX = i * xScale;
                const distance = Math.abs(pointX - relativeX);
                if (distance < closestDistance) {
                    closestDistance = distance;
                    closestIndex = i;
                }
            }
            
            const point = currentData[closestIndex];
            if (point && typeof point.value === 'number' && point.timestamp) {
                const value = formatValue(point.value, currentMetric);
                const timeAgo = getTimeAgo(point.timestamp);
                
                // Enhanced tooltip with range information
                let tooltipContent = `${value}<br><small>${timeAgo}</small>`;
                
                if (typeof minValue === 'number' && typeof maxValue === 'number') {
                    const minFormatted = formatValue(minValue, currentMetric);
                    const maxFormatted = formatValue(maxValue, currentMetric);
                    tooltipContent += `<br><small>Range: ${minFormatted} - ${maxFormatted}</small>`;
                }
                
                // Show enhanced tooltip with proper event object
                if (PulseApp.tooltips) {
                    PulseApp.tooltips.showTooltip(event, tooltipContent);
                }
                
                // Update hover indicator position
                updateHoverIndicator(svg, closestIndex, point, currentData, currentConfig, minValue, maxValue);
            }
        }

        // Mouse events (desktop)
        overlay.addEventListener('mousemove', (event) => {
            showTooltipForPosition(event, event.clientX, event.clientY);
        });

        overlay.addEventListener('mouseenter', () => {
            // Change chart line color on hover based on theme
            const path = svg.querySelector('.chart-line');
            if (path) {
                path.setAttribute('data-original-color', path.getAttribute('stroke'));
                // Use black for light mode, white for dark mode
                const isDarkMode = document.documentElement.classList.contains('dark');
                const hoverColor = isDarkMode ? '#ffffff' : '#000000';
                path.setAttribute('stroke', hoverColor);
            }
        });

        overlay.addEventListener('mouseleave', () => {
            // Restore original color and hide tooltip
            const path = svg.querySelector('.chart-line');
            if (path) {
                const originalColor = path.getAttribute('data-original-color');
                if (originalColor) {
                    path.setAttribute('stroke', originalColor);
                }
            }
            // Remove hover indicator
            const hoverDot = svg.querySelector('.hover-indicator');
            if (hoverDot) {
                hoverDot.remove();
            }
            if (PulseApp.tooltips) {
                PulseApp.tooltips.hideTooltip();
            }
        });

        // Touch events disabled for now - they interfere with scrolling
        // Charts will still work with mouse events on devices that support them

        svg.appendChild(overlay);
        
        // Initialize with current data
        overlay._chartData = chartData.slice(); // Create a copy to avoid reference issues
        overlay._metric = metric;
        overlay._config = config;
        
        // Remove the touch indicator - charts are discoverable enough without it
    }

    function updateChartPath(svg, chartData, config, metric, isNewChart = false, color) {
        const chartGroup = svg.querySelector('.chart-group');
        const path = chartGroup?.querySelector('.chart-line');
        const area = chartGroup?.querySelector('.chart-area');
        if (!path || !chartData || chartData.length < 2) return;

        // Update colors
        if (color) {
            path.setAttribute('stroke', color);
            
            // Update gradient colors
            const gradientStart = svg.querySelector('.gradient-start');
            const gradientEnd = svg.querySelector('.gradient-end');
            if (gradientStart && gradientEnd) {
                gradientStart.setAttribute('stop-color', color);
                gradientStart.setAttribute('stop-opacity', '0.3');
                gradientEnd.setAttribute('stop-color', color);
            }
        }

        const values = chartData.map(d => d.value);
        const minValue = Math.min(...values);
        const maxValue = Math.max(...values);
        const valueRange = maxValue - minValue;
        
        // Adjust for chart area (no label space needed anymore)
        const chartAreaWidth = config.width - 2 * config.padding;
        const chartAreaHeight = config.height - 2 * config.padding;
        
        const yScale = valueRange > 0 ? chartAreaHeight / valueRange : 0;
        const xScale = chartAreaWidth / Math.max(1, chartData.length - 1);

        // Build line path
        let lineData = '';
        let areaData = '';
        const baseY = config.height - config.padding; // Bottom of chart area
        
        chartData.forEach((point, index) => {
            const x = config.padding + index * xScale;
            const y = config.height - config.padding - (valueRange > 0 ? (point.value - minValue) * yScale : chartAreaHeight / 2);
            
            if (index === 0) {
                lineData += `M ${x} ${y}`;
                areaData += `M ${x} ${baseY} L ${x} ${y}`; // Start from bottom
            } else {
                lineData += ` L ${x} ${y}`;
                areaData += ` L ${x} ${y}`;
            }
            
            // Close area path on last point
            if (index === chartData.length - 1) {
                areaData += ` L ${x} ${baseY} Z`; // Line to bottom and close
            }
        });

        // Update or create axis labels (now empty function)
        updateAxisLabels(svg, minValue, maxValue, config, metric);

        // Update hover interaction data with min/max info
        const overlay = svg.querySelector('.chart-overlay');
        if (overlay) {
            overlay._chartData = chartData.slice();
            overlay._metric = metric;
            overlay._config = config;
            overlay._minValue = minValue;
            overlay._maxValue = maxValue;
        }

        // Update paths immediately to prevent blinking
        path.setAttribute('d', lineData);
        if (area) area.setAttribute('d', areaData);
    }

    function updateAxisLabels(svg, minValue, maxValue, config, metric) {
        // Remove existing labels
        svg.querySelectorAll('.axis-label').forEach(label => label.remove());
        
        // No axis labels - information moved to hover tooltips for cleaner design
        return;
    }

    // Create different chart HTML for different layouts
    function createUsageChartHTML(guestId, metric) {
        const chartId = `chart-${guestId}-${metric}`;
        return `<div id="${chartId}" class="usage-chart-container"></div>`;
    }

    function createSparklineHTML(guestId, metric) {
        const chartId = `chart-${guestId}-${metric}`;
        return `<div id="${chartId}" class="sparkline-container"></div>`;
    }

    async function fetchChartData() {
        try {
            // Get current time range from dropdown
            const timeRangeSelect = document.getElementById('time-range-select');
            const timeRange = timeRangeSelect ? timeRangeSelect.value : '60';
            
            // Clear the processed data cache when fetching new data with different time range
            processedDataCache.clear();
            console.log(`[Charts] Fetching data for ${timeRange} minute range, cache cleared`);
            
            const response = await fetch(`/api/charts?range=${timeRange}`);
            if (!response.ok) {
                throw new Error(`HTTP ${response.status}`);
            }
            const data = await response.json();
            chartDataCache = data.data;
            nodeChartDataCache = data.nodeData || {};
            lastChartFetch = Date.now();
            return { guestData: data.data, nodeData: data.nodeData || {} };
        } catch (error) {
            console.error('Failed to fetch chart data:', error);
            return null;
        }
    }

    function shouldFetchChartData() {
        return !chartDataCache || (Date.now() - lastChartFetch) > CHART_FETCH_INTERVAL;
    }

    async function getChartData() {
        if (shouldFetchChartData()) {
            return await fetchChartData();
        }
        return { guestData: chartDataCache, nodeData: nodeChartDataCache };
    }

    function renderGuestCharts(guestId) {
        if (!chartDataCache || !chartDataCache[guestId]) {
            return;
        }

        const guestData = chartDataCache[guestId];
        
        // Batch DOM checks for efficiency
        const metricsToRender = [];
        
        // Check which charts exist in DOM
        ['cpu', 'memory', 'disk'].forEach(metric => {
            if (document.getElementById(`chart-${guestId}-${metric}`)) {
                metricsToRender.push({ metric, type: 'mini' });
            }
        });
        
        ['diskread', 'diskwrite', 'netin', 'netout'].forEach(metric => {
            if (document.getElementById(`chart-${guestId}-${metric}`)) {
                metricsToRender.push({ metric, type: 'sparkline' });
            }
        });
        
        // Render only existing charts
        metricsToRender.forEach(({ metric, type }) => {
            const chartId = `chart-${guestId}-${metric}`;
            const data = guestData[metric];
            createOrUpdateChart(chartId, data, metric, type, guestId);
        });
    }

    function renderNodeCharts(nodeId) {
        if (!nodeChartDataCache || !nodeChartDataCache[nodeId]) {
            return;
        }

        const nodeData = nodeChartDataCache[nodeId];
        
        // Check which charts exist in DOM for this node
        ['cpu', 'memory', 'disk'].forEach(metric => {
            const chartId = `chart-${nodeId}-${metric}`;
            if (document.getElementById(chartId)) {
                const data = nodeData[metric];
                createOrUpdateChart(chartId, data, metric, 'mini', nodeId);
            }
        });
    }

    function updateAllCharts() {
        if (!chartDataCache) return;
        
        // Batch updates using requestAnimationFrame immediately
        scheduleChartUpdates();
    }
    
    function scheduleChartUpdates() {
        if (updateRAF) {
            cancelAnimationFrame(updateRAF);
        }
        
        updateRAF = requestAnimationFrame(() => {
            performBatchedChartUpdates();
            updateRAF = null;
        });
    }
    
    function performBatchedChartUpdates() {
        if (!chartDataCache && !nodeChartDataCache) return;
        
        const startTime = performance.now();
        const maxUpdateTime = 16; // Target 60fps
        let updatedCount = 0;
        
        // Update guest charts
        if (chartDataCache) {
            const guestIds = Object.keys(chartDataCache);
            
            for (const guestId of guestIds) {
                // Check if we're over time budget
                if (performance.now() - startTime > maxUpdateTime && updatedCount > 0) {
                    // Schedule remaining updates for next frame
                    scheduleChartUpdates();
                    return;
                }
                
                // Only update if chart is visible
                const hasVisibleCharts = isGuestChartVisible(guestId);
                
                if (hasVisibleCharts) {
                    renderGuestCharts(guestId);
                    updatedCount++;
                }
            }
        }
        
        // Update node charts
        if (nodeChartDataCache) {
            const nodeIds = Object.keys(nodeChartDataCache);
            
            for (const nodeId of nodeIds) {
                // Check if we're over time budget
                if (performance.now() - startTime > maxUpdateTime && updatedCount > 0) {
                    // Schedule remaining updates for next frame
                    scheduleChartUpdates();
                    break;
                }
                
                // Only update if chart is visible
                const hasVisibleCharts = isNodeChartVisible(nodeId);
                
                if (hasVisibleCharts) {
                    renderNodeCharts(nodeId);
                    updatedCount++;
                }
            }
        }
    }
    
    function isGuestChartVisible(guestId) {
        // First check if charts mode is active
        const chartsToggle = document.getElementById('toggle-charts-checkbox');
        if (!chartsToggle || !chartsToggle.checked) {
            return false;
        }
        
        // Check visibility set for more accurate tracking
        const metrics = ['cpu', 'memory', 'disk', 'diskread', 'diskwrite', 'netin', 'netout'];
        return metrics.some(metric => {
            const chartId = `chart-${guestId}-${metric}`;
            return visibleCharts.has(chartId) || document.getElementById(chartId);
        });
    }
    
    function isNodeChartVisible(nodeId) {
        // First check if charts mode is active
        const chartsToggle = document.getElementById('toggle-charts-checkbox');
        if (!chartsToggle || !chartsToggle.checked) {
            return false;
        }
        
        // Check visibility set for more accurate tracking
        const metrics = ['cpu', 'memory', 'disk'];
        return metrics.some(metric => {
            const chartId = `chart-${nodeId}-${metric}`;
            return visibleCharts.has(chartId) || document.getElementById(chartId);
        });
    }

    function clearChart(guestId, metric) {
        const chartId = `chart-${guestId}-${metric}`;
        const container = document.getElementById(chartId);
        if (container) {
            container.innerHTML = '';
        }
    }


    let chartUpdateInterval = null;

    function startChartUpdates() {
        if (chartUpdateInterval) return;
        
        chartUpdateInterval = setInterval(async () => {
            // Skip updates if document is hidden (tab not active)
            if (document.hidden) {
                return;
            }
            
            const data = await getChartData();
            if (data) {
                updateAllCharts();
            }
        }, CHART_FETCH_INTERVAL);
        
        // Initial fetch
        getChartData();
        
        // Handle visibility change to pause/resume updates
        document.addEventListener('visibilitychange', () => {
            if (!document.hidden && chartDataCache) {
                // Tab became visible, update charts
                updateAllCharts();
            }
        });
    }


    // Adaptive sampling: more points for changing data, fewer for stable sections
    function processChartData(serverData, chartType = 'mini', guestId, metric) {
        if (!serverData || serverData.length === 0) {
            return [];
        }

        // Generate cache key
        const cacheKey = `${guestId}-${metric}-${chartType}`;
        
        // Quick hash of data for cache validation
        const dataHash = generateDataHash(serverData);
        
        // Check cache first
        const cached = processedDataCache.get(cacheKey);
        if (cached && cached.hash === dataHash && (Date.now() - cached.timestamp < 30000)) {
            // Use cached data if it's less than 30 seconds old and data hasn't changed
            return cached.data;
        }

        let targetPoints = CHART_CONFIG.renderPoints;
        
        // Sparklines are narrower, so they need fewer points to avoid bunching
        if (chartType === 'sparkline') {
            targetPoints = Math.round(targetPoints * 0.6); // 60% of mini chart points
            // Example: 1080p gets 40 points for usage charts, 24 points for I/O sparklines
        }
        
        let processedData;
        if (serverData.length <= targetPoints) {
            // Use all available data if we have fewer points than target
            processedData = serverData;
        } else {
            // Use adaptive sampling for optimal information density
            processedData = adaptiveSample(serverData, targetPoints);
        }
        
        // Cache the processed data
        processedDataCache.set(cacheKey, {
            data: processedData,
            timestamp: Date.now(),
            hash: dataHash
        });
        
        // Clean up old cache entries periodically
        if (processedDataCache.size > 200) {
            cleanupProcessedCache();
        }
        
        return processedData;
    }

    // Adaptive sampling algorithm: more points where data changes, fewer where stable
    function adaptiveSample(data, targetPoints) {
        if (data.length <= targetPoints) return data;
        
        // STRICT target enforcement: never exceed the limit
        const maxPoints = Math.max(2, targetPoints); // At least 2 points for a line
        
        // Step 1: Calculate importance scores for each point
        const importance = calculateImportanceScores(data);
        
        // Step 2: Always include first and last points
        const selectedIndices = new Set([0, data.length - 1]);
        const remainingPoints = maxPoints - 2; // Reserve 2 slots for start/end
        
        if (remainingPoints <= 0) {
            return [data[0], data[data.length - 1]]; // Just start and end
        }
        
        // Step 3: Find all candidates (excluding start/end)
        const candidates = [];
        for (let i = 1; i < data.length - 1; i++) {
            candidates.push({ index: i, importance: importance[i] });
        }
        
        // Step 4: Sort by importance and take only what we have room for
        candidates.sort((a, b) => b.importance - a.importance);
        
        // Step 5: Add the most important points up to our limit
        for (let i = 0; i < Math.min(remainingPoints, candidates.length); i++) {
            selectedIndices.add(candidates[i].index);
        }
        
        // Convert to sorted array and extract data points
        const sortedIndices = Array.from(selectedIndices).sort((a, b) => a - b);
        const result = sortedIndices.map(i => data[i]);
        
        return result;
    }

    // Calculate importance score for each data point (optimized)
    function calculateImportanceScores(data) {
        const scores = new Array(data.length);
        const windowSize = Math.min(5, Math.max(3, Math.floor(data.length / 50))); // Limit window size
        
        // Pre-calculate values array to avoid repeated property access
        const values = new Array(data.length);
        for (let i = 0; i < data.length; i++) {
            values[i] = data[i].value;
        }
        
        // Simplified scoring for better performance
        for (let i = 0; i < data.length; i++) {
            let score = 0;
            
            // 1. Simple rate of change (no complex derivatives)
            if (i > 0 && i < data.length - 1) {
                const change = Math.abs(values[i + 1] - values[i - 1]);
                score += change;
            }
            
            // 2. Local extrema detection (peaks and valleys)
            if (i > 0 && i < data.length - 1) {
                const isPeak = values[i] > values[i - 1] && values[i] > values[i + 1];
                const isValley = values[i] < values[i - 1] && values[i] < values[i + 1];
                if (isPeak || isValley) {
                    score += Math.abs(values[i] - values[i - 1]) + Math.abs(values[i] - values[i + 1]);
                }
            }
            
            // 3. Edge points get bonus
            if (i === 0 || i === data.length - 1) {
                score += 1000; // Ensure edges are always included
            }
            
            scores[i] = score;
        }
        
        return scores;
    }



    // Helper function to generate quick hash of data for cache validation
    function generateDataHash(data) {
        if (!data || data.length === 0) return '0';
        // Simple hash based on length, first, last, and middle values
        const first = data[0]?.value || 0;
        const last = data[data.length - 1]?.value || 0;
        const middle = data[Math.floor(data.length / 2)]?.value || 0;
        return `${data.length}-${first.toFixed(2)}-${middle.toFixed(2)}-${last.toFixed(2)}`;
    }
    
    // Cleanup old cache entries
    function cleanupProcessedCache() {
        const now = Date.now();
        const maxAge = 60000; // 1 minute
        
        for (const [key, value] of processedDataCache.entries()) {
            if (now - value.timestamp > maxAge) {
                processedDataCache.delete(key);
            }
        }
    }
    
    // Initialize visibility observer for better performance
    function initVisibilityObserver() {
        if (!window.IntersectionObserver) return;
        
        visibilityObserver = new IntersectionObserver((entries) => {
            entries.forEach(entry => {
                if (entry.isIntersecting) {
                    visibleCharts.add(entry.target.id);
                } else {
                    visibleCharts.delete(entry.target.id);
                }
            });
        }, {
            root: null,
            rootMargin: '50px', // Start rendering slightly before visible
            threshold: 0.01
        });
    }
    
    // Observe chart container for visibility
    function observeChartVisibility(container) {
        if (visibilityObserver && container.id) {
            visibilityObserver.observe(container);
        }
    }
    
    // Helper function to update hover indicator
    function updateHoverIndicator(svg, index, point, data, config, minValue, maxValue) {
        // Remove existing hover indicator
        let hoverDot = svg.querySelector('.hover-indicator');
        if (!hoverDot) {
            // Create new hover dot
            hoverDot = document.createElementNS('http://www.w3.org/2000/svg', 'circle');
            hoverDot.setAttribute('class', 'hover-indicator');
            hoverDot.setAttribute('r', '3');
            hoverDot.setAttribute('fill', '#ffffff');
            hoverDot.setAttribute('stroke', '#000000');
            hoverDot.setAttribute('stroke-width', '1.5');
            hoverDot.style.pointerEvents = 'none';
            svg.appendChild(hoverDot);
        }
        
        // Calculate position
        const chartAreaWidth = config.width - 2 * config.padding;
        const chartAreaHeight = config.height - 2 * config.padding;
        const xScale = chartAreaWidth / Math.max(1, data.length - 1);
        const valueRange = maxValue - minValue;
        const yScale = valueRange > 0 ? chartAreaHeight / valueRange : 0;
        
        const x = config.padding + index * xScale;
        const y = config.height - config.padding - (valueRange > 0 ? (point.value - minValue) * yScale : chartAreaHeight / 2);
        
        hoverDot.setAttribute('cx', x);
        hoverDot.setAttribute('cy', y);
        
        // Update stroke color based on theme
        const isDarkMode = document.documentElement.classList.contains('dark');
        hoverDot.setAttribute('stroke', isDarkMode ? '#ffffff' : '#000000');
        hoverDot.setAttribute('fill', isDarkMode ? '#000000' : '#ffffff');
    }
    
    // Initialize performance optimizations
    initVisibilityObserver();
    
    return {
        createUsageChartHTML,
        createSparklineHTML,
        renderGuestCharts,
        renderNodeCharts,
        updateAllCharts,
        getChartData,
        fetchChartData,
        startChartUpdates,
        processChartData,
        adaptiveSample,
        calculateImportanceScores,
        createOrUpdateChart,
        // Expose for testing
        cleanupProcessedCache,
        observeChartVisibility
    };
})(); 