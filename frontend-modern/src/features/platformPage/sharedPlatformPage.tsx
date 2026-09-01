import { A } from '@solidjs/router';
import ChevronDownIcon from 'lucide-solid/icons/chevron-down';
import ChevronLeftIcon from 'lucide-solid/icons/chevron-left';
import ChevronRightIcon from 'lucide-solid/icons/chevron-right';
import ChevronUpIcon from 'lucide-solid/icons/chevron-up';
import RotateCcwIcon from 'lucide-solid/icons/rotate-ccw';
import TriangleAlertIcon from 'lucide-solid/icons/triangle-alert';
import {
  For,
  Show,
  createEffect,
  createMemo,
  createSignal,
  onCleanup,
  onMount,
  type Accessor,
  type Component,
  type JSX,
} from 'solid-js';
import { EmptyState } from '@/components/shared/EmptyState';
import { type FilterOption as PlatformTableFilterOption } from '@/components/shared/FilterButtonGroup';
import { FilterBar, filterChipStatusDot, type FilterDef } from '@/components/shared/FilterBar';
import { FilterSegmentedControl } from '@/components/shared/FilterToolbar';
import { type SearchInputProps, type SearchInputSuggestion } from '@/components/shared/SearchInput';
import {
  Table,
  TableBody,
  TableHead,
  TableHeader,
  TableRow,
  type TableProps,
} from '@/components/shared/Table';
import { getTableSortIndicator } from '@/components/shared/tableSortPresentation';
import { TableCard } from '@/components/shared/TableCard';
import { TableCardHeader } from '@/components/shared/TableCardHeader';
import { useActiveHorizontalRailItemVisibility } from '@/components/shared/useActiveHorizontalRailItemVisibility';
import { useBreakpoint } from '@/hooks/useBreakpoint';
import { usePersistentSignal } from '@/hooks/usePersistentSignal';
import { UnifiedResourceTable } from '@/components/Infrastructure/UnifiedResourceTable';
import type { Resource } from '@/types/resource';
import { formatBytes, formatRelativeTime, formatUptime } from '@/utils/format';
import {
  matchesSearchTermSplit,
  splitSearchExclusions,
  type SearchTermSplit,
} from '@/utils/searchQuery';
import { asTrimmedString } from '@/utils/stringUtils';
import { formatVmwareClusterServices } from '@/utils/vmwareDisplay';
import { getPlatformColumnAlign, type PlatformTableColumnKind } from './columnAlignment';
import {
  buildPlatformResourceSearchSuggestions,
  isPlatformSearchSuggestionResource,
} from './platformSearchSuggestions';
import {
  PLATFORM_ESTATE_COUNTS_STORAGE_KEY,
  deserializePlatformEstateCountsVisibility,
} from './platformEstateOverviewModel';

export type { PlatformTableFilterOption };
export { PlatformWindowedRows } from './PlatformWindowedRows';
export { PlatformWindowedList } from './PlatformWindowedList';

// Decorates every status option with the row count the table would show if
// that option were selected. Callers pass the same predicate the table's
// filter applies (createPlatformTableFilterState exposes it as
// countForStatus), so a chip's number can never disagree with the rows that
// clicking it renders. The optional attention entry escalates that option's
// tone while it has matches.
export function withPlatformStatusCounts<T extends string | number>(
  options: readonly PlatformTableFilterOption<T>[],
  countForStatus: (value: T) => number,
  attention?: { value: T; tone: 'warning' | 'danger' },
): PlatformTableFilterOption<T>[] {
  return options.map((option) => {
    const raw = countForStatus(option.value);
    const count = Number.isFinite(raw) ? Math.max(0, Math.trunc(raw)) : 0;
    const escalate = attention !== undefined && option.value === attention.value && count > 0;
    return {
      ...option,
      count,
      ...(escalate ? { tone: attention.tone } : {}),
    };
  });
}

export type PlatformTabSpec<TabId extends string> = {
  id: TabId;
  label: string;
  path: string;
};

export function PlatformSectionTabs<TabId extends string>(props: {
  tabs: readonly PlatformTabSpec<TabId>[];
  active: TabId;
  ariaLabel: string;
}) {
  let tabListRef: HTMLElement | undefined;
  const [hasOverflow, setHasOverflow] = createSignal(false);
  const [canScrollLeft, setCanScrollLeft] = createSignal(false);
  const [canScrollRight, setCanScrollRight] = createSignal(false);

  const updateScrollControls = () => {
    const rail = tabListRef;
    if (!rail) return;
    const maxScrollLeft = Math.max(0, rail.scrollWidth - rail.clientWidth);
    setHasOverflow(maxScrollLeft > 1);
    setCanScrollLeft(rail.scrollLeft > 1);
    setCanScrollRight(rail.scrollLeft < maxScrollLeft - 1);
  };

  const activeItemVisibility = useActiveHorizontalRailItemVisibility({
    active: () => props.active,
    rail: () => tabListRef,
    // The phone-only arrow overlays occupy 40px at either rail edge. Keep
    // selected labels beyond those controls, with the normal 8px breathing
    // room, whenever route navigation brings a tab into view.
    edgePadding: 48,
  });

  createEffect(() => {
    void props.active;
    queueMicrotask(updateScrollControls);
  });

  onMount(() => {
    const rail = tabListRef;
    if (!rail) return;

    rail.addEventListener('scroll', updateScrollControls, { passive: true });
    window.addEventListener('resize', updateScrollControls, { passive: true });
    const resizeObserver =
      typeof ResizeObserver === 'function' ? new ResizeObserver(updateScrollControls) : undefined;
    resizeObserver?.observe(rail);
    updateScrollControls();

    onCleanup(() => {
      rail.removeEventListener('scroll', updateScrollControls);
      window.removeEventListener('resize', updateScrollControls);
      resizeObserver?.disconnect();
    });
  });

  const scrollSections = (direction: -1 | 1) => {
    const rail = tabListRef;
    if (!rail) return;
    activeItemVisibility.markManualScrollIntent();
    rail.scrollBy({
      left: direction * Math.max(160, Math.round(rail.clientWidth * 0.7)),
      behavior: 'smooth',
    });
  };

  return (
    <Show when={props.tabs.length > 1}>
      <div class="relative min-w-0 border-b border-border">
        <nav
          ref={(element) => {
            tabListRef = element;
          }}
          class={`touch-scroll flex min-w-0 items-center gap-1 overflow-x-auto pl-0 overscroll-x-contain scrollbar-hide sm:pr-0 ${
            hasOverflow() ? 'pr-10' : 'pr-0'
          }`}
          aria-label={props.ariaLabel}
        >
          <For each={props.tabs}>
            {(tab) => (
              <A
                href={tab.path}
                class={`inline-flex min-h-10 shrink-0 select-none items-center whitespace-nowrap border-b-2 px-3 text-sm font-medium transition-colors ${
                  props.active === tab.id
                    ? 'border-blue-500 text-blue-600 dark:text-blue-300'
                    : 'border-transparent text-muted hover:border-border hover:text-base-content'
                }`}
                aria-current={props.active === tab.id ? 'page' : undefined}
              >
                {tab.label}
              </A>
            )}
          </For>
        </nav>

        <Show when={canScrollLeft()}>
          <button
            type="button"
            class="absolute inset-y-0 left-0 z-10 flex w-10 items-center justify-start bg-gradient-to-r from-surface via-surface to-transparent pl-1 text-muted hover:text-base-content focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-[-2px] focus-visible:outline-blue-500 sm:hidden"
            onClick={() => scrollSections(-1)}
            aria-label={`${props.ariaLabel}: scroll left`}
          >
            <span class="flex h-8 w-8 items-center justify-center rounded-full border border-border bg-surface shadow-sm">
              <ChevronLeftIcon class="h-4 w-4" aria-hidden="true" />
            </span>
          </button>
        </Show>
        <Show when={canScrollRight()}>
          <button
            type="button"
            class="absolute inset-y-0 right-0 z-10 flex w-10 items-center justify-end bg-gradient-to-l from-surface via-surface to-transparent pr-1 text-muted hover:text-base-content focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-[-2px] focus-visible:outline-blue-500 sm:hidden"
            onClick={() => scrollSections(1)}
            aria-label={`${props.ariaLabel}: scroll right`}
          >
            <span class="flex h-8 w-8 items-center justify-center rounded-full border border-border bg-surface shadow-sm">
              <ChevronRightIcon class="h-4 w-4" aria-hidden="true" />
            </span>
          </button>
        </Show>
      </div>
    </Show>
  );
}

