import { test, expect, type Page } from '@playwright/test';
import { ensureAuthenticated, getMockMode, setMockMode } from './helpers';

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

const waitForInfrastructureReady = async (page: Page) => {
  await expect(page).toHaveURL(/\/infrastructure(?:\?.*)?$/);
  await expect(page.getByTestId('infrastructure-page')).toBeVisible();
  await page.waitForFunction(
    () => {
      if (document.querySelector('[data-testid="infrastructure-summary"]')) return true;
      const text = document.body?.textContent || '';
      return text.includes('No infrastructure resources yet');
    },
    undefined,
    { timeout: 30_000 },
  );
};

const waitForWorkloadsReady = async (page: Page) => {
  await expect(page).toHaveURL(/\/workloads(?:\?.*)?$/);
  await expect(page.getByTestId('workloads-summary')).toBeVisible();
};

const measureTabTransition = async (
  page: Page,
  tabName: 'Infrastructure' | 'Workloads',
  waitForReady: (page: Page) => Promise<void>,
): Promise<number> => {
  const start = Date.now();
  await page.getByRole('tab', { name: new RegExp(`^${tabName}$`) }).first().click();
  await waitForReady(page);
  return Date.now() - start;
};

test.describe.serial('Navigation performance budgets', () => {
  test.skip(!truthy(process.env.PULSE_E2E_PERF), 'Set PULSE_E2E_PERF=1 to enable navigation perf checks');

  test('infrastructure and workloads tab switches stay within budget', async ({ page }) => {
    test.slow();

    const iterations = toPositiveInt(process.env.PULSE_E2E_PERF_ITERATIONS, 3);
    const infraToWorkloadsBudgetMs = toPositiveInt(process.env.PULSE_E2E_PERF_INFRA_TO_WORKLOADS_BUDGET_MS, 2200);
    const workloadsToInfraBudgetMs = toPositiveInt(process.env.PULSE_E2E_PERF_WORKLOADS_TO_INFRA_BUDGET_MS, 2200);

    await ensureAuthenticated(page);

    const initialMockMode = await getMockMode(page);
    if (!initialMockMode.enabled) {
      await setMockMode(page, true);
    }

    try {
      // Warm both routes first so budgets represent interactive tab switching.
      await page.goto('/infrastructure');
      await waitForInfrastructureReady(page);
      await measureTabTransition(page, 'Workloads', waitForWorkloadsReady);
      await measureTabTransition(page, 'Infrastructure', waitForInfrastructureReady);

      const infraToWorkloadsSamples: number[] = [];
      const workloadsToInfraSamples: number[] = [];

      for (let i = 0; i < iterations; i++) {
        infraToWorkloadsSamples.push(
          await measureTabTransition(page, 'Workloads', waitForWorkloadsReady),
        );
        workloadsToInfraSamples.push(
          await measureTabTransition(page, 'Infrastructure', waitForInfrastructureReady),
        );
      }

      const infraToWorkloadsMedianMs = median(infraToWorkloadsSamples);
      const workloadsToInfraMedianMs = median(workloadsToInfraSamples);

      console.log(
        `[perf] infra->workloads samples=${infraToWorkloadsSamples.join(',')} median=${infraToWorkloadsMedianMs}ms budget=${infraToWorkloadsBudgetMs}ms`,
      );
      console.log(
        `[perf] workloads->infra samples=${workloadsToInfraSamples.join(',')} median=${workloadsToInfraMedianMs}ms budget=${workloadsToInfraBudgetMs}ms`,
      );

      expect(infraToWorkloadsMedianMs).toBeLessThanOrEqual(infraToWorkloadsBudgetMs);
      expect(workloadsToInfraMedianMs).toBeLessThanOrEqual(workloadsToInfraBudgetMs);
    } finally {
      if (!initialMockMode.enabled) {
        await setMockMode(page, false);
      }
    }
  });
});
