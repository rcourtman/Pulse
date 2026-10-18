const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const crypto = require('node:crypto');
const { chromium, webkit } = require('playwright');
const root = '/workspace/frontend-modern';
const output = '/workspace/tmp/filesystem-read-provenance/browser-final';
const origin = 'http://127.0.0.1:5324';
const runtime = [
  'frontend-modern/src/components/Workloads/GuestDrawerOverview.tsx',
  'frontend-modern/src/components/Workloads/diskListModel.ts',
];
const hash = file => crypto.createHash('sha256').update(fs.readFileSync(file)).digest('hex');
const settle = page => page.evaluate(async () => {
  await document.fonts.ready;
  await new Promise(resolve => requestAnimationFrame(() => requestAnimationFrame(resolve)));
});
(async () => {
  fs.mkdirSync(output, { recursive: true });
  const result = {
    result: 'incomplete', playwright: require('playwright/package.json').version,
    runtime_hashes: Object.fromEntries(runtime.map(file => [file, hash('/workspace/' + file)])),
    cases: [], screenshots: [], cleanup: { browser: false, server: false },
    limits: 'Real GuestRow/full GuestDrawer/DetailSectionTable and CSS; synthetic guest observations and APIs. No native QGA, backup, cause, thaw, writes, installed/reporter or release acceptance.',
  };
  assert.equal(result.playwright, JSON.parse(fs.readFileSync('/workspace/tests/integration/package-lock.json')).packages['node_modules/@playwright/test'].version);
  process.chdir(root);
  const { createServer } = await import(path.join(root, 'node_modules/vite/dist/node/index.js'));
  const server = await createServer({root, configFile: path.join(root, 'vite.config.ts'), cacheDir: output + '/cache', server: { host: '127.0.0.1', port: 5324, strictPort: true }});
  let browser;
  try {
    await server.listen();
    for (const [name, engine, width, dark] of [
      ['chromium-desktop-light', chromium, 1365, false],
      ['chromium-desktop-dark', chromium, 1365, true],
      ['webkit-phone-dark', webkit, 390, true],
      ['webkit-narrow-light', webkit, 320, false],
    ]) {
      browser = await engine.launch(engine === chromium ? { headless: true, channel: 'chromium', args: ['--no-sandbox'] } : { headless: true });
      const context = await browser.newContext({viewport: { width, height: width <= 768 ? 844 : 900 }, isMobile: width <= 768, hasTouch: width <= 768, locale: 'en-GB', timezoneId: 'UTC'});
      const page = await context.newPage();
      page.setDefaultTimeout(15000);
      page.setDefaultNavigationTimeout(60000);
      const record = {name, browser: browser.version(), width, dark, errors: [], requests: [], checks: [], states: []};
      result.cases.push(record);
      page.on('pageerror', error => record.errors.push(error.message));
      await page.route('**/*', route => {
        const request = route.request(); const url = new URL(request.url());
        if (url.origin !== origin) { record.requests.push({ method: request.method(), path: url.pathname, disposition: 'off-origin blocked' }); return route.abort(); }
        if (url.pathname.startsWith('/api/')) {
          record.requests.push({ method: request.method(), path: url.pathname, disposition: 'synthetic' });
          return route.fulfill({status: 200, contentType: 'application/json', body: JSON.stringify({ success: true, data: [], alerts: [], anomalies: [], config: null })});
        }
        return route.continue();
      });
      await page.addInitScript(isDark => {
        document.addEventListener('DOMContentLoaded', () => { if (isDark) document.documentElement.classList.add('dark'); });
        window.__trustedInput = [];
        document.addEventListener('click', event => window.__trustedInput.push({ type: 'click', trusted: event.isTrusted }), true);
        document.addEventListener('keydown', event => window.__trustedInput.push({ type: event.key, trusted: event.isTrusted }), true);
      }, dark);
      await page.goto(`${origin}/browser-tests/guest-disk-provenance.html`);
      await page.waitForFunction(() => window.__guestDiskProvenance);
      const apply = async next => { await page.evaluate(next => window.__guestDiskProvenance.apply(next), next); await settle(page); };
      await apply({ reason: 'prev-vm-locked', data: true, usage: 90, lock: 'backup', kind: 'qemu' });
      const row = page.locator('[data-guest-id="fixture-pve:pve-a:101"]');
      const disclosure = row.locator('[data-row-action]');
      if (width <= 768) await row.locator('[data-workload-disk-read-status]').tap();
      else { await disclosure.focus(); await page.keyboard.press('Enter'); }
      const drawer = page.getByRole('region', {name: 'Guest details'});
      await drawer.waitFor({state: 'visible'});
      const technical = drawer.getByTestId('guest-technical-details');
      const filesystem = technical.getByText('/data', {exact: true}).locator('xpath=ancestor::tr');
      const progress = filesystem.getByRole('progressbar');
      const text = async () => (await filesystem.innerText()).replace(/\s+/g, ' ').trim();
      const capture = async state => {
        await technical.scrollIntoViewIfNeeded(); await settle(page);
        const file = `${name}-${state}.png`;
        await technical.screenshot({path: path.join(output, file)});
        result.screenshots.push({file, sha256: hash(path.join(output, file)), name, state});
      };
      assert.match(await text(), /Last known 90% · 9\.00 GB\/10\.0 GB · EXT4/);
      assert.equal(await progress.count(), 0);
      record.checks.push('Retained quantitative values are labelled last known beside their filesystem identity, with no current utilization bar.');
      record.states.push({state: 'retained', text: await text()});
      await capture('retained');
      await apply({ lock: '' });
      assert.match(await text(), /Last known 90%/); assert.equal(await progress.count(), 0);
      record.checks.push('Clearing only the operation lock cannot promote a deferred reading.');
      const reasons = await page.evaluate(() => window.__guestDiskProvenance.deferrals.map(item => item[0]));
      for (const reason of reasons) {
        await apply({reason: 'prev-' + reason});
        assert.match(await text(), /Last known 90%/); assert.equal(await progress.count(), 0);
        await apply({reason});
        assert.match(await text(), /Usage unavailable · \?\/10\.0 GB · EXT4/);
        assert.doesNotMatch(await text(), /90%|9\.00 GB/); assert.equal(await progress.count(), 0);
      }
      record.checks.push('All eight fixed read reasons distinguish retained data from unavailable numeric carriers.');
      record.states.push({state: 'unavailable', text: await text()});
      await capture('unavailable');
      await apply({reason: '', usage: 30, lock: ''});
      assert.match(await text(), /30% · 3\.00 GB\/10\.0 GB · EXT4/);
      assert.equal(await progress.getAttribute('aria-valuenow'), '30');
      assert.equal(await progress.getAttribute('aria-label'), 'Filesystem /data utilization');
      assert.equal(await drawer.isVisible(), true);
      record.checks.push('A same-guest fresh read restores the correct numeric value and accessible utilization bar without closing the drawer.');
      record.states.push({state: 'fresh', text: await text()});
      await capture('fresh');
      await apply({reason: 'prev-vm-locked', kind: 'lxc'});
      assert.match(await text(), /30% · 3\.00 GB\/10\.0 GB · EXT4/); assert.equal(await progress.getAttribute('aria-valuenow'), '30');
      record.checks.push('Independent LXC filesystem evidence does not inherit a VM-only reason.');
      await apply({kind: 'qemu', data: false, reason: 'vm-locked'});
      assert.equal(await technical.getByText('/data', {exact: true}).count(), 0);
      assert.match(await technical.innerText(), /Guest reads paused while Proxmox reports a VM operation lock/);
      record.checks.push('With no prior filesystem sample, the readable explanation remains and no filesystem value is invented.');
      const overflow = await technical.evaluate(el => ({width: el.clientWidth, scrollWidth: el.scrollWidth, documentWidth: document.documentElement.clientWidth, documentScroll: document.documentElement.scrollWidth}));
      assert.ok(overflow.scrollWidth <= overflow.width + 1, JSON.stringify(overflow));
      record.layout = overflow;
      await drawer.getByRole('button', {name: 'Collapse backup-guest details'}).click();
      assert.equal(await drawer.count(), 0);
      record.input = await page.evaluate(() => window.__trustedInput);
      assert.ok(record.input.some(event => event.trusted));
      assert.deepEqual(record.errors, []);
      assert.equal(record.requests.filter(request => !['GET','HEAD'].includes(request.method)).length, 0);
      record.checks.push('Trusted keyboard/touch disclosure and close work; no uncaught page error, overflow or mutating request.');
      await context.close(); await browser.close(); browser = null;
    }
    result.result = 'passed';
  } catch (error) {
    result.error = String(error.stack || error); throw error;
  } finally {
    if (browser) await browser.close(); result.cleanup.browser = true;
    await server.close(); result.cleanup.server = true;
    fs.writeFileSync(path.join(output, 'result.json'), JSON.stringify(result, null, 2) + '\n');
  }
})().catch(error => { console.error(error); process.exitCode = 1; });
