import { Show, type Accessor } from 'solid-js';
import ArrowDownIcon from 'lucide-solid/icons/arrow-down';
import ArrowUpIcon from 'lucide-solid/icons/arrow-up';
import ArrowUpDownIcon from 'lucide-solid/icons/arrow-up-down';
import { type FilterOption } from '@/components/shared/FilterButtonGroup';
import { ProgressBar } from '@/components/shared/ProgressBar';
import { TableHead } from '@/components/shared/Table';
import { formatRelativeTime } from '@/utils/format';

import type { RecoverableArtifact } from './proxmoxBackupRecoveryModel';
import type {
  CoverageFilterValue,
  RecoverableFilterValue,
  SnapshotFilterValue,
} from './proxmoxBackupsTableModel';

// Shared presentational pieces for the Proxmox backups tabs: filter option
// catalogs and the small row/header components reused across the coverage,
// restore-points, source-detail, and job-history tables. Kept separate from
// the orchestrating ProxmoxBackupsTable so each sub-view can import only what
// it renders.

const statusDot = (className: string) => <span class={`h-2 w-2 rounded-full ${className}`} />;

export const ARCHIVE_STATUS_FILTERS: FilterOption<'all' | 'protected' | 'verified' | 'unverified'>[] =
  [
    { value: 'all', label: 'All' },
    { value: 'protected', label: 'Protected', tone: 'info', leading: statusDot('bg-blue-500') },
    { value: 'verified', label: 'Verified', tone: 'success', leading: statusDot('bg-emerald-500') },
    {
      value: 'unverified',
      label: 'Unverified',
      tone: 'warning',
      leading: statusDot('bg-amber-500'),
    },
  ];

export const PBS_STATUS_FILTERS: FilterOption<'all' | 'protected' | 'verified' | 'unverified'>[] = [
  { value: 'all', label: 'All' },
  { value: 'protected', label: 'Protected', tone: 'info', leading: statusDot('bg-blue-500') },
  { value: 'verified', label: 'Verified', tone: 'success', leading: statusDot('bg-emerald-500') },
  { value: 'unverified', label: 'Unverified', tone: 'warning', leading: statusDot('bg-amber-500') },
];

export const TASK_STATUS_FILTERS: FilterOption<'all' | 'ok' | 'failed' | 'running'>[] = [
  { value: 'all', label: 'All' },
  { value: 'ok', label: 'OK', tone: 'success', leading: statusDot('bg-emerald-500') },
  { value: 'failed', label: 'Failed', tone: 'danger', leading: statusDot('bg-red-500') },
  { value: 'running', label: 'Running', tone: 'info', leading: statusDot('bg-blue-500') },
];

export const COVERAGE_FILTERS: FilterOption<CoverageFilterValue>[] = [
  { value: 'all', label: 'All' },
  { value: 'attention', label: 'Attention', tone: 'warning', leading: statusDot('bg-amber-500') },
  { value: 'current', label: 'Current', tone: 'success', leading: statusDot('bg-emerald-500') },
  { value: 'uncovered', label: 'Uncovered', tone: 'danger', leading: statusDot('bg-red-500') },
];

export const RECOVERABLE_FILTERS: FilterOption<RecoverableFilterValue>[] = [
  { value: 'all', label: 'All' },
  { value: 'pbs', label: 'PBS', tone: 'info', leading: statusDot('bg-cyan-500') },
  { value: 'archive', label: 'Archives', tone: 'info', leading: statusDot('bg-blue-500') },
  { value: 'snapshot', label: 'Snapshots', tone: 'info', leading: statusDot('bg-violet-500') },
  { value: 'verified', label: 'Verified', tone: 'success', leading: statusDot('bg-emerald-500') },
  { value: 'unverified', label: 'Unverified', tone: 'warning', leading: statusDot('bg-amber-500') },
];

export const SNAPSHOT_FILTERS: FilterOption<SnapshotFilterValue>[] = [
  { value: 'all', label: 'All' },
  {
    value: 'recent',
    label: 'Recent ≤30d',
    tone: 'success',
    leading: statusDot('bg-emerald-500'),
  },
  {
    value: 'stale',
    label: 'Stale >30d',
    tone: 'warning',
    leading: statusDot('bg-amber-500'),
  },
  {
    value: 'with-ram',
    label: 'With RAM',
    tone: 'info',
    leading: statusDot('bg-violet-500'),
  },
];

// Canonical row metric bar — same shape as Storage's usage bar, Ceph's
// pool usage bar, and Workloads' MetricBar: a full-cell-width
// ProgressBar with the value text overlaid on top of the fill. The
// shared `ProgressBar` primitive (foreignObject-based fill that clips
// the label) is the one source of truth for this pattern in Pulse, so
// the Backups tabs read identically to the rest of the app.
export function RowMetricBar(props: {
  valuePct: number;
  fillClass: string;
  label: string;
  tooltip?: string;
}) {
  return (
    <div
      class="metric-text relative h-4 w-full min-w-[5rem] overflow-hidden"
      title={props.tooltip ?? props.label}
    >
      <ProgressBar
        value={props.valuePct}
        class="h-full"
        fillClass={props.fillClass}
        label={
          <span class="absolute inset-0 flex items-center justify-center text-[10px] font-medium leading-none text-base-content tabular-nums">
            <span class="max-w-full truncate px-1 text-center">{props.label}</span>
          </span>
        }
      />
    </div>
  );
}

