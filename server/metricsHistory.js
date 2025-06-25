const HISTORY_RETENTION_MS = 7 * 24 * 60 * 60 * 1000; // 7 days
const MAX_DATA_POINTS = 50400; // Store data every 2 seconds for 7 days would be too much (302,400), 
                                // so we'll store every ~12 seconds on average (7*24*60*5 = 50,400)
const CLEANUP_INTERVAL_MS = 30 * 60 * 1000; // Clean up every 30 minutes

class MetricsHistory {
    constructor() {
        this.guestMetrics = new Map(); // guestId -> { dataPoints: CircularBuffer, lastCleanup: timestamp }
        this.nodeMetrics = new Map(); // nodeId -> { dataPoints: CircularBuffer, lastCleanup: timestamp }
        this.startCleanupTimer();
    }

    // Circular buffer implementation for efficient memory usage
    createCircularBuffer(maxSize) {
        return {
            buffer: new Array(maxSize),
            size: 0,
            head: 0,
            maxSize: maxSize,
            push(item) {
                this.buffer[this.head] = item;
                this.head = (this.head + 1) % this.maxSize;
                if (this.size < this.maxSize) this.size++;
            },
            toArray() {
                if (this.size === 0) return [];
                if (this.size < this.maxSize) {
                    return this.buffer.slice(0, this.size);
                }
                // Return items in chronological order
                const tail = this.head;
                return [...this.buffer.slice(tail), ...this.buffer.slice(0, tail)]
                    .filter(item => item !== undefined);
            },
            filter(fn) {
                return this.toArray().filter(fn);
            },
            get length() {
                return this.size;
            }
        };
    }


    addMetricData(guestId, currentMetrics) {
        const timestamp = Date.now();
        
        if (!this.guestMetrics.has(guestId)) {
            this.guestMetrics.set(guestId, {
                dataPoints: this.createCircularBuffer(MAX_DATA_POINTS),
                lastValues: null, // For rate calculation
                lastDataPoint: null // For compression
            });
        }

        const guestHistory = this.guestMetrics.get(guestId);
        const lastValues = guestHistory.lastValues;

        // Calculate rates if we have previous values
        let rates = null;
        if (lastValues && currentMetrics) {
            const timeDiffSeconds = (timestamp - lastValues.timestamp) / 1000;
            if (timeDiffSeconds > 0) {
                rates = {
                    diskReadRate: this.calculateRate(currentMetrics.diskread, lastValues.diskread, timeDiffSeconds),
                    diskWriteRate: this.calculateRate(currentMetrics.diskwrite, lastValues.diskwrite, timeDiffSeconds),
                    netInRate: this.calculateRate(currentMetrics.netin, lastValues.netin, timeDiffSeconds),
                    netOutRate: this.calculateRate(currentMetrics.netout, lastValues.netout, timeDiffSeconds)
                };
            }
        }

        // Compress data by only storing changed values for cumulative metrics
        const lastDataPoint = guestHistory.lastDataPoint;
        const dataPoint = {
            timestamp,
            cpu: currentMetrics?.cpu || 0,
            mem: currentMetrics?.mem || 0,
            disk: currentMetrics?.disk || 0
        };
        
        // Only store cumulative values if they changed (saves memory)
        if (!lastDataPoint || lastDataPoint.diskread !== (currentMetrics?.diskread || 0)) {
            dataPoint.diskread = currentMetrics?.diskread || 0;
        }
        if (!lastDataPoint || lastDataPoint.diskwrite !== (currentMetrics?.diskwrite || 0)) {
            dataPoint.diskwrite = currentMetrics?.diskwrite || 0;
        }
        if (!lastDataPoint || lastDataPoint.netin !== (currentMetrics?.netin || 0)) {
            dataPoint.netin = currentMetrics?.netin || 0;
        }
        if (!lastDataPoint || lastDataPoint.netout !== (currentMetrics?.netout || 0)) {
            dataPoint.netout = currentMetrics?.netout || 0;
        }
        
        // Guest memory if available and changed
        if (currentMetrics?.guest_mem_actual_used_bytes !== undefined && 
            (!lastDataPoint || lastDataPoint.guest_mem_actual_used_bytes !== currentMetrics.guest_mem_actual_used_bytes)) {
            dataPoint.guest_mem_actual_used_bytes = currentMetrics.guest_mem_actual_used_bytes;
        }
        if (currentMetrics?.guest_mem_total_bytes !== undefined && 
            (!lastDataPoint || lastDataPoint.guest_mem_total_bytes !== currentMetrics.guest_mem_total_bytes)) {
            dataPoint.guest_mem_total_bytes = currentMetrics.guest_mem_total_bytes;
        }
        
        // Calculated rates
        if (rates) {
            Object.assign(dataPoint, rates);
        }

        // Don't store if values haven't changed significantly (within 0.1% for CPU/mem)
        if (lastDataPoint && 
            Math.abs(dataPoint.cpu - lastDataPoint.cpu) < 0.001 &&
            Math.abs(dataPoint.mem - lastDataPoint.mem) < 0.001 &&
            dataPoint.disk === lastDataPoint.disk &&
            !rates) {
            // Skip storing this data point since values haven't changed
            // Note: We do NOT update the timestamp of the last point to preserve accurate time history
        } else {
            guestHistory.dataPoints.push(dataPoint);
            guestHistory.lastDataPoint = { ...dataPoint }; // Store full copy for comparison
        }
        
        // Update last values for next rate calculation
        guestHistory.lastValues = {
            timestamp,
            diskread: currentMetrics?.diskread || 0,
            diskwrite: currentMetrics?.diskwrite || 0,
            netin: currentMetrics?.netin || 0,
            netout: currentMetrics?.netout || 0
        };
    }

