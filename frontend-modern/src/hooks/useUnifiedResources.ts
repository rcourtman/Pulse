import { batch, createEffect, createSignal, onCleanup, type Accessor } from 'solid-js';
import { createStore, reconcile } from 'solid-js/store';
import { canonicalizeMetricsHistoryTargetType } from '@/api/charts';
import { readAPIErrorMessage } from '@/api/responseUtils';
import { apiFetch, getOrgID } from '@/utils/apiClient';
import { normalizeOrgScope } from '@/utils/orgScope';
import { asTrimmedString } from '@/utils/stringUtils';
import { getGlobalWebSocketStore } from '@/stores/websocket-global';
import type {
  Resource,
  ResourceAgentUnraidMeta,
  ResourceCephMeta,
  ResourceChange,
  ResourceFacetCounts,
  ResourceDiscoveryReadiness,
  ResourceDiscoveryTarget,
  ResourceDockerMeta,
  ResourceMetricsTarget,
  ResourceAvailabilityMeta,
  ResourcePBSMeta,
  ResourcePolicyPostureSummary,
  ResourcePhysicalDiskMeta,
  ResourceStatus,
  ResourceStorageMeta,
  ResourceStorageRisk,
  ResourceTrueNASMeta,
  ResourceType,
  ResourceVMwareMeta,
} from '@/types/resource';
import { normalizeDiskArray } from '@/utils/format';
import { logger } from '@/utils/logger';
import { eventBus } from '@/stores/events';
import { canonicalDiscoveryResourceType } from '@/utils/discoveryTarget';
import { canonicalizeFrontendResourceType } from '@/utils/resourceTypeCompat';
import { getPreferredNormalizedPlatformId } from '@/utils/resourceIdentity';
import { getExplicitResourceClusterName } from '@/utils/agentResources';
import { mergeCanonicalResourceSnapshot } from '@/utils/resourceStateAdapters';
import {
  normalizeSourcePlatformScopes,
  resolvePlatformTypeFromSources,
  resolveSourceTypeFromSources,
} from '@/utils/sourcePlatforms';

const UNIFIED_RESOURCES_BASE_URL = '/api/resources';
const DEFAULT_UNIFIED_RESOURCES_QUERY =
  'type=agent,docker-host,pbs,pmg,k8s-cluster,k8s-node,network-endpoint';
const STORAGE_RECOVERY_UNIFIED_RESOURCES_QUERY =
  'type=storage,pbs,pmg,vm,system-container,pod,agent,k8s-cluster,k8s-node,physical_disk,ceph';
const UNIFIED_RESOURCES_PAGE_LIMIT = 100;
const UNIFIED_RESOURCES_CACHE_MAX_AGE_MS = 15_000;
const UNIFIED_RESOURCES_WS_DEBOUNCE_MS = 800;
const UNIFIED_RESOURCES_WS_MIN_REFETCH_INTERVAL_MS = 2_500;
const UNIFIED_RESOURCES_WS_INITIAL_HYDRATION_WAIT_MS = 1_200;
const UNIFIED_RESOURCES_WS_CANONICAL_REVALIDATE_DELAY_MS = UNIFIED_RESOURCES_CACHE_MAX_AGE_MS + 250;

type APIMetricValue = {
  value?: number;
  used?: number;
  total?: number;
  percent?: number;
  unit?: string;
};

type APIAgentDiskInfo = {
  device?: string;
  mountpoint?: string;
  filesystem?: string;
  type?: string;
  total?: number;
  used?: number;
  free?: number;
};

type APIAgentNetworkInterface = {
  name: string;
  mac?: string;
  addresses?: string[];
  rxBytes?: number;
  txBytes?: number;
  speedMbps?: number;
};

type APIAgentDiskIO = {
  device: string;
  readBytes?: number;
  writeBytes?: number;
  readOps?: number;
  writeOps?: number;
  readTimeMs?: number;
  writeTimeMs?: number;
  ioTimeMs?: number;
};

type APIAgentSensorSummary = {
  temperatureCelsius?: Record<string, number>;
  fanRpm?: Record<string, number>;
  additional?: Record<string, number>;
  thermalState?: {
    source?: string;
    pressure?: string;
    thermalWarningLevel?: number;
    performanceWarningLevel?: number;
    cpuPowerStatus?: number;
    limitsPercent?: Record<string, number>;
  };
  smart?: Array<{
    device: string;
    model?: string;
    serial?: string;
    wwn?: string;
    type?: string;
    temperature: number;
    health?: string;
    standby?: boolean;
  }>;
};

type APIHostRAIDDevice = {
  device: string;
  state: string;
  slot: number;
};

type APIHostRAIDArray = {
  device: string;
  name?: string;
  level: string;
  state: string;
  totalDevices: number;
  activeDevices: number;
  workingDevices: number;
  failedDevices: number;
  spareDevices: number;
  uuid?: string;
  devices: APIHostRAIDDevice[];
  rebuildPercent: number;
  rebuildSpeed?: string;
};

type APIKubernetesData = {
  clusterId?: string;
  clusterName?: string;
  context?: string;
  nodeName?: string;
  namespace?: string;
  podName?: string;
  podUid?: string;
  podPhase?: string;
  podReason?: string;
  podMessage?: string;
  podContainers?: Array<{
    name?: string;
    image?: string;
    ready?: boolean;
    restartCount?: number;
    state?: string;
    reason?: string;
    message?: string;
  }>;
  restarts?: number;
  ownerKind?: string;
  ownerName?: string;
  image?: string;
  labels?: Record<string, string>;
  uptimeSeconds?: number;
  temperature?: number;
  metricCapabilities?: {
    nodeCpuMemory?: boolean;
    nodeTelemetry?: boolean;
    podCpuMemory?: boolean;
    podNetwork?: boolean;
    podEphemeralDisk?: boolean;
    podDiskIo?: boolean;
  };
};

