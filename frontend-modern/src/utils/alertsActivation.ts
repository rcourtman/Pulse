export const ALERTS_DETECTION_EVENT = 'pulse-alerts-detection-change';

declare global {
  interface Window {
    __pulseAlertsDetectionEnabled?: boolean | null;
  }
}

const isBrowser = typeof window !== 'undefined';

export const setGlobalAlertsDetectionEnabled = (enabled: boolean | null): void => {
  if (!isBrowser) return;
  window.__pulseAlertsDetectionEnabled = enabled;
  window.dispatchEvent(
    new CustomEvent<boolean | null>(ALERTS_DETECTION_EVENT, {
      detail: enabled,
    }),
  );
};

export const isAlertsDetectionEnabled = (): boolean => {
  if (!isBrowser) return true;
  const enabled = window.__pulseAlertsDetectionEnabled;
  if (enabled === undefined || enabled === null) {
    return true;
  }
  return enabled;
};
