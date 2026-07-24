import type { Resource } from '@/types/resource';
import type { ResourceType as DiscoveryResourceType } from '@/types/discovery';
import {
  canonicalDiscoveryResourceType,
  isAgentDiscoveryResourceType,
} from '@/utils/discoveryTarget';
import {
  getActionableAgentIdFromResource,
  getActionableDockerRuntimeIdFromResource,
  getActionableKubernetesClusterIdFromResource,
} from '@/utils/agentResources';
import {
  getPreferredResourceClusterName,
  getPreferredInfrastructureDisplayName,
  getPreferredResourceHostname,
} from '@/utils/resourceIdentity';
import {
  buildAppContainerMetadataId,
  buildKubernetesWorkloadMetadataId,
  getCanonicalWorkloadIdForResource,
} from '@/utils/workloads';

export type DiscoveryConfig = {
  resourceType: DiscoveryResourceType;
  agentId: string;
  resourceId: string;
  hostname: string;
  metadataKind: 'guest' | 'agent' | 'docker' | 'docker-host';
  metadataId: string;
  targetLabel: string;
};

type ProxmoxPlatformData = {
  sourceId?: string;
  nodeName?: string;
  vmid?: number;
};

type DockerPlatformData = {
  hostSourceId?: string;
  containerId?: string;
  hostname?: string;
};

type KubernetesPlatformData = {
  clusterId?: string;
  agentId?: string;
  namespace?: string;
  podName?: string;
  podUid?: string;
};

type PlatformData = {
  sources?: string[];
  agent?: {
    hostname?: string;
  };
  docker?: DockerPlatformData;
  kubernetes?: KubernetesPlatformData;
  proxmox?: ProxmoxPlatformData;
  vmware?: unknown;
};

const asString = (value: unknown): string | undefined =>
  typeof value === 'string' && value.trim().length > 0 ? value.trim() : undefined;

const asNumber = (value: unknown): number | undefined =>
  typeof value === 'number' && Number.isFinite(value) ? value : undefined;

const isDiscoveryLookupValue = (value: unknown): value is string => {
  const candidate = asString(value);
  return Boolean(candidate && candidate.toLowerCase() !== 'redacted by policy');
};

const hasSource = (resource: Resource, platformData: PlatformData | undefined, source: string) => {
  const normalizedSource = source.toLowerCase();
  return [
    ...(Array.isArray(resource.sources) ? resource.sources : []),
    ...(Array.isArray(platformData?.sources) ? platformData.sources : []),
    ...(Array.isArray(resource.platformScopes) ? resource.platformScopes : []),
  ].some((candidate) => asString(candidate)?.toLowerCase() === normalizedSource);
};

const hasVMwareScope = (resource: Resource, platformData: PlatformData | undefined): boolean =>
  resource.platformType === 'vmware-vsphere' ||
  hasSource(resource, platformData, 'vmware') ||
  hasSource(resource, platformData, 'vmware-vsphere') ||
  Boolean(resource.vmware || platformData?.vmware);

const getPreferredHostLabel = (resource: Resource): string =>
  getPreferredResourceHostname(resource) ||
  getPreferredInfrastructureDisplayName(resource) ||
  resource.id;

const getDockerContainerMetadataId = (
  resource: Resource,
  platformData: PlatformData | undefined,
): string | undefined => {
  const dockerPlatformData = platformData?.docker;
  const hostSourceId =
    asString(resource.docker?.hostSourceId) || asString(dockerPlatformData?.hostSourceId);
  const containerId =
    asString(resource.docker?.containerId) || asString(dockerPlatformData?.containerId);

  if (!hostSourceId || !containerId) return undefined;
  return `${hostSourceId}:container:${containerId}`;
};

const getMetadataTarget = (
  resource: Resource,
  resourceType: Resource['type'],
  platformData: PlatformData | undefined,
): Pick<DiscoveryConfig, 'metadataKind' | 'metadataId'> => {
  if (resourceType === 'app-container') {
    // Docker container web-interface metadata lives under the stable
    // host-plus-container-name guest key — the same identity the Workloads
    // drawer saves to and the highest-priority key the backend reads when
    // projecting unified customUrl. Saving under the runtime container ID
    // instead strands the URL below stable records in the lookup chain, so a
    // URL saved here never reaches the tables.
    const stableMetadataId = buildAppContainerMetadataId({
      dockerHostId:
        asString(resource.docker?.hostSourceId) || asString(platformData?.docker?.hostSourceId),
      name: asString(resource.name) ?? '',
    });
    if (stableMetadataId) {
      return {
        metadataKind: 'guest',
        metadataId: stableMetadataId,
      };
    }

    const dockerMetadataId = getDockerContainerMetadataId(resource, platformData);
    if (dockerMetadataId) {
      return {
        metadataKind: 'docker',
        metadataId: dockerMetadataId,
      };
    }
  }

  if (
    resourceType === 'pod' ||
    resourceType === 'k8s-deployment' ||
    resourceType === 'k8s-service'
  ) {
    const kubernetesPlatformData = platformData?.kubernetes;
    const stableMetadataId = buildKubernetesWorkloadMetadataId({
      kubernetesClusterId:
        asString(resource.kubernetes?.clusterId) || asString(kubernetesPlatformData?.clusterId),
      kind: resourceType,
      namespace:
        asString(resource.kubernetes?.namespace) || asString(kubernetesPlatformData?.namespace),
      name: asString(resource.name),
    });
    if (stableMetadataId) {
      return {
        metadataKind: 'guest',
        metadataId: stableMetadataId,
      };
    }
  }

  // Guest metadata is keyed by the canonical workload id shared with the
  // workloads surfaces (`instance:node:vmid` for PVE guests, resource id
  // otherwise — also the key v5 upgrades carry over). Saving under any other
  // id (unified resource hash, bare vmid, discovery resource id) strands the
  // URL where no table reads it.
  return {
    metadataKind: 'guest',
    metadataId: getCanonicalWorkloadIdForResource(resource),
  };
};

