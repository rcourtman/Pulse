import type { PBSBackup, PMGBackup, StorageBackup } from '@/types/api';
import type { Resource, ResourceType } from '@/types/resource';
import type {
  BackupCapability,
  BackupCategory,
  BackupOutcome,
  BackupRecordV2,
  BackupScope,
  BackupV2Adapter,
  BackupV2AdapterContext,
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

const isProxmoxPlatform = (platform: string): boolean =>
  platform === 'proxmox-pve' || platform === 'proxmox-pbs' || platform === 'proxmox-pmg';

const hasLegacyBackupData = (ctx: BackupV2AdapterContext): boolean =>
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

const inferModeFromResource = (
  platformData: Record<string, unknown>,
  family: SourceDescriptor['family'],
): 'snapshot' | 'local' | 'remote' => {
  const explicit = readString(platformData, 'mode').toLowerCase() || readString(platformData, 'backupMode').toLowerCase();
  if (explicit === 'snapshot' || explicit === 'local' || explicit === 'remote') return explicit;

  if (readBoolean(platformData, 'snapshot') === true) return 'snapshot';
  if (readBoolean(platformData, 'remote') === true) return 'remote';
  if (family === 'cloud') return 'remote';
  return 'local';
};

const backupCapabilitiesForResource = (
  family: SourceDescriptor['family'],
  mode: 'snapshot' | 'local' | 'remote',
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

const locationFromResource = (resource: Resource): BackupRecordV2['location'] => {
  const platformData = asRecord(resource.platformData);
  const proxmoxData = asRecord(platformData.proxmox);
  const kubernetesData = asRecord(platformData.kubernetes);
  const node =
    readString(platformData, 'node') ||
    readString(platformData, 'nodeName') ||
    readString(platformData, 'hostName') ||
    readString(proxmoxData, 'nodeName') ||
    readString(kubernetesData, 'nodeName');
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

const inferOutcome = (verified: boolean | null | undefined, status?: string): BackupOutcome => {
  const normalized = (status || '').toLowerCase();
  if (normalized.includes('running')) return 'running';
  if (normalized.includes('fail') || normalized.includes('error')) return 'failed';
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

const buildSnapshotRecords = (ctx: BackupV2AdapterContext): BackupRecordV2[] => {
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
      details: {
        vmid: snapshot.vmid,
        instance: snapshot.instance,
        node: snapshot.node,
        snapshotType: snapshot.type,
        mode: 'snapshot',
      },
    };
  });
};

const buildPveStorageBackupRecords = (ctx: BackupV2AdapterContext): BackupRecordV2[] => {
  const pve = ctx.state.backups?.pve ?? ctx.state.pveBackups;
  const storageBackups = pve?.storageBackups || [];
  return storageBackups
    .filter((backup) => backup.type !== 'vztmpl' && backup.type !== 'iso')
    .map((backup: StorageBackup) => {
      const completedAt = parseDateMillis(backup.ctime);
      const category = inferCategory(backup.type, backup.vmid);
      const verified = typeof backup.verified === 'boolean' ? backup.verified : null;
      const encrypted = backup.encryption ? true : null;
      const backupName = backup.volid?.split('/').pop() || backup.notes || `Backup ${backup.vmid}`;
      const backupSource = backup.isPBS ? 'proxmox-pbs' : 'proxmox-pve';
      return {
        id: `pve:${backup.instance}:${backup.node}:${backup.vmid}:${backup.ctime}:${backup.volid || backupName}`,
        name: backupName,
        category,
        outcome: inferOutcome(verified, backup.verification),
        scope: scope(
          category === 'host-backup' ? (backup.node || 'Host') : `VMID ${backup.vmid}`,
          category === 'host-backup' ? 'host' : 'workload',
          category === 'vm-backup'
            ? 'vm'
            : category === 'container-backup'
              ? 'container'
              : category === 'host-backup'
                ? 'host'
                : 'other',
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
        details: {
          vmid: backup.vmid,
          node: backup.node,
          instance: backup.instance,
          storage: backup.storage,
          backupType: backup.type,
          notes: backup.notes,
          mode: backup.isPBS ? 'remote' : 'local',
        },
      };
    });
};

const buildPbsRecords = (ctx: BackupV2AdapterContext): BackupRecordV2[] => {
  const backups = ctx.state.backups?.pbs ?? ctx.state.pbsBackups;
  return (backups || []).map((backup: PBSBackup) => {
    const category = inferCategory(backup.backupType, backup.vmid);
    const completedAt = parseDateMillis(backup.backupTime);
    const encrypted =
      Array.isArray(backup.files) &&
      backup.files.some((entry) =>
        typeof entry === 'string' ? entry.includes('.enc') : String(entry ?? '').includes('.enc'),
      );
    return {
      id:
        backup.id ||
        `pbs:${backup.instance}:${backup.datastore}:${backup.namespace}:${backup.backupType}:${backup.vmid}:${backup.backupTime}`,
      name: backup.comment || `${backup.backupType}/${backup.vmid}`,
      category,
      outcome: inferOutcome(backup.verified, undefined),
      scope: scope(
        category === 'host-backup' ? (backup.instance || 'Host') : `VMID ${backup.vmid}`,
        category === 'host-backup' ? 'host' : 'workload',
        category === 'vm-backup'
          ? 'vm'
          : category === 'container-backup'
            ? 'container'
            : category === 'host-backup'
              ? 'host'
              : 'other',
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
      details: {
        datastore: backup.datastore,
        namespace: backup.namespace,
        owner: backup.owner,
        comment: backup.comment,
        mode: 'remote',
      },
    };
  });
};

const buildPmgRecords = (ctx: BackupV2AdapterContext): BackupRecordV2[] => {
  const backups = ctx.state.backups?.pmg ?? ctx.state.pmgBackups;
  return (backups || []).map((backup: PMGBackup) => ({
    id: backup.id || `pmg:${backup.instance}:${backup.node}:${backup.filename}:${backup.backupTime}`,
    name: backup.filename || `PMG backup ${backup.node}`,
    category: 'config-backup',
    outcome: 'success',
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
    details: {
      node: backup.node,
      filename: backup.filename,
      mode: 'local',
    },
  }));
};

const buildResourceBackupRecords = (ctx: BackupV2AdapterContext): BackupRecordV2[] => {
  const legacyPresent = hasLegacyBackupData(ctx);
  const records: BackupRecordV2[] = [];

  for (const resource of ctx.resources || []) {
    const completedAt = parseResourceLastBackup(resource);
    if (!completedAt) continue;

    const platform = (resource.platformType || 'generic') as StorageBackupPlatform;
    if (legacyPresent && isProxmoxPlatform(platform)) continue;

    const platformData = asRecord(resource.platformData);
    const backupData = asRecord(platformData.backup);
    const mergedDetails = { ...backupData, ...platformData };
    const verified = readBoolean(platformData, 'verified') ?? readBoolean(backupData, 'verified');
    const protectedFlag = readBoolean(platformData, 'protected') ?? readBoolean(backupData, 'protected');
    const encrypted = readBoolean(platformData, 'encrypted') ?? readBoolean(backupData, 'encrypted');

    const sourceDescriptor = source(platform, 'resource-backups', 'resource');
    const mode = inferModeFromResource(mergedDetails, sourceDescriptor.family);
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
        mode,
        resourceType: resource.type,
        resourceId: resource.id,
        platformId: resource.platformId,
        node: readString(platformData, 'node') || readString(platformData, 'nodeName'),
        namespace: readString(platformData, 'namespace'),
        vmid: vmidHint,
      },
    });
  }

  return records;
};

