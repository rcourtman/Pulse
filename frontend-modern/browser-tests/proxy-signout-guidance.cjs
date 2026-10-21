// The shipped Docs/router/Markdown consumer, not an authentication fixture.
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const http = require('node:http');
const { createHash } = require('node:crypto');
const { chromium, webkit } = require('playwright');
const workspace = process.cwd();
const root = path.join(workspace, 'frontend-modern');
const output = path.join(workspace, 'tmp/proxy-signout-guidance');
const preview = path.join(workspace, 'tmp/doc-command-help/static');
const hash = bytes => createHash('sha256').update(bytes).digest('hex');
const binding = JSON.parse(fs.readFileSync(path.join(preview, 'binding.json')));
const result = { result: 'incomplete', source: binding, cases: [], captures: [], errors: [], requests: [], cleanup: {} };
fs.mkdirSync(output, { recursive: true });
(async () => {
  let browser, server;
  try {
    result.playwright = require('playwright/package.json').version;
    assert.equal(result.playwright, '1.56.1');
    assert.equal(result.playwright, JSON.parse(fs.readFileSync(path.join(workspace,
      'tests/integration/package-lock.json'))).packages['node_modules/@playwright/test'].version);
    for (const [file, expected] of Object.entries(binding.sources))
      assert.equal(hash(fs.readFileSync(path.join(root, file))), expected, file);
    for (const [file, expected] of Object.entries(binding.artifacts))
      assert.equal(hash(fs.readFileSync(path.join(preview, file))), expected, file);
    assert.equal(binding.artifacts['docs/PROXY_AUTH.md'], hash(fs.readFileSync(path.join(workspace, 'docs/PROXY_AUTH.md'))));
    server = http.createServer((req, res) => {
      const pathname = new URL(req.url, 'http://fixture.invalid').pathname;
      const file = pathname.startsWith('/docs/') && !pathname.endsWith('.md')
        ? 'browser-tests/doc-command-help.html' : pathname.slice(1);
      if (!Object.hasOwn(binding.artifacts, file)) { res.writeHead(404); res.end(); return; }
      const bytes = fs.readFileSync(path.join(preview, file));
      assert.equal(hash(bytes), binding.artifacts[file]);
      const type = ({ '.md': 'text/markdown', '.css': 'text/css', '.js': 'text/javascript', '.html': 'text/html' })[path.extname(file)];
      res.writeHead(200, { 'Content-Type': `${type}; charset=utf-8`, 'Cache-Control': 'no-store' }); res.end(bytes);
    });
    await new Promise((resolve, reject) => { server.once('error', reject); server.listen(0, '127.0.0.1', resolve); });
    const origin = `http://127.0.0.1:${server.address().port}`;
    for (const [name, engine, width, dark] of [['desktop', chromium, 1360, false], ['phone', webkit, 320, true]]) {
      browser = await engine.launch(engine === chromium
        ? { headless: true, channel: 'chromium', args: ['--no-sandbox'] } : { headless: true });
      const context = await browser.newContext({ viewport: { width, height: 960 }, isMobile: width < 500, hasTouch: width < 500, locale: 'en-GB' });
      const page = await context.newPage(); page.setDefaultTimeout(15000);
      const record = { name, width, height: 960, dark, browserVersion: browser.version(), checks: [] }; result.cases.push(record);
      page.on('pageerror', e => result.errors.push(e.message));
      page.on('console', m => { if (m.type() === 'error') result.errors.push(m.text()); });
      page.on('response', r => { if (!r.ok()) result.errors.push(`HTTP ${r.status()}: ${r.url()}`); });
      await page.route('**/*', route => {
        const req = route.request(); const url = new URL(req.url());
        result.requests.push({ method: req.method(), path: url.pathname });
        assert.equal(url.origin, origin); assert.equal(req.method(), 'GET');
        assert(!url.pathname.startsWith('/api/'), 'documentation proof must not sign out or call an API');
        return route.continue();
      });
      await page.addInitScript(dark => document.addEventListener('DOMContentLoaded', () => document.documentElement.classList.toggle('dark', dark)), dark);
      const focus = () => page.waitForFunction(() => document.activeElement?.id === 'sign-out-and-access-removal');
      const capture = async (suffix, locator) => {
        await locator.evaluate(e => e.scrollIntoView({ block: 'start', behavior: 'instant' }));
        await page.evaluate(() => new Promise(r => requestAnimationFrame(() => requestAnimationFrame(r))));
        assert(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1));
        const file = `${name}-${suffix}.png`; await page.screenshot({ path: path.join(output, file) });
        result.captures.push({ file, sha256: hash(fs.readFileSync(path.join(output, file))) });
      };
      await page.goto(`${origin}/docs/PROXY_AUTH`, { waitUntil: 'domcontentloaded' });
      const row = page.locator('article tr').filter({ hasText: 'PROXY_AUTH_LOGOUT_URL' }); await row.waitFor();
      assert((await row.innerText()).includes('not an access-revocation operation'));
      const link = row.getByRole('link', { name: 'sign-out', exact: true });
      assert.equal(await link.getAttribute('href'), '#sign-out-and-access-removal');
      if (width < 500) await link.tap(); else { await link.focus(); await page.keyboard.press('Enter'); }
      await focus();
      const heading = page.getByRole('heading', { name: 'Sign-out and access removal', exact: true });
      const text = await heading.evaluate(h => {
        let text = ''; for (let n = h.nextElementSibling; n && !/^H[123]$/.test(n.tagName); n = n.nextElementSibling) text += ` ${n.textContent}`;
        return text.replace(/\s+/g, ' ');
      });
      for (const phrase of ['A logout redirect is not proof that access ended', "instead of calling Pulse's /api/logout",
        'does not first clear its own session', 'Without that URL', 'does not sign out of the authenticating proxy',
        'without showing Pulse’s login form', 'sign out all sessions', 'API tokens and agent credentials',
        'same browser session', 'new private window tests a different session', 'do not hand that browser or device',
        'blocking only future IdP logins', 'Do not disable Pulse authentication']) {
        assert(text.replace(/[’]/g, "'").includes(phrase.replace(/[’]/g, "'")), phrase);
      }
      await capture('boundary', heading);
      if (width < 500) await capture('fallback', page.locator('article li').filter({ hasText: 'Without that URL, Pulse requests' }));
      await capture('verification', page.locator('article p').filter({ hasText: 'For a planned sign-out check' }));
      if (width < 500) await capture('removal', page.locator('article p').filter({ hasText: "To remove a person's access" }));
      await page.reload({ waitUntil: 'domcontentloaded' }); await focus();
      const trust = page.locator('article p').filter({ hasText: "To remove a person's access" }).getByRole('link', { name: 'backend isolation' });
      if (width < 500) await trust.tap(); else { await trust.focus(); await page.keyboard.press('Enter'); }
      await page.waitForFunction(() => document.activeElement?.id === '-header-trust-boundary');
      await page.goBack(); await focus();
      const response = await page.request.get(`${origin}/docs/PROXY_AUTH.md`);
      assert.equal(response.status(), 200); assert.equal(hash(await response.body()), binding.artifacts['docs/PROXY_AUTH.md']);
      record.checks.push('Exact served guide and both logout branches', 'Keyboard or touch table-to-guidance link', 'Same-session and access-removal guidance', 'Fragment focus/reload', 'Backend isolation link and Back', 'No page overflow');
      await context.close(); await browser.close(); browser = null;
    }
    assert.deepEqual(result.errors, []); result.result = 'passed';
  } catch (error) { result.failure = error.stack; process.exitCode = 1; }
  finally {
    if (browser) await browser.close(); result.cleanup.browserClosed = true;
    if (server) await new Promise((resolve, reject) => { server.close(e => e ? reject(e) : resolve()); server.closeAllConnections(); });
    result.cleanup.serverClosed = true; result.verifiedAt = new Date().toISOString();
    result.limits = 'Docs consumer only, not proxy/IdP logout, Pulse session invalidation, original SSO-save recovery or installed/released access revocation.';
    fs.writeFileSync(path.join(output, 'result.json'), JSON.stringify(result, null, 2) + '\n');
    console.log(JSON.stringify({ source: binding.source_sha, result: result.result, cases: result.cases.length, captures: result.captures.length, errors: result.errors, cleanup: result.cleanup, failure: result.failure }));
  }
})();
