// Prove that late action reads cannot change the route-backed review or reopen it.
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const { chromium } = require('playwright');

const action = (id) => ({
  id,
  createdAt: '2026-09-29T18:00:00Z',
  updatedAt: '2026-09-29T18:01:00Z',
  state: 'executing',
  decisionRevision: 1,
  request: {
    requestId: `request-${id}`,
    resourceId: `docker:container:${id}`,
    capabilityName: 'update',
    reason: `Update ${id}`,
    requestedBy: 'operator',
  },
  plan: {
    actionId: id,
    requestId: `request-${id}`,
    allowed: true,
    requiresApproval: false,
    approvalPolicy: 'none',
    rollbackAvailable: false,
    expiresAt: '2026-09-29T21:00:00Z',
    planHash: `sha256:plan-${id}`,
  },
  verificationOutcome: { status: 'unknown' },
});
const detail = (id) => ({
  audit: action(id),
  events: [],
  readiness: {
    ready: false,
    code: 'receipt_pending',
    message: 'Awaiting receipt',
    refreshable: false,
    checkedAt: '2026-09-29T18:01:00Z',
  },
  attempt: {
    id: `attempt-${id}`,
    actionId: id,
    state: 'receipt_pending',
    createdAt: '2026-09-29T18:00:00Z',
    updatedAt: '2026-09-29T18:01:00Z',
    dispatchCount: 1,
  },
});

(async () => {
  const root = '/workspace/frontend-modern';
  const artifacts = path.join(root, 'node_modules', 'action-review-navigation-proof');
  fs.mkdirSync(artifacts, { recursive: true });
  process.chdir(root);
  const { createServer } = await import(path.join(root, 'node_modules/vite/dist/node/index.js'));
  const server = await createServer({
    root,
    configFile: path.join(root, 'vite.config.ts'),
    server: { host: '127.0.0.1', port: 5204, strictPort: true },
  });
  let browser;
  try {
    await server.listen();
    browser = await chromium.launch({
      headless: true,
      channel: 'chromium',
      args: ['--no-sandbox'],
    });
    const results = [];
    for (const width of [1365, 390]) {
      let releaseA;
      let releaseReceipt;
      let bReads = 0;
      const requests = [];
      const errors = [];
      const page = await browser.newPage({
        viewport: { width, height: width === 390 ? 844 : 900 },
      });
      page.on('pageerror', (error) => errors.push(error.message));
      await page.route('**/*', async (route) => {
        const request = route.request();
        const url = new URL(request.url());
        if (url.origin !== 'http://127.0.0.1:5204') return route.abort();
        if (url.pathname.startsWith('/api/')) requests.push(`${request.method()} ${url.pathname}`);
        if (url.pathname === '/api/actions') {
          const view = url.searchParams.get('view');
          return route.fulfill({
            json: {
              view,
              actions: view === 'pending' ? [action('edge-a'), action('edge-b')] : [],
              count: view === 'pending' ? 2 : 0,
              readOnly: false,
            },
          });
        }
        if (url.pathname === '/api/actions/edge-a') {
          await new Promise((resolve) => {
            releaseA = resolve;
          });
          return route.fulfill({ json: detail('edge-a') });
        }
        if (url.pathname === '/api/actions/edge-b') {
          bReads += 1;
          if (bReads === 2)
            await new Promise((resolve) => {
              releaseReceipt = resolve;
            });
          return route.fulfill({ json: detail('edge-b') });
        }
        if (url.pathname === '/api/security/status')
          return route.fulfill({
            json: {
              hasAuthentication: true,
              requiresAuth: true,
              settingsCapabilities: { authenticationWrite: false },
            },
          });
        if (url.pathname.startsWith('/api/'))
          return route.fulfill({ status: 503, json: { error: 'fixture unavailable' } });
        return route.continue();
      });
      await page.goto('http://127.0.0.1:5204/browser-tests/action-review-navigation.html', {
        waitUntil: 'domcontentloaded',
        timeout: 120_000,
      });
      await page.getByRole('button', { name: /Review Update on docker:container:edge-a/ }).click();
      await page.waitForFunction(() => location.search === '?action=edge-a');
      await page.getByRole('button', { name: /Review Update on docker:container:edge-b/ }).click();
      await page.getByRole('dialog').getByText('docker:container:edge-b').first().waitFor();
      releaseA();
      await page.waitForTimeout(300);
      assert.match(await page.getByRole('dialog').innerText(), /docker:container:edge-b/);
      assert.doesNotMatch(await page.getByRole('dialog').innerText(), /docker:container:edge-a/);
      assert.equal(new URL(page.url()).search, '?action=edge-b');
      await page.screenshot({ path: path.join(artifacts, `newest-review-${width}.png`) });

      const receiptRead = page.waitForRequest(
        (request) =>
          new URL(request.url()).pathname === '/api/actions/edge-b' && request.method() === 'GET',
      );
      await page.getByRole('button', { name: 'Check for receipt' }).click();
      await receiptRead;
      await page.waitForTimeout(50);
      // The pending route is observable through the second GET, not a mutation.
      assert.equal(bReads, 2);
      await page.getByRole('button', { name: 'Close action review' }).click();
      await page.waitForFunction(() => location.search === '');
      releaseReceipt();
      await page.waitForTimeout(300);
      assert.equal(await page.getByRole('dialog').count(), 0);
      assert.equal(new URL(page.url()).search, '');
      assert.ok(
        requests.every((item) => item.startsWith('GET ')),
        JSON.stringify(requests),
      );
      assert.deepEqual(errors, []);
      const dimensions = await page.evaluate(() => ({
        scroll: document.documentElement.scrollWidth,
        inner: innerWidth,
      }));
      assert.ok(dimensions.scroll <= dimensions.inner + 1, JSON.stringify(dimensions));
      results.push({ width, requests, errors, dimensions });
      await page.close();
    }
    console.log(
      JSON.stringify({
        result: 'passed',
        browser: browser.version(),
        playwright: '1.56.1',
        results,
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
