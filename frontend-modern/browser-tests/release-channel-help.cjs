// Read the changed release guide through shipped Docs, Markdown, routing and CSS.
// No update, diagnostic request, notification or release operation is performed.
const fs = require('node:fs');
const path = require('node:path');
const http = require('node:http');
const crypto = require('node:crypto');
const assert = require('node:assert/strict');
const { chromium, webkit } = require('playwright');
const root = '/workspace/frontend-modern';
const preview = '/workspace/tmp/doc-command-help/static';
const output = '/workspace/tmp/release-channel-help';
const digest = bytes => crypto.createHash('sha256').update(bytes).digest('hex');
const bindingBytes = fs.readFileSync(path.join(preview, 'binding.json'));
const binding = JSON.parse(bindingBytes);
const result = { result: 'incomplete', source_sha: binding.source_sha,
  binding_sha256: digest(bindingBytes), playwright: require('playwright/package.json').version,
  cases: [], captures: [], errors: [], served: [], cleanup: {},
  limits: 'Changed help rendering/navigation only; not installed updates, release inclusion or workload recovery.' };

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
    assert.equal(binding.artifacts['docs/RELEASE_PROCESS.md'], binding.sources['public/docs/RELEASE_PROCESS.md']);
    assert.equal(binding.artifacts['docs/RELEASE_PROCESS.md'], digest(fs.readFileSync('/workspace/docs/RELEASE_PROCESS.md')));
    result.source_inputs = Object.keys(binding.sources).length;
    server = http.createServer((req, res) => {
      if (req.method !== 'GET') { res.writeHead(405); res.end(); return; }
      const pathname = new URL(req.url, 'http://fixture.invalid').pathname;
      const file = pathname.startsWith('/docs/') && !pathname.endsWith('.md')
        ? 'browser-tests/doc-command-help.html' : pathname.slice(1);
      if (!assets.has(file)) { res.writeHead(404); res.end(); return; }
      const bytes = assets.get(file);
      result.served.push({ pathname, file, sha256: digest(bytes) });
      const type = ({ '.md': 'text/markdown', '.css': 'text/css', '.js': 'text/javascript', '.html': 'text/html' })[path.extname(file)];
      res.writeHead(200, { 'Content-Type': `${type}; charset=utf-8`, 'Cache-Control': 'no-store' }); res.end(bytes);
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
      page.on('console', message => { if (message.type() === 'error') result.errors.push(message.text()); });
      page.on('response', response => { if (!response.ok()) result.errors.push(`HTTP ${response.status()}: ${response.url()}`); });
      await page.route('**/*', route => {
        const url = new URL(route.request().url());
        assert.equal(url.origin, origin, 'no external requests permitted');
        assert(!url.pathname.startsWith('/api/'), 'help must not probe a backend');
        assert.equal(route.request().method(), 'GET');
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
      const atFragment = async id => page.waitForFunction(id => {
        const heading = document.getElementById(id);
        return heading && document.activeElement === heading && heading.getBoundingClientRect().top >= 0
          && heading.getBoundingClientRect().top < innerHeight;
      }, id);
      await page.goto(`${origin}/docs/AUTO_UPDATE`, { waitUntil: 'domcontentloaded' });
      await page.locator('article a[href="/docs/RELEASE_PROCESS"]').waitFor();
      await page.evaluate(() => window.__releaseHelpShell = 'same-shell');
      await activate(page.locator('article a[href="/docs/RELEASE_PROCESS"]'));
      await page.waitForURL('**/docs/RELEASE_PROCESS');
      await page.getByRole('heading', { name: 'Daily betas', exact: true }).waitFor();
      assert.equal(await page.evaluate(() => window.__releaseHelpShell), 'same-shell');
      const text = (await page.locator('article').innerText()).replace(/\s+/g, ' ');
      for (const phrase of ['every 24 hours', 'when there are changes since the previous beta',
        'No beta is cut within a day', 'no soak and is never promoted to stable',
        'A fix merged after a beta was cut is not in that beta',
        'only when a stable release containing it ships', 'not a selected patch release or a testing exemption',
        'seven days after the last stable release', 'one day earlier',
        'reserving the full 24-hour soak', 'Preparation and qualification can move publication later',
        'Do not repeat a guest freeze',
        'if the problem persists, reply on the original issue']) assert(text.includes(phrase), phrase);
      assert(!text.includes('urgent hotfix exception'));
      assert(!text.includes('Fourteen days after the last stable release'));
      record.checks.push('shipped update-guide entry; daily-beta scope, cut/inclusion, stable separation, weekly stable target, early eligibility, unchanged candidate checks and safe retest');
      await page.goto(`${origin}/docs/RELEASE_PROCESS#daily-betas`, { waitUntil: 'domcontentloaded' });
      await atFragment('daily-betas'); await capture('betas');
      await page.goto(`${origin}/docs/RELEASE_PROCESS#how-releases-are-prepared`, { waitUntil: 'domcontentloaded' });
      await atFragment('how-releases-are-prepared'); await capture('preparation');
      await page.goto(`${origin}/docs/RELEASE_PROCESS#cadence-and-stable-promotion`, { waitUntil: 'domcontentloaded' });
      await atFragment('cadence-and-stable-promotion'); await capture('promotion');
      await page.goto(`${origin}/docs/RELEASE_PROCESS#taking-part-in-preview-testing`, { waitUntil: 'domcontentloaded' });
      await atFragment('taking-part-in-preview-testing'); await capture('safe-testing');
      // Direct fragment loads above start new documents. Check subsequent
      // cross-guide routing against this document, not an earlier page load.
      await page.evaluate(() => window.__releaseHelpShell = 'same-shell');
      for (const [href, heading] of [
        ['/docs/MIGRATION#full-state-recovery', 'full-state-recovery'],
        ['/docs/DEPLOYMENT_MODELS#updates-by-model', 'updates-by-model'],
        ['/docs/TROUBLESHOOTING#collect-diagnostics-safely', 'collect-diagnostics-safely'],
      ]) {
        await activate(page.locator(`article a[href="${href}"]`));
        await page.waitForURL(`**${href}`); await atFragment(heading);
        assert.equal(await page.evaluate(() => window.__releaseHelpShell), 'same-shell');
        await page.goBack(); await page.waitForURL('**/docs/RELEASE_PROCESS#taking-part-in-preview-testing');
        await atFragment('taking-part-in-preview-testing');
      }
      record.checks.push('keyboard/touch backup, deployment-limits and safe-evidence links; focused destinations; browser back');
      await activate(page.getByRole('link', { name: '← All documentation', exact: true }));
      await page.waitForURL('**/docs/README');
      await activate(page.getByRole('link', { name: 'Releases and update channels', exact: true }));
      await page.waitForURL('**/docs/RELEASE_PROCESS');
      await page.getByRole('heading', { name: 'Daily betas', exact: true }).waitFor();
      record.checks.push('documentation index entry remains reachable');
      await context.close(); await browser.close(); browser = null;
    }
    assert.equal(result.errors.length, 0); result.result = 'passed';
  } catch (error) { result.failure = error.stack; throw error; }
  finally {
    if (browser) await browser.close(); result.cleanup.browser_closed = true;
    if (server) await new Promise((resolve, reject) => {
      server.close(error => error ? reject(error) : resolve()); server.closeAllConnections();
    }); result.cleanup.server_closed = true;
    fs.writeFileSync(path.join(output, 'result.json'), JSON.stringify(result, null, 2) + '\n');
  }
})().catch(error => { console.error(error); process.exitCode = 1; });
