const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const crypto = require('node:crypto');
const { chromium, webkit } = require('playwright');
const root = '/workspace';
const frontend = path.join(root, 'frontend-modern');
const output = path.join(root, 'tmp/backup-details-access/browser');
const origin = 'http://127.0.0.1:5298';
const runtime = [
  'frontend-modern/src/components/Workloads/GuestRowCells.tsx',
  'frontend-modern/src/components/Workloads/GuestRow.tsx',
];
const hash = (p) => crypto.createHash('sha256').update(fs.readFileSync(p)).digest('hex');
(async () => {
  fs.mkdirSync(output, { recursive: true });
  const version = require('playwright/package.json').version;
  assert.equal(
    version,
    JSON.parse(fs.readFileSync(path.join(root, 'tests/integration/package-lock.json'))).packages[
      'node_modules/@playwright/test'
    ].version,
  );
  const hashes = Object.fromEntries(
    runtime.map((source) => [source, hash(path.join(root, source))]),
  );
  process.chdir(frontend);
  const { createServer } = await import(
    path.join(frontend, 'node_modules/vite/dist/node/index.js')
  );
  const server = await createServer({
    cacheDir: path.join(output, 'vite-cache'),
    server: { host: '127.0.0.1', port: 5298, strictPort: true },
  });
  const results = [];
  const captures = [];
  const phases = [];
  const started = Date.now();
  const mark = (stage) => {
    const fact = {
      stage,
      elapsedMs: Date.now() - started,
      memory: process.memoryUsage(),
      resources: process.resourceUsage(),
    };
    phases.push(fact);
    console.log(JSON.stringify({ browserPhase: fact }));
  };
  let browser;
  let phase = 'server';
  try {
    mark('listen-start');
    await server.listen();
    mark('listen-complete');
    // Supported Vite transform warmup, not a production build or app execution.
    // Keep page navigation and the tool deadline unchanged; collect phases.
    await server.warmupRequest('/browser-tests/backup-details-access.tsx');
    await server.warmupRequest('/src/index.css');
    mark('entry-css-warmup-complete');
    for (const [name, engine, phone, dark] of [
      ['desktop-light', chromium, false, false],
      ['desktop-dark', chromium, false, true],
      ['phone-light', webkit, true, false],
      ['phone-dark', webkit, true, true],
    ]) {
      phase = name;
      browser = await engine.launch(
        engine === chromium
          ? { headless: true, channel: 'chromium', args: ['--no-sandbox'] }
          : { headless: true },
      );
      const page = await browser.newPage({
        viewport: { width: phone ? 390 : 1365, height: phone ? 844 : 900 },
        isMobile: phone,
        hasTouch: phone,
        locale: 'en-GB',
        timezoneId: 'UTC',
      });
      page.setDefaultTimeout(12000);
      page.setDefaultNavigationTimeout(45000);
      await page.clock.setFixedTime(new Date('2026-10-05T08:00:00Z'));
      const errors = [];
      const operations = [];
      const reads = [];
      const checks = [];
      page.on('pageerror', (e) => errors.push(e.message));
      await page.route('**/*', (route) => {
        const request = route.request();
        const url = new URL(request.url());
        assert.equal(url.origin, origin, 'no off-origin requests');
        if (!url.pathname.startsWith('/api/')) return route.continue();
        if (request.method() !== 'GET') operations.push(`${request.method()} ${url.pathname}`);
        reads.push(url.pathname);
        return route.fulfill({
          json: { success: true, data: [], alerts: [], config: null, anomalies: [] },
        });
      });
      if (dark)
        await page.addInitScript(() =>
          document.addEventListener('DOMContentLoaded', () =>
            document.documentElement.classList.add('dark'),
          ),
        );
      mark(`${name}:navigate-start`);
      await page.goto(`${origin}/browser-tests/backup-details-access.html`, {
        waitUntil: 'domcontentloaded',
      });
      mark(`${name}:domcontentloaded`);
      const row = page.getByRole('region', { name: 'backup-vm row' });
      const badge = row.getByRole('button', { name: /^Backup status:/ });
      await badge.waitFor();
      const identity = await page.evaluate(() => window.__backupDetails.identity());
      const rowActionCount = async () =>
        Number(await page.getByRole('status', { name: 'Row action count' }).innerText());
      const activate = async (el) => (phone ? el.tap() : el.click());
      const capture = async (suffix) => {
        const file = path.join(output, `${name}-${suffix}.png`);
        await page.screenshot({ path: file, fullPage: true });
        captures.push({
          path: path.relative(root, file),
          bytes: fs.statSync(file).size,
          sha256: hash(file),
        });
      };
      const dialog = page.getByRole('dialog', { name: /^Backup status details for / });
      const close = page.getByRole('button', { name: 'Close backup details' });
      const checkOpen = async (trigger) => {
        await dialog.waitFor({ state: 'visible' });
        assert.equal(await trigger.getAttribute('aria-expanded'), 'true');
        assert.equal(await rowActionCount(), 0, 'backup action must not open the guest row');
        await close.waitFor();
        await page.waitForFunction(
          () => document.activeElement?.getAttribute('aria-label') === 'Close backup details',
        );
        // DOM visibility can precede the CSS entrance animation. Verify and
        // capture the final user-visible panel, not its initial off-screen frame.
        await dialog.evaluate(async (el) => {
          await Promise.all(
            el
              .getAnimations({ subtree: true })
              .filter((a) => Number.isFinite(a.effect?.getTiming().iterations))
              .map((a) => a.finished.catch(() => {})),
          );
        });
        const rect = await dialog.boundingBox();
        assert(
          rect.x >= 0 && rect.x + rect.width <= (phone ? 390 : 1365),
          'dialog stays within viewport',
        );
        assert(
          rect.y >= 0 && rect.y + rect.height <= (phone ? 844 : 900),
          'settled panel stays vertically in view',
        );
        assert(
          await page.locator('main').evaluate((el) => el.closest('body > *').hasAttribute('inert')),
          'background isolated',
        );
      };
      const checkClosed = async (trigger) => {
        await dialog.waitFor({ state: 'hidden' });
        assert.equal(await trigger.getAttribute('aria-expanded'), 'false');
        assert(
          await trigger.evaluate((el) => document.activeElement === el),
          'focus returns to source evidence',
        );
        assert.equal(
          await page.locator('main').evaluate((el) => el.closest('body > *').hasAttribute('inert')),
          false,
        );
      };
      assert.equal(await badge.getAttribute('aria-haspopup'), 'dialog');
      assert.equal((await badge.innerText()).trim(), '', 'fresh shield remains compact');
      if (!phone) {
        await badge.hover();
        const tooltip = page.locator('[data-tooltip-portal="true"]');
        await tooltip.waitFor({ state: 'visible' });
        assert((await tooltip.innerText()).includes('Last completed backup'));
        await page.getByRole('heading').hover();
        // Genuine sequential keyboard traversal, not synthetic click/focus.
        await page.keyboard.press('Tab'); // column control
        await page.keyboard.press('Tab'); // guest disclosure
        await page.keyboard.press('Tab'); // backup disclosure
        assert(await badge.evaluate((el) => document.activeElement === el));
        await page.keyboard.press('Enter');
        checks.push('desktop hover preserved and sequential Tab/Enter opens actual badge');
      } else {
        const rect = await badge.boundingBox();
        assert(rect.width >= 44 && rect.height >= 44, 'touch disclosure is at least 44px');
        await activate(badge);
        assert.equal(
          await page.locator('[data-tooltip-portal="true"]').count(),
          0,
          'no synthetic hover overlay',
        );
        checks.push('ordinary first tap opens 44px target, without hover UI');
      }
      await checkOpen(badge);
      assert((await dialog.innerText()).includes('backup-vm'));
      assert((await dialog.innerText()).includes('Mon, 5 Oct 2026'));
      assert((await dialog.innerText()).includes('06:00:00'));
      assert((await dialog.innerText()).includes('2 hours ago'));
      await capture('fresh-completed');
      await page.keyboard.press('Tab');
      assert(
        await close.evaluate((el) => document.activeElement === el),
        'one-action dialog retains focus',
      );
      await page.evaluate(() =>
        window.__backupDetails.update({ timestamp: '2026-10-05T06:00:00Z', running: true }),
      );
      assert((await dialog.innerText()).includes('Backup running now'));
      assert((await dialog.innerText()).includes('Last completed backup'));
      assert.equal(await page.evaluate(() => window.__backupDetails.identity()), identity);
      checks.push('same identity updates running while preserving prior completed observation');
      await capture('running-completed');
      await page.evaluate(() =>
        window.__backupDetails.update({ timestamp: '2026-10-05T08:01:00Z', running: true }),
      );
      await dialog.getByText(/timestamp is in the future/).waitFor();
      assert(!(await dialog.innerText()).includes('Last completed backup'));
      assert(!(await dialog.innerText()).includes('No backup has ever'));
      await capture('future-running');
      await page.keyboard.press('Escape');
      await checkClosed(badge);
      if (!phone) {
        await page.keyboard.press('Space');
        await checkOpen(badge);
        assert((await dialog.innerText()).includes('timestamp is in the future'));
        await page.keyboard.press('Escape');
        await checkClosed(badge);
        checks.push('native Space reopens current evidence and Escape restores focus');
      }
      await page.getByRole('button', { name: 'Hide Backup column' }).click();
      const indicator = row.getByRole('button', { name: /^Backup running now/ });
      await activate(indicator);
      await checkOpen(indicator);
      assert((await dialog.innerText()).includes('timestamp is in the future'));
      checks.push('hidden-column indicator exposes exactly the same uncertainty');
      await page.evaluate(() =>
        window.__backupDetails.update({ timestamp: 'bad-time', running: false }),
      );
      await dialog.getByText(/invalid timestamp/).waitFor();
      await page.evaluate(() => window.__backupDetails.update({ timestamp: '', running: false }));
      await dialog
        .getByText('No backup has ever been recorded for this guest.', { exact: true })
        .waitFor();
      assert(!(await dialog.innerText()).includes('Invalid Date'));
      assert(!(await dialog.innerText()).includes('NaN'));
      checks.push('malformed then absence retain distinct truthful states');
      await activate(close);
      await checkClosed(row.getByRole('button', { name: 'No backup found', exact: true }));
      const ct = page
        .getByRole('region', { name: 'backup-ct row' })
        .getByRole('button', { name: 'No backup found', exact: true });
      await activate(ct);
      await checkOpen(ct);
      assert((await dialog.innerText()).includes('backup-ct'));
      assert(
        !(await dialog.innerText()).includes('2 hours ago'),
        'adjacent guest never inherits another observation',
      );
      await capture('ct-missing');
      await page.locator('[data-dialog-backdrop]').click({ position: { x: 3, y: 3 } });
      await checkClosed(ct);
      await page.evaluate(() =>
        window.__backupDetails.update({ timestamp: '2026-10-05T06:00:00Z', running: false }),
      );
      const recovered = row.getByRole('button', { name: /^Last backup:/ });
      await activate(recovered);
      await checkOpen(recovered);
      assert((await dialog.innerText()).includes('2 hours ago'));
      await page.evaluate(() => window.__backupDetails.remove());
      await dialog.waitFor({ state: 'hidden' });
      assert.equal(await page.locator('[data-dialog-layer]').count(), 0);
      assert.equal(
        await page.locator('main').evaluate((el) => el.closest('body > *').hasAttribute('inert')),
        false,
      );
      checks.push(
        'recovery and actual row removal release dialog/inertness without extra operations',
      );
      assert.equal(await rowActionCount(), 0);
      await activate(
        page.getByRole('region', { name: 'backup-ct row' }).getByText('backup-ct', { exact: true }),
      );
      assert.equal(await rowActionCount(), 1, 'ordinary row action still works');
      assert.deepEqual(errors, []);
      assert.deepEqual(operations, []);
      assert(
        !reads.some((p) => /backup|metrics|guest|resources/.test(p)),
        'opening evidence adds no collector/history read',
      );
      results.push({
        name,
        browser: browser.version(),
        checks,
        apiReads: reads,
        operations,
        errors,
        finalRowActions: await rowActionCount(),
      });
      mark(`${name}:checks-complete`);
      await page.close();
      await browser.close();
      browser = undefined;
    }
    fs.writeFileSync(
      path.join(output, 'result.json'),
      JSON.stringify(
        {
          result: 'passed',
          playwrightVersion: version,
          runtime: hashes,
          phases,
          results,
          captures,
        },
        null,
        2,
      ) + '\n',
    );
    console.log(
      JSON.stringify({
        result: 'passed',
        cases: results.length,
        checks: results.reduce((n, r) => n + r.checks.length, 0),
        captures: captures.length,
        hashes,
        playwrightVersion: version,
      }),
    );
  } catch (e) {
    fs.writeFileSync(
      path.join(output, 'failure.json'),
      JSON.stringify(
        {
          result: 'failed',
          phase,
          error: String(e),
          runtime: hashes,
          phases,
          playwrightVersion: version,
          captures,
          results,
        },
        null,
        2,
      ) + '\n',
    );
    throw e;
  } finally {
    if (browser) await browser.close();
    await server.close();
    mark('browser-server-closed');
  }
})().catch((e) => {
  console.error(e);
  process.exitCode = 1;
});
