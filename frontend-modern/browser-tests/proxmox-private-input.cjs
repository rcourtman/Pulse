// Browser checks only; server responses are synthetic, not installed bootstrap.
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const { chromium, webkit } = require('playwright');
(async () => {
  const root = '/workspace/frontend-modern';
  process.chdir(root);
  const { createServer } = await import(path.join(root, 'node_modules/vite/dist/node/index.js'));
  const server = await createServer({
    root,
    configFile: path.join(root, 'vite.config.ts'),
    server: { host: '127.0.0.1', port: 5237, strictPort: true },
  });
  const engine = process.argv.includes('--phone') ? 'webkit' : 'chromium';
  const width = engine === 'webkit' ? 390 : 1280;
  const artifacts = path.join(root, 'node_modules', `proxmox-private-${engine}`);
  fs.mkdirSync(artifacts, { recursive: true });
  let browser;
  const observations = [];
  try {
    await server.listen();
    browser = await { chromium, webkit }[engine].launch(
      engine === 'chromium'
        ? { headless: true, channel: 'chromium', args: ['--no-sandbox'] }
        : { headless: true },
    );
    for (const type of ['pve', 'pbs']) {
      const page = await browser.newPage({
        viewport: { width, height: 900 },
        ...(engine === 'webkit' ? { isMobile: true, hasTouch: true } : {}),
      });
      const errors = [];
      page.on('pageerror', (e) => errors.push(e.message));
      await page.addInitScript(() => {
        window.__copies = [];
        Object.defineProperty(navigator, 'clipboard', {
          value: {
            writeText: async (value) => {
              window.__copies.push(value);
            },
          },
        });
      });
      let count = 0;
      let holdEndpoint, releaseHeld, sawHeld;
      await page.route('**/api/**', async (route) => {
        const url = new URL(route.request().url());
        if (!url.pathname.startsWith('/api/')) return route.continue();
        if (url.pathname === holdEndpoint) {
          holdEndpoint = undefined;
          await new Promise((resolve) => {
            releaseHeld = resolve;
            sawHeld();
          });
        }
        if (url.pathname === '/api/setup-script-url') {
          count++;
          const body = route.request().postDataJSON();
          assert.equal(body.type, type);
          const scriptURL = `http://127.0.0.1:5237/api/setup-script?type=${type}`;
          const command = `( curl -fsSL '${scriptURL}' -o "$install_script"; sudo bash -c 'read -r -s pulse_token; PULSE_SETUP_TOKEN_FILE="$token_file" bash "$1"'; )`;
          return route.fulfill({
            json: {
              type,
              host: body.host,
              url: scriptURL,
              downloadURL: scriptURL,
              scriptFileName: `pulse-setup-${type}.sh`,
              command,
              commandWithEnv: command,
              commandWithoutEnv: command,
              setupToken: 'a'.repeat(32),
              tokenHint: 'aaa…aaa',
              expires: Math.floor(Date.now() / 1000) + 300,
            },
          });
        }
        if (url.pathname === '/api/agent-install-command') {
          const body = route.request().postDataJSON();
          assert.equal(body.type, type);
          assert.equal(body.enableCommands, false);
          assert.equal(body.insecure, false);
          return route.fulfill({
            json: {
              command:
                '( bash "$install_script" --preflight-only; sudo bash -c \'read -r -s pulse_token; bash "$1" --token-file "$token_file" --enable-proxmox\'; )',
              token: 'b'.repeat(32),
            },
          });
        }
        if (url.pathname === '/api/setup-script') {
          assert.ok(!url.searchParams.has('setup_token'));
          return route.fulfill({
            status: 200,
            contentType: 'text/x-shellscript; charset=utf-8',
            headers: { 'Content-Disposition': `attachment; filename="pulse-setup-${type}.sh"` },
            body: '#!/bin/bash\n# no credential in the download\n',
          });
        }
        return route.fulfill({ json: { data: [] } });
      });
      await page.goto(
        `http://127.0.0.1:5237/browser-tests/proxmox-private-input.html?type=${type}`,
      );
      await page.locator('h1').waitFor();
      if (engine === 'webkit')
        await page.evaluate(() => document.documentElement.classList.add('dark'));
      console.log(
        JSON.stringify({
          type,
          errors,
          buttons: await page.getByRole('button').allTextContents(),
          body: (await page.locator('body').innerText()).slice(0, 300),
        }),
      );
      const copy = page.getByRole('button', { name: 'Copy command', exact: true });
      await copy.first().click();
      let dialog = page.getByRole('dialog', { name: 'API token ready' });
      await dialog.waitFor();
      await page.waitForFunction(() => {
        const panel = document.querySelector('[role="dialog"]');
        return panel && getComputedStyle(panel).opacity === '1';
      });
      assert.ok((await dialog.innerText()).includes('silent “Pulse setup token” prompt'));
      assert.ok(!(await page.evaluate(() => window.__copies[0])).includes('a'.repeat(32)));
      await page.screenshot({ path: path.join(artifacts, `${type}-setup.png`), fullPage: true });
      const tokenButton = dialog.getByRole('button', { name: 'Copy token', exact: true });
      await tokenButton.focus();
      await page.keyboard.press('Enter');
      assert.equal(await page.evaluate(() => window.__copies.at(-1)), 'a'.repeat(32));
      await page.keyboard.press('Escape');
      await dialog.waitFor({ state: 'hidden' });
      await copy.first().click();
      await dialog.waitFor();
      assert.equal(count, 1, 'live setup artifact must be reused');
      await dialog.getByRole('button', { name: 'Dismiss', exact: true }).click();
      await page.getByRole('button', { name: 'Close setup', exact: true }).click();
      assert.equal(await page.getByTestId('cache-state').innerText(), 'empty');
      await page.getByRole('button', { name: 'Reopen setup', exact: true }).click();
      await copy.first().click();
      await dialog.waitFor();
      assert.equal(count, 2, 'closed modal must not retain its setup secret');
      await dialog.getByRole('button', { name: 'Dismiss', exact: true }).click();
      await page.getByText('Alternative: Download script manually', { exact: true }).click();
      await page.getByRole('button', { name: 'Download setup script', exact: true }).click();
      await dialog.waitFor();
      assert.equal(count, 2, 'download must reuse the live credential-free artifact');
      await dialog.getByRole('button', { name: 'Dismiss', exact: true }).click();
      await page.getByRole('button', { name: 'Host Telemetry Agent', exact: true }).click();
      await page
        .getByRole('button', {
          name: type === 'pbs' ? 'Copy to clipboard' : 'Copy command',
          exact: true,
        })
        .first()
        .click();
      await dialog.waitFor();
      await page.waitForFunction(() => {
        const panel = document.querySelector('[role="dialog"]');
        return panel && getComputedStyle(panel).opacity === '1';
      });
      assert.ok((await dialog.innerText()).includes('silent “Pulse agent token” prompt'));
      assert.ok(!(await page.evaluate(() => window.__copies.at(-1))).includes('b'.repeat(32)));
      await page.screenshot({ path: path.join(artifacts, `${type}-agent.png`), fullPage: true });
      await dialog.getByRole('button', { name: 'Copy token', exact: true }).click();
      assert.equal(await page.evaluate(() => window.__copies.at(-1)), 'b'.repeat(32));
      await dialog.getByRole('button', { name: 'Dismiss', exact: true }).click();
      // A late issuance must not repopulate a closed modal's secret cache
      // or reopen the global reveal dialog after the user has left the flow.
      for (const endpoint of ['/api/setup-script-url', '/api/agent-install-command']) {
        const closeSetup = page.getByRole('button', { name: 'Close setup', exact: true });
        if (await closeSetup.count()) await closeSetup.click();
        await page.getByRole('button', { name: 'Reopen setup', exact: true }).click();
        if (endpoint.includes('agent-install'))
          await page.getByRole('button', { name: 'Host Telemetry Agent', exact: true }).click();
        else await page.getByRole('button', { name: /Connect via API/ }).click();
        holdEndpoint = endpoint;
        let deadline;
        const held = new Promise((resolve, reject) => {
          sawHeld = resolve;
          deadline = setTimeout(() => reject(new Error('held request not observed')), 15000);
        });
        const nextCopy = page
          .getByRole('button', {
            name:
              endpoint.includes('agent-install') && type === 'pbs'
                ? 'Copy to clipboard'
                : 'Copy command',
            exact: true,
          })
          .first();
        await nextCopy.click();
        await held;
        clearTimeout(deadline);
        await page.getByRole('button', { name: 'Close setup', exact: true }).click();
        const completed = page.waitForResponse(
          (response) => new URL(response.url()).pathname === endpoint,
        );
        releaseHeld();
        await completed;
        await page.waitForTimeout(200); // allow the actual promise continuation, not an API retry
        assert.equal(await page.getByTestId('cache-state').innerText(), 'empty');
        assert.equal(await page.getByRole('dialog').count(), 0);
      }
      assert.deepEqual(errors, []);
      observations.push({
        type,
        engine,
        width,
        setupRequests: count,
        credentialFreeCopies: true,
        separateTokenCopy: true,
        cacheClose: true,
        lateIssuanceDiscarded: true,
        commandsEnabled: false,
        insecure: false,
      });
      await page.close();
    }
    fs.writeFileSync(
      path.join(artifacts, 'result.json'),
      JSON.stringify(
        {
          playwright: require('playwright/package.json').version,
          browser: browser.version(),
          observations,
        },
        null,
        2,
      ),
    );
    console.log(JSON.stringify({ engine, browser: browser.version(), observations, artifacts }));
  } finally {
    if (browser) await browser.close();
    await server.close();
  }
})();
