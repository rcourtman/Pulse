import { createEffect, createSignal, onCleanup, type Accessor } from 'solid-js';

/**
 * Sum of the colspans of the visible cells in a table's header row.
 *
 * Fixed-layout tables give any width their column percentages leave over to
 * every column that exists, and a cell spanning more columns than the header
 * has (a `colspan={100}` spacer or group row) creates the missing ones. On a
 * phone, where hidden columns leave the visible percentages short of 100%,
 * those phantom columns took the rest of the table: every row and drawer
 * stopped well short of the edge. Full-width helper rows span exactly the
 * visible header cells instead.
 */
export const countVisibleHeaderColumns = (table: HTMLTableElement | null): number => {
  const headerRow = table?.tHead?.rows[0];
  if (!headerRow) return 0;
  return Array.from(headerRow.cells).reduce(
    (total, cell) =>
      window.getComputedStyle(cell).display === 'none' ? total : total + cell.colSpan,
    0,
  );
};

/**
 * Tracks the visible header column count of the table containing `anchor`,
 * re-measuring when the anchor mounts, the table's shell resizes, or a header
 * cell is shown, hidden or re-spanned (column pickers, breakpoint tiers).
 */
export function createVisibleTableColumnSpan(
  anchor: Accessor<HTMLElement | null | undefined>,
  fallback = 1,
): Accessor<number> {
  const [span, setSpan] = createSignal(fallback);

  createEffect(() => {
    const table = anchor()?.closest('table') ?? null;
    if (!table) return;
    const measure = () => setSpan(countVisibleHeaderColumns(table) || fallback);
    measure();

    window.addEventListener('resize', measure);
    const shell = table.closest('.table-scroll-shell') ?? table;
    const resizeObserver =
      typeof ResizeObserver === 'function' ? new ResizeObserver(measure) : undefined;
    resizeObserver?.observe(shell);
    const headerRow = table.tHead?.rows[0];
    const headerObserver =
      headerRow && typeof MutationObserver === 'function'
        ? new MutationObserver(measure)
        : undefined;
    if (headerRow && headerObserver) {
      headerObserver.observe(headerRow, {
        childList: true,
        subtree: true,
        attributes: true,
        attributeFilter: ['class', 'style', 'hidden', 'colspan'],
      });
    }
    onCleanup(() => {
      window.removeEventListener('resize', measure);
      resizeObserver?.disconnect();
      headerObserver?.disconnect();
    });
  });

  return span;
}
