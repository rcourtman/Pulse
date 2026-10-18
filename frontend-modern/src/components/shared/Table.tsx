import { JSX, splitProps } from 'solid-js';

export interface TableProps extends JSX.HTMLAttributes<HTMLTableElement> {
  phoneVerticalScrollOwner?: 'page' | 'table';
  wrapperClass?: string;
  wrapperProps?: JSX.HTMLAttributes<HTMLDivElement>;
  wrapperRef?: (el: HTMLDivElement) => void;
  width?: string | number;
}

export function Table(props: TableProps) {
  const [local, rest] = splitProps(props, [
    'class',
    'children',
    'phoneVerticalScrollOwner',
    'wrapperClass',
    'wrapperProps',
    'wrapperRef',
  ]);
  return (
    <div
      {...local.wrapperProps}
      ref={local.wrapperRef}
      class={`table-scroll-shell w-full min-w-0 max-w-full overflow-x-auto touch-scroll ${local.phoneVerticalScrollOwner === 'page' ? 'table-scroll-shell-phone-page' : ''} ${local.wrapperClass || ''}`}
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

// WebKit touch needs a native click target on otherwise-static table rows.
// Keep actions document-delegated: nested controls must run first and retain
// their existing stopPropagation behaviour instead of opening the row too.
// Custom row shells reuse this marker without inheriting TableRow's styling.
export const nativeRowClickTarget = () => undefined;

export type RowTextSelectionGuard = {
  onMouseDown: (event: MouseEvent) => void;
  isSelectionClick: (event: MouseEvent) => boolean;
};

// Further than this between press and release, the pointer was dragging.
const ROW_CLICK_DRAG_TOLERANCE_PX = 3;

// Dragging across a name to copy it (a pod name for kubectl, a container name
// for docker) ends with a click on the row the drag stayed inside. That click
// finishes a text selection; it is not a request to open the row, and running
// the row action would shift the layout under the selection, so row actions
// skip it. A press released where it went down is still a plain click, even
// on text that is already selected: the browser only clears that selection
// after the click, and the row must open on the first click, not the second.
// Nested controls such as the disclosure button handle their own clicks, so
// they and keyboard activation are unaffected. TableRow wires this itself;
// custom row shells create one per row and wire both handlers.
export function createRowTextSelectionGuard(): RowTextSelectionGuard {
  let pressedAt: { x: number; y: number } | undefined;
  return {
    onMouseDown: (event) => {
      pressedAt = { x: event.clientX, y: event.clientY };
    },
    isSelectionClick: (event) => {
      const press = pressedAt;
      pressedAt = undefined;
      const row = event.currentTarget;
      if (!(row instanceof Node)) return false;
      const selection = row.ownerDocument?.getSelection();
      if (!selection || selection.isCollapsed) return false;
      if (!row.contains(selection.anchorNode) && !row.contains(selection.focusNode)) return false;
      if (!press) return true;
      return (
        Math.abs(event.clientX - press.x) > ROW_CLICK_DRAG_TOLERANCE_PX ||
        Math.abs(event.clientY - press.y) > ROW_CLICK_DRAG_TOLERANCE_PX
      );
    },
  };
}

export function TableRow(props: TableRowProps) {
  const [local, rest] = splitProps(props, ['class', 'children', 'onClick']);
  const selectionGuard = createRowTextSelectionGuard();
  const runRowAction: JSX.EventHandler<HTMLTableRowElement, MouseEvent> = (event) => {
    if (selectionGuard.isSelectionClick(event)) return;
    const action = local.onClick;
    if (typeof action === 'function') action(event);
    else action?.[0](action[1], event);
  };
  // A spread, because Solid's lint reads onClick beside on:click as a duplicate.
  const rowAction = {
    get onClick() {
      return local.onClick ? runRowAction : undefined;
    },
  };
  return (
    <tr
      class={`group transition-colors duration-150 hover:bg-surface-hover ${local.class || ''}`}
      on:click={local.onClick ? nativeRowClickTarget : undefined}
      on:mousedown={local.onClick ? selectionGuard.onMouseDown : undefined}
      {...rowAction}
      {...rest}
    >
      {local.children}
    </tr>
  );
}

// Tailwind emits every padding utility at the same specificity, ordered by the
// spacing scale, so a caller's `px-1` loses to the base `px-2` no matter where
// it sits in the class string, and a caller's `px-3` only wins by luck. Like
// TableHeader's border and TableBody's divider, a side whose padding the caller
// names belongs to the caller: the base padding for that side is left out.
// Prefixed variants (`lg:px-0`) layer on top of the base the way they always
// did, so they do not count as owning the side.
type TableAxisPadding = { both: string; start: string; end: string };

const TABLE_HORIZONTAL_PADDING: TableAxisPadding = {
  both: 'px-2 sm:px-3',
  start: 'pl-2 sm:pl-3',
  end: 'pr-2 sm:pr-3',
};

const TABLE_HEAD_VERTICAL_PADDING: TableAxisPadding = {
  both: 'py-1.5',
  start: 'pt-1.5',
  end: 'pb-1.5',
};

const TABLE_CELL_VERTICAL_PADDING: TableAxisPadding = {
  both: 'py-0.5',
  start: 'pt-0.5',
  end: 'pb-0.5',
};

const OWNS_PADDING_LEFT = /(?:^|\s)!?(?:p|px|pl|ps)-/;
const OWNS_PADDING_RIGHT = /(?:^|\s)!?(?:p|px|pr|pe)-/;
const OWNS_PADDING_TOP = /(?:^|\s)!?(?:p|py|pt)-/;
const OWNS_PADDING_BOTTOM = /(?:^|\s)!?(?:p|py|pb)-/;

const resolveAxisPadding = (
  axis: TableAxisPadding,
  callerOwnsStart: boolean,
  callerOwnsEnd: boolean,
): string => {
  if (callerOwnsStart) return callerOwnsEnd ? '' : axis.end;
  return callerOwnsEnd ? axis.start : axis.both;
};

const resolveTablePaddingClass = (vertical: TableAxisPadding, customClass?: string): string => {
  const custom = customClass ?? '';
  return [
    resolveAxisPadding(
      TABLE_HORIZONTAL_PADDING,
      OWNS_PADDING_LEFT.test(custom),
      OWNS_PADDING_RIGHT.test(custom),
    ),
    resolveAxisPadding(vertical, OWNS_PADDING_TOP.test(custom), OWNS_PADDING_BOTTOM.test(custom)),
  ]
    .filter(Boolean)
    .join(' ');
};

export type TableHeadProps = JSX.HTMLAttributes<HTMLTableCellElement> & {
  colSpan?: number;
  colspan?: number;
  width?: string | number;
};

export function TableHead(props: TableHeadProps) {
  const [local, rest] = splitProps(props, ['class', 'children']);
  return (
    <th
      class={`${resolveTablePaddingClass(TABLE_HEAD_VERTICAL_PADDING, local.class)} text-[11px] sm:text-xs font-semibold uppercase tracking-wider align-middle ${local.class || ''}`.trim()}
      {...rest}
    >
      {local.children}
    </th>
  );
}

export type TableCellProps = JSX.HTMLAttributes<HTMLTableCellElement> & {
  colSpan?: number;
  colspan?: number;
  height?: string | number;
  width?: string | number;
};

export function TableCell(props: TableCellProps) {
  const [local, rest] = splitProps(props, ['class', 'children']);
  return (
    <td
      class={`${resolveTablePaddingClass(TABLE_CELL_VERTICAL_PADDING, local.class)} align-middle ${local.class || ''}`.trim()}
      {...rest}
    >
      {local.children}
    </td>
  );
}
