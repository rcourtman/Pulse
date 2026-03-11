export function getAlertGroupingCardClass(selected: boolean): string {
  return `relative flex items-center gap-2 rounded-md border-2 p-3 transition-all ${
    selected
      ? 'border-blue-500 bg-blue-50 shadow-sm dark:bg-blue-900'
      : 'border-border hover:bg-surface-hover'
  }`;
}

export function getAlertGroupingCheckboxClass(selected: boolean): string {
  return `flex h-4 w-4 items-center justify-center rounded border-2 ${
    selected ? 'border-blue-500 bg-blue-500' : 'border-border'
  }`;
}
