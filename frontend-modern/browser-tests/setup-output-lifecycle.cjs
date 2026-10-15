const { chromium } = require('playwright');
const { expect } = require('playwright/test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const http = require('node:http');
const crypto = require('node:crypto');
const root = path.resolve('frontend-modern');
const compiled = path.resolve('tmp/setup-output-lifecycle/static');
const output = path.resolve('tmp/setup-output-lifecycle/browser');
fs.mkdirSync(output, { recursive: true });
const hash = (bytes) => crypto.createHash('sha256').update(bytes).digest('hex');
const binding = JSON.parse(fs.readFileSync(path.join(compiled, 'binding.json')));
for (const [file, digest] of Object.entries(binding.sources))
  assert.equal(hash(fs.readFileSync(path.join(root, file))), digest, `Source changed: ${file}`);
const assets = new Map(
  Object.entries(binding.artifacts).map(([file, digest]) => {
    const bytes = fs.readFileSync(path.join(compiled, file));
    assert.equal(hash(bytes), digest);
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
    pathname === '/settings/infrastructure'
      ? '/browser-tests/setup-output-lifecycle.html'
      : pathname;
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
    await new Promise((resolve) => server.listen(5239, '127.0.0.1', resolve));
    browser = await chromium.launch({
      headless: true,
      channel: 'chromium',
      args: ['--no-sandbox'],
    });
    result.browser = browser.version();
    for (const type of ['pve', 'pbs'])
      for (const width of [1360, 390]) {
        const context = await browser.newContext({
          viewport: { width, height: 940 },
          ...(width === 390 ? { isMobile: true, hasTouch: true } : {}),
          acceptDownloads: true,
        });
        const page = await context.newPage();
        const errors = [];
        const operations = [];
        const downloads = [];
        page.on('pageerror', (error) => errors.push(error.message));
        page.on('download', (download) => downloads.push(download.suggestedFilename()));
        await page.addInitScript(() => {
          // Synthetic CSRF fixture only; this is not full-application auth proof.
          document.cookie = 'pulse_csrf=synthetic-setup-fixture; path=/; SameSite=Lax';
          window.__copies = [];
          window.__holdClipboard = false;
          Object.defineProperty(navigator, 'clipboard', {
            value: {
              writeText: async (value) => {
                window.__copies.push(value);
                if (window.__holdClipboard) {
                  window.__holdClipboard = false;
                  await new Promise((resolve) => {
                    window.__releaseClipboard = resolve;
                  });
                }
              },
            },
          });
        });
        let held = null;
        let responseMode = 'success';
        let counter = 0;
        const waitForHold = async () => {
          for (let i = 0; i < 60 && !held?.release; i++) await page.waitForTimeout(50);
          assert.ok(held?.release, 'held API observed');
        };
        await page.route('**/api/**', async (route) => {
          const req = route.request();
          const url = new URL(req.url());
          operations.push({
            method: req.method(),
            path: url.pathname,
            ...(req.method() === 'POST' ? { body: req.postDataJSON() } : {}),
          });
          if (held && held.path === url.pathname) {
            await new Promise((resolve) => (held.release = resolve));
            held = null;
          }
          if (url.pathname === '/api/setup-script-url') {
            const body = req.postDataJSON();
            assert.equal(body.type, type);
            assert.equal(body.backupPerms, type === 'pve');
            counter++;
            const command = `fixture-setup-command-${counter}`;
            return route.fulfill({
              json: {
                type,
                host: body.host,
                url: '/api/setup-script',
                downloadURL: '/api/setup-script',
                scriptFileName: `pulse-setup-${type}.sh`,
                command,
                commandWithEnv: command,
                commandWithoutEnv: command,
                setupToken: `synthetic-setup-token-${counter}`,
                tokenHint: 'synthetic-hint',
                expires: Math.floor(Date.now() / 1000) + 300,
              },
            });
          }
          if (url.pathname === '/api/setup-script')
            return route.fulfill({
              status: 200,
              contentType: 'text/x-shellscript',
              headers: { 'Content-Disposition': `attachment; filename="pulse-setup-${type}.sh"` },
              body: '#!/bin/bash\n# synthetic, no execution\n',
            });
          if (url.pathname === '/api/agent-install-command') {
            const body = req.postDataJSON();
            assert.deepEqual(body, {
              type,
              enableProxmox: true,
              enableCommands: false,
              insecure: false,
            });
            if (responseMode === 'error')
              return route.fulfill({
                status: 503,
                json: { error: 'Synthetic current setup failure' },
              });
            return route.fulfill({
              json: { command: 'fixture-agent-command', token: 'synthetic-agent-token' },
            });
          }
          throw new Error(`Unexpected API: ${url.pathname}`);
        });
        await page.goto(`http://127.0.0.1:5239/settings/infrastructure?type=${type}`);
        if (width === 390)
          await page.evaluate(() => document.documentElement.classList.add('dark'));
        const name = page.getByRole('textbox', { name: 'Node Name' });
        const host = page.getByRole('textbox', { name: 'Endpoint URL' });
        const copy = () => page.getByRole('button', { name: 'Copy command', exact: true });
        const dialog = page.getByRole('dialog', { name: 'API token ready' });
        const checkNoNewOutput = async (copies, downloadCount) => {
          await expect(
            page.getByRole('status').filter({ hasText: 'Preparing setup output' }),
          ).toHaveCount(0);
          await expect(dialog).toHaveCount(0);
          assert.equal(await page.evaluate(() => window.__copies.length), copies);
          assert.equal(downloads.length, downloadCount);
          await expect(page.getByRole('region', { name: 'Notifications' })).toHaveCount(0);
        };
        const dismiss = async () => {
          await dialog.getByRole('button', { name: 'Dismiss', exact: true }).click();
          await expect(dialog).toHaveCount(0);
        };
        const clearNotifications = async () => {
          for (const button of await page
            .getByRole('button', { name: 'Dismiss notification' })
            .all())
            await button.click();
          await expect(page.getByRole('region', { name: 'Notifications' })).toHaveCount(0);
        };
        const capture = async (label) => {
          await page.waitForTimeout(600);
          const file = `${type}-${width}-${label}.png`;
          await page.screenshot({
            path: path.join(output, file),
            fullPage: true,
            animations: 'disabled',
          });
          result.captures.push({
            file,
            sha256: hash(fs.readFileSync(path.join(output, file))),
            type,
            width,
            label,
          });
        };
        // Endpoint ABA cannot revive a minted setup token or copy command.
        held = { path: '/api/setup-script-url' };
        await copy().click();
        await waitForHold();
        await expect(copy()).toBeDisabled();
        await host.fill('https://obsolete.example.invalid');
        await host.fill(`https://${type}.example.invalid`);
        held.release();
        await checkNoNewOutput(0, 0);
        await expect(page.getByTestId('cache-state')).toHaveText('empty');
        assert.equal(counter, 1);
        // Explicit current copy still works and reuses its live setup artifact.
        await name.fill('Synthetic current connection');
        await copy().click();
        await expect(dialog).toBeVisible();
        await expect(dialog).toContainText('silent “Pulse setup token” prompt');
        assert.equal(await page.evaluate(() => window.__copies.at(-1)), 'fixture-setup-command-2');
        await dismiss();
        await copy().click();
        await expect(dialog).toBeVisible();
        assert.equal(counter, 2);
        await dismiss();
        await clearNotifications();
        // Retire the script download after the name changes, not just minting.
        await page.getByText('Alternative: Download script manually', { exact: true }).click();
        held = { path: '/api/setup-script' };
        await page.getByRole('button', { name: 'Download setup script', exact: true }).click();
        await waitForHold();
        await name.fill('Synthetic revised connection');
        await expect(copy()).toBeDisabled();
        await expect(
          page.getByRole('button', { name: 'Download setup script', exact: true }),
        ).toBeDisabled();
        await page.evaluate(() => window.scrollTo(0, 0));
        await capture('pending-revised-draft');
        held.release();
        await checkNoNewOutput(2, 0);
        await expect(page.getByTestId('cache-state')).toHaveText('empty');
        // Fresh download through the current consuming UI still produces a file and a separate token.
        await page.getByRole('button', { name: 'Download setup script', exact: true }).click();
        await expect(dialog).toBeVisible();
        await expect(dialog).toContainText('silent “Pulse setup token” prompt');
        assert.deepEqual(downloads, [`pulse-setup-${type}.sh`]);
        assert.equal(counter, 3);
        await dismiss();
        await clearNotifications();
        // Independent optional-agent path shares single-flight and revision ownership.
        await page.getByRole('button', { name: 'Host Telemetry Agent', exact: true }).click();
        const agentCopy = page.getByRole('button', {
          name: type === 'pbs' ? 'Copy to clipboard' : 'Copy command',
          exact: true,
        });
        held = { path: '/api/agent-install-command' };
        await agentCopy.click();
        await waitForHold();
        const tls = page.getByRole('checkbox', {
          name: /Skip TLS certificate verification when downloading the installer/,
        });
        await tls.check();
        await tls.uncheck();
        await expect(agentCopy).toBeDisabled();
        held.release();
        await checkNoNewOutput(2, 1);
        // A write already sent to the clipboard cannot be cancelled, but cannot reveal a stale token.
        await page.evaluate(() => (window.__holdClipboard = true));
        await agentCopy.click();
        await page.waitForFunction(() => Boolean(window.__releaseClipboard));
        await host.fill('https://new-draft.example.invalid');
        await page.evaluate(() => window.__releaseClipboard());
        await checkNoNewOutput(3, 1);
        // Current error remains actionable; next explicit operation succeeds with unchanged safe defaults.
        responseMode = 'error';
        await agentCopy.click();
        await expect(
          page.getByText('Synthetic current setup failure', { exact: true }).first(),
        ).toBeVisible();
        await capture('current-failure');
        await clearNotifications();
        responseMode = 'success';
        await agentCopy.focus();
        if (width === 1360) await page.keyboard.press('Enter');
        else await agentCopy.tap();
        await expect(dialog).toBeVisible();
        await expect(dialog).toContainText('silent “Pulse agent token” prompt');
        assert.equal(await page.evaluate(() => window.__copies.at(-1)), 'fixture-agent-command');
        await dialog.getByRole('button', { name: 'Copy token', exact: true }).click();
        assert.equal(await page.evaluate(() => window.__copies.at(-1)), 'synthetic-agent-token');
        await capture('current-separate-token');
        await dismiss();
        assert.deepEqual(errors, []);
        assert.equal(
          await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth),
          true,
          'no horizontal overflow',
        );
        result.journeys.push({
          type,
          width,
          height: 940,
          operations,
          downloads,
          copies: await page.evaluate(() => window.__copies),
          obsoleteMintRetired: true,
          obsoleteScriptRetired: true,
          agentTLSABARetired: true,
          lateClipboardRetired: true,
          currentFailureKept: true,
          currentCopiesAndDownload: true,
          separateToken: true,
          noPageErrors: true,
          noOverflow: true,
        });
        await context.close();
      }
  } catch (error) {
    result.errors.push(error.stack || String(error));
    process.exitCode = 1;
  } finally {
    if (browser) {
      await browser.close();
      result.cleanup.browserClosed = true;
    }
    await new Promise((resolve) => server.close(resolve));
    result.cleanup.serverClosed = true;
    fs.writeFileSync(path.join(output, 'result.json'), JSON.stringify(result, null, 2) + '\n');
    console.log(JSON.stringify(result, null, 2));
  }
})();
