export const ALERT_DESTINATIONS_CONFIG_LOAD_ERROR =
  'Unable to load notification settings. Your existing configuration could not be retrieved.';
export const ALERT_DESTINATIONS_WEBHOOK_LOAD_ERROR = 'Unable to load webhook settings.';
export const ALERT_DESTINATIONS_LOAD_ERROR_RISK_NOTICE =
  'Saving now may overwrite your existing settings with defaults.';
export const ALERT_DESTINATIONS_RETRY_LABEL = 'Retry';
export const ALERT_DESTINATIONS_RETRYING_LABEL = 'Retrying…';
export const ALERT_DESTINATIONS_ENABLED_LABEL = 'Enabled';
export const ALERT_DESTINATIONS_DISABLED_LABEL = 'Disabled';
export const ALERT_DESTINATION_MINIMUM_SEVERITY_LABEL = 'Minimum alert severity';
export const ALERT_DESTINATION_MINIMUM_SEVERITY_HELP =
  'Choose whether this destination receives every alert or only critical incidents. Recoveries follow the destination that received the original alert.';
export const ALERT_DESTINATION_ALL_SEVERITIES_LABEL = 'All alerts';
export const ALERT_DESTINATION_CRITICAL_ONLY_LABEL = 'Critical alerts only';
export const ALERT_DESTINATIONS_EMAIL_PANEL_TITLE = 'Email notifications';
export const ALERT_DESTINATIONS_EMAIL_PANEL_DESCRIPTION =
  'Configure SMTP delivery for alert emails.';
export const ALERT_DESTINATIONS_APPRISE_PANEL_TITLE = 'Apprise notifications';
export const ALERT_DESTINATIONS_APPRISE_PANEL_DESCRIPTION =
  'Relay grouped alerts through Apprise by using the CLI or a remote API.';
export const ALERT_DESTINATIONS_APPRISE_TEST_LABEL = 'Send test';
export const ALERT_DESTINATIONS_APPRISE_TESTING_LABEL = 'Testing…';
export const ALERT_DESTINATIONS_APPRISE_MODE_LABEL = 'Delivery mode';
export const ALERT_DESTINATIONS_APPRISE_MODE_CLI_LABEL = 'Local Apprise CLI';
export const ALERT_DESTINATIONS_APPRISE_MODE_HTTP_LABEL = 'Remote Apprise API';
export const ALERT_DESTINATIONS_APPRISE_MODE_HELP =
  'Choose how Pulse should execute Apprise notifications.';
export const ALERT_DESTINATIONS_APPRISE_TARGETS_LABEL = 'Delivery targets';
export const ALERT_DESTINATIONS_APPRISE_TARGETS_PLACEHOLDER = `tgram://bot-token/-1001234567890:42
discord://token
mailto://alerts@example.com`;
export const ALERT_DESTINATIONS_APPRISE_TARGETS_HELP_CLI =
  'Enter one Apprise URL per line. Commas are also supported. For Telegram forum topics, append :topic to the chat ID.';
export const ALERT_DESTINATIONS_APPRISE_TARGETS_HELP_HTTP =
  'Optional: override the URLs defined on your Apprise API instance. Leave blank to use the server defaults.';
export const ALERT_DESTINATIONS_APPRISE_CLI_PATH_LABEL = 'CLI path';
export const ALERT_DESTINATIONS_APPRISE_CLI_PATH_PLACEHOLDER = 'apprise';
export const ALERT_DESTINATIONS_APPRISE_CLI_PATH_HELP =
  'Leave blank to use the default `apprise` executable.';
export const ALERT_DESTINATIONS_APPRISE_SERVER_URL_LABEL = 'Server URL';
export const ALERT_DESTINATIONS_APPRISE_SERVER_URL_PLACEHOLDER =
  'https://apprise-api.internal:8000';
export const ALERT_DESTINATIONS_APPRISE_SERVER_URL_HELP =
  'Point to an Apprise API endpoint such as https://host:8000.';
