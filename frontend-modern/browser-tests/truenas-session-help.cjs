// Shipped help/renderer/router/styles only: no appliance, credential or poll.
const fs = require('node:fs');
const path = require('node:path');
const http = require('node:http');
const crypto = require('node:crypto');
const assert = require('node:assert/strict');
const { chromium } = require('playwright');
const root = '/workspace/frontend-modern';
const preview = '/workspace/tmp/doc-command-help/static';
const output = '/workspace/tmp/truenas-session-help';
const digest = bytes => crypto.createHash('sha256').update(bytes).digest('hex');
(async () => {
  fs.mkdirSync(output, { recursive: true });
  const binding = JSON.parse(fs.readFileSync(path.join(preview, 'binding.json')));
  const result = { result: 'incomplete', source_sha: binding.source_sha,
    playwright: require('playwright/package.json').version,
    scope: 'shipped TrueNAS help, not appliance recovery or the Pause API',
    cases: [], captures: [], errors: [], cleanup: {} };
  let browser, server;
  try {
    assert.equal(result.playwright, JSON.parse(fs.readFileSync('/workspace/tests/integration/package-lock.json'))
      .packages['node_modules/@playwright/test'].version);
    for (const [file, hash] of Object.entries(binding.sources))
      assert.equal(digest(fs.readFileSync(path.join(root, file))), hash, file);
    const assets = new Map();
    for (const [file, hash] of Object.entries(binding.artifacts)) {
      const bytes = fs.readFileSync(path.join(preview, file));
      assert.equal(digest(bytes), hash, file); assets.set(file, bytes);
    }
    assert.equal(binding.artifacts['docs/TRUENAS.md'], digest(fs.readFileSync('/workspace/docs/TRUENAS.md')));
    server = http.createServer((req, res) => {
      assert.equal(req.method, 'GET');
      const pathname = new URL(req.url, 'http://fixture.invalid').pathname;
      const file = pathname.startsWith('/docs/') && !pathname.endsWith('.md')
        ? 'browser-tests/doc-command-help.html' : pathname.slice(1);
      if (!assets.has(file)) { res.writeHead(404); res.end(); return; }
      const type = ({ '.md': 'text/markdown', '.css': 'text/css', '.js': 'text/javascript', '.html': 'text/html' })[path.extname(file)];
      res.writeHead(200, { 'Content-Type': `${type}; charset=utf-8`, 'Cache-Control': 'no-store' }); res.end(assets.get(file));
    });
    await new Promise((resolve, reject) => { server.once('error', reject); server.listen(0, '127.0.0.1', resolve); });
    const origin = `http://127.0.0.1:${server.address().port}`;
    for (const [name, width, dark] of [['desktop', 1360, false], ['phone', 390, true]]) {
      browser = await chromium.launch({ headless: true, channel: 'chromium', args: ['--no-sandbox'] });
      const context = await browser.newContext({ viewport: { width, height: 900 },
        isMobile: width < 500, hasTouch: width < 500, locale: 'en-GB' });
      const page = await context.newPage(); page.setDefaultTimeout(12000);
      const record = { name, width, height: 900, dark, browser: browser.version(), checks: [] }; result.cases.push(record);
      page.on('pageerror', error => result.errors.push(error.message));
      page.on('console', message => { if (message.type() === 'error') result.errors.push(message.text()); });
      page.on('response', response => { if (!response.ok()) result.errors.push(`HTTP ${response.status()}`); });
      await page.route('**/*', route => {
        const url = new URL(route.request().url());
        assert.equal(url.origin, origin); assert(!url.pathname.startsWith('/api/')); assert.equal(route.request().method(), 'GET');
        return route.continue();
      });
      await page.addInitScript(dark => document.addEventListener('DOMContentLoaded', () =>
        document.documentElement.classList.toggle('dark', dark)), dark);
      const atFragment = id => page.waitForFunction(id => {
        const heading = document.getElementById(id);
        return heading && document.activeElement === heading && heading.getBoundingClientRect().top >= 0
          && heading.getBoundingClientRect().top < innerHeight;
      }, id);
      const activate = async link => {
        if (width < 500) await link.tap(); else { await link.focus(); await page.keyboard.press('Enter'); }
      };
      const capture = async suffix => {
        await page.evaluate(() => new Promise(resolve => requestAnimationFrame(() => requestAnimationFrame(resolve))));
        assert(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth), 'page overflow');
        const file = `${name}-${suffix}.png`; await page.screenshot({ path: path.join(output, file) });
        result.captures.push({ file, sha256: digest(fs.readFileSync(path.join(output, file))) });
      };
      await page.goto(`${origin}/docs/TRUENAS#stale-truenas-data`, { waitUntil: 'domcontentloaded' });
      await atFragment('stale-truenas-data');
      await page.evaluate(() => window.__nasHelpShell = 'same-shell');
      await activate(page.getByRole('link', { name: 'repeated sign-ins', exact: true }));
      await atFragment('repeated-sign-ins-or-unexpectedly-high-traffic'); await capture('sessions');
      const text = (await page.locator('article').innerText()).replace(/\s+/g, ' ');
      for (const phrase of ['direct connection too', 'not an authentication verdict',
        'not a measured daily total', 'separate connection and can add a login itself',
        'rather than restarting Pulse', 'not an immediate cancellation receipt',
        'does not stop a separate Pulse Agent', 'removes this API monitoring coverage',
        'Do not click the toggle again blindly', 'only for the connection you deliberately paused',
        'requires a Pulse restart, unlike the scoped pause']) assert(text.includes(phrase), phrase);
      await activate(page.getByRole('link', { name: 'scoped pause', exact: true }));
      await atFragment('disabling-truenas-integration'); await capture('pause');
      await activate(page.getByRole('link', { name: 'existing read-only view', exact: true }));
      await atFragment('stale-truenas-data');
      await page.goBack(); await atFragment('disabling-truenas-integration');
      await page.reload({ waitUntil: 'domcontentloaded' }); await atFragment('disabling-truenas-integration');
      await page.evaluate(() => window.__nasHelpShell = 'after-reload');
      const safety = page.getByRole('link', { name: 'backup safety', exact: true });
      await safety.scrollIntoViewIfNeeded(); await capture('pause-limits');
      await activate(safety); await page.waitForURL('**/docs/VM_DISK_MONITORING#backup-safety');
      await atFragment('backup-safety'); assert.equal(await page.evaluate(() => window.__nasHelpShell), 'after-reload');
      await page.goBack(); await atFragment('disabling-truenas-integration');
      record.checks.push('shipped bytes and actual Markdown; session diagnosis and pause limitations',
        'keyboard/touch churn, pause, readback and safety links; focused destinations; Back/reload',
        '1360/390 viewport and no horizontal overflow; no external/API/mutation requests');
      await context.close(); await browser.close(); browser = null;
    }
    assert.deepEqual(result.errors, []); result.result = 'passed';
  } catch (error) { result.failure = error.stack; throw error; }
  finally {
    if (browser) await browser.close(); result.cleanup.browser_closed = true;
    if (server) await new Promise((resolve, reject) => {
      server.close(error => error ? reject(error) : resolve()); server.closeAllConnections();
    }); result.cleanup.server_closed = true;
    fs.writeFileSync(path.join(output, 'result.json'), JSON.stringify(result, null, 2) + '\n');
    console.log(JSON.stringify(result));
  }
})().catch(error => { console.error(error); process.exitCode = 1; });
