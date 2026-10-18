// Fresh combined-content proof, not native Podman or alert-delivery acceptance.
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const crypto = require('node:crypto');
const { chromium, webkit } = require('playwright');
const root = '/workspace/frontend-modern';
const output = '/workspace/tmp/docker-composed-20261005-controls';
const origin = 'http://127.0.0.1:5341';
const runtimePath = 'frontend-modern/src/features/docker/DockerContainersTable.tsx';
const expectedHash = '85753788c2e982dcedbab9aed99ff74a1f1623ec35bc7886ab25661e86de2dd9';
const hash = (file) => crypto.createHash('sha256').update(fs.readFileSync(file)).digest('hex');

(async () => {
  fs.mkdirSync(output, { recursive: true });
  assert.equal(hash('/workspace/' + runtimePath), expectedHash);
  const result = {
    result: 'incomplete',
    playwright: require('playwright/package.json').version,
    content_sha256: { [runtimePath]: expectedHash },
    cases: [],
    screenshots: [],
    limits:
      'Production table, drawer and CSS with selected synthetic resources/APIs. Emulated phone, not a native phone. No collector, engine, threshold duration, notification, installed or release acceptance.',
  };
  assert.equal(
    result.playwright,
    JSON.parse(fs.readFileSync('/workspace/tests/integration/package-lock.json')).packages[
      'node_modules/@playwright/test'
    ].version,
  );
  process.chdir(root);
  const { createServer } = await import(path.join(root, 'node_modules/vite/dist/node/index.js'));
  const server = await createServer({
    root,
    configFile: path.join(root, 'vite.config.ts'),
    cacheDir: output + '/cache',
    optimizeDeps: { noDiscovery: true, include: [] },
    server: { host: '127.0.0.1', port: 5341, strictPort: true, watch: null },
  });
  let browser;
  try {
    await server.listen();
    for (const [name, engine, width, dark] of [
      ['desktop-light', chromium, 1440, false],
      ['desktop-dark', chromium, 1440, true],
      ['phone-light', webkit, 390, false],
      ['phone-dark', webkit, 390, true],
    ]) {
      browser = await engine.launch(
        engine === chromium
          ? { headless: true, channel: 'chromium', args: ['--no-sandbox'] }
          : { headless: true },
      );
      const page = await browser.newPage({
        viewport: { width, height: width < 768 ? 844 : 900 },
        isMobile: width < 768,
        hasTouch: width < 768,
        locale: 'en-GB',
        timezoneId: 'UTC',
      });
      page.setDefaultTimeout(12000);
      page.setDefaultNavigationTimeout(60000);
      const record = {
        name,
        width,
        dark,
        browser: browser.version(),
        errors: [],
        requests: [],
        checks: [],
      };
      result.cases.push(record);
      page.on('pageerror', (error) => record.errors.push(error.message));
      page.on('console', (message) => {
        if (message.type() === 'error') record.errors.push(message.text());
      });
      await page.routeWebSocket('**/*', (socket) => {
        if (socket.url().startsWith('ws://127.0.0.1:5341/')) socket.connectToServer();
        else {
          record.requests.push({ blocked: true, socket: socket.url() });
          socket.close();
        }
      });
      await page.route('**/*', (route) => {
        const request = route.request(),
          url = new URL(request.url());
        if (url.origin !== origin || request.method() !== 'GET') {
          record.requests.push({ blocked: true, url: request.url(), method: request.method() });
          return route.abort();
        }
        if (!url.pathname.startsWith('/api/')) return route.continue();
        record.requests.push({ path: url.pathname, method: request.method() });
        return route.fulfill({
          json: {
            success: true,
            data: [],
            alerts: [],
            anomalies: [],
            config: null,
            capabilities: [],
            relationships: [],
            recentChanges: [],
            available: false,
            audits: [],
            count: 0,
            blocked_capabilities: [],
            limits: [],
            max_history_days: 90,
          },
        });
      });
      await page.addInitScript((dark) => {
        const apply = () => document.documentElement.classList.toggle('dark', dark);
        if (document.documentElement) apply();
        else document.addEventListener('DOMContentLoaded', apply, { once: true });
        window.__inputs = [];
        for (const type of ['click', 'keydown'])
          document.addEventListener(
            type,
            (event) => window.__inputs.push({ type, trusted: event.isTrusted }),
            true,
          );
      }, dark);
      await page.goto(origin + '/browser-tests/docker-composed.html', { waitUntil: 'networkidle' });
      const activate = async (locator) => {
        if (width < 768) await locator.tap();
        else {
          await locator.focus();
          await page.keyboard.press('Enter');
        }
      };
      const table = page.getByRole('table');
      const cpuCell = async (id) => {
        const index = await table
          .locator('thead th')
          .evaluateAll((headers) =>
            headers.findIndex((header) => header.textContent.trim() === 'CPU'),
          );
        assert.ok(index >= 0, JSON.stringify(await table.locator('thead th').allTextContents()));
        return page.locator(`[data-docker-container-row="${id}"]`).getByRole('cell').nth(index);
      };
      const labels = async () => {
        for (const [id, text] of [
          ['small-positive', '0.3%'],
          ['measured-zero', '0%'],
          ['tiny-positive', '<0.1%'],
        ]) {
          const cell = await cpuCell(id);
          await cell.getByText(text, { exact: true }).waitFor();
        }
      };
      const screenshot = async (state) => {
        await page.evaluate(async () => {
          await document.fonts.ready;
          await new Promise((resolve) =>
            requestAnimationFrame(() => requestAnimationFrame(resolve)),
          );
        });
        assert.ok(
          await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1),
          'No document overflow',
        );
        const file = `${name}-${state}.png`;
        await page.screenshot({ path: path.join(output, file), fullPage: true });
        result.screenshots.push({ file, name, state, sha256: hash(path.join(output, file)) });
      };
      await page
        .locator('[data-docker-host-group="edge-a"] [data-docker-host-group-stale]')
        .waitFor();
      assert.equal(
        await page.locator('[data-docker-host-group-stale]').innerText(),
        'No report for 9m',
      );
      assert.match(
        await page.locator('[data-docker-host-group-stale]').getAttribute('title'),
        /last values received/,
      );
      assert.equal(
        await table
          .locator('thead th')
          .filter({ hasText: /^Host$/ })
          .count(),
        0,
      );
      assert.equal(
        await table
          .locator('thead th')
          .filter({ hasText: /^Engine$/ })
          .count(),
        0,
      );
      assert.match(
        await page.locator('[data-docker-host-group="edge-a"]').innerText(),
        /docker fixture/,
      );
      assert.match(
        await page.locator('[data-docker-host-group="edge-b"]').innerText(),
        /podman fixture/,
      );
      assert.equal(
        await page
          .locator('[data-docker-host-group="edge-b"] [data-docker-host-group-stale]')
          .count(),
        0,
      );
      await labels();
      const guidance = page.getByTestId('container-table-cpu-scale');
      assert.match(await guidance.innerText(), /100% means all host CPUs.*per-core scale/);
      await screenshot('grouped-with-hosts');
      record.checks.push(
        'Both fractional labels and shared host/engine headings survive composition; only the silent host is qualified.',
      );
      await activate(page.getByRole('button', { name: 'Remove host context', exact: true }));
      assert.equal(await page.locator('[data-docker-host-group-stale]').count(), 0);
      assert.match(
        await page.locator('[data-docker-host-group="edge-a"]').innerText(),
        /docker fixture/,
      );
      await labels();
      await screenshot('grouped-without-hosts');
      record.checks.push(
        'Missing host context removes stale/status claims, keeps container engine fallback, fractions and scale guidance.',
      );
      await activate(page.getByRole('button', { name: 'Restore host context', exact: true }));
      if (width < 768) await activate(page.getByRole('button', { name: 'Filters', exact: true }));
      await activate(page.getByRole('button', { name: 'View', exact: true }));
      await activate(page.getByRole('button', { name: 'List', exact: true }));
      await page.keyboard.press('Escape');
      assert.equal(await page.locator('[data-docker-host-group]').count(), 0);
      if (width >= 1024) {
        assert.equal(
          await table
            .locator('thead th')
            .filter({ hasText: /^Host$/ })
            .count(),
          1,
        );
        assert.equal(
          await table
            .locator('thead th')
            .filter({ hasText: /^Engine$/ })
            .count(),
          1,
        );
      }
      await labels();
      const containerRow = page.locator('[data-docker-container-row="small-positive"]');
      // On phones the production disclosure button is intentionally hidden;
      // use its existing whole-row pointer convenience, not a forced click.
      if (width < 768) await containerRow.tap();
      else await activate(containerRow.getByRole('button').first());
      const detail = page.locator('[data-inline-platform-resource-detail-for="small-positive"]');
      await detail.waitFor();
      assert.match(
        await detail.getByTestId('container-drawer-cpu-scale').innerText(),
        /total host capacity/,
      );
      await screenshot('flat-expanded');
      await activate(
        detail.getByRole('button', { name: 'Collapse small-positive details', exact: true }),
      );
      await detail.waitFor({ state: 'detached' });
      record.checks.push(
        'Flat layout retains CPU labels; trusted expansion/collapse shows the same capacity guidance in the real drawer.',
      );
      assert.deepEqual(record.errors, []);
      assert.ok(!record.requests.some((request) => request.blocked));
      record.trustedInputs = await page.evaluate(() => window.__inputs);
      assert.ok(record.trustedInputs.some((event) => event.trusted));
      await browser.close();
      browser = undefined;
    }
    result.result = 'passed';
    result.verified_at = new Date().toISOString();
  } catch (error) {
    result.error = String(error.stack || error);
    const failedPage = browser?.contexts()[0]?.pages()[0];
    if (failedPage) {
      result.failure_headers = await failedPage.locator('thead th').allTextContents();
      result.failure_body = (await failedPage.locator('body').innerText()).slice(0, 5000);
      await failedPage.screenshot({ path: output + '/failure.png', fullPage: true });
    }
    throw error;
  } finally {
    if (browser) await browser.close();
    await server.close();
    result.browser_closed = true;
    result.server_closed = true;
    fs.writeFileSync(output + '/result.json', JSON.stringify(result, null, 2) + '\n');
  }
  process.stdout.write(
    JSON.stringify({
      result: result.result,
      states: result.cases.length,
      screenshots: result.screenshots.length,
      resultPath: output + '/result.json',
    }) + '\n',
  );
})().catch((error) => {
  process.stderr.write(String(error.stack || error) + '\n');
  process.exitCode = 1;
});
