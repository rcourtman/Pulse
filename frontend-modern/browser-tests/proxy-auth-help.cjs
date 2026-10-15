// Exercise the shipped guide through the production Docs page and router.
// This checks readable/copyable help, not an IdP or a real proxy deployment.
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const { createHash } = require('node:crypto');
const { chromium, webkit } = require('playwright');

(async () => {
  const root = path.resolve('frontend-modern');
  const artifacts = path.join(root, 'node_modules', 'proxy-auth-browser');
  fs.mkdirSync(artifacts, { recursive: true });
  const guide = fs.readFileSync(path.join(root, 'public/docs/PROXY_AUTH.md'), 'utf8');
  const recipe = guide.match(/```bash\n([\s\S]*?)```/)[1].trim();
  const port = 5259;
  process.chdir(root);
  const { createServer } = await import(path.join(root, 'node_modules/vite/dist/node/index.js'));
  const server = await createServer({
    root, configFile: path.join(root, 'vite.config.ts'),
    cacheDir: path.join(root, 'node_modules', '.vite-proxy-auth'),
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
      const entry = page.locator('a[data-doc-link][href="/docs/PROXY_AUTH"]').first();
      await entry.focus();
      await page.keyboard.press('Enter');
      await page.getByRole('heading', { name: 'Verify Headers', exact: true }).waitFor();
      if (tone === 'dark') await page.evaluate(() => document.documentElement.classList.add('dark'));
      const code = page.locator('pre').filter({ hasText: 'probe_dir=$(mktemp -d)' });
      await code.scrollIntoViewIfNeeded();
      assert.equal((await code.locator('code').textContent()).trim(), recipe);
      assert.ok((await code.textContent()).includes("--proto '=http' --noproxy 127.0.0.1"));
      assert.ok(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1));
      const captures = [`${engine}-probe.png`, `${engine}-boundary.png`];
      await page.screenshot({ path: path.join(artifacts, captures[0]) });
      const warning = page.locator('p').filter({ hasText: 'explicitly bypasses proxy environment settings' });
      await warning.scrollIntoViewIfNeeded();
      const text = await page.locator('article').innerText();
      for (const phrase of ["not those environment settings", "mistaken for Pulse's role result",
        "exact backend hostname or IP", "Keep backend isolation intact",
        "not a successful denial test", "not your proxy's header replacement"]) {
        assert.ok(text.includes(phrase), phrase);
      }
      assert.ok(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1));
      await page.screenshot({ path: path.join(artifacts, captures[1]) });
      // Fragment interaction and reload must retain the real guide and exact
      // shell text, not a separately rendered copy of the example.
      await page.evaluate(() => { window.location.hash = '#verify-headers'; });
      await page.waitForFunction(() => document.activeElement?.id === 'verify-headers');
      await page.reload({ waitUntil: 'domcontentloaded' });
      await page.waitForFunction(() => document.activeElement?.id === 'verify-headers');
      assert.equal((await page.locator('pre').filter({ hasText: 'probe_dir=$(mktemp -d)' })
        .locator('code').textContent()).trim(), recipe);
      assert.deepEqual(errors, []);
      results.push({ engine, browserVersion: browser.version(), width, height: 1000, tone,
        keyboardEntry: true, exactCopiedShell: true, proxyAndTransportLimits: true,
        fragmentFocusAndReload: true, noPageOverflow: true, captures, errors });
      await browser.close();
      browser = undefined;
    }
    const result = { result: 'passed', verifiedAt: new Date().toISOString(),
      playwrightVersion: require('playwright/package.json').version,
      scope: 'Production Docs/router and copied help; not installed IdP/proxy acceptance',
      docs: { 'frontend-modern/public/docs/PROXY_AUTH.md':
        createHash('sha256').update(guide).digest('hex') }, results };
    fs.writeFileSync(path.join(artifacts, 'result.json'), JSON.stringify(result, null, 2) + '\n');
    console.log(JSON.stringify(result));
  } finally {
    if (browser) await browser.close();
    await server.close();
  }
})().catch(error => { console.error(error); process.exitCode = 1; });
