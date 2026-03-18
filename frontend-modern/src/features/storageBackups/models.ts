import type { State } from '@/types/api';
import type { Resource } from '@/types/resource';

export const KNOWN_STORAGE_BACKUP_PLATFORMS = [
  'proxmox-pve',
  'proxmox-pbs',
  'proxmox-pmg',
  'kubernetes',
  'docker',
  'agent',
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

export type NormalizedHealth = 'healthy' | 'warning' | 'critical' | 'offline' | 'unknown';

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

export interface SourceDescriptor {
  platform: StorageBackupPlatform;
  family: PlatformFamily;
  origin: 'resource';
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

export interface StorageMetricsTarget {
  resourceType: string;
  resourceId: string;
}

export interface StorageRecord {
  id: string;
  name: string;
  category: StorageCategory;
  health: NormalizedHealth;
  statusLabel?: string;
  hostLabel?: string;
  platformLabel?: string;
  platformKey?: StorageBackupPlatform;
  topologyLabel?: string;
  protectionLabel?: string;
  protectionReduced?: boolean;
  rebuildInProgress?: boolean;
  incidentCategory?: string;
  incidentSeverity?: string;
  incidentPriority?: number;
  issueLabel?: string;
  issueSummary?: string;
  actionSummary?: string;
  impactSummary?: string;
  consumerCount?: number;
  protectedWorkloadCount?: number;
  affectedDatastoreCount?: number;
  location: StorageLocation;
  capacity: CapacitySnapshot;
  capabilities: StorageCapability[];
  source: SourceDescriptor;
  observedAt: number;
  metricsTarget?: StorageMetricsTarget;
  refs?: {
    resourceId?: string;
    platformEntityId?: string;
  };
  details?: Record<string, unknown>;
}

export interface StorageAdapterContext {
  state: State;
  resources: Resource[];
}

export interface StorageAdapter {
  id: string;
  supports: (ctx: StorageAdapterContext) => boolean;
  build: (ctx: StorageAdapterContext) => StorageRecord[];
}
