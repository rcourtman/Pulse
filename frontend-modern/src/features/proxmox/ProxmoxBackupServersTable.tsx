import { For, Show, type JSX } from 'solid-js';

import { StatusDot } from '@/components/shared/StatusDot';
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/shared/Table';
import { TableCard } from '@/components/shared/TableCard';
import { formatBytes } from '@/utils/format';
import {
  PLATFORM_TABLE_BODY_CLASS,
  PLATFORM_TABLE_CARD_CLASS,
  PLATFORM_TABLE_HEADER_ROW_CLASS,
  getPlatformTableCellClassForKind,
  getPlatformTableHeadClassForKind,
} from '@/features/platformPage/sharedPlatformPage';
import type { Resource, ResourcePBSDatastore } from '@/types/resource';
import type { StatusIndicatorVariant } from '@/utils/status';

// "Backup servers" answers the two questions the coverage table can't: is my
// PBS reachable, and is its datastore about to fill? Datastore fill is the
// headline backup risk — a full datastore silently fails every future backup —
// so it lives here on the Backups page, not buried on the platform Storage tab
// where the rows read as generic "PVE" storage. One row per datastore, labelled
// by its server; a server with no datastore data still gets a reachability row.

interface BackupServerRow {
  key: string;
  serverName: string;
  online: boolean;
  connectionLabel: string;
  version?: string;
  datastore?: ResourcePBSDatastore;
}

function serverIsOnline(resource: Resource): boolean {
  const status = (resource.status ?? '').toLowerCase();
  const conn = (resource.pbs?.connectionHealth ?? '').toLowerCase();
  if (conn) return conn === 'healthy' || conn === 'ok';
  return status === 'online' || status === 'running';
}

function connectionLabel(resource: Resource): string {
  const conn = resource.pbs?.connectionHealth?.trim();
  if (conn) return conn.charAt(0).toUpperCase() + conn.slice(1);
  return serverIsOnline(resource) ? 'Online' : 'Offline';
}

function usagePercent(datastore: ResourcePBSDatastore): number | undefined {
  if (typeof datastore.usagePercent === 'number') return datastore.usagePercent;
  if (datastore.total > 0) return (datastore.used / datastore.total) * 100;
  return undefined;
}

// >=90% is the silent-backup-failure danger zone; >=75% is the early warning.
function usageVariant(pct: number | undefined): StatusIndicatorVariant {
  if (pct === undefined) return 'muted';
  if (pct >= 90) return 'danger';
  if (pct >= 75) return 'warning';
  return 'success';
}

function usageToneClass(pct: number | undefined): string {
  if (pct === undefined) return 'text-muted';
  if (pct >= 90) return 'text-red-600 dark:text-red-300';
  if (pct >= 75) return 'text-amber-600 dark:text-amber-300';
  return 'text-base-content';
}

export function buildBackupServerRows(servers: readonly Resource[]): BackupServerRow[] {
  const rows: BackupServerRow[] = [];
  // `model().pbs` is scope-filtered (proxmox-pbs), which also catches PBS
  // *datastore* storage resources (type 'storage', sources ['pbs']). This table
  // is about the server, so keep only actual PBS server instances — otherwise a
  // datastore renders as a phantom offline "server" row.
  for (const server of servers.filter((resource) => resource.type === 'pbs')) {
    const datastores = server.pbs?.datastores ?? [];
    if (datastores.length === 0) {
      rows.push({
        key: server.id,
        serverName: server.name,
        online: serverIsOnline(server),
        connectionLabel: connectionLabel(server),
        version: server.pbs?.version,
      });
      continue;
    }
    for (const datastore of datastores) {
      rows.push({
        key: `${server.id}:${datastore.name}`,
        serverName: server.name,
        online: serverIsOnline(server),
        connectionLabel: connectionLabel(server),
        version: server.pbs?.version,
        datastore,
      });
    }
  }
  return rows;
}

