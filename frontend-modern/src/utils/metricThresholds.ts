/**
 * Centralized metric threshold definitions and color utilities.
 *
 * ALL metric color/threshold logic MUST live here.
 * Do NOT define threshold constants or color- functions
 * in components — import from this module instead.
 */

import type {
  AlertConfig,
  AlertThresholds,
  DockerThresholdConfig,
  HysteresisThreshold,
  RawOverrideConfig,
} from '@/types/alerts';
import {
  FACTORY_AGENT_DEFAULTS,
  FACTORY_DISK_TEMP_BY_TYPE,
  FACTORY_DOCKER_DEFAULTS,
  FACTORY_GUEST_DEFAULTS,
  FACTORY_KUBERNETES_DEFAULTS,
  FACTORY_NODE_DEFAULTS,
  FACTORY_PBS_DEFAULTS,
  FACTORY_STORAGE_DEFAULT,
  FACTORY_TRUENAS_DEFAULTS,
  FACTORY_VMWARE_DEFAULTS,
} from '@/utils/alertThresholdDefaults';

export type MetricType = 'cpu' | 'memory' | 'disk';
export type DisplayMetricType = MetricType | 'temperature' | 'diskTemperature' | 'usage';
export type DisplayMetricBarType = MetricType | 'generic';
export type AlertThresholdScope =
  'guest' | 'node' | 'pbs' | 'agent' | 'docker' | 'storage' | 'kubernetes' | 'truenas' | 'vmware';

export interface MetricDisplayThresholds {
  warning: number;
  critical: number;
}

export const METRIC_THRESHOLDS: Record<MetricType, MetricDisplayThresholds> = {
  cpu: { warning: 80, critical: 90 },
  memory: { warning: 75, critical: 85 },
  disk: { warning: 80, critical: 90 },
};

export type MetricSeverity = 'normal' | 'warning' | 'critical';

const DEFAULT_GENERIC_THRESHOLDS: MetricDisplayThresholds = {
  warning: 75,
  critical: 90,
};

const DEFAULT_HYSTERESIS_MARGIN = 5;

const DISPLAY_METRIC_DEFAULT_SCOPES: Record<DisplayMetricType, AlertThresholdScope> = {
  cpu: 'guest',
  memory: 'guest',
  disk: 'guest',
  temperature: 'node',
  diskTemperature: 'agent',
  usage: 'storage',
};

const SCOPE_DEFAULTS: Record<
  Exclude<AlertThresholdScope, 'storage'>,
  Partial<Record<DisplayMetricType, number>>
> = {
  guest: FACTORY_GUEST_DEFAULTS,
  node: FACTORY_NODE_DEFAULTS,
  pbs: FACTORY_PBS_DEFAULTS,
  agent: FACTORY_AGENT_DEFAULTS,
  docker: FACTORY_DOCKER_DEFAULTS,
  kubernetes: FACTORY_KUBERNETES_DEFAULTS,
  truenas: FACTORY_TRUENAS_DEFAULTS,
  vmware: FACTORY_VMWARE_DEFAULTS,
};

const toFiniteNumber = (value: unknown): number | null => {
  if (typeof value !== 'number' || !Number.isFinite(value)) {
    return null;
  }
  return value;
};

const normalizeMargin = (value: number | undefined): number => {
  const numeric = toFiniteNumber(value);
  if (numeric === null) {
    return DEFAULT_HYSTERESIS_MARGIN;
  }
  return Math.max(0, numeric);
};

const isHysteresisThreshold = (value: unknown): value is HysteresisThreshold => {
  return value !== null && typeof value === 'object' && 'trigger' in value;
};

const getScopeThresholds = (
  config: AlertConfig | null,
  scope: AlertThresholdScope,
): AlertThresholds | DockerThresholdConfig | HysteresisThreshold | undefined => {
  switch (scope) {
    case 'guest':
      return config?.guestDefaults;
    case 'node':
      return config?.nodeDefaults;
    case 'pbs':
      return config?.pbsDefaults;
    case 'agent':
      return config?.agentDefaults;
    case 'docker':
      return config?.dockerDefaults;
    case 'storage':
      return config?.storageDefault;
    case 'kubernetes':
      return config?.kubernetesDefaults;
    case 'truenas':
      return config?.truenasDefaults;
    case 'vmware':
      return config?.vmwareDefaults;
  }
};

