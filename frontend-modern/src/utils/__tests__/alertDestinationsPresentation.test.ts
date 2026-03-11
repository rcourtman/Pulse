import { describe, expect, it } from 'vitest';
import {
  ALERT_DESTINATIONS_CONFIG_LOAD_ERROR,
  ALERT_DESTINATIONS_APPRISE_API_KEY_HEADER_HELP,
  ALERT_DESTINATIONS_APPRISE_ENABLE_FOR_TEST_ERROR,
  ALERT_DESTINATIONS_APPRISE_MISSING_TARGETS_ERROR,
  ALERT_DESTINATIONS_APPRISE_MISSING_SERVER_URL_ERROR,
  ALERT_DESTINATIONS_APPRISE_MODE_HELP,
  ALERT_DESTINATIONS_APPRISE_PANEL_TITLE,
  ALERT_DESTINATIONS_APPRISE_TARGETS_PLACEHOLDER,
  ALERT_DESTINATIONS_APPRISE_TEST_FAILURE,
  ALERT_DESTINATIONS_APPRISE_TEST_SUCCESS,
  ALERT_DESTINATIONS_EMAIL_PANEL_TITLE,
  ALERT_DESTINATIONS_EMAIL_TEST_FAILURE,
  ALERT_DESTINATIONS_EMAIL_TEST_SUCCESS,
  ALERT_DESTINATIONS_LOAD_ERROR_RISK_NOTICE,
  ALERT_DESTINATIONS_RETRYING_LABEL,
  ALERT_DESTINATIONS_RETRY_LABEL,
  ALERT_DESTINATIONS_WEBHOOK_LOAD_ERROR,
  getAlertDestinationsAppriseTargetsHelp,
  getAlertDestinationsAppriseTestLabel,
  getAlertDestinationsAppriseTestError,
  getAlertDestinationsAppriseTestFailure,
  getAlertDestinationsAppriseTestSuccess,
  getAlertDestinationsAppriseValidationError,
  getAlertDestinationsConfigLoadError,
  getAlertDestinationsEmailTestFailure,
  getAlertDestinationsEmailTestSuccess,
  getAlertDestinationsLoadErrorBanner,
  getAlertDestinationsRetryLabel,
  getAlertDestinationsStatusLabel,
  getAlertDestinationsWebhookLoadError,
} from '@/utils/alertDestinationsPresentation';

describe('alertDestinationsPresentation', () => {
  it('returns canonical destinations load error copy', () => {
    expect(ALERT_DESTINATIONS_CONFIG_LOAD_ERROR).toBe(
      'Failed to load notification configuration. Your existing settings could not be retrieved.',
    );
    expect(ALERT_DESTINATIONS_WEBHOOK_LOAD_ERROR).toBe('Failed to load webhook configuration.');
    expect(getAlertDestinationsConfigLoadError()).toBe(
      'Failed to load notification configuration. Your existing settings could not be retrieved.',
    );
    expect(getAlertDestinationsWebhookLoadError()).toBe(
      'Failed to load webhook configuration.',
    );
  });

  it('returns canonical destinations retry and warning copy', () => {
    expect(ALERT_DESTINATIONS_LOAD_ERROR_RISK_NOTICE).toBe(
      'Saving now may overwrite your existing settings with defaults.',
    );
    expect(getAlertDestinationsLoadErrorBanner('Failed to load webhook configuration.')).toBe(
      'Failed to load webhook configuration. Saving now may overwrite your existing settings with defaults.',
    );
    expect(ALERT_DESTINATIONS_RETRY_LABEL).toBe('Retry');
    expect(ALERT_DESTINATIONS_RETRYING_LABEL).toBe('Retrying…');
    expect(getAlertDestinationsRetryLabel(false)).toBe('Retry');
    expect(getAlertDestinationsRetryLabel(true)).toBe('Retrying…');
  });

  it('returns canonical destinations panel and apprise vocabulary', () => {
    expect(ALERT_DESTINATIONS_EMAIL_PANEL_TITLE).toBe('Email notifications');
    expect(ALERT_DESTINATIONS_APPRISE_PANEL_TITLE).toBe('Apprise notifications');
    expect(getAlertDestinationsStatusLabel(true)).toBe('Enabled');
    expect(getAlertDestinationsStatusLabel(false)).toBe('Disabled');
    expect(getAlertDestinationsAppriseTestLabel(false)).toBe('Send test');
    expect(getAlertDestinationsAppriseTestLabel(true)).toBe('Testing...');
    expect(ALERT_DESTINATIONS_APPRISE_MODE_HELP).toBe(
      'Choose how Pulse should execute Apprise notifications.',
    );
    expect(getAlertDestinationsAppriseTargetsHelp('cli')).toBe(
      'Enter one Apprise URL per line. Commas are also supported.',
    );
    expect(getAlertDestinationsAppriseTargetsHelp('http')).toBe(
      'Optional: override the URLs defined on your Apprise API instance. Leave blank to use the server defaults.',
    );
    expect(ALERT_DESTINATIONS_APPRISE_TARGETS_PLACEHOLDER).toContain('discord://token');
    expect(ALERT_DESTINATIONS_APPRISE_API_KEY_HEADER_HELP).toBe(
      'Defaults to X-API-KEY for Apprise API deployments.',
    );
    expect(getAlertDestinationsAppriseTestError('disabled')).toBe(
      ALERT_DESTINATIONS_APPRISE_ENABLE_FOR_TEST_ERROR,
    );
    expect(getAlertDestinationsAppriseTestError('missingTargets')).toBe(
      ALERT_DESTINATIONS_APPRISE_MISSING_TARGETS_ERROR,
    );
    expect(getAlertDestinationsAppriseValidationError('missingServerUrl')).toBe(
      ALERT_DESTINATIONS_APPRISE_MISSING_SERVER_URL_ERROR,
    );
    expect(ALERT_DESTINATIONS_EMAIL_TEST_SUCCESS).toBe(
      'Test email sent successfully! Check your inbox.',
    );
    expect(ALERT_DESTINATIONS_EMAIL_TEST_FAILURE).toBe('Failed to send test email');
    expect(getAlertDestinationsEmailTestSuccess()).toBe(
      'Test email sent successfully! Check your inbox.',
    );
    expect(getAlertDestinationsEmailTestFailure()).toBe('Failed to send test email');
    expect(ALERT_DESTINATIONS_APPRISE_TEST_SUCCESS).toBe(
      'Test Apprise notification sent successfully!',
    );
    expect(ALERT_DESTINATIONS_APPRISE_TEST_FAILURE).toBe(
      'Failed to send test notification',
    );
    expect(getAlertDestinationsAppriseTestSuccess()).toBe(
      'Test Apprise notification sent successfully!',
    );
    expect(getAlertDestinationsAppriseTestFailure()).toBe(
      'Failed to send test notification',
    );
  });
});