export function ProxmoxBackupServersTable(props: {
  servers: readonly Resource[];
  emptyIcon?: JSX.Element;
}) {
  const rows = () => buildBackupServerRows(props.servers);

  return (
    <Show when={rows().length > 0}>
      <TableCard class={PLATFORM_TABLE_CARD_CLASS}>
        <Table class="min-w-[760px] table-fixed text-xs">
          <colgroup>
            <col style={{ width: '22%' }} />
            <col style={{ width: '14%' }} />
            <col style={{ width: '12%' }} />
            <col style={{ width: '20%' }} />
            <col style={{ width: '22%' }} />
            <col style={{ width: '10%' }} />
          </colgroup>
          <TableHeader>
            <TableRow class={PLATFORM_TABLE_HEADER_ROW_CLASS}>
              <TableHead class={getPlatformTableHeadClassForKind('name')}>Backup server</TableHead>
              <TableHead class={getPlatformTableHeadClassForKind('text')}>Status</TableHead>
              <TableHead class={getPlatformTableHeadClassForKind('text')}>Version</TableHead>
              <TableHead class={getPlatformTableHeadClassForKind('text')}>Datastore</TableHead>
              <TableHead class={getPlatformTableHeadClassForKind('numeric-value')}>Used</TableHead>
              <TableHead class={getPlatformTableHeadClassForKind('numeric-value')}>Dedup</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody class={PLATFORM_TABLE_BODY_CLASS}>
            <For each={rows()}>
              {(row) => {
                const pct = () => (row.datastore ? usagePercent(row.datastore) : undefined);
                return (
                  <TableRow class="hover:bg-surface-hover">
                    <TableCell
                      class={`${getPlatformTableCellClassForKind('name')} text-base-content truncate font-medium`}
                    >
                      {row.serverName}
                    </TableCell>
                    <TableCell class={getPlatformTableCellClassForKind('text')}>
                      <div class="flex items-center gap-2">
                        <StatusDot
                          size="sm"
                          variant={row.online ? 'success' : 'danger'}
                          title={row.connectionLabel}
                          ariaHidden
                        />
                        <span class="truncate text-[11px] text-base-content">
                          {row.connectionLabel}
                        </span>
                      </div>
                    </TableCell>
                    <TableCell
                      class={`${getPlatformTableCellClassForKind('text')} text-muted truncate text-[11px]`}
                    >
                      {row.version || '—'}
                    </TableCell>
                    <TableCell
                      class={`${getPlatformTableCellClassForKind('text')} text-base-content truncate font-mono text-[11px]`}
                    >
                      {row.datastore?.name ?? '—'}
                    </TableCell>
                    <TableCell class={getPlatformTableCellClassForKind('numeric-value')}>
                      <Show
                        when={row.datastore}
                        fallback={<span class="text-muted">No datastore data</span>}
                      >
                        {(datastore) => (
                          <div class="flex items-center justify-end gap-2">
                            <StatusDot
                              size="sm"
                              variant={usageVariant(pct())}
                              title={`Datastore ${Math.round(pct() ?? 0)}% used`}
                              ariaHidden
                            />
                            <span class={`tabular-nums font-medium ${usageToneClass(pct())}`}>
                              <Show when={pct() !== undefined} fallback="—">
                                {Math.round(pct() ?? 0)}%
                              </Show>
                            </span>
                            <span class="text-[10px] text-muted tabular-nums">
                              {formatBytes(datastore().used)} / {formatBytes(datastore().total)}
                            </span>
                          </div>
                        )}
                      </Show>
                    </TableCell>
                    <TableCell
                      class={`${getPlatformTableCellClassForKind('numeric-value')} text-muted tabular-nums text-[11px]`}
                    >
                      <Show
                        when={row.datastore?.deduplicationFactor}
                        fallback={<span class="text-muted">—</span>}
                      >
                        {(factor) => <>{factor().toFixed(1)}×</>}
                      </Show>
                    </TableCell>
                  </TableRow>
                );
              }}
            </For>
          </TableBody>
        </Table>
      </TableCard>
    </Show>
  );
}

export default ProxmoxBackupServersTable;
