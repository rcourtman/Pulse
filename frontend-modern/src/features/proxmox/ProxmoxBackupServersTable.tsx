import { For, Show, type JSX } from 'solid-js';

import { StatusDot } from '@/components/shared/StatusDot';
import { TableCell, TableHead, TableRow } from '@/components/shared/Table';
import {
  formatPlatformTableBytesValue,
  formatPlatformTableIntegerValue,
  formatPlatformTablePercentValue,
  formatPlatformTableUptimeValue,
  getPlatformTableCellClassForKind,
  getPlatformTableHeadClassForKind,
  PlatformTableNumberValue,
  PlatformTablePercentValue,
  PlatformTableShell,
} from '@/features/platformPage/sharedPlatformPage';
import type { PBSBackup } from '@/types/api';
import type { Resource, ResourcePBSDatastore } from '@/types/resource';
import type { StatusIndicatorVariant } from '@/utils/status';

// "Backup servers" answers the two questions the coverage table can't: is my
// PBS reachable, and is its datastore about to fill? Datastore fill is the
// headline backup risk — a full datastore silently fails every future backup —
// so it lives here on the Backups page, not buried on the platform Storage tab
// where the rows read as generic "PVE" storage. One row per datastore, labelled
// by its server; a server with no datastore data still gets a reachability row.
// Host CPU/memory/uptime ride along on each of the server's rows: PBS hosts
// left the v5 nodes table in the v6 IA, so this is where their health lives.

interface BackupServerRow {
  key: string;
  serverName: string;
  online: boolean;
  connectionLabel: string;
  version?: string;
  cpuPercent?: number;
  memoryPercent?: number;
  memoryUsed?: number;
  memoryTotal?: number;
  uptimeSeconds?: number;
  datastore?: ResourcePBSDatastore;
  backupCount: number;
}

