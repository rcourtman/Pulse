import type { EmailConfig } from '@/api/notifications';
import type { Resource, ResourceType } from '@/types/resource';
import type { RawOverrideConfig, HysteresisThreshold } from '@/types/alerts';
import { getResourceTypeLabel } from '@/utils/resourceTypePresentation';
import {
  MAX_ALERTS_MIN,
  MAX_ALERTS_MAX,
  MAX_ALERTS_DEFAULT,
  COOLDOWN_DEFAULT_MINUTES,
  GROUPING_WINDOW_DEFAULT_MINUTES,
} from './types';
import { unwrap } from 'solid-js/store';
import type {
  QuietHoursConfig,
  CooldownConfig,
  GroupingConfig,
  UIAppriseConfig,
  UIEmailConfig,
  EscalationConfig,
} from './types';

export const clampMaxAlertsPerHour = (value?: number): number => {
  const numericValue = typeof value === 'number' ? value : Number.NaN;
  if (!Number.isFinite(numericValue)) {
    return MAX_ALERTS_MIN;
  }
  return Math.min(MAX_ALERTS_MAX, Math.max(MAX_ALERTS_MIN, numericValue));
};

export const fallbackMaxAlertsPerHour = (value?: number): number => {
  const numericValue = typeof value === 'number' ? value : Number.NaN;
  if (!Number.isFinite(numericValue) || numericValue <= 0) {
    return MAX_ALERTS_DEFAULT;
  }
  return clampMaxAlertsPerHour(numericValue);
};

export const getLocalTimezone = () => Intl.DateTimeFormat().resolvedOptions().timeZone || 'UTC';

export const createDefaultQuietHours = (): QuietHoursConfig => ({
  enabled: false,
  start: '22:00',
  end: '08:00',
  timezone: getLocalTimezone(),
  days: {
    monday: true,
    tuesday: true,
    wednesday: true,
    thursday: true,
    friday: true,
    saturday: false,
    sunday: false,
  },
  suppress: {
    performance: false,
    storage: false,
    offline: false,
  },
});

export const createDefaultCooldown = (): CooldownConfig => ({
  enabled: true,
  minutes: COOLDOWN_DEFAULT_MINUTES,
  maxAlerts: MAX_ALERTS_DEFAULT,
});

export const createDefaultGrouping = (): GroupingConfig => ({
  enabled: true,
  window: GROUPING_WINDOW_DEFAULT_MINUTES,
  byNode: true,
  byGuest: false,
});

export const createDefaultResolveNotifications = (): boolean => true;

export const createDefaultAppriseConfig = (): UIAppriseConfig => ({
  enabled: false,
  mode: 'cli',
  targetsText: '',
  cliPath: 'apprise',
  timeoutSeconds: 15,
  serverUrl: '',
  configKey: '',
  apiKey: '',
  apiKeyHeader: 'X-API-KEY',
  skipTlsVerify: false,
});

export const createDefaultEmailConfig = (): UIEmailConfig => ({
  enabled: false,
  provider: '',
  server: '',
  port: 587,
  username: '',
  password: '',
  from: '',
  to: [],
  tls: true,
  startTLS: false,
  replyTo: '',
  maxRetries: 3,
  retryDelay: 5,
  rateLimit: 60,
});

export const readStringValue = (value: unknown, fallback = ''): string =>
  typeof value === 'string' ? value : fallback;

export const readBooleanValue = (value: unknown, fallback = false): boolean =>
  typeof value === 'boolean' ? value : fallback;

export const readNumberValue = (value: unknown, fallback: number): number =>
  typeof value === 'number' && Number.isFinite(value) ? value : fallback;

export const readStringArrayValue = (value: unknown): string[] =>
  Array.isArray(value) ? value.filter((entry): entry is string => typeof entry === 'string') : [];

export const normalizeEmailConfigFromAPI = (
  value: Partial<EmailConfig> | null | undefined,
): UIEmailConfig => {
  const defaults = createDefaultEmailConfig();

  return {
    ...defaults,
    enabled: readBooleanValue(value?.enabled, defaults.enabled),
    provider: readStringValue(value?.provider, defaults.provider),
    server: readStringValue(value?.server, defaults.server),
    port: readNumberValue(value?.port, defaults.port),
    username: readStringValue(value?.username, defaults.username),
    password: readStringValue(value?.password, defaults.password),
    from: readStringValue(value?.from, defaults.from),
    to: readStringArrayValue(value?.to),
    tls: readBooleanValue(value?.tls, defaults.tls),
    startTLS: readBooleanValue(value?.startTLS, defaults.startTLS),
    rateLimit: readNumberValue(value?.rateLimit, defaults.rateLimit),
  };
};

export const parseAppriseTargets = (value: string): string[] =>
  value
    .split(/\r?\n|,/)
    .map((entry) => entry.trim())
    .filter((entry, index, arr) => entry.length > 0 && arr.indexOf(entry) === index);

export const formatAppriseTargets = (targets: string[] | undefined | null): string =>
  targets && targets.length > 0 ? targets.join('\n') : '';

