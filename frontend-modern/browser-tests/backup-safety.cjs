// Read-only synthetic Resources responses consumed by the real production UI.
const assert = require('node:assert/strict');
const crypto = require('node:crypto');
const fs = require('node:fs');
const http = require('node:http');
const path = require('node:path');
const { chromium, webkit } = require('playwright');
const root = '/workspace';
const frontend = path.join(root, 'frontend-modern');
const compiled = path.join(root, 'tmp/backup-safety-static/final');
const output = path.join(root, 'tmp/backup-safety/browser');
const origin = 'http://127.0.0.1:5298';
const hash = (data) => crypto.createHash('sha256').update(data).digest('hex');
const binding = JSON.parse(fs.readFileSync(path.join(compiled, 'binding.json')));
const result = {
  base: '29a8636037e801fff6536cbaef94d3b84ba57df8',
  scope:
    'Canonical Resources API/useWorkloads -> GuestRow -> both BackupStatusCell disclosures; actual shipped Docs. No guest command, backup, thaw, credential or recovery operation.',
  playwright: require('playwright/package.json').version,
  buildBinding: binding,
  harnessSha256: hash(fs.readFileSync(__filename)),
  results: [],
  requests: [],
  captures: [],
  served: {},
  errors: [],
  cleanup: {},
};
const resource = (vmid, observation = {}) => {
  const name =
    vmid === 101
      ? 'windows-backup-server-with-a-long-name'
      : vmid === 102
        ? 'healthy-peer'
        : vmid === 103
          ? 'linux-container'
          : 'vsphere-control';
  const total = 10 * 1024 ** 3;
  return {
    id: `lab-pve-a-${vmid}`,
    vmid,
    name,
    type: vmid === 103 ? 'system-container' : 'vm',
    status: 'online',
    sources: vmid === 104 ? ['vsphere'] : ['proxmox'],
    platformScopes: vmid === 104 ? ['vmware-vsphere'] : ['proxmox-pve'],
    lastSeen: '2026-10-13T01:00:00Z',
    metrics: {
      cpu: { percent: 10 },
      memory: { percent: 25, used: total / 4, total },
      disk: { percent: 50, used: total / 2, total },
    },
    ...(vmid === 104
      ? {
          vmware: {
            managedObjectId: 'vm-104',
            powerState: 'running',
            runtimeHostName: 'vsphere-a',
          },
        }
      : {
          proxmox: {
            vmid,
            instance: 'lab',
            nodeName: 'pve-a',
            runtimeStatus: 'running',
            lastBackup: '2026-10-13T00:30:00Z',
            backupInProgress: false,
            guestAgentStatus: 'available',
            lock: '',
            diskStatusReason: '',
            ...observation,
          },
        }),
    ...(vmid === 101
      ? {
          agent: {
            agentId: 'synthetic-agent',
            agentVersion: '6.5.0',
            stale: false,
            osName: 'Windows',
          },
        }
      : {}),
  };
};
let server;
let browser;
(async () => {
  fs.mkdirSync(output, { recursive: true });
  assert.equal(
    result.playwright,
    JSON.parse(fs.readFileSync(path.join(root, 'tests/integration/package-lock.json'))).packages[
      'node_modules/@playwright/test'
    ].version,
  );
  assert.equal(binding.graph, hash(fs.readFileSync(path.join(frontend, 'package-lock.json'))));
  for (const [file, digest] of Object.entries(binding.sources))
    assert.equal(
      hash(fs.readFileSync(path.join(frontend, file))),
      digest,
      `source changed: ${file}`,
    );
  for (const file of [
    'src/components/Workloads/GuestRow.tsx',
    'src/components/Workloads/GuestRowCells.tsx',
    'src/components/Workloads/guestDrawerModel.ts',
    'src/hooks/useWorkloads.ts',
    'src/hooks/useUnifiedResources.ts',
    'src/pages/Docs.tsx',
  ])
    assert(binding.sources[file], `source not compiled: ${file}`);
  const assets = new Map();
  for (const [file, digest] of Object.entries(binding.artifacts)) {
    const bytes = fs.readFileSync(path.join(compiled, file));
    assert.equal(hash(bytes), digest, `artifact changed: ${file}`);
    assets.set('/' + file, bytes);
  }
  server = http.createServer((request, response) => {
    let file = new URL(request.url, origin).pathname;
    if (file === '/docs/VM_DISK_MONITORING') file = '/browser-tests/backup-safety.html';
    const bytes = assets.get(file);
    if (!bytes || !['GET', 'HEAD'].includes(request.method)) {
      response.writeHead(404);
      response.end();
      return;
    }
    result.served[file] = hash(bytes);
    response.writeHead(200, {
      'Content-Type':
        {
          '.html': 'text/html',
          '.js': 'text/javascript',
          '.css': 'text/css',
          '.md': 'text/markdown',
        }[path.extname(file)] || 'application/octet-stream',
    });
    response.end(request.method === 'HEAD' ? undefined : bytes);
  });
  await new Promise((resolve, reject) => {
    server.once('error', reject);
    server.listen(5298, '127.0.0.1', resolve);
  });
  for (const spec of [
    { name: 'desktop', engine: chromium, width: 1280, height: 900, colorScheme: 'light' },
    { name: 'phone', engine: webkit, width: 375, height: 812, colorScheme: 'dark' },
  ]) {
    browser = await spec.engine.launch(
      spec.engine === chromium
        ? { headless: true, channel: 'chromium', args: ['--no-sandbox'] }
        : { headless: true },
    );
    const page = await browser.newPage({
      viewport: { width: spec.width, height: spec.height },
      hasTouch: spec.name === 'phone',
      isMobile: spec.name === 'phone',
      locale: 'en-GB',
      timezoneId: 'UTC',
      colorScheme: spec.colorScheme,
    });
    page.setDefaultTimeout(12000);
    await page.clock.setFixedTime(new Date('2026-10-13T01:00:00Z'));
    if (spec.colorScheme === 'dark')
      await page.addInitScript(() =>
        document.addEventListener('DOMContentLoaded', () =>
          document.documentElement.classList.add('dark'),
        ),
      );
    let observation = {};
    let primaryPresent = true;
    const checks = [];
    const errors = [];
    const queries = [];
    const evidence = [];
    page.on('pageerror', (error) => errors.push(error.message));
    await page.route('**/*', async (route) => {
      const request = route.request();
      const url = new URL(request.url());
      assert.equal(url.origin, origin, 'no off-origin request');
      if (!url.pathname.startsWith('/api/')) return route.continue();
      assert.equal(request.method(), 'GET', 'no mutating operation');
      let body;
      if (url.pathname === '/api/resources')
        body = {
          data: [
            ...(primaryPresent ? [resource(101, observation)] : []),
            resource(102),
            resource(103, { lock: 'backup', backupInProgress: true, guestAgentStatus: 'deferred' }),
            resource(104),
          ],
          meta: { totalPages: 1 },
        };
      else if (url.pathname === '/api/license/runtime-capabilities')
        body = { plan: 'free', capabilities: {}, features: [] };
      else if (url.pathname === '/api/alerts/activation') body = { enabled: false };
      else if (url.pathname === '/api/alerts/config')
        body = { guestDefaults: {}, hostDefaults: {} };
      else if (url.pathname === '/api/alerts/active') body = [];
      else if (url.pathname === '/api/ai/intelligence/anomalies')
        body = { anomalies: [], count: 0 };
      else if (url.pathname.startsWith('/api/guests/metadata/')) body = { customUrl: '' };
      else if (
        url.pathname.startsWith('/api/resources/') &&
        url.pathname.endsWith('/operator-state')
      )
        body = { configured: false, state: null };
      else {
        throw new Error(`unmatched API read: ${url.pathname}`);
      }
      queries.push(url.pathname);
      result.requests.push({
        browser: spec.name,
        path: url.pathname,
        method: request.method(),
        bodySha256: hash(JSON.stringify(body)),
        ...(url.pathname === '/api/resources' ? { body } : {}),
      });
      await route.fulfill({ json: body });
    });
    await page.goto(origin + '/browser-tests/backup-safety.html');
    const row = page.getByRole('region', { name: 'Guest 101 row' });
    const badge = row.getByRole('button', { name: /^Backup status:/ });
    const dialog = page.getByRole('dialog', { name: /^Backup status details for windows-backup/ });
    const close = () => dialog.getByRole('button', { name: 'Close backup details' });
    const activate = (element) => (spec.name === 'phone' ? element.tap() : element.click());
    const settle = async () => {
      await dialog.waitFor({ state: 'visible' });
      await dialog.evaluate(async (el) => {
        await Promise.all(
          el
            .getAnimations({ subtree: true })
            .filter((animation) => Number.isFinite(animation.effect?.getTiming().iterations))
            .map((animation) => animation.finished.catch(() => {})),
        );
      });
      const rect = await dialog.boundingBox();
      assert(
        rect.x >= 0 &&
          rect.y >= 0 &&
          rect.x + rect.width <= spec.width &&
          rect.y + rect.height <= spec.height,
        'settled dialog fits viewport',
      );
      assert.equal(
        await page.evaluate(() => window.__backupSafety.actions()),
        0,
        'evidence disclosure never dispatches row action',
      );
    };
    const capture = async (state) => {
      const file = path.join(output, `${spec.name}-${state}.png`);
      await page.screenshot({ path: file, fullPage: false });
      result.captures.push({
        path: path.relative(root, file),
        sha256: hash(fs.readFileSync(file)),
        bytes: fs.statSync(file).size,
        state,
        browser: spec.name,
      });
    };
    const refetch = async (value) => {
      observation = value;
      await page.evaluate(() => window.__backupSafety.refetch());
      evidence.push(await page.evaluate(() => window.__backupSafety.evidence()));
    };
    const noExtraReads = async (action) => {
      const before = queries.length;
      await action();
      assert.equal(queries.length, before, 'disclosing retained evidence adds no API read');
    };
    await badge.waitFor();
    assert.equal((await badge.innerText()).trim(), '', 'fresh badge remains compact');
    if (spec.name === 'desktop') {
      await page.keyboard.press('Tab'); // existing column control
      await page.keyboard.press('Tab'); // row disclosure
      await page.keyboard.press('Tab'); // backup evidence
      assert(await badge.evaluate((el) => document.activeElement === el));
      await noExtraReads(() => page.keyboard.press('Enter'));
      checks.push('sequential Tab/Enter discloses backup, not row');
    } else {
      const rect = await badge.boundingBox();
      assert(rect.height >= 44 && rect.width >= 44);
      await noExtraReads(() => activate(badge));
      checks.push('ordinary first tap on 44px badge discloses backup, not row');
    }
    await settle();
    await dialog.getByText('Guest-agent reads: Reported available', { exact: true }).waitFor();
    assert(
      (await dialog.innerText()).includes(
        'does not independently confirm thaw or filesystem writes',
      ),
    );
    assert.equal(await dialog.getByRole('link').count(), 0);
    const time = await dialog.locator('time').getAttribute('datetime');
    assert.equal(time, '2026-10-13T00:30:00.000Z');
    await capture('available');
    checks.push('fresh completion and reported availability remain distinct from thaw/write proof');
    await refetch({
      lock: 'backup',
      backupInProgress: true,
      guestAgentStatus: 'available',
      diskStatusReason: 'prev-vm-locked',
    });
    await dialog.getByText('Guest-agent reads: Deferred', { exact: true }).waitFor();
    await settle();
    assert(
      (await dialog.innerText()).includes('An OK backup or a running VM does not prove thaw.'),
    );
    assert.equal(await dialog.locator('time').getAttribute('datetime'), time);
    assert(
      (await dialog.innerText()).includes('Using last known disk stats.') === false,
      'the caution is about current requests, not inherited disk figures',
    );
    await capture('locked');
    checks.push('same open disclosure follows lock and running backup without renewing completion');
    if (spec.name === 'desktop') {
      await close().click();
      await dialog.waitFor({ state: 'hidden' });
      await badge.hover();
      const tooltip = page.locator('[data-tooltip-portal="true"]');
      await tooltip.waitFor({ state: 'visible' });
      assert(
        (await tooltip.innerText()).includes('An OK backup or a running VM does not prove thaw.'),
      );
      assert.equal(
        await tooltip.getByRole('link').count(),
        0,
        'hover has no unreachable interaction',
      );
      const rect = await tooltip.boundingBox();
      assert(
        rect.x >= 0 &&
          rect.y >= 0 &&
          rect.x + rect.width <= spec.width &&
          rect.y + rect.height <= spec.height,
        'safety hover fits viewport',
      );
      await capture('hover-locked');
      await activate(badge);
      await settle();
      checks.push(
        'fine-pointer hover preserves the same precaution, without a dead interactive link',
      );
    }

    // Follow the actual existing guidance link through the shipped Docs renderer.
    const link = dialog.getByRole('link', { name: 'Backup safety guidance' });
    assert.equal(await link.getAttribute('href'), '/docs/VM_DISK_MONITORING');
    if (spec.name === 'desktop') {
      await close().focus();
      await page.keyboard.press('Tab');
      assert(await link.evaluate((el) => document.activeElement === el));
      await page.keyboard.press('Enter');
    } else await link.tap();
    await page.waitForURL(origin + '/docs/VM_DISK_MONITORING');
    await page.getByRole('heading', { name: 'Backup safety', exact: true }).waitFor();
    assert(
      (await page.locator('main').innerText()).includes(
        'Pulse monitoring and alerts are unavailable while it',
      ),
    );
    assert((await page.locator('main').innerText()).includes('does not prove thaw succeeded'));
    assert.equal(await page.locator('[data-dialog-layer]').count(), 0);
    await page
      .getByRole('heading', { name: 'Backup safety', exact: true })
      .scrollIntoViewIfNeeded();
    await capture('guidance');
    checks.push(
      'keyboard/touch link opens actual shipped safety guidance and releases dialog isolation',
    );
    await page.goBack();
    await badge.waitFor();
    await activate(badge);
    await settle();
    // A fresh backup/absent lock/reporting agent must not clear uncertain request completion.
    await refetch({
      lock: '',
      backupInProgress: false,
      guestAgentStatus: 'available',
      diskStatusReason: 'prev-agent-timeout',
    });
    await dialog.getByText(/Guest request timed out\. Completion is uncertain\./).waitFor();
    assert(
      (await dialog.innerText()).includes(
        'Confirm thaw and writes to the filesystems covered by the backup independently.',
      ),
    );
    assert.equal(await dialog.locator('time').getAttribute('datetime'), time);
    const mapped = await page.evaluate(() => window.__backupSafety.evidence());
    assert.equal(mapped.agentKind, 'pulse');
    assert.equal(mapped.agentStale, undefined); // the adapter omits a false stale flag
    assert.equal(mapped.agentVersion, '6.5.0');
    await settle();
    await capture('uncertain');
    checks.push('unlocked fresh backup plus reporting Pulse Agent cannot erase QGA uncertainty');
    await noExtraReads(() => page.keyboard.press('Escape'));
    await dialog.waitFor({ state: 'hidden' });
    assert(await badge.evaluate((el) => document.activeElement === el));
    await page.getByRole('button', { name: 'Hide Backup column' }).click();
    const indicator = row.getByRole('button', { name: /^Last backup:/ });
    await noExtraReads(() => activate(indicator));
    await settle();
    assert((await dialog.innerText()).includes('Completion is uncertain.'));
    assert.equal(await dialog.locator('time').getAttribute('datetime'), time);
    await capture('hidden-column');
    checks.push(
      'hidden-column shield shows identical safety/completion evidence without row dispatch',
    );
    await refetch({
      guestAgentStatus: 'https://synthetic.invalid/private=fixture',
      diskStatusReason: 'unrecognised-fixture-reason',
    });
    await dialog.getByText('Guest-agent reads: Unknown', { exact: true }).waitFor();
    assert(!(await dialog.innerText()).includes('synthetic.invalid'));
    assert(!(await dialog.innerText()).includes('unrecognised-fixture-reason'));
    assert(!(await dialog.innerText()).includes('Completion is uncertain.'));
    await refetch({ guestAgentStatus: 'available', diskStatusReason: '' });
    await dialog.getByText('Guest-agent reads: Reported available', { exact: true }).waitFor();
    assert.equal(await dialog.getByRole('link').count(), 0);
    assert(!(await dialog.innerText()).includes('Completion is uncertain.'));
    checks.push(
      'unknown raw values stay private; replacement availability withdraws old caution, never proves thaw',
    );
    await activate(close());
    await dialog.waitFor({ state: 'hidden' });
    assert(await indicator.evaluate((el) => document.activeElement === el));
    const peer = page
      .getByRole('region', { name: 'Guest 102 row' })
      .getByRole('button', { name: /^Last backup:/ });
    await activate(peer);
    const peerDialog = page.getByRole('dialog', { name: 'Backup status details for healthy-peer' });
    await peerDialog.getByText('Guest-agent reads: Reported available', { exact: true }).waitFor();
    assert(!(await peerDialog.innerText()).includes('Completion is uncertain.'));
    await peerDialog.getByRole('button', { name: 'Close backup details' }).click();
    const container = page
      .getByRole('region', { name: 'Guest 103 row' })
      .getByRole('button', { name: /^Backup running now/ });
    await activate(container);
    const ctDialog = page.getByRole('dialog', {
      name: 'Backup status details for linux-container',
    });
    await ctDialog.waitFor();
    assert(!(await ctDialog.innerText()).includes('Guest-agent reads:'));
    await ctDialog.getByRole('button', { name: 'Close backup details' }).click();
    assert.equal(
      await page
        .getByRole('region', { name: 'Guest 104 row' })
        .getByRole('button', { name: /backup/i })
        .count(),
      0,
    );
    checks.push('peer, LXC and vSphere retain independent interpretation and provider scope');
    await activate(indicator);
    await settle();
    primaryPresent = false;
    await page.evaluate(() => window.__backupSafety.refetch());
    await dialog.waitFor({ state: 'hidden' });
    assert.equal(await page.locator('[data-dialog-layer]').count(), 0);
    assert.equal(
      await page.locator('main').evaluate((el) => el.closest('body > *').hasAttribute('inert')),
      false,
    );
    assert.equal(await page.evaluate(() => document.body.style.overflow), '');
    checks.push('resource removal unmounts disclosure and restores background scrolling/inertness');
    assert.equal(await page.evaluate(() => window.__backupSafety.actions()), 0);
    await page
      .getByRole('region', { name: 'Guest 102 row' })
      .getByText('healthy-peer', { exact: true })
      .click();
    assert.equal(await page.evaluate(() => window.__backupSafety.actions()), 1);
    assert.deepEqual(errors, []);
    assert(
      !queries.some((query) => /\/agent|probe|diagnostic|backup|history/.test(query)),
      'no collector, history or live-probe reads',
    );
    assert(
      await page.locator('main').evaluate((el) => el.scrollWidth <= window.innerWidth),
      'no horizontal overflow',
    );
    result.results.push({
      name: spec.name,
      browser: browser.version(),
      viewport: { width: spec.width, height: spec.height },
      colorScheme: spec.colorScheme,
      checks,
      mappedEvidence: evidence,
      queries,
      errors,
    });
    await page.close();
    await browser.close();
    browser = undefined;
  }
  result.result = 'passed';
})()
  .catch(async (error) => {
    result.result = 'failed';
    result.errors.push(error.stack || String(error));
    process.exitCode = 1;
    if (browser) {
      const pages = browser.contexts().flatMap((context) => context.pages());
      if (pages[0])
        await pages[0]
          .screenshot({ path: path.join(output, 'failure.png'), fullPage: true })
          .catch(() => {});
    }
  })
  .finally(async () => {
    if (browser) await browser.close();
    result.cleanup.browserClosed = true;
    if (server)
      await new Promise((resolve, reject) =>
        server.close((error) => (error ? reject(error) : resolve())),
      );
    result.cleanup.serverClosed = true;
    fs.writeFileSync(path.join(output, 'result.json'), JSON.stringify(result, null, 2) + '\n');
    console.log(
      JSON.stringify({
        result: result.result,
        viewports: result.results.length,
        checks: result.results.reduce((count, entry) => count + entry.checks.length, 0),
        captures: result.captures.length,
        errors: result.errors,
        cleanup: result.cleanup,
      }),
    );
  });
