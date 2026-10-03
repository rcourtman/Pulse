// Offline browser proof: a changed registry badge cannot complete an unknown action.
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const { chromium } = require('playwright');

(async () => {
  const root = '/workspace/frontend-modern';
  const artifacts = path.join(root, 'browser-tests', 'update-action-receipt-proof');
  fs.mkdirSync(artifacts, { recursive: true });
  process.chdir(root);
  const { createServer } = await import(path.join(root, 'node_modules/vite/dist/node/index.js'));
  const server = await createServer({
    root,
    configFile: path.join(root, 'vite.config.ts'),
    server: { host: '127.0.0.1', port: 5207, strictPort: true },
  });
  let browser;
  try {
    await server.listen();
    browser = await chromium.launch({ headless: true, channel: 'chromium', args: ['--no-sandbox'] });
    const results = [];
    for (const width of [1365, 390]) {
      let mode = 'pending';
      const page = await browser.newPage({ viewport: { width, height: width === 390 ? 844 : 900 } });
      const pageErrors = [];
      const actionRequests = [];
      page.on('pageerror', (error) => pageErrors.push(error.message));
      await page.route('**/*', (route) => {
        const url = new URL(route.request().url());
        if (url.origin !== 'http://127.0.0.1:5207') return route.abort();
        if (url.pathname === '/api/security/status') {
          return route.fulfill({ json: {
            hasAuthentication: true, requiresAuth: true,
            settingsCapabilities: { authenticationWrite: true },
          } });
        }
        if (url.pathname === '/api/actions/action-fixture-1') {
          actionRequests.push(route.request().method());
          return route.fulfill({ json: {
            audit: {
              id: 'action-fixture-1', createdAt: new Date().toISOString(),
              updatedAt: new Date().toISOString(), state: mode === 'pending' ? 'executing' : 'completed',
              decisionRevision: 1,
              request: { requestId: 'request-fixture-1', resourceId: 'docker:container:edge',
                capabilityName: 'update', reason: 'Update edge.', requestedBy: 'operator' },
              plan: { actionId: 'action-fixture-1', requestId: 'request-fixture-1', allowed: true,
                requiresApproval: false, approvalPolicy: 'none', rollbackAvailable: false,
                expiresAt: new Date(Date.now() + 30 * 60_000).toISOString(),
                planHash: 'sha256:fixture-plan', policyDecision: { status: 'resolved', authorities: [] } },
              verificationOutcome: { status: mode === 'pending' ? 'unknown' : 'verified' },
              ...(mode === 'complete' ? { result: { success: true,
                actionResultV2: { execution: { status: 'succeeded' },
                  verification: { status: 'verified', evidenceClass: 'agent' },
                  compensation: { status: 'not_attempted', support: 'unavailable' } } } } : {}),
            },
            events: [],
            attempt: { id: 'attempt-fixture-1', actionId: 'action-fixture-1',
              state: mode === 'pending' ? 'receipt_pending' : 'receipt_recorded',
              createdAt: new Date(Date.now() - 5 * 60_000).toISOString(),
              updatedAt: new Date(Date.now() - 5 * 60_000).toISOString(), dispatchCount: 1 },
          } });
        }
        if (url.pathname.startsWith('/api/')) return route.abort();
        return route.continue();
      });
      await page.goto('http://127.0.0.1:5207/browser-tests/update-action-receipt.html', {
        waitUntil: 'domcontentloaded', timeout: 120_000,
      });
      const review = page.getByRole('button', { name: /outcome not yet known/i });
      await review.waitFor();
      assert.equal(await page.getByText('Current', { exact: true }).count(), 0);
      assert.equal(await page.getByText('Completed', { exact: true }).count(), 0);
      await page.screenshot({ path: path.join(artifacts, `pending-${width}.png`) });

      await review.click();
      await page.getByText('Receipt pending', { exact: true }).waitFor();
      assert.equal(actionRequests.length, 1);
      await page.waitForTimeout(400); // Let the dialog entrance finish before capture.
      await page.screenshot({ path: path.join(artifacts, `review-${width}.png`) });

      mode = 'complete';
      await page.getByRole('button', { name: 'Check for receipt' }).click();
      await page.getByText('Completed', { exact: true }).first().waitFor();
      assert.equal(actionRequests.length, 2);
      assert.ok(actionRequests.every((method) => method === 'GET'), JSON.stringify(actionRequests));
      assert.deepEqual(pageErrors, []);
      const dimensions = await page.evaluate(() => ({ scroll: document.documentElement.scrollWidth, inner: innerWidth }));
      assert.ok(dimensions.scroll <= dimensions.inner + 1, JSON.stringify(dimensions));
      await page.waitForTimeout(400);
      await page.screenshot({ path: path.join(artifacts, `completed-${width}.png`) });
      results.push({ width, actionRequests, pageErrors, dimensions });
      await page.close();
    }
    console.log(JSON.stringify({ result: 'passed', browser: browser.version(), playwright: '1.56.1', results }));
  } finally {
    if (browser) await browser.close();
    await server.close();
  }
})().catch((error) => {
  console.error(error);
  process.exitCode = 1;
});
