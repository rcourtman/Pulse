import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { expect, test as base } from '@playwright/test';

import { createAuthenticatedStorageState } from './helpers';

const __dirname = path.dirname(fileURLToPath(import.meta.url));
type WorkerFixtures = {
  authStorageStatePath: string;
};

const test = base.extend<{}, WorkerFixtures>({
  storageState: async ({ authStorageStatePath }, use) => {
    await use(authStorageStatePath);
  },
  authStorageStatePath: [
    async ({ browser }, use, workerInfo) => {
      const storageStatePath = path.resolve(
        __dirname,
        '..',
        '..',
        'tmp',
        'playwright-auth',
        `pbs-history-targets-${workerInfo.project.name}.json`,
      );
      fs.mkdirSync(path.dirname(storageStatePath), { recursive: true });
      await createAuthenticatedStorageState(browser, storageStatePath);
      try {
        await use(storageStatePath);
      } finally {
        fs.rmSync(storageStatePath, { force: true });
      }
    },
    { scope: 'worker' },
  ],
});

test.use({ serviceWorkers: 'block' });


for (const width of [1280, 390]) {
  test(`PBS History uses merged and standalone telemetry at ${width}px`, async ({ page }, testInfo) => {
    test.setTimeout(120_000);
    await page.setViewportSize({ width, height: 844 });
    await page.addInitScript(() => {
      class FakeWebSocket {
        static CONNECTING = 0;
        static OPEN = 1;
        static CLOSING = 2;
        static CLOSED = 3;

        readonly url: string;
        readyState = FakeWebSocket.CLOSED;
        onopen: ((event: Event) => void) | null = null;
        onclose:
          | ((event: {
              code?: number;
              reason?: string;
              wasClean?: boolean;
            }) => void)
          | null = null;
        onerror: ((event: Event) => void) | null = null;
        onmessage: ((event: MessageEvent) => void) | null = null;

        constructor(url: string) {
          this.url = url;
          queueMicrotask(() => {
            this.onclose?.({
              code: 1006,
              reason: 'e2e websocket disabled',
              wasClean: false,
            });
          });
        }

        close() {
          this.readyState = FakeWebSocket.CLOSED;
        }

        send() {}

        addEventListener() {}

        removeEventListener() {}
      }

      // @ts-expect-error Playwright init script runs in the browser context.
      window.WebSocket = FakeWebSocket;
    });

    const common = { status: 'online', lastSeen: Date.now(), sourceType: 'hybrid',
      cpu: { current: 10 }, memory: { current: 20, total: 1000, used: 200, free: 800 } };
    const servers = ['vm', 'agent'].map(type => ({ ...common,
      id: `pbs-${type}`, type: 'pbs', name: `backup-${type}`, displayName: `backup-${type}`,
      platformId: `backup-${type}`, platformType: 'proxmox-pbs', sources: ['pbs'],
      metricsTarget: { resourceType: 'agent', resourceId: `wrong-${type}` },
      pbs: { instanceId: `backup-${type}`, hostname: `backup-${type}`, datastores: [] },
    }));
    const hosts = ['vm', 'agent'].map(type => ({ ...common, id: `host-${type}`, type,
      name: `backup-${type}`, displayName: `backup-${type}`, platformId: `host-${type}`,
      platformType: type === 'vm' ? 'proxmox-pve' : 'proxmox-pbs',
      sources: type === 'vm' ? ['proxmox', 'agent'] : ['pbs', 'agent'],
      agent: { agentId: `host-${type}`, hostname: `backup-${type}`, osName: 'Debian' },
      metricsTarget: { resourceType: type, resourceId: `history-${type}` },
    }));
    await page.route('**/api/resources?**', async route => {
      const query = new URL(route.request().url()).searchParams;
      const types = (query.get('type') || '').split(',');
      const source = (query.get('source') || '').split(',');
      const data = [...servers, ...hosts].filter(r => types.includes(r.type) && r.sources.some(s => source.includes(s)));
      await route.fulfill({ json: { data, total: data.length } });
    });
    const targets: string[] = [];
    await page.route('**/api/metrics-store/history?**', async route => {
      const q = new URL(route.request().url()).searchParams;
      targets.push(`${q.get('resourceType')}/${q.get('resourceId')}`);
      await route.fulfill({ json: { metrics: { cpu: [0, 1, 2].map(i => ({ timestamp: Date.now() - (2-i)*60000, value: 10+i, min: 10+i, max: 10+i, count: 1 })), memory: [0, 1, 2].map(i => ({ timestamp: Date.now() - (2-i)*60000, value: 20+i, min: 20+i, max: 20+i, count: 1 })) }, resourceId: q.get('resourceId'), resourceType: q.get('resourceType') } });
    });
    await page.goto('/proxmox/backups', { waitUntil: 'domcontentloaded' });
    for (const type of ['vm', 'agent']) {
      const expand = page.getByRole('button', { name: `Expand details for backup-${type}`, exact: true });
      await expand.focus();
      await page.keyboard.press('Enter');
      await page.getByRole('tab', { name: 'History', exact: true }).click();
      await expect.poll(() => targets).toContain(`${type}/history-${type}`);
      expect(targets.some(t => t.includes('wrong-'))).toBe(false);
      const utilization = page.getByTestId('guest-history-group-chart').first();
      await expect(utilization.getByText('Collecting history')).toHaveCount(0);
      await expect(utilization.locator('svg path')).toHaveCount(2);
      await page.screenshot({ path: testInfo.outputPath(`pbs-${type}-${width}.png`), fullPage: true });
      await page.getByRole('button', { name: `Collapse details for backup-${type}`, exact: true }).focus();
      await page.keyboard.press('Enter');
    }
  });
}
