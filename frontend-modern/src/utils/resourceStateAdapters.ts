import type {
  Disk,
  Memory,
  Node,
  PBSBackupJob,
  PBSGarbageJob,
  PBSInstance,
  PBSNamespace,
  PBSPruneJob,
  PBSSyncJob,
  PBSDatastore,
  PBSVerifyJob,
  PMGDomainStat,
  PMGInstance,
  PMGMailCountPoint,
  PMGMailStats,
  PMGNodeStatus,
  PMGQuarantineTotals,
  PMGRelayDomain,
  PMGSpamBucket,
  Temperature,
} from '@/types/api';
import type { Resource } from '@/types/resource';
import {
  getActionableAgentIdFromResource,
  getExplicitResourceClusterName,
} from '@/utils/agentResources';
import {
  getPreferredInfrastructureDisplayName,
  getPreferredResourceClusterName,
  getPreferredResourceHostname,
  resolveGuestUrlWithIdentity,
} from '@/utils/resourceIdentity';
import {
  normalizeSourcePlatformScopes,
  normalizeSourcePlatformKey,
  resolvePlatformTypeFromSources,
  resolveSourceTypeFromSources,
} from '@/utils/sourcePlatforms';

type JsonRecord = Record<string, unknown>;

const asRecord = (value: unknown): JsonRecord | undefined =>
  value && typeof value === 'object' ? (value as JsonRecord) : undefined;

const asString = (value: unknown): string | undefined =>
  typeof value === 'string' && value.trim().length > 0 ? value.trim() : undefined;

const asNumber = (value: unknown): number | undefined =>
  typeof value === 'number' && Number.isFinite(value) ? value : undefined;

const asBoolean = (value: unknown): boolean | undefined =>
  typeof value === 'boolean' ? value : undefined;

const asArray = (value: unknown): unknown[] => (Array.isArray(value) ? value : []);

const toISOTime = (value: unknown, fallbackMs?: number): string => {
  const asStr = asString(value);
  if (asStr) return asStr;
  if (typeof fallbackMs === 'number' && Number.isFinite(fallbackMs)) {
    return new Date(fallbackMs).toISOString();
  }
  return new Date(0).toISOString();
};

const getCanonicalPlatformId = (resource: Resource): string | undefined => {
  const platformId = resource.canonicalIdentity?.platformId;
  return typeof platformId === 'string' && platformId.trim().length > 0
    ? platformId.trim()
    : undefined;
};

const normalizeResourceIdentityToken = (value: string | undefined): string | undefined => {
  if (!value) return undefined;
  const normalized = value.trim().toLowerCase();
  return normalized.length > 0 ? normalized : undefined;
};

export const resourcePlatformData = (resource: Resource): Record<string, unknown> | undefined =>
  asRecord(resource.platformData);

const mergeStringArrays = (incoming?: string[], existing?: string[]): string[] | undefined => {
  const merged = [...(incoming ?? []), ...(existing ?? [])]
    .map((value) => asString(value))
    .filter((value): value is string => Boolean(value));
  return merged.length > 0 ? Array.from(new Set(merged)) : undefined;
};

const readStringArray = (value: unknown): string[] | undefined => {
  if (!Array.isArray(value)) return undefined;
  const normalized = value
    .map((entry) => asString(entry))
    .filter((entry): entry is string => Boolean(entry));
  return normalized.length > 0 ? Array.from(new Set(normalized)) : undefined;
};

const readExplicitPlatformScopes = (resource: Resource): string[] | undefined =>
  readStringArray(getResourceRecord(resource).platformScopes) ??
  readStringArray(asRecord(resource.platformData)?.platformScopes);

const normalizeSourceToken = (value: string): string =>
  normalizeSourcePlatformKey(value) || value.trim().toLowerCase();

const sourceListHas = (sources: string[] | undefined, ...candidates: string[]): boolean => {
  if (!sources || sources.length === 0) return false;
  const sourceSet = new Set(sources.map((source) => normalizeSourceToken(source)));
  return candidates.some((candidate) => sourceSet.has(normalizeSourceToken(candidate)));
};

const shouldKeepSourceFacet = (
  authoritativeSources: string[] | undefined,
  ...sourceCandidates: string[]
): boolean => !authoritativeSources || sourceListHas(authoritativeSources, ...sourceCandidates);

const mergeRecord = <T extends JsonRecord>(incoming?: T, existing?: T): T | undefined => {
  if (!incoming) return existing;
  if (!existing) return incoming;
  return { ...existing, ...incoming };
};

const mergePlatformData = (
  incomingValue: Resource['platformData'],
  existingValue: Resource['platformData'],
): Resource['platformData'] => {
  const incoming = asRecord(incomingValue);
  const existing = asRecord(existingValue);
  if (!incoming) return existingValue;
  if (!existing) return incomingValue;

  const incomingSources = readStringArray(incoming.sources);
  const existingSources = readStringArray(existing.sources);
  const sources = incomingSources ?? existingSources;
  const merged: JsonRecord = { ...existing, ...incoming };

  for (const [key, ...sourceCandidates] of [
    ['agent', 'agent'],
    ['docker', 'docker'],
    ['proxmox', 'proxmox-pve'],
    ['pbs', 'proxmox-pbs'],
    ['pmg', 'proxmox-pmg'],
    ['kubernetes', 'kubernetes'],
    ['vmware', 'vmware-vsphere'],
    ['truenas', 'truenas'],
    ['availability', 'availability'],
  ] as const) {
    if (!shouldKeepSourceFacet(incomingSources, ...sourceCandidates)) {
      delete merged[key];
      continue;
    }
    const nested = mergeRecord(asRecord(incoming[key]), asRecord(existing[key]));
    if (nested) {
      merged[key] = nested;
    }
  }

  for (const key of [
    'storage',
    'physicalDisk',
    'ceph',
    'metrics',
    'discoveryTarget',
    'discoveryReadiness',
  ]) {
    const nested = mergeRecord(asRecord(incoming[key]), asRecord(existing[key]));
    if (nested) {
      merged[key] = nested;
    }
  }

  const sourceStatus = mergeRecord(
    asRecord(incoming.sourceStatus),
    asRecord(existing.sourceStatus),
  );
  if (sourceStatus) {
    merged.sourceStatus = sourceStatus;
  }

  if (sources) {
    merged.sources = sources;
  }

  return merged;
};

const getResourceRecord = (resource: Resource): JsonRecord => resource as unknown as JsonRecord;

