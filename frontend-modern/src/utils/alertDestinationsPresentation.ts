export const ALERT_DESTINATIONS_CONFIG_LOAD_ERROR =
  'Failed to load notification configuration. Your existing settings could not be retrieved.';
export const ALERT_DESTINATIONS_WEBHOOK_LOAD_ERROR = 'Failed to load webhook configuration.';
export const ALERT_DESTINATIONS_LOAD_ERROR_RISK_NOTICE =
  'Saving now may overwrite your existing settings with defaults.';
export const ALERT_DESTINATIONS_RETRY_LABEL = 'Retry';
export const ALERT_DESTINATIONS_RETRYING_LABEL = 'Retrying…';
export const ALERT_DESTINATIONS_ENABLED_LABEL = 'Enabled';
export const ALERT_DESTINATIONS_DISABLED_LABEL = 'Disabled';
export const ALERT_DESTINATIONS_EMAIL_PANEL_TITLE = 'Email notifications';
export const ALERT_DESTINATIONS_EMAIL_PANEL_DESCRIPTION =
  'Configure SMTP delivery for alert emails.';
export const ALERT_DESTINATIONS_APPRISE_PANEL_TITLE = 'Apprise notifications';
export const ALERT_DESTINATIONS_APPRISE_PANEL_DESCRIPTION =
  'Relay grouped alerts through Apprise via CLI or remote API.';
export const ALERT_DESTINATIONS_APPRISE_TEST_LABEL = 'Send test';
export const ALERT_DESTINATIONS_APPRISE_TESTING_LABEL = 'Testing...';
export const ALERT_DESTINATIONS_APPRISE_MODE_LABEL = 'Delivery mode';
export const ALERT_DESTINATIONS_APPRISE_MODE_CLI_LABEL = 'Local Apprise CLI';
export const ALERT_DESTINATIONS_APPRISE_MODE_HTTP_LABEL = 'Remote Apprise API';
export const ALERT_DESTINATIONS_APPRISE_MODE_HELP =
  'Choose how Pulse should execute Apprise notifications.';
export const ALERT_DESTINATIONS_APPRISE_TARGETS_LABEL = 'Delivery targets';
export const ALERT_DESTINATIONS_APPRISE_TARGETS_PLACEHOLDER = `discord://token
mailto://alerts@example.com`;
export const ALERT_DESTINATIONS_APPRISE_TARGETS_HELP_CLI =
  'Enter one Apprise URL per line. Commas are also supported.';
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
export const ALERT_DESTINATIONS_EMAIL_TEST_SUCCESS =
  'Test email sent successfully! Check your inbox.';
export const ALERT_DESTINATIONS_EMAIL_TEST_FAILURE = 'Failed to send test email';
export const ALERT_DESTINATIONS_APPRISE_TEST_SUCCESS =
  'Test Apprise notification sent successfully!';
export const ALERT_DESTINATIONS_APPRISE_TEST_FAILURE = 'Failed to send test notification';

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
