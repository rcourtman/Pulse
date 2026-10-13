// Same consuming journey before and after the action-only repair. The parent
// mode reports unmet accessibility criteria, not a passing repair proof.
const fs = require('node:fs');
const path = require('node:path');
const http = require('node:http');
const crypto = require('node:crypto');
const assert = require('node:assert/strict');
const { chromium, webkit } = require('playwright');
const root = '/workspace';
const frontend = root + '/frontend-modern';
const origin = 'http://127.0.0.1:5326';
const hash = (bytes) => crypto.createHash('sha256').update(bytes).digest('hex');

async function presentation(action) {
  return action.evaluate((element) => {
    const rgba = (colour) => {
      const values = colour.match(/[\d.]+/g).map(Number);
      return [...values.slice(0, 3), values[3] ?? 1];
    };
    const luminance = (colour) => {
      const rgb = colour.slice(0, 3).map((channel) => {
        const value = channel / 255;
        return value <= 0.04045 ? value / 12.92 : ((value + 0.055) / 1.055) ** 2.4;
      });
      return rgb[0] * 0.2126 + rgb[1] * 0.7152 + rgb[2] * 0.0722;
    };
    const contrast = (a, b) => {
      const values = [luminance(a), luminance(b)].sort((x, y) => y - x);
      return (values[0] + 0.05) / (values[1] + 0.05);
    };
    const backgrounds = [];
    for (let node = element; node; node = node.parentElement) {
      const background = rgba(getComputedStyle(node).backgroundColor);
      backgrounds.push(background);
      if (background[3] === 1) break;
    }
    const background = backgrounds.reverse().reduce((base, layer) =>
      layer.slice(0, 3).map((value, index) => value * layer[3] + base[index] * (1 - layer[3])),
    [255, 255, 255]);
    const style = getComputedStyle(element), bounds = element.getBoundingClientRect();
    return {
      width: bounds.width, height: bounds.height, top: bounds.top, bottom: bounds.bottom,
      viewport_height: window.innerHeight, focused: element === document.activeElement,
      focus_visible: element.matches(':focus-visible'), outline_style: style.outlineStyle,
      outline_width: style.outlineWidth, outline_offset: style.outlineOffset,
      colour: style.color, outline_colour: style.outlineColor, background,
      text_contrast: contrast(rgba(style.color), background),
      outline_contrast: contrast(rgba(style.outlineColor), background),
      underlined: style.textDecorationLine.includes('underline'),
      document_width: document.documentElement.clientWidth,
      document_scroll: document.documentElement.scrollWidth,
      client_width: element.clientWidth, scroll_width: element.scrollWidth,
    };
  });
}

