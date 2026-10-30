import { For, Show, createMemo, type Accessor, type JSX } from 'solid-js';

import {
  InlineDetailPanel,
  compactDetailRows,
  compactDetailSections,
  makeDetailRow,
  type DetailSection,
} from '@/components/shared/DetailSectionTable';
import { InlineDetailTableRow } from '@/components/shared/InlineDetailTableRow';
import { TableCell, TableHead, TableRow } from '@/components/shared/Table';
import type { StatusIndicator } from '@/utils/status';
import {
  getRecoveryFullDateLabel,
  recoveryDateKeyFromTimestamp,
} from '@/utils/recoveryDatePresentation';
import {
  formatPlatformTableBytesValue,
  formatPlatformTableDateTimeValue,
  getPlatformTableCellClassForKind,
  getPlatformTableHeadClassForKind,
  PlatformTableEmptyState,
  PlatformTableShell,
} from '@/features/platformPage/sharedPlatformPage';
import {
  PlatformResourceDetailToggleButton,
  createPlatformResourceDetailState,
  getPlatformResourceDetailRowInteractionProps,
} from '@/features/platformPage/PlatformResourceDetailTableRow';
import { RecordOwnerLink } from '@/features/platformPage/RecordOwnerLink';
import { useObservedElementWidth } from '@/hooks/useObservedElementWidth';

import type { RecoverableArtifact } from './proxmoxBackupRecoveryModel';
import type { RecoverableSortKey } from './proxmoxBackupsTableModel';
import {
  ArtifactSourceBadge,
  ArtifactSourceIdentity,
  ArtifactStateBadge,
  PROXMOX_BACKUP_COLUMN_LABELS,
  ProxmoxBackupAgeText,
  ProxmoxBackupWorkloadTypeBadge,
  RowMetricBar,
  SortableHead,
  artifactStateLabel,
  getBackupWorkloadPVEConnection,
  getBackupWorkloadNodeLabel,
  getBackupWorkloadDisclosureLabel,
} from './proxmoxBackupsTableShared';
import {
  getRecoverableColumns,
  getRecoverableColumnWidthStyle,
  getRecoverableLayoutForContainer,
  isCompactBackupIdentityLayout,
  type RecoverableColumnId,
} from './proxmoxBackupsTablePresentation';
import { getProxmoxBackupSourcePresentation } from './proxmoxBackupSourcePresentation';
import { proxmoxBackupWorkloadSubject } from './proxmoxBackupWorkloadSubject';
import { PlatformWindowedRows } from '@/features/platformPage/PlatformWindowedRows';

// Flat recoverable-artifact table. Parent state owns filtering, sorting, and
// date/source facets; optional day grouping is presentation only.

interface DayGroup {
  key: string;
  label: string;
  items: RecoverableArtifact[];
}

type RecoverableTableItem =
  | { kind: 'day'; key: string; label: string; count: number }
  | { kind: 'artifact'; key: string; artifact: RecoverableArtifact };

function groupByDay(artifacts: readonly RecoverableArtifact[]): DayGroup[] {
  const groups: DayGroup[] = [];
  let current: DayGroup | undefined;
  for (const artifact of artifacts) {
    const key =
      artifact.createdMs === undefined
        ? 'unknown'
        : recoveryDateKeyFromTimestamp(artifact.createdMs);
    if (!current || current.key !== key) {
      current = {
        key,
        label: key === 'unknown' ? 'Unknown date' : getRecoveryFullDateLabel(key),
        items: [],
      };
      groups.push(current);
    }
    current.items.push(artifact);
  }
  return groups;
}

// The drawer dot repeats the row's State badge: the same states, the same colours.
const artifactStateIndicator = (artifact: RecoverableArtifact): StatusIndicator => {
  const label = artifactStateLabel(artifact);
  if (artifact.running) return { variant: 'info', label };
  if (artifact.failed) return { variant: 'danger', label };
  if (artifact.sourceKind === 'snapshot') return { variant: 'info', label };
  if (artifact.verified === true) return { variant: 'success', label };
  if (artifact.verified === false) return { variant: 'warning', label };
  if (artifact.protected) return { variant: 'info', label };
  return { variant: 'muted', label };
};

