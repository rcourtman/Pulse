import { Card } from '@/components/shared/Card';
import {
  ALERTS_OVERVIEW_PATH,
  INFRASTRUCTURE_PATH,
  WORKLOADS_PATH,
  buildStoragePath,
} from '@/routing/resourceLinks';
import { getDashboardAlertTone } from '@/utils/dashboardAlertPresentation';
import { getDashboardKpiPresentation } from '@/utils/dashboardKpiPresentation';
import { getAlertSeverityTextClass } from '@/utils/alertSeverityPresentation';
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
  const infrastructurePresentation = getDashboardKpiPresentation('infrastructure');
  const workloadsPresentation = getDashboardKpiPresentation('workloads');
  const storagePresentation = getDashboardKpiPresentation('storage');
  const alertsPresentation = getDashboardKpiPresentation('alerts');
  const InfrastructureIcon = infrastructurePresentation.icon;
  const WorkloadsIcon = workloadsPresentation.icon;
  const StorageIcon = storagePresentation.icon;
  const AlertsIcon = alertsPresentation.icon;

  return (
    <div class="grid grid-cols-2 lg:grid-cols-4 gap-3">
      <a href={INFRASTRUCTURE_PATH} class="group block">
        <Card
          hoverable
          padding="none"
          class={infrastructurePresentation.cardClassName}
        >
          <div class="px-3.5 py-3">
            <div class="flex items-center justify-between">
              <p class="text-[11px] font-medium text-muted uppercase tracking-wide">
                {infrastructurePresentation.label}
              </p>
              <InfrastructureIcon class={infrastructurePresentation.iconClassName} aria-hidden="true" />
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
          class={workloadsPresentation.cardClassName}
        >
          <div class="px-3.5 py-3">
            <div class="flex items-center justify-between">
              <p class="text-[11px] font-medium text-muted uppercase tracking-wide">
                {workloadsPresentation.label}
              </p>
              <WorkloadsIcon class={workloadsPresentation.iconClassName} aria-hidden="true" />
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
          class={storagePresentation.cardClassName}
        >
          <div class="px-3.5 py-3">
            <div class="flex items-center justify-between">
              <p class="text-[11px] font-medium text-muted uppercase tracking-wide">
                {storagePresentation.label}
              </p>
              <StorageIcon class={storagePresentation.iconClassName} aria-hidden="true" />
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
          tone={getDashboardAlertTone(props.alerts)}
          padding="none"
          class={alertsPresentation.cardClassName}
        >
          <div class="px-3.5 py-3">
            <div class="flex items-center justify-between">
              <p class="text-[11px] font-medium text-muted uppercase tracking-wide">
                {alertsPresentation.label}
              </p>
              <AlertsIcon class={alertsPresentation.iconClassName} aria-hidden="true" />
            </div>
            <p class="text-2xl font-mono font-semibold text-base-content mt-1">
              {props.alerts.total}
            </p>
            <p class="text-xs text-muted mt-0.5">
              <span class={`font-mono font-medium ${getAlertSeverityTextClass('critical')}`}>
                {props.alerts.activeCritical}
              </span>{' '}
              critical ·{' '}
              <span class={`font-mono font-medium ${getAlertSeverityTextClass('warning')}`}>
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
