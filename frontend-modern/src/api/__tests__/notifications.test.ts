import { beforeEach, describe, expect, it, vi } from 'vitest';

vi.mock('@/utils/apiClient', () => ({
  apiFetchJSON: vi.fn(),
}));

import { NotificationsAPI, type NotificationTestRequest, type Webhook } from '@/api/notifications';
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
      minimumSeverity: 'all',
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
      minimumSeverity: 'all',
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
      minimumSeverity: 'all',
    });
  });

  it('maps email tag routing and sends explicit routing updates', async () => {
    apiFetchJSONMock.mockResolvedValueOnce({
      enabled: true,
      provider: 'smtp',
      server: 'smtp.internal',
      port: 587,
      username: 'ops',
      password: '',
      from: 'pulse@internal',
      to: ['alerts@internal'],
      tls: false,
      startTLS: true,
      tagFilter: ['customer:alpha', 'critical'],
      tagFilterMode: 'any',
    });

    const config = await NotificationsAPI.getEmailConfig();

    expect(config).toEqual(
      expect.objectContaining({
        tagFilter: ['customer:alpha', 'critical'],
        tagFilterMode: 'any',
      }),
    );

    apiFetchJSONMock.mockResolvedValueOnce({ success: true });
    await NotificationsAPI.updateEmailConfig({
      ...config,
      tagFilter: [],
      tagFilterMode: 'all',
    });

    expect(apiFetchJSONMock).toHaveBeenLastCalledWith(
      '/api/notifications/email',
      expect.objectContaining({
        method: 'PUT',
        body: expect.stringContaining('"tagFilter":[]'),
      }),
    );
    expect(JSON.parse(apiFetchJSONMock.mock.calls.at(-1)?.[1]?.body as string)).toEqual(
      expect.objectContaining({
        tagFilter: [],
        tagFilterMode: 'all',
      }),
    );
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
        failure_classes_7d: {
          authentication: 3,
          rate_limited: 0,
          connectivity: 1,
          tls: 0,
          configuration: 0,
          rejected: 0,
          server_error: 0,
          unknown: 0,
        },
        failure_classes_available: true,
        failure_class_window_days: 7,
      },
    } as any);

    const health = await NotificationsAPI.getHealth();

    expect(apiFetchJSONMock).toHaveBeenCalledWith('/api/notifications/health', undefined);
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
        failureClasses7d: {
          authentication: 3,
          rate_limited: 0,
          connectivity: 1,
          tls: 0,
          configuration: 0,
          rejected: 0,
          server_error: 0,
          unknown: 0,
        },
        failureClassesAvailable: true,
        failureClassWindowDays: 7,
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

  it('keeps readable delivery evidence and explicitly marks dropped malformed entries', async () => {
    apiFetchJSONMock.mockResolvedValueOnce({
      entries: [
        {
          notificationId: 'webhook-1',
          type: 'webhook',
          destinationId: 'webhook:wh-ops',
          outcome: 'dead_letter',
          alertIds: ['vm-offline-101'],
          alertCount: 1,
          attempts: 3,
          success: false,
          errorMessage: 'HTTP 401 Unauthorized',
          failureClass: 'authentication',
          timestamp: '2026-08-20T12:00:00Z',
        },
        // Malformed rows must be dropped, not rendered: unknown outcome,
        // missing timestamp, and a non-object entry.
        {
          notificationId: 'bad-outcome',
          type: 'email',
          outcome: 'exploded',
          timestamp: '2026-08-20T12:00:00Z',
        },
        { notificationId: 'no-timestamp', type: 'email', outcome: 'sent' },
        'not-an-object',
        {
          notificationId: 'email-1',
          type: 'email',
          outcome: 'sent',
          alertIds: ['disk-critical-1'],
          alertCount: 1,
          attempts: 1,
          success: true,
          // An unrecognized failure class is omitted rather than passed through.
          failureClass: 'made-up',
          timestamp: '2026-08-20T11:00:00Z',
        },
      ],
      window_days: 30,
      completed_retention_days: 7,
      dead_letter_retention_days: 30,
    } as any);

    const log = await NotificationsAPI.getDeliveryLog(25);

    expect(apiFetchJSONMock).toHaveBeenCalledWith(
      '/api/notifications/delivery-log?limit=25',
      undefined,
    );
    expect(log.windowDays).toBe(30);
    expect(log.completedRetentionDays).toBe(7);
    expect(log.deadLetterRetentionDays).toBe(30);
    expect(log.entries).toHaveLength(2);
    expect(log.incomplete).toBe(true);
    expect(log.entries[0]).toEqual(
      expect.objectContaining({
        notificationId: 'webhook-1',
        outcome: 'dead_letter',
        destinationId: 'webhook:wh-ops',
        failureClass: 'authentication',
        errorMessage: 'HTTP 401 Unauthorized',
      }),
    );
    expect(log.entries[1].failureClass).toBeUndefined();
  });

  it('requests the delivery log without a limit query when none is given', async () => {
    apiFetchJSONMock.mockResolvedValueOnce({ entries: [], window_days: 0 } as any);

    const log = await NotificationsAPI.getDeliveryLog();

    expect(apiFetchJSONMock).toHaveBeenCalledWith('/api/notifications/delivery-log', undefined);
    expect(log.entries).toEqual([]);
    expect(log.incomplete).toBe(false);
    // A missing or nonsensical window falls back to the seven-day default.
    expect(log.windowDays).toBe(7);
    expect(log.completedRetentionDays).toBe(7);
    expect(log.deadLetterRetentionDays).toBe(7);
  });

  it.each([null, {}, [], { entries: null }, { entries: {} }, { entries: 'unreadable' }])(
    'rejects an unreadable activity envelope instead of inventing an empty log: %j',
    async (payload) => {
      apiFetchJSONMock.mockResolvedValueOnce(payload as never);
      await expect(NotificationsAPI.getDeliveryLog()).rejects.toThrow(
        'Notification delivery log could not be read.',
      );
    },
  );

  const readableAttempt = {
    notificationId: 'email-1',
    type: 'email',
    outcome: 'sent',
    alertIds: ['disk-1'],
    alertCount: 1,
    attempts: 1,
    success: true,
    timestamp: '2026-10-20T12:00:00Z',
  };
  it.each([
    { notificationId: ' ' },
    { type: null },
    { timestamp: 'not-a-time' },
    { outcome: 'unknown' },
    { success: false },
    { success: 'true' },
    { attempts: -1 },
    { attempts: 0.5 },
    { alertCount: null },
    { alertIds: [1] },
    { alertIds: null },
    { attempts: Number.MAX_SAFE_INTEGER + 1 },
  ])('marks unreadable attempt evidence as incomplete: %j', async (patch) => {
    apiFetchJSONMock.mockResolvedValueOnce({
      entries: [{ ...readableAttempt, ...patch }],
    } as never);
    expect(await NotificationsAPI.getDeliveryLog()).toMatchObject({
      entries: [],
      incomplete: true,
    });
  });

  it.each(['retryTerminalFailures', 'dismissTerminalFailures'] as const)(
    '%s requires an explicit acknowledgement and exact safe affected count',
    async (action) => {
      for (const affected of [0, 1, 3]) {
        apiFetchJSONMock.mockResolvedValueOnce({ success: true, affected } as never);
        expect(await NotificationsAPI[action]()).toEqual({ success: true, affected });
      }
      expect(apiFetchJSONMock).toHaveBeenCalledWith(
        `/api/notifications/terminal-failures/${action === 'retryTerminalFailures' ? 'retry' : 'dismiss'}`,
        { method: 'POST' },
      );
    },
  );

  describe.each(['retryTerminalFailures', 'dismissTerminalFailures'] as const)('%s', (action) => {
    it.each([
      null,
      {},
      { success: false, affected: 2 },
      { success: 'yes', affected: 2 },
      { success: true },
      { success: true, affected: -1 },
      { success: true, affected: 1.5 },
      { success: true, affected: '3' },
      { success: true, affected: Number.MAX_SAFE_INTEGER + 1 },
    ])('never treats a rejected or malformed result as success: %j', async (payload) => {
      apiFetchJSONMock.mockResolvedValueOnce(payload as never);
      await expect(NotificationsAPI[action]()).rejects.toThrow(
        'Notification recovery result could not be confirmed.',
      );
    });
  });

  it('passes the deliveryPaused flag through from test-send responses', async () => {
    apiFetchJSONMock.mockResolvedValueOnce({
      status: 'success',
      message: 'Test notification sent, but alert delivery is paused',
      deliveryPaused: true,
    } as any);

    const result = await NotificationsAPI.testNotification({ type: 'email' });

    expect(result.deliveryPaused).toBe(true);
    expect(result.status).toBe('success');
  });
});

