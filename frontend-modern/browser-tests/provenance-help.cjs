// Real shipped Docs/router/Markdown/styles; a synthetic surrounding shell only.
// No release download, Sigstore verification or installation runs in the browser.
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const http = require('node:http');
const crypto = require('node:crypto');
const { chromium, webkit } = require('playwright');

const root = '/workspace';
const frontend = path.join(root, 'frontend-modern');
const preview = path.join(root, 'tmp/doc-command-help/static');
const artifacts = path.join(root, 'tmp/provenance-help');
const hash = bytes => crypto.createHash('sha256').update(bytes).digest('hex');
const bindingBytes = fs.readFileSync(path.join(preview, 'binding.json'));
const binding = JSON.parse(bindingBytes);
const guide = fs.readFileSync(path.join(frontend, 'public/docs/INSTALL.md'), 'utf8');
const section = guide.split('## Verify release build provenance\n')[1].split('\n## ')[0];
const recipes = [...section.matchAll(/```bash\n([\s\S]*?)```/g)].map(match => match[1].trim());
const result = { source: binding.source_sha, binding_sha256: hash(bindingBytes),
  cases: [], captures: [], pageErrors: [], requests: [], served: [] };

(async () => {
  let server, browser;
  fs.mkdirSync(artifacts, { recursive: true });
  try {
    assert.equal(recipes.length, 2);
    for (const [file, expected] of Object.entries(binding.sources))
      assert.equal(hash(fs.readFileSync(path.join(frontend, file))), expected, file);
    for (const [file, expected] of Object.entries(binding.artifacts))
      assert.equal(hash(fs.readFileSync(path.join(preview, file))), expected, file);
    server = http.createServer((req, res) => {
      const route = new URL(req.url, 'http://fixture.invalid').pathname;
      let asset = route.slice(1);
      if (route.startsWith('/docs/') && !route.endsWith('.md'))
        asset = 'browser-tests/doc-command-help.html';
      if (!Object.hasOwn(binding.artifacts, asset)) { res.writeHead(404); res.end(); return; }
      const bytes = fs.readFileSync(path.join(preview, asset));
      assert.equal(hash(bytes), binding.artifacts[asset]);
      result.served.push({ route, asset, sha256: hash(bytes) });
      const type = asset.endsWith('.md') ? 'text/markdown' : asset.endsWith('.css') ? 'text/css' :
        asset.endsWith('.js') ? 'text/javascript' : 'text/html';
      res.writeHead(200, { 'Content-Type': `${type}; charset=utf-8`, 'Cache-Control': 'no-store' });
      res.end(bytes);
    });
    await new Promise(resolve => server.listen(0, '127.0.0.1', resolve));
    const origin = `http://127.0.0.1:${server.address().port}`;
    result.playwright = require('playwright/package.json').version;
    assert.equal(result.playwright, '1.56.1');
    for (const state of [
      { name: 'desktop-light', engine: 'chromium', width: 1440, height: 1000, dark: false },
      { name: 'phone-dark', engine: 'webkit', width: 320, height: 900, dark: true },
    ]) {
      browser = await { chromium, webkit }[state.engine].launch(state.engine === 'chromium'
        ? { headless: true, channel: 'chromium', args: ['--no-sandbox'] } : { headless: true });
      const context = await browser.newContext({ viewport: { width: state.width, height: state.height },
        isMobile: state.width < 500, hasTouch: state.width < 500 });
      const page = await context.newPage();
      page.on('pageerror', error => result.pageErrors.push(error.message));
      page.on('request', req => result.requests.push({ method: req.method(), url: req.url() }));
      await page.route('**/*', route => new URL(route.request().url()).origin === origin
        ? route.continue() : route.abort());
      await page.addInitScript(dark => {
        document.addEventListener('DOMContentLoaded', () => document.documentElement.classList.toggle('dark', dark));
      }, state.dark);
      const activate = async locator => {
        if (state.width < 500) await locator.tap();
        else { await locator.focus(); await page.keyboard.press('Enter'); }
      };
      const settledFragment = id => page.waitForFunction(fragment =>
        document.activeElement?.id === fragment, id);
      const capture = async (name, locator, offset = 0) => {
        await locator.evaluate((el, delta) => window.scrollBy(0,
          el.getBoundingClientRect().top - 64 + delta), offset);
        await page.evaluate(() => new Promise(resolve => requestAnimationFrame(() => requestAnimationFrame(resolve))));
        assert.ok(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1));
        const file = `${state.name}-${name}.png`;
        await page.screenshot({ path: path.join(artifacts, file) });
        result.captures.push({ file, sha256: hash(fs.readFileSync(path.join(artifacts, file))), state });
      };
      await page.goto(`${origin}/docs/README`, { waitUntil: 'domcontentloaded' });
      await activate(page.locator('article a[href="/docs/INSTALL"]').first());
      await page.waitForURL('**/docs/INSTALL');
      await page.getByRole('heading', { name: 'Verify release build provenance', exact: true }).waitFor();
      await page.goto(`${origin}/docs/INSTALL#verify-release-build-provenance`, { waitUntil: 'domcontentloaded' });
      await settledFragment('verify-release-build-provenance');
      const article = (await page.locator('article').innerText()).replace(/\s+/g, ' ');
      for (const phrase of ['private directory', 'does not install or execute', 'partial files',
        'moving reference', 'no acquisition request', 'refuses to overwrite',
        'later key revocation', 'does not replace', 'not proof of a completed installation'])
        assert.ok(article.includes(phrase), phrase);
      const codes = page.locator('article pre code');
      const actual = (await codes.allTextContents()).map(text => text.trim());
      for (const recipe of recipes) assert.ok(actual.includes(recipe), 'exact copied recipe');
      const connected = page.locator('article pre').filter({ hasText: 'provenance_dir=' }).first();
      const offline = page.locator('article pre').filter({ hasText: 'set -euC' }).first();
      await capture('connected-context', page.locator('#verify-release-build-provenance'));
      const lineOffset = await connected.evaluate(pre => {
        const code = pre.querySelector('code');
        return code.textContent.split('\n').findIndex(line => line.startsWith('SOURCE_SHA='))
          * parseFloat(getComputedStyle(code).lineHeight);
      });
      assert.ok(Number.isFinite(lineOffset) && lineOffset > 0);
      await capture('connected-binding', connected, lineOffset);
      const codeGeometry = await connected.evaluate(pre => {
        pre.scrollLeft = pre.scrollWidth;
        const geometry = { width: pre.clientWidth, content: pre.scrollWidth, offset: pre.scrollLeft };
        pre.scrollLeft = 0;
        return geometry;
      });
      if (codeGeometry.content > codeGeometry.width) assert.ok(codeGeometry.offset > 0);
      // In-page links retain a native fragment href; the real Docs handler
      // resolves it without reloading the viewer or losing fragment focus.
      await activate(page.locator('article a[href="#verify-on-an-offline-target"]'));
      await page.waitForURL('**/docs/INSTALL#verify-on-an-offline-target');
      await settledFragment('verify-on-an-offline-target');
      await capture('offline-recipe', page.locator('#verify-on-an-offline-target'));
      await capture('offline-limits', page.locator('article p').filter({ hasText: 'Refresh the trusted root' }));
      assert.ok((await offline.innerText()).includes('--custom-trusted-root trusted_root.jsonl'));
      await page.goBack({ waitUntil: 'domcontentloaded' });
      await settledFragment('verify-release-build-provenance');
      await page.reload({ waitUntil: 'domcontentloaded' });
      await settledFragment('verify-release-build-provenance');
      const response = await page.request.get(`${origin}/docs/INSTALL.md`);
      assert.equal(response.status(), 200);
      assert.equal(hash(await response.body()), binding.sources['public/docs/INSTALL.md']);
      result.cases.push({ ...state, browser: browser.version(), exactRenderedRecipes: recipes.length,
        indexAndOfflineLinks: true, interaction: state.width < 500 ? 'touch' : 'keyboard',
        codeGeometry, fragmentFocusBackAndReload: true, noDocumentOverflow: true, exactFetchedGuide: true });
      await context.close(); await browser.close(); browser = undefined;
    }
    assert.deepEqual(result.pageErrors, []);
    assert.ok(result.requests.every(request => request.method === 'GET'
      && new URL(request.url).origin === origin && !new URL(request.url).pathname.startsWith('/api/')));
    result.result = 'passed';
  } catch (error) {
    result.result = 'failed'; result.error = String(error.stack || error); process.exitCode = 1;
  } finally {
    if (browser) await browser.close();
    if (server) await new Promise(resolve => server.close(resolve));
    fs.writeFileSync(path.join(artifacts, 'result.json'), JSON.stringify(result, null, 2) + '\n');
    console.log(JSON.stringify(result));
  }
})().catch(error => { console.error(error); process.exitCode = 1; });
