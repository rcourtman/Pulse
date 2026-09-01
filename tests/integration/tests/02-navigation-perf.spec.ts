import { test, expect, type Page } from '@playwright/test';
import { ensureAuthenticated, getMockMode, setMockMode, waitForPulseReady } from './helpers';

const truthy = (value: string | undefined) => {
  if (!value) return false;
  return ['1', 'true', 'yes', 'on'].includes(value.trim().toLowerCase());
};

const toPositiveInt = (value: string | undefined, fallback: number) => {
  const parsed = Number(value);
  if (!Number.isFinite(parsed) || parsed <= 0) return fallback;
  return Math.floor(parsed);
};

const median = (values: number[]): number => {
  if (values.length === 0) return 0;
  const sorted = [...values].sort((a, b) => a - b);
  const mid = Math.floor(sorted.length / 2);
  if (sorted.length % 2 === 0) {
    return Math.round((sorted[mid - 1] + sorted[mid]) / 2);
  }
  return sorted[mid];
};

const isTransientBackendError = (error: unknown): boolean => {
  const message = String(error);
  return (
    message.includes('ERR_CONNECTION_REFUSED') ||
    message.includes('ECONNREFUSED') ||
    message.includes('socket hang up') ||
    message.includes('ETIMEDOUT')
  );
};

const gotoWithBackendRetry = async (page: Page, url: string, attempts = 3): Promise<void> => {
  let lastError: unknown = null;
  for (let attempt = 1; attempt <= attempts; attempt++) {
    try {
      await waitForPulseReady(page, 30_000);
      await page.goto(url);
      return;
    } catch (error) {
      lastError = error;
      if (!isTransientBackendError(error) || attempt === attempts) {
        throw error;
      }
      console.warn(`[perf] transient backend outage during ${url}, retrying (${attempt}/${attempts})`);
      await page.waitForTimeout(1_000);
    }
  }
  throw lastError ?? new Error(`Failed to navigate to ${url}`);
};

// Platform-first navigation: the perf budget covers switching between two
// primary platform tabs, matching how users move between platform pages.
const waitForProxmoxReady = async (page: Page) => {
  await expect(page).toHaveURL(/\/proxmox(?:\/|\?|$)/);
  await expect(page.getByTestId('proxmox-page')).toBeVisible();
  await page.waitForFunction(
    () => {
      const root = document.querySelector('[data-testid="proxmox-page"]');
      if (!root) return false;
      if (root.querySelector('table')) return true;
      const text = root.textContent || '';
      return /No Proxmox|Add infrastructure/i.test(text);
    },
    undefined,
    { timeout: 30_000 },
  );
};

const waitForDockerReady = async (page: Page) => {
  await expect(page).toHaveURL(/\/docker(?:\/|\?|$)/);
  await expect(page.getByTestId('docker-page')).toBeVisible();
  await page.waitForFunction(
    () => {
      const root = document.querySelector('[data-testid="docker-page"]');
      if (!root) return false;
      if (root.querySelector('table')) return true;
      const text = root.textContent || '';
      return /No Docker|Add infrastructure/i.test(text);
    },
    undefined,
    { timeout: 30_000 },
  );
};

const measureTabTransition = async (
  page: Page,
  tabName: 'Proxmox' | 'Docker',
  waitForReady: (page: Page) => Promise<void>,
): Promise<number> => {
  const start = Date.now();
  await page.getByRole('tab', { name: new RegExp(`^${tabName}$`) }).first().click();
  await waitForReady(page);
  return Date.now() - start;
};

