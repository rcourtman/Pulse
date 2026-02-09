import type { State } from '@/types/api';
import type { Resource } from '@/types/resource';

export const KNOWN_STORAGE_BACKUP_PLATFORMS = [
  'proxmox-pve',
  'proxmox-pbs',
  'proxmox-pmg',
  'kubernetes',
  'docker',
  'host-agent',
  'truenas',
  'unraid',
  'synology-dsm',
  'vmware-vsphere',
  'microsoft-hyperv',
  'aws',
  'azure',
  'gcp',
  'generic',
] as const;

export type KnownStorageBackupPlatform = (typeof KNOWN_STORAGE_BACKUP_PLATFORMS)[number];
export type StorageBackupPlatform = KnownStorageBackupPlatform | (string & {});

export type PlatformFamily = 'onprem' | 'container' | 'virtualization' | 'cloud' | 'generic';
export type DataOrigin = 'resource' | 'legacy';
export const STORAGE_DATA_ORIGIN_PRECEDENCE: Record<DataOrigin, number> = {
  legacy: 0,
  resource: 1,
};

export type NormalizedHealth = 'healthy' | 'warning' | 'critical' | 'offline' | 'unknown';
export type BackupOutcome = 'success' | 'warning' | 'failed' | 'running' | 'offline' | 'unknown';

export type StorageCategory =
  | 'pool'
  | 'datastore'
  | 'dataset'
  | 'volume'
  | 'filesystem'
  | 'object-bucket'
  | 'backup-repository'
  | 'share'
  | 'other';

export type BackupCategory =
  | 'snapshot'
  | 'vm-backup'
  | 'container-backup'
  | 'host-backup'
  | 'config-backup'
  | 'database-backup'
  | 'volume-backup'
  | 'other';

export type StorageCapability =
  | 'capacity'
  | 'health'
  | 'object-bucket'
  | 'deduplication'
  | 'compression'
  | 'encryption'
  | 'replication'
  | 'snapshots'
  | 'immutability'
  | 'namespaces'
  | 'tiering'
  | 'multi-node'
  | 'backup-repository';

export type BackupCapability =
  | 'retention'
  | 'verification'
  | 'encryption'
  | 'immutability'
  | 'incremental'
  | 'policy-driven'
  | 'cross-site'
  | 'application-aware';

export type BackupMode = 'snapshot' | 'local' | 'remote';

export interface SourceDescriptor {
  platform: StorageBackupPlatform;
  family: PlatformFamily;
  origin: DataOrigin;
  adapterId: string;
}

export interface StorageLocation {
  label: string;
  scope: 'node' | 'cluster' | 'namespace' | 'host' | 'region' | 'global' | 'unknown';
}

export interface CapacitySnapshot {
  totalBytes: number | null;
  usedBytes: number | null;
  freeBytes: number | null;
  usagePercent: number | null;
}

export interface StorageRecord {
  id: string;
  name: string;
  category: StorageCategory;
  health: NormalizedHealth;
  location: StorageLocation;
  capacity: CapacitySnapshot;
  capabilities: StorageCapability[];
  source: SourceDescriptor;
  observedAt: number;
  refs?: {
    resourceId?: string;
    legacyStorageId?: string;
    platformEntityId?: string;
  };
  details?: Record<string, unknown>;
}

export interface BackupScope {
  label: string;
  scope: 'workload' | 'host' | 'cluster' | 'namespace' | 'tenant' | 'unknown';
  workloadType?: 'vm' | 'container' | 'pod' | 'host' | 'other';
}

export interface ProxmoxBackupDetails {
  vmid?: string;
  node?: string;
  instance?: string;
  storage?: string;
  datastore?: string;
  namespace?: string;
  backupType?: string;
  owner?: string;
  comment?: string;
  notes?: string;
  filename?: string;
}

export interface KubernetesBackupDetails {
  cluster?: string;
  namespace?: string;
  node?: string;
  workloadKind?: string;
  workloadName?: string;
  policy?: string;
  repository?: string;
  snapshotClass?: string;
  backupId?: string;
  runId?: string;
}

export interface DockerBackupDetails {
  host?: string;
  containerId?: string;
  containerName?: string;
  image?: string;
  volume?: string;
  repository?: string;
  policy?: string;
  backupId?: string;
}

export interface BackupRecord {
  id: string;
  name: string;
  category: BackupCategory;
  outcome: BackupOutcome;
  mode: BackupMode;
  scope: BackupScope;
  location: StorageLocation;
  source: SourceDescriptor;
  completedAt: number | null;
  sizeBytes: number | null;
  verified: boolean | null;
  protected: boolean | null;
  encrypted: boolean | null;
  capabilities: BackupCapability[];
  refs?: {
    resourceId?: string;
    legacyBackupId?: string;
    platformEntityId?: string;
  };
  proxmox?: ProxmoxBackupDetails;
  kubernetes?: KubernetesBackupDetails;
  docker?: DockerBackupDetails;
  details?: Record<string, unknown>;
}

export interface StorageAdapterContext {
  state: State;
  resources: Resource[];
}

export interface BackupAdapterContext {
  state: State;
  resources: Resource[];
}

export interface StorageAdapter {
  id: string;
  supports: (ctx: StorageAdapterContext) => boolean;
  build: (ctx: StorageAdapterContext) => StorageRecord[];
}

export interface BackupAdapter {
  id: string;
  supports: (ctx: BackupAdapterContext) => boolean;
  build: (ctx: BackupAdapterContext) => BackupRecord[];
}
