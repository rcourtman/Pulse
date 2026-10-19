// Production connection editor and HTTP clients, safe synthetic endpoints only.
process.env.RAYON_NUM_THREADS = '2';
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const crypto = require('node:crypto');
const { pathToFileURL } = require('node:url');
const { chromium, webkit, expect } = require('playwright/test');
const root = '/workspace';
const frontend = path.join(root, 'frontend-modern');
const output = path.join(root, 'tmp/node-fingerprint-validation');
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
  requests: [],
  cleanup: {},
};
let server, browser;
const corrected = 'CD:'.repeat(31) + 'CD';
const malformed = {
  pve: 'SHA256:' + 'ab'.repeat(32),
  pbs: 'ab'.repeat(31),
  pmg: 'not-a-valid-pin',
};
(async () => {
  try {
    const { createServer } = await import(
      pathToFileURL(path.join(frontend, 'node_modules/vite/dist/node/index.js')).href
    );
    server = await createServer({
      root: frontend,
      configFile: path.join(frontend, 'vite.config.ts'),
      logLevel: 'error',
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
          : {
              headless: true,
              channel: 'chromium',
              args: ['--no-sandbox'],
            },
      );
      const browserVersion = browser.version();
      for (const type of ['pve', 'pbs', 'pmg']) {
        const context = await browser.newContext({ viewport, hasTouch: narrow, isMobile: narrow });
        await context.addCookies([{ name: 'pulse_csrf', value: 'synthetic-csrf', url: origin }]);
        const page = await context.newPage();
        page.on('pageerror', (error) => report.errors.push(error.message));
        let stored = {
          id: `${type}-fixture`,
          type,
          name: `${type.toUpperCase()} fixture`,
          host: 'https://fixture.example.invalid:8006',
          user: 'fixture@pam',
          tokenName: 'fixture@pam!read',
          hasToken: type !== 'pmg',
          hasPassword: type === 'pmg',
          verifySSL: true,
          fingerprint: 'ab'.repeat(32),
          status: 'connected',
        };
        const calls = [];
        await page.route('**/*', async (route) => {
          const request = route.request(),
            url = new URL(request.url());
          assert.equal(url.origin, origin, 'No off-origin request');
          if (!url.pathname.startsWith('/api/')) return route.continue();
          const method = request.method(),
            body = request.postDataJSON();
          const observed = { method, path: url.pathname };
          calls.push(observed);
          if (method === 'POST') {
            assert.equal(request.headers()['x-csrf-token'], 'synthetic-csrf');
            if (url.pathname === `/api/config/nodes/${type}-fixture/test`) {
              assert.equal(body, null, 'No draft trust or secret sent to saved test');
              return route.fulfill({
                json: { status: 'success', message: 'Synthetic saved record responded' },
              });
            }
            assert.equal(url.pathname, '/api/config/nodes/test-connection');
            assert.equal(body.fingerprint, corrected);
            assert.equal(body.verifySSL, true);
            assert.equal(body.password, 'synthetic-only-password');
            assert.equal(body.host, stored.host);
            return route.fulfill({
              json: {
                status: 'error',
                message: 'Synthetic certificate mismatch: valid format does not prove trust.',
              },
            });
          }
          if (method === 'PUT') {
            assert.equal(request.headers()['x-csrf-token'], 'synthetic-csrf');
            assert.equal(url.pathname, `/api/config/nodes/${type}-fixture`);
            assert.equal(body.fingerprint, corrected);
            assert.equal(body.verifySSL, true);
            for (const key of ['password', 'tokenValue', 'clusterEndpoints'])
              assert(!(key in body), 'Saved credential and member trust preserved');
            stored = { ...stored, ...body };
            return route.fulfill({ json: { success: true } });
          }
          assert.equal(method, 'GET');
          assert.equal(url.pathname, '/api/config/nodes');
          return route.fulfill({ json: [stored] });
        });
        const activate = async (locator) => {
          if (narrow) await locator.tap();
          else {
            await locator.focus();
            await page.keyboard.press('Enter');
          }
        };
        const capture = async (state, anchor) => {
          await anchor.evaluate((element) =>
            element.scrollIntoView({ block: 'center', behavior: 'instant' }),
          );
          await page.evaluate(async () => {
            await document.fonts.ready;
          });
          await page.evaluate(
            () =>
              new Promise((resolve) => requestAnimationFrame(() => requestAnimationFrame(resolve))),
          );
          assert(
            await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth),
            'No page overflow',
          );
          const file = `${engine}-${type}-${state}.png`;
          await page.screenshot({ path: path.join(output, file) });
          report.captures.push({
            file,
            state,
            type,
            viewport,
            engine,
            sha256: hash(fs.readFileSync(path.join(output, file))),
          });
        };
        await page.goto(`${origin}/browser-tests/node-fingerprint-validation.html?type=${type}`, {
          waitUntil: 'networkidle',
        });
        const pin = page.getByRole('textbox', { name: 'SSL Fingerprint (optional)' });
        const verification = page.getByRole('checkbox', { name: 'Verify SSL certificate' });
        const save = page.getByRole('button', { name: 'Save changes', exact: true });
        const error = page.getByRole('alert');
        await pin.fill(malformed[type]);
        await expect(pin).toHaveAttribute('aria-invalid', 'true');
        await expect(pin).toHaveAccessibleDescription(/exactly 64 hexadecimal characters/);
        await expect(error).toContainText('Valid format does not prove trust');
        assert.equal(await pin.evaluate((input) => input.validity.customError), true);
        await activate(save);
        assert.equal(
          await pin.evaluate((input) => document.activeElement === input),
          true,
          'Native validation focuses the field',
        );
        assert.equal(calls.length, 0, 'Malformed save never reaches server');
        await expect(pin).toHaveValue(malformed[type]);
        await expect(verification).toBeChecked();
        await capture('invalid-pin', pin);
        // A non-submit action can still test the original saved connection;
        // malformed unsaved trust is neither sent nor silently cleared.
        await activate(page.getByRole('button', { name: 'Test saved connection', exact: true }));
        await expect(page.getByRole('status', { name: 'Connection test result' })).toContainText(
          'Unsaved changes are not tested',
        );
        assert.equal(calls.length, 1);
        await expect(pin).toHaveValue(malformed[type]);
        await pin.fill(corrected);
        await expect(pin).not.toHaveAttribute('aria-invalid');
        assert.equal(await pin.evaluate((input) => input.validity.valid), true);
        await expect(error).toHaveCount(0);
        await expect(page.getByRole('status', { name: 'Connection test result' })).toHaveCount(0);
        await activate(save);
        await expect(page.getByLabel('Fixture save readback')).toContainText('Saved pin read back');
        assert.deepEqual(
          calls.map((call) => call.method),
          ['POST', 'PUT', 'GET'],
        );
        assert.equal(stored.fingerprint, corrected);
        assert.equal(stored.verifySSL, true);
        await expect(pin).toHaveValue(corrected);
        await capture('corrected-pin', pin);
        // Entered-settings tests are separately guarded, without performing a
        // save or a saved-credential/unsaved-endpoint combination.
        if (type !== 'pmg') await page.getByRole('radio', { name: 'Username & Password' }).check();
        await page.getByLabel('Password', { exact: true }).fill('synthetic-only-password');
        await pin.fill(' ' + 'ab'.repeat(32));
        const testEntered = page.getByRole('button', {
          name: 'Test entered settings',
          exact: true,
        });
        await activate(testEntered);
        assert.equal(calls.length, 3, 'Malformed entered pin never sends credentials');
        await expect(error).toContainText('Do not include spaces');
        await pin.fill(corrected);
        await activate(testEntered);
        const result = page.getByRole('status', { name: 'Connection test result' });
        await expect(result).toContainText('Synthetic certificate mismatch');
        await expect(result).toContainText('No settings are saved');
        assert.equal(calls.length, 4, 'Only the explicit corrected test was sent, no retry');
        if (type === 'pve') await capture('valid-format-not-trust', result);
        assert.equal(stored.fingerprint, corrected);
        assert.equal(stored.verifySSL, true);
        report.cases.push({
          type,
          engine,
          browserVersion,
          viewport,
          checks: [
            'Invalid pin remains visible; accessible field error and native save focus; no save or probe',
            'Explicit saved-record test omits unsaved draft and new credentials',
            'Correction clears invalidity and saves exact colon/case bytes, checkbox and saved credentials intact',
            'Actual API PUT and GET readback use the exact saved target; no add/delete or capture',
            'Malformed entered-settings test sends nothing; valid-format mismatch stays a failed connection result',
          ],
        });
        report.requests.push({ type, engine, calls });
        await context.close();
      }
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
