import { Card } from '@/components/shared/Card';
import {
  ALERTS_OVERVIEW_PATH,
  INFRASTRUCTURE_PATH,
  WORKLOADS_PATH,
  buildStoragePath,
} from '@/routing/resourceLinks';
import { formatBytes } from '@/utils/format';

interface KPIStripProps {
  infrastructure: {
    total: number;
    online: number;
  };
  workloads: {
    total: number;
    running: number;
  };
  storage: {
    capacityPercent: number;
    totalUsed: number;
    totalCapacity: number;
  };
  alerts: {
    activeCritical: number;
    activeWarning: number;
    total: number;
  };
}

export function KPIStrip(props: KPIStripProps) {
  const alertsTone = () => {
    if (props.alerts.activeCritical > 0) return 'danger' as const;
    if (props.alerts.activeWarning > 0) return 'warning' as const;
    return 'default' as const;
  };

  return (
    <div class="grid grid-cols-2 lg:grid-cols-4 gap-3">
      <Card hoverable class="h-full">
        <a href={INFRASTRUCTURE_PATH} class="block">
          <p class="text-xs font-medium text-gray-600 dark:text-gray-400">Infrastructure</p>
          <p class="mt-1 text-2xl font-mono text-gray-900 dark:text-gray-100">
            {props.infrastructure.total}
          </p>
          <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
            <span class="font-mono">{props.infrastructure.online}</span> online
          </p>
        </a>
      </Card>

      <Card hoverable class="h-full">
        <a href={WORKLOADS_PATH} class="block">
          <p class="text-xs font-medium text-gray-600 dark:text-gray-400">Workloads</p>
          <p class="mt-1 text-2xl font-mono text-gray-900 dark:text-gray-100">
            {props.workloads.total}
          </p>
          <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
            <span class="font-mono">{props.workloads.running}</span> running /{' '}
            <span class="font-mono">{props.workloads.total}</span> total
          </p>
        </a>
      </Card>

      <Card hoverable class="h-full">
        <a href={buildStoragePath()} class="block">
          <p class="text-xs font-medium text-gray-600 dark:text-gray-400">Storage</p>
          <p class="mt-1 text-2xl font-mono text-gray-900 dark:text-gray-100">
            {Math.round(props.storage.capacityPercent)}%
          </p>
          <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
            {formatBytes(props.storage.totalUsed)} / {formatBytes(props.storage.totalCapacity)}
          </p>
        </a>
      </Card>

      <Card hoverable tone={alertsTone()} class="h-full">
        <a href={ALERTS_OVERVIEW_PATH} class="block">
          <p class="text-xs font-medium text-gray-600 dark:text-gray-400">Alerts</p>
          <p class="mt-1 text-2xl font-mono text-gray-900 dark:text-gray-100">{props.alerts.total}</p>
          <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
            <span class="font-mono">{props.alerts.activeCritical}</span> critical ·{' '}
            <span class="font-mono">{props.alerts.activeWarning}</span> warning
          </p>
        </a>
      </Card>
    </div>
  );
}

export default KPIStrip;

