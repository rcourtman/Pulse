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
    <div class="grid grid-cols-2 lg:grid-cols-4 gap-2.5">
      <a href={INFRASTRUCTURE_PATH} class="group block">
        <Card hoverable border={false} padding="none" class="h-full px-3.5 py-2.5 bg-slate-50 dark:bg-slate-800 group-hover:bg-slate-100 dark:group-hover:bg-slate-700/50 transition-colors">
          <p class="text-[11px] font-medium text-slate-500 dark:text-slate-400 uppercase tracking-wide">Infrastructure</p>
          <p class="text-2xl font-mono font-semibold text-slate-900 dark:text-slate-100 mt-0.5">
            {props.infrastructure.total}
          </p>
          <p class="text-xs text-slate-500 dark:text-slate-400 mt-0.5">
            <span class="font-mono font-medium text-slate-700 dark:text-slate-300">{props.infrastructure.online}</span> online
          </p>
        </Card>
      </a>

      <a href={WORKLOADS_PATH} class="group block">
        <Card hoverable border={false} padding="none" class="h-full px-3.5 py-2.5 bg-slate-50 dark:bg-slate-800 group-hover:bg-slate-100 dark:group-hover:bg-slate-700/50 transition-colors">
          <p class="text-[11px] font-medium text-slate-500 dark:text-slate-400 uppercase tracking-wide">Workloads</p>
          <p class="text-2xl font-mono font-semibold text-slate-900 dark:text-slate-100 mt-0.5">
            {props.workloads.total}
          </p>
          <p class="text-xs text-slate-500 dark:text-slate-400 mt-0.5">
            <span class="font-mono font-medium text-slate-700 dark:text-slate-300">{props.workloads.running}</span> running
          </p>
        </Card>
      </a>

      <a href={buildStoragePath()} class="group block">
        <Card hoverable border={false} padding="none" class="h-full px-3.5 py-2.5 bg-slate-50 dark:bg-slate-800 group-hover:bg-slate-100 dark:group-hover:bg-slate-700/50 transition-colors">
          <p class="text-[11px] font-medium text-slate-500 dark:text-slate-400 uppercase tracking-wide">Storage</p>
          <p class="text-2xl font-mono font-semibold text-slate-900 dark:text-slate-100 mt-0.5">
            {Math.round(props.storage.capacityPercent)}%
          </p>
          <p class="text-xs text-slate-500 dark:text-slate-400 mt-0.5">
            {formatBytes(props.storage.totalUsed)} / {formatBytes(props.storage.totalCapacity)}
          </p>
        </Card>
      </a>

      <a href={ALERTS_OVERVIEW_PATH} class="group block">
        <Card hoverable tone={alertsTone()} border={false} padding="none" class="h-full px-3.5 py-2.5 group-hover:brightness-95 dark:group-hover:brightness-110 transition-all">
          <p class="text-[11px] font-medium text-slate-500 dark:text-slate-400 uppercase tracking-wide">Alerts</p>
          <p class="text-2xl font-mono font-semibold text-slate-900 dark:text-slate-100 mt-0.5">{props.alerts.total}</p>
          <p class="text-xs text-slate-500 dark:text-slate-400 mt-0.5">
            <span class="font-mono font-medium text-red-600 dark:text-red-400">{props.alerts.activeCritical}</span> critical · <span class="font-mono font-medium text-amber-600 dark:text-amber-400">{props.alerts.activeWarning}</span> warning
          </p>
        </Card>
      </a>
    </div>
  );
}

export default KPIStrip;
