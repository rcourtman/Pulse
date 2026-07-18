import { afterEach, beforeEach, describe, expect, it } from 'vitest';
import { DEFAULT_LOCALE, setActiveLocale } from '@/i18n';
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
  beforeEach(() => {
    setActiveLocale(DEFAULT_LOCALE);
  });

  afterEach(() => {
    setActiveLocale(DEFAULT_LOCALE);
  });

  it('returns the enabled notification presentation', () => {
    expect(getAlertActivationPresentation({ isActive: true })).toEqual({
      label: 'Notifications enabled',
      labelClass: 'text-green-600 dark:text-green-400',
      trackClass: 'relative h-6 w-11 rounded-full transition bg-blue-600',
      thumbClass:
        'absolute top-[2px] left-[2px] h-5 w-5 rounded-full bg-white shadow transition-all translate-x-5',
    });
  });

  it('returns the paused busy notification presentation', () => {
    expect(getAlertActivationPresentation({ isActive: false, isBusy: true })).toEqual({
      label: 'Notifications paused',
      labelClass: 'text-muted',
      trackClass: 'relative h-6 w-11 rounded-full transition bg-surface-hover opacity-50',
      thumbClass:
        'absolute top-[2px] left-[2px] h-5 w-5 rounded-full bg-white shadow transition-all translate-x-0',
    });
  });

  it('returns canonical activation feedback copy', () => {
    expect(ALERT_ACTIVATION_SUCCESS).toBe(
      "Notifications enabled. You'll now receive notifications when issues are detected.",
    );
    expect(ALERT_ACTIVATION_FAILURE).toBe('Unable to enable notifications. Please try again.');
    expect(ALERT_DEACTIVATION_SUCCESS).toBe(
      'Notifications paused. Pulse will keep detecting and showing active alerts.',
    );
    expect(ALERT_DEACTIVATION_FAILURE).toBe('Unable to pause notifications. Please try again.');
    expect(getAlertActivationSuccess()).toBe(ALERT_ACTIVATION_SUCCESS);
    expect(getAlertActivationFailure()).toBe(ALERT_ACTIVATION_FAILURE);
    expect(getAlertDeactivationSuccess()).toBe(ALERT_DEACTIVATION_SUCCESS);
    expect(getAlertDeactivationFailure()).toBe(ALERT_DEACTIVATION_FAILURE);
  });

  it('localizes activation labels and feedback through the active locale', () => {
    setActiveLocale('es');

    expect(getAlertActivationPresentation({ isActive: true }).label).toBe(
      'Notificaciones activadas',
    );
    expect(getAlertActivationPresentation({ isActive: false }).label).toBe(
      'Notificaciones pausadas',
    );
    expect(getAlertActivationSuccess()).toBe(
      'Notificaciones activadas. Ahora recibiras avisos cuando se detecten problemas.',
    );
    expect(getAlertActivationFailure()).toBe(
      'No se pudieron activar las notificaciones. Intentalo de nuevo.',
    );
    expect(getAlertDeactivationSuccess()).toBe(
      'Notificaciones pausadas. Pulse seguira detectando y mostrando alertas activas.',
    );
    expect(getAlertDeactivationFailure()).toBe(
      'No se pudieron pausar las notificaciones. Intentalo de nuevo.',
    );
  });
});