export function PlatformTableEmptyState(props: {
  icon?: JSX.Element;
  title: string;
  description: string;
  actions?: JSX.Element;
}) {
  return (
    <TableCard>
      <div class="p-6">
        <EmptyState
          icon={props.icon}
          title={props.title}
          description={props.description}
          actions={props.actions}
        />
      </div>
    </TableCard>
  );
}

export function PlatformTableLoadingState(props: { title: string; description: string }) {
  return (
    <TableCard>
      <div class="px-3 py-2 text-xs text-muted" role="status">
        <span class="font-medium text-base-content">{props.title}</span>{' '}
        <span class="ml-2">{props.description}</span>
      </div>
    </TableCard>
  );
}

export type PlatformTableCellAlign = 'left' | 'right' | 'center';
export type PlatformTableContainerLayout =
  'compact' | 'basic' | 'operational' | 'expanded' | 'full';

export const getPlatformTableContainerLayout = (
  width: number,
  breakpoints: readonly [number, number, number, number],
): PlatformTableContainerLayout => {
  if (!Number.isFinite(width) || width < breakpoints[0]) return 'compact';
  if (width < breakpoints[1]) return 'basic';
  if (width < breakpoints[2]) return 'operational';
  if (width < breakpoints[3]) return 'expanded';
  return 'full';
};

export const PLATFORM_TABLE_CARD_CLASS = 'rounded-md';
export const PLATFORM_TABLE_HEADER_ROW_CLASS = 'bg-surface-alt text-muted border-b border-border';
export const PLATFORM_TABLE_BODY_CLASS = 'divide-y divide-border';
export const PLATFORM_TABLE_CLASS = 'platform-table';
export const PLATFORM_TABLE_SUMMARY_ROW_CLASS = 'h-8';
export const PLATFORM_TABLE_PHONE_IDENTITY_WIDTH_PERCENT = 30;
export const PLATFORM_TABLE_NARROW_IDENTITY_WIDTH_PERCENT = 40;
// Responsive platform tables already decide which columns remain useful at
// each breakpoint. A shared 48rem floor defeated that work by forcing a
// horizontal rail even when the visible columns could fit the real container.
// Dense tables that genuinely need a larger canvas must opt into an explicit
// base min-width; the default should use the space it actually receives.
export const PLATFORM_TABLE_DEFAULT_RESPONSIVE_MIN_WIDTH_CLASS = 'min-w-[0px]';

export function getPlatformTableResponsiveMinWidthClass(tableClass?: string): string {
  const hasExplicitBaseFloor = /(?:^|\s)min-w-\[[^\]]+\]/.test(tableClass ?? '');
  return hasExplicitBaseFloor ? '' : PLATFORM_TABLE_DEFAULT_RESPONSIVE_MIN_WIDTH_CLASS;
}

export function getPlatformTableClass(tableClass?: string): string {
  const featureClasses = (tableClass ?? '')
    .split(/\s+/)
    .filter(
      (className) =>
        className.length > 0 &&
        className !== 'min-w-full' &&
        // Viewport-responsive floors are especially harmful inside the app's
        // resizable content pane: they can activate while the actual table
        // container is much narrower. Keep explicit base floors for the few
        // genuinely dense tables, but let ordinary responsive tables fill
        // their real container and rely on their existing column priorities.
        !/^(?:sm:|md:|lg:|xl:|2xl:)min-w-\[[^\]]+\]$/.test(className),
    )
    .join(' ');

  return [PLATFORM_TABLE_CLASS, getPlatformTableResponsiveMinWidthClass(tableClass), featureClasses]
    .filter(Boolean)
    .join(' ');
}

// Cardless tables inside drawers and inline detail rows share the exact same
// header, body, density, overflow, and responsive class boundary as page
// tables. Domain-specific columns remain caller-owned; the table chrome does
// not fork merely because it is nested.
export function PlatformDetailTable(props: TableProps) {
  return (
    <Table {...props} class={getPlatformTableClass(props.class)} phoneVerticalScrollOwner="page" />
  );
}

export function PlatformDetailTableHeader(props: { children: JSX.Element; class?: string }) {
  return (
    <TableHeader>
      <TableRow class={`${PLATFORM_TABLE_HEADER_ROW_CLASS} ${props.class ?? ''}`.trim()}>
        {props.children}
      </TableRow>
    </TableHeader>
  );
}

export function PlatformDetailTableBody(props: { children: JSX.Element; class?: string }) {
  return (
    <TableBody class={`${PLATFORM_TABLE_BODY_CLASS} ${props.class ?? ''}`.trim()}>
      {props.children}
    </TableBody>
  );
}

export type PlatformTableShellProps = {
  title?: JSX.Element;
  actions?: JSX.Element;
  footer?: JSX.Element;
  tableClass?: string;
  tableWrapperClass?: string;
  cardClass?: string;
  colgroup?: JSX.Element;
  header: JSX.Element;
  body: JSX.Element;
};

export function PlatformTableShell(props: PlatformTableShellProps) {
  return (
    <TableCard class={props.cardClass ?? PLATFORM_TABLE_CARD_CLASS}>
      <TableCardHeader title={props.title} actions={props.actions} />
      <PlatformDetailTable class={props.tableClass} wrapperClass={props.tableWrapperClass}>
        {props.colgroup}
        <PlatformDetailTableHeader>{props.header}</PlatformDetailTableHeader>
        <PlatformDetailTableBody>{props.body}</PlatformDetailTableBody>
      </PlatformDetailTable>
      {props.footer}
    </TableCard>
  );
}

