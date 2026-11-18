import { batch, createSignal } from 'solid-js';
import { AlertsAPI } from '@/api/alerts';
import type { AlertConfig, ActivationState as ActivationStateType } from '@/types/alerts';
import type { Resource } from '@/types/resource';
import { getOrgID, type APIContextOptions } from '@/utils/apiClient';
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
const [isReadingConfig, setIsReadingConfig] = createSignal(false);
const [isWritingActivation, setIsWritingActivation] = createSignal(false);
const [lastError, setLastError] = createSignal<string | null>(null);
const isLoading = () => isReadingConfig() || isWritingActivation();
const detectionEnabled = () => config()?.enabled ?? true;
const notificationDeliveryEnabled = () => detectionEnabled() && activationState() === 'active';

const applyActivationState = (state: ActivationStateType | null) => {
  setActivationStateSignal(state);
};

let configRead: AbortController | undefined;
let activationWrite: AbortController | undefined;

// Refresh config from API
const refreshConfig = async (): Promise<void> => {
  configRead?.abort();
  const controller = new AbortController();
  configRead = controller;
  const { signal } = controller;
  const orgID = getOrgID();
  const ownsRead = () => !signal.aborted && getOrgID() === orgID;
  try {
    setIsReadingConfig(true);
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
      setIsReadingConfig(false);
    }
  }
};

const runActivationWrite = async (
  failureMessage: string,
  write: (options: APIContextOptions, ownsWrite: () => boolean) => Promise<boolean>,
): Promise<boolean> => {
  if (activationWrite) return false;
  const controller = new AbortController();
  activationWrite = controller;
  const orgID = getOrgID();
  const ownsWrite = () => activationWrite === controller && getOrgID() === orgID;
  try {
    batch(() => {
      setIsWritingActivation(true);
      setLastError(null);
    });
    if (!ownsWrite()) return false;
    // An unbound workspace still sends no org header. Never replay an explicit
    // confirmation through the client's automatic org or CSRF recovery.
    return await write(
      { expectedOrgID: orgID ?? undefined, retry: false, signal: controller.signal },
      ownsWrite,
    );
  } catch (error) {
    if (ownsWrite()) {
      logger.error(failureMessage, error);
      setLastError(error instanceof Error ? error.message : 'Unknown error');
    }
    return false;
  } finally {
    if (activationWrite === controller) {
      activationWrite = undefined;
      setIsWritingActivation(false);
    }
  }
};

// A completed outgoing activation must not cause a follow-up read for its
// replacement workspace.
const activate = () =>
  runActivationWrite('Failed to activate alerts:', async (options, ownsWrite) => {
    const result = await AlertsAPI.activate(options);
    if (!ownsWrite() || !result.success) return false;
    await refreshConfig();
    return ownsWrite();
  });

const updateActivationState = (state: ActivationStateType) =>
  runActivationWrite('Failed to update activation state:', async (options, ownsWrite) => {
    if (!config()) await refreshConfig();
    if (!ownsWrite()) return false;
    const current = config();
    if (!current) {
      throw new Error('Alert configuration is unavailable');
    }
    const updated: AlertConfig = { ...current, activationState: state };
    const result = await AlertsAPI.updateConfig(updated, options);
    if (!ownsWrite() || !result.success) return false;
    batch(() => {
      setConfig(updated);
      applyActivationState(state);
    });
    return true;
  });

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
  // Retire the outgoing browser operation, not the server-side write it may
  // already have received. Even a response that outlives abort loses ownership.
  activationWrite?.abort();
  activationWrite = undefined;
  batch(() => {
    setConfig(null);
    setGlobalAlertsDetectionEnabled(null);
    applyActivationState(null);
    setLastError(null);
    setIsReadingConfig(false);
    setIsWritingActivation(false);
  });
});

// Export the store
export const useAlertsActivation = () => ({
  // Signals
  config,
  activationState,
  detectionEnabled,
  notificationDeliveryEnabled,
  isLoading,
  isWritingActivation,
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
