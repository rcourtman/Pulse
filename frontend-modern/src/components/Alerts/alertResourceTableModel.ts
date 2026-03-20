import type { Resource as UnifiedResource, ResourcePolicy } from '@/types/resource';
import { getPreferredResourceDisplayName } from '@/utils/resourceIdentity';

const COLUMN_TOOLTIP_LOOKUP: Record<string, string> = {
  'cpu %': 'Percent CPU utilization allowed before an alert fires.',
  'memory %': 'Percent memory usage threshold for triggering alerts.',
  'disk %': 'Percent disk usage threshold for this resource.',
  'disk r mb/s': 'Maximum sustained disk read throughput before alerting.',
  'disk w mb/s': 'Maximum sustained disk write throughput before alerting.',
  'net in mb/s': 'Inbound network throughput threshold for alerts.',
  'net out mb/s': 'Outbound network throughput threshold for alerts.',
  'usage %': 'Storage capacity usage percentage that triggers an alert.',
  'temp °c': 'CPU temperature limit for node alerts.',
  'temperature °c': 'CPU temperature limit for node alerts.',
  temperature: 'CPU temperature limit for node alerts.',
  'disk temp °c': 'Individual disk temperature threshold for agents.',
  'restart count': 'Maximum container restarts within the evaluation window.',
  'restart window': 'Time window used to evaluate the restart count threshold.',
  'restart window (s)': 'Time window used to evaluate the restart count threshold.',
  'memory warn %': 'Warning threshold for container memory usage.',
  'memory critical %': 'Critical threshold for container memory usage.',
  'queue warn': 'Early warning when total mail queue exceeds this message count.',
  'queue crit': 'Critical alert requiring urgent action when queue reaches this size.',
  'deferred warn':
    'Early warning for messages stuck in deferred queue (waiting to retry delivery).',
  'deferred crit': 'Critical threshold for deferred messages indicating serious delivery problems.',
  'hold warn': 'Early warning when administratively held messages exceed this count.',
  'hold crit': 'Critical alert for held messages requiring immediate moderation attention.',
  'oldest warn (min)': 'Early warning when oldest queued message exceeds this age in minutes.',
  'oldest crit (min)': 'Critical alert when message queue age indicates delivery has stalled.',
  'spam warn': 'Early warning for spam messages accumulating in quarantine.',
  'spam crit': 'Critical spam quarantine level requiring urgent intervention.',
  'virus warn': 'Early warning for virus-positive messages in quarantine.',
  'virus crit': 'Critical virus quarantine threshold indicating potential outbreak.',
  'growth warn %': 'Early warning when quarantine growth rate exceeds this percentage.',
  'growth warn min': 'Minimum new messages required before growth percentage triggers warning.',
  'growth crit %': 'Critical quarantine growth rate requiring immediate investigation.',
  'growth crit min':
    'Minimum new messages required before growth percentage triggers critical alert.',
  'warning size (gib)': 'Total snapshot size in GiB that raises a warning.',
  'critical size (gib)': 'Total snapshot size in GiB that raises a critical alert.',
};

export const ALERT_RESOURCE_TABLE_SLIDER_METRICS = new Set([
  'cpu',
  'memory',
  'disk',
  'temperature',
  'diskTemperature',
]);

export type AlertResourceThresholdMap = Record<string, number | undefined>;

export interface AlertResourceTableResourceLike {
  id: string;
  name: string;
  displayName?: string;
  policy?: ResourcePolicy;
  aiSafeSummary?: string;
  rawName?: string;
  type?: string;
  thresholds?: AlertResourceThresholdMap;
  defaults?: AlertResourceThresholdMap;
  note?: string;
  [key: string]: unknown;
}

export function flattenAlertResourceTableResources<T extends AlertResourceTableResourceLike>(
  resources?: T[],
  groupedResources?: Record<string, T[]>,
): T[] {
  if (groupedResources) {
    return Object.values(groupedResources).flat();
  }
  return resources ?? [];
}

export function hasAlertResourceTableRows<T extends AlertResourceTableResourceLike>(
  resources?: T[],
  groupedResources?: Record<string, T[]>,
  globalDefaults?: AlertResourceThresholdMap,
): boolean {
  if (flattenAlertResourceTableResources(resources, groupedResources).length > 0) {
    return true;
  }
  if (groupedResources && Object.keys(groupedResources).length > 0) {
    return true;
  }
  return Boolean(globalDefaults);
}

export function hasCustomAlertResourceGlobalDefaults(
  globalDefaults?: AlertResourceThresholdMap,
  factoryDefaults?: AlertResourceThresholdMap,
): boolean {
  if (!globalDefaults || !factoryDefaults) {
    return false;
  }

  return Object.keys(factoryDefaults).some((key) => {
    const current = globalDefaults[key];
    const factory = factoryDefaults[key];
    return current !== undefined && current !== factory;
  });
}

export function normalizeAlertResourceMetricKey(column: string): string {
  const key = column.trim().toLowerCase();
  const mapped = new Map<string, string>([
    ['cpu %', 'cpu'],
    ['memory %', 'memory'],
    ['disk %', 'disk'],
    ['disk r mb/s', 'diskRead'],
    ['disk w mb/s', 'diskWrite'],
    ['net in mb/s', 'networkIn'],
    ['net out mb/s', 'networkOut'],
    ['usage %', 'usage'],
    ['temp °c', 'temperature'],
    ['temperature °c', 'temperature'],
    ['temperature', 'temperature'],
    ['restart count', 'restartCount'],
    ['restart window', 'restartWindow'],
    ['restart window (s)', 'restartWindow'],
    ['memory warn %', 'memoryWarnPct'],
    ['memory critical %', 'memoryCriticalPct'],
    ['warning size (gib)', 'warningSizeGiB'],
    ['critical size (gib)', 'criticalSizeGiB'],
    ['disk temp °c', 'diskTemperature'],
    ['backup', 'backup'],
    ['snapshot', 'snapshot'],
  ]).get(key);

  if (mapped) {
    return mapped;
  }

  return key
    .replace(' %', '')
    .replace(' °c', '')
    .replace(' mb/s', '')
    .replace('disk r', 'diskRead')
    .replace('disk w', 'diskWrite')
    .replace('net in', 'networkIn')
    .replace('net out', 'networkOut');
}

