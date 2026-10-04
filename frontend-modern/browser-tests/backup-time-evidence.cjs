const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const crypto = require('node:crypto');
const { chromium, webkit } = require('playwright');

const root = '/workspace/frontend-modern';
const output = '/workspace/tmp/backup-time/browser';
const origin = 'http://127.0.0.1:5297';
const now = '2026-10-04T08:00:00Z';
const fresh = '2026-10-04T06:00:00Z';
const future = '2026-10-04T08:01:00Z';
const futureMessage =
  'Backup time unavailable: timestamp is in the future. Check the Proxmox and browser clocks.';
const invalidMessage = 'Backup time unavailable: invalid timestamp.';
const runtime = [
  'frontend-modern/src/utils/format.ts',
  'frontend-modern/src/utils/workloadGuestPresentation.ts',
  'frontend-modern/src/hooks/useWorkloads.ts',
  'frontend-modern/src/components/Workloads/GuestRowCells.tsx',
  'frontend-modern/src/components/Workloads/guestDrawerModel.ts',
  'frontend-modern/src/components/Workloads/useGuestDrawerState.ts',
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
    server: { host: '127.0.0.1', port: 5297, strictPort: true },
  });
  const results = [];
  const screenshots = [];
  let browser;
  let phase = 'server';
  try {
    await server.listen();
    for (const [name, engine, width, dark] of [
      ['chromium-desktop', chromium, 1365, false],
      ['webkit-phone', webkit, 390, true],
    ]) {
      browser = await engine.launch(
        engine === chromium
          ? { headless: true, channel: 'chromium', args: ['--no-sandbox'] }
          : { headless: true },
      );
      for (const transport of ['api', 'canonical-full', 'canonical-delta']) {
        for (const kind of ['qemu', 'lxc']) {
          phase = `${name}/${transport}/${kind}`;
          const page = await browser.newPage({
            viewport: { width, height: width === 390 ? 844 : 900 },
            isMobile: width === 390,
            hasTouch: width === 390,
            locale: 'en-GB',
            timezoneId: 'UTC',
          });
          page.setDefaultTimeout(15000);
          page.setDefaultNavigationTimeout(60000);
          await page.clock.setFixedTime(new Date(now));
          let observation = { timestamp: fresh, running: false };
          let resourceReads = 0;
          const errors = [];
          const writes = [];
          const checks = [];
          page.on('pageerror', (error) => errors.push(error.message));
          await page.route('**/*', (route) => {
            const request = route.request();
            const url = new URL(request.url());
            assert.equal(url.origin, origin, 'no off-origin requests');
            if (!url.pathname.startsWith('/api/')) return route.continue();
            if (request.method() !== 'GET') writes.push(`${request.method()} ${url.pathname}`);
            let body = { success: true, data: [], alerts: [], anomalies: [], config: null };
            if (url.pathname === '/api/resources') {
              resourceReads++;
              body = {
                data: [
                  {
                    id: 'fixture-a-pve1-101',
                    type: kind === 'lxc' ? 'system-container' : 'vm',
                    name: 'backup-guest',
                    status: 'running',
                    sources: ['proxmox'],
                    metrics: { cpu: { percent: 10 }, memory: { percent: 25 } },
                    proxmox: {
                      instance: 'fixture-a',
                      nodeName: 'pve1',
                      vmid: 101,
                      runtimeStatus: 'running',
                      lastBackup: observation.timestamp,
                      backupInProgress: observation.running,
                      lock: observation.running ? 'backup' : '',
                      disks: [],
                    },
                  },
                ],
                meta: { totalPages: 1 },
              };
            }
            return route.fulfill({
              status: 200,
              contentType: 'application/json',
              body: JSON.stringify(body),
            });
          });
          if (dark)
            await page.addInitScript(() =>
              document.addEventListener('DOMContentLoaded', () =>
                document.documentElement.classList.add('dark'),
              ),
            );
          await page.goto(
            `${origin}/browser-tests/backup-time-evidence.html?transport=${transport}&kind=${kind}`,
            { waitUntil: 'domcontentloaded' },
          );
          const row = page.getByRole('region', { name: 'Backup age row' });
          const badge = row.locator('[aria-label^="Backup status:"]');
          const indicatorRow = page.getByRole('region', { name: 'Backup indicator row' });
          const indicator = indicatorRow.locator(
            '[aria-label^="Last backup:"], [aria-label^="No backup"], [aria-label^="Backup running now"], [aria-label^="Backup time unavailable:"]',
          );
          const details = page.getByTestId('guest-technical-details');
          await details.waitFor();
          await badge.waitFor();
          const identity = await page.evaluate(() => window.__backupTimeEvidence.identity());
          await page.evaluate(() => {
            window.__originalBackupRow = document.querySelector('[aria-label="Backup age row"] tr');
            window.__originalBackupDetails = document.querySelector(
              '[data-testid="guest-technical-details"]',
            );
          });
          const observe = async (timestamp, running = false) => {
            observation = { timestamp, running };
            await page.evaluate(
              async (value) => window.__backupTimeEvidence.update(value),
              observation,
            );
            await page.evaluate(async () => {
              await document.fonts.ready;
              await new Promise((r) => requestAnimationFrame(() => requestAnimationFrame(r)));
            });
            assert.equal(
              await page.evaluate(() => window.__backupTimeEvidence.identity()),
              identity,
            );
            assert(
              await page.evaluate(
                () =>
                  window.__originalBackupRow ===
                  document.querySelector('[aria-label="Backup age row"] tr'),
              ),
            );
            assert(
              await page.evaluate(
                () =>
                  window.__originalBackupDetails ===
                  document.querySelector('[data-testid="guest-technical-details"]'),
              ),
            );
          };
          const capture = async (suffix) => {
            const p = path.join(output, `${name}-${kind}-${suffix}.png`);
            await page.screenshot({ path: p, fullPage: true });
            screenshots.push({ path: path.relative('/workspace', p), sha256: sha256(p) });
          };
          const checkUnknown = async (message, running) => {
            assert.equal((await badge.innerText()).trim(), running ? 'Running' : 'Unknown');
            assert((await badge.getAttribute('aria-label')).includes(message));
            assert(
              (await indicator.getAttribute('aria-label'))
                .toLowerCase()
                .includes(message.toLowerCase()),
            );
            assert(!(await badge.getAttribute('class')).includes('green'));
            assert(!(await indicator.getAttribute('class')).includes('green'));
            const reason = details.getByText(message, { exact: true });
            await reason.waitFor({ state: 'visible' });
            assert((await reason.locator('..').getAttribute('class')).includes('amber'));
            assert.equal(
              await details.getByText('No completed backup found', { exact: true }).count(),
              0,
            );
            assert.equal(await details.getByText('Today', { exact: true }).count(), 0);
            assert.equal(
              await details.getByText('Running · not completed yet', { exact: true }).count(),
              running ? 1 : 0,
            );
            assert(
              !(await details.innerText()).includes('NaN') &&
                !(await details.innerText()).includes('Invalid Date'),
            );
            checks.push(`${message} running=${running}: row, indicator, drawer and identity`);
          };

          assert((await badge.getAttribute('class')).includes('green'));
          await details.getByText('Today', { exact: true }).waitFor();
          checks.push('valid initial completion retains fresh policy');
          await observe(future);
          await checkUnknown(futureMessage, false);
          if (transport === 'api' && kind === 'qemu') await capture('future');
          await observe(future, true);
          await checkUnknown(futureMessage, true);
          await observe('not-a-date', true);
          await checkUnknown(invalidMessage, true);
          if (name === 'chromium-desktop') {
            await badge.hover();
            const tooltip = page.locator('[data-tooltip-portal="true"]');
            await tooltip.waitFor({ state: 'visible' });
            assert((await tooltip.innerText()).includes(invalidMessage));
            assert(
              !(await tooltip.innerText()).includes('No backup has ever') &&
                !(await tooltip.innerText()).includes('Invalid Date'),
            );
            await page.locator('h1').hover();
            checks.push(
              'hover tooltip preserves timestamp uncertainty without invalid date or absence claim',
            );
          }
          if (transport === 'api' && kind === 'qemu') await capture('invalid-running');
          await observe('not-a-date');
          await checkUnknown(invalidMessage, false);
          await observe('0002-01-01T00:00:00Z');
          await checkUnknown(invalidMessage, false);
          for (const timestamp of [
            undefined,
            '',
            '0001-01-01T00:00:00Z',
            '0001-01-01T00:00:00.000000000Z',
          ]) {
            await observe(timestamp);
            assert.equal((await badge.innerText()).trim(), 'None');
            assert((await badge.getAttribute('class')).includes('red'));
            await details.getByText('No completed backup found', { exact: true }).waitFor();
            checks.push(`explicit absence ${timestamp}: missing, not unknown`);
          }
          if (transport === 'api' && kind === 'lxc') await capture('missing');
          await observe('2026-09-24T08:00:00Z', true);
          await details.getByText('10d ago', { exact: true }).waitFor();
          assert.equal((await badge.innerText()).trim(), 'Running');
          assert((await badge.getAttribute('aria-label')).includes('last completed backup'));
          checks.push('valid overdue completion retained independently of running activity');
          await observe(fresh);
          assert((await badge.getAttribute('class')).includes('green'));
          assert((await indicator.getAttribute('class')).includes('green'));
          await details.getByText('Today', { exact: true }).waitFor();
          assert(!(await details.innerText()).includes('unavailable'));
          const close = page.getByRole('button', { name: 'Collapse backup-guest details' });
          if (width === 390) await close.tap();
          else {
            await close.focus();
            await page.keyboard.press('Enter');
          }
          assert.equal(await page.evaluate(() => window.__backupTimeEvidence.closed()), 1);
          assert(
            await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth),
          );
          checks.push(
            'valid recovery, same mounted surfaces, keyboard/touch collapse and no page overflow',
          );
          if (transport === 'api' && kind === 'qemu') await capture('recovered');
          assert.deepEqual(errors, []);
          assert.deepEqual(writes, []);
          assert.equal(
            resourceReads > 0,
            transport === 'api',
            'canonical path makes no parallel workload request',
          );
          results.push({
            name,
            browserVersion: browser.version(),
            transport,
            kind,
            identity,
            resourceReads,
            checks,
            errors,
            writes,
          });
          await page.close();
        }
      }
      await browser.close();
      browser = undefined;
    }
    fs.writeFileSync(
      path.join(output, 'result.json'),
      JSON.stringify(
        { result: 'passed', playwrightVersion, runtime: hashes, results, screenshots },
        null,
        2,
      ),
    );
  } catch (error) {
    fs.writeFileSync(
      path.join(output, 'failure.json'),
      JSON.stringify({ phase, error: error.message, results, screenshots }, null, 2),
    );
    throw error;
  } finally {
    if (browser) await browser.close();
    await server.close();
  }
})().catch((error) => {
  process.stderr.write(`${error.stack}\n`);
  process.exitCode = 1;
});