const getExplicitResourceSources = (resource: Resource): string[] | undefined =>
  readStringArray(getResourceRecord(resource).sources);

const getFacetRecord = (
  resource: Resource,
  platformData: Resource['platformData'] | undefined,
  key: string,
): JsonRecord | undefined => {
  const resourceRecord = getResourceRecord(resource);
  const platformRecord = asRecord(platformData);
  return asRecord(resourceRecord[key]) || asRecord(platformRecord?.[key]);
};

const deriveLegacySourceList = (
  resource: Resource,
  platformData: Resource['platformData'] | undefined = resource.platformData,
): string[] | undefined => {
  const resourceSources = getExplicitResourceSources(resource);
  if (resourceSources && resourceSources.length > 0) {
    return resourceSources;
  }

  const sources: string[] = [];
  if (getFacetRecord(resource, platformData, 'proxmox')) sources.push('proxmox');
  if (getFacetRecord(resource, platformData, 'pbs')) sources.push('pbs');
  if (getFacetRecord(resource, platformData, 'pmg')) sources.push('pmg');
  if (getFacetRecord(resource, platformData, 'vmware')) sources.push('vmware');
  if (getFacetRecord(resource, platformData, 'truenas')) sources.push('truenas');
  if (getFacetRecord(resource, platformData, 'kubernetes')) sources.push('kubernetes');
  if (getFacetRecord(resource, platformData, 'docker')) sources.push('docker');
  if (getFacetRecord(resource, platformData, 'availability')) sources.push('availability');
  if (getFacetRecord(resource, platformData, 'agent')) sources.push('agent');
  if (sources.length > 0) {
    return Array.from(new Set(sources));
  }

  if (
    resource.type === 'network-endpoint' ||
    resource.platformType === 'availability' ||
    Boolean(resource.availability) ||
    Boolean(asRecord(resource.platformData)?.availability)
  ) {
    return ['availability'];
  }

  switch (resource.platformType) {
    case 'proxmox-pve':
      return resource.sourceType === 'hybrid' ? ['proxmox', 'agent'] : ['proxmox'];
    case 'docker':
      return ['docker'];
    case 'kubernetes':
      return resource.sourceType === 'hybrid' ? ['agent', 'kubernetes'] : ['kubernetes'];
    case 'proxmox-pbs':
      return ['pbs'];
    case 'proxmox-pmg':
      return ['pmg'];
    case 'truenas':
      return ['truenas'];
    case 'vmware-vsphere':
      return ['vmware'];
    default:
      return resource.sourceType === 'agent' ? ['agent'] : undefined;
  }
};

const hasLegacyProxmoxShape = (
  resource: Resource,
  platformData: JsonRecord,
  sources?: string[],
): boolean =>
  resource.platformType === 'proxmox-pve' ||
  sourceListHas(sources, 'proxmox-pve') ||
  [
    'instance',
    'node',
    'clusterName',
    'vmid',
    'cpus',
    'template',
    'swapUsed',
    'swapTotal',
    'balloon',
  ].some((key) => platformData[key] !== undefined);

