import type { ResourceChange } from '@/types/resource';

export function formatResourceChangeKind(kind: ResourceChange['kind']): string {
  switch (kind) {
    case 'state_transition':
      return 'State transition';
    case 'restart':
      return 'Restart';
    case 'config_update':
      return 'Config update';
    case 'metric_anomaly':
      return 'Metric anomaly';
    case 'relationship_change':
      return 'Relationship change';
    case 'capability_change':
      return 'Capability change';
    default:
      return String(kind).replace(/_/g, ' ');
  }
}

export function formatResourceChangeHeadline(change: ResourceChange): string {
  if (change.kind === 'state_transition' && change.from && change.to) {
    return `${formatResourceChangeKind(change.kind)}: ${change.from} → ${change.to}`;
  }
  if (change.kind === 'restart' && change.from && change.to) {
    return `${formatResourceChangeKind(change.kind)}: ${change.from} → ${change.to}`;
  }
  if (change.reason) {
    return `${formatResourceChangeKind(change.kind)}: ${change.reason}`;
  }
  return `${formatResourceChangeKind(change.kind)}: ${change.resourceId}`;
}