export function RecoverySourceSummary(props: {
  artifact?: RecoverableArtifact;
  count: number;
  emptyLabel: string;
}) {
  return (
    <Show when={props.artifact} fallback={<span class="text-muted">{props.emptyLabel}</span>}>
      {(artifact) => (
        <div class="min-w-0">
          <div class="text-base-content">
            {formatRelativeTime(artifact().createdAt, { compact: true })}
          </div>
          <div class="truncate text-[10px] text-muted" title={artifact().location}>
            {props.count === 1
              ? artifact().location
              : `${props.count} total · ${artifact().location}`}
          </div>
        </div>
      )}
    </Show>
  );
}

export function ArtifactStateBadge(props: { artifact: RecoverableArtifact; label: string }) {
  if (props.artifact.sourceKind === 'snapshot') {
    return (
      <span class="inline-flex items-center rounded-sm bg-violet-100 px-1.5 py-0.5 text-[10px] font-semibold text-violet-700 dark:bg-violet-900/40 dark:text-violet-200">
        {props.label}
      </span>
    );
  }
  if (props.artifact.protected) {
    return (
      <span class="inline-flex items-center rounded-sm bg-amber-100 px-1.5 py-0.5 text-[10px] font-semibold text-amber-700 dark:bg-amber-900/40 dark:text-amber-200">
        {props.label}
      </span>
    );
  }
  if (props.artifact.verified === true) {
    return (
      <span class="inline-flex items-center rounded-sm bg-emerald-100 px-1.5 py-0.5 text-[10px] font-semibold text-emerald-700 dark:bg-emerald-900/40 dark:text-emerald-200">
        {props.label}
      </span>
    );
  }
  if (props.artifact.verified === false) {
    return (
      <span class="inline-flex items-center rounded-sm bg-amber-100 px-1.5 py-0.5 text-[10px] font-semibold text-amber-700 dark:bg-amber-900/40 dark:text-amber-200">
        {props.label}
      </span>
    );
  }
  return <span class="text-muted">{props.label}</span>;
}

export function ArtifactSourceBadge(props: { artifact: RecoverableArtifact }) {
  return (
    <span
      class={`inline-flex items-center rounded-sm px-1.5 py-0.5 text-[10px] font-semibold ${
        props.artifact.sourceKind === 'pbs'
          ? 'bg-cyan-100 text-cyan-700 dark:bg-cyan-900/40 dark:text-cyan-200'
          : props.artifact.sourceKind === 'archive'
            ? 'bg-blue-100 text-blue-700 dark:bg-blue-900/40 dark:text-blue-200'
            : 'bg-violet-100 text-violet-700 dark:bg-violet-900/40 dark:text-violet-200'
      }`}
    >
      {props.artifact.sourceLabel}
    </span>
  );
}

// Sortable column header — matches Storage's pattern (StoragePoolsTable.tsx).
// Clicking an inactive column sorts it with the supplied default direction;
// clicking the active column flips direction. Renders the idle / asc / desc
// arrow trio so the affordance reads identically across the app.
const SORT_BUTTON_CLASS =
  'inline-flex min-w-0 max-w-full items-center gap-1 rounded-sm outline-none transition-colors hover:text-base-content focus-visible:ring-2 focus-visible:ring-blue-500 focus-visible:ring-offset-1 focus-visible:ring-offset-surface';
const SORT_ICON_CLASS = 'h-3 w-3 shrink-0';

export function SortableHead<K extends string>(props: {
  label: string;
  sortKey: K;
  currentSort: Accessor<K>;
  direction: Accessor<'asc' | 'desc'>;
  onSort: (key: K) => void;
  align?: 'left' | 'right' | 'center';
  headClass: string;
}) {
  const isActive = () => props.currentSort() === props.sortKey;
  const buttonAlignClass = () => {
    if (props.align === 'right') return 'justify-end';
    if (props.align === 'center') return 'justify-center';
    return 'justify-start';
  };
  const ariaLabel = () => {
    if (!isActive()) return `Sort by ${props.label}`;
    return `Sort by ${props.label} ${props.direction() === 'asc' ? 'descending' : 'ascending'}`;
  };
  return (
    <TableHead
      class={props.headClass}
      aria-sort={
        isActive() ? (props.direction() === 'asc' ? 'ascending' : 'descending') : undefined
      }
    >
      <button
        type="button"
        class={`${SORT_BUTTON_CLASS} ${buttonAlignClass()} w-full`}
        onClick={() => props.onSort(props.sortKey)}
        aria-label={ariaLabel()}
        title={ariaLabel()}
      >
        <span class="min-w-0 truncate">{props.label}</span>
        <Show
          when={isActive()}
          fallback={
            <ArrowUpDownIcon class={`${SORT_ICON_CLASS} text-muted/70`} aria-hidden="true" />
          }
        >
          <Show
            when={props.direction() === 'asc'}
            fallback={
              <ArrowDownIcon class={`${SORT_ICON_CLASS} text-base-content`} aria-hidden="true" />
            }
          >
            <ArrowUpIcon class={`${SORT_ICON_CLASS} text-base-content`} aria-hidden="true" />
          </Show>
        </Show>
      </button>
    </TableHead>
  );
}