const canonicalizeLegacyPlatformData = (resource: Resource): Resource['platformData'] => {
  const platformData = asRecord(resource.platformData);
  if (!platformData) {
    const normalizedSources = deriveLegacySourceList(resource);
    if (!normalizedSources || normalizedSources.length === 0) {
      return resource.platformData;
    }
    const normalized: JsonRecord = { sources: normalizedSources };
    const resourceRecord = getResourceRecord(resource);
    for (const [key, value] of [
      ['agent', resourceRecord.agent],
      ['docker', resourceRecord.docker],
      ['proxmox', resourceRecord.proxmox],
      ['pbs', resourceRecord.pbs],
      ['pmg', resourceRecord.pmg],
      ['kubernetes', resourceRecord.kubernetes],
      ['vmware', resourceRecord.vmware],
      ['truenas', resourceRecord.truenas],
      ['storage', resourceRecord.storage],
      ['availability', resourceRecord.availability],
      ['physicalDisk', resourceRecord.physicalDisk],
    ] as const) {
      if (value !== undefined) {
        normalized[key] = value;
      }
    }
    return normalized;
  }

  const normalized: JsonRecord = { ...platformData };
  const resourceRecord = getResourceRecord(resource);
  for (const key of [
    'agent',
    'docker',
    'proxmox',
    'pbs',
    'pmg',
    'kubernetes',
    'vmware',
    'truenas',
    'storage',
    'availability',
    'physicalDisk',
  ] as const) {
    if (!asRecord(normalized[key]) && asRecord(resourceRecord[key])) {
      normalized[key] = resourceRecord[key];
    }
  }

  const explicitResourceSources = getExplicitResourceSources(resource);
  const normalizedSources =
    explicitResourceSources ??
    (Array.isArray(platformData.sources) && platformData.sources.length > 0
      ? (platformData.sources as string[])
      : deriveLegacySourceList(resource, platformData));
  if (normalizedSources && normalizedSources.length > 0) {
    normalized.sources = normalizedSources;
  }

  if (!asRecord(normalized.agent)) {
    const agentPayload: JsonRecord = {};
    for (const [legacyKey, nextKey] of [
      ['agentId', 'agentId'],
      ['agentVersion', 'agentVersion'],
      ['hostname', 'hostname'],
      ['platform', 'platform'],
      ['osName', 'osName'],
      ['osVersion', 'osVersion'],
      ['kernelVersion', 'kernelVersion'],
      ['architecture', 'architecture'],
      ['commandsEnabled', 'commandsEnabled'],
    ] as const) {
      if (platformData[legacyKey] !== undefined) {
        agentPayload[nextKey] = platformData[legacyKey];
      }
    }
    if (platformData.memory !== undefined) agentPayload.memory = platformData.memory;
    if (platformData.interfaces !== undefined)
      agentPayload.networkInterfaces = platformData.interfaces;
    if (platformData.disks !== undefined) agentPayload.disks = platformData.disks;
    if (Object.keys(agentPayload).length > 0) {
      normalized.agent = agentPayload;
    }
  }

  if (!asRecord(normalized.docker)) {
    const dockerPayload: JsonRecord = {};
    for (const [legacyKey, nextKey] of [
      ['agentId', 'agentId'],
      ['runtime', 'runtime'],
      ['runtimeVersion', 'runtimeVersion'],
      ['dockerVersion', 'dockerVersion'],
      ['os', 'os'],
      ['kernelVersion', 'kernelVersion'],
      ['architecture', 'architecture'],
      ['agentVersion', 'agentVersion'],
      ['hostname', 'hostname'],
      ['displayName', 'displayName'],
      ['machineId', 'machineId'],
      ['containerCount', 'containerCount'],
      ['uptimeSeconds', 'uptimeSeconds'],
      ['intervalSeconds', 'intervalSeconds'],
      ['temperature', 'temperature'],
      ['hostSourceId', 'hostSourceId'],
    ] as const) {
      if (platformData[legacyKey] !== undefined) {
        dockerPayload[nextKey] = platformData[legacyKey];
      }
    }
    if (platformData.swarm !== undefined) dockerPayload.swarm = platformData.swarm;
    if (platformData.interfaces !== undefined)
      dockerPayload.networkInterfaces = platformData.interfaces;
    if (platformData.disks !== undefined) dockerPayload.disks = platformData.disks;
    if (Object.keys(dockerPayload).length > 0) {
      normalized.docker = dockerPayload;
    }
  }

  if (
    !asRecord(normalized.proxmox) &&
    hasLegacyProxmoxShape(resource, platformData, normalizedSources)
  ) {
    const proxmoxPayload: JsonRecord = {};
    for (const [legacyKey, nextKey] of [
      ['instance', 'instance'],
      ['node', 'nodeName'],
      ['clusterName', 'clusterName'],
      ['vmid', 'vmid'],
      ['cpus', 'cpus'],
      ['template', 'template'],
      ['swapUsed', 'swapUsed'],
      ['swapTotal', 'swapTotal'],
      ['balloon', 'balloon'],
    ] as const) {
      if (platformData[legacyKey] !== undefined) {
        proxmoxPayload[nextKey] = platformData[legacyKey];
      }
    }
    if (platformData.disks !== undefined) proxmoxPayload.disks = platformData.disks;
    if (Object.keys(proxmoxPayload).length > 0) {
      normalized.proxmox = proxmoxPayload;
    }
  }

  if (!asRecord(normalized.pbs)) {
    const pbsPayload: JsonRecord = {};
    if (platformData.host !== undefined) pbsPayload.hostname = platformData.host;
    if (platformData.version !== undefined) pbsPayload.version = platformData.version;
    if (platformData.connectionHealth !== undefined) {
      pbsPayload.connectionHealth = platformData.connectionHealth;
    }
    if (platformData.numDatastores !== undefined) {
      pbsPayload.datastoreCount = platformData.numDatastores;
    }
    if (Object.keys(pbsPayload).length > 0) {
      normalized.pbs = pbsPayload;
    }
  }

  if (!asRecord(normalized.pmg)) {
    const pmgPayload: JsonRecord = {};
    if (platformData.host !== undefined) pmgPayload.hostname = platformData.host;
    if (platformData.version !== undefined) pmgPayload.version = platformData.version;
    if (platformData.connectionHealth !== undefined) {
      pmgPayload.connectionHealth = platformData.connectionHealth;
    }
    for (const [legacyKey, nextKey] of [
      ['nodeCount', 'nodeCount'],
      ['queueActive', 'queueActive'],
      ['queueDeferred', 'queueDeferred'],
      ['queueHold', 'queueHold'],
      ['queueIncoming', 'queueIncoming'],
      ['queueTotal', 'queueTotal'],
    ] as const) {
      if (platformData[legacyKey] !== undefined) {
        pmgPayload[nextKey] = platformData[legacyKey];
      }
    }
    if (Object.keys(pmgPayload).length > 0) {
      normalized.pmg = pmgPayload;
    }
  }

  if (!asRecord(normalized.kubernetes)) {
    const kubernetesPayload: JsonRecord = {};
    for (const [legacyKey, nextKey] of [
      ['agentId', 'agentId'],
      ['clusterId', 'clusterId'],
      ['context', 'context'],
      ['nodeName', 'nodeName'],
      ['namespace', 'namespace'],
      ['clusterName', 'clusterName'],
      ['pendingUninstall', 'pendingUninstall'],
    ] as const) {
      if (platformData[legacyKey] !== undefined) {
        kubernetesPayload[nextKey] = platformData[legacyKey];
      }
    }
    if (Object.keys(kubernetesPayload).length > 0) {
      normalized.kubernetes = kubernetesPayload;
    }
  }

  return normalized;
};

const getCanonicalSourceList = (
  resource: Resource,
  platformData?: Resource['platformData'],
): string[] | undefined => {
  const resourceSources = getExplicitResourceSources(resource);
  if (resourceSources && resourceSources.length > 0) return resourceSources;
  const platformRecord = asRecord(platformData);
  return Array.isArray(platformRecord?.sources) && platformRecord.sources.length > 0
    ? (platformRecord.sources as string[])
    : deriveLegacySourceList(resource, platformData);
};

const sourceListContainsRuntimePlatform = (sources: string[] | undefined): boolean =>
  sourceListHas(sources, 'proxmox-pve', 'docker', 'kubernetes', 'vmware-vsphere', 'truenas');

const getHostResourceMergeKey = (resource: Resource): string | undefined => {
  if (resource.type !== 'agent') return undefined;
  const platform = asRecord(resource.platformData);
  const canonical = resource.canonicalIdentity;
  const candidates = [
    canonical?.platformId,
    canonical?.hostname,
    resource.platformId,
    asString(asRecord(resource.agent)?.hostname),
    asString(asRecord(platform?.agent)?.hostname),
    asString(asRecord(resource.proxmox)?.nodeName),
    asString(asRecord(platform?.proxmox)?.nodeName),
    getPreferredResourceHostname(resource),
    getPreferredInfrastructureDisplayName(resource),
    resource.displayName,
    resource.name,
  ];
  const hostKey = candidates.map(normalizeResourceIdentityToken).find(Boolean);
  return hostKey ? `agent:${hostKey}` : undefined;
};

const shouldMergeRealtimeHostResources = (incoming: Resource, existing: Resource): boolean => {
  if (incoming.type !== 'agent' || existing.type !== 'agent') return false;
  const incomingSources = getCanonicalSourceList(incoming, incoming.platformData);
  const existingSources = getCanonicalSourceList(existing, existing.platformData);
  const unionSources = mergeStringArrays(incomingSources, existingSources);
  return sourceListHas(unionSources, 'agent') && sourceListContainsRuntimePlatform(unionSources);
};

