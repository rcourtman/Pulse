// Offline real-browser regression for #1723.
//
// Mounts the production ProxmoxBackupServersTable with a PBS host whose agent
// is present twice (PVE guest with agent telemetry, and a standalone source=pbs
// host row), plus two datastores. Switches between them, replaces/reorders
// snapshots with fresh CPU telemetry, and checks chart paths and tab retention.
// Records the metrics
// history target the drawer requests. The correct target is the guest's vm
// series; the PBS service target has no host history and leaves the tab on
// "Collecting history". Run with:
//   pulse-worker-browser scripts/check-pbs-host-history-correlation.cjs
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
    server: { host: '127.0.0.1', port: 5198, strictPort: true },
  });
  let browser;
  const results = [];
  const pageErrors = [];
  const expected = 'vm/proxmox:100';
  const wrong = 'agent/pbs-1';
  try {
    await server.listen();
    browser = await chromium.launch(launchOptions);
    for (const viewport of [
      { width: 1280, height: 900 },
      { width: 390, height: 844 },
    ]) {
      const targets = [];
      const page = await browser.newPage({ viewport });
      page.on('pageerror', (error) => pageErrors.push(`${viewport.width}px: ${error.message}`));

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
        targets.push(`${query.get('resourceType')}/${query.get('resourceId')}`);
        await route.fulfill({
          json: {
            resourceType: query.get('resourceType'),
            resourceId: query.get('resourceId'),
            range: query.get('range'),
            start: Date.now() - 3600000,
            end: Date.now(),
            source: 'store',
            metrics: Object.fromEntries(
              ['cpu', 'memory', 'disk', 'netin', 'netout', 'diskread', 'diskwrite'].map(
                (metric) => [
                  metric,
                  [30, 20, 10].map((minutes, i) => ({
                    timestamp: Date.now() - minutes * 60000,
                    value: 10 + i * 5,
                    min: 10 + i * 5,
                    max: 10 + i * 5,
                  })),
                ],
              ),
            ),
          },
        });
      });

      await page.goto('http://127.0.0.1:5198/browser-tests/pbs-host-history-correlation.html');
      const rows = page.locator('tr').filter({
        has: page.getByRole('button', {
          name: 'Expand details for proxback',
          exact: true,
        }),
      });
      await rows.first().waitFor({ timeout: 20000 });
      const openDatastore = async (name) => {
        const row = page
          .locator('tr')
          .filter({ has: page.locator(`td[title="proxback · ${name}"]`) });
        await row.getByRole('button').focus();
        await page.keyboard.press('Enter');
        await page.getByRole('tab', { name: 'History', exact: true }).click();
        await page.locator('[data-testid="guest-history-plot"] path').first().waitFor();
        assert.equal(
          await page
            .getByRole('tab', { name: 'History', exact: true })
            .getAttribute('aria-selected'),
          'true',
        );
      };
      const observations = [];
      // Both datastore rows must use the same host series, including revisiting
      // the first row after its history component has been disposed.
      for (const [index, datastore] of ['tank', 'archive', 'tank'].entries()) {
        assert.deepEqual(
          await page
            .locator('td[title^="proxback · "]')
            .evaluateAll((nodes) => nodes.map((node) => node.getAttribute('title'))),
          ['proxback · archive', 'proxback · tank'],
        );
        await openDatastore(datastore);
        const detail = page.locator('[data-inline-platform-resource-detail-for="pbs-1"]');
        await detail.evaluate((node) => {
          node.dataset.proofIdentity = 'retained';
        });
        const pathsBefore = await detail.locator('[data-testid="guest-history-plot"] path').count();
        assert.ok(pathsBefore > 0);
        await page.getByRole('button', { name: 'Refresh resource snapshot' }).click();
        // Datastore usage is visible at both widths and proves that the fresh
        // snapshot reached the UI (the CPU column is hidden on narrow screens).
        await page
          .getByText(`${(41 + index).toFixed(1)}%`, { exact: true })
          .first()
          .waitFor();
        assert.equal(
          await detail.getAttribute('data-proof-identity'),
          'retained',
          'drawer remounted',
        );
        assert.equal(
          await page
            .getByRole('tab', { name: 'History', exact: true })
            .getAttribute('aria-selected'),
          'true',
          'history reset after snapshot',
        );
        assert.equal(
          await detail.locator('[data-testid="guest-history-plot"] path').count(),
          pathsBefore,
        );
        observations.push({
          datastore,
          paths: pathsBefore,
          refreshPreservedHistory: true,
        });
      }

      if (!targets.includes(expected)) {
        throw new Error(
          `${viewport.width}px: expected the guest history target ${expected}; recorded ${JSON.stringify(targets)}`,
        );
      }
      if (targets.includes(wrong)) {
        throw new Error(
          `${viewport.width}px: PBS service target ${wrong} was requested instead of the guest series`,
        );
      }

      await page.screenshot({
        path: path.join(ROOT, 'browser-tests', `pbs-host-history-${viewport.width}.png`),
      });
      results.push({ viewport, targets, observations });
      await page.close();
    }

    if (pageErrors.length > 0) {
      throw new Error(`page errors: ${pageErrors.join(' | ')}`);
    }

    console.log(
      JSON.stringify({ result: 'passed', browser: browser.version(), expected, results }, null, 2),
    );
  } finally {
    if (browser) await browser.close();
    await server.close();
  }
})().catch((error) => {
  console.error('FAILED:', error && error.stack ? error.stack : error);
  process.exitCode = 1;
});
