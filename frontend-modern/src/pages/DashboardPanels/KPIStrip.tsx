import { Card } from '@/components/shared/Card';
import {
  ALERTS_OVERVIEW_PATH,
  INFRASTRUCTURE_PATH,
  WORKLOADS_PATH,
  buildStoragePath,
} from '@/routing/resourceLinks';
import { formatBytes } from '@/utils/format';
import ServerIcon from 'lucide-solid/icons/server';
import ContainerIcon from 'lucide-solid/icons/box';
import HardDriveIcon from 'lucide-solid/icons/hard-drive';
import BellIcon from 'lucide-solid/icons/bell';

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
      <a href={INFRASTRUCTURE_PATH} class="group block">
        <Card
          hoverable
          padding="none"
          class="h-full border-l-[3px] border-l-blue-500 dark:border-l-blue-400 bg-surface group-hover:bg-surface-hover transition-colors"
        >
          <div class="px-3.5 py-3">
            <div class="flex items-center justify-between">
              <p class="text-[11px] font-medium text-muted uppercase tracking-wide">
                Infrastructure
              </p>
              <ServerIcon
                class="w-3.5 h-3.5 text-blue-500/50 dark:text-blue-400/50"
                aria-hidden="true"
              />
            </div>
            <p class="text-2xl font-mono font-semibold text-base-content mt-1">
              {props.infrastructure.total}
            </p>
            <p class="text-xs text-muted mt-0.5">
              <span class="font-mono font-medium text-emerald-600 dark:text-emerald-400">
                {props.infrastructure.online}
              </span>{' '}
              online
            </p>
          </div>
        </Card>
      </a>

      <a href={WORKLOADS_PATH} class="group block">
        <Card
          hoverable
          padding="none"
          class="h-full border-l-[3px] border-l-violet-500 dark:border-l-violet-400 bg-surface group-hover:bg-surface-hover transition-colors"
        >
          <div class="px-3.5 py-3">
            <div class="flex items-center justify-between">
              <p class="text-[11px] font-medium text-muted uppercase tracking-wide">Workloads</p>
              <ContainerIcon
                class="w-3.5 h-3.5 text-violet-500/50 dark:text-violet-400/50"
                aria-hidden="true"
              />
            </div>
            <p class="text-2xl font-mono font-semibold text-base-content mt-1">
              {props.workloads.total}
            </p>
            <p class="text-xs text-muted mt-0.5">
              <span class="font-mono font-medium text-emerald-600 dark:text-emerald-400">
                {props.workloads.running}
              </span>{' '}
              running
            </p>
          </div>
        </Card>
      </a>

      <a href={buildStoragePath()} class="group block">
        <Card
          hoverable
          padding="none"
          class="h-full border-l-[3px] border-l-cyan-500 dark:border-l-cyan-400 bg-surface group-hover:bg-surface-hover transition-colors"
        >
          <div class="px-3.5 py-3">
            <div class="flex items-center justify-between">
              <p class="text-[11px] font-medium text-muted uppercase tracking-wide">Storage</p>
              <HardDriveIcon
                class="w-3.5 h-3.5 text-cyan-500/50 dark:text-cyan-400/50"
                aria-hidden="true"
              />
            </div>
            <p class="text-2xl font-mono font-semibold text-base-content mt-1">
              {Math.round(props.storage.capacityPercent)}%
            </p>
            <p class="text-xs text-muted mt-0.5">
              {formatBytes(props.storage.totalUsed)} / {formatBytes(props.storage.totalCapacity)}
            </p>
          </div>
        </Card>
      </a>

      <a href={ALERTS_OVERVIEW_PATH} class="group block">
        <Card
          hoverable
          tone={alertsTone()}
          padding="none"
          class="h-full border-l-[3px] border-l-amber-500 dark:border-l-amber-400 group-hover:brightness-95 dark:group-hover:brightness-110 transition-all"
        >
          <div class="px-3.5 py-3">
            <div class="flex items-center justify-between">
              <p class="text-[11px] font-medium text-muted uppercase tracking-wide">Alerts</p>
              <BellIcon
                class="w-3.5 h-3.5 text-amber-500/50 dark:text-amber-400/50"
                aria-hidden="true"
              />
            </div>
            <p class="text-2xl font-mono font-semibold text-base-content mt-1">
              {props.alerts.total}
            </p>
            <p class="text-xs text-muted mt-0.5">
              <span class="font-mono font-medium text-red-600 dark:text-red-400">
                {props.alerts.activeCritical}
              </span>{' '}
              critical ·{' '}
              <span class="font-mono font-medium text-amber-600 dark:text-amber-400">
                {props.alerts.activeWarning}
              </span>{' '}
              warning
            </p>
          </div>
        </Card>
      </a>
    </div>
  );
}

export default KPIStrip;