const preferHostResourcePrimary = (candidate: Resource, other: Resource): boolean => {
  const candidateSources = getCanonicalSourceList(candidate, candidate.platformData);
  const otherSources = getCanonicalSourceList(other, other.platformData);
  if (sourceListHas(candidateSources, 'agent') && !sourceListHas(otherSources, 'agent')) {
    return true;
  }
  if (!sourceListHas(candidateSources, 'agent') && sourceListHas(otherSources, 'agent')) {
    return false;
  }
  return candidate.lastSeen >= other.lastSeen;
};

const withMergedSnapshotSources = (resource: Resource, sources: string[] | undefined): Resource => {
  if (!sources || sources.length === 0) return resource;
  const platform = asRecord(resource.platformData);
  return canonicalizeRealtimeResource({
    ...resource,
    sources,
    platformData: {
      ...(platform ?? {}),
      sources,
    },
  });
};

const mergeRealtimeHostResources = (incoming: Resource, existing: Resource): Resource => {
  const unionSources = mergeStringArrays(
    getCanonicalSourceList(incoming, incoming.platformData),
    getCanonicalSourceList(existing, existing.platformData),
  );
  const primary = preferHostResourcePrimary(incoming, existing) ? incoming : existing;
  const secondary = primary === incoming ? existing : incoming;
  return canonicalizeRealtimeResource(
    mergeCanonicalResource(
      withMergedSnapshotSources(primary, unionSources),
      withMergedSnapshotSources(secondary, unionSources),
    ),
  );
};

const coalesceRealtimeResourceSnapshot = (resources: Resource[]): Resource[] => {
  const coalesced: Resource[] = [];
  const indexByHostKey = new Map<string, number>();

  for (const rawResource of resources) {
    const resource = canonicalizeRealtimeResource(rawResource, { synthesizePlatformScopes: false });
    const hostKey = getHostResourceMergeKey(resource);
    if (!hostKey) {
      coalesced.push(resource);
      continue;
    }

    const existingIndex = indexByHostKey.get(hostKey);
    if (existingIndex === undefined) {
      indexByHostKey.set(hostKey, coalesced.length);
      coalesced.push(resource);
      continue;
    }

    const existing = coalesced[existingIndex];
    if (!shouldMergeRealtimeHostResources(resource, existing)) {
      coalesced.push(resource);
      continue;
    }

    coalesced[existingIndex] = mergeRealtimeHostResources(resource, existing);
  }

  return coalesced;
};

const hasAvailabilityFacet = (
  resource: Resource,
  platformData?: Resource['platformData'],
): boolean => {
  const platformRecord = asRecord(platformData);
  const sources = getCanonicalSourceList(resource, platformData);
  return (
    resource.type === 'network-endpoint' ||
    resource.platformType === 'availability' ||
    Boolean(resource.availability) ||
    Boolean(platformRecord?.availability) ||
    Boolean(sources?.some((source) => source.trim().toLowerCase() === 'availability'))
  );
};

export const canonicalizeRealtimeResource = (
  resource: Resource,
  options: { synthesizePlatformScopes?: boolean } = {},
): Resource => {
  const platformData = canonicalizeLegacyPlatformData(resource);
  const platformRecord = asRecord(platformData);
  const sources = getCanonicalSourceList(resource, platformData);
  const platformType =
    resolvePlatformTypeFromSources(sources) ||
    (hasAvailabilityFacet(resource, platformData) ? 'availability' : resource.platformType);
  const explicitPlatformScopes = readExplicitPlatformScopes(resource);
  const platformScopes =
    explicitPlatformScopes !== undefined
      ? normalizeSourcePlatformScopes(explicitPlatformScopes, platformType)
      : options.synthesizePlatformScopes === false
        ? undefined
        : normalizeSourcePlatformScopes(undefined, platformType);
  const sourceType =
    sources && sources.length > 0 ? resolveSourceTypeFromSources(sources) : resource.sourceType;
  const normalizedBase = {
    ...resource,
    platformType,
    platformScopes,
    sourceType,
    platformData,
  };
  return {
    ...normalizedBase,
    clusterId: resource.clusterId ?? getExplicitResourceClusterName(normalizedBase),
    platformData,
    agent: resource.agent ?? (platformRecord?.agent as Resource['agent']),
    proxmox: resource.proxmox ?? (platformRecord?.proxmox as Resource['proxmox']),
    pbs: resource.pbs ?? (platformRecord?.pbs as Resource['pbs']),
    kubernetes: resource.kubernetes ?? (platformRecord?.kubernetes as Resource['kubernetes']),
    vmware: resource.vmware ?? (platformRecord?.vmware as Resource['vmware']),
    truenas: resource.truenas ?? (platformRecord?.truenas as Resource['truenas']),
    storage: resource.storage ?? (platformRecord?.storage as Resource['storage']),
    availability:
      resource.availability ?? (platformRecord?.availability as Resource['availability']),
    physicalDisk:
      resource.physicalDisk ?? (platformRecord?.physicalDisk as Resource['physicalDisk']),
  };
};

const mergeCanonicalIdentity = (
  incoming?: Resource['canonicalIdentity'],
  existing?: Resource['canonicalIdentity'],
): Resource['canonicalIdentity'] => {
  if (!incoming) return existing;
  if (!existing) return incoming;
  const aliases = mergeStringArrays(incoming.aliases, existing.aliases);
  return {
    ...existing,
    ...incoming,
    aliases,
  };
};

const mergeCanonicalSourceFacet = <T extends JsonRecord>(
  incomingFacet: T | undefined,
  existingFacet: T | undefined,
  incomingSources: string[] | undefined,
  ...sourceCandidates: string[]
): T | undefined =>
  shouldKeepSourceFacet(incomingSources, ...sourceCandidates)
    ? mergeRecord(incomingFacet, existingFacet)
    : incomingFacet;

