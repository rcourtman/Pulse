import { clampWorkloadColumnWidth, type WorkloadColumnWidths } from './workloadColumnWidths';

/**
 * Shareable workload table layout, carried in the `cols` query parameter as
 * `id:width` pairs — for example `cols=name:172,cpu:151,netIo:170`.
 *
 * The parameter is authoritative while it is present: the ids decide which
 * columns render and in what order, and the widths pin them. That is what makes
 * a copied URL reproduce the same table for whoever opens it, instead of
 * landing on their own stored column preference. Removing the parameter returns
 * the table to that stored preference, which is never overwritten by a link.
 */
export const WORKLOAD_COLUMNS_URL_PARAM = 'cols';

export interface WorkloadColumnLayoutEntry {
  id: string;
  width: number;
}

/** Guards against a hand-edited URL asking for an absurd number of columns. */
const MAX_LAYOUT_ENTRIES = 32;

const isSafeColumnId = (id: string): boolean => /^[A-Za-z][A-Za-z0-9_-]*$/.test(id);

export const parseWorkloadColumnLayoutParam = (
  raw: string | null | undefined,
): WorkloadColumnLayoutEntry[] => {
  if (!raw) return [];

  const entries: WorkloadColumnLayoutEntry[] = [];
  const seen = new Set<string>();

  for (const chunk of raw.split(',')) {
    if (entries.length >= MAX_LAYOUT_ENTRIES) break;

    const separator = chunk.lastIndexOf(':');
    if (separator <= 0) continue;

    const id = chunk.slice(0, separator).trim();
    if (!isSafeColumnId(id) || seen.has(id)) continue;

    const width = Number(chunk.slice(separator + 1).trim());
    if (!Number.isFinite(width) || width <= 0) continue;

    seen.add(id);
    entries.push({ id, width: clampWorkloadColumnWidth(width) });
  }

  return entries;
};

export const serializeWorkloadColumnLayout = (
  entries: readonly WorkloadColumnLayoutEntry[],
): string => entries.map((entry) => `${entry.id}:${Math.round(entry.width)}`).join(',');

/**
 * Drops ids the current view cannot render — a stale link, a column retired by
 * an upgrade, or a vSphere layout opened on Proxmox. Order follows the link.
 */
export const resolveWorkloadColumnLayoutEntries = (
  entries: readonly WorkloadColumnLayoutEntry[],
  renderableColumnIds: ReadonlySet<string>,
): WorkloadColumnLayoutEntry[] => entries.filter((entry) => renderableColumnIds.has(entry.id));

export const workloadColumnLayoutWidths = (
  entries: readonly WorkloadColumnLayoutEntry[],
): WorkloadColumnWidths => {
  const widths: Record<string, number> = {};
  for (const entry of entries) widths[entry.id] = entry.width;
  return widths;
};

export const workloadColumnLayoutIds = (entries: readonly WorkloadColumnLayoutEntry[]): string[] =>
  entries.map((entry) => entry.id);

export const buildWorkloadColumnLayoutEntries = (
  orderedColumnIds: readonly string[],
  widths: WorkloadColumnWidths,
): WorkloadColumnLayoutEntry[] => {
  const entries: WorkloadColumnLayoutEntry[] = [];
  for (const id of orderedColumnIds) {
    const width = widths[id];
    if (typeof width !== 'number' || !Number.isFinite(width) || width <= 0) continue;
    entries.push({ id, width: clampWorkloadColumnWidth(width) });
  }
  return entries;
};

/**
 * Adds or removes one column while a link-driven layout is active. The stored
 * column preference is deliberately left alone so that toggling inside somebody
 * else's shared view does not rewrite the viewer's own defaults.
 */
export const toggleWorkloadColumnLayoutEntry = (
  entries: readonly WorkloadColumnLayoutEntry[],
  columnId: string,
  defaultWidth: number,
): WorkloadColumnLayoutEntry[] => {
  const id = columnId.trim();
  if (!isSafeColumnId(id)) return [...entries];

  const existing = entries.findIndex((entry) => entry.id === id);
  if (existing >= 0) {
    const next = entries.filter((entry) => entry.id !== id);
    // Never let a link strip the table down to nothing.
    return next.length > 0 ? next : [...entries];
  }

  if (entries.length >= MAX_LAYOUT_ENTRIES) return [...entries];
  return [...entries, { id, width: clampWorkloadColumnWidth(defaultWidth) }];
};
