// Read the real shipped Docs/router/Markdown/styles, in a synthetic outer shell.
// No AI provider, MCP, telemetry, setting or other application API is exercised.
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const http = require('node:http');
const crypto = require('node:crypto');
const { chromium, webkit } = require('playwright');
const root = '/workspace';
const frontend = path.join(root, 'frontend-modern');
const preview = path.join(root, 'tmp/doc-command-help/static');
const artifacts = path.join(root, 'tmp/ai-privacy-help');
const hash = bytes => crypto.createHash('sha256').update(bytes).digest('hex');
const bindingBytes = fs.readFileSync(path.join(preview, 'binding.json'));
const binding = JSON.parse(bindingBytes);
const guide = fs.readFileSync(path.join(frontend, 'public/docs/AI.md'), 'utf8');
const privacyText = guide.split('## Privacy\n')[1].split('\n---')[0];
const result = { source: binding.source_sha, binding_sha256: hash(bindingBytes),
  cases: [], captures: [], pageErrors: [], requests: [], served: [] };
(async () => {
  let server, browser;
  fs.mkdirSync(artifacts, { recursive: true });
  try {
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
      const focused = id => page.waitForFunction(fragment => document.activeElement?.id === fragment, id);
      const capture = async (name, locator) => {
        await locator.evaluate(el => window.scrollBy(0, el.getBoundingClientRect().top - 64));
        await page.evaluate(() => new Promise(resolve => requestAnimationFrame(() => requestAnimationFrame(resolve))));
        assert.ok(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1));
        const file = `${state.name}-${name}.png`;
        await page.screenshot({ path: path.join(artifacts, file) });
        result.captures.push({ file, sha256: hash(fs.readFileSync(path.join(artifacts, file))), state });
      };
      await page.goto(`${origin}/docs/README`, { waitUntil: 'domcontentloaded' });
      await activate(page.locator('article a[href="/docs/AI"]').first());
      await page.waitForURL('**/docs/AI');
      const intro = page.locator('article p').filter({ hasText: "Patrol's monitoring and orchestration" });
      await intro.waitFor();
      assert.match(await intro.innerText(), /BYOK means using your own provider key, not keeping inference local/);
      assert.ok(!(await page.locator('article').innerText()).includes('complete privacy'));
      await capture('overview', intro);
      await page.evaluate(() => window.__pi_privacy_shell_marker = 'same-shell');
      await activate(page.locator('article a[href="#privacy"]').first());
      await page.waitForURL('**/docs/AI#privacy');
      await focused('privacy');
      assert.equal(await page.evaluate(() => window.__pi_privacy_shell_marker), 'same-shell');
      await capture('destinations', page.locator('#where-model-context-goes'));
      const plain = (await page.locator('article').innerText()).replace(/\s+/g, ' ');
      for (const phrase of ['Local model endpoint under your control', 'Hosted model with your own key or subscription',
        'External agent using Pulse MCP', 'names and addresses can still be shared', 'not blanket anonymisation',
        'limit actions, not model requests', 'never paste credentials', 'read-only explanation',
        'Disabling usage telemetry does not disable AI provider requests or external MCP access'])
        assert.ok(plain.includes(phrase), phrase);
      await capture('redaction-and-actions', page.locator('#redaction-and-action-permissions-are-different-boundaries'));
      await activate(page.locator('article a[href="#usage-telemetry-is-separate"]'));
      await page.waitForURL('**/docs/AI#usage-telemetry-is-separate');
      await focused('usage-telemetry-is-separate');
      await capture('telemetry', page.locator('#usage-telemetry-is-separate'));
      await activate(page.getByRole('link', { name: 'Privacy disclosure', exact: true }));
      await page.waitForURL('**/docs/PRIVACY');
      await page.getByRole('heading', { name: 'Privacy', exact: true }).waitFor();
      await page.goBack({ waitUntil: 'domcontentloaded' });
      await focused('usage-telemetry-is-separate');
      await page.reload({ waitUntil: 'domcontentloaded' });
      await focused('usage-telemetry-is-separate');
      for (const name of ['AI', 'PRIVACY']) {
        const response = await page.request.get(`${origin}/docs/${name}.md`);
        assert.equal(response.status(), 200);
        assert.equal(hash(await response.body()), binding.sources[`public/docs/${name}.md`]);
      }
      result.cases.push({ ...state, browser: browser.version(),
        interaction: state.width < 500 ? 'touch' : 'keyboard', indexAndDisclosureLinks: true,
        providerAndControlLimits: true, fragmentFocusBackAndReload: true,
        noDocumentOverflow: true, exactFetchedGuides: ['AI', 'PRIVACY'],
        privacy_source_sha256: hash(privacyText) });
      await context.close(); await browser.close(); browser = undefined;
    }
    assert.deepEqual(result.pageErrors, []);
    assert.ok(result.requests.every(request => request.method === 'GET'
      && new URL(request.url()).origin === origin && !new URL(request.url()).pathname.startsWith('/api/')));
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
