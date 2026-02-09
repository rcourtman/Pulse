import type { PBSBackup, PMGBackup, StorageBackup } from '@/types/api';
import type { Resource, ResourceType } from '@/types/resource';
import type {
  BackupCapability,
  BackupCategory,
  BackupMode,
  BackupOutcome,
  BackupRecord,
  BackupScope,
  BackupAdapter,
  BackupAdapterContext,
  SourceDescriptor,
  StorageBackupPlatform,
} from './models';

const dedupe = <T>(values: T[]): T[] => Array.from(new Set(values));

const resolveFamily = (platform: StorageBackupPlatform): SourceDescriptor['family'] => {
  const value = String(platform).toLowerCase();
  if (value.includes('kubernetes') || value.includes('docker')) return 'container';
  if (value === 'aws' || value === 'azure' || value === 'gcp') return 'cloud';
  if (value.includes('proxmox') || value.includes('vmware') || value.includes('hyperv')) {
    return 'virtualization';
  }
  if (value === 'generic') return 'generic';
  return 'onprem';
};

const source = (
  platform: StorageBackupPlatform,
  adapterId: string,
  origin: SourceDescriptor['origin'],
): SourceDescriptor => ({
  platform,
  family: resolveFamily(platform),
  adapterId,
  origin,
});

const parseDateMillis = (value: string | number | null | undefined): number | null => {
  if (typeof value === 'number' && Number.isFinite(value)) {
    if (value > 1_000_000_000_000) return value;
    if (value > 1_000_000_000) return Math.floor(value * 1000);
  }
  if (typeof value === 'string' && value.trim()) {
    const numeric = Number(value.trim());
    if (Number.isFinite(numeric)) {
      if (numeric > 1_000_000_000_000) return numeric;
      if (numeric > 1_000_000_000) return Math.floor(numeric * 1000);
    }
    const parsed = Date.parse(value);
    if (Number.isFinite(parsed)) return parsed;
  }
  return null;
};

const asRecord = (value: unknown): Record<string, unknown> =>
  value && typeof value === 'object' ? (value as Record<string, unknown>) : {};

const readString = (record: Record<string, unknown>, key: string): string => {
  const value = record[key];
  if (typeof value === 'string') return value;
  if (typeof value === 'number' && Number.isFinite(value)) return String(value);
  return '';
};

const readBoolean = (record: Record<string, unknown>, key: string): boolean | null => {
  const value = record[key];
  if (typeof value === 'boolean') return value;
  return null;
};

const readNumber = (record: Record<string, unknown>, key: string): number | null => {
  const value = record[key];
  if (typeof value === 'number' && Number.isFinite(value)) return value;
  if (typeof value === 'string' && value.trim()) {
    const parsed = Number(value);
    if (Number.isFinite(parsed)) return parsed;
  }
  return null;
};

const firstNonEmpty = (...values: Array<string | null | undefined>): string => {
  for (const value of values) {
    const normalized = (value || '').trim();
    if (normalized) return normalized;
  }
  return '';
};

const readStringAny = (record: Record<string, unknown>, keys: string[]): string => {
  for (const key of keys) {
    const value = readString(record, key).trim();
    if (value) return value;
  }
  return '';
};

const readBooleanAny = (record: Record<string, unknown>, keys: string[]): boolean | null => {
  for (const key of keys) {
    const value = readBoolean(record, key);
    if (value !== null) return value;
  }
  return null;
};

const readNumberAny = (record: Record<string, unknown>, keys: string[]): number | null => {
  for (const key of keys) {
    const value = readNumber(record, key);
    if (value !== null) return value;
  }
  return null;
};

const readDateAny = (record: Record<string, unknown>, keys: string[]): number | null => {
  for (const key of keys) {
    const parsed = parseDateMillis(record[key] as string | number | null | undefined);
    if (parsed) return parsed;
  }
  return null;
};

const isProxmoxPlatform = (platform: string): boolean =>
  platform === 'proxmox-pve' || platform === 'proxmox-pbs' || platform === 'proxmox-pmg';

const hasLegacyBackupData = (ctx: BackupAdapterContext): boolean =>
  (ctx.state.backups?.pve?.guestSnapshots || ctx.state.pveBackups?.guestSnapshots || []).length > 0 ||
  (ctx.state.backups?.pve?.storageBackups || ctx.state.pveBackups?.storageBackups || []).length > 0 ||
  (ctx.state.backups?.pbs || ctx.state.pbsBackups || []).length > 0 ||
  (ctx.state.backups?.pmg || ctx.state.pmgBackups || []).length > 0;

const workloadTypeFromResourceType = (resourceType: ResourceType): BackupScope['workloadType'] | undefined => {
  if (resourceType === 'vm') return 'vm';
  if (resourceType === 'container' || resourceType === 'oci-container' || resourceType === 'jail') return 'container';
  if (resourceType === 'pod' || resourceType === 'docker-container') return 'pod';
  if (resourceType === 'host' || resourceType === 'node' || resourceType === 'docker-host' || resourceType === 'k8s-node') {
    return 'host';
  }
  return undefined;
};

