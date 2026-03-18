const DASHBOARD_TREND_COLORS = [
  '#3b82f6',
  '#8b5cf6',
  '#10b981',
  '#f97316',
  '#ec4899',
  '#06b6d4',
  '#f59e0b',
  '#ef4444',
];

export function getDashboardTrendColor(index: number): string {
  return DASHBOARD_TREND_COLORS[index % DASHBOARD_TREND_COLORS.length];
}

export function getDashboardTrendErrorState() {
  return {
    text: 'Unable to load trends',
  } as const;
}
