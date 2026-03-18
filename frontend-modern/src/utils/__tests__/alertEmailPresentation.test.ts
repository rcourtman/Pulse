import { describe, expect, it } from 'vitest';
import {
  ALERT_EMAIL_MANUAL_CONFIGURATION_LABEL,
  ALERT_EMAIL_PASSWORD_PLACEHOLDER,
  ALERT_EMAIL_PROVIDER_LABEL,
  ALERT_EMAIL_REAPPLY_DEFAULTS_LABEL,
  ALERT_EMAIL_REPLY_TO_PLACEHOLDER,
  ALERT_EMAIL_SMTP_PORT_PLACEHOLDER,
  ALERT_EMAIL_SMTP_SERVER_PLACEHOLDER,
  ALERT_EMAIL_TESTING_LABEL,
  getAlertEmailAdvancedToggleLabel,
  getAlertEmailProviderOptionLabel,
  getAlertEmailRecipientsPlaceholder,
  getAlertEmailSetupInstructionsToggleLabel,
  getAlertEmailTestButtonLabel,
  getAlertEmailUsernamePlaceholder,
} from '@/utils/alertEmailPresentation';

describe('alertEmailPresentation', () => {
  it('exposes canonical provider vocabulary and option formatting', () => {
    expect(ALERT_EMAIL_PROVIDER_LABEL).toBe('Email provider');
    expect(ALERT_EMAIL_MANUAL_CONFIGURATION_LABEL).toBe('Manual configuration');
    expect(ALERT_EMAIL_REAPPLY_DEFAULTS_LABEL).toBe('Reapply defaults');
    expect(
      getAlertEmailProviderOptionLabel({
        name: 'SendGrid',
        smtpHost: 'smtp.sendgrid.net',
        smtpPort: 587,
      }),
    ).toBe('SendGrid (smtp.sendgrid.net:587)');
  });

  it('exposes canonical placeholders and toggle labels', () => {
    expect(ALERT_EMAIL_SMTP_SERVER_PLACEHOLDER).toBe('smtp.example.com');
    expect(ALERT_EMAIL_SMTP_PORT_PLACEHOLDER).toBe('587');
    expect(ALERT_EMAIL_REPLY_TO_PLACEHOLDER).toBe('admin@example.com');
    expect(ALERT_EMAIL_PASSWORD_PLACEHOLDER).toBe('••••••••');
    expect(getAlertEmailUsernamePlaceholder('SendGrid')).toBe('apikey');
    expect(getAlertEmailUsernamePlaceholder('SMTP2GO')).toBe('username@example.com');
    expect(getAlertEmailRecipientsPlaceholder('ops@example.com')).toBe(
      'Leave empty to use ops@example.com\nOr add one recipient per line',
    );
    expect(getAlertEmailSetupInstructionsToggleLabel(true)).toBe('Hide setup instructions');
    expect(getAlertEmailSetupInstructionsToggleLabel(false)).toBe('Show setup instructions');
    expect(getAlertEmailAdvancedToggleLabel(true)).toBe('Hide advanced options');
    expect(getAlertEmailAdvancedToggleLabel(false)).toBe('Show advanced options');
  });

  it('exposes canonical test-email button labels', () => {
    expect(getAlertEmailTestButtonLabel(false)).toBe('Send test email');
    expect(getAlertEmailTestButtonLabel(true)).toBe(ALERT_EMAIL_TESTING_LABEL);
  });
});