const scopeFromResourceType = (resourceType: ResourceType): BackupScope['scope'] => {
  if (
    resourceType === 'vm' ||
    resourceType === 'container' ||
    resourceType === 'oci-container' ||
    resourceType === 'docker-container' ||
    resourceType === 'pod' ||
    resourceType === 'jail'
  ) {
    return 'workload';
  }
  if (resourceType === 'host' || resourceType === 'node' || resourceType === 'docker-host' || resourceType === 'k8s-node') {
    return 'host';
  }
  if (
    resourceType === 'k8s-cluster' ||
    resourceType === 'k8s-deployment' ||
    resourceType === 'k8s-service' ||
    resourceType === 'pbs' ||
    resourceType === 'pmg'
  ) {
    return 'cluster';
  }
  return 'unknown';
};

const categoryFromResourceType = (resourceType: ResourceType): BackupCategory => {
  if (resourceType === 'vm') return 'vm-backup';
  if (
    resourceType === 'container' ||
    resourceType === 'oci-container' ||
    resourceType === 'docker-container' ||
    resourceType === 'pod' ||
    resourceType === 'jail'
  ) {
    return 'container-backup';
  }
  if (resourceType === 'host' || resourceType === 'node' || resourceType === 'docker-host' || resourceType === 'k8s-node') {
    return 'host-backup';
  }
  return 'other';
};

const parseResourceLastBackup = (resource: Resource): number | null => {
  const platformData = asRecord(resource.platformData);
  const proxmoxData = asRecord(platformData.proxmox);
  const backupData = asRecord(platformData.backup);
  const candidates: Array<string | number | null | undefined> = [
    platformData.lastBackup as string | number | null | undefined,
    proxmoxData.lastBackup as string | number | null | undefined,
    backupData.lastBackup as string | number | null | undefined,
    (resource as unknown as Record<string, unknown>).lastBackup as string | number | null | undefined,
  ];

  for (const candidate of candidates) {
    const parsed = parseDateMillis(candidate);
    if (parsed) return parsed;
  }
  return null;
};

const inferModeFromRecord = (
  payload: Record<string, unknown>,
  family: SourceDescriptor['family'],
  fallback: BackupMode = 'local',
): BackupMode => {
  const explicit =
    readStringAny(payload, ['mode', 'backupMode', 'transport', 'strategy', 'type']).toLowerCase();
  if (explicit === 'snapshot' || explicit === 'local' || explicit === 'remote') return explicit;

  if (readBooleanAny(payload, ['snapshot', 'isSnapshot']) === true) return 'snapshot';
  if (readBooleanAny(payload, ['remote', 'offsite', 'crossSite']) === true) return 'remote';
  if (family === 'cloud') return 'remote';
  return fallback;
};

const backupCapabilitiesForResource = (
  family: SourceDescriptor['family'],
  mode: BackupMode,
  encrypted: boolean | null,
  protectedFlag: boolean | null,
): BackupCapability[] => {
  const caps: BackupCapability[] = ['retention'];
  if (mode === 'snapshot') caps.push('policy-driven');
  if (mode === 'local' || mode === 'remote') caps.push('incremental');
  if (family === 'container') caps.push('policy-driven');
  if (family === 'cloud' || mode === 'remote') caps.push('cross-site');
  if (encrypted) caps.push('encryption');
  if (protectedFlag) caps.push('immutability');
  return dedupe(caps);
};

const backupCapabilitiesForArtifact = (
  family: SourceDescriptor['family'],
  mode: BackupMode,
  encrypted: boolean | null,
  protectedFlag: boolean | null,
  verified: boolean | null,
  appAware: boolean,
): BackupCapability[] => {
  const caps = backupCapabilitiesForResource(family, mode, encrypted, protectedFlag);
  if (verified !== null) caps.push('verification');
  if (appAware) caps.push('application-aware');
  return dedupe(caps);
};

const locationFromResource = (resource: Resource): BackupRecord['location'] => {
  const platformData = asRecord(resource.platformData);
  const proxmoxData = asRecord(platformData.proxmox);
  const kubernetesData = asRecord(platformData.kubernetes);
  const dockerData = asRecord(platformData.docker);
  const node =
    readString(platformData, 'node') ||
    readString(platformData, 'nodeName') ||
    readString(platformData, 'hostName') ||
    readString(proxmoxData, 'nodeName') ||
    readString(kubernetesData, 'nodeName') ||
    readString(dockerData, 'hostname');
  const cluster =
    readString(platformData, 'clusterId') ||
    readString(kubernetesData, 'clusterId') ||
    resource.clusterId ||
    resource.platformId;
  const namespace = readString(platformData, 'namespace') || readString(kubernetesData, 'namespace');

  if (node) return { label: node, scope: 'node' };
  if (namespace) return { label: namespace, scope: 'namespace' };
  if (cluster) return { label: cluster, scope: 'cluster' };
  if (resource.parentId) return { label: resource.parentId, scope: 'cluster' };
  return { label: 'Unknown', scope: 'unknown' };
};

const scope = (
  label: string,
  scopeValue: BackupScope['scope'],
  workloadType?: BackupScope['workloadType'],
): BackupScope => ({
  label,
  scope: scopeValue,
  workloadType,
});

const inferCategory = (type: string | undefined, vmid: number | string | null | undefined): BackupCategory => {
  const normalized = (type || '').toLowerCase();
  const vmidNum = typeof vmid === 'string' ? Number.parseInt(vmid, 10) : Number(vmid);
  if (vmidNum === 0 || normalized === 'host') return 'host-backup';
  if (normalized === 'qemu' || normalized === 'vm') return 'vm-backup';
  if (normalized === 'lxc' || normalized === 'ct' || normalized === 'container') return 'container-backup';
  if (normalized === 'snapshot') return 'snapshot';
  return 'other';
};

