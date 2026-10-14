// Browser proof for #2263. Uses the production guest overview and resource hook
// in Vite, with only the network response replaced by bounded fixture data.
const assert = require('node:assert/strict');
const path = require('node:path');
const { chromium } = require('playwright');

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
        if (new URL(request.url()).pathname === '/src/components/Storage/DiskDetail.tsx') {
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
          historyReads.push({ width, metric: q.get('metric'), range: q.get('range') });
          const end = Date.now();
          return route.fulfill({
            json: {
              resourceType: q.get('resourceType'),
              resourceId: q.get('resourceId'),
              metric: q.get('metric'),
              range: q.get('range'),
              start: end - 600_000,
              end,
              source: 'store',
              points: [0, 1, 2].map((i) => ({
                timestamp: end - (2 - i) * 300_000,
                value: 40 + i,
                min: 40 + i,
                max: 40 + i,
              })),
            },
          });
        }
        if (url.pathname.startsWith('/api/')) return route.fulfill({ json: {} });
        return route.continue();
      });
      let releaseDetail;
      await page.route('**/src/components/Storage/DiskDetail.tsx*', async (route) => {
        await new Promise((resolve) => {
          releaseDetail = resolve;
        });
        await route.continue();
      });
      // Disk charts poll every 10-30 seconds; the clock drives that here.
      await page.clock.install();
      // Canvas text is not in the DOM: record each chart's drawn labels since
      // its last clear, so the time axis can be checked for overlap.
      await page.addInitScript(() => {
        const proto = CanvasRenderingContext2D.prototype;
        const clear = proto.clearRect;
        const fill = proto.fillText;
        proto.clearRect = function (...args) {
          this.canvas.__labels = [];
          return clear.apply(this, args);
        };
        proto.fillText = function (text, x, y, ...rest) {
          (this.canvas.__labels ||= []).push({
            text: String(text),
            x,
            width: this.measureText(String(text)).width,
            time: this.textAlign === 'center' && this.textBaseline === 'bottom',
          });
          return fill.call(this, text, x, y, ...rest);
        };
      });
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
        'SMART history module must not load before disk expansion',
      );
      await page.screenshot({
        path: path.join(artifacts, `guest-storage-2263-collapsed-${width}.png`),
        fullPage: true,
      });
      await page.locator('[data-testid="guest-physical-disk"] summary').focus();
      const detailRequest = page.waitForRequest(
        (request) => new URL(request.url()).pathname === '/src/components/Storage/DiskDetail.tsx',
      );
      await page.keyboard.press('Enter');
      await detailRequest;
      assert.equal(
        await page.getByText('Reallocated Sectors').first().count(),
        0,
        'SMART detail must remain suspended while its module is loading',
      );
      assert.equal(
        await page.getByRole('status').filter({ hasText: 'Loading SMART details…' }).count(),
        1,
        'expanded disk exposes a loading state while the SMART module is pending',
      );
      assert.equal(
        await page
          .locator('[data-testid="guest-physical-disk"] summary')
          .evaluate((summary) => summary === document.activeElement),
        true,
        'keyboard focus remains on disk summary',
      );
      await page.screenshot({
        path: path.join(artifacts, `guest-storage-2263-loading-${width}.png`),
        fullPage: true,
      });
      releaseDetail();
      await page.getByText('Reallocated Sectors').first().waitFor();
      assert.equal(
        detailRequests.filter((request) => request.width === width).length,
        1,
        'SMART history module loads once after disk expansion',
      );
      await page.screenshot({
        path: path.join(artifacts, `guest-storage-2263-expanded-${width}.png`),
        fullPage: true,
      });
      // The expanded disk shows its SMART evidence and History in one view,
      // with no Overview/History tabs to switch between.
      assert.equal(await page.getByRole('tab').count(), 0);
      assert.equal(await page.getByTestId('disk-detail-history').isVisible(), true);
      // Each chart's time labels stand apart, however narrow its card.
      const timeLabels = await page.evaluate(() =>
        Array.from(document.querySelectorAll('canvas'), (canvas) => ({
          chart: canvas.getAttribute('aria-label'),
          width: canvas.clientWidth,
          labels: (canvas.__labels || []).filter((label) => label.time),
        })),
      );
      for (const chart of timeLabels) {
        assert.ok(chart.labels.length >= 2, `${chart.chart} has its first and last time`);
        const sorted = [...chart.labels].sort((a, b) => a.x - b.x);
        for (let i = 1; i < sorted.length; i++) {
          const gap = sorted[i].x - sorted[i - 1].x - (sorted[i].width + sorted[i - 1].width) / 2;
          assert.ok(gap >= 4, `${chart.chart} time labels touch: ${JSON.stringify(sorted)}`);
        }
      }
      console.log(
        JSON.stringify({
          width,
          timeLabels: timeLabels.map((chart) => [chart.chart, chart.width, chart.labels.length]),
        }),
      );
      await page.locator('[data-testid="guest-physical-disk"] summary').click();
      await page.getByText('Reallocated Sectors').first().waitFor({ state: 'hidden' });
      assert.equal(
        await page.getByText('Reallocated Sectors').first().isVisible(),
        false,
        'pointer closes the expanded SMART detail',
      );
      await page.locator('[data-testid="guest-physical-disk"] summary').click();
      await page.getByText('Reallocated Sectors').first().waitFor();
      assert.equal(
        detailRequests.filter((request) => request.width === width).length,
        1,
        'reopening a disk does not reload the SMART module',
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
      await page.getByText('Reallocated Sectors').first().waitFor();
      assert.ok(
        await page.locator('[data-testid="disk-detail-history"] canvas').first().isVisible(),
        'charts return with their readings',
      );
      // A resize asks for the same series at a new point cap: the charts keep
      // their readings and show no loading state while it is read.
      const spinners = () => page.getByLabel('Loading history').count();
      const resizedReads = reads();
      const resizedRead = page.waitForRequest((request) =>
        request.url().includes('/api/metrics-store/history'),
      );
      await page.setViewportSize({ width: width - 200, height: width === 390 ? 844 : 900 });
      await resizedRead;
      assert.equal(await spinners(), 0, 'a resize keeps the charts on screen');
      await page.waitForTimeout(300);
      assert.ok(reads() > resizedReads, 'the resized charts read at their new cap');
      assert.equal(await spinners(), 0, 'no spinner after the resized read');
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