const makeLargeProxmoxEstate = () => {
  const nodes = Array.from({ length: 50 }, (_, index) => {
    const nodeNumber = index + 1;
    return {
      id: `synthetic:proxmox:node-${nodeNumber}`,
      type: 'agent',
      name: `pve-${String(nodeNumber).padStart(2, '0')}`,
      status: 'online',
      lastSeen: '2026-09-01T10:00:00Z',
      sources: ['proxmox'],
      platformScopes: ['proxmox-pve'],
      metrics: {
        cpu: { percent: (nodeNumber % 70) + 10 },
        memory: { used: 24, total: 64, percent: 37.5 },
        disk: { used: 320, total: 1024, percent: 31.25 },
      },
      proxmox: {
        nodeName: `pve-${String(nodeNumber).padStart(2, '0')}`,
        clusterName: 'synthetic-large-estate',
        instance: 'synthetic-lab',
        pveVersion: '9.0.3',
        uptime: 1_000_000 + nodeNumber,
      },
    };
  });

  const guests = Array.from({ length: 900 }, (_, index) => {
    const guestNumber = index + 1;
    const nodeIndex = index % nodes.length;
    const node = nodes[nodeIndex];
    const nodeName = node.proxmox.nodeName;
    const vmid = 100 + guestNumber;
    return {
      id: `synthetic:proxmox:${nodeName}:${vmid}`,
      type: guestNumber % 4 === 0 ? 'system-container' : 'vm',
      name: `workload-${String(guestNumber).padStart(4, '0')}`,
      status: guestNumber % 9 === 0 ? 'stopped' : 'online',
      parentId: node.id,
      parentName: nodeName,
      lastSeen: '2026-09-01T10:00:00Z',
      sources: ['proxmox'],
      platformScopes: ['proxmox-pve'],
      metrics: {
        cpu: { percent: guestNumber % 80 },
        memory: { used: 2, total: 8, percent: 25 },
        disk: { used: 20, total: 100, percent: 20 },
      },
      proxmox: {
        nodeName,
        clusterName: 'synthetic-large-estate',
        instance: 'synthetic-lab',
        vmid,
        cpus: 4,
        uptime: 100_000 + guestNumber,
      },
    };
  });

  const storage = Array.from({ length: 300 }, (_, index) => {
    const storageNumber = index + 1;
    const node = nodes[index % nodes.length];
    const physicalDisk = storageNumber % 3 === 0;
    return {
      id: `synthetic:storage:${storageNumber}`,
      type: physicalDisk ? 'physical_disk' : 'storage',
      name: physicalDisk
        ? `/dev/disk/by-id/synthetic-${storageNumber}`
        : `pool-${String(storageNumber).padStart(3, '0')}`,
      status: 'online',
      parentId: node.id,
      parentName: node.proxmox.nodeName,
      lastSeen: '2026-09-01T10:00:00Z',
      sources: physicalDisk ? ['agent', 'proxmox'] : ['proxmox'],
      platformScopes: ['proxmox-pve'],
      metrics: { disk: { used: 400, total: 1000, percent: 40 } },
      storage: physicalDisk ? undefined : { type: 'zfs', isZfs: true },
      physicalDisk: physicalDisk
        ? { device: `/dev/sd${String.fromCharCode(97 + (storageNumber % 20))}` }
        : undefined,
      proxmox: {
        nodeName: node.proxmox.nodeName,
        clusterName: 'synthetic-large-estate',
        instance: 'synthetic-lab',
      },
    };
  });

  return { overview: [...nodes, ...guests], storage: [...nodes, ...storage] };
};

const measureProxmoxSectionTransition = async (
  page: Page,
  section: 'Overview' | 'Storage',
): Promise<number> => {
  const start = Date.now();
  await page
    .getByRole('navigation', { name: 'Proxmox sections' })
    .getByRole('link', { name: section, exact: true })
    .click();

  if (section === 'Storage') {
    await expect(page).toHaveURL(/\/proxmox\/storage(?:\?|$)/);
    await expect(page.getByTestId('storage-page')).toBeVisible();
    await expect(
      page.getByTestId('storage-page').locator('table').first(),
    ).toBeVisible();
  } else {
    await expect(page).toHaveURL(/\/proxmox\/overview(?:\?|$)/);
    await expect(page.getByTestId('proxmox-guests-section')).toBeVisible();
    await expect(
      page.getByTestId('proxmox-guests-section').locator('table').first(),
    ).toBeVisible();
  }

  return Date.now() - start;
};

