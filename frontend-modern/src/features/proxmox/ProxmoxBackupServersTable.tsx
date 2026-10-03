import { For, Show, createMemo, type Accessor, type JSX } from 'solid-js';
import { unwrap } from 'solid-js/store';

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
import { getPlatformAgentRecord } from '@/utils/agentResources';
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
// Host CPU/memory/uptime ride along on each of the server's rows: PBS hosts
// left the v5 nodes table in the v6 IA, so this is where their health lives.

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

// Only the hostname PBS reports about itself can corroborate a host without
// a backend link. Connection labels, instance IDs, canonical presentation
// fields and configured endpoints are not machine identity. In particular,
// the shared dotted-token helper would make unrelated IPs/FQDNs collide.
const machineHostname = (value: unknown): string =>
  typeof value === 'string' ? value.trim().toLowerCase().replace(/\.$/, '') : '';

const correlationEvidenceKey = (server: Resource): string | undefined => {
  const linkedAgentId = server.pbs?.linkedAgentId?.trim();
  if (linkedAgentId) return `agent:${linkedAgentId}`;
  const nodeName = machineHostname(server.pbs?.nodeName);
  return nodeName ? `node:${nodeName}` : undefined;
};

const stringValues = (...candidates: unknown[]): string[] =>
  candidates.flatMap((candidate) =>
    Array.isArray(candidate)
      ? candidate.filter((value): value is string => typeof value === 'string')
      : [],
  );

const isGuestWithAgent = (resource: Resource): boolean =>
  (resource.type === 'vm' || resource.type === 'system-container') &&
  Boolean(resource.agent ?? resource.platformData?.agent);

// The agent identity is the same on a PVE guest and on the standalone host row
// that represents the same machine. It is the only reliable way to tell one
// host surfaced twice from two genuinely different hosts sharing a name.
const correlatedAgentKey = (resource: Resource): string | undefined => {
  const direct = resource.agent?.agentId?.trim();
  if (direct) return direct;
  const platformAgent = resource.platformData?.agent;
  if (platformAgent && typeof platformAgent === 'object') {
    const agentId = (platformAgent as { agentId?: unknown }).agentId;
    if (typeof agentId === 'string' && agentId.trim()) return agentId.trim();
  }
  return undefined;
};

// PVE-only nodes are also type 'agent'. They must not masquerade as an agent
// host merely because their hostname matches the PBS service (#1723).
const isHostWithAgent = (candidate: Resource): boolean =>
  (candidate.type === 'agent' || isGuestWithAgent(candidate)) &&
  Boolean(candidate.agent ?? candidate.platformData?.agent);

const matchingAgentHosts = (server: Resource, candidates: readonly Resource[]): Resource[] => {
  // Endpoint/interface and provider guest links belong to the backend. An
  // explicit link also vetoes same-name Agents with a different identity.
  const linkedAgentId = server.pbs?.linkedAgentId?.trim();
  if (linkedAgentId) {
    return candidates.filter(
      (candidate) => isHostWithAgent(candidate) && correlatedAgentKey(candidate) === linkedAgentId,
    );
  }
  const nodeName = machineHostname(server.pbs?.nodeName);
  if (!nodeName) return [];
  return candidates.filter((candidate) => {
    if (!isHostWithAgent(candidate) || !correlatedAgentKey(candidate)) return false;
    const agent = candidate.agent ?? getPlatformAgentRecord(candidate);
    // Do not let stale host observations undo a withdrawn backend link.
    const reportSeen =
      typeof agent?.lastReportAt === 'string' ? Date.parse(agent.lastReportAt) : candidate.lastSeen;
    const delta = Math.abs(server.lastSeen - reportSeen);
    return (
      agent?.stale !== true &&
      server.lastSeen > 0 &&
      reportSeen > 0 &&
      delta <= PBS_CORRELATION_RETENTION_MAX_STALENESS_MS &&
      machineHostname(agent?.hostname) === nodeName
    );
  });
};

const preferredCorrelatedHost = (matches: readonly Resource[]): Resource | undefined =>
  matches.find((match) => isGuestWithAgent(match) && match.metricsTarget) ??
  matches.find((match) => match.metricsTarget) ??
  matches[0];

const uniquelyCorrelatedAgent = (
  server: Resource,
  candidates: readonly Resource[],
): Resource | undefined => {
  const matches = matchingAgentHosts(server, candidates);
  if (matches.length === 0) return undefined;
  if (matches.length === 1) return matches[0];

  // A single agent can surface twice: folded into its PVE guest and as the
  // standalone host row. Those are one host, not an ambiguous pair. Collapse by
  // agent identity and prefer the guest, whose metrics target carries the
  // persisted history the Backups drawer renders.
  const byAgentKey = new Map<string, Resource[]>();
  for (const match of matches) {
    const key = correlatedAgentKey(match);
    // Without an agent identity we cannot prove the rows are the same host, so
    // stay conservative and decline to guess.
    if (!key) return undefined;
    const bucket = byAgentKey.get(key);
    if (bucket) {
      bucket.push(match);
    } else {
      byAgentKey.set(key, [match]);
    }
  }
  if (byAgentKey.size !== 1) return undefined;
  const group = Array.from(byAgentKey.values())[0];
  return preferredCorrelatedHost(group);
};

