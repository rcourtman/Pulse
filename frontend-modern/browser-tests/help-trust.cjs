// Actual compiled Docs/router/Markdown and Provider & Models/state/API/CSS.
// All service data, saved settings and the extra link matrix are safe local fixtures.
const fs = require('node:fs');
const path = require('node:path');
const http = require('node:http');
const crypto = require('node:crypto');
const assert = require('node:assert/strict');
const { chromium, webkit } = require('playwright');
const root = '/workspace/frontend-modern';
const preview = '/workspace/tmp/help-trust/static';
const output = '/workspace/tmp/help-trust/browser';
const digest = bytes => crypto.createHash('sha256').update(bytes).digest('hex');
const bindingBytes = fs.readFileSync(path.join(preview, 'binding.json'));
const binding = JSON.parse(bindingBytes);
const result = { result: 'incomplete', source_sha: binding.source_sha,
  binding_sha256: digest(bindingBytes), playwright: require('playwright/package.json').version,
  cases: [], captures: [], errors: [], requests: [], served: [], cleanup: {},
  limits: 'Synthetic offline settings only. No real provider, backend budget check, usage deletion, customer save, Patrol execution or published recovery.' };

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
    assert.equal(binding.artifacts['docs/AI.md'], digest(fs.readFileSync('/workspace/docs/AI.md')));
    result.source_inputs = Object.keys(binding.sources).length;
    const probe = [
      '# Navigation fixture', '## Target', 'A focused destination.',
      '## Привет 你好', 'Encoded fragment destination.',
      ...Array.from({ length: 30 }, (_, i) => `Read-only fixture paragraph ${i + 1}. Leave the heading above the viewport before following the ordinary links below.`),
      '[File return](NAVIGATION_FIXTURE.md#target)',
      '[Bare return](#target)',
      '[Encoded return](#%D0%BF%D1%80%D0%B8%D0%B2%D0%B5%D1%82-%E4%BD%A0%E5%A5%BD)',
      '[Missing target](#missing)', '[Malformed target](#%invalid)',
      '[External help](https://example.invalid/help)',
      '<script>window.__unsafeDocScript=true</script><img src="/inert.svg" onerror="window.__unsafeDocHandler=true">',
    ].join('\n\n');
    assets.set('docs/NAVIGATION_FIXTURE.md', Buffer.from(probe));
    result.synthetic_navigation_fixture_sha256 = digest(Buffer.from(probe));
    let settings;
    const emptySettings = () => ({ enabled: false, configured: false, model: '', custom_context: '',
      auth_method: 'api_key', oauth_connected: false, configured_providers: [],
      anthropic_configured: false, openai_configured: false, openrouter_configured: false,
      deepseek_configured: false, gemini_configured: false, ollama_configured: false,
      ollama_base_url: 'http://localhost:11434', ollama_keep_alive: '',
      cost_budget_usd_30d: 0, request_timeout_seconds: 300, patrol_auto_fix: false,
      control_level: 'read_only' });
    const capabilities = { capabilities: ['ai_patrol'], limits: [], max_history_days: 7,
      runtime: { build: 'community', label: 'Pulse Community runtime' } };
    server = http.createServer(async (req, res) => {
      try {
        const pathname = new URL(req.url, 'http://fixture.invalid').pathname;
        const json = value => {
          res.writeHead(200, { 'Content-Type': 'application/json', 'Cache-Control': 'no-store',
            'Set-Cookie': 'pulse_csrf=offline-fixture; SameSite=Strict; Path=/' });
          res.end(JSON.stringify(value));
        };
        if (pathname.startsWith('/api/')) {
          result.requests.push({ method: req.method, pathname });
          if (req.method === 'PUT' && pathname === '/api/settings/ai/update') {
            const chunks = []; for await (const chunk of req) chunks.push(chunk);
            const payload = JSON.parse(Buffer.concat(chunks));
            assert.deepEqual(Object.keys(payload).sort(), ['cost_budget_usd_30d', 'model']);
            assert.equal(payload.model, '');
            assert([0, 60].includes(payload.cost_budget_usd_30d));
            result.requests[result.requests.length - 1].payload = payload;
            await new Promise(resolve => setTimeout(resolve, 250));
            settings = { ...settings, ...payload }; return json(settings);
          }
          assert.equal(req.method, 'GET', 'no model, reset or diagnostic mutation');
          if (pathname === '/api/settings/ai') return json(settings);
          if (pathname === '/api/license/runtime-capabilities') return json(capabilities);
          if (pathname === '/api/ai/patrol/model-guidance') return json({ rules: [] });
          if (pathname === '/api/health') return json({ status: 'ok' });
          if (pathname === '/api/security/status') return json({ sessionCapabilities: { assistantEnabled: false } });
          throw new Error(`Unexpected API fixture read: ${pathname}`);
        }
        assert.equal(req.method, 'GET');
        if (pathname === '/inert.svg') {
          res.writeHead(200, { 'Content-Type': 'image/svg+xml' });
          res.end('<svg xmlns="http://www.w3.org/2000/svg" width="1" height="1"/>'); return;
        }
        let file = pathname.slice(1);
        if (!assets.has(file)) file = 'browser-tests/help-trust.html';
        const bytes = assets.get(file);
        result.served.push({ pathname, file, sha256: digest(bytes) });
        const type = ({ '.md': 'text/markdown', '.css': 'text/css', '.js': 'text/javascript', '.html': 'text/html' })[path.extname(file)];
        res.writeHead(200, { 'Content-Type': `${type}; charset=utf-8`, 'Cache-Control': 'no-store' }); res.end(bytes);
      } catch (error) { result.errors.push(String(error)); res.writeHead(500); res.end('Fixture rejected request'); }
    });
    await new Promise((resolve, reject) => { server.once('error', reject); server.listen(0, '127.0.0.1', resolve); });
    const origin = `http://127.0.0.1:${server.address().port}`;
    for (const [name, engine, width, dark] of [
      ['desktop-chromium', chromium, 1360, false],
      ['phone-chromium', chromium, 390, true], ['phone-webkit', webkit, 390, true],
    ]) {
      settings = emptySettings();
      browser = await engine.launch(engine === chromium
        ? { headless: true, channel: 'chromium', args: ['--no-sandbox'] } : { headless: true });
      const context = await browser.newContext({ viewport: { width, height: 900 },
        isMobile: width < 500, hasTouch: width < 500, locale: 'en-GB', colorScheme: dark ? 'dark' : 'light' });
      const page = await context.newPage(); page.setDefaultTimeout(10000);
      const record = { name, width, height: 900, dark, browser: browser.version(), checks: [], fragments: [] };
      result.cases.push(record);
      await context.route('**/*', route => {
        const url = new URL(route.request().url());
        // External destinations are inert interception fixtures, never remote reads.
        if (url.href === 'https://example.invalid/help') return route.fulfill({
          status: 200, contentType: 'text/html', body: '<title>Offline external destination</title>' });
        assert.equal(url.origin, origin, 'external request not allowed');
        assert(['GET', 'PUT'].includes(route.request().method()));
        if (route.request().method() === 'PUT') assert.equal(url.pathname, '/api/settings/ai/update');
        return route.continue();
      });
      page.on('pageerror', error => result.errors.push(error.message));
      page.on('console', message => { if (message.type() === 'error') result.errors.push(message.text()); });
      page.on('response', response => { if (!response.ok()) result.errors.push(`HTTP ${response.status()}: ${response.url()}`); });
      await page.addInitScript(dark => document.addEventListener('DOMContentLoaded', () =>
        document.documentElement.classList.toggle('dark', dark)), dark);
      const activate = async link => {
        await link.scrollIntoViewIfNeeded();
        if (width < 500) await link.tap(); else { await link.focus(); await page.keyboard.press('Enter'); }
      };
      const atFragment = async id => {
        await page.waitForFunction(id => {
          const heading = document.getElementById(id);
          return heading && document.activeElement === heading && heading.getBoundingClientRect().top >= 0
            && heading.getBoundingClientRect().top < innerHeight;
        }, id);
        record.fragments.push(await page.locator(`[id="${id}"]`).evaluate(element => ({
          id: element.id, top: element.getBoundingClientRect().top,
          focused: document.activeElement === element, hash: location.hash,
        })));
      };
      const capture = async suffix => {
        await page.evaluate(() => new Promise(resolve => requestAnimationFrame(() => requestAnimationFrame(resolve))));
        assert(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth), 'page overflow');
        const file = `${name}-${suffix}.png`;
        await page.screenshot({ path: path.join(output, file), animations: 'disabled' });
        result.captures.push({ file, sha256: digest(fs.readFileSync(path.join(output, file))), name,
          state: await page.evaluate(() => ({ href: location.pathname + location.hash,
            scroll_y: scrollY, focused_id: document.activeElement?.id })) });
      };
      // Exact shipped guide: default entry, actual existing self-link, repeat,
      // report route and Back. No test-driven scroll correction after activation.
      await page.goto(`${origin}/docs/AI`, { waitUntil: 'domcontentloaded' });
      await page.getByRole('heading', { level: 1, name: 'Pulse Intelligence' }).waitFor();
      await page.evaluate(() => { window.__helpTrustShell = 'retained'; });
      const costLink = () => page.locator('article a[href="#cost-tracking"]').last();
      await activate(costLink()); await atFragment('cost-tracking');
      const historyLength = await page.evaluate(() => history.length);
      const guideReads = () => result.served.filter(x => x.pathname === '/docs/AI.md').length;
      const readsBeforeRepeat = guideReads();
      await activate(costLink()); await atFragment('cost-tracking');
      assert.equal(await page.evaluate(() => history.length), historyLength);
      assert.equal(guideReads(), readsBeforeRepeat);
      await capture('repeated-cost-link');
      record.checks.push('default shipped entry; actual repeated bare Cost Tracking link restores visible heading and focus without history or fetch churn');
      await activate(page.locator('article a[href="/docs/TROUBLESHOOTING#-getting-help"]').last());
      await atFragment('-getting-help');
      assert.equal(await page.evaluate(() => window.__helpTrustShell), 'retained');
      await page.goBack(); await atFragment('cost-tracking');
      await page.reload({ waitUntil: 'domcontentloaded' }); await atFragment('cost-tracking');
      record.checks.push('cross-document report help, retained shell, Back, deep link and reload focus after asynchronous content arrives');

      await page.goto(`${origin}/docs/NAVIGATION_FIXTURE#target`, { waitUntil: 'domcontentloaded' });
      await atFragment('target');
      await page.evaluate(() => { window.__helpTrustShell = 'retained'; });
      for (const label of ['File return', 'Bare return']) {
        const link = page.getByRole('link', { name: label, exact: true });
        await link.scrollIntoViewIfNeeded();
        assert(await page.locator('#target').evaluate(element => element.getBoundingClientRect().top < 0));
        const length = await page.evaluate(() => history.length);
        await activate(link); await atFragment('target');
        assert.equal(await page.evaluate(() => history.length), length);
      }
      await capture('repeated-file-link');
      await activate(page.getByRole('link', { name: 'Encoded return' })); await atFragment('привет-你好');
      for (const label of ['Missing target', 'Malformed target']) {
        await activate(page.getByRole('link', { name: label }));
        assert.equal(await page.locator('article').count(), 1);
      }
      assert.equal(await page.evaluate(() => window.__unsafeDocScript || window.__unsafeDocHandler || false), false);
      assert.equal(await page.locator('article script, article [onerror]').count(), 0);
      const external = page.getByRole('link', { name: 'External help' });
      assert.equal(await external.getAttribute('data-doc-link'), null);
      const popupPromise = context.waitForEvent('page');
      await activate(external); const popup = await popupPromise;
      await popup.waitForLoadState('domcontentloaded');
      assert.equal(await popup.title(), 'Offline external destination');
      assert.equal(await popup.evaluate(() => window.opener), null); await popup.close();
      assert.equal(await page.evaluate(() => window.__helpTrustShell), 'retained');
      if (width >= 1024) {
        const link = page.getByRole('link', { name: 'File return' });
        await link.scrollIntoViewIfNeeded();
        // Observe the actual event after the article handler; suppress only the
        // browser default so this matrix does not open four redundant tabs.
        await page.evaluate(() => {
          window.__modifiedEvents = [];
          document.addEventListener('click', event => {
            if (event.ctrlKey || event.metaKey || event.shiftKey || event.altKey || event.button === 1) {
              window.__modifiedEvents.push({ prevented: event.defaultPrevented, hash: location.hash });
              event.preventDefault();
            }
          });
        });
        for (const key of ['ctrlKey', 'metaKey', 'shiftKey', 'altKey'])
          await link.dispatchEvent('click', { button: 0, [key]: true });
        await link.dispatchEvent('click', { button: 1 });
        const events = await page.evaluate(() => window.__modifiedEvents);
        assert.equal(events.length, 5); assert(events.every(event => !event.prevented));
      }
      record.checks.push('synthetic file and bare repeats, encoded fragments, absent/malformed targets, sanitiser unchanged, isolated external tab; desktop modifiers/middle remain native');

      // Actual Provider & Models, actual parser/state/save, synthetic API only.
      await page.goto(`${origin}/settings?section=system-ai`, { waitUntil: 'domcontentloaded' });
      const budget = page.getByRole('spinbutton', { name: '30-day Budget' }); await budget.waitFor();
      const descriptionId = await budget.getAttribute('aria-describedby');
      assert.equal(descriptionId, 'ai-cost-budget-help');
      const helper = page.locator(`#${descriptionId}`);
      const checkHelper = async () => {
        const text = (await helper.innerText()).replace(/\s+/g, ' ');
        for (const phrase of ['refuses new AI calls, including Patrol', 'preceding 30 days',
          'Set 0 or leave empty to disable this check', 'Missing pricing or untracked usage',
          'if no pricing is known, calls are allowed', 'not a provider spending cap']) assert(text.includes(phrase), phrase);
        assert(!text.includes('receive usage alerts'));
      };
      await checkHelper();
      assert.equal(await budget.inputValue(), '');
      await budget.scrollIntoViewIfNeeded(); await capture('budget-disabled');
      await budget.fill('0'); await checkHelper();
      await budget.fill('60'); await checkHelper();
      await page.getByText('≈ $2.00/day', { exact: true }).waitFor();
      await budget.scrollIntoViewIfNeeded(); await capture('budget-positive');
      const save = () => page.getByRole('button', { name: 'Save provider settings', exact: true });
      const savePositive = page.waitForResponse(response => response.url().endsWith('/api/settings/ai/update') && response.request().method() === 'PUT');
      await activate(save());
      assert.equal(await budget.isDisabled(), true);
      assert.equal((await savePositive).ok(), true);
      await save().waitFor(); await page.waitForFunction(() => !document.getElementById('ai-cost-budget').disabled);
      await page.reload({ waitUntil: 'domcontentloaded' }); await budget.waitFor();
      assert.equal(await budget.inputValue(), '60'); await checkHelper();
      await budget.fill(''); await checkHelper();
      const saveEmpty = page.waitForResponse(response => response.url().endsWith('/api/settings/ai/update') && response.request().method() === 'PUT');
      await activate(save()); assert.equal((await saveEmpty).ok(), true);
      await save().waitFor();
      await page.reload({ waitUntil: 'domcontentloaded' }); await budget.waitFor();
      assert.equal(await budget.inputValue(), ''); await checkHelper();
      assert.equal(settings.cost_budget_usd_30d, 0);
      record.checks.push('actual Provider & Models: associated helper at empty/zero/positive, daily estimate, saving disabled, exact synthetic positive and empty-to-zero saves survive reload; no provider/reset call');
      await context.close(); await browser.close(); browser = null;
    }
    assert.equal(result.errors.length, 0); result.result = 'passed';
  } catch (error) { result.failure = String(error.stack).slice(0, 5000); throw error; }
  finally {
    if (browser) await browser.close(); result.cleanup.browser_closed = true;
    if (server) await new Promise((resolve, reject) => {
      server.close(error => error ? reject(error) : resolve()); server.closeAllConnections();
    }); result.cleanup.server_closed = true;
    fs.writeFileSync(path.join(output, 'result.json'), JSON.stringify(result, null, 2) + '\n');
    console.log(JSON.stringify({ result: result.result, source: result.source_sha, cases: result.cases.length,
      captures: result.captures.length, errors: result.errors, cleanup: result.cleanup, failure: result.failure }));
  }
})().catch(error => { console.error(error); process.exitCode = 1; });
