import {
  For,
  Show,
  createEffect,
  createMemo,
  createSignal,
  on,
  type Accessor,
  type JSX,
} from 'solid-js';

import {
  InlineDetailPanel,
  DetailSectionTable,
  compactDetailRows,
  compactDetailSections,
  makeDetailRow,
  type DetailSection,
  type DetailValueTone,
} from '@/components/shared/DetailSectionTable';
import { ResourceDetailDrawerSupportDisclosure } from '@/components/Infrastructure/ResourceDetailDrawerSupportDisclosure';
import { InlineDetailTableRow } from '@/components/shared/InlineDetailTableRow';
import { InlineNotice } from '@/components/shared/InlineNotice';
import { StatusDot } from '@/components/shared/StatusDot';
import { TableCell, TableHead, TableRow } from '@/components/shared/Table';
import {
  formatPlatformTableBytesValue,
  formatPlatformTableDateTimeValue,
  getPlatformTableCellClassForKind,
  getPlatformTableHeadClassForKind,
  PlatformDetailTable,
  PlatformDetailTableBody,
  PlatformDetailTableHeader,
  PlatformTableEmptyState,
  PlatformTableShell,
} from '@/features/platformPage/sharedPlatformPage';
import {
  getPlatformResourceDetailRowInteractionProps,
  PlatformResourceDetailToggleButton,
} from '@/features/platformPage/PlatformResourceDetailTableRow';
import { RecordOwnerLink } from '@/features/platformPage/RecordOwnerLink';
import type { StatusIndicatorVariant } from '@/utils/status';
import { useObservedElementWidth } from '@/hooks/useObservedElementWidth';
import { parseBackupTimestampMs } from '@/utils/backupTimestamp';

import {
  getWorkloadRecoveryPostureLabel,
  type WorkloadCoverageRow,
  type CoverageTask,
  type RecoverableArtifact,
} from './proxmoxBackupRecoveryModel';
import { proxmoxBackupWorkloadSubject } from './proxmoxBackupWorkloadSubject';
import { formatDurationFromSeconds, type CoverageSortKey } from './proxmoxBackupsTableModel';
import {
  ArtifactSourceIdentity,
  ArtifactStateBadge,
  PROXMOX_BACKUP_COLUMN_LABELS,
  ProxmoxBackupAgeText,
  ProxmoxBackupWorkloadTypeBadge,
  SortableHead,
  retainBackupTableFocus,
  artifactDateRows,
  artifactStateLabel,
  getBackupWorkloadPVEConnection,
  hasMultipleConnections,
  getBackupWorkloadNodeLabel,
  getBackupWorkloadDisclosureLabel,
} from './proxmoxBackupsTableShared';
import {
  getCoverageColumns,
  getCoverageColumnWidthStyle,
  getCoverageLayoutForContainer,
  isCoverageEvidenceColumnVisible,
  selectCoverageRestoreEvidence,
  selectCoverageTaskEvidence,
  COVERAGE_TASK_EVIDENCE_PAGE_SIZE,
  COVERAGE_RESTORE_EVIDENCE_PAGE_SIZE,
  type CoverageColumnId,
} from './proxmoxBackupsTablePresentation';
import { compactBackupReasonLabel } from './proxmoxBackupVerdict';
import { PlatformWindowedRows } from '@/features/platformPage/PlatformWindowedRows';

// Each evidence column is already named by its header, so an empty cell says
// "None" instead of restating the source ("No PVE backup file"). The long form
// truncated at most widths and stays available as the hover title.
const COVERAGE_EMPTY_LABEL = 'None';
const COVERAGE_NO_BACKUP_TITLE = 'No PBS backup or PVE backup file';
// Matches the posture engine's own wording for a snapshot-only subject.
const COVERAGE_SNAPSHOT_ONLY_TITLE =
  'No PBS backup or PVE backup file. A guest snapshot is not a separate backup, so it does not count.';
const COVERAGE_HISTORY_UNKNOWN_TITLE =
  'No completed backup is visible in the current list. Missing entries do not mean no backups exist. Expand this row to inspect the evidence.';

// Provider quality supports the backup-age fallback, not absence of every
// restore point. In particular, complete PVE backup enumeration says nothing
// about an unread snapshot list or the posture's retained snapshot evidence.
// A fulfilled raw request and the posture colour cannot establish that either.
const hasCompleteBackupHistory = (row: WorkloadCoverageRow): boolean => {
  const posture = row.protectionPosture;
  const providers = posture?.providerStates ?? [];
  return (
    (posture?.coverage === 'complete' || posture?.coverage === 'none') &&
    providers.length > 0 &&
    providers.every(
      (provider) =>
        provider.historyCompleteness === 'complete' && provider.permissions === 'sufficient',
    )
  );
};

function CoverageAgeFallback(props: {
  unknown?: boolean;
  historyUnknown?: boolean;
  emptyTitle: string;
  compact?: boolean;
}) {
  const unknown = () => props.unknown || props.historyUnknown;
  const title = () =>
    props.unknown
      ? 'One or more completed restore-point dates are unavailable or in the future. The latest point is unknown. Expand this row to inspect the evidence.'
      : props.historyUnknown
        ? COVERAGE_HISTORY_UNKNOWN_TITLE
        : props.emptyTitle;
  return (
    <span
      class={unknown() ? 'font-semibold text-amber-600 dark:text-amber-300' : 'text-muted'}
      classList={{ 'text-[10px]': unknown() && props.compact }}
      title={title()}
      aria-label={unknown() ? `Unknown age. ${title()}` : undefined}
    >
      {unknown() ? 'Unknown' : COVERAGE_EMPTY_LABEL}
    </span>
  );
}

const coveragePostureVariant = (
  posture: WorkloadCoverageRow['posture'],
): StatusIndicatorVariant => {
  if (posture === 'protected') return 'success';
  if (posture === 'unprotected') return 'danger';
  if (posture === 'attention') return 'warning';
  if (posture === 'checking') return 'info';
  return 'muted';
};

