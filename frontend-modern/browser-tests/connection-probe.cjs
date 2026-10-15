const { chromium } = require('playwright');
const { expect } = require('playwright/test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const http = require('node:http');
const crypto = require('node:crypto');

const root = path.resolve('frontend-modern');
const compiled = path.resolve('tmp/node-connection-test/static');
const output = path.resolve('tmp/connection-probe/browser');
fs.mkdirSync(output, { recursive: true });
const hash = (bytes) => crypto.createHash('sha256').update(bytes).digest('hex');
const binding = JSON.parse(fs.readFileSync(path.join(compiled, 'binding.json')));
for (const [file, digest] of Object.entries(binding.sources))
  assert.equal(hash(fs.readFileSync(path.join(root, file))), digest, `Source changed: ${file}`);
const assets = new Map(
  Object.entries(binding.artifacts).map(([file, digest]) => {
    const bytes = fs.readFileSync(path.join(compiled, file));
    assert.equal(hash(bytes), digest, `Artifact changed: ${file}`);
    return ['/' + file, bytes];
  }),
);
const result = {
  source: binding.source_sha,
  playwright: require('playwright/package.json').version,
  inputs: Object.keys(binding.sources).length,
  artifacts: assets.size,
  journeys: [],
  captures: [],
  errors: [],
  cleanup: {},
};
const server = http.createServer((request, response) => {
  const pathname = new URL(request.url, 'http://127.0.0.1').pathname;
  const file =
    pathname === '/settings/infrastructure' ? '/browser-tests/connection-probe.html' : pathname;
  const bytes = assets.get(file);
  if (!bytes) {
    response.writeHead(404);
    response.end();
    return;
  }
  response.setHeader(
    'Content-Type',
    file.endsWith('.js') ? 'text/javascript' : file.endsWith('.css') ? 'text/css' : 'text/html',
  );
  response.setHeader('Cache-Control', 'no-store');
  response.end(bytes);
});
let browser;
(async () => {
  try {
    await new Promise((resolve, reject) => {
      server.once('error', reject);
      server.listen(0, '127.0.0.1', resolve);
    });
    const origin = `http://127.0.0.1:${server.address().port}`;
    browser = await chromium.launch({
      headless: true,
      channel: 'chromium',
      args: ['--no-sandbox'],
    });
    result.browser = browser.version();
    for (const viewport of [
      { width: 1360, height: 900 },
      { width: 390, height: 900 },
    ]) {
      const narrow = viewport.width < 768;
      const context = await browser.newContext({
        viewport,
        hasTouch: narrow,
        isMobile: narrow,
        deviceScaleFactor: 1,
      });
      await context.addCookies([{ name: 'pulse_csrf', value: 'fixture-csrf', url: origin }]);
      const page = await context.newPage();
      page.on('pageerror', (error) => result.errors.push(error.message));
      const pending = [],
        requests = [];
      await page.route('**/*', async (route) => {
        const request = route.request(),
          url = new URL(request.url());
        assert.equal(url.origin, origin, 'Off-origin request');
        if (!url.pathname.startsWith('/api/')) return route.continue();
        assert.equal(url.pathname, '/api/connections/probe', 'Unexpected API');
        assert.equal(request.method(), 'POST');
        requests.push(request.postDataJSON());
        await new Promise((resolve) =>
          pending.push(async (body, status = 200) => {
            await route.fulfill({
              status,
              contentType: status === 200 ? 'application/json' : 'text/plain',
              body: status === 200 ? JSON.stringify(body) : body,
            });
            resolve();
          }),
        );
      });
      const activate = async (locator) => {
        if (narrow) await locator.tap();
        else {
          await locator.focus();
          await page.keyboard.press('Enter');
        }
      };
      const capture = async (name) => {
        await page.evaluate(() => window.scrollTo(0, 0));
        await expect
          .poll(() =>
            page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth),
          )
          .toBe(true);
        const file = path.join(output, `${viewport.width}-${name}.png`);
        await page.screenshot({ path: file, fullPage: true });
        result.captures.push({
          file: path.relative(process.cwd(), file),
          sha256: hash(fs.readFileSync(file)),
          state: name,
          viewport,
        });
      };
      const input = page.getByRole('textbox', { name: 'API endpoint', exact: true });
      const submit = page.getByRole('button', { name: 'Probe API endpoint', exact: true });
      const oldHost = 'https://alpha.example.invalid:8006';
      const newHost = 'https://beta.example.invalid:8007';
      const match = (type, host, port) => ({ candidates: [{ type, host, port }], probedMs: 42 });
      const respond = async (body, status = 200) => {
        await expect.poll(() => pending.length).toBe(1);
        await pending.shift()(body, status);
      };
      const assertIdle = async () => {
        await expect(page.getByText('Detected', { exact: true })).toHaveCount(0);
        await expect(page.getByText('Probed in 42 ms', { exact: true })).toHaveCount(0);
        await expect(
          page.getByText('No supported API-backed platform detected at that address.', {
            exact: true,
          }),
        ).toHaveCount(0);
        await expect(page.getByRole('alert')).toHaveCount(0);
        await expect(page.getByText(oldHost, { exact: true })).toHaveCount(0);
      };
      await page.goto(`${origin}/settings/infrastructure`, { waitUntil: 'networkidle' });
      await expect(input).toHaveAccessibleDescription(
        /detected platform does not confirm credentials or certificate trust/,
      );
      await expect(submit).toBeDisabled();
      assert.equal(requests.length, 0);
      await input.fill('alpha.example.invalid');
      await activate(submit);
      await expect.poll(() => pending.length).toBe(1);
      await expect(input).toBeDisabled();
      await expect(page.getByRole('button', { name: 'Probing…', exact: true })).toBeDisabled();
      // Even programmatic duplicate submit cannot bypass the mounted action guard.
      await input.evaluate((element) => element.form.requestSubmit());
      assert.equal(requests.length, 1);
      await respond(match('pve', oldHost, 8006));
      await expect(page.getByText(oldHost, { exact: true })).toBeVisible();
      await capture('original-match');
      await input.fill('beta.example.invalid');
      await assertIdle();
      await expect(input).toBeFocused();
      assert.equal(requests.length, 1, 'Edit must not probe');
      await capture('edited-address-no-old-match');
      await input.fill('alpha.example.invalid');
      await assertIdle();
      await input.fill('');
      await expect(submit).toBeDisabled();
      await input.fill('beta.example.invalid');
      await activate(submit);
      await respond({ candidates: [], probedMs: 42 });
      await expect(
        page.getByText('No supported API-backed platform detected at that address.', {
          exact: true,
        }),
      ).toBeVisible();
      await input.fill('gamma.example.invalid');
      await assertIdle();
      assert.equal(requests.length, 2);
      await activate(submit);
      await respond('certificate fingerprint mismatch: expected fixture-old, got fixture-new', 400);
      await expect(page.getByRole('alert')).toContainText('Keep TLS verification enabled.');
      await capture('current-failure-safe-help');
      await input.fill('beta.example.invalid');
      await assertIdle();
      assert.equal(requests.length, 3);
      await activate(submit);
      await respond(match('pbs', newHost, 8007));
      await activate(
        page.getByRole('button', { name: `Proxmox Backup Server ${newHost}`, exact: true }),
      );
      await expect(page.getByRole('textbox', { name: 'Endpoint URL', exact: false })).toHaveValue(
        newHost,
      );
      await expect(
        page.getByRole('checkbox', { name: 'Verify SSL certificate', exact: true }),
      ).toBeChecked();
      await expect(
        page.getByRole('status', { name: 'Form TLS policy', exact: true }),
      ).toContainText('Trusted certificate chain required');
      await expect(
        page.getByRole('textbox', { name: 'SSL Fingerprint (optional)', exact: true }),
      ).toHaveValue('');
      assert.equal(requests.length, 4, 'Selecting a platform must not test or save credentials');
      await capture('fresh-address-in-credential-editor');
      await activate(page.getByRole('button', { name: '← Back to API probe', exact: true }));
      await expect(input).toHaveValue('');
      await assertIdle();
      await expect(submit).toBeDisabled();
      assert.deepEqual(requests, [
        { address: 'alpha.example.invalid' },
        { address: 'beta.example.invalid' },
        { address: 'gamma.example.invalid' },
        { address: 'beta.example.invalid' },
      ]);
      assert.equal(pending.length, 0);
      result.journeys.push({
        viewport,
        mode: narrow ? 'touch' : 'keyboard',
        requests,
        oldResultAndTimingRetired: true,
        failureAndNoMatchRetired: true,
        sameAddressRequiresFreshProbe: true,
        downstreamEndpoint: newHost,
        verificationPreserved: true,
        noUnexpectedAPIs: true,
      });
      await context.close();
    }
    assert.deepEqual(result.errors, []);
    result.result = 'passed';
  } catch (error) {
    result.result = 'failed';
    result.failure = String(error.stack || error);
    process.exitCode = 1;
  } finally {
    if (browser) {
      await browser.close();
      result.cleanup.browserClosed = true;
    }
    await new Promise((resolve) => {
      server.close(resolve);
      server.closeAllConnections();
    });
    result.cleanup.serverClosed = true;
    fs.writeFileSync(path.join(output, 'result.json'), JSON.stringify(result, null, 2) + '\n');
    process.stdout.write(JSON.stringify(result, null, 2) + '\n');
  }
})();
