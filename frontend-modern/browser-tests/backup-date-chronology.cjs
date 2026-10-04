// Existing production router, inventory readers, backup tables and CSS.
// Synthetic timestamps and HTTP; no native restore, thaw or published result.
// Bound Tailwind's native preview scanner, rather than size its thread pool
// from the shared host's CPU count. This changes no executor capacity policy.
process.env.RAYON_NUM_THREADS = '2';
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const crypto = require('node:crypto');
const { chromium, webkit } = require('playwright');
const root = '/workspace/frontend-modern';
const output = '/workspace/tmp/backup-dates/browser';
const origin = 'http://127.0.0.1:5316';
const sha256 = (file) => crypto.createHash('sha256').update(fs.readFileSync(file)).digest('hex');
const emptyPVE = { data: { backupTasks: [], storageBackups: [], guestSnapshots: [] } };
const archive = {
  id: 'archive-112',
  vmid: 112,
  type: 'ct',
  instance: 'pve-a',
  node: 'pve-a',
  storage: 'local',
  format: 'zst',
  time: '2026-09-01T01:00:00Z',
  size: 1024,
  protected: false,
};
const pbs = {
  id: 'pbs-112',
  vmid: '112',
  backupType: 'ct',
  instance: 'pbs-main',
  datastore: 'main',
  namespace: '',
  files: ['index.json.blob'],
  backupTime: 'not-a-date',
  size: 1024,
  protected: false,
  verified: true,
};
const posture = {
  subjectResourceId: 'ct-112',
  state: 'protected',
  freshness: 'current',
  verification: 'verified',
  coverage: 'complete',
  evidenceIds: [],
  repositoryResourceIds: [],
  providerStates: [],
  explanation: 'Synthetic provider-owned protection judgment, independent of inventory date.',
  evaluatedAt: '2026-10-04T12:00:00Z',
};
const runtime = [
  'src/features/proxmox/proxmoxBackupRecoveryModel.ts',
  'src/features/proxmox/proxmoxBackupActivityPresentation.ts',
  'src/features/proxmox/proxmoxBackupsTableShared.tsx',
  'src/features/proxmox/ProxmoxCoverageTable.tsx',
  'src/features/proxmox/ProxmoxBackupsTable.tsx',
  'src/features/proxmox/proxmoxBackupsTablePresentation.ts',
];
(async () => {
  fs.mkdirSync(output, { recursive: true });
  process.chdir(root);
  const playwrightVersion = require('playwright/package.json').version;
  assert.equal(
    playwrightVersion,
    JSON.parse(fs.readFileSync('/workspace/tests/integration/package-lock.json')).packages[
      'node_modules/@playwright/test'
    ].version,
  );
  const { createServer } = await import(path.join(root, 'node_modules/vite/dist/node/index.js'));
  const server = await createServer({
    cacheDir: path.join(output, 'vite-cache'),
    optimizeDeps: { entries: ['browser-tests/backup-inventory.html'] },
    server: { host: '127.0.0.1', port: 5316, strictPort: true },
  });
  const observations = [];
  let browser;
  let currentChecks = [];
  try {
    await server.listen();
    for (const [name, engine, width] of [
      ['chromium-desktop', chromium, 1365],
      ['webkit-phone', webkit, 390],
      ['webkit-narrow-phone', webkit, 320],
    ]) {
      const phone = engine === webkit;
      browser = await engine.launch(
        engine === chromium
          ? { headless: true, channel: 'chromium', args: ['--no-sandbox'] }
          : { headless: true },
      );
      const viewport = { width, height: phone ? 844 : 900 };
      const page = await browser.newPage({
        viewport,
        isMobile: phone,
        hasTouch: phone,
        locale: 'en-GB',
        timezoneId: 'UTC',
      });
      page.setDefaultTimeout(20000);
      await page.clock.setFixedTime(new Date('2026-10-04T12:00:00Z'));
      const errors = [],
        requests = [],
        offOrigin = [],
        screenshots = [];
      const checks = (currentChecks = []);
      const sources = {
        pve: {
          data: {
            ...emptyPVE.data,
            storageBackups: [archive],
            backupTasks: [
              {
                id: 'task-112',
                vmid: 112,
                type: 'ct',
                node: 'pve-a',
                instance: 'pve-a',
                status: 'OK',
                startTime: '2026-10-04T11:00:00Z',
                endTime: '2026-10-04T11:05:00Z',
              },
            ],
          },
        },
        pbs: { data: { backups: [pbs] } },
      };
      page.on('pageerror', (error) => errors.push(error.message));
      await page.route('**/*', async (route) => {
        const request = route.request();
        const url = new URL(request.url());
        if (url.origin !== origin) {
          offOrigin.push(url.origin);
          return route.abort();
        }
        if (!url.pathname.startsWith('/api/')) return route.continue();
        requests.push({ path: url.pathname, method: request.method() });
        const payload =
          url.pathname === '/api/backups/pve'
            ? sources.pve
            : url.pathname === '/api/backups/pbs'
              ? sources.pbs
              : { data: [posture], policy: {}, meta: { total: 1, totalPages: 1 } };
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify(payload),
        });
      });
      const rows = (kind) => page.locator(`[data-proxmox-backup-row="${kind}"]`);
      const ageCell = async () => {
        const table = page.locator('[data-proxmox-backups-table="coverage"] table').first();
        const headers = await table.locator('thead th').allTextContents();
        const index = headers.findIndex((text) => /^(Age|Backup|Last backup)$/.test(text.trim()));
        assert(index >= 0, headers.join(' | '));
        return rows('coverage').first().locator('td').nth(index);
      };
      const load = async (view) => {
        await page.goto(`${origin}/browser-tests/backup-inventory.html?view=${view}`, {
          waitUntil: 'domcontentloaded',
          timeout: 90000,
        });
        if (phone) await page.evaluate(() => document.documentElement.classList.add('dark'));
        await rows(view === 'coverage' ? 'coverage' : 'recoverable')
          .first()
          .waitFor();
        if (view === 'coverage') {
          await page.getByRole('link', { name: 'Coverage', exact: true }).click();
          await page.waitForURL('**/proxmox/backups/coverage');
        }
      };
      const capture = async (suffix) => {
        await page.evaluate(() => {
          for (const animation of document.getAnimations()) {
            try {
              animation.finish();
            } catch {}
          }
        });
        const file = path.join(output, `${name}-${suffix}.png`);
        await page.screenshot({ path: file, fullPage: true, animations: 'disabled' });
        screenshots.push({ path: file, sha256: sha256(file) });
      };
      const assertUnknown = async (element) => {
        const unknown = element.getByText('Unknown', { exact: true });
        await unknown.waitFor();
        assert.match(await unknown.getAttribute('class'), /text-amber/);
        assert.match(await unknown.getAttribute('aria-label'), /Unknown age/);
        assert.match(await unknown.getAttribute('title'), /unavailable or in the future/);
        assert(!String(await unknown.getAttribute('title')).includes('not-a-date'));
        const clipping = await unknown.evaluate((text) => {
          const range = document.createRange();
          range.selectNodeContents(text);
          const rect = range.getBoundingClientRect();
          const cell = text.closest('td');
          const bounds = cell.getBoundingClientRect();
          const style = getComputedStyle(cell);
          return {
            left: rect.left,
            right: rect.right,
            availableLeft: bounds.left + parseFloat(style.paddingLeft),
            availableRight: bounds.right - parseFloat(style.paddingRight),
          };
        });
        assert(
          clipping.left >= clipping.availableLeft - 0.1 &&
            clipping.right <= clipping.availableRight + 0.1,
          JSON.stringify(clipping),
        );
      };
      const assertFits = async () => {
        const geometry = await rows('coverage')
          .first()
          .locator('td')
          .evaluateAll((cells) =>
            cells
              .map((cell) => {
                const text = cell.querySelector('span[aria-label^="Unknown age"]');
                if (!text) return null;
                const range = document.createRange();
                range.selectNodeContents(text);
                const rect = range.getBoundingClientRect(),
                  container = cell.getBoundingClientRect();
                return {
                  left: rect.left,
                  right: rect.right,
                  cellLeft: container.left,
                  cellRight: container.right,
                };
              })
              .filter(Boolean),
          );
        for (const box of geometry)
          assert(
            box.left >= box.cellLeft - 1 && box.right <= box.cellRight + 1,
            JSON.stringify(box),
          );
        assert.equal(
          await page.evaluate(() => document.documentElement.scrollWidth > innerWidth + 1),
          false,
        );
        checks.push({
          check: 'Unknown ages fit their existing cells without overlap or page overflow',
          geometry,
        });
      };

      await load('coverage');
      await assertUnknown(await ageCell());
      await rows('coverage')
        .getByText(phone ? 'Prot.' : 'Protected', { exact: true })
        .waitFor();
      await assertFits();
      await capture('mixed-coverage');
      checks.push({
        check:
          'Known older PVE plus undated completed PBS cannot assert newest backup; server posture stays independent',
      });

      const callCount = requests.length;
      if (phone) await page.getByRole('button', { name: /Filters/, exact: false }).tap();
      const filter = page.getByRole('combobox', { name: 'Filter', exact: true });
      const value = await filter
        .locator('option')
        .filter({ hasText: 'Backup location: pve-a / local' })
        .getAttribute('value');
      await filter.selectOption(value);
      await page.waitForFunction(
        () => new URLSearchParams(location.search).get('location') === 'archive:pve-a:local',
      );
      assert.equal(await (await ageCell()).getByText('Unknown', { exact: true }).count(), 0);
      assert.match(await (await ageCell()).innerText(), /33d/);
      assert.equal(requests.length, callCount);
      checks.push({
        check:
          'Location filtering restores known archive age without another read or global date contamination',
      });

      await page.getByRole('button', { name: 'Clear filters', exact: true }).click();
      await assertUnknown(await ageCell());
      await page.clock.setFixedTime(new Date('2026-10-04T12:02:00Z'));
      sources.pbs = { data: { backups: [{ ...pbs, backupTime: '2026-10-04T12:01:00Z' }] } };
      await page.evaluate(() => window.__switchBackupOrg('date-replacement'));
      await page.waitForFunction(() =>
        [...document.querySelectorAll('[data-proxmox-backup-row="coverage"] td')].some(
          (cell) => cell.textContent.trim() === '1m' || cell.textContent.trim() === '1m ago',
        ),
      );
      await (await ageCell()).getByText(/^1m( ago)?$/).waitFor();
      assert.equal(await (await ageCell()).getByText('Unknown', { exact: true }).count(), 0);
      const replacementGap = await rows('coverage')
        .first()
        .evaluate((row) => {
          const ages = [...row.querySelectorAll('td > span')].filter((span) =>
            /^1m( ago)?$/.test(span.textContent.trim()),
          );
          if (ages.length < 2) return null;
          const range1 = document.createRange(),
            range2 = document.createRange();
          range1.selectNodeContents(ages[0]);
          range2.selectNodeContents(ages[1]);
          return range2.getBoundingClientRect().left - range1.getBoundingClientRect().right;
        });
      assert(replacementGap >= 3.5, String(replacementGap));
      await capture('replaced-coverage');
      checks.push({
        check:
          'Later valid inventory replaces uncertainty using its new observation time, not page-mount time',
      });

      await page.clock.setFixedTime(new Date('2026-10-04T12:00:00Z'));
      sources.pve = {
        data: {
          ...emptyPVE.data,
          storageBackups: [{ ...archive, time: 'not-a-date' }],
          guestSnapshots: [
            {
              id: 'guest-snapshot',
              type: 'ct',
              vmid: 112,
              node: 'pve-a',
              instance: 'pve-a',
              time: '0001-01-01T00:00:00Z',
              name: 'local-snapshot',
            },
          ],
        },
      };
      sources.pbs = {
        data: {
          backups: [
            { ...pbs, id: 'known', backupTime: '2026-10-04T11:00:00Z', size: 256 },
            { ...pbs, id: 'future', backupTime: '2026-10-04T12:00:01Z' },
            { ...pbs, id: 'running', inProgress: true, backupTime: '', verified: false },
            {
              ...pbs,
              id: 'failed',
              inProgress: true,
              writeActivityObserved: true,
              writeActive: false,
              backupTime: '',
              verified: false,
            },
          ],
        },
      };
      await load('date');
      await page.waitForFunction(
        () => document.querySelectorAll('[data-proxmox-backup-row="recoverable"]').length === 6,
      );
      await page
        .getByRole('status')
        .filter({ hasText: 'Backup entries with unavailable or future dates remain listed' })
        .waitFor();
      assert.equal(await rows('recoverable').getByText('Unknown', { exact: true }).count(), 5);
      const today = page.getByRole('button', { name: /: 1 backup$/ });
      await today.waitFor();
      await page.getByText('Running', { exact: true }).waitFor();
      await page.getByText('Failed', { exact: true }).waitFor();
      assert.equal(await page.getByText('not-a-date', { exact: true }).count(), 0);
      assert(!(await page.locator('body').innerText()).includes('Invalid Date'));
      const dateGeometry = await rows('recoverable').evaluateAll((rows) =>
        rows.flatMap((row) => {
          const spans = [...row.querySelectorAll('span[aria-label^="Unknown age"]')];
          return spans.map((text) => {
            const range = document.createRange();
            range.selectNodeContents(text);
            const bounds = range.getBoundingClientRect();
            const cell = text.closest('td').getBoundingClientRect();
            return {
              left: bounds.left,
              right: bounds.right,
              cellLeft: cell.left,
              cellRight: cell.right,
            };
          });
        }),
      );
      for (const box of dateGeometry)
        assert(box.left >= box.cellLeft - 1 && box.right <= box.cellRight - 4, JSON.stringify(box));
      const clippedStateLabels = await rows('recoverable').evaluateAll((rows) =>
        rows
          .map((row) => {
            const cell = row.lastElementChild;
            const text = cell.querySelector('span');
            return text && text.scrollWidth > text.clientWidth + 1 ? cell.textContent : null;
          })
          .filter(Boolean),
      );
      assert.deepEqual(clippedStateLabels, []);
      checks.push({
        check:
          'By date unknown-age words and independent state labels remain unclipped at phone widths',
        geometry: dateGeometry,
      });
      await capture('date-evidence');
      checks.push({
        check:
          'PBS/PVE/guest snapshots keep unknown ages, running/failed/verified states and all artifacts; today counts one dated observation only',
      });

      const beforeDay = requests.length;
      if (phone) await today.tap();
      else {
        await today.focus();
        await page.keyboard.press('Enter');
      }
      await page.waitForFunction(
        () =>
          document.querySelectorAll('[data-proxmox-backup-row="recoverable"]').length === 1 &&
          new URLSearchParams(location.search).get('day') === '2026-10-04',
      );
      assert.equal(await rows('recoverable').getByText('Unknown', { exact: true }).count(), 0);
      assert.equal(requests.length, beforeDay);
      await page.getByRole('button', { name: 'Clear date filter', exact: true }).click();
      await page.waitForFunction(
        () => document.querySelectorAll('[data-proxmox-backup-row="recoverable"]').length === 6,
      );
      checks.push({
        check:
          'Keyboard/touch day selection omits unknown/future dates; clearing restores all six without another read',
      });
      assert.deepEqual(errors, []);
      assert.deepEqual(offOrigin, []);
      assert(requests.every((request) => request.method === 'GET'));
      observations.push({
        name,
        browserVersion: browser.version(),
        viewport,
        checks,
        requests,
        errors,
        offOrigin,
        screenshots,
      });
      await browser.close();
      browser = null;
    }
    fs.writeFileSync(
      path.join(output, 'result.json'),
      JSON.stringify(
        {
          result: 'passed',
          playwrightVersion,
          runtimeSha256: Object.fromEntries(
            runtime.map((file) => [`frontend-modern/${file}`, sha256(path.join(root, file))]),
          ),
          observations,
        },
        null,
        2,
      ) + '\n',
    );
    console.log(`${observations.length} browser contexts passed; receipts at ${output}`);
  } catch (error) {
    fs.writeFileSync(
      path.join(output, 'failure.json'),
      JSON.stringify(
        {
          result: 'failed',
          message: error.message,
          stack: error.stack,
          completedContexts: observations,
          currentChecks,
        },
        null,
        2,
      ) + '\n',
    );
    throw error;
  } finally {
    if (browser) await browser.close();
    await server.close();
  }
})().catch((error) => {
  console.error(error);
  process.exitCode = 1;
});