export function createPlatformTablePreview<Row>(options: {
  rows: Accessor<readonly Row[]>;
  limit: Accessor<number>;
}) {
  const [expanded, setExpanded] = createSignal(false);
  const limit = createMemo(() => Math.max(1, Math.trunc(options.limit())));
  const canExpand = createMemo(() => options.rows().length > limit());
  const visibleRows = createMemo<readonly Row[]>(() =>
    expanded() && canExpand() ? options.rows() : options.rows().slice(0, limit()),
  );

  createEffect(() => {
    if (!canExpand()) setExpanded(false);
  });

  return {
    expanded,
    canExpand,
    limit,
    visibleRows,
    setExpanded,
    toggle: () => {
      if (canExpand()) setExpanded((current) => !current);
    },
  };
}

export function PlatformTablePreviewFooter(props: {
  expanded: boolean;
  canExpand: boolean;
  total: number;
  visibleCount: number;
  noun: string;
  showCount?: boolean;
  onToggle: () => void;
}) {
  const collapsedLabel = () =>
    props.showCount === false ? `Show all ${props.noun}` : `Show all ${props.total} ${props.noun}`;
  const label = () => (props.expanded ? `Show fewer ${props.noun}` : collapsedLabel());
  const hiddenCount = () => Math.max(0, props.total - props.visibleCount);

  return (
    <Show when={props.canExpand}>
      <div
        class="relative z-10 border-t border-border bg-surface"
        data-platform-table-preview-footer
      >
        <Show when={!props.expanded}>
          <div
            class="pointer-events-none absolute inset-x-0 bottom-full h-9 bg-gradient-to-t from-surface via-surface/80 to-transparent"
            aria-hidden="true"
          />
        </Show>
        <button
          type="button"
          class="group inline-flex min-h-11 w-full items-center justify-center gap-2 px-3 py-2 text-[11px] font-semibold text-muted transition-colors hover:bg-surface-hover hover:text-base-content focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-[-2px] focus-visible:outline-blue-500"
          aria-expanded={props.expanded}
          aria-label={label()}
          onClick={props.onToggle}
        >
          <Show
            when={props.expanded}
            fallback={
              <ChevronDownIcon
                class="h-4 w-4 transition-transform group-hover:translate-y-0.5"
                aria-hidden="true"
              />
            }
          >
            <ChevronUpIcon
              class="h-4 w-4 transition-transform group-hover:-translate-y-0.5"
              aria-hidden="true"
            />
          </Show>
          <span>{label()}</span>
          <Show when={!props.expanded && props.showCount !== false}>
            <span class="text-[10px] font-medium text-muted">{hiddenCount()} more below</span>
          </Show>
        </button>
      </div>
    </Show>
  );
}

const getPlatformTableAlignClass = (align: PlatformTableCellAlign = 'left'): string => {
  if (align === 'right') return 'text-right';
  if (align === 'center') return 'text-center';
  return '';
};

export const getPlatformTableHeadClass = (align?: PlatformTableCellAlign): string =>
  `px-1.5 sm:px-2 py-0.5 font-medium ${getPlatformTableAlignClass(align)}`.trim();

export const getPlatformTableCellClass = (align?: PlatformTableCellAlign): string =>
  `px-1.5 sm:px-2 py-1 ${getPlatformTableAlignClass(align)}`.trim();

// Canonical kind-based wrappers. Tables should consume these instead of
// passing literal align strings, so every CPU/Memory/Disk/Storage header
// in the app lines up the same way (and any future column type can be
// added once in columnAlignment.ts and propagated automatically). See
// PlatformTableColumnKind for the kind list and rationale.
export const getPlatformTableHeadClassForKind = (kind: PlatformTableColumnKind): string =>
  `${getPlatformTableHeadClass(getPlatformColumnAlign(kind))} ${
    kind === 'name' ? 'platform-table-name-column' : ''
  }`.trim();

export const getPlatformTableCellClassForKind = (kind: PlatformTableColumnKind): string =>
  getPlatformTableCellClass(getPlatformColumnAlign(kind));

// Summary rows stay on one 32px scanning rhythm across every provider. Detail
// rows are deliberately excluded at the shared CSS boundary so their content
// can expand without weakening the overview-table contract.
export const getPlatformTableRowClass = (): string => PLATFORM_TABLE_SUMMARY_ROW_CLASS;

export function PlatformResponsiveTableLabel(props: { compact: string; full: string }) {
  return (
    <>
      <span class="platform-table-label-compact">{props.compact}</span>
      <span class="platform-table-label-full">{props.full}</span>
    </>
  );
}

// --- User-controlled column sorting ----------------------------------------
//
// Platform tables keep their built-in attention-first ordering until the user
// clicks a sortable header. Clicking cycles per column: first-click direction
// → flipped → back to the built-in order. The chosen column and direction
// persist per table via usePersistentSignal, so the preference survives
// reloads. Sorting only reorders rows, so it stays orthogonal to grouping: a
// grouped table re-buckets the sorted rows, which sorts rows within each
// group without changing the group order.

export type PlatformTableSortDirection = 'asc' | 'desc';

// null means "no value for this column" — those rows sink to the bottom in
// either direction so real values stay scannable.
export type PlatformTableSortValue = string | number | null;

const isEmptyPlatformTableSortValue = (value: PlatformTableSortValue): boolean =>
  value === null || (typeof value === 'number' && Number.isNaN(value));

const comparePlatformTableSortValues = (
  a: PlatformTableSortValue,
  b: PlatformTableSortValue,
): number => {
  const aEmpty = isEmptyPlatformTableSortValue(a);
  const bEmpty = isEmptyPlatformTableSortValue(b);
  if (aEmpty && bEmpty) return 0;
  if (aEmpty) return 1;
  if (bEmpty) return -1;
  if (typeof a === 'number' && typeof b === 'number') return a === b ? 0 : a < b ? -1 : 1;
  return String(a).localeCompare(String(b), undefined, { sensitivity: 'base' });
};

// Timestamp columns (Created / Started) sort numerically via this helper so
// string timestamps compare chronologically instead of lexically.
export const getPlatformTableDateTimeSortValue = (
  value: PlatformTableDateTimeValueInput,
): number | null => {
  const parsed = resolvePlatformTableDateTime(value);
  return parsed ? parsed.getTime() : null;
};

export type PlatformTableSortState<SortKey extends string> = {
  sortKey: () => SortKey | null;
  sortDirection: () => PlatformTableSortDirection;
  handleSort: (key: SortKey) => void;
  getAriaSort: (key: SortKey) => 'ascending' | 'descending' | undefined;
  sortRows: <Row>(
    rows: readonly Row[],
    getSortValue: (row: Row, key: SortKey) => PlatformTableSortValue,
  ) => readonly Row[];
};

