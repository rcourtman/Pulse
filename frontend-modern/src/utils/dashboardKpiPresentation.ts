import type { Component } from 'solid-js';
import ServerIcon from 'lucide-solid/icons/server';
import ContainerIcon from 'lucide-solid/icons/box';
import HardDriveIcon from 'lucide-solid/icons/hard-drive';
import BellIcon from 'lucide-solid/icons/bell';

export type DashboardKpiKey = 'infrastructure' | 'workloads' | 'storage' | 'alerts';

export interface DashboardKpiPresentation {
  label: string;
  cardClassName: string;
  iconClassName: string;
  icon: Component<{ class?: string }>;
}

const DASHBOARD_KPI_PRESENTATION: Record<DashboardKpiKey, DashboardKpiPresentation> = {
  infrastructure: {
    label: 'Infrastructure',
    cardClassName:
      'h-full border-l-[3px] border-l-blue-500 dark:border-l-blue-400 bg-surface group-hover:bg-surface-hover transition-colors',
    iconClassName: 'w-3.5 h-3.5 text-blue-500/50 dark:text-blue-400/50',
    icon: ServerIcon,
  },
  workloads: {
    label: 'Workloads',
    cardClassName:
      'h-full border-l-[3px] border-l-violet-500 dark:border-l-violet-400 bg-surface group-hover:bg-surface-hover transition-colors',
    iconClassName: 'w-3.5 h-3.5 text-violet-500/50 dark:text-violet-400/50',
    icon: ContainerIcon,
  },
  storage: {
    label: 'Storage',
    cardClassName:
      'h-full border-l-[3px] border-l-cyan-500 dark:border-l-cyan-400 bg-surface group-hover:bg-surface-hover transition-colors',
    iconClassName: 'w-3.5 h-3.5 text-cyan-500/50 dark:text-cyan-400/50',
    icon: HardDriveIcon,
  },
  alerts: {
    label: 'Alerts',
    cardClassName:
      'h-full border-l-[3px] border-l-amber-500 dark:border-l-amber-400 group-hover:brightness-95 dark:group-hover:brightness-110 transition-all',
    iconClassName: 'w-3.5 h-3.5 text-amber-500/50 dark:text-amber-400/50',
    icon: BellIcon,
  },
};

export function getDashboardKpiPresentation(key: DashboardKpiKey): DashboardKpiPresentation {
  return DASHBOARD_KPI_PRESENTATION[key];
}
