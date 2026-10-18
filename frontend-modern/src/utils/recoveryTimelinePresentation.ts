export function getRecoveryTimelineDayFilterStateLabel(
  selected: boolean,
  timelineHasDayFilter: boolean,
): string {
  if (selected) return 'Day filter';
  if (timelineHasDayFilter) return 'Outside day filter';
  return 'Timeline day';
}

export function getRecoveryTimelineColumnButtonClass(
  _selected: boolean,
  _timelineHasSelection = false,
): string {
  const base =
    'group rounded-xs transition-all duration-150 focus-visible:outline-solid focus-visible:outline-2 focus-visible:outline-offset-1 focus-visible:outline-blue-500';
  return base;
}

export function getRecoveryTimelineBarMarkerClass(
  selected: boolean,
  timelineHasSelection = false,
): string {
  const base =
    'absolute inset-x-0 bottom-0 overflow-hidden rounded-xs transition-all duration-150 group-hover:ring-1 group-hover:ring-inset group-hover:ring-border group-focus-visible:ring-1 group-focus-visible:ring-inset group-focus-visible:ring-blue-500/60';
  if (selected) {
    return `${base} opacity-100 ring-2 ring-inset ring-blue-500/80`;
  }

  const focusClass = timelineHasSelection
    ? 'opacity-40 group-hover:opacity-100 group-focus-visible:opacity-100'
    : 'opacity-100';
  return `${base} ${focusClass}`;
}

export function getRecoveryTimelineEmptyMarkerClass(
  selected: boolean,
  timelineHasSelection = false,
): string {
  const base =
    'absolute inset-x-0 bottom-0 rounded-xs transition-all duration-150 group-hover:h-1 group-hover:bg-border group-focus-visible:h-1 group-focus-visible:bg-blue-500/60';
  if (selected) {
    return `${base} h-1 bg-blue-500 ring-2 ring-inset ring-blue-500/80`;
  }

  const focusClass = timelineHasSelection
    ? 'opacity-40 group-hover:opacity-100 group-focus-visible:opacity-100'
    : 'opacity-100';
  return `${base} h-0.5 bg-transparent ${focusClass}`;
}
