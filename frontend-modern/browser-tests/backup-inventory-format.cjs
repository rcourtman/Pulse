// Production backup readers/table/router/CSS, synthetic HTTP only.
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const crypto = require('node:crypto');
const { chromium, webkit } = require('playwright');
const phase = 'final';
const root = '/workspace/frontend-modern';
const output = `/workspace/tmp/backup-format-${phase}`;
const sha256 = (file) => crypto.createHash('sha256').update(fs.readFileSync(file)).digest('hex');
const emptyPVE = { data: { backupTasks: [], storageBackups: [], guestSnapshots: [] } };
const emptyPBS = { data: { backups: [] } };
const archive = {
  id: 'archive-112',
  instance: 'pve-a',
  node: 'pve-a',
  storage: 'local',
  type: 'ct',
  vmid: 112,
  time: '2026-10-03T01:00:00Z',
  format: 'zst',
  volid: 'local:backup/vzdump-lxc-112.tar.zst',
};
const snapshot = {
  id: 'pbs-main/main/ct/112/2026-10-03T02:00:00Z',
  instance: 'pbs-main',
  datastore: 'main',
  backupType: 'ct',
  vmid: '112',
  backupTime: '2026-10-03T02:00:00Z',
  files: ['index.json.blob'],
};
const pvePayload = { data: { ...emptyPVE.data, storageBackups: [archive] } };
const pbsPayload = { data: { backups: [snapshot] } };

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
    server: { host: '127.0.0.1', port: 5298, strictPort: true },
  });
  const observations = [];
  let browser;
  try {
    await server.listen();
    for (const [name, engine, phone] of [
      ['chromium-desktop', chromium, false],
      ['webkit-phone', webkit, true],
    ]) {
      browser = await engine.launch(
        engine === chromium
          ? { headless: true, channel: 'chromium', args: ['--no-sandbox'] }
          : { headless: true },
      );
      const viewport = phone ? { width: 390, height: 844 } : { width: 1365, height: 900 };
      const page = await browser.newPage({
        viewport,
        isMobile: phone,
        hasTouch: phone,
        locale: 'en-GB',
        timezoneId: 'UTC',
      });
      page.setDefaultTimeout(20000);
      await page.clock.setFixedTime(new Date('2026-10-03T12:00:00Z'));
      const errors = [],
        requests = [],
        offOrigin = [],
        screenshots = [],
        checks = [];
      page.on('pageerror', (error) => errors.push(error.message));
      const sources = { pve: emptyPVE, pbs: {} };
      let retryGate;
      await page.route('**/*', async (route) => {
        const request = route.request();
        const url = new URL(request.url());
        if (url.origin !== 'http://127.0.0.1:5298') {
          offOrigin.push(url.origin);
          return route.abort();
        }
        if (!url.pathname.startsWith('/api/')) return route.continue();
        requests.push({ path: url.pathname, method: request.method() });
        const key =
          url.pathname === '/api/backups/pbs'
            ? 'pbs'
            : url.pathname === '/api/backups/pve'
              ? 'pve'
              : null;
        const payload = key ? sources[key] : { data: [], policy: {}, meta: {} };
        if (key === 'pbs' && retryGate) await retryGate;
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify(payload),
        });
      });
      const rows = () => page.locator('[data-proxmox-backup-row="recoverable"]');
      const notice = (source) =>
        page.getByRole('status').filter({
          hasText: `${source} backup inventory is unavailable`,
        });
      const load = async () => {
        await page.goto('http://127.0.0.1:5298/browser-tests/backup-inventory.html?view=date', {
          waitUntil: 'domcontentloaded',
        });
        if (phone) await page.evaluate(() => document.documentElement.classList.add('dark'));
      };
      const capture = async (suffix) => {
        await page.evaluate(() =>
          Promise.all(
            document
              .getAnimations()
              .filter((animation) => animation.effect?.getTiming().iterations !== Infinity)
              .map((animation) => animation.finished.catch(() => undefined)),
          ),
        );
        const file = path.join(output, `${name}-${suffix}.png`);
        await page.screenshot({ path: file, fullPage: true });
        screenshots.push({ path: path.relative('/workspace', file), sha256: sha256(file) });
      };

      await load();
      if (phase === 'baseline') {
        await page.getByText('No backups yet', { exact: true }).waitFor();
        assert.equal(await notice('PBS').count(), 0);
        checks.push('HTTP200 missing PBS data is incorrectly shown as No backups yet');
      } else {
        await notice('PBS').waitFor();
        await page.getByText('Backup inventory is incomplete', { exact: true }).waitFor();
        assert.equal(await page.getByText('No backups yet', { exact: true }).count(), 0);
        await page.getByText(/The response format is invalid/).waitFor();
        if (!phone) await page.locator('[title="PBS backup inventory is unavailable"]').waitFor();
        checks.push(
          'HTTP200 missing data remains incomplete with a fixed format explanation, not empty/zero',
        );
      }
      await capture('missing-data');

      sources.pve = pvePayload;
      sources.pbs = { data: { backups: {} }, message: 'SYNTHETIC_PRIVATE_BODY_DO_NOT_DISPLAY' };
      await load();
      if (phase === 'baseline') {
        await page.getByRole('alert').filter({ hasText: 'Backup view failed' }).waitFor();
        assert.equal(await rows().count(), 0);
        checks.push(
          'HTTP200 non-array PBS inventory removes independently readable PVE evidence at ErrorBoundary',
        );
      } else {
        await notice('PBS').waitFor();
        await rows().first().waitFor();
        assert.equal(await rows().count(), 1);
        assert.match(await rows().first().innerText(), /PVE file/);
        assert.equal(await page.getByRole('alert').count(), 0);
        assert.equal(await page.getByText('SYNTHETIC_PRIVATE_BODY_DO_NOT_DISPLAY').count(), 0);
        checks.push(
          'Malformed PBS is source-local; readable PVE remains and raw response content is not displayed',
        );
      }
      await capture('pbs-invalid');

      sources.pbs = pbsPayload;
      sources.pve = { data: { ...emptyPVE.data, backupTasks: {} } };
      await load();
      if (phase === 'baseline') {
        await page.getByRole('alert').filter({ hasText: 'Backup view failed' }).waitFor();
        assert.equal(await rows().count(), 0);
        checks.push(
          'HTTP200 non-array PVE tasks removes independently readable PBS evidence at ErrorBoundary',
        );
      } else {
        await notice('PVE').waitFor();
        await rows().first().waitFor();
        assert.equal(await rows().count(), 1);
        await rows().first().getByText('PBS', { exact: true }).waitFor();
        await rows().first().getByText('Snapshot', { exact: true }).waitFor();
        assert.equal(await page.getByRole('alert').count(), 0);
        checks.push('Malformed PVE is source-local and independent PBS evidence remains usable');
      }
      await capture('pve-invalid');

      if (phase === 'final') {
        sources.pve = pvePayload;
        sources.pbs = { data: { backups: [{ ...snapshot, inProgress: 'true' }] } };
        await load();
        await notice('PBS').waitFor();
        await rows().first().waitFor();
        assert.equal(
          await rows().count(),
          1,
          'a mistyped completion flag must not become recovery evidence',
        );
        const before = requests.filter((r) => r.path === '/api/backups/pve').length;
        sources.pbs = pbsPayload;
        let release;
        retryGate = new Promise((resolve) => (release = resolve));
        const retry = page.getByRole('button', { name: 'Retry PBS inventory', exact: true });
        if (phone) await retry.tap();
        else {
          await retry.focus();
          await page.keyboard.press('Enter');
        }
        assert.equal(await retry.isDisabled(), true);
        await notice('PBS').waitFor();
        assert.equal(await rows().count(), 1);
        release();
        await page.waitForFunction(
          () => document.querySelectorAll('[data-proxmox-backup-row="recoverable"]').length === 2,
        );
        retryGate = null;
        assert.equal(await notice('PBS').count(), 0);
        assert.equal(requests.filter((r) => r.path === '/api/backups/pve').length, before);
        await capture('recovered');
        checks.push(
          'Mistyped incomplete flag is rejected; warning persists through keyboard/touch isolated retry until valid recovery',
        );

        sources.pve = { data: { backupTasks: null, storageBackups: null, guestSnapshots: null } };
        sources.pbs = emptyPBS;
        await load();
        await page.getByText('No backups yet', { exact: true }).waitFor();
        assert.equal(await page.getByRole('status').filter({ hasText: 'inventory' }).count(), 0);
        checks.push('Explicit Go nil PVE collections remain a valid empty observation');
      }
      const overflow = await page.evaluate(
        () => document.documentElement.scrollWidth > window.innerWidth + 1,
      );
      assert.equal(overflow, false);
      assert.deepEqual(errors, []);
      assert.deepEqual(offOrigin, []);
      assert(requests.every((r) => r.method === 'GET'));
      observations.push({
        name,
        browserVersion: browser.version(),
        viewport,
        checks,
        requests,
        errors,
        offOrigin,
        overflow,
        screenshots,
      });
      await browser.close();
      browser = null;
    }
    const runtime = [
      'src/features/proxmox/ProxmoxBackupsTable.tsx',
      'src/features/proxmox/proxmoxBackupInventory.ts',
    ].filter((p) => fs.existsSync(path.join(root, p)));
    fs.writeFileSync(
      path.join(output, 'result.json'),
      JSON.stringify(
        {
          phase,
          playwrightVersion,
          runtimeSha256: Object.fromEntries(runtime.map((p) => [p, sha256(path.join(root, p))])),
          observations,
        },
        null,
        2,
      ) + '\n',
    );
    console.log(`${phase}: ${observations.length} browser contexts passed; receipts at ${output}`);
  } finally {
    if (browser) await browser.close();
    await server.close();
  }
})().catch((error) => {
  console.error(error);
  process.exitCode = 1;
});