export const ALERT_DESTINATIONS_APPRISE_CONFIG_KEY_LABEL = 'Config key (optional)';
export const ALERT_DESTINATIONS_APPRISE_CONFIG_KEY_PLACEHOLDER = 'default';
export const ALERT_DESTINATIONS_APPRISE_CONFIG_KEY_HELP =
  'Targets the /notify/<key> endpoint when provided.';
export const ALERT_DESTINATIONS_APPRISE_API_KEY_LABEL = 'API key';
export const ALERT_DESTINATIONS_APPRISE_API_KEY_PLACEHOLDER = 'Optional API key';
export const ALERT_DESTINATIONS_APPRISE_API_KEY_SAVED_PLACEHOLDER =
  'Saved. Leave blank to keep the current key';
export const ALERT_DESTINATIONS_APPRISE_API_KEY_HELP =
  'Included with each request when your Apprise API requires authentication.';
export const ALERT_DESTINATIONS_APPRISE_API_KEY_HEADER_LABEL = 'API key header';
export const ALERT_DESTINATIONS_APPRISE_API_KEY_HEADER_PLACEHOLDER = 'X-API-KEY';
export const ALERT_DESTINATIONS_APPRISE_API_KEY_HEADER_HELP =
  'Defaults to X-API-KEY for Apprise API deployments.';
export const ALERT_DESTINATIONS_APPRISE_TLS_LABEL = 'TLS verification';
export const ALERT_DESTINATIONS_APPRISE_TLS_CHECKBOX_LABEL = 'Allow self-signed certificates';
export const ALERT_DESTINATIONS_APPRISE_TLS_HELP =
  'Enable only when the Apprise API uses a self-signed certificate.';
export const ALERT_DESTINATIONS_APPRISE_TIMEOUT_LABEL = 'Timeout (seconds)';
export const ALERT_DESTINATIONS_APPRISE_TIMEOUT_HELP =
  'Maximum time to wait for Apprise to respond.';
export const ALERT_DESTINATIONS_APPRISE_ENABLE_FOR_TEST_ERROR =
  'Enable Apprise notifications before sending a test.';
export const ALERT_DESTINATIONS_APPRISE_MISSING_TARGETS_ERROR =
  'Add at least one Apprise target to test CLI delivery.';
export const ALERT_DESTINATIONS_APPRISE_MISSING_SERVER_URL_ERROR =
  'Enter an Apprise API server URL to test API delivery.';
export const ALERT_DESTINATIONS_EMAIL_TEST_SUCCESS = 'Test email sent. Check your inbox.';
export const ALERT_DESTINATIONS_EMAIL_TEST_FAILURE = 'Unable to send the test email.';
export const ALERT_DESTINATIONS_PUSH_PANEL_TITLE = 'Mobile push notifications';
export const ALERT_DESTINATIONS_PUSH_PANEL_DESCRIPTION =
  'Deliver alerts to your phone through the Pulse Mobile app.';
export const ALERT_DESTINATIONS_PUSH_READY_MESSAGE =
  'Alerts are pushed to Pulse Mobile devices paired with this instance. Manage pairing and connectivity in Remote Access settings.';
export const ALERT_DESTINATIONS_PUSH_MINIMUM_SEVERITY_HELP =
  'Choose whether phones receive warning and critical pushes or critical pushes only. Push copy stays private. Open Pulse Mobile for current alert state.';
export const ALERT_DESTINATIONS_PUSH_SETUP_LINK_LABEL = 'Open Remote Access settings';
export const ALERT_DESTINATIONS_PUSH_GATE_TITLE = 'Get alerts on your phone';
export const ALERT_DESTINATIONS_PUSH_GATE_MESSAGE =
  'Pair the Pulse Mobile app to receive alert push notifications and check on your systems from anywhere — no port forwarding or VPN required. Available with Relay and Pro plans.';
