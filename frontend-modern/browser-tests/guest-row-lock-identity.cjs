// Precompiled production GuestRow/CSS/drawer fixture, not a live guest or full App.
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const crypto = require('node:crypto');
const { chromium, webkit } = require('playwright');
const hash = (file) => crypto.createHash('sha256').update(fs.readFileSync(file)).digest('hex');
const root = '/workspace/frontend-modern';
const output = '/workspace/tmp/guest-row-lock-identity';
const compiled = '/workspace/tmp/guest-row-lock-static';
const http = require('node:http');
const origin = 'http://127.0.0.1:5334';
fs.mkdirSync(output, { recursive: true });
const result = {
  result: 'incomplete',
  playwright: require('playwright/package.json').version,
  binding: {
    builder: hash(root + '/browser-tests/build-guest-row-lock-fixture.mjs'),
    runtime: hash(root + '/src/components/Workloads/GuestRow.tsx'),
    stylesheet: hash(root + '/src/index.css'),
    fixture: hash(root + '/browser-tests/guest-row-memory-provenance.tsx'),
    script: hash(root + '/browser-tests/guest-row-lock-identity.cjs'),
    graph: hash(root + '/package-lock.json'),
  },
  cases: [],
  captures: [],
  cleanup: {},
  limits:
    'Production GuestRow, source stylesheet, actual drawer and existing synthetic memory/History fixture in a precompiled static bundle. No full App or production-build browser claim, native QGA, thaw/writes/liveness/restoration, installed/release/reporter or physical-device acceptance.',
};
assert.equal(
  result.playwright,
  JSON.parse(fs.readFileSync('/workspace/tests/integration/package-lock.json')).packages[
    'node_modules/@playwright/test'
  ].version,
);
(async () => {
  const bindings = {};
  for (const variant of ['parent', 'final']) {
    const dir = path.join(compiled, variant);
    const binding = JSON.parse(fs.readFileSync(path.join(dir, 'binding.json')));
    assert.equal(binding.graph, hash(root + '/package-lock.json'));
    assert.equal(binding.builder, hash(root + '/browser-tests/build-guest-row-lock-fixture.mjs'));
    for (const [file, digest] of Object.entries(binding.artifacts))
      assert.equal(hash(path.join(dir, file)), digest, 'served artifact matches compiler output');
    for (const [file, digest] of Object.entries(binding.sources))
      assert.equal(
        hash(
          variant === 'parent' && file === 'src/components/Workloads/GuestRow.tsx'
            ? path.join(compiled, 'parent-row.tsx')
            : path.join(root, file),
        ),
        digest,
        'every imported source byte matches its binding',
      );
    bindings[variant] = binding;
  }
  result.compiled = bindings;
  const server = http.createServer((req, res) => {
    const pathname = new URL(req.url, origin).pathname;
    const file = path.resolve(compiled, '.' + pathname);
    if (!file.startsWith(compiled + '/') || !fs.existsSync(file) || !fs.statSync(file).isFile()) {
      res.writeHead(404);
      res.end();
      return;
    }
    const ext = path.extname(file);
    res.setHeader(
      'Content-Type',
      ext === '.html'
        ? 'text/html'
        : ext === '.js'
          ? 'application/javascript'
          : ext === '.css'
            ? 'text/css'
            : 'application/octet-stream',
    );
    res.end(fs.readFileSync(file));
  });
  await new Promise((resolve, reject) => {
    server.once('error', reject);
    server.listen(5334, '127.0.0.1', resolve);
  });
  let browser;
  try {
    for (const [name, engine, width, dark] of [
      ['parent-control', webkit, 320, false],
      ['narrow-light', webkit, 320, false],
      ['phone-dark', webkit, 390, true],
      ['narrow-dark-chromium', chromium, 320, true],
      ['phone-light-chromium', chromium, 390, false],
      ['desktop-light', chromium, 1365, false],
      ['desktop-dark', chromium, 1365, true],
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
        errors: [],
        requests: [],
      };
      result.cases.push(record);
      page.on('pageerror', (error) => record.errors.push(error.message));
      page.on('response', (response) => {
        if (response.status() >= 400) record.errors.push(`${response.status()} ${response.url()}`);
      });
      await page.route('**/*', (route) => {
        const url = new URL(route.request().url());
        assert.equal(url.origin, origin, 'no external access');
        if (!url.pathname.startsWith('/api/')) return route.continue();
        assert.equal(route.request().method(), 'GET');
        assert(!/diagnostics|\/run|guest-agent|qemu/.test(url.pathname));
        record.requests.push(url.pathname);
        return route.fulfill({
          json: { success: true, data: [], alerts: [], anomalies: [], config: null },
        });
      });
      await page.addInitScript(
        (value) =>
          document.addEventListener('DOMContentLoaded', () =>
            document.documentElement.classList.toggle('dark', value),
          ),
        dark,
      );
      await page.goto(
        origin +
          (name === 'parent-control' ? '/parent' : '/final') +
          '/browser-tests/guest-row-memory-provenance.html',
        {
          timeout: 10000,
          waitUntil: 'domcontentloaded',
        },
      );
      const row = page.locator('[data-guest-id="fixture:pve1:101"]');
      const cell = row.locator('[data-workload-col="name"]');
      const notice = row.locator('[data-workload-memory-read-status]');
      await notice.waitFor();
      const identity = await row.elementHandle();
      const capture = async (state) => {
        const file = `${name}-${state}.png`;
        await page.screenshot({ path: path.join(output, file), fullPage: true });
        result.captures.push({ file, sha256: hash(path.join(output, file)) });
      };
      const geometry = async (guestName = 'backup-guest', lock = 'backup') =>
        cell.evaluate(
          (cell, values) => {
            const name = [...cell.querySelectorAll('[title]')].find(
              (el) => el.title === values.guestName,
            );
            const badge = [...cell.querySelectorAll('[title]')].find(
              (el) => el.title === `Guest is locked (${values.lock})`,
            );
            const box = (el) => {
              const r = el.getBoundingClientRect();
              return {
                x: r.x,
                y: r.y,
                width: r.width,
                height: r.height,
                right: r.right,
                bottom: r.bottom,
              };
            };
            return {
              cell: box(cell),
              name: box(name),
              nameWidth: name.clientWidth,
              nameScrollWidth: name.scrollWidth,
              lock: box(badge),
              lockWidth: badge.clientWidth,
              lockScrollWidth: badge.scrollWidth,
            };
          },
          { guestName, lock },
        );
      const g = await geometry();
      record.geometry = g;
      await capture('backup-bars');
      if (name === 'parent-control') {
        assert(g.nameWidth < g.nameScrollWidth - 1, 'current parent reproduces clipped guest name');
        record.checks.push('same static fixture rejects current parent: ordinary name clipped');
        await context.close();
        await browser.close();
        browser = undefined;
        record.closed = true;
        continue;
      }
      assert(g.nameWidth >= g.nameScrollWidth - 1, 'ordinary guest name remains readable');
      assert(
        g.lock.x >= g.cell.x && g.lock.right <= g.cell.right + 1,
        'complete lock fits its cell',
      );
      assert(g.lockScrollWidth <= g.lockWidth + 1, 'lock is not clipped');
      if (width <= 768)
        assert(g.lock.y >= g.name.bottom, 'lock is below, not competing with identity');
      assert((await notice.textContent()).includes('Not a current measurement'));
      assert((await notice.locator('.sr-only').textContent()).includes('2026-09-30 11:00:00 UTC'));
      assert(
        await notice
          .locator('[aria-hidden="true"]')
          .getByText(width <= 768 ? 'Prior' : 'Last known', { exact: true })
          .isVisible(),
      );
      record.checks.push(
        'ordinary name, full backup lock and original memory freshness fit without hover',
      );
      await page.getByLabel('Metric display').selectOption('sparklines');
      await row
        .getByRole('img', { name: 'backup-guest memory history, last known 25%', exact: true })
        .waitFor();
      assert.equal((await geometry()).nameWidth, g.nameWidth);
      if (name === 'phone-dark') await capture('backup-sparkline');
      record.checks.push(
        'sparkline mode retains the same readable identity/lock and last-known value',
      );
      await page.getByLabel('Metric display').selectOption('bars');
      const backupControl = cell.getByRole('button', { name: 'No backup found', exact: true });
      const backupBounds = await backupControl.boundingBox();
      assert(backupBounds.width >= 44 || width > 768, 'phone backup target retains 44 px width');
      assert(backupBounds.height >= 44 || width > 768, 'phone backup target retains 44 px height');
      assert(backupBounds.x >= g.cell.x && backupBounds.x + backupBounds.width <= g.cell.right + 1);
      if (width <= 768) await backupControl.tap();
      else await backupControl.click();
      const backupDialog = page.getByRole('dialog', {
        name: 'Backup status details for backup-guest',
        exact: true,
      });
      await backupDialog.waitFor();
      assert((await backupDialog.textContent()).includes('No completed backup found'));
      assert.equal(await page.getByRole('region', { name: 'Guest details' }).count(), 0);
      if (name === 'narrow-light') await capture('backup-evidence-dialog');
      await backupDialog.getByRole('button', { name: 'Close backup details', exact: true }).click();
      record.checks.push(
        'backup control retains its touch target and opens only same-guest completion evidence',
      );
      if (width <= 768) await cell.getByText('backup-guest', { exact: true }).tap();
      else {
        await page.getByLabel('Metric display').focus();
        for (let i = 0; i < 2; i++) await page.keyboard.press('Tab');
        assert(
          await row
            .getByRole('button', { name: 'Expand backup-guest', exact: true })
            .evaluate((el) => document.activeElement === el),
        );
        await page.keyboard.press('Enter');
      }
      const details = page.getByRole('region', { name: 'Guest details' });
      await details.waitFor();
      assert((await details.getByText('backup-guest', { exact: true }).count()) > 0);
      const precaution = await details.getByTestId('guest-read-precaution').textContent();
      assert(precaution.includes('Do not run live diagnostics or restart the guest agent'));
      assert(precaution.includes('An OK backup or a running VM does not prove thaw'));
      assert(
        precaution.includes(
          'Confirm thaw and writes to the filesystems covered by the backup independently',
        ),
      );
      assert((await details.textContent()).includes('Last known'));
      if (name === 'phone-dark') await capture('same-guest-drawer');
      await details
        .getByRole('button', { name: 'Collapse backup-guest details', exact: true })
        .click();
      assert.equal(await details.count(), 0);
      record.checks.push(
        'first touch or real keyboard Enter opens the same guest and preserves recovery precaution; collapse works',
      );
      await page.evaluate(() =>
        window.__memoryRow.update({
          name: 'long-backup-guest-identity-101',
          lock: 'snapshot-delete',
          backupInProgress: true,
        }),
      );
      const long = await geometry('long-backup-guest-identity-101', 'snapshot-delete');
      assert(long.name.width > 20, 'long identity must not disappear');
      if (width <= 768) {
        assert(
          long.nameScrollWidth > long.nameWidth,
          'long phone name truncates with its full title',
        );
      }
      assert(
        long.lock.right <= long.cell.right + 1 && long.lockScrollWidth <= long.lockWidth + 1,
        'long lock wraps inside identity cell',
      );
      if (name === 'narrow-light') await capture('long-identity-lock');
      record.longGeometry = long;
      await page.evaluate(() =>
        window.__memoryRow.update({
          name: 'backup-guest',
          lock: '',
          backupInProgress: false,
          lastSeen: '2026-10-06T10:00:00Z',
        }),
      );
      assert.equal(await cell.locator('[title^="Guest is locked"]').count(), 0);
      assert((await notice.textContent()).includes('2026-09-30 11:00:00 UTC'));
      assert(await identity.evaluate((el) => el.isConnected));
      assert(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth));
      assert.deepEqual(record.errors, []);
      record.checks.push(
        'same mounted row updates/removes locks without renewing memory; long identity stays titled, no page overflow/errors',
      );
      await context.close();
      await browser.close();
      browser = undefined;
      record.closed = true;
    }
    result.result = 'passed';
  } catch (error) {
    result.error = error.message;
    throw error;
  } finally {
    if (browser) await browser.close();
    result.cleanup.browser_closed = true;
    await new Promise((resolve) => server.close(resolve));
    result.cleanup.compiler_not_started = true;
    result.cleanup.server_closed = true;
    fs.writeFileSync(output + '/result.json', JSON.stringify(result, null, 2));
  }
})().catch((error) => {
  console.error(error);
  process.exitCode = 1;
});
