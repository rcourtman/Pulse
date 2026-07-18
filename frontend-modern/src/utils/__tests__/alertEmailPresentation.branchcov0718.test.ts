import { describe, expect, it } from 'vitest';
import {
  ALERT_EMAIL_FROM_ADDRESS_LABEL,
  ALERT_EMAIL_FROM_ADDRESS_PLACEHOLDER,
  ALERT_EMAIL_HIDE_ADVANCED_OPTIONS_LABEL,
  ALERT_EMAIL_HIDE_SETUP_INSTRUCTIONS_LABEL,
  ALERT_EMAIL_MAX_RETRIES_LABEL,
  ALERT_EMAIL_PASSWORD_LABEL,
  ALERT_EMAIL_PASSWORD_PLACEHOLDER,
  ALERT_EMAIL_RATE_LIMIT_LABEL,
  ALERT_EMAIL_RATE_LIMIT_SUFFIX,
  ALERT_EMAIL_RECIPIENTS_FALLBACK_FROM,
  ALERT_EMAIL_RECIPIENTS_LABEL,
  ALERT_EMAIL_RECIPIENTS_PLACEHOLDER_SUFFIX,
  ALERT_EMAIL_REPLY_TO_LABEL,
  ALERT_EMAIL_REPLY_TO_PLACEHOLDER,
  ALERT_EMAIL_RETRY_DELAY_LABEL,
  ALERT_EMAIL_SECURITY_LABEL,
  ALERT_EMAIL_SECURITY_NONE_LABEL,
  ALERT_EMAIL_SECURITY_STARTTLS_LABEL,
  ALERT_EMAIL_SECURITY_TLS_LABEL,
  ALERT_EMAIL_SENDGRID_USERNAME_PLACEHOLDER,
  ALERT_EMAIL_SHOW_ADVANCED_OPTIONS_LABEL,
  ALERT_EMAIL_SHOW_SETUP_INSTRUCTIONS_LABEL,
  ALERT_EMAIL_SMTP_PORT_LABEL,
  ALERT_EMAIL_SMTP_PORT_PLACEHOLDER,
  ALERT_EMAIL_SMTP_SERVER_LABEL,
  ALERT_EMAIL_SMTP_SERVER_PLACEHOLDER,
  ALERT_EMAIL_TEST_LABEL,
  ALERT_EMAIL_TESTING_LABEL,
  ALERT_EMAIL_USERNAME_LABEL,
  ALERT_EMAIL_USERNAME_PLACEHOLDER,
  getAlertEmailAdvancedToggleLabel,
  getAlertEmailProviderOptionLabel,
  getAlertEmailRecipientsPlaceholder,
  getAlertEmailSetupInstructionsToggleLabel,
  getAlertEmailTestButtonLabel,
  getAlertEmailUsernamePlaceholder,
} from '@/utils/alertEmailPresentation';

// Branch-coverage companion to alertEmailPresentation.test.ts.
//
// The sibling suite already exercises:
//   - getAlertEmailProviderOptionLabel on the SendGrid (587) sample,
//   - both arms of getAlertEmailUsernamePlaceholder ('SendGrid' vs 'SMTP2GO'),
//   - getAlertEmailRecipientsPlaceholder on the TRUTHY fromAddress arm only,
//   - both arms of the two toggle helpers,
//   - both arms of getAlertEmailTestButtonLabel,
//   - a small slice of the exported label/placeholder constants.
//
// This file targets the RESIDUAL:
//   1. The falsy arm (`fromAddress || ALERT_EMAIL_RECIPIENTS_FALLBACK_FROM`) of
//      getAlertEmailRecipientsPlaceholder for `undefined` and the empty string,
//      including the exact rendered newline-joined string.
//   2. Additional provider variants for getAlertEmailUsernamePlaceholder and
//      getAlertEmailProviderOptionLabel to confirm the non-SendGrid branch is
//      stable across provider strings (and that integer port rendering is
//      literal, no coercion).
//   3. The full canonical label/placeholder vocabulary that the sibling suite
//      does not yet assert — every remaining exported string constant.
//
// Note on item-spec wording: the task brief mentions "factory functions
// returning objects of getters" and "email-config state arms (configured vs
// not)". This module has neither — it is 69 lines of pure value-in/value-out
// functions plus exported string constants. The residual coverage is therefore
// the falsy recipients arm + the unused-on-this-path vocabulary. There are no
// factory/getter functions to wrap in createRoot here, so we follow the
// sibling's plain direct-invocation pattern exactly.