// Colour marks the exception, not the baseline: healthy rows keep neutral text
// (the green dot already signals "fine"), while attention/danger words take an
// amber/red tone so a mixed fleet is scannable at a glance. Matches the
// datastore-usage tones on the Backup servers table and the status-word colour
// pattern in the Replication table.
const statusWordToneClass = (variant: StatusIndicatorVariant): string => {
  if (variant === 'danger') return 'text-red-600 dark:text-red-300';
  if (variant === 'warning') return 'text-amber-600 dark:text-amber-300';
  return 'text-base-content';
};

const statusDetailTone = (variant: StatusIndicatorVariant): DetailValueTone | undefined => {
  if (variant === 'danger') return 'danger';
  if (variant === 'warning') return 'warning';
  return undefined;
};

// The fallback for a row with no reason this build can name.
const compactPostureLabel = (posture: WorkloadCoverageRow['posture']): string => {
  switch (posture) {
    case 'protected':
      return 'Protected';
    case 'attention':
      return 'Attn.';
    case 'unprotected':
      return 'Unprot.';
    case 'checking':
      return 'Check';
    case 'not-evaluated':
      return 'N/A';
    default:
      return 'Unknown';
  }
};

const taskWordVariant = (label: string): StatusIndicatorVariant => {
  if (label === 'Failed') return 'danger';
  if (label === 'OK') return 'success';
  return 'warning';
};

const providerLabel = (provider: string): string => {
  if (provider === 'proxmox-pbs') return 'Proxmox Backup Server';
  if (provider === 'proxmox-pve') return 'Proxmox VE';
  return provider;
};

const evidenceQualityLabel = (value: string): string => {
  if (!value) return 'Unknown';
  return value.charAt(0).toUpperCase() + value.slice(1);
};

const providerEvidenceSourceLabel = (source: string | undefined): string => {
  switch (source?.trim()) {
    case 'pbs-backup-enumeration':
      return 'PBS backup inventory';
    case 'pve-backup-enumeration':
      return 'PVE backup inventory';
    case 'legacy-recovery-point':
      return 'Retained restore point';
    default:
      return source?.trim() || 'Unknown';
  }
};

const unmatchedBackupsNote = (count: number): string =>
  count === 1 ? '1 unmatched backup has this VMID.' : `${count} unmatched backups have this VMID.`;

const postureExplanation = (row: WorkloadCoverageRow): string => {
  const unmatched = row.unmatchedPBSEntries
    ? ` ${unmatchedBackupsNote(row.unmatchedPBSEntries)}`
    : '';
  if (row.protectionPosture?.explanation) return `${row.protectionPosture.explanation}${unmatched}`;
  if (row.posture === 'checking') return "Pulse is checking this guest's backups.";
  if (row.posture === 'not-evaluated') {
    const guests = row.sameVmidGuestLabels ?? [];
    return guests.length > 0
      ? `Guests with this VMID: ${guests.join(', ')}. Pulse cannot tell which one this backup belongs to, so it is not counted. A PBS namespace named after each connection can settle it.`
      : 'This backup does not match a guest Pulse currently monitors, so it is not rated.';
  }
  return `Pulse cannot tell yet whether this guest is backed up.${unmatched}`;
};

const TASK_ORDER_NOTICE =
  'Task start times do not establish which task ran last. Possible latest tasks are listed below without a time order.';

// Keep the assessment's clocks independent of the currently visible artifacts.
// Use explicit UTC instants, not relative ages which could imply a fresh read
// or turn the evaluation time into a backup completion time.
const assessmentTimeRow = (label: string, value: string | null | undefined) => {
  const ms = parseBackupTimestampMs(value ?? undefined);
  const display =
    ms === undefined
      ? value
        ? 'Unknown'
        : 'Not reported'
      : formatPlatformTableDateTimeValue(ms, {
          dateTimeFormat: {
            year: 'numeric',
            month: 'short',
            day: '2-digit',
            hour: '2-digit',
            minute: '2-digit',
            second: '2-digit',
            timeZone: 'UTC',
            timeZoneName: 'short',
          },
        });
  return makeDetailRow(label, display, {
    layout: 'stacked',
    wrap: true,
    tone: ms === undefined ? 'muted' : undefined,
    valueContent:
      ms === undefined ? undefined : (
        <time dateTime={value!} class="block whitespace-normal text-left tabular-nums">
          {display}
        </time>
      ),
  });
};

const buildCoverageTaskSection = (task: CoverageTask, label: string): DetailSection => ({
  label,
  rows: compactDetailRows([
    makeDetailRow('Status', task.label, {
      tone: statusDetailTone(taskWordVariant(task.label)),
    }),
    makeDetailRow(
      'Started',
      task.startedMs === undefined ? 'Unknown' : formatPlatformTableDateTimeValue(task.startedMs),
      { tone: task.startedMs === undefined ? 'warning' : undefined },
    ),
    makeDetailRow(
      'Duration',
      task.durationUnknown
        ? 'Unknown'
        : task.durationSeconds === undefined
          ? 'Not reported'
          : task.durationSeconds === 0
            ? '0s'
            : formatDurationFromSeconds(task.durationSeconds),
      { tone: task.durationUnknown ? 'warning' : undefined },
    ),
    makeDetailRow(
      'Error',
      task.error?.trim() || (task.label === 'Failed' ? 'Not reported' : undefined),
      {
        tone: 'danger',
        valueContent: (
          <span class="block whitespace-pre-wrap text-left font-normal leading-snug wrap-anywhere">
            {task.error?.trim() || 'Not reported'}
          </span>
        ),
      },
    ),
    makeDetailRow('Task connection', task.instance || 'Not reported', { wrap: true }),
    makeDetailRow('Task node', task.node || 'Not reported', { wrap: true }),
    makeDetailRow('Task ID', task.id, { wrap: true }),
  ]),
  footerContent: (
    <>
      <Show when={task.durationUnknown}>
        <p class="whitespace-normal text-[11px] leading-snug text-muted wrap-anywhere">
          The reported timestamps or running status do not establish a completed duration.
        </p>
      </Show>
      <p class="whitespace-normal text-[11px] leading-snug text-muted wrap-anywhere">
        Connection and node are where the task ran.
      </p>
      <p class="whitespace-normal text-[11px] leading-snug text-muted wrap-anywhere">
        A task result doesn't prove a restore works.
      </p>
    </>
  ),
});

