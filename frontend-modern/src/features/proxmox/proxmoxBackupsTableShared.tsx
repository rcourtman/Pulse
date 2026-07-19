import { Show, type Accessor } from 'solid-js';
import ArrowDownIcon from 'lucide-solid/icons/arrow-down';
import ArrowUpIcon from 'lucide-solid/icons/arrow-up';
import ArrowUpDownIcon from 'lucide-solid/icons/arrow-up-down';
import { filterChipStatusDot } from '@/components/shared/FilterBar';
import { type FilterOption } from '@/components/shared/FilterButtonGroup';
import { MetadataBadge } from '@/components/shared/MetadataBadge';
import { ProgressBar } from '@/components/shared/ProgressBar';
import { TableHead } from '@/components/shared/Table';
import { WorkloadTypeBadge as SharedWorkloadTypeBadge } from '@/components/shared/WorkloadTypeBadge';
import { PlatformTableRelativeTimeValue } from '@/features/platformPage/sharedPlatformPage';

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
  unknown: 'text-muted',
};

const recoveryAgeTitleByBand: Record<RecoveryAgeBand, string> = {
  current: 'Current backup age',
  aging: 'Aging backup age',
  stale: 'Stale backup age',
  unknown: 'Backup age unavailable',
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

export function ProxmoxBackupAgeText(props: { artifact: RecoverableArtifact }) {
  const band = () => getRecoveryAgeBand(props.artifact.createdMs);
  const title = () => {
    const parts = [recoveryAgeTitleByBand[band()], props.artifact.createdAt].filter(Boolean);
    return parts.join(' · ');
  };

  return (
    <span class={`font-semibold tabular-nums ${recoveryAgeClassByBand[band()]}`} title={title()}>
      <PlatformTableRelativeTimeValue value={props.artifact.createdAt} />
    </span>
  );
}

// Short state label for a recoverable artifact, paired with ArtifactStateBadge
// for colour. Snapshot and protected take precedence over verification state.
export function artifactStateLabel(artifact: RecoverableArtifact): string {
  if (artifact.sourceKind === 'snapshot') {
    return getProxmoxBackupSourcePresentation('snapshot').stateFallbackLabel;
  }
  if (artifact.protected) return 'Protected';
  if (artifact.verified === true) return 'Verified';
  if (artifact.verified === false) return 'Unverified';
  return getProxmoxBackupSourcePresentation(artifact.sourceKind).stateFallbackLabel;
}

export function ArtifactStateBadge(props: { artifact: RecoverableArtifact; label: string }) {
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