const getHostMetadataTarget = (
  resource: Resource,
  platformData: PlatformData | undefined,
  fallbackId: string,
): Pick<DiscoveryConfig, 'metadataKind' | 'metadataId'> => {
  const canonicalResourceType = canonicalDiscoveryResourceType(resource.type) || resource.type;
  const kubernetesPlatformData = platformData?.kubernetes;
  const kubernetesClusterId =
    asString(resource.kubernetes?.clusterId) || asString(kubernetesPlatformData?.clusterId);
  const kubernetesNodeName =
    asString(resource.kubernetes?.nodeName) ||
    asString((kubernetesPlatformData as { nodeName?: unknown } | undefined)?.nodeName) ||
    asString(resource.name);

  if (canonicalResourceType === 'docker-host') {
    return {
      metadataKind: 'docker-host',
      metadataId: getActionableDockerRuntimeIdFromResource(resource) || fallbackId,
    };
  }

  if (canonicalResourceType === 'k8s-node' && kubernetesClusterId && kubernetesNodeName) {
    return {
      metadataKind: 'agent',
      metadataId: `${kubernetesClusterId}:node:${kubernetesNodeName}`,
    };
  }

  if (canonicalResourceType === 'k8s-cluster' && kubernetesClusterId) {
    return { metadataKind: 'agent', metadataId: kubernetesClusterId };
  }

  if (canonicalResourceType === 'pbs' && asString(resource.pbs?.instanceId)) {
    return { metadataKind: 'agent', metadataId: asString(resource.pbs?.instanceId)! };
  }

  if (canonicalResourceType === 'pmg' && asString(resource.pmg?.instanceId)) {
    return { metadataKind: 'agent', metadataId: asString(resource.pmg?.instanceId)! };
  }

  const proxmoxSourceId =
    asString(resource.proxmox?.sourceId) || asString(platformData?.proxmox?.sourceId);
  if (canonicalResourceType === 'agent' && proxmoxSourceId) {
    return { metadataKind: 'agent', metadataId: proxmoxSourceId };
  }

  return {
    metadataKind: 'agent',
    metadataId: fallbackId,
  };
};