const workloadTypeFromCategory = (category: BackupCategory): BackupScope['workloadType'] => {
  if (category === 'vm-backup') return 'vm';
  if (category === 'container-backup') return 'container';
  if (category === 'host-backup') return 'host';
  return 'other';
};

const canonicalBackupIdentity = (
  type: string | undefined,
  vmid: number | string | null | undefined,
  when: string | number | null | undefined,
): string | null => {
  const category = inferCategory(type, vmid);
  if (category === 'snapshot' || category === 'other') return null;
  const vmidLabel = vmid == null ? '' : String(vmid).trim();
  if (!vmidLabel) return null;
  const millis = parseDateMillis(when);
  if (!millis) return null;
  const seconds = Math.floor(millis / 1000);
  return `${category}:${vmidLabel}:${seconds}`;
};

const buildPbsIdentitySet = (ctx: BackupAdapterContext): Set<string> => {
  const backups = ctx.state.backups?.pbs ?? ctx.state.pbsBackups;
  const identities = new Set<string>();
  for (const backup of backups || []) {
    const identity = canonicalBackupIdentity(backup.backupType, backup.vmid, backup.backupTime);
    if (identity) identities.add(identity);
  }
  return identities;
};

const inferOutcome = (verified: boolean | null | undefined, status?: string): BackupOutcome => {
  const normalized = (status || '').toLowerCase();
  if (normalized.includes('running')) return 'running';
  if (normalized.includes('fail') || normalized.includes('error')) return 'failed';
  if (normalized.includes('warn') || normalized.includes('degraded')) return 'warning';
  if (verified === true) return 'success';
  if (verified === false) return 'warning';
  return 'unknown';
};

const pveBackupCapabilities = (category: BackupCategory, encrypted: boolean | null): BackupCapability[] => {
  const caps: BackupCapability[] = ['retention', 'incremental'];
  if (category === 'snapshot') caps.push('policy-driven');
  caps.push('verification');
  if (encrypted) caps.push('encryption');
  return dedupe(caps);
};

const pbsCapabilities = (encrypted: boolean | null, isProtected: boolean): BackupCapability[] => {
  const caps: BackupCapability[] = ['retention', 'incremental', 'verification'];
  if (encrypted) caps.push('encryption');
  if (isProtected) caps.push('immutability');
  return dedupe(caps);
};

const buildSnapshotRecords = (ctx: BackupAdapterContext): BackupRecord[] => {
  const pve = ctx.state.backups?.pve ?? ctx.state.pveBackups;
  const snapshots = pve?.guestSnapshots || [];
  return snapshots.map((snapshot) => {
    const completedAt = parseDateMillis(snapshot.time);
    const category = inferCategory('snapshot', snapshot.vmid);
    const sizeBytes =
      typeof snapshot.sizeBytes === 'number' && Number.isFinite(snapshot.sizeBytes)
        ? snapshot.sizeBytes
        : null;
    return {
      id: `snapshot:${snapshot.instance}:${snapshot.node}:${snapshot.vmid}:${snapshot.name}:${snapshot.time}`,
      name: snapshot.name || `Snapshot ${snapshot.vmid}`,
      category,
      outcome: 'success',
      mode: 'snapshot',
      scope: scope(`VMID ${snapshot.vmid}`, 'workload', snapshot.type === 'qemu' ? 'vm' : 'container'),
      location: { label: snapshot.node || snapshot.instance || 'Unknown', scope: 'node' },
      source: source('proxmox-pve', 'legacy-pve-snapshots', 'legacy'),
      completedAt,
      sizeBytes,
      verified: null,
      protected: null,
      encrypted: null,
      capabilities: pveBackupCapabilities(category, null),
      refs: {
        platformEntityId: snapshot.instance,
      },
      proxmox: {
        vmid: String(snapshot.vmid),
        instance: snapshot.instance,
        node: snapshot.node,
        backupType: snapshot.type,
      },
    };
  });
};

const buildPveStorageBackupRecords = (ctx: BackupAdapterContext): BackupRecord[] => {
  const pve = ctx.state.backups?.pve ?? ctx.state.pveBackups;
  const pbsIdentitySet = buildPbsIdentitySet(ctx);
  const storageBackups = pve?.storageBackups || [];
  return storageBackups
    .filter((backup) => {
      if (backup.type === 'vztmpl' || backup.type === 'iso') return false;
      if (!backup.isPBS) return true;
      const identity = canonicalBackupIdentity(backup.type, backup.vmid, backup.ctime || backup.time);
      if (!identity) return true;
      // Prefer canonical PBS records for overlapping remote backups; keep PVE-only entries as fallback.
      return !pbsIdentitySet.has(identity);
    })
    .map((backup: StorageBackup) => {
      const completedAt = parseDateMillis(backup.ctime);
      const category = inferCategory(backup.type, backup.vmid);
      const verified = typeof backup.verified === 'boolean' ? backup.verified : null;
      const encrypted = backup.encryption ? true : null;
      const backupName = backup.volid?.split('/').pop() || backup.notes || `Backup ${backup.vmid}`;
      const backupSource = backup.isPBS ? 'proxmox-pbs' : 'proxmox-pve';
      const mode: BackupMode = backup.isPBS ? 'remote' : 'local';
      return {
        id: `pve:${backup.instance}:${backup.node}:${backup.vmid}:${backup.ctime}:${backup.volid || backupName}`,
        name: backupName,
        category,
        outcome: inferOutcome(verified, backup.verification),
        mode,
        scope: scope(
          category === 'host-backup' ? (backup.node || 'Host') : `VMID ${backup.vmid}`,
          category === 'host-backup' ? 'host' : 'workload',
          workloadTypeFromCategory(category),
        ),
        location: { label: backup.storage || backup.node || 'Unknown', scope: 'node' },
        source: source(backupSource, 'legacy-pve-storage-backups', 'legacy'),
        completedAt,
        sizeBytes: Number.isFinite(backup.size) ? backup.size : null,
        verified,
        protected: typeof backup.protected === 'boolean' ? backup.protected : null,
        encrypted,
        capabilities: pveBackupCapabilities(category, encrypted),
        refs: {
          platformEntityId: backup.instance,
        },
        proxmox: {
          vmid: String(backup.vmid),
          node: backup.node,
          instance: backup.instance,
          storage: backup.storage,
          backupType: backup.type,
          notes: backup.notes,
        },
      };
    });
};

