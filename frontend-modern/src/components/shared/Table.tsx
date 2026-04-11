import { JSX, splitProps } from 'solid-js';

export interface TableProps extends JSX.HTMLAttributes<HTMLTableElement> {
  wrapperClass?: string;
  wrapperProps?: JSX.HTMLAttributes<HTMLDivElement>;
  wrapperRef?: (el: HTMLDivElement) => void;
  width?: string | number;
}

export function Table(props: TableProps) {
  const [local, rest] = splitProps(props, [
    'class',
    'children',
    'wrapperClass',
    'wrapperProps',
    'wrapperRef',
  ]);
  return (
    <div
      {...local.wrapperProps}
      ref={local.wrapperRef}
      class={`w-full overflow-x-auto touch-scroll ${local.wrapperClass || ''}`}
    >
      <table
        class={`w-full border-collapse text-left whitespace-nowrap ${local.class || ''}`}
        {...rest}
      >
        {local.children}
      </table>
    </div>
  );
}

export type TableHeaderProps = JSX.HTMLAttributes<HTMLTableSectionElement>;

export function TableHeader(props: TableHeaderProps) {
  const [local, rest] = splitProps(props, ['class', 'children']);
  const customBorderPattern = /(?:^|\s)border-[^\s]+/;
  const borderClass = customBorderPattern.test(local.class ?? '') ? '' : 'border-b border-border';
  return (
    <thead class={`bg-surface text-muted ${borderClass} ${local.class || ''}`.trim()} {...rest}>
      {local.children}
    </thead>
  );
}

export type TableBodyProps = JSX.HTMLAttributes<HTMLTableSectionElement>;

export function TableBody(props: TableBodyProps) {
  const [local, rest] = splitProps(props, ['class', 'children']);
  const customDividePattern = /(?:^|\s)divide-[^\s]+/;
  const bodyClass = customDividePattern.test(local.class ?? '')
    ? (local.class ?? '')
    : `divide-y divide-border ${local.class || ''}`;
  return (
    <tbody class={bodyClass.trim()} {...rest}>
      {local.children}
    </tbody>
  );
}

export type TableRowProps = JSX.HTMLAttributes<HTMLTableRowElement>;

export function TableRow(props: TableRowProps) {
  const [local, rest] = splitProps(props, ['class', 'children']);
  return (
    <tr
      class={`group transition-colors duration-150 hover:bg-surface-hover ${local.class || ''}`}
      {...rest}
    >
      {local.children}
    </tr>
  );
}

export type TableHeadProps = JSX.HTMLAttributes<HTMLTableCellElement> & {
  colSpan?: number;
  colspan?: number;
  width?: string | number;
};

export function TableHead(props: TableHeadProps) {
  const [local, rest] = splitProps(props, ['class', 'children']);
  return (
    <th
      class={`px-2 sm:px-3 py-1.5 text-[11px] sm:text-xs font-semibold uppercase tracking-wider align-middle ${local.class || ''}`}
      {...rest}
    >
      {local.children}
    </th>
  );
}

export type TableCellProps = JSX.HTMLAttributes<HTMLTableCellElement> & {
  colSpan?: number;
  colspan?: number;
  width?: string | number;
};

export function TableCell(props: TableCellProps) {
  const [local, rest] = splitProps(props, ['class', 'children']);
  return (
    <td class={`px-2 sm:px-3 py-0.5 align-middle ${local.class || ''}`} {...rest}>
      {local.children}
    </td>
  );
}
