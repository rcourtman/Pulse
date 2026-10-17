// Actual shipped Docs/router/Markdown/styles inside a fixture shell.
// No live API, identity provider, credential or customer state is used.
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const { createHash } = require('node:crypto');
const http = require('node:http');
const { chromium, webkit } = require('playwright');

const workspace = process.cwd();
const root = path.join(workspace, 'frontend-modern');
const output = path.join(workspace, 'tmp/oidc-save-recovery');
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
    assert.equal(binding.artifacts['docs/OIDC.md'], digest(fs.readFileSync(path.join(workspace, 'docs/OIDC.md'))));
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
      await page.goto(`${origin}/docs/OIDC#sign-in-fails-after-editing-a-provider`,
        { waitUntil: 'domcontentloaded', timeout: 60000 });
      await page.getByRole('heading', { name: 'Sign-in fails after editing a provider', exact: true }).waitFor();
      await page.evaluate(dark => {
        document.documentElement.classList.toggle('dark', dark);
        location.hash = '#sign-in-fails-after-editing-a-provider';
      }, dark);
      const focused = id => page.waitForFunction(id => document.activeElement?.id === id, id);
      await focused('sign-in-fails-after-editing-a-provider');
      const section = await page.locator('#sign-in-fails-after-editing-a-provider').evaluate(heading => {
        let text = '';
        for (let node = heading.nextElementSibling; node && node.tagName !== 'H3'; node = node.nextElementSibling)
          text += ` ${node.textContent}`;
        return text.replace(/\s+/g, ' ');
      });
      for (const phrase of ['starts blank deliberately', 'does not show whether',
        'Keep independent administrator access.', '?show_local=true',
        'does not restore credentials', 'Recover without replacing the provider.',
        'Do not put it in a command, URL or report', 'Do not delete and recreate',
        'Check sign-in separately.', 'not the authenticated code exchange',
        'do not repeat the save or login sequence', 'not proof of a repaired save operation'])
        assert(section.includes(phrase), phrase);
      assert(await page.locator('article a[href="#sign-in-fails-after-editing-a-provider"]')
        .evaluate(a => a.closest('tr').textContent.includes('invalid_client')));
      const capture = async (suffix, locator) => {
        await locator.evaluate(e => e.scrollIntoView({ block: 'start', behavior: 'instant' }));
        await page.evaluate(() => new Promise(r => requestAnimationFrame(() => requestAnimationFrame(r))));
        assert(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1));
        const file = `${name}-${suffix}.png`;
        await page.screenshot({ path: path.join(output, file) });
        result.captures.push({ file, sha256: digest(fs.readFileSync(path.join(output, file))) });
      };
      await capture('overview', page.locator('#sign-in-fails-after-editing-a-provider'));
      await capture('recovery', page.locator('article li').filter({ hasText: 'Recover without replacing the provider.' }));
      await capture('limits', page.locator('article li').filter({ hasText: 'Check sign-in separately.' }));
      const link = page.locator('article a[href="#access-control"]').first();
      if (width < 500) await link.tap(); else { await link.focus(); await page.keyboard.press('Enter'); }
      await focused('access-control');
      assert.equal(new URL(page.url()).hash, '#access-control');
      await page.goBack({ waitUntil: 'domcontentloaded' });
      await focused('sign-in-fails-after-editing-a-provider');
      await page.reload({ waitUntil: 'domcontentloaded' });
      await focused('sign-in-fails-after-editing-a-provider');
      record.checks.push('Exact source and shipped OIDC guide match; administrator preservation and recovery limits render',
        'No page overflow at desktop and 320px; all recovery steps and warnings captured',
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
    result.limits = 'Shipped documentation only; no IdP, authenticated login or encrypted-secret save recovery, installed repair or release acceptance.';
    fs.writeFileSync(path.join(output, 'result.json'), JSON.stringify(result, null, 2) + '\n');
    console.log(JSON.stringify({ source: binding.source_sha, result: result.result,
      cases: result.cases.length, captures: result.captures.length,
      errors: result.errors, cleanup: result.cleanup, failure: result.failure }));
  }
})();
