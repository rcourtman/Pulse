// Read-only help journey through the actual shipped Docs page and renderer.
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const http = require('node:http');
const { createHash } = require('node:crypto');
const { chromium, webkit } = require('playwright');
const workspace = process.cwd();
const root = path.join(workspace, 'frontend-modern');
const output = path.join(workspace, 'tmp/configuration-recovery-guidance');
const preview = path.join(workspace, 'tmp/doc-command-help/static');
const hash = bytes => createHash('sha256').update(bytes).digest('hex');
const binding = JSON.parse(fs.readFileSync(path.join(preview, 'binding.json')));
const baseline = fs.readFileSync(path.join(output, 'base-CONFIGURATION.md'));
const baseIdentity = JSON.parse(fs.readFileSync(path.join(output, 'base.json')));
assert.equal(hash(baseline), baseIdentity.configuration_sha256);
const result = { result: 'incomplete', source: binding, baseline: baseIdentity,
  cases: [], captures: [], errors: [], requests: [], cleanup: {} };
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
    for (const name of ['CONFIGURATION', 'DEPLOYMENT_MODELS', 'MIGRATION', 'AUTO_UPDATE', 'OIDC', 'VM_DISK_MONITORING'])
      assert.equal(binding.artifacts[`docs/${name}.md`], hash(fs.readFileSync(path.join(workspace, `docs/${name}.md`))));
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
      let oldConfiguration = true;
      await page.route('**/*', route => {
        const req = route.request(); const url = new URL(req.url());
        result.requests.push({ method: req.method(), path: url.pathname });
        assert.equal(url.origin, origin); assert.equal(req.method(), 'GET');
        assert(!url.pathname.startsWith('/api/'), 'reading recovery help must not operate an installation');
        if (oldConfiguration && url.pathname === '/docs/CONFIGURATION.md')
          return route.fulfill({ status: 200, contentType: 'text/markdown', body: baseline });
        return route.continue();
      });
      await page.addInitScript(dark => document.addEventListener('DOMContentLoaded', () => document.documentElement.classList.toggle('dark', dark)), dark);
      const focus = id => page.waitForFunction(id => document.activeElement?.id === id, id);
      const activate = async link => {
        await link.scrollIntoViewIfNeeded();
        if (width < 500) await link.tap(); else { await link.focus(); await page.keyboard.press('Enter'); }
      };
      const capture = async (suffix, locator) => {
        await locator.evaluate(e => e.scrollIntoView({ block: 'start', behavior: 'instant' }));
        await page.evaluate(() => new Promise(r => requestAnimationFrame(() => requestAnimationFrame(r))));
        assert(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1));
        const file = `${name}-${suffix}.png`; await page.screenshot({ path: path.join(output, file) });
        result.captures.push({ file, sha256: hash(fs.readFileSync(path.join(output, file))) });
      };
      await page.goto(`${origin}/docs/CONFIGURATION`, { waitUntil: 'domcontentloaded' });
      await page.locator('article li').filter({ hasText: 'Back up .env separately from system.json.' }).waitFor();
      assert.equal(await page.getByRole('heading', { name: 'Back up recoverable state', exact: true }).count(), 0);
      record.checks.push('Exact-base two-file checklist reproduced in the real consumer');
      oldConfiguration = false;
      await page.goto(`${origin}/docs/DEPLOYMENT_MODELS#where-configuration-lives`, { waitUntil: 'domcontentloaded' });
      await focus('where-configuration-lives');
      const storage = page.locator('article li').filter({ hasText: 'UI/API-managed OIDC and SAML providers' });
      assert((await storage.innerText()).includes('sso.enc'));
      assert((await page.locator('article li').filter({ hasText: 'Legacy single-provider OIDC config' }).innerText()).includes('oidc.enc'));
      await capture('storage-list', storage);
      await activate(page.getByRole('link', { name: 'recovery-state checklist', exact: true }));
      await focus('back-up-recoverable-state');
      await capture('checklist', page.getByRole('heading', { name: 'Back up recoverable state', exact: true }));
      const article = (await page.locator('article').innerText()).replace(/\s+/g, ' ');
      for (const text of ['.env and system.json alone are not a full backup', 'matching .encryption.key',
        'audit signing key', 'organisation directories', 'deployment-supplied credentials',
        'monitoring and alerts pause', 'isolated destination before retiring the source',
        'Do not restore over a running instance', 'not a recovery shortcut', 'working administrator session'])
        assert(article.includes(text), text);
      await activate(page.getByRole('link', { name: 'full-state recovery', exact: true }));
      await focus('full-state-recovery');
      await capture('full-state', page.getByRole('heading', { name: 'Full-state recovery', exact: true }));
      assert((await page.locator('article').innerText()).includes('matching .encryption.key'));
      await page.goBack(); await focus('back-up-recoverable-state');
      await activate(page.getByRole('link', { name: 'configuration export', exact: true }));
      await focus('configuration-transfer');
      assert((await page.locator('article').innerText()).includes('SSO configuration is included'));
      await page.goBack(); await focus('back-up-recoverable-state');
      await activate(page.getByRole('link', { name: 'updater snapshot', exact: true }));
      await focus('what-an-update-snapshot-contains');
      await page.goBack(); await focus('back-up-recoverable-state');
      await activate(page.getByRole('link', { name: 'provider-save recovery', exact: true }));
      await focus('sign-in-fails-after-editing-a-provider');
      assert((await page.locator('article').innerText()).includes('administrator session'));
      await page.goBack(); await focus('back-up-recoverable-state');
      await activate(page.getByRole('link', { name: 'backup safety', exact: true }));
      await focus('backup-safety');
      await page.goBack(); await focus('back-up-recoverable-state');
      await page.reload({ waitUntil: 'domcontentloaded' }); await focus('back-up-recoverable-state');
      const providerRow = page.locator('article tr').filter({ hasText: 'UI/API-managed OIDC and SAML providers' });
      assert((await providerRow.innerText()).includes('sso.enc'));
      await capture('storage-table', providerRow);
      for (const name of ['CONFIGURATION', 'DEPLOYMENT_MODELS']) {
        const response = await page.request.get(`${origin}/docs/${name}.md`);
        assert.equal(response.status(), 200); assert.equal(hash(await response.body()), binding.artifacts[`docs/${name}.md`]);
      }
      record.checks.push('Current and legacy OIDC storage are distinct in both shipped inventories',
        'Keyboard/touch recovery links reach the actual full-state, export and update-snapshot scope',
        'Existing provider-save and backup-safety routes retain administrator/stopped-state precautions',
        'Fragment focus, reload, Back, exact served bytes and no document overflow');
      await context.close(); await browser.close(); browser = null;
    }
    assert.deepEqual(result.errors, []); result.result = 'passed';
  } catch (error) { result.failure = error.stack; process.exitCode = 1; }
  finally {
    if (browser) await browser.close(); result.cleanup.browserClosed = true;
    if (server) await new Promise((resolve, reject) => { server.close(e => e ? reject(e) : resolve()); server.closeAllConnections(); });
    result.cleanup.serverClosed = true; result.verifiedAt = new Date().toISOString();
    result.limits = 'Shipped help consumer only; no real backup, crypto runtime, restore, native migration, SSO recovery or release acceptance.';
    fs.writeFileSync(path.join(output, 'result.json'), JSON.stringify(result, null, 2) + '\n');
    console.log(JSON.stringify({ source: binding.source_sha, result: result.result, cases: result.cases.length,
      captures: result.captures.length, errors: result.errors, cleanup: result.cleanup, failure: result.failure }));
  }
})();
