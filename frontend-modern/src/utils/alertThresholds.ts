import type {
  AlertConfig,
  AlertThresholds,
  DockerThresholdConfig,
  HysteresisThreshold,
  RawOverrideConfig,
} from '@/types/alerts';

export type DisplayMetricType = 'cpu' | 'memory' | 'disk' | 'temperature' | 'diskTemperature';
export type DisplayMetricBarType = 'cpu' | 'memory' | 'disk' | 'generic';
export type AlertThresholdScope = 'guest' | 'node' | 'pbs' | 'host' | 'docker';
export type MetricSeverity = 'green' | 'yellow' | 'red';

export interface MetricDisplayThresholds {
  warning: number;
  critical: number;
}

export interface FactoryDockerDefaults {
  [key: string]: number;
  cpu: number;
  memory: number;
  disk: number;
  restartCount: number;
  restartWindow: number;
  memoryWarnPct: number;
  memoryCriticalPct: number;
  serviceWarnGapPercent: number;
  serviceCriticalGapPercent: number;
}

export const FACTORY_GUEST_DEFAULTS: Record<string, number> = {
  cpu: 80,
  memory: 85,
  disk: 90,
  diskRead: -1,
  diskWrite: -1,
  networkIn: -1,
  networkOut: -1,
};

export const FACTORY_NODE_DEFAULTS: Record<string, number> = {
  cpu: 80,
  memory: 85,
  disk: 90,
  temperature: 80,
};

export const FACTORY_PBS_DEFAULTS: Record<string, number> = {
  cpu: 80,
  memory: 85,
};

export const FACTORY_HOST_DEFAULTS: Record<string, number> = {
  cpu: 80,
  memory: 85,
  disk: 90,
  diskTemperature: 55,
};

export const FACTORY_DOCKER_DEFAULTS: FactoryDockerDefaults = {
  cpu: 80,
  memory: 85,
  disk: 85,
  restartCount: 3,
  restartWindow: 300,
  memoryWarnPct: 90,
  memoryCriticalPct: 95,
  serviceWarnGapPercent: 10,
  serviceCriticalGapPercent: 50,
};

const DEFAULT_GENERIC_THRESHOLDS: MetricDisplayThresholds = {
  warning: 75,
  critical: 90,
};

const DEFAULT_HYSTERESIS_MARGIN = 5;

const SCOPE_DEFAULTS: Record<AlertThresholdScope, Partial<Record<DisplayMetricType, number>>> = {
  guest: FACTORY_GUEST_DEFAULTS,
  node: FACTORY_NODE_DEFAULTS,
  pbs: FACTORY_PBS_DEFAULTS,
  host: FACTORY_HOST_DEFAULTS,
  docker: FACTORY_DOCKER_DEFAULTS,
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
): AlertThresholds | DockerThresholdConfig | undefined => {
  switch (scope) {
    case 'guest':
      return config?.guestDefaults;
    case 'node':
      return config?.nodeDefaults;
    case 'pbs':
      return config?.pbsDefaults;
    case 'host':
      return config?.hostDefaults;
    case 'docker':
      return config?.dockerDefaults;
  }
};

const getOverrideValue = (
  override: RawOverrideConfig | undefined,
  metric: DisplayMetricType,
): number | HysteresisThreshold | undefined => {
  const value = override?.[metric];
  if (typeof value === 'number' || isHysteresisThreshold(value)) {
    return value;
  }
  return undefined;
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
    const warning = clear === null
      ? Math.max(0, critical - margin)
      : Math.max(0, Math.min(clear, critical));
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

  const critical = SCOPE_DEFAULTS.guest[metric];
  if (!critical || critical <= 0) {
    return DEFAULT_GENERIC_THRESHOLDS;
  }

  return {
    warning: Math.max(0, critical - DEFAULT_HYSTERESIS_MARGIN),
    critical,
  };
};

export const resolveMetricDisplayThresholds = (
  config: AlertConfig | null,
  scope: AlertThresholdScope,
  metric: DisplayMetricType,
  resourceId?: string,
): MetricDisplayThresholds | null => {
  const margin = normalizeMargin(config?.hysteresisMargin);
  const scopeThresholds = getScopeThresholds(config, scope);
  const override = resourceId ? config?.overrides?.[resourceId] : undefined;
  const overrideValue = getOverrideValue(override, metric);
  const baseValue = scopeThresholds
    ? (scopeThresholds as Partial<Record<DisplayMetricType, number | HysteresisThreshold>>)[metric]
    : undefined;
  const fallbackCritical = SCOPE_DEFAULTS[scope][metric];
  return resolveThreshold(overrideValue ?? (typeof baseValue === 'number' || isHysteresisThreshold(baseValue) ? baseValue : undefined), fallbackCritical, margin);
};

export const getMetricSeverity = (
  value: number,
  thresholds: MetricDisplayThresholds | null | undefined,
): MetricSeverity => {
  if (!thresholds) {
    return 'green';
  }
  if (value >= thresholds.critical) {
    return 'red';
  }
  if (value >= thresholds.warning) {
    return 'yellow';
  }
  return 'green';
};

export const getMetricVisualSeverity = (
  value: number,
  metric: DisplayMetricBarType,
  thresholds: MetricDisplayThresholds | null | undefined,
): MetricSeverity => {
  const displayThresholds = thresholds ?? getDefaultMetricDisplayThresholds(metric);
  return getMetricSeverity(value, displayThresholds);
};
