export function getAlertQuietDayButtonClass(selected: boolean): string {
  return `rounded-md px-2 py-2 text-xs font-medium transition-all duration-200 ${
    selected ? 'bg-blue-500 text-white shadow-sm' : 'text-muted hover:bg-surface-hover'
  }`;
}

export function getAlertQuietSuppressCardClass(selected: boolean): string {
  return `flex cursor-pointer items-start gap-3 rounded-md border px-3 py-2 transition-colors ${
    selected
      ? 'border-blue-500 bg-blue-50 dark:border-blue-400 dark:bg-blue-500'
      : 'border-border hover:bg-surface-hover'
  }`;
}

export function getAlertQuietSuppressCheckboxClass(selected: boolean): string {
  return `mt-1 flex h-4 w-4 items-center justify-center rounded border-2 ${
    selected ? 'border-blue-500 bg-blue-500' : 'border-border'
  }`;
}
