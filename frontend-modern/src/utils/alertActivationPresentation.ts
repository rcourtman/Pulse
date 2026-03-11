export interface AlertActivationPresentationOptions {
  isActive: boolean;
  isBusy?: boolean;
}

export interface AlertActivationPresentation {
  label: string;
  labelClass: string;
  trackClass: string;
  thumbClass: string;
}

export const ALERT_ACTIVATION_SUCCESS =
  "Alerts activated! You'll now receive alerts when issues are detected.";
export const ALERT_ACTIVATION_FAILURE = 'Unable to activate alerts. Please try again.';
export const ALERT_DEACTIVATION_SUCCESS =
  'Alerts deactivated. Nothing will be sent until you activate them again.';
export const ALERT_DEACTIVATION_FAILURE = 'Unable to deactivate alerts. Please try again.';

export function getAlertActivationPresentation({
  isActive,
  isBusy = false,
}: AlertActivationPresentationOptions): AlertActivationPresentation {
  return {
    label: isActive ? 'Alerts enabled' : 'Alerts disabled',
    labelClass: isActive ? 'text-green-600 dark:text-green-400' : 'text-muted',
    trackClass: `relative h-6 w-11 rounded-full transition ${
      isActive ? 'bg-blue-600' : 'bg-surface-hover'
    }${isBusy ? ' opacity-50' : ''}`,
    thumbClass: `absolute top-[2px] left-[2px] h-5 w-5 rounded-full bg-white shadow transition-all ${
      isActive ? 'translate-x-5' : 'translate-x-0'
    }`,
  };
}

export function getAlertActivationSuccess() {
  return ALERT_ACTIVATION_SUCCESS;
}

export function getAlertActivationFailure() {
  return ALERT_ACTIVATION_FAILURE;
}

export function getAlertDeactivationSuccess() {
  return ALERT_DEACTIVATION_SUCCESS;
}

export function getAlertDeactivationFailure() {
  return ALERT_DEACTIVATION_FAILURE;
}
