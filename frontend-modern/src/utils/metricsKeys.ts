/**
 * Metrics Key Utilities
 *
 * Centralized helper for building namespaced metric keys to prevent ID collisions
 * across different resource types.
 */

export type MetricResourceKind =
  | 'node'
  | 'vm'
  | 'container'
  | 'agent'
  | 'host'
  | 'dockerHost'
  | 'dockerContainer'
  | 'k8s';

function canonicalMetricKind(kind: MetricResourceKind): Exclude<MetricResourceKind, 'agent'> {
  return kind === 'agent' ? 'host' : kind;
}

/**
 * Build a namespaced metric key for a resource
 * Format: {kind}:{id}
 *
 * This prevents collisions if different resource types happen to share the same ID.
 */
export function buildMetricKey(kind: MetricResourceKind, id: string): string {
  return `${canonicalMetricKind(kind)}:${id}`;
}

/**
 * Extract the prefix from a metric resource kind
 * Used for bulk operations on a specific resource type
 */
export function getMetricKeyPrefix(kind: MetricResourceKind): string {
  return `${canonicalMetricKind(kind)}:`;
}
