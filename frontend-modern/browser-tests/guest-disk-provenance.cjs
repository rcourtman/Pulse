const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const crypto = require('node:crypto');
const { chromium, webkit } = require('playwright');

const root = '/workspace/frontend-modern';
const output = '/workspace/tmp/guest-disk-provenance';
const origin = 'http://127.0.0.1:5315';
const runtime = [
  'frontend-modern/src/components/Workloads/GuestRow.tsx',
  'frontend-modern/src/components/Workloads/MetricMiniSparkline.tsx',
];
const sha256 = (file) => crypto.createHash('sha256').update(fs.readFileSync(file)).digest('hex');
const settle = (page) =>
  page.evaluate(async () => {
    await document.fonts.ready;
    await new Promise((resolve) => requestAnimationFrame(() => requestAnimationFrame(resolve)));
  });

(async () => {
  fs.mkdirSync(output, { recursive: true });
  const result = {
    playwright: require('playwright/package.json').version,
    runtime_hashes: Object.fromEntries(runtime.map((file) => [file, sha256('/workspace/' + file)])),
    cases: [],
    screenshots: [],
    result: 'incomplete',
    limits:
      'Production row/full drawer/column sizing/CSS with synthetic observations. No physical phone, complete application route, collector, native backup/thaw, installed recovery or release evidence.',
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
    server: { host: '127.0.0.1', port: 5315, strictPort: true },
  });
  let browser;
  try {
    await server.listen();
    for (const [name, engine, width, dark] of [
      ['chromium-desktop', chromium, 1365, false],
      ['chromium-phone', chromium, 390, false],
      ['webkit-phone', webkit, 360, true],
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
      });
      const page = await context.newPage();
      page.setDefaultTimeout(15000);
      page.setDefaultNavigationTimeout(60000);
      const record = {
        name,
        browser: browser.version(),
        width,
        dark,
        errors: [],
        requests: [],
        states: [],
        checks: [],
      };
      result.cases.push(record);
      page.on('pageerror', (error) => record.errors.push(error.message));
      await page.route('**/*', (route) => {
        const request = route.request();
        const url = new URL(request.url());
        if (url.origin !== origin) {
          record.requests.push({
            method: request.method(),
            origin: url.origin,
            path: url.pathname,
            disposition: 'blocked',
          });
          return route.abort();
        }
        if (url.pathname.startsWith('/api/')) {
          record.requests.push({
            method: request.method(),
            origin: url.origin,
            path: url.pathname,
            disposition: 'synthetic',
          });
          return route.fulfill({
            status: 200,
            contentType: 'application/json',
            body: JSON.stringify({
              success: true,
              data: [],
              alerts: [],
              anomalies: [],
              config: null,
            }),
          });
        }
        return route.continue();
      });
      await page.addInitScript((isDark) => {
        document.addEventListener('DOMContentLoaded', () => {
          if (isDark) document.documentElement.classList.add('dark');
        });
        window.__trustedClicks = [];
        document.addEventListener(
          'click',
          (event) =>
            window.__trustedClicks.push({ trusted: event.isTrusted, tag: event.target.tagName }),
          true,
        );
      }, dark);
      await page.goto(`${origin}/browser-tests/guest-disk-provenance.html`);
      await page.waitForFunction(() => window.__guestDiskProvenance);
      await settle(page);
      const row = page.locator('[data-guest-id="fixture-pve:pve-a:101"]');
      const freshRow = page.locator('[data-guest-id="fixture-pve:pve-a:102"]');
      const cell = row.locator('[data-workload-col="disk"]');
      const notice = cell.locator('[data-workload-disk-read-status]');
      const compact = width <= 768;
      await page.evaluate(() => {
        window.__originalGuestRow = document.querySelector(
          '[data-guest-id="fixture-pve:pve-a:101"]',
        );
      });
      const initialHeight = await freshRow.evaluate((el) => el.getBoundingClientRect().height);
      const checkValueFit = async () => {
        const value = cell.locator('[data-testid="metric-mini-sparkline"] > span');
        if (!(await value.count())) return undefined;
        const geometry = await value.evaluate((el) => {
          const rect = el.getBoundingClientRect();
          const cellRect = el.closest('td').getBoundingClientRect();
          const range = document.createRange();
          range.selectNodeContents(el);
          const textRect = range.getBoundingClientRect();
          return {
            value: el.textContent,
            width: rect.width,
            textWidth: textRect.width,
            fits:
              el.scrollWidth <= el.clientWidth + 1 &&
              textRect.left >= cellRect.left &&
              textRect.right <= cellRect.right &&
              textRect.left >= rect.left - 1 &&
              textRect.right <= rect.right + 1,
          };
        });
        assert.ok(geometry.fits, `Inline value clipped: ${JSON.stringify(geometry)}`);
        return geometry;
      };
      const check = async (retained, message) => {
        assert.equal(await notice.count(), 1);
        assert.equal(
          await notice.locator('[aria-hidden="true"]').innerText(),
          retained ? (compact ? 'Prior' : 'Last known') : compact ? 'N/A' : 'Unavailable',
        );
        assert.equal(await notice.locator('.sr-only').textContent(), message);
        const geometry = await notice.locator('[aria-hidden="true"]').evaluate((el) => {
          const rect = el.getBoundingClientRect();
          const cellRect = el.closest('td').getBoundingClientRect();
          return {
            labelWidth: rect.width,
            cellWidth: cellRect.width,
            fits:
              rect.left >= cellRect.left &&
              rect.right <= cellRect.right &&
              rect.top >= cellRect.top &&
              rect.bottom <= cellRect.bottom,
            rowHeight: el.closest('tr').getBoundingClientRect().height,
            colour: getComputedStyle(el).color,
          };
        });
        assert.ok(geometry.fits, JSON.stringify(geometry));
        assert.ok(
          geometry.rowHeight <= initialHeight + 0.5,
          `Dense row grew: ${JSON.stringify(geometry)}, initial ${initialHeight}`,
        );
        assert.ok(
          await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1),
        );
        assert.equal(
          await page.evaluate(
            () =>
              window.__originalGuestRow ===
              document.querySelector('[data-guest-id="fixture-pve:pve-a:101"]'),
          ),
          true,
        );
        assert.equal(await freshRow.locator('[data-workload-disk-read-status]').count(), 0);
        geometry.valueFit = await checkValueFit();
        return geometry;
      };
      const screenshot = async (suffix) => {
        await settle(page);
        const file = `${name}-${suffix}.png`;
        await page.screenshot({ path: path.join(output, file), fullPage: true });
        result.screenshots.push({
          file,
          sha256: sha256(path.join(output, file)),
          name,
          state: suffix,
        });
      };
      await check(
        true,
        'Using last known disk stats. Guest reads paused while Proxmox reports a VM operation lock, such as a backup. Pulse will check again on a later poll.',
      );
      await screenshot('retained-bars');
      const deferrals = await page.evaluate(() => window.__guestDiskProvenance.deferrals);
      for (const mode of ['bars', 'sparklines']) {
        await page.evaluate((mode) => window.__guestDiskProvenance.mode(mode), mode);
        for (const retained of [true, false]) {
          for (const [reason, message] of deferrals) {
            await page.evaluate(
              ({ retained, reason }) =>
                window.__guestDiskProvenance.apply({
                  reason: retained ? `prev-${reason}` : reason,
                  data: retained,
                  usage: 50,
                  kind: 'qemu',
                }),
              { retained, reason },
            );
            await settle(page);
            const fullMessage = retained ? `Using last known disk stats. ${message}` : message;
            const geometry = await check(retained, fullMessage);
            if (mode === 'sparklines' && retained)
              assert.equal(
                await cell
                  .getByRole('img', { name: 'backup-guest disk usage history, last known 50%' })
                  .count(),
                1,
              );
            record.states.push({ mode, retained, reason, geometry });
          }
        }
      }
      await screenshot('unavailable-sparkline');
      await page.evaluate(() => {
        window.__guestDiskProvenance.mode('bars');
        window.__guestDiskProvenance.apply({
          reason: 'prev-agent-busy',
          data: true,
          lock: '',
          kind: 'qemu',
        });
      });
      await settle(page);
      await check(
        true,
        'Using last known disk stats. Guest reads deferred while an earlier guest request is still in progress. Pulse will check again on a later poll.',
      );
      record.checks.push('Lock clearance alone does not withdraw retained-read provenance.');
      const disclosure = row.locator('[data-row-action]');
      if (width > 768) {
        await row.hover();
        assert.equal(
          await cell
            .getByRole('img', { name: 'backup-guest disk usage history, last known 50%' })
            .count(),
          1,
        );
        await check(
          true,
          'Using last known disk stats. Guest reads deferred while an earlier guest request is still in progress. Pulse will check again on a later poll.',
        );
        await page.locator('h1').hover();
        await disclosure.focus();
        assert.equal(
          await cell
            .getByRole('img', { name: 'backup-guest disk usage history, last known 50%' })
            .count(),
          1,
        );
        await page.keyboard.press('Enter');
        record.checks.push(
          'Mouse and keyboard history lenses retain the visible notice and truthful accessible numeric label.',
        );
      } else {
        await notice.locator('[aria-hidden="true"]').tap();
        assert.equal(await row.getAttribute('data-history-lens-active'), null);
        record.checks.push(
          'Trusted touch on the noninteractive provenance cue opens the existing drawer once without a hover lens or new tab stop.',
        );
      }
      const drawer = page.getByRole('region', { name: 'Guest details' });
      assert.ok(await drawer.isVisible());
      assert.equal(await page.evaluate(() => window.__guestDiskProvenance.actionCount()), 1);
      assert.ok(
        await drawer
          .getByText(
            'Using last known disk stats. Guest reads deferred while an earlier guest request is still in progress. Pulse will check again on a later poll.',
            { exact: true },
          )
          .isVisible(),
      );
      await screenshot('drawer-reason');
      await drawer.getByRole('button', { name: 'Collapse backup-guest details' }).click();
      await page.evaluate(() =>
        window.__guestDiskProvenance.apply({ reason: '', data: true, usage: 75, lock: '' }),
      );
      await page.evaluate(() => window.__guestDiskProvenance.mode('sparklines'));
      await settle(page);
      assert.equal(await notice.count(), 0);
      assert.equal(await cell.getAttribute('title'), null);
      assert.equal(
        await cell
          .getByRole('img', { name: 'backup-guest disk usage history, current 75%' })
          .count(),
        1,
      );
      assert.equal(
        await page.evaluate(
          () =>
            window.__originalGuestRow ===
            document.querySelector('[data-guest-id="fixture-pve:pve-a:101"]'),
        ),
        true,
      );
      const freshValueFit = await checkValueFit();
      assert.equal(freshValueFit.value, '75%');
      record.freshValueFit = freshValueFit;
      await screenshot('fresh-resumption');
      record.checks.push(
        'A new same-identity fresh observation withdraws the warning and updates the value and chart name without remount.',
      );
      await page.evaluate(() =>
        window.__guestDiskProvenance.apply({ reason: 'prev-vm-locked', kind: 'lxc' }),
      );
      await settle(page);
      assert.equal(await notice.count(), 0);
      assert.equal(await cell.getAttribute('title'), null);
      record.checks.push(
        'A CT does not inherit VM-only guest-agent guidance; fresh linked data stays unmarked.',
      );
      assert.equal(await row.locator('[data-workload-disk-read-status][tabindex]').count(), 0);
      assert.ok(
        record.requests.every((request) => request.method === 'GET' && request.origin === origin),
      );
      assert.deepEqual(record.errors, []);
      record.trustedClicks = await page.evaluate(() => window.__trustedClicks);
      assert.ok(record.trustedClicks.some((event) => event.trusted));
      record.columnIds = await page.evaluate(() => window.__guestDiskProvenance.columnIds);
      record.layout = await page.evaluate(() => window.__guestDiskProvenance.layout);
      record.initialRowHeight = initialHeight;
      await context.close();
      await browser.close();
      browser = undefined;
    }
    result.result = 'passed';
  } catch (error) {
    result.error = error.stack || String(error);
    throw error;
  } finally {
    if (browser) await browser.close();
    await server.close();
    fs.writeFileSync(path.join(output, 'result.json'), JSON.stringify(result, null, 2) + '\n');
    console.log(
      JSON.stringify({
        result: result.result,
        cases: result.cases.length,
        stateChecks: result.cases.reduce((count, item) => count + item.states.length, 0),
        screenshots: result.screenshots.length,
        output,
      }),
    );
  }
})().catch((error) => {
  console.error(error);
  process.exitCode = 1;
});
