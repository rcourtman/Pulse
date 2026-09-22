// Offline real-browser regression for #1723.
//
// Mounts the production ProxmoxBackupServersTable with a PBS host whose agent
// is present twice (PVE guest with agent telemetry, and a standalone source=pbs
// host row). Expands the PBS row, opens History, and records the metrics
// history target the drawer requests. The correct target is the guest's vm
// series; the PBS service target has no host history and leaves the tab on
// "Collecting history". Run with:
//   pulse-worker-browser scripts/check-pbs-host-history-correlation.cjs
const path = require('node:path');
const { chromium } = require('playwright');

const ROOT = path.resolve(process.cwd(), 'frontend-modern');
const launchOptions = { headless: true, channel: 'chromium', args: ['--no-sandbox'] };

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

      await page.route('**/api/metrics-store/history?**', async (route) => {
        const query = new URL(route.request().url()).searchParams;
        targets.push(`${query.get('resourceType')}/${query.get('resourceId')}`);
        await route.fulfill({
          json: {
            resourceType: query.get('resourceType'),
            resourceId: query.get('resourceId'),
            points: [],
          },
        });
      });

      await page.goto('http://127.0.0.1:5198/browser-tests/pbs-host-history-correlation.html');
      const expand = page.getByRole('button', {
        name: 'Expand details for proxback',
        exact: true,
      });
      await expand.waitFor({ timeout: 20000 });
      await expand.focus();
      await page.keyboard.press('Enter');
      await page.getByRole('tab', { name: 'History', exact: true }).click();
      // Give the history fetch a beat to fire.
      await page.waitForTimeout(750);

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
      results.push({ viewport, targets });
      await page.close();
    }

    if (pageErrors.length > 0) {
      throw new Error(`page errors: ${pageErrors.join(' | ')}`);
    }

    console.log(JSON.stringify({ result: 'passed', expected, results }, null, 2));
  } finally {
    if (browser) await browser.close();
    await server.close();
  }
})().catch((error) => {
  console.error('FAILED:', error && error.stack ? error.stack : error);
  process.exitCode = 1;
});
