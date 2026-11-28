import { For, Show, createMemo, createUniqueId, type Accessor, type JSX } from 'solid-js';
import { unwrap } from 'solid-js/store';

import { StatusDot } from '@/components/shared/StatusDot';
import { DrawerSubjectHeading } from '@/components/shared/DrawerSubjectHeading';
import { ObjectDrawerHeader } from '@/components/shared/ObjectDrawerHeader';
import {
  GROUPED_TABLE_ROW_META_CLASS,
  getGroupedTableRowCellClass,
  getGroupedTableRowClass,
} from '@/components/shared/groupedTableRowPresentation';
import { ResourceNameWithWebInterfaceLink } from '@/components/shared/WebInterfaceLink';
import { TableCell, TableHead, TableRow } from '@/components/shared/Table';
import { DetailSectionTable, type DetailValueTone } from '@/components/shared/DetailSectionTable';
import {
  formatPlatformTableBytesValue,
  formatPlatformTableIntegerValue,
  formatPlatformTablePercentValue,
  getPlatformTableCellClassForKind,
  getPlatformTableHeadClassForKind,
  PlatformResponsiveTableLabel,
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
import type { Alert, PBSBackup } from '@/types/api';
import type { Resource, ResourcePBSDatastore } from '@/types/resource';
import { getPlatformAgentRecord } from '@/utils/agentResources';
import {
  formatStatusLabel,
  isConnectedHealthStatus,
  OFFLINE_HEALTH_STATUSES,
  type StatusIndicatorVariant,
} from '@/utils/status';
import { useObservedElementWidth } from '@/hooks/useObservedElementWidth';

import {
  getBackupServerColumns,
  getBackupServerColumnWidthStyle,
  getBackupServerLayoutForContainer,
  type BackupServerColumnId,
} from './proxmoxBackupsTablePresentation';
import {
  describeDatastoreJobs,
  summarizeDatastoreJobs,
  type DatastoreJobState,
  type DatastoreJobSummary,
} from './proxmoxBackupServerJobs';
import {
  backupServerDatastoreAlertIds,
  getDatastoreFillForecast,
  isBackupServerDatastoreAlert,
  type DatastoreFillForecast,
} from './proxmoxBackupServerForecast';
import { isPBSServerPollStale } from './proxmoxBackupStaleness';

// "Backup servers" answers the two questions the coverage table can't: is my
// PBS reachable, and is its datastore about to fill? Datastore fill is the
// headline backup risk — a full datastore silently fails every future backup —
// so it lives here on the Backups page, not buried on the platform Storage tab
// where the rows read as generic "PVE" storage. Each datastore row also says
// whether its GC, prune, verify and sync jobs last succeeded, because a failed
// sync or verify leaves backups that exist but cannot be relied on. One row
// per datastore, grouped under a header row per PBS server that carries its
// reachability, web link and datastore count, the way guest tables group by
// node. Rows named by their server read as two servers; a muted server name
// on each datastore row still left "how many servers do I have?" unanswered.
// A server with no datastore data still gets a reachability row.
// PBS hosts left the v5 nodes table in the v6 IA, so this is where their health
// lives. The row says whether the server is reachable; its CPU, memory, uptime,
// version and their history are in the server's drawer, one click away.

interface BackupServerRow {
  key: string;
  resource: Resource;
  serverName: string;
  online: boolean;
  pollStale: boolean;
  connectionLabel: string;
  connectionVariant: StatusIndicatorVariant;
  datastore?: ResourcePBSDatastore;
  // The worst backend-judged GC, prune, verify or sync result for this store.
  jobs: DatastoreJobSummary;
  backupCount: number;
}

interface BackupServerGroup {
  key: string;
  // Only plain values: each windowed item is its own reconciled store, so the
  // header must not alias a datastore row's resource snapshot.
  customUrl?: string;
  serverName: string;
  online: boolean;
  pollStale: boolean;
  connectionLabel: string;
  connectionVariant: StatusIndicatorVariant;
  statusLabel: string;
  statusVariant: StatusIndicatorVariant;
  datastoreCount: number;
}

type BackupServerTableItem =
  { kind: 'server'; group: BackupServerGroup } | { kind: 'datastore'; row: BackupServerRow };

const PBS_DATA_STALE = 'PBS data stale';
const STALE_NOTE_CLASS = 'block text-[10px] font-medium text-amber-800 dark:text-amber-300';
const UNREAD_CAPACITY_NOTE = 'This does not mean the datastore is empty.';
const unreadCapacityLabel = (state: string) =>
  state === 'unavailable' ? 'Unavailable' : 'Unknown';
const datastoreByteUsage = (datastore: ResourcePBSDatastore) =>
  `${formatPlatformTableBytesValue(datastore.used, '0 B')} / ${formatPlatformTableBytesValue(datastore.total)}`;

function withPBSPollFreshness(
  status: { variant: StatusIndicatorVariant; label: string },
  pollStale: boolean,
): { variant: StatusIndicatorVariant; label: string } {
  if (!pollStale) return status;
  return {
    variant: status.variant === 'danger' ? 'danger' : 'warning',
    label: `${PBS_DATA_STALE} · Last status: ${status.label}`,
  };
}

// Combine server reachability with datastore job outcomes so a reachable PBS
// server with a failed or warned sync/verify/prune/GC job never displays a
// healthy green status dot beside a red job failure.
export function effectiveBackupServerStatus(
  connectionVariant: StatusIndicatorVariant,
  connectionLabel: string,
  jobSummaries: readonly DatastoreJobSummary[],
): { variant: StatusIndicatorVariant; label: string } {
  if (connectionVariant === 'danger') {
    return { variant: 'danger', label: connectionLabel };
  }
  const criticalJob = jobSummaries.find((summary) => summary.state === 'critical');
  if (criticalJob) {
    return { variant: 'danger', label: criticalJob.label };
  }
  if (connectionVariant === 'warning') {
    return { variant: 'warning', label: connectionLabel };
  }
  const warningJob = jobSummaries.find((summary) => summary.state === 'warning');
  if (warningJob) {
    return { variant: 'warning', label: warningJob.label };
  }
  return { variant: connectionVariant, label: connectionLabel };
}

// Rows arrive sorted by server, so each server's header precedes its rows.
export function groupBackupServerRows(rows: readonly BackupServerRow[]): BackupServerTableItem[] {
  const items: BackupServerTableItem[] = [];
  let current: BackupServerGroup | undefined;
  let currentJobs: DatastoreJobSummary[] = [];
  const finalizeCurrent = () => {
    if (!current) return;
    const status = withPBSPollFreshness(
      effectiveBackupServerStatus(current.connectionVariant, current.connectionLabel, currentJobs),
      current.pollStale,
    );
    current.statusVariant = status.variant;
    current.statusLabel = status.label;
  };
  for (const row of rows) {
    if (current?.key !== row.resource.id) {
      finalizeCurrent();
      currentJobs = [];
      current = {
        key: row.resource.id,
        customUrl: row.resource.customUrl,
        serverName: row.serverName,
        online: row.online,
        pollStale: row.pollStale,
        connectionLabel: row.connectionLabel,
        connectionVariant: row.connectionVariant,
        statusLabel: row.connectionLabel,
        statusVariant: row.connectionVariant,
        datastoreCount: 0,
      };
      items.push({ kind: 'server', group: current });
    }
    currentJobs.push(row.jobs);
    if (row.datastore) current.datastoreCount += 1;
    items.push({ kind: 'datastore', row });
  }
  finalizeCurrent();
  return items;
}

// Key by instance and datastore so multi-datastore servers get accurate counts.
function buildBackupCounts(backups: readonly PBSBackup[]): Map<string, Map<string, number>> {
  const counts = new Map<string, Map<string, number>>();
  for (const backup of backups) {
    // Both fields are provider identities, not a delimiter-joined display key.
    // An unattributed entry cannot establish a count for any named datastore.
    if (!backup.instance || !backup.datastore) continue;
    let datastores = counts.get(backup.instance);
    if (!datastores) {
      datastores = new Map();
      counts.set(backup.instance, datastores);
    }
    datastores.set(backup.datastore, (datastores.get(backup.datastore) ?? 0) + 1);
  }
  return counts;
}

// The resource model also accepts online/running/connected as successful PBS
// connection evidence. An explicit warning or unknown value must not fall
// through to a retained online status, nor turn into a failed connection.
function serverConnection(
  resource: Resource,
): Pick<BackupServerRow, 'online' | 'connectionLabel' | 'connectionVariant'> {
  const value = resource.pbs?.connectionHealth?.trim() || (resource.status ?? '').trim();
  const state = value.toLowerCase();
  const online = state === 'ok' || state === 'connected' || isConnectedHealthStatus(state);
  const connectionVariant = online
    ? 'success'
    : state === 'warning' || state === 'degraded'
      ? 'warning'
      : state === 'unhealthy' || (state !== 'inactive' && OFFLINE_HEALTH_STATUSES.has(state))
        ? 'danger'
        : 'muted';
  return {
    online,
    connectionLabel: formatStatusLabel(value),
    connectionVariant,
  };
}

function connectionToneClass(variant: StatusIndicatorVariant): string {
  if (variant === 'danger') return 'text-red-600 dark:text-red-400';
  if (variant === 'warning') return 'text-amber-600 dark:text-amber-300';
  return 'text-muted';
}

// PBS retains a named datastore when its status read fails, with zero-valued
// capacity and explicit unavailable/error evidence. Those zeros are not an
// empty datastore. Reachability alone cannot establish a successful store read.
function datastoreCapacity(
  datastore: ResourcePBSDatastore | undefined,
  connection: StatusIndicatorVariant,
) {
  if (!datastore) return { state: 'absent' as const, percent: undefined };
  const status =
    typeof datastore.status === 'string' ? datastore.status.trim().toLowerCase() : datastore.status;
  if (
    connection === 'danger' ||
    (typeof datastore.error === 'string' && Boolean(datastore.error.trim())) ||
    ['unavailable', 'offline', 'error', 'failed'].includes(status ?? '')
  ) {
    return { state: 'unavailable' as const, percent: undefined };
  }
  // Empty status is supported by older payloads; an unrecognised explicit
  // status must not be interpreted as current healthy capacity.
  if (
    connection !== 'success' ||
    (status && !['available', 'online', 'healthy', 'ok'].includes(status)) ||
    !Number.isFinite(datastore.total) ||
    datastore.total <= 0 ||
    !Number.isFinite(datastore.used) ||
    datastore.used < 0
  ) {
    return { state: 'unknown' as const, percent: undefined };
  }
  // Fullness and its warning tone must describe the bytes shown beside them.
  // A defaulted or retained usagePercent must not make a full store look empty
  // (or an empty store look full). Both byte fields passed admission above.
  const percent = (datastore.used / datastore.total) * 100;
  if (!Number.isFinite(percent) || percent < 0) {
    return { state: 'unknown' as const, percent: undefined };
  }
  return { state: 'available' as const, percent };
}

// >=90% is the silent-backup-failure danger zone; >=75% is the early warning.
function usagePressure(pct?: number): 'danger' | 'warning' | undefined {
  if (pct !== undefined) return pct >= 90 ? 'danger' : pct >= 75 ? 'warning' : undefined;
}

const PRESSURE_TEXT = {
  normal: 'text-base-content',
  danger: 'text-red-600 dark:text-red-300',
  warning: 'text-amber-600 dark:text-amber-300',
};

function jobToneClass(state: DatastoreJobState): string {
  const tone = jobDetailTone(state);
  if (tone) return PRESSURE_TEXT[tone];
  if (state === 'ok') return 'text-base-content';
  return 'text-muted';
}

const jobDetailTone = (state: DatastoreJobState): 'danger' | 'warning' | undefined =>
  state === 'critical' ? 'danger' : state === 'warning' ? 'warning' : undefined;

const forecastDetailTone = (forecast: DatastoreFillForecast): 'danger' | 'warning' =>
  forecast.level === 'critical' ? 'danger' : 'warning';

const FORECAST_PILL_CLASS: Record<DatastoreFillForecast['level'], string> = {
  critical: 'bg-red-100 text-red-700 dark:bg-red-900/40 dark:text-red-300',
  warning: 'bg-amber-100 text-amber-800 dark:bg-amber-900/40 dark:text-amber-300',
};

const PBS_HISTORY_GAP = 'Backup history is incomplete for this PBS connection.';
const formatListedBackupEntries = (count: number): string =>
  `${formatPlatformTableIntegerValue(count)} listed`;

type BackupInventoryState = 'available' | 'loading' | 'unavailable';
const inventoryPendingLabel = (state: BackupInventoryState | undefined) =>
  state && state !== 'available' ? (state === 'loading' ? 'Loading' : 'Unavailable') : undefined;

// A server can have several identically prefixed datastore names. The compact
// scan keeps its single-line cells, but hover titles are not an inspection path
// on a phone. Keep the selected store's full identity and admitted readings in
// its existing disclosure, without changing the host inspector's identity.
function BackupServerDatastoreContext(props: {
  row: BackupServerRow;
  inventoryState?: BackupInventoryState;
  historyIncomplete: boolean;
  forecast?: DatastoreFillForecast | null;
  onClose?: () => void;
}) {
  const id = createUniqueId();
  const headingId = () => `backup-datastore-drawer-${id}`;
  const capacity = createMemo(() =>
    datastoreCapacity(props.row.datastore, props.row.connectionVariant),
  );
  const headingStatus = createMemo<{ variant: StatusIndicatorVariant; label: string }>(() => {
    const status = effectiveBackupServerStatus(
      props.row.connectionVariant,
      props.row.connectionLabel,
      [props.row.jobs],
    );
    const state = capacity().state;
    // A healthy PBS connection can still fail to read an individual datastore.
    // Keep the server's own connection indicator and any known job problem.
    const displayStatus =
      status.variant === 'success' && (state === 'unavailable' || state === 'unknown')
        ? { variant: 'warning' as const, label: `Datastore capacity ${state}` }
        : status;
    return withPBSPollFreshness(displayStatus, props.row.pollStale);
  });
  const capacityText = () => {
    const datastore = props.row.datastore;
    if (!datastore) return 'No datastore data';
    const reading = capacity();
    if (reading.state !== 'available') {
      return `${unreadCapacityLabel(reading.state)}. ${UNREAD_CAPACITY_NOTE}`;
    }
    // The available result already entails finite numeric used/total bytes.
    const free = formatPlatformTableBytesValue(
      Math.max(0, datastore.total - datastore.used),
      '0 B',
    );
    return `${formatPlatformTablePercentValue(reading.percent)} used · ${datastoreByteUsage(datastore)} · ${free} free`;
  };
  const inventoryText = () => {
    if (!props.row.datastore) return 'Not reported';
    const pending = inventoryPendingLabel(props.inventoryState);
    if (pending) return pending;
    const listed = formatListedBackupEntries(props.row.backupCount);
    return props.historyIncomplete ? `${listed} · ${PBS_HISTORY_GAP}` : listed;
  };
  const capacityTone = (): DetailValueTone | undefined => {
    const reading = capacity();
    if (reading.state === 'unavailable' || reading.state === 'unknown') return 'warning';
    return usagePressure(reading.percent);
  };

  return (
    <section class="mb-3 space-y-2" aria-label="Selected PBS datastore">
      {/* A datastore row's drawer is about the datastore; its server is one
          click away below. A server row with no datastore is about the server,
          whose own inspector header names it. */}
      <Show when={props.row.datastore}>
        {(datastore) => (
          <ObjectDrawerHeader
            collapseLabel={`Collapse ${datastore().name} details`}
            onCollapse={() => props.onClose?.()}
          >
            <DrawerSubjectHeading
              headingId={headingId()}
              title={datastore().name}
              statusVariant={headingStatus().variant}
              statusLabel={headingStatus().label}
              trailing={<span class="truncate text-xs text-muted">on {props.row.serverName}</span>}
            />
          </ObjectDrawerHeader>
        )}
      </Show>
      <DetailSectionTable
        wrapValues
        sections={[
          {
            label: 'Datastore',
            rows: [
              ...(props.row.pollStale
                ? [
                    {
                      label: 'PBS data',
                      value: 'Stale. Capacity, forecasts and jobs are last reported values.',
                      tone: 'warning' as const,
                    },
                  ]
                : []),
              { label: 'Capacity', value: capacityText(), tone: capacityTone() },
              ...(props.forecast
                ? [
                    {
                      label: 'Fill forecast',
                      value: props.forecast.detail,
                      tone: forecastDetailTone(props.forecast),
                    },
                  ]
                : []),
              {
                label: 'Jobs',
                value: describeDatastoreJobs(props.row.jobs),
                tone: jobDetailTone(props.row.jobs.state),
              },
              { label: 'Backup entries', value: inventoryText() },
            ],
            footerContent: (
              <p class="whitespace-normal text-[11px] leading-snug text-muted">
                Backup entries can include running and unverified backups. They do not prove a guest
                is protected.
              </p>
            ),
          },
        ]}
      />
    </section>
  );
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

const uniquelyCorrelatedAgent = (matches: readonly Resource[]): Resource | undefined => {
  const first = matches[0];
  if (matches.length < 2) return first;

  // A single agent can surface twice: folded into its PVE guest and as the
  // standalone host row. Those are one host, not an ambiguous pair. Collapse by
  // agent identity and prefer the guest, whose metrics target carries the
  // persisted history the Backups drawer renders.
  const key = correlatedAgentKey(first);
  if (key && matches.every((match) => correlatedAgentKey(match) === key)) {
    return preferredCorrelatedHost(matches);
  }
};

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
  // The PBS producer reports the configured connection name in both
  // PBSBackup.instance and Resource.name. PBS.instanceId is a service identity
  // ("pbs-" + that name), not another backup reader: using it as a fallback can
  // borrow a different connection's backups when this one has none.
  const countFor = (server: Resource, datastoreName: string): number =>
    counts.get(server.name)?.get(datastoreName) ?? 0;
  // `model().pbs` is scope-filtered (proxmox-pbs), which also catches PBS
  // *datastore* storage resources (type 'storage', sources ['pbs']). This table
  // is about the server, so keep only actual PBS server instances — otherwise a
  // datastore renders as a phantom offline "server" row.
  const pbsServers = servers.filter((resource) => resource.type === 'pbs');
  const sortedServers = pbsServers
    .map((server) => {
      const matches = matchingAgentHosts(server, servers);
      const agent = uniquelyCorrelatedAgent(matches);
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
        if (fresh && matches.length === 0) {
          return mergePBSAgentPresentation(server, retained.agent);
        }
        // An ambiguous match revokes the remembered choice too; a later
        // omission must not resurrect an identity we already declined.
        retention?.delete(server.id);
      }
      return server;
    })
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
    const host = {
      serverName: server.name,
      // Use the registry's PBS cadence, not aggregate status or lastSeen
      // (which can still advance from a linked host agent).
      pollStale: isPBSServerPollStale(server),
      ...serverConnection(server),
    };
    // Each keyed row has a reconciled Solid store. Give it its own JSON DTO
    // snapshot: otherwise joining an Agent mutates the source's service
    // metricsTarget (or another datastore row), defeating later revocation.
    // Unwrap reactive source DTOs before cloning; never clone their proxies.
    const resourceSnapshot = () => structuredClone(unwrap(server));
    if (datastores.length === 0) {
      rows.push({
        key: server.id,
        ...host,
        resource: resourceSnapshot(),
        jobs: summarizeDatastoreJobs(server.pbs?.jobHealthEvidence, undefined),
        backupCount: 0,
      });
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
        jobs: summarizeDatastoreJobs(server.pbs?.jobHealthEvidence, datastore.name),
        backupCount: countFor(server, datastore.name),
      });
    }
  }
  return rows;
}

