// Actual operations/install/Doctor/workspace components with synthetic local API data.
// Copied commands are observed, not executed; no host, credential or service changes.
const assert = require('node:assert/strict');
const fs = require('node:fs');
const crypto = require('node:crypto');
const { chromium, webkit } = require('playwright');
const hash = (p) => crypto.createHash('sha256').update(fs.readFileSync(p)).digest('hex');
const origin = 'http://127.0.0.1:5361';
const token = 'synthetic-browser-install-token';
const binding = JSON.parse(fs.readFileSync('/workspace/tmp/uninstall-browser-binding.json'));
const output = '/workspace/tmp/agent-uninstall-browser';
fs.mkdirSync(output, { recursive: true });
const result = {
  binding,
  playwright: require('playwright/package.json').version,
  cases: [],
  captures: [],
  completed: false,
  cleanup: [],
  limits:
    'Synthetic production UI components and API doubles; no PowerShell execution, physical device, native removal, publication or release.',
};
assert.equal(
  result.playwright,
  JSON.parse(fs.readFileSync('/workspace/tests/integration/package-lock.json')).packages[
    'node_modules/@playwright/test'
  ].version,
);
(async () => {
  for (const [mode, source] of Object.entries(binding.scenarios)) {
    const root = '/workspace/' + source.root;
    for (const [p, h] of Object.entries(source.content_sha256))
      assert.equal(hash(root + '/' + p.replace('frontend-modern/', '')), h, p);
    const { createServer } = await import(root + '/node_modules/vite/dist/node/index.js');
    const server = await createServer({
      root,
      configFile: root + '/vite.config.ts',
      cacheDir: output + '/' + mode + '-cache',
      server: { host: '127.0.0.1', port: 5361, strictPort: true, watch: null },
      optimizeDeps: { noDiscovery: true, include: [] },
    });
    let browser;
    try {
      await server.listen();
      for (const [engine, width, dark] of [
        [chromium, 1280, false],
        [webkit, 390, true],
      ]) {
        browser = await engine.launch(
          engine === chromium
            ? { headless: true, channel: 'chromium', args: ['--no-sandbox'] }
            : { headless: true },
        );
        const page = await browser.newPage({
          viewport: { width, height: 900 },
          isMobile: width < 500,
          hasTouch: width < 500,
          locale: 'en-GB',
        });
        page.setDefaultTimeout(9000);
        const record = {
          mode,
          width,
          dark,
          browser: browser.version(),
          reads: [],
          writes: [],
          unexpected: [],
          offOrigin: [],
          errors: [],
          commands: [],
          screenshots: [],
          minted: 0,
        };
        result.cases.push(record);
        let optional = false;
        page.on('pageerror', (e) => record.errors.push(e.message));
        page.on('console', (m) => {
          if (m.type() === 'error') record.errors.push(m.text().slice(0, 500));
        });
        await page.addInitScript((dark) => {
          window.__copies = [];
          Object.defineProperty(navigator, 'clipboard', {
            value: { writeText: async (text) => window.__copies.push(text) },
          });
          document.addEventListener(
            'DOMContentLoaded',
            () => document.documentElement.classList.toggle('dark', dark),
            { once: true },
          );
        }, dark);
        await page.route('**/*', async (route) => {
          const req = route.request(),
            url = new URL(req.url()),
            p = url.pathname;
          if (url.origin !== origin) {
            record.offOrigin.push(url.origin);
            return route.abort();
          }
          if (!p.startsWith('/api/')) return route.continue();
          if (req.method() !== 'GET') {
            record.writes.push({ p, method: req.method() });
            if (p === '/api/agent-install-command') {
              assert.equal(req.postDataJSON().type, 'host');
              record.minted++;
              return route.fulfill({
                json: {
                  token,
                  record: {
                    id: 'synthetic-install-record',
                    name: 'Synthetic installer',
                    prefix: 'synthetic',
                    suffix: 'test',
                    createdAt: new Date().toISOString(),
                  },
                },
              });
            }
            return route.fulfill({ status: 403, json: { error: 'Fixture refuses mutation' } });
          }
          record.reads.push(p);
          let data;
          if (p === '/api/security/status')
            data = {
              requiresAuth: !optional,
              hasAuthentication: !optional,
              apiTokenConfigured: !optional,
              agentUrl: 'https://download.invalid/base',
            };
          else if (p === '/api/state') data = { connectedInfrastructure: [] };
          else if (p === '/api/connections')
            data = {
              systems: [],
              connections: [
                {
                  id: 'agent:workspace-agent',
                  type: 'agent',
                  name: 'workspace-host',
                  address: 'workspace.example',
                  state: 'active',
                  stateReason: '',
                  enabled: true,
                  surfaces: ['host'],
                  scope: { host: true },
                  lastSeen: new Date().toISOString(),
                  lastError: null,
                  source: 'agent',
                  agentVersion: '6.5.0',
                  agentIdentity: { hostname: 'workspace.example', platform: 'debian' },
                  capabilities: { supportsPause: false, supportsScope: false, supportsTest: false },
                },
              ],
            };
          else if (p === '/api/license/runtime-capabilities')
            data = {
              capabilities: [],
              limits: [],
              hosted_mode: false,
              blocked_capabilities: [],
              runtime: { build: 'community' },
            };
          else if (p === '/api/health') data = { status: 'ok' };
          else {
            record.unexpected.push(p);
            return route.fulfill({ status: 404, json: { error: 'Unexpected fixture read' } });
          }
          return route.fulfill({ json: data });
        });
        const capture = async (name, locator) => {
          await locator.scrollIntoViewIfNeeded();
          await page.waitForTimeout(180);
          const path = `${output}/${mode}-${width}-${name}.png`;
          await page.screenshot({ path, animations: 'disabled' });
          const item = { path: path.replace('/workspace/', ''), sha256: hash(path) };
          result.captures.push(item);
          record.screenshots.push(item);
          assert.ok(
            await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1),
            'page overflow',
          );
        };
        const copy = async (name, platform, stage, scope = page) => {
          const button = scope.getByRole('button', { name, exact: true });
          await button.focus();
          await page.keyboard.press('Enter');
          const command = await page.evaluate(() => window.__copies.at(-1));
          assert.equal(typeof command, 'string');
          const safe =
            platform === 'windows'
              ? !command.includes('PULSE_TOKEN') &&
                !command.includes(token) &&
                command.includes("-Url '' -Token '' -TokenFile ''")
              : !command.includes('Pulse agent token') &&
                !command.includes('--token') &&
                !command.includes(token) &&
                !command.includes('--url');
          assert.ok(command.includes(platform === 'windows' ? 'install.ps1' : '--uninstall'));
          assert.ok(!command.includes('\n'));
          record.commands.push({ name, platform, stage, safe, command });
          if (mode === 'final') assert.ok(safe, name + ' ' + stage);
          return command;
        };
        await page.goto(origin + '/browser-tests/agent-uninstall.html', {
          waitUntil: 'domcontentloaded',
          timeout: 60000,
        });
        await page
          .getByRole('heading', { name: 'Saved-credential agent removal', exact: true })
          .waitFor();
        const summary = page.locator('summary').filter({ hasText: 'Uninstall agent' });
        await summary.click();
        const checkInstaller = async (stage) => {
          for (const [name, platform] of [
            ['Copy Linux macOS FreeBSD uninstall command', 'unix'],
            ['Copy Windows uninstall command', 'windows'],
          ]) {
            const cmd = await copy(name, platform, stage);
            const shown = page
              .getByRole('button', { name, exact: true })
              .locator('..')
              .locator('code');
            assert.equal(await shown.textContent(), cmd, 'render/copy parity');
          }
          assert.equal(record.minted, stage === 'minted' ? 1 : 0);
          await capture(stage, summary);
        };
        await checkInstaller('unminted');
        if (binding.installer_only) {
          assert.ok(
            (await page.locator('body').innerText()).includes(
              'If a Unix install has saved credentials, its server must confirm removal',
            ),
          );
          assert.deepEqual(record.writes, []);
          assert.deepEqual(record.errors, []);
          assert.deepEqual(record.unexpected, []);
          assert.deepEqual(record.offOrigin, []);
          await browser.close();
          browser = undefined;
          console.log(
            JSON.stringify({
              mode,
              width,
              installer_only: true,
              copies: record.commands.length,
              minted: record.minted,
            }),
          );
          continue;
        }
        for (const target of ['removed-fixture', 'removed-windows']) {
          await page
            .getByRole('button', { name: 'Show details for ' + target, exact: true })
            .click();
        }
        for (const [name, platform, id] of [
          [
            'Copy Linux / macOS / FreeBSD uninstall command for removed-fixture',
            'unix',
            'removed-agent-42',
          ],
          [
            'Copy Windows PowerShell uninstall command for removed-windows',
            'windows',
            'removed-windows-42',
          ],
        ]) {
          const cmd = await copy(name, platform, 'doctor-unminted');
          assert.ok(cmd.includes(id));
        }
        await capture(
          'doctor',
          page.getByRole('button', {
            name: 'Copy Windows PowerShell uninstall command for removed-windows',
            exact: true,
          }),
        );
        await page.getByRole('button', { name: 'Generate token', exact: true }).first().click();
        const dialog = page.getByRole('dialog', { name: 'API token ready' });
        await dialog.waitFor();
        await dialog.getByRole('button', { name: 'Dismiss', exact: true }).click();
        await checkInstaller('minted');
        await page
          .getByRole('button', {
            name: 'Show advanced connection and install options',
            exact: true,
          })
          .click();
        await page
          .getByLabel('Connection URL (Agent → Pulse)', { exact: true })
          .fill('https://download.invalid/custom/');
        await page
          .getByLabel('Custom CA certificate path (optional)', { exact: true })
          .fill('/explicit/ca.pem');
        const identityCopy = await copy('Copy Unix uninstall', 'unix', 'identity-ca');
        assert.ok(
          identityCopy.includes('agent-fixture-42') &&
            identityCopy.includes('node.example') &&
            identityCopy.includes('--cacert'),
        );
        const winCopy = await copy('Copy Windows uninstall command', 'windows', 'custom-ca');
        assert.ok(winCopy.includes('/explicit/ca.pem'));
        await capture('custom-ca', summary);
        // Installation remains credential-gated/private; removal never unlocks it.
        const installCommand = await page
          .locator('code')
          .filter({ hasText: 'Pulse agent token' })
          .first()
          .textContent();
        assert.ok(installCommand.includes('--token-file') && !installCommand.includes(token));
        optional = true;
        await page.goto(origin + '/browser-tests/agent-uninstall.html?optional=1', {
          waitUntil: 'domcontentloaded',
          timeout: 60000,
        });
        await summary.waitFor();
        await summary.click();
        await copy('Copy Linux macOS FreeBSD uninstall command', 'unix', 'optional-auth');
        await copy('Copy Windows uninstall command', 'windows', 'optional-auth');
        assert.equal(record.minted, 1);
        optional = false;
        await page.goto(origin + '/browser-tests/agent-uninstall.html?workspace=1', {
          waitUntil: 'domcontentloaded',
          timeout: 60000,
        });
        await page
          .getByRole('button', { name: /^Manage/ })
          .first()
          .click();
        const section = page
          .getByText('Uninstall commands', { exact: true })
          .locator('..')
          .locator('..');
        const buttons = section.getByRole('button', {
          name: 'Copy uninstall command',
          exact: true,
        });
        assert.equal(await buttons.count(), 2);
        for (let i = 0; i < 2; i++) {
          await buttons.nth(i).click();
          const command = await page.evaluate(() => window.__copies.at(-1));
          const platform = i === 0 ? 'unix' : 'windows';
          const safe =
            !command.includes(token) &&
            !command.includes('Pulse agent token') &&
            !command.includes('--token') &&
            (platform !== 'windows' || !command.includes('PULSE_TOKEN'));
          record.commands.push({
            name: 'Workspace copy ' + platform,
            stage: 'workspace',
            platform,
            safe,
            command,
          });
          if (mode === 'final') assert.ok(safe);
        }
        await capture('workspace', section);
        assert.equal(record.minted, 1, 'removal minted a token');
        assert.deepEqual(record.writes, [{ p: '/api/agent-install-command', method: 'POST' }]);
        assert.deepEqual(record.unexpected, []);
        assert.deepEqual(record.offOrigin, []);
        assert.deepEqual(record.errors, []);
        if (mode === 'parent') {
          assert.equal(
            record.commands.find((x) => x.stage === 'unminted' && x.platform === 'unix').safe,
            false,
          );
          assert.equal(
            record.commands.find((x) => x.stage === 'unminted' && x.platform === 'windows').safe,
            false,
          );
        }
        await browser.close();
        browser = undefined;
        console.log(
          JSON.stringify({
            mode,
            width,
            copies: record.commands.length,
            minted: record.minted,
            errors: record.errors,
          }),
        );
      }
    } finally {
      if (browser) await browser.close();
      await server.close();
      result.cleanup.push({ mode, browserClosed: true, serverClosed: true });
      fs.writeFileSync(output + '/result.json', JSON.stringify(result, null, 2) + '\n');
    }
  }
  result.completed = true;
  fs.writeFileSync(output + '/result.json', JSON.stringify(result, null, 2) + '\n');
  console.log('PASS copied removal controls for every supplied source scenario');
})().catch((e) => {
  console.error(e);
  process.exitCode = 1;
});
