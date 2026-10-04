// Scoped copy proof: actual installer/Doctor/styles, synthetic token API only.
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const { chromium, webkit } = require('playwright');
(async () => {
  const root = '/workspace/frontend-modern';
  process.chdir(root);
  const artifacts = path.join(root, 'node_modules', 'unix-guidance-browser');
  fs.mkdirSync(artifacts, { recursive: true });
  const { createServer } = await import(path.join(root, 'node_modules/vite/dist/node/index.js'));
  const server = await createServer({
    root,
    configFile: path.join(root, 'vite.config.ts'),
    cacheDir: path.join(root, 'node_modules', '.vite-unix-guidance-copy'),
    optimizeDeps: {
      noDiscovery: true,
      include: ['solid-js', 'solid-js/web', '@solidjs/router'],
    },
    server: { host: '127.0.0.1', port: 5244, strictPort: true },
  });
  const secret = 'browser-synthetic-install-token';
  const results = [];
  let browser;
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
      page.on('pageerror', (error) => {
        errors.push(error.message);
        console.log('PAGE_ERROR', error.message);
      });
      page.on('console', (message) => {
        if (message.type() === 'error') {
          errors.push(message.text());
          console.log('CONSOLE_ERROR', message.text());
        }
      });
      await page.addInitScript(() => {
        window.__copies = [];
        Object.defineProperty(navigator, 'clipboard', {
          value: {
            writeText: async (text) => window.__copies.push(text),
          },
        });
      });
      await page.route('**/api/**', async (route) => {
        const pathname = new URL(route.request().url()).pathname;
        // Do not intercept the production /src/api/*.ts module requests.
        if (!pathname.startsWith('/api/')) return route.continue();
        if (pathname === '/api/security/status')
          return route.fulfill({
            json: {
              requiresAuth: true,
              hasAuthentication: true,
              apiTokenConfigured: true,
            },
          });
        if (pathname === '/api/agent-install-command')
          return route.fulfill({
            json: {
              token: secret,
              record: {
                id: 'synthetic-record',
                name: 'Synthetic installer',
                prefix: 'browser',
                suffix: 'token',
                createdAt: new Date().toISOString(),
              },
            },
          });
        if (pathname === '/api/state')
          return route.fulfill({ json: { connectedInfrastructure: [] } });
        return route.fulfill({ json: { data: [] } });
      });
      await page.goto('http://127.0.0.1:5244/browser-tests/unix-private-install.html', {
        waitUntil: 'domcontentloaded',
        timeout: 60_000,
      });
      await page
        .getByRole('heading', { name: 'Private Unix agent installation', exact: true })
        .waitFor({ timeout: 60_000 });
      if (tone === 'dark')
        await page.evaluate(() => document.documentElement.classList.add('dark'));
      await page.getByRole('button', { name: 'Generate token', exact: true }).first().click();
      const dialog = page.getByRole('dialog', { name: 'API token ready' });
      await dialog.waitFor();
      await dialog.getByRole('button', { name: 'Dismiss', exact: true }).click();
      await dialog.waitFor({ state: 'hidden' });
      const privateNote = page
        .locator('span')
        .filter({
          hasText: /^Before running, save the separately revealed token/,
        })
        .first();
      const fileText = await privateNote.innerText();
      assert.ok(
        fileText.includes(
          'This command does not prompt or delete your file. Remove it through the same trusted file path afterwards.',
        ),
      );
      for (const warning of [
        '0600',
        '0700',
        'Never put the token in a GUI command field',
        'console or SSH instead',
      ])
        assert.ok(fileText.includes(warning));
      await page
        .getByRole('button', {
          name: 'Copy Install from a private token file (no terminal) command',
          exact: true,
        })
        .click();
      const fileCommand = await page.evaluate(() => window.__copies.at(-1));
      assert.ok(fileCommand.includes('/root/.config/pulse-agent/bootstrap-token'));
      assert.ok(fileCommand.includes('--token-file'));
      assert.ok(!fileCommand.includes('read -r'));
      assert.ok(!fileCommand.includes(secret));
      await privateNote.screenshot({ path: path.join(artifacts, `${engine}-private-file.png`) });
      await page
        .getByRole('button', { name: 'Show details for removed-fixture', exact: true })
        .click();
      const removalNote = page.locator('p').filter({
        hasText:
          'This agent was removed from Pulse, but the agent software may still be installed on its host.',
      });
      const removalText = await removalNote.innerText();
      assert.ok(
        removalText.includes('token at its silent prompt. Never insert it into the command.'),
      );
      assert.ok(removalText.includes('Pulse does not run commands remotely'));
      await page
        .getByRole('button', {
          name: 'Copy Linux / macOS / FreeBSD uninstall command for removed-fixture',
          exact: true,
        })
        .click();
      const removalCommand = await page.evaluate(() => window.__copies.at(-1));
      assert.ok(removalCommand.includes('removed-agent-42'));
      assert.ok(removalCommand.includes('--uninstall'));
      assert.ok(!removalCommand.includes('--preflight-only'));
      assert.ok(!removalCommand.includes(secret));
      await removalNote.screenshot({ path: path.join(artifacts, `${engine}-removed-note.png`) });
      assert.ok(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1));
      await page.screenshot({ path: path.join(artifacts, `${engine}-viewport.png`) });
      assert.deepEqual(errors, []);
      results.push({
        engine,
        version: browser.version(),
        width,
        tone,
        fileText,
        removalText,
        privateFileCopied: true,
        scopedRemovalCopied: true,
        errors,
      });
      await browser.close();
      browser = undefined;
    }
    fs.writeFileSync(
      path.join(artifacts, 'result.json'),
      JSON.stringify(
        {
          playwright: require('playwright/package.json').version,
          results,
        },
        null,
        2,
      ),
    );
    console.log(JSON.stringify({ result: 'passed', artifacts, results }));
  } finally {
    if (browser) await browser.close();
    await server.close();
  }
})().catch((error) => {
  console.error(error);
  process.exitCode = 1;
});
