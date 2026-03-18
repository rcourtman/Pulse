import { getGlobalWebSocketStore } from './websocket-global';
import { logger } from '@/utils/logger';
import { recordDiskMetric } from './diskMetricsHistory';
import { eventBus } from '@/stores/events';
import { getCachedUnifiedResources } from '@/hooks/useUnifiedResources';

const SAMPLE_INTERVAL_MS = 2000;

interface DiskCounterState {
  timestamp: number;
  readBytes: number;
  writeBytes: number;
  readOps: number;
  writeOps: number;
  ioTimeMs: number;
}

interface DiskCounterSample {
  agentId: string;
  device: string;
  readBytes: number;
  writeBytes: number;
  readOps: number;
  writeOps: number;
  ioTimeMs: number;
}

const previousDiskCounters = new Map<string, DiskCounterState>();

const asRecord = (value: unknown): Record<string, unknown> | undefined =>
  value && typeof value === 'object' ? (value as Record<string, unknown>) : undefined;
const asArray = (value: unknown): unknown[] => (Array.isArray(value) ? value : []);
const asString = (value: unknown): string | undefined =>
  typeof value === 'string' && value.trim().length > 0 ? value.trim() : undefined;
const asNumber = (value: unknown): number | undefined =>
  typeof value === 'number' && Number.isFinite(value) ? value : undefined;

const getResourceDiskCounters = (resource: unknown): DiskCounterSample[] => {
  const raw = asRecord(resource);
  if (!raw) return [];

  const agent = asRecord(raw.agent);
  const discoveryTarget = asRecord(raw.discoveryTarget);
  const agentId =
    asString(agent?.agentId) ||
    asString(discoveryTarget?.agentId) ||
    asString(raw.id) ||
    asString(raw.platformId);
  if (!agentId) return [];

  const platformData = asRecord(raw.platformData);
  const platformAgent = asRecord(platformData?.agent);

  const diskIoCandidates = [
    platformData?.diskIo,
    platformData?.diskIO,
    platformAgent?.diskIo,
    platformAgent?.diskIO,
    agent?.diskIo,
    agent?.diskIO,
  ] as unknown[];
  const diskIoRaw = diskIoCandidates.find((value) => Array.isArray(value));
  const diskIo = asArray(diskIoRaw);
  if (diskIo.length === 0) return [];

  const counters: DiskCounterSample[] = [];
  for (const entry of diskIo) {
    const disk = asRecord(entry);
    const device = asString(disk?.device);
    if (!device) continue;
    counters.push({
      agentId,
      device,
      readBytes: asNumber(disk?.readBytes) ?? 0,
      writeBytes: asNumber(disk?.writeBytes) ?? 0,
      readOps: asNumber(disk?.readOps) ?? 0,
      writeOps: asNumber(disk?.writeOps) ?? 0,
      ioTimeMs: asNumber(disk?.ioTimeMs) ?? 0,
    });
  }

  return counters;
};

function sampleMetrics(): void {
  const wsStore = getGlobalWebSocketStore();
  const wsResources = wsStore.state.resources || [];
  const cachedUnifiedResources = getCachedUnifiedResources({ cacheKey: 'all-resources' });
  const sourceResources = wsResources.some(
    (resource) => getResourceDiskCounters(resource).length > 0,
  )
    ? wsResources
    : cachedUnifiedResources;
  const now = Date.now();
  const observedKeys = new Set<string>();

  for (const resource of sourceResources) {
    const diskCounters = getResourceDiskCounters(resource);
    if (diskCounters.length === 0) continue;

    for (const disk of diskCounters) {
      const key = `${disk.agentId}:${disk.device}`;
      if (observedKeys.has(key)) continue;
      observedKeys.add(key);

      const prev = previousDiskCounters.get(key);
      const current: DiskCounterState = {
        timestamp: now,
        readBytes: disk.readBytes,
        writeBytes: disk.writeBytes,
        readOps: disk.readOps,
        writeOps: disk.writeOps,
        ioTimeMs: disk.ioTimeMs,
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

          recordDiskMetric(key, readBps, writeBps, readIops, writeIops, utilPercent);
        }
      }
      previousDiskCounters.set(key, current);
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

// Clear stale disk counter state on org switch
eventBus.on('org_switched', () => {
  previousDiskCounters.clear();
});
