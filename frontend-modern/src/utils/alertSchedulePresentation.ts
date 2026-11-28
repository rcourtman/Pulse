export function getAlertQuietDayButtonClass(selected: boolean): string {
  return `min-h-11 rounded-md px-2 py-2 text-xs font-medium transition-all duration-200 focus-visible:outline-hidden focus-visible:ring-2 focus-visible:ring-blue-500 focus-visible:ring-offset-1 sm:min-h-0 dark:focus-visible:ring-offset-surface ${
    selected ? 'bg-blue-500 text-white shadow-xs' : 'text-muted hover:bg-surface-hover'
  }`;
}

export function getAlertQuietSuppressCardClass(selected: boolean): string {
  return `flex cursor-pointer items-start gap-3 rounded-md border px-3 py-2 transition-colors focus-within:ring-2 focus-within:ring-blue-500 focus-within:ring-offset-1 dark:focus-within:ring-offset-surface ${
    selected
      ? 'border-blue-500 bg-blue-50 dark:border-blue-400 dark:bg-blue-900/25'
      : 'border-border hover:bg-surface-hover'
  }`;
}

export function getAlertQuietSuppressCheckboxClass(selected: boolean): string {
  return `mt-1 flex h-4 w-4 shrink-0 items-center justify-center rounded-sm border-2 ${
    selected ? 'border-blue-500 bg-blue-500' : 'border-border'
  }`;
}
