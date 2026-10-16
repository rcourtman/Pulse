// Real Docs consumer and exact private Docker authentication guidance.
// No credential, administrative API or installed-service action is exercised.
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const http = require('node:http');
const crypto = require('node:crypto');
const { chromium } = require('playwright');

const root = '/workspace';
const frontend = path.join(root, 'frontend-modern');
const artifacts = path.join(root, 'tmp/configuration-auth-help');
const preview = path.join(root, 'tmp/doc-command-help/static');
const hash = bytes => crypto.createHash('sha256').update(bytes).digest('hex');
const bindingBytes = fs.readFileSync(path.join(preview, 'binding.json'));
const binding = JSON.parse(bindingBytes);
const guide = fs.readFileSync(path.join(frontend, 'public/docs/CONFIGURATION.md'), 'utf8');
const section = guide.split('### Private Docker authentication file\n')[1].split('<details>')[0];
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
      await page.goto(`${origin}/docs/CONFIGURATION#private-docker-authentication-file`,
        { waitUntil: 'domcontentloaded' });
      await page.locator('#private-docker-authentication-file').waitFor();
      await page.waitForFunction(() => document.activeElement?.id === 'private-docker-authentication-file');
      await capture('boundary', page.locator('#private-docker-authentication-file'));
      const article = await page.locator('article').innerText();
      for (const phrase of ["credential's owner, not with `sudo`".replaceAll('`', ''),
        'non-regular or foreign-owned credential files', 'launch block checks again',
        'do not lock the path against concurrent changes', "not out of Docker's",
        'this new-container example is not an upgrade procedure'])
        assert.ok(article.includes(phrase), phrase);
      const blocks = page.locator('article pre code');
      const actual = (await blocks.allTextContents()).map(text => text.trim());
      for (const recipe of recipes) assert.ok(actual.includes(recipe), 'exact copied recipe');
      const preparation = page.locator('article pre').filter({ hasText: 'vi "$auth_file"' });
      const launch = page.locator('article pre').filter({ hasText: 'docker run -d' });
      for (const block of [preparation, launch]) {
        const text = await block.innerText();
        for (const guard of ['[ -L "$HOME/.config" ]', '[ ! -O "$directory" ]',
          '[ ! -f "$auth_file" ]', '[ ! -O "$auth_file" ]', 'chmod 600 "$auth_file"'])
          assert.ok(text.includes(guard), guard);
      }
      assert.ok((await launch.innerText()).includes('[ ! -r "$auth_file" ]'));
      assert.ok((await launch.innerText()).includes('--env-file "$auth_file"'));
      await capture('preparation', preparation);
      await capture('launch', launch);
      // Long commands scroll inside their own block, not the whole document.
      await launch.evaluate(pre => { pre.scrollLeft = pre.scrollWidth; });
      assert.ok(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1));
      await capture('launch-limits', page.locator('article p').filter({ hasText: 'not out of Docker' }));
      // Follow an actual shipped link and return; no hand-rendered article.
      const bootstrap = page.locator('article a[href="/docs/INSTALL#step-1-get-the-token"]').first();
      await activate(bootstrap);
      await page.waitForURL('**/docs/INSTALL#step-1-get-the-token');
      await page.waitForFunction(() => document.activeElement?.id === 'step-1-get-the-token');
      await page.goBack({ waitUntil: 'domcontentloaded' });
      await page.waitForURL('**/docs/CONFIGURATION#private-docker-authentication-file');
      await page.waitForFunction(() => document.activeElement?.id === 'private-docker-authentication-file');
      await page.reload({ waitUntil: 'domcontentloaded' });
      await page.waitForFunction(() => document.activeElement?.id === 'private-docker-authentication-file');
      for (const name of ['CONFIGURATION', 'INSTALL']) {
        const response = await page.request.get(`${origin}/docs/${name}.md`);
        assert.equal(response.status(), 200);
        assert.equal(hash(await response.body()), binding.sources[`public/docs/${name}.md`]);
      }
      result.cases.push({ ...state, exactRenderedRecipes: 2, bootstrapLinkAndHistoryBack: true,
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
