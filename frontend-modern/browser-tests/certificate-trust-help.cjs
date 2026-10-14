// Exercise certificate guidance through the shipped Docs renderer, router and CSS.
// No TLS handshake, API request, credential edit or platform operation is performed.
const fs = require('node:fs');
const path = require('node:path');
const http = require('node:http');
const crypto = require('node:crypto');
const assert = require('node:assert/strict');
const { chromium, webkit } = require('playwright');
const root = '/workspace/frontend-modern';
const preview = '/workspace/tmp/doc-command-help/static';
const output = '/workspace/tmp/certificate-trust-help';
const digest = bytes => crypto.createHash('sha256').update(bytes).digest('hex');
const bindingBytes = fs.readFileSync(path.join(preview, 'binding.json'));
const binding = JSON.parse(bindingBytes);
const result = { result: 'incomplete', source_sha: binding.source_sha,
  binding_sha256: digest(bindingBytes), playwright: require('playwright/package.json').version,
  cases: [], captures: [], errors: [], served: [], cleanup: {},
  limits: 'Changed help rendering/navigation only; not native TLS, installed trust, release inclusion or workload recovery.' };

(async () => {
  let server, browser;
  try {
    fs.mkdirSync(output, { recursive: true });
    assert.equal(result.playwright, '1.56.1');
    assert.equal(result.playwright, JSON.parse(fs.readFileSync('/workspace/tests/integration/package-lock.json'))
      .packages['node_modules/@playwright/test'].version);
    for (const [file, hash] of Object.entries(binding.sources))
      assert.equal(digest(fs.readFileSync(path.join(root, file))), hash, file);
    const assets = new Map();
    for (const [file, hash] of Object.entries(binding.artifacts)) {
      const bytes = fs.readFileSync(path.join(preview, file));
      assert.equal(digest(bytes), hash, file); assets.set(file, bytes);
    }
    for (const doc of ['TROUBLESHOOTING', 'PBS']) {
      const hash = binding.artifacts[`docs/${doc}.md`];
      assert.equal(hash, binding.sources[`public/docs/${doc}.md`]);
      assert.equal(hash, digest(fs.readFileSync(`/workspace/docs/${doc}.md`)));
    }
    result.source_inputs = Object.keys(binding.sources).length;
    server = http.createServer((req, res) => {
      if (req.method !== 'GET') { res.writeHead(405); res.end(); return; }
      const pathname = new URL(req.url, 'http://fixture.invalid').pathname;
      const file = pathname.startsWith('/docs/') && !pathname.endsWith('.md')
        ? 'browser-tests/doc-command-help.html' : pathname.slice(1);
      if (!assets.has(file)) { res.writeHead(404); res.end(); return; }
      const bytes = assets.get(file);
      result.served.push({ pathname, file, sha256: digest(bytes) });
      const type = ({ '.md': 'text/markdown', '.css': 'text/css', '.js': 'text/javascript', '.html': 'text/html' })[path.extname(file)];
      res.writeHead(200, { 'Content-Type': `${type}; charset=utf-8`, 'Cache-Control': 'no-store' }); res.end(bytes);
    });
    await new Promise((resolve, reject) => {
      server.once('error', reject); server.listen(0, '127.0.0.1', resolve);
    });
    const origin = `http://127.0.0.1:${server.address().port}`;
    for (const [name, engine, width, dark] of [
      ['desktop-chromium', chromium, 1360, false], ['phone-webkit', webkit, 390, true],
    ]) {
      browser = await engine.launch(engine === chromium
        ? { headless: true, channel: 'chromium', args: ['--no-sandbox'] } : { headless: true });
      const context = await browser.newContext({ viewport: { width, height: 900 },
        isMobile: width < 500, hasTouch: width < 500, locale: 'en-GB' });
      const page = await context.newPage(); page.setDefaultTimeout(12000);
      const record = { name, width, height: 900, dark, browser: browser.version(), checks: [] };
      result.cases.push(record);
      page.on('pageerror', error => result.errors.push(error.message));
      page.on('console', message => { if (message.type() === 'error') result.errors.push(message.text()); });
      page.on('response', response => { if (!response.ok()) result.errors.push(`HTTP ${response.status()}: ${response.url()}`); });
      await page.route('**/*', route => {
        const url = new URL(route.request().url());
        assert.equal(url.origin, origin, 'no external requests permitted');
        assert(!url.pathname.startsWith('/api/'), 'help must not probe a backend');
        assert.equal(route.request().method(), 'GET');
        return route.continue();
      });
      await page.addInitScript(dark => document.addEventListener('DOMContentLoaded', () =>
        document.documentElement.classList.toggle('dark', dark)), dark);
      const capture = async suffix => {
        await page.evaluate(() => new Promise(resolve => requestAnimationFrame(() => requestAnimationFrame(resolve))));
        assert(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth), 'page overflow');
        const file = `${name}-${suffix}.png`; await page.screenshot({ path: path.join(output, file) });
        result.captures.push({ file, sha256: digest(fs.readFileSync(path.join(output, file))), name });
      };
      const activate = async link => {
        if (width < 500) await link.tap(); else { await link.focus(); await page.keyboard.press('Enter'); }
      };
      const atFragment = async id => page.waitForFunction(id => {
        const heading = document.getElementById(id);
        return heading && document.activeElement === heading && heading.getBoundingClientRect().top >= 0
          && heading.getBoundingClientRect().top < innerHeight;
      }, id);
      await page.goto(`${origin}/docs/TROUBLESHOOTING`, { waitUntil: 'domcontentloaded' });
      const entry = page.getByRole('link', { name: 'certificate trust, renewal and saved-pin conflicts', exact: true });
      await entry.waitFor();
      await page.evaluate(() => window.__certificateHelpShell = 'same-shell');
      await activate(entry);
      await page.waitForURL('**/docs/TROUBLESHOOTING#proxmox-or-pbs-certificate-validation-failed');
      await atFragment('proxmox-or-pbs-certificate-validation-failed');
      assert.equal(await page.evaluate(() => window.__certificateHelpShell), 'same-shell');
      const text = (await page.locator('article').innerText()).replace(/\s+/g, ' ');
      for (const phrase of ['A TLS error is not an invalid API token or a missing permission',
        'Endpoint URL', 'Verify SSL certificate', 'SSL Fingerprint (optional)',
        'different client and trust store', 'Keep certificate verification enabled',
        'Unknown authority or self-signed certificate', 'Expired, not-yet-valid or hostname error',
        'Renewal is one possible reason, not proof that this is the same server',
        'contradictory saved trust, not a reason to choose whichever pin connects',
        'instead of normal CA chain and hostname validation',
        'first establish the runtime’s trusted chain and matching endpoint hostname',
        'remove only that connection’s pin while keeping verification enabled',
        'do not copy the primary’s fingerprint to all members',
        'A display-name edit does not reconcile certificate trust',
        'leave it intact', 'Unknown is valid', 'Do not edit encrypted files',
        'certificate help does not establish workload recovery'])
        assert(text.includes(phrase.replaceAll('’', "'")), phrase);
      record.checks.push('symptom entry, four TLS error distinctions, CA versus leaf trust, verified renewal, per-member safety and private evidence');
      await capture('errors');
      for (const [id, suffix] of [
        ['certificate-renewal-and-saved-fingerprints', 'renewal'],
        ['cluster-members-and-conflicting-saved-trust', 'members'],
      ]) {
        await page.goto(`${origin}/docs/TROUBLESHOOTING#${id}`, { waitUntil: 'domcontentloaded' });
        await atFragment(id); await capture(suffix);
        await page.reload({ waitUntil: 'domcontentloaded' }); await atFragment(id);
      }
      record.checks.push('direct and reloaded renewal/member fragments receive visible heading focus without page overflow');
      await page.evaluate(() => window.__certificateHelpShell = 'same-shell');
      await activate(page.getByRole('link', { name: 'Getting Help', exact: true }).last());
      await page.waitForURL('**/docs/TROUBLESHOOTING#-getting-help');
      await atFragment('-getting-help');
      assert.equal(await page.evaluate(() => window.__certificateHelpShell), 'same-shell');
      await page.goBack(); await atFragment('cluster-members-and-conflicting-saved-trust');
      const backup = page.locator('article a[href="/docs/VM_DISK_MONITORING#backup-safety"]').last();
      // The preview does not include VM_DISK_MONITORING. Check the exact link
      // without following it or pretending to have proved platform recovery.
      assert.equal(await backup.count(), 1);
      await page.goto(`${origin}/docs/TROUBLESHOOTING#proxmox-or-pbs-certificate-validation-failed`, { waitUntil: 'domcontentloaded' });
      await atFragment('proxmox-or-pbs-certificate-validation-failed');
      await page.evaluate(() => window.__certificateHelpShell = 'same-shell');
      await activate(page.getByRole('link', { name: 'PBS certificate check', exact: true }));
      await page.waitForURL('**/docs/PBS#connection-failed-error'); await atFragment('connection-failed-error');
      const returnLink = page.getByRole('link', { name: 'certificate-trust checks', exact: true });
      await returnLink.waitFor();
      const pbsText = (await returnLink.locator('..').innerText()).replace(/\s+/g, ' ');
      assert(pbsText.includes('A saved leaf pin is a different trust method from CA validation'));
      assert(pbsText.includes('do not disable verification or clear it'));
      await returnLink.evaluate(link => link.closest('p').scrollIntoView({ block: 'start' }));
      await capture('pbs-entry');
      await activate(returnLink);
      await page.waitForURL('**/docs/TROUBLESHOOTING#proxmox-or-pbs-certificate-validation-failed');
      await atFragment('proxmox-or-pbs-certificate-validation-failed');
      assert.equal(await page.evaluate(() => window.__certificateHelpShell), 'same-shell');
      record.checks.push('keyboard/touch Getting Help and PBS round-trip links, focused destinations and browser back');
      await context.close(); await browser.close(); browser = null;
    }
    assert.equal(result.errors.length, 0); result.result = 'passed';
  } catch (error) { result.failure = error.stack; throw error; }
  finally {
    if (browser) await browser.close(); result.cleanup.browser_closed = true;
    if (server) await new Promise((resolve, reject) => {
      server.close(error => error ? reject(error) : resolve()); server.closeAllConnections();
    }); result.cleanup.server_closed = true;
    fs.writeFileSync(path.join(output, 'result.json'), JSON.stringify(result, null, 2) + '\n');
  }
})().catch(error => { console.error(error); process.exitCode = 1; });
