import { describe, expect, it } from 'vitest';
import type { WebhookTemplate } from '@/api/notifications';
import {
  getAlertWebhookServices,
  hasAlertWebhookMentionSupportFromTemplates,
  getAlertWebhooksSectionTitle,
  getAlertWebhooksSectionDescription,
  getAlertWebhookMutationFailure,
} from '@/utils/alertWebhookPresentation';

function makeTemplate(overrides: Partial<WebhookTemplate> = {}): WebhookTemplate {
  return {
    service: 'discord',
    name: 'Discord Webhook',
    urlPattern: 'https://discord.com/api/webhooks/.../...',
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    payloadTemplate: '',
    instructions: '',
    ...overrides,
  };
}

describe('alertWebhookPresentation (branch coverage 2)', () => {
  describe('getAlertWebhookServices', () => {
    it('returns an empty array when no templates arg is provided (default param)', () => {
      expect(getAlertWebhookServices()).toEqual([]);
    });

    it('returns an empty array for an explicit empty list', () => {
      expect(getAlertWebhookServices([])).toEqual([]);
    });

    it('uses the template label and description when both are present', () => {
      const templates = [
        makeTemplate({
          service: 'discord',
          label: 'Discord',
          description: 'Discord server webhook',
        }),
      ];
      expect(getAlertWebhookServices(templates)).toEqual([
        {
          id: 'discord',
          label: 'Discord',
          description: 'Discord server webhook',
        },
      ]);
    });

    it('falls back to the canonical service label when label is missing', () => {
      const templates = [
        makeTemplate({ service: 'slack', label: undefined, description: 'Slack channel' }),
      ];
      expect(getAlertWebhookServices(templates)).toEqual([
        {
          id: 'slack',
          label: 'Slack',
          description: 'Slack channel',
        },
      ]);
    });

    it('falls back to the canonical service label when label is an empty string', () => {
      const templates = [
        makeTemplate({ service: 'telegram', label: '', description: 'Telegram chat' }),
      ];
      expect(getAlertWebhookServices(templates)).toEqual([
        {
          id: 'telegram',
          label: 'Telegram',
          description: 'Telegram chat',
        },
      ]);
    });

    it('falls back to template.name when description is missing', () => {
      const templates = [
        makeTemplate({
          service: 'discord',
          label: 'Discord',
          description: undefined,
          name: 'Discord Webhook',
        }),
      ];
      expect(getAlertWebhookServices(templates)).toEqual([
        {
          id: 'discord',
          label: 'Discord',
          description: 'Discord Webhook',
        },
      ]);
    });

    it('falls back to template.name when description is an empty string', () => {
      const templates = [
        makeTemplate({
          service: 'generic',
          label: 'Generic',
          description: '',
          name: 'Generic Webhook',
        }),
      ];
      expect(getAlertWebhookServices(templates)).toEqual([
        {
          id: 'generic',
          label: 'Generic',
          description: 'Generic Webhook',
        },
      ]);
    });

    it('echoes the raw service id when the service is unknown and no label/description', () => {
      const templates = [
        makeTemplate({
          service: 'totally-unknown',
          label: undefined,
          description: undefined,
          name: 'Mystery Hook',
        }),
      ];
      expect(getAlertWebhookServices(templates)).toEqual([
        {
          id: 'totally-unknown',
          label: 'totally-unknown',
          description: 'Mystery Hook',
        },
      ]);
    });

    it('preserves order across multiple templates with mixed fallbacks', () => {
      const templates = [
        makeTemplate({ service: 'discord', label: 'Discord', description: 'desc-d' }),
        makeTemplate({
          service: 'slack',
          label: undefined,
          description: undefined,
          name: 'Slack Webhook',
        }),
      ];
      expect(getAlertWebhookServices(templates)).toEqual([
        { id: 'discord', label: 'Discord', description: 'desc-d' },
        { id: 'slack', label: 'Slack', description: 'Slack Webhook' },
      ]);
    });
  });

  describe('hasAlertWebhookMentionSupportFromTemplates', () => {
    it('returns false when no templates are provided (default param)', () => {
      expect(hasAlertWebhookMentionSupportFromTemplates('discord')).toBe(false);
    });

    it('returns false when no template matches the service', () => {
      expect(
        hasAlertWebhookMentionSupportFromTemplates('unknown', [
          makeTemplate({ service: 'discord' }),
        ]),
      ).toBe(false);
    });

    it('returns true when a matching template has a non-empty mentionPlaceholder', () => {
      const templates = [
        makeTemplate({
          service: 'discord',
          mentionPlaceholder: '@here',
          mentionHelp: undefined,
        }),
      ];
      expect(hasAlertWebhookMentionSupportFromTemplates('discord', templates)).toBe(true);
    });

    it('returns true when mentionPlaceholder is absent but mentionHelp is non-empty', () => {
      const templates = [
        makeTemplate({
          service: 'slack',
          mentionPlaceholder: undefined,
          mentionHelp: 'Slack: use @here',
        }),
      ];
      expect(hasAlertWebhookMentionSupportFromTemplates('slack', templates)).toBe(true);
    });

    it('returns true when mentionPlaceholder is whitespace-only but mentionHelp is non-empty', () => {
      const templates = [
        makeTemplate({
          service: 'slack',
          mentionPlaceholder: '   ',
          mentionHelp: 'Slack: use @here',
        }),
      ];
      expect(hasAlertWebhookMentionSupportFromTemplates('slack', templates)).toBe(true);
    });

    it('returns false when both mentionPlaceholder and mentionHelp are whitespace-only', () => {
      const templates = [
        makeTemplate({
          service: 'discord',
          mentionPlaceholder: '  ',
          mentionHelp: '\t',
        }),
      ];
      expect(hasAlertWebhookMentionSupportFromTemplates('discord', templates)).toBe(false);
    });

    it('returns false when both mention fields are absent on the matching template', () => {
      const templates = [
        makeTemplate({
          service: 'telegram',
          mentionPlaceholder: undefined,
          mentionHelp: undefined,
        }),
      ];
      expect(hasAlertWebhookMentionSupportFromTemplates('telegram', templates)).toBe(false);
    });

    it('uses the first matching template when duplicates exist', () => {
      const templates = [
        makeTemplate({ service: 'discord', mentionPlaceholder: 'first-support' }),
        makeTemplate({
          service: 'discord',
          mentionPlaceholder: undefined,
          mentionHelp: undefined,
        }),
      ];
      expect(hasAlertWebhookMentionSupportFromTemplates('discord', templates)).toBe(true);
    });
  });

  describe('getAlertWebhooksSectionTitle', () => {
    it('returns the canonical section title', () => {
      expect(getAlertWebhooksSectionTitle()).toBe('Webhooks');
    });
  });

  describe('getAlertWebhooksSectionDescription', () => {
    it('returns the canonical section description', () => {
      expect(getAlertWebhooksSectionDescription()).toBe(
        'Push alerts to chat apps or automation systems.',
      );
    });
  });

  describe('getAlertWebhookMutationFailure', () => {
    it('returns the add failure message', () => {
      expect(getAlertWebhookMutationFailure('add')).toBe('Failed to add webhook');
    });

    it('returns the update failure message', () => {
      expect(getAlertWebhookMutationFailure('update')).toBe('Failed to update webhook');
    });

    it('returns the delete failure message for the delete action', () => {
      expect(getAlertWebhookMutationFailure('delete')).toBe('Failed to delete webhook');
    });

    it('returns the delete failure message for any non-matching action (default arm)', () => {
      expect(
        getAlertWebhookMutationFailure(
          'totally-bogus' as unknown as Parameters<
            typeof getAlertWebhookMutationFailure
          >[0],
        ),
      ).toBe('Failed to delete webhook');
    });
  });
});