const normalizeResourceIds = (resourceIds?: string | string[]): string[] => {
  const ids = Array.isArray(resourceIds) ? resourceIds : [resourceIds];
  const result: string[] = [];
  const seen = new Set<string>();
  ids.forEach((value) => {
    const normalized = (value || '').trim();
    if (!normalized || seen.has(normalized)) return;
    seen.add(normalized);
    result.push(normalized);
  });
  return result;
};

const findOverride = (
  overrides: AlertConfig['overrides'] | undefined,
  resourceIds?: string | string[],
): RawOverrideConfig | undefined => {
  if (!overrides) {
    return undefined;
  }
  for (const resourceId of normalizeResourceIds(resourceIds)) {
    const override = overrides[resourceId];
    if (override) {
      return override;
    }
  }
  return undefined;
};

const getOverrideValue = (
  override: RawOverrideConfig | undefined,
  metric: DisplayMetricType,
): number | HysteresisThreshold | undefined => {
  const value = override?.[metric];
  if (typeof value === 'number' || isHysteresisThreshold(value)) {
    return value;
  }
  if (metric === 'disk') {
    const usageValue = override?.usage;
    if (typeof usageValue === 'number' || isHysteresisThreshold(usageValue)) {
      return usageValue;
    }
  }
  return undefined;
};

const getBaseThresholdValue = (
  thresholds: AlertThresholds | DockerThresholdConfig | HysteresisThreshold | undefined,
  metric: DisplayMetricType,
): number | HysteresisThreshold | undefined => {
  if (!thresholds) {
    return undefined;
  }
  if (isHysteresisThreshold(thresholds)) {
    return metric === 'disk' || metric === 'usage' ? thresholds : undefined;
  }
  const value = (thresholds as Partial<Record<DisplayMetricType, number | HysteresisThreshold>>)[
    metric
  ];
  if (typeof value === 'number' || isHysteresisThreshold(value)) {
    return value;
  }
  return undefined;
};

const getFallbackCritical = (
  scope: AlertThresholdScope,
  metric: DisplayMetricType,
): number | undefined => {
  if (scope === 'storage') {
    return metric === 'disk' || metric === 'usage' ? FACTORY_STORAGE_DEFAULT : undefined;
  }
  return SCOPE_DEFAULTS[scope][metric];
};

const resolveThreshold = (
  value: number | HysteresisThreshold | undefined,
  fallbackCritical: number | undefined,
  margin: number,
): MetricDisplayThresholds | null => {
  if (isHysteresisThreshold(value)) {
    const critical = toFiniteNumber(value.trigger);
    if (critical === null || critical <= 0) {
      return null;
    }
    const clear = toFiniteNumber(value.clear);
    const warning =
      clear === null ? Math.max(0, critical - margin) : Math.max(0, Math.min(clear, critical));
    return { warning, critical };
  }

  const numeric = toFiniteNumber(value);
  if (numeric !== null) {
    if (numeric <= 0) {
      return null;
    }
    return {
      warning: Math.max(0, numeric - margin),
      critical: numeric,
    };
  }

  if (!fallbackCritical || fallbackCritical <= 0) {
    return null;
  }

  return {
    warning: Math.max(0, fallbackCritical - margin),
    critical: fallbackCritical,
  };
};

export const getDefaultMetricDisplayThresholds = (
  metric: DisplayMetricBarType,
): MetricDisplayThresholds => {
  if (metric === 'generic') {
    return DEFAULT_GENERIC_THRESHOLDS;
  }

  return (
    resolveThreshold(undefined, SCOPE_DEFAULTS.guest[metric], DEFAULT_HYSTERESIS_MARGIN) ??
    METRIC_THRESHOLDS[metric]
  );
};