type APIResource = {
  id: string;
  type?: string;
  name?: string;
  status?: string;
  uptime?: number;
  lastSeen?: string;
  parentName?: string;
  sources?: string[];
  platformScopes?: string[];
  sourceStatus?: Record<string, { status: string; lastSeen: string; error?: string }>;
  identity?: {
    machineId?: string;
    hostnames?: string[];
    ipAddresses?: string[];
    clusterName?: string;
  };
  metrics?: {
    cpu?: APIMetricValue;
    memory?: APIMetricValue;
    disk?: APIMetricValue;
    netIn?: APIMetricValue;
    netOut?: APIMetricValue;
    diskRead?: APIMetricValue;
    diskWrite?: APIMetricValue;
  };
  parentId?: string;
  tags?: string[];
  proxmox?: {
    nodeName?: string;
    clusterName?: string;
    instance?: string;
    host?: string;
    guestUrl?: string;
    connectionHealth?: string;
    pveVersion?: string;
    kernelVersion?: string;
    vmid?: number;
    cpus?: number;
    uptime?: number;
    temperature?: number;
    temperatureDetails?: {
      available?: boolean;
      legacySensorsFormat?: boolean;
    };
    template?: boolean;
    disks?: APIAgentDiskInfo[];
    swapUsed?: number;
    swapTotal?: number;
    balloon?: number;
  };
  agent?: {
    agentId?: string;
    agentVersion?: string;
    hostname?: string;
    platform?: string;
    osName?: string;
    osVersion?: string;
    kernelVersion?: string;
    architecture?: string;
    uptimeSeconds?: number;
    temperature?: number;
    cpuCount?: number;
    memory?: {
      total?: number;
      used?: number;
      free?: number;
      usage?: number;
      swapUsed?: number;
      swapTotal?: number;
    };
    networkInterfaces?: APIAgentNetworkInterface[];
    diskIO?: APIAgentDiskIO[];
    diskIo?: APIAgentDiskIO[];
    disks?: APIAgentDiskInfo[];
    sensors?: APIAgentSensorSummary;
    raid?: APIHostRAIDArray[];
    unraid?: ResourceAgentUnraidMeta;
    storageRisk?: ResourceStorageRisk;
    storageRiskSummary?: string;
    storagePostureSummary?: string;
    protectionReduced?: boolean;
    protectionSummary?: string;
    rebuildInProgress?: boolean;
    rebuildSummary?: string;
    commandsEnabled?: boolean;
    tokenId?: string;
    tokenName?: string;
    tokenHint?: string;
    tokenLastUsedAt?: number;
  };
  docker?: ResourceDockerMeta;
  truenas?: ResourceTrueNASMeta;
  pbs?: {
    instanceId?: string;
    hostname?: string;
    version?: string;
    uptimeSeconds?: number;
    datastoreCount?: number;
    backupJobCount?: number;
    syncJobCount?: number;
    verifyJobCount?: number;
    pruneJobCount?: number;
    garbageJobCount?: number;
    connectionHealth?: string;
    affectedDatastoreCount?: number;
    affectedDatastores?: string[];
    affectedDatastoreSummary?: string;
    protectedWorkloadCount?: number;
    protectedWorkloadTypes?: string[];
    protectedWorkloadNames?: string[];
    protectedWorkloadSummary?: string;
    postureSummary?: string;
    storageRisk?: {
      level?: string;
      reasons?: Array<{
        code?: string;
        severity?: string;
        summary?: string;
      }>;
    };
  };
  storage?: {
    type?: string;
    content?: string;
    contentTypes?: string[];
    shared?: boolean;
    isCeph?: boolean;
    isZfs?: boolean;
    platform?: string;
    topology?: string;
    protection?: string;
    risk?: {
      level?: string;
      reasons?: Array<{
        code?: string;
        severity?: string;
        summary?: string;
      }>;
    };
    riskSummary?: string;
    consumerCount?: number;
    consumerTypes?: string[];
    topConsumers?: Array<{
      resourceId?: string;
      resourceType?: string;
      name?: string;
      diskCount?: number;
    }>;
    consumerImpactSummary?: string;
    postureSummary?: string;
    protectionReduced?: boolean;
    protectionSummary?: string;
    rebuildInProgress?: boolean;
    rebuildSummary?: string;
    nodes?: string[];
    pool?: string;
    path?: string;
    zfsPool?: unknown;
    zfsPoolState?: string;
    zfsReadErrors?: number;
    zfsWriteErrors?: number;
    zfsChecksumErrors?: number;
    arrayState?: string;
    syncAction?: string;
    syncProgress?: number;
    numProtected?: number;
    numDisabled?: number;
    numInvalid?: number;
    numMissing?: number;
  };
  pmg?: {
    instanceId?: string;
    hostname?: string;
    version?: string;
    nodeCount?: number;
    uptimeSeconds?: number;
    queueActive?: number;
    queueDeferred?: number;
    queueHold?: number;
    queueIncoming?: number;
    queueTotal?: number;
    mailCountTotal?: number;
    spamIn?: number;
    virusIn?: number;
    connectionHealth?: string;
    lastUpdated?: string;
  };
  kubernetes?: APIKubernetesData;
  vmware?: {
    connectionId?: string;
    connectionName?: string;
    vcenterHost?: string;
    managedObjectId?: string;
    entityType?: string;
    hostUuid?: string;
    datacenterId?: string;
    datacenterName?: string;
    computeResourceId?: string;
    computeResourceName?: string;
    clusterId?: string;
    clusterName?: string;
    folderId?: string;
    folderName?: string;
    resourcePoolId?: string;
    resourcePoolName?: string;
    runtimeHostId?: string;
    runtimeHostName?: string;
    connectionState?: string;
    powerState?: string;
    overallStatus?: string;
    cpuCount?: number;
    memorySizeMib?: number;
    datastoreType?: string;
    datastoreIds?: string[];
    datastoreNames?: string[];
    datastoreUrl?: string;
    datastoreAccessible?: boolean;
    multipleHostAccess?: boolean;
    maintenanceMode?: string;
    instanceUuid?: string;
    biosUuid?: string;
    guestOsFamily?: string;
    guestHostname?: string;
    guestIpAddresses?: string[];
    activeAlarmCount?: number;
    activeAlarmSummary?: string;
    recentTaskCount?: number;
    recentTaskSummary?: string;
    snapshotCount?: number;
  };
  availability?: ResourceAvailabilityMeta;
  availabilityChecks?: ResourceAvailabilityMeta[];
  recentChanges?: ResourceChange[];
  facetCounts?: ResourceFacetCounts;
  physicalDisk?: ResourcePhysicalDiskMeta;
  ceph?: {
    fsid?: string;
    healthStatus?: string;
    healthMessage?: string;
    numMons?: number;
    numMgrs?: number;
    numOsds?: number;
    numOsdsUp?: number;
    numOsdsIn?: number;
    numPGs?: number;
    pools?: Array<{
      name: string;
      storedBytes: number;
      availableBytes: number;
      objects: number;
      percentUsed: number;
    }>;
    services?: Array<{
      type: string;
      running: number;
      total: number;
    }>;
  };
  discoveryTarget?: {
    resourceType?: string;
    agentId?: string;
    resourceId?: string;
    hostname?: string;
  };
  discoveryReadiness?: ResourceDiscoveryReadiness;
  metricsTarget?: {
    resourceType?: string;
    resourceId?: string;
  };
  canonicalIdentity?: {
    displayName?: string;
    hostname?: string;
    platformId?: string;
    primaryId?: string;
    aliases?: string[];
    supersededIds?: string[];
  };
  policy?: {
    sensitivity?: string;
    routing?: {
      scope?: string;
      redact?: string[];
    };
  };
  aiSafeSummary?: string;
  incidents?: Array<{
    provider?: string;
    nativeId?: string;
    code?: string;
    severity?: string;
    source?: string;
    summary?: string;
    startedAt?: string;
  }>;
  incidentCount?: number;
  incidentCode?: string;
  incidentSeverity?: string;
  incidentSummary?: string;
  incidentCategory?: string;
  incidentLabel?: string;
  incidentPriority?: number;
  incidentImpactSummary?: string;
  incidentUrgency?: string;
  incidentAction?: string;
};

