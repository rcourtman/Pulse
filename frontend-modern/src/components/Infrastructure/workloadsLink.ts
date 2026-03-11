import type { Resource } from '@/types/resource';
import { buildWorkloadsPath } from '@/routing/resourceLinks';
import {
  getActionableDockerRuntimeIdFromResource,
  hasDockerWorkloadsScope,
} from '@/utils/agentResources';
import { getPreferredWorkloadsAgentHint } from '@/utils/resourceIdentity';

type ProxmoxPlatformData = {
  nodeName?: string;
};

type AgentPlatformData = {
  hostname?: string;
};

type KubernetesPlatformData = {
  clusterId?: string;
  clusterName?: string;
  context?: string;
};

type PlatformData = {
  proxmox?: ProxmoxPlatformData;
  agent?: AgentPlatformData;
  kubernetes?: KubernetesPlatformData;
};

const firstNonEmpty = (values: Array<string | undefined | null>): string | undefined => {
  for (const value of values) {
    if (typeof value !== 'string') continue;
    const trimmed = value.trim();
    if (trimmed.length > 0) return trimmed;
  }
  return undefined;
};

const resolveKubernetesContext = (resource: Resource): string | undefined => {
  const platformData = resource.platformData as PlatformData | undefined;
  const kubernetes = platformData?.kubernetes;
  if (resource.type === 'k8s-cluster') {
    return firstNonEmpty([
      kubernetes?.clusterName,
      kubernetes?.context,
      kubernetes?.clusterId,
      resource.clusterId,
      resource.displayName,
      resource.name,
    ]);
  }
  if (resource.type === 'k8s-node') {
    return firstNonEmpty([
      kubernetes?.clusterName,
      kubernetes?.context,
      kubernetes?.clusterId,
      resource.clusterId,
    ]);
  }
  return undefined;
};

const resolveHostHint = (resource: Resource): string | undefined => {
  return getPreferredWorkloadsAgentHint(resource);
};

const resolveDockerWorkloadsHint = (resource: Resource): string | undefined =>
  getActionableDockerRuntimeIdFromResource(resource) || resolveHostHint(resource);

export const buildWorkloadsHref = (resource: Resource): string | null => {
  if (resource.type === 'k8s-cluster' || resource.type === 'k8s-node') {
    const context = resolveKubernetesContext(resource);
    return buildWorkloadsPath({ type: 'pod', context });
  }

  if (resource.type === 'docker-host') {
    const agent = resolveDockerWorkloadsHint(resource);
    return buildWorkloadsPath({ type: 'app-container', agent });
  }

  if (resource.type === 'agent') {
    const agent = resolveHostHint(resource);
    if (hasDockerWorkloadsScope(resource)) {
      return buildWorkloadsPath({ type: 'app-container', agent: resolveDockerWorkloadsHint(resource) });
    }
    return buildWorkloadsPath({ agent });
  }

  return null;
};
