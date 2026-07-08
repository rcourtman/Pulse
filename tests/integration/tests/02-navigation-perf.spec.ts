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
});
