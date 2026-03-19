import { describe, expect, it } from 'vitest';
import {
  ALERT_WEBHOOK_TEST_FAILURE,
  ALERT_WEBHOOK_TEST_SUCCESS,
  getAlertWebhookCustomFieldPresets,
  getAlertWebhookTestFailure,
  getAlertWebhookTestSuccess,
  normalizeAlertWebhookCustomFields,
} from '@/utils/alertWebhookPresentation';

describe('alertWebhookPresentation', () => {
  it('returns canonical webhook test-result copy', () => {
    expect(ALERT_WEBHOOK_TEST_SUCCESS).toBe('Test webhook sent successfully!');
    expect(ALERT_WEBHOOK_TEST_FAILURE).toBe('Failed to send test webhook');
    expect(getAlertWebhookTestSuccess()).toBe('Test webhook sent successfully!');
    expect(getAlertWebhookTestFailure()).toBe('Failed to send test webhook');
  });

  it('exposes pushover custom-field presets and canonicalizes legacy aliases', () => {
    expect(getAlertWebhookCustomFieldPresets('pushover')).toEqual([
      {
        key: 'token',
        label: 'Application Token',
        placeholder: 'Your Pushover application token',
        required: true,
      },
      {
        key: 'user',
        label: 'User Key',
        placeholder: 'Primary user key or group key',
        required: true,
      },
    ]);

    expect(
      normalizeAlertWebhookCustomFields('pushover', {
        app_token: 'legacy-token',
        user_token: 'legacy-user',
        priority: '2',
      }),
    ).toEqual({
      token: 'legacy-token',
      user: 'legacy-user',
      priority: '2',
    });

    expect(
      normalizeAlertWebhookCustomFields('discord', {
        app_token: 'legacy-token',
        user_token: 'legacy-user',
      }),
    ).toEqual({
      app_token: 'legacy-token',
      user_token: 'legacy-user',
    });
  });
});
