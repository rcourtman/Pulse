import { beforeEach, describe, expect, it, vi } from 'vitest';

vi.mock('@/utils/apiClient', () => ({
  apiFetchJSON: vi.fn(),
}));

import { NotificationsAPI } from '@/api/notifications';
import { apiFetchJSON } from '@/utils/apiClient';

describe('NotificationsAPI', () => {
  const apiFetchJSONMock = vi.mocked(apiFetchJSON);

  beforeEach(() => {
    apiFetchJSONMock.mockReset();
  });

  it('preserves valid zero values when mapping email config', async () => {
    apiFetchJSONMock.mockResolvedValueOnce({
      enabled: false,
      provider: 'smtp',
      server: 'smtp.internal',
      port: 0,
      username: 'ops',
      password: '',
      from: 'pulse@internal',
      to: ['alerts@internal', 12],
      tls: false,
      startTLS: false,
      rateLimit: 0,
    } as any);

    const config = await NotificationsAPI.getEmailConfig();

    expect(apiFetchJSONMock).toHaveBeenCalledWith('/api/notifications/email');
    expect(config).toEqual({
      enabled: false,
      provider: 'smtp',
      server: 'smtp.internal',
      port: 0,
      username: 'ops',
      password: '',
      from: 'pulse@internal',
      to: ['alerts@internal'],
      tls: false,
      startTLS: false,
      rateLimit: 0,
    });
  });

  it('falls back to safe defaults for invalid backend field types', async () => {
    apiFetchJSONMock.mockResolvedValueOnce({
      enabled: 'true',
      provider: 99,
      server: null,
      port: '587',
      username: undefined,
      password: false,
      from: {},
      to: 'alerts@internal',
      tls: 'false',
      startTLS: 1,
      rateLimit: '0',
    } as any);

    const config = await NotificationsAPI.getEmailConfig();

    expect(config).toEqual({
      enabled: false,
      provider: '',
      server: '',
      port: 587,
      username: '',
      password: '',
      from: '',
      to: [],
      tls: false,
      startTLS: false,
      rateLimit: undefined,
    });
  });

  it('normalizes malformed webhook collections to empty arrays', async () => {
    apiFetchJSONMock.mockResolvedValueOnce({ webhooks: [] } as any);

    const result = await NotificationsAPI.getWebhooks();

    expect(apiFetchJSONMock).toHaveBeenCalledWith('/api/notifications/webhooks');
    expect(result).toEqual([]);
  });

  it('normalizes retained terminal delivery health from the API', async () => {
    apiFetchJSONMock.mockResolvedValueOnce({
      overall_healthy: false,
      queue: {
        pending: 2,
        sending: 1,
        sent: 8,
        failed: 1,
        dlq: 3,
        healthy: false,
        status: 'degraded',
        attention_required: 4,
        reason_codes: ['retained_failed_deliveries', 'retained_dead_letter_deliveries'],
        completed_retention_days: 7,
        dead_letter_retention_days: 30,
        counts_are_retention_bounded: true,
        retry_attempts_affect_health: false,
        terminal_failures_affect_health: true,
      },
    } as any);

    const health = await NotificationsAPI.getHealth();

    expect(apiFetchJSONMock).toHaveBeenCalledWith('/api/notifications/health');
    expect(health).toEqual({
      overallHealthy: false,
      queue: {
        pending: 2,
        sending: 1,
        sent: 8,
        failed: 1,
        deadLetter: 3,
        healthy: false,
        status: 'degraded',
        attentionRequired: 4,
        reasonCodes: ['retained_failed_deliveries', 'retained_dead_letter_deliveries'],
        completedRetentionDays: 7,
        deadLetterRetentionDays: 30,
        countsAreRetentionBounded: true,
        retryAttemptsAffectHealth: false,
        terminalFailuresAffectHealth: true,
      },
    });
  });

  it('fails closed when notification health fields are malformed', async () => {
    apiFetchJSONMock.mockResolvedValueOnce({
      overall_healthy: 'yes',
      queue: {
        failed: '4',
        dlq: null,
        healthy: 'yes',
        status: 'unknown',
        reason_codes: ['queue_stats_unavailable', 42],
        terminal_failures_affect_health: 'yes',
      },
    } as any);

    const health = await NotificationsAPI.getHealth();

    expect(health.overallHealthy).toBe(false);
    expect(health.queue).toEqual(
      expect.objectContaining({
        failed: 0,
        deadLetter: 0,
        healthy: false,
        status: 'unavailable',
        reasonCodes: ['queue_stats_unavailable'],
        terminalFailuresAffectHealth: true,
      }),
    );
  });

  it('fails closed when nominal health contradicts retained terminal counts', async () => {
    apiFetchJSONMock.mockResolvedValueOnce({
      overall_healthy: true,
      queue: {
        pending: 0,
        sending: 0,
        sent: 8,
        failed: 1,
        dlq: 0,
        healthy: true,
        status: 'healthy',
        attention_required: 0,
        reason_codes: [],
        completed_retention_days: 7,
        dead_letter_retention_days: 30,
        counts_are_retention_bounded: true,
        retry_attempts_affect_health: false,
        terminal_failures_affect_health: true,
      },
    } as any);

    const health = await NotificationsAPI.getHealth();

    expect(health.queue).toEqual(
      expect.objectContaining({
        healthy: false,
        status: 'unavailable',
        failed: 1,
      }),
    );
    expect(health.overallHealthy).toBe(false);
  });

  it('surfaces webhook template labels from the API', async () => {
    apiFetchJSONMock.mockResolvedValueOnce([
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
    ] as any);

    const templates = await NotificationsAPI.getWebhookTemplates();

    expect(apiFetchJSONMock).toHaveBeenCalledWith('/api/notifications/webhook-templates');
    expect(templates).toEqual([
      expect.objectContaining({
        service: 'discord',
        label: 'Discord',
        description: 'Discord server webhook',
      }),
    ]);
  });
});
