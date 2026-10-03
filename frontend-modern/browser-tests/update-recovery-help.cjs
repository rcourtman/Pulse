// Actual shipped Markdown in the production Docs/router/styles; no backend.
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const { chromium, webkit } = require('playwright');

(async () => {
  const root = '/workspace/frontend-modern';
  process.chdir(root);
  const artifacts = path.join(root, 'node_modules', 'update-recovery-help-browser');
  fs.mkdirSync(artifacts, { recursive: true });
  const { createServer } = await import(path.join(root, 'node_modules/vite/dist/node/index.js'));
  const server = await createServer({
    root,
    configFile: path.join(root, 'vite.config.ts'),
    cacheDir: path.join(root, 'node_modules', '.vite-update-recovery-help'),
    server: { host: '127.0.0.1', port: 5248, strictPort: true },
    optimizeDeps: { noDiscovery: true, include: [] },
  });
  const results = [];
  let browser;
  try {
    await server.listen();
    for (const [engine, width, tone] of [['chromium', 1280, 'light'], ['webkit', 390, 'dark']]) {
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
      page.on('pageerror', (e) => errors.push(e.message));
      page.on('console', (m) => { if (m.type() === 'error') errors.push(m.text()); });
      await page.goto('http://127.0.0.1:5248/browser-tests/docs-fragment-navigation.html',
        { waitUntil: 'domcontentloaded', timeout: 60_000 });
      await page.getByRole('link', { name: '← All documentation' }).click();
      await page.getByRole('link', { name: 'Automatic updates', exact: true }).click();
      await page.getByRole('heading', { name: 'Rollback', exact: true }).waitFor();
      if (tone === 'dark') await page.evaluate(() => document.documentElement.classList.add('dark'));
      const article = await page.locator('article').innerText();
      for (const text of ['Roll back from Update History', 'successful in-app update',
        'not necessarily a complete backup', 'Copy errors can leave a partial snapshot',
        'matching .encryption.key', 'SQLite sidecar files', 'keep reversible copies',
        'An image change alone is not a data rollback']) assert.ok(article.includes(text), text);
      assert.ok(!article.includes('There is no rollback UI'));
      assert.ok(!article.includes('sudo rm -rf'));
      const scope = page.getByRole('heading', { name: 'What an update snapshot contains', exact: true });
      await scope.scrollIntoViewIfNeeded();
      assert.ok(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1));
      await page.screenshot({ path: path.join(artifacts, `${engine}-scope.png`) });
      const manual = page.getByRole('heading', { name: 'Manual Rollback', exact: true });
      await manual.scrollIntoViewIfNeeded();
      await page.screenshot({ path: path.join(artifacts, `${engine}-manual.png`) });

      await page.getByRole('link', { name: 'audit storage and safe recovery', exact: true }).click();
      await page.waitForFunction(() => {
        const heading = document.getElementById('storage');
        return heading && heading.getBoundingClientRect().top >= 0 && heading.getBoundingClientRect().top < innerHeight;
      });
      assert.ok(page.url().endsWith('/docs/AUDIT_LOGGING#storage'));
      await page.getByRole('link', { name: '← All documentation' }).click();
      await page.getByRole('link', { name: 'Automatic updates', exact: true }).click();
      await page.getByRole('link', { name: 'Docker server updates', exact: true }).click();
      const updates = page.getByRole('heading', { name: '🔄 Updates', exact: true });
      await updates.waitFor();
      await page.waitForFunction(() => {
        const heading = document.getElementById('-updates');
        return heading && heading.getBoundingClientRect().top >= 0 && heading.getBoundingClientRect().top < innerHeight;
      });
      assert.ok(page.url().endsWith('/docs/DOCKER#-updates'));
      const dockerText = await page.locator('article').innerText();
      for (const text of ['persist PULSE_IMAGE', 'has no effect on a hardcoded image line',
        'docker compose pull pulse', 'docker compose up -d --no-deps pulse',
        'paid Pro installs must keep the private image', 'no need to bring the whole Compose project down'])
        assert.ok(dockerText.includes(text), text);
      assert.ok(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1));
      await page.screenshot({ path: path.join(artifacts, `${engine}-docker.png`) });
      await page.getByRole('link', { name: 'rollback and backup scope', exact: true }).click();
      await page.getByRole('heading', { name: 'Rollback', exact: true }).waitFor();
      assert.ok(page.url().endsWith('/docs/AUTO_UPDATE#rollback'));
      assert.deepEqual(errors, []);
      results.push({ engine, version: browser.version(), width, tone,
        snapshotScope: true, noDestructiveRecipe: true, scopedCompose: true,
        auditLink: true, dockerFragment: true, rollbackLink: true, errors });
      await browser.close();
      browser = undefined;
    }
    const result = { result: 'passed', playwright: require('playwright/package.json').version,
      scope: 'shipped help rendering and links, not installation or recovery', results };
    fs.writeFileSync(path.join(artifacts, 'result.json'), JSON.stringify(result, null, 2) + '\n');
    console.log(JSON.stringify(result));
  } finally {
    if (browser) await browser.close();
    await server.close();
  }
})().catch((error) => { console.error(error); process.exitCode = 1; });
