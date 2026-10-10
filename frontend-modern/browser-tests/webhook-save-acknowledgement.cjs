const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const crypto = require('node:crypto');
const { chromium, webkit } = require('playwright');
const root = '/workspace/frontend-modern';
const output = '/workspace/tmp/webhook-save-proof/browser';
const hash = (file) => crypto.createHash('sha256').update(fs.readFileSync(file)).digest('hex');

(async () => {
  const binding = JSON.parse(
    fs.readFileSync('/workspace/tmp/webhook-save-proof/browser-binding.json'),
  );
  for (const [file, digest] of Object.entries(binding.content_sha256)) {
    assert.equal(hash(path.join('/workspace', file)), digest);
  }
  const playwrightVersion = require('playwright/package.json').version;
  assert.equal(
    playwrightVersion,
    JSON.parse(fs.readFileSync('/workspace/tests/integration/package-lock.json')).packages[
      'node_modules/@playwright/test'
    ].version,
  );
  fs.mkdirSync(output, { recursive: true });
  process.chdir(root);
  const { createServer } = await import(path.join(root, 'node_modules/vite/dist/node/index.js'));
  const server = await createServer({
    root,
    configFile: path.join(root, 'vite.config.ts'),
    cacheDir: path.join(output, 'vite-cache'),
    server: { host: '127.0.0.1', port: 5310, strictPort: true, watch: null },
  });
  const origin = 'http://127.0.0.1:5310';
  const results = [],
    screenshots = [],
    browserVersions = [];
  let browser,
    phase = 'server',
    browserClosed = false,
    serverClosed = false,
    passed = false;
  try {
    await server.listen();
    for (const [name, engine, width, height, dark] of [
      ['desktop-light', chromium, 1365, 900, false],
      ['desktop-dark', chromium, 1365, 900, true],
      ['phone-dark', webkit, 390, 844, true],
      ['narrow-light', webkit, 320, 740, false],
    ]) {
      browserClosed = false;
      browser = await engine.launch(
        engine === chromium
          ? { headless: true, channel: 'chromium', args: ['--no-sandbox'] }
          : { headless: true },
      );
      browserVersions.push({
        name,
        engine: engine === chromium ? 'chromium' : 'webkit',
        version: browser.version(),
      });
      const context = await browser.newContext({
        viewport: { width, height },
        isMobile: width < 500,
        hasTouch: width < 500,
        locale: 'en-GB',
      });
      await context.addInitScript((dark) => {
        const apply = () => document.documentElement.classList.toggle('dark', dark);
        if (document.documentElement) apply();
        else document.addEventListener('DOMContentLoaded', apply, { once: true });
      }, dark);
      const saved = {
        id: 'saved-hook',
        name: 'Existing destination',
        url: 'https://example.test/receiver',
        method: 'POST',
        enabled: true,
        service: 'generic',
        headers: { Authorization: '********' },
        customFields: { team: 'ops' },
        template: '{"message":"{{message}}"}',
        mention: 'operators',
        tagFilter: ['department-a'],
        tagFilterMode: 'any',
        minimumSeverity: 'warning',
      };
      let inventory = [saved],
        mode = 'pending',
        release;
      const writes = [],
        unexpected = [],
        errors = [],
        checks = [];
      await context.routeWebSocket(/.*/, (socket) => {
        const url = new URL(socket.url());
        if (url.origin === 'ws://127.0.0.1:5310' && url.pathname === '/') socket.connectToServer();
        else {
          unexpected.push({ type: 'websocket' });
          socket.close();
        }
      });
      await context.route('**/*', async (route) => {
        const req = route.request(),
          url = new URL(req.url());
        if (url.origin !== origin) {
          unexpected.push({ type: 'off-origin' });
          return route.abort();
        }
        if (!url.pathname.startsWith('/api/')) return route.continue();
        if (req.method() === 'GET') {
          if (url.pathname === '/api/health') return route.fulfill({ json: { status: 'healthy' } });
          if (url.pathname === '/api/notifications/webhooks')
            return route.fulfill({ json: inventory });
          if (url.pathname === '/api/notifications/webhook-templates')
            return route.fulfill({ json: [] });
        }
        if (
          (req.method() === 'POST' && url.pathname === '/api/notifications/webhooks') ||
          (req.method() === 'PUT' && url.pathname === '/api/notifications/webhooks/saved-hook')
        ) {
          const write = { method: req.method(), path: url.pathname, payload: req.postDataJSON() };
          writes.push(write);
          if (mode === 'pending')
            await new Promise((resolve) => {
              release = resolve;
            });
          if (mode === 'failure')
            return route.fulfill({
              status: 500,
              json: { error: 'Synthetic configuration save unavailable' },
            });
          const accepted = {
            ...write.payload,
            id: req.method() === 'POST' ? `new-hook-${writes.length}` : 'saved-hook',
            headers: { Authorization: '********' },
          };
          inventory =
            req.method() === 'POST'
              ? [...inventory, accepted]
              : inventory.map((hook) => (hook.id === accepted.id ? accepted : hook));
          if (mode === 'lost-response') return route.abort('failed');
          return route.fulfill({ json: accepted });
        }
        unexpected.push({ method: req.method(), path: url.pathname });
        return route.fulfill({
          status: 403,
          json: { error: 'Fixture refuses Test, deletion and unrelated operations' },
        });
      });
      results.push({ name, checks, writes, unexpected, errors });
      const page = await context.newPage();
      page.on('pageerror', (error) => errors.push(error.message));
      page.setDefaultTimeout(8000);
      page.setDefaultNavigationTimeout(45000);
      const check = async (label, action) => {
        phase = `${name}: ${label}`;
        await action();
        checks.push(label);
      };
      const click = async (locator) => {
        if (width < 500) await locator.tap();
        else {
          await locator.focus();
          await page.keyboard.press('Enter');
        }
      };
      const capture = async (state) => {
        const file = `${name}-${state}.png`;
        await page.screenshot({ path: path.join(output, file) });
        screenshots.push({
          file,
          state,
          sha256: hash(path.join(output, file)),
          viewport: { width, height },
        });
      };
      const noOverflow = async () =>
        assert.ok(
          await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1),
        );
      phase = `${name}: load actual editor`;
      await page.goto(`${origin}/browser-tests/webhook-save-acknowledgement.html`);
      await page.getByText('Existing destination', { exact: true }).first().waitFor();
      await click(page.getByRole('button', { name: '+ Add Webhook', exact: true }));
      await page.getByLabel('Name', { exact: true }).fill('New destination');
      await page.getByLabel('Webhook URL', { exact: true }).fill('https://example.test/new');
      await page
        .getByLabel('Custom header 1 value', { exact: true })
        .fill('application/custom+json');
      await page.getByLabel('Minimum alert severity', { exact: true }).selectOption('warning');
      await click(page.getByRole('button', { name: 'Add Webhook', exact: true }));
      await check(
        'pending save keeps draft, locks conflicting actions and admits one write',
        async () => {
          await page.getByRole('button', { name: 'Saving…', exact: true }).waitFor();
          assert.equal(
            await page.getByLabel('Name', { exact: true }).inputValue(),
            'New destination',
          );
          assert.equal(await page.getByLabel('Name', { exact: true }).isEnabled(), false);
          for (const label of ['Saving…', 'Cancel', 'Edit', 'Delete', 'Enabled', 'Disable All'])
            assert.equal(
              await page.getByRole('button', { name: label, exact: true }).isEnabled(),
              false,
            );
          await page.getByRole('button', { name: 'Saving…' }).evaluate((button) => {
            button.click();
            button.click();
          });
          await page.waitForFunction(
            () => document.querySelector('fieldset')?.getAttribute('aria-busy') === 'true',
          );
          assert.equal(writes.length, 1);
          await noOverflow();
        },
      );
      await capture('pending');
      mode = 'failure';
      release();
      await check(
        'failed create retains values and announces fixed uncertainty, without a replay',
        async () => {
          await page.getByRole('alert').waitFor();
          assert.ok((await page.getByRole('alert').innerText()).includes('another tab'));
          assert.equal(
            await page.getByLabel('Custom header 1 value').inputValue(),
            'application/custom+json',
          );
          assert.equal(
            await page.getByLabel('Minimum alert severity', { exact: true }).inputValue(),
            'warning',
          );
          assert.equal(await page.getByLabel('Name', { exact: true }).isEnabled(), true);
          assert.equal(writes.length, 1);
          await noOverflow();
        },
      );
      await page.getByRole('alert').scrollIntoViewIfNeeded();
      await capture('failed-create');
      mode = 'success';
      await click(page.getByRole('button', { name: 'Add Webhook', exact: true }));
      await check('explicit create retry closes only after accepted response', async () => {
        await page.getByRole('button', { name: '+ Add Webhook', exact: true }).waitFor();
        assert.equal(
          await page.getByLabel('Saved destination names').innerText(),
          'Existing destination, New destination',
        );
        assert.equal(writes.length, 2);
        assert.deepEqual(writes[0].payload, writes[1].payload);
        assert.equal(await page.getByRole('alert').count(), 0);
      });
      await click(page.getByRole('button', { name: 'Edit', exact: true }).first());
      await page.getByLabel('Name', { exact: true }).fill('Edited destination');
      mode = 'failure';
      await click(page.getByRole('button', { name: 'Update Webhook', exact: true }));
      await check('failed edit keeps masking, template, tag and severity policy', async () => {
        await page.getByRole('alert').waitFor();
        assert.equal(await page.getByLabel('Custom header 1 value').inputValue(), '********');
        assert.equal(
          await page.getByLabel('Name', { exact: true }).inputValue(),
          'Edited destination',
        );
        assert.equal(
          await page.getByLabel('Minimum alert severity', { exact: true }).inputValue(),
          'warning',
        );
        const payload = writes[2].payload;
        for (const key of [
          'headers',
          'customFields',
          'template',
          'mention',
          'tagFilter',
          'tagFilterMode',
          'minimumSeverity',
        ])
          assert.deepEqual(payload[key], saved[key]);
        assert.equal(
          await page.getByLabel('Saved destination names').innerText(),
          'Existing destination, New destination',
        );
        await noOverflow();
      });
      await page.getByRole('alert').scrollIntoViewIfNeeded();
      await capture('failed-edit');
      mode = 'success';
      await click(page.getByRole('button', { name: 'Update Webhook', exact: true }));
      await check('accepted edit replaces inventory with acknowledged response', async () => {
        await page.getByRole('button', { name: '+ Add Webhook', exact: true }).waitFor();
        assert.equal(
          await page.getByLabel('Saved destination names').innerText(),
          'Edited destination, New destination',
        );
        assert.deepEqual(writes[2].payload, writes[3].payload);
      });
      await click(page.getByRole('button', { name: '+ Add Webhook', exact: true }));
      await page.getByLabel('Name', { exact: true }).fill('Unconfirmed destination');
      await page
        .getByLabel('Webhook URL', { exact: true })
        .fill('https://example.test/unconfirmed');
      mode = 'lost-response';
      await click(page.getByRole('button', { name: 'Add Webhook', exact: true }));
      await check('lost response leaves the draft and makes no unrequested duplicate', async () => {
        await page.getByRole('alert').waitFor();
        assert.equal(
          await page.getByLabel('Name', { exact: true }).inputValue(),
          'Unconfirmed destination',
        );
        assert.equal(writes.length, 5);
        assert.ok(!(await page.getByRole('alert').innerText()).includes('not saved'));
        const other = await context.newPage();
        await other.goto(`${origin}/browser-tests/webhook-save-acknowledgement.html`);
        await other.getByText('Unconfirmed destination', { exact: true }).first().waitFor();
        assert.ok(
          (await other.getByLabel('Saved destination names').innerText()).includes(
            'Unconfirmed destination',
          ),
        );
        await other.close();
        assert.equal(writes.length, 5);
        assert.equal(new Set(inventory.map((hook) => hook.id)).size, inventory.length);
        await noOverflow();
      });
      await page.getByRole('alert').scrollIntoViewIfNeeded();
      await capture('lost-response');
      await click(page.getByRole('button', { name: 'Cancel', exact: true }));
      await click(page.getByRole('button', { name: '+ Add Webhook', exact: true }));
      await check(
        'deliberate Cancel clears old draft and error without another write',
        async () => {
          assert.equal(await page.getByLabel('Name', { exact: true }).inputValue(), '');
          assert.equal(await page.getByRole('alert').count(), 0);
          assert.equal(writes.length, 5);
          assert.deepEqual(unexpected, []);
          assert.deepEqual(errors, []);
          await noOverflow();
        },
      );

      await browser.close();
      browserClosed = true;
    }
    passed = true;
  } catch (error) {
    fs.writeFileSync(
      path.join(output, 'failure.json'),
      JSON.stringify({ phase, error: String(error) }, null, 2),
    );
    throw error;
  } finally {
    if (browser && !browserClosed) {
      await browser.close();
      browserClosed = true;
    }
    await server.close();
    serverClosed = true;
    fs.writeFileSync(
      path.join(output, 'result.json'),
      JSON.stringify(
        {
          result: passed ? 'passed' : 'failed',
          phase,
          base_sha: binding.base_sha,
          content_sha256: binding.content_sha256,
          playwrightVersion,
          browserVersions,
          results,
          screenshots,
          browserClosed,
          serverClosed,
          scope:
            'Actual existing Webhook section/editor/mutation owner and API client with synthetic transport. No provider/notification, native persistence, deployment or release acceptance.',
        },
        null,
        2,
      ),
    );
  }
})().catch((error) => {
  process.stderr.write(`${error.stack || error}\n`);
  process.exitCode = 1;
});
