const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const { chromium, webkit } = require('playwright');

// Each Kubernetes row's status words read in its dot's colour: a crash-looping
// Pod's reason red, a Pending one amber, a short Deployment's Ready count and a
// ReplicaSet's "1 not ready" amber, a healthy row plain.
const now = '2026-10-03T12:00:00Z';
const base = (id, type, kubernetes, extra = {}) => ({
  id,
  type,
  name: id,
  status: 'online',
  lastSeen: now,
  platformType: 'kubernetes',
  sources: ['kubernetes'],
  kubernetes: { clusterName: 'prod', namespace: 'shop', ...kubernetes },
  ...extra,
});
const container = (name, ready, extra = {}) => ({
  name,
  image: `ghcr.io/pulse-demo/${name}:1`,
  ready,
  restartCount: 0,
  state: ready ? 'running' : 'waiting',
  ...extra,
});
const observations = () => [
  base('checkout-api-7d9f-a1', 'pod', {
    podName: 'checkout-api-7d9f-a1',
    podPhase: 'Running',
    podContainers: [container('checkout-api', true)],
    ownerKind: 'ReplicaSet',
    ownerName: 'checkout-api-7d9f',
  }),
  base('checkout-api-7d9f-b2', 'pod', {
    podName: 'checkout-api-7d9f-b2',
    podPhase: 'Running',
    podContainers: [
      container('checkout-api', false, { reason: 'CrashLoopBackOff', restartCount: 14 }),
    ],
    restarts: 14,
    ownerKind: 'ReplicaSet',
    ownerName: 'checkout-api-7d9f',
  }),
  base('worker-5c2-c3', 'pod', {
    podName: 'worker-5c2-c3',
    podPhase: 'Pending',
    podContainers: [container('worker', false)],
  }),
  base('checkout-api', 'k8s-deployment', {
    desiredReplicas: 3,
    updatedReplicas: 3,
    readyReplicas: 1,
    availableReplicas: 1,
  }),
  base('storefront', 'k8s-deployment', {
    desiredReplicas: 2,
    updatedReplicas: 2,
    readyReplicas: 2,
    availableReplicas: 2,
  }),
  base('checkout-api-7d9f', 'k8s-replicaset', {
    resourceKind: 'ReplicaSet',
    desiredReplicas: 4,
    currentReplicas: 4,
    readyReplicas: 3,
    availableReplicas: 3,
  }),
];

const rgb = (colour) => colour.match(/\d+/g).map(Number);
// Red in light mode is deep, in dark mode a light pink: red leads either way.
const isRed = (colour) => {
  const [r, g, b] = rgb(colour);
  return r > 150 && r - g > 60 && r - b > 60;
};
const isAmber = (colour) => {
  const [r, g, b] = rgb(colour);
  return r > 150 && g > 70 && b < 80;
};