export function createPlatformTableSortState<SortKey extends string>(options: {
  // Storage namespace, e.g. 'dockerContainers' → dockerContainersSortKey /
  // dockerContainersSortDirection in localStorage.
  storageKey: string;
  sortKeys: readonly SortKey[];
  // Columns whose first click sorts descending — metrics and counts read
  // "biggest on top". Everything else starts ascending.
  descendingFirst?: readonly SortKey[];
}): PlatformTableSortState<SortKey> {
  const isSortKey = (value: string): value is SortKey =>
    (options.sortKeys as readonly string[]).includes(value);
  const [sortKey, setSortKey] = usePersistentSignal<SortKey | null>(
    `${options.storageKey}SortKey`,
    null,
    // A persisted key for a renamed or removed column falls back to the
    // table's built-in order instead of throwing the sort into limbo.
    { deserialize: (raw) => (isSortKey(raw) ? raw : null) },
  );
  const [sortDirection, setSortDirection] = usePersistentSignal<PlatformTableSortDirection>(
    `${options.storageKey}SortDirection`,
    'asc',
    { deserialize: (raw) => (raw === 'desc' ? 'desc' : 'asc') },
  );
  const descendingFirst = new Set<SortKey>(options.descendingFirst ?? []);
  const firstClickDirection = (key: SortKey): PlatformTableSortDirection =>
    descendingFirst.has(key) ? 'desc' : 'asc';

  const handleSort = (key: SortKey) => {
    if (sortKey() === key) {
      if (sortDirection() === firstClickDirection(key)) {
        setSortDirection(firstClickDirection(key) === 'asc' ? 'desc' : 'asc');
      } else {
        setSortKey(null);
        setSortDirection('asc');
      }
      return;
    }
    setSortKey(() => key);
    setSortDirection(firstClickDirection(key));
  };

  const getAriaSort = (key: SortKey): 'ascending' | 'descending' | undefined => {
    if (sortKey() !== key) return undefined;
    return sortDirection() === 'asc' ? 'ascending' : 'descending';
  };

  const sortRows = <Row,>(
    rows: readonly Row[],
    getSortValue: (row: Row, key: SortKey) => PlatformTableSortValue,
  ): readonly Row[] => {
    const key = sortKey();
    if (!key) return rows;
    const dir = sortDirection() === 'asc' ? 1 : -1;
    const decorated = rows.map((row) => [getSortValue(row, key), row] as const);
    decorated.sort(([a], [b]) => {
      // Missing values stay last in either direction.
      if (isEmptyPlatformTableSortValue(a) || isEmptyPlatformTableSortValue(b)) {
        return comparePlatformTableSortValues(a, b);
      }
      return comparePlatformTableSortValues(a, b) * dir;
    });
    return decorated.map(([, row]) => row);
  };

  return { sortKey, sortDirection, handleSort, getAriaSort, sortRows };
}

export function PlatformSortableTableHead<SortKey extends string>(props: {
  kind: PlatformTableColumnKind;
  sort: PlatformTableSortState<SortKey>;
  // Omit to render a non-sortable header with the same canonical alignment.
  sortKey?: SortKey;
  class?: string;
  children: JSX.Element;
}) {
  const isSorted = () => props.sortKey !== undefined && props.sort.sortKey() === props.sortKey;
  const handleClick = () => {
    const key = props.sortKey;
    if (key !== undefined) props.sort.handleSort(key);
  };
  return (
    <TableHead
      class={`${getPlatformTableHeadClassForKind(props.kind)} ${
        props.sortKey !== undefined ? 'cursor-pointer select-none hover:bg-surface-hover' : ''
      } ${props.class ?? ''}`
        .replace(/\s+/g, ' ')
        .trim()}
      aria-sort={props.sortKey !== undefined ? props.sort.getAriaSort(props.sortKey) : undefined}
      onClick={handleClick}
    >
      {props.children}
      {isSorted() ? ` ${getTableSortIndicator(true, props.sort.sortDirection())}` : null}
    </TableHead>
  );
}

export const formatPlatformTableTextValue = (value: unknown, emptyText = '—'): string =>
  asTrimmedString(value) || emptyText;

export type PlatformTableValueSummary = { label: string; title: string; values: string[] };

export type PlatformTableValueSummaryOptions = {
  emptyText?: string;
  maxVisible?: number;
  transform?: (value: string) => string;
};

export const summarizePlatformTableValues = (
  values: readonly unknown[] | undefined,
  options: PlatformTableValueSummaryOptions = {},
): PlatformTableValueSummary => {
  const emptyText = options.emptyText ?? '—';
  const maxVisible = options.maxVisible ?? 2;
  const normalized = (values ?? [])
    .map((value) => asTrimmedString(value))
    .filter((value): value is string => Boolean(value))
    .map((value) => options.transform?.(value) ?? value);

  if (normalized.length === 0) return { label: emptyText, title: '', values: [] };

  const visible = normalized.slice(0, maxVisible);
  const suffix =
    normalized.length > visible.length ? ` +${normalized.length - visible.length}` : '';
  return {
    label: `${visible.join(', ')}${suffix}`,
    title: normalized.join(', '),
    values: normalized,
  };
};

export const formatPlatformTableTitleCaseValue = (
  value: unknown,
  emptyText = 'Unknown',
): string => {
  const normalized = asTrimmedString(value);
  if (!normalized) return emptyText;
  return normalized.charAt(0).toUpperCase() + normalized.slice(1).toLowerCase();
};

export type PlatformTableUptimeValueOptions = {
  compact?: boolean;
  emptyText?: string;
};

export const formatPlatformTableUptimeValue = (
  seconds: number | undefined,
  emptyTextOrOptions: string | PlatformTableUptimeValueOptions = '—',
): string => {
  const options =
    typeof emptyTextOrOptions === 'string'
      ? { emptyText: emptyTextOrOptions, compact: true }
      : { emptyText: '—', compact: true, ...emptyTextOrOptions };
  if (typeof seconds !== 'number' || !Number.isFinite(seconds) || seconds <= 0) {
    return options.emptyText;
  }
  return formatUptime(seconds, options.compact);
};

export const formatPlatformTableBytesValue = (
  bytes: number | undefined,
  emptyText = '—',
): string => {
  if (typeof bytes !== 'number' || !Number.isFinite(bytes) || bytes <= 0) {
    return emptyText;
  }
  return formatBytes(bytes);
};

export const PLATFORM_TABLE_COMPACT_DATE_TIME_FORMAT: Intl.DateTimeFormatOptions = {
  month: 'short',
  day: 'numeric',
  hour: '2-digit',
  minute: '2-digit',
};

export type PlatformTableDateTimeValueInput = string | number | Date | null | undefined;

export type PlatformTableDateTimeValueOptions = {
  emptyText?: string;
  dateTimeFormat?: Intl.DateTimeFormatOptions;
  minYear?: number;
};

