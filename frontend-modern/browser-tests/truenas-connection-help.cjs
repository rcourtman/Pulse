// The real Docs page/router/styles and shipped TrueNAS help, without a backend.
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const { chromium, webkit } = require('playwright');

(async () => {
  const root = '/workspace/frontend-modern';
  const artifacts = path.join(root, 'node_modules', 'truenas-connection-help-browser');
  fs.mkdirSync(artifacts, { recursive: true });
  const { createServer } = await import(path.join(root, 'node_modules/vite/dist/node/index.js'));
  const server = await createServer({
    root,
    configFile: path.join(root, 'vite.config.ts'),
    cacheDir: path.join(root, 'node_modules', '.vite-truenas-connection-help'),
    server: { host: '127.0.0.1', port: 5250, strictPort: true },
    optimizeDeps: { noDiscovery: true, include: [] },
  });
  const results = [];
  let browser;
  try {
    await server.listen();
    for (const [engine, width] of [['chromium', 1280], ['webkit', 390]]) {
      browser = await { chromium, webkit }[engine].launch(engine === 'chromium'
        ? { headless: true, channel: 'chromium', args: ['--no-sandbox'] }
        : { headless: true });
      const page = await browser.newPage({ viewport: { width, height: 900 },
        ...(engine === 'webkit' ? { isMobile: true, hasTouch: true } : {}) });
      const errors = [];
      page.on('pageerror', (error) => errors.push(error.message));
      page.on('console', (message) => {
        if (message.type() === 'error') errors.push(message.text());
      });
      await page.goto('http://127.0.0.1:5250/browser-tests/docs-fragment-navigation.html?scenario=truenas',
        { waitUntil: 'domcontentloaded', timeout: 60_000 });
      const quickStart = page.getByRole('heading', { name: 'Quick Start', exact: true });
      await quickStart.waitFor();
      if (engine === 'webkit') {
        await page.evaluate(async () => {
          document.documentElement.classList.add('dark');
          await new Promise((resolve) => requestAnimationFrame(() => requestAnimationFrame(resolve)));
          await Promise.all(document.getAnimations().map((animation) => animation.finished.catch(() => {})));
        });
      }
      const article = await page.locator('article').innerText();
      for (const text of ['it does not validate inventory or metric collection',
        'an elapsed interval is not proof that a poll completed',
        'Some older builds, including Pulse 6.4.1 and 6.4.5',
        'use the inventory observation time rather than the test time',
        'do not copy a token or cookie', 'Do not post the full connection response'])
        assert.ok(article.includes(text), text);
      assert.ok(!article.includes('Data appears within one configured polling cycle'));
      await quickStart.evaluate((heading) => heading.scrollIntoView({ block: 'start' }));
      await page.screenshot({ path: path.join(artifacts, `${engine}-setup.png`) });
      const checks = page.getByRole('link', { name: 'polling checks', exact: true }).first();
      await checks.focus();
      await page.keyboard.press('Enter');
      await page.waitForFunction(() => {
        const heading = document.getElementById('stale-truenas-data');
        return heading && heading.getBoundingClientRect().top >= 0
          && heading.getBoundingClientRect().top < innerHeight;
      });
      assert.ok(page.url().endsWith('/docs/TRUENAS#stale-truenas-data'));
      assert.ok(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1));
      await page.screenshot({ path: path.join(artifacts, `${engine}-polling.png`) });
      assert.deepEqual(errors, []);
      results.push({ engine, version: browser.version(), width,
        probeScope: true, noGuaranteedPoll: true, versionQualifiedWarning: true,
        keyboardPollingLink: true, noDocumentOverflow: true, errors });
      await browser.close();
      browser = undefined;
    }
    const result = { result: 'passed', playwright: require('playwright/package.json').version,
      scope: 'shipped help rendering/navigation, not native TrueNAS polling', results };
    fs.writeFileSync(path.join(artifacts, 'result.json'), JSON.stringify(result, null, 2) + '\n');
    console.log(JSON.stringify(result));
  } finally {
    if (browser) await browser.close();
    await server.close();
  }
})().catch((error) => { console.error(error); process.exitCode = 1; });