test.describe.serial('Navigation performance budgets', () => {
  test.skip(!truthy(process.env.PULSE_E2E_PERF), 'Set PULSE_E2E_PERF=1 to enable navigation perf checks');

  test('platform tab switches stay within budget', async ({ page, browserName, isMobile }) => {
    test.skip(browserName !== 'chromium' || Boolean(isMobile), 'Perf budgets are pinned to desktop Chromium');
    test.slow();

    const iterations = toPositiveInt(process.env.PULSE_E2E_PERF_ITERATIONS, 3);
    const proxmoxToDockerBudgetMs = toPositiveInt(process.env.PULSE_E2E_PERF_PROXMOX_TO_DOCKER_BUDGET_MS, 2200);
    const dockerToProxmoxBudgetMs = toPositiveInt(process.env.PULSE_E2E_PERF_DOCKER_TO_PROXMOX_BUDGET_MS, 2200);

    await ensureAuthenticated(page);

    let initialMockMode: { enabled: boolean } | null = null;
    try {
      initialMockMode = await getMockMode(page);
      if (!initialMockMode.enabled) {
        await setMockMode(page, true);
      }
    } catch (error) {
      // Some auth/bootstrap paths can delay privileged settings APIs.
      // Perf measurements can still run with the compose default mock mode.
      console.warn(`[perf] unable to read/set mock mode, continuing: ${String(error)}`);
    }

    try {
      // Warm both routes first so budgets represent interactive tab switching.
      await gotoWithBackendRetry(page, '/proxmox');
      await waitForProxmoxReady(page);
      await measureTabTransition(page, 'Docker', waitForDockerReady);
      await measureTabTransition(page, 'Proxmox', waitForProxmoxReady);

      const proxmoxToDockerSamples: number[] = [];
      const dockerToProxmoxSamples: number[] = [];

      for (let i = 0; i < iterations; i++) {
        proxmoxToDockerSamples.push(
          await measureTabTransition(page, 'Docker', waitForDockerReady),
        );
        dockerToProxmoxSamples.push(
          await measureTabTransition(page, 'Proxmox', waitForProxmoxReady),
        );
      }

      const proxmoxToDockerMedianMs = median(proxmoxToDockerSamples);
      const dockerToProxmoxMedianMs = median(dockerToProxmoxSamples);

      console.log(
        `[perf] proxmox->docker samples=${proxmoxToDockerSamples.join(',')} median=${proxmoxToDockerMedianMs}ms budget=${proxmoxToDockerBudgetMs}ms`,
      );
      console.log(
        `[perf] docker->proxmox samples=${dockerToProxmoxSamples.join(',')} median=${dockerToProxmoxMedianMs}ms budget=${dockerToProxmoxBudgetMs}ms`,
      );

      expect(proxmoxToDockerMedianMs).toBeLessThanOrEqual(proxmoxToDockerBudgetMs);
      expect(dockerToProxmoxMedianMs).toBeLessThanOrEqual(dockerToProxmoxBudgetMs);
    } finally {
      if (initialMockMode && !initialMockMode.enabled) {
        try {
          await setMockMode(page, false);
        } catch (error) {
          console.warn(`[perf] unable to restore mock mode, continuing: ${String(error)}`);
        }
      }
    }
  });

  test('large-estate Proxmox section switches stay within budget', async ({
    page,
    browserName,
    isMobile,
  }) => {
    test.skip(
      browserName !== 'chromium' || Boolean(isMobile),
      'Perf budgets are pinned to desktop Chromium',
    );
    test.slow();

    const iterations = toPositiveInt(process.env.PULSE_E2E_PERF_ITERATIONS, 3);
    const overviewToStorageBudgetMs = toPositiveInt(
      process.env.PULSE_E2E_PERF_PROXMOX_OVERVIEW_TO_STORAGE_BUDGET_MS,
      2200,
    );
    const storageToOverviewBudgetMs = toPositiveInt(
      process.env.PULSE_E2E_PERF_PROXMOX_STORAGE_TO_OVERVIEW_BUDGET_MS,
      2200,
    );
    const estate = makeLargeProxmoxEstate();
    const typeCounts = {
      agent: 50,
      vm: 675,
      'system-container': 225,
      storage: 200,
      physical_disk: 100,
    };

    await page.routeWebSocket('**/ws*', () => {});
    await page.route('**/api/resources?**', async (route) => {
      const url = new URL(route.request().url());
      const types = url.searchParams.get('type');
      const sources = url.searchParams.get('source');
      let resources: readonly Record<string, unknown>[] | undefined;

      if (
        types === 'agent,vm,system-container,oci-container' &&
        sources === 'proxmox'
      ) {
        resources = estate.overview;
      } else if (
        types === 'agent,pbs,storage,physical_disk,ceph' &&
        sources === 'proxmox,pbs,agent'
      ) {
        resources = estate.storage;
      }

      if (!resources) {
        await route.continue();
        return;
      }

      const pageNumber = Math.max(1, Number(url.searchParams.get('page')) || 1);
      const limit = Math.max(1, Number(url.searchParams.get('limit')) || 100);
      const totalPages = Math.max(1, Math.ceil(resources.length / limit));
      const start = (pageNumber - 1) * limit;
      const data = resources.slice(start, start + limit);
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          data,
          meta: {
            page: pageNumber,
            limit,
            total: resources.length,
            totalPages,
          },
          aggregations: { total: 1_250, byType: typeCounts },
        }),
      });
    });

    await ensureAuthenticated(page);
    await gotoWithBackendRetry(page, '/proxmox/overview');
    await waitForProxmoxReady(page);
    await expect(page.locator('tr[data-guest-id]').first()).toBeVisible();
    const mountedGuestRows = await page.locator('tr[data-guest-id]').count();
    expect(mountedGuestRows).toBeGreaterThan(0);
    expect(mountedGuestRows).toBeLessThanOrEqual(140);

    // Warm both retained sections before measuring the operator's steady-state
    // workflow. The synthetic estate matches the demo performance profile but
    // keeps this browser gate independent of mutable backend fixture settings.
    await measureProxmoxSectionTransition(page, 'Storage');
    await measureProxmoxSectionTransition(page, 'Overview');

    const overviewToStorageSamples: number[] = [];
    const storageToOverviewSamples: number[] = [];
    for (let index = 0; index < iterations; index++) {
      overviewToStorageSamples.push(
        await measureProxmoxSectionTransition(page, 'Storage'),
      );
      storageToOverviewSamples.push(
        await measureProxmoxSectionTransition(page, 'Overview'),
      );
    }

    const overviewToStorageMedianMs = median(overviewToStorageSamples);
    const storageToOverviewMedianMs = median(storageToOverviewSamples);
    console.log(
      `[perf] proxmox overview->storage samples=${overviewToStorageSamples.join(',')} median=${overviewToStorageMedianMs}ms budget=${overviewToStorageBudgetMs}ms`,
    );
    console.log(
      `[perf] proxmox storage->overview samples=${storageToOverviewSamples.join(',')} median=${storageToOverviewMedianMs}ms budget=${storageToOverviewBudgetMs}ms`,
    );

    expect(overviewToStorageMedianMs).toBeLessThanOrEqual(
      overviewToStorageBudgetMs,
    );
    expect(storageToOverviewMedianMs).toBeLessThanOrEqual(
      storageToOverviewBudgetMs,
    );
  });
});
