import { For, createMemo } from 'solid-js';
import { Card } from '@/components/shared/Card';
import { statusBadgeClass, type StatusTone } from './dashboardHelpers';

interface HealthStatusBarProps {
  totalResources: number;
  criticalAlerts: number;
  warningAlerts: number;
  byStatus: Record<string, number>;
}

type StatusEntry = { key: string; label: string; count: number; tone: StatusTone };

export function HealthStatusBar(props: HealthStatusBarProps) {
  const tone = createMemo(() => {
    if (props.criticalAlerts > 0) return 'danger' as const;
    if (props.warningAlerts > 0) return 'warning' as const;
    return 'success' as const;
  });

  const healthSentence = createMemo(() => {
    if (props.criticalAlerts > 0) {
      const n = props.criticalAlerts;
      return `${n} critical alert${n === 1 ? '' : 's'}`;
    }
    if (props.warningAlerts > 0) {
      const n = props.warningAlerts;
      return `${n} warning${n === 1 ? '' : 's'}`;
    }
    return 'All systems operational';
  });

  const statusEntries = createMemo<StatusEntry[]>(() => {
    const byStatus = props.byStatus || {};
    return [
      {
        key: 'online',
        label: 'Online',
        count: (byStatus.online ?? 0) + (byStatus.running ?? 0),
        tone: 'online' as const,
      },
      {
        key: 'offline',
        label: 'Offline',
        count: (byStatus.offline ?? 0) + (byStatus.stopped ?? 0),
        tone: 'offline' as const,
      },
      {
        key: 'warning',
        label: 'Warning',
        count: byStatus.degraded ?? 0,
        tone: 'warning' as const,
      },
      {
        key: 'critical',
        label: 'Critical',
        count: byStatus.critical ?? 0,
        tone: 'critical' as const,
      },
      {
        key: 'unknown',
        label: 'Unknown',
        count: (byStatus.unknown ?? 0) + (byStatus.paused ?? 0),
        tone: 'unknown' as const,
      },
    ].filter((entry) => entry.count > 0);
  });

  return (
    <Card tone={tone()} padding="lg" class="w-full">
      <div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <div>
          <p class="text-sm font-semibold text-gray-900 dark:text-gray-100">{healthSentence()}</p>
          <p class="mt-0.5 text-xs text-gray-500 dark:text-gray-400">
            <span class="font-mono">{props.totalResources}</span> resources
          </p>
        </div>

        <div class="flex flex-wrap gap-2">
          <For each={statusEntries()}>
            {(status) => (
              <span class={statusBadgeClass(status.tone)}>
                <span class="font-mono">{status.count}</span>
                <span>{status.label}</span>
              </span>
            )}
          </For>
        </div>

        <div class="flex flex-wrap gap-2">
          <span class={statusBadgeClass('critical')}>
            <span class="font-mono">{props.criticalAlerts}</span>
            <span>Critical alerts</span>
          </span>
          <span class={statusBadgeClass('warning')}>
            <span class="font-mono">{props.warningAlerts}</span>
            <span>Warning alerts</span>
          </span>
        </div>
      </div>
    </Card>
  );
}

export default HealthStatusBar;

