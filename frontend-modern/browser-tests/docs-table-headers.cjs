// Production Docs/router/styles and shipped plan Markdown, no backend.
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const { chromium, webkit } = require('playwright');
(async () => {
  const observe = process.argv.includes('observe');
  const root = '/workspace/frontend-modern';
  process.chdir(root);
  const artifacts = path.join(root, 'node_modules', 'docs-header-browser');
  fs.mkdirSync(artifacts, { recursive: true });
  const { createServer } = await import(path.join(root, 'node_modules/vite/dist/node/index.js'));
  const server = await createServer({
    root,
    configFile: path.join(root, 'vite.config.ts'),
    cacheDir: path.join(root, 'node_modules', '.vite-docs-table-headers'),
    server: { host: '127.0.0.1', port: 5243, strictPort: true },
    optimizeDeps: { noDiscovery: true, include: [] },
  });
  let browser;
  const results = [];
  try {
    await server.listen();
    for (const [engine, width, tone] of [
      ['chromium', 1280, 'light'],
      ['webkit', 390, 'dark'],
    ]) {
      browser = await { chromium, webkit }[engine].launch(
        engine === 'chromium'
          ? { headless: true, channel: 'chromium', args: ['--no-sandbox'] }
          : { headless: true },
      );
      const page = await browser.newPage({
        viewport: { width, height: 900 },
        ...(engine === 'webkit' ? { isMobile: true, hasTouch: true } : {}),
      });
      const errors = [];
      page.on('response', (response) => {
        if (response.status() >= 400)
          console.log('FAILED_RESPONSE', response.status(), response.url());
      });
      page.on('pageerror', (e) => errors.push(e.message));
      page.on('console', (m) => {
        if (m.type() === 'error') {
          console.log('CONSOLE_ERROR', m.text(), m.location());
          errors.push(m.text());
        }
      });
      await page.goto(
        'http://127.0.0.1:5243/browser-tests/docs-fragment-navigation.html?scenario=plans',
        { waitUntil: 'domcontentloaded', timeout: 60_000 },
      );
      await page.locator('article table').first().waitFor();
      if (tone === 'dark')
        await page.evaluate(() => document.documentElement.classList.add('dark'));
      const table = page.locator('article table').filter({ hasText: 'Relay (legacy)' }).first();
      const th = await table.locator('thead th').allTextContents();
      const headers = await table.getByRole('columnheader').allTextContents();
      const snapshot = await table.ariaSnapshot();
      const scopes = await table
        .locator('thead th')
        .evaluateAll((nodes) => nodes.map((n) => n.getAttribute('scope')));
      const scroll = await table.evaluate((t) => ({
        wrapper: t.parentElement.dataset.docTableScroll !== undefined,
        outer: document.documentElement.scrollWidth,
        viewport: window.innerWidth,
        width: t.parentElement.clientWidth,
        content: t.parentElement.scrollWidth,
      }));
      if (!observe) {
        assert.equal(th.length, 8);
        assert.deepEqual(headers, th);
        assert.deepEqual(scopes, Array(th.length).fill('col'));
        assert.ok(snapshot.includes('columnheader "Relay (legacy)"'));
        assert.ok(scroll.wrapper);
        assert.ok(scroll.outer <= scroll.viewport + 1);
        if (width === 390) {
          assert.ok(scroll.content > scroll.width);
          const moved = await table.evaluate((t) => {
            t.parentElement.scrollLeft = 180;
            return t.parentElement.scrollLeft;
          });
          assert.ok(moved > 0);
        }
        assert.deepEqual(errors, []);
      }
      await table.locator('thead').scrollIntoViewIfNeeded();
      await page.screenshot({ path: path.join(artifacts, `${engine}-headers.png`) });
      await page.screenshot({
        path: path.join(artifacts, `${engine}-${observe ? 'before' : 'after'}.png`),
        fullPage: true,
      });
      results.push({
        engine,
        version: browser.version(),
        width,
        tone,
        th,
        headers,
        scopes,
        scroll,
        snapshot,
        errors,
      });
      await browser.close();
      browser = undefined;
    }
    fs.writeFileSync(
      path.join(artifacts, observe ? 'before.json' : 'after.json'),
      JSON.stringify(
        { playwright: require('playwright/package.json').version, observe, results },
        null,
        2,
      ),
    );
    console.log(JSON.stringify({ result: observe ? 'observed' : 'passed', results, artifacts }));
  } finally {
    if (browser) await browser.close();
    await server.close();
  }
})().catch((e) => {
  console.error(e);
  process.exitCode = 1;
});
