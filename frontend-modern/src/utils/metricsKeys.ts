/**
 * Metrics Key Utilities
 *
 * Centralized helper for building namespaced metric keys to prevent ID collisions
 * across different resource types.
 */

import type { Resource } from '@/types/resource';

export type MetricResourceKind =
  | 'node'
  | 'vm'
  | 'container'
  | 'agent'
  | 'dockerHost'
  | 'dockerContainer'
  | 'k8s';

/**
 * Build a namespaced metric key for a resource
 * Format: {kind}:{id}
 *
 * This prevents collisions if different resource types happen to share the same ID.
 */
export function buildMetricKey(kind: MetricResourceKind, id: string): string {
  return `${kind}:${id}`;
}

/**
 * Extract the prefix from a metric resource kind
 * Used for bulk operations on a specific resource type
 */
export function getMetricKeyPrefix(kind: MetricResourceKind): string {
  return `${kind}:`;
}

function toMetricResourceKind(
  resource: Pick<Resource, 'type' | 'metricsTarget'>,
): MetricResourceKind {
  const targetType = resource.metricsTarget?.resourceType;
  switch (targetType) {
    case 'vm':
      return 'vm';
    case 'system-container':
    case 'oci-container':
    case 'app-container':
      return 'container';
    case 'docker-host':
      return 'dockerHost';
    case 'k8s-cluster':
    case 'k8s-node':
    case 'pod':
      return 'k8s';
    case 'agent':
      return 'agent';
  }

  switch (resource.type) {
    case 'docker-host':
      return 'dockerHost';
    case 'k8s-cluster':
    case 'k8s-node':
    case 'pod':
      return 'k8s';
    case 'vm':
      return 'vm';
    case 'system-container':
    case 'oci-container':
    case 'app-container':
      return 'container';
    default:
      return 'agent';
  }
}

export function buildMetricKeyForUnifiedResource(
  resource: Pick<Resource, 'id' | 'type' | 'metricsTarget'>,
): string {
  const id = resource.metricsTarget?.resourceId || resource.id;
  return buildMetricKey(toMetricResourceKind(resource), id);
}
