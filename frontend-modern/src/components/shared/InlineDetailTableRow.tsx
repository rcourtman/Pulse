import { splitProps, type JSX } from 'solid-js';

import { TableCell, TableRow, type TableRowProps } from './Table';

export const INLINE_DETAIL_TABLE_CELL_CLASS = 'p-0 border-b border-border bg-surface-alt';
export const INLINE_DETAIL_TABLE_CONTENT_CLASS = 'px-2 py-3 sm:px-4 sm:py-4';

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