export const getDefaultDisplayMetricThresholds = (
  metric: DisplayMetricType,
  scope: AlertThresholdScope = DISPLAY_METRIC_DEFAULT_SCOPES[metric],
): MetricDisplayThresholds | null =>
  resolveThreshold(undefined, getFallbackCritical(scope, metric), DEFAULT_HYSTERESIS_MARGIN);

export const resolveMetricDisplayThresholds = (
  config: AlertConfig | null,
  scope: AlertThresholdScope,
  metric: DisplayMetricType,
  resourceIds?: string | string[],
): MetricDisplayThresholds | null => {
  const margin = normalizeMargin(config?.hysteresisMargin);
  const scopeThresholds = getScopeThresholds(config, scope);
  const override = findOverride(config?.overrides, resourceIds);
  const overrideValue = getOverrideValue(override, metric);
  const baseValue = getBaseThresholdValue(scopeThresholds, metric);
  return resolveThreshold(overrideValue ?? baseValue, getFallbackCritical(scope, metric), margin);
};

// The disk temperature policy for a disk type, with no host override: the
// type's diskTempByType entry, else the agent Disk Temp default. A switched-off
// Disk Temp default switches every type off.
const resolveDiskTemperatureTypePolicy = (
  config: AlertConfig | null,
  normalizedType: string,
  margin: number,
): MetricDisplayThresholds | null => {
  const baseValue = getBaseThresholdValue(config?.agentDefaults, 'diskTemperature');
  if (baseValue !== undefined && resolveThreshold(baseValue, undefined, margin) === null) {
    return null;
  }

  const byTypeFallback = normalizedType ? FACTORY_DISK_TEMP_BY_TYPE[normalizedType] : undefined;
  if (normalizedType) {
    const byType = config?.diskTempByType?.[normalizedType];
    if (isHysteresisThreshold(byType)) {
      return resolveThreshold(byType, byTypeFallback, margin);
    }
  }

  return resolveThreshold(
    baseValue,
    byTypeFallback ?? FACTORY_AGENT_DEFAULTS.diskTemperature,
    margin,
  );
};

/**
 * Resolve display thresholds for a physical disk's SMART temperature.
 * Mirrors the backend precedence: an explicit diskTemperature override on the
 * host (or inherited linked resource) wins, then the per-type map
 * (diskTempByType: nvme/sas/sata), then the global agent default. A
 * switched-off agent default switches every type off, as disk temperature
 * alerts do, and so does an override (or agent default) that switches the
 * host's alerts off. Null means disk temperature alerting is off for this
 * disk. `resourceIds` are the alert override keys of the machine whose agent
 * reports the disk, in the order the backend reads them.
 */
export const resolveDiskTemperatureDisplayThresholds = (
  config: AlertConfig | null,
  diskType: string | null | undefined,
  resourceIds?: string | string[],
): MetricDisplayThresholds | null => {
  const margin = normalizeMargin(config?.hysteresisMargin);
  const normalizedType = (diskType ?? '').trim().toLowerCase();

  const override = findOverride(config?.overrides, resourceIds);
  if (override?.disabled || config?.agentDefaults?.disabled === true) {
    return null;
  }
  const overrideValue = getOverrideValue(override, 'diskTemperature');
  if (overrideValue !== undefined) {
    return resolveThreshold(overrideValue, FACTORY_AGENT_DEFAULTS.diskTemperature, margin);
  }

  return resolveDiskTemperatureTypePolicy(config, normalizedType, margin);
};

/**
 * Resolve display thresholds for a TrueNAS disk's temperature by the tiers its
 * temperature alert resolves (backend `trueNASDiskTemperatureDefaultNoLock`
 * under `effectiveAlertPolicyNoLock`): the disk's own temperature override,
 * stored under its resource ID, then a TrueNAS-wide value saved under TrueNAS
 * Disks, then the disk temperature policy for its type. An override or a
 * TrueNAS Disks default that switches alerts off, or a value of 0 at either
 * explicit tier, means off (null). No host agent reports a TrueNAS disk, so
 * host overrides and the agent default's alert switch do not reach it; only
 * the agent Disk Temp value does, through the per-type tier.
 */
