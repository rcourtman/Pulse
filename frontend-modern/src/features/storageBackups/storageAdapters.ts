import type { Resource } from '@/types/resource';
import type {
  StorageRecord,
  StorageAdapter,
  StorageAdapterContext,
} from './models';
import {
  getCanonicalStoragePlatformKey,
  getResourceStorageActionSummary,
  getResourceStorageImpactSummary,
  getResourceStorageIssueLabel,
  getResourceStorageIssueSummary,
  getResourceStoragePlatformLabel,
  getResourceStorageProtectionLabel,
  getResourceStorageTopologyLabel,
} from './resourceStoragePresentation';
import {
  getStorageCapabilitiesForResource,
  getStorageCategoryFromType,
  readResourceStorageMeta,
  resolveResourceStorageContent,
} from './resourceStorageMapping';
import {
  asNumberOrNull,
  buildStorageCapacity,
  buildStorageSource,
  canonicalStorageIdentityKey,
  dedupe,
  getStringArray,
  metricsTargetForStorageResource,
  normalizeStorageResourceHealth,
} from './storageAdapterCore';

const mapResourceStorageRecord = (resource: Resource, adapterId: string): StorageRecord => {
  const platformData = (resource.platformData as Record<string, unknown> | undefined) || {};
  const storageMeta = readResourceStorageMeta(resource, platformData);
  const resourceType = (resource.type || '').toLowerCase();
  const isDatastore = resourceType === 'datastore';
  const storageType =
    storageMeta?.type ||
    (platformData.type as string | undefined) ||
    (isDatastore ? 'pbs' : resourceType);
  const content = resolveResourceStorageContent(
    storageMeta,
    platformData,
    isDatastore ? 'backup' : '',
  );
  const shared =
    typeof storageMeta?.shared === 'boolean'
      ? storageMeta.shared
      : typeof platformData.shared === 'boolean'
        ? platformData.shared
        : undefined;
  const proxmoxNode =
    resource.proxmox?.node ||
    ((platformData.proxmox as Record<string, unknown> | undefined)?.nodeName as string | undefined);
  const storageNodes = getStringArray(
    (resource.storage as Record<string, unknown> | undefined)?.nodes,
  );
  const nodeHints = dedupe(
    [
      resource.parentName,
      proxmoxNode,
      platformData.node as string | undefined,
      ...storageNodes,
      resource.parentId,
      resource.platformId,
    ].filter((value): value is string => typeof value === 'string' && value.trim().length > 0),
  );
  const locationLabel = isDatastore
    ? resource.parentName ||
      (platformData.pbsInstanceName as string | undefined) ||
      resource.parentId ||
      resource.platformId ||
      'Unknown'
    : resource.parentName ||
      proxmoxNode ||
      (platformData.node as string | undefined) ||
      storageNodes[0] ||
      resource.parentId ||
      resource.platformId ||
      'Unknown';
  const usagePercent = asNumberOrNull(resource.disk?.current);
  const totalBytes = asNumberOrNull(resource.disk?.total);
  const usedBytes = asNumberOrNull(resource.disk?.used);
  const freeBytes = asNumberOrNull(resource.disk?.free);
  const canonicalPlatform = getCanonicalStoragePlatformKey(resource, resource.storage?.platform);
  const hostLabel = locationLabel;
  const platformLabel = getResourceStoragePlatformLabel(canonicalPlatform);
  const issueLabel = getResourceStorageIssueLabel(resource);
  const issueSummary = getResourceStorageIssueSummary(resource);
  const impactSummary = getResourceStorageImpactSummary(resource);
  const actionSummary = getResourceStorageActionSummary(resource);
  const protectionLabel = getResourceStorageProtectionLabel(resource);
  const topologyLabel = getResourceStorageTopologyLabel(
    resource,
    storageType,
    resource.storage?.topology,
  );
  const metricsTarget = metricsTargetForStorageResource(resource);

  return {
    id: resource.id,
    name: resource.name,
    category: isDatastore
      ? 'backup-repository'
      : storageMeta?.isCeph || storageMeta?.isZfs
        ? 'pool'
        : getStorageCategoryFromType(storageType),
    health: normalizeStorageResourceHealth(resource.status, resource.tags, resource.incidentSeverity),
    statusLabel: resource.status,
    hostLabel,
    platformLabel,
    platformKey: canonicalPlatform,
    topologyLabel,
    protectionLabel,
    protectionReduced: resource.storage?.protectionReduced,
    rebuildInProgress: resource.storage?.rebuildInProgress,
    incidentCategory: resource.incidentCategory,
    incidentSeverity: resource.incidentSeverity,
    incidentPriority: resource.incidentPriority || 0,
    issueLabel,
    issueSummary,
    actionSummary,
    impactSummary,
    consumerCount: resource.storage?.consumerCount || 0,
    protectedWorkloadCount: resource.pbs?.protectedWorkloadCount || 0,
    affectedDatastoreCount: resource.pbs?.affectedDatastoreCount || 0,
    location: {
      label: locationLabel,
      scope: isDatastore ? 'cluster' : 'node',
    },
    capacity: buildStorageCapacity(totalBytes, usedBytes, freeBytes, usagePercent),
    capabilities: isDatastore
      ? dedupe(['capacity', 'health', 'backup-repository', 'deduplication', 'namespaces'])
      : getStorageCapabilitiesForResource(storageType, storageMeta),
    source: buildStorageSource(canonicalPlatform, adapterId),
    observedAt:
      typeof resource.lastSeen === 'number' && Number.isFinite(resource.lastSeen)
        ? resource.lastSeen
        : Date.now(),
    metricsTarget,
    refs: {
      resourceId: metricsTarget?.resourceId || resource.id,
      platformEntityId: resource.platformId,
    },
    details: {
      type: storageType,
      status: resource.status,
      parentId: resource.parentId,
      parentName: resource.parentName,
      node: proxmoxNode || (platformData.node as string | undefined) || storageNodes[0],
      nodeHints,
      hostLabel,
      platformLabel,
      topologyLabel,
      issueLabel,
      issueSummary,
      impactSummary,
      actionSummary,
      incidentCategory: resource.incidentCategory,
      incidentSeverity: resource.incidentSeverity,
      incidentPriority: resource.incidentPriority,
      protectionLabel,
      protectionReduced: resource.storage?.protectionReduced,
      rebuildInProgress: resource.storage?.rebuildInProgress,
      content,
      contentTypes: storageMeta?.contentTypes,
      shared,
      isCeph: storageMeta?.isCeph,
      isZfs: storageMeta?.isZfs,
      zfsPool: platformData.zfsPool,
    },
  };
};

