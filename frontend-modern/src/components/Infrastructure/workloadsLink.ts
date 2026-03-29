import type { Resource } from '@/types/resource';
import { buildWorkloadsPath } from '@/routing/resourceLinks';
import {
  getActionableDockerRuntimeIdFromResource,
  getPlatformDataRecord,
  hasDockerWorkloadsScope,
} from '@/utils/agentResources';
import {
  getPreferredInfrastructureDisplayName,
  getPreferredWorkloadsAgentHint,
  getPreferredResourceKubernetesContext,
} from '@/utils/resourceIdentity';
import { requiresGovernedResourceDisplay } from '@/types/resource';
import { normalizeSourcePlatformKey } from '@/utils/sourcePlatforms';

const firstNonEmpty = (values: Array<string | undefined | null>): string | undefined => {
  for (const value of values) {
    if (typeof value !== 'string') continue;
    const trimmed = value.trim();
    if (trimmed.length > 0) return trimmed;
  }
  return undefined;
};

const resolveKubernetesContext = (resource: Resource): string | undefined => {
  const kubernetesContext = getPreferredResourceKubernetesContext(resource);
  if (resource.type === 'k8s-cluster') {
    const displayLabel = requiresGovernedResourceDisplay(resource.policy)
      ? getPreferredInfrastructureDisplayName(resource)
      : resource.displayName?.trim() || resource.name?.trim() || undefined;
    return firstNonEmpty([
      kubernetesContext,
      displayLabel,
    ]);
  }
  if (resource.type === 'k8s-node') {
    return kubernetesContext;
  }
  return undefined;
};

const resolveHostHint = (resource: Resource): string | undefined => {
  return getPreferredWorkloadsAgentHint(resource);
};

const resolveDockerWorkloadsHint = (resource: Resource): string | undefined =>
  getActionableDockerRuntimeIdFromResource(resource) || resolveHostHint(resource);

const hasMergedSource = (resource: Resource, source: string): boolean => {
  const platformData = getPlatformDataRecord(resource);
  const mergedSources = Array.isArray(platformData?.sources) ? platformData.sources : [];
  return mergedSources.some((value) => normalizeSourcePlatformKey(value) === source);
};

export const buildWorkloadsHref = (resource: Resource): string | null => {
  if (resource.type === 'k8s-cluster' || resource.type === 'k8s-node') {
    const context = resolveKubernetesContext(resource);
    return buildWorkloadsPath({ type: 'pod', platform: 'kubernetes', context });
  }

  if (resource.type === 'docker-host') {
    const agent = resolveDockerWorkloadsHint(resource);
    return buildWorkloadsPath({ type: 'app-container', platform: 'docker', agent });
  }

  if (resource.type === 'truenas') {
    const agent = resolveHostHint(resource);
    return buildWorkloadsPath({ type: 'app-container', platform: 'truenas', agent });
  }

  if (resource.type === 'agent') {
    const agent = resolveHostHint(resource);
    if (resource.platformType === 'truenas' || hasMergedSource(resource, 'truenas')) {
      return buildWorkloadsPath({ type: 'app-container', platform: 'truenas', agent });
    }
    if (hasDockerWorkloadsScope(resource)) {
      return buildWorkloadsPath({
        type: 'app-container',
        platform: 'docker',
        agent: resolveDockerWorkloadsHint(resource),
      });
    }
    return buildWorkloadsPath({ agent });
  }

  return null;
};