export const ALERT_DESTINATIONS_APPRISE_TEST_SUCCESS = 'Test Apprise notification sent.';
export const ALERT_DESTINATIONS_APPRISE_TEST_FAILURE = 'Unable to send the test notification.';
export const ALERT_DESTINATIONS_DELIVERY_DEGRADED_TITLE = 'Notification delivery needs attention';
export const ALERT_DESTINATIONS_DELIVERY_UNAVAILABLE_TITLE =
  'Notification delivery status is unavailable';
export const ALERT_DESTINATIONS_DELIVERY_REFRESH_LABEL = 'Refresh delivery status';
export const ALERT_DESTINATIONS_DELIVERY_RETRY_LABEL = 'Retry retained deliveries';
export const ALERT_DESTINATIONS_DELIVERY_DISMISS_LABEL = 'Dismiss retained failures';

export function getAlertDestinationsConfigLoadError() {
  return ALERT_DESTINATIONS_CONFIG_LOAD_ERROR;
}

export function getAlertDestinationsWebhookLoadError() {
  return ALERT_DESTINATIONS_WEBHOOK_LOAD_ERROR;
}

export function getAlertDestinationsLoadErrorBanner(message: string) {
  return `${message} ${ALERT_DESTINATIONS_LOAD_ERROR_RISK_NOTICE}`;
}

export function getAlertDestinationsRetryLabel(isRetrying: boolean) {
  return isRetrying ? ALERT_DESTINATIONS_RETRYING_LABEL : ALERT_DESTINATIONS_RETRY_LABEL;
}

export function getAlertDestinationsStatusLabel(enabled: boolean) {
  return enabled ? ALERT_DESTINATIONS_ENABLED_LABEL : ALERT_DESTINATIONS_DISABLED_LABEL;
}

export function getAlertDestinationSeverityLabel(minimumSeverity: 'all' | 'critical') {
  return minimumSeverity === 'critical'
    ? ALERT_DESTINATION_CRITICAL_ONLY_LABEL
    : ALERT_DESTINATION_ALL_SEVERITIES_LABEL;
}

export function getAlertDestinationsAppriseTestLabel(isTesting: boolean) {
  return isTesting
    ? ALERT_DESTINATIONS_APPRISE_TESTING_LABEL
    : ALERT_DESTINATIONS_APPRISE_TEST_LABEL;
}

export function getAlertDestinationsAppriseTargetsHelp(mode: 'cli' | 'http') {
  return mode === 'http'
    ? ALERT_DESTINATIONS_APPRISE_TARGETS_HELP_HTTP
    : ALERT_DESTINATIONS_APPRISE_TARGETS_HELP_CLI;
}

export function getAlertDestinationsAppriseTestError(type: 'disabled' | 'missingTargets') {
  return type === 'disabled'
    ? ALERT_DESTINATIONS_APPRISE_ENABLE_FOR_TEST_ERROR
    : ALERT_DESTINATIONS_APPRISE_MISSING_TARGETS_ERROR;
}

export function getAlertDestinationsAppriseValidationError(
  type: 'disabled' | 'missingTargets' | 'missingServerUrl',
) {
  if (type === 'missingServerUrl') {
    return ALERT_DESTINATIONS_APPRISE_MISSING_SERVER_URL_ERROR;
  }
  return getAlertDestinationsAppriseTestError(type);
}

export function getAlertDestinationsEmailTestSuccess() {
  return ALERT_DESTINATIONS_EMAIL_TEST_SUCCESS;
}

export function getAlertDestinationsEmailTestFailure() {
  return ALERT_DESTINATIONS_EMAIL_TEST_FAILURE;
}

export function getAlertDestinationsAppriseTestSuccess() {
  return ALERT_DESTINATIONS_APPRISE_TEST_SUCCESS;
}

export function getAlertDestinationsAppriseTestFailure() {
  return ALERT_DESTINATIONS_APPRISE_TEST_FAILURE;
}