type APIListResponse = {
  data?: APIResource[];
  resources?: APIResource[];
  meta?: {
    totalPages?: number;
  };
  aggregations?: {
    policyPosture?: APIResourcePolicyPostureSummary;
  };
};

type APIResourcePolicyPostureSummary = {
  totalResources?: number;
  sensitivityCounts?: Partial<Record<string, number>>;
  routingCounts?: Partial<Record<string, number>>;
  redactionCounts?: Partial<Record<string, number>>;
};

type UnifiedResourcesSnapshot = {
  resources: Resource[];
  policyPosture: ResourcePolicyPostureSummary | null;
};

type UnifiedResourcesCacheEntry = {
  resources: Resource[];
  policyPosture: ResourcePolicyPostureSummary | null;
  hasSnapshot: boolean;
  cachedAt: number;
  lastFetchAt: number;
  sharedFetch: Promise<Resource[]> | null;
};

const unifiedResourcesCaches = new Map<string, UnifiedResourcesCacheEntry>();
const ALL_RESOURCES_CACHE_KEY = 'all-resources';

const buildScopedUnifiedResourcesCacheKey = (cacheKey: string, orgScope: string): string =>
  `${encodeURIComponent(orgScope)}::${cacheKey}`;

const resolveStatus = (status?: string): ResourceStatus => {
  const normalized = (status || '').toLowerCase();
  if (normalized === 'online' || normalized === 'running') return 'online';
  if (normalized === 'offline' || normalized === 'stopped') return 'offline';
  if (normalized === 'warning' || normalized === 'degraded') return 'degraded';
  if (normalized === 'paused') return 'paused';
  return 'unknown';
};

const resolveType = (value?: string): ResourceType => {
  const normalized = (value || '').toLowerCase();
  const canonicalFrontendType = canonicalizeFrontendResourceType(normalized);
  switch (canonicalFrontendType) {
    case 'agent':
    case 'storage':
    case 'docker-host':
    case 'docker-image':
    case 'docker-volume':
    case 'docker-network':
    case 'docker-task':
    case 'docker-swarm-node':
    case 'docker-secret':
    case 'docker-config':
    case 'k8s-cluster':
    case 'k8s-node':
    case 'k8s-deployment':
    case 'k8s-replicaset':
    case 'k8s-service':
    case 'k8s-namespace':
    case 'k8s-statefulset':
    case 'k8s-daemonset':
    case 'k8s-job':
    case 'k8s-cronjob':
    case 'k8s-ingress':
    case 'k8s-endpoint-slice':
    case 'k8s-network-policy':
    case 'k8s-persistent-volume':
    case 'k8s-persistent-volume-claim':
    case 'k8s-storage-class':
    case 'k8s-configmap':
    case 'k8s-secret':
    case 'k8s-serviceaccount':
    case 'k8s-resource-quota':
    case 'k8s-limit-range':
    case 'k8s-pod-disruption-budget':
    case 'k8s-horizontal-pod-autoscaler':
    case 'k8s-event':
    case 'vm':
    case 'system-container':
    case 'oci-container':
    case 'app-container':
    case 'pod':
    case 'pbs':
    case 'pmg':
    case 'ceph':
    case 'network':
    case 'network-share':
    case 'network-endpoint':
      return canonicalFrontendType;
    case 'disk':
      return 'physical_disk';
    default:
      break;
  }

  switch (normalized) {
    case 'jail':
      return 'jail';
    case 'docker-service':
      return 'docker-service';
    case 'docker-image':
      return 'docker-image';
    case 'docker-volume':
      return 'docker-volume';
    case 'docker-network':
      return 'docker-network';
    case 'docker-task':
      return 'docker-task';
    case 'docker-swarm-node':
      return 'docker-swarm-node';
    case 'docker-secret':
      return 'docker-secret';
    case 'docker-config':
      return 'docker-config';
    case 'k8s-deployment':
      return 'k8s-deployment';
    case 'k8s-replicaset':
      return 'k8s-replicaset';
    case 'k8s-service':
      return 'k8s-service';
    case 'k8s-namespace':
      return 'k8s-namespace';
    case 'k8s-statefulset':
      return 'k8s-statefulset';
    case 'k8s-daemonset':
      return 'k8s-daemonset';
    case 'k8s-job':
      return 'k8s-job';
    case 'k8s-cronjob':
      return 'k8s-cronjob';
    case 'k8s-ingress':
      return 'k8s-ingress';
    case 'k8s-endpoint-slice':
      return 'k8s-endpoint-slice';
    case 'k8s-network-policy':
      return 'k8s-network-policy';
    case 'k8s-persistent-volume':
      return 'k8s-persistent-volume';
    case 'k8s-persistent-volume-claim':
      return 'k8s-persistent-volume-claim';
    case 'k8s-storage-class':
      return 'k8s-storage-class';
    case 'k8s-configmap':
      return 'k8s-configmap';
    case 'k8s-secret':
      return 'k8s-secret';
    case 'k8s-serviceaccount':
      return 'k8s-serviceaccount';
    case 'k8s-resource-quota':
      return 'k8s-resource-quota';
    case 'k8s-limit-range':
      return 'k8s-limit-range';
    case 'k8s-pod-disruption-budget':
      return 'k8s-pod-disruption-budget';
    case 'k8s-horizontal-pod-autoscaler':
      return 'k8s-horizontal-pod-autoscaler';
    case 'k8s-event':
      return 'k8s-event';
    case 'storage':
      return 'storage';
    case 'datastore':
      return 'datastore';
    case 'pool':
      return 'pool';
    case 'dataset':
      return 'dataset';
    case 'physical_disk':
    case 'physical-disk':
      return 'physical_disk';
    case 'network-endpoint':
    case 'network_endpoint':
    case 'availability':
      return 'network-endpoint';
    default:
      return 'agent';
  }
};