export const mergeCanonicalResource = (incoming: Resource, existing?: Resource): Resource => {
  if (!existing) {
    return canonicalizeRealtimeResource(incoming);
  }
  const existingCanonical = canonicalizeRealtimeResource(existing);
  const incomingSources = getCanonicalSourceList(incoming, incoming.platformData);
  return {
    ...existingCanonical,
    ...incoming,
    clusterId: incoming.clusterId ?? existingCanonical.clusterId,
    platformScopes: normalizeSourcePlatformScopes(
      incoming.platformScopes ?? existingCanonical.platformScopes,
      incoming.platformType ?? existingCanonical.platformType,
    ),
    discoveryTarget: incoming.discoveryTarget ?? existingCanonical.discoveryTarget,
    discoveryReadiness: incoming.discoveryReadiness ?? existingCanonical.discoveryReadiness,
    metricsTarget: incoming.metricsTarget ?? existingCanonical.metricsTarget,
    canonicalIdentity: mergeCanonicalIdentity(
      incoming.canonicalIdentity,
      existingCanonical.canonicalIdentity,
    ),
    policy: incoming.policy ?? existingCanonical.policy,
    aiSafeSummary: incoming.aiSafeSummary ?? existingCanonical.aiSafeSummary,
    recentChanges: incoming.recentChanges ?? existingCanonical.recentChanges,
    facetCounts: incoming.facetCounts ?? existingCanonical.facetCounts,
    diskIO: incoming.diskIO ?? existingCanonical.diskIO,
    agent: mergeCanonicalSourceFacet(
      incoming.agent as JsonRecord | undefined,
      existingCanonical.agent as JsonRecord | undefined,
      incomingSources,
      'agent',
    ) as Resource['agent'],
    proxmox: mergeCanonicalSourceFacet(
      incoming.proxmox as JsonRecord | undefined,
      existingCanonical.proxmox as JsonRecord | undefined,
      incomingSources,
      'proxmox-pve',
    ) as Resource['proxmox'],
    pbs: mergeCanonicalSourceFacet(
      incoming.pbs as JsonRecord | undefined,
      existingCanonical.pbs as JsonRecord | undefined,
      incomingSources,
      'proxmox-pbs',
    ) as Resource['pbs'],
    kubernetes: mergeCanonicalSourceFacet(
      incoming.kubernetes as JsonRecord | undefined,
      existingCanonical.kubernetes as JsonRecord | undefined,
      incomingSources,
      'kubernetes',
    ) as Resource['kubernetes'],
    vmware: mergeCanonicalSourceFacet(
      incoming.vmware as JsonRecord | undefined,
      existingCanonical.vmware as JsonRecord | undefined,
      incomingSources,
      'vmware-vsphere',
    ) as Resource['vmware'],
    truenas: mergeCanonicalSourceFacet(
      incoming.truenas as JsonRecord | undefined,
      existingCanonical.truenas as JsonRecord | undefined,
      incomingSources,
      'truenas',
    ) as Resource['truenas'],
    availability: mergeCanonicalSourceFacet(
      incoming.availability as JsonRecord | undefined,
      existingCanonical.availability as JsonRecord | undefined,
      incomingSources,
      'availability',
    ) as Resource['availability'],
    storage: mergeRecord(
      incoming.storage as JsonRecord | undefined,
      existingCanonical.storage as JsonRecord | undefined,
    ) as Resource['storage'],
    physicalDisk: mergeRecord(
      incoming.physicalDisk as JsonRecord | undefined,
      existingCanonical.physicalDisk as JsonRecord | undefined,
    ) as Resource['physicalDisk'],
    identity: mergeRecord(
      incoming.identity as JsonRecord | undefined,
      existingCanonical.identity as JsonRecord | undefined,
    ) as Resource['identity'],
    platformData: mergePlatformData(incoming.platformData, existingCanonical.platformData),
    tags: incoming.tags && incoming.tags.length > 0 ? incoming.tags : existingCanonical.tags,
    labels:
      incoming.labels && Object.keys(incoming.labels).length > 0
        ? incoming.labels
        : existingCanonical.labels,
  };
};

export const mergeCanonicalResourceSnapshot = (
  incoming: Resource[],
  existing: Resource[],
): Resource[] => {
  if (incoming.length === 0) {
    return [];
  }
  const coalescedIncoming = coalesceRealtimeResourceSnapshot(incoming);
  const existingById = new Map(existing.map((resource) => [resource.id, resource] as const));
  return coalescedIncoming.map((resource) =>
    mergeCanonicalResource(resource, existingById.get(resource.id)),
  );
};

const buildMemory = (
  metric: Resource['memory'],
  fallback?: Record<string, unknown>,
  proxmoxMeta?: Record<string, unknown>,
): Memory => {
  const total = metric?.total ?? asNumber(fallback?.total) ?? 0;
  const used = metric?.used ?? asNumber(fallback?.used) ?? 0;
  const cache = asNumber(proxmoxMeta?.memoryCache) ?? asNumber(fallback?.cache) ?? 0;
  // The metric ships no free bytes for PVE payloads; total-used is the
  // reclaimable-inclusive available, so carve the cache back out when known.
  const free =
    metric?.free ?? asNumber(fallback?.free) ?? Math.max(total - used - cache, 0);
  const usage =
    metric?.current ?? (total > 0 ? (used / total) * 100 : (asNumber(fallback?.usage) ?? 0));
  return {
    total,
    used,
    free,
    usage,
    cache: cache > 0 ? cache : undefined,
    swapUsed: asNumber(proxmoxMeta?.swapUsed) ?? asNumber(fallback?.swapUsed),
    swapTotal: asNumber(proxmoxMeta?.swapTotal) ?? asNumber(fallback?.swapTotal),
    balloon: asNumber(fallback?.balloon),
  };
};

const buildDisk = (metric: Resource['disk'], fallback?: Record<string, unknown>): Disk => {
  const total = metric?.total ?? asNumber(fallback?.total) ?? 0;
  const used = metric?.used ?? asNumber(fallback?.used) ?? 0;
  const free = metric?.free ?? asNumber(fallback?.free) ?? Math.max(total - used, 0);
  const usage =
    metric?.current ?? (total > 0 ? (used / total) * 100 : (asNumber(fallback?.usage) ?? 0));
  return {
    total,
    used,
    free,
    usage,
    mountpoint: asString(fallback?.mountpoint),
    type: asString(fallback?.type),
    device: asString(fallback?.device),
  };
};

