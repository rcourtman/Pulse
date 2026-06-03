import type { Resource } from '@/types/resource';
import { normalizeSourcePlatformKey } from '@/utils/sourcePlatforms';

export type ResourceSourceHealth = 'connected' | 'impaired' | 'unknown';

type SourceStatusEntry = {
  status?: unknown;
  lastSeen?: unknown;
  error?: unknown;
};

const CONNECTED_SOURCE_STATUSES = new Set(['online', 'running', 'healthy', 'connected', 'ok']);

const asRecord = (value: unknown): Record<string, unknown> | undefined =>
  value && typeof value === 'object' ? (value as Record<string, unknown>) : undefined;

const normalizeStatus = (value: unknown): string =>
  typeof value === 'string' ? value.trim().toLowerCase() : '';

export const getResourceSourceStatus = (
  resource: Resource,
  source: string,
): SourceStatusEntry | undefined => {
  const normalizedSource = normalizeSourcePlatformKey(source) || source.trim().toLowerCase();
  if (!normalizedSource) return undefined;

  const sourceStatus = asRecord(resource.platformData?.sourceStatus);
  if (!sourceStatus) return undefined;

  for (const [key, value] of Object.entries(sourceStatus)) {
    const normalizedKey = normalizeSourcePlatformKey(key) || key.trim().toLowerCase();
    if (normalizedKey !== normalizedSource) continue;
    return asRecord(value) as SourceStatusEntry | undefined;
  }

  return undefined;
};

export const getResourceSourceHealth = (
  resource: Resource,
  source: string,
): ResourceSourceHealth => {
  const status = getResourceSourceStatus(resource, source);
  if (!status) return 'unknown';

  const normalized = normalizeStatus(status.status);
  if (CONNECTED_SOURCE_STATUSES.has(normalized)) return 'connected';
  return 'impaired';
};

export const hasImpairedResourceSource = (resource: Resource, source: string): boolean =>
  getResourceSourceHealth(resource, source) === 'impaired';
