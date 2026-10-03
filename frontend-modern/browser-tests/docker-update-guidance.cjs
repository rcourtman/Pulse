// Production Docs viewer and shipped Docker guide; no container action is run.
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const { chromium, webkit } = require('playwright');

(async () => {
  const root = '/workspace/frontend-modern';
  process.chdir(root);
  const artifacts = path.join(root, 'node_modules', 'docker-update-guidance-browser');
  fs.mkdirSync(artifacts, { recursive: true });
  const { createServer } = await import(path.join(root, 'node_modules/vite/dist/node/index.js'));
  const server = await createServer({
    root,
    configFile: path.join(root, 'vite.config.ts'),
    cacheDir: path.join(root, 'node_modules', '.vite-docker-update-guidance'),
    server: { host: '127.0.0.1', port: 5250, strictPort: true },
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
        viewport: { width, height: 1000 },
        ...(engine === 'webkit' ? { isMobile: true, hasTouch: true } : {}),
      });
      const errors = [];
      page.on('pageerror', (error) => errors.push(error.message));
      page.on('console', (message) => {
        if (message.type() === 'error') errors.push(message.text());
      });
      await page.goto('http://127.0.0.1:5250/browser-tests/docs-fragment-navigation.html', {
        waitUntil: 'domcontentloaded',
        timeout: 60000,
      });
      await page.evaluate(() => {
        history.pushState({}, '', '/docs/DOCKER#before-updating-a-workload');
        dispatchEvent(new PopStateEvent('popstate'));
      });
      if (dark) await page.evaluate(() => document.documentElement.classList.add('dark'));
      const waitForHeading = async (id) => {
        await page.waitForFunction((target) => {
          const heading = document.getElementById(target);
          if (!heading) return false;
          const rect = heading.getBoundingClientRect();
          return document.activeElement === heading && rect.top >= 0 && rect.top < innerHeight;
        }, id);
        assert.ok(
          await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1),
          'page must not overflow the viewport',
        );
      };
      await waitForHeading('before-updating-a-workload');
      const article = await page.locator('article').innerText();
      for (const text of [
        'independent, consistent backup',
        'not a backup of mounted data',
        'five minutes after a successful update',
        'Removal, rename or restart can fail too',
        'check the current state before retrying',
        'not a monitoring-only security boundary',
      ])
        assert.ok(article.replace(/\s+/g, ' ').includes(text), text);
      await page.screenshot({ path: path.join(artifacts, `${engine}-precautions.png`) });
      await page.evaluate(() => {
        history.pushState({}, '', '/docs/DOCKER#safety-features');
        dispatchEvent(new PopStateEvent('popstate'));
      });
      await waitForHeading('safety-features');
      await page.screenshot({ path: path.join(artifacts, `${engine}-limits.png`) });
      const link = page.getByRole('link', {
        name: 'container lifecycle requirements',
        exact: true,
      });
      const target = decodeURIComponent((await link.getAttribute('href')).split('#')[1]);
      await link.focus();
      await page.keyboard.press('Enter');
      await waitForHeading(target);
      assert.equal(
        await page
          .getByRole('heading', { name: '▶️ Container Lifecycle Actions', exact: true })
          .textContent(),
        '▶️ Container Lifecycle Actions',
      );
      assert.deepEqual(errors, []);
      results.push({
        engine,
        version: browser.version(),
        width,
        dark,
        precautionsAndLimitsRendered: true,
        keyboardRequirementsLink: true,
        headingsFocusedAndVisible: true,
        noHorizontalOverflow: true,
        errors,
      });
      await browser.close();
      browser = undefined;
    }
    const result = {
      result: 'passed',
      playwright: require('playwright/package.json').version,
      scope: 'shipped help rendering and keyboard navigation, not native updates or rollback',
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
