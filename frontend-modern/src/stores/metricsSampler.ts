/**
 * Metrics Sampler
 *
 * Dedicated 30-second ticker that samples metrics from WebSocket state
 * and records them to the metrics history store.
 *
 * This decouples metric sampling from the WebSocket message handler,
 * reducing CPU waste and ensuring consistent sampling regardless of
 * WebSocket message frequency.
 */

import { getGlobalWebSocketStore } from './websocket-global';
import { recordMetric } from './metricsHistory';
import { getMetricsViewMode } from './metricsViewMode';
import { buildMetricKey } from '@/utils/metricsKeys';
import { logger } from '@/utils/logger';

const SAMPLE_INTERVAL_MS = 30 * 1000; // 30 seconds

let isRunning = false;

/**
 * Sample metrics from current WebSocket state
 */
function sampleMetrics(): void {
  const wsStore = getGlobalWebSocketStore();
  const state = wsStore.state;

  let sampledCount = 0;

  // Sample Proxmox/PBS nodes
  if (state.nodes) {
    for (const node of state.nodes) {
      if (!node.id) continue;

      const cpu = (node.cpu || 0) * 100;
      const memory = node.memory?.usage || 0;
      const disk = node.disk && node.disk.total > 0
        ? (node.disk.used / node.disk.total) * 100
        : 0;

      const key = buildMetricKey('node', node.id);
      recordMetric(key, cpu, memory, disk);
      sampledCount++;
    }
  }

  // Sample VMs
  if (state.vms) {
    for (const vm of state.vms) {
      if (!vm.id) continue;

      const cpu = (vm.cpu || 0) * 100;
      const memory = vm.memory?.usage || 0;
      const disk = vm.disk && vm.disk.total > 0
        ? (vm.disk.used / vm.disk.total) * 100
        : 0;

      const key = buildMetricKey('vm', vm.id);
      recordMetric(key, cpu, memory, disk);
      sampledCount++;
    }
  }

  // Sample LXC containers
  if (state.containers) {
    for (const container of state.containers) {
      if (!container.id) continue;

      const cpu = (container.cpu || 0) * 100;
      const memory = container.memory?.usage || 0;
      const disk = container.disk && container.disk.total > 0
        ? (container.disk.used / container.disk.total) * 100
        : 0;

      const key = buildMetricKey('container', container.id);
      recordMetric(key, cpu, memory, disk);
      sampledCount++;
    }
  }

  // Sample Docker hosts
  if (state.dockerHosts) {
    for (const host of state.dockerHosts) {
      if (!host.id) continue;

      const cpu = host.cpuUsagePercent || 0;
      const memory = host.memory?.usage || 0;
      const disk = host.disks && host.disks.length > 0
        ? host.disks.reduce((acc, d) => {
            if (d.total > 0) {
              acc.used += d.used || 0;
              acc.total += d.total || 0;
            }
            return acc;
          }, { used: 0, total: 0 })
        : { used: 0, total: 0 };
      const diskPercent = disk.total > 0 ? (disk.used / disk.total) * 100 : 0;

      const hostKey = buildMetricKey('dockerHost', host.id);
      recordMetric(hostKey, cpu, memory, diskPercent);
      sampledCount++;

      // Sample Docker containers within this host
      if (host.containers) {
        for (const container of host.containers) {
          if (!container.id) continue;

          const contCpu = container.cpuPercent || 0;
          const contMem = container.memoryPercent || 0;
          const usableTotal = container.rootFilesystemBytes ?? 0;
          const writable = container.writableLayerBytes ?? 0;
          const contDisk = usableTotal > 0 ? (writable / usableTotal) * 100 : 0;

          const containerKey = buildMetricKey('dockerContainer', container.id);
          recordMetric(containerKey, contCpu, contMem, contDisk);
          sampledCount++;
        }
      }
    }
  }

  if (import.meta.env.DEV) {
    logger.debug('[MetricsSampler] Sampled metrics', { count: sampledCount });
  }
}

/**
 * Start the metrics sampler
 * Only runs when view mode is 'sparklines'
 */
export function startMetricsSampler(): void {
  if (isRunning) {
    logger.warn('[MetricsSampler] Already running');
    return;
  }

  isRunning = true;

  // Create interval that checks view mode before sampling
  window.setInterval(() => {
    const viewMode = getMetricsViewMode();

    if (viewMode === 'sparklines') {
      sampleMetrics();
    }
    // If in 'bars' mode, skip sampling to save CPU
  }, SAMPLE_INTERVAL_MS);

  logger.info('[MetricsSampler] Started', { intervalMs: SAMPLE_INTERVAL_MS });

  // Take initial sample immediately
  if (getMetricsViewMode() === 'sparklines') {
    sampleMetrics();
  }
}

