const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const http = require('node:http');
const crypto = require('node:crypto');
const { chromium, webkit } = require('playwright');
const root = '/workspace';
const frontend = path.join(root, 'frontend-modern');
const staticRoot = path.join(root, 'tmp/drawer-reading-static/final');
const output = path.join(root, 'tmp/drawer-reading/browser');
const origin = 'http://127.0.0.1:5298';
const hash = (b) => crypto.createHash('sha256').update(b).digest('hex');
const longName = 'Microsoft Windows Server 2025 Datacenter Evaluation';
const version = '10.0.26100.1742 (64-bit)';
const summary = `${longName} • ${version}`;
const changed = [
  'frontend-modern/src/components/Workloads/GuestDrawerOverview.tsx',
  'frontend-modern/src/content/help/updates.ts',
];
const binding = JSON.parse(fs.readFileSync(path.join(staticRoot, 'binding.json')));
const result = {
  base: 'c56cc44bd8686413f2769cc9623dc30c87c589d8',
  sourceHashes: Object.fromEntries(
    changed.map((p) => [p, hash(fs.readFileSync(path.join(root, p)))]),
  ),
  buildBinding: binding,
  docHash: hash(fs.readFileSync(path.join(root, 'docs/TROUBLESHOOTING.md'))),
  playwright: require('playwright/package.json').version,
  scope:
    'Production Resources API/useWorkloads/GuestRow/GuestDrawer, NodeDrawer/HelpIcon, and shipped Docs viewer. Synthetic observations only; no guest command, permission, update, freeze or native recovery operation.',
  checks: [],
  requests: [],
  served: {},
  captures: [],
  browsers: [],
  errors: [],
  cleanup: {},
};
for (const [file, digest] of Object.entries(binding.sources)) {
  assert.equal(hash(fs.readFileSync(path.join(frontend, file))), digest, file);
}
const resource = (read) => {
  const total = 10 * 1024 ** 3;
  return {
    id: 'lab-pve-a-101',
    type: read.type,
    name: 'system-guest',
    status: 'online',
    sources: ['proxmox'],
    lastSeen: '2026-10-12T22:00:00Z',
    metrics: {
      cpu: { percent: 10 },
      memory: { percent: 25, used: total / 4, total },
      disk: { percent: 50, used: total / 2, total },
    },
    proxmox: {
      vmid: 101,
      instance: 'lab',
      nodeName: 'pve-a',
      runtimeStatus: 'running',
      osName: read.name,
      osVersion: read.version,
      guestAgentStatus: read.status,
      disks: [{ mountpoint: '/data', filesystem: 'ext4', total, used: total / 2 }],
    },
  };
};
let server;
(async () => {
  fs.mkdirSync(output, { recursive: true });
  assert.equal(
    result.playwright,
    JSON.parse(fs.readFileSync(path.join(root, 'tests/integration/package-lock.json'))).packages[
      'node_modules/@playwright/test'
    ].version,
  );
  server = http.createServer((request, response) => {
    const url = new URL(request.url, origin);
    let file;
    if (url.pathname === '/docs/TROUBLESHOOTING.md')
      file = path.join(root, 'docs/TROUBLESHOOTING.md');
    else {
      file = path.resolve(staticRoot, '.' + decodeURIComponent(url.pathname));
      if (!file.startsWith(staticRoot + '/')) {
        response.writeHead(403);
        response.end();
        return;
      }
      if (url.pathname === '/docs/TROUBLESHOOTING')
        file = path.join(staticRoot, 'browser-tests/drawer-reading-access.html');
    }
    if (!fs.existsSync(file) || !fs.statSync(file).isFile()) {
      response.writeHead(404);
      response.end();
      return;
    }
    const bytes = fs.readFileSync(file);
    const rel = path.relative(staticRoot, file);
    assert.equal(
      hash(bytes),
      file.endsWith('TROUBLESHOOTING.md') ? result.docHash : binding.artifacts[rel],
    );
    result.served[url.pathname] = hash(bytes);
    response.writeHead(200, {
      'Content-Type':
        {
          '.html': 'text/html',
          '.js': 'text/javascript',
          '.css': 'text/css',
          '.md': 'text/markdown',
        }[path.extname(file)] || 'application/octet-stream',
    });
    response.end(bytes);
  });
  await new Promise((resolve, reject) => {
    server.once('error', reject);
    server.listen(5298, '127.0.0.1', resolve);
  });
  for (const spec of [
    {
      name: 'desktop',
      engine: chromium,
      width: 1280,
      height: 900,
      colorScheme: 'light',
      timezoneId: 'Europe/London',
    },
    {
      name: 'phone',
      engine: webkit,
      width: 390,
      height: 844,
      colorScheme: 'dark',
      timezoneId: 'Asia/Tokyo',
    },
  ]) {
    const browser = await spec.engine.launch(
      spec.engine === chromium
        ? { headless: true, channel: 'chromium', args: ['--no-sandbox'] }
        : { headless: true },
    );
    const context = await browser.newContext({
      viewport: { width: spec.width, height: spec.height },
      hasTouch: spec.name === 'phone',
      isMobile: spec.name === 'phone',
      colorScheme: spec.colorScheme,
      locale: 'en-GB',
      timezoneId: spec.timezoneId,
    });
    result.browsers.push({
      ...Object.fromEntries(Object.entries(spec).filter(([k]) => k !== 'engine')),
      version: browser.version(),
    });
    let read = { name: longName, version, type: 'vm', status: 'available' };
    try {
      await context.route('**/*', async (route) => {
        const url = new URL(route.request().url());
        if (url.origin !== origin) {
          result.errors.push({ external: url.origin });
          await route.abort();
          return;
        }
        if (!url.pathname.startsWith('/api/')) {
          await route.continue();
          return;
        }
        result.requests.push({
          browser: spec.name,
          method: route.request().method(),
          path: url.pathname,
        });
        assert.equal(route.request().method(), 'GET');
        let body;
        if (url.pathname === '/api/resources')
          body = { data: [resource(read)], meta: { totalPages: 1 } };
        else if (url.pathname === '/api/license/runtime-capabilities')
          body = { plan: 'free', capabilities: {}, features: [] };
        else if (url.pathname === '/api/alerts/activation') body = { enabled: false };
        else if (url.pathname === '/api/alerts/config')
          body = { guestDefaults: {}, hostDefaults: {} };
        else if (url.pathname === '/api/alerts/active') body = [];
        else if (url.pathname === '/api/ai/intelligence/anomalies')
          body = { anomalies: [], count: 0 };
        else if (url.pathname === '/api/ai/intelligence') body = null;
        else if (url.pathname.startsWith('/api/guests/metadata/')) body = { customUrl: '' };
        else if (
          url.pathname.startsWith('/api/resources/') &&
          url.pathname.endsWith('/operator-state')
        )
          body = { configured: false, state: null };
        else if (url.pathname === '/api/metrics-store/history')
          body = {
            resourceType: url.searchParams.get('resourceType'),
            resourceId: url.searchParams.get('resourceId'),
            range: '24h',
            start: 1,
            end: 2,
            metrics: {},
            source: 'store',
          };
        else {
          result.errors.push({ unmatched: url.pathname });
          body = {};
        }
        await route.fulfill({ json: body });
      });
      const page = await context.newPage();
      page.setDefaultTimeout(10000);
      page.on('pageerror', (e) => result.errors.push({ browser: spec.name, error: e.message }));
      page.on('console', (e) => {
        if (e.type() === 'error') result.errors.push({ browser: spec.name, console: e.text() });
      });
      await page.clock.setFixedTime(new Date('2026-10-12T22:00:00Z'));
      await page.routeWebSocket(/\/ws(?:\?|$)/, () => {});
      await page.goto(`${origin}/browser-tests/drawer-reading-access.html`, {
        waitUntil: 'domcontentloaded',
        timeout: 30000,
      });
      if (spec.colorScheme === 'dark')
        await page.evaluate(() => document.documentElement.classList.add('dark'));
      const check = (name, detail = {}) =>
        result.checks.push({ browser: spec.name, name, ...detail });
      const capture = async (target, name) => {
        await target.evaluate(async () => {
          await document.fonts.ready;
          await new Promise((r) => requestAnimationFrame(() => requestAnimationFrame(r)));
        });
        const file = path.join(output, `${spec.name}-${name}.png`);
        await target.screenshot({ path: file, fullPage: false, animations: 'disabled' });
        result.captures.push({
          file: path.relative(root, file),
          sha256: hash(fs.readFileSync(file)),
          bytes: fs.statSync(file).size,
        });
      };
      const noOverflow = async (target) =>
        assert.ok(
          await target.evaluate(
            () => document.documentElement.scrollWidth <= window.innerWidth + 1,
          ),
        );
      const trigger = page.getByRole('button', { name: 'Expand system-guest', exact: true });
      await trigger.waitFor();
      const requests = result.requests.length;
      if (spec.name === 'desktop') {
        await trigger.focus();
        await page.keyboard.press('Enter');
      } else await page.locator('tr.workload-row').tap();
      const drawer = page.getByTestId('guest-technical-details');
      await drawer.waitFor();
      const systemValue = drawer.getByText(summary, { exact: true });
      await systemValue.waitFor();
      const original = await drawer.evaluate((el) => {
        window.__originalDrawer = el;
        return true;
      });
      assert.ok(original);
      const measure = async (value) =>
        value.evaluate((el) => {
          const rect = el.getBoundingClientRect(),
            style = getComputedStyle(el);
          const range = document.createRange();
          range.selectNodeContents(el);
          return {
            text: el.textContent,
            width: rect.width,
            height: rect.height,
            scrollWidth: el.scrollWidth,
            clientWidth: el.clientWidth,
            whiteSpace: style.whiteSpace,
            textOverflow: style.textOverflow,
            lineHeight: parseFloat(style.lineHeight),
            lineRects: [...range.getClientRects()].map((r) => ({
              x: r.x,
              right: r.right,
              y: r.y,
              bottom: r.bottom,
            })),
          };
        });
      const wrapped = await measure(systemValue);
      assert.equal(wrapped.text, summary);
      assert.equal(wrapped.whiteSpace, 'normal');
      assert.notEqual(wrapped.textOverflow, 'ellipsis');
      assert.ok(wrapped.scrollWidth <= wrapped.clientWidth + 1);
      assert.ok(wrapped.height > wrapped.lineHeight);
      assert.ok(wrapped.lineRects.every((r) => r.x >= 0 && r.right <= spec.width + 1));
      assert.equal(result.requests.length, requests, 'disclosure adds no API read');
      await noOverflow(page);
      check(
        'keyboard/touch opens the production row and full System wraps without tooltip interaction',
        wrapped,
      );
      await systemValue.scrollIntoViewIfNeeded();
      await capture(page, 'system');
      for (const next of [
        {
          name: 'Debian GNU/Linux',
          version: '13.7 (trixie)',
          type: 'system-container',
          status: 'available',
        },
        {
          name: longName,
          version: 'Build-' + '1234567890'.repeat(24),
          type: 'vm',
          status: 'not-running',
        },
        { name: 'Debian GNU/Linux', version: '', type: 'system-container', status: 'available' },
        { name: '', version: '13.7 (trixie)', type: 'system-container', status: 'available' },
        { name: '', version: '', type: 'vm', status: 'available' },
        { name: longName, version, type: 'vm', status: 'available' },
      ]) {
        read = next;
        await page.evaluate(() => window.__drawerReading.refetch());
        const expected = [next.name, next.version].filter(Boolean).join(' • ');
        if (expected) {
          const value = drawer.getByText(expected, { exact: true });
          await value.waitFor();
          const metrics = await measure(value);
          assert.ok(metrics.scrollWidth <= metrics.clientWidth + 1);
          assert.ok(metrics.lineRects.every((r) => r.x >= 0 && r.right <= spec.width + 1));
          if (next.status === 'not-running')
            assert.equal(await drawer.getByText('Not running', { exact: true }).count(), 1);
          check('current synthetic OS update remains complete and qualified', {
            ...next,
            geometry: metrics,
          });
        } else {
          assert.equal(
            await drawer
              .locator('td')
              .filter({ hasText: /^System$/ })
              .count(),
            0,
          );
          check('no OS evidence omits the System row');
        }
        assert.ok(await drawer.evaluate((el) => el === window.__originalDrawer));
        assert.equal(
          (await page.evaluate(() => window.__drawerReading.evidence())).type,
          next.type,
        );
        await noOverflow(page);
      }
      const helpTrigger = page.getByRole('button', { name: 'Why update checks are unavailable' });
      await helpTrigger.scrollIntoViewIfNeeded();
      const count = result.requests.length;
      if (spec.name === 'desktop') {
        await helpTrigger.focus();
        await page.keyboard.press('Enter');
      } else await helpTrigger.tap();
      const dialog = page.getByRole('dialog', { name: 'Proxmox update-check access' });
      await dialog.waitFor();
      await page.waitForFunction(() => {
        const d = document.querySelector('[role="dialog"]');
        return (
          d &&
          getComputedStyle(d).opacity === '1' &&
          d.getAnimations().every((a) => a.playState === 'finished')
        );
      });
      assert.ok((await dialog.innerText()).includes('broader than read-only monitoring'));
      assert.ok(
        (await dialog.innerText()).includes('does not identify the exact missing privilege'),
      );
      const link = dialog.getByRole('link', { name: 'Learn more' });
      assert.equal(
        await link.getAttribute('href'),
        '/docs/TROUBLESHOOTING#proxmox-pending-update-access',
      );
      assert.equal(await link.getAttribute('rel'), 'noopener noreferrer');
      assert.equal(result.requests.length, count);
      await page.evaluate(() => {
        window.__originalHelp = document.querySelector('[role="dialog"]');
        window.__drawerReading.node({
          cpu: 0.2,
          pendingUpdates: 7,
          pendingUpdatesStatus: 'stale',
          pendingUpdatesCheckedAt: '2026-10-12T21:58:00Z',
        });
      });
      assert.ok(await dialog.evaluate((el) => el === window.__originalHelp));
      assert.ok((await page.getByTestId('node-drawer').innerText()).includes('7 pending'));
      check(
        'denied and retained update guidance persists through synthetic polling; local protected docs link, no read or mutation',
      );
      // Show the link and complete help panel in the settled capture.
      await link.scrollIntoViewIfNeeded();
      await capture(page, 'update-help');
      const popupPromise = context.waitForEvent('page');
      if (spec.name === 'desktop') {
        await link.focus();
        await page.keyboard.press('Enter');
      } else await link.tap();
      const popup = await popupPromise;
      popup.on('pageerror', (e) =>
        result.errors.push({ browser: spec.name, docsError: e.message }),
      );
      await popup.waitForLoadState('domcontentloaded');
      await popup.locator('article').waitFor();
      if (spec.colorScheme === 'dark')
        await popup.evaluate(() => document.documentElement.classList.add('dark'));
      const heading = popup.getByRole('heading', {
        name: 'Proxmox pending-update access',
        exact: true,
      });
      await heading.waitFor();
      await popup.waitForFunction(() => {
        const h = document.querySelector('[id="proxmox-pending-update-access"]');
        const r = h?.getBoundingClientRect();
        return r && r.top >= 0 && r.top < window.innerHeight;
      });
      assert.equal(new URL(popup.url()).hash, '#proxmox-pending-update-access');
      assert.ok(
        (await popup.locator('article').innerText()).includes('Do not add it automatically'),
      );
      await noOverflow(popup);
      check(
        'actual help link opens the shipped Docs renderer at its access section, containing the read-only/TLS/no-POST caveats',
      );
      await capture(popup, 'shipped-docs');
      await popup.close();
      if (spec.name === 'desktop') await page.keyboard.press('Escape');
      else await dialog.getByRole('button', { name: 'Close help' }).tap();
      assert.equal(await dialog.count(), 0);
      assert.equal(
        await page.evaluate(() => document.activeElement?.getAttribute('aria-label')),
        'Why update checks are unavailable',
      );
      check('Escape/Close help returns focus to the existing trigger');
      await page.evaluate(() =>
        window.__drawerReading.node({
          pendingUpdates: 0,
          pendingUpdatesStatus: 'checked',
          pendingUpdatesReason: undefined,
          pendingUpdatesCheckedAt: '2026-10-12T22:00:00Z',
        }),
      );
      assert.equal(await helpTrigger.count(), 0);
      assert.ok((await page.getByTestId('node-drawer').innerText()).includes('No pending updates'));
      check('confirmed zero withdraws denial advice without editing access');
      assert.deepEqual(result.errors, []);
    } finally {
      await context.close();
      result.cleanup[`${spec.name}Context`] = true;
      await browser.close();
      result.cleanup[`${spec.name}Browser`] = true;
    }
  }
  result.result = 'passed';
})()
  .catch((error) => {
    result.result = 'failed';
    result.failure = { message: error.message, stack: error.stack };
    process.exitCode = 1;
  })
  .finally(async () => {
    if (server) await new Promise((resolve) => server.close(resolve));
    result.cleanup.server = true;
    fs.mkdirSync(output, { recursive: true });
    fs.writeFileSync(path.join(output, 'result.json'), JSON.stringify(result, null, 2) + '\n');
    console.log(
      JSON.stringify({
        result: result.result,
        checks: result.checks.length,
        captures: result.captures.length,
        errors: result.errors,
        cleanup: result.cleanup,
        failure: result.failure,
      }),
    );
  });