const resolveDiscoveryResourceType = (
  value?: string,
): ResourceDiscoveryTarget['resourceType'] | undefined => {
  const normalized = canonicalDiscoveryResourceType(value);
  switch (normalized) {
    case 'agent':
      return 'agent';
    case 'vm':
      return 'vm';
    case 'system-container':
      return 'system-container';
    case 'app-container':
      return 'app-container';
    case 'pod':
      return 'pod';
    case 'disk':
      return 'disk';
    case 'ceph':
      return 'ceph';
    default:
      return undefined;
  }
};

const resolveMetricsTarget = (
  resourceType: string | undefined,
  metricsTarget?: { resourceType?: string; resourceId?: string },
): ResourceMetricsTarget | undefined => {
  const canonicalType = metricsTarget?.resourceType
    ? canonicalizeMetricsHistoryTargetType(metricsTarget.resourceType, resourceType)
    : null;
  const resourceID = asTrimmedString(metricsTarget?.resourceId);
  return canonicalType && resourceID
    ? { resourceType: canonicalType, resourceId: resourceID }
    : undefined;
};

const metricToResourceMetric = (metric?: APIMetricValue) => {
  if (!metric) return undefined;
  const used = metric.used ?? undefined;
  const total = metric.total ?? undefined;
  const current = metric.percent ?? metric.value ?? 0;
  const free = total !== undefined && used !== undefined ? total - used : undefined;
  return {
    current,
    total,
    used,
    free,
  };
};

const toResource = (v2: APIResource): Resource => {
  const sources = (v2.sources || []).filter(
    (s): s is string => typeof s === 'string' && s.trim().length > 0,
  );
  const lastSeen = v2.lastSeen ? Date.parse(v2.lastSeen) : NaN;
  const canonical = v2.canonicalIdentity;
  const name = asTrimmedString(canonical?.displayName) || v2.name || v2.id;
  const platformId = asTrimmedString(canonical?.platformId) || getPreferredNormalizedPlatformId(v2);
  const resourceType = resolveType(v2.type);
  const platformType =
    resolvePlatformTypeFromSources(sources) ||
    (resourceType === 'network-endpoint' ? 'availability' : 'agent');
  const platformScopes = normalizeSourcePlatformScopes(v2.platformScopes, platformType);

  const discoveryResourceType = resolveDiscoveryResourceType(v2.discoveryTarget?.resourceType);
  const discoveryAgentId = v2.discoveryTarget?.agentId;
  const discoveryTarget =
    discoveryResourceType && discoveryAgentId && v2.discoveryTarget?.resourceId
      ? {
          resourceType: discoveryResourceType,
          agentId: discoveryAgentId,
          resourceId: v2.discoveryTarget.resourceId,
          hostname: v2.discoveryTarget.hostname,
        }
      : undefined;

  const metricsTarget = resolveMetricsTarget(v2.type, v2.metricsTarget);
  return {
    id: v2.id,
    type: resourceType,
    name,
    displayName: name,
    platformId,
    platformType,
    platformScopes,
    sourceType: resolveSourceTypeFromSources(sources),
    parentId: v2.parentId,
    parentName: v2.parentName,
    clusterId: getExplicitResourceClusterName(v2),
    status: resolveStatus(v2.status),
    incidents: (v2.incidents || [])
      .map((incident) => ({
        provider: incident.provider,
        nativeId: incident.nativeId,
        code: incident.code || '',
        severity: incident.severity || '',
        source: incident.source,
        summary: incident.summary || '',
        startedAt: incident.startedAt,
      }))
      .filter((incident) => incident.code.trim() || incident.summary.trim()),
    incidentCount: v2.incidentCount,
    incidentCode: v2.incidentCode,
    incidentSeverity: v2.incidentSeverity,
    incidentSummary: v2.incidentSummary,
    incidentCategory: v2.incidentCategory,
    incidentLabel: v2.incidentLabel,
    incidentPriority: v2.incidentPriority,
    incidentImpactSummary: v2.incidentImpactSummary,
    incidentUrgency: v2.incidentUrgency,
    incidentAction: v2.incidentAction,
    agent: v2.agent,
    kubernetes: v2.kubernetes,
    docker: v2.docker as ResourceDockerMeta | undefined,
    truenas: v2.truenas as ResourceTrueNASMeta | undefined,
    vmware: v2.vmware as ResourceVMwareMeta | undefined,
    pbs: v2.pbs as ResourcePBSMeta | undefined,
    availability: v2.availability as ResourceAvailabilityMeta | undefined,
    availabilityChecks: v2.availabilityChecks as ResourceAvailabilityMeta[] | undefined,
    physicalDisk: v2.physicalDisk,
    storage: v2.storage as ResourceStorageMeta | undefined,
    ceph: v2.ceph as ResourceCephMeta | undefined,
    proxmox: v2.proxmox
      ? {
          vmid: v2.proxmox.vmid,
          node: v2.proxmox.nodeName,
          nodeName: v2.proxmox.nodeName,
          instance: v2.proxmox.instance,
          clusterName: v2.proxmox.clusterName,
          host: v2.proxmox.host,
          guestUrl: v2.proxmox.guestUrl,
          connectionHealth: v2.proxmox.connectionHealth,
          cpus: v2.proxmox.cpus,
          template: v2.proxmox.template,
          disks: normalizeDiskArray(v2.proxmox.disks),
          swapUsed: v2.proxmox.swapUsed,
          swapTotal: v2.proxmox.swapTotal,
          balloon: v2.proxmox.balloon,
          pveVersion: v2.proxmox.pveVersion,
          kernelVersion: v2.proxmox.kernelVersion,
          temperatureDetails: v2.proxmox.temperatureDetails,
        }
      : undefined,
    cpu: metricToResourceMetric(v2.metrics?.cpu),
    memory: metricToResourceMetric(v2.metrics?.memory),
    disk: metricToResourceMetric(v2.metrics?.disk),
    network:
      v2.metrics?.netIn || v2.metrics?.netOut
        ? {
            rxBytes: v2.metrics?.netIn?.value ?? 0,
            txBytes: v2.metrics?.netOut?.value ?? 0,
          }
        : undefined,
    diskIO:
      v2.metrics?.diskRead || v2.metrics?.diskWrite
        ? {
            readRate: v2.metrics?.diskRead?.value ?? 0,
            writeRate: v2.metrics?.diskWrite?.value ?? 0,
          }
        : undefined,
    uptime:
      v2.agent?.uptimeSeconds ??
      v2.proxmox?.uptime ??
      v2.pbs?.uptimeSeconds ??
      v2.pmg?.uptimeSeconds ??
      v2.kubernetes?.uptimeSeconds ??
      // Canonical Resource.Uptime is the universal fallback — vSphere
      // adapters populate only this field (no platform-specific carve-out
      // for hosts/datastores/networks), so the chain has to land here for
      // ESXi host uptime to surface on the unified-resources side. Same
      // fallback shape as the workloads mapping in useWorkloads.ts.
      v2.uptime,
    temperature:
      v2.agent?.temperature ??
      v2.proxmox?.temperature ??
      v2.docker?.temperature ??
      v2.kubernetes?.temperature ??
      v2.physicalDisk?.temperature,
    tags: v2.tags,
    lastSeen: Number.isFinite(lastSeen) ? lastSeen : Date.now(),
    identity: {
      hostname: asTrimmedString(canonical?.hostname) || v2.identity?.hostnames?.[0],
      machineId: v2.identity?.machineId,
      ips: v2.identity?.ipAddresses,
    },
    discoveryTarget,
    discoveryReadiness: v2.discoveryReadiness,
    metricsTarget,
    canonicalIdentity: v2.canonicalIdentity,
    policy: v2.policy as Resource['policy'],
    aiSafeSummary: v2.aiSafeSummary as Resource['aiSafeSummary'],
    recentChanges: v2.recentChanges,
    facetCounts: v2.facetCounts,
    platformData: {
      sources,
      platformScopes,
      sourceStatus: v2.sourceStatus,
      proxmox: v2.proxmox,
      agent: v2.agent,
      docker: v2.docker,
      truenas: v2.truenas,
      pbs: v2.pbs,
      storage: v2.storage,
      pmg: v2.pmg,
      kubernetes: v2.kubernetes,
      vmware: v2.vmware,
      availability: v2.availability,
      availabilityChecks: v2.availabilityChecks,
      physicalDisk: v2.physicalDisk,
      ceph: v2.ceph,
      metrics: v2.metrics,
      discoveryTarget: v2.discoveryTarget,
      discoveryReadiness: v2.discoveryReadiness,
    },
  };
};

