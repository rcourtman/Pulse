import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { test as base, type Page } from '@playwright/test';
import {
  apiRequest,
  createAuthenticatedStorageState,
  ensureSessionAuthenticated,
  waitForPulseReady,
} from '../helpers';

const __dirname = path.dirname(fileURLToPath(import.meta.url));

/**
 * Worker-scoped cookie-session fixture for the journey suite.
 *
 * Each Playwright test runs in a fresh browser context, and the token auth
 * path (ensureAuthenticated) leaves page.request cookie-less, so API calls
 * 401. Switching every test to ensureSessionAuthenticated fixes the cookie
 * gap but forces a login POST per test — with ~30 journey tests that trips
 * the backend's 10-logins/min/IP rate limit ("retry login form
 * unavailable").
 *
 * This fixture logs in once per worker (createAuthenticatedStorageState
 * further reuses a single shared cookie session across the whole run) and
 * hands every test a pre-authenticated storage state. Test bodies still call
 * ensureSessionAuthenticated, but with cookies already present login()
 * short-circuits on the authenticated-redirect and issues no POST.
 *
 * Tests that must exercise the login flow itself (01 'health check',
 * 01 'bootstrap and login land on Infrastructure') import the plain
 * @playwright/test `test` instead so they start unauthenticated.
 */
export type JourneyWorkerFixtures = {
  authStorageStatePath: string;
};

export const test = base.extend<Record<string, never>, JourneyWorkerFixtures>({
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
        `journey-${workerInfo.project.name}-${workerInfo.workerIndex}.json`,
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

export { expect } from '@playwright/test';

/**
 * Ready-and-authenticated check for cookie-session journey tests.
 *
 * The worker fixture hands each test a valid cookie session via storageState.
 * That cookie already authenticates page.request (apiRequest) and any direct
 * navigation to a real app route. What it must NEVER do is route through '/'
 * or '/login': those are pre-auth bootstrap paths that require a
 * sessionStorage hint the storage-state cookie doesn't carry (issue #1574), so
 * the app shows the login form there even with a valid session — which is
 * exactly what a per-test login()/ensureSessionAuthenticated would trigger,
 * issuing a real login POST every test and tripping the backend's
 * 10-logins/min/IP rate limit.
 *
 * So we verify the cookie against the API directly (no navigation, no login),
 * land the page on a non-bootstrap authenticated route for UI-facing tests,
 * and only fall back to a real session login if the cookie is genuinely
 * missing or expired. In the normal path this issues zero logins per test.
 */
export async function ensureJourneyReady(page: Page): Promise<void> {
  await waitForPulseReady(page);

  let authed = false;
  try {
    const res = await apiRequest(page, '/api/state');
    authed = res.ok();
  } catch {
    authed = false;
  }

  if (!authed) {
    // Cookie missing/expired — establish a real session (rare; the fixture
    // normally guarantees a live cookie for the whole worker).
    await ensureSessionAuthenticated(page);
    return;
  }

  // Cookie is valid. Land on a real authenticated route (never '/') so
  // UI-facing tests find the app shell already rendered.
  await page.goto('/infrastructure', { waitUntil: 'domcontentloaded' });
}
