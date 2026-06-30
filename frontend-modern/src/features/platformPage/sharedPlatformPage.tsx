import { A } from '@solidjs/router';
import RotateCcwIcon from 'lucide-solid/icons/rotate-ccw';
import TriangleAlertIcon from 'lucide-solid/icons/triangle-alert';
import { For, Show, createMemo, createSignal, type Component, type JSX } from 'solid-js';
import { EmptyState } from '@/components/shared/EmptyState';
import { type FilterOption as PlatformTableFilterOption } from '@/components/shared/FilterButtonGroup';
import { FilterBar, filterChipStatusDot, type FilterDef } from '@/components/shared/FilterBar';
import { type SearchInputProps } from '@/components/shared/SearchInput';
import { Table, TableBody, TableHeader, TableRow } from '@/components/shared/Table';
import { TableCard } from '@/components/shared/TableCard';
import { TableCardHeader } from '@/components/shared/TableCardHeader';
import { useBreakpoint } from '@/hooks/useBreakpoint';
import { UnifiedResourceTable } from '@/components/Infrastructure/UnifiedResourceTable';
import type { Resource } from '@/types/resource';
import { formatBytes, formatRelativeTime, formatUptime } from '@/utils/format';
import { asTrimmedString } from '@/utils/stringUtils';
import { formatVmwareClusterServices } from '@/utils/vmwareDisplay';
import { getPlatformColumnAlign, type PlatformTableColumnKind } from './columnAlignment';

export type { PlatformTableFilterOption };

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
  return (
    <Show when={props.tabs.length > 1}>
      <nav
        class="flex flex-wrap items-center gap-1 border-b border-border"
        aria-label={props.ariaLabel}
      >
        <For each={props.tabs}>
          {(tab) => (
            <A
              href={tab.path}
              class={`inline-flex min-h-10 items-center border-b-2 px-3 text-sm font-medium transition-colors ${
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

export const PLATFORM_TABLE_CARD_CLASS = 'rounded-md';
export const PLATFORM_TABLE_HEADER_ROW_CLASS = 'bg-surface-alt text-muted border-b border-border';
export const PLATFORM_TABLE_BODY_CLASS = 'divide-y divide-border';

export type PlatformTableShellProps = {
  title?: JSX.Element;
  actions?: JSX.Element;
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
      <Table class={props.tableClass} wrapperClass={props.tableWrapperClass}>
        {props.colgroup}
        <TableHeader>
          <TableRow class={PLATFORM_TABLE_HEADER_ROW_CLASS}>{props.header}</TableRow>
        </TableHeader>
        <TableBody class={PLATFORM_TABLE_BODY_CLASS}>{props.body}</TableBody>
      </Table>
    </TableCard>
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
  getPlatformTableHeadClass(getPlatformColumnAlign(kind));

export const getPlatformTableCellClassForKind = (kind: PlatformTableColumnKind): string =>
  getPlatformTableCellClass(getPlatformColumnAlign(kind));

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
): JSX.CSSProperties => {
  const columnWeight = weights[columnId] ?? 0;
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
const matchesPlatformSearch = (resource: Resource, search: string): boolean => {
  if (!search) return true;
  const needle = search.trim().toLowerCase();
  if (!needle) return true;
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
  return haystack.includes(needle);
};

export const filterPlatformResources = (
  resources: Resource[],
  search: string,
  status: PlatformResourceStatusFilter,
  resolveStatus: (resource: Resource) => string | undefined = (resource) => resource.status,
): Resource[] => {
  const result: Resource[] = [];
  for (const resource of resources) {
    if (!matchesPlatformSearch(resource, search)) continue;
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
  const hasActiveFilters = createMemo(
    () => search().trim().length > 0 || status() !== props.initialStatus,
  );
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
    hasActiveFilters,
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
const PlatformResourceCounter: Component<{ visible: number; total: number; rowNoun: string }> = (
  props,
) => (
  <span class="ml-auto whitespace-nowrap text-xs font-medium text-muted">
    <Show
      when={props.visible !== props.total}
      fallback={
        <>
          {props.total} {props.rowNoun}
        </>
      }
    >
      {props.visible} of {props.total} {props.rowNoun}
    </Show>
  </span>
);

export function PlatformTableToolbar<T extends string | number>(props: {
  search: () => string;
  onSearchChange: (value: string) => void;
  searchPlaceholder: string;
  searchHistory?: SearchInputProps['history'];
  searchTips?: SearchInputProps['tips'];
  status: T;
  onStatusChange: (value: T) => void;
  statusOptions: PlatformTableFilterOption<T>[];
  visible: number;
  total: number;
  rowNoun: string;
  hasActiveFilters?: boolean;
  onResetFilters?: () => void;
  // Optional scope filters (host / node / namespace / pool ...) appended after
  // the status facet, plus an optional saved-views storage key. Tables opt into
  // richer combinable filtering without bypassing the shared toolbar; the
  // status facet stays the inline segmented control and scope filters render as
  // chips behind "+ Filter".
  filters?: FilterDef[];
  savedViewsKey?: string;
  viewOptionsTrailing?: JSX.Element;
}) {
  const { isMobile } = useBreakpoint();

  // Migrated onto the shared FilterBar so every platform table inherits the
  // same combinable-filter UX (chip rail, saved-view scaffolding, mobile
  // collapse) instead of a bespoke search + segmented-status row. The public
  // prop surface is unchanged: search passes straight through and the single
  // status facet is modelled as an inline segmented control. Tables that want
  // additional scope filters or saved views opt in via the FilterBar directly.
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
      }}
      filters={allFilters}
      savedViewsKey={props.savedViewsKey}
      viewOptionsTrailing={
        <>
          {props.viewOptionsTrailing}
          <PlatformResourceCounter
            visible={props.visible}
            total={props.total}
            rowNoun={props.rowNoun}
          />
        </>
      }
      showClearAll={() => Boolean(props.hasActiveFilters && props.onResetFilters)}
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
          status={tableState.status()}
          onStatusChange={tableState.setStatus}
          statusOptions={PLATFORM_STATUS_FILTER_OPTIONS}
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
