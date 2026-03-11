export type PatrolSummaryTone = 'critical' | 'warning' | 'success';

export interface PatrolSummaryPresentation {
  iconClass: string;
  iconContainerClass: string;
  valueClass: string;
}

export const PATROL_NO_ISSUES_LABEL = 'No issues found';

const QUIET_ICON_CONTAINER = 'bg-surface border-border';
const QUIET_ICON = 'text-muted';
const QUIET_VALUE = 'text-muted';

const ACTIVE_PRESENTATION: Record<PatrolSummaryTone, PatrolSummaryPresentation> = {
  critical: {
    iconClass: 'text-red-500 dark:text-red-400',
    iconContainerClass: 'bg-red-50 dark:bg-red-900 border-red-200 dark:border-red-800',
    valueClass: 'text-red-600 dark:text-red-400',
  },
  success: {
    iconClass: 'text-green-500 dark:text-green-400',
    iconContainerClass: 'bg-green-50 dark:bg-green-900 border-green-200 dark:border-green-800',
    valueClass: 'text-green-600 dark:text-green-400',
  },
  warning: {
    iconClass: 'text-amber-500 dark:text-amber-400',
    iconContainerClass: 'bg-amber-50 dark:bg-amber-900 border-amber-200 dark:border-amber-800',
    valueClass: 'text-amber-600 dark:text-amber-400',
  },
};

export function getPatrolSummaryPresentation(
  tone: PatrolSummaryTone,
  active: boolean,
): PatrolSummaryPresentation {
  if (!active) {
    return {
      iconClass: QUIET_ICON,
      iconContainerClass: QUIET_ICON_CONTAINER,
      valueClass: QUIET_VALUE,
    };
  }
  return ACTIVE_PRESENTATION[tone];
}
