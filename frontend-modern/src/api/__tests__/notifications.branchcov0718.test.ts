import { beforeEach, describe, expect, it, vi } from 'vitest';

vi.mock('@/utils/apiClient', () => ({
  apiFetchJSON: vi.fn(),
}));

import { NotificationsAPI } from '@/api/notifications';
import type {
  AppriseConfig,
  EmailConfig,
  NotificationTestRequest,
  Webhook,
} from '@/api/notifications';
import { apiFetchJSON } from '@/utils/apiClient';

const apiFetchJSONMock = vi.mocked(apiFetchJSON);

// Pull the JSON body out of a recorded apiFetchJSON(url, opts) call so we can
// assert the exact request shape the module built — not just "was called".
const parseBody = (call: unknown[]): unknown => {
  const opts = call[1] as { body?: string } | undefined;
  return opts?.body ? JSON.parse(opts.body) : undefined;
};

describe('NotificationsAPI — branch coverage (0718)', () => {
  beforeEach(() => {
    apiFetchJSONMock.mockReset();
  });

  describe('getAppriseConfig', () => {
    it('GETs /apprise with no options and returns the parsed payload verbatim', async () => {
      const payload: AppriseConfig = {
        enabled: true,
        mode: 'http',
        targets: ['tgram://abc'],
        serverUrl: 'https://apprise.local',
        apiKey: 'k',
      };
      apiFetchJSONMock.mockResolvedValueOnce(payload as any);

      const result = await NotificationsAPI.getAppriseConfig();

      expect(apiFetchJSONMock).toHaveBeenCalledTimes(1);
      expect(apiFetchJSONMock).toHaveBeenCalledWith('/api/notifications/apprise');
      expect(result).toEqual(payload);
    });
  });

  describe('updateAppriseConfig', () => {
    it('PUTs to /apprise serializing the config object verbatim (no field rewrite)', async () => {
      const config: AppriseConfig = {
        enabled: true,
        mode: 'cli',
        targets: ['mailto://a@b', 'tgram://c'],
        cliPath: '/usr/bin/apprise',
        timeoutSeconds: 15,
        serverUrl: 'https://apprise.local',
        configKey: 'pulse',
        apiKey: 'secret',
        apiKeyHeader: 'X-Api-Key',
        skipTlsVerify: false,
      };
      apiFetchJSONMock.mockResolvedValueOnce(config as any);

      const result = await NotificationsAPI.updateAppriseConfig(config);

      expect(apiFetchJSONMock).toHaveBeenCalledWith('/api/notifications/apprise', {
        method: 'PUT',
        body: JSON.stringify(config),
      });
      expect(parseBody(apiFetchJSONMock.mock.calls[0])).toEqual(config);
      expect(result).toEqual(config);
    });

    it('forwards a minimal config (only the required field) untouched', async () => {
      const minimal: AppriseConfig = { enabled: false };
      apiFetchJSONMock.mockResolvedValueOnce(minimal as any);

      await NotificationsAPI.updateAppriseConfig(minimal);

      expect(apiFetchJSONMock.mock.calls[0][0]).toBe('/api/notifications/apprise');
      expect(parseBody(apiFetchJSONMock.mock.calls[0])).toEqual({ enabled: false });
    });
  });

  describe('updateEmailConfig', () => {
    const baseConfig: EmailConfig = {
      enabled: true,
      provider: 'smtp',
      server: 'smtp.example.com',
      port: 587,
      username: 'ops',
      password: 'pw',
      from: 'pulse@example.com',
      to: ['alerts@example.com', 'ops@example.com'],
      tls: true,
      startTLS: false,
    };

    it('PUTs to /email WITHOUT rateLimit when rateLimit is undefined', async () => {
      apiFetchJSONMock.mockResolvedValueOnce({ success: true } as any);

      const result = await NotificationsAPI.updateEmailConfig(baseConfig);

      expect(apiFetchJSONMock).toHaveBeenCalledWith('/api/notifications/email', {
        method: 'PUT',
        body: expect.any(String),
      });
      const body = parseBody(apiFetchJSONMock.mock.calls[0]);
      // The "rateLimit is undefined" branch: key must be absent entirely.
      expect(body).not.toHaveProperty('rateLimit');
      // All other fields pass through with their original names (no rename).
      expect(body).toEqual({
        enabled: true,
        server: 'smtp.example.com',
        port: 587,
        username: 'ops',
        password: 'pw',
        from: 'pulse@example.com',
        to: ['alerts@example.com', 'ops@example.com'],
        tls: true,
        startTLS: false,
        provider: 'smtp',
      });
      expect(result).toEqual({ success: true });
    });

    it('PUTs to /email WITH rateLimit=0 (rateLimit present-vs-absent branch keeps falsy-but-defined)', async () => {
      apiFetchJSONMock.mockResolvedValueOnce({ success: true } as any);

      await NotificationsAPI.updateEmailConfig({ ...baseConfig, rateLimit: 0 });

      const body = parseBody(apiFetchJSONMock.mock.calls[0]);
      // The `!== undefined` check preserves 0 — distinct from the omit branch.
      expect(body).toHaveProperty('rateLimit', 0);
    });

    it('PUTs to /email WITH rateLimit when set to a positive integer', async () => {
      apiFetchJSONMock.mockResolvedValueOnce({ success: true } as any);

      await NotificationsAPI.updateEmailConfig({ ...baseConfig, rateLimit: 120 });

      const body = parseBody(apiFetchJSONMock.mock.calls[0]);
      expect(body).toHaveProperty('rateLimit', 120);
    });
  });

  describe('createWebhook', () => {
    it('POSTs to /webhooks with the webhook payload serialized verbatim', async () => {
      const webhook: Omit<Webhook, 'id'> = {
        name: 'PagerDuty',
        url: 'https://events.pagerduty.com/v2/enqueue',
        method: 'POST',
        headers: { 'Content-Type': 'application/json', Authorization: 'Token T' },
        enabled: true,
        service: 'pagerduty',
        customFields: { routing_key: 'abc' },
        mention: '@everyone',
      };
      apiFetchJSONMock.mockResolvedValueOnce({ id: 'wh-1', ...webhook } as any);

      const result = await NotificationsAPI.createWebhook(webhook);

      expect(apiFetchJSONMock).toHaveBeenCalledWith('/api/notifications/webhooks', {
        method: 'POST',
        body: JSON.stringify(webhook),
      });
      expect(parseBody(apiFetchJSONMock.mock.calls[0])).toEqual(webhook);
      expect(result).toEqual({ id: 'wh-1', ...webhook });
    });
  });

  describe('updateWebhook', () => {
    it('PUTs to /webhooks/:id with the partial body verbatim', async () => {
      const partial: Partial<Webhook> = { enabled: false, mention: '<@123>' };
      apiFetchJSONMock.mockResolvedValueOnce({ id: 'wh-7', ...partial } as any);

      const result = await NotificationsAPI.updateWebhook('wh-7', partial);

      expect(apiFetchJSONMock).toHaveBeenCalledWith('/api/notifications/webhooks/wh-7', {
        method: 'PUT',
        body: JSON.stringify(partial),
      });
      expect(result).toEqual({ id: 'wh-7', enabled: false, mention: '<@123>' });
    });

    it('URL-encodes special characters in the webhook id path segment', async () => {
      apiFetchJSONMock.mockResolvedValueOnce({ id: 'a/b c?x=1' } as any);

      await NotificationsAPI.updateWebhook('a/b c?x=1', { enabled: true });

      // encodeURIComponent branch — '/', '?', '=', and space all encoded.
      const expectedPath = `/api/notifications/webhooks/${encodeURIComponent('a/b c?x=1')}`;
      expect(apiFetchJSONMock.mock.calls[0][0]).toBe(expectedPath);
      expect(apiFetchJSONMock.mock.calls[0][0]).toBe(
        '/api/notifications/webhooks/a%2Fb%20c%3Fx%3D1',
      );
      expect(parseBody(apiFetchJSONMock.mock.calls[0])).toEqual({ enabled: true });
    });
  });

  describe('getEmailProviders', () => {
    it('GETs /email-providers with no options and returns the parsed list verbatim', async () => {
      const providers = [
        {
          id: 'gmail',
          name: 'Gmail',
          smtpHost: 'smtp.gmail.com',
          smtpPort: 587,
          tls: true,
          startTLS: false,
          authRequired: true,
          instructions: 'Use an app password.',
        },
      ];
      apiFetchJSONMock.mockResolvedValueOnce(providers as any);

      const result = await NotificationsAPI.getEmailProviders();

      expect(apiFetchJSONMock).toHaveBeenCalledWith('/api/notifications/email-providers');
      expect(result).toBe(providers);
    });
  });

  describe('testNotification', () => {
    it('includes config and omits webhookId for a config-only email test', async () => {
      apiFetchJSONMock.mockResolvedValueOnce({ success: true, message: 'sent' } as any);

      const config = { server: 'smtp.x', port: 25 };
      const result = await NotificationsAPI.testNotification({
        type: 'email',
        config,
      });

      expect(apiFetchJSONMock).toHaveBeenCalledWith('/api/notifications/test', {
        method: 'POST',
        body: JSON.stringify({ method: 'email', config }),
      });
      expect(result).toEqual({ success: true, message: 'sent' });
    });

    it('includes webhookId and omits config for a webhook-id-only test', async () => {
      apiFetchJSONMock.mockResolvedValueOnce({ success: false, message: 'no webhook' } as any);

      await NotificationsAPI.testNotification({
        type: 'webhook',
        webhookId: 'wh-9',
      });

      expect(apiFetchJSONMock).toHaveBeenCalledWith('/api/notifications/test', {
        method: 'POST',
        body: JSON.stringify({ method: 'webhook', webhookId: 'wh-9' }),
      });
    });

    it('includes BOTH config and webhookId when both are supplied', async () => {
      apiFetchJSONMock.mockResolvedValueOnce({ success: true } as any);

      const appriseCfg: AppriseConfig = {
        enabled: true,
        mode: 'cli',
        targets: ['tgram://x'],
      };
      await NotificationsAPI.testNotification({
        type: 'apprise',
        config: appriseCfg,
        webhookId: 'wh-1',
      });

      expect(apiFetchJSONMock).toHaveBeenCalledWith('/api/notifications/test', {
        method: 'POST',
        body: JSON.stringify({
          method: 'apprise',
          config: appriseCfg,
          webhookId: 'wh-1',
        }),
      });
    });

    it('omits BOTH config and webhookId when neither is supplied', async () => {
      apiFetchJSONMock.mockResolvedValueOnce({ success: true } as any);

      await NotificationsAPI.testNotification({ type: 'email' });

      expect(apiFetchJSONMock).toHaveBeenCalledWith('/api/notifications/test', {
        method: 'POST',
        body: JSON.stringify({ method: 'email' }),
      });
    });

    it('treats a falsy config value as absent (truthy-check branch)', async () => {
      apiFetchJSONMock.mockResolvedValueOnce({ success: true } as any);

      // NotificationTestRequest.config is typed as object-only; cast an
      // empty string through `unknown` to exercise the `if (request.config)`
      // falsy branch without leaving a tsc error.
      const req = {
        type: 'email',
        config: '',
      } as unknown as NotificationTestRequest;
      await NotificationsAPI.testNotification(req);

      const body = parseBody(apiFetchJSONMock.mock.calls[0]);
      expect(body).toEqual({ method: 'email' });
      expect(body).not.toHaveProperty('config');
    });

    it('propagates transport errors unchanged (non-ok response arm)', async () => {
      const err = new Error('boom');
      apiFetchJSONMock.mockRejectedValueOnce(err);

      await expect(NotificationsAPI.testNotification({ type: 'email' })).rejects.toBe(err);

      expect(apiFetchJSONMock).toHaveBeenCalledWith('/api/notifications/test', {
        method: 'POST',
        body: JSON.stringify({ method: 'email' }),
      });
    });
  });

  describe('testWebhook', () => {
    it('POSTs to /webhooks/test with the webhook payload serialized verbatim', async () => {
      const webhook: Omit<Webhook, 'id'> = {
        name: 'Slack',
        url: 'https://hooks.slack.com/services/x',
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        enabled: true,
        service: 'slack',
        mention: '<@channel>',
      };
      apiFetchJSONMock.mockResolvedValueOnce({ success: true, message: 'ok' } as any);

      const result = await NotificationsAPI.testWebhook(webhook);

      expect(apiFetchJSONMock).toHaveBeenCalledWith(
        '/api/notifications/webhooks/test',
        {
          method: 'POST',
          body: JSON.stringify(webhook),
        },
      );
      expect(parseBody(apiFetchJSONMock.mock.calls[0])).toEqual(webhook);
      expect(result).toEqual({ success: true, message: 'ok' });
    });
  });
});
