const fs = require('fs').promises;
const path = require('path');
const zlib = require('zlib');
const { promisify } = require('util');

const gzip = promisify(zlib.gzip);
const gunzip = promisify(zlib.gunzip);

class MetricsPersistence {
    constructor(dataPath = '/opt/pulse/data') {
        this.dataPath = dataPath;
        this.snapshotFile = path.join(dataPath, 'metrics-snapshot.json.gz');
        this.tempFile = path.join(dataPath, '.metrics-snapshot.tmp.gz');
        this.maxRetentionHours = 7 * 24; // Keep 7 days of data to match memory retention
    }

    async saveSnapshot(metricsHistory) {
        try {
            console.log('[MetricsPersistence] Starting snapshot save...');
            const startTime = Date.now();
            
            // Get current data from metricsHistory
            const snapshot = {
                version: 2, // Bumped for timezone support
                timestamp: Date.now(),
                timezone: Intl.DateTimeFormat().resolvedOptions().timeZone,
                timezoneOffset: new Date().getTimezoneOffset(), // Minutes offset from UTC
                guests: {},
                nodes: {}
            };

            // Export guest metrics with downsampling
            const guestData = metricsHistory.exportGuestMetrics(this.maxRetentionHours * 60);
            for (const [guestId, metrics] of Object.entries(guestData)) {
                snapshot.guests[guestId] = this.downsampleMetrics(metrics);
            }

            // Export node metrics with downsampling
            const nodeData = metricsHistory.exportNodeMetrics(this.maxRetentionHours * 60);
            for (const [nodeId, metrics] of Object.entries(nodeData)) {
                snapshot.nodes[nodeId] = this.downsampleMetrics(metrics);
            }

            // Convert to JSON and compress
            const jsonData = JSON.stringify(snapshot);
            const compressed = await gzip(jsonData);

            await fs.writeFile(this.tempFile, compressed);
            await fs.rename(this.tempFile, this.snapshotFile);

            const elapsedMs = Date.now() - startTime;
            const stats = {
                guests: Object.keys(snapshot.guests).length,
                nodes: Object.keys(snapshot.nodes).length,
                totalDataPoints: this.countDataPoints(snapshot),
                sizeBytes: compressed.length,
                elapsedMs
            };

            console.log(`[MetricsPersistence] Snapshot saved: ${stats.guests} guests, ${stats.nodes} nodes, ${stats.totalDataPoints} points, ${(stats.sizeBytes / 1024).toFixed(1)}KB in ${stats.elapsedMs}ms`);
            return stats;
        } catch (error) {
            console.error('[MetricsPersistence] Failed to save snapshot:', error.message);
            // Clean up temp file if it exists
            try {
                await fs.unlink(this.tempFile);
            } catch {}
            throw error;
        }
    }

