import type { Resource } from '@/types/resource';
import type {
  CapacitySnapshot,
  NormalizedHealth,
  PlatformFamily,
  SourceDescriptor,
  StorageBackupPlatform,
  StorageMetricsTarget,
  StorageRecord,
} from './models';

export const asNumberOrNull = (value: unknown): number | null => {
  if (typeof value === 'number' && Number.isFinite(value)) return value;
  return null;
};

export const dedupe = <T>(values: T[]): T[] => Array.from(new Set(values));

const normalizeIdentityPart = (value: string | undefined | null): string =>
  (value || '').trim().toLowerCase();

export const getStringArray = (value: unknown): string[] =>
  Array.isArray(value)
    ? value.filter((item): item is string => typeof item === 'string' && item.trim().length > 0)
    : [];

export const canonicalStorageIdentityKey = (record: StorageRecord): string => {
  const platform = normalizeIdentityPart(String(record.source.platform || 'generic'));
  const location =
    normalizeIdentityPart(record.location?.label) ||
    normalizeIdentityPart(record.refs?.platformEntityId);
  const name = normalizeIdentityPart(record.name) || normalizeIdentityPart(record.id);
  const category = normalizeIdentityPart(record.category || 'other');

  return [platform, location || 'unknown-location', name || 'unknown-name', category].join('|');
};

export const resolveStoragePlatformFamily = (
  platform: StorageBackupPlatform,
): PlatformFamily => {
  const value = String(platform).toLowerCase();
  if (value.includes('kubernetes') || value.includes('docker')) return 'container';
  if (value.includes('cloud') || value === 'aws' || value === 'azure' || value === 'gcp') {
    return 'cloud';
  }
  if (value.includes('proxmox') || value.includes('vmware') || value.includes('hyperv')) {
    return 'virtualization';
  }
  if (value.includes('generic')) return 'generic';
  return 'onprem';
};

export const buildStorageSource = (
  platform: StorageBackupPlatform,
  adapterId: string,
): SourceDescriptor => ({
  platform,
  family: resolveStoragePlatformFamily(platform),
  adapterId,
  origin: 'resource',
});

export const buildStorageCapacity = (
  totalBytes: number | null,
  usedBytes: number | null,
  freeBytes: number | null,
  usagePercent: number | null,
): CapacitySnapshot => ({
  totalBytes,
  usedBytes,
  freeBytes,
  usagePercent,
});

export const metricsTargetForStorageResource = (
  resource: Resource,
): StorageMetricsTarget | undefined => {
  const resourceType = resource.metricsTarget?.resourceType;
  const resourceId = resource.metricsTarget?.resourceId;
  if (!resourceType || !resourceId) return undefined;

  return {
    resourceType,
    resourceId,
  };
};

const extractHealthTag = (tags: string[] | undefined): string | undefined => {
  if (!Array.isArray(tags)) return undefined;
  const healthTag = tags
    .map((tag) => tag.trim())
    .filter((tag) => tag.toLowerCase().startsWith('health:'))
    .at(-1);
  if (!healthTag) return undefined;
  return healthTag.slice('health:'.length).trim();
};

const normalizeHealthValue = (value: string | undefined): NormalizedHealth | undefined => {
  const normalized = (value || '').trim().toLowerCase();
  if (!normalized) return undefined;

  if (
    normalized === 'online' ||
    normalized === 'running' ||
    normalized === 'available' ||
    normalized === 'healthy' ||
    normalized === 'ok' ||
    normalized === 'optimal'
  ) {
    return 'healthy';
  }

  if (
    normalized === 'warning' ||
    normalized === 'warn' ||
    normalized === 'degraded' ||
    normalized === 'health_warn'
  ) {
    return 'warning';
  }

  if (
    normalized === 'critical' ||
    normalized === 'faulted' ||
    normalized === 'failed' ||
    normalized === 'error' ||
    normalized === 'unhealthy' ||
    normalized === 'health_crit' ||
    normalized === 'health_err'
  ) {
    return 'critical';
  }

  if (
    normalized === 'offline' ||
    normalized === 'stopped' ||
    normalized === 'down' ||
    normalized === 'unavailable'
  ) {
    return 'offline';
  }

  if (
    normalized.includes('fault') ||
    normalized.includes('fail') ||
    normalized.includes('critical') ||
    normalized.includes('error') ||
    normalized.includes('health_err') ||
    normalized.includes('health_crit') ||
    normalized.includes('unhealthy')
  ) {
    return 'critical';
  }

  if (normalized.includes('degraded') || normalized.includes('warn')) {
    return 'warning';
  }

  if (
    normalized.includes('offline') ||
    normalized.includes('stopped') ||
    normalized.includes('down')
  ) {
    return 'offline';
  }

  if (
    normalized.includes('healthy') ||
    normalized.includes('online') ||
    normalized.includes('available')
  ) {
    return 'healthy';
  }

  if (normalized === 'unknown') return 'unknown';
  return undefined;
};

export const normalizeStorageResourceHealth = (
  status: string | undefined,
  tags: string[] | undefined,
  incidentSeverity?: string,
): NormalizedHealth =>
  normalizeHealthValue(incidentSeverity) ||
  normalizeHealthValue(extractHealthTag(tags)) ||
  normalizeHealthValue(status) ||
  'unknown';
