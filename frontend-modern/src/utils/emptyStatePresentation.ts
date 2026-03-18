export type EmptyStateTone = 'default' | 'info' | 'success' | 'warning' | 'danger';

export interface EmptyStatePresentation {
  iconClass: string;
  titleClass: string;
  descriptionClass: string;
}

const EMPTY_STATE_PRESENTATION: Record<EmptyStateTone, EmptyStatePresentation> = {
  default: {
    iconClass: 'bg-surface-alt text-muted',
    titleClass: 'text-base-content',
    descriptionClass: 'text-muted',
  },
  info: {
    iconClass: 'bg-blue-50 dark:bg-blue-900 text-blue-500',
    titleClass: 'text-blue-700 dark:text-blue-300',
    descriptionClass: 'text-blue-600 dark:text-blue-300',
  },
  success: {
    iconClass: 'bg-green-50 dark:bg-green-900 text-green-500',
    titleClass: 'text-green-700 dark:text-green-300',
    descriptionClass: 'text-green-600 dark:text-green-300',
  },
  warning: {
    iconClass: 'bg-amber-50 dark:bg-amber-900 text-amber-500',
    titleClass: 'text-amber-700 dark:text-amber-300',
    descriptionClass: 'text-amber-600 dark:text-amber-300',
  },
  danger: {
    iconClass: 'bg-red-50 dark:bg-red-900 text-red-500',
    titleClass: 'text-red-700 dark:text-red-300',
    descriptionClass: 'text-red-600 dark:text-red-300',
  },
};

export function getEmptyStatePresentation(tone: EmptyStateTone): EmptyStatePresentation {
  return EMPTY_STATE_PRESENTATION[tone];
}
