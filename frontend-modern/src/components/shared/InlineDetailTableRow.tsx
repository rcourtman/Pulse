import { createSignal, onCleanup, onMount, splitProps, type JSX } from 'solid-js';

import { TableCell, TableRow, type TableRowProps } from './Table';

export const INLINE_DETAIL_TABLE_CELL_CLASS = 'p-0 border-b border-border bg-surface-alt';
// overflow-x-clip keeps over-wide content from painting outside the row's
// border below the lg breakpoint (issue #1622) without creating a scroll
// container the way overflow-hidden would.
export const INLINE_DETAIL_TABLE_CONTENT_CLASS =
  'sticky left-0 min-w-0 max-w-[calc(100vw-3.5rem)] whitespace-normal overflow-x-clip px-2 py-3 sm:px-4 sm:py-4 lg:static lg:max-w-none lg:overflow-x-visible';

export interface InlineDetailTableRowProps extends TableRowProps {
  cellId?: string;
  cellClass?: string;
  colSpan?: number;
  colspan?: number;
  contentClass?: string;
  containClicks?: boolean;
  children?: JSX.Element;
}

const joinClasses = (...classes: Array<string | undefined>): string =>
  classes.filter(Boolean).join(' ');

export function InlineDetailTableRow(props: InlineDetailTableRowProps) {
  const [local, rest] = splitProps(props, [
    'cellId',
    'cellClass',
    'children',
    'class',
    'colSpan',
    'colspan',
    'containClicks',
    'contentClass',
  ]);
  const containClicks = () => local.containClicks ?? true;
  const contentClass = () => local.contentClass ?? INLINE_DETAIL_TABLE_CONTENT_CLASS;
  const requestedColspan = () => local.colspan ?? local.colSpan ?? 1;
  const [effectiveColspan, setEffectiveColspan] = createSignal(requestedColspan());
  let detailRow: HTMLTableRowElement | undefined;

  onCleanup(() => {
    const activeElement = document.activeElement;
    const cellId = local.cellId;
    if (!cellId || !activeElement || !detailRow?.contains(activeElement)) return;

    // A drawer close commonly disposes the button that currently owns focus.
    // Restore it to the disclosure controlling this detail row once Solid has
    // removed the row, so keyboard users stay anchored to the same resource.
    queueMicrotask(() => {
      const disclosure = Array.from(
        document.querySelectorAll<HTMLButtonElement>('button[aria-controls]'),
      ).find((button) => button.getAttribute('aria-controls') === cellId);
      // Closing or replacing a detail row can happen while the surrounding
      // live table is well below the top of the page. Restore keyboard focus
      // without letting the browser scroll the disclosure into view: the
      // caller owns any deliberate reveal, while polling must not move the
      // operator's viewport.
      disclosure?.focus({ preventScroll: true });
    });
  });

  const syncColspanToVisibleSummaryCells = () => {
    const summaryRow = detailRow?.previousElementSibling;
    if (!(summaryRow instanceof HTMLTableRowElement)) {
      setEffectiveColspan(requestedColspan());
      return;
    }

    const visibleColspan = Array.from(
      summaryRow.querySelectorAll<HTMLTableCellElement>(':scope > th, :scope > td'),
    ).reduce(
      (total, cell) =>
        window.getComputedStyle(cell).display === 'none' ? total : total + cell.colSpan,
      0,
    );
    setEffectiveColspan(visibleColspan || requestedColspan());
  };

  onMount(() => {
    syncColspanToVisibleSummaryCells();
    window.addEventListener('resize', syncColspanToVisibleSummaryCells);

    const tableShell = detailRow?.closest('.table-scroll-shell');
    const observer =
      tableShell && typeof ResizeObserver === 'function'
        ? new ResizeObserver(syncColspanToVisibleSummaryCells)
        : undefined;
    if (tableShell) observer?.observe(tableShell);

    onCleanup(() => {
      window.removeEventListener('resize', syncColspanToVisibleSummaryCells);
      observer?.disconnect();
    });
  });

  return (
    <TableRow ref={detailRow} class={local.class} {...rest}>
      <TableCell
        id={local.cellId}
        colspan={effectiveColspan()}
        class={joinClasses(INLINE_DETAIL_TABLE_CELL_CLASS, local.cellClass)}
      >
        <div
          class={contentClass()}
          onClick={(event) => {
            if (containClicks()) {
              event.stopPropagation();
            }
          }}
        >
          {local.children}
        </div>
      </TableCell>
    </TableRow>
  );
}

export default InlineDetailTableRow;
