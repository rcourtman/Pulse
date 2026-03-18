import { ALERTS_EMPTY_STATE } from '@/utils/alertOverviewPresentation';

export type DashboardAlertTone = 'default' | 'warning' | 'danger';

export interface DashboardAlertOverview {
  activeCritical: number;
  activeWarning: number;
}

export const DASHBOARD_ALERTS_EMPTY_STATE = ALERTS_EMPTY_STATE;

export function getDashboardAlertTone(overview: DashboardAlertOverview): DashboardAlertTone {
  if (overview.activeCritical > 0) return 'danger';
  if (overview.activeWarning > 0) return 'warning';
  return 'default';
}

export function getDashboardAlertSummaryText(overview: DashboardAlertOverview): string {
  return `${overview.activeCritical} critical · ${overview.activeWarning} warning`;
}
