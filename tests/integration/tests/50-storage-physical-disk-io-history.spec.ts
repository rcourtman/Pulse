import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { expect, test as base, type Page } from '@playwright/test';

import { apiRequest, createAuthenticatedStorageState, getMockMode, setMockMode } from './helpers';

const __dirname = path.dirname(fileURLToPath(import.meta.url));

type WorkerFixtures = {
  authStorageStatePath: string;
};

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
      `storage-physical-disk-io-history-${workerInfo.project.name}.json`,
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

async function ensureMockModeEnabled(page: Page): Promise<void> {
  const state = await getMockMode(page);
  if (mockModeWasEnabled === null) {
    mockModeWasEnabled = state.enabled;
  }
  if (!state.enabled) {
    await setMockMode(page, true);
  }
}

test.describe.serial('Storage physical disk drawer history', () => {
  test.setTimeout(180_000);

  test.afterAll(async ({ browser }) => {
    if (mockModeWasEnabled === null) return;

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

  test('renders canonical live I/O history for Proxmox physical disks', async ({
    page,
  }, testInfo) => {
    test.skip(testInfo.project.name.startsWith('mobile-'), 'Desktop physical-disk table contract');

    await ensureMockModeEnabled(page);

    // Fixture disk placement is deterministic by node identity, rather than
    // a fixed model/owner pair. Pin an actual serial-bearing Proxmox disk and
    // prove that the drawer requests history for that same canonical serial.
    const inventory = await apiRequest(page, '/api/resources?type=physical_disk&source=proxmox');
    expect(inventory.ok()).toBeTruthy();
    const { data: disks } = await inventory.json() as { data: Array<{
      id: string;
      physicalDisk?: { serial?: string; model?: string; devPath?: string };
    }> };
    const target = disks.find((resource) => resource.physicalDisk?.serial &&
      resource.physicalDisk.model && resource.physicalDisk.devPath?.startsWith('/dev/nvme'));
    expect(target, 'The mock estate must include a serial-bearing NVMe disk').toBeDefined();
    const targetSerial = target!.physicalDisk!.serial!;

    const diskHistoryResponses = new Map<string, { resourceId: string; points: number }>();
    page.on('response', async (response) => {
      const url = response.url();
      if (!url.includes('/api/metrics-store/history')) {
        return;
      }
      const parsed = new URL(url);
      if (parsed.searchParams.get('resourceType') !== 'disk') {
        return;
      }
      if (parsed.searchParams.get('resourceId') !== targetSerial) return;

      try {
        const payload = (await response.json()) as {
          resourceId?: string; metrics?: Record<string, unknown[]>;
        };
        for (const metric of ['diskread', 'diskwrite', 'disk']) {
          diskHistoryResponses.set(metric, {
            resourceId: payload.resourceId || '',
            points: payload.metrics?.[metric]?.length || 0,
          });
        }
      } catch {
        // The drawer polls on an interval; a response can land while the page
        // is tearing down, and json() on it must not fail the test.
      }
    });

    // Physical disks moved onto the Proxmox platform page's storage section.
    await page.goto('/proxmox/storage', { waitUntil: 'domcontentloaded' });
    await expect(page.getByTestId('storage-page')).toBeVisible({ timeout: 60_000 });
    const disksTab = page.getByRole('tab', { name: 'Physical Disks' });
    await expect(disksTab).toBeVisible({ timeout: 30_000 });
    await disksTab.click({ timeout: 30_000 });
    const search = page.getByRole('textbox', { name: /Search Proxmox storage/ });
    await expect(search).toBeVisible({ timeout: 30_000 });
    await search.fill(target!.physicalDisk!.model!);

    const row = page.locator(`table tbody tr[data-row-id=${JSON.stringify(target!.id)}]`);

    await expect(row).toBeVisible({ timeout: 30_000 });
    await row.getByRole('button', { name: /^Expand / }).click();

    const detail = page.locator(`[data-inline-detail-for=${JSON.stringify(targetSerial)}]`);
    await expect(detail).toBeVisible();
    await expect(detail.getByRole('img', { name: 'Disk I/O history', exact: true })).toBeVisible();
    await expect(detail.getByRole('combobox', { name: 'History range', exact: true })).toHaveValue('24h');

    await expect
      .poll(() => Array.from(diskHistoryResponses.keys()).sort().join(','))
      .toBe('disk,diskread,diskwrite');

    for (const metric of ['diskread', 'diskwrite', 'disk'] as const) {
      await expect
        .poll(() => diskHistoryResponses.get(metric)?.resourceId || '')
        .toBe(targetSerial);
      await expect
        .poll(() => diskHistoryResponses.get(metric)?.points || 0)
        .toBeGreaterThan(0);
    }
  });

  test('keeps visible physical-disk drawers hydrated with live I/O history', async ({
    page,
  }, testInfo) => {
    test.skip(testInfo.project.name.startsWith('mobile-'), 'Desktop physical-disk table contract');

    await ensureMockModeEnabled(page);

    await page.goto('/proxmox/storage', { waitUntil: 'domcontentloaded' });
    await expect(page.getByTestId('storage-page')).toBeVisible({ timeout: 60_000 });
    const disksTab = page.getByRole('tab', { name: 'Physical Disks' });
    await expect(disksTab).toBeVisible({ timeout: 30_000 });
    await disksTab.click();

    const rows = page.locator('table tbody tr[data-row-id]');
    await expect(rows.first()).toBeVisible({ timeout: 30_000 });
    // Live metric updates re-sort the table, so pin rows by identity instead
    // of index; an nth() locator chases a moving target between refreshes.
    const rowIds = (
      await rows.evaluateAll((elements) =>
        elements.map((element) => element.getAttribute('data-row-id')),
      )
    ).filter((value): value is string => Boolean(value));
    expect(rowIds.length).toBeGreaterThan(0);

    const failures: string[] = [];

    for (const rowId of rowIds.slice(0, 12)) {
      const row = page.locator(`tr[data-row-id="${rowId}"]`).first();
      if ((await row.count()) === 0) {
        // Mock state can rotate a disk out between refreshes; skip it.
        continue;
      }
      await row.scrollIntoViewIfNeeded();

      const rowText = (await row.innerText()).replace(/\s+/g, ' ').trim();
      const summarySeriesId = (await row.getAttribute('data-summary-series-id')) || '';
      const detailResponsePoints = new Map<string, number>();

      const responseHandler = async (response: { url(): string; json(): Promise<unknown> }) => {
        const url = response.url();
        if (!url.includes('/api/metrics-store/history')) return;

        const parsed = new URL(url);
        if (parsed.searchParams.get('resourceType') !== 'disk') return;
        if (parsed.searchParams.get('resourceId') !== summarySeriesId) return;

        try {
          const payload = (await response.json()) as { metrics?: Record<string, unknown[]> };
          for (const metric of ['diskread', 'diskwrite', 'disk']) {
            detailResponsePoints.set(metric, payload.metrics?.[metric]?.length || 0);
          }
        } catch {
          // Ignore responses that resolve while the page is tearing down.
        }
      };

      page.on('response', responseHandler);
      try {
        await row.getByRole('button', { name: /^Expand / }).click();

        const detail = page.locator(`[data-inline-detail-for="${summarySeriesId}"]`);
        await expect(detail).toBeVisible();
        await expect(detail.getByRole('img', { name: 'Disk I/O history', exact: true })).toBeVisible();
        for (const metric of ['diskread', 'diskwrite', 'disk'] as const) {
          let points = 0;
          try {
            await expect.poll(() => detailResponsePoints.get(metric) || 0, {
              timeout: 10_000,
            }).toBeGreaterThan(0);
            points = detailResponsePoints.get(metric) || 0;
          } catch {
            // Report every drawer without aborting at the first slow series.
          }
          if (points <= 0) {
            failures.push(`${rowText} [${metric}:0 points via ${summarySeriesId || 'no-series-id'}]`);
            break;
          }
        }

        await row.getByRole('button', { name: /^Collapse / }).click();
      } finally {
        page.off('response', responseHandler);
      }
    }

    expect(failures, failures.join('\n')).toEqual([]);
  });
});
