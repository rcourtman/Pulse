// Safe fixture HTTP reads -> real state -> real command text and clipboard.
// Never execute a copied command or permit an API write/off-origin request.
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const http = require('node:http');
const crypto = require('node:crypto');
const { chromium, webkit } = require('playwright');
const root = '/workspace/frontend-modern';
const compiled = '/workspace/tmp/installer-transport/static';
const output = '/workspace/tmp/installer-transport/browser';
const origin = 'http://127.0.0.1:5353';
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
      'Synthetic status reads and draft transport edits. Clipboard intercepted in page. No command execution, token creation, actual TLS, installed recovery or WebKit Back/reload sequence.',
  };
  let server, browser;
  try {
    assert.equal(
      result.playwright,
      JSON.parse(fs.readFileSync('/workspace/tests/integration/package-lock.json')).packages[
        'node_modules/@playwright/test'
      ].version,
    );
    const binding = JSON.parse(fs.readFileSync(compiled + '/binding.json'));
    for (const [file, digest] of Object.entries(binding.sources))
      assert.equal(hash(fs.readFileSync(path.join(root, file))), digest, `source changed: ${file}`);
    const assets = new Map();
    for (const [file, digest] of Object.entries(binding.artifacts)) {
      const bytes = fs.readFileSync(path.join(compiled, file));
      assert.equal(hash(bytes), digest, `artifact changed: ${file}`);
      assets.set('/' + file, bytes);
    }
    result.binding = binding;
    result.driver_sha256 = hash(fs.readFileSync(root + '/browser-tests/installer-transport.cjs'));
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
      server.listen(5353, '127.0.0.1', resolve);
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
      page.setDefaultTimeout(10000);
      const errors = [],
        offOrigin = [],
        writes = [],
        unexpected = [],
        reads = [],
        checks = [];
      page.on('pageerror', (error) => errors.push(error.message));
      page.on('console', (message) => {
        if (message.type() === 'error') errors.push(message.text());
      });
      await page.addInitScript((dark) => {
        document.addEventListener('DOMContentLoaded', () =>
          document.documentElement.classList.toggle('dark', dark),
        );
        window.fixtureCopies = [];
        window.fixtureChanges = [];
        Object.defineProperty(navigator, 'clipboard', {
          configurable: true,
          value: {
            writeText: async (text) => {
              window.fixtureCopies.push(text);
            },
          },
        });
        document.addEventListener('change', (event) => {
          if (event.target.type === 'checkbox')
            window.fixtureChanges.push({ trusted: event.isTrusted, checked: event.target.checked });
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
          writes.push(request.method() + ' ' + url.pathname);
          return route.abort();
        }
        reads.push(url.pathname);
        if (url.pathname === '/api/security/status')
          return route.fulfill({
            json: {
              requiresAuth: false,
              apiTokenConfigured: false,
              agentUrl: 'http://pulse.example.invalid:7655',
            },
          });
        if (url.pathname === '/api/state')
          return route.fulfill({ json: { connectedInfrastructure: [] } });
        unexpected.push(url.pathname);
        return route.abort();
      });
      await page.goto(origin + '/browser-tests/installer-transport.html');
      const act = async (locator) => {
        if (width === 320) await locator.tap();
        else {
          await locator.focus();
          await page.keyboard.press('Space');
        }
      };
      await act(page.getByRole('button', { name: 'Confirm without token', exact: true }));
      const policy = page.getByRole('status', { name: 'Installation transport policy' });
      await policy.waitFor();
      const show = () =>
        page.getByRole('button', {
          name: 'Show advanced connection and install options',
          exact: true,
        });
      const hide = () =>
        page.getByRole('button', {
          name: 'Hide advanced connection and install options',
          exact: true,
        });
      const skip = () =>
        page.getByRole('checkbox', {
          name: 'Skip TLS certificate verification (unsafe)',
          exact: true,
        });
      const command = async (button) =>
        page
          .getByRole('button', { name: button, exact: true })
          .locator('..')
          .locator('code')
          .textContent();
      const unixName = 'Copy Install command';
      const windowsName = 'Copy Install as Windows Service (PowerShell) command';
      const checkCommands = async (state, { insecure, ca, http }) => {
        const unix = await command(unixName),
          windows = await command(windowsName);
        assert.equal(unix.includes(' --insecure'), insecure || http);
        assert.equal(windows.includes('PULSE_INSECURE_SKIP_VERIFY="true"'), insecure || http);
        assert.equal(unix.includes('curl -kfsSL'), insecure);
        assert.equal(unix.includes("--cacert '"), ca);
        if (ca && !insecure)
          assert(
            windows.includes(
              'ServerCertificateValidationCallback=[PulseInstallerCertificateValidator]::ValidateCustomCaCallback;',
            ),
          );
        if (insecure)
          assert(
            windows.includes(
              'ServerCertificateValidationCallback=[PulseInstallerCertificateValidator]::AcceptAnyCallback;',
            ),
          );
        for (const name of [unixName, windowsName]) {
          const button = page.getByRole('button', { name, exact: true });
          assert.equal(
            await button.getAttribute('aria-describedby'),
            await policy.getAttribute('id'),
          );
          await act(button);
          const copied = await page.evaluate(() => window.fixtureCopies.at(-1));
          assert.equal(copied, await command(name));
        }
        checks.push({
          state,
          unix_sha256: hash(unix),
          windows_sha256: hash(windows),
          insecure,
          ca,
          http,
        });
      };
      const capture = async (state) => {
        await policy.evaluate((element) =>
          window.scrollTo(0, element.getBoundingClientRect().top + window.scrollY - 16),
        );
        await page.evaluate(async () => {
          await document.fonts.ready;
          await new Promise((r) => requestAnimationFrame(() => requestAnimationFrame(r)));
        });
        const geometry = await policy.evaluate((element) => {
          const r = element.getBoundingClientRect();
          return {
            width: r.width,
            left: r.left,
            right: r.right,
            top: r.top,
            bottom: r.bottom,
            scroll: element.scrollWidth,
            client: element.clientWidth,
            page: document.documentElement.scrollWidth,
            viewport: document.documentElement.clientWidth,
          };
        });
        assert(geometry.page <= geometry.viewport + 1, JSON.stringify(geometry));
        assert(geometry.scroll <= geometry.client + 1, JSON.stringify(geometry));
        assert(
          geometry.left >= 0 && geometry.right <= width && geometry.bottom <= 900,
          JSON.stringify(geometry),
        );
        const file = `${name}-${state}.png`;
        await page.screenshot({ path: output + '/' + file });
        result.captures.push({
          file,
          sha256: hash(fs.readFileSync(output + '/' + file)),
          geometry,
        });
      };
      assert.match(await policy.innerText(), /HTTP does not protect/);
      assert.equal(await skip().count(), 0);
      await checkCommands('http-default', { insecure: false, ca: false, http: true });
      await capture('http-default');
      await act(show());
      assert.equal(await skip().isChecked(), false);
      assert.equal(await skip().getAttribute('aria-describedby'), await policy.getAttribute('id'));
      await page
        .getByLabel('Connection URL (Agent → Pulse)', { exact: true })
        .fill('https://pulse.example.invalid');
      assert.match(await policy.innerText(), /HTTPS certificate verification requested/);
      await act(hide());
      await checkCommands('https-verified', { insecure: false, ca: false, http: false });
      await capture('https-verified');
      await act(show());
      const caPath =
        '/etc/pulse/trust/' + 'independently-verified-private-authority-'.repeat(3) + 'ca.pem';
      await page.getByLabel('Custom CA certificate path (optional)', { exact: true }).fill(caPath);
      assert.match(await policy.innerText(), /HTTPS with a custom CA requested/);
      assert.match(await policy.innerText(), /already-trusted channel/);
      await act(hide());
      assert((await policy.innerText()).includes(caPath));
      await checkCommands('custom-ca', { insecure: false, ca: true, http: false });
      await capture('custom-ca');
      await act(show());
      await act(skip());
      assert.equal(await skip().isChecked(), true);
      assert.match(await policy.innerText(), /not just for self-signed/);
      assert.match(await policy.innerText(), /override takes precedence/);
      await act(hide());
      await checkCommands('bypass-over-ca', { insecure: true, ca: true, http: false });
      await capture('bypass-over-ca');
      await act(show());
      await act(skip());
      await page.getByLabel('Custom CA certificate path (optional)', { exact: true }).fill('');
      await page
        .getByLabel('Connection URL (Agent → Pulse)', { exact: true })
        .fill('not-a-pulse-url');
      assert.match(await policy.innerText(), /Check the Pulse URL before installing/);
      assert.doesNotMatch(await policy.innerText(), /HTTPS certificate verification requested/);
      await page.getByLabel('Connection URL (Agent → Pulse)', { exact: true }).fill('');
      assert.match(await policy.innerText(), /HTTP does not protect/);
      assert.equal(await skip().isChecked(), false);
      checks.push({
        state: 'invalid-and-cleared-url',
        fallback: 'configured HTTP',
        stale_https_assurance: false,
      });
      const changes = await page.evaluate(() => window.fixtureChanges);
      assert.deepEqual(changes, [
        { trusted: true, checked: true },
        { trusted: true, checked: false },
      ]);
      assert.equal(await page.evaluate(() => window.fixtureCopies.length), 8);
      assert(reads.includes('/api/security/status') && reads.includes('/api/state'));
      assert.deepEqual(
        { errors, offOrigin, writes, unexpected },
        { errors: [], offOrigin: [], writes: [], unexpected: [] },
      );
      result.cases.push({
        name,
        width,
        dark,
        browser: browser.version(),
        checks,
        changes,
        copies: 8,
        reads,
        errors,
        offOrigin,
        writes,
        unexpected,
      });
      await page.close();
      await browser.close();
      browser = null;
      result.cleanup.browsers_closed++;
    }
    result.result = 'passed';
  } catch (error) {
    result.error = error.stack || String(error);
    process.exitCode = 1;
  } finally {
    if (browser) {
      await browser.close();
      result.cleanup.browsers_closed++;
    }
    if (server) {
      await new Promise((resolve) => server.close(resolve));
      result.cleanup.servers_closed++;
    }
    result.finished_at = new Date().toISOString();
    fs.writeFileSync(output + '/result.json', JSON.stringify(result, null, 2) + '\n');
    console.log(
      JSON.stringify({
        result: result.result,
        cases: result.cases.length,
        captures: result.captures.length,
        cleanup: result.cleanup,
        error: result.error || null,
      }),
    );
  }
})();