function CoverageTaskCandidates(props: { tasks: readonly CoverageTask[] }) {
  const [limit, setLimit] = createSignal(COVERAGE_TASK_EVIDENCE_PAGE_SIZE);
  const [retainControl, setRetainControl] = createSignal(false);
  createEffect(() => {
    if (props.tasks.length > COVERAGE_TASK_EVIDENCE_PAGE_SIZE) setRetainControl(true);
  });
  const preview = createMemo(() => selectCoverageTaskEvidence(props.tasks, limit()));
  const counts = createMemo(() => {
    const tasks = props.tasks;
    return [
      ['Failed', tasks.filter((task) => task.label === 'Failed').length],
      ['Running', tasks.filter((task) => task.label === 'Running').length],
      ['OK', tasks.filter((task) => task.label === 'OK').length],
      [
        'Other status',
        tasks.filter((task) => !['Failed', 'Running', 'OK'].includes(task.label)).length,
      ],
      ['With error text', tasks.filter((task) => task.error?.trim()).length],
      ['Unknown start', tasks.filter((task) => task.startedMs === undefined).length],
    ]
      .filter(([, count]) => count)
      .map(([label, count]) => `${label}: ${count}`)
      .join(' · ');
  });
  return (
    <Show when={props.tasks.length > 0 || retainControl()}>
      <div class="@container/task-evidence min-w-0 space-y-2" data-coverage-task-candidates>
        <p class="text-[11px] text-muted whitespace-normal wrap-anywhere" role="status">
          Showing {preview().length} of {props.tasks.length} possible latest tasks.
          <Show when={props.tasks.length === 0}> No uncertain tasks remain in this read.</Show>
        </p>
        <Show when={props.tasks.length > 0}>
          <p class="text-[11px] text-muted whitespace-normal wrap-anywhere">{counts()}</p>
          <Show when={props.tasks.length > preview().length}>
            <p class="text-[11px] text-muted whitespace-normal wrap-anywhere">
              The preview samples outcomes and known or unknown starts. Full counts include tasks
              outside the preview. It is not a time order.
            </p>
          </Show>
        </Show>
        {/* Outside rebuilt detail sections: the same control keeps keyboard focus
            through expansion, polling, exhaustion and candidate withdrawal. */}
        <Show when={retainControl()}>
          <button
            type="button"
            class="min-h-10 rounded-sm px-2 py-1 text-xs font-medium text-accent hover:bg-surface-hover focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-accent"
            aria-disabled={preview().length >= props.tasks.length}
            onClick={() => {
              if (preview().length < props.tasks.length)
                setLimit(preview().length + COVERAGE_TASK_EVIDENCE_PAGE_SIZE);
            }}
          >
            <Show
              when={preview().length < props.tasks.length}
              fallback={<>All {preview().length} possible latest tasks shown</>}
            >
              Show{' '}
              {Math.min(COVERAGE_TASK_EVIDENCE_PAGE_SIZE, props.tasks.length - preview().length)}{' '}
              more possible latest tasks
            </Show>
          </button>
        </Show>
        <Show when={props.tasks.length > 0}>
          <DetailSectionTable
            wrapValues
            sections={[
              {
                label: 'Task order',
                // The literal Unknown value is always a renderable detail row.
                rows: [makeDetailRow('Latest task', 'Unknown', { tone: 'warning' })!],
                footerContent: (
                  <p class="whitespace-normal text-[11px] leading-snug text-muted wrap-anywhere">
                    {TASK_ORDER_NOTICE}
                  </p>
                ),
              },
            ]}
          />
          {/* Keep existing cards when another page is revealed. Rebuilding one
              large sections array would recreate every previous task as well. */}
          <div class="grid min-w-0 grid-cols-1 gap-2 @min-[48rem]/task-evidence:grid-cols-2 @min-[64rem]/task-evidence:grid-cols-3">
            <For each={preview()}>
              {(task) => (
                <div class="min-w-0" data-coverage-task-card={task.id}>
                  <DetailSectionTable
                    wrapValues
                    sections={[
                      buildCoverageTaskSection(
                        task,
                        `Possible latest task ${props.tasks.indexOf(task) + 1}`,
                      ),
                    ]}
                  />
                </div>
              )}
            </For>
          </div>
        </Show>
      </div>
    </Show>
  );
}

