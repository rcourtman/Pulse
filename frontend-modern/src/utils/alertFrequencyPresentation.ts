export function getAlertFrequencySelectionPresentation() {
  return {
    containerClass:
      'inline-flex items-center gap-2 rounded-full border border-blue-200 bg-blue-50 px-3 py-1 text-xs text-blue-700 dark:border-blue-700 dark:bg-blue-900 dark:text-blue-200',
    labelClass:
      'font-medium uppercase tracking-wide text-[10px] text-blue-600 dark:text-blue-300',
  };
}

export function getAlertFrequencyClearFilterButtonClass(): string {
  return 'rounded bg-blue-100 px-2 py-0.5 text-xs text-blue-700 transition-colors hover:bg-blue-200 dark:bg-blue-900 dark:text-blue-300 dark:hover:bg-blue-800';
}
