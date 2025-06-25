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
                // Clear old reference to prevent memory leak
                if (this.size === this.maxSize) {
                    this.buffer[this.head] = null;
                }
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
                lastValues: null // For rate calculation
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

        // Always store all values to avoid compression artifacts
        // This ensures accurate rate calculations and data reconstruction
        const dataPoint = {
            timestamp,
            cpu: currentMetrics?.cpu || 0,
            mem: currentMetrics?.mem || 0,
            disk: currentMetrics?.disk || 0,
            // Always store cumulative values for accurate rate calculations
            diskread: currentMetrics?.diskread || 0,
            diskwrite: currentMetrics?.diskwrite || 0,
            netin: currentMetrics?.netin || 0,
            netout: currentMetrics?.netout || 0
        };
        
        // Guest memory if available
        if (currentMetrics?.guest_mem_actual_used_bytes !== undefined) {
            dataPoint.guest_mem_actual_used_bytes = currentMetrics.guest_mem_actual_used_bytes;
        }
        if (currentMetrics?.guest_mem_total_bytes !== undefined) {
            dataPoint.guest_mem_total_bytes = currentMetrics.guest_mem_total_bytes;
        }
        
        // Calculated rates
        if (rates) {
            Object.assign(dataPoint, rates);
        }

        // Always store the data point - let persistence layer handle downsampling
        guestHistory.dataPoints.push(dataPoint);
        
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
        
        // Detect counter resets (current < previous)
        // This can happen when:
        // 1. VM restarts and counters reset to 0
        // 2. Counter overflow (very rare)
        // 3. Proxmox API reset
        if (currentValue < previousValue) {
            // Counter reset detected
            // For now, return null to avoid negative rates
            // In the future, we could estimate rate if we knew the max counter value
            return null;
        }
        
        const valueDiff = currentValue - previousValue;
        if (timeDiffSeconds <= 0) {
            return null; // Invalid time difference
        }
        
        // Detect unrealistic time gaps (> 5 minutes) that indicate a restart or data gap
        // This prevents huge spikes when cumulative values jump after a restart
        if (timeDiffSeconds > 300) {
            return null; // Gap too large, don't calculate rate
        }
        
        // Calculate the rate
        const rate = valueDiff / timeDiffSeconds;
        
        // Smart anomaly detection instead of hard limits
        // Check if this is the first rate calculation after a gap
        const isFirstAfterGap = timeDiffSeconds > 10; // Normal interval is 2 seconds
        
        if (isFirstAfterGap) {
            // After a gap, we can't trust the rate calculation
            // The cumulative values might have increased significantly during downtime
            // Only accept if the rate is reasonable for the time period
            const reasonableRate = valueDiff / Math.min(timeDiffSeconds, 10); // Assume at most 10 seconds of activity
            return reasonableRate;
        }
        
        // For continuous monitoring (no gaps), allow any rate
        // Real hardware can achieve very high speeds
        return rate;
    }

    addNodeMetricData(nodeId, nodeData) {
        const timestamp = Date.now();
        
        if (!this.nodeMetrics.has(nodeId)) {
            this.nodeMetrics.set(nodeId, {
                dataPoints: this.createCircularBuffer(MAX_DATA_POINTS)
            });
        }

        const nodeHistory = this.nodeMetrics.get(nodeId);

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
        
        // Always store all values - let persistence layer handle downsampling
        const dataPoint = {
            timestamp,
            cpu: nodeData?.cpu || 0,
            mem: memUsed,
            disk: diskUsed,
            maxmem: memTotal,
            maxdisk: diskTotal
        };
        
        // Always store the data point
        nodeHistory.dataPoints.push(dataPoint);
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
                // Only filter truly impossible rates (>50GB/s)
                const diskReadRate = dataPoint.diskReadRate;
                if (diskReadRate && diskReadRate > 50 * 1024 * 1024 * 1024) {
                    return null;
                }
                return diskReadRate;
            case 'diskwrite':
                const diskWriteRate = dataPoint.diskWriteRate;
                if (diskWriteRate && diskWriteRate > 50 * 1024 * 1024 * 1024) {
                    return null;
                }
                return diskWriteRate;
            case 'netin':
                const netInRate = dataPoint.netInRate;
                if (netInRate && netInRate > 50 * 1024 * 1024 * 1024) {
                    return null;
                }
                return netInRate;
            case 'netout':
                const netOutRate = dataPoint.netOutRate;
                if (netOutRate && netOutRate > 50 * 1024 * 1024 * 1024) {
                    return null;
                }
                return netOutRate;
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
                // Only filter truly impossible rates (>50GB/s)
                const diskReadRate = dataPoint.diskReadRate;
                if (diskReadRate && diskReadRate > 50 * 1024 * 1024 * 1024) {
                    return null;
                }
                return diskReadRate;
            case 'diskwrite':
                const diskWriteRate = dataPoint.diskWriteRate;
                if (diskWriteRate && diskWriteRate > 50 * 1024 * 1024 * 1024) {
                    return null;
                }
                return diskWriteRate;
            case 'netin':
                const netInRate = dataPoint.netInRate;
                if (netInRate && netInRate > 50 * 1024 * 1024 * 1024) {
                    return null;
                }
                return netInRate;
            case 'netout':
                const netOutRate = dataPoint.netOutRate;
                if (netOutRate && netOutRate > 50 * 1024 * 1024 * 1024) {
                    return null;
                }
                return netOutRate;
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

    // Export methods for persistence
    exportGuestMetrics(timeRangeMinutes = null) {
        const result = {};
        const cutoffTime = timeRangeMinutes ? Date.now() - (timeRangeMinutes * 60 * 1000) : 0;

        for (const [guestId, guestHistory] of this.guestMetrics) {
            const dataPoints = guestHistory.dataPoints
                .toArray()
                .filter(point => point && point.timestamp >= cutoffTime);
            
            if (dataPoints.length > 0) {
                result[guestId] = dataPoints;
            }
        }

        return result;
    }

    exportNodeMetrics(timeRangeMinutes = null) {
        const result = {};
        const cutoffTime = timeRangeMinutes ? Date.now() - (timeRangeMinutes * 60 * 1000) : 0;

        for (const [nodeId, nodeHistory] of this.nodeMetrics) {
            const dataPoints = nodeHistory.dataPoints
                .toArray()
                .filter(point => point && point.timestamp >= cutoffTime);
            
            if (dataPoints.length > 0) {
                result[nodeId] = dataPoints;
            }
        }

        return result;
    }

    // Import methods for persistence
    importGuestMetrics(guestId, dataPoints) {
        if (!dataPoints || dataPoints.length === 0) return;

        if (!this.guestMetrics.has(guestId)) {
            this.guestMetrics.set(guestId, {
                dataPoints: this.createCircularBuffer(MAX_DATA_POINTS),
                lastValues: null
            });
        }

        const guestHistory = this.guestMetrics.get(guestId);
        
        // Sort by timestamp and add to buffer
        const sortedPoints = dataPoints.sort((a, b) => a.timestamp - b.timestamp);
        for (const point of sortedPoints) {
            // Skip if point is too old
            if (Date.now() - point.timestamp > HISTORY_RETENTION_MS) continue;
            
            // Sanitize imported rate values only if they're impossibly high
            // 50 GB/s is beyond any current hardware capability
            const impossibleRate = 50 * 1024 * 1024 * 1024;
            
            if (point.diskReadRate && point.diskReadRate > impossibleRate) {
                point.diskReadRate = null;
            }
            if (point.diskWriteRate && point.diskWriteRate > impossibleRate) {
                point.diskWriteRate = null;
            }
            if (point.netInRate && point.netInRate > impossibleRate) {
                point.netInRate = null;
            }
            if (point.netOutRate && point.netOutRate > impossibleRate) {
                point.netOutRate = null;
            }
            
            guestHistory.dataPoints.push(point);
        }

        // Set lastValues for rate calculations
        // Only set if the last point is recent enough (< 5 minutes old)
        // This prevents rate spikes after restarts with old data
        if (sortedPoints.length > 0) {
            const lastPoint = sortedPoints[sortedPoints.length - 1];
            const age = Date.now() - lastPoint.timestamp;
            
            if (age < 5 * 60 * 1000) {
                // Recent data - safe to use for rate calculations
                guestHistory.lastValues = {
                    timestamp: lastPoint.timestamp,
                    diskread: lastPoint.diskread || 0,
                    diskwrite: lastPoint.diskwrite || 0,
                    netin: lastPoint.netin || 0,
                    netout: lastPoint.netout || 0
                };
            } else {
                // Old data - don't use for rate calculations
                guestHistory.lastValues = null;
            }
        }
    }

    importNodeMetrics(nodeId, dataPoints) {
        if (!dataPoints || dataPoints.length === 0) return;

        if (!this.nodeMetrics.has(nodeId)) {
            this.nodeMetrics.set(nodeId, {
                dataPoints: this.createCircularBuffer(MAX_DATA_POINTS)
            });
        }

        const nodeHistory = this.nodeMetrics.get(nodeId);
        
        // Sort by timestamp and add to buffer
        const sortedPoints = dataPoints.sort((a, b) => a.timestamp - b.timestamp);
        for (const point of sortedPoints) {
            // Skip if point is too old
            if (Date.now() - point.timestamp > HISTORY_RETENTION_MS) continue;
            
            nodeHistory.dataPoints.push(point);
        }
    }
}

// Singleton instance
const metricsHistory = new MetricsHistory();

module.exports = metricsHistory; 