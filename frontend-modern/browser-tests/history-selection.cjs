const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const { chromium, webkit } = require('playwright');
(async () => {
  const root = '/workspace/frontend-modern';
  process.chdir(root);
  const artifacts = path.join(root, 'node_modules/history-selection-proof');
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
      await page.goto('http://127.0.0.1:5222/browser-tests/history-selection.html');
      if (scenario.name === 'webkit')
        await page.evaluate(() => document.documentElement.classList.add('dark'));
      const chart = page.getByRole('img', { name: 'CPU usage chart' });
      await chart.waitFor();
      const description = page.locator('#' + (await chart.getAttribute('aria-describedby')));
      await page.getByRole('button', { name: 'Select b', exact: true }).click();
      await page.getByRole('button', { name: 'Finish b', exact: true }).click();
      for (const [phase, button, expected] of [
        ['b-loaded', null, '80.0%'],
        ['old-a-finished', 'Finish old a', '80.0%'],
        ['c-loading', 'Select c', 'Loading'],
        ['c-failed', 'Fail c', 'could not be loaded'],
      ]) {
        if (button) await page.getByRole('button', { name: button, exact: true }).click();
        await page.waitForTimeout(100);
        const actual = await description.textContent();
        const screenshot = path.join(artifacts, `${scenario.name}-${phase}.png`);
        await page.screenshot({ path: screenshot, fullPage: true });
        assert.ok(actual.includes(expected), `${scenario.name} ${phase}: ${actual}`);
        if (phase.startsWith('c-')) assert.ok(!actual.includes('80.0%'));
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
