import { JSX, splitProps } from 'solid-js';

export interface TableProps extends JSX.HTMLAttributes<HTMLTableElement> {
    wrapperClass?: string;
    wrapperRef?: (el: HTMLDivElement) => void;
}

export function Table(props: TableProps) {
    const [local, rest] = splitProps(props, ['class', 'children', 'wrapperClass', 'wrapperRef']);
    return (
        <div ref={local.wrapperRef} class={`w-full overflow-x-auto ${local.wrapperClass || ''}`} style={{ "-webkit-overflow-scrolling": "touch" }}>
            <table class={`w-full border-collapse text-left whitespace-nowrap ${local.class || ''}`} {...rest}>
                {local.children}
            </table>
        </div>
    );
}

export type TableHeaderProps = JSX.HTMLAttributes<HTMLTableSectionElement>;

export function TableHeader(props: TableHeaderProps) {
    const [local, rest] = splitProps(props, ['class', 'children']);
    return (
        <thead class={`bg-slate-50 dark:bg-slate-800 text-slate-600 dark:text-slate-300 border-b border-slate-200 dark:border-slate-700 ${local.class || ''}`} {...rest}>
            {local.children}
        </thead>
    );
}

export type TableBodyProps = JSX.HTMLAttributes<HTMLTableSectionElement>;

export function TableBody(props: TableBodyProps) {
    const [local, rest] = splitProps(props, ['class', 'children']);
    return (
        <tbody class={`divide-y divide-slate-200/50 dark:divide-slate-700/50 ${local.class || ''}`} {...rest}>
            {local.children}
        </tbody>
    );
}

export type TableRowProps = JSX.HTMLAttributes<HTMLTableRowElement>;

export function TableRow(props: TableRowProps) {
    const [local, rest] = splitProps(props, ['class', 'children']);
    return (
        <tr class={`group transition-colors duration-150 hover:bg-slate-50 dark:hover:bg-slate-700/40 ${local.class || ''}`} {...rest}>
            {local.children}
        </tr>
    );
}

export type TableHeadProps = JSX.HTMLAttributes<HTMLTableCellElement> & { colSpan?: number, colspan?: number };

export function TableHead(props: TableHeadProps) {
    const [local, rest] = splitProps(props, ['class', 'children']);
    return (
        <th class={`px-2 sm:px-3 py-1.5 sm:py-2 text-[11px] sm:text-xs font-semibold uppercase tracking-wider align-middle ${local.class || ''}`} {...rest}>
            {local.children}
        </th>
    );
}

export type TableCellProps = JSX.HTMLAttributes<HTMLTableCellElement> & { colSpan?: number, colspan?: number };

export function TableCell(props: TableCellProps) {
    const [local, rest] = splitProps(props, ['class', 'children']);
    return (
        <td class={`px-2 sm:px-3 py-1.5 sm:py-2 align-middle ${local.class || ''}`} {...rest}>
            {local.children}
        </td>
    );
}
