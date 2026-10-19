// Production editor and HTTP transport; synthetic endpoints/credentials only.
process.env.RAYON_NUM_THREADS = '2';
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const crypto = require('node:crypto');
const { pathToFileURL } = require('node:url');
const { chromium, webkit, expect } = require('playwright/test');
const root = '/workspace';
const frontend = path.join(root, 'frontend-modern');
const output = path.join(root, 'tmp/node-connection-deadline');
const hash = (bytes) => crypto.createHash('sha256').update(bytes).digest('hex');
const binding = JSON.parse(fs.readFileSync(path.join(output, 'binding.json')));
for (const [file, digest] of Object.entries(binding.files))
  assert.equal(hash(fs.readFileSync(path.join(root, file))), digest, `Source changed: ${file}`);
const report = {
  binding,
  playwright: require('playwright/package.json').version,
  cases: [],
  captures: [],
  errors: [],
  cleanup: {},
};
let server, browser, fixture;
const held = new Set();
const delay = (ms) => new Promise((resolve) => setTimeout(resolve, ms));
const timeoutText = 'No connection result after 30 seconds. The server may still be testing.';

function serveFixture(req, res, next) {
  if (!req.url.startsWith('/api/')) return next();
  const current = fixture;
  if (!current) throw new Error('Unexpected API request outside a fixture');
  const body = [];
  req.on('data', (chunk) => body.push(chunk));
  req.on('end', () => {
    try {
      const text = Buffer.concat(body).toString();
      current.requests.push({ method: req.method, path: req.url, mode: current.mode });
      const respond = (status, value, headers = {}) => {
        res.writeHead(status, { 'Content-Type': 'application/json', ...headers });
        res.end(JSON.stringify(value));
      };
      if (req.url === '/api/health') {
        assert.equal(req.method, 'GET');
        assert.equal(current.mode, 'csrf-stall');
      } else {
        assert.equal(req.method, 'POST', 'Testing never saves, deletes or changes monitoring');
        assert.equal(req.headers['x-csrf-token'], 'synthetic-csrf');
        if (req.url === `/api/config/nodes/${current.type}-fixture/test`) {
          assert.equal(text, '', 'Saved credentials never consume draft inputs');
        } else {
          assert.equal(req.url, '/api/config/nodes/test-connection');
          const data = JSON.parse(text);
          assert.equal(data.password, 'synthetic-only-password');
          assert.equal(data.host, 'https://entered.example.invalid');
          assert.equal(data.fingerprint, 'CD:'.repeat(31) + 'CD');
          assert.equal(data.verifySSL, true);
          assert.equal(data.type, current.type);
        }
      }
      if (['headers-stall', 'body-stall', 'csrf-stall'].includes(current.mode)) {
        current.held = res;
        held.add(res);
        res.on('close', () => {
          current.closedResponses++;
          held.delete(res);
        });
        if (current.mode === 'body-stall') {
          res.writeHead(200, { 'Content-Type': 'application/json' });
          res.write('{"status":');
          res.flushHeaders();
        }
        return;
      }
      if (current.mode === 'forbidden')
        return respond(
          403,
          { error: 'Synthetic permission refusal' },
          { 'X-CSRF-Token': 'new-csrf' },
        );
      if (current.mode === 'mismatch')
        return respond(200, { status: 'error', message: 'Synthetic certificate mismatch' });
      assert.equal(current.mode, 'normal');
      respond(200, {
        status: 'success',
        message: 'Synthetic saved connection responded',
        warnings: ['Synthetic limited coverage'],
      });
    } catch (error) {
      report.errors.push(String(error.stack || error));
      res.writeHead(500);
      res.end('Fixture assertion failed');
    }
  });
}

