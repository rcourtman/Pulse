import { getGlobalWebSocketStore } from './websocket-global';
import { logger } from '@/utils/logger';
import { recordDiskMetric } from './diskMetricsHistory';

const SAMPLE_INTERVAL_MS = 2000;

interface DiskCounterState {
    timestamp: number;
    readBytes: number;
    writeBytes: number;
    readOps: number;
    writeOps: number;
    ioTimeMs: number;
}

const previousDiskCounters = new Map<string, DiskCounterState>();

function sampleMetrics(): void {
    const wsStore = getGlobalWebSocketStore();
    const state = wsStore.state;

    // Sample Unified Host Agents (Disk I/O)
    if (state.hosts) {
        const now = Date.now();
        for (const host of state.hosts) {
            if (!host.id || !host.diskIO) continue;

            for (const disk of host.diskIO) {
                if (!disk.device) continue;

                const key = `${host.id}:${disk.device}`;
                const prev = previousDiskCounters.get(key);

                const current: DiskCounterState = {
                    timestamp: now,
                    readBytes: disk.readBytes || 0,
                    writeBytes: disk.writeBytes || 0,
                    readOps: disk.readOps || 0,
                    writeOps: disk.writeOps || 0,
                    ioTimeMs: disk.ioTimeMs || 0,
                };

                if (prev) {
                    const deltaSeconds = (now - prev.timestamp) / 1000;

                    if (deltaSeconds > 0) {
                        const readBps = Math.max(0, (current.readBytes - prev.readBytes) / deltaSeconds);
                        const writeBps = Math.max(0, (current.writeBytes - prev.writeBytes) / deltaSeconds);
                        const readIops = Math.max(0, (current.readOps - prev.readOps) / deltaSeconds);
                        const writeIops = Math.max(0, (current.writeOps - prev.writeOps) / deltaSeconds);
                        const deltaIoTime = Math.max(0, current.ioTimeMs - prev.ioTimeMs);
                        const utilPercent = Math.min(100, (deltaIoTime / (deltaSeconds * 1000)) * 100);

                        // Record to Disk Metrics Store
                        recordDiskMetric(key, readBps, writeBps, readIops, writeIops, utilPercent);
                    }
                }
                previousDiskCounters.set(key, current);
            }
        }
    }
}

let collectorInterval: number | null = null;

export function startMetricsCollector() {
    if (collectorInterval !== null) return;

    logger.info('[MetricsCollector] Starting Disk I/O collection service...');

    // Initial sample
    sampleMetrics();

    collectorInterval = window.setInterval(() => {
        sampleMetrics();
    }, SAMPLE_INTERVAL_MS);
}

export function stopMetricsCollector() {
    if (collectorInterval !== null) {
        window.clearInterval(collectorInterval);
        collectorInterval = null;
        logger.info('[MetricsCollector] Stopped');
    }
}
