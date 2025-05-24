const HISTORY_RETENTION_MS = 60 * 60 * 1000; // 1 hour
const MAX_DATA_POINTS = 1800; // 2-second intervals for 1 hour (3600/2 = 1800)
const CLEANUP_INTERVAL_MS = 5 * 60 * 1000; // Clean up every 5 minutes

class MetricsHistory {
    constructor() {
        this.guestMetrics = new Map(); // guestId -> { dataPoints: [], lastCleanup: timestamp }
        this.startCleanupTimer();
    }

    addMetricData(guestId, currentMetrics) {
        const timestamp = Date.now();
        
        if (!this.guestMetrics.has(guestId)) {
            this.guestMetrics.set(guestId, {
                dataPoints: [],
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

        // Store data point with both current values and calculated rates
        const dataPoint = {
            timestamp,
            cpu: currentMetrics?.cpu || 0,
            mem: currentMetrics?.mem || 0,
            disk: currentMetrics?.disk || 0,
            diskread: currentMetrics?.diskread || 0,
            diskwrite: currentMetrics?.diskwrite || 0,
            netin: currentMetrics?.netin || 0,
            netout: currentMetrics?.netout || 0,
            // Guest memory if available
            guest_mem_actual_used_bytes: currentMetrics?.guest_mem_actual_used_bytes,
            guest_mem_total_bytes: currentMetrics?.guest_mem_total_bytes,
            // Calculated rates
            ...rates
        };

        guestHistory.dataPoints.push(dataPoint);
        
        // Update last values for next rate calculation
        guestHistory.lastValues = {
            timestamp,
            diskread: currentMetrics?.diskread || 0,
            diskwrite: currentMetrics?.diskwrite || 0,
            netin: currentMetrics?.netin || 0,
            netout: currentMetrics?.netout || 0
        };

        // Trim to max data points
        if (guestHistory.dataPoints.length > MAX_DATA_POINTS) {
            guestHistory.dataPoints = guestHistory.dataPoints.slice(-MAX_DATA_POINTS);
        }

        // Remove old data points
        this.cleanupOldData(guestHistory);
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

    getChartData(guestId, metric) {
        if (!this.guestMetrics.has(guestId)) {
            return [];
        }

        const guestHistory = this.guestMetrics.get(guestId);
        const cutoffTime = Date.now() - HISTORY_RETENTION_MS;
        
        return guestHistory.dataPoints
            .filter(point => point.timestamp >= cutoffTime)
            .map(point => ({
                timestamp: point.timestamp,
                value: this.getMetricValue(point, metric)
            }))
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

    getAllGuestChartData(guestInfoMap = null) {
        const result = {};
        const cutoffTime = Date.now() - HISTORY_RETENTION_MS;

        for (const [guestId, guestHistory] of this.guestMetrics) {
            const validDataPoints = guestHistory.dataPoints
                .filter(point => point.timestamp >= cutoffTime);

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

    extractMetricSeriesWithContext(dataPoints, metric, guestInfo = null) {
        return dataPoints
            .map(point => ({
                timestamp: point.timestamp,
                value: this.getMetricValueWithContext(point, metric, guestInfo)
            }))
            .filter(point => point.value !== null && point.value !== undefined);
    }

    cleanupOldData(guestHistory) {
        if (!guestHistory) return;
        
        const cutoffTime = Date.now() - HISTORY_RETENTION_MS;
        guestHistory.dataPoints = guestHistory.dataPoints
            .filter(point => point.timestamp >= cutoffTime);
    }

    startCleanupTimer() {
        setInterval(() => {
            const cutoffTime = Date.now() - HISTORY_RETENTION_MS;
            
            for (const [guestId, guestHistory] of this.guestMetrics) {
                this.cleanupOldData(guestHistory);
                
                // Remove guests with no recent data
                if (guestHistory.dataPoints.length === 0) {
                    this.guestMetrics.delete(guestId);
                }
            }
        }, CLEANUP_INTERVAL_MS);
    }

    clearGuest(guestId) {
        this.guestMetrics.delete(guestId);
    }

    getStats() {
        return {
            totalGuests: this.guestMetrics.size,
            totalDataPoints: Array.from(this.guestMetrics.values())
                .reduce((sum, guest) => sum + guest.dataPoints.length, 0)
        };
    }
}

// Singleton instance
const metricsHistory = new MetricsHistory();

module.exports = metricsHistory; 