(async () => {
  try {
    assert.equal(report.playwright, '1.56.1', 'Integration/browser runtime parity');
    const { createServer } = await import(
      pathToFileURL(path.join(frontend, 'node_modules/vite/dist/node/index.js')).href
    );
    server = await createServer({
      root: frontend,
      configFile: path.join(frontend, 'vite.config.ts'),
      logLevel: 'error',
      plugins: [
        {
          name: 'offline-connection-test-http',
          configureServer(s) {
            s.middlewares.use(serveFixture);
          },
        },
      ],
      server: { host: '127.0.0.1', port: 0, strictPort: false },
      optimizeDeps: { noDiscovery: true, include: ['solid-js', 'solid-js/web', '@solidjs/router'] },
    });
    await server.listen();
    const origin = `http://127.0.0.1:${server.httpServer.address().port}`;
    for (const [engine, viewport] of [
      ['chromium', { width: 1365, height: 1000 }],
      ['webkit', { width: 320, height: 1000 }],
    ]) {
      const narrow = viewport.width < 768;
      browser = await (narrow ? webkit : chromium).launch(
        narrow
          ? { headless: true }
          : { headless: true, channel: 'chromium', args: ['--no-sandbox'] },
      );
      const browserVersion = browser.version();
      const visit = async (type, csrf = true) => {
        fixture = { type, mode: 'headers-stall', requests: [], closedResponses: 0 };
        const current = fixture;
        const context = await browser.newContext({ viewport, hasTouch: narrow, isMobile: narrow });
        if (csrf)
          await context.addCookies([{ name: 'pulse_csrf', value: 'synthetic-csrf', url: origin }]);
        const page = await context.newPage();
        page.on('pageerror', (error) => report.errors.push(error.message));
        page.on('request', (request) => assert.equal(new URL(request.url()).origin, origin));
        await page.goto(`${origin}/browser-tests/node-fingerprint-validation.html?type=${type}`, {
          waitUntil: 'networkidle',
        });
        const now = new Date('2026-10-19T12:00:00Z');
        await page.clock.install({ time: now });
        await page.clock.pauseAt(now);
        const activate = async (locator) => {
          if (narrow) await locator.tap();
          else {
            await locator.focus();
            await page.keyboard.press('Enter');
          }
        };
        const result = page.getByRole('status', { name: 'Connection test result' });
        const test = page.getByRole('button', {
          name: /^(Test saved connection|Test entered settings|Testing…)$/,
        });
        const capture = async (state) => {
          await result.scrollIntoViewIfNeeded();
          await page.clock.runFor(40);
          await page.evaluate(() => document.fonts.ready);
          assert(
            await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth),
            'No horizontal page overflow',
          );
          const file = `${engine}-${type}-${state}.png`;
          await page.screenshot({ path: path.join(output, file), animations: 'disabled' });
          report.captures.push({
            file,
            engine,
            viewport,
            state,
            sha256: hash(fs.readFileSync(path.join(output, file))),
          });
        };
        const enterDraft = async () => {
          if (type !== 'pmg')
            await page.getByRole('radio', { name: 'Username & Password' }).check();
          await page.getByLabel('Password', { exact: true }).fill('synthetic-only-password');
          await page
            .getByRole('textbox', { name: /^Endpoint URL/ })
            .fill('https://entered.example.invalid');
          await page
            .getByRole('textbox', { name: 'SSL Fingerprint (optional)' })
            .fill('CD:'.repeat(31) + 'CD');
        };
        const expire = async () => {
          await expect(test).toBeDisabled();
          await page.clock.runFor(29_999);
          await expect(test).toBeDisabled();
          await expect(result).toHaveCount(0);
          await page.clock.runFor(1);
          await expect(test).toBeEnabled();
          await expect(result).toContainText(timeoutText);
          await expect(result).toContainText('Nothing was saved');
        };
        return { context, page, current, activate, result, test, capture, enterDraft, expire };
      };

      for (const type of ['pve', 'pbs', 'pmg']) {
        const { context, page, current, activate, result, test, capture, enterDraft, expire } =
          await visit(type);
        await activate(test);
        await expect.poll(() => current.requests.length).toBe(1);
        await expire();
        await capture('saved-timeout');
        current.held.end(JSON.stringify({ status: 'success', message: 'Obsolete result' }));
        await delay(40);
        await expect(result).toContainText(timeoutText);
        assert.equal(current.requests.length, 1, 'No automatic retry');
        current.mode = 'normal';
        await activate(test);
        await expect(result).toContainText('Synthetic limited coverage');
        await expect(result).toContainText('Unsaved changes are not tested');
        if (type === 'pve') await capture('explicit-saved-recovery');

        await enterDraft();
        current.mode = 'body-stall';
        await activate(test);
        await expect.poll(() => current.requests.length).toBe(3);
        await expire();
        await expect(result).toContainText('Entered settings test');
        await expect(page.getByRole('checkbox', { name: 'Verify SSL certificate' })).toBeChecked();
        await capture('entered-body-timeout');
        current.held.end('"success","message":"Obsolete body"}');
        await delay(40);
        await expect(result).toContainText(timeoutText);
        current.mode = 'mismatch';
        await activate(test);
        await expect(result).toContainText('Synthetic certificate mismatch');
        if (type === 'pve') await capture('explicit-entered-mismatch');
        assert.equal(current.requests.length, 4);
        await expect(page.getByLabel('Fixture save readback')).toContainText('No fixture save');
        report.cases.push({
          type,
          engine,
          browserVersion,
          viewport,
          requests: current.requests,
          checks: [
            'Exact 30-second saved/header and entered/body deadline',
            'Late results retired',
            'Explicit warning/mismatch recovery',
            'No retry/save/trust change; saved/draft credentials separated',
          ],
        });
        await context.close();
      }

      const { context, page, current, activate, result, test, capture, enterDraft, expire } =
        await visit('pve', false);
      await enterDraft();
      current.mode = 'csrf-stall';
      await activate(test);
      await expect.poll(() => current.requests.length).toBe(1);
      assert.equal(current.requests[0].path, '/api/health');
      await expire();
      await capture('csrf-preparation-timeout');
      const response = page.waitForResponse((r) => r.url().endsWith('/api/health'));
      current.held.writeHead(200, {
        'Content-Type': 'application/json',
        'Set-Cookie': 'pulse_csrf=synthetic-csrf; Path=/',
      });
      current.held.end('{}');
      await response;
      await delay(80);
      assert.equal(current.requests.length, 1, 'No stale probe after late CSRF preparation');
      await expect(result).toContainText(timeoutText);
      current.mode = 'forbidden';
      await activate(test);
      await expect(result).toContainText('Synthetic permission refusal');
      await delay(40);
      assert.equal(current.requests.length, 2, '403 must not automatically repeat a bounded test');
      report.cases.push({
        type: 'pve',
        engine,
        browserVersion,
        viewport,
        requests: current.requests,
        checks: [
          'CSRF wait bounded; no late probe',
          '403 with refresh header does not retry',
          'Explicit action required; no save',
        ],
      });
      await context.close();
      await browser.close();
      browser = null;
    }
    assert.deepEqual(report.errors, []);
    report.passed = true;
  } catch (error) {
    report.failure = String(error.stack || error);
    process.exitCode = 1;
  } finally {
    if (browser) await browser.close();
    report.cleanup.browser = true;
    for (const response of held) response.destroy();
    held.clear();
    report.cleanup.heldResponses = true;
    if (server) await server.close();
    report.cleanup.server = true;
    fs.writeFileSync(path.join(output, 'result.json'), JSON.stringify(report, null, 2) + '\n');
    console.log(
      JSON.stringify({
        passed: report.passed ?? false,
        cases: report.cases.length,
        captures: report.captures.length,
        failure: report.failure,
      }),
    );
  }
})();
