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

async function expectFirstTableExcludes(
  page: import('@playwright/test').Page,
  texts: string[],
): Promise<void> {
  const table = page.locator('table').first();
  for (const text of texts) {
    await expect(table).not.toContainText(text);
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

  test('renders canonical legacy and provider-backed mock data on the live platform pages', async ({ page }, testInfo) => {
    test.skip(testInfo.project.name.startsWith('mobile-'), 'Desktop runtime proof');

    await ensureMockModeEnabled(page);
    const resourceNames = await fetchMockResourceNames(page);
    const truenasSystemName = await fetchMockResourceName(page, 'truenas', ['agent']);
    const vmwareHostName = await fetchMockResourceName(page, 'vmware', ['agent']);

    // Provider-backed TrueNAS fixture data reaches the live platform page.
    await page.goto('/truenas', { waitUntil: 'domcontentloaded' });
    await expect(page.getByTestId('truenas-page')).toBeVisible({ timeout: 60_000 });
    await expect(page.getByText(truenasSystemName).first()).toBeVisible({ timeout: 30_000 });
    await page.goto('/truenas/storage', { waitUntil: 'domcontentloaded' });
    await expect(page.getByText('tank', { exact: true }).first()).toBeVisible({ timeout: 30_000 });
    fs.mkdirSync(path.dirname(TRUENAS_SCREENSHOT_PATH), { recursive: true });
    await page.screenshot({ path: TRUENAS_SCREENSHOT_PATH });

    // Provider-backed vSphere fixture data reaches the live platform pages.
    await page.goto('/vmware', { waitUntil: 'domcontentloaded' });
    await expect(page.getByTestId('vmware-page')).toBeVisible({ timeout: 60_000 });
    await expect(page.getByText(vmwareHostName).first()).toBeVisible({ timeout: 30_000 });
    await page.goto('/vmware/storage', { waitUntil: 'domcontentloaded' });
    await expect(page.getByText('nvme-primary').first()).toBeVisible({ timeout: 30_000 });
    await page.screenshot({ path: VMWARE_SCREENSHOT_PATH });

    // Legacy-source mock data (Proxmox node, Docker host, Kubernetes cluster,
    // PBS and PMG instances) reaches its platform surfaces.
    await page.goto('/proxmox', { waitUntil: 'domcontentloaded' });
    await expect(page.getByTestId('proxmox-page')).toBeVisible({ timeout: 60_000 });
    await expect(page.getByText(resourceNames['proxmox-pve']).first()).toBeVisible({
      timeout: 30_000,
    });

    await page.goto('/docker', { waitUntil: 'domcontentloaded' });
    await expect(page.getByTestId('docker-page')).toBeVisible({ timeout: 60_000 });
    await expect(page.getByText(resourceNames.docker).first()).toBeVisible({ timeout: 30_000 });

    await page.goto('/proxmox/backups', { waitUntil: 'domcontentloaded' });
    await expect(page.getByText(resourceNames['proxmox-pbs']).first()).toBeVisible({
      timeout: 30_000,
    });

    await page.goto('/proxmox/mail', { waitUntil: 'domcontentloaded' });
    // The mail page prettifies instance names, so match case- and
    // separator-insensitively.
    const pmgPattern = new RegExp(
      resourceNames['proxmox-pmg'].replace(/[-_\s]+/g, '[-_ ]'),
      'i',
    );
    await expect(page.getByText(pmgPattern).first()).toBeVisible({ timeout: 30_000 });
  });
});