const pveSnapshotsAdapter: BackupV2Adapter = {
  id: 'legacy-pve-snapshots',
  supports: (ctx) => (ctx.state.backups?.pve?.guestSnapshots || ctx.state.pveBackups?.guestSnapshots || []).length > 0,
  build: buildSnapshotRecords,
};

const pveStorageBackupsAdapter: BackupV2Adapter = {
  id: 'legacy-pve-storage-backups',
  supports: (ctx) =>
    (ctx.state.backups?.pve?.storageBackups || ctx.state.pveBackups?.storageBackups || []).length > 0,
  build: buildPveStorageBackupRecords,
};

const pbsBackupsAdapter: BackupV2Adapter = {
  id: 'legacy-pbs-backups',
  supports: (ctx) => (ctx.state.backups?.pbs || ctx.state.pbsBackups || []).length > 0,
  build: buildPbsRecords,
};

const pmgBackupsAdapter: BackupV2Adapter = {
  id: 'legacy-pmg-backups',
  supports: (ctx) => (ctx.state.backups?.pmg || ctx.state.pmgBackups || []).length > 0,
  build: buildPmgRecords,
};

const resourceBackupsAdapter: BackupV2Adapter = {
  id: 'resource-backups',
  supports: (ctx) => (ctx.resources || []).some((resource) => parseResourceLastBackup(resource) !== null),
  build: buildResourceBackupRecords,
};

export const DEFAULT_BACKUP_V2_ADAPTERS: BackupV2Adapter[] = [
  resourceBackupsAdapter,
  pveSnapshotsAdapter,
  pveStorageBackupsAdapter,
  pbsBackupsAdapter,
  pmgBackupsAdapter,
];

const mergeBackupRecords = (current: BackupRecordV2, incoming: BackupRecordV2): BackupRecordV2 => ({
  ...current,
  ...(incoming.source.origin === 'resource' ? incoming : {}),
  capabilities: dedupe([...(current.capabilities || []), ...(incoming.capabilities || [])]),
  details: {
    ...(current.details || {}),
    ...(incoming.details || {}),
  },
});

export const buildBackupRecordsV2 = (
  ctx: BackupV2AdapterContext,
  adapters: BackupV2Adapter[] = DEFAULT_BACKUP_V2_ADAPTERS,
): BackupRecordV2[] => {
  const map = new Map<string, BackupRecordV2>();
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
