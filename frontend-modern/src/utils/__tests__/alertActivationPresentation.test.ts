import { describe, expect, it } from 'vitest';
import {
  ALERT_ACTIVATION_FAILURE,
  ALERT_ACTIVATION_SUCCESS,
  ALERT_DEACTIVATION_FAILURE,
  ALERT_DEACTIVATION_SUCCESS,
  getAlertActivationFailure,
  getAlertActivationPresentation,
  getAlertActivationSuccess,
  getAlertDeactivationFailure,
  getAlertDeactivationSuccess,
} from '@/utils/alertActivationPresentation';

describe('getAlertActivationPresentation', () => {
  it('returns the active alerts presentation', () => {
    expect(getAlertActivationPresentation({ isActive: true })).toEqual({
      label: 'Alerts enabled',
      labelClass: 'text-green-600 dark:text-green-400',
      trackClass: 'relative h-6 w-11 rounded-full transition bg-blue-600',
      thumbClass:
        'absolute top-[2px] left-[2px] h-5 w-5 rounded-full bg-white shadow transition-all translate-x-5',
    });
  });

  it('returns the inactive busy alerts presentation', () => {
    expect(getAlertActivationPresentation({ isActive: false, isBusy: true })).toEqual({
      label: 'Alerts disabled',
      labelClass: 'text-muted',
      trackClass: 'relative h-6 w-11 rounded-full transition bg-surface-hover opacity-50',
      thumbClass:
        'absolute top-[2px] left-[2px] h-5 w-5 rounded-full bg-white shadow transition-all translate-x-0',
    });
  });

  it('returns canonical activation feedback copy', () => {
    expect(ALERT_ACTIVATION_SUCCESS).toBe(
      "Alerts activated! You'll now receive alerts when issues are detected.",
    );
    expect(ALERT_ACTIVATION_FAILURE).toBe('Unable to activate alerts. Please try again.');
    expect(ALERT_DEACTIVATION_SUCCESS).toBe(
      'Alerts deactivated. Nothing will be sent until you activate them again.',
    );
    expect(ALERT_DEACTIVATION_FAILURE).toBe('Unable to deactivate alerts. Please try again.');
    expect(getAlertActivationSuccess()).toBe(ALERT_ACTIVATION_SUCCESS);
    expect(getAlertActivationFailure()).toBe(ALERT_ACTIVATION_FAILURE);
    expect(getAlertDeactivationSuccess()).toBe(ALERT_DEACTIVATION_SUCCESS);
    expect(getAlertDeactivationFailure()).toBe(ALERT_DEACTIVATION_FAILURE);
  });
});
