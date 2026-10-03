const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const crypto = require('node:crypto');
const { chromium, webkit } = require('playwright');

const baseline = false;
const root = '/workspace/frontend-modern';
const output = `/workspace/tmp/guest-backup-protection-${baseline ? 'parent' : 'final'}`;
const runtime = [
  'frontend-modern/src/components/Workloads/GuestDrawerOverview.tsx',
  'frontend-modern/src/utils/workloadGuestPresentation.ts',
  'frontend-modern/src/components/Workloads/GuestDrawer.tsx',
  'frontend-modern/src/components/Workloads/useGuestDrawerState.ts',
  'frontend-modern/src/components/Workloads/guestDrawerModel.ts',
];
const sha256 = (p) => crypto.createHash('sha256').update(fs.readFileSync(p)).digest('hex');

(async () => {
  fs.mkdirSync(output, { recursive: true });
  const playwrightVersion = require('playwright/package.json').version;
  assert.equal(
    playwrightVersion,
    JSON.parse(fs.readFileSync('/workspace/tests/integration/package-lock.json')).packages[
      'node_modules/@playwright/test'
    ].version,
  );
  const hashes = Object.fromEntries(runtime.map((p) => [p, sha256(path.join('/workspace', p))]));
  process.chdir(root);
  const { createServer } = await import(path.join(root, 'node_modules/vite/dist/node/index.js'));
  const server = await createServer({
    cacheDir: path.join(output, 'vite-cache'),
    server: { host: '127.0.0.1', port: 5296, strictPort: true },
  });
  const results = [],
    screenshots = [];
  let browser;
  try {
    await server.listen();
    for (const [name, engine, width, dark] of [
      ['desktop', chromium, 1365, false],
      ['phone', webkit, 390, true],
    ]) {
      browser = await engine.launch(
        engine === chromium
          ? { headless: true, channel: 'chromium', args: ['--no-sandbox'] }
          : { headless: true },
      );
      for (const kind of ['qemu', 'lxc']) {
        const page = await browser.newPage({
          viewport: { width, height: width === 390 ? 844 : 900 },
          isMobile: width === 390,
          hasTouch: width === 390,
          locale: 'en-GB',
          timezoneId: 'UTC',
        });
        page.setDefaultTimeout(15000);
        page.setDefaultNavigationTimeout(60000);
        await page.clock.setFixedTime(new Date('2026-10-03T12:00:00Z'));
        const errors = [],
          writes = [],
          checks = [];
        page.on('pageerror', (e) => errors.push(e.message));
        await page.route('**/*', (route) => {
          const request = route.request();
          const url = new URL(request.url());
          assert.equal(
            url.origin,
            'http://127.0.0.1:5296',
            'only guest-local fixture requests are allowed',
          );
          if (url.pathname.startsWith('/api/')) {
            if (request.method() !== 'GET') writes.push(`${request.method()} ${url.pathname}`);
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
        await page.goto(
          `http://127.0.0.1:5296/browser-tests/guest-backup-protection.html?kind=${kind}`,
          { waitUntil: 'domcontentloaded' },
        );
        await page.waitForFunction(() => window.__backupProtection);
        if (dark) await page.evaluate(() => document.documentElement.classList.add('dark'));
        assert.equal(await page.evaluate(() => window.innerWidth), width, 'actual CSS viewport');
        const details = page.getByTestId('guest-technical-details');
        await details.waitFor({ state: 'visible' });
        await page.evaluate(() => {
          window.__originalGuestDetails = document.querySelector(
            '[data-testid="guest-technical-details"]',
          );
          window.__originalGuestHeading = document.querySelector('h3');
        });
        const update = async (lastBackup, running, reason = '') => {
          await page.evaluate((value) => window.__backupProtection.update(value), {
            lastBackup,
            running,
            reason,
          });
          assert.equal(
            await page.evaluate(
              () =>
                document.querySelector('[data-testid="guest-technical-details"]') ===
                window.__originalGuestDetails,
            ),
            true,
          );
          assert.equal(
            await page.evaluate(() => window.__backupProtection.identity()),
            'fixture-pve:pve-a:101',
          );
        };
        const capture = async (suffix) => {
          const p = path.join(output, `${name}-${kind}-${suffix}.png`);
          await page.screenshot({ path: p, fullPage: true });
          screenshots.push({ path: path.relative('/workspace', p), sha256: sha256(p) });
        };
        const oldBackup = new Date('2026-09-23T12:00:00Z').getTime();
        if (baseline) {
          await update(0, true, kind === 'qemu' ? 'prev-vm-locked' : '');
          const activity = details.getByText('Backup running', { exact: true });
          await activity.waitFor({ state: 'visible' });
          assert((await activity.locator('..').getAttribute('class')).includes('text-emerald-700'));
          assert.equal(await details.getByText('No backup found', { exact: true }).count(), 0);
          checks.push(
            'parent: running backup hides missing completed protection and appears green',
          );
          if (kind === 'qemu') await capture('running-with-none');
          await update(oldBackup, true);
          assert.equal(await details.getByText('10d ago', { exact: true }).count(), 0);
          await update(oldBackup, false);
          await details.getByText('10d ago', { exact: true }).waitFor({ state: 'visible' });
          checks.push('parent: running backup hides existing ten-day-old completed evidence');
        } else {
          const missing = details.getByText('No completed backup found', { exact: true });
          const activity = details.getByText('Running · not completed yet', { exact: true });
          await missing.waitFor({ state: 'visible' });
          assert((await missing.locator('..').getAttribute('class')).includes('text-rose-700'));
          assert.equal(await details.getByText('Backup activity', { exact: true }).count(), 0);
          await update(0, true, kind === 'qemu' ? 'prev-vm-locked' : '');
          await activity.waitFor({ state: 'visible' });
          await missing.waitFor({ state: 'visible' });
          assert((await activity.locator('..').getAttribute('class')).includes('text-amber-700'));
          assert.equal(await details.getByText('Backup running', { exact: true }).count(), 0);
          if (kind === 'qemu')
            await details
              .getByText(/Using last known disk stats\. Guest reads paused/)
              .waitFor({ state: 'visible' });
          checks.push(
            'absent completion stays missing/danger while separate activity is cautionary; VM lock notice survives',
          );
          if (kind === 'qemu') await capture('running-with-none');
          await update(0, false);
          await missing.waitFor({ state: 'visible' });
          assert.equal(await activity.count(), 0);
          checks.push(
            'activity stopping without new completion evidence never manufactures protection',
          );
          await update(oldBackup, true);
          const age = details.getByText('10d ago', { exact: true });
          await age.waitFor({ state: 'visible' });
          assert((await age.locator('..').getAttribute('class')).includes('text-amber-700'));
          await activity.waitFor({ state: 'visible' });
          checks.push('stale completed age and caution tone remain visible during a new backup');
          if (kind === 'qemu') await capture('running-with-old');
          await update(oldBackup, false, kind === 'qemu' ? 'prev-agent-timeout' : '');
          await age.waitFor({ state: 'visible' });
          assert.equal(await activity.count(), 0);
          if (kind === 'qemu')
            await details
              .getByText(/Do not restart the guest agent during a backup/)
              .waitFor({ state: 'visible' });
          checks.push('ended activity preserves old completion and independent timeout safety');
          const freshBackup = new Date('2026-10-03T10:00:00Z').getTime();
          await update(freshBackup, false);
          const today = details.getByText('Today', { exact: true });
          await today.waitFor({ state: 'visible' });
          assert((await today.locator('..').getAttribute('class')).includes('text-emerald-700'));
          assert.equal(await missing.count(), 0);
          await update(freshBackup, true);
          await today.waitFor({ state: 'visible' });
          await activity.waitFor({ state: 'visible' });
          checks.push(
            'only new completed timestamp updates protection; fresh age stays separate from further activity',
          );
          await update(freshBackup, false);
          const close = page.getByRole('button', { name: 'Collapse backup-guest details' });
          if (name === 'phone') await close.tap();
          else {
            await close.focus();
            await page.keyboard.press('Enter');
          }
          assert.equal(await page.evaluate(() => window.__backupProtection.closeCount()), 1);
          checks.push(
            'existing collapse remains touch/keyboard usable and observations trigger no write',
          );
          assert.equal(
            await details.getByText('Last completed backup', { exact: true }).count(),
            1,
          );
          assert.equal(
            await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth),
            true,
          );
          if (kind === 'qemu') await capture('fresh-completion');
        }
        assert.deepEqual(writes, []);
        assert.deepEqual(errors, []);
        results.push({
          name,
          kind,
          browser_version: browser.version(),
          viewport: { width, height: width === 390 ? 844 : 900 },
          checks,
          page_errors: errors,
          writes,
        });
        await page.close();
      }
      await browser.close();
      browser = null;
    }
    const receipt = {
      version: 1,
      result: baseline ? 'defect-reproduced' : 'passed',
      verified_at: new Date().toISOString(),
      content_sha256: hashes,
      playwright_version: playwrightVersion,
      results,
      screenshots,
      limitation:
        'Actual GuestDrawer and production CSS with synthetic VM/CT observations. Not a native backup, usable archive, QGA/thaw, installation or release result.',
    };
    fs.writeFileSync(
      path.join(output, 'browser-result.json'),
      JSON.stringify(receipt, null, 2) + '\n',
    );
    process.stdout.write(JSON.stringify(receipt) + '\n');
  } finally {
    if (browser) await browser.close();
    await server.close();
  }
})().catch((e) => {
  process.stderr.write(e.stack + '\n');
  process.exitCode = 1;
});
