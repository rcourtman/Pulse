// Shipped Recovery -> Troubleshooting navigation, with the production Docs page.
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const { chromium, webkit } = require('playwright');

(async () => {
  const root = '/workspace/frontend-modern';
  process.chdir(root);
  const artifacts = path.join(root, 'node_modules', 'recovery-safe-reporting-browser');
  fs.mkdirSync(artifacts, { recursive: true });
  const { createServer } = await import(path.join(root, 'node_modules/vite/dist/node/index.js'));
  const server = await createServer({
    root,
    configFile: path.join(root, 'vite.config.ts'),
    cacheDir: path.join(root, 'node_modules', '.vite-recovery-safe-reporting'),
    server: { host: '127.0.0.1', port: 5249, strictPort: true },
    optimizeDeps: { noDiscovery: true, include: [] },
  });
  let browser;
  const results = [];
  try {
    await server.listen();
    for (const [engine, width, dark] of [
      ['chromium', 1280, false],
      ['webkit', 390, true],
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
      page.on('pageerror', (error) => errors.push(error.message));
      page.on('console', (message) => {
        if (message.type() === 'error') errors.push(message.text());
      });
      await page.goto('http://127.0.0.1:5249/browser-tests/docs-fragment-navigation.html', {
        waitUntil: 'domcontentloaded',
        timeout: 60000,
      });
      // The fixture mounts the actual /docs router. Navigate without mocking assets.
      await page.evaluate(() => {
        history.pushState({}, '', '/docs/RECOVERY');
        dispatchEvent(new PopStateEvent('popstate'));
      });
      const link = page.getByRole('link', { name: 'safe issue reporting', exact: true });
      await link.waitFor();
      if (dark) await page.evaluate(() => document.documentElement.classList.add('dark'));
      await link.focus();
      await page.keyboard.press('Enter');
      const verifyDestination = async () => {
        await page.waitForFunction(() => {
          const heading = document.getElementById('-getting-help');
          if (!heading) return false;
          const rect = heading.getBoundingClientRect();
          return document.activeElement === heading && rect.top >= 0 && rect.top < innerHeight;
        });
        assert.ok(page.url().endsWith('/docs/TROUBLESHOOTING#-getting-help'));
        assert.equal(await page.locator('h2#-getting-help').textContent(), '🆘 Getting Help');
        const article = await page.locator('article').innerText();
        for (const text of [
          'Export for GitHub (sanitized)',
          'Review before posting',
          'Never post bootstrap/recovery',
          'Do not refile information you have already supplied',
        ])
          assert.ok(article.includes(text), text);
        assert.ok(
          await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1),
        );
      };
      await verifyDestination();
      await page.screenshot({ path: path.join(artifacts, `${engine}-safe-reporting.png`) });
      assert.deepEqual(errors, []);
      results.push({
        engine,
        version: browser.version(),
        width,
        dark,
        keyboardNavigation: true,
        targetFocusedAndVisible: true,
        safeGuidance: true,
        noHorizontalOverflow: true,
        errors,
      });
      await browser.close();
      browser = undefined;
    }
    const result = {
      result: 'passed',
      playwright: require('playwright/package.json').version,
      scope: 'shipped help navigation, not native recovery or release availability',
      results,
    };
    fs.writeFileSync(path.join(artifacts, 'result.json'), JSON.stringify(result, null, 2) + '\n');
    console.log(JSON.stringify(result));
  } finally {
    if (browser) await browser.close();
    await server.close();
  }
})().catch((error) => {
  console.error(error);
  process.exitCode = 1;
});
