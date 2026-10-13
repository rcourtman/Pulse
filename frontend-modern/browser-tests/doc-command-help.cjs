const fs = require('node:fs');
const path = require('node:path');
const http = require('node:http');
const crypto = require('node:crypto');
const assert = require('node:assert/strict');
const { chromium } = require('playwright');
const root = '/workspace';
const frontend = path.join(root, 'frontend-modern');
const evidence = path.join(root, 'tmp/doc-command-help');
const preview = path.join(evidence, 'static');
const hash = bytes => crypto.createHash('sha256').update(bytes).digest('hex');
const bindingBytes = fs.readFileSync(path.join(preview, 'binding.json'));
const binding = JSON.parse(bindingBytes);
const result = { source: binding.source_sha, binding_sha256: hash(bindingBytes),
  cases: [], captures: [], pageErrors: [], requests: [], served: [] };
const failure = 'copied-agent-command-fails-before-installation';
(async () => {
  let server, browser;
  try {
    for (const [file, expected] of Object.entries(binding.sources))
      assert.equal(hash(fs.readFileSync(path.join(frontend, file))), expected, file);
    for (const [file, expected] of Object.entries(binding.artifacts))
      assert.equal(hash(fs.readFileSync(path.join(preview, file))), expected, file);
    result.source_inputs = Object.keys(binding.sources).length;
    server = http.createServer((req, res) => {
      const route = new URL(req.url, 'http://fixture.invalid').pathname;
      let asset = route.slice(1);
      if (route.startsWith('/docs/') && !route.endsWith('.md'))
        asset = 'browser-tests/doc-command-help.html';
      if (!Object.hasOwn(binding.artifacts, asset)) { res.writeHead(404); res.end(); return; }
      const bytes = fs.readFileSync(path.join(preview, asset));
      assert.equal(hash(bytes), binding.artifacts[asset]);
      result.served.push({route, asset, sha256: hash(bytes)});
      const type = asset.endsWith('.md') ? 'text/markdown' : asset.endsWith('.css') ? 'text/css' :
        asset.endsWith('.js') ? 'text/javascript' : 'text/html';
      res.writeHead(200, {'Content-Type': type, 'Cache-Control': 'no-store'}); res.end(bytes);
    });
    await new Promise(resolve => server.listen(0, '127.0.0.1', resolve));
    const origin = `http://127.0.0.1:${server.address().port}`;
    browser = await chromium.launch({headless:true, channel:'chromium', args:['--no-sandbox']});
    result.browser = browser.version(); result.playwright = require('playwright/package.json').version;
    assert.equal(result.playwright, '1.56.1');
    for (const state of [
      { name: 'desktop-light', width: 1360, height: 960, dark: false },
      { name: 'phone-dark', width: 390, height: 844, dark: true },
    ]) {
      const context = await browser.newContext({ viewport: { width: state.width, height: state.height },
        isMobile: state.width < 500, hasTouch: state.width < 500 });
      const page = await context.newPage();
      page.on('pageerror', error => result.pageErrors.push(error.message));
      page.on('request', req => result.requests.push(new URL(req.url()).pathname));
      await page.addInitScript(dark => {
        document.addEventListener('DOMContentLoaded', () => document.documentElement.classList.toggle('dark', dark));
      }, state.dark);
      const capture = async (name) => {
        await page.evaluate(() => new Promise(resolve => requestAnimationFrame(() => requestAnimationFrame(resolve))));
        assert.equal(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth), true, name);
        const file = `${state.name}-${name}.png`;
        await page.screenshot({ path: path.join(evidence, file) });
        result.captures.push({ file, sha256: hash(fs.readFileSync(path.join(evidence, file))), state });
      };
      const jump = async (selector) => {
        await page.locator(selector).evaluate(el => window.scrollBy(0, el.getBoundingClientRect().top - 64));
      };
      const activate = async (locator) => {
        if (state.width < 500) await locator.tap();
        else { await locator.focus(); await page.keyboard.press('Enter'); }
      };
      await page.goto(`${origin}/docs/FAQ#how-do-i-add-a-node`);
      await page.locator('article #how-do-i-add-a-node').waitFor();
      await jump('#how-do-i-add-a-node');
      await capture('faq');
      assert.match(await page.locator('article').innerText(), /token only at its private prompt/);
      const failureLink = page.locator(`article a[href="/docs/UNIFIED_AGENT#${failure}"]`);
      await activate(failureLink);
      await page.waitForURL(`**/docs/UNIFIED_AGENT#${failure}`);
      await page.locator(`#${failure}`).waitFor();
      await jump(`#${failure}`);
      await capture('failure-stage');
      assert.match(await page.locator('article').innerText(), /A real terminal can still produce this error/);
      const stop = page.locator('article p').filter({ hasText: 'Stop rather than bypass the private prompt.' });
      assert.equal(await stop.count(), 1);
      await stop.evaluate(el => window.scrollBy(0, el.getBoundingClientRect().top - 64));
      await capture('failure-safety');
      await page.evaluate(() => window.__pi_docs_shell_marker = 'same-shell');
      const prep = page.locator(`#${failure} ~ p a[href="#private-file-installation-linux-macos-and-nas"]`).first();
      await activate(prep);
      await page.waitForURL('**#private-file-installation-linux-macos-and-nas');
      assert.equal(await page.evaluate(() => window.__pi_docs_shell_marker), 'same-shell');
      assert.equal(await page.locator('#private-file-installation-linux-macos-and-nas').evaluate(el => {
        const box = el.getBoundingClientRect(); return box.top >= 0 && box.top < window.innerHeight;
      }), true);
      await page.goBack();
      await page.waitForURL(`**/docs/UNIFIED_AGENT#${failure}`);
      await page.goto(`${origin}/docs/TROUBLESHOOTING#choose-the-affected-workflow`);
      await page.locator('article #choose-the-affected-workflow').waitFor();
      await page.locator('article li').filter({ hasText: 'Copied agent install or removal command failed:' })
        .evaluate(el => window.scrollBy(0, el.getBoundingClientRect().top - 64));
      await capture('troubleshooting');
      await activate(page.locator(`article a[href="/docs/UNIFIED_AGENT#${failure}"]`));
      await page.waitForURL(`**/docs/UNIFIED_AGENT#${failure}`);
      await page.locator(`#${failure}`).waitFor();
      for (const id of ['agent-lookup-returns-http-401-or-403', 'auto-update', 'uninstall']) {
        const link = page.locator(`#${failure} ~ p a[href="#${id}"]`).first();
        await activate(link); await page.waitForURL(`**#${id}`);
        await page.locator(`#${id}`).waitFor();
        await page.goBack(); await page.waitForURL(`**#${failure}`);
      }
      for (const name of ['FAQ', 'UNIFIED_AGENT', 'TROUBLESHOOTING']) {
        const response = await page.request.get(`${origin}/docs/${name}.md`);
        assert.equal(response.status(), 200);
        assert.equal(hash(await response.body()), binding.sources[`public/docs/${name}.md`]);
      }
      result.cases.push({ ...state, interactions: ['FAQ to failure', 'private-file anchor and history back',
        'Troubleshooting to failure', 'reporting/update/removal anchors and history back'],
        method: state.width < 500 ? 'touch tap' : 'keyboard Enter',
        horizontalOverflow: false, documentBytesMatched: true, sameShell: true });
      await context.close();
    }
    assert.deepEqual(result.pageErrors, []);
    assert.equal(result.requests.some(url => url.startsWith('/api/') || url.startsWith('/ws')), false);
    result.result = 'passed';
  } catch (error) {
    result.result = 'failed'; result.error = String(error.stack || error);
    process.exitCode = 1;
  } finally {
    if (browser) { await browser.close(); result.browser_closed = true; }
    if (server) { await new Promise(resolve => server.close(resolve)); result.server_closed = true; }
    fs.writeFileSync(path.join(evidence, 'browser-result.json'), JSON.stringify(result, null, 2) + '\n');
    // Do not send input manifests, logs or screenshot bytes through the capture.
    console.log(JSON.stringify({source: result.source, result: result.result,
      cases: result.cases.length, captures: result.captures.length,
      source_inputs: result.source_inputs, browser_closed: result.browser_closed,
      server_closed: result.server_closed, error: result.error}));
  }
})().catch(error => { console.error(error); process.exitCode = 1; });