const buildTemperature = (
  resource: Resource,
  nodeMeta?: Record<string, unknown>,
): Temperature | undefined => {
  const platform = resourcePlatformData(resource);
  const raw =
    asRecord(platform?.temperature) ||
    asRecord(nodeMeta?.temperature) ||
    asRecord(platform?.agent) ||
    undefined;

  if (raw) {
    const available = asBoolean(raw.available);
    const cpuPackage = asNumber(raw.cpuPackage) ?? asNumber(raw.temperature) ?? asNumber(raw.cpu);
    const lastUpdate = toISOTime(raw.lastUpdate, resource.lastSeen);
    if (available || typeof cpuPackage === 'number') {
      return {
        cpuPackage,
        cpuMax: asNumber(raw.cpuMax),
        cpuMin: asNumber(raw.cpuMin),
        cpuMaxRecord: asNumber(raw.cpuMaxRecord),
        minRecorded: asString(raw.minRecorded),
        maxRecorded: asString(raw.maxRecorded),
        cores: asArray(raw.cores)
          .map((entry) => {
            const rec = asRecord(entry);
            if (!rec) return null;
            const core = asNumber(rec.core);
            const temp = asNumber(rec.temp);
            if (typeof core !== 'number' || typeof temp !== 'number') return null;
            return { core, temp };
          })
          .filter((entry): entry is NonNullable<typeof entry> => Boolean(entry)),
        gpu: asArray(raw.gpu)
          .map((entry) => {
            const rec = asRecord(entry);
            if (!rec) return null;
            const device = asString(rec.device);
            if (!device) return null;
            return {
              device,
              edge: asNumber(rec.edge),
              junction: asNumber(rec.junction),
              mem: asNumber(rec.mem),
            };
          })
          .filter((entry): entry is NonNullable<typeof entry> => Boolean(entry)),
        nvme: asArray(raw.nvme)
          .map((entry) => {
            const rec = asRecord(entry);
            if (!rec) return null;
            const device = asString(rec.device);
            const temp = asNumber(rec.temp);
            if (!device || typeof temp !== 'number') return null;
            return { device, temp };
          })
          .filter((entry): entry is NonNullable<typeof entry> => Boolean(entry)),
        available: available ?? true,
        hasCPU: asBoolean(raw.hasCPU) ?? (typeof cpuPackage === 'number' ? true : undefined),
        hasGPU: asBoolean(raw.hasGPU),
        hasNVMe: asBoolean(raw.hasNVMe),
        lastUpdate,
      };
    }
  }

  if (typeof resource.temperature === 'number' && Number.isFinite(resource.temperature)) {
    const temp = resource.temperature;
    return {
      cpuPackage: temp,
      cpuMax: temp,
      cpuMin: temp,
      cpuMaxRecord: temp,
      available: true,
      hasCPU: true,
      lastUpdate: toISOTime(undefined, resource.lastSeen),
    };
  }

  return undefined;
};

export const nodeFromResource = (resource: Resource): Node | null => {
  if (resource.type !== 'agent') return null;
  const platform = resourcePlatformData(resource);
  const proxmox =
    asRecord(platform?.proxmox) ||
    (resource.proxmox as unknown as Record<string, unknown> | undefined);
  const cpuInfo = asRecord(proxmox?.cpuInfo);
  const preferredHostLabel =
    getPreferredResourceHostname(resource) ||
    getPreferredInfrastructureDisplayName(resource) ||
    resource.id;
  const instance =
    asString(proxmox?.instance) ||
    resource.platformId ||
    getCanonicalPlatformId(resource) ||
    preferredHostLabel;
  const name = asString(proxmox?.nodeName) || asString(proxmox?.node) || preferredHostLabel;
  const linkedAgentId =
    asString(platform?.linkedAgentId) || getActionableAgentIdFromResource(resource);
  const agentFacet = resource.agent;
  const pveVersion =
    asString(proxmox?.pveVersion) ||
    ((agentFacet?.osName || '').toLowerCase().includes('proxmox')
      ? asString(agentFacet?.osVersion)
      : '') ||
    'Unknown';

  return {
    id: resource.id,
    name,
    displayName: getPreferredInfrastructureDisplayName(resource),
    instance,
    host: name || preferredHostLabel,
    // proxmox.guestUrl is the operator-set link override and proxmox.host the
    // PVE API connection URL; `host` above stays a hostname label for display.
    guestURL: resolveGuestUrlWithIdentity(
      asString(proxmox?.guestUrl) ||
        asString((resource as unknown as Record<string, unknown>).customURL) ||
        asString((resource as unknown as Record<string, unknown>).customUrl) ||
        asString(proxmox?.host) ||
        '',
      resource,
    ),
    status: resource.status || 'unknown',
    type: resource.type,
    cpu: resource.cpu?.current ?? 0,
    memory: buildMemory(resource.memory, asRecord(proxmox?.memory), proxmox),
    disk: buildDisk(resource.disk, asRecord(proxmox?.disk)),
    networkIn: resource.network?.rxBytes,
    networkOut: resource.network?.txBytes,
    diskRead: resource.diskIO?.readRate,
    diskWrite: resource.diskIO?.writeRate,
    uptime: resource.uptime ?? asNumber(proxmox?.uptime) ?? 0,
    loadAverage: asArray(proxmox?.loadAverage)
      .map((value) => asNumber(value))
      .filter((value): value is number => typeof value === 'number'),
    kernelVersion: asString(proxmox?.kernelVersion) || 'Unknown',
    pveVersion,
    cpuInfo: {
      model: asString(cpuInfo?.model) || 'Unknown',
      cores: asNumber(cpuInfo?.cores) ?? 0,
      sockets: asNumber(cpuInfo?.sockets) ?? 0,
      mhz: asString(cpuInfo?.mhz) || '0',
    },
    temperature: buildTemperature(resource, proxmox),
    temperatureMonitoringEnabled:
      asBoolean(platform?.temperatureMonitoringEnabled) ??
      asBoolean(proxmox?.temperatureMonitoringEnabled) ??
      null,
    pendingUpdates: asNumber(proxmox?.pendingUpdates),
    pendingUpdatesCheckedAt: asString(proxmox?.pendingUpdatesCheckedAt),
    lastSeen: toISOTime(undefined, resource.lastSeen),
    connectionHealth: asString(proxmox?.connectionHealth) || resource.status || 'unknown',
    isClusterMember: asBoolean(proxmox?.isClusterMember),
    clusterName: getPreferredResourceClusterName(resource),
    linkedAgentId,
  };
};

const mapPBSNamespace = (value: unknown): PBSNamespace | null => {
  const rec = asRecord(value);
  if (!rec) return null;
  return {
    path: asString(rec.path) || '',
    parent: asString(rec.parent) || '',
    depth: asNumber(rec.depth) ?? 0,
  };
};