// The server's header row: reachability, web link and how many datastores it
// reports, so one server with two datastores never reads as two servers.
function BackupServerGroupRow(props: { group: BackupServerGroup; colSpan: number }) {
  const count = () => props.group.datastoreCount;
  return (
    <tr class={getGroupedTableRowClass()} data-backup-server-group={props.group.serverName}>
      <td colspan={props.colSpan} class={getGroupedTableRowCellClass()}>
        <div class="flex min-w-0 items-center gap-2" title={props.group.statusLabel}>
          <StatusDot
            variant={props.group.statusVariant}
            title={props.group.statusLabel}
            ariaLabel={props.group.statusLabel}
            size="xs"
          />
          <ResourceNameWithWebInterfaceLink
            name={props.group.serverName}
            url={props.group.customUrl}
            class="min-w-0"
            nameClass="block min-w-0 truncate text-base-content"
          />
          {/* No datastore rows can mean the read failed, not that the server
              has none, so the header claims no count; its row says why. */}
          <Show when={count() > 0}>
            <span class={`shrink-0 ${GROUPED_TABLE_ROW_META_CLASS}`}>
              {count()} {count() === 1 ? 'datastore' : 'datastores'}
            </span>
          </Show>
          {/* Connection health is the server's, so only its header says it. */}
          <Show when={!props.group.online}>
            <span
              class={`shrink-0 text-[10px] font-medium ${connectionToneClass(props.group.connectionVariant)}`}
              data-backup-server-connection
            >
              {props.group.connectionLabel}
            </span>
          </Show>
        </div>
        <Show when={props.group.pollStale}>
          <span class={`pl-3.5 ${STALE_NOTE_CLASS}`}>{PBS_DATA_STALE}</span>
        </Show>
      </td>
    </tr>
  );
}

