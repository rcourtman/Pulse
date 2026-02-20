import { JSX, For, Show, createMemo, splitProps } from 'solid-js';
import { useBreakpoint } from '@/hooks/useBreakpoint';
import { Table, TableHeader, TableBody, TableRow, TableHead, TableCell } from '@/components/shared/Table';

export interface TableColumn<T> {
    key: keyof T | string;
    label: string;
    /** Custom render function for the cell content. Useful for badges, links, or complex nested data */
    render?: (row: T) => JSX.Element;
    /** Optional alignment for the header and cell content */
    align?: 'left' | 'center' | 'right';
    /** Optional fixed width or width class */
    width?: string;
    /** Hidden on mobile via CSS class */
    hiddenOnMobile?: boolean;
}

export interface PulseDataGridProps<T> {
    /** The rows of data to display */
    data: T[];
    /** Definitions for each column */
    columns: TableColumn<T>[];

    /** 
     * A unique identifier function for each row to help SolidJS optimize rendering.
     * Typically `(row) => row.id`
     */
    keyExtractor: (row: T) => string | number;

    /** Triggers when a row is clicked. */
    onRowClick?: (row: T) => void;

    /** What to display when the data array is empty */
    emptyState?: JSX.Element;

    /** Set to true to show a loading state */
    isLoading?: boolean;

    /** Determines if the current row should be expanded */
    isRowExpanded?: (row: T) => boolean;

    /** Render function for the expanded content of a row */
    expandedRender?: (row: T) => JSX.Element;

    /** Minimum width on desktop before horizontal scrolling kicks in */
    desktopMinWidth?: string;

    /** 
     * Minimum width on mobile. 
     * Defaults to '100%' so the table flexes into horizontal scroll natively 
     * without artificially breaking the screen width.
     */
    mobileMinWidth?: string;

    /** Custom classes applied to the root container */
    class?: string;
}

/**
 * A standardized, responsive datagrid component for Pulse.
 * 
 * Enforces unified table header styling (uppercase, tracking, specific padding/font size),
 * modern row hover effects, and proper mobile responsive horizontal scrolling behavior.
 */
export function PulseDataGrid<T>(props: PulseDataGridProps<T>) {
    const [local, _] = splitProps(props, [
        'data',
        'columns',
        'keyExtractor',
        'onRowClick',
        'emptyState',
        'isLoading',
        'isRowExpanded',
        'expandedRender',
        'desktopMinWidth',
        'mobileMinWidth',
        'class'
    ]);

    const { isMobile } = useBreakpoint();

    const effectiveMinWidth = createMemo(() => {
        if (isMobile()) {
            return local.mobileMinWidth ?? '100%';
        }
        return local.desktopMinWidth ?? '800px';
    });

    const getAlignClass = (align?: 'left' | 'center' | 'right') => {
        switch (align) {
            case 'center': return 'text-center justify-center';
            case 'right': return 'text-right justify-end';
            case 'left':
            default: return 'text-left justify-start';
        }
    };

    return (
        <div
            class={`overflow-hidden rounded-md border border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-900 ${local.class || ''}`}
        >
            <div
                class="overflow-x-auto"
                style={{
                    '-webkit-overflow-scrolling': 'touch',
                    'scrollbar-width': 'none', // Firefox
                    '-ms-overflow-style': 'none'  // IE 10+
                }}
            >
                <style>
                    {`.overflow-x-auto::-webkit-scrollbar { display: none; }`}
                </style>

                <Table
                    class="w-full border-collapse"
                    style={{ 'min-width': effectiveMinWidth() }}
                >
                    <TableHeader class="bg-slate-50 dark:bg-slate-800 border-b border-slate-200 dark:border-slate-700">
                        <TableRow>
                            <For each={local.columns}>
                                {(col) => (
                                    <TableHead
                                        class={`
                                            px-3 sm:px-4 py-2.5 
                                            text-[11px] sm:text-xs font-semibold uppercase tracking-wider whitespace-nowrap text-slate-500 dark:text-slate-400
                                            ${getAlignClass(col.align)}
                                            ${col.hiddenOnMobile ? 'hidden sm:table-cell' : ''}
                                        `}
                                        style={col.width ? { width: col.width } : {}}
                                    >
                                        {col.label}
                                    </TableHead>
                                )}
                            </For>
                        </TableRow>
                    </TableHeader>
                    <TableBody class="divide-y divide-slate-100 dark:divide-slate-800 transition-colors">
                        <Show when={!local.isLoading && local.data.length > 0}>
                            <For each={local.data}>
                                {(row) => {
                                    const expanded = createMemo(() => local.isRowExpanded?.(row));

                                    return (
                                        <>
                                            <TableRow
                                                class={`
                                                    group transition-colors duration-150
                                                    ${local.onRowClick
                                                        ? 'cursor-pointer hover:bg-blue-50 dark:hover:bg-blue-900'
                                                        : 'hover:bg-slate-50 dark:hover:bg-slate-800'
                                                    }
                                                `}
                                                onClick={() => local.onRowClick?.(row)}
                                            >
                                                <For each={local.columns}>
                                                    {(col) => (
                                                        <TableCell
                                                            class={`
                                                                px-3 sm:px-4 py-2 sm:py-3.5 
                                                                text-sm text-slate-700 dark:text-slate-300 align-middle
                                                                ${getAlignClass(col.align)}
                                                                ${col.hiddenOnMobile ? 'hidden sm:table-cell' : ''}
                                                            `}
                                                        >
                                                            <Show
                                                                when={col.render}
                                                                fallback={<span>{(row as any)[col.key]}</span>}
                                                            >
                                                                {col.render!(row)}
                                                            </Show>
                                                        </TableCell>
                                                    )}
                                                </For>
                                            </TableRow>
                                            <Show when={expanded() && local.expandedRender}>
                                                <TableRow class={local.onRowClick ? 'bg-slate-50 dark:bg-slate-800 hover:bg-blue-50 dark:hover:bg-blue-900' : 'bg-slate-50 dark:bg-slate-800 hover:bg-slate-50 dark:hover:bg-slate-800'}>
                                                    <TableCell colspan={local.columns.length} class="px-0 py-0 border-t-0">
                                                        {local.expandedRender!(row)}
                                                    </TableCell>
                                                </TableRow>
                                            </Show>
                                        </>
                                    );
                                }}
                            </For>
                        </Show>

                        <Show when={local.isLoading}>
                            <TableRow>
                                <TableCell colspan={local.columns.length} class="px-4 py-8 text-center text-sm text-slate-500">
                                    <div class="flex items-center justify-center gap-2">
                                        <div class="w-4 h-4 rounded-full border-2 border-slate-300 border-t-blue-600 animate-spin"></div>
                                        Loading...
                                    </div>
                                </TableCell>
                            </TableRow>
                        </Show>

                        <Show when={!local.isLoading && local.data.length === 0}>
                            <TableRow>
                                <TableCell colspan={local.columns.length} class="px-4 py-8 text-center text-sm text-slate-500 italic bg-slate-50 dark:bg-slate-800">
                                    <Show when={local.emptyState} fallback="No items available.">
                                        {local.emptyState}
                                    </Show>
                                </TableCell>
                            </TableRow>
                        </Show>
                    </TableBody>
                </Table>
            </div>
        </div>
    );
}