const resolvePlatformTableDateTime = (value: PlatformTableDateTimeValueInput): Date | undefined => {
  if (value == null) return undefined;
  if (value instanceof Date) return Number.isNaN(value.getTime()) ? undefined : value;

  const raw = typeof value === 'string' ? value.trim() : value;
  if (raw === '') return undefined;

  const parsed = new Date(raw);
  return Number.isNaN(parsed.getTime()) ? undefined : parsed;
};

export const formatPlatformTableDateTimeValue = (
  value: PlatformTableDateTimeValueInput,
  options: PlatformTableDateTimeValueOptions = {},
): string => {
  const emptyText = options.emptyText ?? '—';
  const parsed = resolvePlatformTableDateTime(value);
  if (!parsed) return emptyText;
  if (options.minYear !== undefined && parsed.getUTCFullYear() < options.minYear) {
    return emptyText;
  }
  return parsed.toLocaleString(undefined, {
    ...PLATFORM_TABLE_COMPACT_DATE_TIME_FORMAT,
    ...options.dateTimeFormat,
  });
};

export function PlatformTableDateTimeValue(props: {
  value: PlatformTableDateTimeValueInput;
  emptyText?: string;
  dateTimeFormat?: Intl.DateTimeFormatOptions;
  minYear?: number;
}) {
  const options = (): PlatformTableDateTimeValueOptions => {
    const resolved: PlatformTableDateTimeValueOptions = {};
    if (props.emptyText !== undefined) resolved.emptyText = props.emptyText;
    if (props.dateTimeFormat !== undefined) resolved.dateTimeFormat = props.dateTimeFormat;
    if (props.minYear !== undefined) resolved.minYear = props.minYear;
    return resolved;
  };

  return (
    <span class="tabular-nums">{formatPlatformTableDateTimeValue(props.value, options())}</span>
  );
}

export type PlatformTableRelativeTimeValueInput = number | string | Date | null | undefined;

export type PlatformTableRelativeTimeValueOptions = {
  compact?: boolean;
  emptyText?: string;
};

export const formatPlatformTableRelativeTimeValue = (
  value: PlatformTableRelativeTimeValueInput,
  options: PlatformTableRelativeTimeValueOptions = {},
): string => {
  const emptyText = options.emptyText ?? '—';
  if (value == null || value === '') return emptyText;
  return (
    formatRelativeTime(value, {
      compact: options.compact ?? true,
      emptyText,
    }) || emptyText
  );
};

export function PlatformTableRelativeTimeValue(props: {
  value: PlatformTableRelativeTimeValueInput;
  compact?: boolean;
  emptyText?: string;
}) {
  const options = (): PlatformTableRelativeTimeValueOptions => {
    const resolved: PlatformTableRelativeTimeValueOptions = {};
    if (props.compact !== undefined) resolved.compact = props.compact;
    if (props.emptyText !== undefined) resolved.emptyText = props.emptyText;
    return resolved;
  };

  return (
    <span class="tabular-nums">{formatPlatformTableRelativeTimeValue(props.value, options())}</span>
  );
}

export type PlatformTableDurationValueOptions = {
  emptyText?: string;
  fallbackText?: string;
};

export const formatPlatformTableDurationValue = (
  seconds: number | undefined,
  options: PlatformTableDurationValueOptions = {},
): string => {
  const explicit = options.fallbackText?.trim();
  if (explicit) return explicit;
  const emptyText = options.emptyText ?? '—';
  if (typeof seconds !== 'number' || !Number.isFinite(seconds) || seconds <= 0) return emptyText;

  const wholeSeconds = Math.max(0, Math.round(seconds));
  if (wholeSeconds < 60) return `${wholeSeconds}s`;

  const totalMinutes = Math.floor(wholeSeconds / 60);
  const remainingSeconds = wholeSeconds % 60;
  if (totalMinutes < 60) {
    return remainingSeconds > 0 ? `${totalMinutes}m ${remainingSeconds}s` : `${totalMinutes}m`;
  }

  const hours = Math.floor(totalMinutes / 60);
  const remainingMinutes = totalMinutes % 60;
  return `${hours}h ${remainingMinutes}m`;
};

export function PlatformTableDurationValue(props: {
  seconds: number | undefined;
  emptyText?: string;
  fallbackText?: string;
}) {
  const options = (): PlatformTableDurationValueOptions => {
    const resolved: PlatformTableDurationValueOptions = {};
    if (props.emptyText !== undefined) resolved.emptyText = props.emptyText;
    if (props.fallbackText !== undefined) resolved.fallbackText = props.fallbackText;
    return resolved;
  };

  return (
    <span class="tabular-nums">{formatPlatformTableDurationValue(props.seconds, options())}</span>
  );
}

const formatPlatformTableWidthPercentage = (value: number): string =>
  `${Number(value.toFixed(4))}%`;

export const getPlatformTableWeightedColumnWidthStyle = <ColumnId extends string>(
  columnId: ColumnId,
  weights: Partial<Record<ColumnId, number>>,
  visibleColumnIds: readonly ColumnId[],
  anchor?: {
    columnId: ColumnId;
    widthPercent: number;
  },
): JSX.CSSProperties => {
  const columnWeight = weights[columnId] ?? 0;
  const anchorIsVisible = anchor !== undefined && visibleColumnIds.includes(anchor.columnId);
  if (anchorIsVisible) {
    const anchorWidth = Math.min(100, Math.max(0, anchor.widthPercent));
    if (columnId === anchor.columnId) {
      return { width: formatPlatformTableWidthPercentage(anchorWidth) };
    }

    const remainingWeight = visibleColumnIds.reduce(
      (total, id) => total + (id === anchor.columnId ? 0 : (weights[id] ?? 0)),
      0,
    );
    const width = remainingWeight > 0 ? (columnWeight / remainingWeight) * (100 - anchorWidth) : 0;
    return { width: formatPlatformTableWidthPercentage(width) };
  }

  const totalWeight = visibleColumnIds.reduce((total, id) => total + (weights[id] ?? 0), 0);
  const width = totalWeight > 0 ? (columnWeight / totalWeight) * 100 : 0;

  return { width: formatPlatformTableWidthPercentage(width) };
};

const platformTableIntegerFormatter = new Intl.NumberFormat(undefined, {
  maximumFractionDigits: 0,
});

export const formatPlatformTableIntegerValue = (
  value: number | null | undefined,
  emptyText = '—',
): string => {
  if (typeof value !== 'number' || !Number.isFinite(value)) {
    return emptyText;
  }
  return platformTableIntegerFormatter.format(Math.round(value));
};

export function PlatformTableNumberValue(props: {
  value: number | undefined;
  emptyText?: string;
  format?: (value: number) => string | number;
}) {
  const label = () => {
    const value = props.value;
    if (typeof value !== 'number' || !Number.isFinite(value)) {
      return props.emptyText ?? '—';
    }
    return props.format?.(value) ?? value;
  };

  return <span class="tabular-nums">{label()}</span>;
}

