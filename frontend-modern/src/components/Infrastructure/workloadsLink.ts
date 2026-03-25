import type { Resource } from '@/types/resource';
import { buildWorkloadsPath } from '@/routing/resourceLinks';
import {
  getActionableDockerRuntimeIdFromResource,
  hasDockerWorkloadsScope,
} from '@/utils/agentResources';
import {
  getPreferredInfrastructureDisplayName,
  getPreferredWorkloadsAgentHint,
  getPreferredResourceKubernetesContext,
} from '@/utils/resourceIdentity';
import { requiresGovernedResourceDisplay } from '@/types/resource';

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
