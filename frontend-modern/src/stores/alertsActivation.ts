import { batch, createSignal } from 'solid-js';
import { AlertsAPI } from '@/api/alerts';
import type { AlertConfig, ActivationState as ActivationStateType } from '@/types/alerts';
import type { Resource } from '@/types/resource';
import { getOrgID } from '@/utils/apiClient';
import { setGlobalAlertsDetectionEnabled } from '@/utils/alertsActivation';
import { FACTORY_NODE_DEFAULTS } from '@/utils/alertThresholdDefaults';
import { logger } from '@/utils/logger';
import {
  type AlertThresholdScope,
  type DisplayMetricType,
  resolveDiskTemperatureDisplayThresholds,
  resolveMetricDisplayThresholds,
  resolveTrueNASDiskTemperatureDisplayThresholds,
} from '@/utils/metricThresholds';
import { eventBus } from './events';

// Create signals for activation state
const [config, setConfig] = createSignal<AlertConfig | null>(null);
const [activationState, setActivationStateSignal] = createSignal<ActivationStateType | null>(null);
const [isLoading, setIsLoading] = createSignal(false);
const [lastError, setLastError] = createSignal<string | null>(null);
const detectionEnabled = () => config()?.enabled ?? true;
const notificationDeliveryEnabled = () => detectionEnabled() && activationState() === 'active';

const applyActivationState = (state: ActivationStateType | null) => {
  setActivationStateSignal(state);
};

const ensureConfigLoaded = async (): Promise<AlertConfig | null> => {
  let current = config();
  if (!current) {
    await refreshConfig();
    current = config();
  }
  return current;
};

let configRead: AbortController | undefined;

// Refresh config from API
const refreshConfig = async (): Promise<void> => {
  configRead?.abort();
  const controller = new AbortController();
  configRead = controller;
  const { signal } = controller;
  const orgID = getOrgID();
  const ownsRead = () => !signal.aborted && getOrgID() === orgID;
  try {
    setIsLoading(true);
    setLastError(null);
    const alertConfig = await AlertsAPI.getConfig({
      signal,
      cache: 'no-store',
      expectedOrgID: orgID ?? undefined,
      // An unbound bootstrap must keep the header absent, while still using
      // the API client's abort-aware response handling.
      retry: orgID !== null,
    });
    if (!ownsRead()) return;
    batch(() => {
      setConfig(alertConfig);
      applyActivationState(alertConfig.activationState || 'active');
      setGlobalAlertsDetectionEnabled(alertConfig.enabled);
    });
  } catch (error) {
    if (!ownsRead()) return;
    logger.error('Failed to fetch alert config:', error);
    setLastError(error instanceof Error ? error.message : 'Unknown error');
  } finally {
    if (configRead === controller) {
      configRead = undefined;
      setIsLoading(false);
    }
  }
};

// Activate alert notifications
const activate = async (): Promise<boolean> => {
  try {
    setIsLoading(true);
    setLastError(null);
    const result = await AlertsAPI.activate();

    if (result.success) {
      // Refresh config to get updated state
      await refreshConfig();
      return true;
    }
    return false;
  } catch (error) {
    logger.error('Failed to activate alerts:', error);
    setLastError(error instanceof Error ? error.message : 'Unknown error');
    return false;
  } finally {
    setIsLoading(false);
  }
};

const updateActivationState = async (state: ActivationStateType): Promise<boolean> => {
  try {
    setIsLoading(true);
    setLastError(null);
    const current = await ensureConfigLoaded();
    if (!current) {
      throw new Error('Alert configuration is unavailable');
    }
    const updated: AlertConfig = { ...current, activationState: state };
    const result = await AlertsAPI.updateConfig(updated);
    if (!result.success) {
      return false;
    }
    setConfig(updated);
    applyActivationState(state);
    return true;
  } catch (error) {
    logger.error('Failed to update activation state:', error);
    setLastError(error instanceof Error ? error.message : 'Unknown error');
    return false;
  } finally {
    setIsLoading(false);
  }
};

const deactivate = async (): Promise<boolean> => updateActivationState('pending_review');

const snooze = async (): Promise<boolean> => updateActivationState('snoozed');

// Check if past observation window
const isPastObservationWindow = (): boolean => {
  const cfg = config();
  if (!cfg || !cfg.activationTime || !cfg.observationWindowHours) {
    return false;
  }

  const activationTime = new Date(cfg.activationTime);
  const windowMs = cfg.observationWindowHours * 60 * 60 * 1000;
  const expiryTime = activationTime.getTime() + windowMs;

  return Date.now() > expiryTime;
};

// Get backup indicator thresholds from config
const getBackupThresholds = (): { freshHours: number; staleHours: number } => {
  const cfg = config();
  return {
    freshHours: cfg?.backupDefaults?.freshHours ?? 24,
    staleHours: cfg?.backupDefaults?.staleHours ?? 72,
  };
};

// Get temperature threshold from config (for display coloring)
const getTemperatureThreshold = (): number => {
  return getMetricThresholds('node', 'temperature')?.critical ?? FACTORY_NODE_DEFAULTS.temperature;
};

const getMetricThresholds = (
  scope: AlertThresholdScope,
  metric: DisplayMetricType,
  resourceIds?: string | string[],
  guestTags?: readonly string[] | null,
) => {
  return resolveMetricDisplayThresholds(config(), scope, metric, resourceIds, guestTags);
};

// Per-type disk SMART temperature thresholds (for display coloring).
const getDiskTemperatureThresholds = (
  diskType: string | null | undefined,
  resourceIds?: string | string[],
) => {
  return resolveDiskTemperatureDisplayThresholds(config(), diskType, resourceIds);
};

// A TrueNAS disk's temperature thresholds, resolved by the tiers its
// temperature alert uses: its own override, the TrueNAS-wide value, then the
// per-type policy.
const getTrueNASDiskTemperatureThresholds = (disk: Pick<Resource, 'id' | 'physicalDisk'>) => {
  return resolveTrueNASDiskTemperatureDisplayThresholds(
    config(),
    disk.physicalDisk?.diskType,
    disk.id,
  );
};

eventBus.on('org_switched', () => {
  configRead?.abort();
  configRead = undefined;
  setConfig(null);
  setGlobalAlertsDetectionEnabled(null);
  applyActivationState(null);
  setLastError(null);
  setIsLoading(false);
});

// Export the store
export const useAlertsActivation = () => ({
  // Signals
  config,
  activationState,
  detectionEnabled,
  notificationDeliveryEnabled,
  isLoading,
  lastError,

  // Computed
  isPastObservationWindow,
  getBackupThresholds,
  getTemperatureThreshold,
  getMetricThresholds,
  getDiskTemperatureThresholds,
  getTrueNASDiskTemperatureThresholds,

  // Actions
  refreshConfig,
  activate,
  deactivate,
  snooze,
});