export const normalizeMetricDelayMap = (
  input: Record<string, Record<string, number>> | undefined | null,
): Record<string, Record<string, number>> => {
  if (!input) return {};
  const normalized: Record<string, Record<string, number>> = {};

  Object.entries(input).forEach(([rawType, metrics]) => {
    if (!metrics) return;
    const typeKey = rawType.trim().toLowerCase();
    if (!typeKey) return;

    const entries: Record<string, number> = {};
    Object.entries(metrics).forEach(([rawMetric, value]) => {
      if (typeof value !== 'number' || Number.isNaN(value) || value < 0) return;
      const metricKey = rawMetric.trim().toLowerCase();
      if (!metricKey) return;
      entries[metricKey] = Math.round(value);
    });

    if (Object.keys(entries).length > 0) {
      normalized[typeKey] = entries;
    }
  });

  return normalized;
};

export const createDefaultEscalation = (): EscalationConfig => ({
  enabled: false,
  levels: [],
});

export const getTriggerValue = (
  threshold: number | boolean | HysteresisThreshold | undefined,
): number => {
  if (typeof threshold === 'number') {
    return threshold; // Legacy format
  }
  if (typeof threshold === 'boolean') {
    return 0;
  }
  if (threshold && typeof threshold === 'object' && 'trigger' in threshold) {
    return threshold.trigger; // New hysteresis format
  }
  return 0; // Default fallback
};

export const extractTriggerValues = (thresholds: RawOverrideConfig): Record<string, number> => {
  const result: Record<string, number> = {};
  Object.entries(thresholds).forEach(([key, value]) => {
    // Skip non-threshold fields
    if (
      key === 'disabled' ||
      key === 'disableConnectivity' ||
      key === 'poweredOffSeverity' ||
      key === 'note' ||
      key === 'backup' ||
      key === 'snapshot'
    )
      return;
    if (typeof value === 'string') return;
    result[key] = getTriggerValue(value as any);
  });
  return result;
};

export const DEFAULT_DELAY_SECONDS = 5;

/**
 * Maps a unified resource type to the display string used in alerts.
 * Exported for testing.
 */
export function unifiedTypeToAlertDisplayType(type: ResourceType): string {
  return getResourceTypeLabel(type) || type;
}

/**
 * Maps raw backend alert type strings to human-readable display labels.
 * Falls back to title-casing with hyphens replaced by spaces for unknown types.
 */
export function alertTypeDisplayLabel(type: string): string {
  switch (type) {
    // Standard metric types
    case 'cpu':
      return 'CPU';
    case 'memory':
      return 'Memory';
    case 'disk':
    case 'disk-usage':
      return 'Disk';
    case 'usage':
      return 'Usage';
    case 'diskRead':
      return 'Disk Read';
    case 'diskWrite':
      return 'Disk Write';
    case 'networkIn':
      return 'Network In';
    case 'networkOut':
      return 'Network Out';
    case 'network':
      return 'Network';
    case 'io':
      return 'I/O';
    case 'swap':
      return 'Swap';
    case 'load':
      return 'Load';
    case 'temperature':
    case 'disk_temperature':
    case 'diskTemperature':
      return 'Temperature';

    // Container/Docker alert types
    case 'docker-container-oom-kill':
      return 'OOM Kill';
    case 'docker-container-restart-loop':
      return 'Restart Loop';
    case 'docker-container-health':
      return 'Container Health';
    case 'docker-container-state':
      return 'Container State';
    case 'docker-container-memory-limit':
      return 'Memory Limit';
    case 'docker-container-cpu':
      return 'Container CPU';
    case 'docker-container-disk':
      return 'Container Disk';
    case 'docker-container-update':
      return 'Update Available';
    case 'docker-service-health':
      return 'Service Health';
    case 'docker-host-offline':
      return 'Host Offline';

    // Infrastructure alert types
    case 'host-offline':
      return 'Host Offline';
    case 'offline':
      return 'Offline';
    case 'powered-off':
      return 'Powered Off';
    case 'connectivity':
      return 'Connectivity';
    case 'node':
      return 'Node';

    // Storage/backup alert types
    case 'snapshot-age':
      return 'Snapshot Age';
    case 'backup-age':
      return 'Backup Age';
    case 'zfs-pool-state':
      return 'Pool State';
    case 'zfs-pool-errors':
      return 'Pool Errors';
    case 'zfs-device':
      return 'ZFS Device';
    case 'disk-health':
      return 'Disk Health';
    case 'disk-wearout':
      return 'Disk Wearout';
    case 'raid':
      return 'RAID';

    // Other
    case 'pbs':
      return 'PBS';
    case 'message-age':
      return 'Message Age';

    default:
      // Title-case with hyphens/underscores replaced by spaces
      return type.replace(/[-_]/g, ' ').replace(/\b\w/g, (c) => c.toUpperCase());
  }
}

// Shared adapters for guest resource details used by ThresholdsTable.
export const platformData = (r: Resource): Record<string, unknown> | undefined =>
  r.platformData ? (unwrap(r.platformData) as Record<string, unknown>) : undefined;

export const guessNumericId = (value: string): number => {
  const match = value.match(/(\d+)\s*$/);
  return match ? parseInt(match[1], 10) : 0;
};
