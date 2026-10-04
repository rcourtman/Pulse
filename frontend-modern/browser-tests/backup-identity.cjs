// Offline production-component proof; no live backup, restore or thaw claims.
// Bound preview build pools only; retain ordinary browser process isolation.
process.env.RAYON_NUM_THREADS = '1';
process.env.GOMAXPROCS = '2';
process.env.UV_THREADPOOL_SIZE = '2';
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const crypto = require('node:crypto');
const { chromium, webkit } = require('playwright');
const root = '/workspace/frontend-modern';
const output = '/workspace/frontend-modern/node_modules/backup-identity-browser';
const origin = 'http://127.0.0.1:5324';
const runtime = [
  'frontend-modern/src/features/proxmox/proxmoxBackupRecoveryModel.ts',
  'frontend-modern/src/features/proxmox/ProxmoxBackupsTable.tsx',
];
const digest = (file) => crypto.createHash('sha256').update(fs.readFileSync(file)).digest('hex');
const guest = (id, instance, node, extra = {}) => ({
  id,
  name: id,
  type: 'vm',
  platformId: instance,
  platformType: 'proxmox-pve',
  sourceType: 'api',
  status: 'running',
  lastSeen: Date.parse('2026-10-04T12:00:00Z'),
  proxmox: { vmid: 100, instance, nodeName: node, ...extra },
});
const east = guest('east-100', 'east', 'pve-1');
const west = guest('west-100', 'west', 'pve-10');
const payloads = {
  '/api/backups/pve': {
    data: {
      storageBackups: [
        {
          id: 'file-100',
          instance: 'east',
          node: 'pve-1',
          type: 'vm',
          vmid: 100,
          storage: 'local',
          time: '2026-10-04T11:00:00Z',
          size: 1024,
          format: 'zst',
          protected: false,
        },
      ],
      guestSnapshots: [
        {
          id: 'snapshot-100',
          instance: 'east',
          node: 'pve-1',
          type: 'vm',
          vmid: 100,
          name: 'before-update',
          time: '2026-10-04T11:30:00Z',
          vmstate: false,
        },
      ],
      backupTasks: [
        {
          id: 'task-100',
          instance: 'east',
          node: 'pve-1',
          type: 'vzdump',
          vmid: 100,
          status: 'OK',
          startTime: '2026-10-04T11:00:00Z',
          endTime: '2026-10-04T11:05:00Z',
        },
      ],
    },
  },
  '/api/backups/pbs': {
    data: {
      backups: [
        {
          id: 'pbs-100',
          instance: 'pbs-server',
          datastore: 'main',
          namespace: 'pve-1',
          backupType: 'vm',
          vmid: '100',
          backupTime: '2026-10-04T10:00:00Z',
          files: ['index.json.blob'],
          size: 2048,
          protected: false,
          verified: true,
        },
      ],
    },
  },
};

