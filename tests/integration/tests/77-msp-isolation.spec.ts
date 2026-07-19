import { createHmac } from 'node:crypto';
import { spawnSync } from 'node:child_process';
import http from 'node:http';
import type { AddressInfo } from 'node:net';
import { expect, test } from '@playwright/test';
import {
  apiRequest,
  createOrg,
  deleteOrg,
  ensureAuthenticated,
  isMultiTenantEnabled,
} from './helpers';

/**
 * MSP isolation E2E: automates the validation checklist from docs/MSP.md
 * against the real server, plus the signed webhook delivery contract from
 * docs/WEBHOOKS.md. These are the cross-layer seams unit tests cannot see:
 * org-bound token scoping at the authorization boundary, instance-wide
 * webhook security settings reaching tenant org managers, the dedicated
 * agent-ingest listener, and signed tenant-stamped webhook delivery.
 */

type APITokenCreateResponse = {
  token?: string;
};

type CapturedDelivery = {
  headers: Record<string, string | string[] | undefined>;
  body: string;
};

const WEBHOOK_SIGNING_SECRET = 'e2e-msp-signing-secret';

// Private ranges that cover Docker bridge/compose networks and the
// host-gateway address, so the capture listener passes SSRF validation once
// the instance-wide allowlist is configured.
const PRIVATE_ALLOWLIST = '10.0.0.0/8,172.16.0.0/12,192.168.0.0/16';

const agentPortBaseURL = () => {
  const host = process.env.PULSE_E2E_HOST || '127.0.0.1';
  const port = process.env.PULSE_E2E_AGENT_PORT || '7656';
  return `http://${host}:${port}`;
};

function startCaptureServer(): Promise<{
  server: http.Server;
  port: number;
  deliveries: CapturedDelivery[];
}> {
  const deliveries: CapturedDelivery[] = [];
  const server = http.createServer((req, res) => {
    let body = '';
    req.on('data', (chunk) => {
      body += chunk;
    });
    req.on('end', () => {
      deliveries.push({ headers: { ...req.headers }, body });
      res.writeHead(200, { 'Content-Type': 'text/plain' });
      res.end('ok');
    });
  });
  return new Promise((resolve, reject) => {
    server.on('error', reject);
    // Bind all interfaces so the Pulse container can reach us through
    // host.docker.internal (mapped via extra_hosts host-gateway).
    server.listen(0, '0.0.0.0', () => {
      resolve({ server, port: (server.address() as AddressInfo).port, deliveries });
    });
  });
}

async function setWebhookAllowlist(page: any, cidrs: string) {
  const res = await apiRequest(page, '/api/system/settings/update', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    data: { webhookAllowedPrivateCIDRs: cidrs },
  });
  expect(res.ok(), `allowlist update: ${res.status()}`).toBeTruthy();
}

