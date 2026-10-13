const fs = require('node:fs');
const path = require('node:path');
const http = require('node:http');
const crypto = require('node:crypto');
const assert = require('node:assert/strict');
const { chromium, webkit } = require('playwright');
const root = '/workspace';
const frontend = path.join(root, 'frontend-modern');
const evidence = path.join(root, 'tmp/doc-command-help');
const preview = path.join(evidence, 'static');
const hash = bytes => crypto.createHash('sha256').update(bytes).digest('hex');
const bindingBytes = fs.readFileSync(path.join(preview, 'binding.json'));
const binding = JSON.parse(bindingBytes);
const result = { source: binding.source_sha, binding_sha256: hash(bindingBytes),
  cases: [], captures: [], pageErrors: [], requests: [], served: [] };
(async () => {
  let server, browser;
  try {
    for (const [file, expected] of Object.entries(binding.sources))
      assert.equal(hash(fs.readFileSync(path.join(frontend, file))), expected, file);
    for (const [file, expected] of Object.entries(binding.artifacts))
      assert.equal(hash(fs.readFileSync(path.join(preview, file))), expected, file);
    result.source_inputs = Object.keys(binding.sources).length;
    assert.equal(require('playwright/package.json').version, '1.56.1');
    result.playwright = '1.56.1';
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
      res.writeHead(200, { 'Content-Type': type, 'Cache-Control': 'no-store' }); res.end(bytes);
    });
    await new Promise(resolve => server.listen(0, '127.0.0.1', resolve));
    const origin = `http://127.0.0.1:${server.address().port}`;
    for (const state of [
      { name: 'desktop', width: 1280, height: 900, dark: false, engine: 'chromium' },
      { name: 'phone', width: 375, height: 812, dark: true, engine: 'webkit' },
    ]) {
      browser = state.engine === 'webkit' ? await webkit.launch({ headless: true }) :
        await chromium.launch({ headless: true, channel: 'chromium', args: ['--no-sandbox'] });
      const browserVersion = browser.version();
      const context = await browser.newContext({ viewport: { width: state.width, height: state.height },
        isMobile: state.width < 500, hasTouch: state.width < 500 });
      const page = await context.newPage();
      page.on('pageerror', error => result.pageErrors.push(error.message));
      page.on('request', req => result.requests.push({ path: new URL(req.url()).pathname, method: req.method() }));
      await page.addInitScript(dark => {
        document.addEventListener('DOMContentLoaded', () => document.documentElement.classList.toggle('dark', dark));
      }, state.dark);
      const activate = async locator => {
        if (state.width < 500) await locator.tap();
        else { await locator.focus(); await page.keyboard.press('Enter'); }
      };
      const capture = async (name, locator) => {
        await locator.evaluate(el => window.scrollBy(0, el.getBoundingClientRect().top - 48));
        await page.evaluate(() => new Promise(resolve => requestAnimationFrame(() => requestAnimationFrame(resolve))));
        assert.equal(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth), true, name);
        const file = `docker-sizing-${state.name}-${name}.png`;
        await page.screenshot({ path: path.join(evidence, file) });
        result.captures.push({ file, sha256: hash(fs.readFileSync(path.join(evidence, file))), state });
      };
      await page.goto(`${origin}/docs/DOCKER#healthcheck-and-resource-sizing`);
      await page.locator('#healthcheck-and-resource-sizing').waitFor();
      const sizing = await page.locator('#healthcheck-and-resource-sizing').evaluate(heading => {
        const section = document.createElement('section');
        for (let node = heading.nextElementSibling; node && !/^H[123]$/.test(node.tagName); node = node.nextElementSibling)
          section.append(node.cloneNode(true));
        return { text: section.innerText || section.textContent, commands: [...section.querySelectorAll('pre code')].map(el => el.textContent) };
      });
      assert.equal(sizing.commands.length, 1);
      assert.match(sizing.commands[0], /services:\s+\n?\s*pulse:\s+\n?\s*healthcheck:/);
      assert.doesNotMatch(sizing.commands[0], /\b(deploy|resources|limits|memory|cpus|mem_limit):/);
      assert.match(sizing.commands[0], /http:\/\/localhost:7655\/api\/health/);
      for (const boundary of ['not a complete', 'not restart the container', 'no universal CPU or memory limit',
        'not an instruction to remove an existing limit', 'current resource policy', 'out-of-memory kill',
        'delay polling and notification delivery', 'Neither repairs an increased baseline',
        'not a safe container limit by itself', 'even when memory no longer grows'])
        assert.equal(sizing.text.includes(boundary), true, boundary);
      await capture('healthcheck', page.locator('#healthcheck-and-resource-sizing'));
      const limits = page.locator('article p').filter({ hasText: 'There is no universal CPU or memory limit for Pulse.' });
      assert.equal(await limits.count(), 1);
      await capture('limits', limits);
      await page.evaluate(() => window.__pi_docs_shell_marker = 'unchanged-shell');
      for (const target of ['excessive-cpu-writes-or-database-growth', 'memory-use-keeps-growing']) {
        await activate(page.locator(`article a[href="/docs/TROUBLESHOOTING#${target}"]`).first());
        await page.waitForURL(`**/docs/TROUBLESHOOTING#${target}`);
        await page.locator(`#${target}`).waitFor();
        assert.equal(await page.locator(`#${target}`).evaluate(el => {
          const box = el.getBoundingClientRect(); return box.top >= 0 && box.top < window.innerHeight;
        }), true);
        assert.equal(await page.evaluate(() => window.__pi_docs_shell_marker), 'unchanged-shell');
        await page.goBack(); await page.waitForURL('**/docs/DOCKER#healthcheck-and-resource-sizing');
        await page.locator('#healthcheck-and-resource-sizing').waitFor();
      }
      for (const name of ['DOCKER', 'TROUBLESHOOTING']) {
        const response = await page.request.get(`${origin}/docs/${name}.md`);
        assert.equal(response.status(), 200);
        assert.equal(hash(await response.body()), binding.sources[`public/docs/${name}.md`]);
      }
      result.cases.push({ ...state, browserVersion, codeHasNoResourceCaps: true,
        healthcheckPreserved: true, policyPreserved: true, diagnosticLinksAndHistory: true,
        activation: state.width < 500 ? 'touch' : 'keyboard', servedBytesMatched: true, horizontalOverflow: false });
      await context.close(); await browser.close(); browser = null;
    }
    assert.deepEqual(result.pageErrors, []);
    assert.equal(result.requests.some(req => req.method !== 'GET' || req.path.startsWith('/api/') || req.path.startsWith('/ws')), false);
    result.result = 'passed';
  } catch (error) {
    result.result = 'failed'; result.error = String(error.stack || error); process.exitCode = 1;
  } finally {
    if (browser) await browser.close(); result.browser_closed = true;
    if (server) { await new Promise(resolve => server.close(resolve)); result.server_closed = true; }
    fs.writeFileSync(path.join(evidence, 'docker-sizing-browser-result.json'), JSON.stringify(result, null, 2) + '\n');
    console.log(JSON.stringify({ source: result.source, result: result.result, cases: result.cases.length,
      captures: result.captures.length, browser_closed: result.browser_closed, server_closed: result.server_closed, error: result.error }));
  }
})().catch(error => { console.error(error); process.exitCode = 1; });
