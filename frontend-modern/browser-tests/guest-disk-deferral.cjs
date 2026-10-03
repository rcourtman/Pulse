const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const crypto = require('node:crypto');
const { chromium, webkit } = require('playwright');
const root = '/workspace/frontend-modern';
const output = '/workspace/tmp/guest-disk-deferral-proof';
const runtime = [
  'frontend-modern/src/utils/workloadGuestPresentation.ts',
  'frontend-modern/src/components/Workloads/GuestRow.tsx',
  'frontend-modern/src/components/Workloads/stackedDiskBarModel.ts',
  'frontend-modern/src/components/Workloads/StackedDiskBar.tsx',
  'frontend-modern/src/components/Workloads/DiskList.tsx',
  'frontend-modern/src/components/Workloads/GuestDrawerOverview.tsx',
];
const deferrals = [
  ['vm-locked', 'VM operation lock'],
  ['lock-unverified', 'cannot verify that the VM is unlocked'],
  ['agent-busy', 'earlier guest request is still in progress'],
  ['agent-cooldown', 'after the cooldown'],
  ['agent-response-incomplete', 'previous response was incomplete'],
  ['agent-capacity', 'guest-read capacity'],
  ['invalid-guest-key', 'VM identity is invalid'],
  ['agent-timeout', 'Do not restart the guest agent during a backup'],
];
(async () => {
  fs.mkdirSync(output, { recursive: true });
  const playwrightVersion = require('playwright/package.json').version;
  assert.equal(
    playwrightVersion,
    JSON.parse(fs.readFileSync('/workspace/tests/integration/package-lock.json')).packages[
      'node_modules/@playwright/test'
    ].version,
  );
  const hashes = Object.fromEntries(
    runtime.map((p) => [
      p,
      crypto
        .createHash('sha256')
        .update(fs.readFileSync(path.join('/workspace', p)))
        .digest('hex'),
    ]),
  );
  process.chdir(root);
  const { createServer } = await import(path.join(root, 'node_modules/vite/dist/node/index.js'));
  const server = await createServer({
    root,
    configFile: path.join(root, 'vite.config.ts'),
    cacheDir: path.join(output, 'vite-cache'),
    server: { host: '127.0.0.1', port: 5294, strictPort: true },
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
      const page = await browser.newPage({
        viewport: { width, height: width === 390 ? 844 : 900 },
        isMobile: width === 390,
        hasTouch: width === 390,
        locale: 'en-GB',
        timezoneId: 'UTC',
      });
      page.setDefaultTimeout(15000);
      page.setDefaultNavigationTimeout(60000);
      const errors = [],
        checks = [];
      page.on('pageerror', (e) => {
        errors.push(e.message);
        process.stderr.write(`BROWSER_PAGE_ERROR ${e.message}\n`);
      });
      page.on('console', (m) => {
        if (m.type() === 'error')
          process.stderr.write(`BROWSER_CONSOLE ${m.text().slice(0, 1500)}\n`);
      });
      page.on('response', (r) => {
        if (r.status() >= 400)
          process.stderr.write(`BROWSER_HTTP ${r.status()} ${new URL(r.url()).pathname}\n`);
      });
      await page.route('http://127.0.0.1:5294/api/**', (route) =>
        route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({
            success: true,
            data: [],
            alerts: [],
            anomalies: [],
            config: null,
          }),
        }),
      );
      await page.goto('http://127.0.0.1:5294/browser-tests/guest-disk-deferral.html', {
        waitUntil: 'domcontentloaded',
      });
      try {
        await page.waitForFunction(() => window.__guestDiskEvidence, null, { timeout: 15000 });
      } catch (error) {
        fs.writeFileSync(
          path.join(output, 'failure.json'),
          JSON.stringify({ phase: 'fixture-readiness', errors }, null, 2),
        );
        throw error;
      }
      if (dark) await page.evaluate(() => document.documentElement.classList.add('dark'));
      const row = page.locator('[data-guest-id="fixture-pve:pve-a:101"]');
      const cell = row.locator('[data-workload-col="disk"]');
      await row.click();
      const drawer = page.getByRole('region', { name: 'Guest Overview' });
      await drawer.waitFor({ state: 'visible' });
      await page.evaluate(
        () =>
          (window.__originalDiskRow = document.querySelector(
            '[data-guest-id="fixture-pve:pve-a:101"]',
          )),
      );
      const update = async (reason, data, usage = 50) => {
        await page.evaluate((value) => window.__guestDiskEvidence.update(value), {
          reason,
          data,
          usage,
        });
        await page.waitForFunction(
          () =>
            document.querySelector('[data-guest-id="fixture-pve:pve-a:101"]') ===
            window.__originalDiskRow,
        );
        assert.equal(
          await page.evaluate(() => window.__guestDiskEvidence.identity()),
          'fixture-pve:pve-a:101',
        );
      };
      const capture = async (suffix) => {
        const p = path.join(output, `${name}-${suffix}.png`);
        await page.screenshot({ path: p, fullPage: true });
        screenshots.push({
          path: path.relative('/workspace', p),
          sha256: crypto.createHash('sha256').update(fs.readFileSync(p)).digest('hex'),
        });
      };
      for (const [reason, expected] of deferrals) {
        await update(`prev-${reason}`, true);
        await page.waitForFunction(
          (text) => document.querySelector('[data-workload-col="disk"]').title.includes(text),
          expected,
        );
        const title = await cell.getAttribute('title');
        assert(title.startsWith('Using last known disk stats. '));
        assert(title.includes(expected));
        assert(
          !title.includes('may not be installed') && !title.includes('may need to be restarted'),
        );
        assert((await drawer.innerText()).includes(title));
        const statusBox = await drawer.getByText(title, { exact: true }).boundingBox();
        assert(
          statusBox.width >= 100 && statusBox.height < 220,
          'status explanation fits its card',
        );
        assert((await drawer.innerText()).includes('/data'));
        assert(
          (await page.getByRole('region', { name: 'Guest disk list' }).innerText()).includes(title),
        );
        if (name === 'desktop') {
          // The disclosure click can land on this bar and dismiss its hover
          // overlay on pointerdown. Leave it before exercising a new hover.
          await page.locator('h1').hover();
          await cell.locator('[data-stacked-disk-trigger]').hover();
          await page.locator('[data-tooltip-portal="true"]').waitFor({ state: 'visible' });
          assert((await page.locator('[data-tooltip-portal="true"]').innerText()).includes(title));
          await page.locator('h1').hover();
        }
        checks.push(`${reason}: retained row, tooltip, drawer, list and identity`);
        if (reason === 'vm-locked') await capture('retained-backup');
        await update(reason, false);
        await page.waitForFunction(
          (text) => document.querySelector('[data-workload-col="disk"]').title.includes(text),
          expected,
        );
        assert(!(await cell.getAttribute('title')).includes('Using last known'));
        assert((await drawer.innerText()).includes(expected));
        assert(!(await drawer.innerText()).includes('/data'));
        assert.equal(await cell.locator('[data-stacked-disk-trigger]').count(), 0);
        checks.push(`${reason}: no previous disk values and no fresh percentage`);
        if (reason === 'agent-timeout') await capture('uncertain-timeout');
      }
      for (const [reason, expected] of [
        ['permission-denied', 'Permission denied'],
        ['agent-disabled', 'disabled in VM configuration'],
        ['agent-not-running', 'Guest agent not running'],
        ['no-data', 'No disk data available'],
        ['vm-stopped', 'VM is stopped'],
        ['no-status', 'could not read the VM status'],
        ['agent-error', 'Error communicating with guest agent'],
      ]) {
        await update(reason, false);
        await page.waitForFunction(
          (text) => document.querySelector('[data-workload-col="disk"]').title.includes(text),
          expected,
        );
        assert((await drawer.innerText()).includes(expected));
        checks.push(`${reason}: existing unavailable case remains distinct`);
      }
      await update('prev-agent-cooldown', true);
      await page.evaluate(() => window.__guestDiskEvidence.mode('sparklines'));
      assert((await cell.getAttribute('title')).includes('Using last known'));
      await update('', true, 75);
      await page.waitForFunction(
        () => !document.querySelector('[data-workload-col="disk"]').hasAttribute('title'),
      );
      assert(!(await drawer.innerText()).includes('Using last known'));
      assert(!(await drawer.innerText()).includes('Guest reads'));
      assert.equal(await drawer.getByRole('progressbar').getAttribute('aria-valuenow'), '75');
      await page.evaluate(() => window.__guestDiskEvidence.mode('bars'));
      assert.equal(
        await page.evaluate(
          () =>
            document.querySelector('[data-guest-id="fixture-pve:pve-a:101"]') ===
            window.__originalDiskRow,
        ),
        true,
      );
      checks.push(
        'same-VM fresh resumption removes notice and updates values in both row modes without remount',
      );
      await capture('fresh-resumption');
      assert.equal(
        await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth),
        true,
      );
      assert.deepEqual(errors, []);
      results.push({
        name,
        browser_version: browser.version(),
        viewport: { width, height: width === 390 ? 844 : 900 },
        checks,
        page_errors: errors,
      });
      await browser.close();
      browser = null;
    }
    const receipt = {
      version: 1,
      result: 'passed',
      verified_at: new Date().toISOString(),
      content_sha256: hashes,
      playwright_version: playwrightVersion,
      results,
      screenshots,
      limitation:
        'Synthetic same-VM observations through production components and CSS; no native QGA/backup, History sample, installation or release acceptance.',
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
