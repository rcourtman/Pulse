import {
  getRecoveryArtifactModePresentation,
  type RecoveryArtifactMode,
} from './recoveryArtifactModePresentation';

export interface RecoveryTimelineBreakdownPoint {
  total: number;
  snapshot: number;
  local: number;
  remote: number;
}

export interface RecoveryTimelineTooltipRow {
  mode: RecoveryArtifactMode;
  label: string;
  count: number;
  value: string;
  segmentClassName: string;
  muted: boolean;
}

const RECOVERY_TIMELINE_TOOLTIP_MODES: RecoveryArtifactMode[] = ['snapshot', 'local', 'remote'];

function getRecoveryTimelineModeValue(
  point: RecoveryTimelineBreakdownPoint,
  mode: RecoveryArtifactMode,
): number {
  return Math.max(0, Number(point[mode] || 0));
}

export function getRecoveryTimelinePointTotalLabel(total: number): string {
  const normalized = Math.max(0, Number(total || 0));
  return `${normalized} recovery point${normalized === 1 ? '' : 's'}`;
}

export function getRecoveryTimelineTooltipRows(
  point: RecoveryTimelineBreakdownPoint,
): RecoveryTimelineTooltipRow[] {
  const total = Math.max(0, Number(point.total || 0));

  return RECOVERY_TIMELINE_TOOLTIP_MODES.map((mode) => {
    const presentation = getRecoveryArtifactModePresentation(mode);
    const count = getRecoveryTimelineModeValue(point, mode);
    const percentage = total > 0 && count > 0 ? Math.round((count / total) * 100) : 0;

    return {
      mode,
      label: presentation.aggregateLabel,
      count,
      value: percentage > 0 ? `${count} (${percentage}%)` : String(count),
      segmentClassName: presentation.segmentClassName,
      muted: count === 0,
    };
  });
}

export function getRecoveryTimelineDayFilterStateLabel(
  selected: boolean,
  timelineHasDayFilter: boolean,
): string {
  if (selected) return 'Day filter';
  if (timelineHasDayFilter) return 'Outside day filter';
  return 'Timeline day';
}

export function getRecoveryTimelineDayFilterLabel(dateLabel: string, total: number): string {
  return `${dateLabel} - ${getRecoveryTimelinePointTotalLabel(total)}`;
}

export function getRecoveryTimelineColumnButtonClass(
  _selected: boolean,
  _timelineHasSelection = false,
): string {
  const base =
    'group rounded-sm transition-all duration-150 focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-1 focus-visible:outline-blue-500';
  return base;
}

export function getRecoveryTimelineBarMarkerClass(
  selected: boolean,
  timelineHasSelection = false,
): string {
  const base =
    'absolute inset-x-0 bottom-0 overflow-hidden rounded-sm transition-all duration-150 group-hover:ring-1 group-hover:ring-inset group-hover:ring-border group-focus-visible:ring-1 group-focus-visible:ring-inset group-focus-visible:ring-blue-500/60';
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
    'absolute inset-x-0 bottom-0 rounded-sm transition-all duration-150 group-hover:h-1 group-hover:bg-border group-focus-visible:h-1 group-focus-visible:bg-blue-500/60';
  if (selected) {
    return `${base} h-1 bg-blue-500 ring-2 ring-inset ring-blue-500/80`;
  }

  const focusClass = timelineHasSelection
    ? 'opacity-40 group-hover:opacity-100 group-focus-visible:opacity-100'
    : 'opacity-100';
  return `${base} h-0.5 bg-transparent ${focusClass}`;
}

export function getRecoveryTimelineColumnAriaLabel(
  dateLabel: string,
  total: number,
  selected: boolean,
): string {
  const countLabel = `${total} recovery point${total === 1 ? '' : 's'}`;
  return selected ? `${dateLabel}: ${countLabel}, selected` : `${dateLabel}: ${countLabel}`;
}
