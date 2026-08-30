import { describe, expect, it } from 'vitest';
import {
  ALERT_DESTINATION_ALL_SEVERITIES_LABEL,
  ALERT_DESTINATION_CRITICAL_ONLY_LABEL,
  ALERT_DESTINATION_MINIMUM_SEVERITY_HELP,
  ALERT_DESTINATION_MINIMUM_SEVERITY_LABEL,
  getAlertDestinationsDeliveryPausedActionLabel,
  getAlertDestinationsDeliveryPausedDescription,
  getAlertDestinationsDeliveryPausedTitle,
  ALERT_DESTINATIONS_CONFIG_LOAD_ERROR,
  ALERT_DESTINATIONS_PUSH_GATE_MESSAGE,
  ALERT_DESTINATIONS_PUSH_GATE_TITLE,
  ALERT_DESTINATIONS_PUSH_PANEL_DESCRIPTION,
  ALERT_DESTINATIONS_PUSH_PANEL_TITLE,
  ALERT_DESTINATIONS_PUSH_READY_MESSAGE,
  ALERT_DESTINATIONS_PUSH_MINIMUM_SEVERITY_HELP,
  ALERT_DESTINATIONS_PUSH_SETUP_LINK_LABEL,
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
  getAlertDeliveryLogFailureClassLabel,
  getAlertDeliveryLogOutcomeLabel,
  getAlertDestinationsDeliveryLogDescription,
  getAlertDestinationsDeliveryLogEmpty,
  getAlertDestinationsDeliveryLogTitle,
  getAlertDestinationsDeliveryLogUnavailable,
  getAlertDestinationsTestPausedWarning,
  getAlertDestinationsAppriseTargetsHelp,
  getAlertDestinationsAppriseTestLabel,
  getAlertDestinationsAppriseTestError,
  getAlertDestinationsAppriseTestFailure,
  getAlertDestinationsAppriseTestSuccess,
  getAlertDestinationsAppriseValidationError,
  getAlertDestinationsConfigLoadError,
  getAlertDestinationsDeliveryHealthDescription,
  getAlertDestinationsDeliveryHealthTitle,
  getAlertDestinationsDeliveryDismissConfirmation,
  getAlertDestinationsDeliveryDismissLabel,
  getAlertDestinationsDeliveryRefreshLabel,
  getAlertDestinationsDeliveryRetryConfirmation,
  getAlertDestinationsDeliveryRetryLabel,
  getAlertDestinationsEmailTestFailure,
  getAlertDestinationsEmailTestSuccess,
  getAlertDestinationSeverityLabel,
  getAlertDestinationsLoadErrorBanner,
  getAlertDestinationsRetryLabel,
  getAlertDestinationsStatusLabel,
  getAlertDestinationsWebhookLoadError,
} from '@/utils/alertDestinationsPresentation';

