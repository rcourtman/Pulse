import { getResourceStaleness } from '@/features/platformPage/resourceStaleness';
import type { Resource } from '@/types/resource';
import { getResourceSourceStatus } from '@/utils/resourceSourceHealth';
import { asTrimmedString } from '@/utils/stringUtils';
import type { RecoverableArtifact, WorkloadCoverageRow } from './proxmoxBackupRecoveryModel';
import { resolveProxmoxPlatformScope } from './proxmoxPageModel';

export interface StaleBackupSource {
  provider: 'proxmox-pve' | 'proxmox-pbs';
  name: string;
  instance?: string;
  node?: string;
  age: string | null;
  error?: string;
  detail: string;
}

export interface BackupStaleNotice {
  badge: string;
  compactBadge: string;
  detail: string;
  error?: string;
  sources: string[];
}

const CONNECTED = new Set(['online', 'running', 'healthy', 'connected', 'ok']);
const IMPAIRED_HEALTH = new Set([
  'stale',
  'error',
  'degraded',
  'offline',
  'unreachable',
  'unhealthy',
]);

const norm = (value: unknown): string =>
  typeof value === 'string' ? value.trim().toLowerCase() : '';

export const resolveProxmoxSourceStaleness = (
  resource: Resource,
  kind: 'proxmox' | 'pbs',
  subjectOverride?: string,
): { name: string; age: string | null; error?: string; reason: string } | null => {
  const source = getResourceSourceStatus(resource, kind);
  const srcStatus = norm(source?.status);
  const connHealth = norm(
    kind === 'pbs' ? resource.pbs?.connectionHealth : resource.proxmox?.connectionHealth,
  );
  if (CONNECTED.has(srcStatus) && !IMPAIRED_HEALTH.has(connHealth)) return null;

  const staleness = getResourceStaleness(resource);
  const error = asTrimmedString(source?.error);
  const resStatus = norm(resource.status);
  const impaired =
    Boolean(staleness) ||
    (srcStatus !== '' && srcStatus !== 'pending' && !CONNECTED.has(srcStatus)) ||
    IMPAIRED_HEALTH.has(connHealth) ||
    ((resStatus === 'offline' || resStatus === 'unknown' || resStatus === 'unreachable') &&
      !CONNECTED.has(srcStatus));
  if (!impaired) return null;

  const label = kind === 'pbs' ? 'Proxmox Backup Server' : 'Proxmox';
  const name =
    asTrimmedString(subjectOverride) ||
    (kind === 'pbs'
      ? asTrimmedString(resource.name) || asTrimmedString(resource.displayName) || resource.id
      : asTrimmedString(resource.proxmox?.nodeName) ||
        (resource.type === 'agent'
          ? asTrimmedString(resource.name)
          : asTrimmedString(resource.parentName)) ||
        asTrimmedString(resource.proxmox?.instance) ||
        asTrimmedString(resource.platformId) ||
        asTrimmedString(resource.name) ||
        'Proxmox');
  const age = staleness?.age ?? null;
  const ageSuffix = age ? ` (last report ${age} ago)` : '';
  const reason =
    error && age
      ? `No ${label} report from ${name} for ${age} (${error})`
      : error
        ? `${label} poll failed on ${name} (${error})`
        : connHealth === 'error' || srcStatus === 'error' || srcStatus === 'failed'
          ? `${label} API poll failed on ${name}${ageSuffix}`
          : connHealth === 'offline' ||
              connHealth === 'unreachable' ||
              srcStatus === 'offline' ||
              resStatus === 'offline'
            ? `${label} ${kind === 'pbs' ? '' : 'node '}${name} is ${kind === 'pbs' ? 'unreachable' : 'offline'} in Pulse${ageSuffix}`
            : age
              ? `No ${label} report from ${name} for ${age}`
              : connHealth === 'degraded'
                ? `${label} connection for ${name} is degraded`
                : `No recent ${label} report from ${name}`;

  return { name, age, ...(error ? { error } : {}), reason };
};

export const isPBSServerPollStale = (server: Resource): boolean => {
  const srcStatus = norm(getResourceSourceStatus(server, 'pbs')?.status);
  const connHealth = norm(server.pbs?.connectionHealth);
  if (CONNECTED.has(srcStatus) && !IMPAIRED_HEALTH.has(connHealth)) return false;
  return srcStatus === 'stale' || connHealth === 'stale' || Boolean(getResourceStaleness(server));
};

const toStaleSource = (
  resource: Resource,
  provider: 'proxmox-pve' | 'proxmox-pbs',
): StaleBackupSource | null => {
  const hit = resolveProxmoxSourceStaleness(
    resource,
    provider === 'proxmox-pbs' ? 'pbs' : 'proxmox',
  );
  if (!hit) return null;
  const isPBS = provider === 'proxmox-pbs';
  return {
    provider,
    name: hit.name,
    instance: isPBS
      ? hit.name
      : asTrimmedString(resource.proxmox?.instance) ||
        asTrimmedString(resource.platformId) ||
        undefined,
    node: isPBS
      ? undefined
      : asTrimmedString(resource.proxmox?.nodeName) ||
        (resource.type === 'agent'
          ? asTrimmedString(resource.name)
          : asTrimmedString(resource.parentName)) ||
        undefined,
    age: hit.age,
    ...(hit.error ? { error: hit.error } : {}),
    detail: hit.reason,
  };
};

