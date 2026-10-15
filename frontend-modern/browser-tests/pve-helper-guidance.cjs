// Real shipped Docs fetch/renderer/router, with no host or agent operation.
const fs = require('node:fs');
const path = require('node:path');
const http = require('node:http');
const crypto = require('node:crypto');
const assert = require('node:assert/strict');
const { chromium, webkit } = require('playwright');
const root = '/workspace/frontend-modern';
const preview = '/workspace/tmp/doc-command-help/static';
const output = '/workspace/tmp/pve-helper-guidance';
const anchor = 'proxmox-lxc-filesystem-readings-with-the-typed-helper';
const digest = bytes => crypto.createHash('sha256').update(bytes).digest('hex');
const bindingBytes = fs.readFileSync(path.join(preview, 'binding.json'));
const binding = JSON.parse(bindingBytes);
const result = { result: 'incomplete', source_sha: binding.source_sha,
  binding_sha256: digest(bindingBytes), playwright: require('playwright/package.json').version,
  cases: [], captures: [], errors: [], requests: [], served: [], cleanup: {},
  limits: 'Shipped help navigation/reading only; no native PVE, helper, installed recovery or release acceptance.' };

(async () => {
  let server, browser;
  try {
    fs.mkdirSync(output, { recursive: true });
    assert.equal(result.playwright, '1.56.1');
    assert.equal(result.playwright, JSON.parse(fs.readFileSync('/workspace/tests/integration/package-lock.json'))
      .packages['node_modules/@playwright/test'].version);
    for (const [file, hash] of Object.entries(binding.sources))
      assert.equal(digest(fs.readFileSync(path.join(root, file))), hash, file);
    const assets = new Map();
    for (const [file, hash] of Object.entries(binding.artifacts)) {
      const bytes = fs.readFileSync(path.join(preview, file));
      assert.equal(digest(bytes), hash, file); assets.set(file, bytes);
    }
    for (const name of ['AGENT_SECURITY', 'UNIFIED_AGENT'])
      assert.equal(binding.artifacts[`docs/${name}.md`], binding.sources[`public/docs/${name}.md`]);
    result.source_inputs = Object.keys(binding.sources).length;
    server = http.createServer((req, res) => {
      if (req.method !== 'GET') { res.writeHead(405); res.end(); return; }
      const pathname = new URL(req.url, 'http://fixture.invalid').pathname;
      const file = pathname.startsWith('/docs/') && !pathname.endsWith('.md')
        ? 'browser-tests/doc-command-help.html' : pathname.slice(1);
      if (!assets.has(file)) { res.writeHead(404); res.end(); return; }
      const bytes = assets.get(file);
      result.served.push({ pathname, file, sha256: digest(bytes) });
      const ext = path.extname(file);
      res.writeHead(200, { 'Content-Type': ({ '.md': 'text/markdown', '.css': 'text/css',
        '.js': 'text/javascript', '.html': 'text/html' })[ext] + '; charset=utf-8', 'Cache-Control': 'no-store' });
      res.end(bytes);
    });
    await new Promise((resolve, reject) => {
      server.once('error', reject); server.listen(0, '127.0.0.1', resolve);
    });
    const origin = `http://127.0.0.1:${server.address().port}`;
    for (const [name, engine, width, dark] of [
      ['desktop-chromium', chromium, 1360, false], ['phone-webkit', webkit, 390, true],
    ]) {
      browser = await engine.launch(engine === chromium
        ? { headless: true, channel: 'chromium', args: ['--no-sandbox'] } : { headless: true });
      const context = await browser.newContext({ viewport: { width, height: 900 },
        isMobile: width < 500, hasTouch: width < 500, locale: 'en-GB' });
      const page = await context.newPage(); page.setDefaultTimeout(12000);
      const record = { name, width, height: 900, dark, browser: browser.version(), checks: [] };
      result.cases.push(record);
      page.on('pageerror', error => result.errors.push(error.message));
      await page.route('**/*', route => {
        const url = new URL(route.request().url()); result.requests.push({ name, path: url.pathname });
        assert.equal(url.origin, origin, 'no external requests permitted');
        assert(!url.pathname.startsWith('/api/'), 'help must not probe a backend');
        return route.continue();
      });
      await page.addInitScript(dark => document.addEventListener('DOMContentLoaded', () =>
        document.documentElement.classList.toggle('dark', dark)), dark);
      const capture = async suffix => {
        await page.evaluate(() => new Promise(resolve => requestAnimationFrame(() => requestAnimationFrame(resolve))));
        assert(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth), 'page overflow');
        const file = `${name}-${suffix}.png`; await page.screenshot({ path: path.join(output, file) });
        result.captures.push({ file, sha256: digest(fs.readFileSync(path.join(output, file))), name });
      };
      const activate = async link => {
        if (width < 500) await link.tap(); else { await link.focus(); await page.keyboard.press('Enter'); }
      };
      await page.goto(`${origin}/docs/UNIFIED_AGENT#inside-guest-runtime-boundaries`, { waitUntil: 'domcontentloaded' });
      const link = page.locator(`article a[href="/docs/AGENT_SECURITY#${anchor}"]`);
      await link.waitFor();
      await link.evaluate(element => window.scrollBy(0, element.closest('p').getBoundingClientRect().top - 64));
      await capture('entry');
      await page.evaluate(() => window.__helperDocsShell = 'same-shell');
      await activate(link); await page.waitForURL(`**/docs/AGENT_SECURITY#${anchor}`);
      await page.waitForFunction(anchor => {
        const heading = document.getElementById(anchor);
        return heading && document.activeElement === heading && heading.getBoundingClientRect().top >= 0
          && heading.getBoundingClientRect().top < innerHeight;
      }, anchor);
      assert.equal(await page.evaluate(() => window.__helperDocsShell), 'same-shell');
      const section = await page.locator(`#${anchor}`).evaluate(heading => {
        let text = ''; for (let el = heading.nextElementSibling; el && el.tagName !== 'H3'; el = el.nextElementSibling)
          text += el.textContent + ' '; return text.replace(/\s+/g, ' ');
      });
      for (const phrase of ['without pct or lxc-info requests', 'fallback cannot recover',
        'cgroup v1', 'degraded or unavailable', 'Do not remove PrivateNetwork',
        'omitted containers cannot refresh old filesystem readings', 'Operation v1 remains complete-only',
        'collector-only automatic update cannot replace its helper']) assert(section.includes(phrase), phrase);
      record.checks.push('actual shipped cross-guide link, keyboard/touch, same shell, focused anchor, all boundary/partial distinctions');
      await capture('helper');
      const partial = page.locator('article p').filter({ hasText: 'When an inventory succeeds only partly' });
      await partial.evaluate(element => window.scrollBy(0, element.getBoundingClientRect().top - 64));
      await capture('partial');
      await page.reload({ waitUntil: 'domcontentloaded' });
      await page.waitForFunction(anchor => document.activeElement?.id === anchor, anchor);
      record.checks.push('reload preserves deep-link heading focus');
      await context.close(); await browser.close(); browser = null;
    }
    assert.equal(result.errors.length, 0);
    result.result = 'passed';
  } catch (error) { result.failure = error.stack; throw error; }
  finally {
    if (browser) await browser.close(); result.cleanup.browser_closed = true;
    if (server) await new Promise((resolve, reject) => {
      server.close(error => error ? reject(error) : resolve()); server.closeAllConnections();
    }); result.cleanup.server_closed = true;
    fs.writeFileSync(path.join(output, 'result.json'), JSON.stringify(result, null, 2) + '\n');
  }
})().catch(error => { console.error(error); process.exitCode = 1; });
