import type { ResourceRelationship, ResourceRelationshipType } from '@/types/resource';
import { formatConfidencePercentage } from '@/utils/confidencePresentation';
import { humanizeToken } from '@/utils/textPresentation';

export interface ResourceRelationshipPresentation {
  typeLabel: string;
  direction: string;
  provenance: string;
  stateLabel: string;
  confidence: string;
  hasMetadata: boolean;
}

export function formatResourceRelationshipType(type: ResourceRelationshipType | string): string {
  switch (type) {
    case 'runs_on':
      return 'Runs on';
    case 'depends_on':
      return 'Depends on';
    case 'mounted_to':
      return 'Mounted to';
    case 'exposed_by':
      return 'Exposed by';
    case 'owned_by':
      return 'Owned by';
    default: {
      return humanizeToken(String(type), { fallback: 'Related to' });
    }
  }
}

export function describeResourceRelationship(
  relationship: ResourceRelationship,
): ResourceRelationshipPresentation {
  return {
    typeLabel: formatResourceRelationshipType(relationship.type),
    direction: `${relationship.sourceId.trim()} → ${relationship.targetId.trim()}`,
    provenance: relationship.discoverer.trim(),
    stateLabel: relationship.active ? 'Active' : 'Historical',
    confidence: relationship.confidence > 0 ? formatConfidencePercentage(relationship.confidence) : '',
    hasMetadata: Boolean(relationship.metadata && Object.keys(relationship.metadata).length > 0),
  };
}
