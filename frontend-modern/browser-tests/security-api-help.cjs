// Real Docs consumer, exact shipped guidance and keyboard/touch navigation.
// No credential, administrative API or installed-service action is exercised.
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const http = require('node:http');
const crypto = require('node:crypto');
const { chromium } = require('playwright');

const root = '/workspace';
const frontend = path.join(root, 'frontend-modern');
const artifacts = path.join(root, 'tmp/security-api-help');
const preview = path.join(root, 'tmp/doc-command-help/static');
const hash = bytes => crypto.createHash('sha256').update(bytes).digest('hex');
const bindingBytes = fs.readFileSync(path.join(preview, 'binding.json'));
const binding = JSON.parse(bindingBytes);
const guide = fs.readFileSync(path.join(frontend, 'public/docs/SECURITY.md'), 'utf8');
const recipes = [...guide.matchAll(/```bash\n([\s\S]*?)```/g)]
  .map(match => match[1]).flatMap(block => block.split(/\n(?=# )/))
  .filter(step => step.includes('pulse_api ')).map(step => step.trim());
const result = { source: binding.source_sha, binding_sha256: hash(bindingBytes),
  cases: [], captures: [], pageErrors: [], requests: [], served: [] };

(async () => {
  let server, browser;
  fs.mkdirSync(artifacts, { recursive: true });
  try {
    assert.equal(recipes.length, 5);
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
    browser = await chromium.launch({ headless: true, channel: 'chromium', args: ['--no-sandbox'] });
    result.browser = browser.version();
    result.playwright = require('playwright/package.json').version;
    assert.equal(result.playwright, '1.56.1');
    for (const state of [
      { name: 'desktop-light', width: 1440, height: 1000, dark: false },
      { name: 'phone-dark', width: 320, height: 900, dark: true },
    ]) {
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
      const capture = async (name, locator) => {
        await locator.evaluate(el => window.scrollBy(0, el.getBoundingClientRect().top - 64));
        await page.evaluate(() => new Promise(resolve => requestAnimationFrame(() => requestAnimationFrame(resolve))));
        assert.ok(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1));
        const file = `${state.name}-${name}.png`;
        await page.screenshot({ path: path.join(artifacts, file) });
        result.captures.push({ file, sha256: hash(fs.readFileSync(path.join(artifacts, file))), state });
      };
      await page.goto(`${origin}/docs/SECURITY#usage`, { waitUntil: 'domcontentloaded' });
      await page.locator('article #usage').waitFor();
      await page.waitForFunction(() => document.activeElement?.id === 'usage');
      await capture('private-response', page.locator('#usage'));
      const article = await page.locator('article').innerText();
      for (const phrase of ['same Bash session', 'not an installed Pulse command', 'new owner-only file',
        'proxy environment settings', 'twenty-second total deadlines', 'do not repeat a reset'])
        assert.ok(article.includes(phrase), phrase);
      const actual = await page.locator('article pre code').allTextContents();
      const shown = actual.filter(block => block.includes('pulse_api '))
        .flatMap(block => block.split(/\n(?=# )/)).map(step => step.trim());
      assert.deepEqual(shown, recipes);
      assert.ok(!actual.some(block => /curl\s/.test(block)));
      await capture('transport', page.locator('article p').filter({ hasText: "The helper's loopback origin" }));
      // The helper is reached by an actual in-app link, not hand-built HTML.
      const helper = page.locator('article a[href="/docs/API#api-token-recommended"]').first();
      await activate(helper);
      await page.waitForURL('**/docs/API#api-token-recommended');
      await page.waitForFunction(() => document.activeElement?.id === 'api-token-recommended');
      const helperBlock = page.locator('article pre').filter({ hasText: 'pulse_api() (' });
      const helperText = await helperBlock.innerText();
      for (const option of ['--noproxy 127.0.0.1', '--connect-timeout 5', '--max-time 20', '--output', '--globoff'])
        assert.ok(helperText.includes(option), option);
      await helperBlock.evaluate(pre => {
        const code = pre.querySelector('code');
        const index = code.textContent.split('\n').findIndex(line => line.includes('--proto'));
        window.scrollTo(0, pre.getBoundingClientRect().top + window.scrollY + index * parseFloat(getComputedStyle(code).lineHeight) - 150);
        if (innerWidth < 500) pre.scrollLeft = pre.scrollWidth;
      });
      const helperCapture = `${state.name}-helper.png`;
      await page.screenshot({ path: path.join(artifacts, helperCapture) });
      result.captures.push({ file: helperCapture,
        sha256: hash(fs.readFileSync(path.join(artifacts, helperCapture))), state });
      assert.ok(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1));
      await page.goBack({ waitUntil: 'domcontentloaded' });
      await page.waitForURL('**/docs/SECURITY#usage');
      await page.locator('#usage').waitFor();
      const preparation = page.locator('article pre').filter({ hasText: 'Request file must be a regular file.' });
      assert.ok((await preparation.innerText()).includes('vi "$request_file"'));
      await capture('export-preparation', preparation);
      const exportCall = page.locator('article pre').filter({ hasText: 'pulse_api POST /api/config/export' });
      await capture('export', exportCall);
      const reset = page.locator('article #manual-recovery-admin');
      await capture('reset', reset);
      await page.evaluate(() => { location.hash = '#scheduler-health-api'; });
      await page.waitForFunction(() => document.activeElement?.id === 'scheduler-health-api');
      await page.reload({ waitUntil: 'domcontentloaded' });
      await page.waitForFunction(() => document.activeElement?.id === 'scheduler-health-api');
      assert.ok((await page.locator('article').innerText()).includes('pulse_api GET /api/monitoring/scheduler/health'));
      for (const name of ['SECURITY', 'API']) {
        const response = await page.request.get(`${origin}/docs/${name}.md`);
        assert.equal(response.status(), 200);
        assert.equal(hash(await response.body()), binding.sources[`public/docs/${name}.md`]);
      }
      result.cases.push({ ...state, exactRenderedCalls: 5, helperLinkAndHistoryBack: true,
        keyboardOrTouch: state.width < 500 ? 'touch' : 'keyboard', fragmentFocusAndReload: true,
        noDocumentOverflow: true, exactFetchedGuides: true });
      await context.close();
    }
    assert.deepEqual(result.pageErrors, []);
    assert.ok(result.requests.every(request => request.method === 'GET'
      && new URL(request.url).origin === origin
      && !new URL(request.url).pathname.startsWith('/api/')));
    result.result = 'passed';
  } catch (error) {
    result.result = 'failed'; result.error = String(error.stack || error); process.exitCode = 1;
  } finally {
    if (browser) { await browser.close(); result.browser_closed = true; }
    if (server) { await new Promise(resolve => server.close(resolve)); result.server_closed = true; }
    fs.writeFileSync(path.join(artifacts, 'result.json'), JSON.stringify(result, null, 2) + '\n');
    console.log(JSON.stringify({ source: result.source, result: result.result, cases: result.cases.length,
      captures: result.captures.length, browser_closed: result.browser_closed,
      server_closed: result.server_closed, error: result.error }));
  }
})().catch(error => { console.error(error); process.exitCode = 1; });
