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

// "Restore points" table: every recoverable artifact (PBS / archive / snapshot)
// across sources in one flat, sortable list. Presentational only — the parent
// owns the filtered + sorted memo and the shared search / day / source filters.
// Single-line rows: the workload identity (type/vmid) is already in the label.
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
}) {
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
        <Table class="min-w-[1150px] table-fixed text-xs">
          {/* table-fixed + colgroup so columns share the row width rather than
              dumping all the slack into the trailing Details column on wide
              viewports. */}
          <colgroup>
            <col style={{ width: '15%' }} />
            <col style={{ width: '7%' }} />
            <col style={{ width: '8%' }} />
            <col style={{ width: '17%' }} />
            <col style={{ width: '10%' }} />
            <col style={{ width: '12%' }} />
            <col style={{ width: '9%' }} />
            <col style={{ width: '22%' }} />
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
            <For each={props.artifacts}>
              {(artifact) => (
                <TableRow class="hover:bg-surface-hover">
                  <TableCell
                    class={`${getPlatformTableCellClassForKind('name')} text-base-content`}
                  >
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
                  <TableCell
                    class={`${getPlatformTableCellClassForKind('text')} text-base-content`}
                  >
                    <ArtifactSourceBadge artifact={artifact} />
                  </TableCell>
                  <TableCell
                    class={`${getPlatformTableCellClassForKind('text')} text-base-content`}
                  >
                    <span class="inline-block max-w-[16rem] truncate" title={artifact.location}>
                      {artifact.location}
                    </span>
                  </TableCell>
                  <TableCell
                    class={`${getPlatformTableCellClassForKind('numeric-value')} text-base-content`}
                  >
                    {formatRelativeTime(artifact.createdAt, { compact: true })}
                  </TableCell>
                  <TableCell
                    class={`${getPlatformTableCellClassForKind('metric-bar')} text-base-content`}
                  >
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
                  <TableCell
                    class={`${getPlatformTableCellClassForKind('text')} text-base-content`}
                  >
                    <ArtifactStateBadge artifact={artifact} label={artifactStateLabel(artifact)} />
                  </TableCell>
                  <TableCell
                    class={`${getPlatformTableCellClassForKind('text')} text-base-content`}
                  >
                    <span class="inline-block max-w-[20rem] truncate" title={artifact.detail}>
                      {artifact.detail || '—'}
                    </span>
                  </TableCell>
                </TableRow>
              )}
            </For>
          </TableBody>
        </Table>
      </TableCard>
    </Show>
  );
}
