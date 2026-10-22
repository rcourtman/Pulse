// Browser proof for #2263. Uses the production guest overview and resource hook
// in Vite, with only the network response replaced by bounded fixture data.
const assert = require('node:assert/strict');
const path = require('node:path');
const { chromium } = require('playwright');

const OVERVIEW_MODULE = '/src/components/Storage/DiskDrawerOverview.tsx';

(async () => {
  const root = '/workspace/frontend-modern';
  const artifacts = path.join(root, 'browser-tests');
  process.chdir(root);
  const { createServer } = await import(path.join(root, 'node_modules/vite/dist/node/index.js'));
  const server = await createServer({
    root,
    configFile: path.join(root, 'vite.config.ts'),
    server: { host: '127.0.0.1', port: 5199, strictPort: true },
  });
  let browser;
  try {
    await server.listen();
    browser = await chromium.launch({
      headless: true,
      channel: 'chromium',
      args: ['--no-sandbox'],
    });
    const requests = [];
    // Requests for the disk Overview module, which loads only on expansion.
    const detailRequests = [];
    const historyReads = [];
    const pageErrors = [];
    const disk = {
      id: 'disk-guest-101',
      parentId: 'vm-resource-101',
      type: 'physical_disk',
      name: 'guest-sata',
      displayName: 'Guest SATA',
      platformId: 'lab',
      platformType: 'proxmox-pve',
      sourceType: 'agent',
      status: 'online',
      lastSeen: new Date().toISOString(),
      physicalDisk: {
        devPath: '/dev/sda',
        model: 'Guest SATA',
        diskType: 'sata',
        sizeBytes: 1_000_000_000,
        health: 'PASSED',
        temperature: 45,
        smart: { reallocatedSectors: 0, pendingSectors: 0, udmaCrcErrors: 2 },
      },
    };
    for (const width of [1440, 390]) {
      const page = await browser.newPage({
        viewport: { width, height: width === 390 ? 844 : 900 },
      });
      page.on('pageerror', (error) => pageErrors.push(`${width}: ${error.message}`));
      page.on('requestfailed', (request) =>
        console.error('requestfailed', request.url(), request.failure()),
      );
      page.on('request', (request) => {
        if (new URL(request.url()).pathname === OVERVIEW_MODULE) {
          detailRequests.push({ width, url: request.url() });
        }
      });
      await page.routeWebSocket(/\/ws(?:\?|$)/, () => {});
      await page.route('**/*', (route) => {
        const url = new URL(route.request().url());
        if (url.origin !== 'http://127.0.0.1:5199') return route.abort();
        if (url.pathname === '/api/resources') {
          const parent = url.searchParams.get('parent');
          requests.push({
            width,
            type: url.searchParams.get('type'),
            parent,
            page: url.searchParams.get('page'),
            limit: url.searchParams.get('limit'),
          });
          if (parent === 'vm-resource-error')
            return route.fulfill({ status: 503, json: { error: 'fixture' } });
          return route.fulfill({
            json: {
              resources: [disk, { ...disk, id: 'host-disk', parentId: 'pve-host' }],
              meta: { totalPages: 1 },
            },
          });
        }
        if (url.pathname === '/api/metrics-store/history') {
          const q = url.searchParams;
          historyReads.push({
            width,
            resourceId: q.get('resourceId'),
            metric: q.get('metric'),
            range: q.get('range'),
          });
          const end = Date.now();
          // Disk reads spike inside their buckets; every other series is flat.
          const series = (metric) =>
            [0, 1, 2].map((i) => ({
              timestamp: end - (2 - i) * 300_000,
              value: 40 + i,
              min: metric === 'diskread' ? 10 + i : 40 + i,
              max: metric === 'diskread' ? 90 + i : 40 + i,
            }));
          // The disk Overview reads every series of its one disk in one request.
          return route.fulfill({
            json: {
              resourceType: q.get('resourceType'),
              resourceId: q.get('resourceId'),
              range: q.get('range'),
              start: end - 600_000,
              end,
              source: 'store',
              metrics: Object.fromEntries(
                ['smart_temp', 'smart_reallocated_sectors', 'diskread', 'diskwrite', 'disk'].map(
                  (metric) => [metric, series(metric)],
                ),
              ),
            },
          });
        }
        if (url.pathname.startsWith('/api/')) return route.fulfill({ json: {} });
        return route.continue();
      });
      let releaseDetail;
      await page.route(`**${OVERVIEW_MODULE}*`, async (route) => {
        await new Promise((resolve) => {
          releaseDetail = resolve;
        });
        await route.continue();
      });
      // Disk charts poll every 10-30 seconds; the clock drives that here.
      await page.clock.install();
      await page.goto('http://127.0.0.1:5199/browser-tests/guest-storage-2263.html', {
        waitUntil: 'domcontentloaded',
        timeout: 120_000,
      });
      await page.locator('[data-testid="guest-physical-disks"]').waitFor();
      assert.match(await page.locator('body').innerText(), /Physical Disks & SMART \(1\)/i);
      assert.equal(await page.locator('[data-testid="guest-physical-disk"]').count(), 1);
      assert.match(await page.locator('body').innerText(), /Guest RAID/i);
      assert.match(await page.locator('body').innerText(), /raid1/);
      assert.equal(
        detailRequests.filter((request) => request.width === width).length,
        0,
        'the disk Overview module must not load before disk expansion',
      );
      await page.screenshot({
        path: path.join(artifacts, `guest-storage-2263-collapsed-${width}.png`),
        fullPage: true,
      });
      await page.locator('[data-testid="guest-physical-disk"] > summary').focus();
      const detailRequest = page.waitForRequest(
        (request) => new URL(request.url()).pathname === OVERVIEW_MODULE,
      );
      await page.keyboard.press('Enter');
      await detailRequest;
      assert.equal(
        await page.getByTestId('disk-drawer-overview').count(),
        0,
        'the disk Overview must remain suspended while its module is loading',
      );
      assert.equal(
        await page.getByRole('status').filter({ hasText: 'Loading SMART details…' }).count(),
        1,
        'expanded disk exposes a loading state while the SMART module is pending',
      );
      assert.equal(
        await page
          .locator('[data-testid="guest-physical-disk"] > summary')
          .evaluate((summary) => summary === document.activeElement),
        true,
        'keyboard focus remains on disk summary',
      );
      await page.screenshot({
        path: path.join(artifacts, `guest-storage-2263-loading-${width}.png`),
        fullPage: true,
      });
      releaseDetail();
      const overview = page.getByTestId('disk-drawer-overview');
      await overview.waitFor();
      // An out-of-range SMART counter is lifted into Worth checking.
      await page.getByTestId('disk-drawer-exceptions').getByText('CRC Errors').waitFor();
      assert.equal(
        detailRequests.filter((request) => request.width === width).length,
        1,
        'the disk Overview module loads once after disk expansion',
      );
      await page.screenshot({
        path: path.join(artifacts, `guest-storage-2263-expanded-${width}.png`),
        fullPage: true,
      });
      // The expanded disk shows its SMART evidence and History in one view,
      // with no Overview/History tabs to switch between.
      assert.equal(await page.getByRole('tab').count(), 0);
      const region = page.getByTestId('guest-history-region');
      await region.waitFor();
      await page.waitForFunction(
        () => document.querySelectorAll('[data-history-series="smart_temp"]').length > 0,
      );
      const visibleCharts = () =>
        page.evaluate(() =>
          Array.from(document.querySelectorAll('[data-testid="guest-history-plot"]'))
            .filter((plot) => plot.getBoundingClientRect().width > 0)
            .map((plot) => plot.getAttribute('aria-label')),
        );
      const phoneToggle = page.getByTestId('history-phone-toggle');
      const ownReads = () => historyReads.filter((r) => r.width === width);
      // One read covers every chart of the disk, for its own target only.
      assert.ok(ownReads().length >= 1);
      for (const read of ownReads()) {
        assert.deepEqual(
          { resourceId: read.resourceId, metric: read.metric, range: read.range },
          { resourceId: 'disk-guest-101', metric: null, range: '24h' },
        );
      }
      if (width === 390) {
        // A phone shows the first chart; the rest fold behind one button.
        assert.deepEqual(await visibleCharts(), ['Temperature history']);
        assert.equal(await phoneToggle.innerText(), 'Show Disk I/O, Busy, Reallocated sectors');
        await page.screenshot({
          path: path.join(artifacts, `guest-storage-2263-phone-folded-${width}.png`),
          fullPage: true,
        });
        const before = ownReads().length;
        await phoneToggle.click();
        assert.equal(await phoneToggle.getAttribute('aria-expanded'), 'true');
        assert.equal(await phoneToggle.innerText(), 'Show fewer charts');
        await page.getByRole('img', { name: 'Disk I/O history' }).waitFor();
        await page.waitForTimeout(300);
        // The folded charts were already read with the first one.
        assert.equal(ownReads().length, before, 'unfolding starts no read of its own');
      } else {
        assert.equal(await phoneToggle.isVisible(), false, 'no fold above the phone breakpoint');
      }
      assert.deepEqual(await visibleCharts(), [
        'Temperature history',
        'Disk I/O history',
        'Busy history',
        'Reallocated sectors history',
      ]);
      // Spiky disk reads draw a min-max band behind their line; flat series do not.
      assert.deepEqual(
        await page.evaluate(() =>
          Array.from(document.querySelectorAll('[data-history-band]'), (band) =>
            band.getAttribute('data-history-band'),
          ),
        ),
        ['diskread'],
      );
      // Each chart's window times stand apart, however narrow its panel.
      const timeLabels = await page.evaluate(() =>
        Array.from(document.querySelectorAll('[data-testid="guest-history-group-chart"]'), (panel) => {
          const times = Array.from(
            panel.querySelectorAll('[data-testid="guest-history-time-window"] time'),
            (time) => time.getBoundingClientRect(),
          );
          return {
            chart: panel.querySelector('[data-testid="guest-history-plot"]').getAttribute('aria-label'),
            width: panel.clientWidth,
            labels: times.map((rect) => ({ left: rect.left, right: rect.right })),
          };
        }),
      );
      for (const chart of timeLabels) {
        assert.equal(chart.labels.length, 2, `${chart.chart} has its first and last time`);
        const [first, last] = chart.labels;
        assert.ok(last.left - first.right >= 4, `${chart.chart} time labels touch: ${JSON.stringify(chart)}`);
      }
      console.log(
        JSON.stringify({
          width,
          timeLabels: timeLabels.map((chart) => [chart.chart, chart.width, chart.labels.length]),
        }),
      );
      await page.screenshot({
        path: path.join(artifacts, `guest-storage-2263-expanded-charts-${width}.png`),
        fullPage: true,
      });
      await page.locator('[data-testid="guest-physical-disk"] > summary').click();
      await overview.waitFor({ state: 'hidden' });
      assert.equal(
        await overview.isVisible(),
        false,
        'pointer closes the expanded disk Overview',
      );
      await page.locator('[data-testid="guest-physical-disk"] > summary').click();
      await overview.waitFor();
      assert.equal(
        detailRequests.filter((request) => request.width === width).length,
        1,
        'reopening a disk does not reload the disk Overview module',
      );
      // A hidden Overview (another drawer tab) must stop the open disk's reads.
      const reads = () => historyReads.filter((read) => read.width === width).length;
      await page.clock.runFor(30_000);
      await page.waitForTimeout(200);
      const whileShown = reads();
      await page.clock.runFor(30_000);
      await page.waitForTimeout(200);
      assert.ok(reads() > whileShown, 'an open disk polls while its Overview is shown');
      await page.getByRole('button', { name: 'Hide overview' }).click();
      await page.waitForTimeout(200);
      const whileHidden = reads();
      await page.clock.runFor(120_000);
      await page.waitForTimeout(200);
      assert.equal(reads(), whileHidden, 'a hidden Overview reads no disk history');
      const resumed = page.waitForRequest((request) =>
        request.url().includes('/api/metrics-store/history'),
      );
      await page.getByRole('button', { name: 'Show overview' }).click();
      await resumed;
      await overview.waitFor();
      assert.ok(
        await page.locator('[data-history-series="smart_temp"]').first().isVisible(),
        'charts return with their readings',
      );
      // The drawer history reads a fixed point cap, so a resize keeps the
      // charts and their readings on screen without another read.
      const spinners = () => page.getByText('Loading history', { exact: true }).count();
      const resizedReads = reads();
      await page.setViewportSize({ width: width - 200, height: width === 390 ? 844 : 900 });
      await page.waitForTimeout(300);
      assert.equal(await spinners(), 0, 'a resize keeps the charts on screen');
      assert.equal(reads(), resizedReads, 'a resize starts no read');
      assert.ok(
        await page.locator('[data-history-series="smart_temp"]').first().isVisible(),
        'the resized charts keep their readings',
      );
      await page.setViewportSize({ width, height: width === 390 ? 844 : 900 });
      await page.waitForTimeout(300);
      await page.screenshot({
        path: path.join(artifacts, `guest-storage-2263-resumed-${width}.png`),
        fullPage: true,
      });
      assert.equal(await spinners(), 0, 'a full-page capture leaves no spinner behind');
      const beforeNoAgent = requests.length;
      await page.getByRole('button', { name: 'No agent' }).click();
      assert.equal(await page.locator('[data-testid="guest-physical-disks"]').count(), 0);
      assert.equal(
        requests.length,
        beforeNoAgent,
        'uninstrumented guest must not fetch physical disks',
      );
      await page.screenshot({
        path: path.join(artifacts, `guest-storage-2263-no-agent-${width}.png`),
        fullPage: true,
      });
      await page.getByRole('button', { name: 'Unavailable query' }).click();
      await page.getByText('Physical disk details are unavailable.').waitFor();
      assert.equal(await page.locator('[data-testid="guest-physical-disks"]').count(), 0);
      await page.screenshot({
        path: path.join(artifacts, `guest-storage-2263-error-${width}.png`),
        fullPage: true,
      });
      const dimensions = await page.evaluate(() => ({
        scroll: document.documentElement.scrollWidth,
        inner: window.innerWidth,
      }));
      assert.ok(
        dimensions.scroll <= dimensions.inner + 1,
        `${width}px horizontal overflow: ${JSON.stringify(dimensions)}`,
      );
      await page.close();
    }
    assert.equal(pageErrors.length, 0, `page errors: ${pageErrors.join('; ')}`);
    assert.deepEqual(
      requests.map(({ type, parent, page, limit }) => ({ type, parent, page, limit })),
      [
        { type: 'physical_disk', parent: 'vm-resource-101', page: '1', limit: '100' },
        { type: 'physical_disk', parent: 'vm-resource-error', page: '1', limit: '100' },
        { type: 'physical_disk', parent: 'vm-resource-101', page: '1', limit: '100' },
        { type: 'physical_disk', parent: 'vm-resource-error', page: '1', limit: '100' },
      ],
    );
    console.log(
      JSON.stringify({
        result: 'passed',
        browser: browser.version(),
        viewports: [1440, 390],
        route: '/browser-tests/guest-storage-2263.html',
        requests,
        detailRequests,
        historyReads: historyReads.length,
        pageErrors,
      }),
    );
  } finally {
    if (browser) await browser.close();
    await server.close();
  }
})().catch((error) => {
  console.error(error);
  process.exitCode = 1;
});
