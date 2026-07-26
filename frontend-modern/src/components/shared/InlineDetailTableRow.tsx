import { splitProps, type JSX } from 'solid-js';

import { TableCell, TableRow, type TableRowProps } from './Table';

export const INLINE_DETAIL_TABLE_CELL_CLASS = 'p-0 border-b border-border bg-surface-alt';
// overflow-x-clip keeps over-wide content from painting outside the row's
// border below the lg breakpoint (issue #1622) without creating a scroll
// container the way overflow-hidden would.
export const INLINE_DETAIL_TABLE_CONTENT_CLASS =
  'sticky left-0 max-w-[calc(100vw-3.5rem)] overflow-x-clip px-2 py-3 sm:px-4 sm:py-4 lg:static lg:max-w-none lg:overflow-x-visible';

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

  return (
    <TableRow class={local.class} {...rest}>
      <TableCell
        id={local.cellId}
        colspan={local.colspan ?? local.colSpan}
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
