// Read-only consuming proof for #3099. No scrollLeft setter or focus patch.
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const http = require('node:http');
const { createHash } = require('node:crypto');
const { chromium, webkit } = require('playwright');
const workspace = process.cwd();
const root = path.join(workspace, 'frontend-modern');
const preview = path.join(workspace, 'tmp/doc-command-help/static');
const output = path.join(workspace, 'tmp/docs-table-keyboard');
const hash = bytes => createHash('sha256').update(bytes).digest('hex');
const binding = JSON.parse(fs.readFileSync(path.join(preview, 'binding.json')));
// Only this additional document is synthetic. It exercises author-supplied
// attributes and native link order through the real sanitizer/router/renderer.
const linkFixture = '# Keyboard fixture\n\n' +
  '<div data-doc-table-scroll tabindex="-1" role="button" aria-hidden="true" aria-label="Forged">' +
  '<table aria-hidden="true"><thead><tr><th role="presentation">Destination</th></tr></thead>' +
  '<tbody><tr><td><a href="CONFIGURATION.md#back-up-recoverable-state">Recovery checklist</a></td></tr></tbody></table></div>\n\n' +
  '[After the table](README.md)';
const result = { result: 'incomplete', source_sha: binding.source_sha,
  binding_sha256: hash(fs.readFileSync(path.join(preview, 'binding.json'))),
  fixture_sha256: hash(linkFixture), cases: [], captures: [], errors: [], requests: [], cleanup: {} };