    calculateRate(currentValue, previousValue, timeDiffSeconds) {
        if (typeof currentValue !== 'number' || typeof previousValue !== 'number') {
            return null;
        }
        
        const valueDiff = currentValue - previousValue;
        if (valueDiff < 0 || timeDiffSeconds <= 0) {
            return null; // Reset or invalid data
        }
        
        return valueDiff / timeDiffSeconds;
    }

    addNodeMetricData(nodeId, nodeData) {
        const timestamp = Date.now();
        
        if (!this.nodeMetrics.has(nodeId)) {
            this.nodeMetrics.set(nodeId, {
                dataPoints: this.createCircularBuffer(MAX_DATA_POINTS),
                lastDataPoint: null // For compression
            });
        }

        const nodeHistory = this.nodeMetrics.get(nodeId);

        // Compress data by only storing changed values
        const lastDataPoint = nodeHistory.lastDataPoint;
        // Handle different data structures from discovery vs status endpoint
        let memUsed = 0, memTotal = 0, diskUsed = 0, diskTotal = 0;
        
        if (typeof nodeData?.memory === 'object' && nodeData.memory !== null) {
            // From /nodes/{node}/status endpoint - memory field
            memUsed = nodeData.memory.used || 0;
            memTotal = nodeData.memory.total || 0;
        } else if (typeof nodeData?.mem === 'object' && nodeData.mem !== null) {
            // Alternative structure - mem field
            memUsed = nodeData.mem.used || 0;
            memTotal = nodeData.mem.total || 0;
        } else {
            // From discovery endpoint
            memUsed = nodeData?.mem || 0;
            memTotal = nodeData?.maxmem || 0;
        }
        
        if (typeof nodeData?.rootfs === 'object' && nodeData.rootfs !== null) {
            // From /nodes/{node}/status endpoint
            diskUsed = nodeData.rootfs.used || 0;
            diskTotal = nodeData.rootfs.total || 0;
        } else if (typeof nodeData?.disk === 'object' && nodeData.disk !== null) {
            // Alternative structure
            diskUsed = nodeData.disk.used || 0;
            diskTotal = nodeData.disk.total || 0;
        } else {
            // From discovery endpoint
            diskUsed = nodeData?.disk || 0;
            diskTotal = nodeData?.maxdisk || 0;
        }
        
        const dataPoint = {
            timestamp,
            cpu: nodeData?.cpu || 0,
            mem: memUsed,
            disk: diskUsed,
            maxmem: memTotal,
            maxdisk: diskTotal
        };
        

        // If maxmem or maxdisk are 0, try to use values from last data point
        if (dataPoint.maxmem === 0 && lastDataPoint && lastDataPoint.maxmem > 0) {
            dataPoint.maxmem = lastDataPoint.maxmem;
        }
        if (dataPoint.maxdisk === 0 && lastDataPoint && lastDataPoint.maxdisk > 0) {
            dataPoint.maxdisk = lastDataPoint.maxdisk;
        }
        
        // Don't store if values haven't changed significantly
        // For CPU: within 0.1% (since it's already a percentage)
        // For memory/disk: within 0.1% of the total capacity
        const cpuChanged = !lastDataPoint || Math.abs(dataPoint.cpu - lastDataPoint.cpu) >= 0.001;
        const memChanged = !lastDataPoint || 
            (dataPoint.maxmem > 0 && Math.abs(dataPoint.mem - lastDataPoint.mem) / dataPoint.maxmem >= 0.001);
        const diskChanged = !lastDataPoint || 
            (dataPoint.maxdisk > 0 && Math.abs(dataPoint.disk - lastDataPoint.disk) / dataPoint.maxdisk >= 0.001);
        
        
        if (cpuChanged || memChanged || diskChanged) {
            nodeHistory.dataPoints.push(dataPoint);
            nodeHistory.lastDataPoint = { ...dataPoint }; // Store full copy for comparison
        } else {
            // Skip storing this data point since values haven't changed
            // Note: We do NOT update the timestamp of the last point to preserve accurate time history
        }
    }

