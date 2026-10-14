const { chromium } = require('playwright');
const { expect } = require('playwright/test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const http = require('node:http');
const crypto = require('node:crypto');

const root = path.resolve('frontend-modern');
const compiled = path.resolve('tmp/node-connection-test/static');
const output = path.resolve('tmp/node-connection-test/browser');
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
  checkedInputs: Object.keys(binding.sources).length,
  checkedArtifacts: assets.size,
  journeys: [],
  captures: [],
  errors: [],
  cleanup: {},
};
const server = http.createServer((request, response) => {
  let route = new URL(request.url, 'http://127.0.0.1').pathname;
  if (route === '/settings/infrastructure') route = '/browser-tests/node-connection-test.html';
  const bytes = assets.get(route);
  if (!bytes) {
    response.writeHead(404);
    response.end();
    return;
  }
  response.setHeader(
    'Content-Type',
    route.endsWith('.js') ? 'text/javascript' : route.endsWith('.css') ? 'text/css' : 'text/html',
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
      for (const type of ['pve', 'pbs', 'pmg']) {
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
          if (url.origin !== origin) throw new Error('Off-origin browser request');
          if (!url.pathname.startsWith('/api/')) {
            await route.continue();
            return;
          }
          assert.equal(request.method(), 'POST');
          assert(
            [
              '/api/config/nodes/test-connection',
              `/api/config/nodes/${type}-fixture/test`,
            ].includes(url.pathname),
            'Unexpected API',
          );
          requests.push({ path: url.pathname, body: request.postDataJSON() });
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
        const settle = async (body, status) => {
          await expect.poll(() => pending.length).toBe(1);
          await pending.shift()(body, status);
        };
        const capture = async (name) => {
          const file = `${viewport.width}-${type}-${name}.png`;
          await page
            .getByRole('button', { name: /Test saved connection|Test entered settings/ })
            .scrollIntoViewIfNeeded();
          await page.screenshot({ path: path.join(output, file) });
          const bytes = fs.readFileSync(path.join(output, file));
          result.captures.push({
            file,
            sha256: hash(bytes),
            bytes: bytes.length,
            viewport,
            type,
            state: name,
          });
        };
        await page.goto(`${origin}/settings/infrastructure?fixtureType=${type}`);
        const endpoint = page.getByLabel(/^Endpoint URL/);
        const pin = page.getByLabel('SSL Fingerprint (optional)');
        const resultPanel = page.getByRole('status', { name: 'Connection test result' });
        await endpoint.fill('https://entered.example.invalid:8006');
        await pin.fill('cd'.repeat(32));
        const savedButton = page.getByRole('button', { name: 'Test saved connection' });
        await expect(savedButton).toHaveAccessibleDescription(/Unsaved changes are not tested/);
        assert((await savedButton.boundingBox()).height >= 44);
        await activate(savedButton);
        await expect(page.getByRole('button', { name: 'Testing…' })).toBeDisabled();
        await settle({ status: 'success', message: 'Saved fixture connection responded' });
        await expect(resultPanel).toContainText('Saved connection test');
        await expect(resultPanel).toContainText('Unsaved changes are not tested');
        assert.equal(requests[0].path, `/api/config/nodes/${type}-fixture/test`);
        assert.equal(requests[0].body, null, 'Saved request must not carry unsaved settings');
        await capture('saved-test-with-unsaved-trust');

        await pin.fill('ef'.repeat(32));
        await expect(resultPanel).toHaveCount(0);
        assert.equal(requests.length, 1, 'Editing must not run another test');
        await activate(savedButton);
        await expect.poll(() => pending.length).toBe(1);
        // Returning to the original input must not revive a pending result.
        await endpoint.fill('https://another.example.invalid');
        await endpoint.fill('https://entered.example.invalid:8006');
        await expect(page.getByRole('button', { name: 'Testing…' })).toBeDisabled();
        await settle({ status: 'success', message: 'Obsolete saved fixture success' });
        await expect(savedButton).toBeEnabled();
        await expect(resultPanel).toHaveCount(0);
        if (type === 'pve') await capture('obsolete-success-removed');

        if (type !== 'pmg') await page.getByRole('radio', { name: 'Username & Password' }).check();
        await page.getByLabel(/^Username/).fill('entered@pam');
        await page.getByLabel('Password', { exact: true }).fill('fixture-only-secret');
        const formButton = page.getByRole('button', { name: 'Test entered settings' });
        await expect(formButton).toHaveAccessibleDescription(/No settings are saved/);
        await activate(formButton);
        await expect.poll(() => pending.length).toBe(1);
        const formRequest = requests[2];
        assert.equal(formRequest.path, '/api/config/nodes/test-connection');
        assert.equal(formRequest.body.host, 'https://entered.example.invalid:8006');
        assert.equal(formRequest.body.fingerprint, 'ef'.repeat(32));
        assert.equal(formRequest.body.verifySSL, true);
        assert.equal(formRequest.body.password, 'fixture-only-secret');
        assert.equal(formRequest.body.tokenValue, undefined);
        await pin.fill('12'.repeat(32));
        await settle('Obsolete fixture certificate mismatch', 400);
        await expect(formButton).toBeEnabled();
        await expect(resultPanel).toHaveCount(0);
        await expect(
          page.getByText('Obsolete fixture certificate mismatch', { exact: true }),
        ).toHaveCount(0);

        await activate(formButton);
        await settle({
          status: 'success',
          message: 'Entered fixture settings responded',
          warnings: ['Fixture inventory permissions are limited'],
        });
        await expect(resultPanel).toContainText('Entered settings test');
        await expect(resultPanel).toContainText('Fixture inventory permissions are limited');
        await expect(resultPanel).toContainText('No settings are saved');
        assert.equal(requests[3].body.fingerprint, '12'.repeat(32));
        await capture('entered-test-warning');
        await page.getByLabel('Password', { exact: true }).fill('');
        await expect(savedButton).toBeEnabled();
        await expect(resultPanel).toHaveCount(0);
        await expect(endpoint).toHaveValue('https://entered.example.invalid:8006');
        await expect(pin).toHaveValue('12'.repeat(32));
        const bounds = await page.evaluate(() => ({
          width: innerWidth,
          scroll: document.documentElement.scrollWidth,
          buttons: [...document.querySelectorAll('button')]
            .filter((button) => button.getClientRects().length)
            .map((button) => {
              const rect = button.getBoundingClientRect();
              return { text: button.textContent.trim(), left: rect.left, right: rect.right };
            }),
        }));
        assert(bounds.scroll <= bounds.width, 'Horizontal page overflow');
        for (const button of bounds.buttons)
          assert(
            button.left >= 0 && button.right <= bounds.width + 1,
            `Clipped button: ${button.text}`,
          );
        assert.equal(
          requests.length,
          4,
          'Only four explicit fixture tests; no save or automatic probe',
        );
        result.journeys.push({
          viewport,
          type,
          interactions: narrow ? 'touch taps and input' : 'keyboard activation and input',
          requests,
          checks: [
            'saved scope before/after result',
            'edited endpoint not sent with saved credentials',
            'completed success removed by trust edit',
            'A-B-A late success discarded',
            'no overlapping test while pending',
            'newly entered secret uses exact form TLS/endpoint',
            'late form error discarded',
            'current warning visible',
            'clearing secret retires form evidence',
            'unsaved edits preserved',
            '44px test target',
            'no horizontal overflow',
          ],
        });
        await context.close();
      }
    }
    assert.equal(result.errors.length, 0);
    result.result = 'passed';
  } catch (error) {
    result.result = 'failed';
    result.failure = error.stack;
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
    console.log(JSON.stringify(result));
  }
})();
