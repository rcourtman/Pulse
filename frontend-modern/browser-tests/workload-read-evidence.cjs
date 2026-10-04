const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const crypto = require('node:crypto');
const { chromium, webkit } = require('playwright');
const root = '/workspace/frontend-modern';
const output = '/workspace/tmp/workload-read-evidence';
const origin = 'http://127.0.0.1:5318';
const runtime = [
  'frontend-modern/src/hooks/useWorkloads.ts',
  'frontend-modern/src/components/Workloads/useGuestRowState.ts',
  'frontend-modern/src/utils/resourceStateAdapters.ts',
];
const hash = (p) => crypto.createHash('sha256').update(fs.readFileSync(p)).digest('hex');
const initial = () => ({
  reason: 'prev-vm-locked',
  usage: 50,
  lock: 'backup',
  status: 'deferred',
  expected: true,
});
const resource = (read) => {
  const capacity = 10 * 1024 ** 3;
  return {
    id: 'fixture-a-pve1-101',
    type: 'vm',
    name: 'backup-guest',
    status: 'online',
    sources: ['proxmox'],
    lastSeen: '2026-10-04T05:00:00Z',
    metrics: {
      cpu: { percent: 10 },
      memory: { percent: 25, used: capacity / 4, total: capacity },
      ...(read.usage === undefined
        ? {}
        : { disk: { percent: read.usage, used: (capacity * read.usage) / 100, total: capacity } }),
    },
    proxmox: {
      vmid: 101,
      nodeName: 'pve1',
      instance: 'fixture-a',
      runtimeStatus: 'running',
      guestAgentStatus: read.status,
      disks:
        read.usage === undefined
          ? []
          : [
              {
                mountpoint: '/data',
                filesystem: 'ext4',
                total: capacity,
                used: (capacity * read.usage) / 100,
              },
            ],
      ...(read.reason === undefined ? {} : { diskStatusReason: read.reason }),
      ...(read.lock === undefined ? {} : { lock: read.lock }),
      ...(read.expected === undefined ? {} : { guestAgentExpected: read.expected }),
    },
  };
};
(async () => {
  fs.mkdirSync(output, { recursive: true });
  const result = {
    result: 'incomplete',
    playwright: require('playwright/package.json').version,
    runtime_hashes: Object.fromEntries(runtime.map((p) => [p, hash('/workspace/' + p)])),
    cases: [],
    screenshots: [],
    limits:
      'Real useWorkloads API client and owned snapshot, full/delta/fast canonical merges, production GuestRow and GuestDrawer/CSS. Synthetic read state and local API responses; no native QGA/thaw, workload liveness, installed or physical-phone acceptance, publication or release proof.',
  };
  assert.equal(
    result.playwright,
    JSON.parse(fs.readFileSync('/workspace/tests/integration/package-lock.json')).packages[
      'node_modules/@playwright/test'
    ].version,
  );
  process.chdir(root);
  const { createServer } = await import(path.join(root, 'node_modules/vite/dist/node/index.js'));
  const server = await createServer({
    root,
    configFile: path.join(root, 'vite.config.ts'),
    cacheDir: output + '/cache',
    server: { host: '127.0.0.1', port: 5318, strictPort: true },
  });
  let browser;
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
      for (const transport of ['api', 'canonical-full', 'canonical-delta', 'canonical-fast']) {
        const page = await browser.newPage({
          viewport: { width, height: width < 768 ? 844 : 900 },
          isMobile: width < 768,
          hasTouch: width < 768,
          locale: 'en-GB',
          timezoneId: 'UTC',
        });
        page.setDefaultTimeout(15000);
        page.setDefaultNavigationTimeout(60000);
        let read = initial();
        const record = {
          name,
          transport,
          width,
          dark,
          browser: browser.version(),
          errors: [],
          requests: [],
          states: [],
        };
        result.cases.push(record);
        page.on('pageerror', (e) => record.errors.push(e.message));
        await page.route('**/*', async (route) => {
          const url = new URL(route.request().url());
          if (url.origin !== origin) {
            record.requests.push({ origin: url.origin, blocked: true });
            return route.abort();
          }
          if (!url.pathname.startsWith('/api/')) return route.continue();
          record.requests.push({ path: url.pathname, method: route.request().method() });
          assert.equal(route.request().method(), 'GET');
          if (url.pathname === '/api/resources')
            return route.fulfill({ json: { data: [resource(read)], meta: { totalPages: 1 } } });
          if (url.pathname === '/api/metrics-store/history')
            return route.fulfill({
              json: {
                resourceType: url.searchParams.get('resourceType'),
                resourceId: url.searchParams.get('resourceId'),
                range: url.searchParams.get('range'),
                start: Date.UTC(2026, 9, 4, 2),
                end: Date.UTC(2026, 9, 4, 3),
                metrics: {},
                source: 'store',
              },
            });
          return route.fulfill({
            json:
              url.pathname === '/api/license/runtime-capabilities'
                ? {
                    capabilities: [],
                    limits: [],
                    max_history_days: 7,
                    hosted_mode: false,
                    runtime: { build: 'community' },
                    blocked_capabilities: [],
                  }
                : { success: true, data: [], alerts: [], anomalies: [], config: null },
          });
        });
        await page.addInitScript(
          (isDark) =>
            document.addEventListener('DOMContentLoaded', () => {
              if (isDark) document.documentElement.classList.add('dark');
            }),
          dark,
        );
        await page.goto(
          origin + '/browser-tests/workload-read-evidence.html?transport=' + transport,
        );
        await page.getByRole('region', { name: 'Workload row' }).waitFor();
        const row = page.getByRole('region', { name: 'Workload row' });
        const cell = row.locator('[data-workload-col="disk"]');
        const badge = row.locator('[data-workload-disk-read-status]');
        const drawer = page.getByRole('region', { name: 'Guest details' });
        const activate = (locator) => (width < 768 ? locator.tap() : locator.click());
        const history = drawer.getByRole('tab', { name: 'History', exact: true });
        await activate(history);
        const chart = drawer.locator('[data-history-group="utilization"]');
        await chart.waitFor();
        await page.waitForFunction(() => document.querySelector('button[aria-busy="false"]'));
        await page.evaluate(() => {
          window.__rowIdentity = document.querySelector('[data-workload-col="disk"]');
          window.__chartIdentity = document.querySelector('[data-history-group="utilization"]');
        });
        const evidence = () => page.evaluate(() => window.__workloadReadEvidence.evidence());
        const id = (await evidence()).id;
        const observe = async (value) => {
          read = value;
          await page.evaluate(async (next) => window.__workloadReadEvidence.update(next), value);
          await page.evaluate(async () => {
            await document.fonts.ready;
            await new Promise((r) => requestAnimationFrame(() => requestAnimationFrame(r)));
          });
          assert.equal((await evidence()).id, id);
          assert.equal(
            await page.evaluate(
              () => window.__rowIdentity === document.querySelector('[data-workload-col="disk"]'),
            ),
            true,
          );
          assert.equal(
            await page.evaluate(
              () =>
                window.__chartIdentity ===
                document.querySelector('[data-history-group="utilization"]'),
            ),
            true,
          );
        };
        const check = async (state, reason, retained, usage, lock, agent, expected) => {
          const current = await evidence();
          assert.equal(current.reason, reason);
          assert.equal(current.lock, lock || '');
          assert.equal(current.agent, agent);
          assert.equal(current.expected, expected);
          const trigger = cell.locator('[data-stacked-disk-trigger]');
          assert.equal(await trigger.count(), reason && !retained ? 0 : 1);
          if (!reason || retained) {
            await page.waitForFunction((text) => {
              const value = document.querySelector(
                '[data-workload-col="disk"] [data-animated-number]',
              );
              return (
                value && value.textContent === text && value.getAttribute('aria-label') === text
              );
            }, usage + '%');
          } else {
            assert.equal(await cell.locator('[data-animated-number]').count(), 0);
            assert.equal((await cell.innerText()).includes('50%'), false);
          }
          assert.equal(await badge.count(), reason ? 1 : 0);
          if (reason) {
            assert.match(
              await badge.innerText(),
              retained ? /Last known|Prior/ : /Unavailable|N\/A/,
            );
            assert.match(
              await cell.getAttribute('title'),
              retained ? /Using last known/ : /Guest reads|Guest request/,
            );
          } else assert.equal(await cell.getAttribute('title'), null);
          assert.equal(/lock:/i.test(await row.innerText()), Boolean(lock));
          assert.equal(
            await chart.locator('[data-history-current="disk"]').count(),
            reason ? 0 : 1,
          );
          assert.equal(
            await chart.locator('[data-history-last-known="disk"]').count(),
            retained ? 1 : 0,
          );
          assert.equal(
            await chart.locator('[data-history-deferred="disk"]').count(),
            reason ? 1 : 0,
          );
          if (retained)
            assert.match(
              await chart.locator('[data-history-last-known="disk"]').innerText(),
              new RegExp(usage.toFixed(1) + '%[\\s\\S]*last known'),
            );
          if (!reason)
            assert.match(
              await chart.locator('[data-history-current="disk"]').innerText(),
              new RegExp(usage.toFixed(1) + '%'),
            );
          assert.equal(await chart.locator('path').count(), 0); // no invented trend from the snapshot
          assert.equal(await chart.locator('[data-history-observation]').count(), 0);
          assert.equal(
            await page.evaluate(() => document.documentElement.scrollWidth > innerWidth),
            false,
          );
          record.states.push({ state, evidence: current });
        };
        const screenshot = async (state) => {
          if (!(
            (transport === 'api' && name === 'chromium-desktop') ||
            (transport === 'canonical-fast' && name === 'webkit-phone')
          ))
            return;
          const file = `${name}-${transport}-${state}.png`;
          await page.screenshot({
            path: output + '/' + file,
            fullPage: true,
            animations: 'disabled',
          });
          result.screenshots.push({
            file,
            sha256: hash(output + '/' + file),
            state,
            name,
            transport,
          });
        };
        await check('retained-backup', 'prev-vm-locked', true, 50, 'backup', 'deferred', true);
        await screenshot('retained');
        // No lock on wire does not by itself restore current disk data.
        await observe({ reason: 'prev-agent-busy', usage: 50, status: 'deferred', expected: true });
        await check(
          'lock-cleared-still-deferred',
          'prev-agent-busy',
          true,
          50,
          '',
          'deferred',
          true,
        );
        // Omitted reason/lock with a fresh explicit native outcome supersedes the old fields.
        await observe({ usage: 50, status: 'available' });
        await check('fresh-same-number', undefined, false, 50, '', 'available', undefined);
        await observe({ reason: 'agent-timeout', status: 'deferred', expected: false });
        await check('unavailable', 'agent-timeout', false, 0, '', 'deferred', false);
        assert.match(
          await chart.locator('[data-history-deferred="disk"]').innerText(),
          /Completion is uncertain.*Do not restart the guest agent during a backup/,
        );
        await screenshot('unavailable');
        await observe({ usage: 0, status: 'available' });
        await check('fresh-measured-zero', undefined, false, 0, '', 'available', undefined);
        await screenshot('zero');
        // Existing Overview updates as well; real History tab can be revisited with keyboard/touch.
        await activate(drawer.getByRole('tab', { name: 'Overview', exact: true }));
        assert.equal((await drawer.innerText()).includes('Using last known'), false);
        assert.match(await drawer.innerText(), /0%/);
        await activate(drawer.getByRole('tab', { name: 'History', exact: true }));
        assert.equal(await chart.locator('[data-history-current="disk"]').count(), 1);
        assert.equal(await chart.locator('[data-history-deferred="disk"]').count(), 0);
        const resourceRequests = record.requests.filter((r) => r.path === '/api/resources');
        if (transport !== 'api') assert.equal(resourceRequests.length, 0);
        assert.deepEqual(record.errors, []);
        assert.equal(
          record.requests.some((r) => r.blocked),
          false,
        );
        record.checks = [
          'Native read-state forwarded rather than discarded.',
          'Recovery clears obsolete optional state without remounting row/History.',
          'Lock clearance alone preserves explicit deferral.',
          'Measured zero remains current; snapshot values never become stored points.',
          'Real tab activation, readable narrow layout, no page errors, mutations or external requests.',
          'Owned snapshot never fetches a second workload inventory.',
        ];
        await page.close();
      }
      await browser.close();
      browser = undefined;
    }
    result.result = 'passed';
    result.verified_at = new Date().toISOString();
  } finally {
    fs.writeFileSync(output + '/result.json', JSON.stringify(result, null, 2) + '\n');
    if (browser) await browser.close();
    await server.close();
  }
})().catch((error) => {
  process.stderr.write(error.stack + '\n');
  process.exitCode = 1;
});
