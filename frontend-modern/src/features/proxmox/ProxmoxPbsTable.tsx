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
import type { PBSBackup } from '@/types/api';

import { pbsRepositoryLabel, pbsWorkloadLabel, type PBSSortKey } from './proxmoxBackupsTableModel';
import { RowMetricBar, SortableHead } from './proxmoxBackupsTableShared';

const hasValidBackupTime = (backup: PBSBackup): boolean =>
  !!backup.backupTime && Number.isFinite(Date.parse(backup.backupTime));

// "Source details > PBS artifacts" table: every Proxmox Backup Server snapshot
// with repository, age, size, verification, and protection. Presentational —
// the parent owns the filtered + sorted memo, shared filters, and the PBS
// resource; this component just renders its error / loading / empty / table
// states. table-fixed + colgroup keeps columns from ballooning on wide views.
export function ProxmoxPbsTable(props: {
  backups: PBSBackup[];
  hasAnyArtifacts: boolean;
  errorMessage?: string;
  isLoading: boolean;
  onRefresh: () => void;
  emptyIcon: JSX.Element;
  emptyTitle: string;
  emptyDescription: string;
  sortKey: Accessor<PBSSortKey>;
  sortDirection: Accessor<'asc' | 'desc'>;
  onSort: (key: PBSSortKey) => void;
  sizeMaxBytes: number;
}) {
  return (
    <Show
      when={!props.errorMessage}
      fallback={
        <Card padding="lg">
          <EmptyState
            icon={props.emptyIcon}
            title="Could not load PBS artifacts"
            description={props.errorMessage ?? 'Refresh to retry.'}
            actions={
              <button
                type="button"
                onClick={() => props.onRefresh()}
                class="inline-flex min-h-10 items-center rounded-md border border-border px-3 py-2 text-sm font-medium hover:bg-surface-hover"
              >
                Refresh
              </button>
            }
          />
        </Card>
      }
    >
      <Show
        when={!props.isLoading}
        fallback={
          <Card padding="lg">
            <EmptyState
              icon={props.emptyIcon}
              title="Loading PBS artifacts"
              description="Reading deduplicated backup snapshots from Proxmox Backup Server."
            />
          </Card>
        }
      >
        <Show
          when={props.backups.length > 0}
          fallback={
            <Card padding="lg">
              <EmptyState
                icon={props.emptyIcon}
                title={
                  !props.hasAnyArtifacts ? props.emptyTitle : 'No PBS artifacts match current filters'
                }
                description={
                  !props.hasAnyArtifacts
                    ? props.emptyDescription
                    : 'Adjust the search or status filter to see more PBS artifacts.'
                }
              />
            </Card>
          }
        >
          <TableCard class={PLATFORM_TABLE_CARD_CLASS}>
            <Table class="min-w-[1050px] table-fixed text-xs">
              <colgroup>
                <col style={{ width: '15%' }} />
                <col style={{ width: '17%' }} />
                <col style={{ width: '13%' }} />
                <col style={{ width: '11%' }} />
                <col style={{ width: '15%' }} />
                <col style={{ width: '10%' }} />
                <col style={{ width: '11%' }} />
                <col style={{ width: '8%' }} />
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
                  <SortableHead
                    label="Repository"
                    sortKey="repository"
                    currentSort={props.sortKey}
                    direction={props.sortDirection}
                    onSort={props.onSort}
                    align="left"
                    headClass={getPlatformTableHeadClassForKind('text')}
                  />
                  <TableHead class={getPlatformTableHeadClassForKind('text')}>Instance</TableHead>
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
                    label="Verified"
                    sortKey="verified"
                    currentSort={props.sortKey}
                    direction={props.sortDirection}
                    onSort={props.onSort}
                    align="left"
                    headClass={getPlatformTableHeadClassForKind('text')}
                  />
                  <SortableHead
                    label="Protection"
                    sortKey="protected"
                    currentSort={props.sortKey}
                    direction={props.sortDirection}
                    onSort={props.onSort}
                    align="left"
                    headClass={getPlatformTableHeadClassForKind('text')}
                  />
                  <TableHead class={getPlatformTableHeadClassForKind('text')}>Files</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody class={PLATFORM_TABLE_BODY_CLASS}>
                <For each={props.backups}>
                  {(backup) => (
                    <TableRow class="hover:bg-surface-hover">
                      <TableCell
                        class={`${getPlatformTableCellClassForKind('name')} text-base-content truncate font-semibold`}
                      >
                        {pbsWorkloadLabel(backup)}
                      </TableCell>
                      <TableCell
                        class={`${getPlatformTableCellClassForKind('text')} text-base-content truncate font-mono text-[11px]`}
                        title={pbsRepositoryLabel(backup)}
                      >
                        {pbsRepositoryLabel(backup)}
                      </TableCell>
                      <TableCell
                        class={`${getPlatformTableCellClassForKind('text')} text-base-content truncate font-mono text-[11px]`}
                        title={backup.instance}
                      >
                        {backup.instance || '—'}
                      </TableCell>
                      <TableCell
                        class={`${getPlatformTableCellClassForKind('numeric-value')} text-base-content`}
                      >
                        <Show
                          when={hasValidBackupTime(backup)}
                          fallback={<span class="text-muted">—</span>}
                        >
                          {formatRelativeTime(backup.backupTime, { compact: true })}
                        </Show>
                      </TableCell>
                      <TableCell
                        class={`${getPlatformTableCellClassForKind('metric-bar')} text-base-content`}
                      >
                        <RowMetricBar
                          valuePct={
                            props.sizeMaxBytes > 0 ? (backup.size / props.sizeMaxBytes) * 100 : 0
                          }
                          fillClass="bg-blue-500/40 dark:bg-blue-500/40"
                          label={formatBytes(backup.size)}
                          tooltip={`${formatBytes(backup.size)} (relative to largest PBS artifact in view)`}
                        />
                      </TableCell>
                      <TableCell
                        class={`${getPlatformTableCellClassForKind('text')} text-base-content`}
                      >
                        <Show
                          when={backup.verified}
                          fallback={
                            <span class="text-amber-600 dark:text-amber-300">Unverified</span>
                          }
                        >
                          <span class="inline-flex items-center rounded-sm bg-emerald-100 px-1.5 py-0.5 text-[10px] font-semibold text-emerald-700 dark:bg-emerald-900/40 dark:text-emerald-200">
                            Verified
                          </span>
                        </Show>
                      </TableCell>
                      <TableCell
                        class={`${getPlatformTableCellClassForKind('text')} text-base-content`}
                      >
                        <Show
                          when={backup.protected}
                          fallback={<span class="text-muted">Unprotected</span>}
                        >
                          <span class="inline-flex items-center rounded-sm bg-blue-100 px-1.5 py-0.5 text-[10px] font-semibold text-blue-700 dark:bg-blue-900/40 dark:text-blue-200">
                            Protected
                          </span>
                        </Show>
                      </TableCell>
                      <TableCell
                        class={`${getPlatformTableCellClassForKind('text')} text-base-content`}
                      >
                        <span
                          class="inline-block max-w-[16rem] truncate"
                          title={(backup.files ?? []).join(', ')}
                        >
                          {(backup.files ?? []).length > 0 ? `${backup.files.length} files` : '—'}
                        </span>
                      </TableCell>
                    </TableRow>
                  )}
                </For>
              </TableBody>
            </Table>
          </TableCard>
        </Show>
      </Show>
    </Show>
  );
}