const normalizeUnifiedResourcesQuery = (query?: string): string =>
  (query || '').trim().replace(/^\?+/, '');

const buildUnifiedResourcesUrl = (query: string, page: number): string => {
  const params = new URLSearchParams(query);
  params.set('page', String(page));
  params.set('limit', String(UNIFIED_RESOURCES_PAGE_LIMIT));
  return `${UNIFIED_RESOURCES_BASE_URL}?${params.toString()}`;
};

const normalizeNonNegativeCount = (value: unknown): number | undefined => {
  const count = typeof value === 'number' ? value : Number(value);
  if (!Number.isFinite(count)) {
    return undefined;
  }
  return Math.max(0, Math.trunc(count));
};

const normalizeCountMap = <T extends string>(value: unknown): Partial<Record<T, number>> => {
  if (!value || typeof value !== 'object' || Array.isArray(value)) {
    return {};
  }

  const counts: Partial<Record<T, number>> = {};
  for (const [key, rawCount] of Object.entries(value as Record<string, unknown>)) {
    const normalizedKey = asTrimmedString(key);
    const normalizedCount = normalizeNonNegativeCount(rawCount);
    if (normalizedKey && normalizedCount !== undefined) {
      counts[normalizedKey as T] = normalizedCount;
    }
  }
  return counts;
};

const normalizeResourcePolicyPosture = (
  value?: APIResourcePolicyPostureSummary | null,
): ResourcePolicyPostureSummary | null => {
  if (!value || typeof value !== 'object' || Array.isArray(value)) {
    return null;
  }

  return {
    totalResources: normalizeNonNegativeCount(value.totalResources) ?? 0,
    sensitivityCounts: normalizeCountMap(value.sensitivityCounts),
    routingCounts: normalizeCountMap(value.routingCounts),
    redactionCounts: normalizeCountMap(value.redactionCounts),
  };
};

const resolveResourcesPayload = (
  payload: unknown,
): {
  data: APIResource[];
  totalPages: number;
  policyPosture: ResourcePolicyPostureSummary | null;
} => {
  if (Array.isArray(payload)) {
    return { data: payload as APIResource[], totalPages: 1, policyPosture: null };
  }
  if (!payload || typeof payload !== 'object') {
    return { data: [], totalPages: 1, policyPosture: null };
  }
  const record = payload as APIListResponse;
  const data = Array.isArray(record.data)
    ? record.data
    : Array.isArray(record.resources)
      ? record.resources
      : [];
  const totalPages = Number.isFinite(record.meta?.totalPages)
    ? Math.max(1, Number(record.meta?.totalPages))
    : 1;
  return {
    data,
    totalPages,
    policyPosture: normalizeResourcePolicyPosture(record.aggregations?.policyPosture),
  };
};

const dedupeResources = (resources: APIResource[]): APIResource[] => {
  const ids = new Set<string>();
  const deduped: APIResource[] = [];
  for (const resource of resources) {
    if (!resource?.id || ids.has(resource.id)) continue;
    ids.add(resource.id);
    deduped.push(resource);
  }
  return deduped;
};

const getUnifiedResourcesCacheEntry = (cacheKey: string): UnifiedResourcesCacheEntry => {
  const existing = unifiedResourcesCaches.get(cacheKey);
  if (existing) {
    return existing;
  }
  const created: UnifiedResourcesCacheEntry = {
    resources: [],
    policyPosture: null,
    hasSnapshot: false,
    cachedAt: 0,
    lastFetchAt: 0,
    sharedFetch: null,
  };
  unifiedResourcesCaches.set(cacheKey, created);
  return created;
};

const hasFreshUnifiedResourcesCache = (entry: UnifiedResourcesCacheEntry) =>
  entry.hasSnapshot && Date.now() - entry.cachedAt <= UNIFIED_RESOURCES_CACHE_MAX_AGE_MS;

const setUnifiedResourcesCache = (
  entry: UnifiedResourcesCacheEntry,
  resources: Resource[],
  at = Date.now(),
  policyPosture: ResourcePolicyPostureSummary | null = entry.policyPosture,
) => {
  entry.resources = resources;
  entry.policyPosture = policyPosture;
  entry.hasSnapshot = true;
  entry.cachedAt = at;
};