const buildPbsRecords = (ctx: BackupAdapterContext): BackupRecord[] => {
  const backups = ctx.state.backups?.pbs ?? ctx.state.pbsBackups;
  return (backups || []).map((backup: PBSBackup) => {
    const category = inferCategory(backup.backupType, backup.vmid);
    const completedAt = parseDateMillis(backup.backupTime);
    const normalizedNamespace = (backup.namespace || '').trim() || 'root';
    const encrypted =
      Array.isArray(backup.files) &&
      backup.files.some((entry) =>
        typeof entry === 'string' ? entry.includes('.enc') : String(entry ?? '').includes('.enc'),
      );
    return {
      id:
        backup.id ||
        `pbs:${backup.instance}:${backup.datastore}:${normalizedNamespace}:${backup.backupType}:${backup.vmid}:${backup.backupTime}`,
      name: backup.comment || `${backup.backupType}/${backup.vmid}`,
      category,
      outcome: inferOutcome(backup.verified, undefined),
      mode: 'remote',
      scope: scope(
        category === 'host-backup' ? (backup.instance || 'Host') : `VMID ${backup.vmid}`,
        category === 'host-backup' ? 'host' : 'workload',
        workloadTypeFromCategory(category),
      ),
      location: {
        label: `${backup.instance || 'PBS'} / ${backup.datastore || 'datastore'}`,
        scope: 'cluster',
      },
      source: source('proxmox-pbs', 'legacy-pbs-backups', 'legacy'),
      completedAt,
      sizeBytes: Number.isFinite(backup.size) ? backup.size : null,
      verified: backup.verified,
      protected: backup.protected,
      encrypted,
      capabilities: pbsCapabilities(encrypted, backup.protected),
      refs: {
        legacyBackupId: backup.id,
        platformEntityId: backup.instance,
      },
      proxmox: {
        vmid: backup.vmid,
        instance: backup.instance,
        datastore: backup.datastore,
        namespace: normalizedNamespace,
        owner: backup.owner,
        comment: backup.comment,
        backupType: backup.backupType,
      },
    };
  });
};

const buildPmgRecords = (ctx: BackupAdapterContext): BackupRecord[] => {
  const backups = ctx.state.backups?.pmg ?? ctx.state.pmgBackups;
  return (backups || []).map((backup: PMGBackup) => ({
    id: backup.id || `pmg:${backup.instance}:${backup.node}:${backup.filename}:${backup.backupTime}`,
    name: backup.filename || `PMG backup ${backup.node}`,
    category: 'config-backup',
    outcome: 'success',
    mode: 'local',
    scope: scope(backup.node || backup.instance || 'PMG host', 'host', 'host'),
    location: { label: backup.instance || 'PMG', scope: 'cluster' },
    source: source('proxmox-pmg', 'legacy-pmg-backups', 'legacy'),
    completedAt: parseDateMillis(backup.backupTime),
    sizeBytes: Number.isFinite(backup.size) ? backup.size : null,
    verified: null,
    protected: null,
    encrypted: null,
    capabilities: ['retention'],
    refs: {
      legacyBackupId: backup.id,
      platformEntityId: backup.instance,
    },
    proxmox: {
      instance: backup.instance,
      node: backup.node,
      filename: backup.filename,
    },
  }));
};

const ARTIFACT_COLLECTION_KEYS = [
  'artifacts',
  'backups',
  'backupArtifacts',
  'backupEntries',
  'backupRecords',
  'entries',
  'records',
  'snapshots',
  'items',
];

const looksLikeBackupArtifact = (record: Record<string, unknown>): boolean => {
  const artifactTimestamp = readDateAny(record, [
    'backupTime',
    'completedAt',
    'finishedAt',
    'timestamp',
    'time',
    'createdAt',
  ]);

  const hasBackupHints =
    readStringAny(record, [
      'backupId',
      'backupUid',
      'backupName',
      'runId',
      'snapshotClass',
      'repository',
      'datastore',
      'backupType',
      'vmid',
      'containerId',
      'volume',
      'workloadName',
      'policy',
    ]) !== '' ||
    readNumberAny(record, ['sizeBytes', 'size', 'bytes']) !== null;

  const hasOperationalHints =
    readStringAny(record, ['name', 'displayName', 'title', 'status', 'phase', 'result']) !== '';

  return hasBackupHints || (artifactTimestamp !== null && hasOperationalHints);
};

