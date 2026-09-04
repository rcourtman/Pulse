import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { expect, test as base, type Page } from '@playwright/test';

import { createAuthenticatedStorageState } from './helpers';

const __dirname = path.dirname(fileURLToPath(import.meta.url));

type WorkerFixtures = {
  authStorageStatePath: string;
};

const connectionsFixture = {
  connections: [
    {
      id: 'pve:Production Cluster',
      type: 'pve',
      name: 'Production Cluster',
      address: 'https://pve1.example.test:8006',
      hostAliases: ['pve1.example.test'],
      state: 'active',
      stateReason: '',
      enabled: true,
      surfaces: ['nodes', 'vms', 'containers', 'storage', 'backups'],
      scope: { nodes: true, vms: true, containers: true, storage: true, backups: true },
      lastSeen: '2026-07-24T14:00:00Z',
      lastError: null,
      source: 'manual',
      capabilities: { supportsPause: true, supportsScope: true, supportsTest: true },
    },
  ],
  systems: [
    {
      id: 'pve:Production Cluster',
      type: 'pve',
      clusterName: 'production',
      components: [
        { connectionId: 'pve:Production Cluster', type: 'pve', role: 'primary' },
      ],
      members: [
        {
          id: 'production-pve1',
          nodeIdentity: 'production-pve1',
          name: 'Render East',
          nativeName: 'pve1',
          endpoint: 'https://pve1.example.test:8006',
          hostAliases: ['pve1', 'pve-old'],
          state: 'active',
          primary: true,
        },
        {
          id: 'production-pve2',
          nodeIdentity: 'production-pve2',
          name: 'pve2',
          nativeName: 'pve2',
          endpoint: 'https://pve2.example.test:8006',
          hostAliases: ['pve2'],
          state: 'active',
        },
      ],
    },
  ],
};

const nodesFixture = [
  {
    id: 'pve-0',
    type: 'pve',
    name: 'Production Cluster',
    host: 'https://pve1.example.test:8006',
    user: 'root@pam',
    hasPassword: false,
    tokenName: 'pulse',
    hasToken: true,
    verifySSL: false,
    monitorVMs: true,
    monitorContainers: true,
    monitorStorage: true,
    monitorBackups: true,
    excludeDatastores: [],
    enabled: true,
    status: 'connected',
    isCluster: true,
    clusterName: 'production',
    clusterEndpoints: [
      {
        nodeId: 'node/pve1',
        nodeIdentity: 'production-pve1',
        nativeNodeId: 1,
        nodeName: 'pve1',
        nativeName: 'pve1',
        nativeAliases: ['pve-old'],
        displayName: 'Render East',
        host: 'https://pve1.example.test:8006',
        ip: '10.0.0.1',
        online: true,
        lastSeen: '2026-07-24T14:00:00Z',
        pulseReachable: true,
      },
      {
        nodeId: 'node/pve2',
        nodeIdentity: 'production-pve2',
        nativeNodeId: 2,
        nodeName: 'pve2',
        nativeName: 'pve2',
        host: 'https://pve2.example.test:8006',
        ip: '10.0.0.2',
        online: true,
        lastSeen: '2026-07-24T14:00:00Z',
        pulseReachable: true,
      },
    ],
  },
];

async function routeClusterNodeDisplayNames(
  page: Page,
  onUpdate: (payload: Record<string, unknown>) => void,
  getConnectionsFixture: () => typeof connectionsFixture = () => connectionsFixture,
) {
  await page.routeWebSocket('**/ws*', () => {});
  await page.route('**/api/connections*', async (route) => {
    if (new URL(route.request().url()).pathname !== '/api/connections') {
      await route.continue();
      return;
    }
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify(getConnectionsFixture()),
    });
  });
  await page.route('**/api/config/nodes**', async (route) => {
    const request = route.request();
    const url = new URL(request.url());
    if (request.method() === 'PUT' && url.pathname === '/api/config/nodes/pve-0') {
      onUpdate(request.postDataJSON() as Record<string, unknown>);
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ status: 'success' }),
      });
      return;
    }
    if (request.method() === 'GET' && url.pathname === '/api/config/nodes') {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(nodesFixture),
      });
      return;
    }
    await route.continue();
  });
}

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
        `proxmox-node-display-names-${workerInfo.project.name}.json`,
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