export function ProxmoxBackupServersTable(props: {
  servers: readonly Resource[];
  backups?: readonly PBSBackup[];
  backupInventoryState?: BackupInventoryState;
  incompleteHistoryReaders?: ReadonlySet<string>;
  emptyIcon?: JSX.Element;
  layoutWidth?: Accessor<number | null | undefined>;
  // Open alerts for a datastore's alert keys. The page owns the live alert
  // store; without it the table shows no fill forecast.
  datastoreAlerts?: (resourceIds: string[]) => Alert[];
}) {
  const retention = createPbsCorrelationRetention();
  const rows = createMemo(() =>
    buildBackupServerRows(props.servers, props.backups ?? [], retention),
  );
  const tableItems = createMemo(() => groupBackupServerRows(rows()));
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
  const serverCount = () => tableItems().filter((item) => item.kind === 'server').length;
  const pendingInventory = () => inventoryPendingLabel(props.backupInventoryState);
  // Only the phone layout has State, because it has no Jobs column and no
  // room beside the percentage. State answers for the datastore: a fill
  // forecast or a job problem, else its job result. Connection health is the
  // server's and lives on its header, except that a datastore on an
  // unreachable server cannot be read, so State says so.
  return (
    <Show when={rows().length > 0}>
      <div
        ref={observedWidth.setElement}
        data-proxmox-backups-table="servers"
        data-proxmox-backups-layout={layoutMode()}
      >
        <PlatformTableShell
          title={
            <span class="inline-flex items-center gap-1.5" data-backup-servers-title>
              Backup servers
              <span class="font-semibold tabular-nums text-base-content">
                {formatPlatformTableIntegerValue(serverCount())}
              </span>
            </span>
          }
          tableClass="min-w-0 table-fixed text-xs"
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
            <For each={visibleColumns()}>
              {(column) => {
                const backupsLabel = column.id === 'backups' ? column.label : undefined;
                return (
                  <TableHead
                    class={getPlatformTableHeadClassForKind(column.kind)}
                    aria-label={backupsLabel}
                    title={backupsLabel}
                  >
                    {backupsLabel ? (
                      <PlatformResponsiveTableLabel
                        compact="Bkps"
                        full={layoutMode() === 'full' ? backupsLabel : 'Count'}
                      />
                    ) : (
                      column.label
                    )}
                  </TableHead>
                );
              }}
            </For>
          }
          body={
            <>
              <PlatformWindowedRows
                items={tableItems}
                keyExtractor={(item) =>
                  item.kind === 'server' ? `server:${item.group.key}` : `datastore:${item.row.key}`
                }
                colSpan={visibleColumns().length}
                estimatedRowHeight={32}
              >
                {(item) => {
                  // The kind is part of the key, so an item never changes kind.
                  if (item.kind === 'server') {
                    return (
                      <BackupServerGroupRow group={item.group} colSpan={visibleColumns().length} />
                    );
                  }
                  const row = item.row;
                  // Read admission is not a generic storage-adapter capacity snapshot.
                  const capacityEvidence = () =>
                    datastoreCapacity(row.datastore, row.connectionVariant);
                  const pct = () => capacityEvidence().percent;
                  const pressure = () => usagePressure(pct());
                  const availableDatastore = () =>
                    capacityEvidence().state === 'available' ? row.datastore : undefined;
                  // Only a readable store can be forecast; Unavailable stays the answer.
                  const forecast = createMemo(() => {
                    const datastore = row.datastore;
                    const lookup = props.datastoreAlerts;
                    if (!datastore || !lookup || capacityEvidence().state !== 'available') {
                      return null;
                    }
                    return getDatastoreFillForecast(
                      lookup(backupServerDatastoreAlertIds(row.resource, datastore.name)).filter(
                        (alert) =>
                          isBackupServerDatastoreAlert(alert, row.resource, datastore.name),
                      ),
                    );
                  });
                  // Phones have no room beside the percentage, so the forecast takes
                  // State like a job problem does. A full store stops every future
                  // backup, so it outranks a job problem unless only the job is critical.
                  const compactForecast = () => {
                    const fill = forecast();
                    return layoutMode() === 'compact' &&
                      fill &&
                      (row.jobs.state !== 'critical' || fill.level === 'critical')
                      ? fill
                      : null;
                  };
                  const capacityLabel = () => unreadCapacityLabel(capacityEvidence().state);
                  const capacityNotice = () =>
                    `Datastore capacity is ${capacityLabel().toLowerCase()}. ${UNREAD_CAPACITY_NOTE}`;
                  const compactState = createMemo(() => {
                    const fill = compactForecast();
                    if (fill) {
                      return {
                        label: fill.shortLabel,
                        title: fill.detail,
                        tone: `font-medium ${PRESSURE_TEXT[forecastDetailTone(fill)]}`,
                        forecast: fill.level,
                      };
                    }
                    if (row.online) {
                      return {
                        label: row.jobs.label,
                        title: describeDatastoreJobs(row.jobs),
                        tone: jobToneClass(row.jobs.state),
                        jobs: row.jobs.state,
                      };
                    }
                    return {
                      label:
                        row.connectionVariant === 'danger'
                          ? 'Unreachable'
                          : row.connectionVariant === 'warning'
                            ? row.connectionLabel
                            : 'Unknown',
                      title: `${row.serverName}: ${row.connectionLabel}`,
                      tone: connectionToneClass(row.connectionVariant),
                    };
                  });
                  const rowIdentity = { id: row.key };
                  const historyIncomplete = () =>
                    props.incompleteHistoryReaders?.has(row.serverName) === true;
                  const deduplication = () => {
                    const factor = availableDatastore()?.deduplicationFactor ?? 0;
                    return Number.isFinite(factor) && factor > 0 ? `${factor.toFixed(1)}×` : '—';
                  };
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
                          class={`${getPlatformTableCellClassForKind('name')} truncate text-base-content`}
                          title={[row.serverName, row.datastore?.name].filter(Boolean).join(' · ')}
                        >
                          <div class="flex min-w-0 items-center gap-1">
                            <PlatformResourceDetailToggleButton
                              expanded={isExpanded()}
                              resourceLabel={
                                row.datastore
                                  ? `${row.serverName} · datastore ${row.datastore.name}`
                                  : row.serverName
                              }
                              controlsId={detailRowId()}
                              onToggle={() => detail.toggle(rowIdentity)}
                            />
                            <span
                              class={`block min-w-0 flex-1 truncate ${row.datastore ? 'font-medium' : 'text-muted'}`}
                              data-backup-server-datastore={row.datastore ? '' : undefined}
                            >
                              {row.datastore?.name ?? '—'}
                            </span>
                          </div>
                        </TableCell>
                        <Show when={columnVisible('status')}>
                          <TableCell class={getPlatformTableCellClassForKind('text')}>
                            <span
                              class={`block truncate text-[11px] ${compactState().tone}`}
                              title={compactState().title}
                              data-backup-server-jobs={compactState().jobs}
                              data-backup-server-forecast={compactState().forecast}
                            >
                              {compactState().label}
                              <Show when={compactState().forecast && row.pollStale}>
                                <span class={STALE_NOTE_CLASS}>Last forecast</span>
                              </Show>
                            </span>
                          </TableCell>
                        </Show>
                        <TableCell class={getPlatformTableCellClassForKind('numeric-value')}>
                          <Show
                            when={availableDatastore()}
                            fallback={
                              <span
                                class={
                                  row.datastore
                                    ? 'text-[11px] text-amber-600 dark:text-amber-300'
                                    : 'text-muted'
                                }
                                title={row.datastore ? capacityNotice() : undefined}
                              >
                                {row.datastore ? capacityLabel() : 'No datastore data'}
                              </span>
                            }
                          >
                            {(datastore) => (
                              <>
                                <div class="flex items-center justify-end gap-2">
                                  <Show
                                    when={layoutMode() !== 'compact' && layoutMode() !== 'basic'}
                                  >
                                    <StatusDot
                                      size="sm"
                                      variant={pressure() ?? (row.pollStale ? 'muted' : 'success')}
                                      title={`Datastore ${formatPlatformTablePercentValue(pct())} used`}
                                      ariaHidden
                                    />
                                  </Show>
                                  <span
                                    class={`tabular-nums font-medium ${PRESSURE_TEXT[pressure() ?? 'normal']}`}
                                  >
                                    <PlatformTablePercentValue value={pct()} />
                                  </span>
                                  {/* Keep the forecast in its existing position and
                                      qualify it along with any retained capacity. */}
                                  <Show
                                    when={layoutMode() !== 'compact' && forecast()}
                                    fallback={
                                      <Show when={layoutMode() !== 'compact'}>
                                        <span class="truncate text-[10px] text-muted tabular-nums">
                                          {datastoreByteUsage(datastore())}
                                        </span>
                                      </Show>
                                    }
                                  >
                                    {(fill) => (
                                      <span
                                        class={`inline-flex items-center whitespace-nowrap rounded-md px-1.5 py-0.5 text-[11px] font-semibold tabular-nums ${FORECAST_PILL_CLASS[fill().level]}`}
                                        title={`${fill().detail}. ${datastoreByteUsage(datastore())} used.`}
                                        data-backup-server-forecast={fill().level}
                                      >
                                        <span aria-hidden="true">{fill().shortLabel}</span>
                                        <span class="sr-only">{fill().label}</span>
                                      </span>
                                    )}
                                  </Show>
                                </div>
                                <Show when={row.pollStale}>
                                  <span class={STALE_NOTE_CLASS}>
                                    {layoutMode() !== 'compact' && forecast()
                                      ? 'Last reading & forecast'
                                      : 'Last reading'}
                                  </span>
                                </Show>
                              </>
                            )}
                          </Show>
                        </TableCell>
                        <Show when={columnVisible('jobs')}>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('text')} truncate text-[11px] ${jobToneClass(row.jobs.state)}`}
                            title={describeDatastoreJobs(row.jobs)}
                            data-backup-server-jobs={row.jobs.state}
                          >
                            {row.jobs.label}
                          </TableCell>
                        </Show>
                        <Show when={columnVisible('backups')}>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('numeric-value')} text-base-content`}
                          >
                            <div
                              class={`text-[11px] leading-3 whitespace-normal tabular-nums ${row.datastore && !pendingInventory() ? '' : 'text-muted'}`}
                              title={
                                row.datastore && pendingInventory()
                                  ? `PBS backup inventory is ${props.backupInventoryState}`
                                  : undefined
                              }
                            >
                              {row.datastore
                                ? (pendingInventory() ?? formatListedBackupEntries(row.backupCount))
                                : '—'}
                              <Show
                                when={row.datastore && !pendingInventory() && historyIncomplete()}
                              >
                                {' '}
                                <span
                                  class="block text-[10px] font-medium text-amber-700 dark:text-amber-300"
                                  title={PBS_HISTORY_GAP}
                                >
                                  History gap
                                </span>
                              </Show>
                            </div>
                          </TableCell>
                        </Show>
                        <Show when={columnVisible('dedup')}>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('numeric-value')} text-muted tabular-nums text-[11px]`}
                          >
                            {deduplication()}
                          </TableCell>
                        </Show>
                      </TableRow>
                      <PlatformResourceDetailTableRow
                        resource={row.resource}
                        statusIndicator={withPBSPollFreshness(
                          { variant: row.connectionVariant, label: row.connectionLabel },
                          row.pollStale,
                        )}
                        open={isExpanded()}
                        detailRowId={detailRowId()}
                        colSpan={visibleColumns().length}
                        context={
                          <BackupServerDatastoreContext
                            row={row}
                            inventoryState={props.backupInventoryState}
                            historyIncomplete={historyIncomplete()}
                            forecast={forecast()}
                            onClose={() => detail.close(rowIdentity)}
                          />
                        }
                        initialShowHostDetails
                        collapseResource={
                          row.datastore
                            ? {
                                title: 'Server',
                                summary: `${row.serverName} · CPU, memory, network, disk`,
                                showLabel: 'Show server',
                                hideLabel: 'Hide server',
                              }
                            : undefined
                        }
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
