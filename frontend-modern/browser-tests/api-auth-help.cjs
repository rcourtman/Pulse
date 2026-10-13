// Consume the shipped API guide through the production Docs page/router.
// Shell transport is proved separately with real curl and synthetic fixtures.
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const { createHash } = require('node:crypto');
const { chromium, webkit } = require('playwright');

(async () => {
  const root = path.resolve('frontend-modern');
  const artifacts = path.join(root, 'node_modules', 'api-auth-browser');
  fs.mkdirSync(artifacts, { recursive: true });
  const guide = fs.readFileSync(path.join(root, 'public/docs/API.md'), 'utf8');
  const helper = [...guide.matchAll(/```bash\n([\s\S]*?)```/g)]
    .map(match => match[1].trim()).find(block => block.startsWith('pulse_api() ('));
  assert.ok(helper);
  const port = 5259;
  process.chdir(root);
  const { createServer } = await import(path.join(root, 'node_modules/vite/dist/node/index.js'));
  const server = await createServer({
    root, configFile: path.join(root, 'vite.config.ts'),
    cacheDir: path.join(root, 'node_modules', '.vite-api-auth'),
    server: { host: '127.0.0.1', port, strictPort: true },
    optimizeDeps: { noDiscovery: true, include: [] },
  });
  const results = [];
  let browser;
  try {
    await server.listen();
    for (const [engine, width, tone] of [['chromium', 1280, 'light'], ['webkit', 390, 'dark']]) {
      browser = await { chromium, webkit }[engine].launch(engine === 'chromium'
        ? { headless: true, channel: 'chromium', args: ['--no-sandbox'] }
        : { headless: true });
      const page = await browser.newPage({ viewport: { width, height: 1000 },
        ...(engine === 'webkit' ? { isMobile: true, hasTouch: true } : {}) });
      const errors = [];
      page.on('pageerror', error => errors.push(error.message));
      page.on('console', message => { if (message.type() === 'error') errors.push(message.text()); });
      await page.route(new RegExp(`^http://127\\.0\\.0\\.1:${port}/api/`), route =>
        route.fulfill({ status: 200, contentType: 'application/json', body: '{}' }));
      await page.goto(`http://127.0.0.1:${port}/browser-tests/docs-fragment-navigation.html`,
        { waitUntil: 'domcontentloaded', timeout: 60_000 });
      await page.getByRole('link', { name: '← All documentation' }).click();
      const entry = page.locator('a[data-doc-link][href="/docs/API"]').first();
      if (engine === 'webkit') await entry.tap();
      else {
        await entry.focus();
        await page.keyboard.press('Enter');
      }
      await page.getByRole('heading', { name: 'API Token (Recommended)', exact: true }).waitFor();
      if (tone === 'dark') await page.evaluate(() => document.documentElement.classList.add('dark'));
      const code = page.locator('pre').filter({ hasText: 'pulse_api() (' });
      assert.equal((await code.locator('code').textContent()).trim(), helper);
      assert.ok(helper.includes('--globoff'));
      // Place the changed command line in the viewport, and use the code
      // block's own horizontal scroll on phones rather than overflowing the page.
      await code.evaluate(pre => {
        const style = getComputedStyle(pre.querySelector('code'));
        const line = pre.textContent.split('\n').findIndex(text => text.includes('--globoff'));
        window.scrollTo(0, pre.getBoundingClientRect().top + window.scrollY
          + parseFloat(getComputedStyle(pre).paddingTop) + line * parseFloat(style.lineHeight) - 200);
        if (innerWidth < 500) pre.scrollLeft = pre.scrollWidth;
      });
      assert.ok(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1));
      const captures = [`${engine}-helper.png`, `${engine}-boundary.png`];
      await page.screenshot({ path: path.join(artifacts, captures[0]) });
      const warning = page.locator('p').filter({ hasText: "shell quotes alone do not disable curl's URL globbing" });
      await warning.scrollIntoViewIfNeeded();
      const text = await warning.innerText();
      for (const phrase of ['{review,ops}', '[1-3]', 'POST, PUT or DELETE',
        'not as a batch', 'URL-encode', 'does not encode them for you']) {
        assert.ok(text.includes(phrase), phrase);
      }
      assert.ok(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1));
      await page.screenshot({ path: path.join(artifacts, captures[1]) });
      // Return through the guide's own linked administration help, then use
      // the real renderer's fragment focus/reload handling on the API guide.
      const rbac = page.locator('a[data-doc-link][href^="/docs/RBAC#"]').first();
      await rbac.scrollIntoViewIfNeeded();
      if (engine === 'webkit') await rbac.tap();
      else { await rbac.focus(); await page.keyboard.press('Enter'); }
      await page.waitForURL(`**/docs/RBAC#managing-roles`);
      await page.waitForFunction(() => document.activeElement?.id === 'managing-roles');
      await page.goBack({ waitUntil: 'domcontentloaded' });
      await page.evaluate(() => { window.location.hash = '#api-token-recommended'; });
      await page.waitForFunction(() => document.activeElement?.id === 'api-token-recommended');
      await page.reload({ waitUntil: 'domcontentloaded' });
      await page.waitForFunction(() => document.activeElement?.id === 'api-token-recommended');
      assert.equal((await page.locator('pre').filter({ hasText: 'pulse_api() (' })
        .locator('code').textContent()).trim(), helper);
      assert.deepEqual(errors, []);
      results.push({ engine, browserVersion: browser.version(), width, height: 1000, tone,
        entry: engine === 'webkit' ? 'touch' : 'keyboard', exactRenderedShell: true,
        literalURLAndEncodingBoundary: true, linkedAdminHelp: true,
        fragmentFocusAndReload: true, noPageOverflow: true, captures, errors });
      await browser.close();
      browser = undefined;
    }
    const result = { result: 'passed', verifiedAt: new Date().toISOString(),
      playwrightVersion: require('playwright/package.json').version,
      scope: 'Production Docs/router and exact copied guide; not installed API or customer acceptance',
      docs: { 'frontend-modern/public/docs/API.md': createHash('sha256').update(guide).digest('hex') },
      results };
    fs.writeFileSync(path.join(artifacts, 'result.json'), JSON.stringify(result, null, 2) + '\n');
    console.log(JSON.stringify(result));
  } finally {
    if (browser) await browser.close();
    await server.close();
  }
})().catch(error => { console.error(error); process.exitCode = 1; });
