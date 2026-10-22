// Production Backups page, row producer, canonical adapter, table and CSS.
// Synthetic collected observations only; no PBS, guest or restore operation.
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const http = require('node:http');
const crypto = require('node:crypto');
const { chromium, webkit } = require('playwright');
const root = '/workspace';
const dir = path.join(root, 'tmp/pbs-capacity-details');
const staticRoot = path.join(dir, 'static');
const output = path.join(root, 'tmp/pbs-capacity-consistency/browser');
const serverName = `pbs-main-${'connection-'.repeat(10)}`;
const storeName = `main/${'capacity-'.repeat(14)}<literal-name>`;
const conflict = 'The reported percentage disagrees with the used and total bytes.';
const hash = (file) => crypto.createHash('sha256').update(fs.readFileSync(file)).digest('hex');

(async () => {
  fs.mkdirSync(output, { recursive: true });
  const binding = JSON.parse(fs.readFileSync(path.join(staticRoot, 'binding.json')));
  assert.match(binding.source_sha, /^[a-f0-9]{40}$/);
  for (const [file, expected] of Object.entries(binding.sources))
    assert.equal(
      hash(path.join(root, 'frontend-modern', file)),
      expected,
      `changed source: ${file}`,
    );
  for (const [file, expected] of Object.entries(binding.artifacts))
    assert.equal(hash(path.join(staticRoot, file)), expected, `changed artifact: ${file}`);
  const clientVersion = require('playwright/package.json').version;
  assert.equal(
    clientVersion,
    JSON.parse(fs.readFileSync(path.join(root, 'tests/integration/package-lock.json'))).packages[
      'node_modules/@playwright/test'
    ].version,
  );
  const requests = [];
  const server = http.createServer((req, res) => {
    const url = new URL(req.url, 'http://127.0.0.1');
    if (url.pathname.startsWith('/api/')) {
      requests.push({ method: req.method, path: url.pathname });
      res.setHeader('Content-Type', 'application/json');
      if (req.method !== 'GET') {
        res.statusCode = 405;
        return res.end('{}');
      }
      const data =
        url.pathname === '/api/backups/pve'
          ? { data: { backupTasks: [], storageBackups: [], guestSnapshots: [] } }
          : url.pathname === '/api/backups/pbs'
            ? { data: { backups: [] } }
            : url.pathname.includes('runtime-capabilities')
              ? {
                  capabilities: [],
                  limits: [],
                  max_history_days: 7,
                  hosted_mode: false,
                  runtime: { build: 'community' },
                  blocked_capabilities: [],
                }
              : url.pathname.includes('/metrics-store/history')
                ? {
                    resourceType: url.searchParams.get('resourceType'),
                    resourceId: url.searchParams.get('resourceId'),
                    range: url.searchParams.get('range'),
                    start: 1,
                    end: 2,
                    metrics: {},
                  }
                : { data: [], alerts: [], anomalies: [], enabled: false };
      return res.end(JSON.stringify(data));
    }
    const file = path.join(staticRoot, decodeURIComponent(url.pathname));
    if (
      !file.startsWith(staticRoot + path.sep) ||
      !fs.existsSync(file) ||
      !fs.statSync(file).isFile()
    ) {
      res.statusCode = 404;
      return res.end();
    }
    res.setHeader(
      'Content-Type',
      file.endsWith('.js')
        ? 'text/javascript'
        : file.endsWith('.css')
          ? 'text/css'
          : file.endsWith('.html')
            ? 'text/html'
            : 'application/octet-stream',
    );
    res.end(fs.readFileSync(file));
  });
  await new Promise((resolve) => server.listen(0, '127.0.0.1', resolve));
  const origin = `http://127.0.0.1:${server.address().port}`;
  const report = {
    result: 'failed',
    source_sha: binding.source_sha,
    binding_sha256: hash(path.join(staticRoot, 'binding.json')),
    clientVersion,
    journeys: [],
    captures: [],
    cleanup: { browser_closed: false, server_closed: false },
  };
  let browser;
  let active;
  let failure;
  try {
    for (const [name, engine, width, height] of [
      ['desktop', chromium, 1365, 1000],
      ['narrow', webkit, 320, 1000],
    ]) {
      browser = await engine.launch(
        engine === chromium
          ? { headless: true, channel: 'chromium', args: ['--no-sandbox'] }
          : { headless: true },
      );
      for (const transport of ['direct', 'canonical']) {
        const phone = name === 'narrow';
        const page = await browser.newPage({
          viewport: { width, height },
          isMobile: phone,
          hasTouch: phone,
          locale: 'en-GB',
          timezoneId: 'UTC',
        });
        page.setDefaultTimeout(12000);
        await page.clock.setFixedTime(new Date('2026-10-22T04:00:00Z'));
        const errors = [],
          offOrigin = [],
          checks = [];
        active = {
          name,
          transport,
          width,
          height,
          browserVersion: browser.version(),
          errors,
          offOrigin,
          checks,
        };
        const requestStart = requests.length;
        page.on('pageerror', (error) => errors.push(error.message));
        await page.route('**/*', (route) => {
          if (new URL(route.request().url()).origin !== origin) {
            offOrigin.push(route.request().url());
            return route.abort();
          }
          return route.continue();
        });
        await page.goto(
          `${origin}/browser-tests/pbs-capacity.html?details=1&transport=${transport}`,
          { waitUntil: 'domcontentloaded' },
        );
        if (phone) await page.evaluate(() => document.documentElement.classList.add('dark'));
        const table = page.locator('[data-proxmox-backups-table="servers"]');
        await table.waitFor();
        await page.waitForFunction(
          () => document.fonts.status === 'loaded' && !!window.__pbsCapacityFixture,
        );
        // These are fixture controls, not product chrome. Leave the production page unchanged.
        await page.locator('[aria-label="Synthetic observation controls"]').evaluate((node) => {
          node.style.display = 'none';
        });
        const index = (await table.locator('th').allTextContents()).indexOf('Used');
        assert(index >= 0);
        const cell = table.locator('tbody tr').first().locator('td').nth(index);
        await cell.getByText('40.0%', { exact: true }).waitFor();
        await cell.evaluate((node) => {
          window.__capacityCell = node;
        });
        const update = async (value) =>
          page.evaluate((state) => window.__pbsCapacityFixture.setState(state), value);
        const activate = async (button, key = 'Enter') => {
          if (phone) await button.tap();
          else {
            await button.focus();
            await button.press(key);
          }
        };
        const geometry = async (locator) => {
          const data = await locator.evaluate((node) => {
            const box = node.getBoundingClientRect();
            const ranges = Array.from(node.querySelectorAll('p,dd')).flatMap((element) => {
              const range = document.createRange();
              range.selectNodeContents(element);
              return Array.from(range.getClientRects()).map((r) => ({ x: r.x, right: r.right }));
            });
            return {
              x: box.x,
              right: box.right,
              ranges,
              width: innerWidth,
              scroll: document.documentElement.scrollWidth,
            };
          });
          assert(data.scroll <= data.width + 1, JSON.stringify(data));
          assert(
            data.ranges.every((r) => r.x >= data.x - 1 && r.right <= data.right + 1),
            JSON.stringify(data),
          );
          return data;
        };
        const capture = async (state, locator) => {
          if (transport !== 'canonical') return;
          await locator.scrollIntoViewIfNeeded();
          const file = path.join(output, `${name}-${state}.png`);
          await page.screenshot({ path: file });
          report.captures.push({
            path: path.relative(root, file),
            sha256: hash(file),
            name,
            transport,
            state,
          });
        };
        await update('False empty');
        const warning = cell.getByRole('button', {
          name: `Explain unknown capacity for ${serverName} / ${storeName}`,
          exact: true,
        });
        await warning.waitFor();
        assert.equal(await cell.textContent(), 'Unknown');
        assert.equal(await cell.locator('.bg-emerald-500, .bg-red-500').count(), 0);
        assert(!(await table.textContent()).includes('2.0×'));
        assert.equal(await warning.getAttribute('aria-expanded'), 'false');
        await capture('conflict', table);
        if (!phone) {
          await warning.focus();
          await page.keyboard.press('Tab');
          await page.keyboard.press('Shift+Tab');
          assert(
            await warning.evaluate((node) => document.activeElement === node),
            'native keyboard reachability',
          );
        }
        await activate(warning);
        const region = page.getByRole('region', {
          name: `Datastore capacity for ${serverName} / ${storeName}`,
          exact: true,
        });
        await region.getByText(conflict).waitFor();
        assert((await region.textContent()).includes('This does not mean the datastore is empty.'));
        assert.equal(await warning.getAttribute('aria-expanded'), 'true');
        assert.equal(
          await region.locator('xpath=ancestor::td[1]').getAttribute('id'),
          await warning.getAttribute('aria-controls'),
        );
        assert(!(await region.textContent()).includes('Used:'));
        await region.evaluate((node) => {
          window.__capacityRegion = node;
        });
        checks.push({ state: 'false-empty conflict', geometry: await geometry(region) });
        await capture('conflict-details', region);
        // Same-identity updates preserve the warning's keyboard focus and open context.
        if (!phone) await warning.focus();
        await update('False full');
        await page.waitForFunction(() => window.__capacityCell?.textContent === 'Unknown');
        assert((await region.textContent()).includes(conflict));
        assert(await region.evaluate((node) => window.__capacityRegion === node));
        if (!phone) assert(await warning.evaluate((node) => document.activeElement === node));
        await activate(warning, ' ');
        assert.equal(await warning.getAttribute('aria-expanded'), 'false');
        await activate(warning, ' ');
        assert.equal(await warning.getAttribute('aria-expanded'), 'true');
        // Re-establish identity after the deliberate close/reopen.
        await region.evaluate((node) => {
          window.__capacityRegion = node;
        });
        for (const [state, expected, red] of [
          ['Rounded up', '74.6%', false],
          ['Rounded down', '90.1%', true],
          ['Full', '95.0%', true],
          ['Zero', '0.0%', false],
        ]) {
          await update(state);
          await cell.getByText(expected, { exact: true }).waitFor();
          await region.getByText(`Used: ${expected}`).waitFor();
          assert(!(await region.textContent()).includes(conflict));
          assert.equal((await cell.locator('.text-red-600').count()) > 0, red);
          assert(await cell.evaluate((node) => window.__capacityCell === node));
          assert(await region.evaluate((node) => window.__capacityRegion === node));
          checks.push({ state, expected, geometry: await geometry(region) });
          if (state === 'Full') await capture('full-recovery', region);
        }
        await update('Overflow ratio');
        await cell.getByRole('button', { name: /Explain unknown capacity/ }).waitFor();
        assert(!(await region.textContent()).includes('Used:'));
        await update('Primary offline');
        await cell.getByRole('button', { name: /Explain unavailable capacity/ }).waitFor();
        assert(!(await region.textContent()).includes(conflict));
        await update('Primary warning');
        await cell.getByText('95.0%', { exact: true }).waitFor();
        assert((await region.textContent()).includes('Used: 95.0%'));
        assert(
          !(await page.locator('body').innerHTML()).includes('PRIVATE_PROVIDER_ERROR_SENTINEL'),
        );
        assert.equal(await region.locator('literal-name, img').count(), 0);
        const observed = requests.slice(requestStart);
        assert(observed.every((request) => request.method === 'GET'));
        assert.equal(observed.filter((request) => request.path === '/api/backups/pve').length, 1);
        assert.equal(observed.filter((request) => request.path === '/api/backups/pbs').length, 1);
        assert.equal(errors.length, 0, errors.join('\n'));
        assert.equal(offOrigin.length, 0);
        report.journeys.push({ ...active, requests: observed, palette: phone ? 'dark' : 'light' });
        active = undefined;
        await page.close();
      }
      await browser.close();
      browser = undefined;
    }
    report.result = 'passed';
  } catch (error) {
    report.failure = { message: error.message, active };
    failure = error;
  } finally {
    if (browser) await browser.close();
    report.cleanup.browser_closed = true;
    await new Promise((resolve) => server.close(resolve));
    report.cleanup.server_closed = true;
    fs.writeFileSync(path.join(output, 'result.json'), JSON.stringify(report, null, 2) + '\n');
  }
  if (failure) throw failure;
  console.log(
    JSON.stringify({
      result: report.result,
      contexts: report.journeys.length,
      captures: report.captures.length,
      cleanup: report.cleanup,
    }),
  );
})().catch((error) => {
  console.error(error);
  process.exitCode = 1;
});
