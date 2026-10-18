import { t } from '@/i18n';

export interface AlertActivationPresentationOptions {
  isActive: boolean;
  isBusy?: boolean;
}

export interface AlertActivationPresentation {
  label: string;
  trackClass: string;
  thumbClass: string;
}

export const ALERT_ACTIVATION_SUCCESS =
  "Notifications enabled. You'll now receive notifications when issues are detected.";
export const ALERT_ACTIVATION_FAILURE = 'Unable to enable notifications. Please try again.';
export const ALERT_DEACTIVATION_SUCCESS =
  'Notifications paused. Pulse will keep detecting and showing active alerts.';
export const ALERT_DEACTIVATION_FAILURE = 'Unable to pause notifications. Please try again.';

export function getAlertActivationPresentation({
  isActive,
  isBusy = false,
}: AlertActivationPresentationOptions): AlertActivationPresentation {
  return {
    // One fixed name for the setting; the switch position alone says whether
    // delivery is on. A label that flipped to "paused" beside an off switch
    // read as "pausing is off".
    label: t('alerts.activation.switchLabel'),
    trackClass: `relative h-6 w-11 rounded-full transition ${
      isActive ? 'bg-blue-600' : 'bg-surface-hover'
    }${isBusy ? ' opacity-50' : ''}`,
    thumbClass: `absolute top-[2px] left-[2px] h-5 w-5 rounded-full bg-white shadow-sm transition-all ${
      isActive ? 'translate-x-5' : 'translate-x-0'
    }`,
  };
}

export function getAlertActivationSuccess() {
  return t('alerts.activation.toast.activated');
}

export function getAlertActivationFailure() {
  return t('alerts.activation.toast.activateFailed');
}

export function getAlertDeactivationSuccess() {
  return t('alerts.activation.toast.deactivated');
}

export function getAlertDeactivationFailure() {
  return t('alerts.activation.toast.deactivateFailed');
}