    async loadSnapshot(metricsHistory) {
        try {
            console.log('[MetricsPersistence] Loading snapshot...');
            const startTime = Date.now();

            // Check if snapshot file exists
            await fs.access(this.snapshotFile);

            // Read and decompress
            const compressed = await fs.readFile(this.snapshotFile);
            const jsonData = await gunzip(compressed);
            const snapshot = JSON.parse(jsonData.toString());

            // Handle version compatibility
            if (snapshot.version !== 1 && snapshot.version !== 2) {
                throw new Error(`Unsupported snapshot version: ${snapshot.version}`);
            }
            
            if (snapshot.version >= 2 && snapshot.timezone) {
                const currentOffset = new Date().getTimezoneOffset();
                const snapshotOffset = snapshot.timezoneOffset || 0;
                const offsetDiff = Math.abs(currentOffset - snapshotOffset);
                
                if (offsetDiff > 0) {
                    console.log(`[MetricsPersistence] Timezone change detected: ${snapshot.timezone} (${snapshotOffset}min) → current (${currentOffset}min)`);
                    // Note: Timestamps are already in UTC (milliseconds), so they remain accurate
                    // This is just for logging awareness
                }
            }

            // Check age of snapshot
            const now = Date.now();
            const ageMs = now - snapshot.timestamp;
            const ageHours = ageMs / (1000 * 60 * 60);
            
            // Log snapshot details for debugging
            console.log(`[MetricsPersistence] Snapshot timestamp: ${new Date(snapshot.timestamp).toISOString()}`);
            console.log(`[MetricsPersistence] Current time: ${new Date(now).toISOString()}`);
            console.log(`[MetricsPersistence] Age: ${ageMs}ms (${ageHours.toFixed(2)} hours)`);
            
            if (ageHours > this.maxRetentionHours * 2) {
                console.log(`[MetricsPersistence] Snapshot too old (${ageHours.toFixed(1)} hours), discarding`);
                return null;
            }

            // Import data into metricsHistory
            let importedGuests = 0;
            let importedNodes = 0;
            let totalPoints = 0;

            // Import guest metrics
            for (const [guestId, dataPoints] of Object.entries(snapshot.guests || {})) {
                if (dataPoints && dataPoints.length > 0) {
                    metricsHistory.importGuestMetrics(guestId, dataPoints);
                    importedGuests++;
                    totalPoints += dataPoints.length;
                }
            }

            // Import node metrics
            for (const [nodeId, dataPoints] of Object.entries(snapshot.nodes || {})) {
                if (dataPoints && dataPoints.length > 0) {
                    metricsHistory.importNodeMetrics(nodeId, dataPoints);
                    importedNodes++;
                    totalPoints += dataPoints.length;
                }
            }

            const elapsedMs = Date.now() - startTime;
            console.log(`[MetricsPersistence] Snapshot loaded: ${importedGuests} guests, ${importedNodes} nodes, ${totalPoints} points in ${elapsedMs}ms`);

            return {
                guests: importedGuests,
                nodes: importedNodes,
                totalPoints,
                ageHours,
                elapsedMs
            };
        } catch (error) {
            if (error.code === 'ENOENT') {
                console.log('[MetricsPersistence] No snapshot file found');
                return null;
            }
            console.error('[MetricsPersistence] Failed to load snapshot:', error.message);
            throw error;
        }
    }

    downsampleMetrics(dataPoints) {
        if (!dataPoints || dataPoints.length === 0) return [];

        const now = Date.now();
        const hourAgo = now - (60 * 60 * 1000);
        const dayAgo = now - (24 * 60 * 60 * 1000);
        
        // Sort by timestamp
        const sorted = [...dataPoints].sort((a, b) => a.timestamp - b.timestamp);
        const downsampled = [];
        
        // Group data into time ranges with different sampling rates
        let lastHourTimestamp = 0;
        let lastDayTimestamp = 0;
        let lastWeekTimestamp = 0;
        let lastStoredPoint = null;
        
        for (const point of sorted) {
            if (!point || !point.timestamp) continue;
            
            const age = now - point.timestamp;
            
            // Check if this point represents a significant change from the last stored point
            const hasSignificantChange = this.hasSignificantChange(point, lastStoredPoint);
            
            // Gradual resolution decay based on age
            // UPDATED: Preserve more data points, especially for recent data
            let interval;
            
            // Define resolution stages with higher density for better chart resolution
            const resolutionStages = [
                { maxAge: 5 * 60 * 1000, interval: 2 * 1000 },         // 0-5 min: 2s (150 points)
                { maxAge: 15 * 60 * 1000, interval: 4 * 1000 },        // 5-15 min: 4s (150 points)
                { maxAge: 30 * 60 * 1000, interval: 6 * 1000 },        // 15-30 min: 6s (150 points)
                { maxAge: 60 * 60 * 1000, interval: 10 * 1000 },       // 30-60 min: 10s (180 points)
                { maxAge: 2 * 60 * 60 * 1000, interval: 20 * 1000 },   // 1-2h: 20s (180 points)
                { maxAge: 4 * 60 * 60 * 1000, interval: 30 * 1000 },   // 2-4h: 30s (240 points)
                { maxAge: 8 * 60 * 60 * 1000, interval: 60 * 1000 },   // 4-8h: 1m (240 points)
                { maxAge: 12 * 60 * 60 * 1000, interval: 90 * 1000 },  // 8-12h: 1.5m (160 points)
                { maxAge: 24 * 60 * 60 * 1000, interval: 3 * 60 * 1000 },   // 12-24h: 3m (240 points)
                { maxAge: 2 * 24 * 60 * 60 * 1000, interval: 5 * 60 * 1000 },   // 1-2d: 5m (288 points)
                { maxAge: 3 * 24 * 60 * 60 * 1000, interval: 10 * 60 * 1000 },  // 2-3d: 10m (144 points)
                { maxAge: 7 * 24 * 60 * 60 * 1000, interval: 30 * 60 * 1000 }   // 3-7d: 30m (224 points)
            ];
            
            // Find appropriate interval based on age
            interval = resolutionStages[resolutionStages.length - 1].interval; // default to largest
            for (const stage of resolutionStages) {
                if (age <= stage.maxAge) {
                    interval = stage.interval;
                    break;
                }
            }
            
            // Store point if:
            // 1. It's the first point
            // 2. Enough time has passed based on the interval
            const timeSinceLastPoint = lastStoredPoint ? point.timestamp - lastStoredPoint.timestamp : Infinity;
            
            // Only check for significant changes at interval boundaries to optimize performance
            let shouldStore = false;
            
            if (!lastStoredPoint) {
                shouldStore = true; // First point
            } else if (timeSinceLastPoint >= interval * 2) {
                shouldStore = true; // Prevent large gaps
            } else if (timeSinceLastPoint >= interval) {
                // At interval boundary - check if we should store
                if (interval === 0) {
                    // No downsampling for very recent data
                    shouldStore = true;
                } else {
                    // Check for significant change only at downsample intervals
                    shouldStore = hasSignificantChange || timeSinceLastPoint >= interval * 1.5;
                }
            }
            
            if (shouldStore) {
                downsampled.push(point);
                lastStoredPoint = point;
            }
        }

        console.log(`[MetricsPersistence] Downsampled ${dataPoints.length} points to ${downsampled.length} points`);
        return downsampled;
    }

