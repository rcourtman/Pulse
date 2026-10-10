const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const crypto = require('node:crypto');
const { chromium, webkit } = require('playwright');
const root = '/workspace/frontend-modern';
const output = '/workspace/tmp/assistant-controls-proof/browser';
const hash = (f) => crypto.createHash('sha256').update(fs.readFileSync(f)).digest('hex');
(async () => {
  fs.mkdirSync(output, { recursive: true });
  const binding = JSON.parse(fs.readFileSync('/workspace/tmp/assistant-controls-proof/browser-binding.json'));
  for (const [file, digest] of Object.entries(binding.content_sha256)) assert.equal(hash(path.join('/workspace', file)), digest);
  const version = require('playwright/package.json').version;
  assert.equal(version, JSON.parse(fs.readFileSync('/workspace/tests/integration/package-lock.json')).packages['node_modules/@playwright/test'].version);
  process.chdir(root);
  const { createServer } = await import(path.join(root, 'node_modules/vite/dist/node/index.js'));
  const server = await createServer({ root, configFile: path.join(root, 'vite.config.ts'), cacheDir: path.join(output, 'vite-cache'), server: { host: '127.0.0.1', port: 5308, strictPort: true, watch: null } });
  const origin = 'http://127.0.0.1:5308';
  const results = [], screenshots = [], browserVersions = [];
  let browser, phase = 'server', browserClosed = false, serverClosed = false;
  const result = () => ({ result: 'passed', base_sha: binding.base_sha, content_sha256: binding.content_sha256, playwrightVersion: version, browserVersions, results, screenshots, browserClosed, serverClosed, scope: 'Actual Settings state/save, Chat menus/scoped context and Docs with synthetic local API transport. No model/action/native persistence or deployed/released acceptance.' });
  try {
    await server.listen();
    for (const [name, engine, width, height, dark] of [ ['desktop-light', chromium, 1365, 900, false], ['phone-dark', webkit, 390, 844, true], ['narrow-light', webkit, 320, 740, false] ]) {
      browserClosed = false;
      browser = await engine.launch(engine === chromium ? { headless: true, channel: 'chromium', args: ['--no-sandbox'] } : { headless: true });
      browserVersions.push({ name, engine: engine === chromium ? 'chromium' : 'webkit', version: browser.version() });
      const page = await browser.newPage({ viewport: { width, height }, isMobile: width < 500, hasTouch: width < 500, locale: 'en-GB' });
      page.setDefaultTimeout(8000); page.setDefaultNavigationTimeout(60000);
      const errors = [], offOrigin = [], writes = [], checks = [], reads = [];
      let saved = 'autonomous', failSave = false;
      await page.addInitScript((dark) => { localStorage.clear(); const apply = () => document.documentElement.classList.toggle('dark', dark); if (document.documentElement) apply(); else document.addEventListener('DOMContentLoaded', apply, { once: true }); }, dark);
      page.on('pageerror', (e) => errors.push(e.message));
      await page.routeWebSocket(/.*/, (s) => s.close());
      const settings = () => ({ enabled: true, configured: false, model: '', custom_context: '', control_level: saved, protected_guests: ['vm-101'], patrol_autonomy_level: 'full', patrol_action_emergency_stop: true, configured_providers: [], providers: [], discovery_enabled: false, request_timeout_seconds: 300 });
      await page.route('**/*', async (route) => {
        const req = route.request(), url = new URL(req.url());
        if (url.origin !== origin) { offOrigin.push(url.origin); return route.abort(); }
        if (!url.pathname.startsWith('/api/')) return route.continue();
        if (req.method() !== 'GET') {
          const entry = { method: req.method(), path: url.pathname, payload: req.postDataJSON() }; writes.push(entry);
          if (url.pathname !== '/api/settings/ai/update' || req.method() !== 'PUT') return route.fulfill({ status: 403, json: { error: 'Fixture refuses inference, actions and unrelated writes' } });
          if (failSave) return route.fulfill({ status: 503, json: { error: 'Synthetic save unavailable' } });
          if (Object.hasOwn(entry.payload, 'control_level')) saved = entry.payload.control_level;
          return route.fulfill({ json: { ...settings(), control_level: ['autonomous','suggest'].includes(saved) ? 'controlled' : saved } });
        }
        reads.push(url.pathname);
        if (url.pathname === '/api/settings/ai') return route.fulfill({ json: settings() });
        if (url.pathname === '/api/security/status') return route.fulfill({ json: { hasAuthentication: true, requiresAuth: false, sessionCapabilities: { assistantEnabled: true } } });
        if (url.pathname.includes('license') || url.pathname.includes('runtime-capabilities')) return route.fulfill({ json: { capabilities: [], entitlements: [], limits: [] } });
        if (url.pathname === '/api/ai/models') return route.fulfill({ json: { models: [] } });
        if (url.pathname === '/api/ai/status') return route.fulfill({ json: { enabled: true, configured: false, running: true } });
        if (url.pathname.endsWith('/sessions')) return route.fulfill({ json: { sessions: [] } });
        if (url.pathname === '/api/agent/capabilities') return route.fulfill({ json: { version: 1, capabilities: [], surfaces: [], workflowPrompts: [], mcpAdapter: {}, operatorSurfaces: [] } });
        return route.fulfill({ json: [] });
      });
      const check = async (label, action) => { phase = `${name}: ${label}`; await action(); checks.push({ label, passed: true }); };
      const capture = async (state) => { const file = `${name}-${state}.png`; await page.screenshot({ path: path.join(output, file) }); screenshots.push({ file, sha256: hash(path.join(output, file)), state, viewport: { width, height } }); };
      const load = async (surface) => page.goto(`${origin}/browser-tests/assistant-control-compatibility.html?surface=${surface}`);
      try {
        for (const level of ['autonomous', 'suggest', 'controlled', 'read_only', 'unknown']) {
          saved = level; await load('settings');
          const select = page.getByLabel('Chat action mode', { exact: true });
          const expected = ['autonomous','suggest','controlled'].includes(level) ? 'controlled' : 'read_only';
          await check(`Settings ${level} projects without granting execution`, async () => {
            await select.waitFor(); assert.equal(await select.inputValue(), expected);
            assert.equal(await select.locator('option').count(), 2);
            const text = await page.locator('main').innerText();
            assert.ok(!text.includes('chat-only') && !text.includes('Infrastructure changes stay with Patrol'));
            if (expected === 'controlled') { assert.ok(text.includes('Chat does not execute the plan')); const legacy = page.getByLabel('Protected guests (legacy)', { exact: true }); assert.equal(await legacy.inputValue(), 'vm-101'); assert.equal(await legacy.getAttribute('aria-describedby'), 'ai-protected-guests-help'); assert.ok(text.includes('This list does not exclude saved action plans.')); assert.ok(text.includes('Review each plan’s target and approval policy in Actions.')); }
            else assert.ok(text.includes('Assistant can query and explain only. It cannot plan infrastructure actions.'));
            assert.ok(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1));
          });
          if (level === 'autonomous' || level === 'read_only') { await select.scrollIntoViewIfNeeded(); await capture(level === 'read_only' ? 'read-only-settings' : 'legacy-settings'); }
          const before = writes.length;
          await page.getByRole('button', { name: 'Save Assistant settings', exact: true }).click();
          await check(`Unrelated save preserves ${level} and scoped policy`, async () => {
            await page.waitForFunction(() => !document.querySelector('button[disabled]')?.textContent?.includes('Saving'));
            for (let i = 0; writes.length === before && i < 20; i++) await page.waitForTimeout(50);
            assert.equal(writes.length, before + 1);
            for (const key of ['control_level','protected_guests','patrol_autonomy_level','patrol_action_emergency_stop']) assert.ok(!Object.hasOwn(writes.at(-1).payload, key));
            assert.equal(saved, level);
          });
        }
        saved = 'autonomous'; await load('chat');
        const trigger = page.getByRole('button', { name: 'Assistant chat action mode: Ask first', exact: true });
        await check('Legacy Chat has two modes and keyboard-safe review-only copy', async () => {
          await trigger.waitFor(); assert.equal(await page.getByRole('status', { name: 'Assistant chat actions warning' }).count(), 0);
          await trigger.focus(); await page.keyboard.press('ArrowDown');
          const modes = page.getByRole('menuitemradio'); assert.equal(await modes.count(), 2);
          const ask = page.getByRole('menuitemradio', { name: /Plans actions for your review/ });
          assert.equal(await ask.getAttribute('aria-checked'), 'true');
          await page.keyboard.press('Home'); assert.ok(await modes.first().evaluate((el) => el === document.activeElement));
          await page.keyboard.press('ArrowUp'); assert.ok(await ask.evaluate((el) => el === document.activeElement));
          const bounds = await page.getByRole('menu').boundingBox();
          const composer = await page.locator('[data-assistant-control-toolbar]').boundingBox();
          assert.ok(bounds.x >= composer.x - 1 && bounds.x + bounds.width <= composer.x + composer.width + 1);
          assert.ok(bounds.x >= 0 && bounds.x + bounds.width <= width + 1);
          await capture('chat-menu');
          await page.keyboard.press('Escape'); await page.waitForFunction(() => document.activeElement?.getAttribute('aria-label') === 'Assistant chat action mode: Ask first');
          assert.equal(await page.getByRole('menu').count(), 0);
        });
        await check('Viewport resize closes the menu and reopened bounds fit', async () => {
          await trigger.click();
          await page.setViewportSize({ width: width + 1, height });
          await page.getByRole('menu').waitFor({ state: 'hidden' });
          await page.setViewportSize({ width, height }); await trigger.click();
          const bounds = await page.getByRole('menu').boundingBox();
          assert.ok(bounds.x >= 0 && bounds.x + bounds.width <= width + 1);
          await page.keyboard.press('Escape');
        });
        await check('Scoped alert attachment keeps identity and approval boundary', async () => {
          await page.evaluate(() => window.__assistantProof.scoped());
          await page.getByText('Approval required before any action.', { exact: true }).waitFor();
          assert.equal(await page.evaluate(() => window.__assistantProof.context().autonomousMode), false);
          assert.equal(await page.evaluate(() => window.__assistantProof.context().handoffResources[0].id), 'vm:lab:101');
          assert.ok(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1));
          await capture('scoped-chat');
        });
        await check('Chat mode save failure retains current mode', async () => {
          failSave = true; await trigger.click();
          await page.getByRole('menuitemradio', { name: /Read-only/ }).click();
          await trigger.waitFor(); assert.equal(saved, 'autonomous'); failSave = false;
        });
        await check('Explicit Chat opt-out changes only the mode', async () => {
          await trigger.click(); await page.getByRole('menuitemradio', { name: /Read-only/ }).click();
          await page.getByRole('button', { name: 'Assistant chat action mode: Read-only', exact: true }).waitFor();
          assert.deepEqual(writes.at(-1).payload, { control_level: 'read_only' });
          assert.equal(saved, 'read_only');
        });
        saved = 'controlled'; await load('settings');
        await page.getByLabel('Chat action mode', { exact: true }).waitFor();
        for (const [link, heading, state] of [ ['Read Assistant modes','Control Levels','help-modes'], ['Read control guide','Assistant Control Levels','help-guide'] ]) {
          if (state === 'help-guide') { await load('settings'); await page.getByLabel('Chat action mode', { exact: true }).waitFor(); }
          await check(`Actual Docs ${heading} describes planning and independent verification`, async () => {
            await page.getByRole('link', { name: link, exact: true }).click();
            const h = page.getByRole('heading', { name: heading, exact: true }); await h.waitFor(); await h.scrollIntoViewIfNeeded();
            const text = await page.locator('article').innerText(); assert.ok(text.includes('Assistant chat does not execute'));
            assert.ok(text.includes('stored') && text.includes('entitlement') && text.includes('independent verification') && text.includes('Protected guests (legacy)') && text.includes('does not exclude saved action plans'));
            assert.ok(!text.includes('executes actions without prompting') && !text.includes('executes commands without prompting'));
            assert.ok(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1));
            await capture(state);
            const tableScroll = await page.locator('article').evaluate((article) => {
              const table = [...article.querySelectorAll('table')].find((t) => t.innerText.includes('Availability') && t.innerText.includes('Ask first'));
              if (!table) return { found: false };
              const wrapper = table.parentElement;
              const overflows = wrapper.scrollWidth > wrapper.clientWidth + 1;
              const overflowX = getComputedStyle(wrapper).overflowX;
              if (overflows) wrapper.scrollLeft = wrapper.scrollWidth;
              const lastCell = table.querySelector('thead tr').lastElementChild;
              return { found: true, overflows, overflowX, scrollLeft: wrapper.scrollLeft, lastCellRight: lastCell.getBoundingClientRect().right, wrapperRight: wrapper.getBoundingClientRect().right };
            });
            assert.equal(tableScroll.found, true);
            if (tableScroll.overflows) {
              assert.equal(tableScroll.overflowX, 'auto'); assert.ok(tableScroll.scrollLeft > 0);
              assert.ok(tableScroll.lastCellRight <= tableScroll.wrapperRight + 1);
              await capture(`${state}-table-end`);
            }
          });
        }
        assert.deepEqual(errors, []); assert.deepEqual(offOrigin, []);
        assert.ok(writes.every((r) => r.method === 'PUT' && r.path === '/api/settings/ai/update'));
        results.push({ name, viewport: { width, height }, dark, checks, writes, errors, offOrigin });
      } catch (error) {
        fs.writeFileSync(path.join(output, 'debug.json'), JSON.stringify({ phase, errors, reads, body: await page.locator('body').innerText(), buttons: await page.getByRole('button').allTextContents() }, null, 2));
        await page.screenshot({ path: path.join(output, 'failed-view.png') });
        throw error;
      } finally { await page.close(); await browser.close(); browser = null; browserClosed = true; }
    }
  } catch (error) {
    fs.writeFileSync(path.join(output, 'failure.json'), JSON.stringify({ phase, error: error.message, results, screenshots }, null, 2));
    throw error;
  } finally {
    if (browser) { await browser.close(); browserClosed = true; }
    await server.close(); serverClosed = true;
  }
  fs.writeFileSync(path.join(output, 'result.json'), JSON.stringify(result(), null, 2) + '\n');
  console.log(JSON.stringify({ result: 'passed', groups: results.reduce((n,r) => n + r.checks.length, 0), screenshots: screenshots.length, browserClosed, serverClosed }));
})().catch((e) => { console.error(e); process.exitCode = 1; });
