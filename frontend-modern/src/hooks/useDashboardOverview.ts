import { createMemo, type Accessor } from 'solid-js';
import type { Alert } from '@/types/api';
import type { Resource, ResourceMetric, ResourceStatus } from '@/types/resource';
import { isInfrastructure, isStorage, isWorkload } from '@/types/resource';
import { METRIC_THRESHOLDS } from '@/utils/metricThresholds';

export interface DashboardOverview {
  health: {
    totalResources: number;
    byStatus: Record<string, number>;
    criticalAlerts: number;
    warningAlerts: number;
  };
  infrastructure: {
    total: number;
    byStatus: Record<string, number>;
    byType: Record<string, number>;
    topCPU: Array<{ id: string; name: string; percent: number }>;
    topMemory: Array<{ id: string; name: string; percent: number }>;
  };
  workloads: {
    total: number;
    running: number;
    stopped: number;
    byType: Record<string, number>;
  };
  storage: {
    total: number;
    totalCapacity: number;
    totalUsed: number;
    warningCount: number;
    criticalCount: number;
  };
  alerts: {
    activeCritical: number;
    activeWarning: number;
    total: number;
  };
}

const RESOURCE_STATUSES: ResourceStatus[] = [
  'online',
  'offline',
  'running',
  'stopped',
  'degraded',
  'paused',
  'unknown',
];

function createStatusCounter(): Record<string, number> {
  return RESOURCE_STATUSES.reduce<Record<string, number>>((acc, status) => {
    acc[status] = 0;
    return acc;
  }, {});
}

function incrementCount(counter: Record<string, number>, key: string): void {
  counter[key] = (counter[key] ?? 0) + 1;
}

function getMetricPercent(metric?: ResourceMetric): number | null {
  if (!metric) return null;

  if (
    typeof metric.total === 'number' &&
    typeof metric.used === 'number' &&
    Number.isFinite(metric.total) &&
    Number.isFinite(metric.used) &&
    metric.total > 0
  ) {
    return (metric.used / metric.total) * 100;
  }

  if (typeof metric.current === 'number' && Number.isFinite(metric.current)) {
    return metric.current;
  }

  return null;
}

function buildTopInfrastructureMetrics(
  resources: Resource[],
  metric: 'cpu' | 'memory',
): Array<{ id: string; name: string; percent: number }> {
  const rows = resources
    .map((resource) => {
      const percent =
        metric === 'cpu' ? getMetricPercent(resource.cpu) : getMetricPercent(resource.memory);
      if (percent === null) return null;

      return {
        id: resource.id,
        name: resource.displayName || resource.name,
        percent,
      };
    })
    .filter((row): row is { id: string; name: string; percent: number } => row !== null);

  rows.sort((a, b) => b.percent - a.percent);
  return rows.slice(0, 5);
}

export function computeDashboardOverview(
  resources: Resource[],
  activeAlerts: Alert[],
): DashboardOverview {
  const healthByStatus = createStatusCounter();
  const infrastructureByStatus = createStatusCounter();
  const infrastructureByType: Record<string, number> = {};
  const workloadsByType: Record<string, number> = {};

  const infrastructureResources: Resource[] = [];

  let workloadsRunning = 0;
  let workloadsStopped = 0;
  let storageTotal = 0;
  let storageTotalCapacity = 0;
  let storageTotalUsed = 0;
  let storageWarningCount = 0;
  let storageCriticalCount = 0;

  resources.forEach((resource) => {
    incrementCount(healthByStatus, resource.status);

    if (isInfrastructure(resource)) {
      infrastructureResources.push(resource);
      incrementCount(infrastructureByStatus, resource.status);
      incrementCount(infrastructureByType, resource.type);
    }

    if (isWorkload(resource)) {
      incrementCount(workloadsByType, resource.type);
      if (resource.status === 'online' || resource.status === 'running') {
        workloadsRunning += 1;
      }
      if (resource.status === 'offline' || resource.status === 'stopped') {
        workloadsStopped += 1;
      }
    }

    if (isStorage(resource)) {
      storageTotal += 1;

      if (typeof resource.disk?.total === 'number' && Number.isFinite(resource.disk.total)) {
        storageTotalCapacity += resource.disk.total;
      }
      if (typeof resource.disk?.used === 'number' && Number.isFinite(resource.disk.used)) {
        storageTotalUsed += resource.disk.used;
      }

      const diskPercent = getMetricPercent(resource.disk);
      if (diskPercent !== null) {
        if (diskPercent > METRIC_THRESHOLDS.disk.critical) {
          storageCriticalCount += 1;
        } else if (diskPercent > METRIC_THRESHOLDS.disk.warning) {
          storageWarningCount += 1;
        }
      }
    }
  });

  const alertCounts = activeAlerts.reduce(
    (acc, alert) => {
      if (alert.level === 'critical') acc.critical += 1;
      if (alert.level === 'warning') acc.warning += 1;
      return acc;
    },
    { critical: 0, warning: 0 },
  );

  return {
    health: {
      totalResources: resources.length,
      byStatus: healthByStatus,
      criticalAlerts: alertCounts.critical,
      warningAlerts: alertCounts.warning,
    },
    infrastructure: {
      total: infrastructureResources.length,
      byStatus: infrastructureByStatus,
      byType: infrastructureByType,
      topCPU: buildTopInfrastructureMetrics(infrastructureResources, 'cpu'),
      topMemory: buildTopInfrastructureMetrics(infrastructureResources, 'memory'),
    },
    workloads: {
      total: Object.values(workloadsByType).reduce((sum, count) => sum + count, 0),
      running: workloadsRunning,
      stopped: workloadsStopped,
      byType: workloadsByType,
    },
    storage: {
      total: storageTotal,
      totalCapacity: storageTotalCapacity,
      totalUsed: storageTotalUsed,
      warningCount: storageWarningCount,
      criticalCount: storageCriticalCount,
    },
    alerts: {
      activeCritical: alertCounts.critical,
      activeWarning: alertCounts.warning,
      total: activeAlerts.length,
    },
  };
}

export function useDashboardOverview(
  resources: Accessor<Resource[]>,
  alerts: Accessor<Alert[]>,
): Accessor<DashboardOverview> {
  return createMemo(() => computeDashboardOverview(resources(), alerts()));
}
