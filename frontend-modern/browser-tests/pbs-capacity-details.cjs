// Existing Backups page and drawer with synthetic resource observations only.
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const http = require('node:http');
const crypto = require('node:crypto');
const { chromium, webkit } = require('playwright');
const root = '/workspace';
const dir = path.join(root, 'tmp/pbs-capacity-details');
const staticRoot = path.join(dir, 'static');
const output = path.join(dir, 'browser');
const serverName = `pbs-main-${'connection-'.repeat(10)}`;
const storeName = `main/${'capacity-'.repeat(14)}<literal-name>`;
const escapeRegex = (value) => value.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
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
  const apiRequests = [];
  const server = http.createServer((req, res) => {
    const url = new URL(req.url, 'http://127.0.0.1');
    if (url.pathname.startsWith('/api/')) {
      apiRequests.push({ method: req.method, path: url.pathname });
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
  const journeys = [],
    captures = [];
  let browser;
  let passed = false;
  let incompleteContext;
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
        await page.clock.setFixedTime(new Date('2026-10-04T12:00:00Z'));
        const errors = [],
          offOrigin = [],
          checks = [];
        incompleteContext = { name, transport, width, height, errors, offOrigin, checks };
        const requestStart = apiRequests.length;
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
        const update = async (state) =>
          page.evaluate((value) => window.__pbsCapacityFixture.setState(value), state);
        const activate = async (button, key = 'Enter') => {
          if (phone) await button.tap();
          else {
            await button.focus();
            await button.press(key);
          }
        };
        const capture = async (state, locator) => {
          if (transport !== 'canonical') return;
          await locator.scrollIntoViewIfNeeded();
          const file = path.join(output, `${name}-${state}.png`);
          await page.screenshot({ path: file });
          captures.push({
            path: path.relative(root, file),
            sha256: hash(file),
            state,
            name,
            transport,
          });
        };
        const checkGeometry = async (region) => {
          const geometry = await region.evaluate((node) => {
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
          assert(geometry.scroll <= geometry.width + 1, JSON.stringify(geometry));
          assert(
            geometry.ranges.every((r) => r.x >= geometry.x - 1 && r.right <= geometry.right + 1),
            JSON.stringify(geometry),
          );
          return geometry;
        };
        await activate(page.getByRole('button', { name: 'Failure', exact: true }));
        const warning = table.getByRole('button', {
          name: new RegExp(
            `^Explain (?:unavailable|unknown) capacity for ${escapeRegex(serverName)} / ${escapeRegex(storeName)}$`,
          ),
          exact: true,
        });
        await warning.waitFor();
        assert.equal(await warning.getAttribute('aria-expanded'), 'false');
        await capture('warning', warning);
        if (!phone) {
          await warning.focus();
          await page.keyboard.press('Tab');
          await page.keyboard.press('Shift+Tab');
          assert(
            await warning.evaluate((node) => document.activeElement === node),
            'native Tab reachability',
          );
        }
        await activate(warning);
        assert.equal(
          await warning.getAttribute('aria-expanded'),
          'true',
          'one activation must open the disclosure',
        );
        let region = page.getByRole('region', {
          name: `Datastore capacity for ${serverName} / ${storeName}`,
          exact: true,
        });
        await region.waitFor();
        const controlled = await warning.getAttribute('aria-controls');
        assert.equal(await region.locator('xpath=ancestor::td[1]').getAttribute('id'), controlled);
        assert(
          (await region.textContent()).includes(
            'Datastore capacity is unavailable. This does not mean the datastore is empty.',
          ),
        );
        assert(
          (await region.textContent()).includes(
            'Server reachability does not confirm a successful datastore capacity read.',
          ),
        );
        assert((await region.textContent()).includes(serverName));
        assert((await region.textContent()).includes(storeName));
        await region.evaluate((node) => {
          window.__capacityRegionWitness = node;
        });
        checks.push({ state: 'unavailable', geometry: await checkGeometry(region) });
        await capture('unavailable-details', region);
        if (!phone) await warning.focus();
        await update('Unknown');
        await region.getByText(/Datastore capacity is unknown/).waitFor();
        assert(await region.evaluate((node) => window.__capacityRegionWitness === node));
        assert.equal(await warning.getAttribute('aria-expanded'), 'true');
        if (!phone) assert(await warning.evaluate((node) => document.activeElement === node));
        await update('Error');
        await region.getByText(/Datastore capacity is unavailable/).waitFor();
        assert(
          !(await page.locator('body').innerHTML()).includes('PRIVATE_PROVIDER_ERROR_SENTINEL'),
        );
        assert.equal(
          await region.locator('literal-name, img').count(),
          0,
          'provider names must be text',
        );
        await activate(warning, ' ');
        assert.equal(await warning.getAttribute('aria-expanded'), 'false');
        await activate(warning, ' ');
        assert.equal(await warning.getAttribute('aria-expanded'), 'true');
        const second = table.getByRole('button', {
          name: `Explain unknown capacity for ${serverName} / secondary`,
          exact: true,
        });
        await activate(second);
        const secondRegion = page.getByRole('region', {
          name: `Datastore capacity for ${serverName} / secondary`,
          exact: true,
        });
        await secondRegion.getByText(/Datastore capacity is unknown/).waitFor();
        assert.equal(await region.count(), 0, 'previous store context must be removed');
        assert.equal(await warning.getAttribute('aria-expanded'), 'false');
        checks.push({ state: 'other datastore', geometry: await checkGeometry(secondRegion) });
        await activate(warning);
        region = page.getByRole('region', {
          name: `Datastore capacity for ${serverName} / ${storeName}`,
          exact: true,
        });
        await region.waitFor();
        await region.evaluate((node) => {
          window.__capacityRegionWitness = node;
        });
        await update('Primary offline');
        const firstRow = table.locator('tbody > tr').first();
        const status = firstRow.locator('td').nth(1);
        await status.getByText('Offline', { exact: true }).waitFor();
        await region.getByText(/Datastore capacity is unavailable/).waitFor();
        assert(!(await firstRow.textContent()).includes('%'));
        assert(!(await firstRow.textContent()).includes('2.0×'));
        assert(!(await region.textContent()).includes('Used:'));
        assert(await region.evaluate((node) => window.__capacityRegionWitness === node));
        assert.equal(await warning.getAttribute('aria-expanded'), 'true');
        checks.push({
          state: 'primary offline / retained healthy facet',
          geometry: await checkGeometry(region),
        });
        await capture('primary-offline', region);
        await update('Primary warning');
        await status.getByText('Healthy', { exact: true }).waitFor();
        await region.getByText(/Used: 95\.0%/).waitFor();
        checks.push({
          state: 'reachable primary storage warning',
          geometry: await checkGeometry(region),
        });
        await update('Offline');
        await status.getByText('Offline', { exact: true }).waitFor();
        await region.getByText(/Datastore capacity is unavailable/).waitFor();
        await update('Zero');
        await region.getByText(/Used: 0\.0%/).waitFor();
        assert.equal(await warning.count(), 0);
        assert(await region.evaluate((node) => window.__capacityRegionWitness === node));
        assert(!(await region.textContent()).includes('does not mean the datastore is empty'));
        await update('Full');
        await region.getByText(/Used: 95\.0%/).waitFor();
        checks.push({
          state: 'measured zero / full replacement',
          geometry: await checkGeometry(region),
        });
        await capture('measured-details', region);
        const detailCell = region.locator('xpath=ancestor::td[1]');
        assert.equal(
          Number(await detailCell.getAttribute('colspan')),
          await table.locator('thead > tr > th').count(),
        );
        const close = page.getByRole('button', {
          name: `Collapse ${serverName} details`,
          exact: true,
        });
        // Exercise focus restoration from a focused close control even on
        // touch: a tap alone need not give buttons focus in WebKit.
        await close.focus();
        await activate(close);
        await region.waitFor({ state: 'detached' });
        const summaryToggle = table
          .getByRole('button', { name: `Expand details for ${serverName}`, exact: true })
          .first();
        await page.waitForFunction(
          (id) => document.activeElement?.getAttribute('aria-controls') === id,
          controlled,
        );
        assert(await summaryToggle.evaluate((node) => document.activeElement === node));
        const focus = await summaryToggle.evaluate((node) => {
          const b = node.getBoundingClientRect();
          const style = getComputedStyle(node);
          return {
            width: b.width,
            height: b.height,
            display: style.display,
            visibility: style.visibility,
            clip: style.clip,
            clipPath: style.clipPath,
            opacity: style.opacity,
            shadow: style.boxShadow,
          };
        });
        checks.push({ state: 'close focus returned', focus });
        // The existing chevron reveals its glyph on focus. Verify visibility,
        // not an invented minimum size for that glyph; the row is tappable.
        assert(
          focus.width > 1 &&
            focus.height > 1 &&
            focus.display !== 'none' &&
            focus.visibility === 'visible' &&
            focus.clip === 'auto' &&
            focus.clipPath === 'none' &&
            Number(focus.opacity) > 0,
          'focus-restored disclosure must be visible',
        );
        if (!phone) assert(focus.shadow.includes('2px'), 'desktop keyboard focus ring');
        await capture('focus-return', summaryToggle);
        await activate(summaryToggle);
        await region.getByText(/Used: 95\.0%/).waitFor();
        await update('No stores');
        await table.getByText('No datastore data').waitFor();
        assert.equal(await region.count(), 0);
        assert.equal(
          await table
            .getByRole('button', { name: /^Explain (?:unavailable|unknown) capacity/ })
            .count(),
          0,
        );
        assert.deepEqual(errors, []);
        assert.deepEqual(offOrigin, []);
        const requests = apiRequests.slice(requestStart);
        assert(requests.some((r) => r.path === '/api/backups/pve'));
        assert(requests.some((r) => r.path === '/api/backups/pbs'));
        assert(requests.every((r) => r.method === 'GET'));
        journeys.push({
          name,
          transport,
          width,
          height,
          browserVersion: browser.version(),
          checks,
          closeFocus: focus,
          requests,
          errors,
          offOrigin,
        });
        await page.close();
      }
      await browser.close();
      browser = undefined;
    }
    passed = true;
  } finally {
    if (browser) await browser.close();
    await new Promise((resolve) => server.close(resolve));
    fs.writeFileSync(
      path.join(output, 'result.json'),
      JSON.stringify(
        {
          result: passed ? 'passed' : 'incomplete',
          source_sha: binding.source_sha,
          binding_sha256: hash(path.join(staticRoot, 'binding.json')),
          clientVersion,
          journeys,
          incompleteContext: passed ? undefined : incompleteContext,
          captures,
          cleanup: { browserClosed: true, serverClosed: true },
        },
        null,
        2,
      ) + '\n',
    );
  }
  console.log(
    JSON.stringify({
      result: 'passed',
      journeys: journeys.length,
      captures: captures.length,
      cleanup: 'complete',
    }),
  );
})().catch((error) => {
  console.error(error.stack);
  process.exitCode = 1;
});
