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
const output = '/workspace/tmp/cost-budget-help';
const digest = bytes => crypto.createHash('sha256').update(bytes).digest('hex');
const bindingBytes = fs.readFileSync(path.join(preview, 'binding.json'));
const binding = JSON.parse(bindingBytes);
const result = { result: 'incomplete', source_sha: binding.source_sha,
  binding_sha256: digest(bindingBytes), playwright: require('playwright/package.json').version,
  cases: [], captures: [], errors: [], served: [], cleanup: {},
  limits: 'Existing cost help rendering/navigation only; no provider call, paid charge, history reset, budget enforcement repair or Patrol recovery.' };

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
    assert.equal(binding.artifacts['docs/AI.md'], binding.sources['public/docs/AI.md']);
    assert.equal(binding.artifacts['docs/AI.md'], digest(fs.readFileSync('/workspace/docs/AI.md')));
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
      await page.goto(`${origin}/docs/AI#cost-tracking`, { waitUntil: 'domcontentloaded' });
      await atFragment('cost-tracking');
      const text = (await page.locator('article').innerText()).replace(/\s+/g, ' ');
      for (const phrase of ['not a provider invoice or a guaranteed spending cap',
        'Select 30d', 'preceding 30 days, not a calendar month',
        'changing the chart range does not change the runtime budget',
        'a pricing correction does not require deleting those events',
        'total tokens alone cannot establish the expected cost',
        'Some cost is unavailable, not free', 'partial total',
        'resetting a circuit breaker does not repair a budget refusal',
        'Raising or disabling the budget permits more spend',
        'Reset usage history clears retained events used by the budget check',
        'It does not refund provider charges', 'tasks, resources and sessions',
        'no raw usage history, invoice, full export, credential, new diagnostic run or paid model call is required'])
        assert(text.includes(phrase), phrase);
      assert(!text.includes('Set monthly budget limits'));
      await capture('window-and-estimate', page.locator('#cost-tracking'));
      await capture('distinct-causes', page.locator('#a-budget-refusal-is-not-a-provider-failure'));
      const table = page.locator('#a-budget-refusal-is-not-a-provider-failure + div, #a-budget-refusal-is-not-a-provider-failure + table').first();
      assert.equal(await table.locator('tbody tr').count(), 4);
      if (width < 500) {
        // Real Docs wraps tables in a horizontal scroll container: text must
        // remain reachable, not clipped or force whole-page overflow.
        const scroll = page.locator('article .overflow-x-auto').filter({ has: page.locator('table').filter({ hasText: 'Existing observation' }) });
        assert.equal(await scroll.count(), 1);
        assert(await scroll.evaluate(element => {
          element.scrollLeft = element.scrollWidth;
          return element.scrollWidth <= element.clientWidth || element.scrollLeft > 0;
        }), 'narrow decision table must be readable or horizontally scrollable');
        await scroll.evaluate(element => { element.scrollLeft = 0; });
      }
      await capture('preserve-history', page.locator('#preserve-usage-history-and-keep-exports-private'));
      record.checks.push('exact shipped cost guide; focused deep link; rolling versus chart window; four distinct causes; prominent reset and export warnings; no page overflow; narrow table reachable');
      await page.evaluate(() => window.__costHelpShell = 'same-shell');
      await activate(page.locator('article a[href="/docs/TROUBLESHOOTING#-getting-help"]').last());
      await page.waitForURL('**/docs/TROUBLESHOOTING#-getting-help');
      await atFragment('-getting-help');
      assert.equal(await page.evaluate(() => window.__costHelpShell), 'same-shell');
      await page.goBack();
      await page.waitForURL('**/docs/AI#cost-tracking');
      await atFragment('cost-tracking');
      // The existing troubleshooting table must point back to this guidance.
      await page.locator('article a[href="/docs/AI#cost-tracking"]').last().evaluate(element => element.scrollIntoView({ block: 'center' }));
      await activate(page.locator('article a[href="/docs/AI#cost-tracking"]').last());
      await atFragment('cost-tracking');
      record.checks.push('keyboard/touch existing-report help and Back preserve shell and heading focus; troubleshooting link returns to cost section; only local GETs and no backend requests');
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
