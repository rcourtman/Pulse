// Inspect the real shipped Docs page and copyable webhook template. This is
// offline rendering, not a ticket receiver, notification or external delivery.
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const crypto = require('node:crypto');
const { chromium, webkit } = require('playwright');

(async () => {
  const root = '/workspace/frontend-modern';
  const artifacts = path.join(root, 'node_modules', 'webhook-guidance-browser');
  fs.mkdirSync(artifacts, { recursive: true });
  const { createServer } = await import(path.join(root, 'node_modules/vite/dist/node/index.js'));
  const server = await createServer({ root, configFile: path.join(root, 'vite.config.ts'),
    cacheDir: path.join(root, 'node_modules', '.vite-webhook-guidance'),
    server: { host: '127.0.0.1', port: 5253, strictPort: true },
    optimizeDeps: { noDiscovery: true, include: [] } });
  const docs = fs.readFileSync(path.join(root, 'public/docs/WEBHOOKS.md'), 'utf8');
  const expectedTemplate = docs.split('### Sample PSA payloads')[1].match(/```json\n([\s\S]*?)\n```/)[1];
  let browser;
  const results = [];
  try {
    await server.listen();
    for (const [engine, width] of [['chromium', 1280], ['webkit', 390]]) {
      browser = await { chromium, webkit }[engine].launch(engine === 'chromium'
        ? { headless: true, channel: 'chromium', args: ['--no-sandbox'] } : { headless: true });
      const page = await browser.newPage({ viewport: { width, height: 1000 },
        ...(engine === 'webkit' ? { isMobile: true, hasTouch: true } : {}) });
      const errors = [];
      const unexpectedAPIRequests = [];
      page.on('pageerror', (error) => errors.push(error.message));
      page.on('console', (message) => { if (message.type() === 'error') errors.push(message.text()); });
      page.on('request', (request) => {
        const pathname = new URL(request.url()).pathname;
        if (pathname.startsWith('/api/') && pathname !== '/api/security/status') {
          unexpectedAPIRequests.push(pathname);
        }
      });
      let securityRequests = 0;
      // Docs imports runtime context, whose initial auth check is unrelated
      // to Markdown rendering. Model first-run state explicitly; there is no
      // authenticated runtime, backend or provider in this offline fixture.
      await page.route('**/api/security/status', async (route) => {
        securityRequests++;
        await route.fulfill({ json: { hasAuthentication: false, requiresAuth: true } });
      });
      await page.goto('http://127.0.0.1:5253/browser-tests/docs-fragment-navigation.html?scenario=webhooks',
        { waitUntil: 'domcontentloaded', timeout: 60_000 });
      const heading = page.getByRole('heading', { name: 'Receiver correlation and deduplication', exact: true });
      await heading.waitFor();
      if (engine === 'webkit') await page.evaluate(() => document.documentElement.classList.add('dark'));
      const body = (await page.locator('article').innerText()).replace(/\s+/g, ' ');
      for (const text of ['not a unique incident or delivery ID', 'Do not deduplicate permanently',
        'old delayed recovery must not close a newer incident', 'event and severity',
        'whole-second precision', 'incomplete or ambiguous events',
        'successful firing-delivery receipt', 'informational conditions can use "info"']) {
        assert.ok(body.includes(text), text);
      }
      assert.ok(!body.includes('deduplicate on it'));
      const follow = async (name, href, target) => {
        const link = page.getByRole('link', { name, exact: true });
        assert.equal(await link.getAttribute('href'), href);
        await link.focus();
        await page.keyboard.press('Enter');
        await page.waitForURL(`**${href}`);
        await page.waitForFunction((id) => document.activeElement?.id === id, target);
      };
      await follow('receiver correlation and deduplication', '#receiver-correlation-and-deduplication',
        'receiver-correlation-and-deduplication');
      const screenshot = async (name, headingName) => {
        await page.getByRole('heading', { name: headingName, exact: true })
          .evaluate((element) => element.scrollIntoView({ block: 'start' }));
        await page.evaluate(async () => {
          await new Promise((resolve) => requestAnimationFrame(() => requestAnimationFrame(resolve)));
          await Promise.all(document.getAnimations().map((animation) => animation.finished.catch(() => {})));
        });
        assert.ok(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1));
        await page.screenshot({ path: path.join(artifacts, `${engine}-${name}.png`) });
      };
      await screenshot('receiver', 'Receiver correlation and deduplication');
      await follow('full PSA template below', '#sample-psa-payloads', 'sample-psa-payloads');
      const template = page.locator('pre code').filter({ hasText: '"alertCount":' });
      assert.equal(await template.count(), 1);
      assert.equal((await template.innerText()).trim(), expectedTemplate.trim());
      assert.ok(await template.evaluate((element) => {
        const pre = element.closest('pre');
        return getComputedStyle(pre).overflowX === 'auto' || pre.scrollWidth <= pre.clientWidth + 1;
      }));
      await screenshot('template', 'Sample PSA payloads');
      await page.reload({ waitUntil: 'domcontentloaded' });
      await page.getByRole('heading', { name: 'Sample PSA payloads', exact: true }).waitFor();
      await page.waitForFunction(() => document.activeElement?.id === 'sample-psa-payloads');
      // An incidental import is not obliged to make an auth request on every
      // engine/reload. Observe those requests, but assert only the Docs result
      // and absence of unintended backend activity or errors.
      await page.waitForLoadState('networkidle');
      assert.deepEqual(errors, []);
      assert.deepEqual(unexpectedAPIRequests, []);
      results.push({ engine, browserVersion: browser.version(), width,
        scope: engine === 'webkit' ? 'phone-emulated, not native' : 'desktop',
        warningsRendered: true, keyboardReceiverLink: true, keyboardTemplateLink: true,
        templateTextExact: true, reloadFragmentFocus: true, noDocumentOverflow: true,
        syntheticFirstRunSecurityRequests: securityRequests, unexpectedAPIRequests, errors });
      await browser.close();
      browser = undefined;
    }
    const files = ['public/docs/WEBHOOKS.md', 'browser-tests/docs-fragment-navigation.tsx',
      'browser-tests/webhook-guidance.cjs', 'src/features/docs/docMarkdown.ts', 'src/pages/Docs.tsx']
      .map((file) => ({ file, sha256: crypto.createHash('sha256').update(fs.readFileSync(path.join(root, file))).digest('hex') }));
    fs.writeFileSync(path.join(artifacts, 'result.json'), JSON.stringify({ playwright: require('playwright/package.json').version,
      files, results, limits: 'No real receiver, ticket, provider, native device or installed notification acceptance.' }, null, 2));
  } finally {
    if (browser) await browser.close();
    await server.close();
  }
})().catch((error) => { console.error(error); process.exitCode = 1; });