const artifactSignature = (record: Record<string, unknown>): string =>
  [
    readStringAny(record, ['id', 'uid', 'backupId', 'backupUid', 'runId']),
    readStringAny(record, ['backupTime', 'completedAt', 'finishedAt', 'timestamp', 'time', 'createdAt']),
    readStringAny(record, ['name', 'backupName', 'workloadName', 'containerName', 'volume']),
  ].join('|');

const dedupeArtifactCandidates = (records: Record<string, unknown>[]): Record<string, unknown>[] => {
  const seenSignatures = new Set<string>();
  const deduped: Record<string, unknown>[] = [];
  for (const record of records) {
    const signature = artifactSignature(record);
    if (seenSignatures.has(signature)) continue;
    seenSignatures.add(signature);
    deduped.push(record);
  }
  return deduped;
};

const collectArtifactCandidates = (seed: unknown, depth = 0): Record<string, unknown>[] => {
  if (depth > 2 || seed == null) return [];

  if (Array.isArray(seed)) {
    return seed.flatMap((value) => collectArtifactCandidates(value, depth + 1));
  }

  if (typeof seed !== 'object') return [];
  const record = asRecord(seed);
  const results: Record<string, unknown>[] = [];

  if (looksLikeBackupArtifact(record)) {
    results.push(record);
  }

  for (const key of ARTIFACT_COLLECTION_KEYS) {
    if (!(key in record)) continue;
    results.push(...collectArtifactCandidates(record[key], depth + 1));
  }

  return results;
};

const extractKubernetesArtifactPayloads = (resource: Resource): Record<string, unknown>[] => {
  const platformData = asRecord(resource.platformData);
  const backupData = asRecord(platformData.backup);
  const kubernetesData = asRecord(platformData.kubernetes);

  return dedupeArtifactCandidates([
    ...collectArtifactCandidates(platformData.backupArtifacts),
    ...collectArtifactCandidates(platformData.backups),
    ...collectArtifactCandidates(platformData.backup),
    ...collectArtifactCandidates(backupData.artifacts),
    ...collectArtifactCandidates(backupData.backups),
    ...collectArtifactCandidates(backupData.entries),
    ...collectArtifactCandidates(backupData.records),
    ...collectArtifactCandidates(backupData.kubernetes),
    ...collectArtifactCandidates(kubernetesData.backup),
    ...collectArtifactCandidates(kubernetesData.backups),
    ...collectArtifactCandidates(kubernetesData.backupArtifacts),
  ]);
};

const extractDockerArtifactPayloads = (resource: Resource): Record<string, unknown>[] => {
  const platformData = asRecord(resource.platformData);
  const backupData = asRecord(platformData.backup);
  const dockerData = asRecord(platformData.docker);

  return dedupeArtifactCandidates([
    ...collectArtifactCandidates(platformData.backupArtifacts),
    ...collectArtifactCandidates(platformData.backups),
    ...collectArtifactCandidates(platformData.backup),
    ...collectArtifactCandidates(backupData.artifacts),
    ...collectArtifactCandidates(backupData.backups),
    ...collectArtifactCandidates(backupData.entries),
    ...collectArtifactCandidates(backupData.records),
    ...collectArtifactCandidates(backupData.docker),
    ...collectArtifactCandidates(dockerData.backup),
    ...collectArtifactCandidates(dockerData.backups),
    ...collectArtifactCandidates(dockerData.backupArtifacts),
  ]);
};

const categoryFromKubernetesArtifact = (resource: Resource, artifact: Record<string, unknown>): BackupCategory => {
  const workloadKind = readStringAny(artifact, ['workloadKind', 'kind', 'ownerKind']).toLowerCase();
  if (workloadKind.includes('node')) return 'host-backup';
  if (workloadKind.includes('cluster') || workloadKind.includes('namespace')) return 'config-backup';
  if (resource.type === 'k8s-cluster') return 'config-backup';
  if (resource.type === 'k8s-node') return 'host-backup';
  return 'container-backup';
};

const scopeFromKubernetesArtifact = (
  category: BackupCategory,
  workloadName: string,
  namespace: string,
  cluster: string,
): BackupScope => {
  if (category === 'host-backup') {
    return scope(workloadName || cluster || 'Node', 'host', 'host');
  }
  if (category === 'config-backup') {
    if (namespace) return scope(`Namespace ${namespace}`, 'namespace', 'other');
    return scope(cluster || 'Cluster', 'cluster', 'other');
  }
  return scope(workloadName || namespace || cluster || 'Workload', 'workload', 'pod');
};

const categoryFromDockerArtifact = (resource: Resource, artifact: Record<string, unknown>): BackupCategory => {
  const volume = readStringAny(artifact, ['volume', 'volumeName', 'volumeId']);
  if (volume) return 'volume-backup';
  if (resource.type === 'docker-host') return 'host-backup';
  if (resource.type === 'docker-container' || readStringAny(artifact, ['containerId', 'containerName']) !== '') {
    return 'container-backup';
  }
  return 'other';
};

const scopeFromDockerArtifact = (
  category: BackupCategory,
  host: string,
  containerName: string,
  volume: string,
): BackupScope => {
  if (category === 'host-backup') return scope(host || 'Docker Host', 'host', 'host');
  if (category === 'volume-backup') return scope(volume || host || 'Volume', 'workload', 'container');
  return scope(containerName || host || 'Container', 'workload', 'container');
};

