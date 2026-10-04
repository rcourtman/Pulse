// Verify the copyable receiver example through production Docs/router/Markdown.
// This does not send a webhook or exercise a native receiver or deduplication.
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const { createHash } = require('node:crypto');
const { chromium, webkit } = require('playwright');

(async () => {
  const root = path.resolve('frontend-modern');
  const artifacts = path.join(root, 'node_modules', 'webhook-verification-browser');
  fs.mkdirSync(artifacts, { recursive: true });
  const guide = fs.readFileSync(path.join(root, 'public/docs/WEBHOOKS.md'), 'utf8');
  const example = guide.match(/```python\n([\s\S]*?)```/)[1].trim();
  process.chdir(root);
  const { createServer } = await import(path.join(root, 'node_modules/vite/dist/node/index.js'));
  const server = await createServer({
    root, configFile: path.join(root, 'vite.config.ts'),
    cacheDir: path.join(root, 'node_modules', '.vite-webhook-verification'),
    server: { host: '127.0.0.1', port: 5255, strictPort: true },
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
      await page.goto('http://127.0.0.1:5255/browser-tests/docs-fragment-navigation.html',
        { waitUntil: 'domcontentloaded', timeout: 60_000 });
      if (tone === 'dark') await page.evaluate(() => document.documentElement.classList.add('dark'));
      await page.getByRole('link', { name: '← All documentation' }).click();
      const entry = page.locator('a[data-doc-link][href="/docs/WEBHOOKS"]').first();
      await entry.focus();
      await page.keyboard.press('Enter');
      const heading = page.getByRole('heading', { name: '📦 Delivery Contract', exact: true });
      await heading.waitFor();
      const code = page.locator('pre').filter({ hasText: 'MAX_SKEW_SECONDS = 300' });
      await code.scrollIntoViewIfNeeded();
      assert.equal((await code.locator('code').textContent()).trim(), example);
      assert.ok(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1));
      await page.screenshot({ path: path.join(artifacts, `${engine}-verifier.png`) });
      const warning = page.locator('p').filter({ hasText: 'A time window rejects old captures' });
      await warning.scrollIntoViewIfNeeded();
      const text = await page.locator('article').innerText();
      for (const phrase of ['within five minutes', 'empty secret', 'not every replay',
        'deduplicate atomically', 'receiver restarts', 'not covered by this HMAC',
        'missing or duplicate signing headers', 'before parsing the JSON']) {
        assert.ok(text.includes(phrase), phrase);
      }
      assert.ok(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1));
      await page.screenshot({ path: path.join(artifacts, `${engine}-deduplication.png`) });
      const payloadHeading = page.getByRole('heading', { name: 'Sample PSA payloads', exact: true });
      await payloadHeading.scrollIntoViewIfNeeded();
      assert.ok((await page.locator('pre').filter({ hasText: '"alertId": "{{.ID | jsonString}}"' })
        .textContent()).includes('"event": "{{.Event}}"'));
      assert.deepEqual(errors, []);
      results.push({ engine, browserVersion: browser.version(), width, tone,
        keyboardEntry: true, exactCopiedPython: true, freshnessAndReplayLimits: true,
        payloadIdentity: true, noBodyOverflow: true, errors });
      await browser.close();
      browser = undefined;
    }
    const result = { result: 'passed', playwrightVersion: require('playwright/package.json').version,
      scope: 'rendered guide and exact copied code; not webhook delivery or receiver processing',
      docs: { 'frontend-modern/public/docs/WEBHOOKS.md':
        createHash('sha256').update(guide).digest('hex') }, results };
    fs.writeFileSync(path.join(artifacts, 'result.json'), JSON.stringify(result, null, 2) + '\n');
    console.log(JSON.stringify(result));
  } finally {
    if (browser) await browser.close();
    await server.close();
  }
})().catch(error => { console.error(error); process.exitCode = 1; });
