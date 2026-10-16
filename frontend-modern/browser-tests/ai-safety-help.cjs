// Changed help through the real shipped Docs/router/Markdown/styles.
// Only the outer shell is synthetic; no model, action or infrastructure runs.
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const http = require('node:http');
const crypto = require('node:crypto');
const { chromium } = require('playwright');
const root = '/workspace';
const preview = path.join(root, 'tmp/doc-command-help/static');
const output = path.join(root, 'tmp/ai-safety-help');
const hash = bytes => crypto.createHash('sha256').update(bytes).digest('hex');
const bindingBytes = fs.readFileSync(path.join(preview, 'binding.json'));
const binding = JSON.parse(bindingBytes);
const result = { source: binding.source_sha, binding_sha256: hash(bindingBytes),
  recipe_sha256: hash(fs.readFileSync(__filename)), cases: [], captures: [],
  errors: [], served: [], requests: [], cleanup: {},
  limits: 'Help content and navigation only, not runtime safety qualification, action execution or recovery. Chromium desktop/phone; the earlier WebKit reload failure remains separate and uncleared.' };

(async () => {
  let server, browser;
  try {
    fs.mkdirSync(output, { recursive: true });
    result.playwright = require('playwright/package.json').version;
    assert.equal(result.playwright, '1.56.1');
    assert.equal(result.playwright, JSON.parse(fs.readFileSync(path.join(root,
      'tests/integration/package-lock.json'))).packages['node_modules/@playwright/test'].version);
    for (const [file, expected] of Object.entries(binding.sources))
      assert.equal(hash(fs.readFileSync(path.join(root, 'frontend-modern', file))), expected, file);
    const assets = new Map();
    for (const [file, expected] of Object.entries(binding.artifacts)) {
      const bytes = fs.readFileSync(path.join(preview, file));
      assert.equal(hash(bytes), expected, file); assets.set(file, bytes);
    }
    for (const name of ['AI_AUTONOMY', 'AI', 'ASSISTANT_SAFETY'])
      assert.equal(binding.artifacts[`docs/${name}.md`], hash(fs.readFileSync(path.join(root, 'docs', `${name}.md`))));
    server = http.createServer((req, res) => {
      if (req.method !== 'GET') { res.writeHead(405); res.end(); return; }
      const route = new URL(req.url, 'http://fixture.invalid').pathname;
      const asset = route.startsWith('/docs/') && !route.endsWith('.md')
        ? 'browser-tests/doc-command-help.html' : route.slice(1);
      if (!assets.has(asset)) { res.writeHead(404); res.end(); return; }
      const bytes = assets.get(asset);
      result.served.push({ route, asset, sha256: hash(bytes) });
      const type = { '.md': 'text/markdown', '.css': 'text/css', '.js': 'text/javascript', '.html': 'text/html' }[path.extname(asset)];
      res.writeHead(200, { 'Content-Type': `${type}; charset=utf-8`, 'Cache-Control': 'no-store' }); res.end(bytes);
    });
    await new Promise((resolve, reject) => {
      server.once('error', reject); server.listen(0, '127.0.0.1', resolve);
    });
    const origin = `http://127.0.0.1:${server.address().port}`;
    browser = await chromium.launch({ headless: true, channel: 'chromium', args: ['--no-sandbox'] });
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
        assert.equal(route.request().method(), 'GET'); return route.continue();
      });
      await page.addInitScript(dark => document.addEventListener('DOMContentLoaded', () =>
        document.documentElement.classList.toggle('dark', dark)), state.dark);
      const activate = async link => {
        if (state.width < 500) await link.tap(); else { await link.focus(); await page.keyboard.press('Enter'); }
      };
      const fragment = id => page.waitForFunction(id => document.activeElement?.id === id, id);
      const capture = async (name, locator) => {
        await locator.evaluate(el => window.scrollBy(0, el.getBoundingClientRect().top - 40));
        await page.evaluate(() => new Promise(resolve => requestAnimationFrame(() => requestAnimationFrame(resolve))));
        assert(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1));
        const file = `${state.name}-${name}.png`; await page.screenshot({ path: path.join(output, file) });
        result.captures.push({ file, sha256: hash(fs.readFileSync(path.join(output, file))), state });
      };
      await page.goto(`${origin}/docs/README`, { waitUntil: 'domcontentloaded' });
      await activate(page.getByRole('link', { name: 'Patrol modes and safety', exact: true }));
      await page.waitForURL('**/docs/AI_AUTONOMY');
      await page.locator('article').waitFor();
      const plain = (await page.locator('article').innerText()).replace(/\s+/g, ' ');
      for (const phrase of ['not a guarantee of a correct diagnosis', 'Chat does not execute those plans',
        'not a proof that every other operation is safe', 'intentionally stop or restart a workload',
        'does not reverse an action already dispatched', 'Execution and verification are separate results',
        'Confirmed by independent observer', 'Confirmed by executing agent', 'Outcome not verified',
        'Outcome inconclusive', 'Outcome contradicted', 'postcondition at its observation time',
        'not current health', 'Do not repeat an uncertain operation', 'quiet timeline does not prove that nothing ran'])
        assert(plain.includes(phrase), phrase);
      assert(!plain.includes('enforced by the FSM'));
      assert(!plain.includes('cannot return to idle'));
      await capture('planning', page.locator('article p').first());
      await page.evaluate(() => window.__safetyHelpShell = 'same-shell');
      await activate(page.getByRole('link', { name: 'what the guardrails establish', exact: true }));
      await fragment('safety-guardrails');
      assert.equal(await page.evaluate(() => window.__safetyHelpShell), 'same-shell');
      await capture('guardrails', page.locator('#safety-guardrails'));
      await capture('verification', page.locator('#verification-after-write'));
      const table = page.locator('article table').filter({ hasText: 'Recorded observation' });
      await table.waitFor(); assert.equal(await table.locator('tbody tr').count(), 6);
      const scroll = table.locator('..');
      assert.equal(await scroll.getAttribute('data-doc-table-scroll'), '');
      const dimensions = await scroll.evaluate(el => ({ width: el.clientWidth, scroll: el.scrollWidth }));
      // The real renderer confines any wide table; it must not widen the page.
      if (dimensions.scroll > dimensions.width) {
        await scroll.hover(); await page.mouse.wheel(dimensions.scroll, 0);
      }
      await capture('outcome-table', scroll);
      for (const cell of await table.locator('td').all())
        assert((await cell.textContent()).trim().length > 0);
      await activate(page.getByRole('link', { name: 'Assistant safety architecture', exact: true }));
      await page.waitForURL('**/docs/ASSISTANT_SAFETY');
      await page.getByRole('heading', { name: 'Pulse Assistant safety architecture', exact: true }).waitFor();
      await page.goBack({ waitUntil: 'domcontentloaded' }); await fragment('safety-guardrails');
      await page.reload({ waitUntil: 'domcontentloaded' }); await fragment('safety-guardrails');
      await page.goto(`${origin}/docs/AI`, { waitUntil: 'domcontentloaded' });
      await activate(page.getByRole('link', { name: 'Assistant safety architecture', exact: true }).last());
      await page.waitForURL('**/docs/ASSISTANT_SAFETY');
      await page.getByRole('heading', { name: 'Pulse Assistant safety architecture', exact: true }).waitFor();
      for (const name of ['AI_AUTONOMY', 'AI', 'ASSISTANT_SAFETY']) {
        const response = await page.request.get(`${origin}/docs/${name}.md`);
        assert.equal(response.status(), 200);
        assert.equal(hash(await response.body()), binding.sources[`public/docs/${name}.md`]);
      }
      result.cases.push({ ...state, browser: browser.version(),
        interaction: state.width < 500 ? 'touch' : 'keyboard', indexAndSafetyLinks: true,
        exactSafetyLimitsAndLabels: true, table: dimensions, fragmentFocusBackAndReload: true,
        noDocumentOverflow: true, exactFetchedGuides: ['AI_AUTONOMY', 'AI', 'ASSISTANT_SAFETY'] });
      await context.close();
    }
    assert.deepEqual(result.errors, []); result.result = 'passed';
  } catch (error) {
    result.result = 'failed'; result.error = String(error.stack || error); process.exitCode = 1;
  } finally {
    if (browser) { await browser.close(); result.cleanup.browser = true; }
    if (server) { await new Promise(resolve => server.close(resolve)); result.cleanup.server = true; }
    fs.writeFileSync(path.join(output, 'result.json'), JSON.stringify(result, null, 2) + '\n');
    console.log(JSON.stringify(result));
  }
})().catch(error => { console.error(error); process.exitCode = 1; });
