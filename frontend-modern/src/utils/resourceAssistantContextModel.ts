import type { AIChatContext } from '@/stores/aiChat';
import type { Resource, ResourceDiscoveryTarget } from '@/types/resource';
import {
  getPreferredResourceDisplayName,
  getPrimaryResourceIdentity,
} from '@/utils/resourceIdentity';

const compact = (values: Array<string | undefined | null | false>): string[] =>
  values.filter((value): value is string => typeof value === 'string' && value.trim().length > 0);

export interface ResourceAssistantContextTarget {
  id: string;
  name: string;
  type: string;
  source: string;
  status?: string;
  technology?: string;
  parentName?: string;
  primaryIdentity?: string;
  discoveryTarget?: ResourceDiscoveryTarget | null;
}

export const buildResourceAssistantContextForTarget = (
  target: ResourceAssistantContextTarget,
): AIChatContext => {
  const displayName = target.name || target.id;
  const subjectParts = compact([target.type, target.status, target.technology]);
  const detailLines = compact([
    `Resource ID: ${target.id}`,
    target.primaryIdentity && target.primaryIdentity !== target.id
      ? `Primary identity: ${target.primaryIdentity}`
      : '',
    target.parentName ? `Parent: ${target.parentName}` : '',
    target.discoveryTarget
      ? `Discovery: ${target.discoveryTarget.resourceType}:${target.discoveryTarget.resourceId}`
      : '',
  ]);

  return {
    targetType: 'resource',
    targetId: target.id,
    context: {
      source: target.source,
      resourceId: target.id,
      resourceType: target.type,
      resourceStatus: target.status,
    },
    briefing: {
      sourceLabel: 'Pulse resource context',
      title: displayName,
      subject: subjectParts.join(' / '),
      statusLabel: 'Read-only context attached',
      detailLines,
      safetyNote: 'Approval required before any action.',
    },
    handoffResources: [
      {
        id: target.id,
        name: displayName,
        type: target.type,
        node: target.parentName,
      },
    ],
    handoffMetadata: {
      kind: 'resource_context',
    },
    autonomousMode: false,
  };
};

export const buildResourceAssistantContext = (resource: Resource): AIChatContext =>
  buildResourceAssistantContextForTarget({
    id: resource.id,
    name: getPreferredResourceDisplayName(resource),
    type: resource.type,
    source: 'resource-detail-drawer',
    status: resource.status,
    technology: resource.technology,
    parentName: resource.parentName,
    primaryIdentity: getPrimaryResourceIdentity(resource),
    discoveryTarget: resource.discoveryTarget,
  });
