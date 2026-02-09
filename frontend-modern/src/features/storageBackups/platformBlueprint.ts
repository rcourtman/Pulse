import type {
  BackupCapability,
  KnownStorageBackupPlatform,
  PlatformFamily,
  StorageCapability,
} from './models';

export interface PlatformBlueprint {
  id: KnownStorageBackupPlatform;
  label: string;
  family: PlatformFamily;
  stage: 'current' | 'next' | 'future';
  storageCapabilities: StorageCapability[];
  backupCapabilities: BackupCapability[];
}

// This matrix drives future adapter planning: adding a new platform should only
// require adapter implementation and capability mapping, not page rewrites.
export const PLATFORM_BLUEPRINTS: PlatformBlueprint[] = [
  {
    id: 'proxmox-pve',
    label: 'Proxmox VE',
    family: 'virtualization',
    stage: 'current',
    storageCapabilities: ['capacity', 'health', 'snapshots', 'multi-node'],
    backupCapabilities: ['retention', 'incremental'],
  },
  {
    id: 'proxmox-pbs',
    label: 'Proxmox Backup Server',
    family: 'virtualization',
    stage: 'current',
    storageCapabilities: ['capacity', 'health', 'deduplication', 'backup-repository', 'namespaces'],
    backupCapabilities: ['retention', 'verification', 'encryption', 'immutability', 'incremental'],
  },
  {
    id: 'proxmox-pmg',
    label: 'Proxmox Mail Gateway',
    family: 'virtualization',
    stage: 'current',
    storageCapabilities: ['health'],
    backupCapabilities: ['retention'],
  },
  {
    id: 'kubernetes',
    label: 'Kubernetes',
    family: 'container',
    stage: 'next',
    storageCapabilities: ['capacity', 'health', 'replication', 'snapshots'],
    backupCapabilities: ['policy-driven', 'cross-site'],
  },
  {
    id: 'truenas',
    label: 'TrueNAS',
    family: 'onprem',
    stage: 'next',
    storageCapabilities: ['capacity', 'health', 'snapshots', 'replication', 'compression', 'encryption'],
    backupCapabilities: ['retention', 'cross-site'],
  },
  {
    id: 'unraid',
    label: 'Unraid',
    family: 'onprem',
    stage: 'next',
    storageCapabilities: ['capacity', 'health', 'snapshots'],
    backupCapabilities: ['retention'],
  },
  {
    id: 'synology-dsm',
    label: 'Synology DSM',
    family: 'onprem',
    stage: 'future',
    storageCapabilities: ['capacity', 'health', 'snapshots', 'replication'],
    backupCapabilities: ['retention', 'verification', 'cross-site'],
  },
  {
    id: 'vmware-vsphere',
    label: 'VMware vSphere',
    family: 'virtualization',
    stage: 'future',
    storageCapabilities: ['capacity', 'health', 'multi-node'],
    backupCapabilities: ['retention', 'application-aware'],
  },
  {
    id: 'microsoft-hyperv',
    label: 'Microsoft Hyper-V',
    family: 'virtualization',
    stage: 'future',
    storageCapabilities: ['capacity', 'health'],
    backupCapabilities: ['retention', 'application-aware'],
  },
  {
    id: 'aws',
    label: 'Amazon Web Services',
    family: 'cloud',
    stage: 'future',
    storageCapabilities: ['capacity', 'health', 'tiering', 'encryption', 'object-bucket'],
    backupCapabilities: ['retention', 'immutability', 'cross-site', 'policy-driven'],
  },
  {
    id: 'azure',
    label: 'Microsoft Azure',
    family: 'cloud',
    stage: 'future',
    storageCapabilities: ['capacity', 'health', 'tiering', 'encryption', 'object-bucket'],
    backupCapabilities: ['retention', 'immutability', 'cross-site', 'policy-driven'],
  },
  {
    id: 'gcp',
    label: 'Google Cloud',
    family: 'cloud',
    stage: 'future',
    storageCapabilities: ['capacity', 'health', 'tiering', 'encryption', 'object-bucket'],
    backupCapabilities: ['retention', 'immutability', 'cross-site', 'policy-driven'],
  },
  {
    id: 'docker',
    label: 'Docker',
    family: 'container',
    stage: 'future',
    storageCapabilities: ['capacity', 'health'],
    backupCapabilities: ['retention'],
  },
  {
    id: 'host-agent',
    label: 'Host Agent',
    family: 'onprem',
    stage: 'future',
    storageCapabilities: ['capacity', 'health'],
    backupCapabilities: ['retention'],
  },
  {
    id: 'generic',
    label: 'Generic',
    family: 'generic',
    stage: 'future',
    storageCapabilities: ['capacity', 'health'],
    backupCapabilities: ['retention'],
  },
];

export const PLATFORM_BLUEPRINT_BY_ID = new Map(
  PLATFORM_BLUEPRINTS.map((blueprint) => [blueprint.id, blueprint] as const),
);

