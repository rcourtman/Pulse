// Offline browser proof; synthetic API responses are not installed History.
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const { createHash } = require('node:crypto');
const { chromium } = require('playwright');

(async () => {
  const root = '/workspace/frontend-modern';
  const artifacts = path.join(root, 'node_modules/pbs-retention-guard-proof');
  fs.mkdirSync(artifacts, { recursive: true });
  process.chdir(root);
  const { createServer } = await import(path.join(root, 'node_modules/vite/dist/node/index.js'));
  const server = await createServer({
    root,
    configFile: path.join(root, 'vite.config.ts'),
    server: { host: '127.0.0.1', port: 5211, strictPort: true },
  });
  let browser;
  let activePage;
  const results = [];
  try {
    await server.listen();
    browser = await chromium.launch({
      headless: true,
      channel: 'chromium',
      args: ['--no-sandbox'],
    });
    for (const width of [1365, 390]) {
      const page = await browser.newPage({
        viewport: { width, height: width === 390 ? 844 : 900 },
      });
      activePage = page;
      page.setDefaultTimeout(20_000);
      const pageErrors = [];
      const requests = [];
      const observations = [];
      page.on('pageerror', (error) => pageErrors.push(error.message));
      await page.route('**/*', (route) => {
        const url = new URL(route.request().url());
        if (url.origin !== 'http://127.0.0.1:5211') return route.abort();
        if (!url.pathname.startsWith('/api/')) return route.continue();
        if (url.pathname === '/api/metrics-store/history') {
          const type = url.searchParams.get('resourceType');
          const id = url.searchParams.get('resourceId');
          requests.push({
            method: route.request().method(),
            target: `${type}:${id}`,
            range: url.searchParams.get('range'),
          });
          return route.fulfill({
            json: {
              resourceType: type,
              resourceId: id,
              range: url.searchParams.get('range'),
              start: Date.now() - 3_600_000,
              end: Date.now(),
              source: 'store',
              metrics: Object.fromEntries(
                (id === 'pbs-service'
                  ? ['cpu', 'memory']
                  : ['cpu', 'memory', 'disk', 'netin', 'netout', 'diskread', 'diskwrite']
                ).map((metric) => [
                  metric,
                  [30, 20, 10].map((minutes, index) => ({
                    timestamp: Date.now() - minutes * 60_000,
                    value: 10 + index * 5,
                    min: 10 + index * 5,
                    max: 10 + index * 5,
                  })),
                ]),
              ),
            },
          });
        }
        if (url.pathname === '/api/license/runtime-capabilities')
          return route.fulfill({
            json: {
              capabilities: [],
              limits: [],
              max_history_days: 7,
              hosted_mode: false,
              runtime: { build: 'community', label: 'Pulse Community runtime' },
              blocked_capabilities: [],
            },
          });
        return route.fulfill({ json: { data: [], enabled: false } });
      });
      await page.goto('http://127.0.0.1:5211/browser-tests/pbs-retention-guard.html', {
        waitUntil: 'domcontentloaded',
        timeout: 120_000,
      });
      const toggle = page.getByRole('button', {
        name: 'Expand details for backup-connection',
        exact: true,
      });
      if (width === 1365) {
        await toggle.focus();
        await page.keyboard.press('Enter');
      } else await page.locator('td[title="backup-connection · tank"]').click();
      const detail = page.locator('[data-inline-platform-resource-detail-for="pbs-service"]');
      await detail.waitFor();
      const inspect = async (state, expected, hostExpected) => {
        await detail.getByRole('tab', { name: 'Overview', exact: true }).click();
        const identity = detail
          .locator('[data-testid="resource-identity-section"] tr')
          .filter({ hasText: 'Metrics Target' });
        await identity.getByText(expected, { exact: true }).waitFor();
        const platformDetails = detail.getByTestId('resource-platform-details');
        if (await platformDetails.count()) {
          if ((await platformDetails.getAttribute('open')) === null)
            await platformDetails.locator('summary').click();
        }
        const hostDetails = detail.getByTestId('resource-host-details-section');
        if (hostExpected) {
          await hostDetails.waitFor();
          const showHost = hostDetails.getByRole('button', { name: /^Show / });
          if (await showHost.count()) await showHost.click();
          await hostDetails.getByText('/old-host-only', { exact: true }).waitFor();
        } else {
          await hostDetails.waitFor({ state: 'detached' });
          assert.ok(
            !(await detail.innerText()).includes('/old-host-only'),
            `host disks in ${state}`,
          );
        }
        await detail.getByRole('tab', { name: 'History', exact: true }).click();
        await detail.locator('[data-testid="guest-history-plot"] path').first().waitFor();
        const start = requests.length;
        // Force fresh History reads even when the drawer has retained its range.
        for (const range of ['6h', '1h']) {
          const response = page.waitForResponse((r) => {
            const u = new URL(r.url());
            return (
              u.pathname === '/api/metrics-store/history' &&
              u.searchParams.get('resourceId') === expected.split(':')[1] &&
              u.searchParams.get('range') === range
            );
          });
          await detail.getByTestId('guest-history-range-control').selectOption(range);
          await response;
        }
        const fresh = requests.slice(start);
        assert.ok(fresh.length >= 2);
        assert.ok(
          fresh.every((r) => r.target === expected && r.method === 'GET'),
          JSON.stringify(fresh),
        );
        const dimensions = await page.evaluate(() => ({
          scroll: document.documentElement.scrollWidth,
          inner: innerWidth,
        }));
        assert.ok(dimensions.scroll <= dimensions.inner + 1, JSON.stringify(dimensions));
        observations.push({ state, expected, hostExpected, dimensions, fresh });
        console.log(JSON.stringify({ width, state, expected }));
      };
      await inspect('initial-service', 'agent:pbs-service', false);
      await page.getByRole('button', { name: 'Match host', exact: true }).click();
      await inspect('correlated-host', 'agent:host-a', true);
      await page.getByRole('button', { name: 'Omit unchanged host', exact: true }).click();
      await inspect('unchanged-identity-omission', 'agent:host-a', true);
      await page.screenshot({
        path: path.join(artifacts, `retained-${width}.png`),
        fullPage: true,
      });
      await page.getByRole('button', { name: 'Change PBS machine', exact: true }).click();
      await inspect('changed-machine-without-host', 'agent:pbs-service', false);
      await page.screenshot({
        path: path.join(artifacts, `replacement-${width}.png`),
        fullPage: true,
      });
      await page.getByRole('button', { name: 'Match host', exact: true }).click();
      await inspect('reacquired-host', 'agent:host-a', true);
      await page.getByRole('button', { name: 'Ambiguous hosts', exact: true }).click();
      await inspect('ambiguous-hosts', 'agent:pbs-service', false);
      await page.getByRole('button', { name: 'Omit ambiguous hosts', exact: true }).click();
      await inspect('omitted-after-ambiguity', 'agent:pbs-service', false);
      await page.screenshot({
        path: path.join(artifacts, `ambiguous-omitted-${width}.png`),
        fullPage: true,
      });
      assert.deepEqual(pageErrors, []);
      assert.ok(requests.every((r) => r.target !== 'agent:host-b' && r.method === 'GET'));
      results.push({ width, observations, requests, pageErrors });
      await page.close();
      activePage = null;
    }
    const file = 'src/features/proxmox/ProxmoxBackupServersTable.tsx';
    const result = {
      result: 'passed',
      verified_at: new Date().toISOString(),
      browser: browser.version(),
      playwright: require('playwright/package.json').version,
      content_sha256: {
        [`frontend-modern/${file}`]: createHash('sha256')
          .update(fs.readFileSync(path.join(root, file)))
          .digest('hex'),
      },
      results,
    };
    fs.writeFileSync(path.join(artifacts, 'result.json'), JSON.stringify(result, null, 2) + '\n');
    console.log(JSON.stringify(result));
  } catch (error) {
    if (activePage)
      await activePage.screenshot({ path: path.join(artifacts, 'failure.png'), fullPage: true });
    fs.writeFileSync(
      path.join(artifacts, 'failure.json'),
      JSON.stringify({ error: String(error), results }, null, 2) + '\n',
    );
    throw error;
  } finally {
    if (browser) await browser.close();
    await server.close();
    for (const name of fs.readdirSync(artifacts)) fs.chmodSync(path.join(artifacts, name), 0o644);
  }
})().catch((error) => {
  console.error(error);
  process.exitCode = 1;
});
