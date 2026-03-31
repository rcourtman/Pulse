import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { expect, test as base } from '@playwright/test';

import { apiRequest, createAuthenticatedStorageState, getMockMode, setMockMode } from './helpers';

const __dirname = path.dirname(fileURLToPath(import.meta.url));

type WorkerFixtures = {
  authStorageStatePath: string;
};

type ResourceSummary = {
  name?: string;
  type?: string;
  sources?: string[];
};

const TRUENAS_SCREENSHOT_PATH = path.resolve(
  __dirname,
  '..',
  '..',
  'tmp',
  'platform-mock-runtime-truenas.png',
);
const VMWARE_SCREENSHOT_PATH = path.resolve(
  __dirname,
  '..',
  '..',
  'tmp',
  'platform-mock-runtime-vmware.png',
);

let mockModeWasEnabled: boolean | null = null;

const test = base.extend<{}, WorkerFixtures>({
  storageState: async ({ authStorageStatePath }, use) => {
    await use(authStorageStatePath);
  },
  authStorageStatePath: [async ({ browser }, use, workerInfo) => {
    const storageStatePath = path.resolve(
      __dirname,
      '..',
      '..',
      'tmp',
      'playwright-auth',
      `platform-mock-runtime-${workerInfo.project.name}.json`,
    );
    fs.mkdirSync(path.dirname(storageStatePath), { recursive: true });
    await createAuthenticatedStorageState(browser, storageStatePath);
    try {
      await use(storageStatePath);
    } finally {
      fs.rmSync(storageStatePath, { force: true });
    }
  }, { scope: 'worker' }],
});

async function ensureMockModeEnabled(page: import('@playwright/test').Page): Promise<void> {
  const state = await getMockMode(page);
  if (mockModeWasEnabled === null) {
    mockModeWasEnabled = state.enabled;
  }
  if (!state.enabled) {
    await setMockMode(page, true);
  }
}

async function fetchMockResourceName(
  page: import('@playwright/test').Page,
  source: string,
  queryTypes: string[] = [],
): Promise<string> {
  const searchParams = new URLSearchParams({
    source,
    limit: '200',
  });
  if (queryTypes.length > 0) {
    searchParams.set('type', queryTypes.join(','));
  }
  const response = await apiRequest(
    page,
    `/api/resources?${searchParams.toString()}`,
  );
  expect(response.ok()).toBeTruthy();

  const payload = (await response.json()) as { data?: ResourceSummary[] };
  const resources = Array.isArray(payload.data) ? payload.data : [];
  const chosen = resources.find((resource) => resource.name?.trim());
  if (!chosen?.name?.trim()) {
    throw new Error(`Expected at least one mock resource for source ${source}`);
  }
  return chosen.name.trim();
}

async function fetchMockResourceNames(page: import('@playwright/test').Page): Promise<Record<string, string>> {
  return {
    'proxmox-pve': await fetchMockResourceName(page, 'proxmox', ['agent']),
    docker: await fetchMockResourceName(page, 'docker', ['docker-host']),
    kubernetes: await fetchMockResourceName(page, 'kubernetes', ['k8s-cluster']),
    'proxmox-pbs': await fetchMockResourceName(page, 'pbs', ['pbs']),
    'proxmox-pmg': await fetchMockResourceName(page, 'pmg', ['pmg']),
  };
}

async function expectInfrastructureSource(
  page: import('@playwright/test').Page,
  source: string,
  resourceName: string,
): Promise<void> {
  await page.goto(`/infrastructure?source=${encodeURIComponent(source)}`, {
    waitUntil: 'domcontentloaded',
  });
  await expect(page.getByTestId('infrastructure-page')).toBeVisible();
  await expect(page.locator('#infra-source-filter')).toHaveValue(source);
  await expect(page.getByText(resourceName).first()).toBeVisible();
}

async function expectFirstTableContains(
  page: import('@playwright/test').Page,
  texts: string[],
): Promise<void> {
  const table = page.locator('table').first();
  for (const text of texts) {
    await expect(table).toContainText(text);
  }
}

