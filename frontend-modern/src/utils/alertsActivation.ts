import type { ActivationState } from '@/types/alerts';

export const ALERTS_ACTIVATION_EVENT = 'pulse-alerts-activation-change';

declare global {
  interface Window {
    __pulseAlertsActivationState?: ActivationState | null;
  }
}

const isBrowser = typeof window !== 'undefined';

export const setGlobalActivationState = (state: ActivationState | null): void => {
  if (!isBrowser) return;
  window.__pulseAlertsActivationState = state;
  window.dispatchEvent(
    new CustomEvent<ActivationState | null>(ALERTS_ACTIVATION_EVENT, {
      detail: state,
    }),
  );
};

export const isAlertsActivationEnabled = (): boolean => {
  if (!isBrowser) return true;
  const state = window.__pulseAlertsActivationState;
  if (state === undefined || state === null) {
    return true;
  }
  return state === 'active';
};
