import type { SourceType } from '@/types/resource';

export interface SourceTypePresentation {
  label: string;
  badgeClasses: string;
}

const SOURCE_TYPE_PRESENTATION: Record<SourceType, SourceTypePresentation> = {
  agent: {
    label: 'Agent',
    badgeClasses: 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900 dark:text-emerald-400',
  },
  api: {
    label: 'API',
    badgeClasses: 'bg-blue-100 text-blue-700 dark:bg-blue-900 dark:text-blue-400',
  },
  hybrid: {
    label: 'Hybrid',
    badgeClasses: 'bg-teal-100 text-teal-700 dark:bg-teal-900 dark:text-teal-400',
  },
};

export const getSourceTypePresentation = (
  sourceType?: SourceType | string | null,
): SourceTypePresentation | null => {
  if (!sourceType) return null;
  return SOURCE_TYPE_PRESENTATION[sourceType as SourceType] || null;
};

export const getSourceTypeLabel = (sourceType?: SourceType | string | null): string | null =>
  getSourceTypePresentation(sourceType)?.label || (sourceType ? String(sourceType) : null);
