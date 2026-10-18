const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const crypto = require('node:crypto');
const { chromium, webkit } = require('playwright');

const root = '/workspace/frontend-modern';
const output = '/workspace/tmp/email-rate-limit-proof';
const runtimePath = 'frontend-modern/src/features/alerts/alertDestinationsModel.ts';
const runtimePaths = [
  runtimePath,
  'frontend-modern/src/features/alerts/AlertEmailDestinationsSection.tsx',
];
const baseEmail = {
  enabled: true,
  provider: '',
  server: 'smtp.example.test',
  port: 587,
  username: 'ops@example.test',
  password: '***REDACTED***',
  from: 'pulse@example.test',
  to: ['alerts@example.test'],
  tls: false,
  startTLS: true,
  rateLimit: 17,
  tagFilter: ['production'],
  tagFilterMode: 'any',
  minimumSeverity: 'warning',
};

(async () => {
  fs.mkdirSync(output, { recursive: true });
  const playwrightVersion = require('playwright/package.json').version;
  assert.equal(
    playwrightVersion,
    JSON.parse(fs.readFileSync('/workspace/tests/integration/package-lock.json')).packages[
      'node_modules/@playwright/test'
    ].version,
  );
  const digests = Object.fromEntries(
    runtimePaths.map((file) => [
      file,
      crypto
        .createHash('sha256')
        .update(fs.readFileSync(path.join('/workspace', file)))
        .digest('hex'),
    ]),
  );
  const { createServer } = await import(path.join(root, 'node_modules/vite/dist/node/index.js'));
  const server = await createServer({
    root,
    configFile: path.join(root, 'vite.config.ts'),
    cacheDir: path.join(output, 'vite-cache'),
    server: { host: '127.0.0.1', port: 5294, strictPort: true },
  });
  const results = [];
  let browser;
  let serverClosed = false;
  try {
    await server.listen();
    for (const [name, engine, width, height, dark] of [
      ['desktop-light', chromium, 1365, 900, false],
      ['desktop-dark', chromium, 1365, 900, true],
      ['phone-dark', webkit, 390, 844, true],
      ['narrow-light', webkit, 320, 844, false],
    ]) {
      browser = await engine.launch(
        engine === chromium
          ? { headless: true, channel: 'chromium', args: ['--no-sandbox'] }
          : { headless: true },
      );
      const browserVersion = browser.version();
      const page = await browser.newPage({
        viewport: { width, height },
        isMobile: width < 500,
        hasTouch: width < 500,
        locale: 'en-GB',
        timezoneId: 'UTC',
        colorScheme: dark ? 'dark' : 'light',
      });
      page.setDefaultTimeout(15000);
      let email = { ...baseEmail };
      const writes = [];
      const unexpected = [];
      const errors = [];
      const checks = [];
      const captures = [];
      page.on('pageerror', (error) => errors.push(error.message));
      await page.addInitScript((isDark) => {
        if (!isDark) return;
        const apply = () => document.documentElement.classList.add('dark');
        if (document.documentElement) apply();
        else document.addEventListener('DOMContentLoaded', apply, { once: true });
      }, dark);
      await page.route('**/*', async (route) => {
        const request = route.request();
        const url = new URL(request.url());
        if (url.origin !== 'http://127.0.0.1:5294') {
          unexpected.push('off-origin request');
          return route.abort();
        }
        if (!url.pathname.startsWith('/api/')) return route.continue();
        const key = `${request.method()} ${url.pathname}`;
        let body;
        if (key === 'GET /api/notifications/email') body = email;
        else if (key === 'GET /api/notifications/email-providers') body = [];
        else if (key === 'GET /api/notifications/webhooks') body = [];
        else if (key === 'GET /api/notifications/apprise') body = { enabled: false, targets: [] };
        else if (key === 'GET /api/alerts/deadman/config')
          body = { pingUrl: '', configured: false };
        else if (key === 'GET /api/health') body = { csrfToken: 'fixture-only' };
        else if (key === 'PUT /api/notifications/email') {
          const submitted = request.postDataJSON();
          assert.equal(submitted.password, baseEmail.password);
          assert.equal(submitted.tls, false);
          assert.equal(submitted.startTLS, true);
          assert.deepEqual(submitted.tagFilter, ['production']);
          assert.equal(submitted.tagFilterMode, 'any');
          assert.equal(submitted.minimumSeverity, 'warning');
          writes.push({ rateLimit: submitted.rateLimit, enabled: submitted.enabled });
          // Model the existing default only in the fixture, not in the client.
          email = { ...submitted, rateLimit: submitted.rateLimit ?? 60 };
          body = { success: true };
        } else if (key === 'PUT /api/notifications/apprise') {
          body = request.postDataJSON();
        } else if (key === 'PUT /api/alerts/deadman/config') {
          body = { success: true, configured: false };
        } else {
          unexpected.push(key);
          return route.abort();
        }
        return route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify(body),
        });
      });
      const open = async () => {
        await page.goto('http://127.0.0.1:5294/browser-tests/email-rate-limit.html');
        await page.getByRole('button', { name: 'Save changes' }).waitFor();
        assert.equal(
          await page.evaluate(() => document.documentElement.classList.contains('dark')),
          dark,
        );
      };
      const advanced = async () => {
        await page.getByRole('button', { name: 'Show advanced options' }).click();
        return page.getByRole('spinbutton', { name: 'Rate limit' });
      };
      const save = async (expectedLimit) => {
        const before = writes.length;
        await page.getByRole('button', { name: 'Save changes' }).click();
        await page
          .getByRole('status')
          .filter({ hasText: /^Saved$/ })
          .waitFor();
        assert.equal(writes.length, before + 1);
        assert.equal(writes.at(-1).rateLimit, expectedLimit);
        assert.equal(email.rateLimit, expectedLimit);
      };
      const check = async (label, run) => {
        await run();
        checks.push(label);
      };
      const capture = async (state) => {
        assert.ok(
          await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1),
          'no horizontal overflow',
        );
        const file = `${name}-${state}.png`;
        await page.screenshot({ path: path.join(output, file), fullPage: true });
        captures.push(file);
      };
      await check('loaded non-default limit is visible', async () => {
        await open();
        assert.equal(await (await advanced()).inputValue(), '17');
      });
      await capture('loaded');
      await check('unrelated edit preserves limit, mask, TLS and routing on the wire', async () => {
        await page
          .getByRole('textbox', { name: 'From address', exact: true })
          .fill('new@example.test');
        await save(17);
      });
      await check('unrelated edit survives a reload with the same limit', async () => {
        await open();
        assert.equal(await (await advanced()).inputValue(), '17');
        assert.equal(
          await page.getByRole('textbox', { name: 'From address', exact: true }).inputValue(),
          'new@example.test',
        );
      });
      await check('edited limit is sent exactly once', async () => {
        await page.getByRole('spinbutton', { name: 'Rate limit' }).fill('12');
        await save(12);
      });
      await capture('saved');
      await check('edited limit is read back after reload', async () => {
        await open();
        assert.equal(await (await advanced()).inputValue(), '12');
      });
      await check('switching email off preserves its limit', async () => {
        await page.getByRole('button', { name: 'Email notifications Enabled' }).click();
        await save(12);
        assert.equal(email.enabled, false);
      });
      await check('switched-off settings read back the same limit', async () => {
        await open();
        await page.getByRole('button', { name: 'Show settings', exact: true }).click();
        assert.equal(await (await advanced()).inputValue(), '12');
        assert.equal(
          await page.getByRole('button', { name: 'Send test email' }).isDisabled(),
          true,
        );
        await page.getByRole('spinbutton', { name: 'Rate limit' }).fill('9');
        await save(9);
        assert.equal(email.enabled, false);
      });
      await capture('off');
      await check('legacy default sentinel zero remains zero in the saved payload', async () => {
        email = { ...baseEmail, rateLimit: 0 };
        await open();
        await save(0);
      });
      await check('no test, retry, dismissal, off-origin request or browser error', async () => {
        assert.deepEqual(unexpected, []);
        assert.deepEqual(errors, []);
        assert.equal(writes.length, 5);
      });
      results.push({ name, browserVersion, width, height, dark, checks, writes, captures });
      await browser.close();
      browser = undefined;
    }
  } finally {
    if (browser) await browser.close();
    await server.close();
    serverClosed = true;
    fs.writeFileSync(
      path.join(output, 'result.json'),
      JSON.stringify(
        {
          result: results.length === 4 ? 'passed' : 'incomplete',
          playwrightVersion,
          content_sha256: digests,
          browserClosed: true,
          serverClosed,
          results,
        },
        null,
        2,
      ) + '\n',
    );
  }
  console.log(JSON.stringify({ result: 'passed', groups: 36, captures: 12, serverClosed }));
})().catch((error) => {
  console.error(error);
  process.exitCode = 1;
});
