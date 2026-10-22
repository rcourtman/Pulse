// Production Docs/router/Markdown/styles; synthetic outer shell and GET-only
// local files. This does not execute an import, cleanup or recovery command.
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const http = require('node:http');
const crypto = require('node:crypto');
const { chromium, webkit } = require('playwright');

const root = '/workspace';
const frontend = path.join(root, 'frontend-modern');
const preview = path.join(root, 'tmp/contribution-import-safety/static');
const output = path.join(root, 'tmp/contribution-import-safety/browser');
const hash = bytes => crypto.createHash('sha256').update(bytes).digest('hex');
const result = { result: 'failed', cases: [], captures: [], requests: [], errors: [], cleanup: {} };
let server, browser;

(async () => {
  fs.mkdirSync(output, { recursive: true });
  try {
    const bindingBytes = fs.readFileSync(path.join(preview, 'binding.json'));
    const binding = JSON.parse(bindingBytes);
    result.source_sha = binding.source_sha;
    result.binding_sha256 = hash(bindingBytes);
    for (const [file, expected] of Object.entries(binding.sources))
      assert.equal(hash(fs.readFileSync(path.join(frontend, file))), expected, file);
    for (const [file, expected] of Object.entries(binding.artifacts))
      assert.equal(hash(fs.readFileSync(path.join(preview, file))), expected, file);
    const guideHash = binding.sources['public/docs/CONTRIBUTING.md'];
    assert.equal(hash(fs.readFileSync(path.join(root, 'CONTRIBUTING.md'))), guideHash);
    result.playwright = require('playwright/package.json').version;
    assert.equal(result.playwright,
      JSON.parse(fs.readFileSync(path.join(root, 'tests/integration/package-lock.json')))
        .packages['node_modules/@playwright/test'].version);
    server = http.createServer((req, res) => {
      const route = new URL(req.url, 'http://fixture.invalid').pathname;
      let asset = route.slice(1);
      if (route.startsWith('/docs/') && !route.endsWith('.md'))
        asset = 'browser-tests/doc-command-help.html';
      result.requests.push({ method: req.method, path: route });
      if (req.method !== 'GET' || !Object.hasOwn(binding.artifacts, asset)) {
        res.writeHead(404); res.end(); return;
      }
      const bytes = fs.readFileSync(path.join(preview, asset));
      assert.equal(hash(bytes), binding.artifacts[asset], asset);
      const type = asset.endsWith('.md') ? 'text/markdown' : asset.endsWith('.css') ? 'text/css' :
        asset.endsWith('.js') ? 'text/javascript' : 'text/html';
      res.writeHead(200, { 'Content-Type': `${type}; charset=utf-8`, 'Cache-Control': 'no-store' });
      res.end(bytes);
    });
    await new Promise(resolve => server.listen(0, '127.0.0.1', resolve));
    const origin = `http://127.0.0.1:${server.address().port}`;
    for (const [name, engine, width, height, dark] of [
      ['desktop-chromium', chromium, 1365, 1000, false],
      ['narrow-webkit', webkit, 320, 960, true],
    ]) {
      browser = await engine.launch(engine === chromium
        ? { headless: true, channel: 'chromium', args: ['--no-sandbox'] }
        : { headless: true });
      const context = await browser.newContext({ viewport: { width, height },
        isMobile: width < 500, hasTouch: width < 500, colorScheme: dark ? 'dark' : 'light' });
      await context.route('**/*', async route => {
        if (new URL(route.request().url()).origin !== origin) {
          result.errors.push({ kind: 'external-request', url: route.request().url() });
          await route.abort();
        } else await route.continue();
      });
      await context.addInitScript(isDark => {
        window.fixtureDocumentIdentity = crypto.randomUUID();
        document.addEventListener('DOMContentLoaded', () =>
          document.documentElement.classList.toggle('dark', isDark), { once: true });
      }, dark);
      const page = await context.newPage();
      page.setDefaultTimeout(10000);
      page.on('pageerror', error => result.errors.push({ kind: 'page', message: error.message }));
      page.on('console', message => {
        if (message.type() === 'error') result.errors.push({ kind: 'console', message: message.text() });
      });
      const focused = id => page.waitForFunction(value => document.activeElement?.id === value, id);
      const activate = async link => {
        if (width < 500) await link.tap();
        else { await link.focus(); await page.keyboard.press('Enter'); }
      };
      const capture = async (suffix, locator, end = false) => {
        await locator.evaluate((element, atEnd) => {
          const box = element.getBoundingClientRect();
          window.scrollBy(0, atEnd ? box.bottom - innerHeight + 64 : box.top - 64);
        }, end);
        await page.evaluate(() => new Promise(resolve => requestAnimationFrame(() => requestAnimationFrame(resolve))));
        assert.ok(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1));
        const file = `${name}-${suffix}.png`;
        await page.screenshot({ path: path.join(output, file), animations: 'disabled' });
        result.captures.push({ file, sha256: hash(fs.readFileSync(path.join(output, file))) });
      };
      await page.goto(`${origin}/docs/README#development-and-reference`, { waitUntil: 'domcontentloaded' });
      await focused('development-and-reference');
      const identity = await page.evaluate(() => window.fixtureDocumentIdentity);
      await activate(page.getByRole('link', { name: 'isolated development workflow', exact: true }));
      await page.waitForURL('**/docs/CONTRIBUTING#hot-reload-dev-loop');
      await focused('hot-reload-dev-loop');
      const paragraph = page.locator('article p').filter({ hasText: 'The explicit copy helper requires GNU' });
      const safetyText = (await paragraph.innerText()).replace(/\s+/g, ' ');
      for (const phrase of [
        'requires GNU realpath and reads installed files only',
        'different installed/development keys, stop the import',
        'do not delete ciphertext or pick the newest backup',
        'leaves development sessions, CSRF state, OIDC, local .env, history and alerts alone',
        'stages reads before per-file replacement',
        'keeps overwritten files privately in .config-import.*/previous',
        'not a full-state backup or a consistent snapshot of a running installation',
        'keep the snapshot and remaining staged files',
        'inspect the partial import privately before starting anything',
        'The helper starts no service',
      ]) assert.ok(safetyText.includes(phrase), phrase);
      const selector = page.locator('article p').filter({ hasText: 'Keep both installed-data selectors' });
      assert.ok((await selector.innerText()).includes('Neither is a workaround for a startup refusal'));
      const geometry = await paragraph.evaluate(element => {
        const box = element.getBoundingClientRect();
        return { x: box.x, right: box.right, width: box.width, height: box.height };
      });
      assert.ok(geometry.x >= 0 && geometry.right <= width + 1);
      await capture('import-refusal', width < 500 ? paragraph : selector);
      if (width < 500) await capture('partial-import', paragraph, true);
      const cleanupHeading = page.locator('#cleaning-development-build-output');
      const cleanupText = (await page.locator('article').innerText()).replace(/\s+/g, ' ');
      for (const phrase of ['Neither command resets Pulse', 'cleanup stops before removing anything',
        'Removal failures are reported without using sudo',
        'Cleanup does not stop processes or restore anything an earlier cleanup deleted'])
        assert.ok(cleanupText.includes(phrase), phrase);
      await capture('cleanup-preserved', cleanupHeading);
      await activate(page.locator('article a[href="#hot-reload-dev-loop"]'));
      await focused('hot-reload-dev-loop');
      assert.equal(await page.evaluate(() => window.fixtureDocumentIdentity), identity);
      const response = await page.request.get(`${origin}/docs/CONTRIBUTING.md`);
      assert.equal(response.status(), 200);
      assert.equal(hash(await response.body()), guideHash);
      result.cases.push({ name, browser: browser.version(), width, height, dark, geometry,
        interaction: width < 500 ? 'touch' : 'keyboard Enter',
        exactFetchedGuide: true, importSafetyAndCleanupRendered: true,
        fragmentFocusAndSameShell: true, noDocumentOverflow: true });
      await context.close();
      await browser.close(); browser = undefined;
    }
    assert.deepEqual(result.errors, []);
    assert.ok(result.requests.every(req => req.method === 'GET' && !req.path.startsWith('/api/')));
    result.result = 'passed';
  } catch (error) {
    result.error = String(error.stack || error); process.exitCode = 1;
  } finally {
    if (browser) await browser.close();
    result.cleanup.browserClosed = true;
    if (server) await new Promise(resolve => server.close(resolve));
    result.cleanup.serverClosed = true;
    fs.writeFileSync(path.join(output, 'result.json'), JSON.stringify(result, null, 2) + '\n');
    console.log(JSON.stringify(result));
  }
})().catch(error => { console.error(error); process.exitCode = 1; });
