import { createSignal } from 'solid-js';
import { AlertsAPI } from '@/api/alerts';
import type { AlertConfig, ActivationState as ActivationStateType } from '@/types/alerts';
import type { Alert } from '@/types/api';

// Create signals for activation state
const [config, setConfig] = createSignal<AlertConfig | null>(null);
const [activationState, setActivationState] = createSignal<ActivationStateType | null>(null);
const [isLoading, setIsLoading] = createSignal(false);
const [activeAlerts, setActiveAlerts] = createSignal<Alert[]>([]);
const [lastError, setLastError] = createSignal<string | null>(null);

// Refresh config from API
const refreshConfig = async (): Promise<void> => {
  try {
    setIsLoading(true);
    setLastError(null);
    const alertConfig = await AlertsAPI.getConfig();
    setConfig(alertConfig);
    setActivationState(alertConfig.activationState || 'active');
  } catch (error) {
    console.error('Failed to fetch alert config:', error);
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
    console.error('Failed to fetch active alerts:', error);
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
    console.error('Failed to activate alerts:', error);
    setLastError(error instanceof Error ? error.message : 'Unknown error');
    return false;
  } finally {
    setIsLoading(false);
  }
};

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

  // Actions
  refreshConfig,
  refreshActiveAlerts,
  activate,
});

// Initialize on module load
refreshConfig();
refreshActiveAlerts();