test.describe('MSP isolation E2E', () => {
  test.setTimeout(180_000);

  test('org-bound token is scoped away from other orgs and the default org', async ({ page }) => {
    await ensureAuthenticated(page);
    const mtEnabled = await isMultiTenantEnabled(page);
    test.skip(!mtEnabled, 'Multi-tenant feature not enabled in this environment');

    const orgA = await createOrg(page, `MSP Client A ${Date.now()}`);
    const orgB = await createOrg(page, `MSP Client B ${Date.now()}`);

    try {
      const createTokenRes = await apiRequest(page, '/api/security/tokens', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          'X-Pulse-Org-ID': orgA.id,
        },
        data: { name: `msp-e2e-read-${Date.now()}`, scopes: ['monitoring:read'] },
        timeout: 30_000,
      });
      expect(createTokenRes.ok()).toBeTruthy();
      const token = ((await createTokenRes.json()) as APITokenCreateResponse).token?.trim() || '';
      expect(token.length).toBeGreaterThan(0);

      const probe = async (orgID: string | null) => {
        const headers: Record<string, string> = { 'X-API-Token': token };
        if (orgID) headers['X-Pulse-Org-ID'] = orgID;
        const res = await apiRequest(page, '/api/alerts/active', { headers });
        return res.status();
      };

      // Own org: allowed (org-bound token auto-routes without a header too).
      expect(await probe(orgA.id)).toBe(200);
      expect(await probe(null)).toBe(200);
      // Sibling client org: denied.
      expect(await probe(orgB.id)).toBe(403);
      // Default org: denied. A leaked client-site token must not read the
      // provider's own estate (docs/MSP.md validation checklist item 4).
      expect(await probe('default')).toBe(403);
    } finally {
      await deleteOrg(page, orgB.id);
      await deleteOrg(page, orgA.id);
    }
  });

  test('agent ingest port serves only the agent surface', async ({ request }) => {
    // docs/MSP.md validation checklist item 1: management paths must 404 on
    // the dedicated agent port; the agent route exists but demands auth.
    const base = agentPortBaseURL();
    const root = await request.get(`${base}/`).catch(() => null);
    test.skip(!root, 'Agent ingest port not reachable in this environment');

    expect(root!.status()).toBe(404);
    const login = await request.get(`${base}/api/login`);
    expect(login.status()).toBe(404);
    const state = await request.get(`${base}/api/state`);
    expect(state.status()).toBe(404);
    const report = await request.post(`${base}/api/agents/agent/report`, { data: {} });
    expect([401, 403]).toContain(report.status());
  });

  test('client org webhook delivery: instance-wide allowlist, tenant stamp, HMAC signature', async ({ page }) => {
    await ensureAuthenticated(page);
    const mtEnabled = await isMultiTenantEnabled(page);
    test.skip(!mtEnabled, 'Multi-tenant feature not enabled in this environment');

    const { server, port, deliveries } = await startCaptureServer();
    const org = await createOrg(page, `MSP Webhook Client ${Date.now()}`);

    try {
      // Allowlist saved in the DEFAULT org context must reach the client
      // org's notification manager (instance-wide setting propagation).
      await setWebhookAllowlist(page, PRIVATE_ALLOWLIST);

      const fire = async () => {
        const res = await apiRequest(page, '/api/notifications/webhooks/test', {
          method: 'POST',
          headers: {
            'Content-Type': 'application/json',
            'X-Pulse-Org-ID': org.id,
          },
          data: {
            name: 'msp-e2e-psa',
            url: `http://host.docker.internal:${port}/hook`,
            service: 'generic',
            enabled: true,
            signingSecret: WEBHOOK_SIGNING_SECRET,
          },
          timeout: 30_000,
        });
        const payload = (await res.json()) as { success?: boolean; error?: string };
        expect(res.ok(), `webhook test endpoint: ${res.status()}`).toBeTruthy();
        expect(payload.success, `webhook delivery failed: ${payload.error || ''}`).toBeTruthy();
      };

      await fire();
      expect(deliveries.length).toBe(1);

      const delivery = deliveries[0];
      const signature = String(delivery.headers['x-pulse-signature'] || '');
      const timestamp = String(delivery.headers['x-pulse-timestamp'] || '');
      const eventID = String(delivery.headers['x-pulse-event-id'] || '');

      // Delivery contract (docs/WEBHOOKS.md): idempotency token and signed
      // payload headers.
      expect(eventID).toMatch(/:alert$/);
      expect(timestamp).toMatch(/^\d+$/);
      expect(signature.startsWith('v1=')).toBeTruthy();
      const expected =
        'v1=' +
        createHmac('sha256', WEBHOOK_SIGNING_SECRET)
          .update(`${timestamp}.${delivery.body}`)
          .digest('hex');
      expect(signature).toBe(expected);

      // Tenant identity must be stamped in the payload so a shared PSA
      // endpoint can route by client.
      const body = JSON.parse(delivery.body) as {
        tenant?: { id?: string; name?: string };
        alert?: { level?: string; type?: string };
      };
      expect(body.tenant?.id).toBe(org.id);
      expect(body.tenant?.name?.length || 0).toBeGreaterThan(0);
      expect(['warning', 'critical']).toContain(body.alert?.level || '');

      // Restart inheritance: after a server restart (tenant monitors are
      // recreated lazily) the persisted allowlist must still apply without
      // anyone re-saving settings. Skip when the docker CLI cannot manage
      // the test container (e.g. local runs against a managed stack).
      const inspect = spawnSync('docker', ['inspect', 'pulse-test-server'], { stdio: 'ignore' });
      if (inspect.status === 0) {
        const restart = spawnSync('docker', ['restart', 'pulse-test-server'], {
          stdio: 'ignore',
          timeout: 120_000,
        });
        expect(restart.status).toBe(0);

        await expect
          .poll(
            async () => {
              const res = await apiRequest(page, '/api/health').catch(() => null);
              return res ? res.status() : 0;
            },
            { timeout: 90_000, intervals: [2_000] },
          )
          .toBe(200);

        await ensureAuthenticated(page);
        await fire();
        expect(deliveries.length).toBe(2);
        const second = deliveries[1];
        expect(String(second.headers['x-pulse-signature'] || '').startsWith('v1=')).toBeTruthy();
        expect((JSON.parse(second.body) as { tenant?: { id?: string } }).tenant?.id).toBe(org.id);
      }
    } finally {
      await setWebhookAllowlist(page, '').catch(() => {});
      await deleteOrg(page, org.id).catch(() => {});
      server.close();
    }
  });
});
