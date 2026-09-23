// Offline real-browser proof for the #1723 drawer tab-retention repair.
//
// Opens the production Proxmox Backups server drawer, selects History, then
// drives a snapshot that transiently removes the merged host metrics target
// (the field that gates the History tab). The drawer must keep the user's
// History selection and show the in-tab availability notice instead of
// silently falling back to Overview; restoring the target must bring the chart
// back on the same selection. Synthetic API responses prove UI behaviour, not
// backend persistence or installed release behaviour.
// Run: pulse-worker-browser scripts/check-drawer-tab-retention.cjs
const path = require('node:path');
const assert = require('node:assert/strict');
const { chromium } = require('playwright');

const ROOT = path.resolve(process.cwd(), 'frontend-modern');
const launchOptions = {
  headless: true,
  channel: 'chromium',
  args: ['--no-sandbox'],
};

(async () => {
  process.chdir(ROOT);
  const { createServer } = await import(
    path.join(ROOT, 'node_modules', 'vite', 'dist', 'node', 'index.js')
  );
  const server = await createServer({
    root: ROOT,
    configFile: path.join(ROOT, 'vite.config.ts'),
    server: { host: '127.0.0.1', port: 5199, strictPort: true },
  });
  let browser;
  const results = [];
  const pageErrors = [];
  const failures = [];
  try {
    await server.listen();
    browser = await chromium.launch(launchOptions);
    for (const viewport of [
      { width: 1280, height: 900 },
      { width: 390, height: 844 },
    ]) {
      const caseName = `${viewport.width}`;
      const page = await browser.newPage({ viewport });
      page.on('pageerror', (error) =>
        pageErrors.push(`${viewport.width}px: ${error.message}`),
      );
      await page.route('**/api/license/runtime-capabilities', (route) =>
        route.fulfill({
          json: {
            capabilities: [],
            limits: [],
            hosted_mode: false,
            max_history_days: 7,
            runtime: { build: 'community', label: 'Pulse Community runtime' },
            blocked_capabilities: [],
          },
        }),
      );
      await page.route('**/api/settings/ai', (route) =>
        route.fulfill({ json: { enabled: false } }),
      );
      await page.route('**/api/metrics-store/history?**', async (route) => {
        const query = new URL(route.request().url()).searchParams;
        await route.fulfill({
          json: {
            resourceType: query.get('resourceType'),
            resourceId: query.get('resourceId'),
            range: query.get('range'),
            start: Date.now() - 3600000,
            end: Date.now(),
            source: 'store',
            metrics: Object.fromEntries(
              [
                'cpu',
                'memory',
                'disk',
                'netin',
                'netout',
                'diskread',
                'diskwrite',
              ].map((metric) => [
                metric,
                [30, 20, 10].map((minutes, i) => ({
                  timestamp: Date.now() - minutes * 60000,
                  value: 10 + i * 5,
                  min: 10 + i * 5,
                  max: 10 + i * 5,
                })),
              ]),
            ),
          },
        });
      });

      try {
        await page.goto(
          'http://127.0.0.1:5199/browser-tests/pbs-host-history-correlation.html?topology=guest&order=stable',
          { timeout: 60000 },
        );
        const serverRow = page
          .locator('tr')
          .filter({
            has: page.getByRole('button', {
              name: 'Expand details for proxback',
              exact: true,
            }),
          })
          .first();
        await serverRow.waitFor({ timeout: 20000 });
        await serverRow.getByRole('button').focus();
        await page.keyboard.press('Enter');
        await page
          .getByRole('tab', { name: 'History', exact: true })
          .click();
        await page
          .locator('[data-testid="guest-history-plot"] path')
          .first()
          .waitFor({ timeout: 20000 });
        const detail = page.locator(
          '[data-inline-platform-resource-detail-for="pbs-1"]',
        );
        await detail.evaluate((node) => {
          node.dataset.proofIdentity = 'retained';
        });
        const pathsBefore = await detail
          .locator('[data-testid="guest-history-plot"] path')
          .count();
        assert.ok(pathsBefore > 0, 'history plot did not render before the drop');

        // Snapshot arrives without the merged host metrics target.
        await page.getByRole('button', { name: 'Drop metrics target' }).click();
        await page
          .getByText('Metrics history is unavailable.', { exact: true })
          .waitFor({ timeout: 20000 });
        assert.equal(
          await detail.getAttribute('data-proof-identity'),
          'retained',
          'drawer remounted on the transient target loss',
        );

        // Next snapshot restores it; the same History selection must recover.
        await page.getByRole('button', { name: 'Restore metrics target' }).click();
        await page
          .locator('[data-testid="guest-history-plot"] path')
          .first()
          .waitFor({ timeout: 20000 });
        assert.equal(
          await page
            .getByRole('tab', { name: 'History', exact: true })
            .getAttribute('aria-selected'),
          'true',
          'history selection was discarded across the transient target loss',
        );
        assert.equal(
          await detail
            .locator('[data-testid="guest-history-plot"] path')
            .count(),
          pathsBefore,
        );
        results.push({ viewport, paths: pathsBefore, result: 'passed' });
      } catch (error) {
        failures.push(`${caseName}: ${error.message}`);
        results.push({ viewport, result: 'failed', error: error.message });
      } finally {
        await page.screenshot({
          path: path.join(ROOT, 'browser-tests', `drawer-tab-retention-${caseName}.png`),
        });
        await page.close();
      }
    }

    if (pageErrors.length > 0) {
      throw new Error(`page errors: ${pageErrors.join(' | ')}`);
    }
    console.log(
      JSON.stringify(
        {
          result: failures.length ? 'failed' : 'passed',
          browser: browser.version(),
          results,
          failures,
        },
        null,
        2,
      ),
    );
    assert.equal(failures.length, 0, failures.join(' | '));
  } finally {
    if (browser) await browser.close();
    await server.close();
  }
})().catch((error) => {
  console.error('FAILED:', error && error.stack ? error.stack : error);
  process.exitCode = 1;
});