const resolvePlatformTableCountRatioParts = (
  current: number | undefined,
  total: number | undefined,
): { current: number; total: number } | undefined => {
  const currentValue =
    typeof current === 'number' && Number.isFinite(current) ? current : undefined;
  const totalValue = typeof total === 'number' && Number.isFinite(total) ? total : undefined;
  if (currentValue === undefined && totalValue === undefined) return undefined;
  const resolvedCurrent = currentValue ?? 0;
  return {
    current: resolvedCurrent,
    total: totalValue ?? resolvedCurrent,
  };
};

export function formatPlatformTableCountRatioValue(
  current: number | undefined,
  total: number | undefined,
  options: { emptyText?: string; suffix?: string } = {},
): string {
  const ratio = resolvePlatformTableCountRatioParts(current, total);
  if (!ratio) return options.emptyText ?? '—';
  const suffix = options.suffix ? ` ${options.suffix}` : '';
  return `${ratio.current}/${ratio.total}${suffix}`;
}

export function PlatformTableCountRatioValue(props: {
  current: number | undefined;
  total: number | undefined;
  currentTone?: 'warning';
  emptyText?: string;
  suffix?: string;
}) {
  const ratio = () => resolvePlatformTableCountRatioParts(props.current, props.total);
  const currentClass = () =>
    props.currentTone === 'warning' ? 'text-amber-700 dark:text-amber-300' : '';

  return (
    <Show
      when={ratio()}
      fallback={<PlatformTableNumberValue value={undefined} emptyText={props.emptyText} />}
    >
      {(resolved) => (
        <span class="inline-flex items-baseline whitespace-nowrap">
          <span class={currentClass()}>
            <PlatformTableNumberValue value={resolved().current} emptyText={props.emptyText} />
          </span>
          <span class="text-muted">/</span>
          <span class="text-muted">
            <PlatformTableNumberValue value={resolved().total} emptyText={props.emptyText} />
          </span>
          <Show when={props.suffix}>
            {(suffix) => <span class="ml-1 text-muted"> {suffix()}</span>}
          </Show>
        </span>
      )}
    </Show>
  );
}

export type PlatformTablePercentValueOptions = {
  emptyText?: string;
  normalizeRatio?: boolean;
  clamp?: boolean;
};

export const formatPlatformTablePercentValue = (
  value: number | null | undefined,
  options: PlatformTablePercentValueOptions = {},
): string => {
  if (typeof value !== 'number' || !Number.isFinite(value)) {
    return options.emptyText ?? '—';
  }
  const normalized = options.normalizeRatio && value <= 1 ? value * 100 : value;
  const clamped = options.clamp ? Math.max(0, Math.min(100, normalized)) : normalized;
  return `${clamped.toFixed(1)}%`;
};

const formatOneDecimalCelsius = (value: number): string => `${value.toFixed(1)}°C`;

export function PlatformTablePercentValue(props: {
  value: number | null | undefined;
  emptyText?: string;
  normalizeRatio?: boolean;
  clamp?: boolean;
}) {
  const finiteValue = () =>
    typeof props.value === 'number' && Number.isFinite(props.value) ? props.value : undefined;

  return (
    <PlatformTableNumberValue
      value={finiteValue()}
      emptyText={props.emptyText}
      format={(value) =>
        formatPlatformTablePercentValue(value, {
          emptyText: props.emptyText,
          normalizeRatio: props.normalizeRatio,
          clamp: props.clamp,
        })
      }
    />
  );
}

export function PlatformTableTemperatureValue(props: {
  value: number | null | undefined;
  emptyText?: string;
}) {
  const finitePositiveValue = () =>
    typeof props.value === 'number' && Number.isFinite(props.value) && props.value > 0
      ? props.value
      : undefined;

  return (
    <PlatformTableNumberValue
      value={finitePositiveValue()}
      emptyText={props.emptyText}
      format={formatOneDecimalCelsius}
    />
  );
}

export const getPlatformTableFiniteMetric = (value: number | undefined): number | undefined =>
  typeof value === 'number' && Number.isFinite(value) ? value : undefined;

export function PlatformTableMetricFallback(props: { label?: string; title?: string } = {}) {
  const label = () => asTrimmedString(props.label);
  const title = () => asTrimmedString(props.title);

  return (
    <div class="flex justify-center">
      <span
        class={label() ? 'text-[9px] font-medium text-muted' : 'text-xs text-muted'}
        title={title() || undefined}
        aria-label={title() || label() || undefined}
        aria-hidden={label() ? undefined : 'true'}
      >
        {label() || '—'}
      </span>
    </div>
  );
}

export function PlatformErrorState(props: {
  title: string;
  description: string;
  onRefresh: () => void;
}) {
  return (
    <TableCard>
      <div class="p-6">
        <EmptyState
          icon={<TriangleAlertIcon class="h-6 w-6 text-slate-400" />}
          title={props.title}
          description={props.description}
          actions={
            <button
              type="button"
              onClick={props.onRefresh}
              class="inline-flex min-h-10 items-center rounded-md border border-border px-3 py-2 text-sm font-medium hover:bg-surface-hover"
            >
              Refresh
            </button>
          }
        />
      </div>
    </TableCard>
  );
}

// Status filter applied client-side by the platform-page toolbar. Mirrors
// the v5 dashboard/storage status segmented control: All / Online (running)
// / Degraded / Offline (stopped). Resource statuses are normalized through
// `mapResourceStatusToTriad` so per-platform vocabulary differences (e.g.
// 'running' vs 'online', 'stopped' vs 'offline') collapse to one chip set.
export type PlatformResourceStatusFilter = 'all' | 'online' | 'degraded' | 'offline';

const statusDot = filterChipStatusDot;

export const PLATFORM_STATUS_FILTER_OPTIONS: PlatformTableFilterOption<PlatformResourceStatusFilter>[] =
  [
    { value: 'all', label: 'All' },
    { value: 'online', label: 'Online', tone: 'success', leading: statusDot('bg-emerald-500') },
    { value: 'degraded', label: 'Degraded', tone: 'warning', leading: statusDot('bg-amber-500') },
    { value: 'offline', label: 'Offline', tone: 'danger', leading: statusDot('bg-red-500') },
  ];

export const PLATFORM_HEALTH_FILTER_OPTIONS: PlatformTableFilterOption<PlatformResourceStatusFilter>[] =
  [
    { value: 'all', label: 'All' },
    { value: 'online', label: 'Healthy', tone: 'success', leading: statusDot('bg-emerald-500') },
    { value: 'degraded', label: 'Degraded', tone: 'warning', leading: statusDot('bg-amber-500') },
    { value: 'offline', label: 'Offline', tone: 'danger', leading: statusDot('bg-red-500') },
  ];

const ONLINE_STATUSES = new Set<string>(['online', 'running']);
const OFFLINE_STATUSES = new Set<string>(['offline', 'stopped']);
const DEGRADED_STATUSES = new Set<string>(['degraded', 'warning', 'paused']);