(async () => {
  const root = '/workspace/frontend-modern';
  const output = '/workspace/tmp/kubernetes-status-rows';
  fs.mkdirSync(output, { recursive: true });
  process.chdir(root);
  const { createServer } = await import(path.join(root, 'node_modules/vite/dist/node/index.js'));
  const server = await createServer({
    root,
    configFile: path.join(root, 'vite.config.ts'),
    cacheDir: path.join(output, 'vite-cache'),
    server: { host: '127.0.0.1', port: 5273, strictPort: true },
  });
  const results = [],
    screenshots = [];
  let browser;
  try {
    await server.listen();
    for (const [name, engine, width] of [
      ['desktop', chromium, 1440],
      ['phone', webkit, 390],
    ]) {
      browser = await engine.launch(
        engine === chromium
          ? { headless: true, channel: 'chromium', args: ['--no-sandbox'] }
          : { headless: true },
      );
      const page = await browser.newPage({
        viewport: { width, height: width === 390 ? 844 : 900 },
        isMobile: width === 390,
        hasTouch: width === 390,
        locale: 'en-GB',
        timezoneId: 'UTC',
      });
      await page.clock.setFixedTime(new Date(now));
      page.setDefaultTimeout(15000);
      const checks = [],
        errors = [];
      page.on('pageerror', (e) => errors.push(e.message));
      await page.route('**/*', async (route) => {
        const request = route.request(),
          url = new URL(request.url());
        if (url.origin !== 'http://127.0.0.1:5273') return route.abort();
        if (!url.pathname.startsWith('/api/')) return route.continue();
        if (url.pathname === '/api/resources')
          return route.fulfill({ json: { data: observations() } });
        return route.fulfill({
          json: {
            enabled: false,
            data: [],
            capabilities: [],
            facets: [],
            relationships: [],
            recentChanges: [],
            alerts: {},
          },
        });
      });
      const check = async (label, run) => {
        try {
          await run();
          checks.push({ label, passed: true });
        } catch (e) {
          checks.push({ label, passed: false, error: e.message });
        }
      };
      const colour = (locator) => locator.evaluate((el) => getComputedStyle(el).color);
      const dotColour = (locator) => locator.evaluate((el) => getComputedStyle(el).backgroundColor);
      // A fresh Vite cache compiles the app on the first load.
      await page.goto('http://127.0.0.1:5273/browser-tests/kubernetes-status-rows.html', {
        timeout: 180000,
      });
      await page.getByText('6 REST-mapped rows', { exact: true }).waitFor({ timeout: 60000 });
      if (width === 390) await page.evaluate(() => document.documentElement.classList.add('dark'));

      await check('A Pod status reads in its dot colour', async () => {
        const pods = page.getByRole('region', { name: 'Pod table' });
        const crash = pods.locator('[data-kubernetes-pod-row="checkout-api-7d9f-b2"]');
        const crashDot = crash.locator('[title="CrashLoopBackOff"]').first();
        await crashDot.waitFor();
        assert.ok(isRed(await dotColour(crashDot)), 'crash-looping dot red');
        const crashWords = crash.locator('[data-kubernetes-pod-status]');
        // The Status column is a desktop column; a phone row keeps the dot.
        if (await crashWords.isVisible()) {
          assert.equal(await crashWords.innerText(), 'CrashLoopBackOff');
          // The failure reason reads whole, not "CrashLoopBa…".
          const fit = await crashWords.evaluate((e) => ({
            scroll: e.scrollWidth,
            client: e.clientWidth,
          }));
          assert.ok(fit.scroll <= fit.client, JSON.stringify(fit));
          assert.ok(isRed(await colour(crashWords)), 'crash-looping words red');
          const pending = pods.locator(
            '[data-kubernetes-pod-row="worker-5c2-c3"] [data-kubernetes-pod-status]',
          );
          assert.ok(isAmber(await colour(pending)), 'pending words amber');
          const healthy = pods.locator(
            '[data-kubernetes-pod-row="checkout-api-7d9f-a1"] [data-kubernetes-pod-status]',
          );
          const plain = await colour(healthy);
          assert.ok(!isRed(plain) && !isAmber(plain), `healthy words plain: ${plain}`);
        } else {
          // A phone reads the reason under the pod name instead.
          assert.equal(width, 390);
          const phoneWords = crash.locator('[data-kubernetes-pod-status-phone]');
          assert.equal((await phoneWords.textContent()).trim(), 'CrashLoopBackOff');
          assert.ok(isRed(await colour(phoneWords)), 'phone crash-looping words red');
          const fit = await phoneWords
            .locator('span')
            .evaluate((e) => ({ scroll: e.scrollWidth, client: e.clientWidth }));
          assert.ok(fit.scroll <= fit.client, JSON.stringify(fit));
        }
        if (width !== 390) {
          // The desktop row has its Status column, so no second line.
          assert.equal(
            await crash.locator('[data-kubernetes-pod-status-phone]').isVisible(),
            false,
          );
        }
      });
      await check('A short Deployment reads its Ready count in amber', async () => {
        const deployments = page.getByRole('region', { name: 'Deployment table' });
        const short = deployments.locator('[data-kubernetes-deployment-ready="warning"]');
        await short.waitFor();
        assert.match((await short.innerText()).replace(/\s+/g, ''), /^1\/?3?$/);
        assert.ok(isAmber(await colour(short)), 'short Ready amber');
        const full = deployments.locator('[data-kubernetes-deployment-ready="success"]');
        const plain = await colour(full);
        assert.ok(!isRed(plain) && !isAmber(plain), `full Ready plain: ${plain}`);
      });
      await check('A controller exception reads in its dot colour', async () => {
        const controllers = page.getByRole('region', { name: 'Controller table' });
        const exception = controllers.getByText('1 not ready', { exact: true });
        await exception.waitFor();
        assert.ok(isAmber(await colour(exception)), 'exception amber');
      });
      const dimensions = await page.evaluate(() => ({
        width: innerWidth,
        scroll: document.documentElement.scrollWidth,
      }));
      checks.push({
        label: 'No horizontal page overflow',
        passed: dimensions.scroll <= dimensions.width + 1,
        dimensions,
      });
      checks.push({ label: 'No page errors', passed: errors.length === 0, errors });
      const file = `${name}-kubernetes.png`;
      await page.screenshot({ path: path.join(output, file), fullPage: true });
      screenshots.push({ file, engine: name, width });
      results.push({ engine: name, width, checks });
      await browser.close();
      browser = undefined;
    }
  } finally {
    if (browser) await browser.close();
    await server.close();
  }
  const failures = results.flatMap((r) =>
    r.checks.filter((c) => !c.passed).map((c) => ({ engine: r.engine, ...c })),
  );
  fs.writeFileSync(
    path.join(output, 'result.json'),
    JSON.stringify({ verified_at: new Date().toISOString(), results, screenshots }, null, 2),
  );
  console.log(
    JSON.stringify({
      checked: results.reduce((n, r) => n + r.checks.length, 0),
      screenshots: screenshots.length,
      failures,
    }),
  );
  assert.equal(failures.length, 0, JSON.stringify(failures));
})().catch((error) => {
  console.error(error);
  process.exitCode = 1;
});