fs.mkdirSync(output, { recursive: true });
(async () => {
  let browser, server;
  try {
    result.playwright = require('playwright/package.json').version;
    assert.equal(result.playwright, JSON.parse(fs.readFileSync(path.join(workspace,
      'tests/integration/package-lock.json'))).packages['node_modules/@playwright/test'].version);
    for (const [file, expected] of Object.entries(binding.sources))
      assert.equal(hash(fs.readFileSync(path.join(root, file))), expected, file);
    const assets = new Map();
    for (const [file, expected] of Object.entries(binding.artifacts)) {
      const bytes = fs.readFileSync(path.join(preview, file));
      assert.equal(hash(bytes), expected, file); assets.set(file, bytes);
    }
    assert.equal(binding.artifacts['docs/CONFIGURATION.md'], hash(fs.readFileSync(path.join(workspace, 'docs/CONFIGURATION.md'))));
    server = http.createServer((req, res) => {
      const pathname = new URL(req.url, 'http://fixture.invalid').pathname;
      result.requests.push({ method: req.method, path: pathname });
      assert.equal(req.method, 'GET'); assert(!pathname.startsWith('/api/'));
      if (pathname === '/docs/KEYBOARD_FIXTURE.md') {
        res.writeHead(200, { 'Content-Type': 'text/markdown', 'Cache-Control': 'no-store' }); res.end(linkFixture); return;
      }
      const file = pathname.startsWith('/docs/') && !pathname.endsWith('.md')
        ? 'browser-tests/doc-command-help.html' : pathname.slice(1);
      if (!assets.has(file)) { res.writeHead(404); res.end(); return; }
      const type = ({ '.md': 'text/markdown', '.css': 'text/css', '.js': 'text/javascript', '.html': 'text/html' })[path.extname(file)];
      res.writeHead(200, { 'Content-Type': `${type}; charset=utf-8`, 'Cache-Control': 'no-store' }); res.end(assets.get(file));
    });
    await new Promise((resolve, reject) => { server.once('error', reject); server.listen(0, '127.0.0.1', resolve); });
    const origin = `http://127.0.0.1:${server.address().port}`;
    for (const [name, engine, width, dark, touch] of [
      ['desktop-chromium', chromium, 1360, false, false],
      ['desktop-webkit', webkit, 1360, true, false],
      ['narrow-webkit', webkit, 320, true, true],
      ['narrow-chromium', chromium, 320, false, true],
    ]) {
      browser = await engine.launch(engine === chromium
        ? { headless: true, channel: 'chromium', args: ['--no-sandbox'] } : { headless: true });
      const context = await browser.newContext({ viewport: { width, height: 960 },
        isMobile: false, hasTouch: touch, locale: 'en-GB', reducedMotion: 'reduce' });
      const page = await context.newPage(); page.setDefaultTimeout(10000);
      const record = { name, width, height: 960, dark, touch, browser: browser.version(), checks: [] }; result.cases.push(record);
      page.on('pageerror', e => result.errors.push({ name, type: 'page', message: e.message }));
      page.on('console', m => { if (m.type() === 'error') result.errors.push({ name, type: 'console', message: m.text() }); });
      page.on('response', r => { if (!r.ok()) result.errors.push({ name, type: 'response', status: r.status(), url: r.url() }); });
      page.on('crash', () => result.errors.push({ name, type: 'crash' }));
      await page.route('**/*', route => {
        assert.equal(new URL(route.request().url()).origin, origin);
        assert.equal(route.request().method(), 'GET'); return route.continue();
      });
      await page.addInitScript(dark => document.addEventListener('DOMContentLoaded', () => document.documentElement.classList.toggle('dark', dark)), dark);
      const capture = async suffix => {
        assert(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1));
        const file = `${name}-${suffix}.png`; await page.screenshot({ path: path.join(output, file) });
        result.captures.push({ file, sha256: hash(fs.readFileSync(path.join(output, file))) });
      };
      await page.goto(`${origin}/docs/CONFIGURATION`, { waitUntil: 'domcontentloaded' });
      const table = page.locator('article table').first(); await table.waitFor();
      const wrapper = table.locator('..');
      const previous = page.getByRole('link', { name: '← All documentation', exact: true });
      const next = page.getByRole('link', { name: 'Audit storage and safe recovery', exact: true });
      const geometry = () => wrapper.evaluate(e => ({ clientWidth: e.clientWidth, scrollWidth: e.scrollWidth,
        scrollLeft: e.scrollLeft, focused: document.activeElement === e, tabindex: e.tabIndex,
        role: e.getAttribute('role'), name: e.getAttribute('aria-label'),
        outlineWidth: getComputedStyle(e).outlineWidth, outlineStyle: getComputedStyle(e).outlineStyle,
        outlineColor: getComputedStyle(e).outlineColor }));
      await previous.focus(); await page.keyboard.press('Tab');
      record.before = await geometry();
      assert(record.before.focused); assert.equal(record.before.tabindex, 0);
      assert.equal(record.before.role, 'region'); assert.equal(record.before.name, 'Scrollable table');
      assert(parseFloat(record.before.outlineWidth) >= 2); assert.equal(record.before.outlineStyle, 'solid');
      assert.deepEqual(await table.getByRole('columnheader').allTextContents(), ['File', 'Purpose', 'Security Level']);
      assert.deepEqual(await table.locator('thead th').evaluateAll(ns => ns.map(n => n.getAttribute('scope'))), ['col', 'col', 'col']);
      record.accessibility = await wrapper.ariaSnapshot();
      assert(record.accessibility.includes('region "Scrollable table"'));
      assert(record.accessibility.includes('columnheader "Security Level"'));
      for (let i = 0; i < 5; i++) await page.keyboard.press('ArrowRight');
      if (width === 320) await page.waitForFunction(() => {
        const e = document.querySelector('[data-doc-table-scroll]');
        return e.scrollLeft >= e.scrollWidth - e.clientWidth - 1;
      });
      record.afterKeys = await geometry();
      if (width === 320) {
        assert(record.before.scrollWidth > record.before.clientWidth);
        const column = await table.locator('thead th').last().evaluate(e => {
          const a = e.getBoundingClientRect(), b = e.closest('[data-doc-table-scroll]').getBoundingClientRect();
          return { left: a.left, right: a.right, viewportLeft: b.left, viewportRight: b.right };
        });
        record.lastColumn = column; assert(column.right <= column.viewportRight + 1); assert(column.left >= column.viewportLeft - 1);
      } else assert.equal(record.afterKeys.scrollLeft, 0);
      await wrapper.evaluate(e => e.scrollIntoView({ block: 'start', behavior: 'instant' }));
      await capture('keyboard');
      for (let i = 0; i < 12; i++) await page.keyboard.press('ArrowLeft');
      await page.waitForFunction(() => document.querySelector('[data-doc-table-scroll]').scrollLeft === 0);
      await page.keyboard.press('Shift+Tab'); assert(await previous.evaluate(e => document.activeElement === e));
      await page.keyboard.press('Tab'); assert(await wrapper.evaluate(e => document.activeElement === e));
      await page.keyboard.press('Tab'); assert(await next.evaluate(e => document.activeElement === e));
      record.checks.push('real Tab admits the named region; native arrows reveal Security Level; Shift+Tab/Tab escape without a trap');
      if (touch && engine === chromium) {
        await wrapper.evaluate(e => e.scrollIntoView({ block: 'start', behavior: 'instant' }));
        const box = await wrapper.boundingBox(); const y = Math.min(250, box.y + 65);
        const cdp = await context.newCDPSession(page);
        await cdp.send('Input.dispatchTouchEvent', { type: 'touchStart', touchPoints: [{ x: 270, y }] });
        for (let i = 1; i <= 10; i++) await cdp.send('Input.dispatchTouchEvent', { type: 'touchMove', touchPoints: [{ x: 270 - 21 * i, y }] });
        await cdp.send('Input.dispatchTouchEvent', { type: 'touchEnd', touchPoints: [] });
        await page.waitForFunction(() => document.querySelector('[data-doc-table-scroll]').scrollLeft > 0);
        record.afterNativeTouch = await geometry(); await capture('touch'); await cdp.detach();
        record.checks.push('Chromium native touch pan scrolls the unchanged overflow container');
      }
      await page.goto(`${origin}/docs/KEYBOARD_FIXTURE`, { waitUntil: 'domcontentloaded' });
      const region = page.getByRole('region', { name: 'Scrollable table', exact: true }); await region.waitFor();
      assert.equal(await page.locator('[aria-hidden], [role="button"]').count(), 0);
      await page.getByRole('link', { name: '← All documentation', exact: true }).focus();
      await page.keyboard.press('Tab'); assert(await region.evaluate(e => document.activeElement === e));
      await page.keyboard.press('Tab');
      const inside = page.getByRole('link', { name: 'Recovery checklist', exact: true });
      assert(await inside.evaluate(e => document.activeElement === e));
      await page.keyboard.press('Tab');
      assert(await page.getByRole('link', { name: 'After the table', exact: true }).evaluate(e => document.activeElement === e));
      await page.keyboard.press('Shift+Tab');
      if (touch) await inside.tap(); else await page.keyboard.press('Enter');
      await page.waitForFunction(() => document.activeElement?.id === 'back-up-recoverable-state');
      await page.goBack(); await region.waitFor();
      assert.equal(await region.count(), 1);
      record.checks.push('sanitized forged markers cannot bypass/hide the table; in-table and following links retain native tab order, activation and Back');
      await context.close(); await browser.close(); browser = null;
    }
    assert.deepEqual(result.errors, []); result.result = 'passed';
  } catch (error) { result.failure = error.stack; process.exitCode = 1; }
  finally {
    if (browser) await browser.close(); result.cleanup.browserClosed = true;
    if (server) await new Promise((resolve, reject) => { server.close(e => e ? reject(e) : resolve()); server.closeAllConnections(); });
    result.cleanup.serverClosed = true; result.verifiedAt = new Date().toISOString();
    result.limits = 'Actual Docs/Markdown/router/CSS; fixture shell and one inert test document. Not screen-reader, native Safari touch-pan, installed-release, backup/SSO recovery, or the separate long-history/reload assessment.';
    fs.writeFileSync(path.join(output, 'result.json'), JSON.stringify(result, null, 2) + '\n');
    console.log(JSON.stringify(result));
  }
})();
