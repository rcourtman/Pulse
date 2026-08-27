import { For, Show, createMemo, type Accessor, type JSX } from 'solid-js';

import { StatusDot } from '@/components/shared/StatusDot';
import { TableCell, TableHead, TableRow } from '@/components/shared/Table';
import {
  formatPlatformTableBytesValue,
  formatPlatformTableIntegerValue,
  formatPlatformTablePercentValue,
  formatPlatformTableUptimeValue,
  getPlatformTableCellClassForKind,
  getPlatformTableHeadClassForKind,
  PlatformResponsiveTableLabel,
  PlatformTableNumberValue,
  PlatformTablePercentValue,
  PlatformTableShell,
  PlatformWindowedRows,
} from '@/features/platformPage/sharedPlatformPage';
import {
  createPlatformResourceDetailState,
  getPlatformResourceDetailRowInteractionProps,
  PlatformResourceDetailTableRow,
  PlatformResourceDetailToggleButton,
} from '@/features/platformPage/PlatformResourceDetailTableRow';
import type { PBSBackup } from '@/types/api';
import type { Resource, ResourcePBSDatastore } from '@/types/resource';
import { getNormalizedIdentityLookupVariants } from '@/utils/resourceIdentity';
import type { StatusIndicatorVariant } from '@/utils/status';
import { useObservedElementWidth } from '@/hooks/useObservedElementWidth';

import {
  getBackupServerColumns,
  getBackupServerColumnWidthStyle,
  getBackupServerLayoutForContainer,
  type BackupServerColumnId,
} from './proxmoxBackupsTablePresentation';

// "Backup servers" answers the two questions the coverage table can't: is my
// PBS reachable, and is its datastore about to fill? Datastore fill is the
// headline backup risk — a full datastore silently fails every future backup —
// so it lives here on the Backups page, not buried on the platform Storage tab
// where the rows read as generic "PVE" storage. One row per datastore, labelled
// by its server; a server with no datastore data still gets a reachability row.
// Host CPU/memory/uptime ride along on each of the server's rows. The shared
// table appears on Overview for estate health and on Backups beside recovery
// evidence, so both paths open the same canonical PBS/agent detail drawer.

