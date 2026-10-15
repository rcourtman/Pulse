// The shipped PMG guide through the actual Docs page, router and Markdown renderer.
// No PMG request, mail operation or diagnostic run is performed.
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const crypto = require('node:crypto');
const { chromium, webkit } = require('playwright');

(async () => {
  const root = process.cwd();
  const frontend = path.join(root, 'frontend-modern');
  const output = path.join(root, 'tmp/pmg-reader-help');
  fs.mkdirSync(output, { recursive: true });
  const hash = (file) => crypto.createHash('sha256').update(fs.readFileSync(file)).digest('hex');
  const { createServer } = await import(path.join(frontend, 'node_modules/vite/dist/node/index.js'));
  const server = await createServer({
    root: frontend, configFile: path.join(frontend, 'vite.config.ts'),
    cacheDir: path.join(frontend, 'node_modules/.vite-pmg-reader-help'),
    server: { host: '127.0.0.1', port: 5199, strictPort: true },
  });
  const results = [];
  const captures = [];
  const cleanup = { browsers: false, server: false };
  let browser;
  let failure;
  try {
    await server.listen();
    for (const [engine, width, height] of [['chromium', 1440, 1000], ['webkit', 390, 844]]) {
      browser = await { chromium, webkit }[engine].launch(engine === 'chromium'
        ? { headless: true, channel: 'chromium', args: ['--no-sandbox'] }
        : { headless: true });
      const page = await browser.newPage({ viewport: { width, height },
        ...(engine === 'webkit' ? { isMobile: true, hasTouch: true } : {}) });
      const errors = [];
      page.on('pageerror', (error) => errors.push(error.message));
      page.on('console', (message) => { if (message.type() === 'error') errors.push(message.text()); });
      page.on('response', (response) => {
        if (response.status() >= 400) errors.push(`${response.status()} ${response.url()}`);
      });
      // The fixture's app-auth probe is synthetic; docs, links and styles are not.
      await page.route(/^http:\/\/127\.0\.0\.1:5199\/api\//, (route) => route.fulfill({
        status: 200, contentType: 'application/json', body: '{"hasAuthentication":false}',
      }));
      await page.goto('http://127.0.0.1:5199/browser-tests/docs-fragment-navigation.html');
      await page.getByRole('link', { name: '← All documentation', exact: true }).click();
      const guide = page.getByRole('link', { name: 'Proxmox Mail Gateway', exact: true });
      await guide.focus();
      await page.keyboard.press('Enter');
      const heading = page.locator('#cluster-response-or-backup-lookup-errors');
      await heading.waitFor();
      const text = await heading.evaluate((node) =>
        [node.nextElementSibling, node.nextElementSibling.nextElementSibling]
          .map((paragraph) => paragraph.textContent).join(' ').replace(/\s+/g, ' '));
      for (const phrase of ['cannot unmarshal string', 'ClusterStatus.cid',
        "Pulse connection's display name", 'native PMG node name', 'hostname lookup failed',
        'Do not rename the connection or gateway, change DNS',
        "PMG's own web interface", 'not proof of an empty queue or no backups',
        'no new Test, backup, mail traffic or diagnostic run']) assert.ok(text.includes(phrase), phrase);
      await heading.evaluate((node) => node.scrollIntoView({ block: 'start' }));
      const capture = async (name) => {
        const file = path.join(output, `${engine}-${name}.png`);
        await page.screenshot({ path: file });
        captures.push({ file: path.relative(root, file), sha256: hash(file) });
      };
      await capture('reader-errors');
      if (engine === 'webkit') {
        await heading.evaluate((node) => node.nextElementSibling.nextElementSibling
          .scrollIntoView({ block: 'start' }));
        await capture('native-fallback');
      }
      assert.ok(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1));
      const limits = page.getByRole('link', { name: 'collection limits', exact: true });
      await limits.focus();
      await page.keyboard.press('Enter');
      await page.waitForFunction(() => document.activeElement?.id === 'read-the-mail-gateway-view');
      assert.equal(new URL(page.url()).hash, '#read-the-mail-gateway-view');
      const bounds = await page.locator('#read-the-mail-gateway-view').boundingBox();
      assert.ok(bounds && bounds.y >= 0 && bounds.y < height);
      await page.goBack();
      await heading.waitFor();
      assert.equal(new URL(page.url()).pathname, '/docs/MAIL_GATEWAY');
      assert.deepEqual(errors, []);
      results.push({ engine, version: browser.version(), width, height, errorPair: true,
        nativeFallback: true, noStateChangeAdvice: true, keyboardIndexNavigation: true,
        keyboardFragmentFocus: true, backPreservesGuide: true, noOverflow: true, errors });
      await browser.close();
      browser = undefined;
    }
  } catch (error) {
    failure = error.stack || error.message;
  } finally {
    if (browser) await browser.close();
    cleanup.browsers = true;
    await server.close();
    cleanup.server = true;
  }
  const result = { result: failure ? 'failed' : 'passed', failure, results, captures, cleanup,
    playwright: require('playwright/package.json').version,
    document_sha256: hash(path.join(frontend, 'public/docs/MAIL_GATEWAY.md')),
    script_sha256: hash(__filename), scope: 'Shipped help only, not PMG collection or recovery.' };
  fs.writeFileSync(path.join(output, 'result.json'), JSON.stringify(result, null, 2) + '\n');
  console.log(JSON.stringify(result));
  process.exitCode = failure ? 1 : 0;
})().catch((error) => { console.error(error); process.exitCode = 1; });
