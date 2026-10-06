import { Show, type Accessor } from 'solid-js';
import { filterChipStatusDot } from '@/components/shared/FilterBar';
import { type FilterOption } from '@/components/shared/FilterButtonGroup';
import { MetadataBadge } from '@/components/shared/MetadataBadge';
import { ProgressBar } from '@/components/shared/ProgressBar';
import { TableHead } from '@/components/shared/Table';
import { getTableSortIndicator } from '@/components/shared/tableSortPresentation';
import { WorkloadTypeBadge as SharedWorkloadTypeBadge } from '@/components/shared/WorkloadTypeBadge';
import {
  PlatformTableRelativeTimeValue,
  formatPlatformTableRelativeTimeValue,
} from '@/features/platformPage/sharedPlatformPage';
import { useRelativeTimeNow } from '@/utils/relativeTimeClock';

import {
  getRecoveryAgeBand,
  type RecoverableArtifact,
  type RecoveryAgeBand,
  type WorkloadReference,
} from './proxmoxBackupRecoveryModel';
import {
  getProxmoxBackupSourcePresentation,
  type ProxmoxBackupSourceKind,
} from './proxmoxBackupSourcePresentation';
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

const PROXMOX_BACKUP_METADATA_BADGE_PROPS = { size: 'xs', shape: 'rounded' } as const;

export const PROXMOX_BACKUP_COLUMN_LABELS = {
  targetId: 'Target ID',
  created: 'Created',
  details: 'Details',
} as const;

const recoveryAgeClassByBand: Record<RecoveryAgeBand, string> = {
  current: 'text-emerald-600 dark:text-emerald-300',
  aging: 'text-amber-600 dark:text-amber-300',
  stale: 'text-red-600 dark:text-red-300',
  unknown: 'text-amber-600 dark:text-amber-300',
};

const recoveryAgeTitleByBand: Record<RecoveryAgeBand, string> = {
  current: 'Current backup age',
  aging: 'Aging backup age',
  stale: 'Stale backup age',
  unknown: 'Restore-point date is unavailable or in the future. Its age is unknown.',
};

export const ARCHIVE_STATUS_FILTERS: FilterOption<
  'all' | 'protected' | 'verified' | 'unverified'
>[] = [
  { value: 'all', label: 'All' },
  {
    value: 'protected',
    label: 'Protected',
    tone: 'info',
    leading: filterChipStatusDot('bg-blue-500'),
  },
  {
    value: 'verified',
    label: 'Verified',
    tone: 'success',
    leading: filterChipStatusDot('bg-emerald-500'),
  },
  {
    value: 'unverified',
    label: 'Unverified',
    tone: 'warning',
    leading: filterChipStatusDot('bg-amber-500'),
  },
];

export const PBS_STATUS_FILTERS: FilterOption<'all' | 'protected' | 'verified' | 'unverified'>[] = [
  { value: 'all', label: 'All' },
  {
    value: 'protected',
    label: 'Protected',
    tone: 'info',
    leading: filterChipStatusDot('bg-blue-500'),
  },
  {
    value: 'verified',
    label: 'Verified',
    tone: 'success',
    leading: filterChipStatusDot('bg-emerald-500'),
  },
  {
    value: 'unverified',
    label: 'Unverified',
    tone: 'warning',
    leading: filterChipStatusDot('bg-amber-500'),
  },
];

export const TASK_STATUS_FILTERS: FilterOption<'all' | 'ok' | 'failed' | 'running'>[] = [
  { value: 'all', label: 'All' },
  { value: 'ok', label: 'OK', tone: 'success', leading: filterChipStatusDot('bg-emerald-500') },
  { value: 'failed', label: 'Failed', tone: 'danger', leading: filterChipStatusDot('bg-red-500') },
  { value: 'running', label: 'Running', tone: 'info', leading: filterChipStatusDot('bg-blue-500') },
];

export const COVERAGE_FILTERS: FilterOption<CoverageFilterValue>[] = [
  { value: 'all', label: 'All' },
  {
    value: 'attention',
    label: 'Attention',
    tone: 'warning',
    leading: filterChipStatusDot('bg-amber-500'),
  },
  {
    value: 'protected',
    label: 'Protected',
    tone: 'success',
    leading: filterChipStatusDot('bg-emerald-500'),
  },
  {
    value: 'unprotected',
    label: 'Unprotected',
    tone: 'danger',
    leading: filterChipStatusDot('bg-red-500'),
  },
  {
    value: 'unknown',
    label: 'Unknown',
    leading: filterChipStatusDot('bg-base-content/40'),
  },
];