const artifactVerificationText = (artifact: RecoverableArtifact): string => {
  if (artifact.verified === true) return 'Verified';
  if (artifact.verified === false) return 'Not verified';
  return artifact.sourceKind === 'snapshot' ? '' : 'Not reported';
};

// State badges report provider evidence, not a tested recovery. Keep those
// limits beside the open point's state, where keyboard and touch readers can
// read them without hovering a badge or leaving for workload coverage.
const artifactEvidenceLimit = (artifact: RecoverableArtifact): string => {
  if (artifact.sourceKind === 'snapshot') {
    return "This guest snapshot shares the guest's storage. It is not a separate backup and does not count towards backup protection.";
  }
  if (artifact.running) {
    return 'This backup is incomplete and may still be in progress. It is not a completed restore point.';
  }
  if (artifact.failed) {
    return 'No current writer is reported for this incomplete backup. It is not a completed restore point.';
  }
  if (artifact.verified === true) {
    return "Verified is the provider's reported backup check, not a tested restore.";
  }
  return artifact.verified === false
    ? 'This backup is not reported as verified. Its presence does not establish a successful restore.'
    : 'Backup verification is not reported. Its presence does not establish a successful restore.';
};

// Size and provider identifiers remain progressive detail. Workload and
// repository identity must also be readable before choosing a point on a phone.
const buildRecoverableDetailSections = (artifact: RecoverableArtifact): DetailSection[] => {
  const source = getProxmoxBackupSourcePresentation(artifact.sourceKind);
  return compactDetailSections([
    {
      label: 'Restore point',
      rows: compactDetailRows([
        makeDetailRow('Source', source.detailFallbackLabel, {
          title: artifact.sourceTitle ?? source.sourceTitle,
        }),
        makeDetailRow('Connection', artifact.locationConnection || 'Not reported', { wrap: true }),
        makeDetailRow('Location', artifact.location, { wrap: true }),
        makeDetailRow(
          PROXMOX_BACKUP_COLUMN_LABELS.created,
          artifact.createdMs === undefined
            ? 'Unknown'
            : formatPlatformTableDateTimeValue(artifact.createdMs),
          { tone: artifact.createdMs === undefined ? 'warning' : undefined },
        ),
        makeDetailRow(
          'Size',
          artifact.size && artifact.size > 0 ? formatPlatformTableBytesValue(artifact.size) : '',
        ),
      ]),
    },
    {
      label: 'State',
      rows: compactDetailRows([
        makeDetailRow('State', artifactStateLabel(artifact)),
        makeDetailRow('Verification', artifactVerificationText(artifact), {
          tone: artifact.verified === false ? 'warning' : undefined,
        }),
        makeDetailRow('Prune protection', artifact.protected ? 'Kept when pruning' : ''),
      ]),
      footerContent: (
        <div
          role="note"
          aria-label="Recovery evidence limits"
          class="space-y-1 whitespace-normal text-[11px] leading-snug text-muted wrap-anywhere"
        >
          <p>{artifactEvidenceLimit(artifact)}</p>
          <Show when={artifact.protected}>
            <p>
              Kept when pruning is a retention setting, not verification or workload protection.
            </p>
          </Show>
          <Show when={artifact.workload.type === 'vm' || artifact.workload.type === 'ct'}>
            <p>
              Backup and snapshot records do not establish that the guest has thawed or that its
              filesystems accept writes.
            </p>
          </Show>
        </div>
      ),
    },
    {
      label: 'Workload',
      rows: compactDetailRows([
        makeDetailRow('Type', artifact.workload.typeLabel),
        makeDetailRow(PROXMOX_BACKUP_COLUMN_LABELS.targetId, artifact.workload.vmid),
        makeDetailRow('Node', getBackupWorkloadNodeLabel(artifact.workload)),
        makeDetailRow('PVE connection', getBackupWorkloadPVEConnection(artifact.workload), {
          wrap: true,
        }),
      ]),
    },
    {
      label: PROXMOX_BACKUP_COLUMN_LABELS.details,
      rows: compactDetailRows([
        makeDetailRow('Identifier', artifact.detailTitle || artifact.detail, { wrap: true }),
      ]),
    },
  ]);
};