export const toDiscoveryConfig = (resource: Resource): DiscoveryConfig | null => {
  const platformData = resource.platformData as PlatformData | undefined;
  const explicitDiscoveryTarget = resource.discoveryTarget;
  const explicitDiscoveryAgentId = asString(
    (explicitDiscoveryTarget as { agentId?: unknown } | undefined)?.agentId,
  );
  const explicitDiscoveryResourceId = asString(
    (explicitDiscoveryTarget as { resourceId?: unknown } | undefined)?.resourceId,
  );

  if (
    explicitDiscoveryTarget &&
    explicitDiscoveryTarget.resourceType &&
    isDiscoveryLookupValue(explicitDiscoveryAgentId) &&
    isDiscoveryLookupValue(explicitDiscoveryResourceId)
  ) {
    const explicitResourceType = canonicalDiscoveryResourceType(
      explicitDiscoveryTarget.resourceType,
    );
    const resourceType = (() => {
      switch (explicitResourceType) {
        case 'agent':
          return 'agent';
        case 'vm':
        case 'system-container':
        case 'app-container':
        case 'pod':
          return explicitResourceType;
        default:
          return null;
      }
    })();

    if (resourceType) {
      const hostname = explicitDiscoveryTarget.hostname || getPreferredHostLabel(resource);
      const isHostDiscovery = isAgentDiscoveryResourceType(resourceType);
      const metadataTarget = isHostDiscovery
        ? getHostMetadataTarget(resource, platformData, explicitDiscoveryAgentId)
        : getMetadataTarget(resource, resourceType, platformData);
      const targetLabel = isHostDiscovery
        ? 'agent'
        : resourceType === 'app-container'
          ? 'container'
          : resourceType === 'pod'
            ? 'workload'
            : 'guest';
      return {
        resourceType,
        agentId: explicitDiscoveryAgentId,
        resourceId: explicitDiscoveryResourceId,
        hostname,
        metadataKind: metadataTarget.metadataKind,
        metadataId: metadataTarget.metadataId,
        targetLabel,
      };
    }
  }

  const dockerPlatformData = platformData?.docker;
  const kubernetesPlatformData = platformData?.kubernetes;
  const proxmoxVmid =
    asNumber(resource.proxmox?.vmid) ??
    asNumber(platformData?.proxmox?.vmid) ??
    asNumber((platformData as { vmid?: unknown } | undefined)?.vmid);
  const vmidResourceId =
    proxmoxVmid !== undefined && proxmoxVmid > 0 ? String(proxmoxVmid) : undefined;
  const proxmoxNodeName =
    asString(resource.proxmox?.nodeName) ||
    platformData?.proxmox?.nodeName ||
    asString((platformData as { nodeName?: unknown } | undefined)?.nodeName);
  const actionableAgentId = getActionableAgentIdFromResource(resource);
  const actionableDockerHostId = getActionableDockerRuntimeIdFromResource(resource);
  const actionableKubernetesId = getActionableKubernetesClusterIdFromResource(resource);
  const kubernetesAgentId =
    asString(resource.kubernetes?.agentId) ||
    asString(kubernetesPlatformData?.agentId) ||
    actionableKubernetesId ||
    getPreferredResourceClusterName(resource);
  const kubernetesResourceId =
    asString(resource.kubernetes?.podUid) ||
    asString(kubernetesPlatformData?.podUid) ||
    (() => {
      const namespace =
        asString(resource.kubernetes?.namespace) || asString(kubernetesPlatformData?.namespace);
      const podName =
        asString(resource.kubernetes?.podName) ||
        asString(kubernetesPlatformData?.podName) ||
        asString(resource.name);
      return namespace && podName ? `${namespace}/${podName}` : undefined;
    })();
  const agentLookupId =
    actionableDockerHostId ||
    actionableKubernetesId ||
    actionableAgentId ||
    asString(resource.pbs?.instanceId) ||
    asString(resource.pmg?.instanceId) ||
    proxmoxNodeName ||
    platformData?.agent?.hostname ||
    asString(dockerPlatformData?.hostname) ||
    getPreferredResourceHostname(resource) ||
    getPreferredInfrastructureDisplayName(resource) ||
    resource.platformId ||
    resource.id;
  const workloadAgentId =
    proxmoxNodeName ||
    actionableDockerHostId ||
    kubernetesAgentId ||
    actionableAgentId ||
    asString(resource.parentName) ||
    resource.parentId ||
    getPreferredResourceHostname(resource) ||
    resource.platformId ||
    resource.id;
  const hostname = getPreferredHostLabel(resource);
  const canonicalResourceType = canonicalDiscoveryResourceType(resource.type) || resource.type;
  const hostMetadataTarget = getHostMetadataTarget(resource, platformData, agentLookupId);

  switch (canonicalResourceType) {
    case 'agent':
    case 'docker-host':
    case 'pbs':
    case 'pmg':
    case 'k8s-cluster':
    case 'k8s-node':
      if (!isDiscoveryLookupValue(hostMetadataTarget.metadataId)) {
        return null;
      }
      return {
        resourceType: 'agent',
        agentId: agentLookupId,
        resourceId: agentLookupId,
        hostname,
        metadataKind: hostMetadataTarget.metadataKind,
        metadataId: hostMetadataTarget.metadataId,
        targetLabel: canonicalResourceType === 'agent' ? 'agent' : 'host',
      };
    case 'vm':
      if (hasVMwareScope(resource, platformData) && !(proxmoxNodeName && vmidResourceId)) {
        return null;
      }
      if (!isDiscoveryLookupValue(workloadAgentId)) {
        return null;
      }
      return {
        resourceType: 'vm',
        agentId: workloadAgentId,
        resourceId: vmidResourceId || resource.id,
        hostname,
        metadataKind: 'guest',
        metadataId: getCanonicalWorkloadIdForResource(resource),
        targetLabel: 'guest',
      };
    case 'system-container':
    case 'oci-container':
      if (!isDiscoveryLookupValue(workloadAgentId)) {
        return null;
      }
      return {
        resourceType: 'system-container',
        agentId: workloadAgentId,
        resourceId: vmidResourceId || resource.id,
        hostname,
        metadataKind: 'guest',
        metadataId: getCanonicalWorkloadIdForResource(resource),
        targetLabel: 'guest',
      };
    case 'app-container':
      if (!isDiscoveryLookupValue(workloadAgentId)) {
        return null;
      }
      return {
        resourceType: 'app-container',
        agentId: workloadAgentId,
        resourceId: asString(dockerPlatformData?.containerId) || resource.id,
        hostname,
        ...getMetadataTarget(resource, 'app-container', platformData),
        targetLabel: 'container',
      };
    case 'pod':
    case 'k8s-deployment':
    case 'k8s-service':
      if (!isDiscoveryLookupValue(workloadAgentId)) {
        return null;
      }
      return {
        resourceType: 'pod',
        agentId: workloadAgentId,
        resourceId: kubernetesResourceId || resource.id,
        hostname,
        ...getMetadataTarget(resource, canonicalResourceType, platformData),
        targetLabel: 'workload',
      };
    default:
      return null;
  }
};