// True when the snapshot still offers a host row for this server, even if the
// match is ambiguous and `uniquelyCorrelatedAgent` declines to choose. The
// distinction matters for correlation retention: a snapshot that simply omits
// the host row is a transient refresh gap, while an ambiguous snapshot is a
// deliberate decline that must not be papered over with a remembered guess.
const hasCorrelationCandidate = (server: Resource, candidates: readonly Resource[]): boolean =>
  matchingAgentHosts(server, candidates).length > 0;

// A live refresh can briefly omit the correlated host row (for example while a
// realtime snapshot replaces the merged estate), which used to flip the Backups
// drawer's Identity and History target between the host series and the PBS
// service key. Keep the last resolved correlation per PBS server and reuse it
// only across such an omission, and only while the remembered host is still
// plausibly current, so a genuinely removed or replaced host is not advertised
// indefinitely. A host that is present but ambiguous still declines.
interface RetainedPbsCorrelation {
  agent: Resource;
  evidenceKey?: string;
}

export type PbsCorrelationRetention = Map<string, RetainedPbsCorrelation>;

export const createPbsCorrelationRetention = (): PbsCorrelationRetention => new Map();

const PBS_CORRELATION_RETENTION_MAX_STALENESS_MS = 5 * 60 * 1000;

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
  retention?: PbsCorrelationRetention,
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
  const pbsServers = servers.filter((resource) => resource.type === 'pbs');
  const sortedServers = pbsServers
    .map((server) => {
      const agent = uniquelyCorrelatedAgent(server, servers);
      if (agent) {
        retention?.set(server.id, {
          agent,
          evidenceKey: correlationEvidenceKey(server),
        });
        return mergePBSAgentPresentation(server, agent);
      }
      const retained = retention?.get(server.id);
      if (retained) {
        // A changed or withdrawn backend link is evidence that the old host
        // correlation is no longer trusted. A transiently missing row is not
        // enough to keep advertising that host's History after revocation.
        if (correlationEvidenceKey(server) !== retained.evidenceKey) {
          retention?.delete(server.id);
          return server;
        }
        const fresh =
          server.lastSeen - retained.agent.lastSeen <= PBS_CORRELATION_RETENTION_MAX_STALENESS_MS;
        if (fresh && !hasCorrelationCandidate(server, servers)) {
          return mergePBSAgentPresentation(server, retained.agent);
        }
        // An ambiguous match revokes the remembered choice too; a later
        // omission must not resurrect an identity we already declined.
        retention?.delete(server.id);
      }
      return server;
    })
    .slice()
    .sort((left, right) => left.name.localeCompare(right.name) || left.id.localeCompare(right.id));
  if (retention) {
    const presentIds = new Set(pbsServers.map((server) => server.id));
    for (const id of retention.keys()) {
      if (!presentIds.has(id)) retention.delete(id);
    }
  }
  for (const server of sortedServers) {
    const datastores = (server.pbs?.datastores ?? [])
      .slice()
      .sort((left, right) => left.name.localeCompare(right.name));
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
    // Each keyed row has a reconciled Solid store. Give it its own JSON DTO
    // snapshot: otherwise joining an Agent mutates the source's service
    // metricsTarget (or another datastore row), defeating later revocation.
    // Unwrap reactive source DTOs before cloning; never clone their proxies.
    const resourceSnapshot = () => structuredClone(unwrap(server));
    if (datastores.length === 0) {
      rows.push({ key: server.id, ...host, resource: resourceSnapshot(), backupCount: 0 });
      continue;
    }
    for (const datastore of datastores) {
      rows.push({
        key: `${server.id}:${datastore.name}`,
        ...host,
        resource: resourceSnapshot(),
        // Row stores reconcile independently from the nested PBS snapshot. Do
        // not alias its datastore object: a reordered snapshot reconciles that
        // array by position and would overwrite this row's datastore identity.
        datastore: { ...datastore },
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
  layoutWidth?: Accessor<number | null | undefined>;
}) {
  const retention = createPbsCorrelationRetention();
  const rows = createMemo(() =>
    buildBackupServerRows(props.servers, props.backups ?? [], retention),
  );
  const observedWidth = useObservedElementWidth();
  const layoutMode = createMemo(() => {
    const width = props.layoutWidth?.() ?? observedWidth.width();
    return typeof width === 'number' && width > 0
      ? getBackupServerLayoutForContainer(width)
      : 'full';
  });
  const visibleColumns = createMemo(() => getBackupServerColumns(layoutMode()));
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
              <TableHead class={`${getPlatformTableHeadClassForKind('name')}`}>
                <PlatformResponsiveTableLabel compact="Server" full="Backup server" />
              </TableHead>
              <TableHead class={`${getPlatformTableHeadClassForKind('text')}`}>
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
                <TableHead class={`${getPlatformTableHeadClassForKind('text')}`}>
                  <PlatformResponsiveTableLabel compact="Store" full="Datastore" />
                </TableHead>
              </Show>
              <TableHead class={`${getPlatformTableHeadClassForKind('numeric-value')}`}>
                Used
              </TableHead>
              <Show when={columnVisible('backups')}>
                <TableHead
                  class={`${getPlatformTableHeadClassForKind('numeric-value')}`}
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
              <PlatformWindowedRows
                items={rows}
                keyExtractor={(row) => row.key}
                estimatedRowHeight={32}
              >
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
                                <Show when={layoutMode() !== 'compact' && layoutMode() !== 'basic'}>
                                  <StatusDot
                                    size="sm"
                                    variant={usageVariant(pct())}
                                    title={`Datastore ${formatPlatformTablePercentValue(pct())} used`}
                                    ariaHidden
                                  />
                                </Show>
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
