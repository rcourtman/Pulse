// Consume the existing v6 migration entry point with the production Docs/router.
// Safe static documentation and one synthetic security-status GET only.
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const crypto = require('node:crypto');
const { chromium } = require('playwright');
const hash = bytes => crypto.createHash('sha256').update(bytes).digest('hex');

(async () => {
  const root = path.resolve('frontend-modern');
  const output = path.resolve('tmp/upgrade-guide-browser-observed');
  fs.mkdirSync(output, { recursive: true });
  const source = fs.readFileSync(path.join(root, 'public/docs/UPGRADE_v6.md'));
  assert.equal(hash(source), hash(fs.readFileSync(path.resolve('docs/UPGRADE_v6.md'))));
  assert.equal(require('playwright/package.json').version,
    JSON.parse(fs.readFileSync('tests/integration/package-lock.json')).packages['node_modules/@playwright/test'].version);
  const port = 5263;
  const origin = `http://127.0.0.1:${port}`;
  process.chdir(root);
  const { createServer } = await import(path.join(root, 'node_modules/vite/dist/node/index.js'));
  const server = await createServer({ root, configFile: path.join(root, 'vite.config.ts'),
    cacheDir: path.join(root, 'node_modules/.vite-upgrade-guide'),
    server: { host: '127.0.0.1', port, strictPort: true },
    optimizeDeps: { noDiscovery: true, include: [] } });
  const result = { result: 'incomplete', playwright: require('playwright/package.json').version,
    driver_sha256: hash(fs.readFileSync(__filename)), document_sha256: hash(source),
    scope: 'Existing migration help and actual linked Docs consumers; not installed upgrade, native recovery or private runtime acceptance',
    cases: [], cleanup: { browser: false, server: false } };
  let browser;
  try {
    await server.listen();
    for (const [width, tone] of [[1280, 'light'], [390, 'dark']]) {
      const phone = width < 500;
      browser = await chromium.launch({ headless: true, channel: 'chromium', args: ['--no-sandbox'] });
      const context = await browser.newContext({ viewport: { width, height: 1000 },
        ...(phone ? { isMobile: true, hasTouch: true } : {}), locale: 'en-GB' });
      const page = await context.newPage();
      page.setDefaultTimeout(15000);
      const record = { width, tone, input: phone ? 'touch' : 'keyboard',
        browserVersion: browser.version(), checks: [], captures: [], pageErrors: [], apiRequests: [],
        externalRequests: [], servedGuide: [] };
      result.cases.push(record);
      page.on('pageerror', error => record.pageErrors.push(error.message));
      page.on('console', message => { if (message.type() === 'error') record.pageErrors.push(message.text()); });
      page.on('response', async response => {
        if (new URL(response.url()).pathname === '/docs/UPGRADE_v6.md') {
          const body = await response.body();
          fs.writeFileSync(path.join(output, `served-${width}.md`), body);
          record.servedGuide.push({ status: response.status(), sha256: hash(body), bytes: body.length });
        }
      });
      await page.route('**/*', route => {
        const request = route.request(); const url = new URL(request.url());
        if (url.origin !== origin) { record.externalRequests.push(url.href); return route.abort(); }
        if (url.pathname.startsWith('/api/')) {
          record.apiRequests.push({ method: request.method(), path: url.pathname });
          if (request.method() !== 'GET' || url.pathname !== '/api/security/status') return route.abort();
          return route.fulfill({ status: 200, contentType: 'application/json', body: '{}' });
        }
        return route.continue();
      });
      const activate = async locator => {
        await locator.scrollIntoViewIfNeeded();
        if (phone) await locator.tap();
        else { await locator.focus(); await page.keyboard.press('Enter'); }
      };
      const overflow = async () => assert.ok(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1));
      const capture = async (id, name) => {
        await page.evaluate(fragment => { window.location.hash = '#' + fragment; }, id);
        await page.waitForFunction(fragment => document.activeElement?.id === fragment, id);
        await overflow();
        const filename = `chromium-${width}-${name}.png`;
        await page.screenshot({ path: path.join(output, filename) });
        record.captures.push(filename);
      };
      await page.goto(`${origin}/browser-tests/docs-fragment-navigation.html`, { waitUntil: 'domcontentloaded' });
      await activate(page.getByRole('link', { name: '← All documentation', exact: true }));
      await activate(page.locator('a[data-doc-link][href="/docs/UPGRADE_v6"]').first());
      await page.getByRole('heading', { name: 'Upgrade to Pulse v6', exact: true }).waitFor();
      if (tone === 'dark') await page.evaluate(() => document.documentElement.classList.add('dark'));
      const article = page.locator('article');
      let text = await article.innerText();
      for (const boundary of ['not a full-state backup', 'history and agent enrolment state',
        'matching encryption/signing keys', 'recreate only the Pulse service',
        'Do not start, cancel or repeat', 'An image or binary change alone is not a data rollback',
        'Relay is no longer sold']) assert.ok(text.includes(boundary), boundary);
      assert.equal(await article.locator('pre').filter({ hasText: '/bin/update --version' }).count(), 0);
      assert.equal(await article.locator('pre').filter({ hasText: 'docker compose up' }).count(), 0);
      record.checks.push('Published guide entry, full-state/edition/uncertainty boundaries, no copied downgrade/all-service shortcut');
      await capture('before-you-upgrade', 'preparation');
      await capture('docker', 'deployment');
      // Real cross-document route and fragment focus, not an asserted href only.
      for (const [href, id, phrase] of [
        ['/docs/MIGRATION#full-state-recovery', 'full-state-recovery', 'configuration export'],
        ['/docs/INSTALL#manual-update', 'manual-update', 'helper'],
        ['/docs/DOCKER#-updates', '-updates', 'image'],
        ['/docs/AUTO_UPDATE#manual-rollback', 'manual-rollback', 'keys'],
      ]) {
        await activate(page.locator(`a[data-doc-link][href="${href}"]`).first());
        await page.waitForURL(`**${href}`);
        await page.waitForFunction(fragment => document.activeElement?.id === fragment, id);
        assert.ok((await page.locator('article').innerText()).toLowerCase().includes(phrase));
        await overflow();
        record.checks.push(`Consuming ${href}: input activation, focused heading and rendered recovery/deployment text`);
        await page.goBack({ waitUntil: 'domcontentloaded' });
        await page.getByRole('heading', { name: 'Upgrade to Pulse v6', exact: true }).waitFor();
      }
      await capture('failed-or-interrupted-upgrade', 'uncertainty');
      await capture('can-i-keep-pulse-v5-stable-while-i-test-pulse-v6', 'rollback');
      await page.evaluate(() => { window.location.hash = '#failed-or-interrupted-upgrade'; });
      await page.waitForFunction(() => document.activeElement?.id === 'failed-or-interrupted-upgrade');
      await page.reload({ waitUntil: 'domcontentloaded' });
      await page.waitForFunction(() => document.activeElement?.id === 'failed-or-interrupted-upgrade');
      assert.ok((await page.locator('article').innerText()).includes('Do not repeat the update'));
      record.checks.push('Direct fragment and real reload retain focused interrupted-upgrade help');
      assert.deepEqual(record.pageErrors, []);
      assert.deepEqual(record.externalRequests, []);
      assert.ok(record.apiRequests.every(request => request.method === 'GET' && request.path === '/api/security/status'));
      assert.ok(record.servedGuide.length >= 1);
      assert.ok(record.servedGuide.every(observation => observation.status === 200 && observation.sha256 === result.document_sha256));
      await browser.close(); browser = undefined;
    }
    result.result = 'passed'; result.verifiedAt = new Date().toISOString();
  } finally {
    if (browser) await browser.close();
    result.cleanup.browser = true;
    await server.close(); result.cleanup.server = true;
    fs.writeFileSync(path.join(output, 'result.json'), JSON.stringify(result, null, 2) + '\n');
    console.log(JSON.stringify(result));
  }
})().catch(error => { console.error(error); process.exitCode = 1; });
