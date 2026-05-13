export function getRecoveryTimelineColumnButtonClass(selected: boolean): string {
  const base =
    'rounded-sm focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-1 focus-visible:outline-blue-500';
  return selected
    ? `${base} bg-blue-100 ring-1 ring-blue-500/70 dark:bg-blue-900`
    : `${base} hover:bg-surface-hover`;
}

export function getRecoveryTimelineColumnAriaLabel(
  dateLabel: string,
  total: number,
  selected: boolean,
): string {
  const countLabel = `${total} recovery point${total === 1 ? '' : 's'}`;
  return selected ? `${dateLabel}: ${countLabel}, selected` : `${dateLabel}: ${countLabel}`;
}
