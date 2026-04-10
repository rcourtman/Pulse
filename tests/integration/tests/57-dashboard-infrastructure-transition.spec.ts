import { test, expect, type Page } from '@playwright/test';
import { ensureAuthenticated, getMockMode, setMockMode } from './helpers';

const waitForDashboardReady = async (page: Page) => {
  await expect(page).toHaveURL(/\/dashboard(?:\?.*)?$/);
  await expect(page.getByTestId('dashboard-page')).toBeVisible();
  await page.waitForFunction(
    () => !document.querySelector('[data-testid="dashboard-loading"]'),
    undefined,
    { timeout: 30_000 },
  );
};

const waitForInfrastructureReady = async (page: Page) => {
  await expect(page).toHaveURL(/\/infrastructure(?:\?.*)?$/);
  await expect(page.getByTestId('infrastructure-page')).toBeVisible();
  await page.waitForFunction(
    () => {
      if (document.querySelector('[data-testid="infrastructure-loading"]')) {
        return false;
      }
      if (document.querySelector('[data-testid="infrastructure-summary"]')) {
        return true;
      }
      const text = document.body?.textContent || '';
      return text.includes('No infrastructure resources yet');
    },
    undefined,
    { timeout: 30_000 },
  );
};

test('dashboard handoff keeps infrastructure tab switches free of transient loading shells', async ({
  page,
}) => {
  await ensureAuthenticated(page);

  let initialMockMode: { enabled: boolean } | null = null;
  try {
    initialMockMode = await getMockMode(page);
    if (!initialMockMode.enabled) {
      await setMockMode(page, true);
    }
  } catch (error) {
    console.warn(`[transition] unable to read/set mock mode, continuing: ${String(error)}`);
  }

  try {
    await page.goto('/dashboard');
    await waitForDashboardReady(page);

    await page.evaluate(() => {
      const state = {
        routeFallback: false,
        infrastructureLoading: false,
      };
      const root = document.getElementById('root') || document.body;
      const observer = new MutationObserver(() => {
        const text = document.body?.innerText || '';
        if (text.includes('Loading view...')) {
          state.routeFallback = true;
        }
        if (document.querySelector('[data-testid="infrastructure-loading"]')) {
          state.infrastructureLoading = true;
        }
      });
      observer.observe(root, {
        childList: true,
        subtree: true,
        characterData: true,
        attributes: true,
      });
      (window as typeof window & {
        __pulseTransitionObserver?: MutationObserver;
        __pulseTransitionState?: typeof state;
      }).__pulseTransitionObserver = observer;
      (window as typeof window & {
        __pulseTransitionObserver?: MutationObserver;
        __pulseTransitionState?: typeof state;
      }).__pulseTransitionState = state;
    });

    await page.getByRole('tab', { name: 'Infrastructure', exact: true }).first().click();
    await waitForInfrastructureReady(page);

    const transitionState = await page.evaluate(() => {
      const win = window as typeof window & {
        __pulseTransitionObserver?: MutationObserver;
        __pulseTransitionState?: {
          routeFallback: boolean;
          infrastructureLoading: boolean;
        };
      };
      win.__pulseTransitionObserver?.disconnect();
      return (
        win.__pulseTransitionState ?? {
          routeFallback: false,
          infrastructureLoading: false,
        }
      );
    });

    expect(transitionState).toEqual({
      routeFallback: false,
      infrastructureLoading: false,
    });
  } finally {
    if (initialMockMode && !initialMockMode.enabled) {
      try {
        await setMockMode(page, false);
      } catch (error) {
        console.warn(`[transition] unable to restore mock mode, continuing: ${String(error)}`);
      }
    }
  }
});
