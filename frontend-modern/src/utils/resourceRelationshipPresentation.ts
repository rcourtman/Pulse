import type { ResourceRelationship, ResourceRelationshipType } from '@/types/resource';

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
      const raw = String(type).trim().replace(/_/g, ' ');
      if (!raw) {
        return 'Related to';
      }
      return raw[0].toUpperCase() + raw.slice(1);
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
    confidence:
      relationship.confidence > 0 ? `${Math.round(relationship.confidence * 100)}%` : '',
    hasMetadata: Boolean(relationship.metadata && Object.keys(relationship.metadata).length > 0),
  };
}
