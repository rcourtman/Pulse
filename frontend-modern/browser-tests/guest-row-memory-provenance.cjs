// Production component source in the existing offline Vite preview.
// No installation, full application build, guest operation or external service.
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const crypto = require('node:crypto');

const { chromium, webkit } = require('playwright');
const root = '/workspace/frontend-modern';
const output = '/workspace/tmp/guest-row-memory-browser';
const origin = 'http://127.0.0.1:5332';
const hash = (file) => crypto.createHash('sha256').update(fs.readFileSync(file)).digest('hex');
const manifest = {
  mode: 'source-dev-preview',
  runtime: Object.fromEntries(
    [
      'frontend-modern/src/components/Workloads/GuestRow.tsx',
      'frontend-modern/src/components/Workloads/MetricMiniSparkline.tsx',
      'frontend-modern/src/components/Workloads/guestDrawerModel.ts',
      'frontend-modern/src/utils/memoryObservation.ts',
      'frontend-modern/browser-tests/guest-row-memory-provenance.tsx',
    ].map((file) => [file, hash('/workspace/' + file)]),
  ),
  graph: hash(root + '/package-lock.json'),
};
fs.mkdirSync(output, { recursive: true });
const result = {
  result: 'incomplete',
  playwright: require('playwright/package.json').version,
  binding: manifest,
  cases: [],
  captures: [],
  cleanup: {},
  limits:
    'Production GuestRow, StackedMemoryBar, MetricMiniSparkline, drawer and actual source CSS in a dev preview; synthetic selected memory/history only. This is not a production application build. No full App, native QGA, thaw, filesystem writes, reporter, installed/release or physical-device acceptance.',
};
assert.equal(
  result.playwright,
  JSON.parse(fs.readFileSync('/workspace/tests/integration/package-lock.json')).packages[
    'node_modules/@playwright/test'
  ].version,
);
(async () => {
  const started = Date.now();
  const { createServer } = await import(path.join(root, 'node_modules/vite/dist/node/index.js'));
  const server = await createServer({
    root,
    configFile: path.join(root, 'vite.config.ts'),
    cacheDir: output + '/cache',
    server: { host: '127.0.0.1', port: 5332, strictPort: true },
  });
  await server.listen();
  let browser;
  try {
    for (const [name, engine, width, dark] of [
      ['desktop-light', chromium, 1365, false],
      ['desktop-dark', chromium, 1365, true],
      ['phone-dark', webkit, 390, true],
      ['narrow-light', webkit, 320, false],
    ]) {
      browser = await engine.launch(
        engine === chromium
          ? { headless: true, channel: 'chromium', args: ['--no-sandbox'] }
          : { headless: true },
      );
      const context = await browser.newContext({
        viewport: { width, height: width <= 768 ? 844 : 900 },
        isMobile: width <= 768,
        hasTouch: width <= 768,
        locale: 'en-GB',
        timezoneId: 'UTC',
        reducedMotion: 'reduce',
      });
      const page = await context.newPage();
      page.setDefaultTimeout(7000);
      const record = {
        name,
        width,
        dark,
        browser: browser.version(),
        checks: [],
        requests: [],
        errors: [],
        captures: [],
      };
      result.cases.push(record);
      page.on('pageerror', (e) => record.errors.push(e.message));
      await page.route('**/*', (route) => {
        const req = route.request(),
          url = new URL(req.url());
        if (url.origin !== origin) {
          record.errors.push('Unexpected external request');
          return route.abort();
        }
        if (!url.pathname.startsWith('/api/')) return route.continue();
        record.requests.push({ path: url.pathname, method: req.method() });
        assert.equal(req.method(), 'GET');
        assert(!/diagnostics|\/run|guest-agent|qemu/.test(url.pathname), 'no live guest operation');
        return route.fulfill({
          json:
            url.pathname === '/api/license/runtime-capabilities'
              ? {
                  capabilities: [],
                  limits: [],
                  max_history_days: 7,
                  hosted_mode: false,
                  runtime: { build: 'community' },
                  blocked_capabilities: [],
                }
              : { success: true, data: [], alerts: [], anomalies: [], config: null },
        });
      });
      await page.addInitScript(
        (value) =>
          document.addEventListener('DOMContentLoaded', () =>
            document.documentElement.classList.toggle('dark', value),
          ),
        dark,
      );
      await page.goto(origin + '/browser-tests/guest-row-memory-provenance.html');
      const row = page.locator('[data-guest-id="fixture-pve1-101"]');
      const memory = row.locator('[data-workload-col="memory"]');
      const notice = memory.locator('[data-workload-memory-read-status]');
      const identity = await row.elementHandle();
      const visibleLabel = () => notice.locator('[aria-hidden="true"]');
      const capture = async (state) => {
        const file = `${name}-${state}.png`;
        await page.screenshot({ path: path.join(output, file), fullPage: true });
        record.captures.push(file);
        result.captures.push({ file, sha256: hash(path.join(output, file)) });
      };
      await visibleLabel()
        .getByText(width <= 768 ? 'Prior' : 'Last known', { exact: true })
        .waitFor();
      assert((await notice.locator('.sr-only').textContent()).includes('2026-09-30 11:00:00 UTC'));
      assert((await notice.textContent()).includes('Not a current measurement'));
      assert(
        await visibleLabel().evaluate((el) => {
          const r = el.getBoundingClientRect(),
            cell = el.closest('td').getBoundingClientRect();
          return r.width > 0 && r.x >= cell.x && r.right <= cell.right;
        }),
        'cue fits memory cell',
      );
      await capture('retained-bars');
      record.checks.push(
        'retained cue is visible without hover and exposes original UTC/source to assistive technology',
      );
      await page.evaluate(() =>
        window.__memoryRow.update({
          lastSeen: '2026-10-06T10:00:00Z',
          lock: '',
          backupInProgress: false,
        }),
      );
      assert((await notice.textContent()).includes('Last known'));
      assert(!(await notice.textContent()).includes('2026-10-06'));
      record.checks.push('Last seen/running/backup completion cannot renew memory');
      await page.getByLabel('Metric display').selectOption('sparklines');
      let svg = memory.getByRole('img', {
        name: 'backup-guest memory history, last known 25%',
        exact: true,
      });
      await svg.waitFor();
      const plot = await svg.locator('path').getAttribute('d');
      assert(plot.includes('M'));
      await page.getByLabel('Memory basis').selectOption('host');
      await memory
        .getByRole('img', {
          name: 'backup-guest host memory share history, last known 13%',
          exact: true,
        })
        .waitFor();
      assert((await notice.textContent()).includes('Last known'));
      await page.getByLabel('Memory basis').selectOption('guest');
      record.checks.push(
        'retained sparkline and host-capacity accessible values never become current',
      );
      await page.evaluate(() => {
        const guest = window.__memoryRow.reading();
        window.__memoryRow.update({ memory: { ...guest.memory, observation: undefined } });
      });
      await memory
        .getByRole('img', {
          name: 'backup-guest memory history, freshness unknown 25%',
          exact: true,
        })
        .waitFor();
      await visibleLabel()
        .getByText(width <= 768 ? 'Unknown' : 'Freshness unknown', { exact: true })
        .waitFor();
      assert.equal(await memory.locator('svg[role="img"] path').getAttribute('d'), plot);
      await capture('unknown-sparkline');
      record.checks.push(
        'legacy unannotated Proxmox values are freshness-unknown without changing History',
      );
      await page.evaluate(() => {
        const guest = window.__memoryRow.reading();
        window.__memoryRow.update({
          memory: {
            ...guest.memory,
            observation: {
              state: 'unavailable',
              source: 'guest-agent-meminfo',
              observedAt: '2026-09-30T11:00:00Z',
            },
          },
        });
      });
      await memory
        .getByRole('img', { name: 'backup-guest memory history, unavailable N/A', exact: true })
        .waitFor();
      assert.equal(await memory.locator('svg[role="img"] path').getAttribute('d'), plot);
      await capture('unavailable-sparkline');
      await page.getByLabel('Metric display').selectOption('bars');
      await memory.getByText('N/A', { exact: true }).first().waitFor();
      assert.equal(
        await memory.locator('[data-stacked-memory-segment]').count(),
        0,
        'unavailable numeric carriers have no fill',
      );
      record.checks.push(
        'unavailable carrier hides percentage/bar while retained recorded points stay intact',
      );
      await page.evaluate(() => {
        const guest = window.__memoryRow.reading();
        window.__memoryRow.update({
          diskStatusReason: 'prev-vm-locked',
          memory: {
            ...guest.memory,
            used: 0,
            usage: 0,
            observation: {
              state: 'last-known',
              source: 'status-mem',
              observedAt: '2026-09-30T11:00:00Z',
            },
          },
        });
      });
      await page.getByLabel('Metric display').selectOption('sparklines');
      await memory
        .getByRole('img', { name: 'backup-guest memory history, last known 0%', exact: true })
        .waitFor();
      await page.evaluate(() => {
        const guest = window.__memoryRow.reading();
        window.__memoryRow.update({
          memory: {
            ...guest.memory,
            used: 1024 ** 3,
            usage: 25,
            observation: { state: 'current', source: 'agent', observedAt: '2026-10-01T11:00:00Z' },
          },
        });
      });
      await memory
        .getByRole('img', { name: 'backup-guest memory history, current 25%', exact: true })
        .waitFor();
      assert.equal(await notice.count(), 0);
      assert.equal(await memory.locator('svg[role="img"] path').getAttribute('d'), plot);
      assert(await identity.evaluate((el) => el.isConnected), 'same row remains mounted');
      record.checks.push(
        'measured zero preserved; independent current Agent memory withdraws only its cue while disk deferral persists',
      );
      await page.getByLabel('Metric display').selectOption('bars');
      if (width > 768) {
        await page.getByRole('heading', { name: 'Workloads memory freshness' }).hover();
        await page.locator('body').click({ position: { x: 1, y: 1 } });
        await page.keyboard.press('Tab');
        await page.keyboard.press('Tab');
        await page.keyboard.press('Tab');
        assert(
          await row
            .getByRole('button', { name: /Expand.*details/ })
            .evaluate((el) => document.activeElement === el),
        );
        await page.keyboard.press('Enter');
      } else {
        await row.tap();
      }
      const details = page.getByRole('region', { name: 'Guest details' });
      await details.waitFor();
      assert(
        (await details.textContent()).includes('Current · Pulse Agent · 2026-10-01 11:00:00 UTC'),
      );
      assert(
        (await details.textContent()).includes('does not prove thaw') ||
          (await details.textContent()).includes('independent thaw'),
      );
      record.checks.push(
        'real keyboard/first-touch guest disclosure shares memory policy without clearing backup safety',
      );
      await details.getByRole('button', { name: 'Collapse backup-guest details' }).click();
      await details.waitFor({ state: 'hidden' });
      if (width > 768) {
        await page.evaluate(() => {
          const guest = window.__memoryRow.reading();
          window.__memoryRow.update({
            memory: {
              ...guest.memory,
              observation: {
                state: 'last-known',
                source: 'guest-agent-meminfo',
                observedAt: '2026-09-30T11:00:00Z',
              },
            },
          });
        });
        await row.hover();
        await memory
          .getByRole('img', { name: 'backup-guest memory history, last known 25%', exact: true })
          .waitFor();
        assert((await notice.textContent()).includes('Last known'));
        await page.getByRole('heading', { name: 'Workloads memory freshness' }).hover();
        await memory.getByText('25%', { exact: true }).waitFor();
        record.checks.push('pointer History lens retains provenance and restores the bar');
      }
      assert(
        await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth),
        'no horizontal overflow',
      );
      assert.deepEqual(record.errors, []);
      await context.close();
      await browser.close();
      browser = null;
      record.closed = true;
    }
    result.result = 'passed';
  } catch (error) {
    result.error = error.stack;
    throw error;
  } finally {
    if (browser) await browser.close();
    result.cleanup.browser_closed = true;
    await server.close();
    const esbuild = await import(path.join(root, 'node_modules/esbuild/lib/main.js'));
    esbuild.stop();
    result.cleanup.compiler_stop_requested = true;
    result.cleanup.server_closed = true;
    result.elapsed_ms = Date.now() - started;
    fs.writeFileSync(path.join(output, 'result.json'), JSON.stringify(result, null, 2) + '\n');
    console.log(
      JSON.stringify({
        result: result.result,
        cases: result.cases.length,
        captures: result.captures.length,
        elapsed_ms: result.elapsed_ms,
        cleanup: result.cleanup,
      }),
    );
  }
})().catch((error) => {
  console.error(error.message);
  process.exitCode = 1;
});
