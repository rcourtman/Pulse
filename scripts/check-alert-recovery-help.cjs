// Consume the real Docs/router/Markdown/styles and the shipped help bytes.
// Build the existing doc-command-help preview in the source-proof VM first.
// This checks help/navigation, not alert execution or reporter recovery.
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const http = require('node:http');
const crypto = require('node:crypto');
const { chromium } = require('playwright');

const root = process.cwd();
const preview = path.join(root, 'tmp/doc-command-help/static');
const output = path.join(root, 'tmp/alert-recovery-help');
const hash = bytes => crypto.createHash('sha256').update(bytes).digest('hex');
const fragment = 'a-usage-alert-stays-open-after-usage-drops';
const bindingBytes = fs.readFileSync(path.join(preview, 'binding.json'));
const binding = JSON.parse(bindingBytes);
const result = {
  source: binding.source_sha, binding_sha256: hash(bindingBytes),
  script_sha256: hash(fs.readFileSync(__filename)), cases: [], captures: [],
  errors: [], requests: [], served: [], cleanup: {},
  limits: 'Help and navigation only. Real Docs consumer with a synthetic outer shell; no backend, alert, guest, backup, notification or recovery operation.',
};

(async () => {
  let server, browser;
  fs.mkdirSync(output, { recursive: true });
  try {
    result.playwright = require('playwright/package.json').version;
    assert.equal(result.playwright, '1.56.1');
    assert.equal(result.playwright, JSON.parse(fs.readFileSync(path.join(root,
      'tests/integration/package-lock.json'))).packages['node_modules/@playwright/test'].version);
    for (const [file, expected] of Object.entries(binding.sources))
      assert.equal(hash(fs.readFileSync(path.join(root, 'frontend-modern', file))), expected, file);
    const assets = new Map();
    for (const [file, expected] of Object.entries(binding.artifacts)) {
      const bytes = fs.readFileSync(path.join(preview, file));
      assert.equal(hash(bytes), expected, file);
      assets.set(file, bytes);
    }
    for (const name of ['TROUBLESHOOTING', 'CONFIGURATION', 'VM_DISK_MONITORING'])
      assert.equal(binding.artifacts[`docs/${name}.md`], hash(fs.readFileSync(path.join(root, 'docs', `${name}.md`))));
    server = http.createServer((req, res) => {
      if (req.method !== 'GET') { res.writeHead(405); res.end(); return; }
      const route = new URL(req.url, 'http://fixture.invalid').pathname;
      const file = route.startsWith('/docs/') && !route.endsWith('.md')
        ? 'browser-tests/doc-command-help.html' : route.slice(1);
      if (!assets.has(file)) { res.writeHead(404); res.end(); return; }
      const bytes = assets.get(file);
      result.served.push({ route, file, sha256: hash(bytes) });
      const type = { '.md': 'text/markdown', '.css': 'text/css', '.js': 'text/javascript', '.html': 'text/html' }[path.extname(file)];
      res.writeHead(200, { 'Content-Type': `${type}; charset=utf-8`, 'Cache-Control': 'no-store' });
      res.end(bytes);
    });
    await new Promise((resolve, reject) => {
      server.once('error', reject); server.listen(0, '127.0.0.1', resolve);
    });
    const origin = `http://127.0.0.1:${server.address().port}`;
    browser = await chromium.launch({ headless: true, channel: 'chromium', args: ['--no-sandbox'] });
    result.chromium = browser.version();
    for (const state of [
      { name: 'desktop-light', width: 1440, height: 1000, dark: false },
      { name: 'phone-dark', width: 320, height: 900, dark: true },
    ]) {
      const context = await browser.newContext({ viewport: { width: state.width, height: state.height },
        isMobile: state.width < 500, hasTouch: state.width < 500, locale: 'en-GB' });
      const page = await context.newPage(); page.setDefaultTimeout(12000);
      page.on('pageerror', error => result.errors.push(error.message));
      page.on('response', response => { if (!response.ok()) result.errors.push(`HTTP ${response.status()}`); });
      page.on('request', request => result.requests.push({ method: request.method(), url: request.url() }));
      await page.route('**/*', route => {
        assert.equal(new URL(route.request().url()).origin, origin);
        assert(!new URL(route.request().url()).pathname.startsWith('/api/'));
        assert.equal(route.request().method(), 'GET');
        return route.continue();
      });
      await page.addInitScript(dark => document.addEventListener('DOMContentLoaded', () =>
        document.documentElement.classList.toggle('dark', dark)), state.dark);
      const activate = async link => {
        if (state.width < 500) await link.tap(); else { await link.focus(); await page.keyboard.press('Enter'); }
      };
      const focus = id => page.waitForFunction(id => document.activeElement?.id === id, id);
      const capture = async (name, locator) => {
        await locator.evaluate(el => window.scrollBy(0, el.getBoundingClientRect().top - 32));
        await page.evaluate(() => new Promise(resolve => requestAnimationFrame(() => requestAnimationFrame(resolve))));
        assert(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1));
        const file = `${state.name}-${name}.png`;
        await page.screenshot({ path: path.join(output, file) });
        result.captures.push({ file, sha256: hash(fs.readFileSync(path.join(output, file))), state });
      };
      await page.goto(`${origin}/docs/TROUBLESHOOTING`, { waitUntil: 'domcontentloaded' });
      await activate(page.getByRole('link', { name: 'the affected reading, clear level and recovery delay', exact: true }));
      await focus(fragment);
      const heading = page.locator(`#${fragment}`);
      const text = await heading.evaluate(node => {
        let value = '';
        for (let next = node.nextElementSibling; next && next.tagName !== 'H4'; next = next.nextElementSibling)
          value += ` ${next.textContent}`;
        return value.replace(/\s+/g, ' ');
      });
      for (const phrase of ['filesystem or mountpoint', 'not measured recovery',
        '90% trigger and 85% clear', '88% can leave an alert open', 'unresolved symptom',
        'include only details not already supplied', 'Unknown settings or times can remain unknown',
        'Do not acknowledge, disable monitoring, alter thresholds, restart Pulse or delete history'])
        assert(text.includes(phrase), phrase);
      await capture('identity', heading);
      await capture('clear-level', page.getByText('Read the saved clear level and timing.', { exact: true }));
      await capture('safety', page.locator('article p').filter({ hasText: /^Do not acknowledge, disable monitoring/ }));
      await activate(page.getByRole('link', { name: 'threshold and timing explanation', exact: true }));
      await focus('metric-thresholds-off-and-inheritance');
      assert((await page.locator('article').innerText()).includes('An already active alert stays active above 80%'));
      await capture('linked-thresholds', page.locator('#metric-thresholds-off-and-inheritance'));
      await page.goBack(); await focus(fragment);
      await activate(page.getByRole('link', { name: 'current and retained reading guidance', exact: true }));
      await focus('current-retained-and-unavailable-disk-readings');
      await page.goBack(); await focus(fragment);
      await page.reload(); await focus(fragment);
      for (const name of ['TROUBLESHOOTING', 'CONFIGURATION', 'VM_DISK_MONITORING']) {
        const response = await page.request.get(`${origin}/docs/${name}.md`);
        assert.equal(response.status(), 200);
        assert.equal(hash(await response.body()), binding.sources[`public/docs/${name}.md`]);
      }
      result.cases.push({ ...state, interaction: state.width < 500 ? 'touch' : 'keyboard',
        indexAndCrossDocumentLinks: true, fragmentFocusBackAndReload: true,
        sourceAndFetchedBytesMatch: true, safetyAndRecoveryDistinctions: true, noDocumentOverflow: true });
      await context.close();
    }
    assert.deepEqual(result.errors, []);
    result.result = 'passed';
  } catch (error) {
    result.result = 'failed'; result.error = String(error.stack || error); process.exitCode = 1;
  } finally {
    if (browser) { await browser.close(); result.cleanup.browser = true; }
    if (server) { await new Promise(resolve => server.close(resolve)); result.cleanup.server = true; }
    fs.writeFileSync(path.join(output, 'result.json'), JSON.stringify(result, null, 2) + '\n');
    console.log(JSON.stringify(result));
  }
})().catch(error => { console.error(error); process.exitCode = 1; });
