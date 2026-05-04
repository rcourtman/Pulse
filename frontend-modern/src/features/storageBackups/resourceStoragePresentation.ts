import type { Resource } from '@/types/resource';
import { getSourcePlatformLabel, normalizeSourcePlatformKey } from '@/utils/sourcePlatforms';
import type { StorageBackupPlatform } from './models';
import { isBackupRepositoryStorageResource } from './resourceStorageMapping';

type StorageRiskLike = {
  level?: string;
  reasons?: { summary?: string }[];
};

const titleize = (value: string | undefined | null): string =>
  (value || '')
    .split(/[\s_-]+/)
    .filter(Boolean)
    .map((part) => part.charAt(0).toUpperCase() + part.slice(1))
    .join(' ');

const trimSummary = (value: string | undefined | null): string => (value || '').trim();

const firstRiskReasonSummary = (risk: StorageRiskLike | undefined): string => {
  if (!risk?.reasons?.length) return '';
  return risk.reasons.map((reason) => trimSummary(reason.summary)).find(Boolean) || '';
};

const getResourceStorageRiskIssue = (resource: Resource): string =>
  trimSummary(resource.storage?.riskSummary) ||
  firstRiskReasonSummary(resource.storage?.risk) ||
  firstRiskReasonSummary(resource.pbs?.storageRisk);

const isAttentionStatus = (value: string | undefined): boolean => {
  const normalized = (value || '').trim().toLowerCase();
  if (!normalized) return false;
  return (
    normalized === 'warning' ||
    normalized === 'warn' ||
    normalized === 'degraded' ||
    normalized === 'critical' ||
    normalized === 'faulted' ||
    normalized === 'failed' ||
    normalized === 'error' ||
    normalized === 'unhealthy' ||
    normalized === 'offline' ||
    normalized === 'down' ||
    normalized === 'unavailable'
  );
};

const getCompositePostureIssue = (resource: Resource): string => {
  const posture =
    trimSummary(resource.storage?.postureSummary) || trimSummary(resource.pbs?.postureSummary);
  if (!posture || !isAttentionStatus(resource.status)) return '';

  const impactSummaries = [
    resource.storage?.consumerImpactSummary,
    resource.pbs?.protectedWorkloadSummary,
    resource.pbs?.affectedDatastoreSummary,
  ]
    .map(trimSummary)
    .filter(Boolean);

  if (impactSummaries.some((summary) => summary === posture)) return '';
  return posture;
};

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

const getStorageClassificationContext = (resource: Resource) => {
  const storage = (resource.storage as Record<string, unknown> | undefined) || {};
  const platformData = (resource.platformData as Record<string, unknown> | undefined) || {};
  return {
    resourceType: resource.type,
    platform:
      (typeof storage.platform === 'string' ? storage.platform : undefined) ||
      (typeof platformData.platform === 'string' ? platformData.platform : undefined) ||
      resource.platformType,
    topology:
      (typeof storage.topology === 'string' ? storage.topology : undefined) ||
      (typeof platformData.topology === 'string' ? platformData.topology : undefined),
    entityType: resource.vmware?.entityType,
  };
};

export const getResourceStorageTopologyLabel = (
  resource: Resource,
  storageType: string,
  topology?: string,
): string => {
  const normalized = (topology || '').trim().toLowerCase();
  if (normalized) {
    return titleize(normalized);
  }
  const classification = getStorageClassificationContext(resource);
  if (
    isBackupRepositoryStorageResource(
      storageType,
      {
        platform: classification.platform,
        topology: classification.topology,
      },
      classification,
    )
  ) {
    return 'Backup Target';
  }
  if (
    resource.type === 'datastore' ||
    (classification.entityType || '').trim().toLowerCase() === 'datastore'
  ) {
    return 'Datastore';
  }
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
  const riskIssue = getResourceStorageRiskIssue(resource);
  if (riskIssue) return riskIssue;
  const postureIssue = getCompositePostureIssue(resource);
  if (postureIssue) return postureIssue;
  return 'Healthy';
};

export const getResourceStorageIssueSummary = (resource: Resource): string => {
  if (resource.incidentSummary?.trim()) return resource.incidentSummary.trim();
  const riskIssue = getResourceStorageRiskIssue(resource);
  if (riskIssue) return riskIssue;
  const postureIssue = getCompositePostureIssue(resource);
  if (postureIssue) return postureIssue;
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
  const storageType =
    resource.storage?.type ||
    ((resource.platformData as Record<string, unknown> | undefined)?.type as string | undefined);
  const classification = getStorageClassificationContext(resource);
  if (
    isBackupRepositoryStorageResource(
      storageType,
      {
        platform: classification.platform,
        topology: classification.topology,
      },
      classification,
    )
  ) {
    return 'Protected';
  }
  return 'Healthy';
};