    getNodeChartData(nodeId, metric) {
        if (!this.nodeMetrics.has(nodeId)) {
            return [];
        }

        const nodeHistory = this.nodeMetrics.get(nodeId);
        const cutoffTime = Date.now() - HISTORY_RETENTION_MS;
        
        const dataPoints = nodeHistory.dataPoints.filter(point => point && point.timestamp >= cutoffTime);
        
        return dataPoints
            .map(point => {
                return {
                    timestamp: point.timestamp,
                    value: this.getNodeMetricValue(point, metric)
                };
            })
            .filter(point => point.value !== null && point.value !== undefined);
    }

    getNodeMetricValue(dataPoint, metric) {
        switch (metric) {
            case 'cpu':
                return dataPoint.cpu * 100; // Convert to percentage
            case 'memory':
                // Calculate percentage
                if (dataPoint.maxmem && dataPoint.maxmem > 0) {
                    return (dataPoint.mem / dataPoint.maxmem) * 100;
                }
                return null;
            case 'disk':
                // Calculate percentage
                if (dataPoint.maxdisk && dataPoint.maxdisk > 0) {
                    return (dataPoint.disk / dataPoint.maxdisk) * 100;
                }
                return null;
            default:
                return dataPoint[metric];
        }
    }

    getChartData(guestId, metric) {
        if (!this.guestMetrics.has(guestId)) {
            return [];
        }

        const guestHistory = this.guestMetrics.get(guestId);
        const cutoffTime = Date.now() - HISTORY_RETENTION_MS;
        
        const dataPoints = guestHistory.dataPoints.filter(point => point && point.timestamp >= cutoffTime);
        
        // Reconstruct full data points from compressed storage
        let lastCompletePoint = null;
        return dataPoints
            .map(point => {
                // Fill in missing cumulative values from last complete point
                const fullPoint = { ...point };
                if (lastCompletePoint) {
                    fullPoint.diskread = point.diskread !== undefined ? point.diskread : lastCompletePoint.diskread;
                    fullPoint.diskwrite = point.diskwrite !== undefined ? point.diskwrite : lastCompletePoint.diskwrite;
                    fullPoint.netin = point.netin !== undefined ? point.netin : lastCompletePoint.netin;
                    fullPoint.netout = point.netout !== undefined ? point.netout : lastCompletePoint.netout;
                }
                lastCompletePoint = fullPoint;
                
                return {
                    timestamp: fullPoint.timestamp,
                    value: this.getMetricValue(fullPoint, metric)
                };
            })
            .filter(point => point.value !== null && point.value !== undefined);
    }

    getMetricValue(dataPoint, metric) {
        switch (metric) {
            case 'cpu':
                return dataPoint.cpu * 100; // Convert to percentage
            case 'memory':
                // Use guest memory if available, fallback to host memory
                if (dataPoint.guest_mem_actual_used_bytes && dataPoint.guest_mem_total_bytes) {
                    return (dataPoint.guest_mem_actual_used_bytes / dataPoint.guest_mem_total_bytes) * 100;
                }
                return null; // Will need total memory from guest info for percentage
            case 'diskread':
                return dataPoint.diskReadRate;
            case 'diskwrite':
                return dataPoint.diskWriteRate;
            case 'netin':
                return dataPoint.netInRate;
            case 'netout':
                return dataPoint.netOutRate;
            default:
                return dataPoint[metric];
        }
    }

