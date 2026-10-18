// Real Notifications tab; synthetic read responses only. Never send or save.
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const http = require('node:http');
const crypto = require('node:crypto');
const { chromium, webkit } = require('playwright');
const root = '/workspace/frontend-modern';
const output = '/workspace/tmp/apprise-transport/browser';
const compiled = '/workspace/tmp/apprise-transport/static';
const origin = 'http://127.0.0.1:5351';
const hash = (bytes) => crypto.createHash('sha256').update(bytes).digest('hex');

(async () => {
  fs.mkdirSync(output, { recursive: true });
  const result = {
    result: 'incomplete',
    playwright: require('playwright/package.json').version,
    cases: [],
    captures: [],
    cleanup: { servers_created: 0, servers_closed: 0, browsers_created: 0, browsers_closed: 0 },
    limits:
      'Synthetic API reads and draft edits, not TLS negotiation, sending, saving or installed delivery. Existing WebKit Back/reload fault is not exercised or cleared.',
  };
  let server, browser;
  try {
    assert.equal(
      result.playwright,
      JSON.parse(fs.readFileSync('/workspace/tests/integration/package-lock.json')).packages[
        'node_modules/@playwright/test'
      ].version,
    );
    const binding = JSON.parse(fs.readFileSync(path.join(compiled, 'binding.json')));
    for (const [file, digest] of Object.entries(binding.sources)) {
      assert.equal(hash(fs.readFileSync(path.join(root, file))), digest, `source changed: ${file}`);
    }
    const assets = new Map();
    for (const [file, digest] of Object.entries(binding.artifacts)) {
      const bytes = fs.readFileSync(path.join(compiled, file));
      assert.equal(hash(bytes), digest, `artifact changed: ${file}`);
      assets.set('/' + file, bytes);
    }
    result.binding = binding;
    result.driver_sha256 = hash(fs.readFileSync(root + '/browser-tests/apprise-transport.cjs'));
    server = http.createServer((req, res) => {
      const pathname = new URL(req.url, origin).pathname;
      const bytes = assets.get(pathname);
      if (req.method !== 'GET' || !bytes) {
        res.writeHead(404);
        return res.end();
      }
      const types = { '.html': 'text/html', '.js': 'text/javascript', '.css': 'text/css' };
      res.setHeader('Content-Type', types[path.extname(pathname)] || 'application/octet-stream');
      res.setHeader('Cache-Control', 'no-store');
      res.end(bytes);
    });
    result.cleanup.servers_created++;
    await new Promise((resolve, reject) => {
      server.once('error', reject);
      server.listen(5351, '127.0.0.1', resolve);
    });
    for (const [name, engine, width, dark] of [
      ['desktop', chromium, 1365, false],
      ['narrow', webkit, 320, true],
    ]) {
      browser = await engine.launch(
        engine === chromium
          ? { headless: true, channel: 'chromium', args: ['--no-sandbox'] }
          : { headless: true },
      );
      result.cleanup.browsers_created++;
      const page = await browser.newPage({
        viewport: { width, height: 900 },
        isMobile: width === 320,
        hasTouch: width === 320,
        locale: 'en-GB',
        timezoneId: 'UTC',
      });
      page.setDefaultTimeout(12000);
      const errors = [],
        offOrigin = [],
        writes = [],
        reads = [];
      const saved = {
        enabled: true,
        mode: 'http',
        serverUrl: 'https://apprise.example.invalid',
        skipTlsVerify: false,
        hasApiKey: true,
        apiKey: '',
        targets: ['fixture://target'],
        timeoutSeconds: 15,
        configKey: 'fixture',
        apiKeyHeader: 'X-API-KEY',
      };
      page.on('pageerror', (error) => errors.push(error.message));
      await page.addInitScript((dark) => {
        document.addEventListener('DOMContentLoaded', () =>
          document.documentElement.classList.toggle('dark', dark),
        );
        window.fixtureInputEvents = [];
        document.addEventListener('change', (event) => {
          if (event.target.id?.includes('skip-tls-verify'))
            window.fixtureInputEvents.push({
              trusted: event.isTrusted,
              checked: event.target.checked,
            });
        });
      }, dark);
      await page.route('**/*', async (route) => {
        const request = route.request(),
          url = new URL(request.url());
        if (url.origin !== origin) {
          offOrigin.push(url.origin);
          return route.abort();
        }
        if (!url.pathname.startsWith('/api/')) return route.continue();
        if (request.method() !== 'GET') {
          writes.push({ method: request.method(), path: url.pathname });
          return route.abort();
        }
        reads.push(url.pathname);
        let json = [];
        if (url.pathname === '/api/notifications/apprise') json = saved;
        if (url.pathname === '/api/notifications/email') json = { enabled: false };
        if (url.pathname === '/api/alerts/deadman/config')
          json = { configured: false, pingUrl: '' };
        if (url.pathname === '/api/notifications/health')
          json = {
            overall_healthy: true,
            queue: {
              pending: 0,
              sending: 0,
              sent: 0,
              failed: 0,
              dlq: 0,
              healthy: true,
              status: 'healthy',
              attention_required: 0,
              reason_codes: [],
            },
          };
        if (url.pathname === '/api/notifications/delivery-log')
          json = {
            entries: [],
            window_days: 30,
            completed_retention_days: 7,
            dead_letter_retention_days: 30,
          };
        if (url.pathname === '/api/security/status')
          json = { hasAuthentication: true, requiresAuth: false };
        if (url.pathname === '/api/license/runtime-capabilities')
          json = {
            capabilities: [],
            limits: [],
            max_history_days: 7,
            hosted_mode: false,
            runtime: { build: 'community' },
            blocked_capabilities: [],
          };
        if (url.pathname === '/api/license/entitlements') json = { entitlements: [] };
        return route.fulfill({ json });
      });
      await page.goto(origin + '/browser-tests/apprise-transport.html');
      const panel = page
        .locator('[data-settings-panel]')
        .filter({ has: page.getByRole('heading', { name: 'Apprise notifications', exact: true }) });
      const checkbox = panel.getByRole('checkbox', {
        name: 'Skip TLS certificate verification (unsafe)',
      });
      const warning = panel.getByRole('status', { name: 'Apprise API transport warning' });
      await checkbox.waitFor();
      assert.equal(await checkbox.isChecked(), false);
      assert.equal(await warning.count(), 0);
      assert.equal(await panel.getByLabel('API key', { exact: true }).inputValue(), '');
      assert.match(
        await panel.getByLabel('API key', { exact: true }).getAttribute('placeholder'),
        /Saved.*keep/,
      );
      assert.equal(
        await page.getByRole('status', { name: 'Fixture draft state' }).textContent(),
        'Unchanged',
      );
      const toggle = async () => {
        if (width === 320)
          await panel
            .getByText('Skip TLS certificate verification (unsafe)', { exact: true })
            .tap();
        else {
          await checkbox.focus();
          await page.keyboard.press('Space');
          assert.equal(await checkbox.evaluate((e) => e === document.activeElement), true);
        }
      };
      const capture = async (state) => {
        await page.evaluate(async () => {
          await document.fonts.ready;
          await new Promise((r) => requestAnimationFrame(() => requestAnimationFrame(r)));
        });
        const overflow = await page.evaluate(() => ({
          scroll: document.documentElement.scrollWidth,
          client: document.documentElement.clientWidth,
        }));
        assert(overflow.scroll <= overflow.client + 1, JSON.stringify(overflow));
        const file = `${name}-${state}.png`;
        await panel.screenshot({ path: output + '/' + file });
        result.captures.push({
          file,
          sha256: hash(fs.readFileSync(output + '/' + file)),
          overflow,
        });
      };
      await toggle();
      await warning.waitFor();
      assert.match(
        await warning.innerText(),
        /Certificate and hostname checks.*not just for self-signed/,
      );
      assert.match(await warning.innerText(), /impersonated server/);
      const description = await checkbox.getAttribute('aria-describedby');
      for (const id of description.split(' '))
        assert.equal(await page.locator(`[id="${id}"]`).count(), 1);
      await capture('https-bypass');
      await panel
        .getByRole('textbox', { name: 'Server URL' })
        .fill('http://apprise.example.invalid');
      assert.match(await warning.innerText(), /HTTP does not use TLS/);
      assert.doesNotMatch(await warning.innerText(), /TLS verification disabled/);
      assert.equal(await checkbox.isChecked(), true);
      await capture('http-checked');
      await toggle();
      assert.equal(await checkbox.isChecked(), false);
      assert.match(await warning.innerText(), /checkbox has no effect on HTTP/);
      await capture('http-unchecked');
      await panel
        .getByRole('textbox', { name: 'Server URL' })
        .fill('https://apprise.example.invalid');
      assert.equal(await warning.count(), 0);
      await panel.getByRole('combobox', { name: 'Delivery mode' }).selectOption('cli');
      assert.equal(await warning.count(), 0);
      assert.equal(await checkbox.count(), 0);
      await panel.getByRole('combobox', { name: 'Delivery mode' }).selectOption('http');
      assert.equal(await checkbox.isChecked(), false);
      assert.equal(await panel.getByLabel('API key', { exact: true }).inputValue(), '');
      assert.equal(
        await panel.getByRole('textbox', { name: 'Delivery targets' }).inputValue(),
        'fixture://target',
      );
      // A same-open server refresh must consume its stored bypass, not a draft
      // warning or a new secret. This button only runs the production GET loader.
      saved.skipTlsVerify = true;
      await page.getByRole('button', { name: 'Reload fixture configuration' }).click();
      await warning.waitFor();
      assert.equal(await checkbox.isChecked(), true);
      assert.match(await warning.innerText(), /HTTPS form settings/);
      const events = await page.evaluate(() => window.fixtureInputEvents);
      assert.equal(events.length, 2);
      assert(events.every((event) => event.trusted));
      assert.equal(reads.filter((read) => read === '/api/notifications/apprise').length, 2);
      assert.deepEqual(writes, []);
      assert.deepEqual(offOrigin, []);
      assert.deepEqual(errors, []);
      result.cases.push({
        name,
        width,
        dark,
        browser: browser.version(),
        passed: true,
        reads,
        writes,
        offOrigin,
        errors,
        trustedCheckboxEvents: events,
      });
      await browser.close();
      browser = null;
      result.cleanup.browsers_closed++;
    }
    result.result = 'passed';
  } catch (error) {
    result.error = { name: error.name, message: error.message, stack: error.stack };
    process.exitCode = 1;
  } finally {
    if (browser) {
      await browser.close();
      result.cleanup.browsers_closed++;
    }
    if (server)
      await new Promise((resolve, reject) => {
        server.close((error) => (error ? reject(error) : resolve()));
        server.closeAllConnections();
      }).then(() => result.cleanup.servers_closed++);
    fs.writeFileSync(output + '/result.json', JSON.stringify(result, null, 2) + '\n');
    console.log(
      JSON.stringify({
        result: result.result,
        cases: result.cases.length,
        captures: result.captures.length,
        cleanup: result.cleanup,
        error: result.error,
      }),
    );
  }
})();
