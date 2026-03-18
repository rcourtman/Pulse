export interface AIExploreStatusPresentation {
  label: string;
  classes: string;
}

const DEFAULT_CLASSES =
  'border-sky-200 bg-sky-50 text-sky-700 dark:border-sky-500 dark:bg-sky-900 dark:text-sky-200';

const AI_EXPLORE_STATUS_PRESENTATION: Record<string, AIExploreStatusPresentation> = {
  started: {
    label: 'Explore Started',
    classes: DEFAULT_CLASSES,
  },
  completed: {
    label: 'Explore Completed',
    classes:
      'border-emerald-200 bg-emerald-50 text-emerald-700 dark:border-emerald-500 dark:bg-emerald-900 dark:text-emerald-200',
  },
  failed: {
    label: 'Explore Failed',
    classes:
      'border-rose-200 bg-rose-50 text-rose-700 dark:border-rose-500 dark:bg-rose-900 dark:text-rose-200',
  },
  skipped: {
    label: 'Explore Skipped',
    classes:
      'border-amber-200 bg-amber-50 text-amber-700 dark:border-amber-500 dark:bg-amber-900 dark:text-amber-200',
  },
};

export function getAIExploreStatusPresentation(phase?: string | null): AIExploreStatusPresentation {
  const normalized = (phase || '').trim().toLowerCase();
  if (!normalized) {
    return {
      label: 'Explore Status',
      classes: DEFAULT_CLASSES,
    };
  }

  return (
    AI_EXPLORE_STATUS_PRESENTATION[normalized] || {
      label: 'Explore Status',
      classes: DEFAULT_CLASSES,
    }
  );
}
