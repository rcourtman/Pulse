// Real inventory parsers, recovery model, Coverage table, router and CSS.
// Fixture-only HTTP: no backup/guest operation or native recovery assertion.
process.env.RAYON_NUM_THREADS = '2';
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const crypto = require('node:crypto');
const { chromium, webkit } = require('playwright');
const root = '/workspace/frontend-modern';
const output = '/workspace/tmp/backup-task-chronology/final';
const origin = 'http://127.0.0.1:5321';
const hash = (file) => crypto.createHash('sha256').update(fs.readFileSync(file)).digest('hex');
const task = { vmid: 112, type: 'ct', node: 'pve-a', instance: 'pve-a' };
const known = { ...task, id: 'known-ok', status: 'OK', startTime: '2026-10-16T10:00:00Z' };
const uncertain = {
  ...task,
  id: 'undated-failure',
  status: 'failed',
  startTime: 'not-a-date',
  error: 'Synthetic backup failure',
};
const posture = {
  subjectResourceId: 'ct-112',
  state: 'unknown',
  freshness: 'unknown',
  verification: 'unknown',
  coverage: 'unknown',
  providerStates: [],
  repositoryResourceIds: [],
  evidenceIds: [],
  explanation: 'Synthetic provider posture stays independent of the task summary.',
  evaluatedAt: '2026-10-16T11:00:00Z',
};
const runtime = [
  'src/features/proxmox/proxmoxBackupRecoveryModel.ts',
  'src/features/proxmox/ProxmoxCoverageTable.tsx',
  'src/features/proxmox/ProxmoxBackupsTable.tsx',
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
    server: { host: '127.0.0.1', port: 5321, strictPort: true },
  });
  const observations = [];
  let browser;
  try {
    await server.listen();
    for (const [name, engine, width] of [
      ['chromium-desktop', chromium, 1365],
      ['webkit-phone', webkit, 320],
    ]) {
      const phone = width === 320;
      browser = await engine.launch(
        engine === chromium
          ? { headless: true, channel: 'chromium', args: ['--no-sandbox'] }
          : { headless: true },
      );
      const page = await browser.newPage({
        viewport: { width, height: 900 },
        hasTouch: phone,
        isMobile: phone,
        timezoneId: 'UTC',
        locale: 'en-GB',
      });
      page.setDefaultTimeout(15000);
      let tasks = [known, uncertain];
      const requests = [],
        errors = [],
        checks = [],
        captures = [];
      page.on('pageerror', (error) => errors.push(error.message));
      await page.clock.setFixedTime(new Date('2026-10-16T11:00:00Z'));
      await page.route('**/*', async (route) => {
        const request = route.request(),
          url = new URL(request.url());
        assert.equal(url.origin, origin);
        if (!url.pathname.startsWith('/api/')) return route.continue();
        assert.equal(request.method(), 'GET');
        assert(
          ['/api/backups/pve', '/api/backups/pbs', '/api/recovery/postures'].includes(url.pathname),
        );
        requests.push({ path: url.pathname, method: request.method() });
        const body =
          url.pathname === '/api/backups/pve'
            ? { data: { backupTasks: tasks, storageBackups: [], guestSnapshots: [] } }
            : url.pathname === '/api/backups/pbs'
              ? { data: { backups: [] } }
              : {
                  data: [posture],
                  policy: {
                    freshnessWindowSeconds: 604800,
                    verificationWindowSeconds: 604800,
                    requireVerification: true,
                  },
                  meta: { total: 1, totalPages: 1 },
                };
        return route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify(body),
        });
      });
      await page.goto(origin + '/browser-tests/backup-inventory.html?view=coverage', {
        waitUntil: 'domcontentloaded',
        timeout: 90000,
      });
      if (phone) await page.evaluate(() => document.documentElement.classList.add('dark'));
      await page.getByRole('link', { name: 'Coverage', exact: true }).click();
      const row = page.locator('[data-proxmox-backup-row="coverage"]').first();
      await row.waitFor();
      const unknown = row.locator('[data-backup-task-unknown]');
      const refreshButton = page.getByRole('button', { name: 'Refresh backups', exact: true });
      const refresh = async (next) => {
        tasks = next;
        await page.waitForFunction(() =>
          document
            .querySelector('button[aria-disabled="false"]')
            ?.textContent.includes('Refresh backups'),
        );
        const response = page.waitForResponse(
          (r) => new URL(r.url()).pathname === '/api/backups/pve',
        );
        if (phone) await refreshButton.tap();
        else await refreshButton.click();
        await response;
        await page.waitForFunction(() =>
          document
            .querySelector('button[aria-disabled="false"]')
            ?.textContent.includes('Refresh backups'),
        );
      };
      const taskCell = async () => {
        const headers = await page
          .locator('[data-proxmox-backups-table="coverage"] table')
          .first()
          .locator('thead th')
          .allTextContents();
        const index = headers.findIndex((s) => /^(Task|Job)$/.test(s.trim()));
        assert(index >= 0, headers.join(' | '));
        return row.locator('td').nth(index);
      };
      const assertUnknown = async () => {
        await unknown.waitFor();
        assert.equal(await unknown.textContent(), phone ? '?' : 'Unknown');
        assert.equal(await unknown.getAttribute('aria-label'), 'Latest backup job unknown');
        assert.match(await unknown.getAttribute('title'), /unavailable or in the future/);
        assert(!String(await (await taskCell()).textContent()).includes('OK'));
        assert.equal(await (await taskCell()).locator('.bg-emerald-500').count(), 0);
      };
      const capture = async (state) => {
        const file = path.join(output, `${name}-${state}.png`);
        await page.screenshot({ path: file, fullPage: true, animations: 'disabled' });
        captures.push({ state, path: file, sha256: hash(file) });
      };
      const assertFits = async () => {
        const geometry = await page
          .locator('[data-backup-task-unknown], [data-backup-task-chronology]')
          .evaluateAll((nodes) =>
            nodes.map((node) => {
              const text = document.createRange();
              text.selectNodeContents(node);
              const box = node.getBoundingClientRect(),
                content = text.getBoundingClientRect();
              return {
                text: node.textContent,
                left: box.left,
                right: box.right,
                textLeft: content.left,
                textRight: content.right,
                scroll: node.scrollWidth,
                width: node.clientWidth,
              };
            }),
          );
        for (const g of geometry) {
          assert(g.scroll <= g.width + 1, JSON.stringify(g));
          assert(g.textLeft >= g.left - 1 && g.textRight <= g.right + 1, JSON.stringify(g));
        }
        assert.equal(
          await page.evaluate(() => document.documentElement.scrollWidth > innerWidth + 1),
          false,
        );
        return geometry;
      };
      await assertUnknown();
      await capture('uncertain-collapsed');
      checks.push('A known OK plus undated failed job is unknown, never a green success');
      const toggle = row.getByRole('button', { name: /details for backup-guest/ });
      if (phone) await row.locator('td').first().tap();
      else {
        await toggle.focus();
        await page.keyboard.press('Enter');
      }
      const explanation = page.locator('[data-backup-task-chronology]');
      await explanation.waitFor();
      assert.match(await explanation.textContent(), /Check the job history in Proxmox/);
      assert.equal(await toggle.getAttribute('aria-expanded'), 'true');
      checks.push({
        check: 'Disclosure explains uncertainty with no clipped or overflowing text',
        geometry: await assertFits(),
      });
      await capture('uncertain-expanded');
      const rowIdentity = await row.evaluate((node) => ((window.__taskChronologyRow = node), true));
      assert(rowIdentity);
      await refresh([uncertain]);
      await assertUnknown();
      checks.push('Undated-only history retains the job column');
      await refresh([{ ...uncertain, startTime: '2026-10-17T10:30:00Z' }, known]);
      await assertUnknown();
      checks.push('Future failed job plus known OK is unknown in reverse order');
      await refresh([known, { ...uncertain, startTime: '2026-10-16T10:30:00Z' }]);
      const job = await taskCell();
      await page.waitForFunction(() => !document.querySelector('[data-backup-task-unknown]'));
      assert.match(await job.textContent(), /Failed/);
      assert.equal(await explanation.count(), 0);
      assert.equal(await row.evaluate((node) => window.__taskChronologyRow === node), true);
      assert.equal(await toggle.getAttribute('aria-expanded'), 'true');
      assert.match(await row.textContent(), /Unknown/); // canonical provider posture unchanged
      await capture('dated-failure');
      checks.push(
        'Same-open read-only refresh restores the actual latest failure; disclosure/row identity and provider posture stay unchanged',
      );
      await refresh([]);
      const headers = await page
        .locator('[data-proxmox-backups-table="coverage"] table')
        .first()
        .locator('thead th')
        .allTextContents();
      assert(!headers.some((s) => /^(Task|Job)$/.test(s.trim())));
      await refresh([known]);
      await page.waitForFunction(() =>
        [...document.querySelectorAll('[data-proxmox-backup-row="coverage"] td')].some((n) =>
          n.textContent.includes('OK'),
        ),
      );
      assert.match(await (await taskCell()).textContent(), /OK/);
      checks.push('Only genuinely empty history hides the job column; a known OK job remains OK');
      await assertFits();
      assert.deepEqual(errors, []);
      observations.push({
        name,
        viewport: { width, height: 900 },
        browser: browser.version(),
        checks,
        captures,
        requests,
        errors,
      });
      await browser.close();
      browser = null;
    }
    fs.writeFileSync(
      path.join(output, 'RESULT.json'),
      JSON.stringify(
        {
          result: 'passed',
          playwrightVersion,
          runtime_sha256: Object.fromEntries(
            runtime.map((file) => ['frontend-modern/' + file, hash(path.join(root, file))]),
          ),
          script_sha256: hash(
            '/workspace/frontend-modern/browser-tests/backup-task-chronology.cjs',
          ),
          observations,
          limits:
            'Synthetic HTTP through real consuming code. No appliance, guest, restore, installed product or release acceptance.',
        },
        null,
        2,
      ) + '\n',
    );
  } finally {
    if (browser) await browser.close();
    await server.close();
  }
})().catch((error) => {
  console.error(error);
  process.exitCode = 1;
});
