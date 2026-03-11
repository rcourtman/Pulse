export function getAlertQuietDayButtonClass(selected: boolean): string {
  return `rounded-md px-2 py-2 text-xs font-medium transition-all duration-200 ${
    selected ? 'bg-blue-500 text-white shadow-sm' : 'text-muted hover:bg-surface-hover'
  }`;
}
