export type DashboardAlertTone = 'default' | 'warning' | 'danger';

export interface DashboardAlertOverview {
  activeCritical: number;
  activeWarning: number;
}

export function getDashboardAlertTone(overview: DashboardAlertOverview): DashboardAlertTone {
  if (overview.activeCritical > 0) return 'danger';
  if (overview.activeWarning > 0) return 'warning';
  return 'default';
}
