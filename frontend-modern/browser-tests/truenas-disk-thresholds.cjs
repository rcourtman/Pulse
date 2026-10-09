// Fresh final-byte production table/drawer proof, with synthetic HTTP only.
process.env.RAYON_NUM_THREADS = '2';
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const { chromium, webkit } = require('playwright');
const root = '/workspace/frontend-modern';
const output = '/workspace/tmp/truenas-threshold-browser';
const origin = 'http://127.0.0.1:5327';
const config = (patch = {}) => ({
  enabled: true,
  activationState: 'active',
  guestDefaults: { memory: { trigger: 85, clear: 80 } },
  nodeDefaults: {},
  storageDefault: { trigger: 85, clear: 80 },
  agentDefaults: { diskTemperature: { trigger: 55, clear: 50 } },
  diskTempByType: {
    nvme: { trigger: 70, clear: 65 },
    sas: { trigger: 65, clear: 60 },
    sata: { trigger: 55, clear: 50 },
  },
  truenasDiskTemperatureByType: true,
  overrides: {},
  ...patch,
});

(async () => {
  fs.mkdirSync(output, { recursive: true });
  process.chdir(root);
  const version = require('playwright/package.json').version;
  assert.equal(
    version,
    JSON.parse(fs.readFileSync('/workspace/tests/integration/package-lock.json')).packages[
      'node_modules/@playwright/test'
    ].version,
  );
  const { createServer } = await import(path.join(root, 'node_modules/vite/dist/node/index.js'));
  const server = await createServer({
    cacheDir: path.join(output, 'vite-cache'),
    optimizeDeps: { entries: ['browser-tests/truenas-disk-thresholds.html'] },
    server: { host: '127.0.0.1', port: 5327, strictPort: true },
  });
  const observations = [];
  let browser;
  try {
    await server.listen();
    for (const [name, engine, width, height] of [
      ['chromium-desktop', chromium, 1440, 1000],
      ['webkit-phone', webkit, 390, 844],
    ]) {
      browser = await engine.launch(
        engine === chromium
          ? { headless: true, channel: 'chromium', args: ['--no-sandbox'] }
          : { headless: true },
      );
      const page = await browser.newPage({ viewport: { width, height } });
      page.setDefaultTimeout(12000);
      const errors = [];
      const consoleErrors = [];
      page.on('console', (m) => {
        if (m.type() === 'error') consoleErrors.push(m.text());
      });
      console.log('BROWSER_VIEWPORT', name);
      page.on('pageerror', (e) => errors.push(e.message));
      await page.routeWebSocket(/\/ws(?:\?|$)/, (ws) =>
        ws.send(
          JSON.stringify({
            type: 'initialState',
            data: { connectedInfrastructure: [], activeAlerts: [], metrics: [], stats: {} },
          }),
        ),
      );

      let current = config();
      await page.route('**/*', (route) => {
        const url = new URL(route.request().url());
        if (url.origin !== origin) return route.abort();
        if (url.pathname === '/api/alerts/config') return route.fulfill({ json: current });
        if (url.pathname.startsWith('/api/'))
          return route.fulfill({
            json: { data: [], resources: [], policy: {}, enabled: false, hasAuthentication: true },
          });
        return route.continue();
      });
      // Readiness is distinct from interaction assertions: allow cold fixture compilation
      // within the unchanged executor deadline, then wait for the production mount.
      await page.goto(`${origin}/browser-tests/truenas-disk-thresholds.html`, {
        waitUntil: 'domcontentloaded',
        timeout: 45000,
      });
      await page.waitForFunction(() => !!window.__thresholdFixture, null, { timeout: 45000 });
      if (width < 768) await page.getByRole('button', { name: /^Filters/ }).click();
      await page
        .getByRole('group', { name: 'Storage type', exact: true })
        .getByRole('button', { name: /(?:Physical disks|Disks), 3/ })
        .click();
      const row = (id) => page.locator(`[data-truenas-storage-resource="${id}"]`);
      const health = async (id) => {
        await row(id).waitFor();
        return row(id).evaluate(
          (element) =>
            element
              .querySelector('[data-truenas-storage-health]')
              ?.getAttribute('data-truenas-storage-health') ?? 'healthy',
        );
      };
      const refresh = async (value) => {
        current = value;
        await page.evaluate(() => window.__thresholdFixture.refresh());
      };
      const capture = async (state) => {
        const file = `${name}-${state}.png`;
        await page.screenshot({ path: path.join(output, file), fullPage: true });
        observations.push({
          name,
          state,
          viewport: { width, height },
          file,
          health: [
            await health('nvme-wide'),
            await health('nvme-raised'),
            await health('nvme-retained'),
          ],
        });
      };
      await refresh(config());
      assert.equal(await health('nvme-wide'), 'healthy');
      assert.equal(await health('nvme-raised'), 'attention');
      assert.equal(await health('nvme-retained'), 'healthy');
      assert.match(await row('nvme-retained').innerText(), /95/);
      await capture('by-type');

      await refresh(
        config({
          truenasDiskDefaults: { temperature: { trigger: 62, clear: 57 } },
          overrides: { 'nvme-raised': { temperature: { trigger: 75, clear: 70 } } },
        }),
      );
      assert.equal(await health('nvme-wide'), 'attention');
      assert.equal(await health('nvme-raised'), 'healthy');
      assert.equal(await health('nvme-retained'), 'healthy');
      await page
        .getByRole('group', { name: 'Status', exact: true })
        .getByRole('button', { name: /Attention/ })
        .click();
      assert.equal(await page.locator('[data-truenas-storage-resource]').count(), 1);
      assert.equal(await row('nvme-wide').count(), 1);
      await page
        .getByRole('group', { name: 'Status', exact: true })
        .getByRole('button', { name: 'All, 3', exact: true })
        .click();
      const toggle = page.getByRole('button', { name: /Expand details for nvme-raised/i });
      await toggle.focus();
      await toggle.press('Enter');
      await page.getByTestId('resource-platform-details').locator('summary').click();
      await page.getByRole('button', { name: 'Show TrueNAS', exact: true }).click();
      const temperatureRow = page
        .getByTestId('resource-truenas-details-section')
        .getByRole('row')
        .filter({ has: page.getByText('Temperature', { exact: true }) });
      assert.match(await temperatureRow.innerText(), /72(?:\.0)?°C/);
      assert.match(
        await temperatureRow.locator('td').last().getAttribute('class'),
        /text-base-content/,
      );
      assert.doesNotMatch(
        await temperatureRow.locator('td').last().getAttribute('class'),
        /text-amber/,
      );
      assert.match(await page.locator('body').innerText(), /72(?:\.0)?°C/);
      await capture('raised-drawer');
      await page.getByRole('button', { name: /Collapse details for nvme-raised/i }).press('Enter');

      await refresh(
        config({
          truenasDiskDefaults: { temperature: { trigger: 0, clear: 0 } },
          overrides: { 'nvme-raised': { temperature: { trigger: 71, clear: 66 } } },
        }),
      );
      assert.equal(await health('nvme-wide'), 'healthy');
      assert.equal(await health('nvme-raised'), 'attention');
      await refresh(
        config({
          truenasDiskDefaults: { disabled: true },
          overrides: { 'nvme-raised': { temperature: { trigger: 71, clear: 66 } } },
        }),
      );
      assert.equal(await health('nvme-raised'), 'healthy');
      await capture('off');
      const guest = await page.evaluate(() => window.__thresholdFixture.guestControl());
      assert.deepEqual(guest, { warning: 80, critical: 92 });
      assert.deepEqual(errors, []);
      assert.deepEqual(consoleErrors, []);
      await browser.close();
      browser = null;
    }
    fs.writeFileSync(
      path.join(output, 'observations.json'),
      JSON.stringify({ result: 'passed', playwright: version, observations }, null, 2),
    );
    console.log(
      JSON.stringify({
        result: 'passed',
        playwright: version,
        captures: observations.length,
        observations,
      }),
    );
  } finally {
    if (browser) await browser.close();
    await server.close();
  }
})().catch((error) => {
  console.error(error);
  process.exitCode = 1;
});
