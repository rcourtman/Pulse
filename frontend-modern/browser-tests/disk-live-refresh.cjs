const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const { createHash } = require('node:crypto');
const { chromium, webkit } = require('playwright');

(async () => {
  const root = '/workspace/frontend-modern';
  process.chdir(root);
  const artifacts = path.join(root, 'node_modules/disk-live-refresh-proof');
  fs.mkdirSync(artifacts, { recursive: true });
  const { createServer } = await import(path.join(root, 'node_modules/vite/dist/node/index.js'));
  const server = await createServer({
    root,
    configFile: path.join(root, 'vite.config.ts'),
    cacheDir: fs.mkdtempSync(path.join(artifacts, 'vite-cache-')),
    server: { host: '127.0.0.1', port: 5221, strictPort: true },
  });
  const observations = [];
  let browser;
  try {
    await server.listen();
    for (const scenario of [
      { engine: chromium, name: 'chromium', width: 1365, height: 900, dark: false },
      { engine: webkit, name: 'webkit', width: 390, height: 844, dark: true },
    ]) {
      browser = await scenario.engine.launch(
        scenario.name === 'chromium'
          ? { headless: true, channel: 'chromium', args: ['--no-sandbox'] }
          : { headless: true },
      );
      const page = await browser.newPage({
        viewport: { width: scenario.width, height: scenario.height },
        isMobile: scenario.name === 'webkit',
        hasTouch: scenario.name === 'webkit',
      });
      const errors = [],
        requests = [];
      page.on('pageerror', (error) => errors.push(error.message));
      await page.route('**/*', async (route) => {
        const url = new URL(route.request().url());
        if (url.origin !== 'http://127.0.0.1:5221') return route.abort();
        if (!url.pathname.startsWith('/api/')) return route.continue();
        requests.push(url.pathname);
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
      await page.goto('http://127.0.0.1:5221/browser-tests/disk-live-refresh.html');
      await page.evaluate(
        (dark) => document.documentElement.classList.toggle('dark', dark),
        scenario.dark,
      );
      const fixture = page.getByTestId('disk-live-refresh-fixture');
      const row = fixture.locator('[data-row-id="disk-one"]');
      await row.getByText('Healthy', { exact: true }).waitFor();
      await row.getByText('Archive SSD', { exact: true }).click();
      const disclosure = row.getByRole('button', { name: 'Collapse Archive SSD', exact: true });
      await disclosure.focus();
      await row.evaluate((element) => {
        element.dataset.proofOwner = 'retained';
      });
      const initialControls = await disclosure.getAttribute('aria-controls');
      const snapshot = (phase) =>
        page.locator(`button[data-update="${phase}"]`).evaluate((button) => button.click());
      const checkContinuity = async () => {
        assert.equal(await row.getAttribute('data-proof-owner'), 'retained');
        assert.equal(
          await row.locator('button').evaluate((button) => document.activeElement === button),
          true,
        );
        assert.equal(await row.locator('button').getAttribute('aria-expanded'), 'true');
        const controls = await row.locator('button').getAttribute('aria-controls');
        assert.equal(await page.locator(`[id="${controls}"]`).count(), 1);
        assert.equal(await fixture.locator('[data-inline-detail-for]').count(), 1);
        const dimensions = await page.evaluate(() => ({
          scroll: document.documentElement.scrollWidth,
          width: innerWidth,
        }));
        assert.ok(dimensions.scroll <= dimensions.width + 1, JSON.stringify(dimensions));
        return dimensions;
      };
      const screenshot = (state) =>
        fixture.screenshot({ path: path.join(artifacts, `${scenario.name}-${state}.png`) });

      await snapshot('fault');
      await row.getByText('Replace Now', { exact: true }).waitFor();
      for (const text of ['Archive SSD (fault)', '4%', '63°C'])
        assert.equal(await row.getByText(text, { exact: true }).count(), 1);
      assert.equal(await row.getByText('Healthy', { exact: true }).count(), 0);
      assert.equal(await row.getAttribute('data-summary-series-id'), 'agent-archive:sdz');
      assert.notEqual(await row.locator('button').getAttribute('aria-controls'), initialControls);
      await fixture.getByText('2', { exact: true }).waitFor();
      const faultDimensions = await checkContinuity();
      await screenshot('fault');

      await snapshot('missing');
      await row.getByText('Unknown', { exact: true }).waitFor();
      for (const text of ['Replace Now', 'SMART failed.', '4%', '63°C'])
        assert.equal(await row.getByText(text, { exact: true }).count(), 0);
      for (const column of ['temp', 'life', 'size', 'role', 'parent'])
        assert.equal(
          (await row.locator(`td[data-storage-column="${column}"]`).innerText()).trim(),
          '—',
        );
      await fixture
        .getByText('Temperature is temporarily unavailable: No current reading', { exact: true })
        .waitFor();
      const missingDimensions = await checkContinuity();
      await screenshot('missing');

      await snapshot('healthy');
      await row.getByText('Healthy', { exact: true }).waitFor();
      assert.equal(await row.getByText('96%', { exact: true }).count(), 1);
      assert.equal(await row.getByText('41°C', { exact: true }).count(), 1);
      assert.equal(await row.getAttribute('data-summary-series-id'), 'agent-archive:sda');
      assert.equal(await row.locator('button').getAttribute('aria-controls'), initialControls);
      const recoveredDimensions = await checkContinuity();
      await screenshot('recovered');

      await snapshot('fault');
      await row.getByText('Replace Now', { exact: true }).waitFor();
      await page.locator('[data-filter="attention"]').evaluate((button) => button.click());
      assert.equal(await row.getByText('Replace Now', { exact: true }).count(), 1);
      await snapshot('healthy');
      await fixture.getByText('No disks need attention', { exact: true }).waitFor();
      await page.locator('[data-filter="all"]').evaluate((button) => button.click());
      await row.getByText('Healthy', { exact: true }).waitFor();
      assert.deepEqual(errors, []);
      observations.push({
        browser: scenario.name,
        version: browser.version(),
        viewport: { width: scenario.width, height: scenario.height },
        dark: scenario.dark,
        faultDimensions,
        missingDimensions,
        recoveredDimensions,
        errors,
        requests,
      });
      await browser.close();
      browser = undefined;
    }
    const source = 'src/components/Storage/DiskList.tsx';
    const result = {
      result: 'passed',
      playwright: require('playwright/package.json').version,
      content_sha256: {
        [`frontend-modern/${source}`]: createHash('sha256')
          .update(fs.readFileSync(path.join(root, source)))
          .digest('hex'),
      },
      observations,
    };
    fs.writeFileSync(path.join(artifacts, 'result.json'), JSON.stringify(result, null, 2) + '\n');
    console.log(JSON.stringify(result));
  } finally {
    if (browser) await browser.close();
    await server.close();
    for (const name of fs.readdirSync(artifacts)) {
      const file = path.join(artifacts, name);
      if (fs.statSync(file).isFile()) fs.chmodSync(file, 0o644);
    }
  }
})().catch((error) => {
  console.error(error);
  process.exitCode = 1;
});
