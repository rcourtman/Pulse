import type { FileChange } from '@/api/aiChat';

export interface AISessionDiffStatusPresentation {
  label: string;
  badgeClasses: string;
}

export function getAISessionDiffStatusPresentation(
  status: FileChange['status'],
): AISessionDiffStatusPresentation {
  switch (status) {
    case 'added':
      return {
        label: 'Added',
        badgeClasses: 'bg-green-100 text-green-700 dark:bg-green-900 dark:text-green-200',
      };
    case 'modified':
      return {
        label: 'Modified',
        badgeClasses: 'bg-amber-100 text-amber-700 dark:bg-amber-900 dark:text-amber-200',
      };
    case 'deleted':
      return {
        label: 'Deleted',
        badgeClasses: 'bg-red-100 text-red-700 dark:bg-red-900 dark:text-red-200',
      };
    default:
      return {
        label: 'Changed',
        badgeClasses: 'bg-surface-alt text-base-content',
      };
  }
}
