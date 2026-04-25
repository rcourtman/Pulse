import { Show } from 'solid-js';
import { Card } from '@/components/shared/Card';
import {
  ALERTS_OVERVIEW_PATH,
  INFRASTRUCTURE_PATH,
  WORKLOADS_PATH,
  buildStoragePath,
} from '@/routing/resourceLinks';
import { getDashboardAlertTone } from '@/utils/alertOverviewPresentation';
import { getDashboardKpiPresentation } from '@/utils/dashboardKpiPresentation';
import { getAlertSeverityTextClass } from '@/utils/alertSeverityPresentation';
import { formatBytes } from '@/utils/format';

export type KPIStripCardId = 'infrastructure' | 'workloads' | 'storage' | 'alerts';

interface KPIStripProps {
  infrastructure: {
    total: number;
    online: number;
    attention?: number;
  };
  workloads: {
    total: number;
    running: number;
    stopped?: number;
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
  // Cards to hide. Use when another panel on the same page already carries the
  // same datum (e.g. Pulse Brief + Estate cover Infrastructure + Alerts counts).
  exclude?: KPIStripCardId[];
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

  const isVisible = (id: KPIStripCardId) => !props.exclude?.includes(id);
  const visibleCount = () =>
    (['infrastructure', 'workloads', 'storage', 'alerts'] as const).filter(isVisible).length;
  const gridCols = () => {
    const count = visibleCount();
    if (count <= 1) return 'grid-cols-1';
    if (count === 2) return 'grid-cols-2';
    if (count === 3) return 'grid-cols-1 sm:grid-cols-3';
    return 'grid-cols-2 lg:grid-cols-4';
  };

  return (
    <div class={`grid gap-3 ${gridCols()}`}>
      <Show when={isVisible('infrastructure')}>
        <a
          href={INFRASTRUCTURE_PATH}
          class="group block"
          data-testid="dashboard-kpi-infrastructure"
        >
          <Card hoverable padding="none" class={infrastructurePresentation.cardClassName}>
            <div class="px-3.5 py-3">
              <div class="flex items-center justify-between">
                <p class="text-[11px] font-medium text-muted uppercase tracking-wide">
                  {infrastructurePresentation.label}
                </p>
                <InfrastructureIcon
                  class={infrastructurePresentation.iconClassName}
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
                {props.infrastructure.attention !== undefined &&
                props.infrastructure.attention > 0 ? (
                  <>
                    {' · '}
                    <span class="font-mono font-medium text-amber-600 dark:text-amber-400">
                      {props.infrastructure.attention}
                    </span>{' '}
                    attention
                  </>
                ) : null}
              </p>
            </div>
          </Card>
        </a>
      </Show>

      <Show when={isVisible('workloads')}>
        <a href={WORKLOADS_PATH} class="group block" data-testid="dashboard-kpi-workloads">
          <Card hoverable padding="none" class={workloadsPresentation.cardClassName}>
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
              <p class="text-[11px] text-muted mt-0.5">{workloadsPresentation.supportingText}</p>
              <p class="text-xs text-muted mt-0.5">
                <span class="font-mono font-medium text-emerald-600 dark:text-emerald-400">
                  {props.workloads.running}
                </span>{' '}
                running
                {props.workloads.stopped !== undefined && props.workloads.stopped > 0 ? (
                  <>
                    {' · '}
                    <span class="font-mono font-medium text-base-content">
                      {props.workloads.stopped}
                    </span>{' '}
                    stopped
                  </>
                ) : null}
              </p>
            </div>
          </Card>
        </a>
      </Show>

      <Show when={isVisible('storage')}>
        <a href={buildStoragePath()} class="group block" data-testid="dashboard-kpi-storage">
          <Card hoverable padding="none" class={storagePresentation.cardClassName}>
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
      </Show>

      <Show when={isVisible('alerts')}>
        <a href={ALERTS_OVERVIEW_PATH} class="group block" data-testid="dashboard-kpi-alerts">
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
      </Show>
    </div>
  );
}

export default KPIStrip;
