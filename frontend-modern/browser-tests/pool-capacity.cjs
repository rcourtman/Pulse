const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const { chromium, webkit } = require('playwright');
(async () => {
  const root = '/workspace/frontend-modern';
  process.chdir(root);
  const artifacts = path.join(root, 'node_modules/pool-capacity-proof');
  fs.mkdirSync(artifacts, { recursive: true });
  const { createServer } = await import(path.join(root, 'node_modules/vite/dist/node/index.js'));
  const server = await createServer({
    root,
    configFile: path.join(root, 'vite.config.ts'),
    cacheDir: fs.mkdtempSync(path.join(artifacts, 'vite-')),
    server: { host: '127.0.0.1', port: 5222, strictPort: true },
  });
  const observations = [];
  let browser;
  try {
    await server.listen();
    for (const scenario of [
      { engine: chromium, name: 'chromium', width: 1365, height: 900 },
      { engine: webkit, name: 'webkit', width: 390, height: 844 },
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
      const errors = [];
      page.on('pageerror', (e) => errors.push(e.message));
      await page.route('**/*', (route) => {
        const url = new URL(route.request().url());
        if (url.origin !== 'http://127.0.0.1:5222') return route.abort();
        if (!url.pathname.startsWith('/api/')) return route.continue();
        return route.fulfill({
          json:
            url.pathname === '/api/license/runtime-capabilities'
              ? {
                  capabilities: [],
                  limits: [],
                  max_history_days: 7,
                  hosted_mode: false,
                  runtime: { build: 'community' },
                  blocked_capabilities: [],
                }
              : { data: [], enabled: false },
        });
      });
      await page.goto('http://127.0.0.1:5222/browser-tests/pool-capacity.html');
      if (scenario.name === 'webkit')
        await page.evaluate(() => document.documentElement.classList.add('dark'));
      for (const [phase, expected] of Object.entries({
        missing: ['n/a', 'n/a', '1.00 KB', 'n/a'],
        empty: ['0 B', '1.00 KB', '1.00 KB', '0%'],
        full: ['1.00 KB', '0 B', '1.00 KB', '100%'],
        partial: ['512 B', '0 B', 'n/a', 'n/a'],
        absent: ['n/a', 'n/a', 'n/a', 'n/a'],
      })) {
        await page.getByRole('button', { name: phase, exact: true }).click();
        const actual = [];
        for (const label of ['Used', 'Free', 'Total', 'Usage'])
          actual.push(
            await page
              .getByText(label, { exact: true })
              .locator('..')
              .locator('span')
              .last()
              .textContent(),
          );
        const screenshot = path.join(artifacts, `${scenario.name}-${phase}.png`);
        await page.screenshot({ path: screenshot, fullPage: true });
        assert.deepEqual(actual, expected, `${scenario.name} ${phase}`);
        const dimensions = await page.evaluate(() => ({
          scroll: document.documentElement.scrollWidth,
          width: innerWidth,
        }));
        assert.ok(dimensions.scroll <= dimensions.width + 1, JSON.stringify(dimensions));
        observations.push({
          browser: scenario.name,
          version: browser.version(),
          phase,
          actual,
          dimensions,
          screenshot,
        });
      }
      assert.deepEqual(errors, []);
      await browser.close();
      browser = null;
    }
    fs.writeFileSync(
      path.join(artifacts, 'result.json'),
      JSON.stringify(
        { playwright: require('playwright/package.json').version, observations },
        null,
        2,
      ),
    );
    console.log(JSON.stringify({ result: 'passed', states: observations.length }));
  } finally {
    if (browser) await browser.close();
    await server.close();
  }
})().catch((error) => {
  console.error(error);
  process.exitCode = 1;
});