(async () => {
  fs.mkdirSync(output, { recursive: true });
  process.chdir(root);
  const version = require('playwright/package.json').version;
  assert.equal(
    version,
    JSON.parse(fs.readFileSync('/workspace/tests/integration/package-lock.json')).packages[
      'node_modules/@playwright/test'
    ].version,
  );
  const { createServer } = await import(path.join(root, 'node_modules/vite/dist/node/index.js'));
  const server = await createServer({
    cacheDir: path.join(output, 'vite-cache'),
    optimizeDeps: { entries: ['browser-tests/backup-identity.html'] },
    server: { host: '127.0.0.1', port: 5324, strictPort: true },
  });
  const observations = [];
  let browser;
  try {
    await server.listen();
    for (const [name, engine, width] of [
      ['chromium-desktop', chromium, 1365],
      ['webkit-phone', webkit, 390],
      ['webkit-narrow', webkit, 320],
    ]) {
      browser = await engine.launch(
        engine === chromium
          ? { headless: true, channel: 'chromium', args: ['--no-sandbox'] }
          : { headless: true },
      );
      const phone = engine === webkit;
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
        offOrigin = [],
        requests = [],
        checks = [],
        screenshots = [];
      page.on('pageerror', (error) => errors.push(error.message));
      page.on('console', (message) => {
        if (message.type() === 'error') errors.push(message.text());
      });
      await page.route('**/*', async (route) => {
        const url = new URL(route.request().url());
        if (url.origin !== origin) {
          offOrigin.push(url.origin);
          return route.abort();
        }
        if (!url.pathname.startsWith('/api/')) return route.continue();
        requests.push({ path: url.pathname, method: route.request().method() });
        // Independent provider-owned posture deliberately remains unchanged.
        const payload = payloads[url.pathname] ?? {
          data: ['east-100', 'west-100'].map((id) => ({
            subjectResourceId: id,
            state: 'unprotected',
            freshness: 'unknown',
            verification: 'unknown',
            coverage: 'none',
            providerStates: [],
            repositoryResourceIds: [],
            evidenceIds: [],
            explanation: 'Synthetic server-owned posture.',
            evaluatedAt: '2026-10-04T12:00:00Z',
          })),
          policy: {},
          meta: { total: 2, totalPages: 1 },
        };
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify(payload),
        });
      });
      const rows = (kind) => page.locator(`[data-proxmox-backup-row="${kind}"]`);
      const coverageRow = (name) => rows('coverage').filter({ hasText: name });
      const replace = (workloads) =>
        page.evaluate((next) => window.__replaceBackupIdentityWorkloads(next), workloads);
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
        screenshots.push({ path: file, sha256: digest(file) });
      };
      await page.goto(`${origin}/browser-tests/backup-identity.html?view=coverage`, {
        waitUntil: 'domcontentloaded',
        timeout: 90000,
      });
      if (phone) await page.evaluate(() => document.documentElement.classList.add('dark'));
      await coverageRow('east-100').waitFor();
      // Navigate explicitly so the router owns the actual Coverage route.
      await page.getByRole('link', { name: 'Coverage', exact: true }).click();
      await page.waitForURL('**/proxmox/backups/coverage');
      await coverageRow('east-100')
        .getByText(phone ? 'Latest task: OK' : 'OK', { exact: true })
        .waitFor();
      assert(!(await coverageRow('west-100').textContent()).includes('OK'));
      checks.push('prefix node names: task stays with exact PVE connection');
      const toggle = coverageRow('east-100').getByRole('button');
      await toggle.focus();
      await page.keyboard.press('Enter');
      assert.equal(await toggle.getAttribute('aria-expanded'), 'true');
      const detailID = await toggle.getAttribute('aria-controls');
      const detail = page.locator(`[id="${detailID}"]`);
      await detail.getByText('PVE file', { exact: true }).waitFor();
      assert((await detail.textContent()).includes('PBS'));
      assert((await detail.textContent()).includes('Snapshot'));
      checks.push('keyboard opens all three correctly attributed artifact kinds');
      await replace([east, { ...west, proxmox: { ...west.proxmox, nodeDisplayName: 'pve-1' } }]);
      assert.equal(await toggle.getAttribute('aria-expanded'), 'true');
      assert.equal(await toggle.evaluate((element) => element === document.activeElement), true);
      assert(!(await coverageRow('west-100').textContent()).includes('OK'));
      checks.push(
        'reorder/display rename retains scoped evidence, expansion and focus without new reads',
      );
      await capture('coverage-exact');
      await page.getByRole('link', { name: 'By date', exact: true }).click();
      await page.waitForURL('**/proxmox/backups/date');
      assert.equal(await rows('recoverable').count(), 3);
      assert(
        (await rows('recoverable').allTextContents()).every((row) => row.includes('east-100')),
      );
      await replace([guest('west-100', 'west', 'pve-1'), east]);
      await rows('recoverable').filter({ hasText: 'PBS' }).waitFor();
      const named = (await rows('recoverable').allTextContents()).filter((row) =>
        row.includes('east-100'),
      );
      assert.equal(named.length, 2);
      assert.equal(await rows('recoverable').count(), 3);
      assert((await rows('recoverable').filter({ hasText: 'PBS' }).textContent()).includes('PBS'));
      checks.push(
        'shared native node: ambiguous PBS is retained unnamed; scoped PVE file/snapshot remain named',
      );
      await capture('date-ambiguous');
      await page.getByRole('link', { name: 'Coverage', exact: true }).click();
      await page.getByRole('button', { name: /1 unmatched backup/ }).click();
      await coverageRow('VM 100')
        .getByText(phone ? 'N/A' : 'Not evaluated', { exact: true })
        .waitFor();
      await coverageRow('east-100')
        .getByText(phone ? 'Latest task: OK' : 'OK', { exact: true })
        .waitFor();
      assert(!(await coverageRow('west-100').textContent()).includes('OK'));
      if (phone) await coverageRow('VM 100').locator('td').first().tap();
      else await coverageRow('VM 100').getByRole('button').click();
      const unresolvedToggle = coverageRow('VM 100').getByRole('button');
      const unresolvedID = await unresolvedToggle.getAttribute('aria-controls');
      const unresolved = page.locator(`[id='${unresolvedID}']`);
      await unresolved.getByText('PBS', { exact: true }).waitFor();
      assert(!(await unresolved.textContent()).includes('PVE file'));
      checks.push(
        'unresolved Coverage evidence stays separate and not evaluated; server posture is independent',
      );
      const noticeBounds = await page
        .getByText('No unambiguous inventory match', { exact: true })
        .evaluate((element) => {
          const range = document.createRange();
          range.selectNodeContents(element);
          const rect = range.getBoundingClientRect();
          const owner = element.closest('button').getBoundingClientRect();
          return {
            left: rect.left,
            right: rect.right,
            ownerLeft: owner.left,
            ownerRight: owner.right,
          };
        });
      assert(
        noticeBounds.left >= noticeBounds.ownerLeft &&
          noticeBounds.right <= noticeBounds.ownerRight,
      );
      checks.push('unmatched explanation wraps readably within the disclosure on phones');
      await capture('coverage-unmatched');
      // Return unique native placement: the same model retracts the orphan.
      await replace([west, east]);
      await page.getByRole('button', { name: /1 unmatched backup/ }).waitFor({ state: 'detached' });
      assert.equal(await rows('coverage').count(), 2);
      checks.push(
        'unambiguous same-resource replacement restores attribution and removes orphan without an extra request',
      );
      const overflow = await page.evaluate(
        () => document.documentElement.scrollWidth > window.innerWidth + 1,
      );
      assert.equal(overflow, false);
      assert.equal(requests.filter((request) => request.path === '/api/backups/pve').length, 1);
      assert.equal(requests.filter((request) => request.path === '/api/backups/pbs').length, 1);
      assert(requests.every((request) => request.method === 'GET'));
      assert.deepEqual(errors, []);
      assert.deepEqual(offOrigin, []);
      observations.push({
        noticeBounds,
        name,
        browser: browser.version(),
        viewport,
        checks,
        requests,
        screenshots,
        overflow,
        errors,
        offOrigin,
      });
      fs.writeFileSync(
        path.join(output, 'progress.json'),
        JSON.stringify({ completedContexts: observations }, null, 2),
      );
      await browser.close();
      browser = undefined;
    }
    fs.writeFileSync(
      path.join(output, 'result.json'),
      JSON.stringify(
        {
          result: 'passed',
          playwright: version,
          content_sha256: Object.fromEntries(
            runtime.map((file) => [file, digest(`/workspace/${file}`)]),
          ),
          observations,
        },
        null,
        2,
      ),
    );
    console.log(
      'Passed backup ownership in Chromium desktop and WebKit 390px/320px; synthetic production-component evidence.',
    );
  } finally {
    if (browser) await browser.close();
    await server.close();
  }
})().catch((error) => {
  console.error(error);
  process.exitCode = 1;
});