// Provider outcomes can come from an inventory observation or a retained point.
// The rollup may keep a newer point's outcome beside an older observation's
// source, so neither "Job" nor "Collection" accurately names every outcome.
// Keep its reported value, identify the source, and leave actual task/point
// states in their own sections. Provider quality explains a posture, not a job.
const buildCoverageDetailSections = (row: WorkloadCoverageRow): DetailSection[] =>
  compactDetailSections([
    {
      label: 'Workload',
      rows: compactDetailRows([
        makeDetailRow('Type', row.workload.typeLabel),
        makeDetailRow(PROXMOX_BACKUP_COLUMN_LABELS.targetId, row.workload.vmid),
        makeDetailRow('Node', getBackupWorkloadNodeLabel(row.workload)),
        makeDetailRow('PVE connection', getBackupWorkloadPVEConnection(row.workload), {
          wrap: true,
        }),
      ]),
    },
    row.protectionPosture
      ? {
          label: 'Protection assessment',
          rows: compactDetailRows([
            assessmentTimeRow('Evaluated', row.protectionPosture.evaluatedAt),
            assessmentTimeRow(
              'Latest successful point',
              row.protectionPosture.lastSuccessfulPointAt,
            ),
            assessmentTimeRow('Last verified', row.protectionPosture.lastVerifiedAt),
          ]),
          footerContent: (
            <p class="whitespace-normal text-[11px] leading-snug text-muted wrap-anywhere">
              From the server assessment, independent of this filtered list. Evaluated isn't a
              backup time, and Last verified doesn't prove a restore works.
            </p>
          ),
        }
      : null,
    row.latestTask ? buildCoverageTaskSection(row.latestTask, 'Latest task') : null,
    {
      label: 'Provider evidence',
      rows: compactDetailRows(
        (row.protectionPosture?.providerStates ?? []).map((provider) =>
          makeDetailRow(
            providerLabel(provider.provider),
            [
              `Scope ${provider.scope?.trim() || 'Unknown'}`,
              `Source ${providerEvidenceSourceLabel(provider.source)}`,
              `Reported outcome ${evidenceQualityLabel(provider.jobState)}`,
              `History ${evidenceQualityLabel(provider.historyCompleteness)}`,
              `Access ${evidenceQualityLabel(provider.permissions)}`,
            ].join(' · '),
            {
              layout: 'stacked',
              valueContent: (
                <span class="block space-y-1 text-left font-normal leading-snug">
                  <span class="block whitespace-normal wrap-anywhere">
                    Scope {provider.scope?.trim() || 'Unknown'}
                  </span>
                  <span class="block whitespace-normal wrap-anywhere">
                    Source {providerEvidenceSourceLabel(provider.source)}
                  </span>
                  <span class="flex flex-wrap gap-x-3">
                    <span>Reported outcome {evidenceQualityLabel(provider.jobState)}</span>
                    <span>History {evidenceQualityLabel(provider.historyCompleteness)}</span>
                    <span>Access {evidenceQualityLabel(provider.permissions)}</span>
                  </span>
                </span>
              ),
            },
          ),
        ),
      ),
      footerContent: row.protectionPosture?.providerStates.length ? (
        <p class="whitespace-normal text-[11px] leading-snug text-muted wrap-anywhere">
          What Pulse could read from inventory and restore points. Reported outcome isn't a backup
          job result.
        </p>
      ) : undefined,
    },
  ]);

// Several points can share a reader, location and rounded age. Tapping a point
// shows what tells it apart, in the same shape as its By date record.
function coverageRestorePointDetails(artifact: RecoverableArtifact) {
  const hasSize = () => Boolean(artifact.size && artifact.size > 0);
  return (
    <div class="min-w-0 whitespace-normal" data-coverage-evidence-detail>
      <DetailSectionTable
        wrapValues
        sections={[
          {
            label: 'Restore point',
            rows: compactDetailRows([
              makeDetailRow('Connection', artifact.locationConnection || 'Not reported', {
                wrap: true,
                valueContent: (
                  <span class="wrap-anywhere" data-backup-source-connection>
                    {artifact.locationConnection || 'Not reported'}
                  </span>
                ),
              }),
              makeDetailRow('Location', artifact.location, { wrap: true }),
              ...artifactDateRows(artifact),
              makeDetailRow('Size', formatPlatformTableBytesValue(artifact.size, 'Unknown size'), {
                tone: hasSize() ? undefined : 'muted',
              }),
              makeDetailRow('Identifier', artifact.nativeId, { wrap: true }),
              makeDetailRow('Metadata', artifact.detailTitle || artifact.detail, { wrap: true }),
            ]),
            footerContent: hasSize() ? undefined : (
              <p class="whitespace-normal text-[11px] leading-snug text-muted wrap-anywhere">
                No usable size was reported. Unknown does not mean this restore point is empty.
              </p>
            ),
          },
        ]}
      />
    </div>
  );
}

