// Exercise final combined Settings consumers, with synthetic GETs only.
const { chromium, webkit, expect } = require('playwright/test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const http = require('node:http');
const crypto = require('node:crypto');
const root = path.resolve('frontend-modern');
const output = path.resolve('tmp/settings-composition/browser');
const compiled = path.resolve('tmp/settings-composition/static');
const hash = bytes => crypto.createHash('sha256').update(bytes).digest('hex');
module.exports = (async () => {
  fs.mkdirSync(output, { recursive: true });
  const result = { result: 'incomplete', playwright: require('playwright/package.json').version,
    profiles: [], captures: [], cleanup: { browsers: 0, closed: 0, serverClosed: false },
    scope: 'Static fixture around real Settings owners and consumers. No API write, token mint, copied command execution, target host, installed trust or recovery.' };
  let server, browser;
  try {
    assert.equal(result.playwright, JSON.parse(fs.readFileSync('tests/integration/package-lock.json')).packages['node_modules/@playwright/test'].version);
    const binding = JSON.parse(fs.readFileSync(path.join(compiled, 'binding.json')));
    result.binding = { source_sha: binding.source_sha, product_graph: binding.product_graph, preview_tools_graph: binding.preview_tools_graph, tool_versions: binding.tool_versions,
      sha256: hash(fs.readFileSync(path.join(compiled, 'binding.json'))) };
    for (const [file, digest] of Object.entries(binding.sources)) assert.equal(hash(fs.readFileSync(path.join(root, file))), digest, file);
    const assets = new Map(Object.entries(binding.artifacts).map(([file, digest]) => {
      const bytes = fs.readFileSync(path.join(compiled, file)); assert.equal(hash(bytes), digest, file); return ['/' + file, bytes];
    }));
    server = http.createServer((req, res) => {
      const pathname = new URL(req.url, 'http://127.0.0.1').pathname;
      const file = pathname === '/settings/infrastructure' ? '/browser-tests/settings-command-composition.html' : pathname;
      const bytes = assets.get(file);
      if (req.method !== 'GET' || !bytes) { res.writeHead(404); return res.end(); }
      res.setHeader('Content-Type', file.endsWith('.js') ? 'text/javascript' : file.endsWith('.css') ? 'text/css' : 'text/html');
      res.setHeader('Cache-Control', 'no-store'); res.end(bytes);
    });
    await new Promise((resolve, reject) => { server.once('error', reject); server.listen(0, '127.0.0.1', resolve); });
    const origin = `http://127.0.0.1:${server.address().port}`;
    for (const [name, engine, width, dark] of [['desktop', chromium, 1365, false], ['narrow', webkit, 320, true]]) {
      browser = await engine.launch(engine === chromium ? { headless: true, channel: 'chromium', args: ['--no-sandbox'] } : { headless: true });
      result.cleanup.browsers++;
      const context = await browser.newContext({ viewport: { width, height: 900 }, hasTouch: width === 320, isMobile: width === 320, locale: 'en-GB', timezoneId: 'UTC', reducedMotion: 'reduce' });
      const page = await context.newPage(); page.setDefaultTimeout(10000);
      const profile = { name, width, dark, browser: browser.version(), checks: [], reads: [], writes: [], unexpected: [], errors: [] };
      result.profiles.push(profile);
      page.on('pageerror', error => profile.errors.push(error.message));
      await page.addInitScript(dark => {
        document.addEventListener('DOMContentLoaded', () => document.documentElement.classList.toggle('dark', dark), { once: true });
        window.fixtureCopies = [];
        Object.defineProperty(navigator, 'clipboard', { value: { writeText: async value => { window.fixtureCopies.push(value); } } });
      }, dark);
      await page.route('**/*', async route => {
        const req = route.request(), url = new URL(req.url());
        if (url.origin !== origin) { profile.unexpected.push(url.origin); return route.abort(); }
        if (!url.pathname.startsWith('/api/')) return route.continue();
        if (req.method() !== 'GET') { profile.writes.push(req.method() + ' ' + url.pathname); return route.abort(); }
        profile.reads.push(url.pathname);
        if (url.pathname === '/api/security/status') return route.fulfill({ json: { requiresAuth: false, apiTokenConfigured: false, agentUrl: 'https://pulse.example.invalid' } });
        if (url.pathname === '/api/state') return route.fulfill({ json: { connectedInfrastructure: [] } });
        profile.unexpected.push(url.pathname); return route.abort();
      });
      const act = async locator => { await locator.scrollIntoViewIfNeeded(); if (width === 320) await locator.tap(); else { await locator.focus(); await page.keyboard.press('Space'); } };
      const capture = async (state, locator) => {
        await locator.evaluate(el => window.scrollTo(0, Math.max(0, el.getBoundingClientRect().top + window.scrollY - 16)));
        await page.evaluate(async () => { await document.fonts.ready; await new Promise(r => requestAnimationFrame(() => requestAnimationFrame(r))); });
        const geometry = await page.evaluate(() => ({ page: document.documentElement.scrollWidth, viewport: document.documentElement.clientWidth }));
        assert(geometry.page <= geometry.viewport + 1, JSON.stringify(geometry));
        const file = `${name}-${state}.png`; await page.screenshot({ path: path.join(output, file) });
        result.captures.push({ file, sha256: hash(fs.readFileSync(path.join(output, file))), geometry });
      };
      await page.goto(origin + '/settings/infrastructure');
      const commands = page.getByRole('checkbox', { name: 'Allow Pulse to run commands on this host', exact: true });
      await expect(commands).toBeVisible(); await expect(commands).not.toBeChecked();
      await expect(page.getByText('Monitoring only. Pulse cannot run commands or change containers on this host.', { exact: true })).toBeVisible();
      assert.equal(await page.getByRole('button', { name: 'Copy Install command', exact: true }).count(), 0);
      await capture('monitoring-before-token', commands);
      await act(commands); await expect(commands).toBeChecked();
      await expect(page.getByText('Pulse can run commands as root on this host, including container updates and Patrol actions.', { exact: true })).toBeVisible();
      await act(page.getByRole('button', { name: 'Confirm without token', exact: true }));
      const linuxCommand = () => page.getByRole('button', { name: 'Copy Install command', exact: true }).locator('..').locator('code');
      await expect(linuxCommand()).toContainText(' --enable-commands');
      const policy = page.getByRole('status', { name: 'Installation transport policy' });
      await expect(policy).toContainText('HTTPS certificate verification requested');
      await act(commands); await expect(linuxCommand()).not.toContainText(' --enable-commands');
      profile.checks.push('Command access defaults off before any token; keyboard/touch toggles actual summary and command producer without minting.');
      await act(page.getByRole('button', { name: 'Show advanced connection and install options', exact: true }));
      assert.equal(await page.getByRole('checkbox').count(), 2, 'Only command access and TLS bypass checkboxes');
      await page.getByLabel('Connection URL (Agent → Pulse)', { exact: true }).fill('http://pulse.example.invalid:7655');
      await expect(policy).toContainText('HTTP does not protect this installation');
      const skip = page.getByRole('checkbox', { name: 'Skip TLS certificate verification (unsafe)', exact: true });
      await expect(skip).not.toBeChecked();
      await act(page.getByRole('button', { name: 'Hide advanced connection and install options', exact: true }));
      await capture('http-policy', policy);
      profile.checks.push('Combined transport warning remains visible without advanced options; unchecked bypass does not promise HTTPS.');
      await page.goto(origin + '/settings/infrastructure?focus=docker');
      const note = page.getByText('Docker inside Proxmox LXCs', { exact: true }).locator('..');
      await expect(note).toContainText('Allow Pulse to run commands on this host');
      await expect(note).toContainText('Discover Docker in LXC guests');
      await expect(note).toContainText('not a monitoring-only install');
      await capture('docker-command-guidance', note);
      profile.checks.push('Docker-LXC guidance preserves command-authority and independent server opt-in disclosures with the shared option name.');
      await page.goto(origin + '/settings/infrastructure?surface=node');
      await act(page.getByRole('button', { name: 'Host Telemetry Agent', exact: true }));
      const setupNote = page.locator('p').filter({ has: page.getByText('Docker inside Proxmox LXCs:', { exact: true }) });
      await expect(setupNote).toContainText('Allow Pulse to run commands on this host');
      await expect(setupNote).toContainText('this monitoring-only setup does not grant the command access');
      await expect(setupNote).toContainText('Discover Docker in LXC guests');
      await expect(page.getByRole('button', { name: 'Copy command', exact: true })).toBeDisabled();
      await capture('node-monitoring-guide', setupNote);
      await act(page.getByRole('button', { name: 'Manual Token Setup', exact: true }));
      await expect(page.getByRole('textbox', { name: 'Token ID' })).toBeVisible();
      await act(page.getByRole('button', { name: /Connect via API/ }));
      await expect(page.getByText('API inventory', { exact: true })).toBeVisible();
      profile.checks.push('Actual node setup switches agent/manual/API strategies; combined note retains least privilege and source policy; disabled synthetic handoff cannot mint.');
      assert.deepEqual(profile.writes, []); assert.deepEqual(profile.unexpected, []); assert.deepEqual(profile.errors, []);
      assert.deepEqual(await page.evaluate(() => window.fixtureCopies), []);
      await context.close(); await browser.close(); browser = null; result.cleanup.closed++;
    }
    result.result = 'passed';
  } catch (error) { result.error = String(error.stack || error); throw error; }
  finally {
    if (browser) { await browser.close(); result.cleanup.closed++; }
    if (server) { await new Promise(resolve => server.close(resolve)); result.cleanup.serverClosed = true; }
    fs.writeFileSync(path.join(output, 'result.json'), JSON.stringify(result, null, 2) + '\n');
    console.log(JSON.stringify({ result: result.result, profiles: result.profiles.length, captures: result.captures.length }));
  }
})();