export const normalizePlatformResourceStatusFilter = (
  value: string | string[] | null | undefined,
): PlatformResourceStatusFilter => {
  const normalized = (Array.isArray(value) ? value[0] : value)?.trim().toLowerCase();
  if (!normalized || normalized === 'all') return 'all';
  if (ONLINE_STATUSES.has(normalized) || normalized === 'healthy') return 'online';
  if (DEGRADED_STATUSES.has(normalized) || normalized === 'attention') return 'degraded';
  if (OFFLINE_STATUSES.has(normalized)) return 'offline';
  return 'all';
};

const mapResourceStatusToTriad = (
  status: string | undefined,
): Exclude<PlatformResourceStatusFilter, 'all'> | 'unknown' => {
  if (!status) return 'unknown';
  const normalized = status.trim().toLowerCase();
  if (ONLINE_STATUSES.has(normalized)) return 'online';
  if (DEGRADED_STATUSES.has(normalized)) return 'degraded';
  if (OFFLINE_STATUSES.has(normalized)) return 'offline';
  return 'unknown';
};

// Cross-platform fallback haystack used by tables that do not have a
// domain-specific search helper. Docker and Kubernetes provide their own
// platform-page filters (filterDockerResources / filterKubernetesResources)
// that already cover docker.* and kubernetes.* fields, so this helper stays
// platform-agnostic and only knows about the generic Resource surface plus
// the small number of provider blocks that still consume it directly
// (Proxmox Mail Gateway, vSphere hosts table).
const matchesPlatformSearch = (resource: Resource, split: SearchTermSplit): boolean => {
  if (!split.needle && split.excludes.length === 0) return true;
  const haystack = [
    resource.name,
    resource.displayName,
    resource.id,
    resource.parentName,
    resource.platformId,
    resource.platformType,
    resource.agent?.hostname,
    resource.identity?.hostname,
    resource.canonicalIdentity?.displayName,
    resource.canonicalIdentity?.hostname,
    resource.canonicalIdentity?.primaryId,
    ...(resource.canonicalIdentity?.aliases ?? []),
    resource.pmg?.hostname,
    resource.pmg?.version,
    resource.vmware?.connectionName,
    resource.vmware?.vcenterHost,
    resource.vmware?.runtimeHostName,
    resource.vmware?.clusterName,
    formatVmwareClusterServices(resource.vmware),
    resource.vmware?.datastoreNames?.join(' '),
    resource.vmware?.networkType,
    resource.vmware?.networkHostNames?.join(' '),
    resource.vmware?.networkVmNames?.join(' '),
    ...(resource.tags ?? []),
  ]
    .filter((value): value is string => typeof value === 'string')
    .join(' ')
    .toLowerCase();
  return matchesSearchTermSplit(haystack, split);
};

export const filterPlatformResources = (
  resources: Resource[],
  search: string,
  status: PlatformResourceStatusFilter,
  resolveStatus: (resource: Resource) => string | undefined = (resource) => resource.status,
): Resource[] => {
  const split = splitSearchExclusions(search);
  const result: Resource[] = [];
  for (const resource of resources) {
    if (!matchesPlatformSearch(resource, split)) continue;
    if (status !== 'all') {
      const mapped = mapResourceStatusToTriad(resolveStatus(resource));
      if (mapped !== status) continue;
    }
    result.push(resource);
  }
  return result;
};

export function createPlatformTableFilterState<Row, Status extends string | number>(props: {
  resources: () => Row[];
  initialStatus: Status;
  filter: (resources: Row[], search: string, status: Status) => Row[];
  // When a page owns a shared toolbar that drives several stacked tables,
  // pass these accessors so each table reads from the shared state instead
  // of its own internal signals. Pass the setters too if the table state
  // itself is allowed to render or reset a controlled toolbar.
  externalSearch?: () => string;
  externalStatus?: () => Status;
  onExternalSearchChange?: (value: string) => void;
  onExternalStatusChange?: (value: Status) => void;
}) {
  const [internalSearch, setInternalSearch] = createSignal('');
  const [internalStatus, setInternalStatus] = createSignal<Status>(props.initialStatus);
  const search = () => props.externalSearch?.() ?? internalSearch();
  const status = () => props.externalStatus?.() ?? internalStatus();
  const setSearch = (value: string) => {
    if (props.onExternalSearchChange) {
      props.onExternalSearchChange(value);
      return;
    }
    setInternalSearch(value);
  };
  const setStatus = (value: Status) => {
    if (props.onExternalStatusChange) {
      props.onExternalStatusChange(value);
      return;
    }
    setInternalStatus(() => value);
  };
  const filtered = createMemo(() => props.filter(props.resources(), search(), status()));
  const visible = createMemo(() => filtered().length);
  const total = createMemo(() => props.resources().length);
  // Chip-count source for withPlatformStatusCounts: runs the table's own
  // predicate with the candidate status, so the count a chip shows is exactly
  // the row set selecting it would render.
  const countForStatus = (value: Status): number =>
    props.filter(props.resources(), search(), value).length;
  const hasActiveFilters = createMemo(
    () => search().trim().length > 0 || status() !== props.initialStatus,
  );
  const searchSuggestions = createMemo(() => {
    const resources: Resource[] = [];
    for (const row of props.resources()) {
      if (isPlatformSearchSuggestionResource(row)) resources.push(row);
    }
    return buildPlatformResourceSearchSuggestions(resources);
  });
  const resetFilters = () => {
    setSearch('');
    setStatus(props.initialStatus);
  };

  return {
    search,
    setSearch,
    status,
    setStatus,
    filtered,
    visible,
    total,
    countForStatus,
    hasActiveFilters,
    searchSuggestions,
    resetFilters,
  };
}

export const PlatformTableResetFiltersButton: Component<{
  onReset: () => void;
  label?: string;
}> = (props) => (
  <button
    type="button"
    onClick={props.onReset}
    class="inline-flex min-h-8 items-center justify-center gap-1.5 rounded-md border border-border bg-surface px-2.5 py-1 text-xs font-medium text-muted transition-colors hover:bg-surface-hover hover:text-base-content focus:outline-none focus-visible:ring-2 focus-visible:ring-blue-500/60"
    title={props.label ?? 'Reset filters'}
    aria-label={props.label ?? 'Reset filters'}
  >
    <RotateCcwIcon class="h-3.5 w-3.5" aria-hidden="true" />
    <span class="hidden sm:inline">{props.label ?? 'Reset filters'}</span>
  </button>
);

// Compact operator-facing counter shown at the right of the toolbar so
// users can read total / matching at a glance, mirroring v5's dense
// dashboard counters without spawning a card grid.
export const getPlatformResourceCountNoun = (rowNoun: string, count: number): string => {
  if (count !== 1) return rowNoun;
  const words = rowNoun.split(' ');
  const last = words.at(-1) ?? rowNoun;
  if (last.length > 1 && last.toLowerCase().endsWith('s')) {
    words[words.length - 1] = last.slice(0, -1);
  }
  return words.join(' ');
};

