// Offline verification of the production table, drawer and History requests.
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const { createHash } = require('node:crypto');
const { chromium } = require('playwright');

(async () => {
  const root = '/workspace/frontend-modern';
  const artifacts = path.join(root, 'node_modules/pbs-identity-boundary-proof');
  fs.mkdirSync(artifacts, { recursive: true });
  fs.chmodSync(artifacts, 0o755);
  process.chdir(root);
  const { createServer } = await import(path.join(root, 'node_modules/vite/dist/node/index.js'));
  const server = await createServer({
    root,
    configFile: path.join(root, 'vite.config.ts'),
    server: { host: '127.0.0.1', port: 5211, strictPort: true },
  });
  let browser;
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
      page.setDefaultTimeout(20_000);
      const pageErrors = [];
      const requests = [];
      page.on('pageerror', (error) => pageErrors.push(error.message));
      await page.route('**/*', (route) => {
        const url = new URL(route.request().url());
        if (url.origin !== 'http://127.0.0.1:5211') return route.abort();
        if (!url.pathname.startsWith('/api/')) return route.continue();
        if (url.pathname === '/api/metrics-store/history') {
          const type = url.searchParams.get('resourceType');
          const id = url.searchParams.get('resourceId');
          requests.push({ method: route.request().method(), target: `${type}:${id}` });
          assert.ok(!id.startsWith('unrelated-'), `wrong-host History requested: ${id}`);
          return route.fulfill({
            json: {
              resourceType: type,
              resourceId: id,
              range: url.searchParams.get('range'),
              start: Date.now() - 3_600_000,
              end: Date.now(),
              source: 'store',
              // PBS API supplies only service CPU/memory, not invented host series.
              metrics: Object.fromEntries(
                (id.startsWith('pbs-')
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
        if (url.pathname === '/api/license/runtime-capabilities') {
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
        }
        return route.fulfill({ json: { data: [], enabled: false } });
      });
      await page.goto('http://127.0.0.1:5211/browser-tests/pbs-identity-boundary.html', {
        waitUntil: 'domcontentloaded',
        timeout: 120_000,
      });
      const observations = [];
      const inspect = async (suffix, expected, state, open = true) => {
        const detail = page.locator(`[data-inline-platform-resource-detail-for="pbs-${suffix}"]`);
        if (open) {
          const toggle = page.getByRole('button', {
            name: `Expand details for backup-connection-${suffix}`,
            exact: true,
          });
          if (width === 1365) {
            await toggle.focus();
            await page.keyboard.press('Enter');
          } else await page.locator(`td[title="backup-connection-${suffix} · tank"]`).click();
        }
        await detail.waitFor();
        const overview = detail.getByRole('tab', { name: 'Overview', exact: true });
        await overview.click();
        const identity = detail
          .locator('[data-testid="resource-identity-section"] tr')
          .filter({ hasText: 'Metrics Target' });
        await identity.getByText(expected, { exact: true }).waitFor();
        assert.ok(!(await detail.innerText()).includes('/WRONG-HOST'), 'wrong-host disks rendered');
        await detail.getByRole('tab', { name: 'History', exact: true }).click();
        await detail.locator('[data-testid="guest-history-plot"] path').first().waitFor();
        assert.equal(
          await detail
            .getByRole('tab', { name: 'History', exact: true })
            .getAttribute('aria-selected'),
          'true',
        );
        assert.ok(
          requests.some((request) => request.target === expected),
          `missing History read for ${expected}`,
        );
        const targetStart = requests.length;
        await detail.getByTestId('guest-history-range-control').selectOption('6h');
        // Force a fresh read even when the open drawer already has 6h selected.
        await detail.getByTestId('guest-history-range-control').selectOption('1h');
        await page.waitForFunction(() => !document.body.innerText.includes('Loading history'));
        const dimensions = await page.evaluate(() => ({
          scroll: document.documentElement.scrollWidth,
          inner: innerWidth,
        }));
        assert.ok(dimensions.scroll <= dimensions.inner + 1, JSON.stringify(dimensions));
        observations.push({
          state,
          suffix,
          expected,
          dimensions,
          newTargets: requests.slice(targetStart),
        });
        console.log(JSON.stringify({ width, state, expected }));
      };
      for (const suffix of ['one', 'two', 'three'])
        await inspect(suffix, `agent:pbs-${suffix}`, 'no-link-label-collisions');
      await page.screenshot({
        path: path.join(artifacts, `unlinked-${width}.png`),
        fullPage: true,
      });
      await page.getByRole('button', { name: 'Corroborate links', exact: true }).click();
      await inspect('three', 'agent:agent-three', 'explicit-link', false);
      await inspect('one', 'vm:vm-one', 'same-Agent-guest-deduplication');
      await inspect('two', 'vm:vm-two', 'second-guest');
      await inspect('three', 'agent:agent-three', 'standalone-host');
      await page.screenshot({ path: path.join(artifacts, `linked-${width}.png`), fullPage: true });
      await page.getByRole('button', { name: 'Omit linked host rows', exact: true }).click();
      await inspect('three', 'agent:agent-three', 'unchanged-link-host-omission', false);
      await page.getByRole('button', { name: 'Restore linked host rows', exact: true }).click();
      await inspect('three', 'agent:agent-three', 'restored-host', false);
      await page.getByRole('button', { name: 'Withdraw third link', exact: true }).click();
      await inspect('three', 'agent:pbs-three', 'withdrawn-link-with-hosts-still-present', false);
      assert.ok(
        !(
          await page.locator('[data-inline-platform-resource-detail-for="pbs-three"]').innerText()
        ).includes('/real-three'),
        'withdrawn host disks remained visible',
      );
      await page.screenshot({
        path: path.join(artifacts, `withdrawn-${width}.png`),
        fullPage: true,
      });
      await page.getByRole('button', { name: 'Report exact node hostname', exact: true }).click();
      await inspect('one', 'vm:vm-one', 'exact-reported-machine-hostname');
      await inspect('three', 'agent:pbs-three', 'remaining-uncorroborated-host');
      assert.ok(requests.length > 0);
      assert.ok(requests.every((request) => request.method === 'GET'));
      assert.deepEqual(pageErrors, []);
      results.push({ width, observations, requests, pageErrors });
      await page.close();
    }
    const file = 'src/features/proxmox/ProxmoxBackupServersTable.tsx';
    const result = {
      result: 'passed',
      browser: browser.version(),
      playwright: '1.56.1',
      content_sha256: {
        [`frontend-modern/${file}`]: createHash('sha256')
          .update(fs.readFileSync(path.join(root, file)))
          .digest('hex'),
      },
      results,
    };
    fs.writeFileSync(path.join(artifacts, 'result.json'), JSON.stringify(result, null, 2) + '\n');
    for (const name of fs.readdirSync(artifacts)) fs.chmodSync(path.join(artifacts, name), 0o644);
    console.log(JSON.stringify(result));
  } finally {
    if (browser) await browser.close();
    await server.close();
  }
})().catch((error) => {
  console.error(error);
  process.exitCode = 1;
});