const mapPBSDatastore = (value: unknown): PBSDatastore | null => {
  const rec = asRecord(value);
  if (!rec) return null;
  const total = asNumber(rec.total) ?? 0;
  const used = asNumber(rec.used) ?? 0;
  const free = asNumber(rec.free) ?? asNumber(rec.available) ?? Math.max(total - used, 0);
  const usage =
    asNumber(rec.usage) ?? asNumber(rec.usagePercent) ?? (total > 0 ? (used / total) * 100 : 0);
  return {
    name: asString(rec.name) || '',
    total,
    used,
    free,
    usage,
    status: asString(rec.status) || '',
    error: asString(rec.error) || '',
    namespaces: asArray(rec.namespaces)
      .map(mapPBSNamespace)
      .filter((entry): entry is PBSNamespace => Boolean(entry)),
    deduplicationFactor: asNumber(rec.deduplicationFactor),
  };
};

const mapPBSBackupJob = (value: unknown): PBSBackupJob | null => {
  const rec = asRecord(value);
  if (!rec) return null;
  return {
    id: asString(rec.id) || '',
    store: asString(rec.store) || '',
    type: asString(rec.type) || '',
    vmid: asString(rec.vmid) || '',
    lastBackup: asString(rec.lastBackup) || '',
    nextRun: asString(rec.nextRun) || '',
    status: asString(rec.status) || '',
    error: asString(rec.error) || '',
  };
};

const mapPBSSyncJob = (value: unknown): PBSSyncJob | null => {
  const rec = asRecord(value);
  if (!rec) return null;
  return {
    id: asString(rec.id) || '',
    store: asString(rec.store) || '',
    remote: asString(rec.remote) || '',
    status: asString(rec.status) || '',
    lastSync: asString(rec.lastSync) || '',
    nextRun: asString(rec.nextRun) || '',
    error: asString(rec.error) || '',
  };
};

const mapPBSVerifyJob = (value: unknown): PBSVerifyJob | null => {
  const rec = asRecord(value);
  if (!rec) return null;
  return {
    id: asString(rec.id) || '',
    store: asString(rec.store) || '',
    status: asString(rec.status) || '',
    lastVerify: asString(rec.lastVerify) || '',
    nextRun: asString(rec.nextRun) || '',
    error: asString(rec.error) || '',
  };
};

const mapPBSPruneJob = (value: unknown): PBSPruneJob | null => {
  const rec = asRecord(value);
  if (!rec) return null;
  return {
    id: asString(rec.id) || '',
    store: asString(rec.store) || '',
    status: asString(rec.status) || '',
    lastPrune: asString(rec.lastPrune) || '',
    nextRun: asString(rec.nextRun) || '',
    error: asString(rec.error) || '',
  };
};

const mapPBSGarbageJob = (value: unknown): PBSGarbageJob | null => {
  const rec = asRecord(value);
  if (!rec) return null;
  return {
    id: asString(rec.id) || '',
    store: asString(rec.store) || '',
    status: asString(rec.status) || '',
    lastGarbage: asString(rec.lastGarbage) || '',
    nextRun: asString(rec.nextRun) || '',
    removedBytes: asNumber(rec.removedBytes) ?? 0,
    error: asString(rec.error) || '',
  };
};

export const pbsInstanceFromResource = (resource: Resource): PBSInstance | null => {
  if (resource.type !== 'pbs') return null;
  const platform = resourcePlatformData(resource);
  const pbs = asRecord(platform?.pbs);
  const memoryTotal = resource.memory?.total ?? asNumber(pbs?.memoryTotal) ?? 0;
  const memoryUsed = resource.memory?.used ?? asNumber(pbs?.memoryUsed) ?? 0;
  const cpu = resource.cpu?.current ?? asNumber(pbs?.cpuPercent) ?? 0;
  const memoryPercent =
    resource.memory?.current ?? (memoryTotal > 0 ? (memoryUsed / memoryTotal) * 100 : 0);
  const hostName = getPreferredResourceHostname(resource) || resource.id;
  const host = resource.platformId || `https://${hostName}:8007`;

  return {
    id: asString(pbs?.instanceId) || resource.id,
    name: getPreferredInfrastructureDisplayName(resource),
    host,
    guestURL:
      asString((resource as unknown as Record<string, unknown>).customURL) ||
      asString((resource as unknown as Record<string, unknown>).customUrl),
    status: resource.status || 'unknown',
    version: asString(pbs?.version) || '',
    cpu,
    memory: memoryPercent,
    memoryUsed,
    memoryTotal,
    uptime: resource.uptime ?? asNumber(pbs?.uptimeSeconds) ?? 0,
    datastores: asArray(pbs?.datastores)
      .map(mapPBSDatastore)
      .filter((entry): entry is PBSDatastore => Boolean(entry)),
    backupJobs: asArray(pbs?.backupJobs)
      .map(mapPBSBackupJob)
      .filter((entry): entry is PBSBackupJob => Boolean(entry)),
    syncJobs: asArray(pbs?.syncJobs)
      .map(mapPBSSyncJob)
      .filter((entry): entry is PBSSyncJob => Boolean(entry)),
    verifyJobs: asArray(pbs?.verifyJobs)
      .map(mapPBSVerifyJob)
      .filter((entry): entry is PBSVerifyJob => Boolean(entry)),
    pruneJobs: asArray(pbs?.pruneJobs)
      .map(mapPBSPruneJob)
      .filter((entry): entry is PBSPruneJob => Boolean(entry)),
    garbageJobs: asArray(pbs?.garbageJobs)
      .map(mapPBSGarbageJob)
      .filter((entry): entry is PBSGarbageJob => Boolean(entry)),
    connectionHealth: asString(pbs?.connectionHealth) || resource.status || 'unknown',
    lastSeen: toISOTime(undefined, resource.lastSeen),
  };
};

const mapPMGNodeStatus = (value: unknown): PMGNodeStatus | null => {
  const rec = asRecord(value);
  if (!rec) return null;
  const queue = asRecord(rec.queueStatus);
  return {
    name: asString(rec.name) || '',
    status: asString(rec.status) || '',
    role: asString(rec.role),
    uptime: asNumber(rec.uptime),
    loadAvg: asString(rec.loadAvg),
    queueStatus: queue
      ? {
          active: asNumber(queue.active) ?? 0,
          deferred: asNumber(queue.deferred) ?? 0,
          hold: asNumber(queue.hold) ?? 0,
          incoming: asNumber(queue.incoming) ?? 0,
          total: asNumber(queue.total) ?? 0,
          oldestAge: asNumber(queue.oldestAge) ?? 0,
          updatedAt: asString(queue.updatedAt) || '',
        }
      : undefined,
  };
};