interface BackupServerRow {
  key: string;
  resource: Resource;
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

const identityValues = (resource: Resource): Array<string | undefined> => [
  resource.canonicalIdentity?.hostname,
  resource.canonicalIdentity?.platformId,
  resource.identity?.hostname,
  ...(resource.identity?.ips ?? []),
  resource.agent?.hostname,
  resource.pbs?.hostname,
  resource.pbs?.instanceId,
  resource.platformId,
  resource.name,
  resource.displayName,
];

const identityTokens = (resource: Resource): Set<string> =>
  new Set(identityValues(resource).flatMap((value) => getNormalizedIdentityLookupVariants(value)));

const stringValues = (...candidates: unknown[]): string[] =>
  candidates.flatMap((candidate) =>
    Array.isArray(candidate)
      ? candidate.filter((value): value is string => typeof value === 'string')
      : [],
  );

const uniquelyCorrelatedAgent = (
  server: Resource,
  candidates: readonly Resource[],
): Resource | undefined => {
  const serverTokens = identityTokens(server);
  if (serverTokens.size === 0) return undefined;
  const matches = candidates.filter((candidate) => {
    if (candidate.type !== 'agent') return false;
    for (const token of identityTokens(candidate)) {
      if (serverTokens.has(token)) return true;
    }
    return false;
  });
  return matches.length === 1 ? matches[0] : undefined;
};

const mergePBSAgentPresentation = (server: Resource, agent: Resource): Resource => {
  const serverPlatform = server.platformData ?? {};
  const agentPlatform = agent.platformData ?? {};
  const sources = Array.from(
    new Set([
      ...stringValues(server.sources, serverPlatform.sources),
      ...stringValues(agent.sources, agentPlatform.sources),
    ]),
  );
  return {
    ...server,
    sourceType: 'hybrid',
    sources,
    cpu: agent.cpu ?? server.cpu,
    memory: agent.memory ?? server.memory,
    disk: agent.disk ?? server.disk,
    network: agent.network ?? server.network,
    diskIO: agent.diskIO ?? server.diskIO,
    temperature: agent.temperature ?? server.temperature,
    uptime: agent.uptime ?? server.uptime,
    agent: agent.agent ?? agentPlatform.agent ?? server.agent,
    metricsTarget: agent.metricsTarget ?? server.metricsTarget,
    discoveryTarget: agent.discoveryTarget ?? server.discoveryTarget,
    lastSeen: Math.max(server.lastSeen, agent.lastSeen),
    platformData: {
      ...agentPlatform,
      ...serverPlatform,
      sources,
      agent: agent.agent ?? agentPlatform.agent ?? serverPlatform.agent,
      pbs: server.pbs ?? serverPlatform.pbs,
    },
  };
};

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
  const sortedServers = servers
    .filter((resource) => resource.type === 'pbs')
    .map((server) => {
      const agent = uniquelyCorrelatedAgent(server, servers);
      return agent ? mergePBSAgentPresentation(server, agent) : server;
    })
    .slice()
    .sort((left, right) => left.name.localeCompare(right.name) || left.id.localeCompare(right.id));
  for (const server of sortedServers) {
    const datastores = (server.pbs?.datastores ?? [])
      .slice()
      .sort((left, right) => left.name.localeCompare(right.name));
    const memoryTotal = server.memory?.total ?? 0;
    const host = {
      resource: server,
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
  showBackupCounts?: boolean;
  emptyIcon?: JSX.Element;
  layoutWidth?: Accessor<number | null | undefined>;
}) {
  const rows = () => buildBackupServerRows(props.servers, props.backups ?? []);
  const observedWidth = useObservedElementWidth();
  const layoutMode = createMemo(() => {
    const width = props.layoutWidth?.() ?? observedWidth.width();
    return typeof width === 'number' && width > 0
      ? getBackupServerLayoutForContainer(width)
      : 'full';
  });
  const visibleColumns = createMemo(() =>
    getBackupServerColumns(layoutMode()).filter(
      (column) => props.showBackupCounts !== false || column.id !== 'backups',
    ),
  );
  const detail = createPlatformResourceDetailState({ idPrefix: 'proxmox-backup-server-detail' });
  const columnVisible = (column: BackupServerColumnId) =>
    visibleColumns().some((candidate) => candidate.id === column);

  return (
    <Show when={rows().length > 0}>
      <div
        ref={observedWidth.setElement}
        data-proxmox-backups-table="servers"
        data-proxmox-backups-layout={layoutMode()}
      >
        <PlatformTableShell
          tableClass="min-w-[0px] table-fixed text-xs"
          colgroup={
            <colgroup>
              <For each={visibleColumns()}>
                {(column) => (
                  <col
                    style={getBackupServerColumnWidthStyle(column.id, layoutMode())}
                    data-proxmox-backups-column={column.id}
                  />
                )}
              </For>
            </colgroup>
          }
          header={
            <>
              <TableHead
                class={`${getPlatformTableHeadClassForKind('name')} platform-table-mobile-w-30 md:w-[15%]`}
              >
                <PlatformResponsiveTableLabel compact="Server" full="Backup server" />
              </TableHead>
              <TableHead
                class={`${getPlatformTableHeadClassForKind('text')} platform-table-mobile-w-15 md:w-[10%]`}
              >
                <PlatformResponsiveTableLabel compact="State" full="Status" />
              </TableHead>
              <Show when={columnVisible('version')}>
                <TableHead class={getPlatformTableHeadClassForKind('text')}>Version</TableHead>
              </Show>
              <Show when={columnVisible('cpu')}>
                <TableHead class={getPlatformTableHeadClassForKind('numeric-value')}>CPU</TableHead>
              </Show>
              <Show when={columnVisible('memory')}>
                <TableHead class={getPlatformTableHeadClassForKind('numeric-value')}>
                  Memory
                </TableHead>
              </Show>
              <Show when={columnVisible('uptime')}>
                <TableHead class={getPlatformTableHeadClassForKind('numeric-value')}>
                  Uptime
                </TableHead>
              </Show>
              <Show when={columnVisible('datastore')}>
                <TableHead
                  class={`${getPlatformTableHeadClassForKind('text')} platform-table-mobile-w-20 md:w-[13%]`}
                >
                  <PlatformResponsiveTableLabel compact="Store" full="Datastore" />
                </TableHead>
              </Show>
              <TableHead
                class={`${getPlatformTableHeadClassForKind('numeric-value')} platform-table-mobile-w-20 md:w-[15%]`}
              >
                Used
              </TableHead>
              <Show when={columnVisible('backups')}>
                <TableHead
                  class={`${getPlatformTableHeadClassForKind('numeric-value')} platform-table-mobile-w-15 md:w-[8%]`}
                  aria-label="Backups"
                  title="Backups"
                >
                  <PlatformResponsiveTableLabel
                    compact="Bkps"
                    full={layoutMode() === 'full' ? 'Backups' : 'Count'}
                  />
                </TableHead>
              </Show>
              <Show when={columnVisible('dedup')}>
                <TableHead class={getPlatformTableHeadClassForKind('numeric-value')}>
                  Dedup
                </TableHead>
              </Show>
            </>
          }
          body={
            <>
              <PlatformWindowedRows items={rows} estimatedRowHeight={32}>
                {(row) => {
                  const pct = () => (row.datastore ? usagePercent(row.datastore) : undefined);
                  const rowIdentity = { id: row.key };
                  const isExpanded = () => detail.isExpanded(rowIdentity);
                  const detailRowId = () => detail.detailRowId(rowIdentity);
                  return (
                    <>
                      <TableRow
                        {...getPlatformResourceDetailRowInteractionProps({
                          expanded: isExpanded(),
                          detailRowId: detailRowId(),
                          onToggle: () => detail.toggle(rowIdentity),
                        })}
                      >
                        <TableCell
                          class={`${getPlatformTableCellClassForKind('name')} text-base-content truncate font-medium`}
                          title={[row.serverName, row.datastore?.name].filter(Boolean).join(' · ')}
                        >
                          <div class="flex min-w-0 items-center gap-1">
                            <PlatformResourceDetailToggleButton
                              expanded={isExpanded()}
                              resourceLabel={row.serverName}
                              controlsId={detailRowId()}
                              onToggle={() => detail.toggle(rowIdentity)}
                            />
                            <div class="min-w-0 truncate" title={row.serverName}>
                              {row.serverName}
                            </div>
                          </div>
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
                        <Show when={columnVisible('version')}>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('text')} text-muted truncate text-[11px]`}
                          >
                            {row.version || '—'}
                          </TableCell>
                        </Show>
                        <Show when={columnVisible('cpu')}>
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
                        </Show>
                        <Show when={columnVisible('memory')}>
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
                              <Show when={row.memoryTotal && layoutMode() === 'full'}>
                                <span class="ml-1 text-[10px] text-muted tabular-nums">
                                  {`(${formatPlatformTableBytesValue(row.memoryUsed, '0 B')}/${formatPlatformTableBytesValue(row.memoryTotal)})`}
                                </span>
                              </Show>
                            </Show>
                          </TableCell>
                        </Show>
                        <Show when={columnVisible('uptime')}>
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
                        </Show>
                        <Show when={columnVisible('datastore')}>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('text')} text-base-content truncate font-mono text-[11px]`}
                          >
                            {row.datastore?.name ?? '—'}
                          </TableCell>
                        </Show>
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
                                <Show when={layoutMode() !== 'compact'}>
                                  <span class="truncate text-[10px] text-muted tabular-nums">
                                    {formatPlatformTableBytesValue(datastore().used, '0 B')} /{' '}
                                    {formatPlatformTableBytesValue(datastore().total)}
                                  </span>
                                </Show>
                              </div>
                            )}
                          </Show>
                        </TableCell>
                        <Show when={columnVisible('backups')}>
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
                        </Show>
                        <Show when={columnVisible('dedup')}>
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
                        </Show>
                      </TableRow>
                      <PlatformResourceDetailTableRow
                        resource={row.resource}
                        open={isExpanded()}
                        detailRowId={detailRowId()}
                        colSpan={visibleColumns().length}
                        initialShowHostDetails
                        onClose={() => detail.close(rowIdentity)}
                      />
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

export default ProxmoxBackupServersTable;
