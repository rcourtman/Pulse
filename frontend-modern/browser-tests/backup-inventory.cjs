// Production backup page, router, API adapters and CSS; synthetic HTTP only.
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const crypto = require('node:crypto');
const { chromium, webkit } = require('playwright');
const sha256 = (p) => crypto.createHash('sha256').update(fs.readFileSync(p)).digest('hex');
const root = '/workspace/frontend-modern';
const output = '/workspace/tmp/backup-inventory-proof';
const emptyPVE = { data: { storageBackups: [], guestSnapshots: [], backupTasks: [] } };
const emptyPBS = { data: { backups: [] } };
const pvePayload = {
  data: {
    ...emptyPVE.data,
    storageBackups: [
      {
        id: 'archive-112',
        instance: 'pve-a',
        node: 'pve-a',
        storage: 'local',
        type: 'ct',
        vmid: 112,
        time: '2026-10-03T01:00:00Z',
        format: 'zst',
        volid: 'local:backup/vzdump-lxc-112.tar.zst',
      },
    ],
  },
};
const pbsPayload = {
  data: {
    backups: [
      {
        id: 'pbs-main/main/ct/112/2026-10-03T02:00:00Z',
        instance: 'pbs-main',
        datastore: 'main',
        backupType: 'ct',
        vmid: '112',
        backupTime: '2026-10-03T02:00:00Z',
        files: ['index.json.blob'],
      },
    ],
  },
};
const reply = (body, status = 200) => ({ body, status });
const held = (body, status = 200) => {
  let release;
  const gate = new Promise((resolve) => (release = resolve));
  return { ...reply(body, status), gate, release };
};
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
    server: { host: '127.0.0.1', port: 5297, strictPort: true },
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
      const missing = held({}, 503);
      const sources = { pve: reply(emptyPVE), pbs: missing };
      await page.route('**/*', async (route) => {
        const request = route.request();
        const url = new URL(request.url());
        if (url.origin !== 'http://127.0.0.1:5297') {
          offOrigin.push(url.origin);
          return route.abort();
        }
        // Do not intercept Vite's source modules under /src/api/.
        if (!url.pathname.startsWith('/api/')) return route.continue();
        requests.push({
          path: url.pathname,
          method: request.method(),
          org: request.headers()['x-pulse-org-id'] ?? null,
        });
        const key =
          url.pathname === '/api/backups/pbs'
            ? 'pbs'
            : url.pathname === '/api/backups/pve'
              ? 'pve'
              : null;
        const selected = key ? sources[key] : reply({ data: [], policy: {}, meta: {} });
        if (selected.gate) await selected.gate;
        await route.fulfill({
          status: selected.status,
          contentType: 'application/json',
          body: JSON.stringify(selected.body),
        });
      });
      const rows = () => page.locator('[data-proxmox-backup-row="recoverable"]');
      const notice = (source, status) =>
        page.getByRole('status').filter({ hasText: `${source} backup inventory is ${status}` });
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
      const activate = async (button) => {
        if (phone) await button.tap();
        else {
          await button.focus();
          await page.keyboard.press('Enter');
        }
      };
      const switchOrg = (org) => page.evaluate((value) => window.__switchBackupOrg(value), org);
      await page.goto('http://127.0.0.1:5297/browser-tests/backup-inventory.html?view=date', {
        waitUntil: 'domcontentloaded',
      });
      if (phone) await page.evaluate(() => document.documentElement.classList.add('dark'));
      await notice('PBS', 'loading').waitFor();
      assert.equal(await page.getByText('No backups yet', { exact: true }).count(), 0);
      await page.getByText('Backup inventory is incomplete', { exact: true }).waitFor();
      if (!phone) await page.locator('[title="PBS backup inventory is loading"]').waitFor();
      await capture('pending');
      missing.release();
      await notice('PBS', 'unavailable').waitFor();
      assert.equal(await page.getByRole('alert').count(), 0);
      const style = await notice('PBS', 'unavailable').evaluate((node) => {
        const rect = node.getBoundingClientRect();
        return {
          color: getComputedStyle(node).color,
          background: getComputedStyle(node).backgroundColor,
          width: rect.width,
          scrollWidth: node.scrollWidth,
          clientWidth: node.clientWidth,
        };
      });
      assert.equal(style.scrollWidth <= style.clientWidth + 1, true);
      assert(
        !style.color.startsWith('rgba') || style.color.endsWith(', 1)'),
        'warning text must be opaque',
      );
      const retry = page.getByRole('button', { name: 'Retry PBS inventory', exact: true });
      const recovering = held(pbsPayload);
      sources.pbs = recovering;
      await activate(retry);
      assert.equal(await retry.isDisabled(), true);
      await notice('PBS', 'unavailable').waitFor();
      await capture('retrying');
      recovering.release();
      await rows().first().waitFor();
      assert.equal(await rows().count(), 1);
      assert.equal(await notice('PBS', 'unavailable').count(), 0);
      assert.equal(
        requests.filter((r) => r.path === '/api/backups/pve').length,
        1,
        'isolated PBS retry does not reread PVE',
      );
      await capture('recovered');
      checks.push(
        'PBS loading never reads empty/zero; 503 remains contained, warning persists through disabled keyboard/touch retry, and success restores its point',
      );

      const newPVE = held(pvePayload);
      sources.pve = newPVE;
      sources.pbs = reply({}, 403);
      await switchOrg('fixture-other-org');
      await notice('PBS', 'unavailable').waitFor();
      assert.equal(await rows().count(), 0, 'org switch withdraws previous inventory');
      assert.equal(await page.getByText('No backups yet', { exact: true }).count(), 0);
      await page.getByText(/Access denied\. Check your permissions/).waitFor();
      newPVE.release();
      await rows().first().waitFor();
      assert.equal(await rows().count(), 1);
      assert.match(await rows().first().innerText(), /PVE file/);
      if (!phone) await page.locator('[title="PBS backup inventory is unavailable"]').waitFor();
      const pveRow = await rows().first().elementHandle();
      sources.pbs = held(pbsPayload);
      await activate(page.getByRole('button', { name: 'Retry PBS inventory', exact: true }));
      await notice('PBS', 'unavailable').waitFor();
      assert.equal(
        await page.evaluate(
          (node) => node === document.querySelector('[data-proxmox-backup-row="recoverable"]'),
          pveRow,
        ),
        true,
        'isolated retry preserves readable row DOM',
      );
      sources.pbs.release();
      await page.waitForFunction(
        () => document.querySelectorAll('[data-proxmox-backup-row="recoverable"]').length === 2,
      );
      checks.push(
        '403 removes old PBS points/counts while PVE evidence survives; only success restores PBS, without remounting the independent row',
      );

      sources.pve = reply({}, 500);
      sources.pbs = reply({}, 503);
      await switchOrg('fixture-final-org');
      await page.getByText('Could not load Proxmox backup inventory', { exact: true }).waitFor();
      assert.equal(await rows().count(), 0);
      const finalPVE = held(pvePayload);
      sources.pve = finalPVE;
      sources.pbs = reply(pbsPayload);
      await activate(page.getByRole('button', { name: 'Refresh', exact: true }));
      await notice('PVE', 'unavailable').waitFor();
      assert.equal(await rows().count(), 1, 'PBS recovery is independent of a pending PVE retry');
      finalPVE.release();
      await page.waitForFunction(
        () => document.querySelectorAll('[data-proxmox-backup-row="recoverable"]').length === 2,
      );
      assert.equal(
        await page.getByRole('status').filter({ hasText: 'Counts are incomplete' }).count(),
        0,
      );
      await activate(page.getByRole('link', { name: 'Coverage', exact: true }));
      await page.locator('[data-proxmox-backups-table="coverage"]').waitFor();
      assert.equal(
        await page
          .getByRole('link', { name: 'Coverage', exact: true })
          .getAttribute('aria-current'),
        'page',
      );
      await capture('coverage');
      assert.equal(
        await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth + 1),
        true,
      );
      assert.equal(await page.evaluate(() => window.innerWidth), viewport.width);
      for (const org of ['fixture-other-org', 'fixture-final-org']) {
        assert(requests.some((r) => r.path === '/api/backups/pbs' && r.org === org));
        assert(requests.some((r) => r.path === '/api/backups/pve' && r.org === org));
      }
      assert(requests.every((r) => r.method === 'GET'));
      assert.deepEqual(offOrigin, []);
      assert.deepEqual(errors, []);
      checks.push(
        'both failed inventories retry, first recovered source renders immediately, healthy Coverage navigation remains usable, GET-only new organisation headers and no page overflow/errors',
      );
      observations.push({
        name,
        viewport,
        browser_version: browser.version(),
        style,
        requests,
        checks,
        screenshots,
      });
      await page.close();
      await browser.close();
      browser = null;
    }
    const paths = [
      'frontend-modern/src/features/proxmox/ProxmoxBackupsTable.tsx',
      'frontend-modern/src/features/proxmox/ProxmoxBackupServersTable.tsx',
    ];
    fs.writeFileSync(
      path.join(output, 'result.json'),
      JSON.stringify(
        {
          playwright_version: playwrightVersion,
          content_sha256: Object.fromEntries(
            paths.map((p) => [p, sha256(path.join('/workspace', p))]),
          ),
          observations,
          limits: [
            'Synthetic responses and organisation contexts, not native permissions, PBS/PVE monitoring, guest thaw, restore verification or release acceptance.',
            'Browser uses production components and stylesheet through offline Vite; exact production/embed build is separately source-proved.',
          ],
        },
        null,
        2,
      ),
    );
    console.log(
      'PASS: independent backup read states, source retry, 403/org withdrawal, recovered Coverage and desktop/phone warning layout.',
    );
  } finally {
    if (browser) await browser.close();
    await server.close();
  }
})().catch((error) => {
  console.error(error);
  process.exitCode = 1;
});
