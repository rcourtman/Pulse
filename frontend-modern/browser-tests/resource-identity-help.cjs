// Actual shipped Docs/router/Markdown/styles inside a fixture shell.
// No API request, customer state or MQTT server is used.
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const { createHash } = require('node:crypto');
const http = require('node:http');
const { chromium, webkit } = require('playwright');

const workspace = process.cwd();
const root = path.join(workspace, 'frontend-modern');
const output = path.join(workspace, 'tmp/api-identity');
const digest = bytes => createHash('sha256').update(bytes).digest('hex');
const preview = path.join(workspace, 'tmp/doc-command-help/static');
const binding = JSON.parse(fs.readFileSync(path.join(preview, 'binding.json')));
const result = { result: 'incomplete', source: binding, cases: [], captures: [], errors: [], cleanup: {} };
fs.mkdirSync(output, { recursive: true });

(async () => {
  let browser, server;
  try {
    result.playwright = require('playwright/package.json').version;
    assert.equal(result.playwright, '1.56.1');
    assert.equal(result.playwright, JSON.parse(fs.readFileSync(path.join(workspace,
      'tests/integration/package-lock.json'))).packages['node_modules/@playwright/test'].version);
    for (const [file, hash] of Object.entries(binding.sources))
      assert.equal(digest(fs.readFileSync(path.join(root, file))), hash, file);
    for (const [file, hash] of Object.entries(binding.artifacts))
      assert.equal(digest(fs.readFileSync(path.join(preview, file))), hash, file);
    assert.equal(binding.artifacts['docs/API.md'], digest(fs.readFileSync(path.join(workspace, 'docs/API.md'))));
    server = http.createServer((req, res) => {
      assert.equal(req.method, 'GET');
      const pathname = new URL(req.url, 'http://fixture.invalid').pathname;
      const file = pathname.startsWith('/docs/') && !pathname.endsWith('.md')
        ? 'browser-tests/doc-command-help.html' : pathname.slice(1);
      if (!Object.hasOwn(binding.artifacts, file)) { res.writeHead(404); res.end(); return; }
      const bytes = fs.readFileSync(path.join(preview, file));
      assert.equal(digest(bytes), binding.artifacts[file]);
      const type = ({ '.md': 'text/markdown', '.css': 'text/css', '.js': 'text/javascript', '.html': 'text/html' })[path.extname(file)];
      res.writeHead(200, { 'Content-Type': `${type}; charset=utf-8`, 'Cache-Control': 'no-store' }); res.end(bytes);
    });
    await new Promise((resolve, reject) => { server.once('error', reject); server.listen(0, '127.0.0.1', resolve); });
    const origin = `http://127.0.0.1:${server.address().port}`;
    for (const [name, engine, width, dark] of [
      ['desktop', chromium, 1360, false], ['phone', webkit, 320, true],
    ]) {
      browser = await engine.launch(engine === chromium
        ? { headless: true, channel: 'chromium', args: ['--no-sandbox'] } : { headless: true });
      const context = await browser.newContext({ viewport: { width, height: 960 },
        isMobile: width < 500, hasTouch: width < 500, locale: 'en-GB' });
      const page = await context.newPage(); page.setDefaultTimeout(15000);
      const record = { name, width, height: 960, dark, browserVersion: browser.version(), api: [], checks: [] };
      result.cases.push(record);
      page.on('pageerror', e => result.errors.push(e.message));
      page.on('console', m => { if (m.type() === 'error') result.errors.push(m.text()); });
      page.on('response', r => { if (!r.ok()) result.errors.push(`HTTP ${r.status()}: ${r.url()}`); });
      await page.route('**/*', route => {
        const url = new URL(route.request().url());
        assert.equal(url.origin, origin); assert.equal(route.request().method(), 'GET');
        assert(!url.pathname.startsWith('/api/'), 'no live API requests');
        return route.continue();
      });
      await page.goto(`${origin}/docs/API#resource-ids-and-integration-labels`,
        { waitUntil: 'domcontentloaded', timeout: 60000 });
      await page.getByRole('heading', { name: 'Resource IDs and integration labels', exact: true }).waitFor();
      await page.evaluate(dark => {
        document.documentElement.classList.toggle('dark', dark);
        location.hash = '#resource-ids-and-integration-labels';
      }, dark);
      const focused = id => page.waitForFunction(id => document.activeElement?.id === id, id);
      await focused('resource-ids-and-integration-labels');
      const section = await page.locator('#resource-ids-and-integration-labels').evaluate(heading => {
        let text = '';
        for (let node = heading.nextElementSibling; node && node.tagName !== 'H4'; node = node.nextElementSibling)
          text += ` ${node.textContent}`;
        return text.replace(/\s+/g, ' ');
      });
      for (const phrase of ['opaque value', 'do not split', 'resource collection',
        'not interchangeable', 'one path segment', 'edit/remove guest_metadata.json',
        'without repeating the connection change', 'private names out of public reports'])
        assert(section.includes(phrase), phrase);
      const capture = async (suffix, locator) => {
        await locator.evaluate(e => e.scrollIntoView({ block: 'start', behavior: 'instant' }));
        await page.evaluate(() => new Promise(r => requestAnimationFrame(() => requestAnimationFrame(r))));
        assert(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1));
        const file = `${name}-${suffix}.png`;
        await page.screenshot({ path: path.join(output, file) });
        result.captures.push({ file, sha256: digest(fs.readFileSync(path.join(output, file))) });
      };
      await capture('identity', page.locator('#resource-ids-and-integration-labels'));
      await capture('relationships', page.locator('article p').filter({ hasText: 'A storage row can represent' }));
      const link = page.locator('article a[href="/docs/API#unified-resources"]').first();
      if (width < 500) await link.tap(); else { await link.focus(); await page.keyboard.press('Enter'); }
      await focused('unified-resources');
      assert.equal(new URL(page.url()).hash, '#unified-resources');
      await page.goBack({ waitUntil: 'domcontentloaded' });
      await focused('resource-ids-and-integration-labels');
      await page.reload({ waitUntil: 'domcontentloaded' });
      await focused('resource-ids-and-integration-labels');
      record.checks.push('Exact source and shipped guide match; identity/context/safety text renders',
        'No page overflow at desktop and 320px; context table and warning inspected',
        'Keyboard/touch link, fragment focus, browser Back and reload work');
      await context.close(); await browser.close(); browser = null;
    }
    assert.deepEqual(result.errors, []); result.result = 'passed';
  } catch (error) { result.failure = error.stack; process.exitCode = 1; }
  finally {
    if (browser) await browser.close(); result.cleanup.browserClosed = true;
    if (server) await new Promise((resolve, reject) => {
      server.close(error => error ? reject(error) : resolve()); server.closeAllConnections();
    }); result.cleanup.serverClosed = true;
    result.verifiedAt = new Date().toISOString();
    result.limits = 'Shipped documentation only; no MQTT/HA installation, API collection repair or metadata migration acceptance.';
    fs.writeFileSync(path.join(output, 'result.json'), JSON.stringify(result, null, 2) + '\n');
    console.log(JSON.stringify({ source: binding.source_sha, result: result.result,
      cases: result.cases.length, captures: result.captures.length,
      errors: result.errors, cleanup: result.cleanup, failure: result.failure }));
  }
})();