export const collectStaleBackupSources = (
  servers: readonly Resource[] | undefined,
  workloads?: readonly Resource[] | undefined,
): StaleBackupSource[] => {
  const byKey = new Map<string, StaleBackupSource>();
  const add = (entry: StaleBackupSource | null) => {
    if (!entry) return;
    const key = `${entry.provider}\u0000${norm(entry.instance)}\u0000${norm(entry.node || entry.name)}`;
    if (!byKey.has(key)) byKey.set(key, entry);
  };
  for (const r of servers ?? []) {
    if (r.type === 'pbs') add(toStaleSource(r, 'proxmox-pbs'));
    else if (r.type === 'agent' && resolveProxmoxPlatformScope(r) === 'proxmox-pve') {
      add(toStaleSource(r, 'proxmox-pve'));
    }
  }
  for (const w of workloads ?? []) add(toStaleSource(w, 'proxmox-pve'));
  return [...byKey.values()].sort((a, b) => a.name.localeCompare(b.name));
};

export const collectStaleWorkloadNotices = (
  workloads: readonly Resource[] | undefined,
): ReadonlyMap<string, StaleBackupSource> => {
  const map = new Map<string, StaleBackupSource>();
  for (const w of workloads ?? []) {
    const stale = toStaleSource(w, 'proxmox-pve');
    if (stale) map.set(w.id, stale);
  }
  return map;
};

export const describeStaleBackupSources = (
  sources: readonly StaleBackupSource[],
): string | null => {
  if (sources.length === 0) return null;
  const labels = sources.slice(0, 3).map((s) => (s.age ? `${s.name} (${s.age})` : s.name));
  if (sources.length > 3) labels.push(`${sources.length - 3} more`);
  const list =
    labels.length === 1 ? labels[0] : `${labels.slice(0, -1).join(', ')} and ${labels.at(-1)}`;
  return `Backup telemetry is stale for ${list}. Protection status, job counts, and restore points show the last values received before polling stopped.`;
};

const matchesPVE = (
  source: StaleBackupSource,
  instance: string | undefined,
  nodes: readonly (string | undefined)[],
): boolean => {
  if (source.provider !== 'proxmox-pve') return false;
  const si = norm(source.instance);
  const ti = norm(instance);
  if (si && ti && si !== ti) return false;
  const sn = norm(source.node || source.name);
  const tn = nodes.map(norm).filter(Boolean);
  return sn && tn.length > 0 ? tn.includes(sn) : Boolean(si && ti && si === ti);
};

const matchesPBS = (source: StaleBackupSource, scope: string | undefined): boolean => {
  if (source.provider !== 'proxmox-pbs') return false;
  const t = norm(scope);
  return Boolean(t && (t === norm(source.name) || t === norm(source.instance)));
};

const toNotice = (
  matches: readonly StaleBackupSource[],
  suffix: string,
): BackupStaleNotice | null => {
  if (matches.length === 0) return null;
  const age = matches.find((m) => m.age)?.age ?? null;
  const error = matches.find((m) => m.error)?.error;
  return {
    badge: age ? `Stale · ${age}` : 'Stale data',
    compactBadge: 'Stale',
    detail: `${matches.map((m) => m.detail).join('. ')} — ${suffix}`,
    ...(error ? { error } : {}),
    sources: matches.map((m) => m.name),
  };
};

const workloadNodes = (w: { nativeNode?: string; node?: string; nativeNodeAliases?: string[] }) => [
  w.nativeNode,
  w.node,
  ...(w.nativeNodeAliases ?? []),
];

export const getCoverageRowStaleNotice = (
  row: WorkloadCoverageRow,
  staleSources: readonly StaleBackupSource[],
  staleWorkloads?: ReadonlyMap<string, StaleBackupSource>,
): BackupStaleNotice | null => {
  const seen = new Map<string, StaleBackupSource>();
  const push = (s: StaleBackupSource | undefined) => {
    if (s) seen.set(`${s.provider}\0${norm(s.name)}`, s);
  };
  if (row.workload.resourceId) push(staleWorkloads?.get(row.workload.resourceId));
  const nodes = workloadNodes(row.workload);
  const pbsScopes = [
    ...(row.protectionPosture?.providerStates ?? [])
      .filter((s) => s.provider === 'proxmox-pbs')
      .map((s) => s.scope),
    ...row.artifacts.filter((a) => a.sourceKind === 'pbs').map((a) => a.locationConnection),
  ];
  for (const s of staleSources) {
    if (matchesPVE(s, row.workload.instance, nodes) || pbsScopes.some((sc) => matchesPBS(s, sc))) {
      push(s);
    }
  }
  return toNotice(
    [...seen.values()],
    'protection status and restore points show the last values received.',
  );
};

export const hasStaleTextMatch = (notice: BackupStaleNotice | null, term: string): boolean =>
  Boolean(
    term &&
    notice &&
    `${notice.badge} ${notice.detail} ${notice.error ?? ''}`.toLowerCase().includes(term),
  );

export const getArtifactStaleNotice = (
  artifact: RecoverableArtifact,
  staleSources: readonly StaleBackupSource[],
): BackupStaleNotice | null => {
  if (!staleSources.length) return null;
  const isPBS = artifact.sourceKind === 'pbs';
  const scope =
    artifact.locationConnection || (isPBS ? artifact.location.split('/')[0] : undefined);
  const nodes = workloadNodes(artifact.workload);
  return toNotice(
    staleSources.filter((s) =>
      isPBS ? matchesPBS(s, scope) : matchesPVE(s, artifact.workload.instance || scope, nodes),
    ),
    'newer backups or prunes may not appear yet.',
  );
};