describe('email minimum severity save/reload (#2069)', () => {
  it.each(['all', 'warning', 'critical'] as const)(
    'preserves %s through the API adapter',
    async (minimumSeverity) => {
      const mock = vi.mocked(apiFetchJSON);
      mock.mockReset();
      mock.mockResolvedValueOnce({ minimumSeverity });
      const loaded = await NotificationsAPI.getEmailConfig();
      expect(loaded.minimumSeverity).toBe(minimumSeverity);
      mock.mockResolvedValueOnce({ success: true });
      await NotificationsAPI.updateEmailConfig(loaded);
      const saved = JSON.parse(String(mock.mock.calls[1][1]?.body));
      expect(saved.minimumSeverity).toBe(minimumSeverity);
      mock.mockResolvedValueOnce(saved);
      expect((await NotificationsAPI.getEmailConfig()).minimumSeverity).toBe(minimumSeverity);
    },
  );
});

const fetchJSON = vi.mocked(apiFetchJSON);
const unconfirmed = 'Test result could not be confirmed. Check the destination before sending another test.';
const draft: Omit<Webhook, 'id'> = {
  name: 'Synthetic destination',
  url: 'https://example.test/notifications',
  method: 'POST',
  headers: {},
  enabled: true,
};
beforeEach(() => fetchJSON.mockReset());

