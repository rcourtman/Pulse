// Exact compiled producer/consumer proof; no external or mutating requests.
const fs = require('node:fs');
const path = require('node:path');
const http = require('node:http');
const crypto = require('node:crypto');
const assert = require('node:assert/strict');
const { chromium, webkit } = require('playwright');
const root = '/workspace',
  origin = 'http://127.0.0.1:5335';
const compiled = root + '/tmp/drawer-integration/final-static';
const output = root + '/tmp/drawer-integration/results';
const hash = (b) => crypto.createHash('sha256').update(b).digest('hex');
async function box(locator) {
  return locator.evaluate((element) => {
    const focused = element === document.activeElement,
      focusVisible = element.matches(':focus-visible');
    const rect = element.getBoundingClientRect(),
      css = getComputedStyle(element);
    return {
      class: element.className,
      width: rect.width,
      height: rect.height,
      left: rect.left,
      right: rect.right,
      client: element.clientWidth,
      scroll: element.scrollWidth,
      text: element.textContent,
      focused,
      focusVisible,
      whiteSpace: css.whiteSpace,
      overflowWrap: css.overflowWrap,
      colour: css.color,
      outline: css.outlineStyle,
      outlineWidth: css.outlineWidth,
      outlineOffset: css.outlineOffset,
    };
  });
}
(async () => {
  fs.mkdirSync(output, { recursive: true });
  const binding = JSON.parse(fs.readFileSync(compiled + '/binding.json'));
  const result = {
    result: 'incomplete',
    source: binding.source_sha,
    driver: hash(fs.readFileSync(__filename)),
    playwright: require('playwright/package.json').version,
    binding,
    cases: [],
    captures: [],
    cleanup: { browser: false, server: false },
    limits:
      'Synthetic observation/API data through actual adapter, rows, drawers, history and Docs; no native thaw, monitoring restoration, QGA restart, permissions, deployed or release acceptance.',
  };
  let server, browser;
  try {
    assert.match(result.source, /^[0-9a-f]{40}$/);
    assert.equal(
      result.playwright,
      JSON.parse(fs.readFileSync(root + '/tests/integration/package-lock.json')).packages[
        'node_modules/@playwright/test'
      ].version,
    );
    for (const [file, digest] of Object.entries(binding.sources))
      assert.equal(hash(fs.readFileSync(root + '/frontend-modern/' + file)), digest, file);
    const assets = new Map();
    for (const [file, digest] of Object.entries(binding.artifacts)) {
      const bytes = fs.readFileSync(compiled + '/' + file);
      assert.equal(hash(bytes), digest, file);
      assets.set('/' + file, bytes);
    }
    for (const doc of ['VM_DISK_MONITORING.md', 'TROUBLESHOOTING.md'])
      assert.equal(hash(fs.readFileSync(root + '/docs/' + doc)), binding.artifacts['docs/' + doc]);
    server = http.createServer((request, response) => {
      if (!['GET', 'HEAD'].includes(request.method)) {
        response.writeHead(405);
        return response.end();
      }
      let pathname = new URL(request.url, origin).pathname;
      if (
        ['/', '/guest', '/node', '/docs/VM_DISK_MONITORING', '/docs/TROUBLESHOOTING'].includes(
          pathname,
        )
      )
        pathname = '/browser-tests/drawer-integration.html';
      const bytes = assets.get(pathname);
      if (!bytes) {
        response.writeHead(404);
        return response.end();
      }
      response.setHeader(
        'Content-Type',
        {
          '.html': 'text/html',
          '.js': 'text/javascript',
          '.css': 'text/css',
          '.md': 'text/markdown; charset=utf-8',
        }[path.extname(pathname)] || 'application/octet-stream',
      );
      response.setHeader('Cache-Control', 'no-store');
      response.end(request.method === 'HEAD' ? undefined : bytes);
    });
    await new Promise((resolve, reject) => {
      server.once('error', reject);
      server.listen(5335, '127.0.0.1', resolve);
    });
    for (const [engine, width, theme] of [
      [chromium, 1365, 'light'],
      [chromium, 1365, 'dark'],
      [webkit, 390, 'light'],
      [webkit, 390, 'dark'],
    ]) {
      const name = `${engine === chromium ? 'chromium' : 'webkit'}-${width}-${theme}`;
      browser = await engine.launch(
        engine === chromium
          ? { headless: true, channel: 'chromium', args: ['--no-sandbox'] }
          : { headless: true },
      );
      const context = await browser.newContext({
        viewport: { width, height: 900 },
        isMobile: width < 768,
        hasTouch: width < 768,
        colorScheme: theme,
        locale: 'en-GB',
        reducedMotion: 'reduce',
      });
      await context.addInitScript((theme) => {
        window.__trustedInputs = [];
        document.addEventListener(
          'click',
          (event) =>
            window.__trustedInputs.push({
              trusted: event.isTrusted,
              label: event.target.closest('a,button,summary')?.textContent,
            }),
          true,
        );
        document.addEventListener('DOMContentLoaded', () =>
          document.documentElement.classList.toggle('dark', theme === 'dark'),
        );
      }, theme);
      const page = await context.newPage();
      page.setDefaultTimeout(10000);
      const record = {
        name,
        browser: browser.version(),
        width,
        theme,
        requests: [],
        errors: [],
        states: [],
      };
      result.cases.push(record);
      page.on('pageerror', (error) => record.errors.push(error.message));
      await page.clock.install({ time: new Date('2026-10-13T13:00:00Z') });
      await context.route('**/*', (route) => {
        const request = route.request(),
          url = new URL(request.url());
        record.requests.push({ method: request.method(), path: url.pathname, search: url.search });
        assert.equal(url.origin, origin);
        assert.equal(request.method(), 'GET');
        assert(!/\/(?:diagnostics|run-command|guest-agent|qemu)(?:\/|$)/.test(url.pathname));
        if (!url.pathname.startsWith('/api/')) return route.continue();
        if (url.pathname === '/api/metrics-store/history') {
          const range = url.searchParams.get('range');
          assert(['24h', '7d'].includes(range));
          const end = Date.parse('2026-10-13T13:00:00Z'),
            start = end - (range === '7d' ? 7 : 1) * 86400000;
          const metrics = Object.fromEntries(
            [
              'cpu',
              'memory',
              'disk',
              'temperature',
              'netin',
              'netout',
              'diskread',
              'diskwrite',
            ].map((metric) => [
              metric,
              [
                {
                  timestamp: start,
                  value: /in|out|read|write/.test(metric) ? 1024 ** 2 : 20,
                  min: 10,
                  max: 40,
                },
                {
                  timestamp: end,
                  value: /in|out|read|write/.test(metric) ? 2 * 1024 ** 2 : 30,
                  min: 20,
                  max: 90,
                },
              ],
            ]),
          );
          return route.fulfill({
            json: {
              resourceType: url.searchParams.get('resourceType'),
              resourceId: url.searchParams.get('resourceId'),
              range,
              start,
              end,
              metrics,
              source: 'mock_synthetic',
            },
          });
        }
        if (url.pathname === '/api/license/runtime-capabilities')
          return route.fulfill({
            json: {
              capabilities: [],
              hosted_mode: false,
              max_history_days: 7,
              runtime: { build: 'free' },
              blocked_capabilities: [],
            },
          });
        return route.fulfill({
          json: { success: true, data: [], alerts: [], anomalies: [], config: null },
        });
      });
      const capture = async (suffix, target = page, fullPage = true) => {
        const file = `${name}-${suffix}.png`;
        await target.screenshot({ path: output + '/' + file, fullPage });
        result.captures.push({
          file,
          bytes: fs.statSync(output + '/' + file).size,
          sha256: hash(fs.readFileSync(output + '/' + file)),
        });
      };
      const noOverflow = async () =>
        assert.equal(
          await page.evaluate(
            () => document.documentElement.scrollWidth - document.documentElement.clientWidth,
          ),
          0,
        );
      const activate = async (control) => {
        if (width < 768) await control.tap();
        else {
          await control.focus();
          await page.keyboard.press('Enter');
        }
      };
      const openGuest = async () => {
        await page.goto(origin + '/guest', { waitUntil: 'networkidle' });
        if (width < 768) await page.getByText('integration-guest', { exact: true }).first().tap();
        else {
          const row = page.locator('[data-guest-id="fixture:pve1:101"] [data-row-action]').first();
          await row.focus();
          await page.keyboard.press('Enter');
        }
      };
      await openGuest();
      const overview = page.getByTestId('guest-drawer-overview');
      await overview.waitFor();
      const details = page.getByTestId('guest-technical-details');
      assert.equal(await details.getAttribute('open'), null);
      const os = (await page.evaluate(() => window.__integration.evidence())).os;
      const fact = page.getByTestId('guest-drawer-facts').getByText(os, { exact: true });
      await fact.waitFor();
      record.os = await box(fact);
      assert.equal(record.os.text, os);
      assert(record.os.scroll <= record.os.client);
      assert.equal(record.os.whiteSpace, 'normal');
      assert(record.os.right <= width && record.os.left >= 0);
      await noOverflow();
      await overview
        .getByTestId('drawer-trend-card-memory')
        .locator('[data-testid="drawer-trend-threshold"]')
        .waitFor({ state: 'attached' });
      await overview.getByTestId('drawer-trend-card-memory').locator('svg').waitFor();
      const reading = overview.getByRole('link', { name: 'Filesystem reading guidance' });
      await reading.scrollIntoViewIfNeeded();
      await reading.focus();
      await page.keyboard.press('Shift+Tab');
      await page.keyboard.press('Tab');
      await page.evaluate(async () => {
        await document.fonts.ready;
        await new Promise((resolve) => requestAnimationFrame(() => requestAnimationFrame(resolve)));
      });
      record.readingBeforeCapture = await box(reading);
      assert.equal(
        await overview.getByTestId('drawer-trend-cards').getAttribute('aria-label'),
        'Last 24 hours',
      );
      assert(
        record.requests.some(
          (r) =>
            r.path === '/api/metrics-store/history' &&
            new URLSearchParams(r.search).get('range') === '24h',
        ),
      );
      await activate(details.locator('summary'));
      const completed = details.locator('time[datetime="2026-10-13T12:30:00.000Z"]');
      assert.equal(await completed.textContent(), '2026-10-13 12:30:00 UTC');
      await activate(details.locator('summary'));
      await reading.focus();
      await page.keyboard.press('Shift+Tab');
      await page.keyboard.press('Tab');
      await capture('guest-unknown');
      record.reading = await box(reading);
      assert.equal(record.reading.focusVisible, true);
      assert.equal(record.reading.outline, 'solid');
      assert.equal(record.reading.outlineWidth, '2px');
      assert.equal(record.reading.height, width < 768 ? 44 : 16);
      await activate(reading);
      await page.waitForURL('**/docs/VM_DISK_MONITORING#*');
      await page
        .getByRole('heading', { name: 'A missing reading is not an installation diagnosis' })
        .waitFor();
      record.filesystemDoc = page.url();
      const digest = await page.evaluate(async () => {
        const b = await (await fetch('/docs/VM_DISK_MONITORING.md')).arrayBuffer();
        return Array.from(new Uint8Array(await crypto.subtle.digest('SHA-256', b)))
          .map((x) => x.toString(16).padStart(2, '0'))
          .join('');
      });
      assert.equal(digest, binding.artifacts['docs/VM_DISK_MONITORING.md']);
      await openGuest();
      await page.evaluate(() => {
        window.__integration.backup('2026-10-08T00:30:00Z');
        window.__integration.mode('retained');
      });
      await overview
        .getByTestId('guest-drawer-filesystems')
        .getByText(/Using last known/)
        .waitFor();
      await overview
        .getByTestId('drawer-trend-card-disk')
        .getByText(/Using last known/)
        .waitFor();
      await overview.getByText('/data', { exact: true }).waitFor();
      assert.equal(await details.getAttribute('open'), null);
      const exceptions = overview.getByTestId('guest-drawer-exceptions');
      assert.equal(await exceptions.locator('h3').textContent(), 'Worth checking');
      const retainedCompletion = exceptions.locator('time');
      assert.equal(await retainedCompletion.textContent(), '2026-10-08 00:30:00 UTC');
      assert.equal(await retainedCompletion.getAttribute('datetime'), '2026-10-08T00:30:00.000Z');
      assert.equal(
        await exceptions.locator('time').evaluate((e) => getComputedStyle(e).display),
        'inline',
      );
      await noOverflow();
      await capture('guest-retained');
      record.states.push(
        'Unknown reading and full OS outside closed Details; retained /data and last-known notice preserved',
      );
      await page.evaluate(() => window.__integration.mode('fresh'));
      await overview
        .getByTestId('guest-drawer-filesystems')
        .getByText(/Using last known/)
        .waitFor({ state: 'hidden' });
      await overview
        .getByTestId('drawer-trend-card-disk')
        .getByText(/Using last known/)
        .waitFor({ state: 'hidden' });
      assert.equal(
        await overview.getByRole('link', { name: 'Filesystem reading guidance' }).count(),
        0,
      );
      const memoryCard = overview.getByTestId('drawer-trend-card-memory');
      assert.match(await memoryCard.getAttribute('class'), /amber/);
      await memoryCard
        .locator('[data-testid="drawer-trend-threshold"]')
        .waitFor({ state: 'attached' });
      await overview.getByTestId('drawer-trend-card-memory').locator('svg').waitFor();
      await activate(page.getByRole('tab', { name: 'History', exact: true }));
      await page.getByTestId('guest-history-group-chart').first().waitFor();
      assert.equal(await page.getByTestId('guest-history-range-control').inputValue(), '7d');
      assert(
        record.requests.some(
          (r) =>
            r.path === '/api/metrics-store/history' &&
            new URLSearchParams(r.search).get('range') === '7d',
        ),
      );
      const historyCpu = page.locator('[data-history-group="utilization"]');
      await historyCpu.getByTestId('guest-history-hover-time').waitFor();
      for (const axis of ['100%', '50%', '0%'])
        await historyCpu.getByText(axis, { exact: true }).waitFor();
      assert.equal(await page.getByText('Avg', { exact: true }).count(), 0);
      const plot = historyCpu.getByTestId('guest-history-plot');
      await plot.scrollIntoViewIfNeeded();
      const rect = await plot.boundingBox();
      if (width < 768) {
        // Touch reveals the real native inspection control; it does not hover.
        await plot.tap({ position: { x: rect.width * 0.6, y: rect.height / 2 } });
        const inspect = historyCpu.getByRole('slider', { name: 'Inspect Utilization history' });
        await inspect.tap();
        await inspect.press('Home');
        record.phoneInspection = await inspect.getAttribute('aria-valuetext');
      } else await plot.hover({ position: { x: rect.width * 0.6, y: rect.height / 2 } });
      // Decorative zero-length stroke groups have no geometric bounding box.
      // Verify attachment/path geometry, then review the rendered pixels.
      await page
        .locator('[data-history-hover-marker="cpu"]')
        .first()
        .waitFor({ state: 'attached' });
      assert.equal(
        await page
          .locator('[data-history-hover-marker="cpu"]')
          .first()
          .locator('path')
          .last()
          .getAttribute('vector-effect'),
        'non-scaling-stroke',
      );
      await noOverflow();
      await capture('guest-history');
      await activate(page.getByRole('tab', { name: 'Overview', exact: true }));
      record.states.push(
        'Fresh read withdraws deferral; metric alert trend threshold and on-demand History retained',
      );

      await page.goto(origin + '/node', { waitUntil: 'networkidle' });
      const node = page.getByTestId('node-drawer-overview'),
        nodeDetails = page.getByTestId('node-technical-details');
      const trigger = node.getByRole('button', { name: 'Why update checks are unavailable' });
      await trigger.waitFor();
      assert.equal(await nodeDetails.getAttribute('open'), null);
      await node.getByText(/7 pending.*stale/).waitFor();
      await node.getByText(/Other monitoring can still work/).waitFor();
      const plain = node.getByTestId('drawer-trend-card-memory');
      assert(
        !/amber|red-/.test(await plain.getAttribute('class')),
        'guest alert must not colour node trend',
      );
      await trigger.scrollIntoViewIfNeeded();
      await activate(trigger);
      const help = page.getByRole('dialog', { name: 'Proxmox update-check access' });
      await help.waitFor();
      assert.match(await help.textContent(), /Sys.Modify/);
      assert.match(await help.textContent(), /broader than read-only monitoring/);
      await help.evaluate((element) => {
        window.__retainedHelp = element;
      });
      await trigger.evaluate((element) => {
        window.__retainedTrigger = element;
      });
      await page.evaluate(() => window.__integration.update({ cpu: 0.2 }));
      await page.clock.runFor(60001);
      assert.equal(await help.evaluate((element) => window.__retainedHelp === element), true);
      assert.equal(await trigger.evaluate((element) => window.__retainedTrigger === element), true);
      await node.getByText(/7 pending.*stale.*checked 2 mins ago/).waitFor();
      await page.evaluate(() =>
        window.__integration.update({ pendingUpdatesStatus: 'unavailable' }),
      );
      assert.equal(await node.getByTestId('node-drawer-exceptions').count(), 0);
      assert.equal(await nodeDetails.getAttribute('open'), null);
      assert.equal(
        await node.getByText('Unavailable · Update check access denied', { exact: true }).count(),
        1,
      );
      assert.equal(await help.evaluate((element) => window.__retainedHelp === element), true);
      record.trigger = await box(trigger);
      assert(record.trigger.height >= (width < 768 ? 44 : 32));
      record.help = await box(help);
      assert(record.help.right <= width && record.help.left >= 0);
      await noOverflow();
      await capture('node-help');
      const learnMore = help.getByRole('link', { name: 'Learn more', exact: true });
      assert.equal(
        await learnMore.getAttribute('href'),
        '/docs/TROUBLESHOOTING#proxmox-pending-update-access',
      );
      const [docsPage] = await Promise.all([context.waitForEvent('page'), learnMore.click()]);
      await docsPage.waitForLoadState('networkidle');
      await docsPage
        .getByRole('heading', { name: 'Proxmox pending-update access', exact: true })
        .waitFor();
      const docDigest = await docsPage.evaluate(async () => {
        const b = await (await fetch('/docs/TROUBLESHOOTING.md')).arrayBuffer();
        return Array.from(new Uint8Array(await crypto.subtle.digest('SHA-256', b)))
          .map((x) => x.toString(16).padStart(2, '0'))
          .join('');
      });
      assert.equal(docDigest, binding.artifacts['docs/TROUBLESHOOTING.md']);
      record.updateDoc = { url: docsPage.url(), sha256: docDigest };
      await docsPage.close();
      await page.keyboard.press('Escape');
      await help.waitFor({ state: 'hidden' });
      assert.equal(await trigger.evaluate((e) => e === document.activeElement), true);
      await activate(nodeDetails.locator('summary'));
      assert.notEqual(await nodeDetails.getAttribute('open'), null);
      const denied = nodeDetails.getByText('Unavailable · Update check access denied', {
        exact: true,
      });
      await denied.waitFor();
      record.denied = await box(denied);
      assert.equal(record.denied.whiteSpace, 'normal');
      assert(!/amber|yellow/.test(await denied.locator('..').getAttribute('class')));
      assert(record.denied.scroll <= record.denied.client);
      const fullModel =
        'Fixture CPU with a deliberately long hardware model and complete revision identifier 2026-A-extended';
      const cpuModel = nodeDetails.getByText(fullModel, { exact: true });
      assert.equal((await box(cpuModel)).whiteSpace, 'normal');
      assert((await box(cpuModel)).scroll <= (await box(cpuModel)).client);
      const storage = nodeDetails
        .getByText('/unknown', { exact: true })
        .locator('xpath=ancestor::tr[1]');
      assert.match(await storage.textContent(), /Usage unavailable.*10.0 GB total/);
      assert.match(
        await nodeDetails
          .getByText('/empty', { exact: true })
          .locator('xpath=ancestor::tr[1]')
          .textContent(),
        /0%.*0 B/,
      );
      assert.match(
        await nodeDetails
          .getByText('/measured', { exact: true })
          .locator('xpath=ancestor::tr[1]')
          .textContent(),
        /50%.*5.00 GB/,
      );
      await page.evaluate(() => window.__integration.update({ status: 'offline' }));
      await node
        .getByTestId('drawer-trend-card-disk')
        .getByText('Node is offline. Showing its history only.', { exact: true })
        .waitFor();
      assert.match(
        await nodeDetails
          .getByText('/measured', { exact: true })
          .locator('xpath=ancestor::tr[1]')
          .textContent(),
        /50%.*5.00 GB/,
      );
      await page.evaluate(() => window.__integration.update({ status: 'online' }));
      await noOverflow();
      await capture('node-details');
      await activate(nodeDetails.locator('summary'));
      assert.equal(await nodeDetails.getAttribute('open'), null);
      await page.evaluate(() =>
        window.__integration.update({
          pendingUpdatesStatus: 'unavailable',
          pendingUpdatesReason: 'source_unavailable',
        }),
      );
      await trigger.waitFor({ state: 'hidden' });
      const failed = node.getByTestId('node-drawer-exceptions');
      await failed.getByText('Unavailable · Proxmox source unavailable', { exact: true }).waitFor();
      assert.match(
        await failed
          .getByText('Unavailable · Proxmox source unavailable', { exact: true })
          .getAttribute('class'),
        /amber|yellow/,
      );
      await noOverflow();
      await capture('node-failed');
      await page.evaluate(() =>
        window.__integration.update({
          pendingUpdates: 0,
          pendingUpdatesStatus: 'checked',
          pendingUpdatesReason: undefined,
        }),
      );
      await failed.waitFor({ state: 'hidden' });
      await trigger.waitFor({ state: 'hidden' });
      record.states.push(
        'Denied status neutral in Details with usable guidance outside closed Details; failed check remains a compact warning; full values and unknown/empty/measured disk controls preserved; same dialog/trigger survives readings, age and verdict changes; Escape restores focus; actual success removes denial help',
      );
      record.inputs = await page.evaluate(() => window.__trustedInputs);
      assert(
        record.inputs.some((event) => event.trusted && event.label?.includes('Why update checks')),
      );
      assert.deepEqual(record.errors, []);
      await context.close();
      await browser.close();
      browser = null;
    }
    result.result = 'passed';
    result.verified_at = new Date().toISOString();
  } catch (error) {
    result.error = String(error.stack || error);
    throw error;
  } finally {
    if (browser) await browser.close();
    result.cleanup.browser = true;
    if (server)
      await new Promise((resolve, reject) => {
        server.close((error) => (error ? reject(error) : resolve()));
        server.closeAllConnections();
      });
    result.cleanup.server = true;
    fs.writeFileSync(output + '/result.json', JSON.stringify(result, null, 2) + '\n');
  }
})().catch((error) => {
  console.error(error);
  process.exitCode = 1;
});