async function run(variant) {
  assert(['parent', 'final'].includes(variant));
  const output = root + '/tmp/inline-notice-access/' + variant + '-browser';
  const compiled = root + '/tmp/inline-notice-access/' + variant + '-static';
  fs.mkdirSync(output, { recursive: true });
  const binding = JSON.parse(fs.readFileSync(compiled + '/binding.json'));
  assert.equal(binding.variant, variant);
  const result = {
    result: 'incomplete', variant, source: binding.source_sha, binding,
    driver_sha256: hash(fs.readFileSync(__filename)),
    playwright: require('playwright/package.json').version, cases: [], captures: [], served: {},
    cleanup: { browser: false, server: false },
    limits: 'Synthetic canonical observations and API replies through actual producer/drawers/recovery panels/Docs. No guest, backup, diagnostic, native recovery, deployed access, release or reporter acceptance.',
  };
  let server, browser;
  try {
    assert.match(result.source, /^[a-f0-9]{40}$/);
    assert.equal(result.playwright, JSON.parse(fs.readFileSync(root + '/tests/integration/package-lock.json')).packages['node_modules/@playwright/test'].version);
    for (const [file, digest] of Object.entries(binding.sources)) {
      // The parent variant is compiled in the proof VM from an explicit,
      // hash-bound original primitive input. Do not replace assigned source
      // or pretend the final primitive supplied those parent pixels.
      const input = variant === 'parent' && file === 'src/components/shared/InlineNotice.tsx'
        ? root + '/tmp/inline-notice-access/parent-runtime.tsx'
        : frontend + '/' + file;
      assert.equal(hash(fs.readFileSync(input)), digest, file);
    }
    const docHash = hash(fs.readFileSync(root + '/docs/VM_DISK_MONITORING.md'));
    assert.equal(binding.artifacts['docs/VM_DISK_MONITORING.md'], docHash);
    result.doc_sha256 = docHash;
    const assets = new Map();
    for (const [file, digest] of Object.entries(binding.artifacts)) {
      const bytes = fs.readFileSync(compiled + '/' + file);
      assert.equal(hash(bytes), digest, file);
      assets.set('/' + file, bytes);
    }
    server = http.createServer((request, response) => {
      if (!['GET', 'HEAD'].includes(request.method)) { response.writeHead(405); return response.end(); }
      let pathname = new URL(request.url, origin).pathname;
      if (['/', '/recovery-fixture', '/docs/VM_DISK_MONITORING'].includes(pathname)) pathname = '/browser-tests/inline-notice-access.html';
      const bytes = assets.get(pathname);
      if (!bytes) { response.writeHead(404); return response.end(); }
      result.served[pathname] = hash(bytes);
      response.setHeader('Content-Type', { '.html': 'text/html', '.js': 'text/javascript', '.css': 'text/css', '.md': 'text/markdown; charset=utf-8' }[path.extname(pathname)] || 'application/octet-stream');
      response.setHeader('Cache-Control', 'no-store');
      response.setHeader('Content-Length', bytes.length);
      response.end(request.method === 'HEAD' ? undefined : bytes);
    });
    await new Promise((resolve, reject) => { server.once('error', reject); server.listen(5326, '127.0.0.1', resolve); });

    for (const [engine, width, theme] of [
      [chromium, 1365, 'light'], [chromium, 1365, 'dark'],
      [webkit, 390, 'light'], [webkit, 390, 'dark'],
    ]) {
      const name = `${engine === chromium ? 'chromium' : 'webkit'}-${width}-${theme}`;
      browser = await engine.launch(engine === chromium ? { headless: true, channel: 'chromium', args: ['--no-sandbox'] } : { headless: true });
      const context = await browser.newContext({ viewport: { width, height: 900 }, isMobile: width < 768, hasTouch: width < 768, colorScheme: theme, locale: 'en-GB', reducedMotion: 'reduce' });
      await context.addInitScript((theme) => {
        window.__noticeInputs = [];
        document.addEventListener('click', (event) => window.__noticeInputs.push({ trusted: event.isTrusted, label: event.target.closest('a,button')?.textContent }), true);
        document.addEventListener('DOMContentLoaded', () => document.documentElement.classList.toggle('dark', theme === 'dark'));
      }, theme);
      const record = { name, browser: browser.version(), width, theme, requests: [], errors: [], actions: [], criteria: [] };
      result.cases.push(record);
      const page = await context.newPage();
      page.setDefaultTimeout(10000);
      page.on('pageerror', (error) => record.errors.push(error.message));
      const readErrors = { roles: true, users: true };
      await page.route('**/*', (route) => {
        const request = route.request(), url = new URL(request.url());
        record.requests.push({ method: request.method(), path: url.pathname, external: url.origin !== origin });
        assert.equal(url.origin, origin, 'no external requests');
        assert.equal(request.method(), 'GET', 'no mutations');
        assert(!/\/(?:diagnostics|run-command|guest-agent|qemu)(?:\/|$)/.test(url.pathname), 'no live probes');
        if (!url.pathname.startsWith('/api/')) return route.continue();
        if (url.pathname === '/api/admin/roles') return readErrors.roles
          ? route.fulfill({ status: 503, json: { error: 'Synthetic read unavailable' } })
          : route.fulfill({ json: [{ id: 'observer', name: 'Fixture observer', description: 'Read-only fixture', permissions: [], isBuiltIn: true }] });
        if (url.pathname === '/api/admin/users') return readErrors.users
          ? route.fulfill({ status: 503, json: { error: 'Synthetic read unavailable' } })
          : route.fulfill({ json: [{ username: 'fixture-operator', displayName: 'Fixture operator', roleIds: ['observer'] }] });
        if (url.pathname === '/api/license/runtime-capabilities') return route.fulfill({ json: { capabilities: ['rbac'], limits: [], hosted_mode: false, max_history_days: 7, runtime: { build: 'pro' }, blocked_capabilities: [] } });
        return route.fulfill({ json: { success: true, data: [], alerts: [], anomalies: [], config: null } });
      });

      const capture = async (file, fullPage = false) => {
        await page.screenshot({ path: output + '/' + file, fullPage });
        result.captures.push({ file, sha256: hash(fs.readFileSync(output + '/' + file)) });
      };
      const inspectAction = async (action, label) => {
        // Traverse backwards then forwards using native keyboard input rather
        // than treating programmatic focus as evidence of tab navigation.
        await action.focus(); await page.keyboard.press('Shift+Tab'); await page.keyboard.press('Tab');
        await action.evaluate((element) => element.scrollIntoView({ block: 'center' }));
        await page.evaluate(async () => { await document.fonts.ready; await new Promise((resolve) => requestAnimationFrame(() => requestAnimationFrame(resolve))); });
        const metrics = await presentation(action);
        assert.equal(metrics.focused, true, `${label} tab focus`);
        assert.equal(metrics.focus_visible, true, `${label} focus visible`);
        assert.ok(metrics.text_contrast >= 4.5, `${label} text contrast`);
        assert.ok(metrics.document_scroll <= metrics.document_width + 1, `${label} document overflow`);
        assert.ok(metrics.scroll_width <= metrics.client_width + 1, `${label} action overflow`);
        const criteria = {
          label, phone_target: width >= 768 || (metrics.height >= 44 && metrics.width >= 44),
          compact_desktop: width < 768 || metrics.height === 16,
          persistent_underline: metrics.underlined,
          explicit_focus: metrics.outline_style === 'solid' && metrics.outline_width === '2px' && metrics.outline_offset === '2px' && metrics.outline_contrast >= 3,
        };
        record.actions.push({ label, ...metrics }); record.criteria.push(criteria);
        if (variant === 'final') for (const [key, value] of Object.entries(criteria)) if (key !== 'label') assert.equal(value, true, `${label}: ${key}`);
        return metrics;
      };
      const openGuest = async () => {
        await page.goto(origin, { waitUntil: 'domcontentloaded' });
        await page.waitForFunction(() => window.__noticeAccess?.evidence().projected);
        const row = page.locator('[data-guest-id="fixture:pve1:101"] [data-row-action]').first();
        if (width < 768) await page.getByText('safety-guest', { exact: true }).first().tap();
        else { await row.focus(); await page.keyboard.press('Enter'); }
        await page.getByRole('region', { name: 'Workload drawer', exact: true }).waitFor();
        record.producer = await page.evaluate(() => window.__noticeAccess.evidence());
        assert.equal(record.producer.projected.diskStatusReason, 'prev-vm-locked');
        assert.equal(record.producer.projected.lock, 'backup');
      };

      for (const [region, testId] of [['Workload drawer', 'guest-read-precaution'], ['Canonical drawer', 'resource-guest-read-precaution']]) {
        await openGuest();
        const drawer = page.getByRole('region', { name: region, exact: true });
        const notice = drawer.getByTestId(testId);
        assert.match(await notice.innerText(), /An OK backup or a running VM does not prove thaw/);
        assert.match(await notice.innerText(), /Confirm thaw and writes to the filesystems covered/);
        for (const tab of ['History', 'Manage', 'Overview']) {
          const button = drawer.getByRole('tab', { name: tab, exact: true });
          if (width < 768) await button.tap(); else { await button.focus(); await page.keyboard.press('Enter'); }
          assert.equal(await notice.isVisible(), true, `${region} notice across tabs`);
        }
        const action = notice.getByRole('link', { name: 'Backup safety guidance' });
        assert.equal(await action.getAttribute('href'), '/docs/VM_DISK_MONITORING');
        await page.evaluate(() => window.__noticeAccess.reason('prev-agent-cooldown'));
        assert.match(await notice.innerText(), /cooldown/);
        assert.equal(await action.getAttribute('href'), '/docs/VM_DISK_MONITORING');
        await page.evaluate(() => window.__noticeAccess.reason('prev-vm-locked'));
        const metrics = await inspectAction(action, region);
        assert.ok(metrics.top >= 4 && metrics.bottom <= metrics.viewport_height - 4, 'whole focus target in capture');
        await capture(`${name}-${region === 'Workload drawer' ? 'workload' : 'canonical'}-focus.png`);
        if (width < 768) await action.tap({ position: { x: 6, y: 4 } });
        else await page.keyboard.press('Enter');
        await page.locator('article').waitFor();
        assert.equal(new URL(page.url()).pathname, '/docs/VM_DISK_MONITORING');
        assert.match(await page.locator('article').innerText(), /every\s+filesystem covered by the backup/);
        // Native browser bytes bind the fetched consumer document, rather
        // than trusting text or a lossy Playwright response.body projection.
        const digest = await page.evaluate(async () => {
          const bytes = await (await fetch('/docs/VM_DISK_MONITORING.md')).arrayBuffer();
          return Array.from(new Uint8Array(await crypto.subtle.digest('SHA-256', bytes))).map((value) => value.toString(16).padStart(2, '0')).join('');
        });
        assert.equal(digest, docHash);
        record[region + ' guidance'] = { url: page.url(), native_document_sha256: digest };
      }

      await page.goto(origin + '/recovery-fixture', { waitUntil: 'domcontentloaded' });
      const roles = page.getByRole('region', { name: 'Roles recovery', exact: true });
      const users = page.getByRole('region', { name: 'User access recovery', exact: true });
      await roles.getByRole('button', { name: 'Retry', exact: true }).waitFor();
      await users.getByRole('button', { name: 'Retry', exact: true }).waitFor();
      for (const [panel, key] of [[roles, 'roles'], [users, 'users']]) {
        const action = panel.getByRole('button', { name: 'Retry', exact: true });
        await inspectAction(action, key + ' retry');
        if (key === 'roles') await capture(`${name}-recovery.png`, true);
        const before = record.requests.filter((request) => request.path === '/api/admin/' + key).length;
        readErrors[key] = false;
        if (width < 768) await action.tap({ position: { x: 6, y: 4 } }); else await page.keyboard.press('Enter');
        await panel.getByText(key === 'roles' ? 'Fixture observer' : 'Fixture operator', { exact: true }).waitFor();
        assert.equal(await panel.getByRole('button', { name: 'Retry', exact: true }).count(), 0);
        const after = record.requests.filter((request) => request.path === '/api/admin/' + key).length;
        assert.equal(after - before, 1, 'one recovery read per activation');
        record[key + '_retry'] = { before, after, result: 'loaded exact synthetic data; no access mutations' };
      }
      for (const tone of ['warning', 'danger', 'info', 'success']) {
        const action = page.getByTestId('tone-' + tone).getByRole('link');
        const metrics = await inspectAction(action, tone + ' long action');
        assert.ok(width > 768 || metrics.height > 16, 'long phone label wraps');
        // Long labels legitimately occupy more than one line on a phone;
        // desktop labels stay one compact line in this production-size card.
      }
      record.inputs = await page.evaluate(() => window.__noticeInputs);
      assert(record.inputs.some((event) => event.trusted && event.label?.includes('Retry')));
      assert.deepEqual(record.errors, []);
      await context.close(); await browser.close(); browser = null;
    }
    result.result = variant === 'parent' ? 'observed-parent-controls' : 'passed';
    result.verified_at = new Date().toISOString();
  } catch (error) { result.error = String(error.stack || error); throw error; }
  finally {
    if (browser) await browser.close(); result.cleanup.browser = true;
    if (server) await new Promise((resolve, reject) => { server.close((error) => error ? reject(error) : resolve()); server.closeAllConnections(); });
    result.cleanup.server = true;
    fs.writeFileSync(output + '/result.json', JSON.stringify(result, null, 2) + '\n');
  }
}
module.exports = { run };
if (require.main === module) run('final').catch((error) => { console.error(error); process.exitCode = 1; });