export const PlatformResourceCounter: Component<{
  visible: number;
  total: number;
  rowNoun: string;
}> = (props) => {
  const totalNoun = () => getPlatformResourceCountNoun(props.rowNoun, props.total);
  return (
    <span
      class="ml-auto whitespace-nowrap text-xs font-medium text-muted"
      role="status"
      aria-live="polite"
      aria-atomic="true"
    >
      <Show
        when={props.visible !== props.total}
        fallback={
          <>
            {props.total} {totalNoun()}
          </>
        }
      >
        {props.visible} of {props.total} {totalNoun()}
      </Show>
    </span>
  );
};

export function PlatformTableToolbar<T extends string | number>(props: {
  search: () => string;
  onSearchChange: (value: string) => void;
  searchPlaceholder: string;
  searchHistory?: SearchInputProps['history'];
  searchTips?: SearchInputProps['tips'];
  searchSuggestions?: () => readonly SearchInputSuggestion[];
  status: T;
  onStatusChange: (value: T) => void;
  statusOptions: PlatformTableFilterOption<T>[];
  visible: number;
  total: number;
  rowNoun: string;
  hasActiveFilters?: boolean;
  onResetFilters?: () => void;
  // Optional scope filters (host / node / namespace / pool ...) appended after
  // the status facet. Tables opt into
  // richer combinable filtering without bypassing the shared toolbar; the
  // status facet stays the inline segmented control and scope filters render as
  // chips behind "+ Filter".
  filters?: FilterDef[];
  /**
   * Compact consumer-owned context that belongs beside the table controls,
   * such as an attention count with a focused review action.
   */
  leadingControls?: JSX.Element;
  /**
   * Low-frequency table presentation controls. PlatformTableToolbar owns the
   * shared View popover so feature tables cannot accidentally render these as
   * permanent toolbar controls.
   */
  viewOptions?: JSX.Element;
}) {
  const { isMobile } = useBreakpoint();
  const [inventoryCountsVisible, setInventoryCountsVisible] = usePersistentSignal(
    PLATFORM_ESTATE_COUNTS_STORAGE_KEY,
    true,
    { deserialize: deserializePlatformEstateCountsVisibility },
  );

  // Migrated onto the shared FilterBar so every platform table inherits the
  // same combinable-filter UX (chip rail, saved-view scaffolding, mobile
  // collapse) instead of a bespoke search + segmented-status row. The public
  // prop surface is unchanged: search passes straight through and the single
  // status facet is modelled as an inline segmented control. Tables that want
  // additional scope filters opt in via the FilterBar directly.
  const allFilters: FilterDef[] = [
    {
      id: 'status',
      label: 'Status',
      group: 'status',
      inline: true,
      options: () =>
        props.statusOptions.map((option) => ({
          value: String(option.value),
          label: option.label,
          ariaLabel: option.ariaLabel,
          title: option.title,
          compactLabel: option.compactLabel,
          leading: option.leading,
          visualLabel: option.visualLabel,
          icon: option.icon,
          tone: option.tone,
          // The same estate-wide preference that gates the rows counter gates
          // the chip counts, so "Inventory totals: Hide" quiets both.
          count: inventoryCountsVisible() ? option.count : undefined,
        })),
      value: () => String(props.status),
      setValue: (value) => {
        const match = props.statusOptions.find((option) => String(option.value) === value);
        if (match) props.onStatusChange(match.value);
      },
      defaultValue: String(props.statusOptions[0]?.value ?? 'all'),
    },
    ...(props.filters ?? []),
  ];

  return (
    <FilterBar
      isMobile={isMobile}
      search={{
        value: props.search,
        setValue: props.onSearchChange,
        placeholder: props.searchPlaceholder,
        historyKey: props.searchHistory?.storageKey,
        emptyMessage: props.searchHistory?.emptyMessage,
        tips: props.searchTips,
        suggestions: props.searchSuggestions,
      }}
      filters={allFilters}
      showAddFilterLabel={false}
      leadingControls={props.leadingControls}
      viewOptions={
        <>
          {props.viewOptions}
          <div>
            <div class="mb-1.5 text-[10px] font-semibold uppercase tracking-wide text-muted">
              Inventory totals
            </div>
            <FilterSegmentedControl
              aria-label="Inventory totals visibility"
              value={inventoryCountsVisible() ? 'shown' : 'hidden'}
              onChange={(value) => setInventoryCountsVisible(value === 'shown')}
              options={[
                { value: 'shown', label: 'Show' },
                { value: 'hidden', label: 'Hide' },
              ]}
            />
          </div>
        </>
      }
      trailingControls={
        <Show when={inventoryCountsVisible()}>
          <PlatformResourceCounter
            visible={props.visible}
            total={props.total}
            rowNoun={props.rowNoun}
          />
        </Show>
      }
      showClearAll={() => Boolean(props.hasActiveFilters) || props.search().trim().length > 0}
      onClearAll={props.onResetFilters}
    />
  );
}

export const PlatformResourceTable: Component<{
  resources: Resource[];
  emptyIcon: JSX.Element;
  emptyTitle: string;
  emptyDescription: string;
  groupingMode?: 'grouped' | 'flat';
  searchPlaceholder?: string;
}> = (props) => {
  const [expandedResourceId, setExpandedResourceId] = createSignal<string | null>(null);
  const tableState = createPlatformTableFilterState({
    resources: () => props.resources,
    initialStatus: 'all' as PlatformResourceStatusFilter,
    filter: filterPlatformResources,
  });

  return (
    <Show
      when={props.resources.length > 0}
      fallback={
        <PlatformTableEmptyState
          icon={props.emptyIcon}
          title={props.emptyTitle}
          description={props.emptyDescription}
        />
      }
    >
      <div class="space-y-3">
        <PlatformTableToolbar
          search={tableState.search}
          onSearchChange={tableState.setSearch}
          searchPlaceholder={props.searchPlaceholder ?? 'Search rows'}
          searchSuggestions={tableState.searchSuggestions}
          status={tableState.status()}
          onStatusChange={tableState.setStatus}
          statusOptions={withPlatformStatusCounts(
            PLATFORM_STATUS_FILTER_OPTIONS,
            tableState.countForStatus,
          )}
          visible={tableState.visible()}
          total={tableState.total()}
          rowNoun="rows"
          hasActiveFilters={tableState.hasActiveFilters()}
          onResetFilters={tableState.resetFilters}
        />
        <Show
          when={tableState.filtered().length > 0}
          fallback={
            <PlatformTableEmptyState
              icon={props.emptyIcon}
              title="No rows match current filters"
              description="Adjust the search or status filter to see more rows."
              actions={<PlatformTableResetFiltersButton onReset={tableState.resetFilters} />}
            />
          }
        >
          <UnifiedResourceTable
            resources={tableState.filtered()}
            expandedResourceId={expandedResourceId()}
            onExpandedResourceChange={setExpandedResourceId}
            groupingMode={props.groupingMode ?? 'grouped'}
          />
        </Show>
      </div>
    </Show>
  );
};
