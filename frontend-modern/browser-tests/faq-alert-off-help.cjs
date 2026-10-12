// Exact own-parent comparison using the production Docs page/router/Markdown/CSS.
// No settings mutation, alert generation, queue replay or backend request.
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const { createHash } = require('node:crypto');
const { chromium, webkit } = require('playwright');

(async () => {
  const root = path.resolve('frontend-modern');
  const artifacts = path.join(root, 'node_modules/faq-alert-off-browser');
  const parent = fs.readFileSync(path.join(artifacts, 'parent.md'), 'utf8');
  const final = fs.readFileSync(path.join(root, 'public/docs/FAQ.md'), 'utf8');
  const hash = (value) => createHash('sha256').update(value).digest('hex');
  process.chdir(root);
  const { createServer } = await import(path.join(root, 'node_modules/vite/dist/node/index.js'));
  const server = await createServer({ root, configFile: path.join(root, 'vite.config.ts'),
    cacheDir: path.join(artifacts, 'vite-cache'),
    server: { host: '127.0.0.1', port: 5259, strictPort: true },
    optimizeDeps: { noDiscovery: true, include: [] } });
  const results = [];
  let browser;
  try {
    await server.listen();
    for (const [engine, width, tone] of [['chromium', 1280, 'light'], ['webkit', 390, 'dark']]) {
      browser = await { chromium, webkit }[engine].launch(engine === 'chromium'
        ? { headless: true, channel: 'chromium', args: ['--no-sandbox'] } : { headless: true });
      const context = await browser.newContext({ viewport: { width, height: 1000 },
        ...(engine === 'webkit' ? { isMobile: true, hasTouch: true } : {}) });
      const captures = [];
      for (const [label, content] of [['parent', parent], ['final', final]]) {
        const page = await context.newPage();
        const errors = [], backendRequests = [];
        page.on('pageerror', (error) => errors.push(error.message));
        page.on('request', (request) => { if (new URL(request.url()).pathname.startsWith('/api/')) backendRequests.push(request.url()); });
        await page.route('**/docs/FAQ.md', (route) => route.fulfill({ status: 200,
          contentType: 'text/markdown; charset=utf-8', body: content }));
        await page.goto('http://127.0.0.1:5259/browser-tests/docs-fragment-navigation.html',
          { waitUntil: 'domcontentloaded', timeout: 30_000 });
        if (tone === 'dark') await page.evaluate(() => document.documentElement.classList.add('dark'));
        await page.getByRole('link', { name: '← All documentation' }).click();
        await page.locator('a[data-doc-link][href="/docs/FAQ"]').first().focus();
        await page.keyboard.press('Enter');
        const heading = page.getByRole('heading', { name: 'Can I disable alerts for specific metrics?', exact: true });
        await heading.waitFor();
        const text = (await page.locator('article').innerText()).replace(/\s+/g, ' ');
        if (label === 'parent') {
          assert.ok(text.includes('toggle next to any metric'));
          assert.ok(!text.includes('Patrol findings are separate from threshold alerts'));
        } else {
          for (const phrase of ['not every default has a per-resource control',
            'explicit threshold can remain active', 'Already queued notifications',
            'Patrol findings are separate from threshold alerts',
            'does not hide its readings, stop collection', 'Do not induce an alert']) assert.ok(text.includes(phrase), phrase);
        }
        await heading.evaluate((element) => { element.scrollIntoView({ block: 'start' }); window.scrollBy(0, -20); });
        assert.ok(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1));
        const capture = `${engine}-${label}.jpg`;
        await page.screenshot({ path: path.join(artifacts, capture), type: 'jpeg', quality: 70 });
        captures.push(capture);
        if (label === 'final') {
          for (const [name, target, title] of [
            ['how Patrol works', '/docs/AI#how-patrol-works', 'How Patrol Works'],
            ['ordinary notification checks', '/docs/TROUBLESHOOTING#test-succeeds-but-real-alerts-are-missing', 'Test succeeds but real alerts are missing'],
          ]) {
            await page.getByRole('link', { name, exact: true }).focus();
            await page.keyboard.press('Enter');
            await page.getByRole('heading', { name: title, exact: true }).waitFor();
            assert.ok(page.url().endsWith(target));
            await page.goBack();
            await heading.waitFor();
          }
        }
        assert.deepEqual(errors, []);
        assert.deepEqual(backendRequests, []);
        await page.close();
      }
      results.push({ engine, version: browser.version(), width, tone, captures,
        parentGapReproduced: true, finalGuidanceRendered: true, keyboardLinks: 2,
        noPageOverflow: true, backendRequests: [], errors: [] });
      await context.close();
      await browser.close();
      browser = undefined;
    }
    const result = { result: 'passed', playwright: require('playwright/package.json').version,
      parentSha256: hash(parent), finalSha256: hash(final), results,
      scope: 'help rendering/navigation, not installed alert or Patrol suppression' };
    fs.writeFileSync(path.join(artifacts, 'result.json'), JSON.stringify(result, null, 2) + '\n');
    console.log(JSON.stringify(result));
  } finally {
    if (browser) await browser.close();
    await server.close();
  }
})().catch((error) => { console.error(error); process.exitCode = 1; });
