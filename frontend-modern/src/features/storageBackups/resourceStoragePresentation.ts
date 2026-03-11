import type { Resource } from '@/types/resource';
import { getSourcePlatformLabel, normalizeSourcePlatformKey } from '@/utils/sourcePlatforms';
import type { StorageBackupPlatform } from './models';

const titleize = (value: string | undefined | null): string =>
  (value || '')
    .split(/[\s_-]+/)
    .filter(Boolean)
    .map((part) => part.charAt(0).toUpperCase() + part.slice(1))
    .join(' ');

export const getCanonicalStoragePlatformKey = (
  resource: Resource,
  storagePlatform?: string,
): StorageBackupPlatform =>
  (normalizeSourcePlatformKey(storagePlatform) ||
    normalizeSourcePlatformKey(resource.platformType) ||
    resource.platformType ||
    (storagePlatform || '').trim().toLowerCase() ||
    'generic') as StorageBackupPlatform;

export const getResourceStoragePlatformLabel = (platform: StorageBackupPlatform): string =>
  getSourcePlatformLabel(platform);

export const getResourceStorageTopologyLabel = (
  resource: Resource,
  storageType: string,
  topology?: string,
): string => {
  const normalized = (topology || '').trim().toLowerCase();
  if (normalized) {
    return titleize(normalized);
  }
  if (resource.type === 'datastore') return 'Backup Target';
  switch ((storageType || '').trim().toLowerCase()) {
    case 'zfspool':
    case 'zfs-pool':
    case 'pool':
      return 'Pool';
    case 'zfs-dataset':
    case 'dataset':
      return 'Dataset';
    case 'dir':
    case 'filesystem':
      return 'Filesystem';
    case 'pbs':
      return 'Backup Target';
    case 'rbd':
    case 'cephfs':
      return 'Cluster Storage';
    default:
      return titleize(storageType) || titleize(resource.type) || 'Storage';
  }
};

export const getResourceStorageIssueLabel = (resource: Resource): string => {
  if (resource.incidentLabel?.trim()) return resource.incidentLabel.trim();
  if (resource.storage?.postureSummary?.trim()) return resource.storage.postureSummary.trim();
  if (resource.storage?.riskSummary?.trim()) return resource.storage.riskSummary.trim();
  if (resource.pbs?.postureSummary?.trim()) return resource.pbs.postureSummary.trim();
  return 'Healthy';
};

export const getResourceStorageIssueSummary = (resource: Resource): string => {
  if (resource.incidentSummary?.trim()) return resource.incidentSummary.trim();
  if (resource.storage?.riskSummary?.trim()) return resource.storage.riskSummary.trim();
  if (resource.storage?.postureSummary?.trim()) return resource.storage.postureSummary.trim();
  if (resource.pbs?.postureSummary?.trim()) return resource.pbs.postureSummary.trim();
  return '';
};

export const getResourceStorageImpactSummary = (resource: Resource): string => {
  if (resource.incidentImpactSummary?.trim()) return resource.incidentImpactSummary.trim();
  if (resource.storage?.consumerImpactSummary?.trim())
    return resource.storage.consumerImpactSummary.trim();
  if (resource.pbs?.protectedWorkloadSummary?.trim())
    return resource.pbs.protectedWorkloadSummary.trim();
  if (resource.pbs?.affectedDatastoreSummary?.trim())
    return resource.pbs.affectedDatastoreSummary.trim();
  return 'No dependent resources';
};

export const getResourceStorageActionSummary = (resource: Resource): string => {
  if (resource.incidentAction?.trim()) return resource.incidentAction.trim();
  if (resource.storage?.rebuildInProgress) {
    return resource.storage.rebuildSummary || 'Monitor rebuild progress';
  }
  if (resource.storage?.protectionReduced) {
    return resource.storage.protectionSummary || 'Restore redundancy';
  }
  return 'Monitor';
};

export const getResourceStorageProtectionLabel = (resource: Resource): string => {
  if (resource.storage?.rebuildInProgress) {
    return resource.storage.rebuildSummary || 'Rebuild In Progress';
  }
  if (resource.storage?.protectionReduced) {
    return resource.storage.protectionSummary || 'Protection Reduced';
  }
  if (resource.incidentCategory === 'recoverability') {
    return resource.incidentLabel || 'Backup Risk';
  }
  if (resource.storage?.protection?.trim()) {
    return titleize(resource.storage.protection.trim());
  }
  if (resource.type === 'datastore' || resource.type === 'pbs') {
    return 'Protected';
  }
  return 'Healthy';
};