const resourceStorageAdapter: StorageAdapter = {
  id: 'resource-storage',
  supports: (ctx) => Array.isArray(ctx.resources) && ctx.resources.length > 0,
  build: (ctx) =>
    (ctx.resources || [])
      .filter((resource) => resource.type === 'storage' || resource.type === 'datastore')
      .map((resource) => mapResourceStorageRecord(resource, 'resource-storage')),
};

export const DEFAULT_STORAGE_ADAPTERS: StorageAdapter[] = [resourceStorageAdapter];

const mergeStorageRecords = (current: StorageRecord, incoming: StorageRecord): StorageRecord => {
  const preferred =
    (current.incidentPriority || 0) >= (incoming.incidentPriority || 0) ? current : incoming;
  const secondary = preferred === current ? incoming : current;

  return {
    ...secondary,
    ...preferred,
    capabilities: dedupe([...(current.capabilities || []), ...(incoming.capabilities || [])]),
    health:
      preferred.health === 'unknown' && secondary.health !== 'unknown'
        ? secondary.health
        : preferred.health,
    statusLabel: preferred.statusLabel || secondary.statusLabel,
    hostLabel: preferred.hostLabel || secondary.hostLabel,
    platformLabel: preferred.platformLabel || secondary.platformLabel,
    platformKey: preferred.platformKey || secondary.platformKey,
    topologyLabel: preferred.topologyLabel || secondary.topologyLabel,
    protectionLabel:
      preferred.protectionReduced || preferred.rebuildInProgress || preferred.incidentPriority
        ? preferred.protectionLabel || secondary.protectionLabel
        : secondary.protectionLabel || preferred.protectionLabel,
    protectionReduced: preferred.protectionReduced || secondary.protectionReduced,
    rebuildInProgress: preferred.rebuildInProgress || secondary.rebuildInProgress,
    issueLabel:
      (preferred.incidentPriority || 0) > 0
        ? preferred.issueLabel || secondary.issueLabel
        : secondary.issueLabel || preferred.issueLabel,
    issueSummary:
      (preferred.incidentPriority || 0) > 0
        ? preferred.issueSummary || secondary.issueSummary
        : secondary.issueSummary || preferred.issueSummary,
    actionSummary:
      (preferred.incidentPriority || 0) > 0
        ? preferred.actionSummary || secondary.actionSummary
        : secondary.actionSummary || preferred.actionSummary,
    impactSummary:
      secondary.impactSummary &&
      secondary.impactSummary !== 'No dependent resources' &&
      (preferred.impactSummary === 'No dependent resources' || !preferred.impactSummary)
        ? secondary.impactSummary
        : preferred.impactSummary || secondary.impactSummary,
    consumerCount: Math.max(current.consumerCount || 0, incoming.consumerCount || 0),
    protectedWorkloadCount: Math.max(
      current.protectedWorkloadCount || 0,
      incoming.protectedWorkloadCount || 0,
    ),
    affectedDatastoreCount: Math.max(
      current.affectedDatastoreCount || 0,
      incoming.affectedDatastoreCount || 0,
    ),
    metricsTarget: preferred.metricsTarget || secondary.metricsTarget,
    refs: {
      resourceId:
        preferred.metricsTarget?.resourceId ||
        secondary.metricsTarget?.resourceId ||
        preferred.refs?.resourceId ||
        secondary.refs?.resourceId,
      platformEntityId: preferred.refs?.platformEntityId || secondary.refs?.platformEntityId,
    },
    details: {
      ...(secondary.details || {}),
      ...(preferred.details || {}),
    },
  };
};

export const buildStorageRecords = (
  ctx: StorageAdapterContext,
  adapters: StorageAdapter[] = DEFAULT_STORAGE_ADAPTERS,
): StorageRecord[] => {
  const map = new Map<string, StorageRecord>();

  for (const adapter of adapters) {
    if (!adapter.supports(ctx)) continue;
    const records = adapter.build(ctx);
    for (const record of records) {
      const key = canonicalStorageIdentityKey(record);
      const existing = map.get(key);
      if (!existing) {
        map.set(key, record);
        continue;
      }
      map.set(key, mergeStorageRecords(existing, record));
    }
  }

  return Array.from(map.values());
};
