// Render shipped authentication help through the real Docs/router/Markdown/CSS.
// No credential, proxy, API write or installation is exercised in the browser.
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const { createHash } = require('node:crypto');
const { chromium, webkit } = require('playwright');

(async () => {
  const root = path.resolve('frontend-modern');
  const artifacts = path.join(root, 'node_modules', 'authentication-diagnostic-browser');
  fs.mkdirSync(artifacts, { recursive: true });
  process.chdir(root);
  const { createServer } = await import(path.join(root, 'node_modules/vite/dist/node/index.js'));
  const server = await createServer({
    root, configFile: path.join(root, 'vite.config.ts'),
    cacheDir: path.join(root, 'node_modules', '.vite-authentication-diagnostic'),
    server: { host: '127.0.0.1', port: 5253, strictPort: true },
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
      page.on('pageerror', (error) => errors.push(error.message));
      page.on('console', (message) => { if (message.type() === 'error') errors.push(message.text()); });
      await page.goto('http://127.0.0.1:5253/browser-tests/docs-fragment-navigation.html',
        { waitUntil: 'domcontentloaded', timeout: 60_000 });
      if (tone === 'dark') await page.evaluate(() => document.documentElement.classList.add('dark'));
      for (const guide of ['AI_AUTONOMY', 'PROXY_AUTH']) {
        await page.getByRole('link', { name: '← All documentation' }).click();
        const entry = page.locator(`a[data-doc-link][href="/docs/${guide}"]`).first();
        await entry.focus();
        await page.keyboard.press('Enter');
        const heading = page.getByRole('heading', { name: guide === 'AI_AUTONOMY' ? 'Configuration' : 'Verify Headers', exact: true }).first();
        await heading.waitFor();
        await heading.scrollIntoViewIfNeeded();
        const text = await page.locator('article').innerText();
        assert.ok(!text.includes('admin:admin') && !text.includes('curl -H "X-Proxy-Secret'));
        const phrases = guide === 'AI_AUTONOMY'
          ? ['settings:write', 'even for GET', 'changes Patrol mode', 'after an uncertain write']
          : ['not expose the backend port', 'HTTP 000', 'old behaviour'];
        for (const phrase of phrases)
          assert.ok(text.includes(phrase), phrase);
        assert.ok(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1));
        if (guide === 'AI_AUTONOMY') {
          await page.screenshot({ path: path.join(artifacts, `${engine}-${guide.toLowerCase()}.png`) });
          const link = page.getByRole('link', { name: 'private header file', exact: true });
          await link.focus();
          await page.keyboard.press('Enter');
          await page.waitForFunction(() => {
            const h = document.getElementById('api-token-recommended');
            return h && h.getBoundingClientRect().top >= 0 && h.getBoundingClientRect().top < innerHeight;
          });
          assert.ok(page.url().endsWith('/docs/API#api-token-recommended'));
          const code = page.locator('pre').filter({ hasText: 'Refusing a symlinked credential path.' });
          await code.scrollIntoViewIfNeeded();
          assert.ok((await code.innerText()).includes('chmod 700 "$auth_dir"'));
          await page.screenshot({ path: path.join(artifacts, `${engine}-private-header.png`) });
        } else {
          const code = page.locator('pre').filter({ hasText: "--write-out 'HTTP %{http_code}" });
          await code.scrollIntoViewIfNeeded();
          assert.ok((await code.innerText()).includes('--output /dev/null'));
          assert.ok((await code.innerText()).includes('--header "@$header_file"'));
          await page.screenshot({ path: path.join(artifacts, `${engine}-${guide.toLowerCase()}.png`) });
        }
      }
      assert.deepEqual(errors, []);
      results.push({ engine, browserVersion: browser.version(), width, tone,
        keyboardEntryAndPrivateHeaderLink: true, credentialFileRecipeRendered: true,
        roleAndTransportLimitsRendered: true, noBodyOverflow: true, errors });
      await browser.close();
      browser = undefined;
    }
    const docs = Object.fromEntries(['API', 'AI_AUTONOMY', 'PROXY_AUTH'].map((guide) => [
      `frontend-modern/public/docs/${guide}.md`,
      createHash('sha256').update(fs.readFileSync(path.join(root, 'public/docs', `${guide}.md`))).digest('hex'),
    ]));
    const result = { result: 'passed', playwrightVersion: require('playwright/package.json').version,
      scope: 'rendered help/links; not native proxy, authentication or AI settings acceptance', docs, results };
    fs.writeFileSync(path.join(artifacts, 'result.json'), JSON.stringify(result, null, 2) + '\n');
    console.log(JSON.stringify(result));
  } finally {
    if (browser) await browser.close();
    await server.close();
  }
})().catch((error) => { console.error(error); process.exitCode = 1; });