const buildKubernetesArtifactRecords = (ctx: BackupAdapterContext): BackupRecord[] => {
  const records: BackupRecord[] = [];

  for (const resource of ctx.resources || []) {
    const platform = String(resource.platformType || '');
    const isKubernetesResource = platform === 'kubernetes' || resource.type.startsWith('k8s') || resource.type === 'pod';
    if (!isKubernetesResource) continue;

    const artifactPayloads = extractKubernetesArtifactPayloads(resource);
    if (artifactPayloads.length === 0) continue;

    const platformData = asRecord(resource.platformData);
    const kubernetesData = asRecord(platformData.kubernetes);

    for (const artifact of artifactPayloads) {
      const completedAt =
        readDateAny(artifact, ['backupTime', 'completedAt', 'finishedAt', 'timestamp', 'time', 'createdAt']) ||
        parseResourceLastBackup(resource);

      const namespace = firstNonEmpty(
        readStringAny(artifact, ['namespace', 'ns']),
        readString(kubernetesData, 'namespace'),
        readString(platformData, 'namespace'),
      );
      const cluster = firstNonEmpty(
        readStringAny(artifact, ['cluster', 'clusterId', 'clusterName']),
        readString(kubernetesData, 'clusterId'),
        readString(kubernetesData, 'clusterName'),
        resource.clusterId,
        resource.platformId,
      );
      const node = firstNonEmpty(
        readStringAny(artifact, ['node', 'nodeName', 'host']),
        readString(kubernetesData, 'nodeName'),
        readString(platformData, 'nodeName'),
      );
      const workloadKind = firstNonEmpty(
        readStringAny(artifact, ['workloadKind', 'kind', 'ownerKind']),
        readString(kubernetesData, 'ownerKind'),
      );
      const workloadName = firstNonEmpty(
        readStringAny(artifact, ['workloadName', 'ownerName', 'target']),
        readString(kubernetesData, 'ownerName'),
        resource.displayName,
        resource.name,
      );
      const repository = firstNonEmpty(readStringAny(artifact, ['repository', 'repo', 'store']), readStringAny(asRecord(platformData.backup), ['repository', 'repo', 'store']));
      const policy = firstNonEmpty(readStringAny(artifact, ['policy', 'schedule', 'policyName']));
      const snapshotClass = firstNonEmpty(readStringAny(artifact, ['snapshotClass', 'volumeSnapshotClass']));
      const backupId = firstNonEmpty(
        readStringAny(artifact, ['backupId', 'backupUid', 'uid']),
        readStringAny(artifact, ['id']),
      );
      const runId = firstNonEmpty(readStringAny(artifact, ['runId', 'jobId', 'executionId']));
      const mode = inferModeFromRecord({ ...platformData, ...kubernetesData, ...artifact }, 'container', 'remote');

      const verified = readBooleanAny(artifact, ['verified', 'isVerified', 'verificationPassed']);
      const protectedFlag = readBooleanAny(artifact, ['protected', 'immutable', 'isImmutable']);
      const encrypted = readBooleanAny(artifact, ['encrypted', 'isEncrypted']);
      const appAware =
        readBooleanAny(artifact, ['applicationAware', 'appAware']) === true ||
        readStringAny(artifact, ['hook', 'quiesce', 'snapshotClass']) !== '';
      const statusHint = readStringAny(artifact, ['status', 'phase', 'state', 'result']);
      const inferredOutcome = inferOutcome(verified, statusHint);
      const outcome = inferredOutcome === 'unknown' && completedAt ? 'success' : inferredOutcome;
      const category = categoryFromKubernetesArtifact(resource, artifact);

      const backupName = firstNonEmpty(
        readStringAny(artifact, ['backupName', 'name', 'displayName', 'title']),
        runId,
        backupId,
        `${workloadName || resource.name} backup`,
      );
      const scopeValue = scopeFromKubernetesArtifact(category, workloadName, namespace, cluster);

      const id =
        firstNonEmpty(backupId, runId) !== ''
          ? `k8s:${resource.id}:${firstNonEmpty(backupId, runId)}`
          : `k8s:${resource.id}:${completedAt || 'na'}:${workloadName || namespace || resource.name}`;

      records.push({
        id,
        name: backupName,
        category,
        outcome,
        mode,
        scope: scopeValue,
        location: node
          ? { label: node, scope: 'node' }
          : namespace
            ? { label: namespace, scope: 'namespace' }
            : cluster
              ? { label: cluster, scope: 'cluster' }
              : locationFromResource(resource),
        source: source('kubernetes', 'kubernetes-artifact-backups', 'resource'),
        completedAt,
        sizeBytes: readNumberAny(artifact, ['sizeBytes', 'size', 'bytes']),
        verified,
        protected: protectedFlag,
        encrypted,
        capabilities: backupCapabilitiesForArtifact('container', mode, encrypted, protectedFlag, verified, appAware),
        refs: {
          resourceId: resource.id,
          platformEntityId: resource.platformId,
        },
        kubernetes: {
          cluster,
          namespace,
          node,
          workloadKind,
          workloadName,
          policy,
          repository,
          snapshotClass,
          backupId,
          runId,
        },
      });
    }
  }

  return records;
};

