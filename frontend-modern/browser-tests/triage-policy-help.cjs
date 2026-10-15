// Real Docs/Markdown/router/styles with synthetic, read-only local delivery.
// No GitHub action, update, diagnostic request or workload is exercised here.
const fs = require('node:fs');
const path = require('node:path');
const http = require('node:http');
const crypto = require('node:crypto');
const assert = require('node:assert/strict');
const { chromium, webkit } = require('playwright');
const root = '/workspace/frontend-modern';
const preview = '/workspace/tmp/doc-command-help/static';
const output = '/workspace/tmp/triage-policy-help';
const digest = bytes => crypto.createHash('sha256').update(bytes).digest('hex');
const bindingBytes = fs.readFileSync(path.join(preview, 'binding.json'));
const binding = JSON.parse(bindingBytes);
const result = { result: 'incomplete', source_sha: binding.source_sha,
  binding_sha256: digest(bindingBytes), playwright: require('playwright/package.json').version,
  cases: [], captures: [], errors: [], served: [], cleanup: {},
  limits: 'Shipped help rendering/navigation only; not the GitHub renderer, issue disposition, release publication or installed recovery.' };

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
    assert.equal(binding.artifacts['docs/AI_TRANSPARENCY.md'], binding.sources['public/docs/AI_TRANSPARENCY.md']);
    assert.equal(binding.artifacts['docs/AI_TRANSPARENCY.md'], digest(fs.readFileSync('/workspace/docs/AI_TRANSPARENCY.md')));
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
      const capture = async (suffix, locator) => {
        await locator.evaluate(element => element.scrollIntoView({ block: 'start', behavior: 'instant' }));
        await page.evaluate(async () => {
          await Promise.all(document.getAnimations().filter(animation =>
            animation.effect?.getComputedTiming().iterations !== Infinity)
            .map(animation => animation.finished.catch(() => {})));
          await new Promise(resolve => requestAnimationFrame(() => requestAnimationFrame(resolve)));
        });
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
      await page.goto(`${origin}/docs/AI_TRANSPARENCY#how-changes-land-and-ship`, { waitUntil: 'domcontentloaded' });
      await atFragment('how-changes-land-and-ship');
      const text = (await page.locator('article').innerText()).replace(/\s+/g, ' ');
      for (const phrase of ['seven days after the last stable release', 'one day earlier',
        'full 24-hour soak', 'Preparation and qualification can move publication later',
        'Usually, the 14 days start with that stable-release retest reply',
        'earlier retest request named a beta or release candidate carrying the same fix',
        "later of that request and the stable release's publication",
        'contains the fix for every reported symptom',
        'A beta or release candidate alone never starts this period',
        'another unanswered reminder is not required', 'not confirmed resolution',
        'an unfixed or unexplained symptom', 'unknown details are valid and credentials stay private'])
        assert(text.includes(phrase), phrase);
      assert(!text.includes('Every 14 days after the last stable release'));
      await capture('weekly-target', page.locator('article p').filter({ hasText: 'The train targets stable publication' }));
      await page.evaluate(() => window.__triageHelpShell = 'same-shell');
      await activate(page.locator('article a[href="/docs/RELEASE_PROCESS"]'));
      await page.waitForURL('**/docs/RELEASE_PROCESS');
      const releases = (await page.locator('article').innerText()).replace(/\s+/g, ' ');
      assert(releases.includes('seven days after the last stable release'));
      assert(releases.includes('one day earlier'));
      assert.equal(await page.evaluate(() => window.__triageHelpShell), 'same-shell');
      await page.goBack(); await atFragment('how-changes-land-and-ship');
      record.checks.push('direct fragment focus; weekly target and delay agree with release help; keyboard/touch link and browser back preserve the shell');

      await page.goto(`${origin}/docs/AI_TRANSPARENCY#issue-resolution-and-retesting`, { waitUntil: 'domcontentloaded' });
      await atFragment('issue-resolution-and-retesting');
      await capture('ordinary-retest', page.locator('#issue-resolution-and-retesting'));
      await capture('preview-clock', page.locator('article p').filter({ hasText: 'If an earlier retest request named a beta' }));
      await capture('timeout-and-reopen', page.locator('article p').filter({ hasText: 'Only after at least 14 days' }));
      record.checks.push('normal stable reply and earlier-preview exception; full-symptom inclusion, no preview-only timeout, other-human recurrence/exclusions and reopening remain visible');
      await page.evaluate(() => window.__triageHelpShell = 'retest-shell');
      await activate(page.locator('article a[href="/docs/TROUBLESHOOTING#collect-diagnostics-safely"]'));
      await page.waitForURL('**/docs/TROUBLESHOOTING#collect-diagnostics-safely');
      await atFragment('collect-diagnostics-safely');
      assert.equal(await page.evaluate(() => window.__triageHelpShell), 'retest-shell');
      await page.goBack(); await atFragment('issue-resolution-and-retesting');
      record.checks.push('keyboard/touch existing safe-evidence link, focused destination and browser back; no live diagnostic run');
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
