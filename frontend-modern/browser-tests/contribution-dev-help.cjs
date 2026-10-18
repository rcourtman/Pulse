// Actual shipped Docs/router/Markdown/CSS in a synthetic read-only shell.
// Commands are inspected as text, never executed by this browser fixture.
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const http = require('node:http');
const crypto = require('node:crypto');
const { chromium } = require('playwright');

const root = '/workspace';
const frontend = path.join(root, 'frontend-modern');
const preview = path.join(root, 'tmp/doc-command-help/static');
const artifacts = path.join(root, 'tmp/contribution-dev-help');
const hash = bytes => crypto.createHash('sha256').update(bytes).digest('hex');
const bindingBytes = fs.readFileSync(path.join(preview, 'binding.json'));
const binding = JSON.parse(bindingBytes);
const guide = fs.readFileSync(path.join(frontend, 'public/docs/CONTRIBUTING.md'), 'utf8');
const section = guide.split('### Hot Reload Dev Loop\n')[1].split('\n---\n')[0];
const recipe = section.match(/```bash\n([\s\S]*?)```/)[1].trim();
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
    assert.equal(hash(Buffer.from(guide)), hash(fs.readFileSync(path.join(root, 'CONTRIBUTING.md'))));
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
    browser = await chromium.launch({ headless: true, channel: 'chromium', args: ['--no-sandbox'] });
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
      const fragment = async id => page.waitForFunction(value => document.activeElement?.id === value, id);
      const capture = async (name, locator) => {
        await locator.evaluate(el => window.scrollBy(0, el.getBoundingClientRect().top - 64));
        await page.evaluate(() => new Promise(resolve => requestAnimationFrame(() => requestAnimationFrame(resolve))));
        assert.ok(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1));
        const file = `${state.name}-${name}.png`;
        await page.screenshot({ path: path.join(artifacts, file) });
        result.captures.push({ file, sha256: hash(fs.readFileSync(path.join(artifacts, file))), state });
      };
      await page.goto(`${origin}/docs/CONTRIBUTING#getting-started`, { waitUntil: 'domcontentloaded' });
      await fragment('getting-started');
      const text = (await page.locator('article').innerText()).replace(/\s+/g, ' ');
      for (const phrase of ['fresh clone alone is not isolation', 'read /etc/pulse/.env',
        'authentication bypass must never face an untrusted network', 'without takeover',
        'refuses to replace it', 'do not prevent configuration-file loading',
        'choose unused', 'not the safe reproduction entrypoint']) assert.ok(text.includes(phrase), phrase);
      await capture('isolation', page.locator('#getting-started'));
      const recipeElement = page.locator('article pre').filter({ hasText: 'HOT_DEV_USE_PRO=false' });
      assert.equal((await recipeElement.locator('code').innerText()).trim(), recipe);
      await capture('recipe', page.locator('#hot-reload-dev-loop'));
      const geometry = await recipeElement.evaluate(pre => {
        pre.scrollLeft = pre.scrollWidth;
        const measured = { width: pre.clientWidth, content: pre.scrollWidth, offset: pre.scrollLeft };
        pre.scrollLeft = 0;
        return measured;
      });
      if (geometry.content > geometry.width) assert.ok(geometry.offset > 0);
      await page.goto(`${origin}/docs/CONTRIBUTING#frontend-workflow`, { waitUntil: 'domcontentloaded' });
      await fragment('frontend-workflow');
      const link = page.locator('article a[href="#hot-reload-dev-loop"]');
      if (state.width < 500) await link.tap();
      else { await link.focus(); await page.keyboard.press('Enter'); }
      await page.waitForURL('**/docs/CONTRIBUTING#hot-reload-dev-loop');
      await fragment('hot-reload-dev-loop');
      await page.reload({ waitUntil: 'domcontentloaded' });
      await fragment('hot-reload-dev-loop');
      const response = await page.request.get(`${origin}/docs/CONTRIBUTING.md`);
      assert.equal(response.status(), 200);
      assert.equal(hash(await response.body()), binding.sources['public/docs/CONTRIBUTING.md']);
      await page.goto(`${origin}/docs/ISSUE_TRIAGE#intake-contract`, { waitUntil: 'domcontentloaded' });
      await fragment('intake-contract');
      const titleGuidance = page.locator('article p').filter({ hasText: 'Without a body declaration' });
      assert.ok((await titleGuidance.innerText()).includes('platform, agent or chart version'));
      await capture('intake-title-evidence', titleGuidance);
      const triageResponse = await page.request.get(`${origin}/docs/ISSUE_TRIAGE.md`);
      assert.equal(triageResponse.status(), 200);
      assert.equal(hash(await triageResponse.body()), binding.sources['public/docs/ISSUE_TRIAGE.md']);
      result.cases.push({ ...state, browser: browser.version(), geometry,
        exactRenderedRecipe: true, exactFetchedGuide: true, fragmentFocusAndReload: true,
        restoredIntakeGuidance: true, interaction: state.width < 500 ? 'touch' : 'keyboard',
        noDocumentOverflow: true });
      await context.close();
    }
    assert.deepEqual(result.pageErrors, []);
    assert.ok(result.requests.every(req => req.method === 'GET' && new URL(req.url).origin === origin
      && !new URL(req.url).pathname.startsWith('/api/')));
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