    // Enhanced method to get metric value with guest context
    getMetricValueWithContext(dataPoint, metric, guestInfo = null) {
        switch (metric) {
            case 'cpu':
                return dataPoint.cpu * 100;
            case 'memory':
                // Priority: guest memory > dataPoint percentage calculation > null
                if (dataPoint.guest_mem_actual_used_bytes && dataPoint.guest_mem_total_bytes) {
                    return (dataPoint.guest_mem_actual_used_bytes / dataPoint.guest_mem_total_bytes) * 100;
                } else if (guestInfo && guestInfo.maxmem && dataPoint.mem) {
                    return (dataPoint.mem / guestInfo.maxmem) * 100;
                }
                return null;
            case 'disk':
                // Calculate disk usage percentage
                if (guestInfo && guestInfo.maxdisk && dataPoint.disk) {
                    return (dataPoint.disk / guestInfo.maxdisk) * 100;
                }
                return null;
            case 'diskread':
                return dataPoint.diskReadRate;
            case 'diskwrite':
                return dataPoint.diskWriteRate;
            case 'netin':
                return dataPoint.netInRate;
            case 'netout':
                return dataPoint.netOutRate;
            default:
                return dataPoint[metric];
        }
    }

    getAllGuestChartData(guestInfoMap = null, timeRangeMinutes = 60) {
        const result = {};
        const timeRangeMs = timeRangeMinutes * 60 * 1000;
        const cutoffTime = Date.now() - timeRangeMs;

        for (const [guestId, guestHistory] of this.guestMetrics) {
            const validDataPoints = guestHistory.dataPoints
                .filter(point => point && point.timestamp >= cutoffTime);

            if (validDataPoints.length > 0) {
                const guestInfo = guestInfoMap ? guestInfoMap[guestId] : null;
                result[guestId] = {
                    cpu: this.extractMetricSeriesWithContext(validDataPoints, 'cpu', guestInfo),
                    memory: this.extractMetricSeriesWithContext(validDataPoints, 'memory', guestInfo),
                    disk: this.extractMetricSeriesWithContext(validDataPoints, 'disk', guestInfo),
                    diskread: this.extractMetricSeriesWithContext(validDataPoints, 'diskread', guestInfo),
                    diskwrite: this.extractMetricSeriesWithContext(validDataPoints, 'diskwrite', guestInfo),
                    netin: this.extractMetricSeriesWithContext(validDataPoints, 'netin', guestInfo),
                    netout: this.extractMetricSeriesWithContext(validDataPoints, 'netout', guestInfo)
                };
            }
        }

        return result;
    }

    getAllNodeChartData(timeRangeMinutes = 60) {
        const result = {};
        const timeRangeMs = timeRangeMinutes * 60 * 1000;
        const cutoffTime = Date.now() - timeRangeMs;

        for (const [nodeId, nodeHistory] of this.nodeMetrics) {
            const validDataPoints = nodeHistory.dataPoints
                .filter(point => point && point.timestamp >= cutoffTime);

            if (validDataPoints.length > 0) {
                result[nodeId] = {
                    cpu: this.extractNodeMetricSeries(validDataPoints, 'cpu'),
                    memory: this.extractNodeMetricSeries(validDataPoints, 'memory'),
                    disk: this.extractNodeMetricSeries(validDataPoints, 'disk')
                };
            }
        }

        return result;
    }

    extractNodeMetricSeries(dataPoints, metric) {
        return dataPoints
            .map(point => {
                return {
                    timestamp: point.timestamp,
                    value: this.getNodeMetricValue(point, metric)
                };
            })
            .filter(point => point.value !== null && point.value !== undefined);
    }

    extractMetricSeriesWithContext(dataPoints, metric, guestInfo = null) {
        // Reconstruct full data points from compressed storage
        let lastCompletePoint = null;
        return dataPoints
            .map(point => {
                // Fill in missing cumulative values from last complete point
                const fullPoint = { ...point };
                if (lastCompletePoint) {
                    fullPoint.diskread = point.diskread !== undefined ? point.diskread : lastCompletePoint.diskread;
                    fullPoint.diskwrite = point.diskwrite !== undefined ? point.diskwrite : lastCompletePoint.diskwrite;
                    fullPoint.netin = point.netin !== undefined ? point.netin : lastCompletePoint.netin;
                    fullPoint.netout = point.netout !== undefined ? point.netout : lastCompletePoint.netout;
                    fullPoint.guest_mem_actual_used_bytes = point.guest_mem_actual_used_bytes !== undefined ? 
                        point.guest_mem_actual_used_bytes : lastCompletePoint.guest_mem_actual_used_bytes;
                    fullPoint.guest_mem_total_bytes = point.guest_mem_total_bytes !== undefined ? 
                        point.guest_mem_total_bytes : lastCompletePoint.guest_mem_total_bytes;
                }
                lastCompletePoint = fullPoint;
                
                return {
                    timestamp: fullPoint.timestamp,
                    value: this.getMetricValueWithContext(fullPoint, metric, guestInfo)
                };
            })
            .filter(point => point.value !== null && point.value !== undefined);
    }

