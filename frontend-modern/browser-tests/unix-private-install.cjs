// UI proof only. No real token issuance, appliance, download or installation.
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const { chromium, webkit } = require('playwright');
(async () => {
  const root = '/workspace/frontend-modern';
  process.chdir(root);
  console.log(
    JSON.stringify({
      fixtureHTML: fs.existsSync(path.join(root, 'browser-tests/unix-private-install.html')),
      fixtureTSX: fs.existsSync(path.join(root, 'browser-tests/unix-private-install.tsx')),
    }),
  );
  const artifacts = path.join(root, 'node_modules', 'unix-private-browser');
  fs.mkdirSync(artifacts, { recursive: true });
  const { createServer } = await import(path.join(root, 'node_modules/vite/dist/node/index.js'));
  const server = await createServer({
    root,
    configFile: path.join(root, 'vite.config.ts'),
    server: { host: '127.0.0.1', port: 5241, strictPort: true },
  });
  const results = [];
  let browser;
  const secret = 'browser-synthetic-install-token';
  try {
    await server.listen();
    for (const [engine, width, tone] of [
      ['chromium', 1280, 'light'],
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
      const errors = [];
      page.on('pageerror', (e) => errors.push(e.message));
      page.on('console', (m) => {
        if (m.type() === 'error') {
          console.log('PAGE_CONSOLE_ERROR', m.text());
          errors.push(m.text());
        }
      });
      page.on('requestfailed', (r) => console.log('REQUEST_FAILED', r.url(), r.failure()));
      let minted = 0;
      let optionalAuth = false;
      await page.addInitScript(() => {
        window.__copies = [];
        Object.defineProperty(navigator, 'clipboard', {
          value: { writeText: async (text) => window.__copies.push(text) },
        });
      });
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
        if (url.pathname === '/api/state')
          return route.fulfill({ json: { connectedInfrastructure: [] } });
        return route.fulfill({ json: { data: [] } });
      });
      await page.goto('http://127.0.0.1:5241/browser-tests/unix-private-install.html');
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
      await page.screenshot({ path: path.join(artifacts, `${engine}-token.png`), fullPage: true });
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
      }
      const guiCode = page.locator('code').filter({ hasText: 'token_parent=${token_file%/*}' });
      assert.equal(await guiCode.count(), 1);
      const guiCommand = await guiCode.innerText();
      assert.ok(guiCommand.includes('/root/.config/pulse-agent/bootstrap-token'));
      assert.ok(!guiCommand.includes('read -r'));
      assert.ok(!guiCommand.includes(secret));
      assert.ok(
        (await page.locator('body').innerText()).includes(
          'Never put the token in a GUI command field',
        ),
      );
      await page.getByRole('button', { name: 'Copy Install command', exact: true }).click();
      assert.equal(await page.evaluate(() => window.__copies.at(-1)), unixCommands[0]);
      await page
        .getByRole('button', {
          name: 'Copy Install from a private token file (no terminal) command',
          exact: true,
        })
        .click();
      assert.equal(await page.evaluate(() => window.__copies.at(-1)), guiCommand);
      await page
        .getByRole('button', {
          name: 'Copy authentication repair command for doctor-fixture',
          exact: true,
        })
        .click();
      assert.ok(!(await page.evaluate(() => window.__copies.at(-1))).includes(secret));

      await page.getByRole('button', { name: 'Show token only', exact: true }).click();
      await dialog.waitFor();
      assert.equal(minted, 1);
      await dialog.getByRole('button', { name: 'Dismiss', exact: true }).click();
      await page
        .getByRole('button', { name: 'Show advanced connection and install options', exact: true })
        .click();
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
      await page.screenshot({
        path: path.join(artifacts, `${engine}-commands.png`),
        fullPage: true,
      });
      optionalAuth = true;
      await page.goto('http://127.0.0.1:5241/browser-tests/unix-private-install.html?optional=1');
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
    fs.writeFileSync(
      path.join(artifacts, 'result.json'),
      JSON.stringify({ playwright: require('playwright/package.json').version, results }, null, 2),
    );
    console.log(JSON.stringify({ result: 'passed', artifacts, results }));
  } finally {
    if (browser) await browser.close();
    await server.close();
  }
})().catch((e) => {
  console.error(e);
  process.exitCode = 1;
});