export const resolveTrueNASDiskTemperatureDisplayThresholds = (
  config: AlertConfig | null,
  diskType: string | null | undefined,
  resourceId: string | null | undefined,
): MetricDisplayThresholds | null => {
  const margin = normalizeMargin(config?.hysteresisMargin);

  const override = findOverride(config?.overrides, resourceId ?? undefined);
  if (override?.disabled || config?.truenasDiskDefaults?.disabled === true) {
    return null;
  }
  const overrideValue = getOverrideValue(override, 'temperature');
  if (overrideValue !== undefined) {
    return resolveThreshold(overrideValue, undefined, margin);
  }

  const trueNASWideValue = getBaseThresholdValue(config?.truenasDiskDefaults, 'temperature');
  if (trueNASWideValue !== undefined) {
    return resolveThreshold(trueNASWideValue, undefined, margin);
  }

  return resolveDiskTemperatureTypePolicy(config, (diskType ?? '').trim().toLowerCase(), margin);
};

const getFallbackSeverityThresholds = (metric: DisplayMetricType): MetricDisplayThresholds => {
  if (metric === 'cpu' || metric === 'memory' || metric === 'disk') {
    return METRIC_THRESHOLDS[metric];
  }
  return getDefaultDisplayMetricThresholds(metric) ?? DEFAULT_GENERIC_THRESHOLDS;
};

/** Determine severity level from a metric value and display thresholds. */
export function getMetricSeverity(
  value: number,
  metric: DisplayMetricType,
  thresholds?: MetricDisplayThresholds | null,
): MetricSeverity {
  const t = thresholds ?? getFallbackSeverityThresholds(metric);
  if (value >= t.critical) return 'critical';
  if (value >= t.warning) return 'warning';
  return 'normal';
}

// -- Tailwind background classes (progress bars) --

const BG_CLASSES: Record<MetricSeverity, string> = {
  critical: 'bg-metric-critical-bg dark:bg-metric-critical-bg',
  warning: 'bg-metric-warning-bg dark:bg-metric-warning-bg',
  normal: 'bg-metric-normal-bg dark:bg-metric-normal-bg',
};

export function getMetricColorClass(
  value: number,
  metric: DisplayMetricType,
  thresholds?: MetricDisplayThresholds | null,
): string {
  return BG_CLASSES[getMetricSeverity(value, metric, thresholds)];
}

// -- RGBA colors (canvas rendering, inline styles) --

const RGBA_COLORS: Record<MetricSeverity, string> = {
  critical: 'rgba(239, 68, 68, 0.6)',
  warning: 'rgba(234, 179, 8, 0.6)',
  normal: 'rgba(34, 197, 94, 0.6)',
};

export function getMetricColorRgba(
  value: number,
  metric: DisplayMetricType,
  thresholds?: MetricDisplayThresholds | null,
): string {
  return RGBA_COLORS[getMetricSeverity(value, metric, thresholds)];
}

// -- Hex colors (sparkline canvas) --

const HEX_COLORS: Record<MetricSeverity, string> = {
  critical: '#ef4444',
  warning: '#eab308',
  normal: '#22c55e',
};

export function getMetricColorHex(
  value: number,
  metric: DisplayMetricType,
  thresholds?: MetricDisplayThresholds | null,
): string {
  return HEX_COLORS[getMetricSeverity(value, metric, thresholds)];
}

// -- Text color classes (labels, percentage text) --

const TEXT_CLASSES: Record<MetricSeverity, string> = {
  critical: 'text-red-600 dark:text-red-400',
  warning: 'text-yellow-600 dark:text-yellow-400',
  normal: 'text-muted',
};

export function getMetricTextColorClass(
  value: number,
  metric: DisplayMetricType,
  thresholds?: MetricDisplayThresholds | null,
): string {
  return TEXT_CLASSES[getMetricSeverity(value, metric, thresholds)];
}
