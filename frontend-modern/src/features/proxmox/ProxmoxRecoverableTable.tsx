import { For, Show, type Accessor, type JSX } from 'solid-js';

import { Card } from '@/components/shared/Card';
import { EmptyState } from '@/components/shared/EmptyState';
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/shared/Table';
import { TableCard } from '@/components/shared/TableCard';
import { formatBytes, formatRelativeTime } from '@/utils/format';
import {
  getRecoveryFullDateLabel,
  recoveryDateKeyFromTimestamp,
} from '@/utils/recoveryDatePresentation';
import {
  PLATFORM_TABLE_BODY_CLASS,
  PLATFORM_TABLE_CARD_CLASS,
  PLATFORM_TABLE_HEADER_ROW_CLASS,
  getPlatformTableCellClassForKind,
  getPlatformTableHeadClassForKind,
} from '@/features/platformPage/sharedPlatformPage';

import type { RecoverableArtifact } from './proxmoxBackupRecoveryModel';
import type { RecoverableSortKey } from './proxmoxBackupsTableModel';
import {
  ArtifactSourceBadge,
  ArtifactStateBadge,
  RowMetricBar,
  SortableHead,
  artifactStateLabel,
} from './proxmoxBackupsTableShared';

// Flat recoverable-artifact table. Parent state owns filtering, sorting, and
// date/source facets; optional day grouping is presentation only.

const COLUMN_COUNT = 8;

interface DayGroup {
  key: string;
  label: string;
  items: RecoverableArtifact[];
}

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
}) {
  const showDayGroups = () => props.groupByDay && props.sortKey() === 'created';

  const renderRow = (artifact: RecoverableArtifact): JSX.Element => (
    <TableRow class="hover:bg-surface-hover">
      <TableCell class={`${getPlatformTableCellClassForKind('name')} text-base-content`}>
        <div class="min-w-0">
          <div class="truncate font-semibold">
            {artifact.workload.name || artifact.workload.label}
          </div>
        </div>
      </TableCell>
      <TableCell
        class={`${getPlatformTableCellClassForKind('numeric-value')} text-muted font-mono text-[11px] tabular-nums`}
      >
        {artifact.workload.vmid || '—'}
      </TableCell>
      <TableCell class={`${getPlatformTableCellClassForKind('text')} text-base-content`}>
        <ArtifactSourceBadge artifact={artifact} />
      </TableCell>
      <TableCell class={`${getPlatformTableCellClassForKind('text')} text-base-content`}>
        <span class="inline-block max-w-[16rem] truncate" title={artifact.location}>
          {artifact.location}
        </span>
      </TableCell>
      <TableCell class={`${getPlatformTableCellClassForKind('numeric-value')} text-base-content`}>
        {formatRelativeTime(artifact.createdAt, { compact: true })}
      </TableCell>
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
            label={formatBytes(artifact.size ?? 0)}
            tooltip={`${formatBytes(artifact.size ?? 0)} (relative to largest artifact in view)`}
          />
        </Show>
      </TableCell>
      <TableCell class={`${getPlatformTableCellClassForKind('text')} text-base-content`}>
        <ArtifactStateBadge artifact={artifact} label={artifactStateLabel(artifact)} />
      </TableCell>
      <TableCell class={`${getPlatformTableCellClassForKind('text')} text-base-content`}>
        <span class="inline-block max-w-[20rem] truncate" title={artifact.detail}>
          {artifact.detail || '—'}
        </span>
      </TableCell>
    </TableRow>
  );

  return (
    <Show
      when={props.artifacts.length > 0}
      fallback={
        <Card padding="lg">
          <EmptyState
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
        </Card>
      }
    >
      <TableCard class={PLATFORM_TABLE_CARD_CLASS}>
        <Table class="min-w-[1000px] table-fixed text-xs">
          {/* Keep column widths stable; long locations/details truncate with title text. */}
          <colgroup>
            <col style={{ width: '20%' }} />
            <col style={{ width: '7%' }} />
            <col style={{ width: '8%' }} />
            <col style={{ width: '15%' }} />
            <col style={{ width: '11%' }} />
            <col style={{ width: '14%' }} />
            <col style={{ width: '10%' }} />
            <col style={{ width: '15%' }} />
          </colgroup>
          <TableHeader>
            <TableRow class={PLATFORM_TABLE_HEADER_ROW_CLASS}>
              <SortableHead
                label="Workload"
                sortKey="workload"
                currentSort={props.sortKey}
                direction={props.sortDirection}
                onSort={props.onSort}
                align="left"
                headClass={getPlatformTableHeadClassForKind('name')}
              />
              <TableHead class={getPlatformTableHeadClassForKind('numeric-value')}>VMID</TableHead>
              <SortableHead
                label="Source"
                sortKey="source"
                currentSort={props.sortKey}
                direction={props.sortDirection}
                onSort={props.onSort}
                align="left"
                headClass={getPlatformTableHeadClassForKind('text')}
              />
              <SortableHead
                label="Location"
                sortKey="location"
                currentSort={props.sortKey}
                direction={props.sortDirection}
                onSort={props.onSort}
                align="left"
                headClass={getPlatformTableHeadClassForKind('text')}
              />
              <SortableHead
                label="Created"
                sortKey="created"
                currentSort={props.sortKey}
                direction={props.sortDirection}
                onSort={props.onSort}
                align="right"
                headClass={getPlatformTableHeadClassForKind('numeric-value')}
              />
              <SortableHead
                label="Size"
                sortKey="size"
                currentSort={props.sortKey}
                direction={props.sortDirection}
                onSort={props.onSort}
                align="center"
                headClass={getPlatformTableHeadClassForKind('metric-bar')}
              />
              <SortableHead
                label="State"
                sortKey="state"
                currentSort={props.sortKey}
                direction={props.sortDirection}
                onSort={props.onSort}
                align="left"
                headClass={getPlatformTableHeadClassForKind('text')}
              />
              <TableHead class={getPlatformTableHeadClassForKind('text')}>Details</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody class={PLATFORM_TABLE_BODY_CLASS}>
            <Show
              when={showDayGroups()}
              fallback={<For each={props.artifacts}>{(artifact) => renderRow(artifact)}</For>}
            >
              <For each={groupByDay(props.artifacts)}>
                {(group) => (
                  <>
                    <TableRow>
                      {/* Cell-level background is reliable across table layout engines. */}
                      <TableCell
                        colspan={COLUMN_COUNT}
                        class="border-t border-border bg-surface-alt px-3 py-1.5 text-[11px] font-semibold uppercase tracking-[0.14em] text-base-content"
                      >
                        {group.label}
                        {' '}
                        <span class="ml-2 normal-case tracking-normal text-muted">
                          {group.items.length} {group.items.length === 1 ? 'backup' : 'backups'}
                        </span>
                      </TableCell>
                    </TableRow>
                    <For each={group.items}>{(artifact) => renderRow(artifact)}</For>
                  </>
                )}
              </For>
            </Show>
          </TableBody>
        </Table>
      </TableCard>
    </Show>
  );
}