describe('alertDestinationsPresentation', () => {
  it('returns canonical destinations load error copy', () => {
    expect(ALERT_DESTINATIONS_CONFIG_LOAD_ERROR).toBe(
      'Unable to load notification settings. Your existing configuration could not be retrieved.',
    );
    expect(ALERT_DESTINATIONS_WEBHOOK_LOAD_ERROR).toBe('Unable to load webhook settings.');
    expect(getAlertDestinationsConfigLoadError()).toBe(
      'Unable to load notification settings. Your existing configuration could not be retrieved.',
    );
    expect(getAlertDestinationsWebhookLoadError()).toBe('Unable to load webhook settings.');
  });

  it('returns canonical destinations retry and warning copy', () => {
    expect(ALERT_DESTINATIONS_LOAD_ERROR_RISK_NOTICE).toBe(
      'Saving now may overwrite your existing settings with defaults.',
    );
    expect(getAlertDestinationsLoadErrorBanner('Unable to load webhook settings.')).toBe(
      'Unable to load webhook settings. Saving now may overwrite your existing settings with defaults.',
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
    expect(getAlertDestinationsAppriseTestLabel(true)).toBe('Testing…');
    expect(ALERT_DESTINATIONS_APPRISE_MODE_HELP).toBe(
      'Choose how Pulse should execute Apprise notifications.',
    );
    expect(getAlertDestinationsAppriseTargetsHelp('cli')).toBe(
      'Enter one Apprise URL per line. Commas are also supported. For Telegram forum topics, append :topic to the chat ID.',
    );
    expect(getAlertDestinationsAppriseTargetsHelp('http')).toBe(
      'Optional: override the URLs defined on your Apprise API instance. Leave blank to use the server defaults.',
    );
    expect(ALERT_DESTINATIONS_APPRISE_TARGETS_PLACEHOLDER).toContain(
      'tgram://bot-token/-1001234567890:42',
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
    expect(ALERT_DESTINATIONS_EMAIL_TEST_SUCCESS).toBe('Test email sent. Check your inbox.');
    expect(ALERT_DESTINATIONS_EMAIL_TEST_FAILURE).toBe('Unable to send the test email.');
    expect(getAlertDestinationsEmailTestSuccess()).toBe('Test email sent. Check your inbox.');
    expect(getAlertDestinationsEmailTestFailure()).toBe('Unable to send the test email.');
    expect(ALERT_DESTINATIONS_APPRISE_TEST_SUCCESS).toBe('Test Apprise notification sent.');
    expect(ALERT_DESTINATIONS_APPRISE_TEST_FAILURE).toBe('Unable to send the test notification.');
    expect(getAlertDestinationsAppriseTestSuccess()).toBe('Test Apprise notification sent.');
    expect(getAlertDestinationsAppriseTestFailure()).toBe('Unable to send the test notification.');
  });

  it('uses one explicit severity policy vocabulary across destinations', () => {
    expect(ALERT_DESTINATION_MINIMUM_SEVERITY_LABEL).toBe('Minimum alert severity');
    expect(ALERT_DESTINATION_ALL_SEVERITIES_LABEL).toBe('All alerts');
    expect(ALERT_DESTINATION_CRITICAL_ONLY_LABEL).toBe('Critical alerts only');
    expect(getAlertDestinationSeverityLabel('all')).toBe('All alerts');
    expect(getAlertDestinationSeverityLabel('critical')).toBe('Critical alerts only');
    expect(ALERT_DESTINATION_MINIMUM_SEVERITY_HELP).toContain(
      'Recoveries follow the destination that received the original alert',
    );
  });

  it('returns canonical mobile push destination copy', () => {
    expect(ALERT_DESTINATIONS_PUSH_PANEL_TITLE).toBe('Mobile push notifications');
    expect(ALERT_DESTINATIONS_PUSH_PANEL_DESCRIPTION).toBe(
      'Deliver alerts to your phone through the Pulse Mobile app.',
    );
    expect(ALERT_DESTINATIONS_PUSH_READY_MESSAGE).toContain('Pulse Mobile devices paired');
    expect(ALERT_DESTINATIONS_PUSH_READY_MESSAGE).toContain('Remote Access settings');
    expect(ALERT_DESTINATIONS_PUSH_MINIMUM_SEVERITY_HELP).toContain('Push copy stays private');
    expect(ALERT_DESTINATIONS_PUSH_MINIMUM_SEVERITY_HELP).toContain(
      'Open Pulse Mobile for current alert state',
    );
    expect(ALERT_DESTINATIONS_PUSH_SETUP_LINK_LABEL).toBe('Open Remote Access settings');
    expect(ALERT_DESTINATIONS_PUSH_GATE_TITLE).toBe('Get alerts on your phone');
    expect(ALERT_DESTINATIONS_PUSH_GATE_MESSAGE).toContain('Pulse Mobile app');
    expect(ALERT_DESTINATIONS_PUSH_GATE_MESSAGE).toContain('no port forwarding or VPN');
    expect(ALERT_DESTINATIONS_PUSH_GATE_MESSAGE).toContain('Available with Relay and Pro plans');
  });

  it('distinguishes retained terminal failures from recoverable retry attempts', () => {
    expect(getAlertDestinationsDeliveryHealthTitle('degraded')).toBe(
      'Notification delivery needs attention',
    );
    expect(getAlertDestinationsDeliveryHealthTitle('unavailable')).toBe(
      'Notification delivery status is unavailable',
    );
    expect(
      getAlertDestinationsDeliveryHealthDescription({
        status: 'degraded',
        failed: 1,
        deadLetter: 2,
        completedRetentionDays: 7,
        deadLetterRetentionDays: 30,
        failureClasses7d: {
          authentication: 3,
          rate_limited: 0,
          connectivity: 0,
          tls: 0,
          configuration: 0,
          rejected: 0,
          unknown: 0,
        },
        failureClassesAvailable: true,
      }),
    ).toBe(
      '1 failed delivery retained for 7 days and 2 dead-lettered deliveries retained for 30 days. These notifications were not delivered. Most recent terminal failures were classified as authentication (3). Check destination credentials, tokens, and account permissions. Review delivery activity in Notifications for timestamps, destinations, alerts, and safely redacted errors. After correcting the destination, retry them; dismiss retained failures to clear this warning without deleting delivery history. Otherwise Pulse removes expired records hourly after their retention limit. Recoverable retry attempts do not trigger this warning.',
    );
    expect(
      getAlertDestinationsDeliveryHealthDescription({
        status: 'unavailable',
        failed: 0,
        deadLetter: 0,
        completedRetentionDays: 7,
        deadLetterRetentionDays: 30,
      }),
    ).toContain('could not verify the notification queue');
    expect(getAlertDestinationsDeliveryRefreshLabel()).toBe('Refresh delivery status');
    expect(getAlertDestinationsDeliveryRetryLabel()).toBe('Retry retained deliveries');
    expect(getAlertDestinationsDeliveryDismissLabel()).toBe('Dismiss retained failures');
    expect(getAlertDestinationsDeliveryRetryConfirmation(1)).toContain(
      'A destination that accepted an earlier attempt may receive a duplicate',
    );
    expect(getAlertDestinationsDeliveryDismissConfirmation(2)).toContain(
      'Delivery history remains available',
    );
  });
});

describe('alert destinations delivery log copy', () => {
  it('labels every delivery outcome in plain language, not queue jargon', () => {
    expect(getAlertDeliveryLogOutcomeLabel('sent')).toBe('Delivered');
    expect(getAlertDeliveryLogOutcomeLabel('retry')).toBe('Retrying');
    expect(getAlertDeliveryLogOutcomeLabel('failed')).toBe('Failed');
    // "Dead letter" is queue vocabulary; a user needs to know retries stopped.
    expect(getAlertDeliveryLogOutcomeLabel('dead_letter')).toBe('Failed, retries exhausted');
    expect(getAlertDeliveryLogOutcomeLabel('cancelled')).toBe('Cancelled');
  });

  it('labels failure classes and falls back to unclassified for unknown values', () => {
    expect(getAlertDeliveryLogFailureClassLabel('authentication')).toBe('Authentication failure');
    expect(getAlertDeliveryLogFailureClassLabel('rate_limited')).toBe('Rate limited');
    expect(getAlertDeliveryLogFailureClassLabel('made-up-class')).toBe('Unclassified failure');
  });

  it('names the retention window and the test-send caveat so absence is not read as failure', () => {
    expect(getAlertDestinationsDeliveryLogTitle()).toBe('Recent delivery activity');
    const description = getAlertDestinationsDeliveryLogDescription(7, 30);
    expect(description).toContain('retained for 7 days');
    expect(description).toContain('remain available for 30 days');
    expect(description).toContain('held notifications');
    expect(description).toContain('Test sends skip the queue');
    expect(getAlertDestinationsDeliveryLogEmpty()).toContain('No alert deliveries were attempted');
    // An unreadable log must never present itself as an empty one.
    expect(getAlertDestinationsDeliveryLogUnavailable()).toContain(
      'could not read the delivery log',
    );
  });

  it('warns that a passing test does not mean live alerts flow while delivery is paused', () => {
    const warning = getAlertDestinationsTestPausedWarning();
    expect(warning).toContain('Test sent');
    expect(warning).toContain('delivery is paused');
    expect(warning).toContain('real alerts are not being sent');
  });
});

describe('alert destinations delivery paused copy', () => {
  it('names the consequence and the test-send caveat for every paused reason', () => {
    expect(getAlertDestinationsDeliveryPausedTitle()).toBe('Notifications are paused');
    expect(getAlertDestinationsDeliveryPausedActionLabel()).toBe('Turn on delivery');

    for (const reason of ['detection_off', 'not_activated', 'snoozed'] as const) {
      const description = getAlertDestinationsDeliveryPausedDescription(reason);
      // The whole point of the card: say that nothing reaches the destinations,
      // and that a passing test is not evidence delivery works.
      expect(description).toContain('none of them will be sent to the destinations below');
      expect(description).toContain('Test messages bypass the pause');
    }

    expect(getAlertDestinationsDeliveryPausedDescription('detection_off')).toContain(
      'Alerts are switched off',
    );
    expect(getAlertDestinationsDeliveryPausedDescription('snoozed')).toContain('snoozed');
    expect(getAlertDestinationsDeliveryPausedDescription('not_activated')).toContain(
      'has not been turned on for this install yet',
    );
  });
});