test.describe('Proxmox cluster node display names', () => {
  test.setTimeout(180_000);

  test('keeps unsaved infrastructure edits across a connection-ledger poll', async ({ page }) => {
    let publishRefreshedConnection = false;
    let refreshedConnectionResponses = 0;
    const refreshedConnectionsFixture: typeof connectionsFixture = {
      ...connectionsFixture,
      connections: connectionsFixture.connections.map((connection) => ({
        ...connection,
        address: 'https://pve-refresh.example.test:8006',
        lastSeen: '2026-09-04T00:15:00Z',
      })),
    };

    await page.clock.install();
    await routeClusterNodeDisplayNames(
      page,
      () => {},
      () => {
        if (!publishRefreshedConnection) return connectionsFixture;
        refreshedConnectionResponses += 1;
        return refreshedConnectionsFixture;
      },
    );

    await page.goto('/settings/infrastructure', { waitUntil: 'domcontentloaded' });
    await page.getByRole('button', { name: 'Manage', exact: true }).click();

    const dialog = page.getByRole('dialog');
    const nodeName = dialog.getByLabel('Node Name');
    const verifyCertificate = dialog.getByRole('checkbox', {
      name: 'Verify SSL certificate',
    });
    await nodeName.fill('Unsaved cluster label');
    await dialog.getByRole('button', { name: 'Host Telemetry Agent', exact: true }).click();
    await verifyCertificate.check();
    await expect(dialog.getByText('Host telemetry agent', { exact: true })).toBeVisible();

    publishRefreshedConnection = true;
    await page.clock.fastForward(15_000);
    await expect.poll(() => refreshedConnectionResponses).toBeGreaterThan(0);

    await expect(dialog).toHaveAccessibleDescription(
      'Proxmox VE cluster · Production Cluster (https://pve-refresh.example.test:8006)',
    );
    await expect(nodeName).toHaveValue('Unsaved cluster label');
    await expect(dialog.getByText('Host telemetry agent', { exact: true })).toBeVisible();
    await expect(verifyCertificate).toBeChecked();
  });

  test('keeps native diagnostics while saving by immutable identity', async ({ page }, testInfo) => {
    let updatePayload: Record<string, unknown> | undefined;
    await routeClusterNodeDisplayNames(page, (payload) => {
      updatePayload = payload;
    });

    await page.goto('/settings/infrastructure', { waitUntil: 'domcontentloaded' });
    const memberToggle = page.getByRole('button', { name: 'Show 2 nodes for production' });
    await expect(memberToggle).toBeVisible();
    await memberToggle.click();

    await expect(page.getByText('Render East', { exact: true })).toBeVisible();
    await expect(page.getByText('Production Cluster (pve1)', { exact: true })).toHaveCount(0);

    const manageButton = page.getByRole('button', { name: 'Manage', exact: true });
    await manageButton.click();
    await expect(
      page.getByText(
        'Give each node an optional display name for Pulse. This never changes its Proxmox name, identity, credentials, or connection address.',
        { exact: true },
      ),
    ).toBeVisible();
    await expect(page.getByText('Proxmox node: pve1', { exact: true })).toBeVisible();
    await expect(page.getByLabel('Display name for pve1')).toHaveValue('Render East');

    await page.screenshot({ path: testInfo.outputPath('manage-dialog.png') });
    const scrollTopBeforeClose = await page.evaluate(() => {
      const shell = document.querySelector<HTMLElement>('.app-scroll-shell');
      return shell?.scrollTop ?? window.scrollY;
    });
    await manageButton.evaluate((element) => {
      const state = window as Window & { __infrastructureManageFocusCalls?: FocusOptions[] };
      state.__infrastructureManageFocusCalls = [];
      const target = element as HTMLButtonElement;
      const originalFocus = target.focus.bind(target);
      target.focus = (options?: FocusOptions) => {
        state.__infrastructureManageFocusCalls?.push(options ?? {});
        originalFocus(options);
      };
    });
    await page.getByRole('button', { name: 'Close edit infrastructure dialog' }).click();
    await expect(page.getByRole('dialog')).toHaveCount(0);
    await expect
      .poll(
        () =>
          page.evaluate(
            () =>
              (window as Window & { __infrastructureManageFocusCalls?: FocusOptions[] })
                .__infrastructureManageFocusCalls?.length ?? 0,
          ),
        { timeout: 2_000 },
      )
      .toBe(1);
    // The row-stability fallback runs later than the shared dialog cleanup.
    // Moving on during that window must not pull focus back to Manage.
    const nextAction = page.getByRole('button', { name: 'Add infrastructure', exact: true });
    await nextAction.evaluate((element) => element.focus({ preventScroll: true }));
    await page.waitForTimeout(350);
    expect(
      await page.evaluate(
        () =>
          (window as Window & { __infrastructureManageFocusCalls?: FocusOptions[] })
            .__infrastructureManageFocusCalls,
      ),
    ).toEqual([{ preventScroll: true }]);
    expect(
      await page.evaluate(() => {
        const shell = document.querySelector<HTMLElement>('.app-scroll-shell');
        return shell?.scrollTop ?? window.scrollY;
      }),
    ).toBe(scrollTopBeforeClose);
    await expect(nextAction).toBeFocused();
    await page.screenshot({ path: testInfo.outputPath('manage-dialog-closed.png') });

    await manageButton.click();
    await expect(page.getByRole('dialog')).toBeVisible();

    // Equal display labels are intentionally valid presentation. The write
    // target remains the second member's immutable identity.
    await page.getByLabel('Display name for pve2').fill('Render East');
    const displayNameUpdate = page.waitForRequest((request) => {
      if (
        request.method() !== 'PUT' ||
        new URL(request.url()).pathname !== '/api/config/nodes/pve-0'
      ) {
        return false;
      }
      const payload = request.postDataJSON() as Record<string, unknown> | null;
      return Array.isArray(payload?.clusterNodeDisplayNameOverrides);
    });
    await page.getByRole('button', { name: 'Save changes', exact: true }).click();
    await displayNameUpdate;

    await expect.poll(() => updatePayload).toBeDefined();
    expect(updatePayload?.clusterNodeDisplayNameOverrides).toEqual([
      { nodeIdentity: 'production-pve2', displayName: 'Render East' },
    ]);
    expect(updatePayload?.host).toBe('https://pve1.example.test:8006');
    expect(updatePayload).not.toHaveProperty('clusterEndpoints');
    expect(updatePayload).not.toHaveProperty('tokenValue');
  });
});