const parseUnifiedResourcesTypeFilter = (query: string): Set<ResourceType> | null => {
  const normalizedQuery = normalizeUnifiedResourcesQuery(query);
  if (normalizedQuery === '') {
    return null;
  }

  const params = new URLSearchParams(normalizedQuery);
  const types = new Set<ResourceType>();

  for (const [key, value] of params.entries()) {
    if (key !== 'type') {
      return null;
    }

    value
      .split(',')
      .map((candidate) => asTrimmedString(candidate))
      .filter((candidate): candidate is string => candidate !== undefined)
      .forEach((candidate) => {
        types.add(resolveType(candidate));
      });
  }

  return types.size > 0 ? types : null;
};

const filterCanonicalUnifiedResources = (
  resources: Resource[],
  query: string,
  typeFilter: Set<ResourceType> | null,
): Resource[] | null => {
  const normalizedQuery = normalizeUnifiedResourcesQuery(query);
  if (normalizedQuery === '') {
    return resources;
  }
  if (!typeFilter) {
    return null;
  }
  return resources.filter((resource) => typeFilter.has(resolveType(resource.type)));
};

const seedUnifiedResourcesCacheFromAllResources = (
  entry: UnifiedResourcesCacheEntry,
  cacheKey: string,
  query: string,
  orgScope: string,
) => {
  if (entry.hasSnapshot || cacheKey === ALL_RESOURCES_CACHE_KEY) {
    return entry;
  }

  const typeFilter = parseUnifiedResourcesTypeFilter(query);
  if (!typeFilter) {
    return entry;
  }

  const allResourcesEntry = getUnifiedResourcesCacheEntry(
    buildScopedUnifiedResourcesCacheKey(ALL_RESOURCES_CACHE_KEY, orgScope),
  );
  if (!hasFreshUnifiedResourcesCache(allResourcesEntry)) {
    return entry;
  }

  entry.resources = allResourcesEntry.resources.filter((resource) =>
    typeFilter.has(resolveType(resource.type)),
  );
  entry.policyPosture = allResourcesEntry.policyPosture;
  entry.hasSnapshot = true;
  entry.cachedAt = allResourcesEntry.cachedAt;
  entry.lastFetchAt = allResourcesEntry.lastFetchAt;

  return entry;
};

async function fetchUnifiedResources(query: string): Promise<UnifiedResourcesSnapshot> {
  const normalizedQuery = normalizeUnifiedResourcesQuery(query);
  const allRawResources: APIResource[] = [];
  let policyPosture: ResourcePolicyPostureSummary | null = null;
  let totalPages = 1;

  for (let page = 1; page <= totalPages; page += 1) {
    const response = await apiFetch(buildUnifiedResourcesUrl(normalizedQuery, page), {
      cache: 'no-store',
    });
    if (!response.ok) {
      const message = await readAPIErrorMessage(
        response,
        `Failed to fetch unified resources (HTTP ${response.status})`,
      );
      throw new Error(message);
    }

    const payload = (await response.json()) as APIListResponse | APIResource[];
    const resolved = resolveResourcesPayload(payload);
    allRawResources.push(...resolved.data);
    policyPosture = policyPosture ?? resolved.policyPosture;
    totalPages = Math.max(totalPages, resolved.totalPages);
  }

  return {
    resources: dedupeResources(allRawResources).map((resource) => toResource(resource)),
    policyPosture,
  };
}

const fetchUnifiedResourcesShared = async (
  entry: UnifiedResourcesCacheEntry,
  query: string,
  force = false,
): Promise<Resource[]> => {
  if (!force && hasFreshUnifiedResourcesCache(entry)) {
    return entry.resources;
  }

  if (entry.sharedFetch) {
    return entry.sharedFetch;
  }

  const request = (async () => {
    const fetched = await fetchUnifiedResources(query);
    const now = Date.now();
    setUnifiedResourcesCache(entry, fetched.resources, now, fetched.policyPosture);
    entry.lastFetchAt = now;
    return fetched.resources;
  })();

  entry.sharedFetch = request;

  try {
    return await request;
  } finally {
    if (entry.sharedFetch === request) {
      entry.sharedFetch = null;
    }
  }
};

const shouldThrottleWsRefetch = (entry: UnifiedResourcesCacheEntry) =>
  Date.now() - entry.lastFetchAt < UNIFIED_RESOURCES_WS_MIN_REFETCH_INTERVAL_MS;

export const __resetUnifiedResourcesCacheForTests = () => {
  unifiedResourcesCaches.clear();
};

export const getCachedUnifiedResources = (options?: {
  cacheKey?: string;
  orgID?: string | null;
}): Resource[] => {
  const cacheKey = (options?.cacheKey || ALL_RESOURCES_CACHE_KEY).trim();
  const scopedCacheKey = buildScopedUnifiedResourcesCacheKey(
    cacheKey,
    normalizeOrgScope(options?.orgID ?? getOrgID()),
  );
  return getUnifiedResourcesCacheEntry(scopedCacheKey).resources;
};

type UnifiedResourcesInitialHydration = 'immediate' | 'prefer-ws' | 'prefer-ws-then-rest';

type UseUnifiedResourcesOptions = {
  query?: string;
  cacheKey?: string;
  initialHydration?: UnifiedResourcesInitialHydration;
  enabled?: Accessor<boolean>;
};