describe('alertEmailPresentation — branch coverage (batch 0718)', () => {
  describe('getAlertEmailRecipientsPlaceholder — falsy fromAddress arm', () => {
    it('falls back to "the from address" when fromAddress is undefined', () => {
      expect(getAlertEmailRecipientsPlaceholder()).toBe(
        `Leave empty to use ${ALERT_EMAIL_RECIPIENTS_FALLBACK_FROM}\n${ALERT_EMAIL_RECIPIENTS_PLACEHOLDER_SUFFIX}`,
      );
      expect(getAlertEmailRecipientsPlaceholder()).toBe(
        'Leave empty to use the from address\nOr add one recipient per line',
      );
    });

    it('falls back to "the from address" when fromAddress is the empty string (|| short-circuits)', () => {
      // The empty string is falsy in JS, so the `||` operator picks the
      // FALLBACK_FROM constant even though an argument was supplied.
      expect(getAlertEmailRecipientsPlaceholder('')).toBe(
        'Leave empty to use the from address\nOr add one recipient per line',
      );
    });

    it('uses the supplied fromAddress verbatim when truthy (whitespace survives)', () => {
      // Sanity check that the truthy arm interpolates the raw string; the
      // sibling test only checked one sample ('ops@example.com').
      expect(getAlertEmailRecipientsPlaceholder('  team@corp.io ')).toBe(
        'Leave empty to use   team@corp.io \nOr add one recipient per line',
      );
    });
  });

  describe('getAlertEmailUsernamePlaceholder — non-SendGrid arm across provider variants', () => {
    it.each([
      ['Mailgun' as const],
      ['Postmark' as const],
      ['Amazon SES' as const],
      ['' as const],
      ['sendgrid' as const], // case-sensitive: only exact 'SendGrid' matches
    ])('returns the generic placeholder for provider %p (non-SendGrid arm)', (provider) => {
      expect(getAlertEmailUsernamePlaceholder(provider)).toBe(ALERT_EMAIL_USERNAME_PLACEHOLDER);
      expect(getAlertEmailUsernamePlaceholder(provider)).toBe('username@example.com');
    });

    it('returns the SendGrid placeholder only for the exact string "SendGrid"', () => {
      expect(getAlertEmailUsernamePlaceholder('SendGrid')).toBe(
        ALERT_EMAIL_SENDGRID_USERNAME_PLACEHOLDER,
      );
      expect(getAlertEmailUsernamePlaceholder('SendGrid')).toBe('apikey');
    });
  });

  describe('getAlertEmailProviderOptionLabel — port & host rendering variants', () => {
    it('renders TLS port 465 and arbitrary host verbatim (no protocol rewrite)', () => {
      expect(
        getAlertEmailProviderOptionLabel({
          name: 'Amazon SES',
          smtpHost: 'email-smtp.us-east-1.amazonaws.com',
          smtpPort: 465,
        }),
      ).toBe('Amazon SES (email-smtp.us-east-1.amazonaws.com:465)');
    });

    it('renders unusual port numbers and unicode / spaced names without coercion', () => {
      // Port 2525 is commonly used by Mailgun/Postmark; we assert the integer
      // is interpolated directly (no zero-padding, no thousands separator).
      expect(
        getAlertEmailProviderOptionLabel({
          name: 'Mailgun EU',
          smtpHost: 'smtp.eu.mailgun.org',
          smtpPort: 2525,
        }),
      ).toBe('Mailgun EU (smtp.eu.mailgun.org:2525)');

      // Port 1 (boundary: smallest truthy port) renders literally.
      expect(
        getAlertEmailProviderOptionLabel({
          name: 'X',
          smtpHost: 'h',
          smtpPort: 1,
        }),
      ).toBe('X (h:1)');
    });
  });

  describe('getAlertEmailSetupInstructionsToggleLabel — exact constant wiring', () => {
    it('returns the canonical HIDE constant for true', () => {
      expect(getAlertEmailSetupInstructionsToggleLabel(true)).toBe(
        ALERT_EMAIL_HIDE_SETUP_INSTRUCTIONS_LABEL,
      );
    });

    it('returns the canonical SHOW constant for false', () => {
      expect(getAlertEmailSetupInstructionsToggleLabel(false)).toBe(
        ALERT_EMAIL_SHOW_SETUP_INSTRUCTIONS_LABEL,
      );
    });
  });

  describe('getAlertEmailAdvancedToggleLabel — exact constant wiring', () => {
    it('returns the canonical HIDE constant for true', () => {
      expect(getAlertEmailAdvancedToggleLabel(true)).toBe(
        ALERT_EMAIL_HIDE_ADVANCED_OPTIONS_LABEL,
      );
    });

    it('returns the canonical SHOW constant for false', () => {
      expect(getAlertEmailAdvancedToggleLabel(false)).toBe(
        ALERT_EMAIL_SHOW_ADVANCED_OPTIONS_LABEL,
      );
    });
  });

  describe('getAlertEmailTestButtonLabel — exact constant wiring', () => {
    it('returns the canonical idle label for false', () => {
      expect(getAlertEmailTestButtonLabel(false)).toBe(ALERT_EMAIL_TEST_LABEL);
    });

    it('returns the canonical testing label for true', () => {
      expect(getAlertEmailTestButtonLabel(true)).toBe(ALERT_EMAIL_TESTING_LABEL);
    });
  });

  describe('residual exported vocabulary — exact canonical strings', () => {
    // The sibling suite asserts a handful of label constants; this block pins
    // the rest so a silent rename in the source will fail loudly here.

    it('exposes the SMTP server / port label + placeholder vocabulary', () => {
      expect(ALERT_EMAIL_SMTP_SERVER_LABEL).toBe('SMTP server');
      expect(ALERT_EMAIL_SMTP_SERVER_PLACEHOLDER).toBe('smtp.example.com');
      expect(ALERT_EMAIL_SMTP_PORT_LABEL).toBe('SMTP port');
      expect(ALERT_EMAIL_SMTP_PORT_PLACEHOLDER).toBe('587');
    });

    it('exposes the from / reply-to label + placeholder vocabulary', () => {
      expect(ALERT_EMAIL_FROM_ADDRESS_LABEL).toBe('From address');
      expect(ALERT_EMAIL_FROM_ADDRESS_PLACEHOLDER).toBe('noreply@example.com');
      expect(ALERT_EMAIL_REPLY_TO_LABEL).toBe('Reply-to address');
      expect(ALERT_EMAIL_REPLY_TO_PLACEHOLDER).toBe('admin@example.com');
    });

    it('exposes the username label + both placeholder constants', () => {
      expect(ALERT_EMAIL_USERNAME_LABEL).toBe('Username');
      expect(ALERT_EMAIL_USERNAME_PLACEHOLDER).toBe('username@example.com');
      expect(ALERT_EMAIL_SENDGRID_USERNAME_PLACEHOLDER).toBe('apikey');
    });

    it('exposes the password label + placeholder vocabulary', () => {
      expect(ALERT_EMAIL_PASSWORD_LABEL).toBe('Password / API key');
      expect(ALERT_EMAIL_PASSWORD_PLACEHOLDER).toBe('••••••••');
    });

    it('exposes the recipients label + fallback + suffix vocabulary', () => {
      expect(ALERT_EMAIL_RECIPIENTS_LABEL).toBe('Recipients (one per line)');
      expect(ALERT_EMAIL_RECIPIENTS_FALLBACK_FROM).toBe('the from address');
      expect(ALERT_EMAIL_RECIPIENTS_PLACEHOLDER_SUFFIX).toBe('Or add one recipient per line');
    });

    it('exposes the setup-instructions show/hide vocabulary', () => {
      expect(ALERT_EMAIL_SHOW_SETUP_INSTRUCTIONS_LABEL).toBe('Show setup instructions');
      expect(ALERT_EMAIL_HIDE_SETUP_INSTRUCTIONS_LABEL).toBe('Hide setup instructions');
    });

    it('exposes the advanced-options show/hide vocabulary', () => {
      expect(ALERT_EMAIL_SHOW_ADVANCED_OPTIONS_LABEL).toBe('Show advanced options');
      expect(ALERT_EMAIL_HIDE_ADVANCED_OPTIONS_LABEL).toBe('Hide advanced options');
    });

    it('exposes the security-option vocabulary (label + all three arms)', () => {
      expect(ALERT_EMAIL_SECURITY_LABEL).toBe('Security');
      expect(ALERT_EMAIL_SECURITY_NONE_LABEL).toBe('None');
      expect(ALERT_EMAIL_SECURITY_STARTTLS_LABEL).toBe('STARTTLS (587)');
      expect(ALERT_EMAIL_SECURITY_TLS_LABEL).toBe('TLS/SSL (465)');
    });

    it('exposes the rate-limit label + suffix vocabulary', () => {
      expect(ALERT_EMAIL_RATE_LIMIT_LABEL).toBe('Rate limit');
      expect(ALERT_EMAIL_RATE_LIMIT_SUFFIX).toBe('/min');
    });

    it('exposes the retry vocabulary (max retries + retry delay)', () => {
      expect(ALERT_EMAIL_MAX_RETRIES_LABEL).toBe('Max retries');
      expect(ALERT_EMAIL_RETRY_DELAY_LABEL).toBe('Retry delay (seconds)');
    });

    it('exposes the test-email idle vocabulary (testing arm covered above)', () => {
      expect(ALERT_EMAIL_TEST_LABEL).toBe('Send test email');
    });
  });
});
