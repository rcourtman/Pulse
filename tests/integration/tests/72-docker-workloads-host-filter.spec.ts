import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { test as base, expect } from '@playwright/test';
import { createAuthenticatedStorageState } from './helpers';

const __dirname = path.dirname(fileURLToPath(import.meta.url));

type WorkerFixtures = {
  authStorageStatePath: string;
};

const SCREENSHOT_PATH = '/tmp/docker-workloads-host-filter.png';

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
        `docker-workloads-host-filter-${workerInfo.project.name}.json`,
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

test.describe('Docker workloads host filter', () => {
  test.setTimeout(180_000);

  test('keeps Docker related-workload links scoped to the selected runtime host', async ({
    page,
  }) => {
    await page.route('**/api/resources**', async (route) => {
      const requestUrl = new URL(route.request().url());
      if (
        requestUrl.pathname !== '/api/resources' ||
        requestUrl.searchParams.get('type') !== 'vm,system-container,app-container,pod'
      ) {
        await route.continue();
        return;
      }

      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          data: [
            {
              id: 'app-container:docker-host-1:grafana',
              type: 'app-container',
              name: 'grafana',
              status: 'running',
              lastSeen: '2026-04-28T12:00:00Z',
              sources: ['docker'],
              parentName: 'tower.local',
              metrics: {
                cpu: { value: 8, percent: 8 },
                memory: { total: 4 * 1024 * 1024 * 1024, used: 1 * 1024 * 1024 * 1024 },
                disk: { total: 40 * 1024 * 1024 * 1024, used: 10 * 1024 * 1024 * 1024 },
                netIn: { value: 1024 },
                netOut: { value: 512 },
                diskRead: { value: 128 },
                diskWrite: { value: 64 },
              },
              docker: {
                containerId: 'docker-grafana',
                hostname: 'tower.local',
                imageName: 'grafana:11',
                runtime: 'docker',
                hostSourceId: 'docker-host-1',
              },
            },
            {
              id: 'app-container:docker-host-1:prometheus',
              type: 'app-container',
              name: 'prometheus',
              status: 'running',
              lastSeen: '2026-04-28T12:00:00Z',
              sources: ['docker'],
              parentName: 'tower.local',
              metrics: {
                cpu: { value: 10, percent: 10 },
                memory: { total: 8 * 1024 * 1024 * 1024, used: 2 * 1024 * 1024 * 1024 },
                disk: { total: 80 * 1024 * 1024 * 1024, used: 20 * 1024 * 1024 * 1024 },
                netIn: { value: 2048 },
                netOut: { value: 1024 },
                diskRead: { value: 256 },
                diskWrite: { value: 128 },
              },
              docker: {
                containerId: 'docker-prometheus',
                hostname: 'tower.local',
                imageName: 'prom/prometheus:latest',
                runtime: 'podman',
                hostSourceId: 'docker-host-1',
              },
            },
            {
              id: 'app-container:docker-host-2:redis',
              type: 'app-container',
              name: 'redis',
              status: 'running',
              lastSeen: '2026-04-28T12:00:00Z',
              sources: ['docker'],
              parentName: 'edge.local',
              metrics: {
                cpu: { value: 4, percent: 4 },
                memory: { total: 2 * 1024 * 1024 * 1024, used: 512 * 1024 * 1024 },
                disk: { total: 20 * 1024 * 1024 * 1024, used: 5 * 1024 * 1024 * 1024 },
                netIn: { value: 512 },
                netOut: { value: 256 },
                diskRead: { value: 64 },
                diskWrite: { value: 32 },
              },
              docker: {
                containerId: 'docker-redis',
                hostname: 'edge.local',
                imageName: 'redis:7',
                runtime: 'docker',
                hostSourceId: 'docker-host-2',
              },
            },
          ],
          meta: {
            page: 1,
            limit: 200,
            total: 3,
            totalPages: 1,
          },
        }),
      });
    });

    await page.goto('/workloads?type=app-container&platform=docker&agent=docker-host-1', {
      waitUntil: 'domcontentloaded',
    });

    await page.waitForURL(/\/workloads\?type=app-container&platform=docker&agent=docker-host-1/);
    await expect(page.locator('#workloads-type')).toHaveValue('app-container');
    await expect(page.locator('#workloads-platform-filter')).toHaveValue('docker');
    await expect(page.locator('#workloads-node-filter')).toHaveValue('docker-host-1');

    const workloadTable = page.locator('table').first();
    await expect(workloadTable).toContainText('grafana');
    await expect(workloadTable).toContainText('prometheus');
    await expect(workloadTable).not.toContainText('redis');
    await expect(page.getByText('No guests found')).toHaveCount(0);

    await page.screenshot({ path: SCREENSHOT_PATH, fullPage: true });
  });
});