export function useUnifiedResources(options?: UseUnifiedResourcesOptions) {
  const query = normalizeUnifiedResourcesQuery(options?.query ?? DEFAULT_UNIFIED_RESOURCES_QUERY);
  const cacheKey = (options?.cacheKey || query || 'all').trim();
  const initialHydration = options?.initialHydration ?? 'immediate';
  const enabled = options?.enabled ?? (() => true);
  const typeFilter = parseUnifiedResourcesTypeFilter(query);
  const supportsCanonicalWsHydration = query === '' || typeFilter !== null;
  const prefersWsInitialHydration =
    (initialHydration === 'prefer-ws' || initialHydration === 'prefer-ws-then-rest') &&
    supportsCanonicalWsHydration;
  const revalidatesRestAfterWsInitialHydration =
    initialHydration === 'prefer-ws-then-rest' && supportsCanonicalWsHydration;
  const [orgScope, setOrgScope] = createSignal(normalizeOrgScope(getOrgID()));
  const resolveScopedCacheKey = () => buildScopedUnifiedResourcesCacheKey(cacheKey, orgScope());
  let cacheEntry = seedUnifiedResourcesCacheFromAllResources(
    getUnifiedResourcesCacheEntry(resolveScopedCacheKey()),
    cacheKey,
    query,
    orgScope(),
  );
  const initialResources = cacheEntry.resources;
  const initialPolicyPosture = cacheEntry.policyPosture;
  const hasCachedResources = cacheEntry.hasSnapshot;

  const [resources, setResources] = createStore<Resource[]>(initialResources);
  const [policyPosture, setPolicyPosture] = createSignal<ResourcePolicyPostureSummary | null>(
    initialPolicyPosture,
  );
  const [loading, setLoading] = createSignal(!hasCachedResources);
  const [error, setError] = createSignal<unknown>(undefined);
  const wsStore = getGlobalWebSocketStore();
  let refreshHandle: ReturnType<typeof setTimeout> | undefined;
  let initialHydrationHandle: ReturnType<typeof setTimeout> | undefined;
  let canonicalRevalidationHandle: ReturnType<typeof setTimeout> | undefined;
  let inFlightRefetch: Promise<Resource[]> | null = null;
  let wsInitialized = false;
  let lastWsUpdateToken = '';
  let blockedWsHydrationToken: string | null = null;
  let scopeVersion = 0;

  const applyResources = (
    next: Resource[],
    targetEntry: UnifiedResourcesCacheEntry = cacheEntry,
  ) => {
    setUnifiedResourcesCache(targetEntry, next);
    if (targetEntry !== cacheEntry) {
      return;
    }
    setResources(reconcile(next, { key: 'id' }));
    setPolicyPosture(targetEntry.policyPosture);
  };

  const runRefetch = async (options?: {
    force?: boolean;
    source?: 'initial' | 'ws' | 'manual';
    background?: boolean;
  }) => {
    if (!enabled()) {
      return resources as unknown as Resource[];
    }
    if (inFlightRefetch) {
      return inFlightRefetch;
    }

    const force = options?.force === true;
    const source = options?.source ?? 'manual';
    const background = options?.background === true;

    if (!force && source === 'ws' && shouldThrottleWsRefetch(cacheEntry)) {
      return resources as unknown as Resource[];
    }

    const shouldForceNetwork = force || source === 'ws';
    const shouldShowLoading = !background && (force || !cacheEntry.hasSnapshot);
    if (shouldShowLoading) {
      setLoading(true);
    }

    const requestVersion = scopeVersion;
    const entryForRequest = cacheEntry;
    let request!: Promise<Resource[]>;
    request = (async () => {
      const isCurrentRequest = () =>
        requestVersion === scopeVersion && entryForRequest === cacheEntry && enabled();

      try {
        const fetched = await fetchUnifiedResourcesShared(
          entryForRequest,
          query,
          shouldForceNetwork,
        );
        if (!isCurrentRequest()) {
          return resources as unknown as Resource[];
        }
        batch(() => {
          applyResources(fetched, entryForRequest);
          setError(undefined);
        });
        return fetched;
      } catch (err) {
        if (!background && isCurrentRequest()) {
          setError(err);
        }
        throw err;
      } finally {
        if (inFlightRefetch === request) {
          inFlightRefetch = null;
        }
        if (shouldShowLoading && isCurrentRequest()) {
          setLoading(false);
        }
      }
    })();

    inFlightRefetch = request;
    return request;
  };

  const refetch = async () => {
    if (!enabled()) {
      return resources as unknown as Resource[];
    }
    return runRefetch({ force: true, source: 'manual' });
  };

  const mutate = (value: Resource[] | ((prev: Resource[]) => Resource[])) => {
    const current = resources as unknown as Resource[];
    const next = typeof value === 'function' ? value(current) : value;
    applyResources(next ?? []);
    return resources as unknown as Resource[];
  };

  const clearInitialHydrationTimeout = () => {
    if (initialHydrationHandle !== undefined) {
      clearTimeout(initialHydrationHandle);
      initialHydrationHandle = undefined;
    }
  };

  const clearRefreshTimeout = () => {
    if (refreshHandle !== undefined) {
      clearTimeout(refreshHandle);
      refreshHandle = undefined;
    }
  };

  const clearCanonicalRevalidationTimeout = () => {
    if (canonicalRevalidationHandle !== undefined) {
      clearTimeout(canonicalRevalidationHandle);
      canonicalRevalidationHandle = undefined;
    }
  };

  const shouldPreferWsInitialHydration = () => prefersWsInitialHydration && !cacheEntry.hasSnapshot;

  const hasWsInitialHydrationSnapshot = () =>
    wsStore.connected() && wsStore.initialDataReceived() && Array.isArray(wsStore.state.resources);

  const scheduleInitialHydrationFallback = () => {
    if (initialHydrationHandle !== undefined) {
      return;
    }
    initialHydrationHandle = setTimeout(() => {
      initialHydrationHandle = undefined;
      if (cacheEntry.hasSnapshot || wsStore.initialDataReceived()) {
        return;
      }
      void runRefetch({ source: 'initial' }).catch((err) => {
        logger.warn('[useUnifiedResources] Failed deferred initial refresh', err);
      });
    }, UNIFIED_RESOURCES_WS_INITIAL_HYDRATION_WAIT_MS);
  };

  const scheduleRefetch = () => {
    clearRefreshTimeout();

    const elapsedSinceFetch = Date.now() - cacheEntry.lastFetchAt;
    const minIntervalDelay = Math.max(
      0,
      UNIFIED_RESOURCES_WS_MIN_REFETCH_INTERVAL_MS - elapsedSinceFetch,
    );
    const delay = Math.max(UNIFIED_RESOURCES_WS_DEBOUNCE_MS, minIntervalDelay);

    refreshHandle = setTimeout(() => {
      refreshHandle = undefined;
      void runRefetch({ source: 'ws' }).catch((err) => {
        logger.debug('[useUnifiedResources] WebSocket-triggered refetch failed', err);
      });
    }, delay);
  };

  const scheduleCanonicalRevalidation = () => {
    if (canonicalRevalidationHandle !== undefined) {
      return;
    }
    // Let websocket-first routes finish their initial summary/table mount
    // before asking REST to enrich the thinner realtime snapshot.
    canonicalRevalidationHandle = setTimeout(() => {
      canonicalRevalidationHandle = undefined;
      if (!enabled()) {
        return;
      }
      void runRefetch({ source: 'initial', background: true }).catch((err) => {
        logger.debug('[useUnifiedResources] Background canonical revalidation failed', err);
      });
    }, UNIFIED_RESOURCES_WS_CANONICAL_REVALIDATE_DELAY_MS);
  };

  createEffect(() => {
    orgScope();

    if (!enabled()) {
      scopeVersion += 1;
      inFlightRefetch = null;
      clearInitialHydrationTimeout();
      clearCanonicalRevalidationTimeout();
      clearRefreshTimeout();
      setLoading(false);
      return;
    }

    if (hasFreshUnifiedResourcesCache(cacheEntry)) {
      clearInitialHydrationTimeout();
      return;
    }

    if (shouldPreferWsInitialHydration()) {
      if (!hasWsInitialHydrationSnapshot()) {
        scheduleInitialHydrationFallback();
      }
      return;
    }

    void runRefetch({ source: 'initial' }).catch((err) => {
      logger.warn('[useUnifiedResources] Failed background refresh for stale cache', err);
    });
  });

  createEffect(() => {
    const currentOrgScope = orgScope();
    if (!enabled()) {
      return;
    }
    if (!supportsCanonicalWsHydration) {
      return;
    }
    if (!wsStore.connected() || !wsStore.initialDataReceived()) {
      return;
    }

    const lastUpdateToken = String(wsStore.state.lastUpdate ?? '');
    if (blockedWsHydrationToken !== null) {
      if (
        lastUpdateToken.length === 0 ||
        lastUpdateToken === '0' ||
        lastUpdateToken === blockedWsHydrationToken
      ) {
        return;
      }
      blockedWsHydrationToken = null;
    }

    const wsResources = Array.isArray(wsStore.state.resources) ? wsStore.state.resources : [];
    // For normal page loads, keep the first paint on the canonical REST contract.
    // Only explicit websocket-first consumers are allowed to render directly
    // from the thinner realtime transport before a canonical snapshot exists.
    if (!cacheEntry.hasSnapshot && !prefersWsInitialHydration) {
      return;
    }
    const shouldRevalidateCanonicalSnapshot =
      revalidatesRestAfterWsInitialHydration && !cacheEntry.hasSnapshot;
    const allResourcesEntry = getUnifiedResourcesCacheEntry(
      buildScopedUnifiedResourcesCacheKey(ALL_RESOURCES_CACHE_KEY, currentOrgScope),
    );
    const mergedWsResources = mergeCanonicalResourceSnapshot(
      wsResources,
      allResourcesEntry.resources,
    );
    const projectedResources = filterCanonicalUnifiedResources(
      mergedWsResources,
      query,
      typeFilter,
    );
    const now = Date.now();
    clearInitialHydrationTimeout();
    setUnifiedResourcesCache(allResourcesEntry, mergedWsResources, now);
    allResourcesEntry.lastFetchAt = now;

    if (projectedResources === null) {
      return;
    }

    const mergedProjectedResources = mergeCanonicalResourceSnapshot(
      projectedResources,
      cacheEntry.resources,
    );
    setUnifiedResourcesCache(cacheEntry, mergedProjectedResources, now);
    cacheEntry.lastFetchAt = now;
    batch(() => {
      setResources(reconcile(mergedProjectedResources, { key: 'id' }));
      setPolicyPosture(cacheEntry.policyPosture);
      setError(undefined);
      setLoading(false);
    });

    if (shouldRevalidateCanonicalSnapshot) {
      scheduleCanonicalRevalidation();
    }
  });

  createEffect(() => {
    orgScope();

    if (!enabled()) {
      wsInitialized = false;
      lastWsUpdateToken = '';
      clearInitialHydrationTimeout();
      clearCanonicalRevalidationTimeout();
      clearRefreshTimeout();
      return;
    }

    if (!wsStore.connected() || !wsStore.initialDataReceived()) {
      wsInitialized = false;
      lastWsUpdateToken = '';
      return;
    }

    const lastUpdateToken = String(wsStore.state.lastUpdate ?? '');

    if (!wsInitialized) {
      wsInitialized = true;
      lastWsUpdateToken = lastUpdateToken;
      if (!supportsCanonicalWsHydration) {
        scheduleRefetch();
      }
      return;
    }

    if (lastUpdateToken === lastWsUpdateToken) {
      return;
    }

    lastWsUpdateToken = lastUpdateToken;
    if (!supportsCanonicalWsHydration) {
      scheduleRefetch();
    }
  });

  const unsubscribeOrgSwitch = eventBus.on('org_switched', (nextOrgID?: string) => {
    const nextOrgScope = normalizeOrgScope(nextOrgID);
    if (nextOrgScope === orgScope()) {
      return;
    }

    scopeVersion += 1;
    const nextCacheEntry = seedUnifiedResourcesCacheFromAllResources(
      getUnifiedResourcesCacheEntry(buildScopedUnifiedResourcesCacheKey(cacheKey, nextOrgScope)),
      cacheKey,
      query,
      nextOrgScope,
    );
    blockedWsHydrationToken = supportsCanonicalWsHydration
      ? String(wsStore.state.lastUpdate ?? '')
      : null;
    cacheEntry = nextCacheEntry;
    inFlightRefetch = null;
    wsInitialized = false;
    lastWsUpdateToken = '';
    clearInitialHydrationTimeout();
    clearCanonicalRevalidationTimeout();
    setOrgScope(nextOrgScope);

    const scopedResources = cacheEntry.resources;
    const scopedPolicyPosture = cacheEntry.policyPosture;
    batch(() => {
      setError(undefined);
      setResources(reconcile(scopedResources, { key: 'id' }));
      setPolicyPosture(scopedPolicyPosture);
      setLoading(enabled() && !cacheEntry.hasSnapshot);
    });

    if (enabled() && !hasFreshUnifiedResourcesCache(cacheEntry)) {
      if (shouldPreferWsInitialHydration()) {
        scheduleInitialHydrationFallback();
      } else {
        void runRefetch({ force: true, source: 'initial' }).catch(() => undefined);
      }
    }
  });

  onCleanup(() => {
    unsubscribeOrgSwitch();
    clearInitialHydrationTimeout();
    clearCanonicalRevalidationTimeout();
    clearRefreshTimeout();
  });

  return {
    resources: () => resources,
    policyPosture,
    refetch,
    mutate,
    loading,
    error,
  };
}

export function useStorageRecoveryResources() {
  return useUnifiedResources({
    query: STORAGE_RECOVERY_UNIFIED_RESOURCES_QUERY,
    cacheKey: 'storage-recovery',
  });
}

export default useUnifiedResources;
