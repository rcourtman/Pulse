import { createSignal } from 'solid-js';
import { AlertsAPI } from '@/api/alerts';
import type { AlertConfig, ActivationState as ActivationStateType } from '@/types/alerts';
import type { Alert } from '@/types/api';
import { setGlobalActivationState } from '@/utils/alertsActivation';
import { logger } from '@/utils/logger';

// Create signals for activation state
const [config, setConfig] = createSignal<AlertConfig | null>(null);
const [activationState, setActivationStateSignal] = createSignal<ActivationStateType | null>(null);
const [isLoading, setIsLoading] = createSignal(false);
const [activeAlerts, setActiveAlerts] = createSignal<Alert[]>([]);
const [lastError, setLastError] = createSignal<string | null>(null);

const applyActivationState = (state: ActivationStateType | null) => {
  setActivationStateSignal(state);
  setGlobalActivationState(state);
};

const ensureConfigLoaded = async (): Promise<AlertConfig | null> => {
  let current = config();
  if (!current) {
    await refreshConfig();
    current = config();
  }
  return current;
};

// Refresh config from API
const refreshConfig = async (): Promise<void> => {
  try {
    setIsLoading(true);
    setLastError(null);
    const alertConfig = await AlertsAPI.getConfig();
    setConfig(alertConfig);
    applyActivationState(alertConfig.activationState || 'active');
  } catch (error) {
    logger.error('Failed to fetch alert config:', error);
    setLastError(error instanceof Error ? error.message : 'Unknown error');
  } finally {
    setIsLoading(false);
  }
};

// Fetch active alerts (for violation count)
const refreshActiveAlerts = async (): Promise<void> => {
  try {
    const alerts = await AlertsAPI.getActive();
    setActiveAlerts(alerts);
  } catch (error) {
    logger.error('Failed to fetch active alerts:', error);
    // Don't set error state for this - it's not critical
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
  const cfg = config();
  // nodeDefaults.temperature is a HysteresisThreshold with trigger/clear
  const tempConfig = cfg?.nodeDefaults?.temperature;
  if (typeof tempConfig === 'object' && tempConfig !== null && 'trigger' in tempConfig) {
    return (tempConfig as { trigger: number }).trigger;
  }
  // Fallback to default 80°C
  return 80;
};

// Export the store
export const useAlertsActivation = () => ({
  // Signals
  config,
  activationState,
  isLoading,
  activeAlerts,
  lastError,

  // Computed
  isPastObservationWindow,
  getBackupThresholds,
  getTemperatureThreshold,

  // Actions
  refreshConfig,
  refreshActiveAlerts,
  activate,
  deactivate,
  snooze,
});
