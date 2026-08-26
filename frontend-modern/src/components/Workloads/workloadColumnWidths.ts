import type { WorkloadTableLayoutMode } from './guestRowModel';

/**
 * User-pinned workload column widths, in CSS pixels, keyed by column id.
 *
 * An empty map is the canonical "untouched" state: the responsive weights in
 * `guestRowModel` own every width and the table keeps the maintained contract
 * of fitting its container without horizontal scroll. A non-empty map means the
 * operator has explicitly dragged a column edge and has therefore opted into
 * manual sizing for the current scope.
 */
export type WorkloadColumnWidths = Readonly<Record<string, number>>;

export const WORKLOAD_COLUMN_MIN_WIDTH = 48;
export const WORKLOAD_COLUMN_MAX_WIDTH = 720;

/**
 * Manual sizing is a pointer-driven desktop affordance. Touch layouts stay on
 * the responsive weights so a width pinned on a wide monitor can never leak
 * into a phone-sized render of the same scope.
 */
const MANUAL_SIZING_LAYOUT_MODES: ReadonlySet<WorkloadTableLayoutMode> = new Set([
  'tablet',
  'compact',
  'wide',
]);

export const isWorkloadManualSizingSupported = (layoutMode: WorkloadTableLayoutMode): boolean =>
  MANUAL_SIZING_LAYOUT_MODES.has(layoutMode);

/**
 * Pointer travel, in CSS pixels, before a press on a resize handle counts as a
 * resize rather than a click.
 *
 * Without a threshold a plain click on the handle would open and immediately
 * commit a zero-delta drag, which pins every rendered width and silently
 * switches the table into manual sizing — a mode change the operator never
 * asked for.
 */
export const WORKLOAD_COLUMN_DRAG_ENGAGE_THRESHOLD = 3;

export const shouldEngageColumnResize = (deltaX: number, alreadyEngaged: boolean): boolean => {
  if (alreadyEngaged) return true;
  if (!Number.isFinite(deltaX)) return false;
  return Math.abs(deltaX) >= WORKLOAD_COLUMN_DRAG_ENGAGE_THRESHOLD;
};

export const clampWorkloadColumnWidth = (width: number): number =>
  Math.min(WORKLOAD_COLUMN_MAX_WIDTH, Math.max(WORKLOAD_COLUMN_MIN_WIDTH, Math.round(width)));

export const hasWorkloadColumnWidths = (widths: WorkloadColumnWidths | undefined): boolean =>
  !!widths && Object.keys(widths).length > 0;

/**
 * Accepts anything `localStorage` hands back and returns a map that the render
 * path can trust. Unusable entries are dropped rather than throwing, so one bad
 * key written by an older build cannot wedge the table.
 */
export const normalizeWorkloadColumnWidths = (value: unknown): WorkloadColumnWidths => {
  if (!value || typeof value !== 'object' || Array.isArray(value)) return {};

  const normalized: Record<string, number> = {};
  for (const [columnId, rawWidth] of Object.entries(value as Record<string, unknown>)) {
    const id = columnId.trim();
    if (!id) continue;
    const width = typeof rawWidth === 'number' ? rawWidth : Number(rawWidth);
    if (!Number.isFinite(width) || width <= 0) continue;
    normalized[id] = clampWorkloadColumnWidth(width);
  }

  return normalized;
};

export const withWorkloadColumnWidth = (
  widths: WorkloadColumnWidths,
  columnId: string,
  width: number,
): WorkloadColumnWidths => {
  const id = columnId.trim();
  if (!id || !Number.isFinite(width)) return widths;
  return { ...widths, [id]: clampWorkloadColumnWidth(width) };
};

export const withoutWorkloadColumnWidth = (
  widths: WorkloadColumnWidths,
  columnId: string,
): WorkloadColumnWidths => {
  if (!(columnId in widths)) return widths;
  const next = { ...widths };
  delete next[columnId];
  return next;
};

/**
 * Drops pins for columns that are no longer part of the visible set so a column
 * the operator has since hidden cannot keep reserving width forever.
 */
export const pruneWorkloadColumnWidths = (
  widths: WorkloadColumnWidths,
  visibleColumnIds: readonly string[],
): WorkloadColumnWidths => {
  const visible = new Set(visibleColumnIds);
  const next: Record<string, number> = {};
  for (const [columnId, width] of Object.entries(widths)) {
    if (visible.has(columnId)) next[columnId] = width;
  }
  return Object.keys(next).length === Object.keys(widths).length ? widths : next;
};

/**
 * `table-fixed` distributes leftover space across columns that have no explicit
 * width, so pinning a single column would silently reflow every other one. The
 * first drag therefore freezes the widths the operator can currently see, and
 * only then applies their delta.
 */
export const snapshotWorkloadColumnWidths = (
  widths: WorkloadColumnWidths,
  measuredWidths: Readonly<Record<string, number>>,
): WorkloadColumnWidths => {
  const next: Record<string, number> = { ...widths };
  for (const [columnId, measured] of Object.entries(measuredWidths)) {
    if (columnId in next) continue;
    if (!Number.isFinite(measured) || measured <= 0) continue;
    next[columnId] = clampWorkloadColumnWidth(measured);
  }
  return next;
};

/**
 * Columns that only become visible once manual sizing engages were never
 * measured, so they would otherwise arrive unpinned and let `table-fixed`
 * redistribute the surplus across every column. Seeding them from their design
 * width keeps the table exactly as wide as the sum of its columns.
 */
export const WORKLOAD_COLUMN_FALLBACK_WIDTH = 120;

export const parsePixelWidth = (value: string | undefined): number | undefined => {
  if (!value) return undefined;
  const match = /^(\d+(?:\.\d+)?)px$/.exec(value.trim());
  if (!match) return undefined;
  const parsed = Number(match[1]);
  return Number.isFinite(parsed) && parsed > 0 ? parsed : undefined;
};

export const seedWorkloadColumnWidthsFromDefaults = (
  widths: WorkloadColumnWidths,
  columns: readonly { id: string; width?: string; minWidth?: string }[],
): WorkloadColumnWidths => {
  const next: Record<string, number> = { ...widths };
  for (const column of columns) {
    if (column.id in next) continue;
    const preferred =
      parsePixelWidth(column.width) ??
      parsePixelWidth(column.minWidth) ??
      WORKLOAD_COLUMN_FALLBACK_WIDTH;
    next[column.id] = clampWorkloadColumnWidth(preferred);
  }
  return next;
};

/**
 * Total width the table must claim so `table-layout: fixed` keeps applying.
 *
 * A table with `width: auto` silently drops out of fixed layout and falls back
 * to auto layout, where a narrow column is re-expanded to its content minimum.
 * Publishing the exact sum keeps the width definite, so every pin is honoured
 * and the shell scrolls by precisely the overflow.
 *
 * Returns null when there is nothing pinned, which leaves the default
 * percentage layout completely untouched.
 */
export const sumWorkloadColumnWidths = (
  widths: WorkloadColumnWidths,
  visibleColumnIds: readonly string[],
): number | null => {
  if (!hasWorkloadColumnWidths(widths) || visibleColumnIds.length === 0) return null;
  let total = 0;
  for (const columnId of visibleColumnIds) {
    const pinned = widths[columnId];
    if (typeof pinned !== 'number' || !Number.isFinite(pinned) || pinned <= 0) return null;
    total += pinned;
  }
  return total > 0 ? total : null;
};

export const workloadColumnWidthsStorageKey = (baseKey: string, scope?: string): string => {
  const trimmed = scope?.trim();
  return trimmed ? `${baseKey}:${trimmed}` : baseKey;
};
