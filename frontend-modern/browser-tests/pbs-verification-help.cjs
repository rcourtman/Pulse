// Read-only help acceptance through the production Docs page, router and CSS.
// No PBS connection, diagnostic run or verification job is involved.
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const crypto = require('node:crypto');
const { chromium, webkit } = require('playwright');

(async () => {
  const root = '/workspace/frontend-modern';
  const artifacts = path.join(root, 'node_modules', 'pbs-verification-help-browser');
  const fragment = 'backups-are-visible-but-coverage-says-unprotected';
  const guide = fs.readFileSync(path.join(root, 'public/docs/PBS.md'));
  assert.deepEqual(guide, fs.readFileSync('/workspace/docs/PBS.md'));
  process.chdir(root);
  fs.mkdirSync(artifacts, { recursive: true });
  const { createServer } = await import(path.join(root, 'node_modules/vite/dist/node/index.js'));
  const server = await createServer({
    root, configFile: path.join(root, 'vite.config.ts'),
    cacheDir: path.join(root, 'node_modules', '.vite-pbs-verification-help'),
    server: { host: '127.0.0.1', port: 5256, strictPort: true },
    optimizeDeps: { noDiscovery: true, include: [] },
  });
  const results = [];
  let browser;
  try {
    await server.listen();
    for (const [engine, width, dark] of [['chromium', 1280, false], ['webkit', 390, true]]) {
      browser = await { chromium, webkit }[engine].launch(engine === 'chromium'
        ? { headless: true, channel: 'chromium', args: ['--no-sandbox'] }
        : { headless: true });
      const page = await browser.newPage({ viewport: { width, height: 1000 },
        ...(engine === 'webkit' ? { isMobile: true, hasTouch: true } : {}) });
      const errors = [], backendRequests = [], mutations = [], servedDocs = [];
      page.on('pageerror', (error) => errors.push(error.message));
      page.on('console', (message) => {
        if (message.type() === 'error') errors.push(message.text());
      });
      page.on('request', (request) => {
        if (!['GET', 'HEAD'].includes(request.method())) mutations.push(request.method());
      });
      page.on('response', async (response) => {
        if (new URL(response.url()).pathname === '/docs/PBS.md') {
          const bytes = await response.body();
          servedDocs.push({ status: response.status(),
            sha256: crypto.createHash('sha256').update(bytes).digest('hex') });
        }
      });
      await page.route('**/api/**', async (route) => {
        backendRequests.push(new URL(route.request().url()).pathname);
        await route.abort();
      });
      await page.goto('http://127.0.0.1:5256/browser-tests/docs-fragment-navigation.html', {
        waitUntil: 'domcontentloaded', timeout: 60000,
      });
      if (dark) await page.evaluate(() => document.documentElement.classList.add('dark'));
      await page.evaluate(() => {
        history.pushState({}, '', '/docs/RECOVERY#missing-or-inconsistent-evidence');
        dispatchEvent(new PopStateEvent('popstate'));
      });
      const entry = page.locator(`a[href="/docs/PBS#${fragment}"]`);
      await entry.waitFor({ state: 'visible' });
      assert.equal(await entry.innerText(), 'backup health checks');
      await entry.focus();
      await page.keyboard.press('Enter');
      await page.waitForFunction((id) => {
        const target = document.getElementById(id);
        const rect = target?.getBoundingClientRect();
        return target && document.activeElement === target && rect.top >= 0 && rect.top < innerHeight;
      }, fragment);
      const heading = page.locator(`#${fragment}`);
      const section = await page.evaluate((id) => {
        const target = document.getElementById(id);
        const nodes = [];
        for (let node = target.nextElementSibling; node && !/^H[1-3]$/.test(node.tagName); node = node.nextElementSibling) {
          nodes.push(node.textContent);
        }
        return nodes.join(' ').replace(/\s+/g, ' ');
      }, fragment);
      for (const phrase of [
        "Protected in a restore point's State", "not the workload's protection assessment",
        'choose the newest point whose Source is PBS', 'Created time and State',
        'Hover the Created age for its absolute timestamp', 'if unavailable, keep the time unknown',
        'an older or absent point in Pulse is an inventory discrepancy',
        "does not prove that PBS's verification job failed",
        'Verified with a workload still needing attention is a separate assessment disagreement',
        'Protected point leaves its verification state unknown from that label alone',
        "Job OK is not an individual point's verification result",
        'without running Diagnostics or another verification',
        'Do not delete backups, clear History, change retention or freshness settings',
      ]) assert.ok(section.includes(phrase), `${engine}: ${phrase}`);
      assert.ok(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1));
      await page.waitForFunction(() => document.getAnimations().length === 0);
      await page.screenshot({ path: path.join(artifacts, `${engine}-evidence.png`) });
      const comparison = page.locator('p').filter({ hasText: 'If PBS says verified but Pulse disagrees:' });
      await comparison.scrollIntoViewIfNeeded();
      await page.screenshot({ path: path.join(artifacts, `${engine}-comparison.png`) });
      const safety = page.getByRole('link', { name: 'Backup safety', exact: true });
      await safety.focus();
      await page.keyboard.press('Enter');
      await page.waitForFunction(() => document.activeElement?.id === 'backup-safety');
      assert.ok(await page.locator('#backup-safety').isVisible());
      assert.ok(await heading.count() === 0);
      assert.deepEqual(backendRequests, []);
      assert.deepEqual(mutations, []);
      assert.deepEqual(errors, []);
      assert.ok(servedDocs.length >= 1);
      for (const served of servedDocs) {
        assert.equal(served.status, 200);
        assert.equal(served.sha256, crypto.createHash('sha256').update(guide).digest('hex'));
      }
      results.push({ engine, browserVersion: browser.version(), width, dark,
        helpRoute: `/docs/PBS#${fragment}`, sourceMatchedResponses: servedDocs,
        recoveryEntryKeyboardNavigation: true, visibleFocusedHeading: true,
        backupSafetyKeyboardNavigation: true, noHorizontalPageOverflow: true,
        backendRequests, mutations, errors });
      await browser.close();
      browser = undefined;
    }
  } finally {
    if (browser) await browser.close();
    await server.close();
  }
  const result = { result: 'passed', playwright: require('playwright/package.json').version,
    docSha256: crypto.createHash('sha256').update(guide).digest('hex'), results,
    browserAndServerClosed: true,
    scope: 'Production documentation renderer, page, router, CSS and exact shipped Markdown; no native PBS, full application, diagnosis or release acceptance.' };
  fs.writeFileSync(path.join(artifacts, 'result.json'), JSON.stringify(result, null, 2) + '\n');
  console.log(JSON.stringify(result));
})().catch((error) => { console.error(error); process.exitCode = 1; });