const buildDockerArtifactRecords = (ctx: BackupAdapterContext): BackupRecord[] => {
  const records: BackupRecord[] = [];

  for (const resource of ctx.resources || []) {
    const platform = String(resource.platformType || '');
    const isDockerResource =
      platform === 'docker' || resource.type === 'docker-host' || resource.type === 'docker-container' || resource.type === 'docker-service';
    if (!isDockerResource) continue;

    const artifactPayloads = extractDockerArtifactPayloads(resource);
    if (artifactPayloads.length === 0) continue;

    const platformData = asRecord(resource.platformData);
    const dockerData = asRecord(platformData.docker);

    for (const artifact of artifactPayloads) {
      const completedAt =
        readDateAny(artifact, ['backupTime', 'completedAt', 'finishedAt', 'timestamp', 'time', 'createdAt']) ||
        parseResourceLastBackup(resource);

      const host = firstNonEmpty(
        readStringAny(artifact, ['host', 'hostName', 'node', 'nodeName']),
        readString(dockerData, 'hostname'),
        readString(platformData, 'hostName'),
        resource.platformId,
      );
      const containerId = firstNonEmpty(
        readStringAny(artifact, ['containerId', 'container', 'targetId']),
        readString(dockerData, 'containerId'),
      );
      const containerName = firstNonEmpty(
        readStringAny(artifact, ['containerName', 'target', 'workloadName']),
        resource.displayName,
        resource.name,
      );
      const image = firstNonEmpty(readStringAny(artifact, ['image', 'imageName']), readString(dockerData, 'image'));
      const volume = firstNonEmpty(readStringAny(artifact, ['volume', 'volumeName', 'volumeId']));
      const repository = firstNonEmpty(readStringAny(artifact, ['repository', 'repo', 'store']));
      const policy = firstNonEmpty(readStringAny(artifact, ['policy', 'schedule', 'policyName']));
      const backupId = firstNonEmpty(readStringAny(artifact, ['backupId', 'backupUid', 'id', 'uid']));
      const mode = inferModeFromRecord({ ...platformData, ...dockerData, ...artifact }, 'container', 'remote');

      const verified = readBooleanAny(artifact, ['verified', 'isVerified', 'verificationPassed']);
      const protectedFlag = readBooleanAny(artifact, ['protected', 'immutable', 'isImmutable']);
      const encrypted = readBooleanAny(artifact, ['encrypted', 'isEncrypted']);
      const appAware = readBooleanAny(artifact, ['applicationAware', 'appAware']) === true;
      const statusHint = readStringAny(artifact, ['status', 'phase', 'state', 'result']);
      const inferredOutcome = inferOutcome(verified, statusHint);
      const outcome = inferredOutcome === 'unknown' && completedAt ? 'success' : inferredOutcome;
      const category = categoryFromDockerArtifact(resource, artifact);
      const scopeValue = scopeFromDockerArtifact(category, host, containerName, volume);

      const backupName = firstNonEmpty(
        readStringAny(artifact, ['backupName', 'name', 'displayName', 'title']),
        volume ? `Volume ${volume}` : containerName,
        backupId,
        `${resource.name} backup`,
      );

      const id =
        backupId !== ''
          ? `docker:${resource.id}:${backupId}`
          : `docker:${resource.id}:${completedAt || 'na'}:${containerId || volume || containerName || host}`;

      records.push({
        id,
        name: backupName,
        category,
        outcome,
        mode,
        scope: scopeValue,
        location: host ? { label: host, scope: 'node' } : locationFromResource(resource),
        source: source('docker', 'docker-artifact-backups', 'resource'),
        completedAt,
        sizeBytes: readNumberAny(artifact, ['sizeBytes', 'size', 'bytes']),
        verified,
        protected: protectedFlag,
        encrypted,
        capabilities: backupCapabilitiesForArtifact('container', mode, encrypted, protectedFlag, verified, appAware),
        refs: {
          resourceId: resource.id,
          platformEntityId: resource.platformId,
        },
        docker: {
          host,
          containerId,
          containerName,
          image,
          volume,
          repository,
          policy,
          backupId,
        },
      });
    }
  }

  return records;
};

const hasArtifactBackups = (resource: Resource): boolean => {
  const platform = String(resource.platformType || '').toLowerCase();
  if (platform === 'kubernetes') return extractKubernetesArtifactPayloads(resource).length > 0;
  if (platform === 'docker') return extractDockerArtifactPayloads(resource).length > 0;
  return false;
};