const recoverableSourceFilterOption = (
  value: ProxmoxBackupSourceKind,
): FilterOption<RecoverableFilterValue> => {
  const presentation = getProxmoxBackupSourcePresentation(value);
  return {
    value,
    label: presentation.filterLabel,
    ariaLabel: presentation.filterAriaLabel,
    compactLabel: presentation.compactFilterLabel,
    title: presentation.filterTitle,
    tone: 'info',
    leading: filterChipStatusDot(presentation.timelineSwatchClassName),
  };
};

export const RECOVERABLE_FILTERS: FilterOption<RecoverableFilterValue>[] = [
  { value: 'all', label: 'All' },
  recoverableSourceFilterOption('pbs'),
  recoverableSourceFilterOption('archive'),
  recoverableSourceFilterOption('snapshot'),
  {
    value: 'verified',
    label: 'Verified',
    tone: 'success',
    leading: filterChipStatusDot('bg-emerald-500'),
  },
  {
    value: 'unverified',
    label: 'Unverified',
    tone: 'warning',
    leading: filterChipStatusDot('bg-amber-500'),
  },
];

export const SNAPSHOT_FILTERS: FilterOption<SnapshotFilterValue>[] = [
  { value: 'all', label: 'All' },
  {
    value: 'recent',
    label: 'Recent ≤30d',
    tone: 'success',
    leading: filterChipStatusDot('bg-emerald-500'),
  },
  {
    value: 'stale',
    label: 'Stale >30d',
    tone: 'warning',
    leading: filterChipStatusDot('bg-amber-500'),
  },
  {
    value: 'with-ram',
    label: 'With RAM',
    tone: 'info',
    leading: filterChipStatusDot('bg-violet-500'),
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
      class="metric-text relative h-4 w-full min-w-20 overflow-hidden"
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

// Phone columns are too narrow for "18h ago", so the compact form drops the
// suffix ("18h", "now") and keeps the full age band and timestamp on hover.
export function formatCompactBackupAge(createdAt: string, now?: number): string {
  const relative = formatPlatformTableRelativeTimeValue(createdAt, { now });
  if (relative === 'just now') return 'now';
  return relative.replace(/ ago$/, '');
}

export function ProxmoxBackupAgeText(props: { artifact: RecoverableArtifact; compact?: boolean }) {
  // Backup rows stay mounted while an artifact's creation time never changes,
  // so the age and its freshness band both read the shared clock. The clock
  // can trail the wall clock by up to one tick, and a backup that finished
  // inside that window must not band as a future (unknown) age.
  const tickNow = useRelativeTimeNow();
  const now = () => Math.max(tickNow(), Date.now());
  const band = () => getRecoveryAgeBand(props.artifact.createdMs, now());
  const title = () => {
    if (band() === 'unknown') return recoveryAgeTitleByBand.unknown;
    const parts = [recoveryAgeTitleByBand[band()], props.artifact.createdAt].filter(Boolean);
    return parts.join(' · ');
  };

  return (
    <span
      class={`font-semibold tabular-nums ${recoveryAgeClassByBand[band()]}`}
      classList={{ 'text-[10px]': props.compact && band() === 'unknown' }}
      title={title()}
      aria-label={band() === 'unknown' ? `Unknown age. ${title()}` : undefined}
    >
      <Show when={band() !== 'unknown'} fallback="Unknown">
        <Show
          when={props.compact}
          fallback={<PlatformTableRelativeTimeValue value={props.artifact.createdAt} />}
        >
          {formatCompactBackupAge(props.artifact.createdAt, now())}
        </Show>
      </Show>
    </span>
  );
}

// Short state label for a recoverable artifact, paired with ArtifactStateBadge
// for colour. Snapshot and protected take precedence over verification state.
export function artifactStateLabel(artifact: RecoverableArtifact): string {
  if (artifact.sourceKind === 'snapshot') {
    return getProxmoxBackupSourcePresentation('snapshot').stateFallbackLabel;
  }
  // Runtime/terminal incompleteness outranks every completed-artifact state.
  if (artifact.running) return 'Running';
  if (artifact.failed) return 'Failed';
  if (artifact.protected) return 'Protected';
  if (artifact.verified === true) return 'Verified';
  if (artifact.verified === false) return 'Unverified';
  return getProxmoxBackupSourcePresentation(artifact.sourceKind).stateFallbackLabel;
}

export function ArtifactStateBadge(props: { artifact: RecoverableArtifact; label: string }) {
  if (props.artifact.running) {
    return (
      <MetadataBadge {...PROXMOX_BACKUP_METADATA_BADGE_PROPS} tone="info">
        {props.label}
      </MetadataBadge>
    );
  }
  if (props.artifact.failed) {
    return (
      <MetadataBadge {...PROXMOX_BACKUP_METADATA_BADGE_PROPS} tone="danger">
        {props.label}
      </MetadataBadge>
    );
  }
  if (props.artifact.sourceKind === 'snapshot') {
    return (
      <MetadataBadge {...PROXMOX_BACKUP_METADATA_BADGE_PROPS} tone="indigo">
        {props.label}
      </MetadataBadge>
    );
  }
  if (props.artifact.protected) {
    return (
      <MetadataBadge {...PROXMOX_BACKUP_METADATA_BADGE_PROPS} tone="warning">
        {props.label}
      </MetadataBadge>
    );
  }
  if (props.artifact.verified === true) {
    return (
      <MetadataBadge {...PROXMOX_BACKUP_METADATA_BADGE_PROPS} tone="success">
        {props.label}
      </MetadataBadge>
    );
  }
  if (props.artifact.verified === false) {
    return (
      <MetadataBadge {...PROXMOX_BACKUP_METADATA_BADGE_PROPS} tone="warning">
        {props.label}
      </MetadataBadge>
    );
  }
  return (
    <MetadataBadge {...PROXMOX_BACKUP_METADATA_BADGE_PROPS} tone="muted">
      {props.label}
    </MetadataBadge>
  );
}

export function ArtifactSourceBadge(props: { artifact: RecoverableArtifact }) {
  const presentation = () => getProxmoxBackupSourcePresentation(props.artifact.sourceKind);
  const title = () => props.artifact.sourceTitle ?? presentation().sourceTitle;

  return (
    <MetadataBadge
      {...PROXMOX_BACKUP_METADATA_BADGE_PROPS}
      tone={presentation().badgeTone}
      title={title()}
    >
      {props.artifact.sourceLabel}
    </MetadataBadge>
  );
}

const proxmoxBackupWorkloadBadgeType = (
  type: WorkloadReference['type'],
): string | null | undefined => {
  if (type === 'ct') return 'system-container';
  if (type === 'host') return 'agent';
  return type === 'unknown' ? undefined : type;
};

const proxmoxBackupWorkloadBadgeTitle = (type: WorkloadReference['type']): string | undefined => {
  if (type === 'host') return 'Host backup';
  return undefined;
};

export function ProxmoxBackupWorkloadTypeBadge(props: {
  type: WorkloadReference['type'];
  label: string;
}) {
  return (
    <SharedWorkloadTypeBadge
      type={proxmoxBackupWorkloadBadgeType(props.type)}
      label={props.label}
      title={proxmoxBackupWorkloadBadgeTitle(props.type)}
    />
  );
}

// Sortable column header — matches the shared active-only table pattern.
// Clicking an inactive column sorts it with the supplied default direction;
// clicking the active column flips direction.
// Buttons reset text-transform and letter-spacing, so a sortable header has to
// restate the header casing or it renders in sentence case beside its
// uppercase non-sortable neighbours.
const SORT_BUTTON_CLASS =
  'inline-flex min-w-0 max-w-full items-center gap-1 rounded-xs uppercase tracking-[inherit] outline-hidden transition-colors hover:text-base-content focus-visible:ring-2 focus-visible:ring-blue-500 focus-visible:ring-offset-1 focus-visible:ring-offset-surface';

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
        {getTableSortIndicator(isActive(), props.direction())}
      </button>
    </TableHead>
  );
}
