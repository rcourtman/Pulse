// Real credential editor and state, synthetic data only. No API action is run.
const { chromium, webkit } = require('playwright');
const { expect } = require('playwright/test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const http = require('node:http');
const crypto = require('node:crypto');

const root = path.resolve('frontend-modern');
const compiled = path.resolve('tmp/node-connection-test/static');
const output = path.resolve('tmp/pmg-account/browser');
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
  source_base: binding.source_sha,
  source_note: 'Working frontend inputs are hash-bound and must match the containing commit.',
  playwright: require('playwright/package.json').version,
  checkedInputs: Object.keys(binding.sources).length,
  checkedArtifacts: assets.size,
  journeys: [],
  captures: [],
  errors: [],
  requests: [],
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
    for (const [engine, type, viewport] of [
      ['chromium', chromium, { width: 1360, height: 900 }],
      ['webkit', webkit, { width: 320, height: 900 }],
    ]) {
      browser = await type.launch(
        engine === 'chromium'
          ? { headless: true, channel: 'chromium', args: ['--no-sandbox'] }
          : { headless: true },
      );
      for (const scheme of ['light', 'dark']) {
        for (const mode of ['add', 'edit']) {
          const context = await browser.newContext({
            viewport,
            colorScheme: scheme,
            hasTouch: viewport.width < 768,
            isMobile: viewport.width < 768,
            deviceScaleFactor: 1,
          });
          const page = await context.newPage();
          page.on('pageerror', (error) => result.errors.push(error.message));
          await page.route('**/*', async (route) => {
            const request = route.request(),
              url = new URL(request.url());
            if (url.origin !== origin || url.pathname.startsWith('/api/')) {
              result.requests.push({
                origin: url.origin,
                path: url.pathname,
                method: request.method(),
              });
              await route.abort();
              return;
            }
            await route.continue();
          });
          await page.goto(`${origin}/settings/infrastructure?fixtureType=pmg&fixtureMode=${mode}`);
          await page.evaluate(
            (dark) => document.documentElement.classList.toggle('dark', dark),
            scheme === 'dark',
          );
          await expect(page.getByText('Username and password', { exact: true })).toBeVisible();
          const note = page.getByRole('note', { name: 'Mail Gateway account guidance' });
          await expect(note).toHaveCount(1);
          await expect(note).toContainText('minimum read permissions your PMG version supports');
          await expect(note).toContainText('Do not grant administrator access');
          await expect(note).toContainText(
            'Keep passwords out of commands, URLs and issue reports',
          );
          assert(
            !/Configuration → API Tokens|Privilege Separation|Auditor|root@pam/.test(
              await note.innerText(),
            ),
          );
          await expect(page.getByRole('radio')).toHaveCount(1);
          await expect(page.getByRole('radio', { name: 'Username & Password' })).toBeChecked();
          await expect(page.getByLabel(/Token ID|Token Value/)).toHaveCount(0);
          await expect(
            page.getByRole('button', {
              name: /Copy command|Manual Token Setup|Host Telemetry Agent/,
            }),
          ).toHaveCount(0);
          const username = page.getByRole('textbox', { name: /^Username/ });
          const password = page.getByLabel(/^Password/);
          await expect(username).toHaveAttribute('placeholder', 'pulse-monitor@pmg');
          await expect(username).toHaveValue(mode === 'edit' ? 'fixture@pam' : '');
          await expect(password).toHaveAttribute('type', 'password');
          await expect(password).toHaveValue('');
          assert.equal(await password.evaluate((field) => field.required), mode === 'add');
          if (mode === 'edit')
            await expect(password).toHaveAttribute('placeholder', 'Leave blank to keep existing');
          const auth = page
            .locator('form > div')
            .filter({ has: page.getByRole('heading', { name: 'Authentication', exact: true }) });
          await expect(auth).toHaveCount(1);
          await username.fill('pulse-monitor@pmg');
          if (viewport.width >= 1024) {
            await username.focus();
            await page.keyboard.press('Tab');
            await expect(password).toBeFocused();
          } else {
            await password.tap();
            await expect(password).toBeFocused();
          }
          // Fake secret stays in an ordinary masked input and is cleared before capture.
          await password.fill('fixture-only-secret');
          await expect(
            page.getByRole('checkbox', { name: 'Verify SSL certificate' }),
          ).toBeChecked();
          await password.fill('');
          await page.evaluate(async () => {
            document.activeElement?.blur();
            await document.fonts.ready;
          });
          const bounds = await page.evaluate(() => ({
            width: innerWidth,
            scroll: document.documentElement.scrollWidth,
          }));
          assert(bounds.scroll <= bounds.width, 'Horizontal page overflow');
          const geometry = await auth.evaluate((section) =>
            [...section.querySelectorAll('input, [role="note"]')].map((el) => {
              const box = el.getBoundingClientRect();
              return {
                left: box.left,
                right: box.right,
                width: box.width,
                client: el.clientWidth,
                scroll: el.scrollWidth,
                role: el.getAttribute('role'),
              };
            }),
          );
          for (const box of geometry) {
            assert(
              box.left >= 0 && box.right <= viewport.width + 1 && box.width > 0,
              'Clipped authentication content',
            );
            if (box.role === 'note') assert(box.scroll <= box.client + 1, 'Clipped guidance');
          }
          const capture = async (locator, suffix) => {
            const file = `${viewport.width}-${scheme}-${mode}-${suffix}.png`;
            await locator.screenshot({ path: path.join(output, file) });
            const bytes = fs.readFileSync(path.join(output, file));
            result.captures.push({
              file,
              sha256: hash(bytes),
              bytes: bytes.length,
              viewport,
              scheme,
              mode,
              engine,
            });
          };
          await capture(auth, 'authentication');
          if (scheme === 'light' && mode === 'add')
            await capture(
              page.getByRole('region', { name: 'Infrastructure connection sequence' }),
              'sequence',
            );
          result.journeys.push({
            engine,
            browser: browser.version(),
            viewport,
            scheme,
            mode,
            geometry,
            interaction:
              viewport.width >= 1024
                ? 'keyboard tab to password and input'
                : 'touch focus and input',
            requests: 0,
          });
          await context.close();
        }
      }
      await browser.close();
      browser = null;
    }
    assert.deepEqual(result.errors, []);
    assert.deepEqual(result.requests, []);
    result.passed = true;
  } catch (error) {
    result.failure = error.stack;
    process.exitCode = 1;
  } finally {
    if (browser) await browser.close();
    result.cleanup.browserClosed = true;
    await new Promise((resolve) => server.close(resolve));
    result.cleanup.serverClosed = true;
    fs.writeFileSync(path.join(output, 'result.json'), JSON.stringify(result, null, 2) + '\n');
    console.log(JSON.stringify(result));
  }
})();
