export type DashboardWidgetId = 'trends' | 'alerts';

export interface DashboardWidgetDef {
  id: DashboardWidgetId;
  label: string;
  size: 'full' | 'quarter';
  defaultVisible: boolean;
  defaultOrder: number;
}

export const DASHBOARD_WIDGETS: DashboardWidgetDef[] = [
  { id: 'alerts', label: 'Recent Alerts', size: 'full', defaultVisible: true, defaultOrder: 0 },
  { id: 'trends', label: 'Trend Charts', size: 'full', defaultVisible: true, defaultOrder: 1 },
];

export function getDashboardWidget(id: DashboardWidgetId): DashboardWidgetDef | undefined {
  return DASHBOARD_WIDGETS.find((widget) => widget.id === id);
}