    cleanupOldData(guestHistory) {
        // Circular buffer handles cleanup automatically, just check timestamps
        // This method is now mainly for compatibility
        if (!guestHistory) return;
    }

    startCleanupTimer() {
        setInterval(() => {
            const cutoffTime = Date.now() - HISTORY_RETENTION_MS;
            
            // Cleanup guest metrics
            for (const [guestId, guestHistory] of this.guestMetrics) {
                this.cleanupOldData(guestHistory);
                
                // Remove guests with no recent data
                const recentData = guestHistory.dataPoints.filter(
                    point => point && point.timestamp >= cutoffTime
                );
                if (recentData.length === 0) {
                    this.guestMetrics.delete(guestId);
                }
            }
            
            // Cleanup node metrics
            for (const [nodeId, nodeHistory] of this.nodeMetrics) {
                this.cleanupOldData(nodeHistory);
                
                // Remove nodes with no recent data
                const recentData = nodeHistory.dataPoints.filter(
                    point => point && point.timestamp >= cutoffTime
                );
                if (recentData.length === 0) {
                    this.nodeMetrics.delete(nodeId);
                }
            }
        }, CLEANUP_INTERVAL_MS);
    }

    clearGuest(guestId) {
        this.guestMetrics.delete(guestId);
    }

    getStats() {
        const guestDataPoints = Array.from(this.guestMetrics.values())
            .reduce((sum, guest) => sum + guest.dataPoints.length, 0);
        const nodeDataPoints = Array.from(this.nodeMetrics.values())
            .reduce((sum, node) => sum + node.dataPoints.length, 0);
            
        return {
            totalGuests: this.guestMetrics.size,
            totalNodes: this.nodeMetrics.size,
            totalDataPoints: guestDataPoints + nodeDataPoints,
            guestDataPoints,
            nodeDataPoints,
            maxDataPointsPerEntity: MAX_DATA_POINTS,
            retentionDays: HISTORY_RETENTION_MS / (24 * 60 * 60 * 1000),
            estimatedMemoryUsage: this.estimateMemoryUsage(),
            oldestDataTimestamp: this.getOldestDataTimestamp()
        };
    }

    getOldestDataTimestamp() {
        let oldestTimestamp = null;
        
        // Check guest metrics
        for (const [guestId, guestHistory] of this.guestMetrics) {
            const dataPoints = guestHistory.dataPoints.toArray();
            if (dataPoints.length > 0) {
                const firstPoint = dataPoints[0];
                if (firstPoint && firstPoint.timestamp) {
                    if (!oldestTimestamp || firstPoint.timestamp < oldestTimestamp) {
                        oldestTimestamp = firstPoint.timestamp;
                    }
                }
            }
        }
        
        // Check node metrics
        for (const [nodeId, nodeHistory] of this.nodeMetrics) {
            const dataPoints = nodeHistory.dataPoints.toArray();
            if (dataPoints.length > 0) {
                const firstPoint = dataPoints[0];
                if (firstPoint && firstPoint.timestamp) {
                    if (!oldestTimestamp || firstPoint.timestamp < oldestTimestamp) {
                        oldestTimestamp = firstPoint.timestamp;
                    }
                }
            }
        }
        
        return oldestTimestamp;
    }

    estimateMemoryUsage() {
        // Rough estimation of memory usage in bytes
        let totalBytes = 0;
        
        // Guest metrics
        for (const [guestId, guestHistory] of this.guestMetrics) {
            // Estimate ~100 bytes per data point (compressed)
            totalBytes += guestHistory.dataPoints.length * 100;
            // Add overhead for maps and structures
            totalBytes += 1024;
        }
        
        // Node metrics
        for (const [nodeId, nodeHistory] of this.nodeMetrics) {
            // Estimate ~80 bytes per data point (less data than guests)
            totalBytes += nodeHistory.dataPoints.length * 80;
            // Add overhead for maps and structures
            totalBytes += 1024;
        }
        
        return totalBytes;
    }
}

// Singleton instance
const metricsHistory = new MetricsHistory();

module.exports = metricsHistory; 