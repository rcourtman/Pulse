import type { Resource } from '@/types/resource';
import { getSourcePlatformLabel, normalizeSourcePlatformKey } from '@/utils/sourcePlatforms';
import type { StorageBackupPlatform } from './models';
import { isBackupRepositoryStorageResource } from './resourceStorageMapping';

type StorageRiskReasonLike = { code?: string; summary?: string };

type StorageRiskLike = {
  level?: string;
  reasons?: StorageRiskReasonLike[];
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

const collectRiskReasons = (resource: Resource): StorageRiskReasonLike[] => {
  const out: StorageRiskReasonLike[] = [];
  for (const reason of resource.storage?.risk?.reasons || []) {
    if (reason) out.push(reason);
  }
  for (const reason of resource.pbs?.storageRisk?.reasons || []) {
    if (reason) out.push(reason);
  }
  return out;
};

const findRiskReason = (
  resource: Resource,
  codes: string[],
): StorageRiskReasonLike | undefined => {
  const wanted = new Set(codes.map((code) => code.toLowerCase()));
  for (const reason of collectRiskReasons(resource)) {
    const code = (reason.code || '').toLowerCase();
    if (wanted.has(code)) return reason;
  }
  return undefined;
};

const isUnraidStorageResource = (resource: Resource): boolean => {
  const storage = resource.storage;
  if (!storage) return false;
  if (storage.arrayState || storage.syncAction) return true;
  for (const reason of storage.risk?.reasons || []) {
    if ((reason.code || '').toLowerCase().startsWith('unraid_')) return true;
  }
  return false;
};

const unraidShortSyncLabel = (
  syncAction: string | undefined,
  syncProgress?: number,
): string => {
  const action = (syncAction || '').trim().toLowerCase();
  let base: string;
  switch (action) {
    case 'check':
      base = 'Parity check';
      break;
    case 'recon':
    case 'recon-p':
    case 'rebuild':
      base = 'Parity rebuild';
      break;
    case 'sync':
      base = 'Parity sync';
      break;
    case 'clear':
      base = 'Clearing';
      break;
    case '':
      base = 'Rebuilding';
      break;
    default:
      base = titleize(action);
      break;
  }
  if (typeof syncProgress === 'number' && Number.isFinite(syncProgress) && syncProgress > 0) {
    return `${base} (${Math.round(syncProgress)}%)`;
  }
  return base;
};

const getUnraidShortProtectionLabel = (resource: Resource): string => {
  const storage = resource.storage;
  if (!storage) return '';
  if (storage.rebuildInProgress) {
    return unraidShortSyncLabel(storage.syncAction, storage.syncProgress);
  }
  if (storage.protectionReduced) {
    if ((storage.protection || '').trim().toLowerCase() === 'none') {
      return 'Unprotected';
    }
    if (findRiskReason(resource, ['unraid_no_parity'])) return 'Unprotected';
    if (findRiskReason(resource, ['unraid_parity_unavailable'])) return 'Parity unavailable';
    return 'Protection reduced';
  }
  return '';
};

const getUnraidShortIssueLabel = (resource: Resource): string => {
  const reasons = collectRiskReasons(resource);
  if (reasons.length === 0) return '';
  for (const reason of reasons) {
    const code = (reason.code || '').toLowerCase();
    switch (code) {
      case 'unraid_parity_unavailable':
        return 'Parity unavailable';
      case 'unraid_no_parity':
        return 'No parity protection';
      case 'unraid_invalid_disks':
      case 'unraid_disabled_disks':
      case 'unraid_missing_disks':
        return trimSummary(reason.summary).replace(/^Unraid array reports\s+/i, '') || 'Disk issue';
      case 'unraid_sync_active':
        return unraidShortSyncLabel(
          resource.storage?.syncAction,
          resource.storage?.syncProgress,
        );
    }
  }
  return '';
};

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
  if (isUnraidStorageResource(resource)) {
    const unraidLabel = getUnraidShortIssueLabel(resource);
    if (unraidLabel) return unraidLabel;
  }
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
  if (isUnraidStorageResource(resource)) {
    const unraidLabel = getUnraidShortProtectionLabel(resource);
    if (unraidLabel) return unraidLabel;
  }
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

export const getResourceStorageProtectionSummary = (resource: Resource): string => {
  if (resource.storage?.rebuildInProgress) {
    return trimSummary(resource.storage.rebuildSummary);
  }
  if (resource.storage?.protectionReduced) {
    return trimSummary(resource.storage.protectionSummary);
  }
  if (resource.incidentCategory === 'recoverability') {
    return trimSummary(resource.incidentSummary);
  }
  return '';
};