// The public producer emits status: "success" for all three methods. An HTTP
// 200, a truthy string or a bare success boolean is not that acknowledgement.
describe.each(['email', 'apprise', 'webhook'] as const)('%s test acknowledgement', (type) => {
  it.each([
    null, undefined, [], ['success'], {}, '', true,
    { success: true }, { success: false }, { status: 'failed' },
    { status: true }, { status: 'SUCCESS' }, { status: 200 },
    { status: 'success', success: false },
    { status: 'success', success: 'true' },
    { status: 'success', error: 'untrusted-destination-diagnostic' },
    { status: 'success', error: null },
    { status: 'success', deliveryPaused: 'false' },
    { status: 'success', deliveryPaused: null },
    { deliveryPaused: true },
  ])('rejects a malformed or contradictory 2xx response: %j', async (payload) => {
    fetchJSON.mockResolvedValueOnce(payload);
    await expect(NotificationsAPI.testNotification({ type })).rejects.toThrow(unconfirmed);
    expect(fetchJSON).toHaveBeenCalledTimes(1); // No automatic second send.
  });

  it.each([undefined, false, true])('preserves valid delivery pause evidence: %s', async (deliveryPaused) => {
    const payload = { status: 'success', deliveryPaused, message: 'Test notification sent' };
    fetchJSON.mockResolvedValueOnce(payload);
    await expect(NotificationsAPI.testNotification({ type })).resolves.toEqual(payload);
    expect(fetchJSON).toHaveBeenCalledTimes(1);
  });

  it('preserves the original transport failure and detail without a second send', async () => {
    const failure = Object.assign(new Error('Synthetic provider failure'), { detail: 'Safe fixture detail' });
    fetchJSON.mockRejectedValueOnce(failure);
    await expect(NotificationsAPI.testNotification({ type })).rejects.toBe(failure);
    expect(fetchJSON).toHaveBeenCalledTimes(1);
  });
});

describe('unsaved webhook test acknowledgement', () => {
  it.each([
    null, undefined, [], {}, '', true,
    { status: 'success' }, { success: false }, { success: 'true' },
    { success: 1 }, { success: true, error: 'untrusted-destination-diagnostic' },
    { success: true, error: null }, { success: true, status: 199 },
    { success: true, status: 300 }, { success: true, status: 400 },
    { success: true, status: 200.5 }, { success: true, status: '200' },
    { success: true, status: 'success' }, { success: true, status: null },
    { success: true, deliveryPaused: 'true' }, { success: true, deliveryPaused: null },
    { success: false, deliveryPaused: true },
  ])('rejects malformed or contradictory 2xx response: %j', async (payload) => {
    fetchJSON.mockResolvedValueOnce(payload);
    await expect(NotificationsAPI.testWebhook(draft)).rejects.toThrow(unconfirmed);
    expect(fetchJSON).toHaveBeenCalledTimes(1);
  });

  it.each([undefined, 200, 202, 204, 299])('accepts an affirmative send with compatible status %s', async (status) => {
    fetchJSON.mockResolvedValueOnce({ success: true, status, response: 'Not consumed' });
    await expect(NotificationsAPI.testWebhook(draft)).resolves.toEqual({ success: true });
  });

  it.each([false, true])('keeps the pause caveat: %s', async (deliveryPaused) => {
    fetchJSON.mockResolvedValueOnce({ success: true, status: 200, deliveryPaused });
    await expect(NotificationsAPI.testWebhook(draft)).resolves.toEqual({ success: true, deliveryPaused });
  });
});

it('keeps the saved, draft and email/config request bodies distinct and unchanged', async () => {
  const requests: NotificationTestRequest[] = [
    { type: 'email', config: { server: 'smtp.example.test', port: 587 } },
    { type: 'apprise', config: { enabled: true, mode: 'cli', targets: ['mailto://operator@example.test'] } },
    { type: 'webhook', webhookId: 'saved-hook' },
  ];
  for (const { type, ...request } of requests) {
    fetchJSON.mockResolvedValueOnce({ status: 'success' });
    await NotificationsAPI.testNotification({ type, ...request });
    expect(fetchJSON).toHaveBeenLastCalledWith('/api/notifications/test', {
      method: 'POST', body: JSON.stringify({ method: type, ...request }),
    });
  }
  fetchJSON.mockResolvedValueOnce({ success: true, status: 200 });
  await NotificationsAPI.testWebhook(draft);
  expect(fetchJSON).toHaveBeenLastCalledWith('/api/notifications/webhooks/test', {
    method: 'POST', body: JSON.stringify(draft),
  });
});
