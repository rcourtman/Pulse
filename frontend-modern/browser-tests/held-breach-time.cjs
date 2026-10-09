const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const crypto = require('node:crypto');
const { chromium, webkit } = require('playwright');
const root = '/workspace/frontend-modern';
const output = '/workspace/tmp/held-breach-proof/browser';
const runtime = [
  'frontend-modern/src/types/api.ts',
  'frontend-modern/src/features/alerts/metricAlertPresentation.ts',
  'frontend-modern/src/components/Alerts/alertAssistantHandoffModel.ts',
];
const hash = (file) => crypto.createHash('sha256').update(fs.readFileSync(file)).digest('hex');
(async () => {
  fs.mkdirSync(output, { recursive: true });
  const binding = JSON.parse(fs.readFileSync('/workspace/tmp/held-breach-proof/binding.json'));
  const hashes = Object.fromEntries(runtime.map((f) => [f, hash(path.join('/workspace', f))]));
  assert.deepEqual(hashes, binding.content_sha256);
  const version = require('playwright/package.json').version;
  assert.equal(
    version,
    JSON.parse(fs.readFileSync('/workspace/tests/integration/package-lock.json')).packages[
      'node_modules/@playwright/test'
    ].version,
  );
  process.chdir(root);
  const { createServer } = await import(path.join(root, 'node_modules/vite/dist/node/index.js'));
  const server = await createServer({
    root,
    configFile: path.join(root, 'vite.config.ts'),
    cacheDir: path.join(output, 'vite-cache'),
    server: { host: '127.0.0.1', port: 5298, strictPort: true, watch: null },
  });
  const origin = 'http://127.0.0.1:5298';
  const results = [],
    screenshots = [];
  let browser,
    phase = 'server';
  try {
    await server.listen();
    for (const [name, engine, width, height, dark] of [
      ['chromium-desktop-light', chromium, 1365, 900, false],
      ['chromium-desktop-dark', chromium, 1365, 900, true],
      ['webkit-phone-dark', webkit, 390, 844, true],
      ['webkit-narrow-light', webkit, 320, 740, false],
    ]) {
      phase = name;
      browser = await engine.launch(
        engine === chromium
          ? { headless: true, channel: 'chromium', args: ['--no-sandbox'] }
          : { headless: true },
      );
      const page = await browser.newPage({
        viewport: { width, height },
        isMobile: width < 500,
        hasTouch: width < 500,
        locale: 'en-GB',
        timezoneId: 'UTC',
      });
      page.setDefaultTimeout(8000);
      page.setDefaultNavigationTimeout(60000);
      await page.clock.setFixedTime(new Date('2026-10-06T10:52:00Z'));
      const errors = [],
        requests = [],
        offOrigin = [],
        writes = [],
        checks = [];
      page.on('pageerror', (error) => errors.push(error.message));
      await page.addInitScript((dark) => {
        localStorage.clear();
        const apply = () => document.documentElement.classList.toggle('dark', dark);
        if (document.documentElement) apply();
        else document.addEventListener('DOMContentLoaded', apply, { once: true });
      }, dark);
      await page.routeWebSocket(/.*/, (socket) => socket.close());
      await page.route('**/*', async (route) => {
        const req = route.request(),
          url = new URL(req.url());
        if (url.origin !== origin) {
          offOrigin.push(url.origin);
          return route.abort();
        }
        if (!url.pathname.startsWith('/api/')) return route.continue();
        requests.push({ path: url.pathname, method: req.method() });
        if (req.method() !== 'GET') {
          writes.push({ path: url.pathname, method: req.method() });
          return route.fulfill({
            status: 403,
            json: { error: 'Fixture refuses every write/inference/action' },
          });
        }
        if (url.pathname === '/api/alerts/history') return route.fulfill({ json: [] });
        if (url.pathname === '/api/alerts/incidents') return route.fulfill({ json: null });
        if (url.pathname === '/api/security/status')
          return route.fulfill({
            json: {
              hasAuthentication: true,
              requiresAuth: false,
              sessionCapabilities: { assistantEnabled: true },
            },
          });
        if (url.pathname.includes('license'))
          return route.fulfill({
            json: { capabilities: [], entitlements: [], limits: [], max_history_days: 90 },
          });
        if (url.pathname === '/api/settings/ai')
          return route.fulfill({
            json: { enabled: true, control_level: 'controlled', providers: [], model: '' },
          });
        if (url.pathname === '/api/ai/models') return route.fulfill({ json: { models: [] } });
        if (url.pathname === '/api/ai/status')
          return route.fulfill({ json: { enabled: true, configured: false } });
        if (url.pathname.endsWith('/sessions')) return route.fulfill({ json: { sessions: [] } });
        return route.fulfill({ json: [] });
      });
      const check = async (label, action) => {
        phase = `${name}: ${label}`;
        await action();
        checks.push({ label, passed: true });
      };
      const capture = async (state, locator = page) => {
        const file = `${name}-${state}.png`,
          target = path.join(output, file);
        await locator.screenshot({ path: target, ...(locator === page ? { fullPage: true } : {}) });
        screenshots.push({ file, state, viewport: { width, height }, sha256: hash(target) });
      };
      try {
        await page.goto(`${origin}/browser-tests/held-breach-time.html`);
        const attention = page.getByRole('region', { name: 'Existing node attention' });
        const overview = page.getByRole('region', { name: 'Existing alert overview' });
        const history = page.getByRole('region', { name: 'Existing alert history' });
        const breachPrefix = 'Last reading at or above 80°C: 80°C';
        const assertBreach = async (expected) => {
          const p = attention
            .locator('p')
            .filter({ hasText: /^Temperature|^Last reading: Temperature/ })
            .first();
          assert.equal((await p.getAttribute('title')).split('\n').at(-1), expected);
          assert.equal(
            await overview
              .locator('p[title]')
              .filter({ hasText: /^Temperature|^Last reading: Temperature/ })
              .first()
              .getAttribute('title'),
            expected,
          );
          if (width < 500)
            await history.getByText(expected, { exact: true }).waitFor({ state: 'visible' });
          else assert.ok((await history.locator('[title]').all()).length > 0);
        };
        await check(
          'status date beats newer legacy poll in attention, overview and History',
          async () => {
            await attention
              .getByText('Temperature 76°C now, back under the 80°C alert level', { exact: true })
              .waitFor();
            await assertBreach(`${breachPrefix}, 20 mins ago`);
            if (width >= 1024)
              assert.ok(
                (await history.locator('[title]').evaluateAll((es) => es.map((e) => e.title))).some(
                  (t) => t.includes('20 mins ago'),
                ),
              );
          },
        );
        await capture('latched');
        await check(
          'real Assistant handoff carries retained ISO evidence, live reading and approval boundary',
          async () => {
            await page
              .getByRole('region', { name: 'Existing Assistant handoff' })
              .getByRole('button')
              .click();
            const context = await page.evaluate(() => window.__heldBreach.context());
            assert.match(context.handoffContext, /Last Breach At: 2026-10-06T10:31:59\.123Z/);
            assert.match(context.handoffContext, /Current Value: 76\.0°C/);
            assert.match(context.handoffContext, /Last Reading At Or Above Threshold: 80\.0°C/);
            assert.match(context.handoffContext, /Operator Boundary:/);
            assert.equal(context.targetId, 'lab-minipc');
            assert.equal(context.autonomousMode, false);
            // Model-only evidence is checked above; production Chat deliberately
            // retains its compact attachment rather than exposing detail lines.
            await page
              .getByRole('button', { name: 'Render attached briefing without inference' })
              .click();
            const briefing = page.getByRole('region', { name: 'Assistant context' });
            await briefing.getByText('Warning temperature on minipc', { exact: true }).waitFor();
            assert.doesNotMatch(await briefing.textContent(), /Last reading at or above/);
            await capture('assistant', briefing);
            await page.evaluate(() => window.__heldBreach.close());
          },
        );
        await check(
          'recovery changes the current reading, not the retained breach date',
          async () => {
            await page.evaluate(() => window.__heldBreach.setScenario('recovering'));
            await attention
              .getByText('Temperature 72°C now, recovering', { exact: true })
              .waitFor();
            await assertBreach(`${breachPrefix}, 20 mins ago`);
            await attention
              .getByText('Clears after 5 minutes at 75°C or lower, 2 minutes so far.', {
                exact: true,
              })
              .waitFor();
          },
        );
        await check('invalid status date falls back to valid legacy date', async () => {
          await page.evaluate(() => window.__heldBreach.setScenario('legacy'));
          await assertBreach(`${breachPrefix}, 20 mins ago`);
        });
        await check(
          'Go-zero and unknown remain undated on every surface and in Assistant',
          async () => {
            await page.evaluate(() => window.__heldBreach.setScenario('unknown'));
            await assertBreach(breachPrefix);
            await page
              .getByRole('region', { name: 'Existing Assistant handoff' })
              .getByRole('button')
              .click();
            const context = await page.evaluate(() => window.__heldBreach.context());
            assert.doesNotMatch(context.handoffContext, /Last Breach At:/);
            await page.evaluate(() => window.__heldBreach.close());
          },
        );
        await capture('unknown');
        await check(
          'stale evaluation is not now, but still retains the correct breach age',
          async () => {
            await page.evaluate(() => window.__heldBreach.setScenario('stale'));
            await attention
              .getByText('Last reading: Temperature 76°C, 12 mins ago', { exact: true })
              .waitFor();
            await assertBreach(`${breachPrefix}, 20 mins ago`);
          },
        );
        await check(
          'restart-before-evaluation keeps the legacy message and fabricates no date',
          async () => {
            await page.evaluate(() => window.__heldBreach.setScenario('restart'));
            await attention.getByText('Node temperature at 80.0°C', { exact: true }).waitFor();
            assert.equal(await overview.locator('[title^="Last reading at or above"]').count(), 0);
            await page
              .getByRole('region', { name: 'Existing Assistant handoff' })
              .getByRole('button')
              .click();
            const context = await page.evaluate(() => window.__heldBreach.context());
            assert.doesNotMatch(context.handoffContext, /Last Breach At:/);
            assert.ok(context.briefing.detailLines.includes('Message: Node temperature at 80.0°C'));
            await page.evaluate(() => window.__heldBreach.close());
          },
        );
        await check('reload retains final-byte date selection', async () => {
          await page.goto(`${origin}/browser-tests/held-breach-time.html?case=legacy`);
          await assertBreach(`${breachPrefix}, 20 mins ago`);
        });
        await check(
          'no horizontal page overflow, JavaScript errors, external contacts or writes',
          async () => {
            assert.equal(
              await page.evaluate(() => document.documentElement.scrollWidth > innerWidth + 1),
              false,
            );
            assert.deepEqual(errors, []);
            assert.deepEqual(offOrigin, []);
            assert.deepEqual(writes, []);
          },
        );
        results.push({
          name,
          engine: engine === chromium ? 'chromium' : 'webkit',
          browserVersion: browser.version(),
          width,
          height,
          dark,
          checks,
          requests,
          errors,
          writes,
          offOrigin,
        });
      } finally {
        await page.close();
        await browser.close();
        browser = undefined;
      }
    }
    assert.deepEqual(
      Object.fromEntries(runtime.map((f) => [f, hash(path.join('/workspace', f))])),
      hashes,
    );
    fs.writeFileSync(
      path.join(output, 'result.json'),
      JSON.stringify(
        {
          result: 'passed',
          version,
          binding,
          hashes,
          results,
          screenshots,
          verifiedAt: new Date().toISOString(),
          cleanup: { browserClosed: true, serverClosed: false },
        },
        null,
        2,
      ),
    );
  } catch (error) {
    fs.writeFileSync(
      path.join(output, 'failure.json'),
      JSON.stringify(
        { phase, message: error.message, stack: error.stack, results, screenshots },
        null,
        2,
      ),
    );
    throw error;
  } finally {
    if (browser) await browser.close();
    await server.close();
  }
  const file = path.join(output, 'result.json'),
    result = JSON.parse(fs.readFileSync(file));
  result.cleanup.serverClosed = true;
  fs.writeFileSync(file, JSON.stringify(result, null, 2));
  console.log(
    JSON.stringify({
      result: result.result,
      checkGroups: results.reduce((n, r) => n + r.checks.length, 0),
      screenshots: screenshots.length,
      cleanup: result.cleanup,
      runtime: hashes,
    }),
  );
})().catch((error) => {
  console.error(error);
  process.exitCode = 1;
});