export function getAlertDestinationsDeliveryHealthTitle(status: 'degraded' | 'unavailable') {
  return status === 'degraded'
    ? ALERT_DESTINATIONS_DELIVERY_DEGRADED_TITLE
    : ALERT_DESTINATIONS_DELIVERY_UNAVAILABLE_TITLE;
}

export function getAlertDestinationsDeliveryHealthDescription(input: {
  status: 'degraded' | 'unavailable';
  failed: number;
  deadLetter: number;
  completedRetentionDays: number;
  deadLetterRetentionDays: number;
  failureClasses7d?: Record<string, number>;
  failureClassesAvailable?: boolean;
}) {
  if (input.status === 'unavailable') {
    return 'Pulse could not verify the notification queue. Review the destination settings below and send a test before relying on delivery.';
  }

  const outcomes: string[] = [];
  if (input.failed > 0) {
    outcomes.push(
      `${input.failed} failed ${input.failed === 1 ? 'delivery' : 'deliveries'} retained for ${input.completedRetentionDays} days`,
    );
  }
  if (input.deadLetter > 0) {
    outcomes.push(
      `${input.deadLetter} dead-lettered ${input.deadLetter === 1 ? 'delivery' : 'deliveries'} retained for ${input.deadLetterRetentionDays} days`,
    );
  }
  const summary = outcomes.join(' and ') || 'A retained terminal delivery failure';
  const guidanceByClass: Record<string, string> = {
    authentication: 'Check destination credentials, tokens, and account permissions.',
    rate_limited: 'Check provider rate limits and reduce delivery volume before retrying.',
    connectivity: 'Check DNS, firewall, proxy, and destination reachability.',
    tls: 'Check certificate trust, hostname matching, and TLS settings.',
    configuration: 'Review the enabled destination configuration and required fields.',
    rejected: 'Check the destination endpoint and payload requirements.',
    unknown: 'Review the local notification audit details for the terminal error.',
  };
  let diagnostic = 'Check each enabled destination and send a test.';
  if (input.failureClassesAvailable && input.failureClasses7d) {
    const dominant = Object.entries(input.failureClasses7d)
      .filter(([, count]) => count > 0)
      .sort((left, right) => right[1] - left[1])[0];
    if (dominant) {
      diagnostic = `Most recent terminal failures were classified as ${dominant[0].replace('_', ' ')} (${dominant[1]}). ${guidanceByClass[dominant[0]] ?? guidanceByClass.unknown}`;
    }
  }
  return `${summary}. These notifications were not delivered. Pulse removes expired records hourly, so this warning clears after the last retained failure reaches its retention limit if no new terminal failures occur. ${diagnostic} Recoverable retry attempts do not trigger this warning.`;
}

export function getAlertDestinationsDeliveryRefreshLabel() {
  return ALERT_DESTINATIONS_DELIVERY_REFRESH_LABEL;
}

export function getAlertDestinationsDeliveryRetryLabel() {
  return ALERT_DESTINATIONS_DELIVERY_RETRY_LABEL;
}

export function getAlertDestinationsDeliveryDismissLabel() {
  return ALERT_DESTINATIONS_DELIVERY_DISMISS_LABEL;
}

export function getAlertDestinationsDeliveryRetryConfirmation(count: number) {
  return `Retry ${count} retained ${count === 1 ? 'delivery' : 'deliveries'} now? A destination that accepted an earlier attempt may receive a duplicate.`;
}

export function getAlertDestinationsDeliveryDismissConfirmation(count: number) {
  return `Dismiss ${count} retained ${count === 1 ? 'failure' : 'failures'}? Pulse will clear the warning and will not retry them. Delivery history remains available.`;
}

// Shown instead of the plain test-success toast when the backend reports the
// test went out while real alert delivery is paused. Without this, a
// successful test is exactly how installs come to believe delivery works
// while every live alert is being suppressed.
export const ALERT_DESTINATIONS_TEST_PAUSED_WARNING =
  'Test sent, but notification delivery is paused: real alerts are not being sent. Turn on delivery at the top of this page.';