test.describe.serial('Platform mock runtime', () => {
  test.setTimeout(180_000);

  test.afterAll(async ({ browser }) => {
    if (mockModeWasEnabled === null) {
      return;
    }

    const context = await browser.newContext();
    const page = await context.newPage();
    try {
      const current = await getMockMode(page);
      if (current.enabled !== mockModeWasEnabled) {
        await setMockMode(page, mockModeWasEnabled);
      }
    } finally {
      await context.close();
    }
  });

  test('renders canonical legacy and provider-backed mock data from the live runtime', async ({ page }, testInfo) => {
    test.skip(testInfo.project.name.startsWith('mobile-'), 'Desktop runtime proof');

    await ensureMockModeEnabled(page);
    const resourceNames = await fetchMockResourceNames(page);

    await page.goto('/settings/infrastructure/platforms/truenas', {
      waitUntil: 'domcontentloaded',
    });
    await page.waitForURL(/\/settings\/infrastructure\/platforms\/truenas/, {
      timeout: 15_000,
    });
    await expect(page.getByText('Archive NAS')).toBeVisible();
    await expect(page.getByText('4 pools')).toBeVisible();
    await expect(page.getByText('9 datasets')).toBeVisible();
    await expect(page.getByText('5 apps')).toBeVisible();
    fs.mkdirSync(path.dirname(TRUENAS_SCREENSHOT_PATH), { recursive: true });
    await page.screenshot({ path: TRUENAS_SCREENSHOT_PATH, fullPage: true });
    await expectInfrastructureSource(page, 'truenas', 'truenas-main');

    await page.goto('/settings/infrastructure/platforms/vmware', {
      waitUntil: 'domcontentloaded',
    });
    await page.waitForURL(/\/settings\/infrastructure\/platforms\/vmware/, {
      timeout: 15_000,
    });
    await expect(page.getByText('Lab vCenter')).toBeVisible();
    await expect(page.getByText('4 hosts')).toBeVisible();
    await expect(page.getByText('6 vms')).toBeVisible();
    await expect(page.getByText('4 datastores')).toBeVisible();
    fs.mkdirSync(path.dirname(VMWARE_SCREENSHOT_PATH), { recursive: true });
    await page.screenshot({ path: VMWARE_SCREENSHOT_PATH, fullPage: true });
    await expectInfrastructureSource(page, 'vmware-vsphere', 'esxi-01.lab.local');
    await expectFirstTableContains(page, ['esxi-02.lab.local', 'esxi-03.lab.local', 'esxi-04.lab.local']);

    await page.goto('/workloads?type=app-container&platform=truenas&agent=truenas-main', {
      waitUntil: 'domcontentloaded',
    });
    await page.waitForURL(/\/workloads\?type=app-container&platform=truenas&agent=.*truenas-main/);
    await expect(page.locator('#dashboard-type-filter')).toHaveValue('app-container');
    await expect(page.locator('#workloads-platform-filter')).toHaveValue('truenas');
    await expectFirstTableContains(page, ['Nextcloud', 'Immich', 'Paperless-ngx', 'Grafana', 'AdGuard Home']);
    await expect(page.getByText('No history yet')).toHaveCount(0);

    await page.goto('/storage?source=truenas&node=truenas-main', {
      waitUntil: 'domcontentloaded',
    });
    await expect(page).toHaveURL(/\/storage\?source=truenas&node=truenas-main/);
    await expectFirstTableContains(page, ['tank', 'fast', 'archive', 'vault']);
    await expect(page.getByText('No history yet')).toHaveCount(0);
    await expect(page.getByText('Derived from storage metrics - live Ceph telemetry unavailable.')).toHaveCount(0);
    await expect(page.locator('table').first()).not.toContainText('UNKNOWN');

    await page.goto('/storage?source=vmware-vsphere', {
      waitUntil: 'domcontentloaded',
    });
    await expect(page).toHaveURL(/\/storage\?source=vmware-vsphere/);
    await expectFirstTableContains(page, ['nvme-primary', 'archive-tier', 'analytics-vsan', 'backup-nfs']);
    await expect(page.getByText('No history yet')).toHaveCount(0);

    await page.goto('/recovery?platform=truenas&node=truenas-main', {
      waitUntil: 'domcontentloaded',
    });
    await expect(page).toHaveURL(/\/recovery\?platform=truenas&node=truenas-main/);
    await expect(page.getByTestId('recovery-page')).toBeVisible();
    await expectFirstTableContains(page, ['tank/apps', 'tank/media', 'vault/compliance']);
    await expect(page.getByText('0 fresh in 24h')).toHaveCount(0);

    for (const [source, resourceName] of Object.entries(resourceNames)) {
      await expectInfrastructureSource(page, source, resourceName);
    }
  });
});
