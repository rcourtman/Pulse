// UI proof only. No real token issuance, appliance, download or installation.
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const http = require('node:http');
const crypto = require('node:crypto');
const { chromium, webkit } = require('playwright');
const hash = (bytes) => crypto.createHash('sha256').update(bytes).digest('hex');
(async () => {
  const workspace = process.env.PULSE_BROWSER_PROOF_ROOT || '/workspace';
  const root = path.join(workspace, 'frontend-modern');
  const artifacts = path.join(workspace, 'tmp/unix-private-install');
  const preview = path.join(artifacts, 'static');
  const bindingBytes = fs.readFileSync(path.join(preview, 'binding.json'));
  const binding = JSON.parse(bindingBytes);
  for (const [file, expected] of Object.entries(binding.sources))
    assert.equal(hash(fs.readFileSync(path.join(root, file))), expected, file);
  for (const [file, expected] of Object.entries(binding.artifacts))
    assert.equal(hash(fs.readFileSync(path.join(preview, file))), expected, file);
  const served = [];
  const server = http.createServer((req, res) => {
    const file = new URL(req.url, 'http://fixture.invalid').pathname.slice(1);
    if (!['GET', 'HEAD'].includes(req.method) || !Object.hasOwn(binding.artifacts, file)) {
      res.writeHead(404);
      res.end();
      return;
    }
    const bytes = fs.readFileSync(path.join(preview, file));
    assert.equal(hash(bytes), binding.artifacts[file]);
    served.push({ file, sha256: hash(bytes) });
    res.writeHead(200, {
      'Content-Type': file.endsWith('.js')
        ? 'text/javascript'
        : file.endsWith('.css')
          ? 'text/css'
          : 'text/html',
      'Cache-Control': 'no-store',
    });
    res.end(bytes);
  });
  const captures = [],
    consumerCommands = [],
    requests = [];
  let cleanup = { browser_closed: false, server_closed: false };
  const results = [];
  let browser;
  const secret = 'browser-synthetic-install-token';
  try {
    await new Promise((resolve) => server.listen(0, '127.0.0.1', resolve));
    const origin = `http://127.0.0.1:${server.address().port}`;
    assert.equal(require('playwright/package.json').version, '1.56.1');
    for (const [engine, width, tone] of [
      ['chromium', 1280, 'light'],
      ['chromium', 1280, 'dark'],
      ['webkit', 390, 'light'],
      ['webkit', 390, 'dark'],
    ]) {
      browser = await { chromium, webkit }[engine].launch(
        engine === 'chromium'
          ? { headless: true, channel: 'chromium', args: ['--no-sandbox'] }
          : { headless: true },
      );
      const page = await browser.newPage({
        viewport: { width, height: 900 },
        ...(engine === 'webkit' ? { isMobile: true, hasTouch: true } : {}),
      });
      await page.clock.install();
      const errors = [];
      const capture = async (name, locator) => {
        if (locator)
          await locator.evaluate((el) => window.scrollBy(0, el.getBoundingClientRect().top - 48));
        await page.evaluate(
          () =>
            new Promise((resolve) => requestAnimationFrame(() => requestAnimationFrame(resolve))),
        );
        assert.equal(
          await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth),
          true,
          name,
        );
        const file = `${engine}-${tone}-${name}.png`;
        await page.screenshot({ path: path.join(artifacts, file) });
        captures.push({
          file,
          sha256: hash(fs.readFileSync(path.join(artifacts, file))),
          engine,
          tone,
          width,
          height: 900,
        });
      };
      const activate = async (locator) => {
        if (engine === 'webkit') await locator.tap();
        else {
          await locator.focus();
          await page.keyboard.press('Enter');
        }
      };
      page.on('pageerror', (e) => errors.push(e.message));
      page.on('console', (m) => {
        if (m.type() === 'error') {
          console.log('PAGE_CONSOLE_ERROR', m.text());
          errors.push(m.text());
        }
      });
      page.on('requestfailed', (r) => console.log('REQUEST_FAILED', r.url(), r.failure()));
      let minted = 0;
      let lookupReads = 0;
      const seen = new Date(Date.now() - 5000).toISOString();
      let optionalAuth = false;
      await page.addInitScript(() => {
        window.__copies = [];
        Object.defineProperty(navigator, 'clipboard', {
          value: { writeText: async (text) => window.__copies.push(text) },
        });
      });
      page.on('request', (req) =>
        requests.push({ path: new URL(req.url()).pathname, method: req.method() }),
      );
      await page.route('**/api/**', async (route) => {
        const url = new URL(route.request().url());
        if (!url.pathname.startsWith('/api/')) return route.continue();
        if (url.pathname === '/api/security/status')
          return route.fulfill({
            json: {
              requiresAuth: !optionalAuth,
              hasAuthentication: !optionalAuth,
              apiTokenConfigured: !optionalAuth,
            },
          });
        if (url.pathname === '/api/agent-install-command') {
          const body = route.request().postDataJSON();
          assert.equal(body.type, 'host');
          assert.equal(body.enableCommands, false);
          minted++;
          return route.fulfill({
            json: {
              token: secret,
              record: {
                id: 'synthetic-token-record',
                name: 'Synthetic installer',
                prefix: 'browser',
                suffix: 'token',
                createdAt: new Date().toISOString(),
              },
            },
          });
        }
        if (url.pathname === '/api/agents/agent/lookup') {
          lookupReads++;
          return route.fulfill({
            json: {
              agent: {
                id: 'fixture-connected',
                hostname: 'fixture-host',
                displayName: 'Fixture host',
                connected: true,
                status: 'online',
                lastSeen: seen,
              },
            },
          });
        }
        if (url.pathname === '/api/state')
          return route.fulfill({ json: { connectedInfrastructure: [] } });
        return route.fulfill({ json: { data: [] } });
      });
      await page.goto(`${origin}/browser-tests/unix-private-install.html`);
      await page.waitForTimeout(1000);
      console.log('PAGE_TITLE', await page.title());
      console.log(
        JSON.stringify({
          engine,
          errors,
          body: (await page.locator('body').innerText()).slice(0, 300),
        }),
      );
      await page
        .getByRole('heading', { name: 'Private Unix agent installation', exact: true })
        .waitFor();
      if (tone === 'dark')
        await page.evaluate(() => document.documentElement.classList.add('dark'));
      await page.getByRole('button', { name: 'Generate token', exact: true }).first().click();
      const dialog = page.getByRole('dialog', { name: 'API token ready' });
      await dialog.waitFor();
      await page.waitForFunction(() => {
        const d = document.querySelector('[role="dialog"]');
        return d && getComputedStyle(d).opacity === '1';
      });
      assert.match(await dialog.innerText(), /silent “Pulse agent token” prompt/);
      const copyToken = dialog.getByRole('button', { name: 'Copy token', exact: true });
      await copyToken.focus();
      await page.keyboard.press('Enter');
      assert.equal(await page.evaluate(() => window.__copies.at(-1)), secret);
      await capture('token-dialog');
      await page.keyboard.press('Escape');
      await dialog.waitFor({ state: 'hidden' });
      // The real component's platform cards and copy handlers.
      const code = page.locator('code').filter({ hasText: 'Pulse agent token' });
      assert.ok((await code.count()) >= 3);
      const unixCommands = await code.allTextContents();
      for (const command of unixCommands) {
        assert.ok(!command.includes(secret));
        assert.ok(command.includes('--token-file "$token_file"'));
        assert.ok(!/[\r\n]/.test(command));
        assert.ok(command.includes('tty_control=(busybox stty)'));
        assert.ok(command.includes('Cannot disable terminal echo'));
        assert.ok(command.includes('private token-file command'));
      }
      const guiCode = page.locator('code').filter({ hasText: 'token_parent=${token_file%/*}' });
      assert.equal(await guiCode.count(), 2);
      const guiCommand = await guiCode.first().innerText();
      assert.equal(await guiCode.last().innerText(), guiCommand);
      assert.ok(guiCommand.includes('/root/.config/pulse-agent/bootstrap-token'));
      assert.ok(!guiCommand.includes('read -r'));
      assert.ok(!guiCommand.includes(secret));
      assert.ok(
        (await page.locator('body').innerText()).includes(
          'Never put the token in a GUI command field',
        ),
      );
      await activate(page.getByRole('button', { name: 'Copy Install command', exact: true }));
      assert.equal(await page.evaluate(() => window.__copies.at(-1)), unixCommands[0]);
      const fileCopy = page
        .getByRole('button', {
          name: 'Copy Install from a private token file (no terminal) command',
          exact: true,
        })
        .first();
      await activate(fileCopy);
      const linuxCard = page
        .getByRole('heading', { name: 'Install on Linux', exact: true })
        .locator('..')
        .locator('..');
      assert.match(await linuxCard.innerText(), /BusyBox/);
      assert.match(await linuxCard.innerText(), /root-owned/);
      await capture('linux-private-file', fileCopy);
      assert.equal(await page.evaluate(() => window.__copies.at(-1)), guiCommand);
      consumerCommands.push({ engine, tone, kind: 'linux-file', command: guiCommand });
      await activate(
        page.getByRole('button', { name: 'Show details for doctor-fixture', exact: true }),
      );
      await page
        .getByRole('button', {
          name: 'Copy authentication repair command for doctor-fixture',
          exact: true,
        })
        .click();
      const doctorCommand = await page.evaluate(() => window.__copies.at(-1));
      assert.ok(!doctorCommand.includes(secret));
      assert.ok(doctorCommand.includes('tty_control=(busybox stty)'));
      assert.ok(doctorCommand.includes('--update --enable-docker'));
      assert.ok(doctorCommand.includes('doctor-fixture'));
      consumerCommands.push({ engine, tone, kind: 'doctor-repair', command: doctorCommand });
      await capture(
        'doctor-repair',
        page.getByRole('button', {
          name: 'Copy authentication repair command for doctor-fixture',
          exact: true,
        }),
      );

      await page.getByRole('button', { name: 'Show token only', exact: true }).click();
      await dialog.waitFor();
      assert.equal(minted, 1);
      await dialog.getByRole('button', { name: 'Dismiss', exact: true }).click();
      await page
        .getByRole('button', { name: 'Show advanced connection and install options', exact: true })
        .click();
      const signing = page
        .locator('div')
        .filter({ has: page.getByText('Config signing (optional)', { exact: true }) })
        .last();
      for (const key of [
        'PULSE_AGENT_CONFIG_SIGNATURE_REQUIRED=true',
        'PULSE_AGENT_CONFIG_SIGNING_KEY',
        'PULSE_AGENT_CONFIG_PUBLIC_KEYS',
      ]) {
        const code = signing.getByText(key, { exact: true });
        await code.scrollIntoViewIfNeeded();
        const bounds = await code.boundingBox();
        assert.ok(bounds && bounds.x >= 0 && bounds.x + bounds.width <= width, key);
        assert.match(await code.getAttribute('class'), /wrap-anywhere/);
      }
      await capture('config-signing', signing);
      await page
        .getByLabel('Connection URL (Agent → Pulse)', { exact: true })
        .fill('https://pulse.example/base/');
      await page
        .getByLabel('Custom CA certificate path (optional)', { exact: true })
        .fill('/etc/pulse/ca.pem');
      for (const label of [
        'Copy Unix uninstall',
        'Copy Unix credential repair',
        'Copy Unix saved-state update',
      ]) {
        await page.getByRole('button', { name: label, exact: true }).click();
        const command = await page.evaluate(() => window.__copies.at(-1));
        assert.ok(!command.includes(secret));
        assert.ok(command.includes('https://pulse.example/base'));
        assert.ok(command.includes('--cacert'));
        assert.ok(!command.includes('| bash'));
        if (label.includes('uninstall')) {
          assert.ok(command.includes('--uninstall'));
          assert.ok(command.includes('agent-fixture-42'));
          assert.ok(command.includes('node.example'));
          assert.ok(!command.includes('--preflight-only'));
        } else {
          assert.ok(command.includes('--update'));
          assert.ok(command.includes('--enable-docker'));
          assert.ok(command.includes('--preflight-only'));
          if (label.includes('credential')) {
            assert.ok(command.includes('agent-fixture-42'));
            assert.ok(command.includes('node.example'));
            assert.ok(command.includes('--token-file'));
          } else {
            assert.ok(!command.includes('--token-file'));
            assert.ok(!command.includes('read -r'));
          }
        }
      }
      assert.ok(
        (await page.locator('body').innerText()).includes(
          'Never insert the token into the command',
        ),
      );
      await capture(
        'trust-options',
        page.getByLabel('Custom CA certificate path (optional)', { exact: true }),
      );
      consumerCommands.push({ engine, tone, kind: 'linux-install', command: unixCommands[0] });
      await page.getByPlaceholder('Hostname or agent ID').fill('fixture-host');
      await page.getByRole('button', { name: 'Check status', exact: true }).click();
      await page.getByText('First host connected', { exact: true }).waitFor();
      const age = page.getByText(/^Last seen /);
      const initialAge = await age.innerText();
      const readsBeforeTick = lookupReads;
      await page.clock.runFor(65_000);
      assert.notEqual(await age.innerText(), initialAge);
      assert.equal(lookupReads, readsBeforeTick);
      await capture('connected-age', age);
      optionalAuth = true;
      await page.goto(`${origin}/browser-tests/unix-private-install.html?optional=1`);
      await page.getByRole('button', { name: 'Confirm without token', exact: true }).click();
      const optionalCode = page.locator('code').filter({ hasText: 'bootstrap_dir=$(mktemp' });
      const optionalCommands = await optionalCode.allTextContents();
      assert.ok(optionalCommands.length >= 3);
      for (const command of optionalCommands) {
        assert.ok(!command.includes('--token-file'));
        assert.ok(!command.includes('read -r'));
      }
      assert.equal(
        await page
          .getByRole('button', {
            name: 'Copy Install from a private token file (no terminal) command',
            exact: true,
          })
          .count(),
        0,
      );
      assert.deepEqual(errors, []);
      results.push({
        engine,
        version: browser.version(),
        width,
        tone,
        issued: minted,
        copied: await page.evaluate(() => window.__copies.length),
        unixCommands: unixCommands.length,
        guiCommand: true,
        optionalAuth: true,
        errors,
      });
      await browser.close();
      browser = undefined;
    }
    assert.equal(captures.length, 24);
  } finally {
    if (browser) await browser.close();
    cleanup.browser_closed = true;
    await new Promise((resolve) => server.close(resolve));
    cleanup.server_closed = true;
    fs.writeFileSync(
      path.join(artifacts, 'consumer-commands.json'),
      JSON.stringify({ source: binding.source_sha, consumerCommands }, null, 2) + '\n',
    );
    fs.writeFileSync(
      path.join(artifacts, 'result.json'),
      JSON.stringify(
        {
          source: binding.source_sha,
          binding_sha256: hash(bindingBytes),
          source_inputs: Object.keys(binding.sources).length,
          playwright: require('playwright/package.json').version,
          results,
          captures,
          requests,
          served,
          cleanup,
        },
        null,
        2,
      ) + '\n',
    );
  }
  console.log(
    JSON.stringify({ result: 'passed', artifacts, results, captures: captures.length, cleanup }),
  );
})().catch((e) => {
  console.error(e);
  process.exitCode = 1;
});
