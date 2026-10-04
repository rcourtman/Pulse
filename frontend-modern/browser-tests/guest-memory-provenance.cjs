const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const crypto = require('node:crypto');
const { chromium, webkit } = require('playwright');
const root = '/workspace/frontend-modern';
const output = '/workspace/tmp/guest-memory-provenance';
const origin = 'http://127.0.0.1:5329';
const runtime = [
  'frontend-modern/src/types/api.ts',
  'frontend-modern/src/types/resource.ts',
  'frontend-modern/src/utils/memoryObservation.ts',
  'frontend-modern/src/utils/resourceStateAdapters.ts',
  'frontend-modern/src/hooks/useUnifiedResources.ts',
  'frontend-modern/src/hooks/useWorkloads.ts',
  'frontend-modern/src/components/Workloads/guestDrawerModel.ts',
  'frontend-modern/src/components/Workloads/GuestDrawerOverview.tsx',
  'frontend-modern/src/components/Workloads/GuestDrawerHistory.tsx',
];
const hash = (p) => crypto.createHash('sha256').update(fs.readFileSync(p)).digest('hex');
const initial = () => ({
  state: 'last-known',
  source: 'guest-agent-meminfo',
  observedAt: '2026-10-04T14:00:00Z',
  usage: 25,
});
const resource = (read) => {
  const capacity = 4 * 1024 ** 3;
  return {
    id: read.peer ? 'fixture-b-pve1-101' : 'fixture-a-pve1-101',
    type: 'vm',
    name: read.peer ? 'peer-guest' : 'backup-guest',
    status: 'online',
    lastSeen: read.lastSeen ?? '2026-10-04T17:00:00Z',
    sources: ['proxmox', 'agent'],
    platformScopes: ['proxmox-pve'],
    metrics: {
      cpu: { percent: 10 },
      ...(read.state === 'unavailable'
        ? {}
        : {
            memory: {
              percent: read.usage,
              used: (capacity * read.usage) / 100,
              total: capacity,
              ...(read.state === undefined
                ? {}
                : {
                    observation: {
                      state: read.state,
                      source: read.source ?? '',
                      ...(read.observedAt ? { observedAt: read.observedAt } : {}),
                    },
                  }),
            },
          }),
      disk: { percent: 50, total: capacity, used: capacity / 2 },
    },
    proxmox: {
      vmid: 101,
      nodeName: 'pve1',
      instance: read.peer ? 'fixture-b' : 'fixture-a',
      runtimeStatus: 'running',
      guestAgentStatus: 'deferred',
      guestAgentExpected: true,
      diskStatusReason: 'prev-vm-locked',
      lock: 'backup',
      memory: {
        total: capacity,
        usageUnavailable: read.state === 'unavailable',
        observation: { state: 'current', source: 'status-mem', observedAt: '2026-10-04T17:00:00Z' },
      },
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
      'Production drawer/history/model with real raw, API, unified API and canonical full/delta/fast consumers. Synthetic source payloads, History replies and disconnected websocket metadata. No full App, native QGA/thaw, filesystem writes, physical phone, reporter, installed or publication acceptance.',
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
    server: { host: '127.0.0.1', port: 5329, strictPort: true },
  });
  let browser;
  try {
    await server.listen();
    for (const [name, engine, width, dark] of [
      ['desktop-light', chromium, 1365, false],
      ['desktop-dark', chromium, 1365, true],
      ['phone-dark', webkit, 390, true],
      ['narrow-light', webkit, 320, false],
    ]) {
      browser = await engine.launch(
        engine === chromium
          ? { headless: true, channel: 'chromium', args: ['--no-sandbox'] }
          : { headless: true },
      );
      for (const transport of [
        'raw',
        'api',
        'unified-api',
        'canonical-full',
        'canonical-delta',
        'canonical-fast',
      ]) {
        const page = await browser.newPage({
          viewport: { width, height: width < 768 ? 844 : 900 },
          isMobile: width < 768,
          hasTouch: width < 768,
          locale: 'en-GB',
          timezoneId: 'UTC',
        });
        page.setDefaultTimeout(15000);
        page.setDefaultNavigationTimeout(60000);
        let read = initial(),
          mode = 'empty';
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
            record.requests.push({ blocked: true, origin: url.origin });
            return route.abort();
          }
          if (!url.pathname.startsWith('/api/')) return route.continue();
          record.requests.push({ path: url.pathname, method: route.request().method() });
          assert.equal(route.request().method(), 'GET');
          if (url.pathname === '/api/resources')
            return route.fulfill({ json: { data: [resource(read)], meta: { totalPages: 1 } } });
          if (url.pathname === '/api/metrics-store/history') {
            if (mode === 'denied')
              return route.fulfill({ status: 403, json: { error: 'private fixture refusal' } });
            const start = Date.UTC(2026, 9, 4, 14),
              end = Date.UTC(2026, 9, 4, 17),
              point = (timestamp, value) => ({ timestamp, value, min: value, max: value });
            return route.fulfill({
              json: {
                resourceType: url.searchParams.get('resourceType'),
                resourceId: url.searchParams.get('resourceId'),
                range: url.searchParams.get('range'),
                start,
                end,
                metrics:
                  mode === 'stored'
                    ? {
                        memory: [point(start, 20), point(end, 30)],
                        cpu: [point(start, 10), point(end, 15)],
                      }
                    : {},
                source: 'store',
              },
            });
          }
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
          (value) =>
            document.addEventListener('DOMContentLoaded', () => {
              if (value) document.documentElement.classList.add('dark');
            }),
          dark,
        );
        await page.goto(
          origin + '/browser-tests/guest-memory-provenance.html?transport=' + transport,
        );
        const drawer = page.getByRole('region', { name: 'Guest details' });
        await drawer.getByText('Memory reading', { exact: true }).waitFor();
        const overview = drawer.getByRole('tab', { name: 'Overview', exact: true });
        const history = drawer.getByRole('tab', { name: 'History', exact: true });
        const activate = async (control) => {
          if (width < 768) await control.tap();
          else {
            await control.focus();
            await page.keyboard.press('Enter');
          }
        };
        // ObjectDrawerHeader deliberately extends its collapse hit area by
        // four pixels into the surrounding padding. Its region scrollWidth is
        // not a text-clipping measure. Check the actual reading cell and the
        // page viewport instead, including retained and unknown long labels.
        const checkReadingLayout = async (state) => {
          const row = drawer.getByText('Memory reading', { exact: true }).locator('..');
          const cell = row.locator('td').nth(1);
          const layout = await cell.evaluate((element) => {
            const text = element.querySelector('span');
            const bounds = element.getBoundingClientRect();
            const valueBounds = text.getBoundingClientRect();
            const style = getComputedStyle(text);
            return {
              documentWidth: document.documentElement.clientWidth,
              documentScrollWidth: document.documentElement.scrollWidth,
              cell: { left: bounds.left, right: bounds.right },
              value: {
                left: valueBounds.left,
                right: valueBounds.right,
                width: text.clientWidth,
                scrollWidth: text.scrollWidth,
                whiteSpace: style.whiteSpace,
                textOverflow: style.textOverflow,
              },
            };
          });
          (record.layouts ??= []).push({ state, ...layout });
          assert(layout.documentScrollWidth <= layout.documentWidth + 1);
          assert(layout.value.scrollWidth <= layout.value.width + 1);
          assert(layout.value.left >= layout.cell.left - 1);
          assert(layout.value.right <= layout.cell.right + 1);
          assert.equal(layout.value.whiteSpace, 'normal');
          assert.notEqual(layout.value.textOverflow, 'ellipsis');
        };
        const capture = async (state) => {
          if (transport !== 'unified-api') return;
          const file = `${name}-${state}.png`;
          await drawer.screenshot({ path: output + '/' + file });
          result.screenshots.push({
            file,
            sha256: hash(output + '/' + file),
            name,
            transport,
            state,
          });
        };
        const note = async (state) =>
          record.states.push({
            state,
            evidence: await page.evaluate(() => window.__guestMemory.evidence()),
          });
        const update = async (next) => {
          read = next;
          await page.evaluate((value) => window.__guestMemory.update(value), next);
        };
        assert.match(
          await drawer.innerText(),
          /Last known · QEMU guest agent · 2026-10-04 14:00:00 UTC/,
        );
        await checkReadingLayout('retained-overview');
        await capture('retained-overview');
        await activate(history);
        const chart = drawer.locator('[data-history-group="utilization"]');
        await chart.locator('[data-history-last-known="memory"]').waitFor();
        assert.equal(await chart.locator('[data-history-current="memory"]').count(), 0);
        assert.match(
          await chart.locator('[data-history-last-known="memory"]').innerText(),
          /25\.0%.*last known/s,
        );
        assert.match(
          await chart.locator('[data-history-deferred="memory"]').innerText(),
          /14:00:00 UTC/,
        );
        assert.equal(await chart.locator('path').count(), 0);
        const historyCalls = record.requests.filter(
          (r) => r.path === '/api/metrics-store/history',
        ).length;
        await page.evaluate(() => {
          window.__chartIdentity = document.querySelector('[data-history-group="utilization"]');
        });
        await note('retained');
        await capture('retained-history');
        await update({ ...initial(), lastSeen: '2026-10-04T18:00:00Z' });
        assert.doesNotMatch(
          await chart.locator('[data-history-deferred="memory"]').innerText(),
          /18:00:00/,
        );
        await update({
          state: 'current',
          source: 'status-mem',
          observedAt: '2026-10-04T17:00:00Z',
          usage: 35,
        });
        await chart.locator('[data-history-current="memory"]').waitFor();
        assert.match(
          await chart.locator('[data-history-current="memory"]').innerText(),
          /35\.0%.*current/s,
        );
        assert.equal(await chart.locator('[data-history-deferred="memory"]').count(), 0);
        assert.equal(await chart.locator('[data-history-last-known="disk"]').count(), 1);
        await note('independent-current-pve');
        await update({ state: 'unavailable', source: 'unavailable', usage: 0 });
        assert.equal(await chart.locator('[data-history-current="memory"]').count(), 0);
        assert.equal(await chart.locator('[data-history-last-known="memory"]').count(), 0);
        assert.match(await chart.innerText(), /Memory\s*-/);
        await note('unavailable');
        await update({ usage: 40 });
        await chart.locator('[data-history-unknown="memory"]').waitFor();
        assert.match(
          await chart.locator('[data-history-unknown="memory"]').innerText(),
          /40\.0%.*freshness unknown/s,
        );
        assert.equal(await chart.locator('[data-history-current="memory"]').count(), 0);
        assert.equal(await chart.locator('[data-history-last-known="memory"]').count(), 0);
        assert.equal(
          await page.evaluate(
            () =>
              window.__chartIdentity ===
              document.querySelector('[data-history-group="utilization"]'),
          ),
          true,
        );
        assert.equal(
          record.requests.filter((r) => r.path === '/api/metrics-store/history').length,
          historyCalls,
        );
        await activate(overview);
        assert.match(await drawer.innerText(), /Freshness unknown · Unknown source · time unknown/);
        await checkReadingLayout('unknown-overview');
        await capture('unknown-overview');
        await note('legacy-unknown');
        await activate(history);
        await update({ state: 'last-known', source: 'private provider detail', usage: 0 });
        await chart.locator('[data-history-last-known="memory"]').waitFor();
        assert.match(
          await chart.locator('[data-history-last-known="memory"]').innerText(),
          /0\.0%.*last known/s,
        );
        assert.match(
          await chart.locator('[data-history-deferred="memory"]').innerText(),
          /Observation time unknown/,
        );
        assert.doesNotMatch(await drawer.innerText(), /private provider detail/);
        await note('retained-zero-unknown-age');
        mode = 'stored';
        await activate(drawer.getByRole('button', { name: 'Refresh history', exact: true }));
        await page.waitForFunction(
          () => document.querySelectorAll('[data-history-group="utilization"] path').length === 2,
        );
        assert.equal(await chart.locator('[data-history-current="memory"]').count(), 0);
        assert.equal(await chart.locator('[data-history-last-known="memory"]').count(), 0);
        assert.match(await chart.innerText(), /Memory\s*30\.0%/);
        const slider = chart.getByRole('slider');
        await slider.focus();
        await page.keyboard.press('Home');
        assert.match(await slider.getAttribute('aria-valuetext'), /Memory 20\.0%/);
        await note('stored-keyboard-inspection');
        mode = 'denied';
        await activate(drawer.getByRole('button', { name: 'Refresh history', exact: true }));
        await drawer.getByRole('button', { name: 'Retry history', exact: true }).waitFor();
        assert.equal(await drawer.locator('[data-history-deferred]').count(), 0);
        assert.equal(await drawer.locator('[data-history-group]').count(), 0);
        assert.doesNotMatch(await drawer.innerText(), /private fixture refusal/);
        mode = 'empty';
        await activate(drawer.getByRole('button', { name: 'Retry history', exact: true }));
        await chart.locator('[data-history-last-known="memory"]').waitFor();
        await note('denied-and-recovered');
        await update({
          peer: true,
          state: 'current',
          source: 'agent',
          observedAt: '2026-10-04T16:00:00Z',
          usage: 70,
        });
        await drawer.getByText('peer-guest', { exact: true }).waitFor();
        assert.equal(await overview.getAttribute('aria-selected'), 'true');
        assert.match(await drawer.innerText(), /Current · Pulse Agent · 2026-10-04 16:00:00 UTC/);
        assert.doesNotMatch(await drawer.innerText(), /14:00:00 UTC/);
        await note('peer-identity-replacement');
        await checkReadingLayout('peer-overview');
        assert.deepEqual(record.errors, []);
        assert(record.requests.every((r) => !r.blocked && r.method === 'GET'));
        await page.close();
      }
      await browser.close();
      browser = undefined;
    }
    result.result = 'passed';
    fs.writeFileSync(output + '/result.json', JSON.stringify(result, null, 2) + '\n');
    process.stdout.write(
      `Guest memory provenance: ${result.cases.length} transport/layout cases passed.\n`,
    );
  } catch (error) {
    fs.writeFileSync(
      output + '/failure.json',
      JSON.stringify({ error: String(error), stack: error.stack, result }, null, 2) + '\n',
    );
    throw error;
  } finally {
    if (browser) await browser.close();
    await server.close();
  }
})();
