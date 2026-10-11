// Run the identical matrix on current-parent and repaired bytes. No external I/O.
const assert = require('node:assert/strict');
const fs = require('node:fs');
const crypto = require('node:crypto');
const { chromium, webkit } = require('playwright');
const start = Date.parse('2026-10-11T17:00:00Z');
const origin = 'http://127.0.0.1:5351';
const hash = (p) => crypto.createHash('sha256').update(fs.readFileSync(p)).digest('hex');

(async () => {
  const binding = JSON.parse(fs.readFileSync('/workspace/tmp/pr2600/binding.json'));
  const root = '/workspace/' + binding.frontend_root;
  const output = '/workspace/tmp/pr2600/' + binding.mode;
  fs.mkdirSync(output, { recursive: true });
  for (const [p, h] of Object.entries(binding.content_sha256))
    assert.equal(hash(root + '/' + p.replace('frontend-modern/', '')), h, p);
  const result = {
    binding,
    playwright: require('playwright/package.json').version,
    cases: [],
    captures: [],
    cleanup: {},
    completed: false,
    limits:
      'Synthetic production Machines page and Debug component/derived state, not full application, physical phone, native collector, guest recovery, publication or release. Parent establishes residual defects only.',
  };
  assert.equal(
    result.playwright,
    JSON.parse(fs.readFileSync('/workspace/tests/integration/package-lock.json')).packages[
      'node_modules/@playwright/test'
    ].version,
  );
  const { createServer } = await import(root + '/node_modules/vite/dist/node/index.js');
  const server = await createServer({
    root,
    configFile: root + '/vite.config.ts',
    cacheDir: output + '/vite-cache',
    server: { host: '127.0.0.1', port: 5351, strictPort: true, watch: null },
    optimizeDeps: { noDiscovery: true, include: [] },
  });
  let browser, currentPage;
  try {
    await server.listen();
    for (const [name, engine, width, dark] of [
      ['desktop', chromium, 1680, false],
      ['phone', webkit, 390, true],
    ]) {
      browser = await engine.launch(
        engine === chromium
          ? { headless: true, channel: 'chromium', args: ['--no-sandbox'] }
          : { headless: true },
      );
      for (const surface of ['machines', 'debug']) {
        const page = await browser.newPage({
          viewport: { width, height: 950 },
          isMobile: width < 500,
          hasTouch: width < 500,
          locale: 'en-GB',
          timezoneId: 'UTC',
        });
        currentPage = page;
        page.setDefaultTimeout(10000);
        const record = {
          name,
          surface,
          browser: browser.version(),
          width,
          dark,
          reads: [],
          writes: [],
          unexpectedReads: [],
          offOrigin: [],
          errors: [],
          consoleErrors: [],
          checks: [],
        };
        result.cases.push(record);
        page.on('pageerror', (err) => record.errors.push(err.message));
        page.on('console', (message) => {
          if (message.type() === 'error') record.consoleErrors.push(message.text().slice(0, 600));
        });
        await page.clock.install({ time: new Date(start - 1000) });
        await page.clock.pauseAt(new Date(start));
        await page.addInitScript((dark) => {
          localStorage.clear();
          document.addEventListener(
            'DOMContentLoaded',
            () => document.documentElement.classList.toggle('dark', dark),
            { once: true },
          );
        }, dark);
        await page.route('**/*', async (route) => {
          const req = route.request(),
            url = new URL(req.url()),
            p = url.pathname;
          if (url.origin !== origin) {
            record.offOrigin.push(url.origin);
            return route.abort();
          }
          if (!p.startsWith('/api/')) return route.continue();
          if (req.method() !== 'GET') {
            record.writes.push({ p, method: req.method() });
            return route.fulfill({ status: 403, json: { error: 'Fixture refuses writes' } });
          }
          record.reads.push(p);
          let data;
          if (p === '/api/resources')
            data = {
              data: [
                {
                  id: 'fixture-agent',
                  name: 'Silent host',
                  displayName: 'Silent host',
                  type: 'agent',
                  platformId: 'fixture-agent',
                  platformType: 'agent',
                  sourceType: 'agent',
                  sources: ['agent'],
                  status: 'online',
                  lastSeen: new Date(start - 60_000).toISOString(),
                  cpu: { current: 24 },
                  identity: { hostname: 'fixture-host' },
                  agent: { agentId: 'fixture-agent', stale: false },
                },
              ],
              meta: { totalPages: 1 },
            };
          else if (p === '/api/agents/metadata') data = {};
          else if (p === '/api/security/status')
            data = { hasAuthentication: true, requiresAuth: false, sessionCapabilities: {} };
          else if (p === '/api/health') data = { status: 'ok' };
          else if (p === '/api/license/runtime-capabilities')
            data = {
              capabilities: [],
              limits: [],
              max_history_days: 7,
              hosted_mode: false,
              blocked_capabilities: [],
              runtime: { build: 'community' },
            };
          else {
            record.unexpectedReads.push(p);
            return route.fulfill({ status: 404, json: { error: 'Unexpected synthetic read' } });
          }
          return route.fulfill({ json: data });
        });
        const capture = async (state) => {
          const path = `${output}/${name}-${surface}-${state}.png`;
          await page.screenshot({ path, fullPage: true });
          result.captures.push({ path: path.replace('/workspace/', ''), sha256: hash(path) });
        };
        await page.goto(`${origin}/browser-tests/pr2600-monitoring.html?surface=${surface}`);
        if (surface === 'machines') {
          const row = page.locator('[data-agents-machine-row="fixture-agent"]');
          const dot = row.locator('span[title="online"]');
          const posture = page.getByTestId('standalone-posture-summary');
          await row.waitFor();
          await page.clock.runFor(20000); // Complete the ordinary initial canonical revalidation.
          await page.waitForFunction(() =>
            document
              .querySelector('[data-testid="standalone-posture-summary"]')
              ?.textContent.includes('reporting normally'),
          );
          assert.match(await dot.getAttribute('class'), /bg-emerald-500/);
          await row.evaluate((el) => {
            window.__originalRow = el;
          });
          const toggle = row.getByRole('button', { name: /details/i }).first();
          await toggle.focus();
          await capture('initial');
          const snapshot = await page.evaluate(() => window.__pr2600.snapshot());
          const reads = record.reads.length;
          await page.clock.runFor(220000); // The original report is exactly five minutes old.
          assert.match(await dot.getAttribute('class'), /bg-emerald-500/);
          await page.clock.runFor(30000);
          assert.match(await posture.innerText(), /1 machine needs attention/);
          const heldClass = await dot.getAttribute('class');
          assert.match(heldClass, binding.mode === 'parent' ? /bg-emerald-500/ : /bg-amber-500/);
          assert.equal(await page.evaluate(() => window.__pr2600.snapshot()), snapshot);
          assert.equal(record.reads.length, reads);
          assert.equal(await row.evaluate((el) => el === window.__originalRow), true);
          assert.equal(await toggle.evaluate((el) => el === document.activeElement), true);
          await capture('held');
          record.checks.push({
            heldClass,
            posture: await posture.innerText(),
            clockReads: record.reads.length - reads,
            originalSnapshotUnchanged: true,
            rowAndFocusRetained: true,
          });
          await page.evaluate(() => window.__pr2600.remount());
          assert.match(
            await row.locator('span[title="online"]').getAttribute('class'),
            /bg-amber-500/,
          );
          record.checks.push(
            'Same-instant remount reads stale; repaired retained row agrees with it.',
          );
        } else {
          const details = page.locator('details').first();
          const summary = details.locator('summary');
          await summary.waitFor();
          if (width < 500) await summary.tap();
          else {
            await summary.focus();
            await page.keyboard.press('Enter');
          }
          assert.equal(await details.evaluate((el) => el.open), true);
          await summary.focus();
          await details.evaluate((el) => {
            window.__originalDetails = el;
          });
          const payload = await details.locator('pre').innerText();
          await capture('initial');
          const snapshot = await page.evaluate(() => window.__pr2600.snapshot());
          const reads = record.reads.length;
          await page.clock.runFor(120000);
          assert.match(await summary.innerText(), /deferred.*3 mins ago/s);
          assert.equal(await page.evaluate(() => window.__pr2600.snapshot()), snapshot);
          await capture('held');
          await page.evaluate(() => window.__pr2600.updateStatus());
          assert.equal(await page.evaluate(() => window.__pr2600.sectionsRetained()), true);
          assert.equal(
            await details.evaluate((el) => el === window.__originalDetails && el.open),
            true,
          );
          assert.equal(await summary.evaluate((el) => el === document.activeElement), true);
          assert.equal(await details.locator('pre').innerText(), payload);
          const arrived = await summary.innerText();
          assert.match(
            arrived,
            binding.mode === 'parent' ? /deferred.*3 mins ago/s : /available.*0s ago/s,
          );
          assert.equal(
            await page.getByText('Guest read deferred', { exact: true }).count(),
            binding.mode === 'parent' ? 1 : 0,
          );
          assert.equal(
            await page.getByText('Agent unavailable', { exact: true }).count(),
            binding.mode === 'parent' ? 0 : 1,
          );
          await capture('arrived');
          record.checks.push({
            arrived,
            sectionsRetained: true,
            disclosureFocusPayloadRetained: true,
          });
          if (binding.mode !== 'parent') {
            await page.clock.runFor(60000);
            assert.match(await summary.innerText(), /available.*1 min ago/s);
            await page.evaluate(() => window.__pr2600.withdrawStatus());
            assert.equal(await summary.innerText(), 'Proxmox\ndeferred');
            assert.match(
              await page.locator('details').nth(1).locator('summary').innerText(),
              /Agent\s*unknown/,
            );
            assert.equal(await page.getByText('Agent unavailable', { exact: true }).count(), 0);
            record.checks.push(
              'New numeric/ISO timestamps age; invalid/withdrawn statuses do not invent age or retain error.',
            );
          }
          assert.equal(record.reads.length, reads);
        }
        assert.equal(
          await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth + 1),
          true,
          `${name}/${surface} page width`,
        );
        assert.deepEqual(record.writes, []);
        assert.deepEqual(record.unexpectedReads, []);
        assert.deepEqual(record.offOrigin, []);
        assert.deepEqual(record.errors, []);
        assert.deepEqual(record.consoleErrors, []);
        await page.close();
        currentPage = null;
      }
      await browser.close();
      browser = null;
    }
    result.completed = true;
  } finally {
    if (currentPage) await currentPage.close();
    if (browser) await browser.close();
    result.cleanup.browserClosed = true;
    await server.close();
    result.cleanup.serverClosed = true;
    fs.writeFileSync(output + '/result.json', JSON.stringify(result, null, 2));
  }
  process.stdout.write(
    JSON.stringify({
      completed: result.completed,
      mode: binding.mode,
      cases: result.cases.length,
      captures: result.captures.length,
      cleanup: result.cleanup,
    }) + '\n',
  );
})().catch((err) => {
  process.stderr.write(err.stack + '\n');
  process.exitCode = 1;
});
