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
  getPreferredResourceDisplayName,
  getPreferredResourceHostname,
} from '@/utils/resourceIdentity';

export type DiscoveryConfig = {
  resourceType: DiscoveryResourceType;
  agentId: string;
  resourceId: string;
  hostname: string;
  metadataKind: 'guest' | 'agent';
  metadataId: string;
  targetLabel: string;
};

type ProxmoxPlatformData = {
  nodeName?: string;
  vmid?: number;
};

type DockerPlatformData = {
  containerId?: string;
  hostname?: string;
};

type KubernetesPlatformData = {
  agentId?: string;
  namespace?: string;
  podName?: string;
  podUid?: string;
};

type PlatformData = {
  agent?: {
    hostname?: string;
  };
  docker?: DockerPlatformData;
  kubernetes?: KubernetesPlatformData;
  proxmox?: ProxmoxPlatformData;
};

const asString = (value: unknown): string | undefined =>
  typeof value === 'string' && value.trim().length > 0 ? value.trim() : undefined;

const asNumber = (value: unknown): number | undefined =>
  typeof value === 'number' && Number.isFinite(value) ? value : undefined;

const getPreferredHostLabel = (resource: Resource): string =>
  getPreferredResourceHostname(resource) ||
  getPreferredResourceDisplayName(resource) ||
  resource.id;

export const toDiscoveryConfig = (resource: Resource): DiscoveryConfig | null => {
  const explicitDiscoveryTarget = resource.discoveryTarget;
  const explicitDiscoveryAgentId = asString(
    (explicitDiscoveryTarget as { agentId?: unknown } | undefined)?.agentId,
  );

  if (
    explicitDiscoveryTarget &&
    explicitDiscoveryTarget.resourceType &&
    explicitDiscoveryAgentId &&
    explicitDiscoveryTarget.resourceId
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
        resourceId: explicitDiscoveryTarget.resourceId,
        hostname,
        metadataKind: isHostDiscovery ? 'agent' : 'guest',
        metadataId: explicitDiscoveryTarget.resourceId,
        targetLabel,
      };
    }
  }

  const platformData = resource.platformData as PlatformData | undefined;
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
    proxmoxNodeName ||
    platformData?.agent?.hostname ||
    asString(dockerPlatformData?.hostname) ||
    getPreferredResourceHostname(resource) ||
    getPreferredResourceDisplayName(resource) ||
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

  switch (resource.type) {
    case 'agent':
    case 'docker-host':
    case 'pbs':
    case 'pmg':
    case 'k8s-cluster':
    case 'k8s-node':
    case 'truenas':
      return {
        resourceType: 'agent',
        agentId: agentLookupId,
        resourceId: agentLookupId,
        hostname,
        metadataKind: 'agent',
        metadataId: agentLookupId,
        targetLabel: 'agent',
      };
    case 'vm':
      return {
        resourceType: 'vm',
        agentId: workloadAgentId,
        resourceId: vmidResourceId || resource.id,
        hostname,
        metadataKind: 'guest',
        metadataId: resource.id,
        targetLabel: 'guest',
      };
    case 'system-container':
    case 'oci-container':
      return {
        resourceType: 'system-container',
        agentId: workloadAgentId,
        resourceId: vmidResourceId || resource.id,
        hostname,
        metadataKind: 'guest',
        metadataId: resource.id,
        targetLabel: 'guest',
      };
    case 'app-container':
      return {
        resourceType: 'app-container',
        agentId: workloadAgentId,
        resourceId: asString(dockerPlatformData?.containerId) || resource.id,
        hostname,
        metadataKind: 'guest',
        metadataId: resource.id,
        targetLabel: 'container',
      };
    case 'pod':
    case 'k8s-deployment':
    case 'k8s-service':
      return {
        resourceType: 'pod',
        agentId: workloadAgentId,
        resourceId: kubernetesResourceId || resource.id,
        hostname,
        metadataKind: 'guest',
        metadataId: resource.id,
        targetLabel: 'workload',
      };
    default:
      return null;
  }
};
