export const ALERT_EMAIL_PROVIDER_LABEL = 'Email provider';
export const ALERT_EMAIL_MANUAL_CONFIGURATION_LABEL = 'Manual configuration';
export const ALERT_EMAIL_REAPPLY_DEFAULTS_LABEL = 'Reapply defaults';
export const ALERT_EMAIL_HIDE_SETUP_INSTRUCTIONS_LABEL = 'Hide setup instructions';
export const ALERT_EMAIL_SHOW_SETUP_INSTRUCTIONS_LABEL = 'Show setup instructions';
export const ALERT_EMAIL_SMTP_SERVER_LABEL = 'SMTP server';
export const ALERT_EMAIL_SMTP_SERVER_PLACEHOLDER = 'smtp.example.com';
export const ALERT_EMAIL_SMTP_PORT_LABEL = 'SMTP port';
export const ALERT_EMAIL_SMTP_PORT_PLACEHOLDER = '587';
export const ALERT_EMAIL_FROM_ADDRESS_LABEL = 'From address';
export const ALERT_EMAIL_FROM_ADDRESS_PLACEHOLDER = 'noreply@example.com';
export const ALERT_EMAIL_REPLY_TO_LABEL = 'Reply-to address';
export const ALERT_EMAIL_REPLY_TO_PLACEHOLDER = 'admin@example.com';
export const ALERT_EMAIL_USERNAME_LABEL = 'Username';
export const ALERT_EMAIL_USERNAME_PLACEHOLDER = 'username@example.com';
export const ALERT_EMAIL_SENDGRID_USERNAME_PLACEHOLDER = 'apikey';
export const ALERT_EMAIL_PASSWORD_LABEL = 'Password / API key';
export const ALERT_EMAIL_PASSWORD_PLACEHOLDER = '••••••••';
export const ALERT_EMAIL_RECIPIENTS_LABEL = 'Recipients (one per line)';
export const ALERT_EMAIL_RECIPIENTS_FALLBACK_FROM = 'the from address';
export const ALERT_EMAIL_RECIPIENTS_PLACEHOLDER_SUFFIX = 'Or add one recipient per line';
export const ALERT_EMAIL_HIDE_ADVANCED_OPTIONS_LABEL = 'Hide advanced options';
export const ALERT_EMAIL_SHOW_ADVANCED_OPTIONS_LABEL = 'Show advanced options';
export const ALERT_EMAIL_SECURITY_LABEL = 'Security';
export const ALERT_EMAIL_SECURITY_NONE_LABEL = 'None';
export const ALERT_EMAIL_SECURITY_STARTTLS_LABEL = 'STARTTLS (587)';
export const ALERT_EMAIL_SECURITY_TLS_LABEL = 'TLS/SSL (465)';
export const ALERT_EMAIL_RATE_LIMIT_LABEL = 'Rate limit';
export const ALERT_EMAIL_RATE_LIMIT_SUFFIX = '/min';
export const ALERT_EMAIL_MAX_RETRIES_LABEL = 'Max retries';
export const ALERT_EMAIL_RETRY_DELAY_LABEL = 'Retry delay (seconds)';
export const ALERT_EMAIL_TEST_LABEL = 'Send test email';
export const ALERT_EMAIL_TESTING_LABEL = 'Sending test email…';

type EmailProviderOption = {
  name: string;
  smtpHost: string;
  smtpPort: number;
};

export function getAlertEmailProviderOptionLabel(provider: EmailProviderOption) {
  return `${provider.name} (${provider.smtpHost}:${provider.smtpPort})`;
}

export function getAlertEmailSetupInstructionsToggleLabel(showing: boolean) {
  return showing
    ? ALERT_EMAIL_HIDE_SETUP_INSTRUCTIONS_LABEL
    : ALERT_EMAIL_SHOW_SETUP_INSTRUCTIONS_LABEL;
}

export function getAlertEmailUsernamePlaceholder(provider: string) {
  return provider === 'SendGrid'
    ? ALERT_EMAIL_SENDGRID_USERNAME_PLACEHOLDER
    : ALERT_EMAIL_USERNAME_PLACEHOLDER;
}

export function getAlertEmailRecipientsPlaceholder(fromAddress?: string) {
  return `Leave empty to use ${fromAddress || ALERT_EMAIL_RECIPIENTS_FALLBACK_FROM}\n${ALERT_EMAIL_RECIPIENTS_PLACEHOLDER_SUFFIX}`;
}

export function getAlertEmailAdvancedToggleLabel(showing: boolean) {
  return showing
    ? ALERT_EMAIL_HIDE_ADVANCED_OPTIONS_LABEL
    : ALERT_EMAIL_SHOW_ADVANCED_OPTIONS_LABEL;
}

export function getAlertEmailTestButtonLabel(testing: boolean) {
  return testing ? ALERT_EMAIL_TESTING_LABEL : ALERT_EMAIL_TEST_LABEL;
}