const buildResourceBackupRecords = (ctx: BackupAdapterContext): BackupRecord[] => {
  const legacyPresent = hasLegacyBackupData(ctx);
  const records: BackupRecord[] = [];

  for (const resource of ctx.resources || []) {
    const completedAt = parseResourceLastBackup(resource);
    if (!completedAt) continue;

    const platform = (resource.platformType || 'generic') as StorageBackupPlatform;
    if (legacyPresent && isProxmoxPlatform(platform)) continue;
    if (hasArtifactBackups(resource)) continue;

    const platformData = asRecord(resource.platformData);
    const backupData = asRecord(platformData.backup);
    const mode = inferModeFromRecord({ ...backupData, ...platformData }, resolveFamily(platform), 'local');
    const verified = readBoolean(platformData, 'verified') ?? readBoolean(backupData, 'verified');
    const protectedFlag = readBoolean(platformData, 'protected') ?? readBoolean(backupData, 'protected');
    const encrypted = readBoolean(platformData, 'encrypted') ?? readBoolean(backupData, 'encrypted');

    const sourceDescriptor = source(platform, 'resource-backups', 'resource');
    const statusHint =
      readString(platformData, 'backupStatus') ||
      readString(platformData, 'status') ||
      readString(backupData, 'status');

    const inferredOutcome = inferOutcome(verified, statusHint);
    const outcome: BackupOutcome = inferredOutcome === 'unknown' ? 'success' : inferredOutcome;
    const category = categoryFromResourceType(resource.type);
    const scopeValue = scopeFromResourceType(resource.type);
    const workloadType = workloadTypeFromResourceType(resource.type);
    const vmidHint = readString(platformData, 'vmid') || readString(backupData, 'vmid');
    const scopeLabel =
      scopeValue === 'workload'
        ? vmidHint
          ? `VMID ${vmidHint}`
          : resource.name
        : resource.name;

    records.push({
      id: `resource:${resource.id}:backup:${completedAt}`,
      name: resource.displayName || resource.name || 'Resource Backup',
      category,
      outcome,
      mode,
      scope: scope(scopeLabel || 'Unknown', scopeValue, workloadType),
      location: locationFromResource(resource),
      source: sourceDescriptor,
      completedAt,
      sizeBytes: null,
      verified,
      protected: protectedFlag,
      encrypted,
      capabilities: backupCapabilitiesForResource(sourceDescriptor.family, mode, encrypted, protectedFlag),
      refs: {
        resourceId: resource.id,
        platformEntityId: resource.platformId,
      },
      details: {
        resourceType: resource.type,
        resourceId: resource.id,
        platformId: resource.platformId,
      },
    });
  }

  return records;
};

const pveSnapshotsAdapter: BackupAdapter = {
  id: 'legacy-pve-snapshots',
  supports: (ctx) => (ctx.state.backups?.pve?.guestSnapshots || ctx.state.pveBackups?.guestSnapshots || []).length > 0,
  build: buildSnapshotRecords,
};

const pveStorageBackupsAdapter: BackupAdapter = {
  id: 'legacy-pve-storage-backups',
  supports: (ctx) =>
    (ctx.state.backups?.pve?.storageBackups || ctx.state.pveBackups?.storageBackups || []).length > 0,
  build: buildPveStorageBackupRecords,
};

const pbsBackupsAdapter: BackupAdapter = {
  id: 'legacy-pbs-backups',
  supports: (ctx) => (ctx.state.backups?.pbs || ctx.state.pbsBackups || []).length > 0,
  build: buildPbsRecords,
};

const pmgBackupsAdapter: BackupAdapter = {
  id: 'legacy-pmg-backups',
  supports: (ctx) => (ctx.state.backups?.pmg || ctx.state.pmgBackups || []).length > 0,
  build: buildPmgRecords,
};

const kubernetesArtifactBackupsAdapter: BackupAdapter = {
  id: 'kubernetes-artifact-backups',
  supports: (ctx) => (ctx.resources || []).some((resource) => extractKubernetesArtifactPayloads(resource).length > 0),
  build: buildKubernetesArtifactRecords,
};

const dockerArtifactBackupsAdapter: BackupAdapter = {
  id: 'docker-artifact-backups',
  supports: (ctx) => (ctx.resources || []).some((resource) => extractDockerArtifactPayloads(resource).length > 0),
  build: buildDockerArtifactRecords,
};

const resourceBackupsAdapter: BackupAdapter = {
  id: 'resource-backups',
  supports: (ctx) => (ctx.resources || []).some((resource) => parseResourceLastBackup(resource) !== null),
  build: buildResourceBackupRecords,
};

export const DEFAULT_BACKUP_ADAPTERS: BackupAdapter[] = [
  kubernetesArtifactBackupsAdapter,
  dockerArtifactBackupsAdapter,
  resourceBackupsAdapter,
  pveSnapshotsAdapter,
  pveStorageBackupsAdapter,
  pbsBackupsAdapter,
  pmgBackupsAdapter,
];

const mergeBackupRecords = (current: BackupRecord, incoming: BackupRecord): BackupRecord => ({
  ...current,
  ...(incoming.source.origin === 'resource' ? incoming : {}),
  mode: incoming.mode || current.mode,
  proxmox: {
    ...(current.proxmox || {}),
    ...(incoming.proxmox || {}),
  },
  kubernetes: {
    ...(current.kubernetes || {}),
    ...(incoming.kubernetes || {}),
  },
  docker: {
    ...(current.docker || {}),
    ...(incoming.docker || {}),
  },
  capabilities: dedupe([...(current.capabilities || []), ...(incoming.capabilities || [])]),
  details: {
    ...(current.details || {}),
    ...(incoming.details || {}),
  },
});

export const buildBackupRecords = (
  ctx: BackupAdapterContext,
  adapters: BackupAdapter[] = DEFAULT_BACKUP_ADAPTERS,
): BackupRecord[] => {
  const map = new Map<string, BackupRecord>();
  for (const adapter of adapters) {
    if (!adapter.supports(ctx)) continue;
    const records = adapter.build(ctx);
    for (const record of records) {
      const existing = map.get(record.id);
      if (!existing) {
        map.set(record.id, record);
        continue;
      }
      map.set(record.id, mergeBackupRecords(existing, record));
    }
  }
  return Array.from(map.values());
};