// "Workload coverage" table: one row per workload answering "does this have a
// backup?" across PBS snapshots / PVE backup files / guest snapshots, each
// expanding to its restore evidence. Presentational — the parent owns the
// filtered+sorted memo, shared filters, and the expansion set. table-fixed + a
// colgroup keeps the columns from ballooning; the inner evidence table stays
// content-sized. The name cell retains the guest's connection at every width;
// Type / Target ID fold beside the name when their columns are hidden. The
// guest's connection is independent of its restore-evidence reader below.
export function ProxmoxCoverageTable(props: {
  rows: WorkloadCoverageRow[];
  // Every row before search and filters, to tell namesakes apart. A filter
  // that leaves one of two namesakes must not change how that row is named.
  identityRows?: readonly WorkloadCoverageRow[];
  hasAnyRows: boolean;
  emptyIcon: JSX.Element;
  emptyTitle: string;
  emptyDescription: string;
  sortKey: Accessor<CoverageSortKey>;
  sortDirection: Accessor<'asc' | 'desc'>;
  onSort: (key: CoverageSortKey) => void;
  /** The one workload whose drawer is open, across the live and unmatched tables. */
  expandedKey: string | null;
  onToggleExpand: (key: string) => void;
  // The task column auto-hides when no workload has task history. Per-source
  // ages (PBS snapshot, PVE file, guest snapshot) live in the row expansion.
  showTaskColumn: boolean;
  /** Inventory or posture read is pending/failed; retained quality cannot confirm absence. */
  historyReadIncomplete?: boolean;
  /** Resolved restore-evidence filter; assessments and tasks remain workload-wide. */
  evidenceLocationLabel?: string;
  layoutWidth?: Accessor<number | null | undefined>;
  /** Name each row's PVE connection; defaults to whether the rows span several. */
  showWorkloadConnection?: boolean;
  /** Register focus retention with each row/detail's own cleanup owner. */
  retainInspectionFocus?: (content: HTMLElement) => void;
}) {
  const [evidenceLimit, setEvidenceLimit] = createSignal(COVERAGE_RESTORE_EVIDENCE_PAGE_SIZE);
  // There is one open workload. Keep its deliberate preview expansion across
  // polling, but start bounded again when a different row is opened or closed.
  const [openPoints, setOpenPoints] = createSignal<ReadonlySet<string>>(new Set());
  const [moreDetails, setMoreDetails] = createSignal(false);
  const togglePoint = (id: string) =>
    setOpenPoints((current) => {
      const next = new Set(current);
      if (!next.delete(id)) next.add(id);
      return next;
    });
  createEffect(
    on(
      () => props.expandedKey,
      () => {
        setEvidenceLimit(COVERAGE_RESTORE_EVIDENCE_PAGE_SIZE);
        setOpenPoints(new Set<string>());
        setMoreDetails(false);
      },
    ),
  );
  let coverageElement: HTMLDivElement | undefined;
  // Keyed rendering retains the same row nodes, but moving those nodes in the
  // native DOM can still blur a disclosure or detail link to BODY. Capture only
  // this table's owned focus before the sorted list reconciles. Never replace
  // the sort order or refocus a removed row, another route, or a user choice.
  const renderedRows = createMemo<readonly WorkloadCoverageRow[]>((previous) => {
    const rows = props.rows;
    if (previous.some((row, index) => row.key !== rows[index]?.key)) {
      retainBackupTableFocus(coverageElement);
    }
    return rows;
  }, []);
  const observedWidth = useObservedElementWidth();
  const containerWidth = () => props.layoutWidth?.() ?? observedWidth.width();
  const layoutMode = createMemo(() => {
    const width = containerWidth();
    return typeof width === 'number' && width > 0 ? getCoverageLayoutForContainer(width) : 'full';
  });
  // Rows stay on one line at every width. The PVE connection is named inline
  // only when the rows span more than one, where it tells namesakes apart;
  // the drawer always shows it in full.
  const showWorkloadConnection = createMemo(
    () =>
      props.showWorkloadConnection ??
      hasMultipleConnections(props.rows.map((row) => getBackupWorkloadPVEConnection(row.workload))),
  );
  // On a phone the name column is narrow; the VM id and connection beside the
  // name cut it to a few letters. There they stay only for namesakes, the rows
  // they tell apart; the disclosure label and the drawer always carry both.
  const sharedNames = createMemo(() => {
    const counts = new Map<string, number>();
    for (const row of props.identityRows ?? props.rows) {
      const name = (row.workload.name || row.workload.label).trim().toLowerCase();
      counts.set(name, (counts.get(name) ?? 0) + 1);
    }
    return new Set([...counts].filter(([, count]) => count > 1).map(([name]) => name));
  });
  const showIdentityBesideName = (row: WorkloadCoverageRow): boolean =>
    layoutMode() !== 'compact' ||
    sharedNames().has((row.workload.name || row.workload.label).trim().toLowerCase());
  const visibleColumns = createMemo(() =>
    getCoverageColumns(layoutMode(), {
      task: props.showTaskColumn,
    }),
  );
  const visibleColumnIds = createMemo(() => visibleColumns().map((column) => column.id));
  const columnVisible = (column: CoverageColumnId) => visibleColumnIds().includes(column);
  const columnCount = () => visibleColumns().length;
  // Full uncertainty words must fit even beside a Job column at 320px. Numeric
  // ages were short enough for the usual gutters; compact ages use that space.
  const ageCellClass = (kind: 'text' | 'numeric-value') =>
    `${layoutMode() === 'compact' ? `px-0.5 py-1 ${kind === 'numeric-value' ? 'text-right' : ''}` : getPlatformTableCellClassForKind(kind)} text-base-content`;

  return (
    <Show
      when={props.rows.length > 0}
      fallback={
        <PlatformTableEmptyState
          icon={props.emptyIcon}
          title={
            !props.hasAnyRows ? props.emptyTitle : 'No workload coverage rows match current filters'
          }
          description={
            !props.hasAnyRows
              ? props.emptyDescription
              : 'Adjust the search or filters to see more workloads.'
          }
        />
      }
    >
      <div
        ref={(element) => {
          coverageElement = element;
          observedWidth.setElement(element);
        }}
        data-proxmox-backups-table="coverage"
        data-proxmox-backups-layout={layoutMode()}
      >
        <PlatformTableShell
          tableClass="min-w-0 table-fixed text-xs"
          colgroup={
            <colgroup>
              <For each={visibleColumns()}>
                {(column) => (
                  <col
                    style={getCoverageColumnWidthStyle(
                      column.id,
                      layoutMode(),
                      visibleColumnIds(),
                      containerWidth() ?? undefined,
                    )}
                    data-proxmox-backups-column={column.id}
                  />
                )}
              </For>
            </colgroup>
          }
          header={
            <>
              <SortableHead
                label="Workload"
                sortKey="workload"
                currentSort={props.sortKey}
                direction={props.sortDirection}
                onSort={props.onSort}
                align="left"
                headClass={`${getPlatformTableHeadClassForKind('name')} platform-table-mobile-w-30`}
              />
              <Show when={columnVisible('type')}>
                <TableHead class={getPlatformTableHeadClassForKind('text')}>Type</TableHead>
              </Show>
              <Show when={columnVisible('targetId')}>
                <TableHead class={getPlatformTableHeadClassForKind('text')}>
                  {PROXMOX_BACKUP_COLUMN_LABELS.targetId}
                </TableHead>
              </Show>
              <Show when={columnVisible('node')}>
                <TableHead class={getPlatformTableHeadClassForKind('text')}>Node</TableHead>
              </Show>
              <SortableHead
                label={layoutMode() === 'compact' ? 'Status' : 'Protection'}
                sortKey="posture"
                currentSort={props.sortKey}
                direction={props.sortDirection}
                onSort={props.onSort}
                align="left"
                headClass={getPlatformTableHeadClassForKind('text')}
              />
              <SortableHead
                label={
                  layoutMode() === 'compact'
                    ? 'Age'
                    : layoutMode() === 'basic'
                      ? 'Backup'
                      : 'Last backup'
                }
                sortKey="latest"
                currentSort={props.sortKey}
                direction={props.sortDirection}
                onSort={props.onSort}
                align="right"
                headClass={getPlatformTableHeadClassForKind('numeric-value')}
              />
              <Show when={columnVisible('task')}>
                <SortableHead
                  label={layoutMode() === 'compact' ? 'Job' : 'Task'}
                  sortKey="task"
                  currentSort={props.sortKey}
                  direction={props.sortDirection}
                  onSort={props.onSort}
                  align="left"
                  headClass={getPlatformTableHeadClassForKind('text')}
                />
              </Show>
            </>
          }
          body={
            <>
              <PlatformWindowedRows
                items={renderedRows}
                keyExtractor={(row) => row.key}
                colSpan={columnCount()}
                estimatedRowHeight={44}
                reserveScrollExtent
              >
                {(row) => {
                  const isExpanded = () => props.expandedKey === row.key;
                  const evidence = createMemo(() =>
                    selectCoverageRestoreEvidence(row.artifacts, evidenceLimit()),
                  );
                  const undatedCount = () =>
                    row.artifacts.filter((artifact) => artifact.createdMs === undefined).length;
                  const backupHistoryComplete = () =>
                    !props.historyReadIncomplete && hasCompleteBackupHistory(row);
                  const detailRowId = () => `proxmox-coverage-evidence-${row.key}`;
                  const evidenceColumnCount = () =>
                    3 +
                    (isCoverageEvidenceColumnVisible(layoutMode(), 'location') ? 1 : 0) +
                    (isCoverageEvidenceColumnVisible(layoutMode(), 'size') ? 1 : 0);
                  return (
                    <>
                      <TableRow
                        ref={(row) => props.retainInspectionFocus?.(row)}
                        {...getPlatformResourceDetailRowInteractionProps({
                          expanded: isExpanded(),
                          onToggle: () => props.onToggleExpand(row.key),
                        })}
                        data-proxmox-backup-row="coverage"
                      >
                        <TableCell
                          class={`${getPlatformTableCellClassForKind('name')} text-base-content`}
                        >
                          <div class="flex min-w-0 items-center gap-2">
                            <PlatformResourceDetailToggleButton
                              expanded={isExpanded()}
                              resourceLabel={getBackupWorkloadDisclosureLabel(row.workload)}
                              controlsId={detailRowId()}
                              hideWhenRowTappableOnMobile={false}
                              onToggle={() => props.onToggleExpand(row.key)}
                            />
                            <div class="min-w-0 flex-1">
                              <div class="flex min-w-0 items-baseline gap-1.5">
                                <span class="truncate font-semibold" title={row.workload.label}>
                                  {row.workload.name || row.workload.label}
                                </span>
                                <Show
                                  when={
                                    showIdentityBesideName(row) &&
                                    !columnVisible('targetId') &&
                                    row.workload.name &&
                                    row.workload.name.trim() !==
                                      `${row.workload.typeLabel} ${row.workload.vmid}` &&
                                    (row.workload.type === 'vm' || row.workload.type === 'ct')
                                  }
                                >
                                  <span class="shrink-0 text-[10px] text-muted">
                                    {row.workload.typeLabel} {row.workload.vmid}
                                  </span>
                                </Show>
                                <Show
                                  when={
                                    showIdentityBesideName(row) &&
                                    showWorkloadConnection() &&
                                    getBackupWorkloadPVEConnection(row.workload)
                                  }
                                >
                                  {(connection) => (
                                    <span
                                      class="min-w-0 truncate text-[10px] text-muted"
                                      title={`PVE connection: ${connection()}`}
                                      data-backup-workload-connection
                                    >
                                      <span class="sr-only">PVE: </span>
                                      {connection()}
                                    </span>
                                  )}
                                </Show>
                              </div>
                            </div>
                          </div>
                        </TableCell>
                        <Show when={columnVisible('type')}>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('text')} text-base-content`}
                          >
                            <ProxmoxBackupWorkloadTypeBadge
                              type={row.workload.type}
                              label={row.workload.typeLabel}
                            />
                          </TableCell>
                        </Show>
                        <Show when={columnVisible('targetId')}>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('text')} text-muted font-mono text-[11px] tabular-nums`}
                          >
                            {row.workload.vmid || '—'}
                          </TableCell>
                        </Show>
                        <Show when={columnVisible('node')}>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('text')} text-base-content`}
                          >
                            <Show
                              when={row.workload.node}
                              fallback={<span class="text-muted">—</span>}
                            >
                              {(node) => (
                                <span class="block truncate" title={node()}>
                                  {node()}
                                </span>
                              )}
                            </Show>
                          </TableCell>
                        </Show>
                        <TableCell
                          class={`${getPlatformTableCellClassForKind('text')} ${
                            layoutMode() === 'compact' ? '!pl-1.5 !pr-0.5' : ''
                          }`}
                        >
                          <div
                            class={`flex items-center ${
                              layoutMode() === 'compact' ? 'gap-1.5' : 'gap-2'
                            }`}
                          >
                            <StatusDot
                              size="sm"
                              variant={coveragePostureVariant(row.posture)}
                              title={postureExplanation(row)}
                              ariaHidden
                            />
                            <span
                              title={postureExplanation(row)}
                              class={`truncate text-[11px] font-medium ${statusWordToneClass(
                                coveragePostureVariant(row.posture),
                              )}`}
                            >
                              <Show
                                when={layoutMode() === 'compact'}
                                fallback={getWorkloadRecoveryPostureLabel(row.posture)}
                              >
                                <span class="sr-only">
                                  {getWorkloadRecoveryPostureLabel(row.posture)}:{' '}
                                  {postureExplanation(row)}
                                </span>
                                <span aria-hidden="true">
                                  {compactBackupReasonLabel(row) ??
                                    compactPostureLabel(row.posture)}
                                </span>
                              </Show>
                            </span>
                          </div>
                        </TableCell>
                        <TableCell class={ageCellClass('numeric-value')}>
                          <Show
                            when={row.latestBackup}
                            fallback={
                              <CoverageAgeFallback
                                unknown={row.ageUnknown?.backup}
                                historyUnknown={
                                  !backupHistoryComplete() ||
                                  Boolean(row.protectionPosture?.lastSuccessfulPointAt)
                                }
                                compact={layoutMode() === 'compact'}
                                emptyTitle={
                                  row.snapshotCount > 0
                                    ? COVERAGE_SNAPSHOT_ONLY_TITLE
                                    : COVERAGE_NO_BACKUP_TITLE
                                }
                              />
                            }
                          >
                            {(artifact) => (
                              <ProxmoxBackupAgeText
                                artifact={artifact()}
                                compact={layoutMode() === 'compact'}
                              />
                            )}
                          </Show>
                        </TableCell>
                        <Show when={columnVisible('task')}>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('text')} text-base-content`}
                          >
                            <Show
                              when={row.latestTask}
                              fallback={
                                <Show
                                  when={!row.taskCandidates?.length}
                                  fallback={
                                    <div
                                      class="flex items-center justify-center gap-2 sm:justify-start"
                                      title={TASK_ORDER_NOTICE}
                                    >
                                      <StatusDot size="sm" variant="warning" ariaHidden />
                                      <span
                                        class={
                                          layoutMode() === 'compact'
                                            ? 'sr-only'
                                            : 'text-amber-600 dark:text-amber-300'
                                        }
                                      >
                                        <span class="sr-only">Latest task: </span>Unknown
                                      </span>
                                    </div>
                                  }
                                >
                                  <Show
                                    when={layoutMode() !== 'compact'}
                                    fallback={<span class="block text-center text-muted">—</span>}
                                  >
                                    <span class="text-muted" title="No recent task">
                                      {COVERAGE_EMPTY_LABEL}
                                    </span>
                                  </Show>
                                </Show>
                              }
                            >
                              {(task) => (
                                <Show
                                  when={layoutMode() !== 'compact'}
                                  fallback={
                                    <div class="flex justify-center">
                                      <StatusDot
                                        size="sm"
                                        variant={taskWordVariant(task().label)}
                                        title={`Latest task: ${task().label}`}
                                        ariaHidden
                                      />
                                      <span class="sr-only">Latest task: {task().label}</span>
                                    </div>
                                  }
                                >
                                  <div class="flex items-center gap-2">
                                    <StatusDot
                                      size="sm"
                                      variant={taskWordVariant(task().label)}
                                      title={task().label}
                                      ariaHidden
                                    />
                                    <span
                                      class={`truncate ${statusWordToneClass(
                                        taskWordVariant(task().label),
                                      )}`}
                                    >
                                      {task().label}
                                    </span>
                                  </div>
                                </Show>
                              )}
                            </Show>
                          </TableCell>
                        </Show>
                      </TableRow>
                      <Show when={isExpanded()}>
                        <InlineDetailTableRow
                          ref={(row) => props.retainInspectionFocus?.(row)}
                          cellId={detailRowId()}
                          class=""
                          colspan={columnCount()}
                          data-inline-detail-for={row.key}
                        >
                          <InlineDetailPanel
                            testId="proxmox-coverage-detail"
                            detailFor={row.key}
                            title={row.workload.name || row.workload.label}
                            status={{
                              variant: coveragePostureVariant(row.posture),
                              label: getWorkloadRecoveryPostureLabel(row.posture),
                            }}
                            summary={postureExplanation(row)}
                            actions={
                              <RecordOwnerLink
                                subject={proxmoxBackupWorkloadSubject(row.workload)}
                              />
                            }
                            sections={[]}
                            onClose={() => props.onToggleExpand(row.key)}
                          >
                            <Show when={row.latestTask?.label === 'Failed'}>
                              <InlineNotice
                                tone="warning"
                                role="status"
                                class="text-xs whitespace-normal wrap-anywhere"
                                data-coverage-failed-task
                              >
                                Last backup job failed:{' '}
                                {row.latestTask?.error?.trim() || 'no error reported'}
                              </InlineNotice>
                            </Show>
                            <Show when={row.latestBackup}>
                              {(backup) => (
                                <p
                                  class="flex min-w-0 items-baseline gap-1.5 text-xs text-muted"
                                  data-coverage-newest-backup
                                >
                                  <span class="shrink-0">Newest backup</span>
                                  <ProxmoxBackupAgeText artifact={backup()} />
                                  <span class="min-w-0 truncate text-base-content">
                                    {backup().location}
                                  </span>
                                </p>
                              )}
                            </Show>
                            <Show when={props.evidenceLocationLabel}>
                              {(label) => (
                                <InlineNotice
                                  tone="info"
                                  role="note"
                                  aria-label="Restore point scope"
                                  class="text-xs whitespace-normal wrap-anywhere"
                                >
                                  Restore points: {label()}. Protection assessments and task history
                                  cover the whole workload, not just this location.
                                </InlineNotice>
                              )}
                            </Show>
                            <div class="overflow-hidden rounded-sm border border-border bg-surface">
                              <Show
                                when={evidence().length > 0}
                                fallback={
                                  <InlineNotice
                                    tone={
                                      row.posture === 'unprotected' || row.posture === 'attention'
                                        ? 'warning'
                                        : 'info'
                                    }
                                    role="status"
                                    class="text-xs"
                                  >
                                    {!backupHistoryComplete()
                                      ? 'Restore-point history is not confirmed complete. Missing entries do not mean no backups exist.'
                                      : 'No restore-point details are visible in the current list. The protection status above may use previously recorded evidence.'}
                                  </InlineNotice>
                                }
                              >
                                <div class="flex items-center justify-between gap-2 bg-surface-alt px-2 py-1 text-[10px] font-semibold uppercase tracking-wide text-muted">
                                  <span>Restore points</span>
                                  <Show when={row.artifacts.length > evidence().length}>
                                    <span class="normal-case tracking-normal">
                                      Showing {evidence().length} of {row.artifacts.length}
                                    </span>
                                  </Show>
                                </div>
                                <Show when={undatedCount() > 0}>
                                  <p class="px-2 py-2 text-[11px] leading-snug text-amber-700 dark:text-amber-300 whitespace-normal wrap-anywhere">
                                    {undatedCount()}{' '}
                                    {undatedCount() === 1 ? 'entry has' : 'entries have'} an
                                    unavailable or future date. These entries cannot be placed in
                                    time.
                                    <Show when={row.artifacts.length > evidence().length}>
                                      {' '}
                                      The preview includes a sample from each affected source and
                                      state.
                                    </Show>
                                  </p>
                                </Show>
                                <PlatformDetailTable class="table-fixed text-[11px]">
                                  <PlatformDetailTableHeader>
                                    <TableHead class={getPlatformTableHeadClassForKind('name')}>
                                      Source
                                    </TableHead>
                                    <Show
                                      when={isCoverageEvidenceColumnVisible(
                                        layoutMode(),
                                        'location',
                                      )}
                                    >
                                      <TableHead class={getPlatformTableHeadClassForKind('text')}>
                                        Location
                                      </TableHead>
                                    </Show>
                                    <TableHead
                                      class={getPlatformTableHeadClassForKind('numeric-value')}
                                    >
                                      {PROXMOX_BACKUP_COLUMN_LABELS.created}
                                    </TableHead>
                                    <Show
                                      when={isCoverageEvidenceColumnVisible(layoutMode(), 'size')}
                                    >
                                      <TableHead
                                        class={getPlatformTableHeadClassForKind('numeric-value')}
                                      >
                                        Size
                                      </TableHead>
                                    </Show>
                                    <TableHead class={getPlatformTableHeadClassForKind('badge')}>
                                      State
                                    </TableHead>
                                  </PlatformDetailTableHeader>
                                  <PlatformDetailTableBody class="divide-border-subtle">
                                    <For each={evidence()}>
                                      {(artifact) => {
                                        const open = () => openPoints().has(artifact.id);
                                        return (
                                          <>
                                            <TableRow
                                              {...getPlatformResourceDetailRowInteractionProps({
                                                expanded: open(),
                                                onToggle: () => togglePoint(artifact.id),
                                              })}
                                              data-coverage-evidence-point={artifact.id}
                                            >
                                              <TableCell
                                                class={getPlatformTableCellClassForKind('name')}
                                              >
                                                <div class="flex min-w-0 items-center gap-1.5">
                                                  <PlatformResourceDetailToggleButton
                                                    expanded={open()}
                                                    resourceLabel={`${artifact.sourceLabel} ${artifact.nativeId}`}
                                                    hideWhenRowTappableOnMobile={false}
                                                    onToggle={() => togglePoint(artifact.id)}
                                                  />
                                                  <div class="shrink-0">
                                                    <ArtifactSourceIdentity artifact={artifact} />
                                                  </div>
                                                </div>
                                              </TableCell>
                                              <Show
                                                when={isCoverageEvidenceColumnVisible(
                                                  layoutMode(),
                                                  'location',
                                                )}
                                              >
                                                <TableCell
                                                  class={`${getPlatformTableCellClassForKind('text')} text-base-content`}
                                                >
                                                  <span
                                                    class="block truncate"
                                                    title={artifact.location}
                                                    data-coverage-evidence-location
                                                  >
                                                    {artifact.location}
                                                  </span>
                                                </TableCell>
                                              </Show>
                                              <TableCell
                                                class={`${getPlatformTableCellClassForKind('numeric-value')} text-base-content`}
                                              >
                                                <ProxmoxBackupAgeText artifact={artifact} />
                                              </TableCell>
                                              <Show
                                                when={isCoverageEvidenceColumnVisible(
                                                  layoutMode(),
                                                  'size',
                                                )}
                                              >
                                                <TableCell
                                                  class={`${getPlatformTableCellClassForKind('numeric-value')} tabular-nums text-base-content`}
                                                >
                                                  {formatPlatformTableBytesValue(
                                                    artifact.size,
                                                    'Unknown size',
                                                  )}
                                                </TableCell>
                                              </Show>
                                              <TableCell
                                                class={getPlatformTableCellClassForKind('badge')}
                                              >
                                                <ArtifactStateBadge
                                                  artifact={artifact}
                                                  label={artifactStateLabel(artifact)}
                                                />
                                              </TableCell>
                                            </TableRow>
                                            <Show when={open()}>
                                              <TableRow>
                                                <TableCell
                                                  class="whitespace-normal"
                                                  colspan={evidenceColumnCount()}
                                                >
                                                  {coverageRestorePointDetails(artifact)}
                                                </TableCell>
                                              </TableRow>
                                            </Show>
                                          </>
                                        );
                                      }}
                                    </For>
                                  </PlatformDetailTableBody>
                                </PlatformDetailTable>
                              </Show>
                              <Show
                                when={
                                  row.artifacts.length > COVERAGE_RESTORE_EVIDENCE_PAGE_SIZE ||
                                  evidenceLimit() > COVERAGE_RESTORE_EVIDENCE_PAGE_SIZE
                                }
                              >
                                <div class="border-t border-border px-2 py-2">
                                  <button
                                    type="button"
                                    class="rounded-sm px-2 py-1 text-xs font-medium text-accent hover:bg-surface-hover focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-accent"
                                    aria-disabled={evidence().length >= row.artifacts.length}
                                    onClick={() => {
                                      if (evidence().length < row.artifacts.length)
                                        setEvidenceLimit(
                                          evidence().length + COVERAGE_RESTORE_EVIDENCE_PAGE_SIZE,
                                        );
                                    }}
                                  >
                                    <Show
                                      when={evidence().length < row.artifacts.length}
                                      fallback={
                                        <>
                                          All {evidence().length} restore{' '}
                                          {evidence().length === 1 ? 'point' : 'points'} shown
                                        </>
                                      }
                                    >
                                      Show{' '}
                                      {Math.min(
                                        COVERAGE_RESTORE_EVIDENCE_PAGE_SIZE,
                                        row.artifacts.length - evidence().length,
                                      )}{' '}
                                      more restore points
                                    </Show>
                                  </button>
                                </div>
                              </Show>
                            </div>
                            <ResourceDetailDrawerSupportDisclosure
                              title="More details"
                              expanded={moreDetails()}
                              onToggle={() => setMoreDetails((value) => !value)}
                              showLabel="Show"
                              hideLabel="Hide"
                              contentClass="mt-2 space-y-3"
                              dataTestId="coverage-more-details"
                              accessibleSubject="more details"
                              contentId={`${detailRowId()}-more`}
                            >
                              <DetailSectionTable
                                sections={buildCoverageDetailSections(row)}
                                wrapValues
                              />
                              <CoverageTaskCandidates tasks={row.taskCandidates ?? []} />
                            </ResourceDetailDrawerSupportDisclosure>
                          </InlineDetailPanel>
                        </InlineDetailTableRow>
                      </Show>
                    </>
                  );
                }}
              </PlatformWindowedRows>
            </>
          }
        />
      </div>
    </Show>
  );
}
