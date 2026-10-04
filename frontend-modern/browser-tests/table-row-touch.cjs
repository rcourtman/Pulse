// Real production table rows and PBS drawers; synthetic bounded HTTP data.
// Do not instrument rows with native listeners: doing so masks the WebKit bug.
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const { createHash } = require('node:crypto');
const { chromium, firefox, webkit } = require('playwright');

(async () => {
  const root = '/workspace/frontend-modern';
  process.chdir(root);
  const engine = process.argv.find((arg) => arg.startsWith('--engine='))?.split('=')[1] || 'webkit';
  const theme = process.argv.includes('--dark') ? 'dark' : 'light';
  const width = process.argv.includes('--desktop') ? 1365 : 390;
  assert.ok(['chromium', 'firefox', 'webkit'].includes(engine));
  const out = path.join(root, 'node_modules', `table-row-touch-${engine}-${width}-${theme}`);
  fs.mkdirSync(out, { recursive: true });
  fs.chmodSync(out, 0o755);
  const { createServer } = await import(path.join(root, 'node_modules/vite/dist/node/index.js'));
  const server = await createServer({
    root,
    configFile: path.join(root, 'vite.config.ts'),
    cacheDir: fs.mkdtempSync(path.join(out, 'vite-cache-')),
    server: { host: '127.0.0.1', port: 5227, strictPort: true },
  });
  const sourceHashes = Object.fromEntries(
    ['src/components/shared/Table.tsx', 'package-lock.json'].map((file) => [
      file,
      createHash('sha256')
        .update(fs.readFileSync(path.join(root, file)))
        .digest('hex'),
    ]),
  );
  const playwrightVersion = require('playwright/package.json').version;
  assert.equal(
    playwrightVersion,
    JSON.parse(fs.readFileSync('/workspace/tests/integration/package-lock.json')).packages[
      'node_modules/@playwright/test'
    ].version,
  );
  let browser, page;
  const errors = [],
    requests = [],
    observations = [];
  const snapshot = async (name) => {
    const file = path.join(out, name + '.png');
    await page.screenshot({ path: file });
    fs.chmodSync(file, 0o644);
  };
  const activate = (locator) => (width === 390 ? locator.tap() : locator.click());
  const noOverflow = async () =>
    assert.ok(
      await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth),
      'document overflow',
    );
  try {
    await server.listen();
    browser = await { chromium, firefox, webkit }[engine].launch(
      engine === 'chromium'
        ? { headless: true, channel: 'chromium', args: ['--no-sandbox'] }
        : { headless: true },
    );
    page = await browser.newPage({
      viewport: { width, height: width === 390 ? 844 : 900 },
      locale: 'en-GB',
      timezoneId: 'UTC',
      ...(width === 390 ? { isMobile: true, hasTouch: true } : {}),
    });
    page.setDefaultTimeout(20_000);
    page.on('pageerror', (error) => errors.push(error.message));
    await page.route('**/*', async (route) => {
      const url = new URL(route.request().url());
      if (url.origin !== 'http://127.0.0.1:5227') return route.abort();
      if (!url.pathname.startsWith('/api/')) return route.continue();
      requests.push({ path: url.pathname, search: url.search, method: route.request().method() });
      assert.equal(route.request().method(), 'GET');
      if (url.pathname === '/api/license/runtime-capabilities')
        return route.fulfill({
          json: {
            capabilities: [],
            limits: [],
            max_history_days: 7,
            hosted_mode: false,
            runtime: { build: 'community', label: 'Pulse Community runtime' },
            blocked_capabilities: [],
          },
        });
      if (url.pathname === '/api/metrics-store/history') {
        const id = url.searchParams.get('resourceId');
        assert.ok(['vm-one', 'vm-two', 'agent-three'].includes(id), `unexpected target ${id}`);
        const end = Date.UTC(2026, 9, 1, 12);
        return route.fulfill({
          json: {
            resourceType: url.searchParams.get('resourceType'),
            resourceId: id,
            range: url.searchParams.get('range'),
            start: end - 86400000,
            end,
            source: 'store',
            metrics: {
              cpu: [
                { timestamp: end - 600000, value: 20 },
                { timestamp: end - 300000, value: 30 },
              ],
              memory: [{ timestamp: end - 300000, value: 40 }],
              netin: [
                { timestamp: end - 600000, value: 1024 },
                { timestamp: end - 300000, value: 2048 },
              ],
            },
          },
        });
      }
      return route.fulfill({ json: { data: [], enabled: false } });
    });
    console.log(JSON.stringify({ stage: 'production-pbs', engine, theme, width, sourceHashes }));
    await page.goto('http://127.0.0.1:5227/browser-tests/pbs-identity-boundary.html', {
      waitUntil: 'domcontentloaded',
      timeout: 120_000,
    });
    await page.evaluate(
      (dark) => document.documentElement.classList.toggle('dark', dark),
      theme === 'dark',
    );
    await activate(page.getByRole('button', { name: 'Corroborate links', exact: true }));
    for (const suffix of ['one', 'two', 'three']) {
      const cell = page.locator(`td[title="backup-connection-${suffix} · tank"]`);
      const disclosure = cell.getByRole('button');
      assert.equal(await disclosure.getAttribute('aria-expanded'), 'false');
      // No focus, force, dispatchEvent, synthetic click or added event listeners.
      await activate(cell);
      await page.locator(`[data-inline-platform-resource-detail-for="pbs-${suffix}"]`).waitFor();
      assert.equal(await disclosure.getAttribute('aria-expanded'), 'true');
      const detail = page.locator(`[data-inline-platform-resource-detail-for="pbs-${suffix}"]`);
      observations.push({ suffix, firstActivationOpened: true });
      await noOverflow();
      if (suffix === 'one') await snapshot('pbs-first-activation');
      await activate(detail.getByRole('tab', { name: 'History', exact: true }));
      await detail.getByTestId('guest-history-plot').first().locator('path').first().waitFor();
      assert.equal(
        await disclosure.getAttribute('aria-expanded'),
        'true',
        'tab must not retoggle row',
      );
      if (suffix === 'three') await snapshot('pbs-third-history');
      // Close by the same ordinary cell; nested History input cannot consume it.
      await activate(cell);
      assert.equal(await disclosure.getAttribute('aria-expanded'), 'false');
      assert.equal(await detail.count(), 0);
    }
    const keyboardCell = page.locator('td[title="backup-connection-one · tank"]');
    const keyboardDisclosure = keyboardCell.getByRole('button');
    await keyboardDisclosure.focus();
    await page.keyboard.press('Enter');
    assert.equal(await keyboardDisclosure.getAttribute('aria-expanded'), 'true');
    await page.keyboard.press('Space');
    assert.equal(await keyboardDisclosure.getAttribute('aria-expanded'), 'false');
    assert.ok(requests.some((request) => request.search.includes('resourceId=vm-one')));
    assert.ok(requests.some((request) => request.search.includes('resourceId=vm-two')));
    assert.ok(requests.some((request) => request.search.includes('resourceId=agent-three')));

    console.log(JSON.stringify({ stage: 'shared-controls', engine, theme, width }));
    await page.goto('http://127.0.0.1:5227/browser-tests/table-row-touch.html', {
      waitUntil: 'domcontentloaded',
      timeout: 120_000,
    });
    await page.evaluate(
      (dark) => document.documentElement.classList.toggle('dark', dark),
      theme === 'dark',
    );
    const state = async () =>
      JSON.parse(await page.getByRole('status', { name: 'Action state' }).innerText());
    const name = page.getByText('Shared resource name', { exact: true });
    await name.waitFor();
    await activate(name);
    assert.equal((await state()).rowActions, 1);
    assert.equal((await state()).expanded, true);
    await activate(page.getByRole('button', { name: 'Child action', exact: true }));
    await activate(page.getByRole('link', { name: 'Native link' }));
    assert.equal(new URL(page.url()).hash, '#native-link');
    await activate(page.getByRole('checkbox', { name: 'Child checkbox' }));
    assert.equal(await page.getByRole('checkbox').isChecked(), true);
    assert.equal((await state()).rowActions, 1, 'nested controls must not run row action');
    assert.equal((await state()).childActions, 2);
    const toggle = page.getByTestId('control-row').locator('[data-row-action="true"]');
    await activate(toggle);
    assert.equal((await state()).rowActions, 2, 'nested disclosure must toggle once');
    await toggle.focus();
    await page.keyboard.press('Enter');
    assert.equal((await state()).rowActions, 3);
    await page.keyboard.press('Space');
    assert.equal((await state()).rowActions, 4);
    await activate(page.getByRole('button', { name: 'Toggle row action' }));
    await activate(name);
    assert.equal((await state()).rowActions, 4);
    await activate(page.getByRole('button', { name: 'Toggle row action' }));
    await activate(name);
    assert.equal((await state()).rowActions, 5);
    await activate(page.getByText('Static row', { exact: true }));
    assert.equal((await state()).rowActions, 5);
    await activate(page.getByText('Grid resource', { exact: true }));
    assert.equal((await state()).gridActions, 1);
    await activate(page.getByRole('button', { name: 'Grid child action', exact: true }));
    assert.equal((await state()).gridActions, 1);
    assert.equal((await state()).childActions, 3);
    await noOverflow();
    await snapshot('shared-controls');
    assert.deepEqual(errors, []);
    const result = {
      engine,
      browserVersion: browser.version(),
      playwrightVersion,
      theme,
      width,
      sourceHashes,
      observations,
      requests,
      controlState: await state(),
      errors,
      result: 'passed',
    };
    fs.writeFileSync(path.join(out, 'result.json'), JSON.stringify(result, null, 2) + '\n', {
      mode: 0o644,
    });
    console.log(JSON.stringify(result));
  } catch (error) {
    if (page) await snapshot('failure').catch(() => {});
    fs.writeFileSync(
      path.join(out, 'failure.json'),
      JSON.stringify(
        { error: String(error), errors, requests, observations, sourceHashes },
        null,
        2,
      ) + '\n',
      { mode: 0o644 },
    );
    throw error;
  } finally {
    await browser?.close();
    await server.close();
  }
})().catch((error) => {
  console.error(error);
  process.exitCode = 1;
});
