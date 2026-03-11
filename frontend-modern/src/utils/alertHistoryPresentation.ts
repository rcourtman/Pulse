export interface AlertHistorySourcePresentation {
  label: string;
  className: string;
}

export function getAlertHistorySourcePresentation(source?: string | null): AlertHistorySourcePresentation {
  const normalized = (source ?? '').trim().toLowerCase();
  if (normalized === 'ai') {
    return {
      label: 'Patrol',
      className:
        'text-[10px] px-1.5 py-0.5 rounded font-medium bg-violet-100 dark:bg-violet-900 text-violet-700 dark:text-violet-300',
    };
  }

  return {
    label: 'Alert',
    className:
      'text-[10px] px-1.5 py-0.5 rounded font-medium bg-sky-100 dark:bg-sky-900 text-sky-700 dark:text-sky-300',
  };
}

export function getAlertHistoryResourceTypeBadgeClass(resourceType?: string | null): string {
  const normalized = (resourceType ?? '').trim().toLowerCase();

  if (normalized === 'vm' || normalized === 'node') {
    return 'text-xs px-1 py-0.5 rounded bg-blue-100 dark:bg-blue-900 text-blue-700 dark:text-blue-300';
  }

  if (normalized === 'container' || normalized === 'ct') {
    return 'text-xs px-1 py-0.5 rounded bg-green-100 dark:bg-green-900 text-green-700 dark:text-green-300';
  }

  if (normalized === 'storage') {
    return 'text-xs px-1 py-0.5 rounded bg-orange-100 dark:bg-orange-900 text-orange-700 dark:text-orange-300';
  }

  return 'text-xs px-1 py-0.5 rounded bg-surface-hover text-base-content';
}