const mapPMGMailStats = (value: unknown): PMGMailStats | undefined => {
  const rec = asRecord(value);
  if (!rec) return undefined;
  return {
    timeframe: asString(rec.timeframe) || '',
    countTotal: asNumber(rec.countTotal) ?? 0,
    countIn: asNumber(rec.countIn) ?? 0,
    countOut: asNumber(rec.countOut) ?? 0,
    spamIn: asNumber(rec.spamIn) ?? 0,
    spamOut: asNumber(rec.spamOut) ?? 0,
    virusIn: asNumber(rec.virusIn) ?? 0,
    virusOut: asNumber(rec.virusOut) ?? 0,
    bouncesIn: asNumber(rec.bouncesIn) ?? 0,
    bouncesOut: asNumber(rec.bouncesOut) ?? 0,
    bytesIn: asNumber(rec.bytesIn) ?? 0,
    bytesOut: asNumber(rec.bytesOut) ?? 0,
    greylistCount: asNumber(rec.greylistCount) ?? 0,
    junkIn: asNumber(rec.junkIn) ?? 0,
    averageProcessTimeMs: asNumber(rec.averageProcessTimeMs) ?? 0,
    rblRejects: asNumber(rec.rblRejects) ?? 0,
    pregreetRejects: asNumber(rec.pregreetRejects) ?? 0,
    updatedAt: toISOTime(rec.updatedAt),
  };
};

const mapPMGQuarantine = (value: unknown): PMGQuarantineTotals | undefined => {
  const rec = asRecord(value);
  if (!rec) return undefined;
  return {
    spam: asNumber(rec.spam) ?? 0,
    virus: asNumber(rec.virus) ?? 0,
    attachment: asNumber(rec.attachment) ?? 0,
    blacklisted: asNumber(rec.blacklisted) ?? 0,
  };
};

const mapPMGSpamBucket = (value: unknown): PMGSpamBucket | null => {
  const rec = asRecord(value);
  if (!rec) return null;
  return {
    score: asString(rec.score) || asString(rec.bucket) || '',
    count: asNumber(rec.count) ?? 0,
  };
};

const mapPMGRelayDomain = (value: unknown): PMGRelayDomain | null => {
  const rec = asRecord(value);
  if (!rec) return null;
  return {
    domain: asString(rec.domain) || '',
    comment: asString(rec.comment),
  };
};

const mapPMGDomainStat = (value: unknown): PMGDomainStat | null => {
  const rec = asRecord(value);
  if (!rec) return null;
  return {
    domain: asString(rec.domain) || '',
    mailCount: asNumber(rec.mailCount) ?? 0,
    spamCount: asNumber(rec.spamCount) ?? 0,
    virusCount: asNumber(rec.virusCount) ?? 0,
    bytes: asNumber(rec.bytes),
  };
};

const mapPMGMailCountPoint = (value: unknown): PMGMailCountPoint | null => {
  const rec = asRecord(value);
  if (!rec) return null;
  return {
    timestamp: toISOTime(rec.timestamp),
    count: asNumber(rec.count) ?? 0,
    countIn: asNumber(rec.countIn) ?? 0,
    countOut: asNumber(rec.countOut) ?? 0,
    spamIn: asNumber(rec.spamIn) ?? 0,
    spamOut: asNumber(rec.spamOut) ?? 0,
    virusIn: asNumber(rec.virusIn) ?? 0,
    virusOut: asNumber(rec.virusOut) ?? 0,
    rblRejects: asNumber(rec.rblRejects) ?? 0,
    pregreet: asNumber(rec.pregreet) ?? 0,
    bouncesIn: asNumber(rec.bouncesIn) ?? 0,
    bouncesOut: asNumber(rec.bouncesOut) ?? 0,
    greylist: asNumber(rec.greylist) ?? 0,
    index: asNumber(rec.index) ?? 0,
    timeframe: asString(rec.timeframe) || '',
    windowStart: asString(rec.windowStart),
    windowEnd: asString(rec.windowEnd),
  };
};

export const pmgInstanceFromResource = (resource: Resource): PMGInstance | null => {
  if (resource.type !== 'pmg') return null;
  const platform = resourcePlatformData(resource);
  const pmg = asRecord(platform?.pmg);
  const hostName = getPreferredResourceHostname(resource) || resource.id;
  const host = asString(pmg?.hostUrl) || resource.platformId || `https://${hostName}:8006`;
  const lastSeen = toISOTime(undefined, resource.lastSeen);
  const mailStats =
    mapPMGMailStats(pmg?.mailStats) ||
    mapPMGMailStats({
      countTotal: asNumber(pmg?.mailCountTotal),
      spamIn: asNumber(pmg?.spamIn),
      virusIn: asNumber(pmg?.virusIn),
      updatedAt: pmg?.lastUpdated,
    });

  return {
    id: asString(pmg?.instanceId) || resource.id,
    name: getPreferredInfrastructureDisplayName(resource),
    host,
    guestURL:
      asString(pmg?.guestUrl) ||
      asString((resource as unknown as Record<string, unknown>).customURL) ||
      asString((resource as unknown as Record<string, unknown>).customUrl),
    status: resource.status || 'unknown',
    version: asString(pmg?.version) || '',
    nodes: asArray(pmg?.nodes)
      .map(mapPMGNodeStatus)
      .filter((entry): entry is PMGNodeStatus => Boolean(entry)),
    mailStats,
    mailCount: asArray(pmg?.mailCount)
      .map(mapPMGMailCountPoint)
      .filter((entry): entry is PMGMailCountPoint => Boolean(entry)),
    spamDistribution: asArray(pmg?.spamDistribution)
      .map(mapPMGSpamBucket)
      .filter((entry): entry is PMGSpamBucket => Boolean(entry)),
    quarantine: mapPMGQuarantine(pmg?.quarantine),
    relayDomains: asArray(pmg?.relayDomains)
      .map(mapPMGRelayDomain)
      .filter((entry): entry is PMGRelayDomain => Boolean(entry)),
    domainStats: asArray(pmg?.domainStats)
      .map(mapPMGDomainStat)
      .filter((entry): entry is PMGDomainStat => Boolean(entry)),
    domainStatsAsOf: toISOTime(pmg?.domainStatsAsOf),
    connectionHealth: asString(pmg?.connectionHealth) || resource.status || 'unknown',
    lastSeen,
    lastUpdated: toISOTime(pmg?.lastUpdated, resource.lastSeen),
  };
};
