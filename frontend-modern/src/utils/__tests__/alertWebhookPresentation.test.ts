import { describe, expect, it } from 'vitest';
import {
  ALERT_WEBHOOK_TEST_FAILURE,
  ALERT_WEBHOOK_TEST_SUCCESS,
  getAlertWebhookServices,
  getAlertWebhookCustomFieldPresets,
  getAlertWebhookMentionHelpFromTemplates,
  getAlertWebhookMentionPlaceholderFromTemplates,
  hasAlertWebhookMentionSupportFromTemplates,
  getAlertWebhookCustomFieldInputs,
  getAlertWebhookServiceLabelFromTemplates,
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

  it('builds pushover custom-field inputs from canonical aliases and extra fields', () => {
    expect(
      getAlertWebhookCustomFieldInputs('pushover', {
        app_token: 'legacy-token',
        user_token: 'legacy-user',
        priority: '2',
      }),
    ).toEqual([
      {
        key: 'token',
        value: 'legacy-token',
        label: 'Application Token',
        placeholder: 'Your Pushover application token',
        required: true,
      },
      {
        key: 'user',
        value: 'legacy-user',
        label: 'User Key',
        placeholder: 'Primary user key or group key',
        required: true,
      },
      {
        key: 'priority',
        value: '2',
      },
    ]);

    expect(getAlertWebhookCustomFieldInputs('discord', { foo: 'bar' })).toEqual([
      { key: 'foo', value: 'bar' },
    ]);
  });

  it('derives webhook service options from the backend template registry', () => {
    expect(
      getAlertWebhookServices([
        {
          service: 'discord',
          label: 'Discord',
          name: 'Discord Webhook',
          description: 'Discord server webhook',
          urlPattern: 'https://discord.com/api/webhooks/.../...',
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          payloadTemplate: '',
          instructions: '',
        },
        {
          service: 'generic',
          label: 'Generic',
          name: 'Generic Webhook',
          description: 'Custom webhook endpoint',
          urlPattern: '',
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          payloadTemplate: '',
          instructions: '',
        },
      ]),
    ).toEqual([
      {
        id: 'discord',
        label: 'Discord',
        description: 'Discord server webhook',
      },
      {
        id: 'generic',
        label: 'Generic',
        description: 'Custom webhook endpoint',
      },
    ]);
  });

  it('prefers backend template labels for service presentation', () => {
    expect(
      getAlertWebhookServiceLabelFromTemplates('discord', [
        {
          service: 'discord',
          label: 'Discord',
          name: 'Discord Webhook',
          description: 'Discord server webhook',
          urlPattern: 'https://discord.com/api/webhooks/.../...',
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          payloadTemplate: '',
          instructions: '',
        },
      ]),
    ).toBe('Discord');

    expect(getAlertWebhookServiceLabelFromTemplates('custom-service', [])).toBe('custom-service');
  });

  it('prefers backend template mention copy for service presentation', () => {
    const templates = [
      {
        service: 'discord',
        label: 'Discord',
        mentionPlaceholder: '@everyone or <@USER_ID> or <@&ROLE_ID>',
        mentionHelp: 'Discord: Use @everyone, @here, <@USER_ID>, or <@&ROLE_ID>',
        name: 'Discord Webhook',
        description: 'Discord server webhook',
        urlPattern: 'https://discord.com/api/webhooks/.../...',
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        payloadTemplate: '',
        instructions: '',
      },
    ];

    expect(getAlertWebhookMentionPlaceholderFromTemplates('discord', templates as any)).toBe(
      '@everyone or <@USER_ID> or <@&ROLE_ID>',
    );
    expect(getAlertWebhookMentionHelpFromTemplates('discord', templates as any)).toBe(
      'Discord: Use @everyone, @here, <@USER_ID>, or <@&ROLE_ID>',
    );
    expect(getAlertWebhookMentionPlaceholderFromTemplates('custom-service', [])).toBe('@everyone');
    expect(getAlertWebhookMentionHelpFromTemplates('custom-service', [])).toBe('');
    expect(hasAlertWebhookMentionSupportFromTemplates('discord', templates as any)).toBe(true);
    expect(hasAlertWebhookMentionSupportFromTemplates('custom-service', [])).toBe(false);
  });
});