const RecoverableArtifactDetail = (props: {
  artifact: RecoverableArtifact;
  onClose: () => void;
}): JSX.Element => {
  const source = () => getProxmoxBackupSourcePresentation(props.artifact.sourceKind);
  return (
    <InlineDetailPanel
      testId="proxmox-recoverable-detail"
      detailFor={props.artifact.id}
      title={props.artifact.workload.name || props.artifact.workload.label}
      status={artifactStateIndicator(props.artifact)}
      summary={[source().detailFallbackLabel, props.artifact.location].filter(Boolean).join(' in ')}
      actions={<RecordOwnerLink subject={proxmoxBackupWorkloadSubject(props.artifact.workload)} />}
      sections={buildRecoverableDetailSections(props.artifact)}
      onClose={props.onClose}
    />
  );
};

export function ProxmoxRecoverableTable(props: {
  artifacts: RecoverableArtifact[];
  hasAnyArtifacts: boolean;
  emptyIcon: JSX.Element;
  emptyTitle: string;
  emptyDescription: string;
  sortKey: Accessor<RecoverableSortKey>;
  sortDirection: Accessor<'asc' | 'desc'>;
  onSort: (key: RecoverableSortKey) => void;
  sizeMaxBytes: number;
  groupByDay?: boolean;
  layoutWidth?: Accessor<number | null | undefined>;
}) {
  const observedWidth = useObservedElementWidth();
  const layoutMode = createMemo(() => {
    const width = props.layoutWidth?.() ?? observedWidth.width();
    return typeof width === 'number' && width > 0
      ? getRecoverableLayoutForContainer(width)
      : 'full';
  });
  const visibleColumns = createMemo(() => getRecoverableColumns(layoutMode()));
  const stackedIdentity = () => isCompactBackupIdentityLayout(layoutMode());
  const columnVisible = (column: RecoverableColumnId) =>
    visibleColumns().some((candidate) => candidate.id === column);
  const showDayGroups = () => props.groupByDay && props.sortKey() === 'created';
  const tableItems = createMemo<RecoverableTableItem[]>(() => {
    if (!showDayGroups()) {
      return props.artifacts.map((item) => ({
        kind: 'artifact',
        key: item.id,
        artifact: item,
      }));
    }

    return groupByDay(props.artifacts).flatMap((group) => [
      { kind: 'day' as const, key: group.key, label: group.label, count: group.items.length },
      ...group.items.map((item) => ({
        kind: 'artifact' as const,
        key: item.id,
        artifact: item,
      })),
    ]);
  });
  const drawer = createPlatformResourceDetailState({ idPrefix: 'proxmox-recoverable-detail' });

  const renderRow = (artifact: RecoverableArtifact): JSX.Element => (
    <>
      <TableRow
        {...getPlatformResourceDetailRowInteractionProps({
          expanded: drawer.isExpanded(artifact),
          onToggle: () => drawer.toggle(artifact),
          class: 'hover:bg-surface-hover',
        })}
        data-proxmox-backup-row="recoverable"
      >
        <TableCell class={`${getPlatformTableCellClassForKind('name')} text-base-content`}>
          <div class={`flex min-w-0 gap-2 ${stackedIdentity() ? 'items-start' : 'items-center'}`}>
            <PlatformResourceDetailToggleButton
              expanded={drawer.isExpanded(artifact)}
              resourceLabel={getBackupWorkloadDisclosureLabel(artifact.workload)}
              controlsId={drawer.detailRowId(artifact)}
              onToggle={() => drawer.toggle(artifact)}
            />
            <div class="min-w-0 flex-1">
              <span
                class={`block font-semibold ${stackedIdentity() ? 'whitespace-normal wrap-anywhere' : 'truncate'}`}
                title={artifact.workload.label}
              >
                {artifact.workload.name || artifact.workload.label}
              </span>
              <Show when={!columnVisible('targetId') && artifact.workload.vmid}>
                <span class="block text-[10px] text-muted">
                  {artifact.workload.typeLabel} {artifact.workload.vmid}
                </span>
              </Show>
              <Show when={getBackupWorkloadPVEConnection(artifact.workload)}>
                {(connection) => (
                  <span
                    class={`block text-[10px] text-muted ${stackedIdentity() ? 'whitespace-normal wrap-anywhere' : 'truncate'}`}
                    title={`PVE connection: ${connection()}`}
                    data-backup-workload-connection
                  >
                    PVE: {connection()}
                  </span>
                )}
              </Show>
              <Show when={stackedIdentity()}>
                <div class="mt-1 space-y-0.5 text-[10px] leading-snug">
                  <ArtifactSourceBadge artifact={artifact} wrap />
                  <span
                    class="block whitespace-normal text-muted wrap-anywhere"
                    data-backup-source-connection
                  >
                    Connection: {artifact.locationConnection || 'Unknown connection'}
                  </span>
                  <span
                    class="block whitespace-normal text-muted wrap-anywhere"
                    data-backup-location
                  >
                    Location: {artifact.location}
                  </span>
                </div>
              </Show>
            </div>
          </div>
        </TableCell>
        <Show when={columnVisible('type')}>
          <TableCell class={`${getPlatformTableCellClassForKind('text')} text-base-content`}>
            <ProxmoxBackupWorkloadTypeBadge
              type={artifact.workload.type}
              label={artifact.workload.typeLabel}
            />
          </TableCell>
        </Show>
        <Show when={columnVisible('targetId')}>
          <TableCell
            class={`${getPlatformTableCellClassForKind('text')} text-muted font-mono text-[11px] tabular-nums`}
          >
            {artifact.workload.vmid || '—'}
          </TableCell>
        </Show>
        <Show when={columnVisible('source')}>
          <TableCell class={`${getPlatformTableCellClassForKind('text')} text-base-content`}>
            <ArtifactSourceIdentity artifact={artifact} />
          </TableCell>
        </Show>
        <Show when={columnVisible('location')}>
          <TableCell class={`${getPlatformTableCellClassForKind('text')} text-base-content`}>
            <span class="block truncate" title={artifact.location}>
              {artifact.location}
            </span>
          </TableCell>
        </Show>
        <TableCell class={`${getPlatformTableCellClassForKind('numeric-value')} text-base-content`}>
          <ProxmoxBackupAgeText artifact={artifact} compact={layoutMode() === 'compact'} />
        </TableCell>
        <Show when={columnVisible('size')}>
          <TableCell class={`${getPlatformTableCellClassForKind('metric-bar')} text-base-content`}>
            <Show
              when={artifact.size && artifact.size > 0}
              fallback={<span class="text-muted">No size</span>}
            >
              <RowMetricBar
                valuePct={
                  props.sizeMaxBytes > 0 && artifact.size
                    ? (artifact.size / props.sizeMaxBytes) * 100
                    : 0
                }
                fillClass="bg-blue-500/40 dark:bg-blue-500/40"
                label={formatPlatformTableBytesValue(artifact.size)}
                tooltip={`${formatPlatformTableBytesValue(artifact.size)} (relative to largest artifact in view)`}
              />
            </Show>
          </TableCell>
        </Show>
        <TableCell class={`${getPlatformTableCellClassForKind('text')} text-base-content`}>
          <ArtifactStateBadge artifact={artifact} label={artifactStateLabel(artifact)} />
        </TableCell>
        <Show when={columnVisible('details')}>
          <TableCell class={`${getPlatformTableCellClassForKind('text')} text-base-content`}>
            <span class="block truncate" title={artifact.detailTitle ?? artifact.detail}>
              {artifact.detail || '—'}
            </span>
          </TableCell>
        </Show>
      </TableRow>
      <Show when={drawer.isExpanded(artifact)}>
        <InlineDetailTableRow
          cellId={drawer.detailRowId(artifact)}
          class=""
          colspan={visibleColumns().length}
          data-inline-detail-for={artifact.id}
        >
          <RecoverableArtifactDetail artifact={artifact} onClose={() => drawer.close(artifact)} />
        </InlineDetailTableRow>
      </Show>
    </>
  );

  return (
    <Show
      when={props.artifacts.length > 0}
      fallback={
        <PlatformTableEmptyState
          icon={props.emptyIcon}
          title={
            !props.hasAnyArtifacts
              ? props.emptyTitle
              : 'No recoverable artifacts match current filters'
          }
          description={
            !props.hasAnyArtifacts
              ? props.emptyDescription
              : 'Adjust the search, source filter, or selected day to see more artifacts.'
          }
        />
      }
    >
      <div
        ref={observedWidth.setElement}
        data-proxmox-backups-table="recoverable"
        data-proxmox-backups-layout={layoutMode()}
      >
        <PlatformTableShell
          tableClass="min-w-0 table-fixed text-xs"
          colgroup={
            <colgroup>
              <For each={visibleColumns()}>
                {(column) => (
                  <col
                    style={getRecoverableColumnWidthStyle(column.id, layoutMode())}
                    data-proxmox-backups-column={column.id}
                  />
                )}
              </For>
            </colgroup>
          }
          header={
            <>
              <SortableHead
                label={stackedIdentity() ? 'Restore point' : 'Workload'}
                sortKey="workload"
                currentSort={props.sortKey}
                direction={props.sortDirection}
                onSort={props.onSort}
                align="left"
                headClass={getPlatformTableHeadClassForKind('name')}
              />
              <Show when={columnVisible('type')}>
                <TableHead class={getPlatformTableHeadClassForKind('text')}>Type</TableHead>
              </Show>
              <Show when={columnVisible('targetId')}>
                <TableHead class={getPlatformTableHeadClassForKind('text')}>
                  {PROXMOX_BACKUP_COLUMN_LABELS.targetId}
                </TableHead>
              </Show>
              <Show when={columnVisible('source')}>
                <SortableHead
                  label="Source"
                  sortKey="source"
                  currentSort={props.sortKey}
                  direction={props.sortDirection}
                  onSort={props.onSort}
                  align="left"
                  headClass={getPlatformTableHeadClassForKind('text')}
                />
              </Show>
              <Show when={columnVisible('location')}>
                <SortableHead
                  label={layoutMode() === 'compact' ? 'Loc' : 'Location'}
                  sortKey="location"
                  currentSort={props.sortKey}
                  direction={props.sortDirection}
                  onSort={props.onSort}
                  align="left"
                  headClass={getPlatformTableHeadClassForKind('text')}
                />
              </Show>
              <SortableHead
                label={layoutMode() === 'compact' ? 'Age' : PROXMOX_BACKUP_COLUMN_LABELS.created}
                sortKey="created"
                currentSort={props.sortKey}
                direction={props.sortDirection}
                onSort={props.onSort}
                align="right"
                headClass={getPlatformTableHeadClassForKind('numeric-value')}
              />
              <Show when={columnVisible('size')}>
                <SortableHead
                  label="Size"
                  sortKey="size"
                  currentSort={props.sortKey}
                  direction={props.sortDirection}
                  onSort={props.onSort}
                  align="center"
                  headClass={getPlatformTableHeadClassForKind('metric-bar')}
                />
              </Show>
              <SortableHead
                label="State"
                sortKey="state"
                currentSort={props.sortKey}
                direction={props.sortDirection}
                onSort={props.onSort}
                align="left"
                headClass={getPlatformTableHeadClassForKind('text')}
              />
              <Show when={columnVisible('details')}>
                <TableHead class={getPlatformTableHeadClassForKind('text')}>
                  {PROXMOX_BACKUP_COLUMN_LABELS.details}
                </TableHead>
              </Show>
            </>
          }
          body={
            <>
              <PlatformWindowedRows
                items={tableItems}
                keyExtractor={(item) => `${item.kind}:${item.key}`}
                colSpan={visibleColumns().length}
                estimatedRowHeight={stackedIdentity() ? 128 : 44}
                reserveScrollExtent
              >
                {(item) => (
                  <Show
                    when={item.kind === 'artifact' ? item.artifact : undefined}
                    fallback={
                      <TableRow data-proxmox-backup-row="day">
                        {/* Cell-level background is reliable across table layout engines. */}
                        <TableCell
                          colspan={visibleColumns().length}
                          class="border-t border-border bg-surface-alt px-3 py-1.5 text-[11px] font-semibold uppercase tracking-[0.14em] text-base-content"
                        >
                          {item.kind === 'day' ? item.label : ''}{' '}
                          <span class="ml-2 normal-case tracking-normal text-muted">
                            {item.kind === 'day' ? item.count : 0}{' '}
                            {item.kind === 'day' && item.count === 1 ? 'entry' : 'entries'}
                          </span>
                        </TableCell>
                      </TableRow>
                    }
                  >
                    {(artifact) => renderRow(artifact())}
                  </Show>
                )}
              </PlatformWindowedRows>
            </>
          }
        />
      </div>
    </Show>
  );
}
