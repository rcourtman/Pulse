/**
 * Metrics Key Utilities
 *
 * Centralized helper for building namespaced metric keys to prevent ID collisions
 * across different resource types.
 */

export type MetricResourceKind = 'node' | 'vm' | 'container' | 'dockerHost' | 'dockerContainer';

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
 * Parse a metric key back into its components
 * Returns null if the key format is invalid
 */
export function parseMetricKey(key: string): { kind: MetricResourceKind; id: string } | null {
  const colonIndex = key.indexOf(':');
  if (colonIndex === -1) return null;

  const kind = key.slice(0, colonIndex) as MetricResourceKind;
  const id = key.slice(colonIndex + 1);

  return { kind, id };
}

/**
 * Extract the prefix from a metric resource kind
 * Used for bulk operations on a specific resource type
 */
export function getMetricKeyPrefix(kind: MetricResourceKind): string {
  return `${kind}:`;
}
