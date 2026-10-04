const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const crypto = require('node:crypto');
const { chromium, webkit } = require('playwright');
(async () => {
  const root = '/workspace/frontend-modern';
  const output = '/workspace/tmp/operator-tables-proof';
  fs.mkdirSync(output, { recursive: true });
  process.chdir(root);
  const playwright = require('playwright/package.json').version;
  assert.equal(
    playwright,
    JSON.parse(fs.readFileSync('/workspace/tests/integration/package-lock.json')).packages[
      'node_modules/@playwright/test'
    ].version,
  );
  const paths = JSON.parse(fs.readFileSync(path.join(output, 'runtime-paths.json')));
  const hashes = Object.fromEntries(
    paths.map((p) => [
      p,
      crypto
        .createHash('sha256')
        .update(fs.readFileSync(path.join('/workspace', p)))
        .digest('hex'),
    ]),
  );
  const { createServer } = await import(path.join(root, 'node_modules/vite/dist/node/index.js'));
  const server = await createServer({
    root,
    configFile: path.join(root, 'vite.config.ts'),
    cacheDir: path.join(output, 'vite-cache'),
    server: { host: '127.0.0.1', port: 5271, strictPort: true },
  });
  const results = [],
    screens = [];
  let browser;
  try {
    await server.listen();
    for (const [name, engine, width] of [
      ['desktop', chromium, 1365],
      ['phone', webkit, 390],
      ['intermediate', chromium, 768],
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
      await page.clock.setFixedTime(new Date('2026-10-03T12:00:00Z'));
      const errors = [],
        offOrigin = [],
        checks = [],
        requests = [];
      let rollbackMode = 'reject',
        finishRollback;
      page.on('pageerror', (e) => errors.push(e.message));
      page.setDefaultTimeout(20000);
      await page.route('**/*', async (route) => {
        const url = new URL(route.request().url());
        if (url.origin !== 'http://127.0.0.1:5271') {
          offOrigin.push(url.origin);
          return route.abort();
        }
        if (!url.pathname.startsWith('/api/')) return route.continue();
        if (url.pathname === '/api/updates/history')
          return route.fulfill({
            json: [
              {
                event_id: 'fixture-update-1',
                timestamp: '2026-10-02T12:00:00Z',
                action: 'update',
                channel: 'stable',
                version_from: '6.4.5',
                version_to: '6.4.6',
                deployment_type: 'systemd',
                initiated_by: 'user',
                initiated_via: 'ui',
                status: 'success',
                duration_ms: 30000,
                backup_path: '/private/fixture-backup',
                notes: 'untrusted restore-everything note',
              },
            ],
          });
        if (url.pathname === '/api/version')
          return route.fulfill({
            json: {
              version: '6.6.0-dev',
              build: 'synthetic',
              runtime: 'community',
              isDocker: false,
              isSourceBuild: true,
              isDevelopment: false,
            },
          });
        if (url.pathname === '/api/updates/rollback') {
          requests.push({ method: route.request().method(), body: route.request().postDataJSON() });
          if (rollbackMode === 'hold')
            await new Promise((resolve) => {
              finishRollback = resolve;
            });
          return rollbackMode === 'accept'
            ? route.fulfill({
                json: { status: 'accepted', message: 'Synthetic acceptance, not recovery' },
              })
            : route.fulfill({
                status: 409,
                json: { error: 'Synthetic incompatible backup scope' },
              });
        }
        if (url.pathname === '/api/license/runtime-capabilities')
          return route.fulfill({
            json: {
              capabilities: [],
              limits: [],
              max_history_days: 7,
              hosted_mode: false,
              runtime: { build: 'community' },
              blocked_capabilities: [],
            },
          });
        if (url.pathname === '/api/license/entitlements')
          return route.fulfill({ json: { entitlements: [] } });
        if (url.pathname.includes('/facets'))
          return route.fulfill({
            json: { facets: [], capabilities: [], relationships: [], recentChanges: [] },
          });
        if (url.pathname === '/api/security/status')
          return route.fulfill({
            json: {
              hasAuthentication: true,
              requiresAuth: true,
              settingsCapabilities: { authenticationWrite: true },
            },
          });
        if (url.pathname === '/api/metrics-store/history')
          return route.fulfill({ json: { points: [], metrics: {}, source: 'store' } });
        return route.fulfill({
          json: {
            enabled: false,
            data: [],
            alerts: {},
            facets: [],
            capabilities: [],
            relationships: [],
            recentChanges: [],
          },
        });
      });
      const check = async (label, run) => {
        try {
          await run();
          checks.push({ label, passed: true });
        } catch (e) {
          checks.push({ label, passed: false, error: e.message });
        }
      };
      const select = async (name) => {
        await page
          .getByRole('navigation', { name: 'Fixture views' })
          .getByRole('button', { name, exact: true })
          .click();
      };
      const capture = async (state) => {
        // Await the production entrance animation instead of capturing translucent frames.
        if (await page.getByRole('dialog').count()) {
          await page.waitForFunction(() => {
            const panel = document.querySelector('[role=dialog]');
            return panel && Number(getComputedStyle(panel).opacity) >= 0.999;
          });
        }
        await page.waitForTimeout(100); // ResizeObserver layout, not freshness proof.
        const p = `${name}-${state}.png`;
        await page.screenshot({ path: path.join(output, p), fullPage: true });
        const dimensions = await page.evaluate(() => ({
          width: innerWidth,
          scroll: document.documentElement.scrollWidth,
        }));
        assert.ok(dimensions.scroll <= dimensions.width + 1, JSON.stringify(dimensions));
        screens.push({ file: p, state, browser: name, width, dimensions });
      };
      await page.goto('http://127.0.0.1:5271/browser-tests/operator-tables-consent.html', {
        waitUntil: 'domcontentloaded',
        timeout: 120000,
      });
      if (width === 390) await page.evaluate(() => document.documentElement.classList.add('dark'));
      await check('Pool state and measured capacity survive the phone projection', async () => {
        const pool = page.getByRole('region', { name: 'Pool table' });
        await pool.getByText('archive-tank', { exact: true }).waitFor();
        await pool.getByText('Degraded', { exact: true }).waitFor();
        const state = pool.locator('tbody td[data-storage-column="state"]').first();
        const bounds = await state.evaluate((e) => ({
          width: e.clientWidth,
          scroll: e.scrollWidth,
          text: e.textContent,
        }));
        assert.ok(bounds.scroll <= bounds.width + 1, JSON.stringify(bounds));
        if (width >= 544) {
          await pool.getByText('Resilvering 45%', { exact: true }).waitFor();
          await pool.getByText('Scrubbing 45%', { exact: true }).waitFor();
          assert.ok(
            await pool
              .locator('[title="ZFS pool archive-tank resilver is running (45.2%)"]')
              .count(),
          );
        } else {
          assert.ok((await pool.getByText('59%', { exact: true }).count()) >= 2);
        }
        await capture('pools');
      });
      if (width !== 768) {
        await select('Disks');
        await check('Disk row and expanded header retain the full SMART reason', async () => {
          await page.getByText('Archive HDD', { exact: true }).waitFor();
          await page
            .locator('[data-row-id="disk-sda"]')
            .getByText('Archive HDD', { exact: true })
            .click();
          const health = page.getByTestId('disk-detail-health');
          await health.getByText('Replace Now', { exact: true }).waitFor();
          assert.ok(
            (await health.innerText()).includes('SMART failed. The full reason remains readable'),
          );
          const bounds = await health.evaluate((e) => ({
            width: e.clientWidth,
            scroll: e.scrollWidth,
          }));
          assert.ok(bounds.scroll <= bounds.width + 1, JSON.stringify(bounds));
          await capture('disk-reason');
        });
        await select('Controllers');
        await check(
          'Job target and absolute started/completed times are reachable from a narrow row',
          async () => {
            await page.getByText('nightly-import', { exact: true }).click();
            const details = page.getByTestId('resource-kubernetes-controller-section');
            await details.getByText('Started', { exact: true }).waitFor();
            await details.getByText('Completed', { exact: true }).waitFor();
            await details.getByText('5m', { exact: true }).waitFor();
            await details.getByText('1 completion', { exact: true }).waitFor();
            assert.ok((await details.innerText()).includes('2026'));
            await capture('job-times');
          },
        );
        await check(
          'CronJob retains last-run, last-success, schedule and suspension in its expansion',
          async () => {
            await page.getByText('billing-rollup', { exact: true }).click();
            const details = page.getByTestId('resource-kubernetes-controller-section');
            await details.getByText('Last run', { exact: true }).waitFor();
            await details.getByText('Last success', { exact: true }).waitFor();
            await details.getByText('*/5 * * * *', { exact: true }).waitFor();
            await details.getByText('Suspended', { exact: true }).waitFor();
            await capture('cron-times');
          },
        );
      }
      await select('Backups');
      await check(
        'Coverage last backup remains older than a fresh local guest snapshot',
        async () => {
          const coverage = page.locator('[data-proxmox-backups-table="coverage"]');
          await coverage.waitFor();
          const columns = await coverage
            .locator('colgroup col')
            .evaluateAll((els) => els.map((e) => e.dataset.proxmoxBackupsColumn));
          const row = coverage.locator('tr[data-proxmox-backup-row=coverage]');
          const backupAge = await row.locator('td').nth(columns.indexOf('latest')).innerText();
          assert.ok(backupAge.includes('13d'), backupAge);
          assert.ok(!backupAge.includes('30m'), backupAge);
          if (columns.includes('snapshot')) {
            assert.ok(
              (await row.locator('td').nth(columns.indexOf('snapshot')).innerText()).includes(
                '30m',
              ),
            );
          }
          await capture('backup-attribution');
        },
      );
      await check(
        'By date keeps archive format visible and full identifier available at wide widths',
        async () => {
          const byDate = page.locator('[data-proxmox-backups-table="recoverable"]');
          await byDate.waitFor();
          if (width >= 1120) {
            await byDate.getByText('tar.zst', { exact: true }).waitFor();
            assert.ok(
              await byDate
                .locator('[title="local:backup/vzdump-lxc-310-2026_09_20-02_00_00.tar.zst"]')
                .count(),
            );
          }
          assert.ok((await page.getByText('artifact-cache-310', { exact: true }).count()) >= 2);
        },
      );
      if (width !== 768) {
        await select('Consent');
        await check(
          'Consent separates running/backup versions and does not promise active-data restoration',
          async () => {
            const action = page.getByRole('button', { name: 'Roll back', exact: true });
            await action.focus();
            await page.keyboard.press('Enter');
            const dialog = page.getByRole('dialog', { name: 'Confirm rollback' });
            await dialog.getByText('Roll back to Pulse v6.4.5?', { exact: true }).waitFor();
            await dialog.getByText('Running now: Pulse v6.6.0-dev.', { exact: true }).waitFor();
            const text = await dialog.innerText();
            assert.ok(text.includes('Restore scope is not reported for this backup.'));
            assert.ok(text.includes('Older recovery can also replace install-local data'));
            assert.ok(text.includes('Do not assume this will undo settings or alerts.'));
            assert.ok(!text.includes('/private/fixture-backup'));
            assert.ok(!text.includes('untrusted restore-everything note'));
            assert.equal(
              await dialog
                .getByRole('link', { name: 'Read the recovery instructions' })
                .getAttribute('href'),
              '/docs/AUTO_UPDATE#manual-rollback',
            );
            assert.equal(requests.length, 0);
            await capture('consent');
            await dialog.getByRole('button', { name: 'Cancel', exact: true }).click();
            assert.equal(await page.getByRole('dialog').count(), 0);
            assert.equal(requests.length, 0);
            assert.equal(await action.evaluate((e) => e === document.activeElement), true);
          },
        );
        await check(
          'Rejection retains consent and pending submission cannot be duplicated or cancelled',
          async () => {
            await page.getByRole('button', { name: 'Roll back', exact: true }).click();
            await page.getByRole('button', { name: 'Roll back to v6.4.5', exact: true }).click();
            await page
              .getByText('Synthetic incompatible backup scope', { exact: true })
              .first()
              .waitFor();
            assert.equal(requests.length, 1);
            assert.ok(
              (await page.getByRole('dialog').innerText()).includes(
                'Restore scope is not reported',
              ),
            );
            await capture('consent-rejected');
            rollbackMode = 'hold';
            await page.getByRole('button', { name: 'Roll back to v6.4.5', exact: true }).click();
            await page.waitForFunction(
              () => document.querySelector('button:disabled')?.textContent,
            );
            assert.ok(
              await page
                .getByRole('button', { name: 'Starting rollback...', exact: true })
                .isDisabled(),
            );
            assert.ok(await page.getByRole('button', { name: 'Cancel', exact: true }).isDisabled());
            await page.keyboard.press('Escape');
            assert.equal(await page.getByRole('dialog').count(), 1);
            while (!finishRollback) await new Promise((resolve) => setTimeout(resolve, 10));
            assert.equal(requests.length, 2);
            rollbackMode = 'accept';
            finishRollback();
            await page.getByRole('dialog').waitFor({ state: 'hidden' });
            assert.deepEqual(requests, [
              { method: 'POST', body: { eventId: 'fixture-update-1' } },
              { method: 'POST', body: { eventId: 'fixture-update-1' } },
            ]);
          },
        );
      }
      await check('No uncaught application errors or external requests', async () => {
        assert.deepEqual(errors, []);
        assert.deepEqual(offOrigin, []);
      });
      results.push({
        name,
        width,
        browser: browser.version(),
        checks,
        errors,
        offOrigin,
        rollbackRequests: requests,
      });
      await page.close();
      await browser.close();
      browser = undefined;
    }
  } finally {
    if (browser) await browser.close();
    await server.close();
  }
  const result = {
    result: results.every((r) => r.checks.every((c) => c.passed)) ? 'passed' : 'failed',
    playwright,
    base_sha: '697ad83e19fa5e8afee634c212c945600d2292b4',
    source_content_sha256: hashes,
    results,
    screens,
  };
  fs.writeFileSync(path.join(output, 'result.json'), JSON.stringify(result, null, 2));
  console.log(
    JSON.stringify({
      result: result.result,
      checks: results.flatMap((r) => r.checks).length,
      failed: results.flatMap((r) => r.checks.filter((c) => !c.passed)),
      screens: screens.length,
    }),
  );
  if (result.result !== 'passed') process.exitCode = 1;
})().catch((e) => {
  console.error(e);
  process.exitCode = 1;
});