export function getAlertResourceMetricBounds(metric: string): { min: number; max: number } {
  if (metric === 'temperature' || metric === 'diskTemperature') {
    return { min: -1, max: 150 };
  }
  if (['diskRead', 'diskWrite', 'networkIn', 'networkOut'].includes(metric)) {
    return { min: -1, max: 10000 };
  }
  if (['cpu', 'memory', 'disk', 'usage', 'memoryWarnPct', 'memoryCriticalPct'].includes(metric)) {
    return { min: -1, max: 100 };
  }
  if (['warningSizeGiB', 'criticalSizeGiB'].includes(metric)) {
    return { min: -1, max: 100000 };
  }
  if (metric === 'restartCount') {
    return { min: -1, max: 50 };
  }
  if (metric === 'restartWindow') {
    return { min: -1, max: 86400 };
  }
  return { min: -1, max: 10000 };
}

export function getAlertResourceMetricStep(metric: string): string | number {
  if (['diskRead', 'diskWrite', 'networkIn', 'networkOut'].includes(metric)) {
    return 'any';
  }
  if (['warningSizeGiB', 'criticalSizeGiB'].includes(metric)) {
    return 'any';
  }
  return 1;
}

export function getAlertResourceEnabledDefault(metric: string): number {
  if (['diskRead', 'diskWrite', 'networkIn', 'networkOut'].includes(metric)) {
    return 100;
  }
  if (metric === 'temperature') {
    return 80;
  }
  if (metric === 'diskTemperature') {
    return 55;
  }
  if (metric === 'restartCount') {
    return 3;
  }
  if (metric === 'restartWindow') {
    return 300;
  }
  if (metric === 'memoryWarnPct') {
    return 90;
  }
  if (metric === 'memoryCriticalPct') {
    return 95;
  }
  return 80;
}

export function getAlertResourceMetricDelayOverride(
  metricDelaySeconds: Record<string, number> | undefined,
  metric: string,
): number | undefined {
  const normalized = metric.trim().toLowerCase();
  const value = metricDelaySeconds?.[normalized] ?? metricDelaySeconds?.[metric];
  if (typeof value !== 'number' || !Number.isFinite(value)) {
    return undefined;
  }
  return value;
}

export function getAlertResourceColumnHeaderTooltip(column: string): string | undefined {
  const normalized = column.trim().toLowerCase();
  return COLUMN_TOOLTIP_LOOKUP[column] ?? COLUMN_TOOLTIP_LOOKUP[normalized];
}

export function alertResourceSupportsMetric(
  resourceType: string | undefined,
  metric: string,
): boolean {
  if (!resourceType) return true;
  if (resourceType === 'node' && ['diskRead', 'diskWrite', 'networkIn', 'networkOut'].includes(metric)) {
    return false;
  }
  if (resourceType === 'pbs') {
    return ['cpu', 'memory'].includes(metric);
  }
  if (resourceType === 'storage') {
    return metric === 'usage';
  }
  if (resourceType === 'dockerContainer') {
    return [
      'cpu',
      'memory',
      'restartCount',
      'restartWindow',
      'memoryWarnPct',
      'memoryCriticalPct',
    ].includes(metric);
  }
  return true;
}

export function getAlertResourceLabel(resource: AlertResourceTableResourceLike): string {
  return getPreferredResourceDisplayName(resource as unknown as UnifiedResource);
}

function parseAlertMetricNumber(value: unknown): number | undefined {
  if (value === undefined || value === null) return undefined;
  if (typeof value === 'number') return value;
  const parsed = Number(value);
  return Number.isFinite(parsed) ? parsed : undefined;
}

export function getAlertResourceMetricDisplayValue(
  resource: AlertResourceTableResourceLike,
  metric: string,
  editingThresholds?: AlertResourceThresholdMap,
  isEditing = false,
): number {
  const extract = (source: Record<string, unknown> | undefined) => parseAlertMetricNumber(source?.[metric]);
  const defaults = resource.defaults as Record<string, unknown> | undefined;

  if (isEditing) {
    const edited = extract(editingThresholds as Record<string, unknown> | undefined);
    if (edited !== undefined) {
      return edited;
    }
    const fallback = extract(defaults);
    return fallback !== undefined ? fallback : 0;
  }

  const liveValue = extract(resource.thresholds as Record<string, unknown> | undefined);
  if (liveValue !== undefined) {
    return liveValue;
  }

  const fallback = extract(defaults);
  return fallback !== undefined ? fallback : 0;
}

export function isAlertResourceMetricOverridden(
  resource: AlertResourceTableResourceLike,
  metric: string,
): boolean {
  return resource.thresholds?.[metric] !== undefined && resource.thresholds?.[metric] !== null;
}

export function buildAlertResourceEditPayload(resource: AlertResourceTableResourceLike) {
  return {
    thresholds: resource.thresholds ? { ...resource.thresholds } : {},
    defaults: resource.defaults ? { ...resource.defaults } : {},
    note: typeof resource.note === 'string' ? resource.note : undefined,
  };
}