// Key by instance and datastore so multi-datastore servers get accurate counts.
function buildBackupCounts(backups: readonly PBSBackup[]): Map<string, number> {
  const counts = new Map<string, number>();
  for (const backup of backups) {
    const key = `${backup.instance ?? ''}::${backup.datastore ?? ''}`;
    counts.set(key, (counts.get(key) ?? 0) + 1);
  }
  return counts;
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

export function buildBackupServerRows(
  servers: readonly Resource[],
  backups: readonly PBSBackup[] = [],
): BackupServerRow[] {
  const rows: BackupServerRow[] = [];
  const counts = buildBackupCounts(backups);
  // PBS backups carry instance; resources may expose it under name or pbs.instanceId.
  const countFor = (server: Resource, datastoreName: string): number => {
    const ids = [server.name, server.pbs?.instanceId].filter(Boolean) as string[];
    for (const id of ids) {
      const n = counts.get(`${id}::${datastoreName}`);
      if (n !== undefined) return n;
    }
    return 0;
  };
  // `model().pbs` is scope-filtered (proxmox-pbs), which also catches PBS
  // *datastore* storage resources (type 'storage', sources ['pbs']). This table
  // is about the server, so keep only actual PBS server instances — otherwise a
  // datastore renders as a phantom offline "server" row.
  for (const server of servers.filter((resource) => resource.type === 'pbs')) {
    const datastores = server.pbs?.datastores ?? [];
    const memoryTotal = server.memory?.total ?? 0;
    const host = {
      serverName: server.name,
      online: serverIsOnline(server),
      connectionLabel: connectionLabel(server),
      version: server.pbs?.version,
      cpuPercent: typeof server.cpu?.current === 'number' ? server.cpu.current : undefined,
      memoryPercent:
        memoryTotal > 0
          ? ((server.memory?.used ?? 0) / memoryTotal) * 100
          : typeof server.memory?.current === 'number'
            ? server.memory.current
            : undefined,
      memoryUsed: server.memory?.used,
      memoryTotal: memoryTotal > 0 ? memoryTotal : undefined,
      uptimeSeconds: server.uptime ?? server.pbs?.uptimeSeconds,
    };
    if (datastores.length === 0) {
      rows.push({ key: server.id, ...host, backupCount: 0 });
      continue;
    }
    for (const datastore of datastores) {
      rows.push({
        key: `${server.id}:${datastore.name}`,
        ...host,
        datastore,
        backupCount: countFor(server, datastore.name),
      });
    }
  }
  return rows;
}

export function ProxmoxBackupServersTable(props: {
  servers: readonly Resource[];
  backups?: readonly PBSBackup[];
  emptyIcon?: JSX.Element;
}) {
  const rows = () => buildBackupServerRows(props.servers, props.backups ?? []);

  return (
    <Show when={rows().length > 0}>
      <PlatformTableShell
        tableClass="min-w-[1020px] table-fixed text-xs"
        colgroup={
          <colgroup>
            <col style={{ width: '16%' }} />
            <col style={{ width: '10%' }} />
            <col style={{ width: '8%' }} />
            <col style={{ width: '6%' }} />
            <col style={{ width: '11%' }} />
            <col style={{ width: '7%' }} />
            <col style={{ width: '13%' }} />
            <col style={{ width: '15%' }} />
            <col style={{ width: '7%' }} />
            <col style={{ width: '7%' }} />
          </colgroup>
        }
        header={
          <>
            <TableHead class={getPlatformTableHeadClassForKind('name')}>Backup server</TableHead>
            <TableHead class={getPlatformTableHeadClassForKind('text')}>Status</TableHead>
            <TableHead class={getPlatformTableHeadClassForKind('text')}>Version</TableHead>
            <TableHead class={getPlatformTableHeadClassForKind('numeric-value')}>CPU</TableHead>
            <TableHead class={getPlatformTableHeadClassForKind('numeric-value')}>Memory</TableHead>
            <TableHead class={getPlatformTableHeadClassForKind('numeric-value')}>Uptime</TableHead>
            <TableHead class={getPlatformTableHeadClassForKind('text')}>Datastore</TableHead>
            <TableHead class={getPlatformTableHeadClassForKind('numeric-value')}>Used</TableHead>
            <TableHead class={getPlatformTableHeadClassForKind('numeric-value')}>Backups</TableHead>
            <TableHead class={getPlatformTableHeadClassForKind('numeric-value')}>Dedup</TableHead>
          </>
        }
        body={
          <>
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
                      class={`${getPlatformTableCellClassForKind('numeric-value')} text-base-content`}
                    >
                      <Show
                        when={row.online && row.cpuPercent !== undefined}
                        fallback={<span class="text-muted">—</span>}
                      >
                        <PlatformTablePercentValue value={row.cpuPercent} />
                      </Show>
                    </TableCell>
                    <TableCell class={getPlatformTableCellClassForKind('numeric-value')}>
                      <Show
                        when={row.online && row.memoryPercent !== undefined}
                        fallback={<span class="text-muted">—</span>}
                      >
                        <span
                          class="text-base-content"
                          title={
                            row.memoryTotal
                              ? `${formatPlatformTableBytesValue(row.memoryUsed, '0 B')} / ${formatPlatformTableBytesValue(row.memoryTotal)}`
                              : undefined
                          }
                        >
                          <PlatformTablePercentValue value={row.memoryPercent} />
                        </span>
                        <Show when={row.memoryTotal}>
                          <span class="ml-1 text-[10px] text-muted tabular-nums">
                            {`(${formatPlatformTableBytesValue(row.memoryUsed, '0 B')}/${formatPlatformTableBytesValue(row.memoryTotal)})`}
                          </span>
                        </Show>
                      </Show>
                    </TableCell>
                    <TableCell
                      class={`${getPlatformTableCellClassForKind('numeric-value')} text-base-content tabular-nums`}
                    >
                      <Show
                        when={row.online && (row.uptimeSeconds ?? 0) > 0}
                        fallback={<span class="text-muted">—</span>}
                      >
                        {formatPlatformTableUptimeValue(row.uptimeSeconds)}
                      </Show>
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
                              title={`Datastore ${formatPlatformTablePercentValue(pct())} used`}
                              ariaHidden
                            />
                            <span class={`tabular-nums font-medium ${usageToneClass(pct())}`}>
                              <PlatformTablePercentValue value={pct()} />
                            </span>
                            <span class="text-[10px] text-muted tabular-nums">
                              {formatPlatformTableBytesValue(datastore().used, '0 B')} /{' '}
                              {formatPlatformTableBytesValue(datastore().total)}
                            </span>
                          </div>
                        )}
                      </Show>
                    </TableCell>
                    <TableCell
                      class={`${getPlatformTableCellClassForKind('numeric-value')} text-base-content`}
                    >
                      <Show when={row.datastore} fallback={<span class="text-muted">—</span>}>
                        <PlatformTableNumberValue
                          value={row.backupCount}
                          format={formatPlatformTableIntegerValue}
                        />
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
          </>
        }
      />
    </Show>
  );
}

export default ProxmoxBackupServersTable;
