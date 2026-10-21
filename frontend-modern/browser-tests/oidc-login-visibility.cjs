// The shipped Docs/router/Markdown consumer, not a substitute login screen.
// No password, IdP, authenticated API or proxy configuration is exercised.
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const { createHash } = require('node:crypto');
const http = require('node:http');
const { chromium, webkit } = require('playwright');

const workspace = process.cwd();
const root = path.join(workspace, 'frontend-modern');
const output = path.join(workspace, 'tmp/oidc-login-visibility');
const preview = path.join(workspace, 'tmp/doc-command-help/static');
const hash = bytes => createHash('sha256').update(bytes).digest('hex');
const binding = JSON.parse(fs.readFileSync(path.join(preview, 'binding.json')));
const result = { result: 'incomplete', source: binding, cases: [], captures: [],
  errors: [], requests: [], cleanup: {} };
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
    for (const name of ['OIDC', 'CONFIGURATION'])
      assert.equal(binding.artifacts[`docs/${name}.md`],
        hash(fs.readFileSync(path.join(workspace, `docs/${name}.md`))));
    server = http.createServer((req, res) => {
      const pathname = new URL(req.url, 'http://fixture.invalid').pathname;
      const file = pathname.startsWith('/docs/') && !pathname.endsWith('.md')
        ? 'browser-tests/doc-command-help.html' : pathname.slice(1);
      if (!Object.hasOwn(binding.artifacts, file)) { res.writeHead(404); res.end(); return; }
      const bytes = fs.readFileSync(path.join(preview, file));
      assert.equal(hash(bytes), binding.artifacts[file]);
      const type = ({ '.md': 'text/markdown', '.css': 'text/css', '.js': 'text/javascript',
        '.html': 'text/html' })[path.extname(file)];
      res.writeHead(200, { 'Content-Type': `${type}; charset=utf-8`, 'Cache-Control': 'no-store' });
      res.end(bytes);
    });
    await new Promise((resolve, reject) => {
      server.once('error', reject); server.listen(0, '127.0.0.1', resolve);
    });
    const origin = `http://127.0.0.1:${server.address().port}`;
    for (const [name, engine, width, dark] of [
      ['desktop', chromium, 1360, false], ['phone', webkit, 320, true],
    ]) {
      browser = await engine.launch(engine === chromium
        ? { headless: true, channel: 'chromium', args: ['--no-sandbox'] } : { headless: true });
      const context = await browser.newContext({ viewport: { width, height: 960 },
        isMobile: width < 500, hasTouch: width < 500, locale: 'en-GB' });
      const page = await context.newPage(); page.setDefaultTimeout(15000);
      const record = { name, width, height: 960, dark, browserVersion: browser.version(), checks: [] };
      result.cases.push(record);
      page.on('pageerror', e => result.errors.push(e.message));
      page.on('console', m => { if (m.type() === 'error') result.errors.push(m.text()); });
      page.on('response', r => { if (!r.ok()) result.errors.push(`HTTP ${r.status()}: ${r.url()}`); });
      await page.route('**/*', route => {
        const req = route.request(); const url = new URL(req.url());
        result.requests.push({ method: req.method(), path: url.pathname });
        assert.equal(url.origin, origin); assert.equal(req.method(), 'GET');
        assert(!url.pathname.startsWith('/api/'), 'no live API requests');
        return route.continue();
      });
      await page.addInitScript(dark => document.addEventListener('DOMContentLoaded', () =>
        document.documentElement.classList.toggle('dark', dark)), dark);
      const focused = id => page.waitForFunction(id => document.activeElement?.id === id, id);
      const capture = async (suffix, locator) => {
        await locator.evaluate(e => e.scrollIntoView({ block: 'start', behavior: 'instant' }));
        await page.evaluate(() => new Promise(r => requestAnimationFrame(() => requestAnimationFrame(r))));
        assert(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1));
        const file = `${name}-${suffix}.png`;
        await page.screenshot({ path: path.join(output, file) });
        result.captures.push({ file, sha256: hash(fs.readFileSync(path.join(output, file))) });
      };
      await page.goto(`${origin}/docs/CONFIGURATION`, { waitUntil: 'domcontentloaded' });
      const row = page.locator('article tr').filter({ hasText: 'PULSE_AUTH_HIDE_LOCAL_LOGIN' });
      await row.waitFor();
      assert((await row.innerText()).includes('does not disable password authentication'));
      await capture('configuration', row);
      const link = row.getByRole('link', { name: 'OIDC login visibility', exact: true });
      assert.equal(await link.getAttribute('href'), '/docs/OIDC#hiding-the-local-login-form');
      if (width < 500) await link.tap(); else { await link.focus(); await page.keyboard.press('Enter'); }
      await focused('hiding-the-local-login-form');
      assert.equal(new URL(page.url()).pathname, '/docs/OIDC');
      const heading = page.getByRole('heading', { name: 'Hiding the local login form', exact: true });
      const text = await heading.evaluate(heading => {
        let text = '';
        for (let node = heading.nextElementSibling; node && !/^H[23]$/.test(node.tagName);
          node = node.nextElementSibling) text += ` ${node.textContent}`;
        return text.replace(/\s+/g, ' ');
      });
      for (const phrase of ['Hiding is not password rejection', 'change the login page only',
        'Local password authentication remains enabled', 'POST /api/login', '?show_local=true',
        'MFA requirement does not apply to a local password sign-in', 'does not enforce it',
        'direct backend access', 'HTTP Basic authentication is a separate path',
        'scoped API-token and agent access', 'do not block all API routes',
        'policy-compliant administrator recovery path', 'Do not disable Pulse authentication'])
        assert(text.includes(phrase), phrase);
      await capture('visibility', heading);
      await capture('policy', page.locator('article p').filter({ hasText: 'If your policy requires OIDC-only' }));
      await page.reload({ waitUntil: 'domcontentloaded' }); await focused('hiding-the-local-login-form');
      await page.goBack({ waitUntil: 'domcontentloaded' });
      await page.waitForURL('**/docs/CONFIGURATION'); await row.waitFor();
      for (const name of ['OIDC', 'CONFIGURATION']) {
        const response = await page.request.get(`${origin}/docs/${name}.md`);
        assert.equal(response.status(), 200);
        assert.equal(hash(await response.body()), binding.sources[`public/docs/${name}.md`]);
      }
      record.checks.push('Configuration warning and exact fetched guides',
        'Keyboard or touch link to the real OIDC section', 'Fragment focus and reload',
        'Visible password/MFA, Basic, backend and recovery limits', 'Browser Back',
        'No page overflow at each capture');
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
    result.limits = 'Documentation consumer only, not OIDC/password rejection, proxy enforcement, encrypted-save or installed/released recovery.';
    fs.writeFileSync(path.join(output, 'result.json'), JSON.stringify(result, null, 2) + '\n');
    console.log(JSON.stringify({ source: binding.source_sha, result: result.result,
      cases: result.cases.length, captures: result.captures.length,
      errors: result.errors, cleanup: result.cleanup, failure: result.failure }));
  }
})();