    hasSignificantChange(point, lastPoint) {
        if (!lastPoint) return true;
        
        const percentageMetrics = ['cpu', 'mem', 'disk'];
        for (const metric of percentageMetrics) {
            if (point[metric] !== undefined && lastPoint[metric] !== undefined) {
                const current = point[metric];
                const previous = lastPoint[metric];
                
                // Special handling for near-zero values
                if (previous < 1 && current < 1) continue; // Both near zero, skip
                
                // For CPU: detect relative changes
                if (metric === 'cpu') {
                    // Always capture transitions to/from idle
                    if ((previous < 5 && current >= 5) || (previous >= 5 && current < 5)) return true;
                    
                    if (current > 5) {
                        const relativeChange = Math.abs(current - previous) / current;
                        if (relativeChange > 0.2) return true;
                    }
                } else {
                    // Memory/Disk: use absolute thresholds for low values, relative for high
                    if (current < 20) {
                        // Below 20%, use 2% absolute threshold
                        if (Math.abs(current - previous) > 2) return true;
                    } else {
                        // Above 20%, use 10% relative threshold
                        const relativeChange = Math.abs(current - previous) / current;
                        if (relativeChange > 0.1) return true;
                    }
                }
            }
        }
        
        const rateMetrics = ['diskReadRate', 'diskWriteRate', 'netInRate', 'netOutRate'];
        for (const metric of rateMetrics) {
            if (point[metric] !== undefined && lastPoint[metric] !== undefined) {
                const current = point[metric] || 0;
                const previous = lastPoint[metric] || 0;
                
                const wasIdle = previous < 1024;
                const isIdle = current < 1024;
                
                if (wasIdle !== isIdle) return true; // State change
                
                if (!isIdle && previous > 0) {
                    const relativeChange = Math.abs(current - previous) / previous;
                    if (relativeChange > 0.3) return true;
                }
                
                if (Math.abs(current - previous) > 10 * 1024 * 1024) return true;
            }
        }
        
        return false;
    }

    countDataPoints(snapshot) {
        let count = 0;
        for (const metrics of Object.values(snapshot.guests || {})) {
            count += metrics.length;
        }
        for (const metrics of Object.values(snapshot.nodes || {})) {
            count += metrics.length;
        }
        return count;
    }

    async cleanup() {
        try {
            // Remove old snapshot files if they exist
            const files = await fs.readdir(this.dataPath);
            for (const file of files) {
                if (file.startsWith('metrics-') && file.endsWith('.json.gz') && file !== 'metrics-snapshot.json.gz') {
                    await fs.unlink(path.join(this.dataPath, file));
                    console.log(`[MetricsPersistence] Cleaned up old snapshot: ${file}`);
                }
            }
        } catch (error) {
            console.error('[MetricsPersistence] Cleanup error:', error.message);
        }
    }
}

module.exports = MetricsPersistence;