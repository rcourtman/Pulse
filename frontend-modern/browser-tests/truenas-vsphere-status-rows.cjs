const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const { chromium, webkit } = require('playwright');

// Each TrueNAS and vSphere row's status words read in its dot's colour: a
// crashed app says "Crashed" under its name (it has no State column), a
// disabled share's State reads red and a locked one amber, a failed vSphere
// task reads red; phones, which hide those State columns, read the words
// under the name.
const now = '2026-10-03T12:00:00Z';

const rgb = (colour) => colour.match(/\d+/g).map(Number);
// Red in light mode is deep, in dark mode a light pink: red leads either way.
const isRed = (colour) => {
  const [r, g, b] = rgb(colour);
  return r > 150 && r - g > 60 && r - b > 60;
};
const isAmber = (colour) => {
  const [r, g, b] = rgb(colour);
  return r > 150 && g > 70 && b < 80;
};

(async () => {
  const root = '/workspace/frontend-modern';
  const output = '/workspace/tmp/truenas-vsphere-status-rows';
  fs.mkdirSync(output, { recursive: true });
  process.chdir(root);
  const { createServer } = await import(path.join(root, 'node_modules/vite/dist/node/index.js'));
  const server = await createServer({
    root,
    configFile: path.join(root, 'vite.config.ts'),
    cacheDir: path.join(output, 'vite-cache'),
    server: { host: '127.0.0.1', port: 5274, strictPort: true },
  });
  const results = [],
    screenshots = [];
  let browser;
  try {
    await server.listen();
    for (const [name, engine, width] of [
      ['desktop', chromium, 1440],
      ['phone', webkit, 390],
    ]) {
      browser = await engine.launch(
        engine === chromium
          ? { headless: true, channel: 'chromium', args: ['--no-sandbox'] }
          : { headless: true },
      );
      const page = await browser.newPage({
        viewport: { width, height: width === 390 ? 844 : 900 },
        isMobile: width === 390,
        hasTouch: width === 390,
        locale: 'en-GB',
        timezoneId: 'UTC',
      });
      await page.clock.setFixedTime(new Date(now));
      page.setDefaultTimeout(15000);
      const checks = [],
        errors = [];
      page.on('pageerror', (e) => errors.push(e.message));
      await page.route('**/*', async (route) => {
        const request = route.request(),
          url = new URL(request.url());
        if (url.origin !== 'http://127.0.0.1:5274') return route.abort();
        if (!url.pathname.startsWith('/api/')) return route.continue();
        return route.fulfill({
          json: {
            enabled: false,
            data: [],
            capabilities: [],
            facets: [],
            relationships: [],
            recentChanges: [],
            alerts: {},
          },
        });
      });
      const check = async (label, run) => {
        try {
          await run();
          checks.push({ label, passed: true });
        } catch (e) {
          checks.push({ label, passed: false, error: e.message });
        }
      };
      const colour = (locator) => locator.evaluate((el) => getComputedStyle(el).color);
      const dotColour = (locator) => locator.evaluate((el) => getComputedStyle(el).backgroundColor);
      // A fresh Vite cache compiles the app on the first load.
      await page.goto('http://127.0.0.1:5274/browser-tests/truenas-vsphere-status-rows.html', {
        timeout: 180000,
      });
      await page.getByText('fixture ready', { exact: true }).waitFor({ timeout: 60000 });
      if (width === 390) await page.evaluate(() => document.documentElement.classList.add('dark'));
      const plain = (c) => !isRed(c) && !isAmber(c);

      await check('A TrueNAS app reads its state under its name', async () => {
        const table = page.getByRole('region', { name: 'App table' });
        const crashed = table.locator('[data-truenas-app-state="warning"]');
        await crashed.waitFor();
        assert.equal((await crashed.textContent()).trim(), 'Crashed');
        assert.ok(isAmber(await colour(crashed)), 'crashed app amber');
        const running = table.locator('[data-truenas-app-state="success"]');
        assert.equal((await running.textContent()).trim(), 'Running');
        assert.ok(plain(await colour(running)), 'running app plain');
      });
      await check('A TrueNAS share state reads in its dot colour', async () => {
        const table = page.getByRole('region', { name: 'Share table' });
        const attr = width === 390 ? 'data-truenas-share-state-phone' : 'data-truenas-share-state';
        const disabled = table.locator(`[${attr}="danger"]`);
        await disabled.waitFor();
        assert.equal((await disabled.textContent()).trim(), 'Disabled');
        assert.ok(isRed(await colour(disabled)), 'disabled share red');
        const locked = table.locator(`[${attr}="warning"]`);
        assert.equal((await locked.textContent()).trim(), 'Attention');
        assert.ok(isAmber(await colour(locked)), 'locked share amber');
      });
      await check('A stopped TrueNAS service that should run says so', async () => {
        const table = page.getByRole('region', { name: 'Service table' });
        const note =
          width === 390
            ? table.locator('[data-truenas-service-state-phone="danger"]')
            : table.locator('[data-truenas-service-state-note="danger"]');
        await note.waitFor();
        assert.match((await note.textContent()).replace(/\s+/g, ' '), /should be running/);
        const words = width === 390 ? note : note.getByText('should be running');
        assert.ok(isRed(await colour(words)), 'should-be-running note red');
      });
      await check('A failed vSphere task reads red', async () => {
        const table = page.getByRole('region', { name: 'Activity table' });
        const attr =
          width === 390 ? 'data-vsphere-activity-state-phone' : 'data-vsphere-activity-state';
        const failed = table.locator(`[${attr}="danger"]`);
        await failed.waitFor();
        assert.equal((await failed.textContent()).trim(), 'Error');
        assert.ok(isRed(await colour(failed)), 'failed task red');
        if (width !== 390) {
          assert.equal(
            await table.locator('[data-vsphere-activity-state-phone]').isVisible(),
            false,
          );
        }
      });
      const dimensions = await page.evaluate(() => ({
        width: innerWidth,
        scroll: document.documentElement.scrollWidth,
      }));
      checks.push({
        label: 'No horizontal page overflow',
        passed: dimensions.scroll <= dimensions.width + 1,
        dimensions,
      });
      checks.push({ label: 'No page errors', passed: errors.length === 0, errors });
      const file = `${name}-truenas-vsphere.png`;
      await page.screenshot({ path: path.join(output, file), fullPage: true });
      screenshots.push({ file, engine: name, width });
      results.push({ engine: name, width, checks });
      await browser.close();
      browser = undefined;
    }
  } finally {
    if (browser) await browser.close();
    await server.close();
  }
  const failures = results.flatMap((r) =>
    r.checks.filter((c) => !c.passed).map((c) => ({ engine: r.engine, ...c })),
  );
  fs.writeFileSync(
    path.join(output, 'result.json'),
    JSON.stringify({ verified_at: new Date().toISOString(), results, screenshots }, null, 2),
  );
  console.log(
    JSON.stringify({
      checked: results.reduce((n, r) => n + r.checks.length, 0),
      screenshots: screenshots.length,
      failures,
    }),
  );
  assert.equal(failures.length, 0, JSON.stringify(failures));
})().catch((error) => {
  console.error(error);
  process.exitCode = 1;
});