export function getAlertDestinationsTestPausedWarning() {
  return ALERT_DESTINATIONS_TEST_PAUSED_WARNING;
}

export const ALERT_DESTINATIONS_DELIVERY_LOG_TITLE = 'Recent delivery activity';
export const ALERT_DESTINATIONS_DELIVERY_LOG_EMPTY =
  'No alert deliveries were attempted in this window.';
export const ALERT_DESTINATIONS_DELIVERY_LOG_UNAVAILABLE =
  'Pulse could not read the delivery log, so recent delivery activity cannot be shown.';

export function getAlertDestinationsDeliveryLogTitle() {
  return ALERT_DESTINATIONS_DELIVERY_LOG_TITLE;
}

// The test-send caveat matters: test messages skip the queue entirely, so a
// user who sends a test and then checks this log would otherwise read its
// absence as a delivery failure.
export function getAlertDestinationsDeliveryLogDescription(windowDays: number) {
  return `Delivery attempts and held notifications for real alerts over the last ${windowDays} days. Test sends skip the queue and are not listed here.`;
}

export function getAlertDestinationsDeliveryLogEmpty() {
  return ALERT_DESTINATIONS_DELIVERY_LOG_EMPTY;
}

export function getAlertDestinationsDeliveryLogUnavailable() {
  return ALERT_DESTINATIONS_DELIVERY_LOG_UNAVAILABLE;
}

export type AlertDeliveryLogOutcome = 'sent' | 'retry' | 'failed' | 'dead_letter' | 'cancelled';

// One plain-language label per outcome. "Dead letter" is queue jargon; what a
// user needs to know is that Pulse stopped retrying.
export function getAlertDeliveryLogOutcomeLabel(outcome: AlertDeliveryLogOutcome) {
  switch (outcome) {
    case 'sent':
      return 'Delivered';
    case 'retry':
      return 'Retrying';
    case 'dead_letter':
      return 'Failed, retries exhausted';
    case 'cancelled':
      return 'Cancelled';
    default:
      return 'Failed';
  }
}

export function getAlertDeliveryLogFailureClassLabel(failureClass: string) {
  const labels: Record<string, string> = {
    authentication: 'Authentication failure',
    rate_limited: 'Rate limited',
    connectivity: 'Connectivity failure',
    tls: 'TLS failure',
    configuration: 'Configuration problem',
    rejected: 'Rejected by destination',
    unknown: 'Unclassified failure',
  };
  return labels[failureClass] ?? labels.unknown;
}

const ALERT_DESTINATIONS_DELIVERY_PAUSED_TITLE = 'Notifications are paused';
const ALERT_DESTINATIONS_DELIVERY_PAUSED_ACTION = 'Turn on delivery';

export type AlertDestinationsDeliveryPausedReason = 'detection_off' | 'not_activated' | 'snoozed';

export function getAlertDestinationsDeliveryPausedTitle() {
  return ALERT_DESTINATIONS_DELIVERY_PAUSED_TITLE;
}

export function getAlertDestinationsDeliveryPausedActionLabel() {
  return ALERT_DESTINATIONS_DELIVERY_PAUSED_ACTION;
}

// Describes why nothing configured on this page will actually reach anyone.
// The test-send caveat is deliberate: test messages bypass the delivery pause,
// so a successful test is not evidence that live alerts are getting through.
export function getAlertDestinationsDeliveryPausedDescription(
  reason: AlertDestinationsDeliveryPausedReason,
) {
  const consequence =
    'Pulse is still detecting alerts, but none of them will be sent to the destinations below. Test messages bypass the pause, so a successful test does not mean live alerts are getting through.';

  if (reason === 'detection_off') {
    return `Alerts are switched off in the alert configuration. ${consequence}`;
  }
  if (reason === 'snoozed') {
    return `Notification delivery is snoozed. ${consequence}`;
  }
  return `Notification delivery has not been turned on for this install yet. ${consequence}`;
}
